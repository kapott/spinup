// Package ui provides TUI components for continueplz.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tmeurs/continueplz/internal/deploy"
)

// DeployStepState represents the state of a deployment step.
type DeployStepState int

const (
	// StepStatePending indicates the step has not started.
	StepStatePending DeployStepState = iota
	// StepStateInProgress indicates the step is currently running.
	StepStateInProgress
	// StepStateCompleted indicates the step completed successfully.
	StepStateCompleted
	// StepStateFailed indicates the step failed.
	StepStateFailed
)

// DeployStepInfo holds information about a deployment step.
type DeployStepInfo struct {
	Step      deploy.DeployStep
	State     DeployStepState
	Message   string
	Detail    string
	StartedAt time.Time
	EndedAt   time.Time
}

// DeployProgressModel is the Bubbletea model for the deployment progress view.
type DeployProgressModel struct {
	// Steps holds the state of each deployment step.
	steps []DeployStepInfo

	// currentStep is the index of the currently active step (0-based).
	currentStep int

	// spinner is the spinner component for in-progress steps.
	spinner spinner.Model

	// width and height are the terminal dimensions.
	width  int
	height int

	// error holds any error that occurred during deployment.
	err error

	// completed indicates if the deployment is complete.
	completed bool

	// failed indicates if the deployment failed.
	failed bool

	// result holds the final deployment result.
	result *DeployProgressResult

	// startedAt is when the deployment started.
	startedAt time.Time
}

// DeployProgressResult holds the final deployment result for display.
type DeployProgressResult struct {
	Provider      string
	GPU           string
	Region        string
	InstanceType  string // "spot" or "on-demand"
	HourlyRate    float64
	Model         string
	Endpoint      string
	DeadmanHours  int
	ProviderCount int
	OfferCount    int
	Duration      time.Duration
}

// NewDeployProgressModel creates a new deploy progress model.
func NewDeployProgressModel() DeployProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = Styles.Spinner

	// Initialize all steps
	steps := make([]DeployStepInfo, deploy.TotalDeploySteps)
	for i := 0; i < deploy.TotalDeploySteps; i++ {
		step := deploy.DeployStep(i + 1) // Steps are 1-indexed
		steps[i] = DeployStepInfo{
			Step:    step,
			State:   StepStatePending,
			Message: step.String(),
		}
	}

	return DeployProgressModel{
		steps:       steps,
		currentStep: -1, // No step started yet
		spinner:     s,
		startedAt:   time.Now(),
	}
}

// Init implements tea.Model.
func (m DeployProgressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// DeployProgressUpdateMsg is sent to update deployment progress.
type DeployProgressUpdateMsg struct {
	Progress deploy.DeployProgress
}

// DeployProgressCompleteMsg is sent when deployment is complete.
type DeployProgressCompleteMsg struct {
	Result *DeployProgressResult
}

// DeployProgressErrorMsg is sent when deployment fails.
type DeployProgressErrorMsg struct {
	Err error
}

// Update implements tea.Model.
func (m DeployProgressModel) Update(msg tea.Msg) (DeployProgressModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case DeployProgressUpdateMsg:
		m = m.handleProgressUpdate(msg.Progress)
		return m, m.spinner.Tick

	case DeployProgressCompleteMsg:
		m.completed = true
		m.result = msg.Result
		// Mark all steps as complete
		for i := range m.steps {
			if m.steps[i].State == StepStateInProgress {
				m.steps[i].State = StepStateCompleted
				m.steps[i].EndedAt = time.Now()
			}
		}
		return m, nil

	case DeployProgressErrorMsg:
		m.failed = true
		m.err = msg.Err
		// Mark current step as failed
		if m.currentStep >= 0 && m.currentStep < len(m.steps) {
			m.steps[m.currentStep].State = StepStateFailed
			m.steps[m.currentStep].EndedAt = time.Now()
		}
		return m, nil
	}

	return m, nil
}

