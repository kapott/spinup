package ui

import (
	"math"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmeurs/continueplz/internal/models"
	"github.com/tmeurs/continueplz/internal/provider"
)

func TestNewCostBreakdownModel(t *testing.T) {
	m := NewCostBreakdownModel()

	if m.storageGB != DefaultStorageGB {
		t.Errorf("Expected storageGB %d, got %d", DefaultStorageGB, m.storageGB)
	}

	if m.workingHours != DefaultWorkingHours {
		t.Errorf("Expected workingHours %d, got %d", DefaultWorkingHours, m.workingHours)
	}

	if !m.useSpot {
		t.Error("Expected useSpot to be true by default")
	}

	if m.selectedOffer != nil {
		t.Error("Expected selectedOffer to be nil")
	}

	if m.selectedModel != nil {
		t.Error("Expected selectedModel to be nil")
	}
}

func TestNewCostBreakdownModelWithOffer(t *testing.T) {
	spotPrice := 0.65
	offer := &provider.Offer{
		Provider:      "vast",
		GPU:           "A100 40GB",
		SpotPrice:     &spotPrice,
		OnDemandPrice: 0.85,
		StoragePrice:  0.0005, // per GB/hr
	}
	model := &models.Model{
		Name:   "qwen2.5-coder:32b",
		Params: "32B",
		VRAM:   36,
	}

	m := NewCostBreakdownModelWithOffer(offer, model)

	if m.selectedOffer == nil {
		t.Fatal("Expected selectedOffer to be set")
	}
	if m.selectedOffer.Provider != "vast" {
		t.Errorf("Expected provider 'vast', got '%s'", m.selectedOffer.Provider)
	}

	if m.selectedModel == nil {
		t.Fatal("Expected selectedModel to be set")
	}
	if m.selectedModel.Name != "qwen2.5-coder:32b" {
		t.Errorf("Expected model 'qwen2.5-coder:32b', got '%s'", m.selectedModel.Name)
	}
}

func TestCostBreakdownCalculation(t *testing.T) {
	spotPrice := 0.65
	offer := &provider.Offer{
		Provider:      "vast",
		GPU:           "A100 40GB",
		SpotPrice:     &spotPrice,
		OnDemandPrice: 0.85,
		StoragePrice:  0.0005, // per GB/hr = €0.05/hr for 100GB
	}

	m := NewCostBreakdownModel()
	m.SetSelectedOffer(offer)
	m.SetDimensions(120, 40)

	costs := m.GetCostBreakdown()

	// Test spot pricing is used
	if !costs.IsSpot {
		t.Error("Expected IsSpot to be true when spot price is available")
	}

	// Test compute hourly (should use spot price)
	if !floatEquals(costs.ComputeHourly, 0.65) {
		t.Errorf("Expected ComputeHourly 0.65, got %.2f", costs.ComputeHourly)
	}

	// Test storage hourly (0.0005 * 100 = 0.05)
	expectedStorageHourly := 0.0005 * float64(DefaultStorageGB)
	if !floatEquals(costs.StorageHourly, expectedStorageHourly) {
		t.Errorf("Expected StorageHourly %.4f, got %.4f", expectedStorageHourly, costs.StorageHourly)
	}

	// Test daily compute (0.65 * 8 = 5.20)
	expectedComputeDaily := 0.65 * float64(DefaultWorkingHours)
	if !floatEquals(costs.ComputeDaily, expectedComputeDaily) {
		t.Errorf("Expected ComputeDaily %.2f, got %.2f", expectedComputeDaily, costs.ComputeDaily)
	}

	// Test daily storage (0.05 * 8 = 0.40)
	expectedStorageDaily := expectedStorageHourly * float64(DefaultWorkingHours)
	if !floatEquals(costs.StorageDaily, expectedStorageDaily) {
		t.Errorf("Expected StorageDaily %.2f, got %.2f", expectedStorageDaily, costs.StorageDaily)
	}

	// Test total daily (5.20 + 0.40 = 5.60)
	expectedTotalDaily := expectedComputeDaily + expectedStorageDaily
	if !floatEquals(costs.TotalDaily, expectedTotalDaily) {
		t.Errorf("Expected TotalDaily %.2f, got %.2f", expectedTotalDaily, costs.TotalDaily)
	}
}

