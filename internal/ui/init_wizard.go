// Package ui provides TUI components for continueplz.
package ui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tmeurs/continueplz/internal/provider"
	"github.com/tmeurs/continueplz/internal/wireguard"
)

// ProviderInfo contains information about a provider for the init wizard.
type ProviderInfo struct {
	Name        string
	DisplayName string
	Description string
	APIKeyURL   string
}

// AllProviders contains all available providers for configuration.
var AllProviders = []ProviderInfo{
	{
		Name:        "vast",
		DisplayName: "Vast.ai",
		Description: "Marketplace for GPU rentals with spot pricing",
		APIKeyURL:   "https://cloud.vast.ai/account/",
	},
	{
		Name:        "lambda",
		DisplayName: "Lambda Labs",
		Description: "Cloud GPU instances (on-demand only)",
		APIKeyURL:   "https://cloud.lambdalabs.com/api-keys",
	},
	{
		Name:        "runpod",
		DisplayName: "RunPod",
		Description: "GPU cloud with spot and on-demand instances",
		APIKeyURL:   "https://www.runpod.io/console/user/settings",
	},
	{
		Name:        "coreweave",
		DisplayName: "CoreWeave",
		Description: "Kubernetes-based GPU cloud",
		APIKeyURL:   "https://cloud.coreweave.com/api-access",
	},
	{
		Name:        "paperspace",
		DisplayName: "Paperspace",
		Description: "GPU cloud (no billing API verification)",
		APIKeyURL:   "https://console.paperspace.com/settings/apikeys",
	},
}

// APIKeyResult contains the result of validating an API key.
type APIKeyResult struct {
	Provider    string
	Valid       bool
	AccountInfo *provider.AccountInfo
	Error       error
}

// ProviderFactory creates provider instances for validation.
// This allows the init wizard to validate API keys without creating a full config.
type ProviderFactory func(providerName, apiKey string) (provider.Provider, error)

// TierOption represents a model tier option for preferences.
type TierOption struct {
	Name        string
	DisplayName string
	Description string
}

// AllTiers contains all available tier options.
var AllTiers = []TierOption{
	{Name: "small", DisplayName: "Small", Description: "7B models, fast, less capable"},
	{Name: "medium", DisplayName: "Medium", Description: "14-34B models, balanced (recommended)"},
	{Name: "large", DisplayName: "Large", Description: "70B+ models, most capable, expensive"},
}

// WireGuardChoice represents user's choice for WireGuard key handling.
type WireGuardChoice int

const (
	WireGuardChoiceGenerate WireGuardChoice = iota
	WireGuardChoiceExisting
)

// InitWizardModel represents the init wizard TUI model.
type InitWizardModel struct {
	// Current step in the wizard
	step InitWizardStep

	// Provider selection
	providers      []ProviderInfo
	selected       map[string]bool
	cursor         int
	providerErrors map[string]error

	// API key input (F040)
	apiKeyInputs       map[string]string        // provider -> API key input
	apiKeyResults      map[string]*APIKeyResult // provider -> validation result
	apiKeyValidating   string                   // provider currently being validated
	currentAPIKeyIndex int                      // index in selectedProvidersList
	apiKeyTextInput    string                   // current text being typed
	apiKeyCursorPos    int                      // cursor position in text input
	providerFactory    ProviderFactory          // factory for creating providers

	// WireGuard setup (F041)
	wireGuardChoice     WireGuardChoice // generate new or use existing
	wireGuardKeyPair    *wireguard.KeyPair
	wireGuardTextInput  string // for existing key input
	wireGuardCursorPos  int
	wireGuardValidating bool
	wireGuardError      error

	// Preferences (F041)
	preferencesCursor   int    // cursor for tier selection
	selectedTier        string // small, medium, large
	deadmanTimeoutInput string // text input for deadman timeout
	deadmanCursorPos    int
	preferenceSubStep   int // 0 = tier, 1 = deadman timeout

	// Saving state (F041)
	savingConfig bool
	saveError    error

	// Terminal dimensions
	width  int
	height int

	// State
	quitting bool
	done     bool
	err      error
}

// InitWizardStep represents the different steps in the init wizard.
type InitWizardStep int

const (
	// StepProviderSelect is the provider selection step.
	StepProviderSelect InitWizardStep = iota
	// StepAPIKeyInput is the API key input step (handled by F040).
	StepAPIKeyInput
	// StepWireGuard is the WireGuard setup step (handled by F041).
	StepWireGuard
	// StepPreferences is the preferences setup step (handled by F041).
	StepPreferences
	// StepComplete is the wizard completion step.
	StepComplete
)

// String returns the string representation of the step.
func (s InitWizardStep) String() string {
	switch s {
	case StepProviderSelect:
		return "provider_select"
	case StepAPIKeyInput:
		return "api_key_input"
	case StepWireGuard:
		return "wireguard"
	case StepPreferences:
		return "preferences"
	case StepComplete:
		return "complete"
	default:
		return "unknown"
	}
}

// NewInitWizardModel creates a new init wizard model.
func NewInitWizardModel() InitWizardModel {
	return InitWizardModel{
		step:                StepProviderSelect,
		providers:           AllProviders,
		selected:            make(map[string]bool),
		providerErrors:      make(map[string]error),
		cursor:              0,
		apiKeyInputs:        make(map[string]string),
		apiKeyResults:       make(map[string]*APIKeyResult),
		wireGuardChoice:     WireGuardChoiceGenerate,
		selectedTier:        "medium",
		deadmanTimeoutInput: "10",
	}
}

