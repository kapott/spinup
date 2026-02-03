// Package alert provides alerting functionality for spinup.
// It implements webhook notifications for critical events and errors.
package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tmeurs/spinup/internal/logging"
)

// Level represents the severity of an alert.
type Level string

const (
	// LevelCritical is for critical errors requiring immediate attention.
	// Examples: billing verification failed, stop command failed.
	LevelCritical Level = "CRITICAL"

	// LevelError is for errors that may require user attention.
	// Examples: API failures after retries.
	LevelError Level = "ERROR"

	// LevelWarn is for warnings about potential issues.
	// Examples: spot interruption, state mismatch.
	LevelWarn Level = "WARN"

	// LevelInfo is for informational messages.
	// Examples: session start/stop.
	LevelInfo Level = "INFO"
)

// Context contains contextual information for an alert.
type Context struct {
	InstanceID string `json:"instance_id,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Action     string `json:"action,omitempty"`
	Model      string `json:"model,omitempty"`
	GPU        string `json:"gpu,omitempty"`
	Region     string `json:"region,omitempty"`
	Error      string `json:"error,omitempty"`
}

// WebhookPayload represents the JSON payload sent to the webhook URL.
// Format matches PRD Section 8.2.
type WebhookPayload struct {
	Level     Level   `json:"level"`
	Message   string  `json:"message"`
	Timestamp string  `json:"timestamp"`
	Context   Context `json:"context"`
}

// WebhookClient sends alerts to a configured webhook URL.
type WebhookClient struct {
	webhookURL string
	httpClient *http.Client
	logger     *logging.Logger
}

// WebhookOption is a functional option for configuring WebhookClient.
type WebhookOption func(*WebhookClient)

// WithHTTPClient sets a custom HTTP client for the webhook client.
func WithHTTPClient(client *http.Client) WebhookOption {
	return func(wc *WebhookClient) {
		wc.httpClient = client
	}
}

// WithLogger sets a custom logger for the webhook client.
func WithLogger(logger *logging.Logger) WebhookOption {
	return func(wc *WebhookClient) {
		wc.logger = logger
	}
}

// NewWebhookClient creates a new WebhookClient with the given webhook URL.
// Returns nil if webhookURL is empty (alerts will be silently ignored).
func NewWebhookClient(webhookURL string, opts ...WebhookOption) *WebhookClient {
	if webhookURL == "" {
		return nil
	}

	wc := &WebhookClient{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logging.Get(),
	}

	for _, opt := range opts {
		opt(wc)
	}

	return wc
}

// SendAlert sends an alert to the configured webhook URL.
// It formats the payload according to PRD Section 8.2 and handles network failures gracefully.
// Returns nil if the webhook is not configured (client is nil or URL is empty).
func (wc *WebhookClient) SendAlert(ctx context.Context, level Level, message string, alertCtx Context) error {
	if wc == nil || wc.webhookURL == "" {
		// No webhook configured, silently ignore
		return nil
	}

	payload := WebhookPayload{
		Level:     level,
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Context:   alertCtx,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		wc.logger.Error().
			Err(err).
			Str("level", string(level)).
			Str("message", message).
			Msg("Failed to marshal webhook payload")
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	wc.logger.Debug().
		Str("url", wc.webhookURL).
		Str("level", string(level)).
		Str("message", message).
		Msg("Sending webhook alert")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wc.webhookURL, bytes.NewReader(jsonData))
	if err != nil {
		wc.logger.Error().
			Err(err).
			Str("url", wc.webhookURL).
			Msg("Failed to create webhook request")
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "spinup/1.0")

	resp, err := wc.httpClient.Do(req)
	if err != nil {
		// Log but don't return error - webhook failures shouldn't crash the app
		wc.logger.Warn().
			Err(err).
			Str("url", wc.webhookURL).
			Str("level", string(level)).
			Str("message", message).
			Msg("Failed to send webhook alert")
		return nil // Don't propagate network errors
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		wc.logger.Warn().
			Int("status_code", resp.StatusCode).
			Str("url", wc.webhookURL).
			Str("level", string(level)).
			Str("message", message).
			Msg("Webhook returned non-success status")
		// Don't return error - webhook failures shouldn't crash the app
		return nil
	}

	wc.logger.Debug().
		Int("status_code", resp.StatusCode).
		Str("level", string(level)).
		Msg("Webhook alert sent successfully")

	return nil
}

// SendCritical sends a CRITICAL level alert.
func (wc *WebhookClient) SendCritical(ctx context.Context, message string, alertCtx Context) error {
	return wc.SendAlert(ctx, LevelCritical, message, alertCtx)
}

// SendError sends an ERROR level alert.
func (wc *WebhookClient) SendError(ctx context.Context, message string, alertCtx Context) error {
	return wc.SendAlert(ctx, LevelError, message, alertCtx)
}

// SendWarn sends a WARN level alert.
func (wc *WebhookClient) SendWarn(ctx context.Context, message string, alertCtx Context) error {
	return wc.SendAlert(ctx, LevelWarn, message, alertCtx)
}

// SendInfo sends an INFO level alert.
func (wc *WebhookClient) SendInfo(ctx context.Context, message string, alertCtx Context) error {
	return wc.SendAlert(ctx, LevelInfo, message, alertCtx)
}

// Global webhook client instance
var globalWebhook *WebhookClient

// Init initializes the global webhook client with the given webhook URL.
// If webhookURL is empty, the global client will be nil and alerts will be ignored.
func Init(webhookURL string, opts ...WebhookOption) {
	globalWebhook = NewWebhookClient(webhookURL, opts...)
}

// Get returns the global webhook client.
// May return nil if no webhook URL was configured.
func Get() *WebhookClient {
	return globalWebhook
}

// Alert sends an alert using the global webhook client.
// If no webhook is configured, the alert is silently ignored.
func Alert(ctx context.Context, level Level, message string, alertCtx Context) error {
	if globalWebhook == nil {
		return nil
	}
	return globalWebhook.SendAlert(ctx, level, message, alertCtx)
}

// Critical sends a CRITICAL alert using the global webhook client.
func Critical(ctx context.Context, message string, alertCtx Context) error {
	return Alert(ctx, LevelCritical, message, alertCtx)
}

// Error sends an ERROR alert using the global webhook client.
func Error(ctx context.Context, message string, alertCtx Context) error {
	return Alert(ctx, LevelError, message, alertCtx)
}

// Warn sends a WARN alert using the global webhook client.
func Warn(ctx context.Context, message string, alertCtx Context) error {
	return Alert(ctx, LevelWarn, message, alertCtx)
}

// Info sends an INFO alert using the global webhook client.
func Info(ctx context.Context, message string, alertCtx Context) error {
	return Alert(ctx, LevelInfo, message, alertCtx)
}
