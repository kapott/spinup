// Package ui provides TUI components for spinup.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AlertLevel represents the severity level of an alert.
type AlertLevel int

const (
	// AlertLevelInfo is for informational messages.
	AlertLevelInfo AlertLevel = iota
	// AlertLevelWarning is for warning messages.
	AlertLevelWarning
	// AlertLevelError is for error messages.
	AlertLevelError
	// AlertLevelCritical is for critical errors requiring immediate action.
	AlertLevelCritical
)

// String returns the string representation of an alert level.
func (l AlertLevel) String() string {
	switch l {
	case AlertLevelInfo:
		return "INFO"
	case AlertLevelWarning:
		return "WARNING"
	case AlertLevelError:
		return "ERROR"
	case AlertLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// Icon returns the icon for an alert level.
func (l AlertLevel) Icon() string {
	switch l {
	case AlertLevelInfo:
		return IconInfo
	case AlertLevelWarning:
		return IconWarning
	case AlertLevelError:
		return IconError
	case AlertLevelCritical:
		return "🔴"
	default:
		return ""
	}
}

// Alert represents an alert to display in the UI.
type Alert struct {
	Level       AlertLevel
	Title       string
	Message     string
	Details     []string
	Actions     []string
	InstanceID  string
	Provider    string
	ConsoleURL  string
	LogFile     string
	CreatedAt   time.Time
	Dismissible bool
}

// NewInfoAlert creates a new info-level alert.
func NewInfoAlert(title, message string) Alert {
	return Alert{
		Level:       AlertLevelInfo,
		Title:       title,
		Message:     message,
		CreatedAt:   time.Now(),
		Dismissible: true,
	}
}

// NewWarningAlert creates a new warning-level alert.
func NewWarningAlert(title, message string) Alert {
	return Alert{
		Level:       AlertLevelWarning,
		Title:       title,
		Message:     message,
		CreatedAt:   time.Now(),
		Dismissible: true,
	}
}

// NewErrorAlert creates a new error-level alert.
func NewErrorAlert(title, message string) Alert {
	return Alert{
		Level:       AlertLevelError,
		Title:       title,
		Message:     message,
		CreatedAt:   time.Now(),
		Dismissible: true,
	}
}

// NewCriticalAlert creates a new critical-level alert.
// Critical alerts require immediate action and are not dismissible.
func NewCriticalAlert(title, message string) Alert {
	return Alert{
		Level:       AlertLevelCritical,
		Title:       title,
		Message:     message,
		CreatedAt:   time.Now(),
		Dismissible: false,
	}
}

// NewBillingNotVerifiedAlert creates a critical alert for billing verification failure.
// This matches PRD Section 8.3 format exactly.
func NewBillingNotVerifiedAlert(instanceID, provider, consoleURL, logFile string) Alert {
	return Alert{
		Level:      AlertLevelCritical,
		Title:      "CRITICAL ERROR",
		Message:    fmt.Sprintf("Could not verify that billing has stopped for instance %s.", instanceID),
		InstanceID: instanceID,
		Provider:   provider,
		ConsoleURL: consoleURL,
		LogFile:    logFile,
		Actions: []string{
			fmt.Sprintf("Log into %s console", provider),
			"Navigate to instances/machines",
			fmt.Sprintf("Verify instance %s is terminated", instanceID),
			"If still running, terminate manually",
		},
		CreatedAt:   time.Now(),
		Dismissible: false,
	}
}

// WithDetails adds detail lines to the alert.
func (a Alert) WithDetails(details ...string) Alert {
	a.Details = append(a.Details, details...)
	return a
}

// WithActions adds action instructions to the alert.
func (a Alert) WithActions(actions ...string) Alert {
	a.Actions = append(a.Actions, actions...)
	return a
}

// AlertModel is the Bubbletea model for alert display.
type AlertModel struct {
	// alert is the current alert to display.
	alert *Alert

	// width and height are terminal dimensions.
	width  int
	height int

	// flashState is used for border flashing animation (0 or 1).
	flashState int

	// dismissed indicates if the alert has been dismissed.
	dismissed bool

	// waitingForAction indicates if we're waiting for user action.
	waitingForAction bool

	// spinner for animation effect.
	spinner spinner.Model
}

// NewAlertModel creates a new alert model.
func NewAlertModel() AlertModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = Styles.Spinner
	return AlertModel{
		spinner: s,
	}
}

// NewAlertModelWithAlert creates a new alert model with an initial alert.
func NewAlertModelWithAlert(alert Alert) AlertModel {
	m := NewAlertModel()
	m.alert = &alert
	m.waitingForAction = !alert.Dismissible
	return m
}

// SetAlert sets the alert to display.
func (m *AlertModel) SetAlert(alert *Alert) {
	m.alert = alert
	m.dismissed = false
	if alert != nil {
		m.waitingForAction = !alert.Dismissible
	}
}

// Alert returns the current alert.
func (m AlertModel) Alert() *Alert {
	return m.alert
}

// IsDismissed returns true if the alert has been dismissed.
func (m AlertModel) IsDismissed() bool {
	return m.dismissed
}

// Init implements tea.Model.
func (m AlertModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickFlash(),
	)
}