func TestCostBreakdownOnDemand(t *testing.T) {
	offer := &provider.Offer{
		Provider:      "lambda",
		GPU:           "A100 40GB",
		SpotPrice:     nil, // No spot available
		OnDemandPrice: 0.85,
		StoragePrice:  0.0005,
	}

	m := NewCostBreakdownModel()
	m.SetSelectedOffer(offer)
	m.SetDimensions(120, 40)

	costs := m.GetCostBreakdown()

	// Test on-demand pricing is used when spot is not available
	if costs.IsSpot {
		t.Error("Expected IsSpot to be false when spot price is nil")
	}

	if !floatEquals(costs.ComputeHourly, 0.85) {
		t.Errorf("Expected ComputeHourly 0.85, got %.2f", costs.ComputeHourly)
	}
}

func TestCostBreakdownForceOnDemand(t *testing.T) {
	spotPrice := 0.65
	offer := &provider.Offer{
		Provider:      "vast",
		GPU:           "A100 40GB",
		SpotPrice:     &spotPrice,
		OnDemandPrice: 0.85,
		StoragePrice:  0.0005,
	}

	m := NewCostBreakdownModel()
	m.SetSelectedOffer(offer)
	m.SetUseSpot(false) // Force on-demand
	m.SetDimensions(120, 40)

	costs := m.GetCostBreakdown()

	if costs.IsSpot {
		t.Error("Expected IsSpot to be false when useSpot is false")
	}

	if !floatEquals(costs.ComputeHourly, 0.85) {
		t.Errorf("Expected ComputeHourly 0.85 (on-demand), got %.2f", costs.ComputeHourly)
	}
}

func TestCostBreakdownCustomSettings(t *testing.T) {
	spotPrice := 0.65
	offer := &provider.Offer{
		Provider:      "vast",
		GPU:           "A100 40GB",
		SpotPrice:     &spotPrice,
		OnDemandPrice: 0.85,
		StoragePrice:  0.0005,
	}

	m := NewCostBreakdownModel()
	m.SetSelectedOffer(offer)
	m.SetStorageGB(200)
	m.SetWorkingHours(12)
	m.SetDimensions(120, 40)

	costs := m.GetCostBreakdown()

	if costs.StorageGB != 200 {
		t.Errorf("Expected StorageGB 200, got %d", costs.StorageGB)
	}

	if costs.WorkingHours != 12 {
		t.Errorf("Expected WorkingHours 12, got %d", costs.WorkingHours)
	}

	// Storage hourly should be 0.0005 * 200 = 0.10
	expectedStorageHourly := 0.0005 * 200
	if !floatEquals(costs.StorageHourly, expectedStorageHourly) {
		t.Errorf("Expected StorageHourly %.4f, got %.4f", expectedStorageHourly, costs.StorageHourly)
	}

	// Daily compute should be 0.65 * 12 = 7.80
	expectedComputeDaily := 0.65 * 12
	if !floatEquals(costs.ComputeDaily, expectedComputeDaily) {
		t.Errorf("Expected ComputeDaily %.2f, got %.2f", expectedComputeDaily, costs.ComputeDaily)
	}
}

func TestCostBreakdownNoSelection(t *testing.T) {
	m := NewCostBreakdownModel()
	m.SetDimensions(120, 40)

	costs := m.GetCostBreakdown()

	if costs.ComputeHourly != 0 {
		t.Errorf("Expected ComputeHourly 0 with no selection, got %.2f", costs.ComputeHourly)
	}

	if costs.TotalDaily != 0 {
		t.Errorf("Expected TotalDaily 0 with no selection, got %.2f", costs.TotalDaily)
	}

	if !m.HasSelection() == false {
		t.Error("Expected HasSelection to return false")
	}
}

func TestCostBreakdownViewNoSelection(t *testing.T) {
	m := NewCostBreakdownModel()
	m.SetDimensions(120, 40)

	view := m.View()

	if !strings.Contains(view, "Cost Breakdown") {
		t.Error("Expected view to contain 'Cost Breakdown' title")
	}

	if !strings.Contains(view, "Select a provider") {
		t.Error("Expected view to show selection prompt when no offer selected")
	}
}

