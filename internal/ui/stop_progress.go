// Package ui provides TUI components for spinup.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tmeurs/spinup/internal/deploy"
)

// StopStepState represents the state of a stop step.
type StopStepState int

const (
	// StopStepStatePending indicates the step has not started.
	StopStepStatePending StopStepState = iota
	// StopStepStateInProgress indicates the step is currently running.
	StopStepStateInProgress
	// StopStepStateCompleted indicates the step completed successfully.
	StopStepStateCompleted
	// StopStepStateWarning indicates the step completed with a warning.
	StopStepStateWarning
	// StopStepStateFailed indicates the step failed.
	StopStepStateFailed
)

// StopStepInfo holds information about a stop step.
type StopStepInfo struct {
	Step      deploy.StopStep
	State     StopStepState
	Message   string
	Detail    string
	StartedAt time.Time
	EndedAt   time.Time
}

// StopProgressModel is the Bubbletea model for the stop progress view.
type StopProgressModel struct {
	// steps holds the state of each stop step.
	steps []StopStepInfo

	// currentStep is the index of the currently active step (0-based).
	currentStep int

	// spinner is the spinner component for in-progress steps.
	spinner spinner.Model

	// width and height are the terminal dimensions.
	width  int
	height int

	// error holds any error that occurred during stop.
	err error

	// completed indicates if the stop process is complete.
	completed bool

	// failed indicates if the stop process failed.
	failed bool

	// result holds the final stop result.
	result *StopProgressResult

	// manualVerification holds manual verification info if required.
	manualVerification *deploy.ManualVerification

	// startedAt is when the stop process started.
	startedAt time.Time

	// waitingForKey indicates if we're waiting for key press to exit.
	waitingForKey bool
}

// StopProgressResult holds the final stop result for display.
type StopProgressResult struct {
	InstanceID                 string
	Provider                   string
	SessionCost                float64
	SessionDuration            time.Duration
	BillingVerified            bool
	ManualVerificationRequired bool
	ConsoleURL                 string
	StopDuration               time.Duration
}

// NewStopProgressModel creates a new stop progress model.
func NewStopProgressModel() StopProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = Styles.Spinner

	// Initialize all steps
	steps := make([]StopStepInfo, deploy.TotalStopSteps)
	for i := 0; i < deploy.TotalStopSteps; i++ {
		step := deploy.StopStep(i + 1) // Steps are 1-indexed
		steps[i] = StopStepInfo{
			Step:    step,
			State:   StopStepStatePending,
			Message: step.String(),
		}
	}

	return StopProgressModel{
		steps:       steps,
		currentStep: -1, // No step started yet
		spinner:     s,
		startedAt:   time.Now(),
	}
}

// Init implements tea.Model.
func (m StopProgressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// StopProgressUpdateMsg is sent to update stop progress.
type StopProgressUpdateMsg struct {
	Progress deploy.StopProgress
}

// StopProgressCompleteMsg is sent when stop is complete.
type StopProgressCompleteMsg struct {
	Result *StopProgressResult
}

// StopProgressErrorMsg is sent when stop fails.
type StopProgressErrorMsg struct {
	Err error
}

// StopManualVerificationMsg is sent when manual verification is required.
type StopManualVerificationMsg struct {
	Verification *deploy.ManualVerification
}

// Update implements tea.Model.
func (m StopProgressModel) Update(msg tea.Msg) (StopProgressModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// If waiting for key press to exit, any key triggers quit
		if m.waitingForKey {
			return m, tea.Quit
		}
		// q/ctrl+c always quits
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case StopProgressUpdateMsg:
		m = m.handleProgressUpdate(msg.Progress)
		return m, m.spinner.Tick

	case StopProgressCompleteMsg:
		m.completed = true
		m.result = msg.Result
		m.waitingForKey = true
		// Mark all steps as complete
		for i := range m.steps {
			if m.steps[i].State == StopStepStateInProgress {
				m.steps[i].State = StopStepStateCompleted
				m.steps[i].EndedAt = time.Now()
			}
		}
		return m, nil

	case StopProgressErrorMsg:
		m.failed = true
		m.err = msg.Err
		m.waitingForKey = true
		// Mark current step as failed
		if m.currentStep >= 0 && m.currentStep < len(m.steps) {
			m.steps[m.currentStep].State = StopStepStateFailed
			m.steps[m.currentStep].EndedAt = time.Now()
		}
		return m, nil

	case StopManualVerificationMsg:
		m.manualVerification = msg.Verification
		return m, nil
	}

	return m, nil
}