// handleProgressUpdate updates the model based on a progress update.
func (m DeployProgressModel) handleProgressUpdate(p deploy.DeployProgress) DeployProgressModel {
	stepIndex := int(p.Step) - 1 // Convert 1-indexed to 0-indexed
	if stepIndex < 0 || stepIndex >= len(m.steps) {
		return m
	}

	// Update step info
	m.steps[stepIndex].Message = p.Message
	m.steps[stepIndex].Detail = p.Detail

	if p.Completed {
		m.steps[stepIndex].State = StepStateCompleted
		m.steps[stepIndex].EndedAt = time.Now()
	} else if p.Error != nil {
		m.steps[stepIndex].State = StepStateFailed
		m.steps[stepIndex].EndedAt = time.Now()
	} else {
		// Step is in progress
		if m.steps[stepIndex].State != StepStateInProgress {
			m.steps[stepIndex].State = StepStateInProgress
			m.steps[stepIndex].StartedAt = time.Now()
		}
		m.currentStep = stepIndex
	}

	return m
}

// View implements tea.Model.
func (m DeployProgressModel) View() string {
	var b strings.Builder

	// Title
	title := "Deployment Progress"
	if m.completed {
		title = Styles.Success.Render(IconSuccess + " Deployment Complete")
	} else if m.failed {
		title = Styles.Error.Render(IconError + " Deployment Failed")
	}
	b.WriteString(Styles.Heading.Render(title))
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

	// Success summary if completed
	if m.completed && m.result != nil {
		b.WriteString("\n")
		b.WriteString(m.renderSummary())
	}

	return Styles.Box.Width(m.width - 4).Render(b.String())
}

// renderStep renders a single deployment step.
func (m DeployProgressModel) renderStep(index int, step DeployStepInfo) string {
	var icon string
	var messageStyle lipgloss.Style

	switch step.State {
	case StepStatePending:
		icon = Styles.Muted.Render(IconPending)
		messageStyle = Styles.Muted
	case StepStateInProgress:
		icon = m.spinner.View()
		messageStyle = Styles.Body
	case StepStateCompleted:
		icon = Styles.Checkmark.Render(IconCheckmark)
		messageStyle = Styles.Success
	case StepStateFailed:
		icon = Styles.CrossMark.Render(IconCrossMark)
		messageStyle = Styles.Error
	}

	// Step number
	stepNum := fmt.Sprintf("[%d/%d]", index+1, deploy.TotalDeploySteps)

	// Main line
	line := fmt.Sprintf("%s %s %s", stepNum, icon, messageStyle.Render(step.Message))

	// Detail line (indented)
	if step.Detail != "" {
		detailStyle := Styles.Muted
		if step.State == StepStateFailed {
			detailStyle = Styles.Error
		}
		line += "\n      " + detailStyle.Render(step.Detail)
	}

	return line
}

// renderSummary renders the deployment success summary.
func (m DeployProgressModel) renderSummary() string {
	if m.result == nil {
		return ""
	}

	r := m.result
	var b strings.Builder

	// Separator
	separator := strings.Repeat(BoxHorizontal, 75)
	b.WriteString(Styles.Muted.Render(separator))
	b.WriteString("\n\n")

	// Success banner
	b.WriteString("  ")
	b.WriteString(Styles.Success.Bold(true).Render(IconSuccess + " READY"))
	b.WriteString("\n\n")

	// Details
	labelStyle := Styles.Muted
	valueStyle := Styles.Body

	// Provider
	instanceType := r.InstanceType
	if instanceType == "" {
		instanceType = "on-demand"
	}
	b.WriteString(fmt.Sprintf("  %s    %s\n",
		labelStyle.Render("Provider:"),
		Styles.Provider.Render(fmt.Sprintf("%s (%s)", r.Provider, instanceType))))

	// GPU
	b.WriteString(fmt.Sprintf("  %s         %s\n",
		labelStyle.Render("GPU:"),
		Styles.GPU.Render(r.GPU)))

	// Region
	b.WriteString(fmt.Sprintf("  %s      %s\n",
		labelStyle.Render("Region:"),
		valueStyle.Render(r.Region)))

	// Model
	b.WriteString(fmt.Sprintf("  %s       %s\n",
		labelStyle.Render("Model:"),
		Styles.Model.Render(r.Model)))

	// Hourly rate
	priceStr := fmt.Sprintf("%s%.2f/hr", CurrencyEUR, r.HourlyRate)
	b.WriteString(fmt.Sprintf("  %s       %s\n",
		labelStyle.Render("Price:"),
		Styles.PriceSpot.Render(priceStr)))

	// Deadman timeout
	b.WriteString(fmt.Sprintf("  %s     %s\n",
		labelStyle.Render("Deadman:"),
		valueStyle.Render(fmt.Sprintf("%dh timeout", r.DeadmanHours))))

	b.WriteString("\n")

	// Endpoint
	b.WriteString(fmt.Sprintf("  %s    %s\n",
		labelStyle.Render("Endpoint:"),
		Styles.Endpoint.Render(r.Endpoint)))

	b.WriteString("\n")

	// Stats
	duration := r.Duration.Round(time.Second)
	statsLine := fmt.Sprintf("  %s from %d providers, deployed in %v",
		Styles.Muted.Render(fmt.Sprintf("%d offers", r.OfferCount)),
		r.ProviderCount,
		duration)
	b.WriteString(Styles.Muted.Render(statsLine))
	b.WriteString("\n")

	return b.String()
}

