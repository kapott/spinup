// Package ui provides TUI styling using lipgloss.
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Color palette for the TUI
// Using a professional color scheme that works well in most terminals
var (
	// Primary colors
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorAccent    = lipgloss.Color("#F59E0B") // Amber

	// Status colors
	ColorSuccess = lipgloss.Color("#10B981") // Green
	ColorWarning = lipgloss.Color("#F59E0B") // Amber
	ColorError   = lipgloss.Color("#EF4444") // Red
	ColorInfo    = lipgloss.Color("#3B82F6") // Blue

	// Neutral colors
	ColorMuted      = lipgloss.Color("#6B7280") // Gray
	ColorBorder     = lipgloss.Color("#374151") // Dark gray
	ColorBackground = lipgloss.Color("#1F2937") // Very dark gray
	ColorForeground = lipgloss.Color("#F9FAFB") // Almost white

	// Highlight colors
	ColorHighlight = lipgloss.Color("#7C3AED") // Purple (same as primary)
	ColorSelected  = lipgloss.Color("#4F46E5") // Indigo
)

// StyleSet contains all the styles used in the TUI
type StyleSet struct {
	// Text styles
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Heading     lipgloss.Style
	Body        lipgloss.Style
	Muted       lipgloss.Style
	Bold        lipgloss.Style
	Italic      lipgloss.Style

	// Status styles
	Success     lipgloss.Style
	Warning     lipgloss.Style
	Error       lipgloss.Style
	Info        lipgloss.Style

	// Container styles
	Header      lipgloss.Style
	Footer      lipgloss.Style
	Box         lipgloss.Style
	AlertBox    lipgloss.Style
	WarningBox  lipgloss.Style
	SuccessBox  lipgloss.Style

	// Table styles
	TableHeader lipgloss.Style
	TableRow    lipgloss.Style
	TableRowAlt lipgloss.Style
	TableCell   lipgloss.Style

	// Selection styles
	Selected    lipgloss.Style
	Highlighted lipgloss.Style
	Cursor      lipgloss.Style

	// Progress styles
	ProgressBar     lipgloss.Style
	ProgressFilled  lipgloss.Style
	ProgressEmpty   lipgloss.Style
	Spinner         lipgloss.Style
	Checkmark       lipgloss.Style
	CrossMark       lipgloss.Style

	// Special styles
	KeyHint     lipgloss.Style
	Price       lipgloss.Style
	PriceSpot   lipgloss.Style
	Provider    lipgloss.Style
	GPU         lipgloss.Style
	Model       lipgloss.Style
	Endpoint    lipgloss.Style

	// Status indicator styles
	StatusRunning    lipgloss.Style
	StatusStopped    lipgloss.Style
	StatusWarning    lipgloss.Style
	StatusConnected  lipgloss.Style
}

// Styles is the global style set for the TUI
var Styles = NewStyleSet()

