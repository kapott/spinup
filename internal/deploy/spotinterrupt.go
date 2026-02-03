// Package deploy provides deployment orchestration for spinup.
package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/tmeurs/spinup/internal/wireguard"
)

// SpotInterruptionReason represents the reason for a spot interruption.
type SpotInterruptionReason string

const (
	// InterruptionReasonPreempted indicates the instance was preempted by the provider.
	InterruptionReasonPreempted SpotInterruptionReason = "preempted"
	// InterruptionReasonSpotPriceExceeded indicates the spot price exceeded the bid.
	InterruptionReasonSpotPriceExceeded SpotInterruptionReason = "spot_price_exceeded"
	// InterruptionReasonCapacityReclaimed indicates capacity was reclaimed.
	InterruptionReasonCapacityReclaimed SpotInterruptionReason = "capacity_reclaimed"
	// InterruptionReasonConnectionLost indicates the connection to the server was lost.
	InterruptionReasonConnectionLost SpotInterruptionReason = "connection_lost"
	// InterruptionReasonUnknown indicates an unknown interruption reason.
	InterruptionReasonUnknown SpotInterruptionReason = "unknown"
)

// SpotInterruption contains information about a spot instance interruption.
type SpotInterruption struct {
	// Reason is the reason for the interruption.
	Reason SpotInterruptionReason

	// Provider is the provider name.
	Provider string

	// InstanceID is the instance ID.
	InstanceID string

	// TerminationTime is when the instance will be terminated (if known).
	TerminationTime *time.Time

	// DetectedAt is when the interruption was detected.
	DetectedAt time.Time

	// SessionCost is the accumulated cost at interruption time.
	SessionCost float64

	// SessionDuration is the session duration at interruption time.
	SessionDuration time.Duration

	// Message is a human-readable message about the interruption.
	Message string
}

// SpotInterruptMonitorConfig holds configuration for the spot interrupt monitor.
type SpotInterruptMonitorConfig struct {
	// ServerIP is the WireGuard IP of the server.
	ServerIP string

	// PollInterval is how often to check for interruption signals.
	PollInterval time.Duration

	// Timeout is the timeout for individual poll requests.
	Timeout time.Duration

	// MaxConsecutiveFailures is the number of consecutive connection failures
	// before considering the instance interrupted due to connection loss.
	MaxConsecutiveFailures int

	// OnInterruption is called when an interruption is detected.
	OnInterruption func(*SpotInterruption)
}

// NewSpotInterruptMonitorConfig creates a config with default values.
func NewSpotInterruptMonitorConfig() *SpotInterruptMonitorConfig {
	return &SpotInterruptMonitorConfig{
		ServerIP:               wireguard.ServerIP,
		PollInterval:           10 * time.Second, // Check every 10 seconds
		Timeout:                5 * time.Second,
		MaxConsecutiveFailures: 3, // After 3 failures (~30s), consider it interrupted
	}
}

// SpotInterruptEndpointPort is the port where the spot interrupt signal server runs.
// This is a simple HTTP server on the instance that reports interruption status.
const SpotInterruptEndpointPort = 51822

// SpotInterruptMonitor monitors for spot instance interruptions.
// It polls the instance for interruption signals and also monitors connection health.
type SpotInterruptMonitor struct {
	config *SpotInterruptMonitorConfig

	mu                  sync.RWMutex
	running             bool
	interrupted         bool
	lastInterruption    *SpotInterruption
	consecutiveFailures int

	cancel context.CancelFunc
	done   chan struct{}

	httpClient *http.Client
}

// NewSpotInterruptMonitor creates a new spot interrupt monitor.
func NewSpotInterruptMonitor(config *SpotInterruptMonitorConfig) (*SpotInterruptMonitor, error) {
	if config == nil {
		config = NewSpotInterruptMonitorConfig()
	}

	if config.PollInterval < time.Second {
		return nil, fmt.Errorf("poll interval must be at least 1 second")
	}
	if config.Timeout < time.Second {
		return nil, fmt.Errorf("timeout must be at least 1 second")
	}
	if config.MaxConsecutiveFailures < 1 {
		return nil, fmt.Errorf("max consecutive failures must be at least 1")
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.Timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        1,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		MaxIdleConnsPerHost: 1,
	}

	return &SpotInterruptMonitor{
		config: config,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
		},
	}, nil
}

// Start begins monitoring for spot interruptions.
func (m *SpotInterruptMonitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("spot interrupt monitor is already running")
	}

	ctx, m.cancel = context.WithCancel(ctx)
	m.running = true
	m.interrupted = false
	m.consecutiveFailures = 0
	m.done = make(chan struct{})
	m.mu.Unlock()

	go m.monitorLoop(ctx)

	return nil
}

// Stop stops the spot interrupt monitor.
func (m *SpotInterruptMonitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	cancel := m.cancel
	done := m.done
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if done != nil {
		<-done
	}
}

