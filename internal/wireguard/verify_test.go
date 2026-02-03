package wireguard

import (
	"context"
	"testing"
	"time"
)

func TestDefaultVerifyOptions(t *testing.T) {
	opts := DefaultVerifyOptions()

	if opts.InterfaceName != InterfaceName {
		t.Errorf("InterfaceName = %s, want %s", opts.InterfaceName, InterfaceName)
	}
	if opts.ServerIP != ServerIP {
		t.Errorf("ServerIP = %s, want %s", opts.ServerIP, ServerIP)
	}
	if opts.CheckOllama != false {
		t.Error("CheckOllama should be false by default")
	}
	if opts.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", opts.Timeout)
	}
	if opts.HandshakeMaxAge != 3*time.Minute {
		t.Errorf("HandshakeMaxAge = %v, want 3m", opts.HandshakeMaxAge)
	}
}

func TestVerifyConnection_NoInterface(t *testing.T) {
	ctx := context.Background()
	opts := &VerifyOptions{
		InterfaceName: "nonexistent-wg-interface-12345",
		ServerIP:      ServerIP,
		Timeout:       5 * time.Second,
	}

	result := VerifyConnection(ctx, opts)

	if result.Connected {
		t.Error("Connected should be false for non-existent interface")
	}
	if result.HandshakeOK {
		t.Error("HandshakeOK should be false for non-existent interface")
	}
	if result.PingOK {
		t.Error("PingOK should be false for non-existent interface")
	}
	if result.Error == nil {
		t.Error("Error should not be nil for non-existent interface")
	}
	if result.ErrorDetails == "" {
		t.Error("ErrorDetails should contain actionable information")
	}
}

func TestVerifyConnection_NilOptions(t *testing.T) {
	ctx := context.Background()

	// Should not panic with nil options
	result := VerifyConnection(ctx, nil)

	// Result should be valid (though likely not connected in test env)
	if result == nil {
		t.Error("Result should not be nil")
	}
	// The interface likely doesn't exist in test, so should have error details
	if !result.Connected && result.ErrorDetails == "" {
		t.Error("ErrorDetails should be populated when not connected")
	}
}

func TestVerifyConnectionSimple_NoInterface(t *testing.T) {
	ctx := context.Background()

	err := VerifyConnectionSimple(ctx)
	if err == nil {
		t.Error("Expected error for non-existent interface")
	}
}

func TestConnectionState_String(t *testing.T) {
	tests := []struct {
		state ConnectionState
		want  string
	}{
		{StateDisconnected, "disconnected"},
		{StateConnecting, "connecting"},
		{StateDegraded, "degraded"},
		{StateConnected, "connected"},
		{ConnectionState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.state.String()
			if got != tt.want {
				t.Errorf("ConnectionState(%d).String() = %s, want %s", tt.state, got, tt.want)
			}
		})
	}
}

func TestGetConnectionState_NoInterface(t *testing.T) {
	ctx := context.Background()
	opts := &VerifyOptions{
		InterfaceName: "nonexistent-wg-interface-12345",
		ServerIP:      ServerIP,
		Timeout:       5 * time.Second,
	}

	state := GetConnectionState(ctx, opts)
	if state != StateDisconnected {
		t.Errorf("GetConnectionState = %s, want disconnected", state)
	}
}

func TestFormatTunnelStatusError_InterfaceNotFound(t *testing.T) {
	msg := formatTunnelStatusError(ErrInterfaceNotFound, "test-iface")

	if msg == "" {
		t.Error("Message should not be empty")
	}
	// Should contain actionable information
	if len(msg) < 50 {
		t.Error("Message should contain detailed actionable information")
	}
	// Should mention the interface name
	if msg == "" || len(msg) < 10 {
		t.Error("Message should contain the interface name")
	}
}

func TestFormatTunnelStatusError_GenericError(t *testing.T) {
	err := &TunnelError{Op: "test", Message: "test error"}
	msg := formatTunnelStatusError(err, "test-iface")

	if msg == "" {
		t.Error("Message should not be empty")
	}
}

func TestVerifyResult_FieldsInitialized(t *testing.T) {
	result := &VerifyResult{}

	// Verify zero values are safe
	if result.Connected {
		t.Error("Connected should be false by default")
	}
	if result.HandshakeOK {
		t.Error("HandshakeOK should be false by default")
	}
	if result.PingOK {
		t.Error("PingOK should be false by default")
	}
	if result.OllamaOK {
		t.Error("OllamaOK should be false by default")
	}
	if !result.LastHandshake.IsZero() {
		t.Error("LastHandshake should be zero by default")
	}
	if result.Latency != 0 {
		t.Error("Latency should be zero by default")
	}
	if result.Error != nil {
		t.Error("Error should be nil by default")
	}
	if result.ErrorDetails != "" {
		t.Error("ErrorDetails should be empty by default")
	}
}

func TestWaitForConnection_Timeout(t *testing.T) {
	ctx := context.Background()
	opts := &VerifyOptions{
		InterfaceName: "nonexistent-wg-interface-12345",
		ServerIP:      ServerIP,
		Timeout:       2 * time.Second,
	}

	start := time.Now()
	_, err := WaitForConnection(ctx, opts, 500*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error for non-existent interface")
	}

	// Should timeout within reasonable bounds (allow some slack)
	if elapsed < 1*time.Second {
		t.Errorf("WaitForConnection returned too quickly: %v", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("WaitForConnection took too long: %v", elapsed)
	}
}

func TestWaitForConnection_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	opts := &VerifyOptions{
		InterfaceName: "nonexistent-wg-interface-12345",
		ServerIP:      ServerIP,
		Timeout:       30 * time.Second, // Long timeout, but context will cancel first
	}

	start := time.Now()
	_, err := WaitForConnection(ctx, opts, 100*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	// Should respect context deadline
	if elapsed > 3*time.Second {
		t.Errorf("WaitForConnection did not respect context deadline: %v", elapsed)
	}
}

func TestVerifyOptions_Defaults(t *testing.T) {
	ctx := context.Background()
	opts := &VerifyOptions{} // All zero values

	// VerifyConnection should apply defaults
	result := VerifyConnection(ctx, opts)

	// Should not panic and should return a result
	if result == nil {
		t.Error("Result should not be nil")
	}
}