// NewStyleSet creates a new style set with default styles
func NewStyleSet() StyleSet {
	return StyleSet{
		// Text styles
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Align(lipgloss.Center).
			MarginTop(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(ColorMuted).
			Align(lipgloss.Center),

		Heading: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorForeground),

		Body: lipgloss.NewStyle().
			Foreground(ColorForeground),

		Muted: lipgloss.NewStyle().
			Foreground(ColorMuted),

		Bold: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorForeground),

		Italic: lipgloss.NewStyle().
			Italic(true).
			Foreground(ColorForeground),

		// Status styles
		Success: lipgloss.NewStyle().
			Foreground(ColorSuccess),

		Warning: lipgloss.NewStyle().
			Foreground(ColorWarning),

		Error: lipgloss.NewStyle().
			Foreground(ColorError),

		Info: lipgloss.NewStyle().
			Foreground(ColorInfo),

		// Container styles
		Header: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1).
			MarginBottom(1).
			Align(lipgloss.Center),

		Footer: lipgloss.NewStyle().
			Padding(0, 1).
			MarginTop(1).
			Align(lipgloss.Center),

		Box: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1),

		AlertBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorError).
			Padding(0, 1),

		WarningBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorWarning).
			Padding(0, 1),

		SuccessBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSuccess).
			Padding(0, 1),

		// Table styles
		TableHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder),

		TableRow: lipgloss.NewStyle().
			Foreground(ColorForeground),

		TableRowAlt: lipgloss.NewStyle().
			Foreground(ColorForeground).
			Background(ColorBackground),

		TableCell: lipgloss.NewStyle().
			Padding(0, 1),

		// Selection styles
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorForeground).
			Background(ColorSelected),

		Highlighted: lipgloss.NewStyle().
			Foreground(ColorHighlight),

		Cursor: lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true),

		// Progress styles
		ProgressBar: lipgloss.NewStyle(),

		ProgressFilled: lipgloss.NewStyle().
			Foreground(ColorPrimary),

		ProgressEmpty: lipgloss.NewStyle().
			Foreground(ColorMuted),

		Spinner: lipgloss.NewStyle().
			Foreground(ColorSecondary),

		Checkmark: lipgloss.NewStyle().
			Foreground(ColorSuccess),

		CrossMark: lipgloss.NewStyle().
			Foreground(ColorError),

		// Special styles
		KeyHint: lipgloss.NewStyle().
			Foreground(ColorMuted).
			Background(ColorBackground).
			Padding(0, 1),

		Price: lipgloss.NewStyle().
			Foreground(ColorForeground),

		PriceSpot: lipgloss.NewStyle().
			Foreground(ColorSuccess),

		Provider: lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true),

		GPU: lipgloss.NewStyle().
			Foreground(ColorAccent),

		Model: lipgloss.NewStyle().
			Foreground(ColorPrimary),

		Endpoint: lipgloss.NewStyle().
			Foreground(ColorInfo),

		// Status indicator styles
		StatusRunning: lipgloss.NewStyle().
			Foreground(ColorSuccess),

		StatusStopped: lipgloss.NewStyle().
			Foreground(ColorMuted),

		StatusWarning: lipgloss.NewStyle().
			Foreground(ColorWarning),

		StatusConnected: lipgloss.NewStyle().
			Foreground(ColorSuccess),
	}
}

// Icons and symbols used in the TUI
const (
	// Status indicators
	IconRunning     = "●"
	IconStopped     = "○"
	IconWarning     = "⚠"
	IconError       = "✗"
	IconSuccess     = "✓"
	IconInfo        = "ℹ"
	IconSpinner     = "⠋"
	IconSelected    = "▶"
	IconUnselected  = " "

	// Progress indicators
	IconCheckmark   = "✓"
	IconCrossMark   = "✗"
	IconPending     = "○"
	IconInProgress  = "◐"

	// Box drawing characters
	BoxTopLeft      = "╭"
	BoxTopRight     = "╮"
	BoxBottomLeft   = "╰"
	BoxBottomRight  = "╯"
	BoxHorizontal   = "─"
	BoxVertical     = "│"

	// Table characters
	TableHorizontal = "─"
	TableVertical   = "│"
	TableCross      = "┼"
	TableTopCross   = "┬"
	TableBottomCross = "┴"
	TableLeftCross  = "├"
	TableRightCross = "┤"

	// Stars for quality rating
	StarFilled  = "★"
	StarEmpty   = "☆"

	// Currency
	CurrencyEUR = "€"
	CurrencyUSD = "$"
)

// QualityStars returns a star rating string for the given quality level (0-5)
func QualityStars(quality int) string {
	if quality < 0 {
		quality = 0
	}
	if quality > 5 {
		quality = 5
	}

	filled := ""
	empty := ""
	for i := 0; i < quality; i++ {
		filled += StarFilled
	}
	for i := quality; i < 5; i++ {
		empty += StarEmpty
	}

	return Styles.Success.Render(filled) + Styles.Muted.Render(empty)
}

// StatusIcon returns the appropriate icon and style for a status
func StatusIcon(running bool) string {
	if running {
		return Styles.StatusRunning.Render(IconRunning)
	}
	return Styles.StatusStopped.Render(IconStopped)
}

// FormatPrice formats a price in EUR with the currency symbol
func FormatPrice(price float64) string {
	return Styles.Price.Render(fmt.Sprintf("%s%.2f", CurrencyEUR, price))
}

// FormatSpotPrice formats a spot price with special highlighting
func FormatSpotPrice(price float64) string {
	return Styles.PriceSpot.Render(fmt.Sprintf("%s%.2f", CurrencyEUR, price))
}

// FormatKeyHint formats a keyboard shortcut hint
func FormatKeyHint(key, description string) string {
	return Styles.KeyHint.Render(fmt.Sprintf("[%s] %s", key, description))
}