// SetDimensions sets the terminal dimensions for the model.
func (m *DeployProgressModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
}

// IsCompleted returns true if the deployment is complete.
func (m DeployProgressModel) IsCompleted() bool {
	return m.completed
}

// IsFailed returns true if the deployment failed.
func (m DeployProgressModel) IsFailed() bool {
	return m.failed
}

// Error returns the deployment error, if any.
func (m DeployProgressModel) Error() error {
	return m.err
}

// Result returns the deployment result, if complete.
func (m DeployProgressModel) Result() *DeployProgressResult {
	return m.result
}

// UpdateProgress sends a progress update message.
func UpdateProgress(p deploy.DeployProgress) tea.Cmd {
	return func() tea.Msg {
		return DeployProgressUpdateMsg{Progress: p}
	}
}

// CompleteDeployment sends a deployment complete message.
func CompleteDeployment(result *DeployProgressResult) tea.Cmd {
	return func() tea.Msg {
		return DeployProgressCompleteMsg{Result: result}
	}
}

// FailDeployment sends a deployment failed message.
func FailDeployment(err error) tea.Cmd {
	return func() tea.Msg {
		return DeployProgressErrorMsg{Err: err}
	}
}

// MakeProgressCallback creates a callback function suitable for deploy.WithProgressCallback.
// The callback sends progress updates to the Bubbletea program.
func MakeProgressCallback(p *tea.Program) func(deploy.DeployProgress) {
	return func(progress deploy.DeployProgress) {
		p.Send(DeployProgressUpdateMsg{Progress: progress})
	}
}

// ResultFromDeployResult converts a deploy.DeployResult to a DeployProgressResult.
func ResultFromDeployResult(r *deploy.DeployResult, deadmanHours int) *DeployProgressResult {
	if r == nil {
		return nil
	}

	instanceType := "on-demand"
	if r.Instance != nil && r.Instance.Spot {
		instanceType = "spot"
	}

	result := &DeployProgressResult{
		ProviderCount: r.ProvidersQueried,
		OfferCount:    r.TotalProviderOffers,
		Duration:      r.Duration(),
		DeadmanHours:  deadmanHours,
	}

	if r.Provider != nil {
		result.Provider = r.Provider.Name()
	}

	if r.Instance != nil {
		result.GPU = r.Instance.GPU
		result.Region = r.Instance.Region
		result.HourlyRate = r.Instance.HourlyRate
	}

	if r.SelectedOffer != nil {
		if result.GPU == "" {
			result.GPU = r.SelectedOffer.GPU
		}
		if result.Region == "" {
			result.Region = r.SelectedOffer.Region
		}
	}

	if r.Model != nil {
		result.Model = r.Model.Name
	}

	result.InstanceType = instanceType
	result.Endpoint = r.OllamaEndpoint

	return result
}
