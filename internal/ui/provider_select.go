// Package ui provides the TUI (Terminal User Interface) for continueplz.
package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tmeurs/continueplz/internal/provider"
)

// ProviderSelectModel is the Bubbletea model for the provider selection view.
// It displays a table of available GPU offers across all configured providers.
type ProviderSelectModel struct {
	// Offers is the list of available GPU offers
	offers []provider.Offer

	// cursor is the index of the currently highlighted row
	cursor int

	// selected is the index of the selected offer (-1 if none)
	selected int

	// loading indicates if offers are being fetched
	loading bool

	// err holds any error during offer fetching
	err error

	// width is the terminal width
	width int

	// height is the terminal height
	height int

	// ready indicates if the component has been initialized
	ready bool

	// focused indicates if this component has focus
	focused bool
}

// NewProviderSelectModel creates a new provider selection model
func NewProviderSelectModel() ProviderSelectModel {
	return ProviderSelectModel{
		offers:   []provider.Offer{},
		cursor:   0,
		selected: -1,
		loading:  true,
		focused:  true,
	}
}

// NewProviderSelectModelWithOffers creates a new model with pre-loaded offers
func NewProviderSelectModelWithOffers(offers []provider.Offer) ProviderSelectModel {
	m := NewProviderSelectModel()
	m.offers = offers
	m.loading = false
	m.sortOffersByPrice()
	return m
}

// Init implements tea.Model
func (m ProviderSelectModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m ProviderSelectModel) Update(msg tea.Msg) (ProviderSelectModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case OffersLoadedMsg:
		m.offers = msg.Offers
		m.loading = false
		m.err = nil
		// Sort offers by effective price (spot if available, else on-demand)
		m.sortOffersByPrice()
		return m, nil

	case OffersLoadErrorMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil
	}

	return m, nil
}

// handleKeyPress processes keyboard input
func (m ProviderSelectModel) handleKeyPress(msg tea.KeyMsg) (ProviderSelectModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.offers)-1 {
			m.cursor++
		}

	case "home":
		m.cursor = 0

	case "end":
		if len(m.offers) > 0 {
			m.cursor = len(m.offers) - 1
		}

	case "pgup":
		m.cursor -= 5
		if m.cursor < 0 {
			m.cursor = 0
		}

	case "pgdown":
		m.cursor += 5
		if m.cursor >= len(m.offers) && len(m.offers) > 0 {
			m.cursor = len(m.offers) - 1
		}

	case "enter", " ":
		if len(m.offers) > 0 {
			m.selected = m.cursor
			return m, func() tea.Msg {
				return OfferSelectedMsg{Offer: m.offers[m.cursor]}
			}
		}

	case "r":
		// Refresh prices - return command to trigger refresh
		m.loading = true
		return m, func() tea.Msg {
			return RefreshOffersMsg{}
		}
	}

	return m, nil
}

// View implements tea.Model
func (m ProviderSelectModel) View() string {
	if !m.ready {
		return ""
	}

	return m.renderTable()
}

