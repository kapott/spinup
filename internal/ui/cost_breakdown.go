// Package ui provides the TUI (Terminal User Interface) for continueplz.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tmeurs/continueplz/internal/models"
	"github.com/tmeurs/continueplz/internal/provider"
)

// DefaultStorageGB is the default disk size used for cost calculations.
const DefaultStorageGB = 100

// DefaultWorkingHours is the default number of working hours per day for estimates.
const DefaultWorkingHours = 8

// CostBreakdownModel is the Bubbletea model for the cost breakdown panel.
// It displays a breakdown of compute, storage, and egress costs.
type CostBreakdownModel struct {
	// selectedOffer is the currently selected GPU offer (nil if none)
	selectedOffer *provider.Offer

	// selectedModel is the currently selected LLM model (nil if none)
	selectedModel *models.Model

	// storageGB is the storage size in GB for calculations
	storageGB int

	// workingHours is the number of hours per day for daily estimate
	workingHours int

	// useSpot indicates if spot pricing should be used (when available)
	useSpot bool

	// width is the terminal width
	width int

	// height is the terminal height
	height int

	// ready indicates if the component has been initialized
	ready bool

	// focused indicates if this component has focus
	focused bool
}

// CostBreakdown holds the calculated cost breakdown values
type CostBreakdown struct {
	// ComputeHourly is the hourly compute cost
	ComputeHourly float64

	// StorageHourly is the hourly storage cost
	StorageHourly float64

	// EgressEstimate is the estimated egress cost (minimal for WireGuard)
	EgressEstimate float64

	// ComputeDaily is the estimated daily compute cost
	ComputeDaily float64

	// StorageDaily is the estimated daily storage cost
	StorageDaily float64

	// TotalHourly is the total hourly cost
	TotalHourly float64

	// TotalDaily is the estimated total daily cost
	TotalDaily float64

	// IsSpot indicates if spot pricing is being used
	IsSpot bool

	// WorkingHours is the number of hours used for daily estimate
	WorkingHours int

	// StorageGB is the storage size in GB
	StorageGB int
}

// NewCostBreakdownModel creates a new cost breakdown model
func NewCostBreakdownModel() CostBreakdownModel {
	return CostBreakdownModel{
		storageGB:    DefaultStorageGB,
		workingHours: DefaultWorkingHours,
		useSpot:      true, // Prefer spot pricing by default
		focused:      false,
	}
}

// NewCostBreakdownModelWithOffer creates a new model with a pre-selected offer
func NewCostBreakdownModelWithOffer(offer *provider.Offer, model *models.Model) CostBreakdownModel {
	m := NewCostBreakdownModel()
	m.selectedOffer = offer
	m.selectedModel = model
	return m
}

// Init implements tea.Model
func (m CostBreakdownModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m CostBreakdownModel) Update(msg tea.Msg) (CostBreakdownModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case OfferSelectedMsg:
		m.selectedOffer = &msg.Offer
		return m, nil

	case ModelSelectedMsg:
		m.selectedModel = &msg.Model
		return m, nil

	case CostSettingsMsg:
		if msg.StorageGB > 0 {
			m.storageGB = msg.StorageGB
		}
		if msg.WorkingHours > 0 {
			m.workingHours = msg.WorkingHours
		}
		m.useSpot = msg.UseSpot
		return m, nil
	}

	return m, nil
}

// View implements tea.Model
func (m CostBreakdownModel) View() string {
	if !m.ready {
		return ""
	}

	return m.renderPanel()
}

// renderPanel renders the cost breakdown panel
func (m CostBreakdownModel) renderPanel() string {
	var b strings.Builder

	// Panel title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary)

	if m.selectedOffer != nil && m.selectedModel != nil {
		title := fmt.Sprintf("Cost Breakdown (Selected: %s %s + %s)",
			m.selectedOffer.Provider,
			m.selectedOffer.GPU,
			m.selectedModel.Name)
		b.WriteString(titleStyle.Render(title))
	} else if m.selectedOffer != nil {
		title := fmt.Sprintf("Cost Breakdown (Selected: %s %s)",
			m.selectedOffer.Provider,
			m.selectedOffer.GPU)
		b.WriteString(titleStyle.Render(title))
	} else {
		b.WriteString(titleStyle.Render("Cost Breakdown"))
	}
	b.WriteString("\n\n")

	// Handle no selection state
	if m.selectedOffer == nil {
		b.WriteString(Styles.Muted.Render("  Select a provider/GPU to see cost breakdown"))
		b.WriteString("\n")
		return m.renderBox(b.String())
	}

	// Calculate costs
	costs := m.calculateCosts()

	// Compute cost line
	pricingType := "On-Demand"
	if costs.IsSpot {
		pricingType = "Spot"
	}
	computeLine := fmt.Sprintf("  Compute:  %s%.2f/hr  ×  %dhr  =  %s%.2f",
		CurrencyEUR, costs.ComputeHourly,
		costs.WorkingHours,
		CurrencyEUR, costs.ComputeDaily)
	if costs.IsSpot {
		b.WriteString(Styles.PriceSpot.Render(computeLine))
		b.WriteString(Styles.Muted.Render(fmt.Sprintf("  (%s)", pricingType)))
	} else {
		b.WriteString(Styles.Body.Render(computeLine))
	}
	b.WriteString("\n")

	// Storage cost line
	storageLine := fmt.Sprintf("  Storage:  %s%.2f/hr  ×  %dhr  =  %s%.2f  (%dGB model storage)",
		CurrencyEUR, costs.StorageHourly,
		costs.WorkingHours,
		CurrencyEUR, costs.StorageDaily,
		costs.StorageGB)
	b.WriteString(Styles.Body.Render(storageLine))
	b.WriteString("\n")

	// Egress cost line
	egressLine := fmt.Sprintf("  Egress:   ~%s%.2f         (WireGuard tunnel, minimal)",
		CurrencyEUR, costs.EgressEstimate)
	b.WriteString(Styles.Muted.Render(egressLine))
	b.WriteString("\n")

	// Separator
	sepLen := 75
	b.WriteString(Styles.Muted.Render("  " + strings.Repeat(TableHorizontal, sepLen)))
	b.WriteString("\n")

	// Daily total
	totalLine := fmt.Sprintf("  Estimated daily total:     %s%.2f", CurrencyEUR, costs.TotalDaily)
	totalStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorForeground)
	b.WriteString(totalStyle.Render(totalLine))
	b.WriteString("\n")

	return m.renderBox(b.String())
}