// alertFlashMsg is sent periodically to update the flash animation.
type alertFlashMsg struct{}

// tickFlash returns a command that sends flash messages periodically.
func tickFlash() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return alertFlashMsg{}
	})
}

// AlertShowMsg is sent to display an alert.
type AlertShowMsg struct {
	Alert Alert
}

// AlertDismissMsg is sent when an alert is dismissed.
type AlertDismissMsg struct{}

// Update implements tea.Model.
func (m AlertModel) Update(msg tea.Msg) (AlertModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case alertFlashMsg:
		// Toggle flash state for critical alerts
		if m.alert != nil && m.alert.Level == AlertLevelCritical {
			m.flashState = 1 - m.flashState
		}
		return m, tickFlash()

	case AlertShowMsg:
		m.alert = &msg.Alert
		m.dismissed = false
		m.waitingForAction = !msg.Alert.Dismissible
		return m, nil

	case AlertDismissMsg:
		if m.alert != nil && m.alert.Dismissible {
			m.dismissed = true
			m.alert = nil
		}
		return m, nil
	}

	return m, nil
}

// handleKeyPress handles keyboard input.
func (m AlertModel) handleKeyPress(msg tea.KeyMsg) (AlertModel, tea.Cmd) {
	if m.alert == nil {
		return m, nil
	}

	switch msg.String() {
	case "enter", " ":
		// Dismiss if dismissible
		if m.alert.Dismissible {
			m.dismissed = true
			return m, func() tea.Msg { return AlertDismissMsg{} }
		}
	case "q", "ctrl+c":
		// Always allow quit
		return m, tea.Quit
	}

	return m, nil
}

// View implements tea.Model.
func (m AlertModel) View() string {
	if m.alert == nil || m.dismissed {
		return ""
	}

	switch m.alert.Level {
	case AlertLevelCritical:
		return m.renderCriticalAlert()
	case AlertLevelError:
		return m.renderErrorAlert()
	case AlertLevelWarning:
		return m.renderWarningAlert()
	case AlertLevelInfo:
		return m.renderInfoAlert()
	default:
		return m.renderInfoAlert()
	}
}

// renderCriticalAlert renders a critical alert with flashing red border.
// Matches PRD Section 8.3 format exactly.
func (m AlertModel) renderCriticalAlert() string {
	if m.alert == nil {
		return ""
	}

	// Use ANSI escape codes for flashing effect
	// \033[5m enables blink mode, \033[0m resets
	// However, not all terminals support blink, so we also alternate border color
	var borderColor lipgloss.Color
	if m.flashState == 0 {
		borderColor = ColorError // Bright red
	} else {
		borderColor = lipgloss.Color("#FF0000") // Pure red
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(borderColor).
		Width(77).
		Padding(0, 1)

	var b strings.Builder

	// Title line with red circle emoji
	title := fmt.Sprintf("  %s %s %s", m.alert.Level.Icon(), m.alert.Title, m.alert.Level.Icon())
	b.WriteString(Styles.Error.Bold(true).Render(title))
	b.WriteString("\n")
	b.WriteString("\n")

	// Main message
	b.WriteString("  " + m.alert.Message)
	b.WriteString("\n")

	// Action instructions
	if len(m.alert.Actions) > 0 {
		b.WriteString("\n")
		b.WriteString(Styles.Error.Bold(true).Render("  IMMEDIATE ACTION REQUIRED:"))
		b.WriteString("\n")
		for i, action := range m.alert.Actions {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, action))
		}
	}

	// Console URL
	if m.alert.ConsoleURL != "" {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Console: %s", Styles.Endpoint.Render(m.alert.ConsoleURL)))
		b.WriteString("\n")
	}

	// Log file
	if m.alert.LogFile != "" {
		b.WriteString(fmt.Sprintf("  Error details logged to: %s", m.alert.LogFile))
		b.WriteString("\n")
	}

	return boxStyle.Render(b.String())
}

