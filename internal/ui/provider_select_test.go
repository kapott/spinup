package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmeurs/spinup/internal/provider"
)

// Helper function to create test offers
func createTestOffers() []provider.Offer {
	spot065 := 0.65
	spot089 := 0.89
	spot075 := 0.75
	spot045 := 0.45

	return []provider.Offer{
		{
			OfferID:       "vast-1",
			Provider:      "vast.ai",
			GPU:           "A100 40GB",
			VRAM:          40,
			Region:        "EU-West",
			SpotPrice:     &spot065,
			OnDemandPrice: 0.95,
			Available:     true,
		},
		{
			OfferID:       "vast-2",
			Provider:      "vast.ai",
			GPU:           "A100 80GB",
			VRAM:          80,
			Region:        "US-East",
			SpotPrice:     &spot089,
			OnDemandPrice: 1.35,
			Available:     true,
		},
		{
			OfferID:       "lambda-1",
			Provider:      "lambda",
			GPU:           "A100 40GB",
			VRAM:          40,
			Region:        "US-West",
			SpotPrice:     nil, // Lambda doesn't support spot
			OnDemandPrice: 1.10,
			Available:     true,
		},
		{
			OfferID:       "runpod-1",
			Provider:      "runpod",
			GPU:           "A100 40GB",
			VRAM:          40,
			Region:        "EU-Central",
			SpotPrice:     &spot075,
			OnDemandPrice: 1.29,
			Available:     true,
		},
		{
			OfferID:       "runpod-2",
			Provider:      "runpod",
			GPU:           "A6000 48GB",
			VRAM:          48,
			Region:        "US-East",
			SpotPrice:     &spot045,
			OnDemandPrice: 0.79,
			Available:     true,
		},
		{
			OfferID:       "paperspace-1",
			Provider:      "paperspace",
			GPU:           "A100 80GB",
			VRAM:          80,
			Region:        "US-East",
			SpotPrice:     nil, // Paperspace doesn't support spot
			OnDemandPrice: 1.89,
			Available:     true,
		},
	}
}

func TestNewProviderSelectModel(t *testing.T) {
	m := NewProviderSelectModel()

	if m.cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", m.cursor)
	}
	if m.selected != -1 {
		t.Errorf("Expected selected -1, got %d", m.selected)
	}
	if !m.loading {
		t.Error("Expected loading to be true initially")
	}
	if len(m.offers) != 0 {
		t.Errorf("Expected empty offers, got %d", len(m.offers))
	}
	if !m.focused {
		t.Error("Expected focused to be true initially")
	}
}

func TestNewProviderSelectModelWithOffers(t *testing.T) {
	offers := createTestOffers()
	m := NewProviderSelectModelWithOffers(offers)

	if len(m.offers) != len(offers) {
		t.Errorf("Expected %d offers, got %d", len(offers), len(m.offers))
	}
	if m.loading {
		t.Error("Expected loading to be false with pre-loaded offers")
	}
}

func TestProviderSelectModel_Navigation(t *testing.T) {
	offers := createTestOffers()
	m := NewProviderSelectModelWithOffers(offers)
	m.SetDimensions(80, 24)

	// Test down navigation
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("Expected cursor 1 after down, got %d", m.cursor)
	}

	// Test 'j' for down (vim-style)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 2 {
		t.Errorf("Expected cursor 2 after 'j', got %d", m.cursor)
	}

	// Test up navigation
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 1 {
		t.Errorf("Expected cursor 1 after up, got %d", m.cursor)
	}

	// Test 'k' for up (vim-style)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 0 {
		t.Errorf("Expected cursor 0 after 'k', got %d", m.cursor)
	}

	// Test boundary - can't go above 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", m.cursor)
	}
}

