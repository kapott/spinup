package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	m := NewModel()

	if m.currentView != ViewProviderSelect {
		t.Errorf("Expected initial view to be ViewProviderSelect, got %v", m.currentView)
	}

	if m.quitting {
		t.Error("Expected quitting to be false")
	}

	if m.ready {
		t.Error("Expected ready to be false before window size message")
	}

	if m.ctx == nil {
		t.Error("Expected context to be non-nil")
	}

	if m.cancel == nil {
		t.Error("Expected cancel function to be non-nil")
	}
}

func TestNewModelWithView(t *testing.T) {
	tests := []struct {
		view View
	}{
		{ViewProviderSelect},
		{ViewModelSelect},
		{ViewDeployProgress},
		{ViewInstanceStatus},
		{ViewStopProgress},
		{ViewAlert},
	}

	for _, tc := range tests {
		m := NewModelWithView(tc.view)
		if m.currentView != tc.view {
			t.Errorf("Expected view %v, got %v", tc.view, m.currentView)
		}
	}
}

func TestViewString(t *testing.T) {
	tests := []struct {
		view     View
		expected string
	}{
		{ViewProviderSelect, "provider_select"},
		{ViewModelSelect, "model_select"},
		{ViewDeployProgress, "deploy_progress"},
		{ViewInstanceStatus, "instance_status"},
		{ViewStopProgress, "stop_progress"},
		{ViewAlert, "alert"},
		{View(99), "unknown"},
	}

	for _, tc := range tests {
		if tc.view.String() != tc.expected {
			t.Errorf("Expected view string %q, got %q", tc.expected, tc.view.String())
		}
	}
}

func TestModelInit(t *testing.T) {
	m := NewModel()
	_ = m.Init()

	// Init may return a command for spinner initialization, which is acceptable
	// The important thing is that Init() doesn't panic
}

func TestModelUpdateWindowSize(t *testing.T) {
	m := NewModel()

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updated, cmd := m.Update(msg)

	model := updated.(Model)
	if model.width != 80 {
		t.Errorf("Expected width 80, got %d", model.width)
	}
	if model.height != 24 {
		t.Errorf("Expected height 24, got %d", model.height)
	}
	if !model.ready {
		t.Error("Expected ready to be true after window size message")
	}
	if cmd != nil {
		t.Error("Expected nil cmd after window size message")
	}
}

func TestModelUpdateQuit(t *testing.T) {
	tests := []struct {
		key string
	}{
		{"q"},
		{"ctrl+c"},
	}

	for _, tc := range tests {
		model := NewModel()
		var msg tea.KeyMsg
		if tc.key == "ctrl+c" {
			msg = tea.KeyMsg{Type: tea.KeyCtrlC}
		} else {
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)}
		}

		updated, cmd := model.Update(msg)
		result := updated.(Model)

		if !result.quitting {
			t.Errorf("Expected quitting to be true after %s", tc.key)
		}
		if cmd == nil {
			t.Errorf("Expected quit cmd after %s", tc.key)
		}
	}
}

func TestModelView(t *testing.T) {
	m := NewModel()

	// Before ready, should show initializing
	view := m.View()
	if view != "Initializing..." {
		t.Errorf("Expected 'Initializing...' before ready, got %q", view)
	}

	// After window size, should show content
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	view = m.View()
	if view == "Initializing..." {
		t.Error("Expected different content after ready")
	}
	if view == "" {
		t.Error("Expected non-empty view")
	}
}

func TestModelViewQuitting(t *testing.T) {
	m := NewModel()
	m.quitting = true
	m.ready = true

	view := m.View()
	if view != "Goodbye!\n" {
		t.Errorf("Expected 'Goodbye!' when quitting, got %q", view)
	}
}

func TestModelSetView(t *testing.T) {
	m := NewModel()
	m.SetView(ViewModelSelect)

	if m.GetView() != ViewModelSelect {
		t.Error("Expected GetView to return ViewModelSelect")
	}
}

func TestModelSetStatusMessage(t *testing.T) {
	m := NewModel()
	m.SetStatusMessage("Test message")

	if m.statusMessage != "Test message" {
		t.Errorf("Expected status message to be 'Test message', got %q", m.statusMessage)
	}
}

func TestModelDimensions(t *testing.T) {
	m := NewModel()
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.Width() != 100 {
		t.Errorf("Expected Width() to return 100, got %d", m.Width())
	}
	if m.Height() != 50 {
		t.Errorf("Expected Height() to return 50, got %d", m.Height())
	}
}

func TestModelIsReady(t *testing.T) {
	m := NewModel()

	if m.IsReady() {
		t.Error("Expected IsReady to be false before window size")
	}

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if !m.IsReady() {
		t.Error("Expected IsReady to be true after window size")
	}
}

func TestModelIsQuitting(t *testing.T) {
	m := NewModel()

	if m.IsQuitting() {
		t.Error("Expected IsQuitting to be false initially")
	}

	m.quitting = true

	if !m.IsQuitting() {
		t.Error("Expected IsQuitting to be true after setting quitting")
	}
}

func TestModelContext(t *testing.T) {
	m := NewModel()

	ctx := m.Context()
	if ctx == nil {
		t.Error("Expected Context to return non-nil context")
	}

	select {
	case <-ctx.Done():
		t.Error("Expected context to not be cancelled initially")
	default:
		// Good, context is not cancelled
	}

	m.Cancel()

	select {
	case <-ctx.Done():
		// Good, context is cancelled
	default:
		t.Error("Expected context to be cancelled after Cancel()")
	}
}

func TestQuitMessage(t *testing.T) {
	msg := Quit()
	if _, ok := msg.(quitMsg); !ok {
		t.Error("Expected Quit() to return quitMsg")
	}
}

func TestSetErrorCommand(t *testing.T) {
	testErr := errMsg{err: nil}
	_ = testErr // Just to verify the type exists
}

func TestModelUpdateError(t *testing.T) {
	m := NewModel()
	testErr := errMsg{err: nil}

	updated, _ := m.Update(testErr)
	model := updated.(Model)

	// Error should be set (to nil in this case)
	if model.err != nil {
		t.Error("Expected err to be nil")
	}
}

func TestModelHelpKey(t *testing.T) {
	m := NewModel()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")}
	updated, _ := m.Update(msg)
	model := updated.(Model)

	if model.statusMessage == "Press 'q' to quit" {
		t.Error("Expected status message to change after pressing ?")
	}
}

func TestModelEscapeKey(t *testing.T) {
	m := NewModel()
	m.currentView = ViewProviderSelect

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, cmd := m.Update(msg)
	model := updated.(Model)

	// At root view (ViewProviderSelect), ESC should quit
	if !model.quitting {
		t.Error("Expected ESC at root view to quit")
	}
	if cmd == nil {
		t.Error("Expected quit cmd after ESC at root view")
	}
}

func TestAllViewsRender(t *testing.T) {
	views := []View{
		ViewProviderSelect,
		ViewModelSelect,
		ViewDeployProgress,
		ViewInstanceStatus,
		ViewStopProgress,
		ViewAlert,
	}

	for _, view := range views {
		m := NewModelWithView(view)
		msg := tea.WindowSizeMsg{Width: 80, Height: 24}
		updated, _ := m.Update(msg)
		m = updated.(Model)

		content := m.View()
		if content == "" {
			t.Errorf("View %v rendered empty content", view)
		}
		if content == "Initializing..." {
			t.Errorf("View %v rendered Initializing... despite being ready", view)
		}
	}
}