// renderErrorAlert renders an error alert.
func (m AlertModel) renderErrorAlert() string {
	if m.alert == nil {
		return ""
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorError).
		Width(75).
		Padding(0, 1)

	var b strings.Builder

	// Title
	title := fmt.Sprintf("  %s ERROR: %s", IconError, m.alert.Title)
	b.WriteString(Styles.Error.Bold(true).Render(title))
	b.WriteString("\n\n")

	// Message
	b.WriteString("  " + m.alert.Message)
	b.WriteString("\n")

	// Details
	for _, detail := range m.alert.Details {
		b.WriteString("  " + Styles.Muted.Render(detail))
		b.WriteString("\n")
	}

	// Dismiss hint
	if m.alert.Dismissible {
		b.WriteString("\n")
		b.WriteString(Styles.Muted.Render("  Press Enter or Space to dismiss"))
	}

	return boxStyle.Render(b.String())
}

// renderWarningAlert renders a warning alert.
func (m AlertModel) renderWarningAlert() string {
	if m.alert == nil {
		return ""
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarning).
		Width(75).
		Padding(0, 1)

	var b strings.Builder

	// Title
	title := fmt.Sprintf("  %s WARNING: %s", IconWarning, m.alert.Title)
	b.WriteString(Styles.Warning.Bold(true).Render(title))
	b.WriteString("\n\n")

	// Message
	b.WriteString("  " + m.alert.Message)
	b.WriteString("\n")

	// Details
	for _, detail := range m.alert.Details {
		b.WriteString("  " + Styles.Muted.Render(detail))
		b.WriteString("\n")
	}

	// Actions if any
	if len(m.alert.Actions) > 0 {
		b.WriteString("\n")
		b.WriteString(Styles.Warning.Render("  Recommended actions:"))
		b.WriteString("\n")
		for i, action := range m.alert.Actions {
			b.WriteString(fmt.Sprintf("    %d. %s\n", i+1, action))
		}
	}

	// Dismiss hint
	if m.alert.Dismissible {
		b.WriteString("\n")
		b.WriteString(Styles.Muted.Render("  Press Enter or Space to dismiss"))
	}

	return boxStyle.Render(b.String())
}

// renderInfoAlert renders an info alert.
func (m AlertModel) renderInfoAlert() string {
	if m.alert == nil {
		return ""
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorInfo).
		Width(75).
		Padding(0, 1)

	var b strings.Builder

	// Title
	title := fmt.Sprintf("  %s %s", IconInfo, m.alert.Title)
	b.WriteString(Styles.Info.Bold(true).Render(title))
	b.WriteString("\n\n")

	// Message
	b.WriteString("  " + m.alert.Message)
	b.WriteString("\n")

	// Details
	for _, detail := range m.alert.Details {
		b.WriteString("  " + Styles.Muted.Render(detail))
		b.WriteString("\n")
	}

	// Dismiss hint
	if m.alert.Dismissible {
		b.WriteString("\n")
		b.WriteString(Styles.Muted.Render("  Press Enter or Space to dismiss"))
	}

	return boxStyle.Render(b.String())
}

// SetDimensions sets the terminal dimensions.
func (m *AlertModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
}

// ShowAlert returns a command to display an alert.
func ShowAlert(alert Alert) tea.Cmd {
	return func() tea.Msg {
		return AlertShowMsg{Alert: alert}
	}
}

// DismissAlert returns a command to dismiss the current alert.
func DismissAlert() tea.Cmd {
	return func() tea.Msg {
		return AlertDismissMsg{}
	}
}

// RenderAlertInline renders an alert as a string without tea.Model context.
// This is useful for non-interactive display or logging.
func RenderAlertInline(alert Alert) string {
	m := NewAlertModelWithAlert(alert)
	m.width = 80
	m.height = 24
	return m.View()
}

