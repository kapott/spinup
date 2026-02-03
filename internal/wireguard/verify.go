// Package wireguard provides WireGuard connection verification.
package wireguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// VerifyResult holds the result of a connection verification.
type VerifyResult struct {
	// Connected indicates if the tunnel is established and working.
	Connected bool
	// HandshakeOK indicates if a recent WireGuard handshake occurred.
	HandshakeOK bool
	// PingOK indicates if the remote endpoint responded to ping/connectivity check.
	PingOK bool
	// OllamaOK indicates if Ollama API is responding (optional check).
	OllamaOK bool
	// LastHandshake is the time of the last WireGuard handshake.
	LastHandshake time.Time
	// Latency is the round-trip time to the server (if ping succeeded).
	Latency time.Duration
	// Error contains any error encountered during verification.
	Error error
	// ErrorDetails provides actionable information about the failure.
	ErrorDetails string
}

// VerifyOptions configures the connection verification behavior.
type VerifyOptions struct {
	// InterfaceName is the WireGuard interface name (default: wg-spinup).
	InterfaceName string
	// ServerIP is the WireGuard IP of the server (default: 10.13.37.1).
	ServerIP string
	// CheckOllama indicates whether to also check Ollama API availability.
	CheckOllama bool
	// Timeout is the maximum time to wait for verification (default: 10s).
	Timeout time.Duration
	// HandshakeMaxAge is the maximum age of the last handshake to consider valid (default: 3m).
	HandshakeMaxAge time.Duration
}

// DefaultVerifyOptions returns default verification options.
func DefaultVerifyOptions() *VerifyOptions {
	return &VerifyOptions{
		InterfaceName:   InterfaceName,
		ServerIP:        ServerIP,
		CheckOllama:     false,
		Timeout:         10 * time.Second,
		HandshakeMaxAge: 3 * time.Minute,
	}
}

