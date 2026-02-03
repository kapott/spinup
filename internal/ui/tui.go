// Package ui provides the TUI (Terminal User Interface) for spinup.
// It uses Bubbletea for the interactive interface and lipgloss for styling.
package ui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tmeurs/spinup/internal/config"
)

// View represents the different views in the TUI
type View int

const (
	// ViewProviderSelect shows provider/GPU selection
	ViewProviderSelect View = iota
	// ViewModelSelect shows model selection
	ViewModelSelect
	// ViewDeployProgress shows deployment progress
	ViewDeployProgress
	// ViewInstanceStatus shows active instance status
	ViewInstanceStatus
	// ViewStopProgress shows stop progress
	ViewStopProgress
	// ViewAlert shows alert/warning messages
	ViewAlert
)

// String returns the string representation of the view
func (v View) String() string {
	switch v {
	case ViewProviderSelect:
		return "provider_select"
	case ViewModelSelect:
		return "model_select"
	case ViewDeployProgress:
		return "deploy_progress"
	case ViewInstanceStatus:
		return "instance_status"
	case ViewStopProgress:
		return "stop_progress"
	case ViewAlert:
		return "alert"
	default:
		return "unknown"
	}
}

// Model represents the main TUI model
type Model struct {
	// Current view being displayed
	currentView View

	// Terminal dimensions
	width  int
	height int

	// Application state
	quitting bool
	ready    bool

	// Error message to display (if any)
	err error

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Sub-models for different views
	providerSelect ProviderSelectModel
	modelSelect    ModelSelectModel
	deployProgress DeployProgressModel
	statusView     StatusModel
	stopProgress   StopProgressModel

	// Status message for the footer
	statusMessage string
}

// NewModel creates a new TUI model
func NewModel() Model {
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		currentView:    ViewProviderSelect,
		ctx:            ctx,
		cancel:         cancel,
		statusMessage:  "Press 'q' to quit, '?' for help",
		providerSelect: NewProviderSelectModel(),
		modelSelect:    NewModelSelectModel(),
		deployProgress: NewDeployProgressModel(),
		statusView:     NewStatusModel(),
		stopProgress:   NewStopProgressModel(),
	}
}

