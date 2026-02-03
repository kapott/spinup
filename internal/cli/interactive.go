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
	"github.com/tmeurs/continueplz/internal/models"
	providerPkg "github.com/tmeurs/continueplz/internal/provider"
	"github.com/tmeurs/continueplz/internal/provider/registry"
	"github.com/tmeurs/continueplz/internal/ui"
)

// InteractiveModel is the main model for interactive mode when no instance is running.
// It orchestrates the provider selection -> model selection -> deployment flow.
type InteractiveModel struct {
	// Core TUI model
	ui.Model

	// Configuration
	cfg      *config.Config
	deployCfg *deploy.DeployConfig

	// State manager
	stateManager *config.StateManager

	// Selected items
	selectedOffer *providerPkg.Offer
	selectedModel *models.Model
	selectedGPU   *models.GPU

	// Fetching state
	fetchingOffers bool
	lastFetchError error

	// Program reference for sending messages from goroutines
	program *tea.Program
}

// NewInteractiveModel creates a new interactive model for the no-instance state.
func NewInteractiveModel(cfg *config.Config, stateManager *config.StateManager, deployCfg *deploy.DeployConfig) InteractiveModel {
	m := InteractiveModel{
		Model:        ui.NewModel(),
		cfg:          cfg,
		stateManager: stateManager,
		deployCfg:    deployCfg,
	}
	// Start in provider select view
	m.SetView(ui.ViewProviderSelect)
	return m
}

// SetProgram sets the tea.Program reference for async operations.
func (m *InteractiveModel) SetProgram(p *tea.Program) {
	m.program = p
}

// Init implements tea.Model.
func (m InteractiveModel) Init() tea.Cmd {
	return tea.Batch(
		m.Model.Init(),
		m.fetchOffersCmd(),
	)
}

// Update implements tea.Model.
func (m InteractiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		// Forward to base model
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case ui.OffersLoadedMsg:
		// Forward to base model
		m.fetchingOffers = false
		m.lastFetchError = nil
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case ui.OffersLoadErrorMsg:
		m.fetchingOffers = false
		m.lastFetchError = msg.Err
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case ui.RefreshOffersMsg:
		// Handle refresh request
		if !m.fetchingOffers {
			m.fetchingOffers = true
			return m, m.fetchOffersCmd()
		}
		return m, nil

	case ui.OfferSelectedMsg:
		// Handle offer selection - transition to model selection
		m.selectedOffer = &msg.Offer
		// Extract GPU info from the offer
		gpuInfo, err := models.GetGPUByName(msg.Offer.GPU)
		if err == nil {
			m.selectedGPU = gpuInfo
		}
		// Transition to model selection view
		m.SetView(ui.ViewModelSelect)
		// Set the GPU filter for model selection
		return m, func() tea.Msg {
			return ui.GPUSelectedMsg{GPU: *m.selectedGPU}
		}

	case ui.ModelSelectedMsg:
		// Handle model selection - transition to deployment
		m.selectedModel = &msg.Model
		// Trigger deployment
		return m, m.startDeploymentCmd()

	case ui.DeployProgressUpdateMsg:
		// Forward to base model
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case ui.DeployProgressCompleteMsg:
		// Deployment complete - could transition to status view
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		m.SetStatusMessage("Deployment complete! Press 'q' to exit.")
		return m, cmd

	case ui.DeployProgressErrorMsg:
		// Deployment failed
		baseModel, cmd := m.Model.Update(msg)
		m.Model = baseModel.(ui.Model)
		return m, cmd

	case deployStartMsg:
		// Transition to deploy progress view and start deployment
		m.SetView(ui.ViewDeployProgress)
		return m, m.runDeploymentCmd(msg.deployCfg)

	case quitInteractiveMsg:
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

// handleKeyPress processes keyboard input for interactive mode.
func (m InteractiveModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc":
		// ESC goes back to previous view
		switch m.GetView() {
		case ui.ViewModelSelect:
			// Go back to provider select
			m.SetView(ui.ViewProviderSelect)
			m.selectedOffer = nil
			m.selectedGPU = nil
			return m, nil
		case ui.ViewDeployProgress:
			// Can't go back during deployment
			return m, nil
		case ui.ViewProviderSelect:
			// Quit from provider select
			return m, tea.Quit
		}
		return m, nil

	case "r":
		// Refresh prices (only in provider select view)
		if m.GetView() == ui.ViewProviderSelect && !m.fetchingOffers {
			m.fetchingOffers = true
			m.SetStatusMessage("Refreshing prices...")
			return m, m.fetchOffersCmd()
		}
		return m, nil

	case "?":
		// Show help
		m.SetStatusMessage("Keys: q=quit, esc=back, ↑↓=navigate, Enter=select, r=refresh")
		return m, nil
	}

	// Forward to base model for navigation keys
	baseModel, cmd := m.Model.Update(msg)
	m.Model = baseModel.(ui.Model)
	return m, cmd
}

// View implements tea.Model.
func (m InteractiveModel) View() string {
	return m.Model.View()
}

// Message types for interactive mode

type deployStartMsg struct {
	deployCfg *deploy.DeployConfig
}

type quitInteractiveMsg struct{}

// Commands