// renderTable renders the provider selection table
func (m ProviderSelectModel) renderTable() string {
	var b strings.Builder

	// Table title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Available Configurations"))
	b.WriteString("\n\n")

	// Handle loading state
	if m.loading {
		spinnerStyle := Styles.Spinner
		b.WriteString(spinnerStyle.Render("  Loading providers..."))
		b.WriteString("\n")
		return Styles.Box.Width(m.width - 4).Render(b.String())
	}

	// Handle error state
	if m.err != nil {
		errorStyle := Styles.Error
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		b.WriteString("\n")
		b.WriteString(Styles.Muted.Render("  Press [r] to retry"))
		b.WriteString("\n")
		return Styles.Box.Width(m.width - 4).Render(b.String())
	}

	// Handle empty state
	if len(m.offers) == 0 {
		b.WriteString(Styles.Muted.Render("  No offers available"))
		b.WriteString("\n")
		b.WriteString(Styles.Muted.Render("  Press [r] to refresh"))
		b.WriteString("\n")
		return Styles.Box.Width(m.width - 4).Render(b.String())
	}

	// Column widths
	colProvider := 12
	colGPU := 14
	colRegion := 12
	colSpot := 10
	colOnDemand := 12
	colDayEst := 10

	// Header
	headerStyle := Styles.TableHeader
	header := fmt.Sprintf("  %-*s %-*s %-*s %*s %*s %*s",
		colProvider, "Provider",
		colGPU, "GPU",
		colRegion, "Region",
		colSpot, "Spot/hr",
		colOnDemand, "OnDemand/hr",
		colDayEst, "Day Est.")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// Separator
	sepLen := colProvider + colGPU + colRegion + colSpot + colOnDemand + colDayEst + 12
	b.WriteString(Styles.Muted.Render("  " + strings.Repeat(TableHorizontal, sepLen)))
	b.WriteString("\n")

	// Rows - calculate visible range for scrolling
	visibleRows := m.height - 12 // Leave room for header/footer
	if visibleRows < 5 {
		visibleRows = 5
	}
	if visibleRows > len(m.offers) {
		visibleRows = len(m.offers)
	}

	// Calculate start and end based on cursor position
	start := 0
	if m.cursor >= visibleRows {
		start = m.cursor - visibleRows + 1
	}
	end := start + visibleRows
	if end > len(m.offers) {
		end = len(m.offers)
		start = end - visibleRows
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		offer := m.offers[i]

		// Format prices
		spotStr := formatSpotPrice(offer.SpotPrice)
		onDemandStr := formatOnDemandPrice(offer.OnDemandPrice)
		dayEstStr := formatDayEstimate(offer)

		// Build row
		row := fmt.Sprintf("  %-*s %-*s %-*s %*s %*s %*s",
			colProvider, offer.Provider,
			colGPU, offer.GPU,
			colRegion, offer.Region,
			colSpot, spotStr,
			colOnDemand, onDemandStr,
			colDayEst, dayEstStr)

		// Apply styling based on selection state
		if i == m.cursor {
			// Current cursor position - highlighted
			cursorStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorForeground).
				Background(ColorSelected)
			indicator := IconSelected
			if i == m.selected {
				indicator = IconCheckmark
			}
			b.WriteString(cursorStyle.Render(indicator + row[1:]))
		} else if i == m.selected {
			// Selected row
			selectedStyle := Styles.Selected
			b.WriteString(selectedStyle.Render("*" + row[1:]))
		} else {
			// Normal row - alternate colors
			if i%2 == 0 {
				b.WriteString(Styles.TableRow.Render(row))
			} else {
				b.WriteString(Styles.TableRowAlt.Render(row))
			}
		}
		b.WriteString("\n")
	}

	// Show scroll indicator if there are more items
	if len(m.offers) > visibleRows {
		scrollInfo := fmt.Sprintf("  Showing %d-%d of %d", start+1, end, len(m.offers))
		b.WriteString(Styles.Muted.Render(scrollInfo))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Key hints
	hints := m.renderKeyHints()
	b.WriteString(hints)

	return Styles.Box.Width(m.width - 4).Render(b.String())
}

// renderKeyHints renders the keyboard shortcut hints
func (m ProviderSelectModel) renderKeyHints() string {
	hints := []string{
		FormatKeyHint("↑↓", "Navigate"),
		FormatKeyHint("Enter", "Select"),
		FormatKeyHint("r", "Refresh"),
		FormatKeyHint("q", "Quit"),
	}
	return Styles.Muted.Render(strings.Join(hints, "  "))
}

// sortOffersByPrice sorts offers by effective price (spot if available, then on-demand)
func (m *ProviderSelectModel) sortOffersByPrice() {
	sort.Slice(m.offers, func(i, j int) bool {
		priceI := effectivePrice(m.offers[i])
		priceJ := effectivePrice(m.offers[j])
		return priceI < priceJ
	})
}