// NewModelWithView creates a new TUI model with a specific starting view
func NewModelWithView(view View) Model {
	m := NewModel()
	m.currentView = view
	return m
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	// Initialize spinners for progress views
	return tea.Batch(
		m.deployProgress.Init(),
		m.stopProgress.Init(),
	)
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		// Update sub-model dimensions
		m.providerSelect.SetDimensions(msg.Width, msg.Height)
		m.modelSelect.SetDimensions(msg.Width, msg.Height)
		m.deployProgress.SetDimensions(msg.Width, msg.Height)
		m.statusView.SetDimensions(msg.Width, msg.Height)
		m.stopProgress.SetDimensions(msg.Width, msg.Height)
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case quitMsg:
		m.quitting = true
		return m, tea.Quit

	case OffersLoadedMsg:
		// Forward to provider select model
		var cmd tea.Cmd
		m.providerSelect, cmd = m.providerSelect.Update(msg)
		return m, cmd

	case OffersLoadErrorMsg:
		// Forward to provider select model
		var cmd tea.Cmd
		m.providerSelect, cmd = m.providerSelect.Update(msg)
		m.err = msg.Err
		return m, cmd

	case OfferSelectedMsg:
		// An offer was selected - transition to model selection
		m.statusMessage = fmt.Sprintf("Selected: %s %s", msg.Offer.Provider, msg.Offer.GPU)
		// In a future feature (F042), this will transition to model selection
		return m, nil

	case RefreshOffersMsg:
		// Handle refresh request - in production this would trigger fetching
		m.statusMessage = "Refreshing prices..."
		return m, nil

	case ModelsLoadedMsg:
		// Forward to model select model
		var cmd tea.Cmd
		m.modelSelect, cmd = m.modelSelect.Update(msg)
		return m, cmd

	case ModelsLoadErrorMsg:
		// Forward to model select model
		var cmd tea.Cmd
		m.modelSelect, cmd = m.modelSelect.Update(msg)
		m.err = msg.Err
		return m, cmd

	case ModelSelectedMsg:
		// A model was selected - in future features, this will trigger deployment
		m.statusMessage = fmt.Sprintf("Selected model: %s", msg.Model.Name)
		return m, nil

	case RefreshModelsMsg:
		// Handle refresh request - in production this would trigger fetching
		m.statusMessage = "Refreshing models..."
		return m, nil

	case GPUSelectedMsg:
		// Forward to model select model
		var cmd tea.Cmd
		m.modelSelect, cmd = m.modelSelect.Update(msg)
		return m, cmd

	case DeployProgressUpdateMsg:
		// Forward to deploy progress model
		var cmd tea.Cmd
		m.deployProgress, cmd = m.deployProgress.Update(msg)
		return m, cmd

	case DeployProgressCompleteMsg:
		// Forward to deploy progress model
		var cmd tea.Cmd
		m.deployProgress, cmd = m.deployProgress.Update(msg)
		m.statusMessage = "Deployment complete!"
		return m, cmd

	case DeployProgressErrorMsg:
		// Forward to deploy progress model
		var cmd tea.Cmd
		m.deployProgress, cmd = m.deployProgress.Update(msg)
		m.err = msg.Err
		m.statusMessage = "Deployment failed"
		return m, cmd

	case StatusTickMsg:
		// Forward to status view model
		var cmd tea.Cmd
		m.statusView, cmd = m.statusView.Update(msg)
		return m, cmd

	case StatusStateUpdatedMsg:
		// Forward to status view model
		var cmd tea.Cmd
		m.statusView, cmd = m.statusView.Update(msg)
		return m, cmd

	case StatusActionMsg:
		// Handle status view actions
		switch msg.Action {
		case StatusActionStop:
			m.statusMessage = "Stopping instance..."
			// In future feature (F043), this will trigger the stop flow
		case StatusActionTest:
			m.statusMessage = "Testing connection..."
			// In future feature (F043), this will trigger a connection test
		case StatusActionLogs:
			m.statusMessage = "Viewing logs..."
			// In future feature (F043), this will show logs
		case StatusActionQuit:
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case StatusTestStartMsg:
		// Forward to status view model
		var cmd tea.Cmd
		m.statusView, cmd = m.statusView.Update(msg)
		return m, cmd

	case StatusTestResultMsg:
		// Forward to status view model
		var cmd tea.Cmd
		m.statusView, cmd = m.statusView.Update(msg)
		if msg.Result != nil && msg.Result.Success {
			m.statusMessage = "Connection test successful"
		} else {
			m.statusMessage = "Connection test failed"
		}
		return m, cmd

	case StopProgressUpdateMsg:
		// Forward to stop progress model
		var cmd tea.Cmd
		m.stopProgress, cmd = m.stopProgress.Update(msg)
		return m, cmd

	case StopProgressCompleteMsg:
		// Forward to stop progress model
		var cmd tea.Cmd
		m.stopProgress, cmd = m.stopProgress.Update(msg)
		m.statusMessage = "Instance stopped successfully"
		return m, cmd

	case StopProgressErrorMsg:
		// Forward to stop progress model
		var cmd tea.Cmd
		m.stopProgress, cmd = m.stopProgress.Update(msg)
		m.err = msg.Err
		m.statusMessage = "Stop failed"
		return m, cmd

	case StopManualVerificationMsg:
		// Forward to stop progress model
		var cmd tea.Cmd
		m.stopProgress, cmd = m.stopProgress.Update(msg)
		return m, cmd
	}

	// Forward messages to active sub-model based on current view
	switch m.currentView {
	case ViewProviderSelect:
		var cmd tea.Cmd
		m.providerSelect, cmd = m.providerSelect.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case ViewModelSelect:
		var cmd tea.Cmd
		m.modelSelect, cmd = m.modelSelect.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case ViewDeployProgress:
		var cmd tea.Cmd
		m.deployProgress, cmd = m.deployProgress.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case ViewInstanceStatus:
		var cmd tea.Cmd
		m.statusView, cmd = m.statusView.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case ViewStopProgress:
		var cmd tea.Cmd
		m.stopProgress, cmd = m.stopProgress.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.quitting {
		return "Goodbye!\n"
	}

	var content string

	switch m.currentView {
	case ViewProviderSelect:
		content = m.providerSelect.View()
	case ViewModelSelect:
		content = m.modelSelect.View()
	case ViewDeployProgress:
		content = m.deployProgress.View()
	case ViewInstanceStatus:
		content = m.statusView.View()
	case ViewStopProgress:
		content = m.stopProgress.View()
	case ViewAlert:
		content = m.renderAlertPlaceholder()
	default:
		content = "Unknown view"
	}

	return m.renderFrame(content)
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global key handlers
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		// ESC goes back to previous view or quits if at root
		if m.currentView == ViewProviderSelect {
			m.quitting = true
			return m, tea.Quit
		}
		// For other views, we'd go back (to be implemented)
		return m, nil

	case "?":
		// Show help
		m.statusMessage = "Help: q=quit, ↑↓=navigate, Enter=select, r=refresh"
		return m, nil
	}

	// Forward to active sub-model based on current view
	switch m.currentView {
	case ViewProviderSelect:
		var cmd tea.Cmd
		m.providerSelect, cmd = m.providerSelect.Update(msg)
		return m, cmd
	case ViewModelSelect:
		var cmd tea.Cmd
		m.modelSelect, cmd = m.modelSelect.Update(msg)
		return m, cmd
	case ViewInstanceStatus:
		var cmd tea.Cmd
		m.statusView, cmd = m.statusView.Update(msg)
		return m, cmd
	}

	return m, nil
}