// NewInitWizardModelWithFactory creates a new init wizard model with a custom provider factory.
func NewInitWizardModelWithFactory(factory ProviderFactory) InitWizardModel {
	m := NewInitWizardModel()
	m.providerFactory = factory
	return m
}

// SetProviderFactory sets the provider factory for API key validation.
func (m *InitWizardModel) SetProviderFactory(factory ProviderFactory) {
	m.providerFactory = factory
}

// Init implements tea.Model.
func (m InitWizardModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m InitWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case InitWizardProvidersSelectedMsg:
		// Providers were selected, move to next step
		m.step = StepAPIKeyInput
		m.currentAPIKeyIndex = 0
		m.apiKeyTextInput = ""
		m.apiKeyCursorPos = 0
		return m, nil
	case APIKeyValidationStartMsg:
		// Start validating the API key
		m.apiKeyValidating = msg.Provider
		return m, m.validateAPIKey(msg.Provider, msg.APIKey)
	case APIKeyValidationResultMsg:
		// Received validation result
		m.apiKeyValidating = ""
		m.apiKeyResults[msg.Provider] = &APIKeyResult{
			Provider:    msg.Provider,
			Valid:       msg.Valid,
			AccountInfo: msg.AccountInfo,
			Error:       msg.Error,
		}
		if msg.Valid {
			// Move to next provider or complete API key step
			m.currentAPIKeyIndex++
			m.apiKeyTextInput = ""
			m.apiKeyCursorPos = 0
			if m.currentAPIKeyIndex >= len(m.SelectedProviders()) {
				// All providers validated, move to WireGuard step
				m.step = StepWireGuard
			}
		}
		return m, nil
	case WireGuardKeyGeneratedMsg:
		// WireGuard key was generated
		m.wireGuardValidating = false
		if msg.Error != nil {
			m.wireGuardError = msg.Error
		} else {
			m.wireGuardKeyPair = msg.KeyPair
			// Move to preferences step
			m.step = StepPreferences
			m.preferencesCursor = 0
			m.preferenceSubStep = 0
		}
		return m, nil
	case WireGuardKeyValidatedMsg:
		// Existing WireGuard key was validated
		m.wireGuardValidating = false
		if msg.Error != nil {
			m.wireGuardError = msg.Error
		} else {
			m.wireGuardKeyPair = msg.KeyPair
			// Move to preferences step
			m.step = StepPreferences
			m.preferencesCursor = 0
			m.preferenceSubStep = 0
		}
		return m, nil
	case ConfigSavedMsg:
		// Configuration was saved
		m.savingConfig = false
		if msg.Error != nil {
			m.saveError = msg.Error
		} else {
			m.step = StepComplete
			m.done = true
		}
		return m, nil
	}
	return m, nil
}

// handleKeyPress handles keyboard input for the init wizard.
func (m InitWizardModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.step {
	case StepProviderSelect:
		return m.handleProviderSelectKey(msg)
	case StepAPIKeyInput:
		return m.handleAPIKeyInputKey(msg)
	case StepWireGuard:
		return m.handleWireGuardKey(msg)
	case StepPreferences:
		return m.handlePreferencesKey(msg)
	case StepComplete:
		return m.handleCompleteKey(msg)
	}
	return m, nil
}

// handleAPIKeyInputKey handles keyboard input during API key input.
func (m InitWizardModel) handleAPIKeyInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If currently validating, ignore input
	if m.apiKeyValidating != "" {
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		// Go back to provider selection
		m.step = StepProviderSelect
		m.done = false
		return m, nil

	case "enter":
		// Submit the API key for validation
		if len(m.apiKeyTextInput) > 0 {
			providers := m.SelectedProviders()
			if m.currentAPIKeyIndex < len(providers) {
				providerName := providers[m.currentAPIKeyIndex]
				m.apiKeyInputs[providerName] = m.apiKeyTextInput
				return m, func() tea.Msg {
					return APIKeyValidationStartMsg{
						Provider: providerName,
						APIKey:   m.apiKeyTextInput,
					}
				}
			}
		}

	case "backspace":
		if m.apiKeyCursorPos > 0 && len(m.apiKeyTextInput) > 0 {
			// Delete character before cursor
			m.apiKeyTextInput = m.apiKeyTextInput[:m.apiKeyCursorPos-1] + m.apiKeyTextInput[m.apiKeyCursorPos:]
			m.apiKeyCursorPos--
		}

	case "delete":
		if m.apiKeyCursorPos < len(m.apiKeyTextInput) {
			// Delete character at cursor
			m.apiKeyTextInput = m.apiKeyTextInput[:m.apiKeyCursorPos] + m.apiKeyTextInput[m.apiKeyCursorPos+1:]
		}

	case "left":
		if m.apiKeyCursorPos > 0 {
			m.apiKeyCursorPos--
		}

	case "right":
		if m.apiKeyCursorPos < len(m.apiKeyTextInput) {
			m.apiKeyCursorPos++
		}

	case "home", "ctrl+a":
		m.apiKeyCursorPos = 0

	case "end", "ctrl+e":
		m.apiKeyCursorPos = len(m.apiKeyTextInput)

	case "ctrl+u":
		// Clear input
		m.apiKeyTextInput = ""
		m.apiKeyCursorPos = 0

	default:
		// Only accept printable characters
		if len(msg.String()) == 1 {
			char := msg.String()[0]
			if char >= 32 && char <= 126 {
				// Insert character at cursor position
				m.apiKeyTextInput = m.apiKeyTextInput[:m.apiKeyCursorPos] + msg.String() + m.apiKeyTextInput[m.apiKeyCursorPos:]
				m.apiKeyCursorPos++
			}
		}
	}

	return m, nil
}

