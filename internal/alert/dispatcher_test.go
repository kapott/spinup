package alert

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockTUINotifier records notifications for testing.
type mockTUINotifier struct {
	mu            sync.Mutex
	notifications []TUIAlertMsg
}

func (m *mockTUINotifier) Notify(level Level, message string, ctx Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, TUIAlertMsg{
		Level:   level,
		Message: message,
		Context: ctx,
	})
}

func (m *mockTUINotifier) getNotifications() []TUIAlertMsg {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]TUIAlertMsg, len(m.notifications))
	copy(result, m.notifications)
	return result
}

func TestNewDispatcher(t *testing.T) {
	d := NewDispatcher()
	if d == nil {
		t.Fatal("NewDispatcher() returned nil")
	}
	if d.enableTUI {
		t.Error("enableTUI should be false by default")
	}
}

func TestDispatcherWithOptions(t *testing.T) {
	mockTUI := &mockTUINotifier{}

	d := NewDispatcher(
		WithTUINotifier(mockTUI),
		WithTUIEnabled(true),
	)

	if d == nil {
		t.Fatal("NewDispatcher() returned nil")
	}
	if !d.enableTUI {
		t.Error("enableTUI should be true when TUI notifier is set")
	}
	if d.tuiNotifier == nil {
		t.Error("tuiNotifier should be set")
	}
}

func TestDispatchCritical(t *testing.T) {
	ResetDispatcher()
	mockTUI := &mockTUINotifier{}

	d := NewDispatcher(
		WithTUINotifier(mockTUI),
	)

	ctx := context.Background()
	alertCtx := Context{
		InstanceID: "test-instance",
		Provider:   "test-provider",
		Action:     "stop",
	}

	d.Critical(ctx, "Critical test message", alertCtx)

	// Wait a bit for async operations
	time.Sleep(10 * time.Millisecond)

	notifications := mockTUI.getNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].Level != LevelCritical {
		t.Errorf("Expected level CRITICAL, got %v", notifications[0].Level)
	}
	if notifications[0].Message != "Critical test message" {
		t.Errorf("Expected message 'Critical test message', got '%s'", notifications[0].Message)
	}
	if notifications[0].Context.InstanceID != "test-instance" {
		t.Errorf("Expected instance_id 'test-instance', got '%s'", notifications[0].Context.InstanceID)
	}
}

func TestDispatchError(t *testing.T) {
	mockTUI := &mockTUINotifier{}

	d := NewDispatcher(
		WithTUINotifier(mockTUI),
	)

	ctx := context.Background()
	alertCtx := Context{
		Provider: "vast",
		Error:    "connection failed",
	}

	d.Error(ctx, "Error test message", alertCtx)

	notifications := mockTUI.getNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].Level != LevelError {
		t.Errorf("Expected level ERROR, got %v", notifications[0].Level)
	}
}

func TestDispatchWarn(t *testing.T) {
	mockTUI := &mockTUINotifier{}

	d := NewDispatcher(
		WithTUINotifier(mockTUI),
	)

	ctx := context.Background()
	alertCtx := Context{
		Provider: "lambda",
	}

	d.Warn(ctx, "Warning test message", alertCtx)

	notifications := mockTUI.getNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].Level != LevelWarn {
		t.Errorf("Expected level WARN, got %v", notifications[0].Level)
	}
}

func TestDispatchInfo(t *testing.T) {
	mockTUI := &mockTUINotifier{}

	d := NewDispatcher(
		WithTUINotifier(mockTUI),
	)

	ctx := context.Background()
	alertCtx := Context{
		Model: "qwen2.5-coder:32b",
	}

	d.Info(ctx, "Info test message", alertCtx)

	notifications := mockTUI.getNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].Level != LevelInfo {
		t.Errorf("Expected level INFO, got %v", notifications[0].Level)
	}
}

func TestDispatchWithTUIDisabled(t *testing.T) {
	mockTUI := &mockTUINotifier{}

	d := NewDispatcher(
		WithTUINotifier(mockTUI),
		WithTUIEnabled(false), // Explicitly disable TUI
	)

	ctx := context.Background()
	d.Error(ctx, "Should not appear", Context{})

	notifications := mockTUI.getNotifications()
	if len(notifications) != 0 {
		t.Fatalf("Expected 0 notifications when TUI disabled, got %d", len(notifications))
	}
}