// renderFrame wraps content in the main application frame
func (m Model) renderFrame(content string) string {
	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Main content
	b.WriteString(content)
	b.WriteString("\n")

	// Footer
	footer := m.renderFooter()
	b.WriteString(footer)

	return b.String()
}

// renderHeader renders the application header
func (m Model) renderHeader() string {
	title := "spinup"
	subtitle := "GPU Code Assistant Launcher"

	titleStyle := Styles.Title.Width(m.width)
	subtitleStyle := Styles.Subtitle.Width(m.width)

	header := lipgloss.JoinVertical(
		lipgloss.Center,
		titleStyle.Render(title),
		subtitleStyle.Render(subtitle),
	)

	return Styles.Header.Width(m.width).Render(header)
}

// renderFooter renders the application footer
func (m Model) renderFooter() string {
	var statusText string
	if m.err != nil {
		statusText = Styles.Error.Render(fmt.Sprintf("Error: %v", m.err))
	} else {
		statusText = Styles.Muted.Render(m.statusMessage)
	}

	return Styles.Footer.Width(m.width).Render(statusText)
}

// Placeholder view renders (to be replaced in future features)

func (m Model) renderAlertPlaceholder() string {
	content := `
╭─────────────────────────────────────────────────────────────────────────────╮
│  ⚠️  ALERT                                                                    │
│                                                                              │
│  (No alerts)                                                                 │
│                                                                              │
╰─────────────────────────────────────────────────────────────────────────────╯
`
	return Styles.AlertBox.Render(content)
}

// Custom messages

type errMsg struct {
	err error
}

type quitMsg struct{}

// Commands

// Quit returns a command that quits the application
func Quit() tea.Msg {
	return quitMsg{}
}

// SetError returns a command that sets an error message
func SetError(err error) tea.Cmd {
	return func() tea.Msg {
		return errMsg{err: err}
	}
}

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(
		NewModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}

// RunWithModel starts the TUI application with a custom model
func RunWithModel(m Model) error {
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}

// RunWithView starts the TUI application with a specific starting view
func RunWithView(view View) error {
	return RunWithModel(NewModelWithView(view))
}

// Getters and setters

// SetView changes the current view
func (m *Model) SetView(view View) {
	m.currentView = view
}

// GetView returns the current view
func (m Model) GetView() View {
	return m.currentView
}

// SetStatusMessage sets the footer status message
func (m *Model) SetStatusMessage(msg string) {
	m.statusMessage = msg
}

// Width returns the terminal width
func (m Model) Width() int {
	return m.width
}

// Height returns the terminal height
func (m Model) Height() int {
	return m.height
}

// IsReady returns whether the TUI has been initialized with terminal dimensions
func (m Model) IsReady() bool {
	return m.ready
}

// IsQuitting returns whether the TUI is in the process of quitting
func (m Model) IsQuitting() bool {
	return m.quitting
}

// Context returns the application context
func (m Model) Context() context.Context {
	return m.ctx
}

// Cancel cancels the application context
func (m Model) Cancel() {
	if m.cancel != nil {
		m.cancel()
	}
}

// SetInstanceState sets the state for the instance status view.
func (m *Model) SetInstanceState(state *config.State) {
	m.statusView.SetState(state)
}

// GetStatusView returns the status view model.
func (m Model) GetStatusView() StatusModel {
	return m.statusView
}

// NewModelWithState creates a new TUI model with initial state for the status view.
func NewModelWithState(state *config.State) Model {
	m := NewModel()
	m.currentView = ViewInstanceStatus
	m.statusView.SetState(state)
	return m
}

// GetStopProgressView returns the stop progress view model.
func (m Model) GetStopProgressView() StopProgressModel {
	return m.stopProgress
}

// NewModelForStop creates a new TUI model configured for the stop progress view.
func NewModelForStop() Model {
	m := NewModel()
	m.currentView = ViewStopProgress
	return m
}
