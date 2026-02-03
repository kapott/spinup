// Package deploy provides deployment orchestration for continueplz.
package deploy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/tmeurs/continueplz/internal/wireguard"
)

// HeartbeatConfig holds configuration for the heartbeat client.
type HeartbeatConfig struct {
	// Interval is the time between heartbeat sends.
	// Default is 5 minutes as specified in PRD.
	Interval time.Duration

	// HeartbeatFile is the path to touch on the remote instance.
	// Default is /tmp/continueplz-heartbeat.
	HeartbeatFile string

	// ServerIP is the WireGuard IP of the server.
	// Default is 10.13.37.1.
	ServerIP string

	// Timeout is the maximum time to wait for a heartbeat request.
	// Default is 30 seconds.
	Timeout time.Duration

	// OnFailure is called when a heartbeat fails.
	// The callback receives the error and the number of consecutive failures.
	OnFailure func(err error, consecutiveFailures int)

	// OnSuccess is called when a heartbeat succeeds.
	// The callback receives the number of previous consecutive failures (0 if none).
	OnSuccess func(previousFailures int)

	// MaxConsecutiveFailures is the maximum number of consecutive failures before
	// escalating to critical status. Default is 3.
	MaxConsecutiveFailures int
}

// HeartbeatClient manages the heartbeat goroutine that keeps the deadman switch alive.
type HeartbeatClient struct {
	config *HeartbeatConfig

	// mu protects the following fields
	mu                   sync.RWMutex
	running              bool
	lastHeartbeat        time.Time
	lastError            error
	consecutiveFailures  int
	totalHeartbeats      int
	totalFailures        int

	// cancel is used to stop the heartbeat goroutine
	cancel context.CancelFunc

	// done signals when the heartbeat goroutine has stopped
	done chan struct{}

	// httpClient is used for HTTP-based heartbeat
	httpClient *http.Client
}

// HeartbeatStatus represents the current status of the heartbeat client.
type HeartbeatStatus struct {
	// Running indicates if the heartbeat goroutine is active.
	Running bool

	// LastHeartbeat is the time of the last successful heartbeat.
	LastHeartbeat time.Time

	// LastError is the most recent error (nil if last heartbeat succeeded).
	LastError error

	// ConsecutiveFailures is the number of consecutive failed heartbeats.
	ConsecutiveFailures int

	// TotalHeartbeats is the total number of successful heartbeats sent.
	TotalHeartbeats int

	// TotalFailures is the total number of failed heartbeat attempts.
	TotalFailures int

	// NextHeartbeat is the expected time of the next heartbeat.
	NextHeartbeat time.Time

	// Healthy indicates if the heartbeat system is functioning normally.
	// False if there are consecutive failures or the client is not running.
	Healthy bool
}

// Constants for heartbeat configuration.
const (
	// DefaultHeartbeatInterval is the default interval between heartbeats (5 minutes).
	DefaultHeartbeatInterval = 5 * time.Minute

	// DefaultHeartbeatTimeout is the default timeout for heartbeat requests.
	DefaultHeartbeatTimeout = 30 * time.Second

	// DefaultMaxConsecutiveFailures is the default max failures before critical.
	DefaultMaxConsecutiveFailures = 3

	// HeartbeatEndpointPort is the port for the heartbeat HTTP endpoint on the server.
	// This is a simple HTTP server that touches the heartbeat file when called.
	HeartbeatEndpointPort = 51821
)

// NewHeartbeatConfig creates a new HeartbeatConfig with default values.
func NewHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Interval:               DefaultHeartbeatInterval,
		HeartbeatFile:          DefaultHeartbeatFile,
		ServerIP:               wireguard.ServerIP,
		Timeout:                DefaultHeartbeatTimeout,
		MaxConsecutiveFailures: DefaultMaxConsecutiveFailures,
	}
}

// Validate validates the heartbeat configuration.
func (c *HeartbeatConfig) Validate() error {
	if c.Interval <= 0 {
		return fmt.Errorf("heartbeat interval must be positive")
	}
	if c.Interval < 30*time.Second {
		return fmt.Errorf("heartbeat interval must be at least 30 seconds")
	}
	if c.HeartbeatFile == "" {
		return fmt.Errorf("heartbeat file path is required")
	}
	if c.ServerIP == "" {
		return fmt.Errorf("server IP is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if c.MaxConsecutiveFailures <= 0 {
		return fmt.Errorf("max consecutive failures must be positive")
	}
	return nil
}

// NewHeartbeatClient creates a new HeartbeatClient.
func NewHeartbeatClient(config *HeartbeatConfig) (*HeartbeatClient, error) {
	if config == nil {
		config = NewHeartbeatConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid heartbeat config: %w", err)
	}

	// Create HTTP client with appropriate timeout and dialer
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

	return &HeartbeatClient{
		config: config,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
		},
	}, nil
}