// handleProviderSelectKey handles keyboard input during provider selection.
func (m InitWizardModel) handleProviderSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.providers)-1 {
			m.cursor++
		}

	case " ", "x":
		// Toggle selection
		provider := m.providers[m.cursor].Name
		m.selected[provider] = !m.selected[provider]

	case "a":
		// Select all
		for _, p := range m.providers {
			m.selected[p.Name] = true
		}

	case "n":
		// Deselect all
		for k := range m.selected {
			delete(m.selected, k)
		}

	case "enter":
		// Confirm selection if at least one provider is selected
		if m.HasSelectedProviders() {
			m.done = true
			return m, func() tea.Msg {
				return InitWizardProvidersSelectedMsg{
					Providers: m.SelectedProviders(),
				}
			}
		}
	}

	return m, nil
}

// handleWireGuardKey handles keyboard input during WireGuard setup.
func (m InitWizardModel) handleWireGuardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If validating/generating, ignore input
	if m.wireGuardValidating {
		return m, nil
	}

	// If entering existing key
	if m.wireGuardChoice == WireGuardChoiceExisting && m.wireGuardKeyPair == nil {
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			// Go back to choice selection
			m.wireGuardChoice = WireGuardChoiceGenerate
			m.wireGuardTextInput = ""
			m.wireGuardCursorPos = 0
			m.wireGuardError = nil
			return m, nil

		case "enter":
			// Validate the existing key
			if len(m.wireGuardTextInput) > 0 {
				m.wireGuardValidating = true
				m.wireGuardError = nil
				return m, m.validateExistingWireGuardKey(m.wireGuardTextInput)
			}

		case "backspace":
			if m.wireGuardCursorPos > 0 && len(m.wireGuardTextInput) > 0 {
				m.wireGuardTextInput = m.wireGuardTextInput[:m.wireGuardCursorPos-1] + m.wireGuardTextInput[m.wireGuardCursorPos:]
				m.wireGuardCursorPos--
			}

		case "delete":
			if m.wireGuardCursorPos < len(m.wireGuardTextInput) {
				m.wireGuardTextInput = m.wireGuardTextInput[:m.wireGuardCursorPos] + m.wireGuardTextInput[m.wireGuardCursorPos+1:]
			}

		case "left":
			if m.wireGuardCursorPos > 0 {
				m.wireGuardCursorPos--
			}

		case "right":
			if m.wireGuardCursorPos < len(m.wireGuardTextInput) {
				m.wireGuardCursorPos++
			}

		case "ctrl+u":
			m.wireGuardTextInput = ""
			m.wireGuardCursorPos = 0

		default:
			// Accept printable characters (base64 keys)
			if len(msg.String()) == 1 {
				char := msg.String()[0]
				if char >= 32 && char <= 126 {
					m.wireGuardTextInput = m.wireGuardTextInput[:m.wireGuardCursorPos] + msg.String() + m.wireGuardTextInput[m.wireGuardCursorPos:]
					m.wireGuardCursorPos++
				}
			}
		}
		return m, nil
	}

	// Choice selection (generate or use existing)
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		// Go back to API key step
		m.step = StepAPIKeyInput
		return m, nil

	case "up", "k":
		if m.wireGuardChoice > WireGuardChoiceGenerate {
			m.wireGuardChoice--
		}

	case "down", "j":
		if m.wireGuardChoice < WireGuardChoiceExisting {
			m.wireGuardChoice++
		}

	case "enter":
		if m.wireGuardChoice == WireGuardChoiceGenerate {
			// Generate new key pair
			m.wireGuardValidating = true
			m.wireGuardError = nil
			return m, m.generateWireGuardKey()
		} else {
			// Switch to existing key input mode (stay on this step)
			m.wireGuardTextInput = ""
			m.wireGuardCursorPos = 0
		}
	}

	return m, nil
}

// handlePreferencesKey handles keyboard input during preferences setup.
func (m InitWizardModel) handlePreferencesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If saving config, ignore input
	if m.savingConfig {
		return m, nil
	}

	switch m.preferenceSubStep {
	case 0: // Tier selection
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			// Go back to WireGuard step
			m.step = StepWireGuard
			return m, nil

		case "up", "k":
			if m.preferencesCursor > 0 {
				m.preferencesCursor--
			}

		case "down", "j":
			if m.preferencesCursor < len(AllTiers)-1 {
				m.preferencesCursor++
			}

		case "enter", " ":
			// Select tier and move to deadman timeout
			m.selectedTier = AllTiers[m.preferencesCursor].Name
			m.preferenceSubStep = 1
		}

	case 1: // Deadman timeout input
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			// Go back to tier selection
			m.preferenceSubStep = 0
			return m, nil

		case "enter":
			// Validate and save config
			timeout, err := strconv.Atoi(m.deadmanTimeoutInput)
			if err != nil || timeout < 1 || timeout > 168 {
				// Invalid, show error but stay on step
				m.saveError = fmt.Errorf("timeout must be 1-168 hours")
				return m, nil
			}
			m.saveError = nil
			m.savingConfig = true
			return m, m.saveConfig()

		case "backspace":
			if m.deadmanCursorPos > 0 && len(m.deadmanTimeoutInput) > 0 {
				m.deadmanTimeoutInput = m.deadmanTimeoutInput[:m.deadmanCursorPos-1] + m.deadmanTimeoutInput[m.deadmanCursorPos:]
				m.deadmanCursorPos--
			}

		case "delete":
			if m.deadmanCursorPos < len(m.deadmanTimeoutInput) {
				m.deadmanTimeoutInput = m.deadmanTimeoutInput[:m.deadmanCursorPos] + m.deadmanTimeoutInput[m.deadmanCursorPos+1:]
			}

		case "left":
			if m.deadmanCursorPos > 0 {
				m.deadmanCursorPos--
			}

		case "right":
			if m.deadmanCursorPos < len(m.deadmanTimeoutInput) {
				m.deadmanCursorPos++
			}

		case "ctrl+u":
			m.deadmanTimeoutInput = ""
			m.deadmanCursorPos = 0

		default:
			// Only accept digits
			if len(msg.String()) == 1 {
				char := msg.String()[0]
				if char >= '0' && char <= '9' {
					m.deadmanTimeoutInput = m.deadmanTimeoutInput[:m.deadmanCursorPos] + msg.String() + m.deadmanTimeoutInput[m.deadmanCursorPos:]
					m.deadmanCursorPos++
				}
			}
		}
	}

	return m, nil
}