func TestDispatchWithNilTUINotifier(t *testing.T) {
	d := NewDispatcher() // No TUI notifier

	ctx := context.Background()
	// Should not panic
	d.Error(ctx, "No TUI", Context{})
	d.Critical(ctx, "No TUI Critical", Context{})
	d.Warn(ctx, "No TUI Warn", Context{})
	d.Info(ctx, "No TUI Info", Context{})
}

func TestGlobalDispatcher(t *testing.T) {
	ResetDispatcher()

	mockTUI := &mockTUINotifier{}
	InitDispatcher(WithTUINotifier(mockTUI))

	d := GetDispatcher()
	if d == nil {
		t.Fatal("GetDispatcher() returned nil")
	}

	ctx := context.Background()
	DispatchError(ctx, "Global error", Context{Provider: "test"})

	notifications := mockTUI.getNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}
}

func TestDispatcherSetters(t *testing.T) {
	d := NewDispatcher()

	mockTUI := &mockTUINotifier{}
	d.SetTUINotifier(mockTUI)

	if !d.enableTUI {
		t.Error("enableTUI should be true after SetTUINotifier")
	}

	d.EnableTUI(false)
	if d.enableTUI {
		t.Error("enableTUI should be false after EnableTUI(false)")
	}

	d.EnableTUI(true)
	if !d.enableTUI {
		t.Error("enableTUI should be true after EnableTUI(true)")
	}
}

func TestAlertRoutingLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		dispatch func(*Dispatcher, context.Context, string, Context)
	}{
		{
			name:  "Critical",
			level: LevelCritical,
			dispatch: func(d *Dispatcher, ctx context.Context, msg string, alertCtx Context) {
				d.Critical(ctx, msg, alertCtx)
			},
		},
		{
			name:  "Error",
			level: LevelError,
			dispatch: func(d *Dispatcher, ctx context.Context, msg string, alertCtx Context) {
				d.Error(ctx, msg, alertCtx)
			},
		},
		{
			name:  "Warn",
			level: LevelWarn,
			dispatch: func(d *Dispatcher, ctx context.Context, msg string, alertCtx Context) {
				d.Warn(ctx, msg, alertCtx)
			},
		},
		{
			name:  "Info",
			level: LevelInfo,
			dispatch: func(d *Dispatcher, ctx context.Context, msg string, alertCtx Context) {
				d.Info(ctx, msg, alertCtx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTUI := &mockTUINotifier{}
			d := NewDispatcher(WithTUINotifier(mockTUI))

			ctx := context.Background()
			alertCtx := Context{
				InstanceID: "test-123",
				Provider:   "vast",
				Action:     "deploy",
				Model:      "qwen2.5-coder:32b",
				GPU:        "A100-80GB",
				Region:     "EU",
				Error:      "test error",
			}

			tt.dispatch(d, ctx, "Test message for "+tt.name, alertCtx)

			// Wait a bit for async operations (critical sends webhook async)
			time.Sleep(10 * time.Millisecond)

			notifications := mockTUI.getNotifications()
			if len(notifications) != 1 {
				t.Fatalf("Expected 1 notification, got %d", len(notifications))
			}

			n := notifications[0]
			if n.Level != tt.level {
				t.Errorf("Expected level %v, got %v", tt.level, n.Level)
			}
			if n.Context.InstanceID != "test-123" {
				t.Errorf("Expected instance_id 'test-123', got '%s'", n.Context.InstanceID)
			}
			if n.Context.Provider != "vast" {
				t.Errorf("Expected provider 'vast', got '%s'", n.Context.Provider)
			}
		})
	}
}

func TestTUIAlertMsg(t *testing.T) {
	msg := TUIAlertMsg{
		Level:   LevelCritical,
		Message: "Test critical message",
		Context: Context{
			InstanceID: "instance-1",
			Provider:   "vast",
		},
	}

	if msg.Level != LevelCritical {
		t.Errorf("Expected level CRITICAL, got %v", msg.Level)
	}
	if msg.Message != "Test critical message" {
		t.Errorf("Expected message 'Test critical message', got '%s'", msg.Message)
	}
}