// Start begins the heartbeat goroutine.
// Returns an error if the client is already running.
func (hc *HeartbeatClient) Start(ctx context.Context) error {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return fmt.Errorf("heartbeat client is already running")
	}

	// Create cancellable context for the heartbeat goroutine
	ctx, hc.cancel = context.WithCancel(ctx)
	hc.running = true
	hc.done = make(chan struct{})
	hc.mu.Unlock()

	// Send initial heartbeat immediately
	hc.sendHeartbeat(ctx)

	// Start the heartbeat goroutine
	go hc.heartbeatLoop(ctx)

	return nil
}

// Stop stops the heartbeat goroutine.
// This method blocks until the goroutine has fully stopped.
func (hc *HeartbeatClient) Stop() {
	hc.mu.Lock()
	if !hc.running {
		hc.mu.Unlock()
		return
	}
	cancel := hc.cancel
	done := hc.done
	hc.mu.Unlock()

	// Cancel the context to signal stop
	if cancel != nil {
		cancel()
	}

	// Wait for the goroutine to finish
	if done != nil {
		<-done
	}
}

// Status returns the current status of the heartbeat client.
func (hc *HeartbeatClient) Status() *HeartbeatStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status := &HeartbeatStatus{
		Running:             hc.running,
		LastHeartbeat:       hc.lastHeartbeat,
		LastError:           hc.lastError,
		ConsecutiveFailures: hc.consecutiveFailures,
		TotalHeartbeats:     hc.totalHeartbeats,
		TotalFailures:       hc.totalFailures,
	}

	// Calculate next heartbeat time
	if hc.running && !hc.lastHeartbeat.IsZero() {
		status.NextHeartbeat = hc.lastHeartbeat.Add(hc.config.Interval)
	}

	// Determine health status
	status.Healthy = hc.running && hc.consecutiveFailures < hc.config.MaxConsecutiveFailures

	return status
}

// IsRunning returns true if the heartbeat client is currently running.
func (hc *HeartbeatClient) IsRunning() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.running
}

// heartbeatLoop is the main loop that sends heartbeats at regular intervals.
func (hc *HeartbeatClient) heartbeatLoop(ctx context.Context) {
	defer func() {
		hc.mu.Lock()
		hc.running = false
		close(hc.done)
		hc.mu.Unlock()
	}()

	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.sendHeartbeat(ctx)
		}
	}
}

// sendHeartbeat sends a single heartbeat to the server.
func (hc *HeartbeatClient) sendHeartbeat(ctx context.Context) {
	err := hc.doHeartbeat(ctx)

	hc.mu.Lock()
	if err != nil {
		hc.lastError = err
		hc.consecutiveFailures++
		hc.totalFailures++

		failures := hc.consecutiveFailures
		callback := hc.config.OnFailure
		hc.mu.Unlock()

		if callback != nil {
			callback(err, failures)
		}
	} else {
		previousFailures := hc.consecutiveFailures
		hc.lastHeartbeat = time.Now()
		hc.lastError = nil
		hc.consecutiveFailures = 0
		hc.totalHeartbeats++

		callback := hc.config.OnSuccess
		hc.mu.Unlock()

		if callback != nil && previousFailures > 0 {
			callback(previousFailures)
		}
	}
}

// doHeartbeat performs the actual heartbeat request.
// It tries multiple methods to touch the heartbeat file on the server.
func (hc *HeartbeatClient) doHeartbeat(ctx context.Context) error {
	// Method 1: Try HTTP endpoint (preferred, if server has heartbeat service)
	err := hc.doHTTPHeartbeat(ctx)
	if err == nil {
		return nil
	}

	// Method 2: Try TCP connection to SSH port as fallback connectivity check
	// This verifies the tunnel is working but doesn't update the heartbeat file
	// The server's heartbeat file should be updated by the HTTP endpoint
	tcpErr := hc.doTCPHeartbeat(ctx)
	if tcpErr == nil {
		// TCP works but HTTP failed - this is a partial success
		// Return the HTTP error so caller knows heartbeat file wasn't updated
		return fmt.Errorf("tunnel is up but heartbeat service unavailable: %w", err)
	}

	// Both methods failed
	return fmt.Errorf("heartbeat failed: HTTP: %v, TCP: %v", err, tcpErr)
}