func TestCostBreakdownViewWithSelection(t *testing.T) {
	spotPrice := 0.65
	offer := &provider.Offer{
		Provider:      "vast",
		GPU:           "A100 40GB",
		SpotPrice:     &spotPrice,
		OnDemandPrice: 0.85,
		StoragePrice:  0.0005,
	}
	model := &models.Model{
		Name:   "qwen2.5-coder:32b",
		Params: "32B",
		VRAM:   36,
	}

	m := NewCostBreakdownModelWithOffer(offer, model)
	m.SetDimensions(120, 40)

	view := m.View()

	// Check title includes selection
	if !strings.Contains(view, "vast") {
		t.Error("Expected view to contain provider name 'vast'")
	}
	if !strings.Contains(view, "A100 40GB") {
		t.Error("Expected view to contain GPU name 'A100 40GB'")
	}
	if !strings.Contains(view, "qwen2.5-coder:32b") {
		t.Error("Expected view to contain model name")
	}

	// Check cost breakdown lines
	if !strings.Contains(view, "Compute:") {
		t.Error("Expected view to contain 'Compute:' line")
	}
	if !strings.Contains(view, "Storage:") {
		t.Error("Expected view to contain 'Storage:' line")
	}
	if !strings.Contains(view, "Egress:") {
		t.Error("Expected view to contain 'Egress:' line")
	}
	if !strings.Contains(view, "Estimated daily total:") {
		t.Error("Expected view to contain 'Estimated daily total:' line")
	}

	// Check WireGuard note
	if !strings.Contains(view, "WireGuard") {
		t.Error("Expected view to mention WireGuard for egress")
	}
}

func TestCostBreakdownUpdate(t *testing.T) {
	m := NewCostBreakdownModel()
	m.SetDimensions(120, 40)

	// Test OfferSelectedMsg
	spotPrice := 0.65
	offer := provider.Offer{
		Provider:      "vast",
		GPU:           "A100 40GB",
		SpotPrice:     &spotPrice,
		OnDemandPrice: 0.85,
	}
	m, _ = m.Update(OfferSelectedMsg{Offer: offer})

	if m.selectedOffer == nil {
		t.Fatal("Expected selectedOffer to be set after OfferSelectedMsg")
	}
	if m.selectedOffer.Provider != "vast" {
		t.Errorf("Expected provider 'vast', got '%s'", m.selectedOffer.Provider)
	}

	// Test ModelSelectedMsg
	model := models.Model{
		Name:   "qwen2.5-coder:32b",
		Params: "32B",
		VRAM:   36,
	}
	m, _ = m.Update(ModelSelectedMsg{Model: model})

	if m.selectedModel == nil {
		t.Fatal("Expected selectedModel to be set after ModelSelectedMsg")
	}
	if m.selectedModel.Name != "qwen2.5-coder:32b" {
		t.Errorf("Expected model name 'qwen2.5-coder:32b', got '%s'", m.selectedModel.Name)
	}

	// Test CostSettingsMsg
	m, _ = m.Update(CostSettingsMsg{StorageGB: 200, WorkingHours: 12, UseSpot: false})

	if m.storageGB != 200 {
		t.Errorf("Expected storageGB 200, got %d", m.storageGB)
	}
	if m.workingHours != 12 {
		t.Errorf("Expected workingHours 12, got %d", m.workingHours)
	}
	if m.useSpot {
		t.Error("Expected useSpot to be false")
	}
}

func TestCostBreakdownWindowSize(t *testing.T) {
	m := NewCostBreakdownModel()

	if m.IsReady() {
		t.Error("Expected IsReady to return false before WindowSizeMsg")
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if !m.IsReady() {
		t.Error("Expected IsReady to return true after WindowSizeMsg")
	}

	if m.Width() != 120 {
		t.Errorf("Expected Width 120, got %d", m.Width())
	}

	if m.Height() != 40 {
		t.Errorf("Expected Height 40, got %d", m.Height())
	}
}

func TestCostBreakdownSetters(t *testing.T) {
	m := NewCostBreakdownModel()

	// Test SetWorkingHours with invalid values
	m.SetWorkingHours(0) // Should be ignored
	if m.workingHours != DefaultWorkingHours {
		t.Error("SetWorkingHours should ignore 0")
	}

	m.SetWorkingHours(25) // Should be ignored (> 24)
	if m.workingHours != DefaultWorkingHours {
		t.Error("SetWorkingHours should ignore values > 24")
	}

	m.SetWorkingHours(10)
	if m.workingHours != 10 {
		t.Errorf("Expected workingHours 10, got %d", m.workingHours)
	}

	// Test SetStorageGB with invalid values
	m.SetStorageGB(0) // Should be ignored
	if m.storageGB != DefaultStorageGB {
		t.Error("SetStorageGB should ignore 0")
	}

	m.SetStorageGB(50)
	if m.storageGB != 50 {
		t.Errorf("Expected storageGB 50, got %d", m.storageGB)
	}

	// Test SetFocused
	m.SetFocused(true)
	if !m.IsFocused() {
		t.Error("Expected IsFocused to return true")
	}
}

// floatEquals compares two floats with tolerance
func floatEquals(a, b float64) bool {
	const epsilon = 0.001
	return math.Abs(a-b) < epsilon
}
