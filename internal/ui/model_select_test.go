package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmeurs/continueplz/internal/models"
)

func TestNewModelSelectModel(t *testing.T) {
	m := NewModelSelectModel()

	if m.cursor != 0 {
		t.Errorf("Expected cursor to be 0, got %d", m.cursor)
	}
	if m.selected != -1 {
		t.Errorf("Expected selected to be -1, got %d", m.selected)
	}
	if m.loading {
		t.Error("Expected loading to be false")
	}
	if !m.focused {
		t.Error("Expected focused to be true")
	}
	if len(m.modelList) == 0 {
		t.Error("Expected modelList to contain models")
	}
	if len(m.allModels) == 0 {
		t.Error("Expected allModels to contain models")
	}
}

func TestNewModelSelectModelWithGPU(t *testing.T) {
	gpu := &models.GPU{Name: "A100-40GB", VRAM: 40}
	m := NewModelSelectModelWithGPU(gpu)

	if m.selectedGPU == nil {
		t.Fatal("Expected selectedGPU to be set")
	}
	if m.selectedGPU.Name != "A100-40GB" {
		t.Errorf("Expected selectedGPU.Name to be A100-40GB, got %s", m.selectedGPU.Name)
	}

	// All displayed models should be compatible with 40GB VRAM
	for _, model := range m.modelList {
		if model.VRAM > 40 {
			t.Errorf("Model %s requires %dGB VRAM but GPU only has 40GB", model.Name, model.VRAM)
		}
	}
}

func TestModelSelectModel_NavigationUp(t *testing.T) {
	m := NewModelSelectModel()
	m.ready = true
	m.cursor = 3

	// Test up arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 2 {
		t.Errorf("Expected cursor 2 after 'k', got %d", m.cursor)
	}

	// Test up key
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 1 {
		t.Errorf("Expected cursor 1 after up arrow, got %d", m.cursor)
	}

	// Test boundary - can't go below 0
	m.cursor = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("Expected cursor 0 at boundary, got %d", m.cursor)
	}
}

func TestModelSelectModel_NavigationDown(t *testing.T) {
	m := NewModelSelectModel()
	m.ready = true
	m.cursor = 0

	// Test down arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("Expected cursor 1 after down arrow, got %d", m.cursor)
	}

	// Test j key (vim-style)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 2 {
		t.Errorf("Expected cursor 2 after 'j', got %d", m.cursor)
	}

	// Test boundary - can't go beyond list length
	m.cursor = len(m.modelList) - 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != len(m.modelList)-1 {
		t.Errorf("Expected cursor %d at boundary, got %d", len(m.modelList)-1, m.cursor)
	}
}

func TestModelSelectModel_NavigationHomeEnd(t *testing.T) {
	m := NewModelSelectModel()
	m.ready = true
	m.cursor = 5

	// Test home key
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.cursor != 0 {
		t.Errorf("Expected cursor 0 after home, got %d", m.cursor)
	}

	// Test end key
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.cursor != len(m.modelList)-1 {
		t.Errorf("Expected cursor %d after end, got %d", len(m.modelList)-1, m.cursor)
	}
}

func TestModelSelectModel_NavigationPageUpDown(t *testing.T) {
	m := NewModelSelectModel()
	m.ready = true

	// Start at position 6
	m.cursor = 6

	// Page up should move 5 positions
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.cursor != 1 {
		t.Errorf("Expected cursor 1 after pgup, got %d", m.cursor)
	}

	// Page up from position 1 should go to 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.cursor != 0 {
		t.Errorf("Expected cursor 0 after second pgup, got %d", m.cursor)
	}

	// Page down should move 5 positions
	m.cursor = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.cursor != 5 {
		t.Errorf("Expected cursor 5 after pgdown, got %d", m.cursor)
	}
}

func TestModelSelectModel_Selection(t *testing.T) {
	m := NewModelSelectModel()
	m.ready = true
	m.cursor = 2

	// Test Enter key
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.selected != 2 {
		t.Errorf("Expected selected 2 after Enter, got %d", m.selected)
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil after selection")
	}

	// Execute the command and check the message
	msg := cmd()
	selectedMsg, ok := msg.(ModelSelectedMsg)
	if !ok {
		t.Fatalf("Expected ModelSelectedMsg, got %T", msg)
	}
	if selectedMsg.Model.Name != m.modelList[2].Name {
		t.Errorf("Expected selected model %s, got %s", m.modelList[2].Name, selectedMsg.Model.Name)
	}
}

func TestModelSelectModel_SelectionWithSpace(t *testing.T) {
	m := NewModelSelectModel()
	m.ready = true
	m.cursor = 1

	// Test space key
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	if m.selected != 1 {
		t.Errorf("Expected selected 1 after space, got %d", m.selected)
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil after selection")
	}
}

