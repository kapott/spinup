// Package cli provides the Cobra CLI commands for continueplz.
package cli

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmeurs/continueplz/internal/config"
	"github.com/tmeurs/continueplz/internal/deploy"
	"github.com/tmeurs/continueplz/internal/logging"
	"github.com/tmeurs/continueplz/internal/ui"
	"github.com/tmeurs/continueplz/internal/wireguard"
)

// ActiveInstanceModel is the main model for interactive mode when an instance is running.
// It displays the instance status and handles actions like stop, test, logs, quit.
type ActiveInstanceModel struct {
	// Core TUI model
	ui.Model

	// Configuration
	cfg *config.Config

	// State manager
	stateManager *config.StateManager

	// Current state
	state *config.State

	// Program reference for sending messages from goroutines
	program *tea.Program

	// Flags to track active operations
	stopping bool
	testing  bool

	// Test result
	testResult *ui.TestResult
}

// NewActiveInstanceModel creates a new model for when an instance is already running.
func NewActiveInstanceModel(cfg *config.Config, stateManager *config.StateManager, state *config.State) ActiveInstanceModel {
	m := ActiveInstanceModel{
		Model:        ui.NewModelWithState(state),
		cfg:          cfg,
		stateManager: stateManager,
		state:        state,
	}
	// Start in instance status view
	m.SetView(ui.ViewInstanceStatus)
	return m
}

// SetProgram sets the tea.Program reference for async operations.
func (m *ActiveInstanceModel) SetProgram(p *tea.Program) {
	m.program = p
}

// Init implements tea.Model.
func (m ActiveInstanceModel) Init() tea.Cmd {
	return tea.Batch(
		m.Model.Init(),
		m.refreshStateCmd(),
	)
}

// Update implements tea.Model.
func (m ActiveInstanceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		// Forward to base model
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case ui.StatusActionMsg:
		return m.handleStatusAction(msg.Action)

	case ui.StatusTickMsg:
		// Forward to base model and refresh state periodically
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		// Refresh state every tick to update costs/timers
		return m, tea.Batch(cmd, m.refreshStateCmd())

	case ui.StatusStateUpdatedMsg:
		// Forward to base model
		m.state = msg.State
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case ui.StatusTestStartMsg:
		m.testing = true
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case ui.StatusTestResultMsg:
		m.testing = false
		m.testResult = msg.Result
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case stopStartMsg:
		// Transition to stop progress view
		m.stopping = true
		m.SetView(ui.ViewStopProgress)
		return m, m.runStopCmd()

	case ui.StopProgressUpdateMsg:
		// Forward to base model
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case ui.StopProgressCompleteMsg:
		// Stop completed successfully
		m.stopping = false
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case ui.StopProgressErrorMsg:
		// Stop failed
		m.stopping = false
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case ui.StopManualVerificationMsg:
		// Forward manual verification info
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case testConnectionMsg:
		// Start connection test
		m.testing = true
		return m, tea.Batch(
			ui.StartTest(),
			m.runTestCmd(),
		)

	case testResultMsg:
		// Connection test completed
		m.testing = false
		m.testResult = &ui.TestResult{
			Success:  msg.success,
			Latency:  msg.latency,
			Error:    msg.err,
			TestedAt: time.Now(),
		}
		return m, ui.FinishTest(m.testResult)

	case quitActiveMsg:
		return m, tea.Quit
	}

	// Forward unhandled messages to base model
	baseModel, cmd := m.Model.Update(msg)
	m.Model = baseModel.(ui.Model)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// handleKeyPress processes keyboard input for active instance mode.
func (m ActiveInstanceModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Don't process keys during operations
	if m.stopping {
		// Only allow quit during stop
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// Forward to stop progress view for "any key to exit"
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd
	}

	switch msg.String() {
	case "q", "Q":
		// Quit without stopping instance
		return m, tea.Quit

	case "ctrl+c":
		return m, tea.Quit

	case "s", "S":
		// Stop instance
		if !m.stopping {
			return m, func() tea.Msg { return stopStartMsg{} }
		}
		return m, nil

	case "t", "T":
		// Test connection
		if !m.testing {
			return m, func() tea.Msg { return testConnectionMsg{} }
		}
		return m, nil

	case "l", "L":
		// Show logs - for now just update status message
		m.SetStatusMessage("Log viewing not yet implemented - check ~/.continueplz.log")
		return m, nil

	case "?":
		// Show help
		m.SetStatusMessage("Keys: s=stop, t=test, l=logs, q=quit (instance keeps running)")
		return m, nil
	}

	// Forward to base model for other keys
	baseModel, cmd := m.Model.Update(msg)
	m.Model = baseModel.(ui.Model)
	return m, cmd
}

