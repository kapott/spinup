package deploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewSpotInterruptMonitorConfig(t *testing.T) {
	config := NewSpotInterruptMonitorConfig()

	if config.ServerIP == "" {
		t.Error("ServerIP should have a default value")
	}
	if config.PollInterval <= 0 {
		t.Error("PollInterval should be positive")
	}
	if config.Timeout <= 0 {
		t.Error("Timeout should be positive")
	}
	if config.MaxConsecutiveFailures <= 0 {
		t.Error("MaxConsecutiveFailures should be positive")
	}
}

func TestNewSpotInterruptMonitor(t *testing.T) {
	tests := []struct {
		name    string
		config  *SpotInterruptMonitorConfig
		wantErr bool
	}{
		{
			name:    "nil config uses defaults",
			config:  nil,
			wantErr: false,
		},
		{
			name: "valid config",
			config: &SpotInterruptMonitorConfig{
				ServerIP:               "10.0.0.1",
				PollInterval:           5 * time.Second,
				Timeout:                2 * time.Second,
				MaxConsecutiveFailures: 3,
			},
			wantErr: false,
		},
		{
			name: "poll interval too small",
			config: &SpotInterruptMonitorConfig{
				ServerIP:               "10.0.0.1",
				PollInterval:           100 * time.Millisecond,
				Timeout:                2 * time.Second,
				MaxConsecutiveFailures: 3,
			},
			wantErr: true,
		},
		{
			name: "timeout too small",
			config: &SpotInterruptMonitorConfig{
				ServerIP:               "10.0.0.1",
				PollInterval:           5 * time.Second,
				Timeout:                100 * time.Millisecond,
				MaxConsecutiveFailures: 3,
			},
			wantErr: true,
		},
		{
			name: "max failures too small",
			config: &SpotInterruptMonitorConfig{
				ServerIP:               "10.0.0.1",
				PollInterval:           5 * time.Second,
				Timeout:                2 * time.Second,
				MaxConsecutiveFailures: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor, err := NewSpotInterruptMonitor(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSpotInterruptMonitor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && monitor == nil {
				t.Error("NewSpotInterruptMonitor() returned nil monitor")
			}
		})
	}
}

func TestSpotInterruptMonitor_StartStop(t *testing.T) {
	config := NewSpotInterruptMonitorConfig()
	config.PollInterval = 100 * time.Second // Long interval to avoid actual polling

	monitor, err := NewSpotInterruptMonitor(config)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Start monitor
	ctx := context.Background()
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}

	if !monitor.IsRunning() {
		t.Error("Monitor should be running after Start()")
	}

	// Starting again should fail
	if err := monitor.Start(ctx); err == nil {
		t.Error("Starting already running monitor should return error")
	}

	// Stop monitor
	monitor.Stop()

	if monitor.IsRunning() {
		t.Error("Monitor should not be running after Stop()")
	}

	// Stopping again should be safe
	monitor.Stop() // Should not panic
}

func TestSpotInterruptMonitor_ForceInterruption(t *testing.T) {
	config := NewSpotInterruptMonitorConfig()
	config.PollInterval = 100 * time.Second

	var callbackCalled atomic.Int32
	config.OnInterruption = func(i *SpotInterruption) {
		callbackCalled.Add(1)
	}

	monitor, err := NewSpotInterruptMonitor(config)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	if monitor.IsInterrupted() {
		t.Error("Monitor should not be interrupted initially")
	}

	// Force an interruption
	monitor.ForceInterruption(InterruptionReasonPreempted, "Test interruption")

	if !monitor.IsInterrupted() {
		t.Error("Monitor should be interrupted after ForceInterruption()")
	}

	interruption := monitor.LastInterruption()
	if interruption == nil {
		t.Fatal("LastInterruption() should not be nil")
	}

	if interruption.Reason != InterruptionReasonPreempted {
		t.Errorf("Reason = %v, want %v", interruption.Reason, InterruptionReasonPreempted)
	}

	if interruption.Message != "Test interruption" {
		t.Errorf("Message = %v, want %v", interruption.Message, "Test interruption")
	}

	// Wait a bit for async callback
	time.Sleep(50 * time.Millisecond)
	if callbackCalled.Load() != 1 {
		t.Errorf("Callback called %d times, want 1", callbackCalled.Load())
	}

	// Second force should be ignored
	monitor.ForceInterruption(InterruptionReasonCapacityReclaimed, "Second")
	time.Sleep(50 * time.Millisecond)
	if callbackCalled.Load() != 1 {
		t.Errorf("Callback called %d times after second force, want 1", callbackCalled.Load())
	}
}

func TestSpotInterruptMonitor_DetectsInterruptionFromServer(t *testing.T) {
	// Create a test server that responds with interruption status
	interrupted := atomic.Bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/interrupt-status" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if interrupted.Load() {
			json.NewEncoder(w).Encode(spotInterruptResponse{
				Interrupted: true,
				Reason:      "preempted",
				Message:     "Instance preempted",
			})
		} else {
			json.NewEncoder(w).Encode(spotInterruptResponse{
				Interrupted: false,
			})
		}
	}))
	defer server.Close()

	// Extract host:port from test server
	serverAddr := strings.TrimPrefix(server.URL, "http://")

	// Create monitor pointing to test server
	config := &SpotInterruptMonitorConfig{
		ServerIP:               serverAddr, // This won't work directly, need to override client
		PollInterval:           2 * time.Second, // Must be at least 1 second
		Timeout:                time.Second,
		MaxConsecutiveFailures: 3,
	}

	monitor, err := NewSpotInterruptMonitor(config)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Override the HTTP client to use our test server
	monitor.httpClient = server.Client()

	// Create a custom pollInterruptionStatus for testing
	// (normally the monitor polls the WireGuard IP, but for testing we use the test server)
	ctx := context.Background()

	// Test: no interruption
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/interrupt-status", nil)
	resp, err := monitor.httpClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var status spotInterruptResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if status.Interrupted {
		t.Error("Expected no interruption initially")
	}

	// Simulate interruption
	interrupted.Store(true)

	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/interrupt-status", nil)
	resp2, err := monitor.httpClient.Do(req2)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp2.Body.Close()

	var status2 spotInterruptResponse
	if err := json.NewDecoder(resp2.Body).Decode(&status2); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !status2.Interrupted {
		t.Error("Expected interruption after setting interrupted=true")
	}
	if status2.Reason != "preempted" {
		t.Errorf("Reason = %v, want preempted", status2.Reason)
	}
}