// handleProgressUpdate updates the model based on a progress update.
func (m StopProgressModel) handleProgressUpdate(p deploy.StopProgress) StopProgressModel {
	stepIndex := int(p.Step) - 1 // Convert 1-indexed to 0-indexed
	if stepIndex < 0 || stepIndex >= len(m.steps) {
		return m
	}

	// Update step info
	m.steps[stepIndex].Message = p.Message
	m.steps[stepIndex].Detail = p.Detail

	if p.Completed {
		if p.Warning {
			m.steps[stepIndex].State = StopStepStateWarning
		} else {
			m.steps[stepIndex].State = StopStepStateCompleted
		}
		m.steps[stepIndex].EndedAt = time.Now()
	} else if p.Error != nil {
		m.steps[stepIndex].State = StopStepStateFailed
		m.steps[stepIndex].EndedAt = time.Now()
	} else {
		// Step is in progress
		if m.steps[stepIndex].State == StopStepStatePending {
			m.steps[stepIndex].State = StopStepStateInProgress
			m.steps[stepIndex].StartedAt = time.Now()
		}
		m.currentStep = stepIndex
	}

	return m
}

// View implements tea.Model.
func (m StopProgressModel) View() string {
	var b strings.Builder

	// Title box
	titleBox := m.renderTitleBox()
	b.WriteString(titleBox)
	b.WriteString("\n\n")

	// Render each step
	for i, step := range m.steps {
		b.WriteString(m.renderStep(i, step))
		b.WriteString("\n")
	}

	// Error message if failed
	if m.failed && m.err != nil {
		b.WriteString("\n")
		b.WriteString(Styles.Error.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
	}

	// Manual verification warning if required
	if m.manualVerification != nil && m.manualVerification.Required {
		b.WriteString("\n")
		b.WriteString(m.renderManualVerification())
	}

	// Session summary if completed
	if m.completed && m.result != nil {
		b.WriteString("\n")
		b.WriteString(m.renderSummary())
	}

	// Footer
	if m.waitingForKey {
		b.WriteString("\n")
		b.WriteString(Styles.Muted.Render("Press any key to exit..."))
	}

	return b.String()
}

// renderTitleBox renders the title box matching PRD format.
func (m StopProgressModel) renderTitleBox() string {
	title := "Stopping Instance"

	// Use header style with border
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorForeground).
		Align(lipgloss.Center).
		Width(75).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	return titleStyle.Render(title)
}

// renderStep renders a single stop step.
func (m StopProgressModel) renderStep(index int, step StopStepInfo) string {
	var icon string
	var messageStyle lipgloss.Style

	switch step.State {
	case StopStepStatePending:
		icon = Styles.Muted.Render("[ ]")
		messageStyle = Styles.Muted
	case StopStepStateInProgress:
		icon = "[" + m.spinner.View() + "]"
		messageStyle = Styles.Body
	case StopStepStateCompleted:
		icon = Styles.Checkmark.Render("[" + IconCheckmark + "]")
		messageStyle = Styles.Success
	case StopStepStateWarning:
		icon = Styles.Warning.Render("[" + IconWarning + "]")
		messageStyle = Styles.Warning
	case StopStepStateFailed:
		icon = Styles.CrossMark.Render("[" + IconCrossMark + "]")
		messageStyle = Styles.Error
	}

	// Step number (matching PRD: [1/4], [2/4], etc.)
	stepNum := fmt.Sprintf("[%d/%d]", index+1, deploy.TotalStopSteps)

	// Main line (indented to match PRD format)
	line := fmt.Sprintf("  %s %s", icon, messageStyle.Render(step.Message))

	// Add step number as prefix for non-TUI output or debugging
	// For TUI, we follow the PRD format which shows checkmarks
	_ = stepNum // unused in TUI view but available

	// Detail line (indented)
	if step.Detail != "" {
		detailStyle := Styles.Muted
		if step.State == StopStepStateFailed {
			detailStyle = Styles.Error
		} else if step.State == StopStepStateWarning {
			detailStyle = Styles.Warning
		}
		line += "\n        " + detailStyle.Render(step.Detail)
	}

	return line
}

