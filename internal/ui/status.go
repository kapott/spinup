// Package ui provides TUI components for spinup.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tmeurs/spinup/internal/config"
)

// StatusAction represents an action that can be triggered from the status view.
type StatusAction int

const (
	// StatusActionNone means no action.
	StatusActionNone StatusAction = iota
	// StatusActionStop triggers instance stop.
	StatusActionStop
	// StatusActionTest triggers connection test.
	StatusActionTest
	// StatusActionLogs triggers log view.
	StatusActionLogs
	// StatusActionQuit quits the application (instance keeps running).
	StatusActionQuit
)

// String returns the string representation of the action.
func (a StatusAction) String() string {
	switch a {
	case StatusActionStop:
		return "stop"
	case StatusActionTest:
		return "test"
	case StatusActionLogs:
		return "logs"
	case StatusActionQuit:
		return "quit"
	default:
		return "none"
	}
}

// StatusModel is the Bubbletea model for the instance status view.
type StatusModel struct {
	// state holds the current application state.
	state *config.State

	// spinner is used for animated elements.
	spinner spinner.Model

	// width and height are the terminal dimensions.
	width  int
	height int

	// lastUpdate is when the state was last refreshed.
	lastUpdate time.Time

	// action holds the action selected by the user.
	action StatusAction

	// testResult holds the result of the last connection test.
	testResult *TestResult

	// testing indicates if a test is in progress.
	testing bool
}

// TestResult holds the result of a connection test.
type TestResult struct {
	Success  bool
	Latency  time.Duration
	Error    error
	TestedAt time.Time
}

// NewStatusModel creates a new status model.
func NewStatusModel() StatusModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = Styles.Spinner

	return StatusModel{
		spinner:    s,
		lastUpdate: time.Now(),
	}
}

// NewStatusModelWithState creates a new status model with initial state.
func NewStatusModelWithState(state *config.State) StatusModel {
	m := NewStatusModel()
	m.state = state
	return m
}

// Init implements tea.Model.
func (m StatusModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.tickCmd())
}

// tickCmd returns a command that ticks every second for live updates.
func (m StatusModel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return StatusTickMsg{Time: t}
	})
}

// StatusTickMsg is sent every second for live updates.
type StatusTickMsg struct {
	Time time.Time
}

// StatusActionMsg is sent when the user triggers an action.
type StatusActionMsg struct {
	Action StatusAction
}

// StatusStateUpdatedMsg is sent when the state is refreshed.
type StatusStateUpdatedMsg struct {
	State *config.State
}

// StatusTestStartMsg is sent when a connection test starts.
type StatusTestStartMsg struct{}

// StatusTestResultMsg is sent when a connection test completes.
type StatusTestResultMsg struct {
	Result *TestResult
}

// Update implements tea.Model.
func (m StatusModel) Update(msg tea.Msg) (StatusModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case StatusTickMsg:
		m.lastUpdate = msg.Time
		return m, m.tickCmd()

	case StatusStateUpdatedMsg:
		m.state = msg.State
		return m, nil

	case StatusTestStartMsg:
		m.testing = true
		m.testResult = nil
		return m, m.spinner.Tick

	case StatusTestResultMsg:
		m.testing = false
		m.testResult = msg.Result
		return m, nil
	}

	return m, nil
}