// IsRunning returns true if the monitor is running.
func (m *SpotInterruptMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// IsInterrupted returns true if an interruption has been detected.
func (m *SpotInterruptMonitor) IsInterrupted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.interrupted
}

// LastInterruption returns the last detected interruption, if any.
func (m *SpotInterruptMonitor) LastInterruption() *SpotInterruption {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastInterruption
}

// monitorLoop is the main monitoring loop.
func (m *SpotInterruptMonitor) monitorLoop(ctx context.Context) {
	defer func() {
		m.mu.Lock()
		m.running = false
		close(m.done)
		m.mu.Unlock()
	}()

	ticker := time.NewTicker(m.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkForInterruption(ctx)
		}
	}
}

// checkForInterruption checks the server for interruption signals.
func (m *SpotInterruptMonitor) checkForInterruption(ctx context.Context) {
	// Try to get interruption status from the server
	interruption, err := m.pollInterruptionStatus(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		m.consecutiveFailures++

		// If we've exceeded max consecutive failures, consider it a connection loss
		if m.consecutiveFailures >= m.config.MaxConsecutiveFailures && !m.interrupted {
			m.interrupted = true
			m.lastInterruption = &SpotInterruption{
				Reason:     InterruptionReasonConnectionLost,
				DetectedAt: time.Now(),
				Message:    fmt.Sprintf("Connection to instance lost after %d consecutive failures", m.consecutiveFailures),
			}

			if m.config.OnInterruption != nil {
				go m.config.OnInterruption(m.lastInterruption)
			}
		}
		return
	}

	// Reset failure counter on successful connection
	m.consecutiveFailures = 0

	// Check if server reported an interruption
	if interruption != nil && !m.interrupted {
		m.interrupted = true
		m.lastInterruption = interruption

		if m.config.OnInterruption != nil {
			go m.config.OnInterruption(interruption)
		}
	}
}

// spotInterruptResponse is the response from the spot interrupt endpoint.
type spotInterruptResponse struct {
	Interrupted     bool   `json:"interrupted"`
	Reason          string `json:"reason,omitempty"`
	TerminationTime string `json:"termination_time,omitempty"`
	Message         string `json:"message,omitempty"`
}

// pollInterruptionStatus polls the server for interruption status.
func (m *SpotInterruptMonitor) pollInterruptionStatus(ctx context.Context) (*SpotInterruption, error) {
	url := fmt.Sprintf("http://%s:%d/interrupt-status", m.config.ServerIP, SpotInterruptEndpointPort)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var status spotInterruptResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !status.Interrupted {
		return nil, nil // No interruption
	}

	interruption := &SpotInterruption{
		Reason:     parseInterruptionReason(status.Reason),
		DetectedAt: time.Now(),
		Message:    status.Message,
	}

	// Parse termination time if provided
	if status.TerminationTime != "" {
		if t, err := time.Parse(time.RFC3339, status.TerminationTime); err == nil {
			interruption.TerminationTime = &t
		}
	}

	return interruption, nil
}

// parseInterruptionReason parses a reason string into a SpotInterruptionReason.
func parseInterruptionReason(reason string) SpotInterruptionReason {
	switch reason {
	case "preempted", "PREEMPTED":
		return InterruptionReasonPreempted
	case "spot_price_exceeded", "SPOT_PRICE_EXCEEDED":
		return InterruptionReasonSpotPriceExceeded
	case "capacity_reclaimed", "CAPACITY_RECLAIMED":
		return InterruptionReasonCapacityReclaimed
	default:
		return InterruptionReasonUnknown
	}
}