// effectivePrice returns the effective hourly price for an offer (spot if available)
func effectivePrice(o provider.Offer) float64 {
	if o.SpotPrice != nil {
		return *o.SpotPrice
	}
	return o.OnDemandPrice
}

// formatSpotPrice formats the spot price for display
func formatSpotPrice(price *float64) string {
	if price == nil {
		return "-"
	}
	return fmt.Sprintf("%s%.2f", CurrencyEUR, *price)
}

// formatOnDemandPrice formats the on-demand price for display
func formatOnDemandPrice(price float64) string {
	return fmt.Sprintf("%s%.2f", CurrencyEUR, price)
}

// formatDayEstimate calculates and formats the estimated daily cost (8 hours)
func formatDayEstimate(o provider.Offer) string {
	price := effectivePrice(o)
	dayEstimate := price * 8 // 8 hours per day estimate
	return fmt.Sprintf("%s%.2f", CurrencyEUR, dayEstimate)
}

// Message types for provider selection

// OffersLoadedMsg is sent when offers have been loaded
type OffersLoadedMsg struct {
	Offers []provider.Offer
}

// OffersLoadErrorMsg is sent when there's an error loading offers
type OffersLoadErrorMsg struct {
	Err error
}

// OfferSelectedMsg is sent when an offer is selected
type OfferSelectedMsg struct {
	Offer provider.Offer
}

// RefreshOffersMsg is sent when offers should be refreshed
type RefreshOffersMsg struct{}

// Getters and setters

// SetOffers sets the offers list
func (m *ProviderSelectModel) SetOffers(offers []provider.Offer) {
	m.offers = offers
	m.loading = false
	m.sortOffersByPrice()
}

// GetOffers returns the current offers
func (m ProviderSelectModel) GetOffers() []provider.Offer {
	return m.offers
}

// SetLoading sets the loading state
func (m *ProviderSelectModel) SetLoading(loading bool) {
	m.loading = loading
}

// IsLoading returns true if offers are being loaded
func (m ProviderSelectModel) IsLoading() bool {
	return m.loading
}

// SetError sets an error
func (m *ProviderSelectModel) SetError(err error) {
	m.err = err
	m.loading = false
}

// GetError returns the current error
func (m ProviderSelectModel) GetError() error {
	return m.err
}

// GetCursor returns the current cursor position
func (m ProviderSelectModel) GetCursor() int {
	return m.cursor
}

// SetCursor sets the cursor position
func (m *ProviderSelectModel) SetCursor(cursor int) {
	if cursor >= 0 && cursor < len(m.offers) {
		m.cursor = cursor
	}
}

// GetSelected returns the selected offer index (-1 if none)
func (m ProviderSelectModel) GetSelected() int {
	return m.selected
}

// GetSelectedOffer returns the currently selected offer, or nil if none
func (m ProviderSelectModel) GetSelectedOffer() *provider.Offer {
	if m.selected >= 0 && m.selected < len(m.offers) {
		return &m.offers[m.selected]
	}
	return nil
}

// GetCurrentOffer returns the offer at the cursor position, or nil if empty
func (m ProviderSelectModel) GetCurrentOffer() *provider.Offer {
	if len(m.offers) > 0 && m.cursor >= 0 && m.cursor < len(m.offers) {
		return &m.offers[m.cursor]
	}
	return nil
}

// SetFocused sets whether this component has focus
func (m *ProviderSelectModel) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether this component has focus
func (m ProviderSelectModel) IsFocused() bool {
	return m.focused
}

// SetDimensions sets the terminal dimensions
func (m *ProviderSelectModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

// Width returns the terminal width
func (m ProviderSelectModel) Width() int {
	return m.width
}

// Height returns the terminal height
func (m ProviderSelectModel) Height() int {
	return m.height
}

// IsReady returns whether the component has been initialized
func (m ProviderSelectModel) IsReady() bool {
	return m.ready
}
