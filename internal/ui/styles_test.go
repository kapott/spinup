package ui

import (
	"strings"
	"testing"
)

func TestNewStyleSet(t *testing.T) {
	ss := NewStyleSet()

	// Verify basic styles are not nil (lipgloss styles are values, not pointers)
	// Just ensure they can render without panic
	_ = ss.Title.Render("Test")
	_ = ss.Subtitle.Render("Test")
	_ = ss.Heading.Render("Test")
	_ = ss.Body.Render("Test")
	_ = ss.Muted.Render("Test")
	_ = ss.Bold.Render("Test")
	_ = ss.Italic.Render("Test")
	_ = ss.Success.Render("Test")
	_ = ss.Warning.Render("Test")
	_ = ss.Error.Render("Test")
	_ = ss.Info.Render("Test")
	_ = ss.Header.Render("Test")
	_ = ss.Footer.Render("Test")
	_ = ss.Box.Render("Test")
	_ = ss.AlertBox.Render("Test")
	_ = ss.WarningBox.Render("Test")
	_ = ss.SuccessBox.Render("Test")
}

func TestQualityStars(t *testing.T) {
	tests := []struct {
		quality int
		filled  int
		empty   int
	}{
		{0, 0, 5},
		{1, 1, 4},
		{2, 2, 3},
		{3, 3, 2},
		{4, 4, 1},
		{5, 5, 0},
		{-1, 0, 5},  // Clamped to 0
		{10, 5, 0},  // Clamped to 5
	}

	for _, tc := range tests {
		result := QualityStars(tc.quality)
		// The result contains ANSI codes, so we check for star characters
		filledCount := strings.Count(result, StarFilled)
		emptyCount := strings.Count(result, StarEmpty)

		if filledCount != tc.filled {
			t.Errorf("QualityStars(%d): expected %d filled stars, got %d", tc.quality, tc.filled, filledCount)
		}
		if emptyCount != tc.empty {
			t.Errorf("QualityStars(%d): expected %d empty stars, got %d", tc.quality, tc.empty, emptyCount)
		}
	}
}

func TestStatusIcon(t *testing.T) {
	running := StatusIcon(true)
	if !strings.Contains(running, IconRunning) {
		t.Error("StatusIcon(true) should contain running icon")
	}

	stopped := StatusIcon(false)
	if !strings.Contains(stopped, IconStopped) {
		t.Error("StatusIcon(false) should contain stopped icon")
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		price    float64
		contains string
	}{
		{0.00, "€0.00"},
		{1.50, "€1.50"},
		{10.99, "€10.99"},
		{100.00, "€100.00"},
	}

	for _, tc := range tests {
		result := FormatPrice(tc.price)
		if !strings.Contains(result, tc.contains) {
			t.Errorf("FormatPrice(%v) should contain %q, got %q", tc.price, tc.contains, result)
		}
	}
}

func TestFormatSpotPrice(t *testing.T) {
	tests := []struct {
		price    float64
		contains string
	}{
		{0.65, "€0.65"},
		{1.25, "€1.25"},
	}

	for _, tc := range tests {
		result := FormatSpotPrice(tc.price)
		if !strings.Contains(result, tc.contains) {
			t.Errorf("FormatSpotPrice(%v) should contain %q, got %q", tc.price, tc.contains, result)
		}
	}
}

func TestFormatKeyHint(t *testing.T) {
	result := FormatKeyHint("q", "Quit")
	if !strings.Contains(result, "[q]") {
		t.Error("FormatKeyHint should contain [q]")
	}
	if !strings.Contains(result, "Quit") {
		t.Error("FormatKeyHint should contain Quit")
	}
}

func TestIconConstants(t *testing.T) {
	// Verify icon constants are not empty
	icons := []struct {
		name  string
		value string
	}{
		{"IconRunning", IconRunning},
		{"IconStopped", IconStopped},
		{"IconWarning", IconWarning},
		{"IconError", IconError},
		{"IconSuccess", IconSuccess},
		{"IconInfo", IconInfo},
		{"IconCheckmark", IconCheckmark},
		{"IconCrossMark", IconCrossMark},
		{"IconPending", IconPending},
		{"IconInProgress", IconInProgress},
		{"StarFilled", StarFilled},
		{"StarEmpty", StarEmpty},
		{"CurrencyEUR", CurrencyEUR},
		{"CurrencyUSD", CurrencyUSD},
	}

	for _, ic := range icons {
		if ic.value == "" {
			t.Errorf("%s should not be empty", ic.name)
		}
	}
}

func TestBoxDrawingCharacters(t *testing.T) {
	// Verify box drawing constants are not empty
	chars := []struct {
		name  string
		value string
	}{
		{"BoxTopLeft", BoxTopLeft},
		{"BoxTopRight", BoxTopRight},
		{"BoxBottomLeft", BoxBottomLeft},
		{"BoxBottomRight", BoxBottomRight},
		{"BoxHorizontal", BoxHorizontal},
		{"BoxVertical", BoxVertical},
		{"TableHorizontal", TableHorizontal},
		{"TableVertical", TableVertical},
	}

	for _, c := range chars {
		if c.value == "" {
			t.Errorf("%s should not be empty", c.name)
		}
	}
}

func TestGlobalStyles(t *testing.T) {
	// Verify the global Styles variable is properly initialized
	// by rendering with each style
	_ = Styles.Title.Render("test")
	_ = Styles.Box.Render("test")
	_ = Styles.AlertBox.Render("test")
}

func TestColorConstants(t *testing.T) {
	// Verify color constants are set (not empty)
	colors := []struct {
		name  string
		value interface{}
	}{
		{"ColorPrimary", ColorPrimary},
		{"ColorSecondary", ColorSecondary},
		{"ColorAccent", ColorAccent},
		{"ColorSuccess", ColorSuccess},
		{"ColorWarning", ColorWarning},
		{"ColorError", ColorError},
		{"ColorInfo", ColorInfo},
		{"ColorMuted", ColorMuted},
		{"ColorBorder", ColorBorder},
	}

	for _, c := range colors {
		if c.value == nil {
			t.Errorf("%s should not be nil", c.name)
		}
	}
}