func TestModelSelectModel_RefreshKey(t *testing.T) {
	m := NewModelSelectModel()
	m.ready = true

	// Test r key
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if !m.loading {
		t.Error("Expected loading to be true after refresh")
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil after refresh")
	}

	// Execute the command
	msg := cmd()
	_, ok := msg.(RefreshModelsMsg)
	if !ok {
		t.Fatalf("Expected RefreshModelsMsg, got %T", msg)
	}
}

func TestModelSelectModel_ShowAllKey(t *testing.T) {
	gpu := &models.GPU{Name: "A100-40GB", VRAM: 40}
	m := NewModelSelectModelWithGPU(gpu)
	m.ready = true

	// Verify GPU filter is set
	if m.selectedGPU == nil {
		t.Fatal("Expected GPU filter to be set")
	}
	filteredCount := len(m.modelList)

	// Test 'a' key to show all
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if m.selectedGPU != nil {
		t.Error("Expected selectedGPU to be nil after 'a' key")
	}
	if len(m.modelList) <= filteredCount {
		t.Errorf("Expected more models after showing all, got %d", len(m.modelList))
	}
}

func TestModelSelectModel_ModelsLoadedMsg(t *testing.T) {
	m := NewModelSelectModel()
	m.loading = true

	testModels := []models.Model{
		{Name: "test-model:7b", Params: "7B", VRAM: 8, Quality: 3, Tier: models.TierSmall},
	}

	m, _ = m.Update(ModelsLoadedMsg{Models: testModels})

	if m.loading {
		t.Error("Expected loading to be false")
	}
	if len(m.allModels) != 1 {
		t.Errorf("Expected 1 model, got %d", len(m.allModels))
	}
	if m.err != nil {
		t.Errorf("Expected err to be nil, got %v", m.err)
	}
}

func TestModelSelectModel_ModelsLoadErrorMsg(t *testing.T) {
	m := NewModelSelectModel()
	m.loading = true

	testErr := ErrTestError
	m, _ = m.Update(ModelsLoadErrorMsg{Err: testErr})

	if m.loading {
		t.Error("Expected loading to be false")
	}
	if m.err == nil {
		t.Error("Expected err to be set")
	}
}

func TestModelSelectModel_GPUSelectedMsg(t *testing.T) {
	m := NewModelSelectModel()
	m.ready = true
	initialCount := len(m.modelList)

	// Select a GPU with limited VRAM
	gpu := models.GPU{Name: "A100-40GB", VRAM: 40}
	m, _ = m.Update(GPUSelectedMsg{GPU: gpu})

	if m.selectedGPU == nil {
		t.Fatal("Expected selectedGPU to be set")
	}
	if m.selectedGPU.Name != "A100-40GB" {
		t.Errorf("Expected GPU name A100-40GB, got %s", m.selectedGPU.Name)
	}
	// Should have fewer models after filtering
	if len(m.modelList) >= initialCount {
		t.Errorf("Expected fewer models after GPU filter, got %d (was %d)", len(m.modelList), initialCount)
	}
}

func TestModelSelectModel_Sorting(t *testing.T) {
	m := NewModelSelectModel()

	// Models should be sorted by tier (large first) and quality (highest first within tier)
	// Tier order: Large(0) < Medium(1) < Small(2)
	tierOrder := map[models.Tier]int{
		models.TierLarge:  0,
		models.TierMedium: 1,
		models.TierSmall:  2,
	}

	// Verify the sorting - tier order should be non-decreasing
	// and within the same tier, quality should be non-increasing
	for i := 1; i < len(m.modelList); i++ {
		prevModel := m.modelList[i-1]
		currModel := m.modelList[i]

		prevTierOrder := tierOrder[prevModel.Tier]
		currTierOrder := tierOrder[currModel.Tier]

		// Tier order should be non-decreasing (Large=0 comes before Medium=1 comes before Small=2)
		if currTierOrder < prevTierOrder {
			t.Errorf("Model at index %d (%s, tier %s) should not come after model at index %d (%s, tier %s)",
				i, currModel.Name, currModel.Tier, i-1, prevModel.Name, prevModel.Tier)
		}

		// Within the same tier, quality should be non-increasing (highest quality first)
		if currTierOrder == prevTierOrder && currModel.Quality > prevModel.Quality {
			t.Errorf("Within tier %s: model at index %d (%s, quality %d) should not have higher quality than model at index %d (%s, quality %d)",
				currModel.Tier, i, currModel.Name, currModel.Quality, i-1, prevModel.Name, prevModel.Quality)
		}
	}

	// Verify we have models from all tiers
	tierCounts := make(map[models.Tier]int)
	for _, model := range m.modelList {
		tierCounts[model.Tier]++
	}
	if tierCounts[models.TierLarge] == 0 {
		t.Error("Expected at least one large tier model")
	}
	if tierCounts[models.TierMedium] == 0 {
		t.Error("Expected at least one medium tier model")
	}
	if tierCounts[models.TierSmall] == 0 {
		t.Error("Expected at least one small tier model")
	}
}

