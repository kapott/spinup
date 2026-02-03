// Package alert provides alerting functionality for spinup.
// This file implements the alert dispatcher which routes alerts to appropriate destinations
// based on their severity level.
package alert

import (
	"context"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmeurs/spinup/internal/logging"
)

// TUINotifier is the interface for sending alerts to the TUI.
// This allows the dispatcher to be decoupled from the TUI implementation.
type TUINotifier interface {
	// Notify sends an alert notification to the TUI.
	// The implementation should convert the alert to the appropriate TUI message.
	Notify(level Level, message string, ctx Context)
}

// TUIProgram wraps a tea.Program to implement TUINotifier.
type TUIProgram struct {
	program *tea.Program
}

// NewTUIProgram creates a new TUIProgram wrapper.
func NewTUIProgram(p *tea.Program) *TUIProgram {
	if p == nil {
		return nil
	}
	return &TUIProgram{program: p}
}

// Notify sends an alert to the TUI program.
func (t *TUIProgram) Notify(level Level, message string, ctx Context) {
	if t == nil || t.program == nil {
		return
	}
	// Send a TUIAlertMsg to the program
	t.program.Send(TUIAlertMsg{
		Level:   level,
		Message: message,
		Context: ctx,
	})
}

// TUIAlertMsg is sent to the TUI to display an alert notification.
// The TUI should handle this message and display the appropriate alert UI.
type TUIAlertMsg struct {
	Level   Level
	Message string
	Context Context
}

// Dispatcher routes alerts to the appropriate destinations based on severity level.
// It coordinates between webhook notifications, logging, and TUI notifications.
//
// Alert routing by level:
// - CRITICAL: webhook + ERROR log + TUI notification
// - ERROR: ERROR log + TUI notification
// - WARN: WARN log + TUI notification
// - INFO: INFO log + TUI notification (optional)
type Dispatcher struct {
	mu            sync.RWMutex
	webhookClient *WebhookClient
	logger        *logging.Logger
	tuiNotifier   TUINotifier

	// enableTUI controls whether TUI notifications are sent.
	// This can be disabled when running in non-interactive mode.
	enableTUI bool
}

// DispatcherOption is a functional option for configuring the Dispatcher.
type DispatcherOption func(*Dispatcher)

// WithWebhookClient sets the webhook client for the dispatcher.
func WithDispatcherWebhookClient(client *WebhookClient) DispatcherOption {
	return func(d *Dispatcher) {
		d.webhookClient = client
	}
}

// WithDispatcherLogger sets the logger for the dispatcher.
func WithDispatcherLogger(logger *logging.Logger) DispatcherOption {
	return func(d *Dispatcher) {
		d.logger = logger
	}
}

// WithTUINotifier sets the TUI notifier for the dispatcher.
func WithTUINotifier(notifier TUINotifier) DispatcherOption {
	return func(d *Dispatcher) {
		d.tuiNotifier = notifier
		d.enableTUI = notifier != nil
	}
}

// WithTUIEnabled explicitly enables or disables TUI notifications.
func WithTUIEnabled(enabled bool) DispatcherOption {
	return func(d *Dispatcher) {
		d.enableTUI = enabled
	}
}

// NewDispatcher creates a new alert Dispatcher with the given options.
func NewDispatcher(opts ...DispatcherOption) *Dispatcher {
	d := &Dispatcher{
		logger:    logging.Get(),
		enableTUI: false,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// SetWebhookClient sets or updates the webhook client.
func (d *Dispatcher) SetWebhookClient(client *WebhookClient) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.webhookClient = client
}

// SetTUINotifier sets or updates the TUI notifier.
func (d *Dispatcher) SetTUINotifier(notifier TUINotifier) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tuiNotifier = notifier
	d.enableTUI = notifier != nil
}

// EnableTUI enables or disables TUI notifications.
func (d *Dispatcher) EnableTUI(enabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enableTUI = enabled
}

// Dispatch sends an alert to the appropriate destinations based on its level.
// This is the main entry point for sending alerts through the dispatcher.
//
// Routing:
// - CRITICAL: webhook (async) + ERROR log + TUI notification
// - ERROR: ERROR log + TUI notification
// - WARN: WARN log + TUI notification
// - INFO: INFO log + TUI notification
func (d *Dispatcher) Dispatch(ctx context.Context, level Level, message string, alertCtx Context) {
	d.mu.RLock()
	webhookClient := d.webhookClient
	tuiNotifier := d.tuiNotifier
	enableTUI := d.enableTUI
	logger := d.logger
	d.mu.RUnlock()

	// Always log the alert
	d.logAlert(logger, level, message, alertCtx)

	// Send webhook for CRITICAL alerts
	if level == LevelCritical && webhookClient != nil {
		// Send webhook asynchronously to avoid blocking
		go func() {
			_ = webhookClient.SendAlert(ctx, level, message, alertCtx)
		}()
	}

	// Send TUI notification if enabled
	if enableTUI && tuiNotifier != nil {
		tuiNotifier.Notify(level, message, alertCtx)
	}
}