func TestProviderSelectModel_NavigationPgUpPgDown(t *testing.T) {
	offers := createTestOffers()
	m := NewProviderSelectModelWithOffers(offers)
	m.SetDimensions(80, 24)

	// Test pgdown
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.cursor != 5 { // Should move 5 positions or to end
		t.Errorf("Expected cursor 5 after pgdown, got %d", m.cursor)
	}

	// Test pgup
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.cursor != 0 { // Should move back 5 positions
		t.Errorf("Expected cursor 0 after pgup, got %d", m.cursor)
	}

	// Test home
	m.cursor = 3
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.cursor != 0 {
		t.Errorf("Expected cursor 0 after home, got %d", m.cursor)
	}

	// Test end
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.cursor != len(offers)-1 {
		t.Errorf("Expected cursor at end, got %d", m.cursor)
	}
}

func TestProviderSelectModel_Selection(t *testing.T) {
	offers := createTestOffers()
	m := NewProviderSelectModelWithOffers(offers)
	m.SetDimensions(80, 24)

	// Navigate to second item
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Select with Enter
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.selected != 1 {
		t.Errorf("Expected selected 1, got %d", m.selected)
	}

	// Verify the command returns an OfferSelectedMsg
	if cmd == nil {
		t.Fatal("Expected command to be returned on selection")
	}

	msg := cmd()
	selectedMsg, ok := msg.(OfferSelectedMsg)
	if !ok {
		t.Fatalf("Expected OfferSelectedMsg, got %T", msg)
	}

	// The offers should be sorted by price, verify we got the correct one
	if selectedMsg.Offer.OfferID == "" {
		t.Error("Expected offer to have an ID")
	}
}

func TestProviderSelectModel_Refresh(t *testing.T) {
	offers := createTestOffers()
	m := NewProviderSelectModelWithOffers(offers)
	m.SetDimensions(80, 24)

	// Press 'r' to refresh
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if !m.loading {
		t.Error("Expected loading to be true after refresh")
	}

	// Verify the command returns a RefreshOffersMsg
	if cmd == nil {
		t.Fatal("Expected command to be returned on refresh")
	}

	msg := cmd()
	if _, ok := msg.(RefreshOffersMsg); !ok {
		t.Fatalf("Expected RefreshOffersMsg, got %T", msg)
	}
}

func TestProviderSelectModel_OffersLoaded(t *testing.T) {
	m := NewProviderSelectModel()
	m.SetDimensions(80, 24)

	if !m.loading {
		t.Error("Expected loading to be true initially")
	}

	offers := createTestOffers()
	m, _ = m.Update(OffersLoadedMsg{Offers: offers})

	if m.loading {
		t.Error("Expected loading to be false after offers loaded")
	}
	if len(m.offers) != len(offers) {
		t.Errorf("Expected %d offers, got %d", len(offers), len(m.offers))
	}
	if m.err != nil {
		t.Errorf("Expected no error, got %v", m.err)
	}
}

func TestProviderSelectModel_OffersLoadError(t *testing.T) {
	m := NewProviderSelectModel()
	m.SetDimensions(80, 24)

	testErr := provider.NewProviderError("test", "test error", nil)
	m, _ = m.Update(OffersLoadErrorMsg{Err: testErr})

	if m.loading {
		t.Error("Expected loading to be false after error")
	}
	if m.err == nil {
		t.Error("Expected error to be set")
	}
	if m.err.Error() != testErr.Error() {
		t.Errorf("Expected error %v, got %v", testErr, m.err)
	}
}

func TestProviderSelectModel_Sorting(t *testing.T) {
	offers := createTestOffers()
	m := NewProviderSelectModelWithOffers(offers)

	// After loading, offers should be sorted by effective price (spot if available)
	// The cheapest should be runpod A6000 at €0.45 spot
	if len(m.offers) == 0 {
		t.Fatal("Expected offers to be loaded")
	}

	// First offer should be the cheapest (runpod A6000 at €0.45)
	first := m.offers[0]
	expectedPrice := 0.45
	actualPrice := effectivePrice(first)
	if actualPrice != expectedPrice {
		t.Errorf("Expected first offer to have effective price %.2f, got %.2f (offer: %s %s)",
			expectedPrice, actualPrice, first.Provider, first.GPU)
	}

	// Verify entire list is sorted
	for i := 1; i < len(m.offers); i++ {
		prevPrice := effectivePrice(m.offers[i-1])
		currPrice := effectivePrice(m.offers[i])
		if prevPrice > currPrice {
			t.Errorf("Offers not sorted: offer %d (%.2f) > offer %d (%.2f)",
				i-1, prevPrice, i, currPrice)
		}
	}
}