// SpotInterruptionAlert represents a spot instance interruption alert.
// This alert is shown when a spot instance is reclaimed by the provider.
type SpotInterruptionAlert struct {
	SessionCost float64       // Total session cost in EUR
	Duration    time.Duration // Session duration
	Provider    string        // Provider name
	InstanceID  string        // Instance ID
}

// NewSpotInterruptionAlert creates a new spot interruption alert.
func NewSpotInterruptionAlert(sessionCost float64, duration time.Duration, provider, instanceID string) SpotInterruptionAlert {
	return SpotInterruptionAlert{
		SessionCost: sessionCost,
		Duration:    duration,
		Provider:    provider,
		InstanceID:  instanceID,
	}
}

// SpotInterruptionAction represents the user's choice after a spot interruption.
type SpotInterruptionAction int

const (
	// SpotInterruptionActionNone means no action has been selected yet.
	SpotInterruptionActionNone SpotInterruptionAction = iota
	// SpotInterruptionActionRestart means the user wants to restart with a new instance.
	SpotInterruptionActionRestart
	// SpotInterruptionActionQuit means the user wants to quit.
	SpotInterruptionActionQuit
)

// SpotInterruptionModel is the Bubbletea model for spot interruption alerts.
// This matches PRD Section 5.3 exactly.
type SpotInterruptionModel struct {
	// alert holds the spot interruption details.
	alert *SpotInterruptionAlert

	// action is the user's selected action.
	action SpotInterruptionAction

	// width and height are terminal dimensions.
	width  int
	height int
}

// NewSpotInterruptionModel creates a new spot interruption model.
func NewSpotInterruptionModel() SpotInterruptionModel {
	return SpotInterruptionModel{}
}

// NewSpotInterruptionModelWithAlert creates a new model with an initial alert.
func NewSpotInterruptionModelWithAlert(alert SpotInterruptionAlert) SpotInterruptionModel {
	return SpotInterruptionModel{
		alert: &alert,
	}
}

// SetAlert sets the alert to display.
func (m *SpotInterruptionModel) SetAlert(alert *SpotInterruptionAlert) {
	m.alert = alert
	m.action = SpotInterruptionActionNone
}

// Alert returns the current alert.
func (m SpotInterruptionModel) Alert() *SpotInterruptionAlert {
	return m.alert
}

// Action returns the user's selected action.
func (m SpotInterruptionModel) Action() SpotInterruptionAction {
	return m.action
}

// HasAlert returns true if there is an active alert.
func (m SpotInterruptionModel) HasAlert() bool {
	return m.alert != nil
}

// Init implements tea.Model.
func (m SpotInterruptionModel) Init() tea.Cmd {
	return nil
}

// SpotInterruptionActionMsg is sent when the user selects an action.
type SpotInterruptionActionMsg struct {
	Action SpotInterruptionAction
}

// Update implements tea.Model.
func (m SpotInterruptionModel) Update(msg tea.Msg) (SpotInterruptionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

// handleKeyPress handles keyboard input for spot interruption alerts.
func (m SpotInterruptionModel) handleKeyPress(msg tea.KeyMsg) (SpotInterruptionModel, tea.Cmd) {
	if m.alert == nil {
		return m, nil
	}

	switch msg.String() {
	case "r", "R":
		// User wants to restart with a new instance
		m.action = SpotInterruptionActionRestart
		return m, func() tea.Msg {
			return SpotInterruptionActionMsg{Action: SpotInterruptionActionRestart}
		}

	case "q", "Q", "ctrl+c":
		// User wants to quit
		m.action = SpotInterruptionActionQuit
		return m, func() tea.Msg {
			return SpotInterruptionActionMsg{Action: SpotInterruptionActionQuit}
		}
	}

	return m, nil
}

// View implements tea.Model.
// Renders the spot interruption alert matching PRD Section 5.3 exactly.
func (m SpotInterruptionModel) View() string {
	if m.alert == nil {
		return ""
	}

	// Use rounded border with warning color to indicate non-critical but important message
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarning).
		Width(77).
		Padding(0, 1)

	var b strings.Builder

	// Title line with warning icon
	title := fmt.Sprintf("  %s  SPOT INSTANCE INTERRUPTED", IconWarning)
	b.WriteString(Styles.Warning.Bold(true).Render(title))
	b.WriteString("\n")
	b.WriteString("\n")

	// Explanation message
	b.WriteString("  Your spot instance was reclaimed by the provider.")
	b.WriteString("\n")
	b.WriteString("  This is normal for spot instances.")
	b.WriteString("\n")
	b.WriteString("\n")

	// Session cost
	costStr := fmt.Sprintf("  Session cost: %s%.2f", CurrencyEUR, m.alert.SessionCost)
	b.WriteString(costStr)
	b.WriteString("\n")

	// Duration
	durationStr := fmt.Sprintf("  Duration: %s", formatDuration(m.alert.Duration))
	b.WriteString(durationStr)
	b.WriteString("\n")
	b.WriteString("\n")

	// Action options
	b.WriteString(Styles.KeyHint.Render("  [r]"))
	b.WriteString(" Restart with new instance")
	b.WriteString("\n")
	b.WriteString(Styles.KeyHint.Render("  [q]"))
	b.WriteString(" Quit")
	b.WriteString("\n")

	return boxStyle.Render(b.String())
}