// handleStatusAction handles actions triggered from the status view.
func (m ActiveInstanceModel) handleStatusAction(action ui.StatusAction) (tea.Model, tea.Cmd) {
	switch action {
	case ui.StatusActionStop:
		if !m.stopping {
			return m, func() tea.Msg { return stopStartMsg{} }
		}
	case ui.StatusActionTest:
		if !m.testing {
			return m, func() tea.Msg { return testConnectionMsg{} }
		}
	case ui.StatusActionLogs:
		m.SetStatusMessage("Log viewing not yet implemented - check ~/.continueplz.log")
	case ui.StatusActionQuit:
		return m, tea.Quit
	}
	return m, nil
}

// View implements tea.Model.
func (m ActiveInstanceModel) View() string {
	return m.Model.View()
}

// Message types for active instance mode

type stopStartMsg struct{}
type quitActiveMsg struct{}
type testConnectionMsg struct{}

type testResultMsg struct {
	success bool
	latency time.Duration
	err     error
}

// Commands

// refreshStateCmd creates a command that refreshes the state from disk.
func (m ActiveInstanceModel) refreshStateCmd() tea.Cmd {
	return func() tea.Msg {
		state, err := m.stateManager.LoadState()
		if err != nil {
			// State load failed - instance may have been stopped externally
			return ui.StatusStateUpdatedMsg{State: nil}
		}
		return ui.StatusStateUpdatedMsg{State: state}
	}
}

// runStopCmd runs the stop process in the background.
func (m ActiveInstanceModel) runStopCmd() tea.Cmd {
	return func() tea.Msg {
		log := logging.Get()
		log.Info().Msg("Starting instance stop process")

		// Create stopper
		stopCfg := deploy.DefaultStopConfig()
		stopper, err := deploy.NewStopper(
			m.cfg,
			stopCfg,
			deploy.WithStopStateManager(m.stateManager),
			deploy.WithStopProgressCallback(func(p deploy.StopProgress) {
				if m.program != nil {
					m.program.Send(ui.StopProgressUpdateMsg{Progress: p})
				}
			}),
			deploy.WithManualVerificationCallback(func(mv *deploy.ManualVerification) {
				if m.program != nil {
					m.program.Send(ui.StopManualVerificationMsg{Verification: mv})
				}
			}),
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create stopper")
			return ui.StopProgressErrorMsg{Err: err}
		}

		// Run stop process
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		result, err := stopper.Stop(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Stop process failed")
			return ui.StopProgressErrorMsg{Err: err}
		}

		log.Info().
			Str("instance_id", result.InstanceID).
			Float64("session_cost", result.SessionCost).
			Dur("session_duration", result.SessionDuration).
			Msg("Instance stopped successfully")

		// Convert to UI result
		uiResult := ui.ResultFromStopResult(result)
		return ui.StopProgressCompleteMsg{Result: uiResult}
	}
}

// runTestCmd runs the connection test in the background.
func (m ActiveInstanceModel) runTestCmd() tea.Cmd {
	return func() tea.Msg {
		log := logging.Get()
		log.Debug().Msg("Starting connection test")

		// Get server IP from state
		serverIP := wireguard.ServerIP // Default
		if m.state != nil && m.state.Instance != nil && m.state.Instance.WireGuardIP != "" {
			serverIP = m.state.Instance.WireGuardIP
		}

		// Set up verification options to also check Ollama
		opts := wireguard.DefaultVerifyOptions()
		opts.ServerIP = serverIP
		opts.CheckOllama = true
		opts.Timeout = 10 * time.Second

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Run verification
		start := time.Now()
		result := wireguard.VerifyConnection(ctx, opts)
		elapsed := time.Since(start)

		if result.Connected {
			log.Info().
				Dur("latency", result.Latency).
				Bool("ollama_ok", result.OllamaOK).
				Msg("Connection test successful")

			return testResultMsg{
				success: true,
				latency: result.Latency,
			}
		}

		// Connection failed
		errMsg := result.ErrorDetails
		if errMsg == "" && result.Error != nil {
			errMsg = result.Error.Error()
		}

		log.Warn().
			Str("error", errMsg).
			Dur("elapsed", elapsed).
			Msg("Connection test failed")

		return testResultMsg{
			success: false,
			latency: elapsed,
			err:     fmt.Errorf("%s", errMsg),
		}
	}
}

// RunActiveInstanceMode starts the interactive TUI mode when an instance is already running.
// It displays the instance status and handles actions like stop, test, logs, quit.
func RunActiveInstanceMode(cfg *config.Config, stateManager *config.StateManager, state *config.State) error {
	log := logging.Get()
	log.Debug().
		Str("instance_id", state.Instance.ID).
		Str("provider", state.Instance.Provider).
		Msg("Starting interactive mode (active instance)")

	model := NewActiveInstanceModel(cfg, stateManager, state)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Set the program reference for async updates
	model.SetProgram(p)

	_, err := p.Run()
	return err
}