// logAlert logs an alert at the appropriate log level.
func (d *Dispatcher) logAlert(logger *logging.Logger, level Level, message string, alertCtx Context) {
	if logger == nil {
		logger = logging.Get()
	}

	// Log at the appropriate level
	switch level {
	case LevelCritical:
		// Critical alerts are logged at ERROR level
		event := logger.Error()
		if alertCtx.InstanceID != "" {
			event = event.Str("instance_id", alertCtx.InstanceID)
		}
		if alertCtx.Provider != "" {
			event = event.Str("provider", alertCtx.Provider)
		}
		if alertCtx.Action != "" {
			event = event.Str("action", alertCtx.Action)
		}
		if alertCtx.Model != "" {
			event = event.Str("model", alertCtx.Model)
		}
		if alertCtx.GPU != "" {
			event = event.Str("gpu", alertCtx.GPU)
		}
		if alertCtx.Region != "" {
			event = event.Str("region", alertCtx.Region)
		}
		if alertCtx.Error != "" {
			event = event.Str("error", alertCtx.Error)
		}
		event.Str("alert_level", "CRITICAL").Msg(message)

	case LevelError:
		event := logger.Error()
		if alertCtx.InstanceID != "" {
			event = event.Str("instance_id", alertCtx.InstanceID)
		}
		if alertCtx.Provider != "" {
			event = event.Str("provider", alertCtx.Provider)
		}
		if alertCtx.Action != "" {
			event = event.Str("action", alertCtx.Action)
		}
		if alertCtx.Model != "" {
			event = event.Str("model", alertCtx.Model)
		}
		if alertCtx.GPU != "" {
			event = event.Str("gpu", alertCtx.GPU)
		}
		if alertCtx.Region != "" {
			event = event.Str("region", alertCtx.Region)
		}
		if alertCtx.Error != "" {
			event = event.Str("error", alertCtx.Error)
		}
		event.Msg(message)

	case LevelWarn:
		event := logger.Warn()
		if alertCtx.InstanceID != "" {
			event = event.Str("instance_id", alertCtx.InstanceID)
		}
		if alertCtx.Provider != "" {
			event = event.Str("provider", alertCtx.Provider)
		}
		if alertCtx.Action != "" {
			event = event.Str("action", alertCtx.Action)
		}
		if alertCtx.Model != "" {
			event = event.Str("model", alertCtx.Model)
		}
		if alertCtx.GPU != "" {
			event = event.Str("gpu", alertCtx.GPU)
		}
		if alertCtx.Region != "" {
			event = event.Str("region", alertCtx.Region)
		}
		if alertCtx.Error != "" {
			event = event.Str("error", alertCtx.Error)
		}
		event.Msg(message)

	case LevelInfo:
		event := logger.Info()
		if alertCtx.InstanceID != "" {
			event = event.Str("instance_id", alertCtx.InstanceID)
		}
		if alertCtx.Provider != "" {
			event = event.Str("provider", alertCtx.Provider)
		}
		if alertCtx.Action != "" {
			event = event.Str("action", alertCtx.Action)
		}
		if alertCtx.Model != "" {
			event = event.Str("model", alertCtx.Model)
		}
		if alertCtx.GPU != "" {
			event = event.Str("gpu", alertCtx.GPU)
		}
		if alertCtx.Region != "" {
			event = event.Str("region", alertCtx.Region)
		}
		if alertCtx.Error != "" {
			event = event.Str("error", alertCtx.Error)
		}
		event.Msg(message)
	}
}

// Critical dispatches a CRITICAL level alert.
// CRITICAL alerts are sent to: webhook (async) + ERROR log + TUI notification
func (d *Dispatcher) Critical(ctx context.Context, message string, alertCtx Context) {
	d.Dispatch(ctx, LevelCritical, message, alertCtx)
}

// Error dispatches an ERROR level alert.
// ERROR alerts are sent to: ERROR log + TUI notification
func (d *Dispatcher) Error(ctx context.Context, message string, alertCtx Context) {
	d.Dispatch(ctx, LevelError, message, alertCtx)
}

// Warn dispatches a WARN level alert.
// WARN alerts are sent to: WARN log + TUI notification
func (d *Dispatcher) Warn(ctx context.Context, message string, alertCtx Context) {
	d.Dispatch(ctx, LevelWarn, message, alertCtx)
}

// Info dispatches an INFO level alert.
// INFO alerts are sent to: INFO log + TUI notification
func (d *Dispatcher) Info(ctx context.Context, message string, alertCtx Context) {
	d.Dispatch(ctx, LevelInfo, message, alertCtx)
}

// Global dispatcher instance
var (
	globalDispatcher     *Dispatcher
	globalDispatcherOnce sync.Once
)

// InitDispatcher initializes the global dispatcher with the given options.
// This should be called once at application startup.
func InitDispatcher(opts ...DispatcherOption) {
	globalDispatcherOnce.Do(func() {
		globalDispatcher = NewDispatcher(opts...)
	})
}

// GetDispatcher returns the global dispatcher.
// If not initialized, returns a default dispatcher.
func GetDispatcher() *Dispatcher {
	if globalDispatcher == nil {
		globalDispatcherOnce.Do(func() {
			globalDispatcher = NewDispatcher()
		})
	}
	return globalDispatcher
}

// ResetDispatcher resets the global dispatcher for testing purposes.
// This should only be used in tests.
func ResetDispatcher() {
	globalDispatcher = nil
	globalDispatcherOnce = sync.Once{}
}

// DispatchCritical dispatches a CRITICAL alert using the global dispatcher.
func DispatchCritical(ctx context.Context, message string, alertCtx Context) {
	GetDispatcher().Critical(ctx, message, alertCtx)
}

// DispatchError dispatches an ERROR alert using the global dispatcher.
func DispatchError(ctx context.Context, message string, alertCtx Context) {
	GetDispatcher().Error(ctx, message, alertCtx)
}

// DispatchWarn dispatches a WARN alert using the global dispatcher.
func DispatchWarn(ctx context.Context, message string, alertCtx Context) {
	GetDispatcher().Warn(ctx, message, alertCtx)
}

// DispatchInfo dispatches an INFO alert using the global dispatcher.
func DispatchInfo(ctx context.Context, message string, alertCtx Context) {
	GetDispatcher().Info(ctx, message, alertCtx)
}