// GenerateSpotInterruptMonitorScript generates the shell script that monitors
// for spot interruptions and serves the status via HTTP.
// This runs on the server (instance) as part of cloud-init.
func GenerateSpotInterruptMonitorScript(port int, provider string) string {
	if port == 0 {
		port = SpotInterruptEndpointPort
	}

	return fmt.Sprintf(`#!/bin/bash
# Spot Interruption Monitor for spinup
# Provider: %s
# Monitors provider metadata service for termination notices
# Serves interruption status via HTTP on port %d

PORT=%d
PROVIDER="%s"
INTERRUPT_FILE="/tmp/spinup-interrupted"
LOG_FILE="/var/log/spinup-spot-monitor.log"

log() {
    echo "$(date -Iseconds) $1" >> "$LOG_FILE"
}

log "Starting spot interruption monitor for provider: $PROVIDER"

# Provider-specific metadata service URLs and detection logic
check_interruption() {
    case "$PROVIDER" in
        vast)
            # Vast.ai doesn't have a metadata service, but we can detect SSH disconnection
            # or check if the instance is being terminated via file marker
            if [ -f "$INTERRUPT_FILE" ]; then
                return 0
            fi
            return 1
            ;;
        runpod)
            # RunPod uses AWS-style metadata service
            # Check for spot termination notice
            TERMINATION_TIME=$(curl -sf --connect-timeout 2 \
                http://169.254.169.254/latest/meta-data/spot/termination-time 2>/dev/null)
            if [ -n "$TERMINATION_TIME" ] && [ "$TERMINATION_TIME" != "null" ]; then
                echo "$TERMINATION_TIME" > /tmp/termination-time
                return 0
            fi
            return 1
            ;;
        coreweave)
            # CoreWeave uses Kubernetes-style preemption
            # Check for preemption via metadata or file marker
            PREEMPTED=$(curl -sf --connect-timeout 2 \
                http://169.254.169.254/v1/spot/preemption-notice 2>/dev/null)
            if [ -n "$PREEMPTED" ] && [ "$PREEMPTED" != "null" ]; then
                return 0
            fi
            if [ -f "$INTERRUPT_FILE" ]; then
                return 0
            fi
            return 1
            ;;
        lambda|paperspace)
            # Lambda and Paperspace don't support spot instances
            # But we still check for the interrupt file marker
            if [ -f "$INTERRUPT_FILE" ]; then
                return 0
            fi
            return 1
            ;;
        *)
            # Generic check - just use file marker
            if [ -f "$INTERRUPT_FILE" ]; then
                return 0
            fi
            return 1
            ;;
    esac
}

# Get termination time if available
get_termination_time() {
    if [ -f /tmp/termination-time ]; then
        cat /tmp/termination-time
    else
        echo ""
    fi
}

# Get interruption reason
get_reason() {
    case "$PROVIDER" in
        vast)
            echo "capacity_reclaimed"
            ;;
        runpod|coreweave)
            echo "preempted"
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

# Background process to continuously monitor for interruption
(
    while true; do
        if check_interruption; then
            if [ ! -f "$INTERRUPT_FILE" ]; then
                TERMINATION_TIME=$(get_termination_time)
                REASON=$(get_reason)
                log "INTERRUPTION DETECTED: reason=$REASON termination_time=$TERMINATION_TIME"
                echo "{\"reason\":\"$REASON\",\"termination_time\":\"$TERMINATION_TIME\"}" > "$INTERRUPT_FILE"
            fi
        fi
        sleep 5
    done
) &
MONITOR_PID=$!
log "Background monitor started with PID: $MONITOR_PID"

# HTTP server using nc (netcat) to serve interruption status
# This is a minimal HTTP server that responds to GET /interrupt-status
while true; do
    # Prepare response based on interruption status
    if [ -f "$INTERRUPT_FILE" ]; then
        INTERRUPT_DATA=$(cat "$INTERRUPT_FILE")
        REASON=$(echo "$INTERRUPT_DATA" | jq -r '.reason // "unknown"' 2>/dev/null || echo "unknown")
        TERM_TIME=$(echo "$INTERRUPT_DATA" | jq -r '.termination_time // ""' 2>/dev/null || echo "")

        RESPONSE_BODY="{\"interrupted\":true,\"reason\":\"$REASON\",\"termination_time\":\"$TERM_TIME\",\"message\":\"Spot instance interruption detected\"}"
    else
        RESPONSE_BODY='{"interrupted":false}'
    fi

    CONTENT_LENGTH=${#RESPONSE_BODY}
    RESPONSE="HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: $CONTENT_LENGTH\r\nConnection: close\r\n\r\n$RESPONSE_BODY"

    # Listen for connections and respond
    echo -e "$RESPONSE" | nc -l -p $PORT -q 1 2>/dev/null | while read line; do
        if echo "$line" | grep -q "GET /interrupt-status"; then
            log "Received interrupt status request"
        fi
    done
done
`, provider, port, port, provider)
}

// SpotInterruptServerScript is an alias for GenerateSpotInterruptMonitorScript.
// Deprecated: Use GenerateSpotInterruptMonitorScript instead.
var SpotInterruptServerScript = GenerateSpotInterruptMonitorScript

// WaitForInterruption blocks until an interruption is detected or the context is cancelled.
// Returns the detected interruption or nil if the context was cancelled.
func (m *SpotInterruptMonitor) WaitForInterruption(ctx context.Context) *SpotInterruption {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if interruption := m.LastInterruption(); interruption != nil {
				return interruption
			}
		}
	}
}

// ForceInterruption forces an interruption (for testing or manual triggering).
func (m *SpotInterruptMonitor) ForceInterruption(reason SpotInterruptionReason, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.interrupted {
		return
	}

	m.interrupted = true
	m.lastInterruption = &SpotInterruption{
		Reason:     reason,
		DetectedAt: time.Now(),
		Message:    message,
	}

	if m.config.OnInterruption != nil {
		go m.config.OnInterruption(m.lastInterruption)
	}
}