// handleCompleteKey handles keyboard input on the completion screen.
func (m InitWizardModel) handleCompleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "enter":
		m.quitting = true
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

// generateWireGuardKey creates a command to generate a new WireGuard key pair.
func (m InitWizardModel) generateWireGuardKey() tea.Cmd {
	return func() tea.Msg {
		keyPair, err := wireguard.GenerateKeyPair()
		if err != nil {
			return WireGuardKeyGeneratedMsg{Error: err}
		}
		return WireGuardKeyGeneratedMsg{KeyPair: keyPair}
	}
}

// validateExistingWireGuardKey creates a command to validate an existing private key.
func (m InitWizardModel) validateExistingWireGuardKey(privateKey string) tea.Cmd {
	return func() tea.Msg {
		// Validate and derive public key
		keyPair, err := wireguard.KeyPairFromPrivate(privateKey)
		if err != nil {
			return WireGuardKeyValidatedMsg{Error: err}
		}
		return WireGuardKeyValidatedMsg{KeyPair: keyPair}
	}
}

// saveConfig creates a command to save the configuration to .env file.
func (m InitWizardModel) saveConfig() tea.Cmd {
	return func() tea.Msg {
		// Build the .env content
		var content strings.Builder

		content.WriteString("# continueplz Configuration\n")
		content.WriteString("# Generated by continueplz init\n")
		content.WriteString("# IMPORTANT: Keep this file secure (permissions should be 0600)\n\n")

		// Provider API Keys
		content.WriteString("# Provider API Keys\n")
		validatedKeys := m.GetValidatedAPIKeys()

		// Write all provider keys (empty for unconfigured)
		for _, p := range AllProviders {
			envKey := strings.ToUpper(p.Name) + "_API_KEY"
			if p.Name == "vast" {
				envKey = "VAST_API_KEY"
			} else if p.Name == "lambda" {
				envKey = "LAMBDA_API_KEY"
			} else if p.Name == "runpod" {
				envKey = "RUNPOD_API_KEY"
			} else if p.Name == "coreweave" {
				envKey = "COREWEAVE_API_KEY"
			} else if p.Name == "paperspace" {
				envKey = "PAPERSPACE_API_KEY"
			}

			if key, ok := validatedKeys[p.Name]; ok {
				content.WriteString(fmt.Sprintf("%s=%s\n", envKey, key))
			} else {
				content.WriteString(fmt.Sprintf("%s=\n", envKey))
			}
		}

		content.WriteString("\n")

		// WireGuard keys
		content.WriteString("# WireGuard Keys\n")
		if m.wireGuardKeyPair != nil {
			content.WriteString(fmt.Sprintf("WIREGUARD_PRIVATE_KEY=%s\n", m.wireGuardKeyPair.PrivateKey))
			content.WriteString(fmt.Sprintf("WIREGUARD_PUBLIC_KEY=%s\n", m.wireGuardKeyPair.PublicKey))
		} else {
			content.WriteString("WIREGUARD_PRIVATE_KEY=\n")
			content.WriteString("WIREGUARD_PUBLIC_KEY=\n")
		}

		content.WriteString("\n")

		// Preferences
		content.WriteString("# Preferences\n")
		content.WriteString(fmt.Sprintf("DEFAULT_TIER=%s\n", m.selectedTier))
		content.WriteString("DEFAULT_REGION=eu-west\n")
		content.WriteString("PREFER_SPOT=true\n")
		content.WriteString(fmt.Sprintf("DEADMAN_TIMEOUT_HOURS=%s\n", m.deadmanTimeoutInput))

		content.WriteString("\n")

		// Alerting (optional, leave empty)
		content.WriteString("# Alerting (optional)\n")
		content.WriteString("ALERT_WEBHOOK_URL=\n")
		content.WriteString("DAILY_BUDGET_EUR=20\n")

		// Write to file with 0600 permissions
		err := os.WriteFile(".env", []byte(content.String()), 0o600)
		if err != nil {
			return ConfigSavedMsg{Error: fmt.Errorf("failed to write .env file: %w", err)}
		}

		return ConfigSavedMsg{}
	}
}