// handleKeyPress processes keyboard input.
func (m StatusModel) handleKeyPress(msg tea.KeyMsg) (StatusModel, tea.Cmd) {
	switch msg.String() {
	case "s", "S":
		m.action = StatusActionStop
		return m, func() tea.Msg {
			return StatusActionMsg{Action: StatusActionStop}
		}

	case "t", "T":
		m.action = StatusActionTest
		return m, func() tea.Msg {
			return StatusActionMsg{Action: StatusActionTest}
		}

	case "l", "L":
		m.action = StatusActionLogs
		return m, func() tea.Msg {
			return StatusActionMsg{Action: StatusActionLogs}
		}

	case "q", "Q":
		m.action = StatusActionQuit
		return m, func() tea.Msg {
			return StatusActionMsg{Action: StatusActionQuit}
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m StatusModel) View() string {
	var b strings.Builder

	// Instance Status Box
	b.WriteString(m.renderInstanceBox())
	b.WriteString("\n")

	// Quick Actions Box
	b.WriteString(m.renderActionsBox())

	// Test result (if any)
	if m.testResult != nil {
		b.WriteString("\n")
		b.WriteString(m.renderTestResult())
	}

	return b.String()
}

// renderInstanceBox renders the current instance information box.
func (m StatusModel) renderInstanceBox() string {
	var b strings.Builder

	// Title
	title := "Current Instance"
	titleLine := fmt.Sprintf("┌─ %s ", title)
	padding := 80 - len(titleLine) - 1
	if padding > 0 {
		titleLine += strings.Repeat("─", padding)
	}
	titleLine += "┐"
	b.WriteString(Styles.Heading.Render(titleLine))
	b.WriteString("\n")

	if m.state == nil || m.state.Instance == nil {
		// No active instance
		b.WriteString("│" + strings.Repeat(" ", 78) + "│\n")
		b.WriteString(fmt.Sprintf("│  %-76s│\n", Styles.Muted.Render("No active instance")))
		b.WriteString("│" + strings.Repeat(" ", 78) + "│\n")
	} else {
		inst := m.state.Instance
		model := m.state.Model
		wg := m.state.WireGuard
		cost := m.state.Cost
		deadman := m.state.Deadman

		b.WriteString("│" + strings.Repeat(" ", 78) + "│\n")

		// Provider
		b.WriteString(m.renderLine("Provider:", Styles.Provider.Render(inst.Provider)))

		// GPU
		b.WriteString(m.renderLine("GPU:", Styles.GPU.Render(inst.GPU)))

		// Region
		b.WriteString(m.renderLine("Region:", inst.Region))

		// Instance ID
		b.WriteString(m.renderLine("Instance ID:", inst.ID))

		// Status
		statusIcon := Styles.StatusRunning.Render(IconRunning)
		statusText := "Running"
		b.WriteString(m.renderLine("Status:", statusIcon+" "+Styles.Success.Render(statusText)))

		b.WriteString("│" + strings.Repeat(" ", 78) + "│\n")

		// Model info
		if model != nil {
			b.WriteString(m.renderLine("Model:", Styles.Model.Render(model.Name)))

			// Model status
			modelStatusIcon := statusIcon
			modelStatusText := "Loaded and ready"
			if model.Status == "loading" {
				modelStatusIcon = m.spinner.View()
				modelStatusText = "Loading..."
			} else if model.Status == "error" {
				modelStatusIcon = Styles.CrossMark.Render(IconError)
				modelStatusText = "Error"
			}
			b.WriteString(m.renderLine("Model Status:", modelStatusIcon+" "+modelStatusText))
		}

		b.WriteString("│" + strings.Repeat(" ", 78) + "│\n")

		// Started time
		if !inst.CreatedAt.IsZero() {
			startedStr := inst.CreatedAt.Format("15:04:05")
			duration := time.Since(inst.CreatedAt)
			durationStr := formatDuration(duration)
			b.WriteString(m.renderLine("Started:", fmt.Sprintf("%s (%s ago)", startedStr, durationStr)))
		}

		// Deadman timer
		if deadman != nil {
			remaining := m.calculateDeadmanRemaining(deadman)
			deadmanStatus := fmt.Sprintf("Active (kills in %s if no heartbeat)", formatDuration(remaining))
			if remaining < time.Hour {
				b.WriteString(m.renderLine("Deadman:", Styles.Warning.Render(deadmanStatus)))
			} else {
				b.WriteString(m.renderLine("Deadman:", deadmanStatus))
			}
		}

		b.WriteString("│" + strings.Repeat(" ", 78) + "│\n")

		// WireGuard status
		if wg != nil {
			wgIcon := Styles.StatusConnected.Render(IconRunning)
			b.WriteString(m.renderLine("WireGuard:", wgIcon+" "+Styles.Success.Render("Connected")))
		}

		// Endpoint
		if inst.WireGuardIP != "" {
			endpoint := fmt.Sprintf("%s:11434", inst.WireGuardIP)
			b.WriteString(m.renderLine("Endpoint:", Styles.Endpoint.Render(endpoint)))
		}

		b.WriteString("│" + strings.Repeat(" ", 78) + "│\n")

		// Current cost
		if cost != nil {
			currentCost := m.state.CalculateAccumulatedCost()
			costStr := fmt.Sprintf("%s%.2f", CurrencyEUR, currentCost)
			b.WriteString(m.renderLine("Current cost:", Styles.Price.Render(costStr)))

			// Projected cost (estimate for 8 hour day)
			hourlyRate := cost.HourlyRate
			projected := hourlyRate * 8
			projectedStr := fmt.Sprintf("%s%.2f (at 8hr)", CurrencyEUR, projected)
			b.WriteString(m.renderLine("Projected:", Styles.Muted.Render(projectedStr)))
		}

		b.WriteString("│" + strings.Repeat(" ", 78) + "│\n")
	}

	// Bottom border
	b.WriteString("└" + strings.Repeat("─", 78) + "┘")

	return b.String()
}

// renderLine renders a single line with label and value, padded to fit the box.
func (m StatusModel) renderLine(label, value string) string {
	// Calculate visible length (without ANSI codes)
	labelLen := len(label)
	// For values with styles, we need to calculate actual display width
	// This is a simplification - in production we'd use lipgloss.Width()
	valueLen := lipgloss.Width(value)

	labelPadding := 14 - labelLen
	if labelPadding < 1 {
		labelPadding = 1
	}

	contentLen := labelLen + labelPadding + valueLen
	rightPadding := 76 - contentLen
	if rightPadding < 0 {
		rightPadding = 0
	}

	return fmt.Sprintf("│  %s%s%s%s│\n",
		Styles.Muted.Render(label),
		strings.Repeat(" ", labelPadding),
		value,
		strings.Repeat(" ", rightPadding))
}

// renderActionsBox renders the quick actions box.
func (m StatusModel) renderActionsBox() string {
	var b strings.Builder

	// Title
	title := "Quick Actions"
	titleLine := fmt.Sprintf("┌─ %s ", title)
	padding := 80 - len(titleLine) - 1
	if padding > 0 {
		titleLine += strings.Repeat("─", padding)
	}
	titleLine += "┐"
	b.WriteString(Styles.Heading.Render(titleLine))
	b.WriteString("\n")

	b.WriteString("│" + strings.Repeat(" ", 78) + "│\n")

	// Actions
	actions := []struct {
		key  string
		desc string
	}{
		{"s", "Stop instance and cleanup"},
		{"t", "Test connection (send ping to model)"},
		{"l", "View logs"},
		{"q", "Quit (instance keeps running)"},
	}

	for _, action := range actions {
		keyStr := fmt.Sprintf("[%s]", action.key)
		line := fmt.Sprintf("│  %s %s", Styles.KeyHint.Render(keyStr), action.desc)
		lineLen := 5 + len(action.desc) // [x] + space + desc
		rightPadding := 76 - lineLen
		if rightPadding < 0 {
			rightPadding = 0
		}
		b.WriteString(line + strings.Repeat(" ", rightPadding) + "│\n")
	}

	b.WriteString("│" + strings.Repeat(" ", 78) + "│\n")

	// Bottom border
	b.WriteString("└" + strings.Repeat("─", 78) + "┘")

	return b.String()
}

// renderTestResult renders the test result.
func (m StatusModel) renderTestResult() string {
	if m.testResult == nil {
		return ""
	}

	var b strings.Builder

	if m.testResult.Success {
		b.WriteString(Styles.Success.Render(fmt.Sprintf(
			"%s Connection test successful (latency: %v)",
			IconSuccess,
			m.testResult.Latency.Round(time.Millisecond))))
	} else {
		errMsg := "unknown error"
		if m.testResult.Error != nil {
			errMsg = m.testResult.Error.Error()
		}
		b.WriteString(Styles.Error.Render(fmt.Sprintf(
			"%s Connection test failed: %s",
			IconError,
			errMsg)))
	}

	return b.String()
}

// calculateDeadmanRemaining calculates the remaining time before deadman kills the instance.
func (m StatusModel) calculateDeadmanRemaining(deadman *config.DeadmanState) time.Duration {
	if deadman == nil {
		return 0
	}

	timeout := time.Duration(deadman.TimeoutHours) * time.Hour
	elapsed := time.Since(deadman.LastHeartbeat)
	remaining := timeout - elapsed

	if remaining < 0 {
		return 0
	}
	return remaining
}

// formatDuration formats a duration in a human-readable format.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// SetDimensions sets the terminal dimensions for the model.
func (m *StatusModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
}

// SetState sets the current state.
func (m *StatusModel) SetState(state *config.State) {
	m.state = state
	m.lastUpdate = time.Now()
}

// State returns the current state.
func (m StatusModel) State() *config.State {
	return m.state
}

// Action returns the action selected by the user.
func (m StatusModel) Action() StatusAction {
	return m.action
}

// ClearAction clears the current action.
func (m *StatusModel) ClearAction() {
	m.action = StatusActionNone
}

// IsTesting returns whether a connection test is in progress.
func (m StatusModel) IsTesting() bool {
	return m.testing
}

// TestResult returns the last test result.
func (m StatusModel) GetTestResult() *TestResult {
	return m.testResult
}

// UpdateState sends a state update message.
func UpdateState(state *config.State) tea.Cmd {
	return func() tea.Msg {
		return StatusStateUpdatedMsg{State: state}
	}
}

// StartTest sends a test start message.
func StartTest() tea.Cmd {
	return func() tea.Msg {
		return StatusTestStartMsg{}
	}
}

// FinishTest sends a test result message.
func FinishTest(result *TestResult) tea.Cmd {
	return func() tea.Msg {
		return StatusTestResultMsg{Result: result}
	}
}