// SetDimensions sets the terminal dimensions for the spot interruption model.
func (m *SpotInterruptionModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
}

// RenderSpotInterruptionInline renders a spot interruption alert as a string.
// This is useful for non-interactive display or logging.
func RenderSpotInterruptionInline(alert SpotInterruptionAlert) string {
	m := NewSpotInterruptionModelWithAlert(alert)
	m.width = 80
	m.height = 24
	return m.View()
}

// RenderCriticalAlertRaw renders a critical alert using raw ANSI codes for maximum compatibility.
// This includes the ANSI blink escape code (\033[5m) for terminals that support it.
func RenderCriticalAlertRaw(alert Alert) string {
	if alert.Level != AlertLevelCritical {
		return RenderAlertInline(alert)
	}

	// ANSI escape codes
	const (
		reset     = "\033[0m"
		bold      = "\033[1m"
		blink     = "\033[5m"
		red       = "\033[31m"
		brightRed = "\033[91m"
	)

	// Box drawing characters (from styles.go)
	const (
		topLeft     = "╔"
		topRight    = "╗"
		bottomLeft  = "╚"
		bottomRight = "╝"
		horizontal  = "═"
		vertical    = "║"
	)

	width := 77

	var b strings.Builder

	// Top border with blink effect
	b.WriteString(blink + brightRed)
	b.WriteString(topLeft)
	for i := 0; i < width; i++ {
		b.WriteString(horizontal)
	}
	b.WriteString(topRight)
	b.WriteString(reset)
	b.WriteString("\n")

	// Helper to write a padded line
	writeLine := func(content string, contentLen int) {
		b.WriteString(blink + brightRed + vertical + reset)
		b.WriteString("  ")
		b.WriteString(content)
		padding := width - 2 - contentLen
		if padding > 0 {
			b.WriteString(strings.Repeat(" ", padding))
		}
		b.WriteString(blink + brightRed + vertical + reset)
		b.WriteString("\n")
	}

	// Empty line
	writeLine("", 0)

	// Title line
	title := fmt.Sprintf("%s%s🔴 %s 🔴%s", bold, brightRed, alert.Title, reset)
	writeLine(title, len(alert.Title)+6)

	// Empty line
	writeLine("", 0)

	// Message
	writeLine(alert.Message, len(alert.Message))

	// Empty line
	writeLine("", 0)

	// Actions
	if len(alert.Actions) > 0 {
		actionHeader := bold + brightRed + "IMMEDIATE ACTION REQUIRED:" + reset
		writeLine(actionHeader, 26)
		for i, action := range alert.Actions {
			line := fmt.Sprintf("%d. %s", i+1, action)
			writeLine(line, len(line))
		}
	}

	// Console URL
	if alert.ConsoleURL != "" {
		// Empty line
		writeLine("", 0)
		line := fmt.Sprintf("Console: %s", alert.ConsoleURL)
		writeLine(line, len(line))
	}

	// Log file
	if alert.LogFile != "" {
		line := fmt.Sprintf("Error details logged to: %s", alert.LogFile)
		writeLine(line, len(line))
	}

	// Empty line
	writeLine("", 0)

	// Bottom border with blink effect
	b.WriteString(blink + brightRed)
	b.WriteString(bottomLeft)
	for i := 0; i < width; i++ {
		b.WriteString(horizontal)
	}
	b.WriteString(bottomRight)
	b.WriteString(reset)
	b.WriteString("\n")

	return b.String()
}