// doHTTPHeartbeat sends a heartbeat via HTTP to the heartbeat service on the server.
// The server should have a simple HTTP endpoint that touches the heartbeat file.
func (hc *HeartbeatClient) doHTTPHeartbeat(ctx context.Context) error {
	url := fmt.Sprintf("http://%s:%d/heartbeat", hc.config.ServerIP, HeartbeatEndpointPort)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}

	resp, err := hc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("heartbeat request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read and discard the response body
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("heartbeat request returned status %d", resp.StatusCode)
	}

	return nil
}

// doTCPHeartbeat verifies connectivity by attempting a TCP connection to a known port.
// This is a fallback to verify the tunnel is working when HTTP fails.
func (hc *HeartbeatClient) doTCPHeartbeat(ctx context.Context) error {
	// Try connecting to common ports
	ports := []int{22, 11434} // SSH and Ollama

	var lastErr error
	for _, port := range ports {
		addr := fmt.Sprintf("%s:%d", hc.config.ServerIP, port)

		dialer := &net.Dialer{
			Timeout: hc.config.Timeout / time.Duration(len(ports)),
		}

		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			conn.Close()
			return nil
		}
		lastErr = err

		// Check context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return fmt.Errorf("failed to connect to server: %w", lastErr)
}

// SendHeartbeatNow sends an immediate heartbeat outside of the regular schedule.
// This can be used to force a heartbeat after a long operation or to test connectivity.
func (hc *HeartbeatClient) SendHeartbeatNow(ctx context.Context) error {
	hc.mu.RLock()
	if !hc.running {
		hc.mu.RUnlock()
		return fmt.Errorf("heartbeat client is not running")
	}
	hc.mu.RUnlock()

	err := hc.doHeartbeat(ctx)

	hc.mu.Lock()
	if err != nil {
		hc.lastError = err
		hc.consecutiveFailures++
		hc.totalFailures++
	} else {
		hc.lastHeartbeat = time.Now()
		hc.lastError = nil
		hc.consecutiveFailures = 0
		hc.totalHeartbeats++
	}
	hc.mu.Unlock()

	return err
}

// TimeSinceLastHeartbeat returns the duration since the last successful heartbeat.
// Returns 0 if no heartbeat has been sent yet.
func (hc *HeartbeatClient) TimeSinceLastHeartbeat() time.Duration {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if hc.lastHeartbeat.IsZero() {
		return 0
	}
	return time.Since(hc.lastHeartbeat)
}

// IsCritical returns true if the heartbeat system is in a critical state.
// This occurs when consecutive failures exceed the maximum threshold.
func (hc *HeartbeatClient) IsCritical() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.consecutiveFailures >= hc.config.MaxConsecutiveFailures
}

// HeartbeatServerHandler returns an HTTP handler that can be used on the server
// to receive heartbeats and touch the heartbeat file.
// This is included here for reference - the actual server runs in the cloud-init script.
func HeartbeatServerHandler(heartbeatFile string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This would touch the heartbeat file
		// In practice, this runs on the server via cloud-init
		// The actual implementation would be:
		//   if err := os.WriteFile(heartbeatFile, []byte(time.Now().String()), 0644); err != nil {
		//     http.Error(w, err.Error(), http.StatusInternalServerError)
		//     return
		//   }
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
}

// GenerateHeartbeatServerScript generates the shell script that runs
// the heartbeat HTTP server on the instance. This is included in cloud-init.
func GenerateHeartbeatServerScript(port int, heartbeatFile string) string {
	if port == 0 {
		port = HeartbeatEndpointPort
	}
	if heartbeatFile == "" {
		heartbeatFile = DefaultHeartbeatFile
	}

	return fmt.Sprintf(`#!/bin/bash
# Heartbeat server for continueplz
# Listens on port %d and touches %s on POST /heartbeat

HEARTBEAT_FILE="%s"
PORT=%d

while true; do
    # Use nc (netcat) to create a simple HTTP server
    echo -e "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK" | nc -l -p $PORT -q 1 | while read line; do
        # Check if this is a POST to /heartbeat
        if echo "$line" | grep -q "POST /heartbeat"; then
            touch "$HEARTBEAT_FILE"
        fi
    done
done
`, port, heartbeatFile, heartbeatFile, port)
}