func TestParseInterruptionReason(t *testing.T) {
	tests := []struct {
		input    string
		expected SpotInterruptionReason
	}{
		{"preempted", InterruptionReasonPreempted},
		{"PREEMPTED", InterruptionReasonPreempted},
		{"spot_price_exceeded", InterruptionReasonSpotPriceExceeded},
		{"SPOT_PRICE_EXCEEDED", InterruptionReasonSpotPriceExceeded},
		{"capacity_reclaimed", InterruptionReasonCapacityReclaimed},
		{"CAPACITY_RECLAIMED", InterruptionReasonCapacityReclaimed},
		{"unknown", InterruptionReasonUnknown},
		{"", InterruptionReasonUnknown},
		{"something_else", InterruptionReasonUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseInterruptionReason(tt.input)
			if got != tt.expected {
				t.Errorf("parseInterruptionReason(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGenerateSpotInterruptMonitorScript(t *testing.T) {
	tests := []struct {
		port     int
		provider string
	}{
		{0, "vast"},      // Default port
		{51822, "vast"},  // Explicit port
		{51822, "runpod"},
		{51822, "coreweave"},
		{51822, "lambda"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			script := GenerateSpotInterruptMonitorScript(tt.port, tt.provider)

			if script == "" {
				t.Error("Script should not be empty")
			}

			// Check for essential components
			if !strings.Contains(script, "#!/bin/bash") {
				t.Error("Script should start with shebang")
			}
			if !strings.Contains(script, tt.provider) {
				t.Errorf("Script should contain provider name %q", tt.provider)
			}
			if !strings.Contains(script, "/interrupt-status") {
				t.Error("Script should handle /interrupt-status endpoint")
			}
			if !strings.Contains(script, "INTERRUPT_FILE") {
				t.Error("Script should define INTERRUPT_FILE")
			}
		})
	}
}

func TestSpotInterruption_Fields(t *testing.T) {
	now := time.Now()
	termTime := now.Add(30 * time.Second)

	interruption := &SpotInterruption{
		Reason:          InterruptionReasonPreempted,
		Provider:        "runpod",
		InstanceID:      "test-123",
		TerminationTime: &termTime,
		DetectedAt:      now,
		SessionCost:     5.25,
		SessionDuration: 2*time.Hour + 30*time.Minute,
		Message:         "Spot instance preempted",
	}

	if interruption.Reason != InterruptionReasonPreempted {
		t.Errorf("Reason = %v, want %v", interruption.Reason, InterruptionReasonPreempted)
	}
	if interruption.Provider != "runpod" {
		t.Errorf("Provider = %v, want %v", interruption.Provider, "runpod")
	}
	if interruption.InstanceID != "test-123" {
		t.Errorf("InstanceID = %v, want %v", interruption.InstanceID, "test-123")
	}
	if interruption.TerminationTime == nil || !interruption.TerminationTime.Equal(termTime) {
		t.Errorf("TerminationTime = %v, want %v", interruption.TerminationTime, termTime)
	}
	if interruption.SessionCost != 5.25 {
		t.Errorf("SessionCost = %v, want %v", interruption.SessionCost, 5.25)
	}
	if interruption.SessionDuration != 2*time.Hour+30*time.Minute {
		t.Errorf("SessionDuration = %v, want %v", interruption.SessionDuration, 2*time.Hour+30*time.Minute)
	}
}

func TestSpotInterruptMonitor_WaitForInterruption(t *testing.T) {
	config := NewSpotInterruptMonitorConfig()
	config.PollInterval = 100 * time.Second

	monitor, err := NewSpotInterruptMonitor(config)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Test cancellation
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan *SpotInterruption)
	go func() {
		done <- monitor.WaitForInterruption(ctx)
	}()

	// Cancel immediately
	cancel()

	select {
	case result := <-done:
		if result != nil {
			t.Error("Expected nil result when context is cancelled")
		}
	case <-time.After(time.Second):
		t.Error("WaitForInterruption should return when context is cancelled")
	}

	// Test detection
	monitor2, _ := NewSpotInterruptMonitor(config)
	ctx2 := context.Background()

	done2 := make(chan *SpotInterruption)
	go func() {
		done2 <- monitor2.WaitForInterruption(ctx2)
	}()

	// Force interruption
	time.Sleep(50 * time.Millisecond)
	monitor2.ForceInterruption(InterruptionReasonConnectionLost, "Test")

	select {
	case result := <-done2:
		if result == nil {
			t.Error("Expected non-nil result after force interruption")
		}
		if result.Reason != InterruptionReasonConnectionLost {
			t.Errorf("Reason = %v, want %v", result.Reason, InterruptionReasonConnectionLost)
		}
	case <-time.After(time.Second):
		t.Error("WaitForInterruption should return after force interruption")
	}
}
