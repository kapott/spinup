package alert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewWebhookClient(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		wantNil    bool
	}{
		{
			name:       "valid URL",
			webhookURL: "https://hooks.slack.com/services/xxx",
			wantNil:    false,
		},
		{
			name:       "empty URL returns nil",
			webhookURL: "",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewWebhookClient(tt.webhookURL)
			if (client == nil) != tt.wantNil {
				t.Errorf("NewWebhookClient(%q) returned nil=%v, want nil=%v", tt.webhookURL, client == nil, tt.wantNil)
			}
		})
	}
}

func TestWebhookClient_SendAlert(t *testing.T) {
	var receivedPayload WebhookPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("User-Agent") != "continueplz/1.0" {
			t.Errorf("expected User-Agent continueplz/1.0, got %s", r.Header.Get("User-Agent"))
		}

		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		if err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewWebhookClient(server.URL)
	ctx := context.Background()

	alertCtx := Context{
		InstanceID: "12345678",
		Provider:   "vast.ai",
		Action:     "stop",
	}

	err := client.SendAlert(ctx, LevelCritical, "Could not verify billing stopped", alertCtx)
	if err != nil {
		t.Errorf("SendAlert returned error: %v", err)
	}

	// Verify payload matches PRD Section 8.2 format
	if receivedPayload.Level != LevelCritical {
		t.Errorf("expected level CRITICAL, got %s", receivedPayload.Level)
	}
	if receivedPayload.Message != "Could not verify billing stopped" {
		t.Errorf("expected message 'Could not verify billing stopped', got %s", receivedPayload.Message)
	}
	if receivedPayload.Context.InstanceID != "12345678" {
		t.Errorf("expected instance_id 12345678, got %s", receivedPayload.Context.InstanceID)
	}
	if receivedPayload.Context.Provider != "vast.ai" {
		t.Errorf("expected provider vast.ai, got %s", receivedPayload.Context.Provider)
	}
	if receivedPayload.Context.Action != "stop" {
		t.Errorf("expected action stop, got %s", receivedPayload.Context.Action)
	}

	// Verify timestamp is valid RFC3339
	_, err = time.Parse(time.RFC3339, receivedPayload.Timestamp)
	if err != nil {
		t.Errorf("timestamp is not valid RFC3339: %s", receivedPayload.Timestamp)
	}
}

func TestWebhookClient_SendAlert_NilClient(t *testing.T) {
	var client *WebhookClient = nil
	ctx := context.Background()

	// Should not panic and return nil
	err := client.SendAlert(ctx, LevelCritical, "test", Context{})
	if err != nil {
		t.Errorf("expected nil error for nil client, got %v", err)
	}
}

func TestWebhookClient_SendAlert_NetworkFailure(t *testing.T) {
	// Server that immediately closes connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Close without responding to simulate network failure
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer server.Close()

	client := NewWebhookClient(server.URL)
	ctx := context.Background()

	// Should not return error - network failures are handled gracefully
	err := client.SendAlert(ctx, LevelCritical, "test", Context{})
	if err != nil {
		t.Errorf("expected nil error for network failure, got %v", err)
	}
}

func TestWebhookClient_SendAlert_NonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewWebhookClient(server.URL)
	ctx := context.Background()

	// Should not return error - non-success status is handled gracefully
	err := client.SendAlert(ctx, LevelCritical, "test", Context{})
	if err != nil {
		t.Errorf("expected nil error for non-success status, got %v", err)
	}
}

func TestWebhookClient_ConvenienceMethods(t *testing.T) {
	var receivedLevels []Level

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookPayload
		json.NewDecoder(r.Body).Decode(&payload)
		receivedLevels = append(receivedLevels, payload.Level)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewWebhookClient(server.URL)
	ctx := context.Background()
	alertCtx := Context{}

	client.SendCritical(ctx, "critical message", alertCtx)
	client.SendError(ctx, "error message", alertCtx)
	client.SendWarn(ctx, "warn message", alertCtx)
	client.SendInfo(ctx, "info message", alertCtx)

	expectedLevels := []Level{LevelCritical, LevelError, LevelWarn, LevelInfo}
	if len(receivedLevels) != len(expectedLevels) {
		t.Fatalf("expected %d alerts, got %d", len(expectedLevels), len(receivedLevels))
	}

	for i, expected := range expectedLevels {
		if receivedLevels[i] != expected {
			t.Errorf("alert %d: expected level %s, got %s", i, expected, receivedLevels[i])
		}
	}
}

func TestGlobalWebhookClient(t *testing.T) {
	var received bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Initialize global client
	Init(server.URL)
	defer Init("") // Reset

	ctx := context.Background()
	alertCtx := Context{InstanceID: "test123"}

	// Test global functions
	err := Critical(ctx, "test critical", alertCtx)
	if err != nil {
		t.Errorf("Critical returned error: %v", err)
	}
	if !received {
		t.Error("expected webhook to be called")
	}

	// Test Get returns the client
	client := Get()
	if client == nil {
		t.Error("Get returned nil")
	}
}

func TestGlobalWebhookClient_NotConfigured(t *testing.T) {
	// Reset global client
	Init("")

	ctx := context.Background()

	// Should not panic and return nil
	err := Critical(ctx, "test", Context{})
	if err != nil {
		t.Errorf("expected nil error when not configured, got %v", err)
	}

	// Get should return nil
	client := Get()
	if client != nil {
		t.Error("Get should return nil when not configured")
	}
}

func TestWebhookPayloadFormat(t *testing.T) {
	// Test that the payload format matches PRD Section 8.2
	payload := WebhookPayload{
		Level:     LevelCritical,
		Message:   "Could not verify billing stopped",
		Timestamp: "2026-02-02T17:00:15Z",
		Context: Context{
			InstanceID: "12345678",
			Provider:   "vast.ai",
			Action:     "stop",
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	// Parse back to verify structure
	var parsed map[string]any
	err = json.Unmarshal(jsonData, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	// Verify all expected fields exist
	expectedFields := []string{"level", "message", "timestamp", "context"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("missing field: %s", field)
		}
	}

	// Verify context fields
	contextMap, ok := parsed["context"].(map[string]any)
	if !ok {
		t.Fatal("context is not an object")
	}

	expectedContextFields := []string{"instance_id", "provider", "action"}
	for _, field := range expectedContextFields {
		if _, ok := contextMap[field]; !ok {
			t.Errorf("missing context field: %s", field)
		}
	}
}