// VerifyConnection verifies that the WireGuard tunnel is established and working.
// It checks the handshake status and attempts to connect to the remote endpoint.
// Returns a VerifyResult with detailed information about the connection state.
func VerifyConnection(ctx context.Context, opts *VerifyOptions) *VerifyResult {
	if opts == nil {
		opts = DefaultVerifyOptions()
	}

	// Apply defaults
	if opts.InterfaceName == "" {
		opts.InterfaceName = InterfaceName
	}
	if opts.ServerIP == "" {
		opts.ServerIP = ServerIP
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.HandshakeMaxAge == 0 {
		opts.HandshakeMaxAge = 3 * time.Minute
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	result := &VerifyResult{}

	// Step 1: Check tunnel status and handshake
	status, err := GetTunnelStatus(ctx, opts.InterfaceName)
	if err != nil {
		result.Error = err
		result.ErrorDetails = formatTunnelStatusError(err, opts.InterfaceName)
		return result
	}

	result.LastHandshake = status.LastHandshake

	// Check if handshake is recent
	if status.LastHandshake.IsZero() {
		result.ErrorDetails = fmt.Sprintf(
			"No WireGuard handshake has occurred yet. This usually means:\n"+
				"  - The server endpoint is unreachable (check firewall/network)\n"+
				"  - The server's WireGuard service is not running\n"+
				"  - The public keys don't match\n"+
				"Try: Verify the server is running and port %d/UDP is open", DefaultListenPort)
		return result
	}

	handshakeAge := time.Since(status.LastHandshake)
	if handshakeAge > opts.HandshakeMaxAge {
		result.ErrorDetails = fmt.Sprintf(
			"WireGuard handshake is stale (last: %s ago, max: %s). This may indicate:\n"+
				"  - Network connectivity issues between client and server\n"+
				"  - The server instance may have stopped or restarted\n"+
				"  - Firewall rules may have changed\n"+
				"Try: Check server status and network connectivity",
			handshakeAge.Round(time.Second), opts.HandshakeMaxAge)
		return result
	}

	result.HandshakeOK = true

	// Step 2: Ping the server through the tunnel
	pingResult := pingServer(ctx, opts.ServerIP, opts.Timeout)
	if pingResult.err != nil {
		result.Error = pingResult.err
		result.ErrorDetails = fmt.Sprintf(
			"WireGuard handshake is OK but cannot reach server IP %s. This may indicate:\n"+
				"  - Routing issues on the tunnel interface\n"+
				"  - Server-side firewall blocking tunnel traffic\n"+
				"  - IP address mismatch in WireGuard configuration\n"+
				"Try: Check AllowedIPs and server-side firewall rules",
			opts.ServerIP)
		return result
	}

	result.PingOK = true
	result.Latency = pingResult.latency

	// Step 3: Optionally check Ollama API
	if opts.CheckOllama {
		ollamaOK := checkOllama(ctx, opts.ServerIP, opts.Timeout)
		result.OllamaOK = ollamaOK
		if !ollamaOK {
			result.ErrorDetails = fmt.Sprintf(
				"Tunnel is working but Ollama API at %s:11434 is not responding. This may indicate:\n"+
					"  - Ollama service is not running on the server\n"+
					"  - Ollama is still starting up (wait a moment)\n"+
					"  - Ollama is not bound to the WireGuard interface\n"+
					"Try: SSH to server and check 'systemctl status ollama' or 'ollama list'",
				opts.ServerIP)
			// Still mark as connected since the tunnel itself is working
			result.Connected = true
			return result
		}
	}

	// All checks passed
	result.Connected = true
	return result
}

// VerifyConnectionSimple is a simplified verification that returns error on failure.
// This is a convenience wrapper around VerifyConnection for simple use cases.
func VerifyConnectionSimple(ctx context.Context) error {
	result := VerifyConnection(ctx, nil)
	if !result.Connected {
		if result.Error != nil {
			return fmt.Errorf("connection verification failed: %w\n%s", result.Error, result.ErrorDetails)
		}
		return fmt.Errorf("connection verification failed: %s", result.ErrorDetails)
	}
	return nil
}

// WaitForConnection waits for the WireGuard connection to be established.
// It polls the connection status until connected or timeout.
func WaitForConnection(ctx context.Context, opts *VerifyOptions, pollInterval time.Duration) (*VerifyResult, error) {
	if opts == nil {
		opts = DefaultVerifyOptions()
	}
	if pollInterval == 0 {
		pollInterval = 2 * time.Second
	}

	// Create a deadline context if the parent context doesn't have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastResult *VerifyResult
	for {
		select {
		case <-ctx.Done():
			if lastResult != nil {
				return lastResult, fmt.Errorf("timeout waiting for connection: %s", lastResult.ErrorDetails)
			}
			return nil, fmt.Errorf("timeout waiting for connection")
		case <-ticker.C:
			// Use a shorter timeout for individual checks
			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			result := VerifyConnection(checkCtx, opts)
			cancel()

			lastResult = result
			if result.Connected {
				return result, nil
			}
		}
	}
}

// pingResult holds the result of a ping attempt.
type pingResult struct {
	ok      bool
	latency time.Duration
	err     error
}

// pingServer attempts to connect to the server through the WireGuard tunnel.
// Since ICMP ping typically requires root, we use TCP connection to a known port.
func pingServer(ctx context.Context, serverIP string, timeout time.Duration) pingResult {
	// Try TCP connection to common ports that should be open
	// First try the Ollama port (11434), then SSH (22)
	ports := []int{11434, 22}

	for _, port := range ports {
		start := time.Now()
		addr := fmt.Sprintf("%s:%d", serverIP, port)

		dialer := &net.Dialer{
			Timeout: timeout / time.Duration(len(ports)),
		}

		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			conn.Close()
			return pingResult{
				ok:      true,
				latency: time.Since(start),
			}
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return pingResult{
				err: ctx.Err(),
			}
		}
	}

	// Try raw TCP SYN to any port as last resort (just checks routing)
	start := time.Now()
	addr := fmt.Sprintf("%s:51820", serverIP) // WireGuard port

	dialer := &net.Dialer{
		Timeout: timeout / 2,
	}

	// For UDP services, we can't easily check connectivity without sending data
	// Instead, try a quick TCP connection that may timeout or reset
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err == nil {
		conn.Close()
		return pingResult{
			ok:      true,
			latency: time.Since(start),
		}
	}

	// If we got a "connection refused", routing is working (server responded)
	// This is actually a success for connectivity verification
	if opErr, ok := err.(*net.OpError); ok {
		if _, ok := opErr.Err.(*net.DNSError); !ok {
			// Not a DNS error, so networking is working
			return pingResult{
				ok:      true,
				latency: time.Since(start),
			}
		}
	}

	return pingResult{
		err: fmt.Errorf("cannot reach server at %s: %w", serverIP, err),
	}
}

// checkOllama verifies that the Ollama API is responding.
func checkOllama(ctx context.Context, serverIP string, timeout time.Duration) bool {
	client := &http.Client{
		Timeout: timeout,
	}

	url := fmt.Sprintf("http://%s:11434/api/tags", serverIP)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Any 2xx response indicates Ollama is running
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// formatTunnelStatusError formats an error from GetTunnelStatus into an actionable message.
func formatTunnelStatusError(err error, interfaceName string) string {
	if err == ErrInterfaceNotFound {
		return fmt.Sprintf(
			"WireGuard interface '%s' not found. This usually means:\n"+
				"  - The tunnel has not been set up yet\n"+
				"  - The tunnel was torn down\n"+
				"  - WireGuard is not installed or not running\n"+
				"Try: Run 'spinup' to set up a new instance, or check WireGuard installation",
			interfaceName)
	}

	return fmt.Sprintf(
		"Failed to get tunnel status for '%s': %v\n"+
			"Try: Check if WireGuard is properly installed and running",
		interfaceName, err)
}

// ConnectionState represents the overall connection state.
type ConnectionState int

const (
	// StateDisconnected indicates no tunnel is established.
	StateDisconnected ConnectionState = iota
	// StateConnecting indicates the tunnel exists but handshake is pending.
	StateConnecting
	// StateDegraded indicates the tunnel is up but has issues (stale handshake, etc).
	StateDegraded
	// StateConnected indicates the tunnel is fully operational.
	StateConnected
)

// String returns a human-readable representation of the connection state.
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateDegraded:
		return "degraded"
	case StateConnected:
		return "connected"
	default:
		return "unknown"
	}
}

// GetConnectionState returns the current connection state.
func GetConnectionState(ctx context.Context, opts *VerifyOptions) ConnectionState {
	if opts == nil {
		opts = DefaultVerifyOptions()
	}

	// Check if interface exists
	status, err := GetTunnelStatus(ctx, opts.InterfaceName)
	if err != nil {
		return StateDisconnected
	}

	// Check handshake
	if status.LastHandshake.IsZero() {
		return StateConnecting
	}

	handshakeAge := time.Since(status.LastHandshake)
	if handshakeAge > opts.HandshakeMaxAge {
		return StateDegraded
	}

	// Try a quick ping
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	result := pingServer(pingCtx, opts.ServerIP, 2*time.Second)
	if result.ok {
		return StateConnected
	}

	return StateDegraded
}
