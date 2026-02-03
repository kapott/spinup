package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewInitWizardModel(t *testing.T) {
	m := NewInitWizardModel()

	if m.step != StepProviderSelect {
		t.Errorf("expected step %v, got %v", StepProviderSelect, m.step)
	}

	if len(m.providers) != 5 {
		t.Errorf("expected 5 providers, got %d", len(m.providers))
	}

	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}

	if m.HasSelectedProviders() {
		t.Error("expected no providers selected initially")
	}
}

func TestInitWizardProviderSelection(t *testing.T) {
	m := NewInitWizardModel()

	// Toggle first provider
	m.ToggleProvider("vast")
	if !m.IsProviderSelected("vast") {
		t.Error("expected vast to be selected")
	}

	// Toggle again to deselect
	m.ToggleProvider("vast")
	if m.IsProviderSelected("vast") {
		t.Error("expected vast to be deselected")
	}

	// Select specific providers
	m.SelectProvider("vast")
	m.SelectProvider("lambda")
	m.SelectProvider("runpod")

	selected := m.SelectedProviders()
	if len(selected) != 3 {
		t.Errorf("expected 3 selected providers, got %d", len(selected))
	}

	// Verify order matches AllProviders order
	expectedOrder := []string{"vast", "lambda", "runpod"}
	for i, name := range expectedOrder {
		if selected[i] != name {
			t.Errorf("expected %s at position %d, got %s", name, i, selected[i])
		}
	}

	// Deselect
	m.DeselectProvider("lambda")
	selected = m.SelectedProviders()
	if len(selected) != 2 {
		t.Errorf("expected 2 selected providers, got %d", len(selected))
	}
}

func TestInitWizardSelectedProviderInfos(t *testing.T) {
	m := NewInitWizardModel()

	m.SelectProvider("vast")
	m.SelectProvider("lambda")

	infos := m.SelectedProviderInfos()
	if len(infos) != 2 {
		t.Errorf("expected 2 provider infos, got %d", len(infos))
	}

	if infos[0].Name != "vast" || infos[1].Name != "lambda" {
		t.Error("provider info order doesn't match")
	}
}

func TestInitWizardKeyNavigation(t *testing.T) {
	m := NewInitWizardModel()

	// Move down
	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(InitWizardModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}

	// Move down with 'j'
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newModel.(InitWizardModel)
	if m.cursor != 2 {
		t.Errorf("expected cursor 2, got %d", m.cursor)
	}

	// Move up
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(InitWizardModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}

	// Move up with 'k'
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = newModel.(InitWizardModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}

	// Can't go above 0
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(InitWizardModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}

	// Go to last item
	for i := 0; i < 10; i++ {
		newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
		m = newModel.(InitWizardModel)
	}
	if m.cursor != 4 { // 5 providers, so max index is 4
		t.Errorf("expected cursor 4, got %d", m.cursor)
	}

	// Can't go below max
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(InitWizardModel)
	if m.cursor != 4 {
		t.Errorf("expected cursor 4, got %d", m.cursor)
	}
}

func TestInitWizardKeyToggle(t *testing.T) {
	m := NewInitWizardModel()

	// Toggle with space
	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	m = newModel.(InitWizardModel)
	if !m.IsProviderSelected("vast") {
		t.Error("expected vast to be selected after space")
	}

	// Toggle with 'x'
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = newModel.(InitWizardModel)
	if m.IsProviderSelected("vast") {
		t.Error("expected vast to be deselected after x")
	}
}

func TestInitWizardSelectAll(t *testing.T) {
	m := NewInitWizardModel()

	// Select all with 'a'
	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = newModel.(InitWizardModel)

	selected := m.SelectedProviders()
	if len(selected) != 5 {
		t.Errorf("expected 5 selected providers, got %d", len(selected))
	}

	// Deselect all with 'n'
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(InitWizardModel)

	selected = m.SelectedProviders()
	if len(selected) != 0 {
		t.Errorf("expected 0 selected providers, got %d", len(selected))
	}
}

func TestInitWizardConfirm(t *testing.T) {
	m := NewInitWizardModel()

	// Enter without selection should not complete
	newModel, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(InitWizardModel)
	if m.IsDone() {
		t.Error("expected wizard not done without selection")
	}
	if cmd != nil {
		t.Error("expected no command without selection")
	}

	// Select a provider and confirm
	m.SelectProvider("vast")
	newModel, cmd = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(InitWizardModel)
	if !m.IsDone() {
		t.Error("expected wizard done after confirm with selection")
	}
	if cmd == nil {
		t.Error("expected command after confirm")
	}

	// Check the command produces the right message
	msg := cmd()
	selectedMsg, ok := msg.(InitWizardProvidersSelectedMsg)
	if !ok {
		t.Fatal("expected InitWizardProvidersSelectedMsg")
	}
	if len(selectedMsg.Providers) != 1 || selectedMsg.Providers[0] != "vast" {
		t.Error("expected [vast] in selected providers message")
	}
}