func TestModelSelectModel_ViewRendering_Loading(t *testing.T) {
	m := NewModelSelectModel()
	m.SetDimensions(80, 24)
	m.loading = true

	view := m.View()

	if view == "" {
		t.Error("Expected non-empty view")
	}
	if !contains(view, "Loading") {
		t.Error("Expected 'Loading' in view during loading state")
	}
}

func TestModelSelectModel_ViewRendering_Error(t *testing.T) {
	m := NewModelSelectModel()
	m.SetDimensions(80, 24)
	m.err = ErrTestError
	m.loading = false

	view := m.View()

	if view == "" {
		t.Error("Expected non-empty view")
	}
	if !contains(view, "Error") {
		t.Error("Expected 'Error' in view during error state")
	}
	if !contains(view, "retry") {
		t.Error("Expected retry hint in view during error state")
	}
}

func TestModelSelectModel_ViewRendering_Empty(t *testing.T) {
	m := NewModelSelectModel()
	m.SetDimensions(80, 24)
	m.modelList = []models.Model{}
	m.allModels = []models.Model{}

	view := m.View()

	if view == "" {
		t.Error("Expected non-empty view")
	}
	if !contains(view, "No compatible models") {
		t.Error("Expected 'No compatible models' in view when empty")
	}
}

func TestModelSelectModel_ViewRendering_Normal(t *testing.T) {
	m := NewModelSelectModel()
	m.SetDimensions(100, 30)

	view := m.View()

	if view == "" {
		t.Error("Expected non-empty view")
	}

	// Check for expected column headers
	if !contains(view, "Model") {
		t.Error("Expected 'Model' column header in view")
	}
	if !contains(view, "Size") {
		t.Error("Expected 'Size' column header in view")
	}
	if !contains(view, "VRAM") {
		t.Error("Expected 'VRAM' column header in view")
	}
	if !contains(view, "Quality") {
		t.Error("Expected 'Quality' column header in view")
	}
	if !contains(view, "Compatible GPUs") {
		t.Error("Expected 'Compatible GPUs' column header in view")
	}
}

func TestModelSelectModel_ViewRendering_WithGPUFilter(t *testing.T) {
	gpu := &models.GPU{Name: "A100-40GB", VRAM: 40}
	m := NewModelSelectModelWithGPU(gpu)
	m.SetDimensions(100, 30)

	view := m.View()

	if !contains(view, "A100-40GB") {
		t.Error("Expected GPU name in title when filtered")
	}
}

func TestModelSelectModel_QualityStarsDisplay(t *testing.T) {
	m := NewModelSelectModel()
	m.SetDimensions(100, 30)

	view := m.View()

	// View should contain star characters
	if !contains(view, "★") && !contains(view, "☆") {
		t.Error("Expected quality stars in view")
	}
}

func TestModelSelectModel_FocusManagement(t *testing.T) {
	m := NewModelSelectModel()
	m.ready = true

	// When unfocused, key presses should be ignored
	m.SetFocused(false)
	m.cursor = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 0 {
		t.Error("Expected cursor to not change when unfocused")
	}

	// When focused, key presses should work
	m.SetFocused(true)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor == 0 {
		t.Error("Expected cursor to change when focused")
	}
}

func TestModelSelectModel_GettersSetters(t *testing.T) {
	m := NewModelSelectModel()
	m.SetDimensions(80, 24)

	// Test dimensions
	if m.Width() != 80 {
		t.Errorf("Expected width 80, got %d", m.Width())
	}
	if m.Height() != 24 {
		t.Errorf("Expected height 24, got %d", m.Height())
	}
	if !m.IsReady() {
		t.Error("Expected IsReady to be true after SetDimensions")
	}

	// Test cursor
	m.SetCursor(5)
	if m.GetCursor() != 5 {
		t.Errorf("Expected cursor 5, got %d", m.GetCursor())
	}

	// Test invalid cursor - should not change
	m.SetCursor(-1)
	if m.GetCursor() != 5 {
		t.Error("Expected cursor to not change for invalid value")
	}
	m.SetCursor(1000)
	if m.GetCursor() != 5 {
		t.Error("Expected cursor to not change for out of bounds value")
	}

	// Test loading
	m.SetLoading(true)
	if !m.IsLoading() {
		t.Error("Expected IsLoading to be true")
	}

	// Test error
	m.SetError(ErrTestError)
	if m.GetError() == nil {
		t.Error("Expected error to be set")
	}
	if m.IsLoading() {
		t.Error("Expected IsLoading to be false after SetError")
	}

	// Test focus
	m.SetFocused(false)
	if m.IsFocused() {
		t.Error("Expected IsFocused to be false")
	}
}