func TestProviderSelectModel_View(t *testing.T) {
	offers := createTestOffers()
	m := NewProviderSelectModelWithOffers(offers)
	m.SetDimensions(100, 30)

	view := m.View()

	// Check that view contains expected elements
	if view == "" {
		t.Error("Expected non-empty view")
	}

	// Should contain the title
	if !contains(view, "Available Configurations") {
		t.Error("Expected view to contain title 'Available Configurations'")
	}

	// Should contain column headers
	if !contains(view, "Provider") {
		t.Error("Expected view to contain 'Provider' header")
	}
	if !contains(view, "GPU") {
		t.Error("Expected view to contain 'GPU' header")
	}
	if !contains(view, "Region") {
		t.Error("Expected view to contain 'Region' header")
	}
	if !contains(view, "Spot/hr") {
		t.Error("Expected view to contain 'Spot/hr' header")
	}

	// Should contain key hints
	if !contains(view, "Navigate") {
		t.Error("Expected view to contain navigation hint")
	}
	if !contains(view, "Select") {
		t.Error("Expected view to contain select hint")
	}
	if !contains(view, "Refresh") {
		t.Error("Expected view to contain refresh hint")
	}
}

func TestProviderSelectModel_ViewLoading(t *testing.T) {
	m := NewProviderSelectModel()
	m.SetDimensions(100, 30)

	view := m.View()

	// Should show loading state
	if !contains(view, "Loading") {
		t.Error("Expected view to show loading state")
	}
}

func TestProviderSelectModel_ViewError(t *testing.T) {
	m := NewProviderSelectModel()
	m.SetDimensions(100, 30)
	m.SetError(provider.NewProviderError("test", "Test error message", nil))

	view := m.View()

	// Should show error
	if !contains(view, "Error") {
		t.Error("Expected view to show error")
	}
	if !contains(view, "Test error") {
		t.Error("Expected view to contain error message")
	}
	// Should show retry hint
	if !contains(view, "retry") {
		t.Error("Expected view to show retry hint")
	}
}

func TestProviderSelectModel_ViewEmpty(t *testing.T) {
	m := NewProviderSelectModel()
	m.SetDimensions(100, 30)
	m.SetOffers([]provider.Offer{}) // Empty offers

	view := m.View()

	// Should show empty state
	if !contains(view, "No offers available") {
		t.Error("Expected view to show 'No offers available'")
	}
}

func TestProviderSelectModel_Focus(t *testing.T) {
	m := NewProviderSelectModel()
	m.SetDimensions(80, 24)

	// Should be focused by default
	if !m.IsFocused() {
		t.Error("Expected model to be focused by default")
	}

	// Unfocus
	m.SetFocused(false)
	if m.IsFocused() {
		t.Error("Expected model to be unfocused")
	}

	// Key presses should be ignored when unfocused
	m.cursor = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 0 {
		t.Error("Expected cursor to remain at 0 when unfocused")
	}

	// Re-focus
	m.SetFocused(true)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Still at 0 because no offers loaded, but the point is focus check
}