// renderBox wraps content in a styled box
func (m CostBreakdownModel) renderBox(content string) string {
	boxWidth := m.width - 4
	if boxWidth < 80 {
		boxWidth = 80
	}
	return Styles.Box.Width(boxWidth).Render(content)
}

// calculateCosts calculates the cost breakdown
func (m CostBreakdownModel) calculateCosts() CostBreakdown {
	if m.selectedOffer == nil {
		return CostBreakdown{
			WorkingHours: m.workingHours,
			StorageGB:    m.storageGB,
		}
	}

	costs := CostBreakdown{
		WorkingHours: m.workingHours,
		StorageGB:    m.storageGB,
	}

	// Determine compute price (spot vs on-demand)
	if m.useSpot && m.selectedOffer.SpotPrice != nil {
		costs.ComputeHourly = *m.selectedOffer.SpotPrice
		costs.IsSpot = true
	} else {
		costs.ComputeHourly = m.selectedOffer.OnDemandPrice
		costs.IsSpot = false
	}

	// Calculate storage cost (per hour)
	costs.StorageHourly = m.selectedOffer.StoragePrice * float64(m.storageGB)

	// Egress estimate (minimal for WireGuard - essentially 0 for most use cases)
	costs.EgressEstimate = 0.00

	// Calculate daily estimates
	costs.ComputeDaily = costs.ComputeHourly * float64(m.workingHours)
	costs.StorageDaily = costs.StorageHourly * float64(m.workingHours)

	// Calculate totals
	costs.TotalHourly = costs.ComputeHourly + costs.StorageHourly
	costs.TotalDaily = costs.ComputeDaily + costs.StorageDaily + costs.EgressEstimate

	return costs
}

// Message types for cost breakdown

// CostSettingsMsg is sent when cost calculation settings change
type CostSettingsMsg struct {
	StorageGB    int
	WorkingHours int
	UseSpot      bool
}

// Getters and setters

// SetSelectedOffer sets the currently selected offer
func (m *CostBreakdownModel) SetSelectedOffer(offer *provider.Offer) {
	m.selectedOffer = offer
}

// GetSelectedOffer returns the currently selected offer
func (m CostBreakdownModel) GetSelectedOffer() *provider.Offer {
	return m.selectedOffer
}

// SetSelectedModel sets the currently selected model
func (m *CostBreakdownModel) SetSelectedModel(model *models.Model) {
	m.selectedModel = model
}

// GetSelectedModel returns the currently selected model
func (m CostBreakdownModel) GetSelectedModel() *models.Model {
	return m.selectedModel
}

// SetStorageGB sets the storage size for calculations
func (m *CostBreakdownModel) SetStorageGB(gb int) {
	if gb > 0 {
		m.storageGB = gb
	}
}

// GetStorageGB returns the storage size
func (m CostBreakdownModel) GetStorageGB() int {
	return m.storageGB
}

// SetWorkingHours sets the working hours per day for estimates
func (m *CostBreakdownModel) SetWorkingHours(hours int) {
	if hours > 0 && hours <= 24 {
		m.workingHours = hours
	}
}

// GetWorkingHours returns the working hours per day
func (m CostBreakdownModel) GetWorkingHours() int {
	return m.workingHours
}

// SetUseSpot sets whether to prefer spot pricing
func (m *CostBreakdownModel) SetUseSpot(useSpot bool) {
	m.useSpot = useSpot
}

// UseSpot returns whether spot pricing is preferred
func (m CostBreakdownModel) UseSpot() bool {
	return m.useSpot
}

// SetFocused sets whether this component has focus
func (m *CostBreakdownModel) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether this component has focus
func (m CostBreakdownModel) IsFocused() bool {
	return m.focused
}

// SetDimensions sets the terminal dimensions
func (m *CostBreakdownModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

// Width returns the terminal width
func (m CostBreakdownModel) Width() int {
	return m.width
}

// Height returns the terminal height
func (m CostBreakdownModel) Height() int {
	return m.height
}

// IsReady returns whether the component has been initialized
func (m CostBreakdownModel) IsReady() bool {
	return m.ready
}

// GetCostBreakdown returns the current calculated cost breakdown
func (m CostBreakdownModel) GetCostBreakdown() CostBreakdown {
	return m.calculateCosts()
}

// HasSelection returns true if an offer is selected
func (m CostBreakdownModel) HasSelection() bool {
	return m.selectedOffer != nil
}