func TestModelSelectModel_GetCurrentModel(t *testing.T) {
	m := NewModelSelectModel()
	m.cursor = 2

	model := m.GetCurrentModel()
	if model == nil {
		t.Fatal("Expected current model to be non-nil")
	}
	if model.Name != m.modelList[2].Name {
		t.Errorf("Expected model %s, got %s", m.modelList[2].Name, model.Name)
	}

	// Test with empty list
	m.modelList = []models.Model{}
	model = m.GetCurrentModel()
	if model != nil {
		t.Error("Expected nil when modelList is empty")
	}
}

func TestModelSelectModel_GetSelectedModel(t *testing.T) {
	m := NewModelSelectModel()

	// No selection
	model := m.GetSelectedModel()
	if model != nil {
		t.Error("Expected nil when nothing selected")
	}

	// With selection
	m.selected = 1
	model = m.GetSelectedModel()
	if model == nil {
		t.Fatal("Expected selected model to be non-nil")
	}
	if model.Name != m.modelList[1].Name {
		t.Errorf("Expected model %s, got %s", m.modelList[1].Name, model.Name)
	}
}

func TestModelSelectModel_ModelCount(t *testing.T) {
	m := NewModelSelectModel()

	if m.ModelCount() == 0 {
		t.Error("Expected ModelCount > 0")
	}
	if m.TotalModelCount() == 0 {
		t.Error("Expected TotalModelCount > 0")
	}
	if m.ModelCount() != m.TotalModelCount() {
		t.Error("Expected ModelCount == TotalModelCount when no filter")
	}

	// With GPU filter
	gpu := &models.GPU{Name: "A100-40GB", VRAM: 40}
	m.SetSelectedGPU(gpu)
	if m.ModelCount() >= m.TotalModelCount() {
		t.Error("Expected ModelCount < TotalModelCount with filter")
	}
}

func TestModelSelectModel_FilterModels(t *testing.T) {
	m := NewModelSelectModel()
	totalCount := len(m.modelList)

	// Filter by GPU with limited VRAM
	gpu := &models.GPU{Name: "Test-8GB", VRAM: 8}
	m.SetSelectedGPU(gpu)

	// Should have fewer models
	if len(m.modelList) >= totalCount {
		t.Error("Expected fewer models after filtering")
	}

	// All remaining models should fit in 8GB
	for _, model := range m.modelList {
		if model.VRAM > 8 {
			t.Errorf("Model %s requires %dGB but GPU only has 8GB", model.Name, model.VRAM)
		}
	}

	// Clear filter
	m.SetSelectedGPU(nil)
	if len(m.modelList) != totalCount {
		t.Errorf("Expected %d models after clearing filter, got %d", totalCount, len(m.modelList))
	}
}

func TestModelSelectModel_FormatCompatibleGPUs(t *testing.T) {
	m := NewModelSelectModel()

	// Test with a model that should be compatible with multiple GPUs
	smallModel := &models.Model{Name: "test:7b", VRAM: 8}
	gpusStr := m.formatCompatibleGPUs(smallModel)

	// Should have multiple GPU names
	if gpusStr == "" || gpusStr == "-" {
		t.Error("Expected compatible GPUs for small model")
	}

	// Test with a model that requires more VRAM than any GPU
	hugeModel := &models.Model{Name: "huge:1000b", VRAM: 1000}
	gpusStr = m.formatCompatibleGPUs(hugeModel)
	if gpusStr != "-" {
		t.Errorf("Expected '-' for model requiring 1000GB VRAM, got %s", gpusStr)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestModelSelectModel_WindowSizeMsg(t *testing.T) {
	m := NewModelSelectModel()

	if m.ready {
		t.Error("Expected ready to be false initially")
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if !m.ready {
		t.Error("Expected ready to be true after WindowSizeMsg")
	}
	if m.width != 120 {
		t.Errorf("Expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("Expected height 40, got %d", m.height)
	}
}

func TestModelSelectModel_Init(t *testing.T) {
	m := NewModelSelectModel()
	cmd := m.Init()

	if cmd != nil {
		t.Error("Expected Init to return nil command")
	}
}

func TestModelSelectModel_NotReadyView(t *testing.T) {
	m := NewModelSelectModel()
	// Don't set dimensions, so ready remains false

	view := m.View()
	if view != "" {
		t.Error("Expected empty view when not ready")
	}
}

// ErrTestError is a test error for testing
var ErrTestError = &testError{}

type testError struct{}

func (e *testError) Error() string {
	return "test error"
}

// Note: contains() function is defined in provider_select_test.go and is available
// to all tests in this package