func TestProviderSelectModel_Getters(t *testing.T) {
	offers := createTestOffers()
	m := NewProviderSelectModelWithOffers(offers)
	m.SetDimensions(80, 24)

	// Test GetOffers
	if len(m.GetOffers()) != len(offers) {
		t.Errorf("Expected %d offers from GetOffers", len(offers))
	}

	// Test GetCursor
	if m.GetCursor() != 0 {
		t.Errorf("Expected cursor 0, got %d", m.GetCursor())
	}

	// Test GetSelected (should be -1 initially)
	if m.GetSelected() != -1 {
		t.Errorf("Expected selected -1, got %d", m.GetSelected())
	}

	// Test GetSelectedOffer (should be nil initially)
	if m.GetSelectedOffer() != nil {
		t.Error("Expected GetSelectedOffer to return nil initially")
	}

	// Test GetCurrentOffer
	current := m.GetCurrentOffer()
	if current == nil {
		t.Error("Expected GetCurrentOffer to return an offer")
	}

	// Test dimensions
	if m.Width() != 80 {
		t.Errorf("Expected width 80, got %d", m.Width())
	}
	if m.Height() != 24 {
		t.Errorf("Expected height 24, got %d", m.Height())
	}
	if !m.IsReady() {
		t.Error("Expected IsReady to be true")
	}
}

func TestProviderSelectModel_SetCursor(t *testing.T) {
	offers := createTestOffers()
	m := NewProviderSelectModelWithOffers(offers)

	// Valid cursor position
	m.SetCursor(3)
	if m.GetCursor() != 3 {
		t.Errorf("Expected cursor 3, got %d", m.GetCursor())
	}

	// Invalid cursor position (negative)
	m.SetCursor(-1)
	if m.GetCursor() != 3 { // Should remain unchanged
		t.Errorf("Expected cursor to remain 3 for negative value, got %d", m.GetCursor())
	}

	// Invalid cursor position (too high)
	m.SetCursor(100)
	if m.GetCursor() != 3 { // Should remain unchanged
		t.Errorf("Expected cursor to remain 3 for out of range value, got %d", m.GetCursor())
	}
}

func TestFormatSpotPrice_ProviderSelect(t *testing.T) {
	// Test nil price
	result := formatSpotPrice(nil)
	if result != "-" {
		t.Errorf("Expected '-' for nil price, got %s", result)
	}

	// Test valid price
	price := 0.65
	result = formatSpotPrice(&price)
	if result != "€0.65" {
		t.Errorf("Expected '€0.65', got %s", result)
	}
}

func TestFormatOnDemandPrice_ProviderSelect(t *testing.T) {
	result := formatOnDemandPrice(1.50)
	if result != "€1.50" {
		t.Errorf("Expected '€1.50', got %s", result)
	}
}

func TestFormatDayEstimate(t *testing.T) {
	spot := 0.65
	offer := provider.Offer{
		SpotPrice:     &spot,
		OnDemandPrice: 0.95,
	}

	result := formatDayEstimate(offer)
	// 0.65 * 8 = 5.20
	if result != "€5.20" {
		t.Errorf("Expected '€5.20', got %s", result)
	}

	// Test without spot price
	offer2 := provider.Offer{
		SpotPrice:     nil,
		OnDemandPrice: 1.00,
	}
	result2 := formatDayEstimate(offer2)
	// 1.00 * 8 = 8.00
	if result2 != "€8.00" {
		t.Errorf("Expected '€8.00', got %s", result2)
	}
}

func TestEffectivePrice(t *testing.T) {
	spot := 0.50
	offer := provider.Offer{
		SpotPrice:     &spot,
		OnDemandPrice: 1.00,
	}

	// Should prefer spot price
	if effectivePrice(offer) != 0.50 {
		t.Errorf("Expected effective price 0.50, got %f", effectivePrice(offer))
	}

	// Without spot, should use on-demand
	offer2 := provider.Offer{
		SpotPrice:     nil,
		OnDemandPrice: 1.00,
	}
	if effectivePrice(offer2) != 1.00 {
		t.Errorf("Expected effective price 1.00, got %f", effectivePrice(offer2))
	}
}

func TestProviderSelectModel_WindowSizeMsg(t *testing.T) {
	m := NewProviderSelectModel()

	if m.IsReady() {
		t.Error("Expected model to not be ready initially")
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if !m.IsReady() {
		t.Error("Expected model to be ready after WindowSizeMsg")
	}
	if m.Width() != 120 {
		t.Errorf("Expected width 120, got %d", m.Width())
	}
	if m.Height() != 40 {
		t.Errorf("Expected height 40, got %d", m.Height())
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