// renderManualVerification renders the manual verification warning box.
func (m StopProgressModel) renderManualVerification() string {
	if m.manualVerification == nil || !m.manualVerification.Required {
		return ""
	}

	var b strings.Builder

	// Warning box with double border (matching PRD)
	warningBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorWarning).
		Width(75).
		Padding(0, 1)

	// Content
	b.WriteString(Styles.Warning.Bold(true).Render("  " + IconWarning + "  MANUAL VERIFICATION REQUIRED"))
	b.WriteString("\n\n")
	b.WriteString("  " + m.manualVerification.WarningMessage)
	b.WriteString("\n")
	b.WriteString("  Please verify manually that the instance is terminated:")
	b.WriteString("\n\n")

	for i, instruction := range m.manualVerification.Instructions {
		b.WriteString(fmt.Sprintf("    %d. %s\n", i+1, instruction))
	}

	b.WriteString(fmt.Sprintf("\n    Instance ID: %s", m.manualVerification.InstanceID))

	return warningBoxStyle.Render(b.String())
}

// renderSummary renders the session summary.
func (m StopProgressModel) renderSummary() string {
	if m.result == nil {
		return ""
	}

	r := m.result
	var b strings.Builder

	// Session cost
	costStr := fmt.Sprintf("%s%.2f", CurrencyEUR, r.SessionCost)
	b.WriteString(fmt.Sprintf("Total session cost: %s\n", Styles.PriceSpot.Render(costStr)))

	// Session duration
	durationStr := formatDuration(r.SessionDuration)
	b.WriteString(fmt.Sprintf("Session duration: %s", Styles.Body.Render(durationStr)))

	return b.String()
}

// SetDimensions sets the terminal dimensions for the model.
func (m *StopProgressModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
}

// IsCompleted returns true if the stop process is complete.
func (m StopProgressModel) IsCompleted() bool {
	return m.completed
}

// IsFailed returns true if the stop process failed.
func (m StopProgressModel) IsFailed() bool {
	return m.failed
}

// Error returns the stop error, if any.
func (m StopProgressModel) Error() error {
	return m.err
}

// Result returns the stop result, if complete.
func (m StopProgressModel) Result() *StopProgressResult {
	return m.result
}

// ManualVerification returns the manual verification info, if required.
func (m StopProgressModel) ManualVerification() *deploy.ManualVerification {
	return m.manualVerification
}

// IsWaitingForKey returns true if waiting for key press to exit.
func (m StopProgressModel) IsWaitingForKey() bool {
	return m.waitingForKey
}

// UpdateStopProgress sends a stop progress update message.
func UpdateStopProgress(p deploy.StopProgress) tea.Cmd {
	return func() tea.Msg {
		return StopProgressUpdateMsg{Progress: p}
	}
}

// CompleteStop sends a stop complete message.
func CompleteStop(result *StopProgressResult) tea.Cmd {
	return func() tea.Msg {
		return StopProgressCompleteMsg{Result: result}
	}
}

// FailStop sends a stop failed message.
func FailStop(err error) tea.Cmd {
	return func() tea.Msg {
		return StopProgressErrorMsg{Err: err}
	}
}

// SetManualVerification sends a manual verification message.
func SetManualVerification(verification *deploy.ManualVerification) tea.Cmd {
	return func() tea.Msg {
		return StopManualVerificationMsg{Verification: verification}
	}
}

// MakeStopProgressCallback creates a callback function suitable for deploy.WithStopProgressCallback.
// The callback sends progress updates to the Bubbletea program.
func MakeStopProgressCallback(p *tea.Program) func(deploy.StopProgress) {
	return func(progress deploy.StopProgress) {
		p.Send(StopProgressUpdateMsg{Progress: progress})
	}
}

// MakeManualVerificationCallback creates a callback for manual verification display.
// The callback sends the verification info to the Bubbletea program.
func MakeManualVerificationCallback(p *tea.Program) deploy.ManualVerificationCallback {
	return func(verification *deploy.ManualVerification) {
		p.Send(StopManualVerificationMsg{Verification: verification})
	}
}

// ResultFromStopResult converts a deploy.StopResult to a StopProgressResult.
func ResultFromStopResult(r *deploy.StopResult) *StopProgressResult {
	if r == nil {
		return nil
	}

	return &StopProgressResult{
		InstanceID:                 r.InstanceID,
		Provider:                   r.Provider,
		SessionCost:                r.SessionCost,
		SessionDuration:            r.SessionDuration,
		BillingVerified:            r.BillingVerified,
		ManualVerificationRequired: r.ManualVerificationRequired,
		ConsoleURL:                 r.ConsoleURL,
		StopDuration:               r.Duration(),
	}
}

