package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmeurs/continueplz/internal/config"
)

func TestNewStatusModel(t *testing.T) {
	m := NewStatusModel()

	if m.state != nil {
		t.Error("Expected state to be nil initially")
	}

	if m.action != StatusActionNone {
		t.Errorf("Expected action to be StatusActionNone, got %v", m.action)
	}

	if m.testing {
		t.Error("Expected testing to be false initially")
	}
}

func TestNewStatusModelWithState(t *testing.T) {
	state := &config.State{
		Instance: &config.InstanceState{
			ID:       "test-123",
			Provider: "vast.ai",
			GPU:      "A100 40GB",
			Region:   "EU-West",
		},
	}

	m := NewStatusModelWithState(state)

	if m.state == nil {
		t.Error("Expected state to be set")
	}

	if m.state.Instance.ID != "test-123" {
		t.Errorf("Expected instance ID to be 'test-123', got %v", m.state.Instance.ID)
	}
}

func TestStatusModelKeyHandling(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		expectedAction StatusAction
	}{
		{"stop action", "s", StatusActionStop},
		{"stop action uppercase", "S", StatusActionStop},
		{"test action", "t", StatusActionTest},
		{"test action uppercase", "T", StatusActionTest},
		{"logs action", "l", StatusActionLogs},
		{"logs action uppercase", "L", StatusActionLogs},
		{"quit action", "q", StatusActionQuit},
		{"quit action uppercase", "Q", StatusActionQuit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewStatusModel()

			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			m, cmd := m.handleKeyPress(keyMsg)

			if m.action != tt.expectedAction {
				t.Errorf("Expected action %v, got %v", tt.expectedAction, m.action)
			}

			if cmd == nil {
				t.Error("Expected command to be returned")
			}
		})
	}
}

func TestStatusModelViewRendering(t *testing.T) {
	// Test with no state
	m := NewStatusModel()
	m.width = 100
	m.height = 40

	view := m.View()

	if view == "" {
		t.Error("Expected non-empty view")
	}

	// View should contain the instance box
	if !statusTestContains(view, "Current Instance") {
		t.Error("Expected view to contain 'Current Instance'")
	}

	// View should contain quick actions
	if !statusTestContains(view, "Quick Actions") {
		t.Error("Expected view to contain 'Quick Actions'")
	}

	// View should show no active instance message
	if !statusTestContains(view, "No active instance") {
		t.Error("Expected view to show 'No active instance' when state is nil")
	}
}

func TestStatusModelViewWithState(t *testing.T) {
	state := &config.State{
		Instance: &config.InstanceState{
			ID:          "test-123",
			Provider:    "vast.ai",
			GPU:         "A100 40GB",
			Region:      "EU-West",
			Type:        "spot",
			WireGuardIP: "10.13.37.2",
			CreatedAt:   time.Now().Add(-2 * time.Hour),
		},
		Model: &config.ModelState{
			Name:   "qwen2.5-coder:32b",
			Status: "ready",
		},
		WireGuard: &config.WireGuardState{
			ServerPublicKey: "test-key",
			InterfaceName:   "wg-continueplz",
		},
		Cost: &config.CostState{
			HourlyRate:  0.65,
			Accumulated: 1.30,
			Currency:    "EUR",
		},
		Deadman: &config.DeadmanState{
			TimeoutHours:  10,
			LastHeartbeat: time.Now().Add(-1 * time.Hour),
		},
	}

	m := NewStatusModelWithState(state)
	m.width = 100
	m.height = 40

	view := m.View()

	// View should contain instance info
	if !statusTestContains(view, "vast.ai") {
		t.Error("Expected view to contain provider name")
	}

	if !statusTestContains(view, "A100 40GB") {
		t.Error("Expected view to contain GPU type")
	}

	if !statusTestContains(view, "EU-West") {
		t.Error("Expected view to contain region")
	}

	if !statusTestContains(view, "qwen2.5-coder:32b") {
		t.Error("Expected view to contain model name")
	}

	if !statusTestContains(view, "10.13.37.2:11434") {
		t.Error("Expected view to contain endpoint")
	}
}

func TestStatusModelUpdate(t *testing.T) {
	m := NewStatusModel()

	// Test window size message
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	if m.width != 120 || m.height != 50 {
		t.Errorf("Expected dimensions 120x50, got %dx%d", m.width, m.height)
	}

	// Test state update message
	state := &config.State{
		Instance: &config.InstanceState{
			ID:       "new-instance",
			Provider: "lambda",
		},
	}
	m, _ = m.Update(StatusStateUpdatedMsg{State: state})
	if m.state == nil || m.state.Instance.ID != "new-instance" {
		t.Error("Expected state to be updated")
	}

	// Test test start message
	m, _ = m.Update(StatusTestStartMsg{})
	if !m.testing {
		t.Error("Expected testing to be true after test start")
	}

	// Test test result message
	result := &TestResult{
		Success: true,
		Latency: 50 * time.Millisecond,
	}
	m, _ = m.Update(StatusTestResultMsg{Result: result})
	if m.testing {
		t.Error("Expected testing to be false after test result")
	}
	if m.testResult == nil || !m.testResult.Success {
		t.Error("Expected test result to be stored")
	}
}

func TestStatusActionString(t *testing.T) {
	tests := []struct {
		action   StatusAction
		expected string
	}{
		{StatusActionNone, "none"},
		{StatusActionStop, "stop"},
		{StatusActionTest, "test"},
		{StatusActionLogs, "logs"},
		{StatusActionQuit, "quit"},
	}

	for _, tt := range tests {
		if tt.action.String() != tt.expected {
			t.Errorf("Expected %v.String() to be %q, got %q", tt.action, tt.expected, tt.action.String())
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{10 * time.Hour, "10h 0m"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, expected %q", tt.duration, result, tt.expected)
		}
	}
}

func TestCalculateDeadmanRemaining(t *testing.T) {
	m := NewStatusModel()

	// Test nil deadman
	remaining := m.calculateDeadmanRemaining(nil)
	if remaining != 0 {
		t.Errorf("Expected 0 for nil deadman, got %v", remaining)
	}

	// Test active deadman
	deadman := &config.DeadmanState{
		TimeoutHours:  10,
		LastHeartbeat: time.Now().Add(-1 * time.Hour),
	}
	remaining = m.calculateDeadmanRemaining(deadman)

	// Should be approximately 9 hours
	expectedMin := 8*time.Hour + 59*time.Minute
	expectedMax := 9*time.Hour + 1*time.Minute
	if remaining < expectedMin || remaining > expectedMax {
		t.Errorf("Expected remaining to be around 9 hours, got %v", remaining)
	}
}

func TestSetDimensions(t *testing.T) {
	m := NewStatusModel()
	m.SetDimensions(80, 24)

	if m.width != 80 {
		t.Errorf("Expected width 80, got %d", m.width)
	}
	if m.height != 24 {
		t.Errorf("Expected height 24, got %d", m.height)
	}
}

func TestSetState(t *testing.T) {
	m := NewStatusModel()

	state := &config.State{
		Instance: &config.InstanceState{
			ID: "test-id",
		},
	}

	m.SetState(state)

	if m.State() != state {
		t.Error("Expected State() to return the set state")
	}
}

func TestClearAction(t *testing.T) {
	m := NewStatusModel()
	m.action = StatusActionStop

	m.ClearAction()

	if m.Action() != StatusActionNone {
		t.Errorf("Expected action to be cleared, got %v", m.Action())
	}
}

// statusTestContains is a helper function for tests
func statusTestContains(s, substr string) bool {
	return strings.Contains(s, substr)
}