func TestInitWizardQuit(t *testing.T) {
	m := NewInitWizardModel()

	// Quit with 'q'
	newModel, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m2 := newModel.(InitWizardModel)
	if !m2.IsQuitting() {
		t.Error("expected quitting after 'q'")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestInitWizardReset(t *testing.T) {
	m := NewInitWizardModel()

	// Make some changes
	m.SelectProvider("vast")
	m.SelectProvider("lambda")
	m.cursor = 3
	m.step = StepAPIKeyInput
	m.quitting = true
	m.done = true

	// Reset
	m.Reset()

	if m.step != StepProviderSelect {
		t.Error("expected step to be reset")
	}
	if m.HasSelectedProviders() {
		t.Error("expected no selected providers after reset")
	}
	if m.cursor != 0 {
		t.Error("expected cursor 0 after reset")
	}
	if m.IsQuitting() {
		t.Error("expected not quitting after reset")
	}
	if m.IsDone() {
		t.Error("expected not done after reset")
	}
}

func TestInitWizardView(t *testing.T) {
	m := NewInitWizardModel()
	m.SetDimensions(80, 24)

	view := m.View()

	// Check for key elements in the view
	expectedElements := []string{
		"spinup setup",
		"Which providers do you want to configure",
		"Vast.ai",
		"Lambda Labs",
		"RunPod",
		"CoreWeave",
		"Paperspace",
		"navigate",
		"toggle",
		"quit",
	}

	for _, elem := range expectedElements {
		if !containsString(view, elem) {
			t.Errorf("expected view to contain %q", elem)
		}
	}
}

func TestInitWizardViewWithSelection(t *testing.T) {
	m := NewInitWizardModel()
	m.SetDimensions(80, 24)

	// Select some providers
	m.SelectProvider("vast")
	m.SelectProvider("runpod")

	view := m.View()

	// Should show selection count
	if !containsString(view, "2 provider(s) selected") {
		t.Error("expected view to show '2 provider(s) selected'")
	}
}

func TestInitWizardViewNoSelection(t *testing.T) {
	m := NewInitWizardModel()
	m.SetDimensions(80, 24)

	view := m.View()

	// Should show no selection message
	if !containsString(view, "No providers selected") {
		t.Error("expected view to show 'No providers selected'")
	}
}

func TestInitWizardQuittingView(t *testing.T) {
	m := NewInitWizardModel()
	m.quitting = true

	view := m.View()

	if view != "Setup cancelled.\n" {
		t.Errorf("expected cancelled message, got %q", view)
	}
}

func TestInitWizardStepString(t *testing.T) {
	tests := []struct {
		step InitWizardStep
		want string
	}{
		{StepProviderSelect, "provider_select"},
		{StepAPIKeyInput, "api_key_input"},
		{StepWireGuard, "wireguard"},
		{StepPreferences, "preferences"},
		{StepComplete, "complete"},
		{InitWizardStep(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.step.String()
		if got != tt.want {
			t.Errorf("step %d: expected %q, got %q", tt.step, tt.want, got)
		}
	}
}

func TestAllProvidersComplete(t *testing.T) {
	// Verify all providers have required fields
	for _, p := range AllProviders {
		if p.Name == "" {
			t.Error("provider has empty name")
		}
		if p.DisplayName == "" {
			t.Errorf("provider %s has empty display name", p.Name)
		}
		if p.Description == "" {
			t.Errorf("provider %s has empty description", p.Name)
		}
		if p.APIKeyURL == "" {
			t.Errorf("provider %s has empty API key URL", p.Name)
		}
	}

	// Verify expected providers exist
	expectedProviders := []string{"vast", "lambda", "runpod", "coreweave", "paperspace"}
	providerMap := make(map[string]bool)
	for _, p := range AllProviders {
		providerMap[p.Name] = true
	}

	for _, name := range expectedProviders {
		if !providerMap[name] {
			t.Errorf("expected provider %s not found in AllProviders", name)
		}
	}
}

func TestInitWizardSetDimensions(t *testing.T) {
	m := NewInitWizardModel()

	m.SetDimensions(100, 50)

	if m.width != 100 {
		t.Errorf("expected width 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height 50, got %d", m.height)
	}
}

func TestInitWizardWindowSizeMsg(t *testing.T) {
	m := NewInitWizardModel()

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newModel.(InitWizardModel)

	if m.width != 120 || m.height != 40 {
		t.Error("window size not updated correctly")
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