// View implements tea.Model.
func (m InitWizardModel) View() string {
	if m.quitting {
		return "Setup cancelled.\n"
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")

	// Content based on current step
	switch m.step {
	case StepProviderSelect:
		b.WriteString(m.renderProviderSelect())
	case StepAPIKeyInput:
		b.WriteString(m.renderAPIKeyInput())
	case StepWireGuard:
		b.WriteString(m.renderWireGuard())
	case StepPreferences:
		b.WriteString(m.renderPreferences())
	case StepComplete:
		b.WriteString(m.renderComplete())
	}

	// Footer with key hints
	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// renderHeader renders the wizard header.
func (m InitWizardModel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Align(lipgloss.Center)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1).
		Align(lipgloss.Center)

	width := m.width
	if width == 0 {
		width = 60
	}

	title := titleStyle.Render("continueplz setup")
	return boxStyle.Width(width - 2).Render(title)
}

// renderProviderSelect renders the provider selection view.
func (m InitWizardModel) renderProviderSelect() string {
	var b strings.Builder

	// Prompt
	promptStyle := Styles.Body
	b.WriteString(promptStyle.Render("Let's configure your GPU providers."))
	b.WriteString("\n\n")

	// Question
	questionStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)
	b.WriteString(questionStyle.Render("? Which providers do you want to configure?"))
	b.WriteString("\n")

	// Provider list
	for i, provider := range m.providers {
		var line string

		// Cursor indicator
		if i == m.cursor {
			line += Styles.Cursor.Render("> ")
		} else {
			line += "  "
		}

		// Checkbox
		if m.selected[provider.Name] {
			line += Styles.Success.Render("[x] ")
		} else {
			line += Styles.Muted.Render("[ ] ")
		}

		// Provider name
		nameStyle := Styles.Body
		if i == m.cursor {
			nameStyle = Styles.Highlighted
		}
		line += nameStyle.Render(provider.DisplayName)

		// Description (dimmed)
		line += Styles.Muted.Render(fmt.Sprintf(" - %s", provider.Description))

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Selection count
	b.WriteString("\n")
	count := len(m.SelectedProviders())
	if count == 0 {
		b.WriteString(Styles.Warning.Render("  No providers selected"))
	} else {
		b.WriteString(Styles.Success.Render(fmt.Sprintf("  %d provider(s) selected", count)))
	}

	return b.String()
}

// renderAPIKeyInput renders the API key input view.
func (m InitWizardModel) renderAPIKeyInput() string {
	var b strings.Builder

	providers := m.SelectedProviders()
	if m.currentAPIKeyIndex >= len(providers) {
		b.WriteString(Styles.Success.Render("All API keys validated!\n"))
		return b.String()
	}

	currentProvider := providers[m.currentAPIKeyIndex]

	// Find provider info
	var providerInfo ProviderInfo
	for _, p := range m.providers {
		if p.Name == currentProvider {
			providerInfo = p
			break
		}
	}

	// Progress indicator
	progressStyle := Styles.Muted
	b.WriteString(progressStyle.Render(fmt.Sprintf("Provider %d of %d", m.currentAPIKeyIndex+1, len(providers))))
	b.WriteString("\n\n")

	// Provider name
	providerStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)
	b.WriteString(providerStyle.Render(fmt.Sprintf("? Enter API key for %s:", providerInfo.DisplayName)))
	b.WriteString("\n")

	// Show API key URL hint
	b.WriteString(Styles.Muted.Render(fmt.Sprintf("  Get your API key at: %s", providerInfo.APIKeyURL)))
	b.WriteString("\n\n")

	// Show validation status or input field
	if m.apiKeyValidating == currentProvider {
		// Show loading spinner
		b.WriteString("  ")
		b.WriteString(Styles.Spinner.Render(IconSpinner))
		b.WriteString(" ")
		b.WriteString(Styles.Info.Render("Validating API key..."))
		b.WriteString("\n")
	} else {
		// Show text input field
		inputStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1).
			Width(50)

		// Show masked input (show only last 4 chars for security)
		maskedInput := m.getMaskedAPIKey()
		inputBox := inputStyle.Render(maskedInput + "_")
		b.WriteString("  ")
		b.WriteString(inputBox)
		b.WriteString("\n")
	}

	// Show result for current provider if we have one
	if result, ok := m.apiKeyResults[currentProvider]; ok {
		b.WriteString("\n")
		if result.Valid {
			b.WriteString("  ")
			b.WriteString(Styles.Success.Render(IconSuccess + " API key valid!"))
			b.WriteString("\n")
			if result.AccountInfo != nil {
				m.renderAccountInfo(&b, result.AccountInfo)
			}
		} else {
			b.WriteString("  ")
			b.WriteString(Styles.Error.Render(IconError + " Invalid API key"))
			b.WriteString("\n")
			if result.Error != nil {
				b.WriteString("  ")
				b.WriteString(Styles.Muted.Render(result.Error.Error()))
				b.WriteString("\n")
			}
		}
	}

	// Show completed providers
	if m.currentAPIKeyIndex > 0 {
		b.WriteString("\n")
		b.WriteString(Styles.Muted.Render("Completed:"))
		b.WriteString("\n")
		for i := 0; i < m.currentAPIKeyIndex; i++ {
			provName := providers[i]
			if result, ok := m.apiKeyResults[provName]; ok && result.Valid {
				// Find display name
				displayName := provName
				for _, p := range m.providers {
					if p.Name == provName {
						displayName = p.DisplayName
						break
					}
				}
				b.WriteString("  ")
				b.WriteString(Styles.Success.Render(IconSuccess))
				b.WriteString(" ")
				b.WriteString(Styles.Body.Render(displayName))
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

// getMaskedAPIKey returns the API key with most characters masked for security.
func (m InitWizardModel) getMaskedAPIKey() string {
	if len(m.apiKeyTextInput) == 0 {
		return ""
	}
	if len(m.apiKeyTextInput) <= 4 {
		return m.apiKeyTextInput
	}
	// Show only last 4 characters
	masked := strings.Repeat("*", len(m.apiKeyTextInput)-4) + m.apiKeyTextInput[len(m.apiKeyTextInput)-4:]
	return masked
}

// renderAccountInfo renders the account information from validation.
func (m InitWizardModel) renderAccountInfo(b *strings.Builder, info *provider.AccountInfo) {
	if info.Email != "" {
		b.WriteString("    ")
		b.WriteString(Styles.Muted.Render("Email: "))
		b.WriteString(Styles.Body.Render(info.Email))
		b.WriteString("\n")
	}
	if info.Username != "" {
		b.WriteString("    ")
		b.WriteString(Styles.Muted.Render("Account: "))
		b.WriteString(Styles.Body.Render(info.Username))
		b.WriteString("\n")
	}
	if info.Balance != nil {
		b.WriteString("    ")
		b.WriteString(Styles.Muted.Render("Balance: "))
		balanceStr := fmt.Sprintf("%.2f %s", *info.Balance, info.BalanceCurrency)
		if *info.Balance > 0 {
			b.WriteString(Styles.Success.Render(balanceStr))
		} else {
			b.WriteString(Styles.Warning.Render(balanceStr))
		}
		b.WriteString("\n")
	}
}

// validateAPIKey creates a command to validate an API key.
func (m InitWizardModel) validateAPIKey(providerName, apiKey string) tea.Cmd {
	return func() tea.Msg {
		// Use the provider factory if available
		if m.providerFactory == nil {
			return APIKeyValidationResultMsg{
				Provider: providerName,
				Valid:    false,
				Error:    fmt.Errorf("no provider factory configured"),
			}
		}

		// Create provider instance
		p, err := m.providerFactory(providerName, apiKey)
		if err != nil {
			return APIKeyValidationResultMsg{
				Provider: providerName,
				Valid:    false,
				Error:    err,
			}
		}

		// Validate the API key
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		info, err := p.ValidateAPIKey(ctx)
		if err != nil {
			return APIKeyValidationResultMsg{
				Provider: providerName,
				Valid:    false,
				Error:    err,
			}
		}

		return APIKeyValidationResultMsg{
			Provider:    providerName,
			Valid:       info != nil && info.Valid,
			AccountInfo: info,
			Error:       nil,
		}
	}
}

// renderWireGuard renders the WireGuard setup view.
func (m InitWizardModel) renderWireGuard() string {
	var b strings.Builder

	// Progress indicator
	b.WriteString(Styles.Muted.Render("Step 3 of 4: WireGuard Setup"))
	b.WriteString("\n\n")

	questionStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	// If validating, show spinner
	if m.wireGuardValidating {
		b.WriteString("  ")
		b.WriteString(Styles.Spinner.Render(IconSpinner))
		b.WriteString(" ")
		if m.wireGuardChoice == WireGuardChoiceGenerate {
			b.WriteString(Styles.Info.Render("Generating WireGuard key pair..."))
		} else {
			b.WriteString(Styles.Info.Render("Validating WireGuard key..."))
		}
		b.WriteString("\n")
		return b.String()
	}

	// If entering existing key
	if m.wireGuardChoice == WireGuardChoiceExisting && m.wireGuardKeyPair == nil {
		b.WriteString(questionStyle.Render("? Enter your existing WireGuard private key:"))
		b.WriteString("\n\n")

		// Text input
		inputStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1).
			Width(50)

		inputBox := inputStyle.Render(m.wireGuardTextInput + "_")
		b.WriteString("  ")
		b.WriteString(inputBox)
		b.WriteString("\n")

		// Show error if any
		if m.wireGuardError != nil {
			b.WriteString("\n  ")
			b.WriteString(Styles.Error.Render(IconError + " " + m.wireGuardError.Error()))
			b.WriteString("\n")
		}

		return b.String()
	}

	// Choice selection
	b.WriteString(questionStyle.Render("? How would you like to set up WireGuard?"))
	b.WriteString("\n\n")

	choices := []struct {
		choice      WireGuardChoice
		name        string
		description string
	}{
		{WireGuardChoiceGenerate, "Generate new keys", "Create a new WireGuard key pair (recommended)"},
		{WireGuardChoiceExisting, "Use existing key", "Enter an existing WireGuard private key"},
	}

	for _, c := range choices {
		var line string

		// Cursor
		if m.wireGuardChoice == c.choice {
			line += Styles.Cursor.Render("> ")
		} else {
			line += "  "
		}

		// Radio button
		if m.wireGuardChoice == c.choice {
			line += Styles.Success.Render("(●) ")
		} else {
			line += Styles.Muted.Render("( ) ")
		}

		// Name and description
		nameStyle := Styles.Body
		if m.wireGuardChoice == c.choice {
			nameStyle = Styles.Highlighted
		}
		line += nameStyle.Render(c.name)
		line += Styles.Muted.Render(" - " + c.description)

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Show error if any
	if m.wireGuardError != nil {
		b.WriteString("\n  ")
		b.WriteString(Styles.Error.Render(IconError + " " + m.wireGuardError.Error()))
		b.WriteString("\n")
	}

	return b.String()
}

// renderPreferences renders the preferences setup view.
func (m InitWizardModel) renderPreferences() string {
	var b strings.Builder

	// Progress indicator
	b.WriteString(Styles.Muted.Render("Step 4 of 4: Preferences"))
	b.WriteString("\n\n")

	questionStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	// If saving config, show spinner
	if m.savingConfig {
		b.WriteString("  ")
		b.WriteString(Styles.Spinner.Render(IconSpinner))
		b.WriteString(" ")
		b.WriteString(Styles.Info.Render("Saving configuration..."))
		b.WriteString("\n")
		return b.String()
	}

	switch m.preferenceSubStep {
	case 0: // Tier selection
		b.WriteString(questionStyle.Render("? What is your default model tier?"))
		b.WriteString("\n\n")

		for i, tier := range AllTiers {
			var line string

			// Cursor
			if i == m.preferencesCursor {
				line += Styles.Cursor.Render("> ")
			} else {
				line += "  "
			}

			// Radio button
			if i == m.preferencesCursor {
				line += Styles.Success.Render("(●) ")
			} else {
				line += Styles.Muted.Render("( ) ")
			}

			// Name and description
			nameStyle := Styles.Body
			if i == m.preferencesCursor {
				nameStyle = Styles.Highlighted
			}
			line += nameStyle.Render(tier.DisplayName)
			line += Styles.Muted.Render(" - " + tier.Description)

			b.WriteString(line)
			b.WriteString("\n")
		}

	case 1: // Deadman timeout
		b.WriteString(questionStyle.Render("? Deadman timeout (hours without heartbeat before auto-termination):"))
		b.WriteString("\n")
		b.WriteString(Styles.Muted.Render("  (Enter 1-168 hours, default: 10)"))
		b.WriteString("\n\n")

		// Text input
		inputStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1).
			Width(10)

		inputBox := inputStyle.Render(m.deadmanTimeoutInput + "_")
		b.WriteString("  ")
		b.WriteString(inputBox)
		b.WriteString(" hours")
		b.WriteString("\n")

		// Show selected tier
		b.WriteString("\n")
		b.WriteString(Styles.Muted.Render("  Selected tier: "))
		b.WriteString(Styles.Success.Render(m.selectedTier))
		b.WriteString("\n")
	}

	// Show error if any
	if m.saveError != nil {
		b.WriteString("\n  ")
		b.WriteString(Styles.Error.Render(IconError + " " + m.saveError.Error()))
		b.WriteString("\n")
	}

	return b.String()
}

// renderComplete renders the completion view.
func (m InitWizardModel) renderComplete() string {
	var b strings.Builder

	// Success message
	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSuccess)

	b.WriteString(successStyle.Render(IconSuccess + " Configuration complete!"))
	b.WriteString("\n\n")

	// Summary
	b.WriteString(Styles.Body.Render("Your configuration has been saved to "))
	b.WriteString(Styles.Highlighted.Render(".env"))
	b.WriteString("\n\n")

	// What was configured
	b.WriteString(Styles.Muted.Render("Configured:"))
	b.WriteString("\n")

	// Providers
	validatedKeys := m.GetValidatedAPIKeys()
	b.WriteString("  ")
	b.WriteString(Styles.Success.Render(IconSuccess))
	b.WriteString(fmt.Sprintf(" %d provider(s): ", len(validatedKeys)))
	var providerNames []string
	for _, p := range AllProviders {
		if _, ok := validatedKeys[p.Name]; ok {
			providerNames = append(providerNames, p.DisplayName)
		}
	}
	b.WriteString(Styles.Body.Render(strings.Join(providerNames, ", ")))
	b.WriteString("\n")

	// WireGuard
	b.WriteString("  ")
	b.WriteString(Styles.Success.Render(IconSuccess))
	b.WriteString(" WireGuard keys configured")
	b.WriteString("\n")

	// Preferences
	b.WriteString("  ")
	b.WriteString(Styles.Success.Render(IconSuccess))
	b.WriteString(fmt.Sprintf(" Default tier: %s", m.selectedTier))
	b.WriteString("\n")

	b.WriteString("  ")
	b.WriteString(Styles.Success.Render(IconSuccess))
	b.WriteString(fmt.Sprintf(" Deadman timeout: %s hours", m.deadmanTimeoutInput))
	b.WriteString("\n")

	// Next steps
	b.WriteString("\n")
	b.WriteString(Styles.Muted.Render("Next steps:"))
	b.WriteString("\n")
	b.WriteString("  1. Run ")
	b.WriteString(Styles.Highlighted.Render("continueplz"))
	b.WriteString(" to start a GPU instance\n")
	b.WriteString("  2. The cheapest available option will be shown\n")
	b.WriteString("\n")
	b.WriteString(Styles.Muted.Render("Press Enter or q to exit"))

	return b.String()
}

// renderFooter renders the footer with key hints.
func (m InitWizardModel) renderFooter() string {
	var hints []string

	switch m.step {
	case StepProviderSelect:
		hints = []string{
			"↑/↓: navigate",
			"space: toggle",
			"a: select all",
			"n: deselect all",
			"enter: confirm",
			"q: quit",
		}
	case StepAPIKeyInput:
		hints = []string{
			"enter: validate",
			"esc: back",
			"ctrl+u: clear",
			"ctrl+c: quit",
		}
	case StepWireGuard:
		if m.wireGuardChoice == WireGuardChoiceExisting && m.wireGuardKeyPair == nil {
			hints = []string{
				"enter: validate",
				"esc: back",
				"ctrl+u: clear",
				"ctrl+c: quit",
			}
		} else {
			hints = []string{
				"↑/↓: navigate",
				"enter: confirm",
				"esc: back",
				"q: quit",
			}
		}
	case StepPreferences:
		if m.preferenceSubStep == 0 {
			hints = []string{
				"↑/↓: navigate",
				"enter: select",
				"esc: back",
				"q: quit",
			}
		} else {
			hints = []string{
				"enter: save",
				"esc: back",
				"ctrl+c: quit",
			}
		}
	case StepComplete:
		hints = []string{
			"enter: exit",
			"q: exit",
		}
	default:
		hints = []string{
			"enter: continue",
			"esc: back",
			"ctrl+c: quit",
		}
	}

	hintStyle := Styles.Muted
	return hintStyle.Render(strings.Join(hints, "  |  "))
}

// HasSelectedProviders returns true if at least one provider is selected.
func (m InitWizardModel) HasSelectedProviders() bool {
	for _, selected := range m.selected {
		if selected {
			return true
		}
	}
	return false
}

// SelectedProviders returns the list of selected provider names.
func (m InitWizardModel) SelectedProviders() []string {
	var providers []string
	for _, p := range m.providers {
		if m.selected[p.Name] {
			providers = append(providers, p.Name)
		}
	}
	return providers
}

// SelectedProviderInfos returns the list of selected provider infos.
func (m InitWizardModel) SelectedProviderInfos() []ProviderInfo {
	var providers []ProviderInfo
	for _, p := range m.providers {
		if m.selected[p.Name] {
			providers = append(providers, p)
		}
	}
	return providers
}

// GetAPIKeys returns a map of provider name to API key for validated providers.
func (m InitWizardModel) GetAPIKeys() map[string]string {
	return m.apiKeyInputs
}

// GetAPIKeyResults returns a map of provider name to validation result.
func (m InitWizardModel) GetAPIKeyResults() map[string]*APIKeyResult {
	return m.apiKeyResults
}

// GetValidatedAPIKeys returns only the API keys that have been successfully validated.
func (m InitWizardModel) GetValidatedAPIKeys() map[string]string {
	result := make(map[string]string)
	for provider, apiKey := range m.apiKeyInputs {
		if res, ok := m.apiKeyResults[provider]; ok && res.Valid {
			result[provider] = apiKey
		}
	}
	return result
}

// AllAPIKeysValidated returns true if all selected providers have valid API keys.
func (m InitWizardModel) AllAPIKeysValidated() bool {
	providers := m.SelectedProviders()
	for _, p := range providers {
		result, ok := m.apiKeyResults[p]
		if !ok || !result.Valid {
			return false
		}
	}
	return len(providers) > 0
}

// GetWireGuardKeyPair returns the configured WireGuard key pair.
func (m InitWizardModel) GetWireGuardKeyPair() *wireguard.KeyPair {
	return m.wireGuardKeyPair
}

// GetSelectedTier returns the selected default tier.
func (m InitWizardModel) GetSelectedTier() string {
	return m.selectedTier
}

// GetDeadmanTimeout returns the configured deadman timeout in hours.
func (m InitWizardModel) GetDeadmanTimeout() int {
	if timeout, err := strconv.Atoi(m.deadmanTimeoutInput); err == nil {
		return timeout
	}
	return 10 // default
}

// SetDimensions sets the terminal dimensions.
func (m *InitWizardModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
}

// IsQuitting returns true if the wizard is quitting.
func (m InitWizardModel) IsQuitting() bool {
	return m.quitting
}

// IsDone returns true if the current step is done.
func (m InitWizardModel) IsDone() bool {
	return m.done
}

// Step returns the current step.
func (m InitWizardModel) Step() InitWizardStep {
	return m.step
}

// SetStep sets the current step.
func (m *InitWizardModel) SetStep(step InitWizardStep) {
	m.step = step
}

// Error returns any error that occurred.
func (m InitWizardModel) Error() error {
	return m.err
}

// Reset resets the wizard to the initial state.
func (m *InitWizardModel) Reset() {
	m.step = StepProviderSelect
	m.selected = make(map[string]bool)
	m.providerErrors = make(map[string]error)
	m.cursor = 0
	m.quitting = false
	m.done = false
	m.err = nil
}

// ToggleProvider toggles the selection of a provider.
func (m *InitWizardModel) ToggleProvider(name string) {
	m.selected[name] = !m.selected[name]
}

// SelectProvider selects a provider.
func (m *InitWizardModel) SelectProvider(name string) {
	m.selected[name] = true
}

// DeselectProvider deselects a provider.
func (m *InitWizardModel) DeselectProvider(name string) {
	delete(m.selected, name)
}

// IsProviderSelected returns true if the provider is selected.
func (m InitWizardModel) IsProviderSelected(name string) bool {
	return m.selected[name]
}

// Messages

// InitWizardProvidersSelectedMsg is sent when providers are selected.
type InitWizardProvidersSelectedMsg struct {
	Providers []string
}

// InitWizardErrorMsg is sent when an error occurs.
type InitWizardErrorMsg struct {
	Err error
}

// InitWizardCompleteMsg is sent when the wizard is complete.
type InitWizardCompleteMsg struct{}

// APIKeyValidationStartMsg is sent when API key validation should start.
type APIKeyValidationStartMsg struct {
	Provider string
	APIKey   string
}

// APIKeyValidationResultMsg is sent when API key validation completes.
type APIKeyValidationResultMsg struct {
	Provider    string
	Valid       bool
	AccountInfo *provider.AccountInfo
	Error       error
}

// WireGuardKeyGeneratedMsg is sent when a new WireGuard key pair is generated.
type WireGuardKeyGeneratedMsg struct {
	KeyPair *wireguard.KeyPair
	Error   error
}

// WireGuardKeyValidatedMsg is sent when an existing WireGuard key is validated.
type WireGuardKeyValidatedMsg struct {
	KeyPair *wireguard.KeyPair
	Error   error
}

// ConfigSavedMsg is sent when the configuration is saved to .env file.
type ConfigSavedMsg struct {
	Error error
}

// Commands

// RunInitWizard starts the init wizard.
func RunInitWizard() error {
	p := tea.NewProgram(
		NewInitWizardModel(),
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("init wizard failed: %w", err)
	}

	model, ok := finalModel.(InitWizardModel)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	if model.IsQuitting() && !model.IsDone() {
		return fmt.Errorf("setup cancelled by user")
	}

	return nil
}