// fetchOffersCmd creates a command that fetches offers from all providers.
func (m InteractiveModel) fetchOffersCmd() tea.Cmd {
	return func() tea.Msg {
		log := logging.Get()

		providers, err := registry.GetConfiguredProviders(m.cfg)
		if err != nil {
			log.Warn().Err(err).Msg("No providers configured")
			return ui.OffersLoadErrorMsg{Err: fmt.Errorf("no providers configured: %w", err)}
		}

		// Build filter based on deploy config
		filter := providerPkg.OfferFilter{}
		if m.deployCfg != nil {
			if m.deployCfg.GPUType != "" {
				filter.GPUType = m.deployCfg.GPUType
			}
			if m.deployCfg.Region != "" {
				filter.Region = m.deployCfg.Region
			}
			if !m.deployCfg.PreferSpot {
				filter.OnDemandOnly = true
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var allOffers []providerPkg.Offer
		for _, p := range providers {
			offers, err := p.GetOffers(ctx, filter)
			if err != nil {
				log.Warn().Err(err).Str("provider", p.Name()).Msg("Failed to fetch offers")
				continue
			}
			for _, o := range offers {
				if o.Available {
					allOffers = append(allOffers, o)
				}
			}
		}

		if len(allOffers) == 0 {
			return ui.OffersLoadErrorMsg{Err: fmt.Errorf("no offers available from any provider")}
		}

		log.Info().Int("count", len(allOffers)).Msg("Loaded offers")
		return ui.OffersLoadedMsg{Offers: allOffers}
	}
}

// startDeploymentCmd creates a command that starts the deployment process.
func (m InteractiveModel) startDeploymentCmd() tea.Cmd {
	return func() tea.Msg {
		// Build deploy config
		deployCfg := deploy.DefaultDeployConfig()
		if m.selectedModel != nil {
			deployCfg.Model = m.selectedModel.Name
		}
		if m.selectedOffer != nil {
			deployCfg.ProviderName = m.selectedOffer.Provider
			deployCfg.GPUType = m.selectedOffer.GPU
			deployCfg.Region = m.selectedOffer.Region
		}
		if m.deployCfg != nil {
			deployCfg.PreferSpot = m.deployCfg.PreferSpot
			deployCfg.DeadmanTimeoutHours = m.deployCfg.DeadmanTimeoutHours
		}

		return deployStartMsg{deployCfg: deployCfg}
	}
}

// runDeploymentCmd runs the actual deployment in the background.
func (m InteractiveModel) runDeploymentCmd(deployCfg *deploy.DeployConfig) tea.Cmd {
	return func() tea.Msg {
		log := logging.Get()

		// Create deployer
		deployer, err := deploy.NewDeployer(
			m.cfg,
			deployCfg,
			deploy.WithStateManager(m.stateManager),
			deploy.WithProgressCallback(func(p deploy.DeployProgress) {
				if m.program != nil {
					m.program.Send(ui.DeployProgressUpdateMsg{Progress: p})
				}
			}),
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create deployer")
			return ui.DeployProgressErrorMsg{Err: err}
		}

		// Run deployment
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		result, err := deployer.Deploy(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Deployment failed")
			return ui.DeployProgressErrorMsg{Err: err}
		}

		log.Info().
			Str("provider", result.Provider.Name()).
			Str("instance_id", result.Instance.ID).
			Dur("duration", result.Duration()).
			Msg("Deployment complete")

		// Convert to UI result
		uiResult := ui.ResultFromDeployResult(result, deployCfg.DeadmanTimeoutHours)
		return ui.DeployProgressCompleteMsg{Result: uiResult}
	}
}

// RunInteractiveMode starts the interactive TUI mode for when no instance is running.
// It handles the full flow: provider selection -> model selection -> deployment.
func RunInteractiveMode(cfg *config.Config, stateManager *config.StateManager, deployCfg *deploy.DeployConfig) error {
	log := logging.Get()
	log.Debug().Msg("Starting interactive mode (no active instance)")

	model := NewInteractiveModel(cfg, stateManager, deployCfg)

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

// CheckAndRunInteractive checks the state and runs the appropriate mode.
// If no instance is running, it starts interactive mode for deployment.
// If an instance is running, it shows the status view with actions.
// Returns true if interactive mode was started, false if error occurred before start.
func CheckAndRunInteractive() (bool, error) {
	log := logging.Get()

	// Load configuration
	cfg, warnings, err := config.LoadConfig("")
	if err != nil {
		return false, fmt.Errorf("failed to load config: %w", err)
	}

	// Log any warnings
	for _, w := range warnings {
		log.Warn().Msg(w)
	}

	// Check for at least one configured provider
	if !cfg.HasAnyProvider() {
		return false, fmt.Errorf("no providers configured - run 'continueplz init' first")
	}

	// Create state manager
	stateManager, err := config.NewStateManager("")
	if err != nil {
		return false, fmt.Errorf("failed to create state manager: %w", err)
	}

	// Load state to check for active instance
	state, err := stateManager.LoadState()
	if err != nil {
		// Corrupt state or locked - try to continue without state
		log.Warn().Err(err).Msg("Failed to load state")
	}

	// If there's an active instance, show status view with actions
	if state != nil && state.Instance != nil {
		log.Info().Str("instance_id", state.Instance.ID).Msg("Active instance found - showing status view")
		err = RunActiveInstanceMode(cfg, stateManager, state)
		return true, err
	}

	// No active instance - start interactive mode for deployment
	deployCfg := deploy.DefaultDeployConfig()

	// Apply command line flags to deploy config
	if model != "" {
		deployCfg.Model = model
	}
	if gpu != "" {
		deployCfg.GPUType = gpu
	}
	if provider != "" {
		deployCfg.ProviderName = provider
	}
	if region != "" {
		deployCfg.Region = region
	}
	if onDemand {
		deployCfg.PreferSpot = false
	} else {
		deployCfg.PreferSpot = spot
	}

	err = RunInteractiveMode(cfg, stateManager, deployCfg)
	return true, err
}
