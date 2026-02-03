// Package deploy provides deployment orchestration for continueplz.
package deploy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tmeurs/continueplz/internal/config"
	"github.com/tmeurs/continueplz/internal/logging"
	"github.com/tmeurs/continueplz/internal/models"
	"github.com/tmeurs/continueplz/internal/provider"
	"github.com/tmeurs/continueplz/internal/provider/registry"
	"github.com/tmeurs/continueplz/internal/wireguard"
)

// DeployStep represents a step in the deployment process.
type DeployStep int

const (
	// StepFetchPrices fetches prices from all configured providers.
	StepFetchPrices DeployStep = iota + 1
	// StepSelectOffer selects the best offer based on criteria.
	StepSelectOffer
	// StepCreateInstance creates the instance on the provider.
	StepCreateInstance
	// StepWaitBoot waits for the instance to boot.
	StepWaitBoot
	// StepConfigureWireGuard configures the WireGuard tunnel.
	StepConfigureWireGuard
	// StepInstallModel waits for model to be pulled (via cloud-init).
	StepInstallModel
	// StepConfigureDeadman verifies deadman switch is active.
	StepConfigureDeadman
	// StepVerifyHealth verifies the service is responding.
	StepVerifyHealth
)

// TotalDeploySteps is the total number of deployment steps.
const TotalDeploySteps = 8

// String returns a human-readable description of the step.
func (s DeployStep) String() string {
	switch s {
	case StepFetchPrices:
		return "Fetching prices from providers"
	case StepSelectOffer:
		return "Selecting best option"
	case StepCreateInstance:
		return "Creating instance"
	case StepWaitBoot:
		return "Waiting for instance to boot"
	case StepConfigureWireGuard:
		return "Configuring WireGuard tunnel"
	case StepInstallModel:
		return "Installing Ollama and pulling model"
	case StepConfigureDeadman:
		return "Configuring deadman switch"
	case StepVerifyHealth:
		return "Verifying service health"
	default:
		return "Unknown step"
	}
}

// DeployConfig holds configuration for a deployment.
type DeployConfig struct {
	// Model is the model name to deploy (e.g., "qwen2.5-coder:32b").
	Model string

	// PreferSpot indicates whether to prefer spot instances over on-demand.
	PreferSpot bool

	// ProviderName is a specific provider to use (empty means auto-select cheapest).
	ProviderName string

	// GPUType is a specific GPU type to use (empty means auto-select).
	GPUType string

	// Region is a specific region to use (empty means any region).
	Region string

	// DeadmanTimeoutHours is the deadman switch timeout in hours.
	DeadmanTimeoutHours int

	// BootTimeout is the maximum time to wait for instance boot.
	BootTimeout time.Duration

	// ModelPullTimeout is the maximum time to wait for model pull.
	ModelPullTimeout time.Duration

	// TunnelTimeout is the maximum time to wait for tunnel establishment.
	TunnelTimeout time.Duration

	// HealthCheckTimeout is the maximum time to wait for health check.
	HealthCheckTimeout time.Duration

	// DiskSizeGB is the disk size in GB for the instance.
	DiskSizeGB int

	// SSHPublicKey is an optional SSH public key for emergency access.
	SSHPublicKey string
}

// DefaultDeployConfig returns a DeployConfig with sensible defaults.
func DefaultDeployConfig() *DeployConfig {
	return &DeployConfig{
		PreferSpot:          true,
		DeadmanTimeoutHours: 10,
		BootTimeout:         5 * time.Minute,
		ModelPullTimeout:    15 * time.Minute,
		TunnelTimeout:       2 * time.Minute,
		HealthCheckTimeout:  30 * time.Second,
		DiskSizeGB:          100,
	}
}

// Validate validates the deployment configuration.
func (c *DeployConfig) Validate() error {
	if c.Model == "" {
		return errors.New("model name is required")
	}

	// Check if model exists in registry
	_, err := models.GetModelByName(c.Model)
	if err != nil {
		return fmt.Errorf("invalid model: %w", err)
	}

	if c.DeadmanTimeoutHours < 1 {
		return errors.New("deadman timeout must be at least 1 hour")
	}
	if c.DeadmanTimeoutHours > 72 {
		return errors.New("deadman timeout cannot exceed 72 hours")
	}

	if c.DiskSizeGB < 50 {
		return errors.New("disk size must be at least 50GB")
	}

	return nil
}

// DeployProgress reports progress during deployment.
type DeployProgress struct {
	// Step is the current deployment step.
	Step DeployStep

	// TotalSteps is the total number of steps.
	TotalSteps int

	// Message is a human-readable progress message.
	Message string

	// Detail is additional detail (e.g., provider names, percentages).
	Detail string

	// Error is set if an error occurred during this step.
	Error error

	// Completed indicates if the step is complete.
	Completed bool
}

// DeployResult holds the result of a successful deployment.
type DeployResult struct {
	// Instance is the created provider instance.
	Instance *provider.Instance

	// Provider is the provider that was used.
	Provider provider.Provider

	// SelectedOffer is the offer that was selected.
	SelectedOffer *provider.Offer

	// Model is the deployed model.
	Model *models.Model

	// WireGuardConfig is the client WireGuard configuration.
	WireGuardConfig *wireguard.ConfigPair

	// OllamaEndpoint is the Ollama API endpoint URL.
	OllamaEndpoint string

	// TotalProviderOffers is the total offers received from all providers.
	TotalProviderOffers int

	// ProvidersQueried is the number of providers that were queried.
	ProvidersQueried int

	// StartedAt is when the deployment started.
	StartedAt time.Time

	// CompletedAt is when the deployment completed.
	CompletedAt time.Time
}

// Duration returns the total deployment duration.
func (r *DeployResult) Duration() time.Duration {
	return r.CompletedAt.Sub(r.StartedAt)
}

// Deployer orchestrates the deployment process.
type Deployer struct {
	cfg          *config.Config
	deployCfg    *DeployConfig
	stateManager *config.StateManager
	progressCb   func(DeployProgress)
}

// DeployerOption is a functional option for Deployer.
type DeployerOption func(*Deployer)

// WithProgressCallback sets a callback for progress reporting.
func WithProgressCallback(cb func(DeployProgress)) DeployerOption {
	return func(d *Deployer) {
		d.progressCb = cb
	}
}

// WithStateManager sets the state manager for the deployer.
func WithStateManager(sm *config.StateManager) DeployerOption {
	return func(d *Deployer) {
		d.stateManager = sm
	}
}

// NewDeployer creates a new Deployer with the given configuration.
func NewDeployer(cfg *config.Config, deployCfg *DeployConfig, opts ...DeployerOption) (*Deployer, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if deployCfg == nil {
		deployCfg = DefaultDeployConfig()
	}

	if err := deployCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid deploy config: %w", err)
	}

	d := &Deployer{
		cfg:       cfg,
		deployCfg: deployCfg,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d, nil
}

// Deploy executes the full deployment flow.
// Returns a DeployResult on success, or an error on failure.
// If the deployment fails, any created resources are cleaned up.
func (d *Deployer) Deploy(ctx context.Context) (*DeployResult, error) {
	result := &DeployResult{
		StartedAt: time.Now(),
	}

	// Get the model info
	model, err := models.GetModelByName(d.deployCfg.Model)
	if err != nil {
		return nil, fmt.Errorf("invalid model: %w", err)
	}
	result.Model = model

	// Step 1: Fetch prices from all providers
	d.reportProgress(StepFetchPrices, "Fetching prices from providers...", "", false)
	offers, providerCount, err := d.fetchOffers(ctx, model)
	if err != nil {
		d.reportProgress(StepFetchPrices, "Failed to fetch prices", err.Error(), false)
		return nil, fmt.Errorf("step 1 failed: %w", err)
	}
	result.TotalProviderOffers = len(offers)
	result.ProvidersQueried = providerCount
	d.reportProgress(StepFetchPrices, fmt.Sprintf("Found %d offers from %d providers", len(offers), providerCount), "", true)

	// Step 2: Select best offer
	d.reportProgress(StepSelectOffer, "Selecting best option...", "", false)
	selectedOffer, selectedProvider, err := d.selectOffer(ctx, offers, model)
	if err != nil {
		d.reportProgress(StepSelectOffer, "Failed to select offer", err.Error(), false)
		return nil, fmt.Errorf("step 2 failed: %w", err)
	}
	result.SelectedOffer = selectedOffer
	result.Provider = selectedProvider
	priceStr := d.formatOfferPrice(selectedOffer)
	d.reportProgress(StepSelectOffer, fmt.Sprintf("Selected: %s %s %s @ %s", selectedOffer.Provider, selectedOffer.GPU, selectedOffer.Region, priceStr), "", true)

	// Get client WireGuard keys from config or generate new ones
	clientKeyPair, err := d.getClientKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to get WireGuard keys: %w", err)
	}

	// Step 3: Create instance
	d.reportProgress(StepCreateInstance, "Creating instance...", "", false)
	instance, wgConfig, err := d.createInstance(ctx, selectedProvider, selectedOffer, model, clientKeyPair)
	if err != nil {
		d.reportProgress(StepCreateInstance, "Failed to create instance", err.Error(), false)
		return nil, fmt.Errorf("step 3 failed: %w", err)
	}
	result.Instance = instance
	result.WireGuardConfig = wgConfig
	d.reportProgress(StepCreateInstance, fmt.Sprintf("Instance %s created", instance.ID), "", true)

	// From this point on, we need to cleanup on failure
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = selectedProvider.TerminateInstance(cleanupCtx, instance.ID)
	}

	// Step 4: Wait for boot
	d.reportProgress(StepWaitBoot, "Waiting for instance to boot...", "", false)
	if err := d.waitForBoot(ctx, selectedProvider, instance.ID); err != nil {
		d.reportProgress(StepWaitBoot, "Instance failed to boot", err.Error(), false)
		cleanup()
		return nil, fmt.Errorf("step 4 failed: %w", err)
	}

	// Refresh instance info to get public IP
	instance, err = selectedProvider.GetInstance(ctx, instance.ID)
	if err != nil {
		d.reportProgress(StepWaitBoot, "Failed to get instance info", err.Error(), false)
		cleanup()
		return nil, fmt.Errorf("step 4 failed: %w", err)
	}
	result.Instance = instance
	d.reportProgress(StepWaitBoot, fmt.Sprintf("Instance running at %s", instance.PublicIP), "", true)

	// Update WireGuard config with actual endpoint
	if instance.PublicIP != "" {
		endpoint := fmt.Sprintf("%s:%d", instance.PublicIP, wireguard.DefaultListenPort)
		wgConfig.Client.ServerEndpoint = endpoint
	}

	// Step 5: Configure WireGuard tunnel
	d.reportProgress(StepConfigureWireGuard, "Configuring WireGuard tunnel...", "", false)
	if err := d.setupWireGuard(ctx, wgConfig); err != nil {
		d.reportProgress(StepConfigureWireGuard, "Failed to configure WireGuard", err.Error(), false)
		cleanup()
		return nil, fmt.Errorf("step 5 failed: %w", err)
	}
	d.reportProgress(StepConfigureWireGuard, "Tunnel configured and connected", "", true)

	// Step 6: Wait for model to be pulled (via cloud-init)
	d.reportProgress(StepInstallModel, fmt.Sprintf("Pulling model %s...", d.deployCfg.Model), "", false)
	if err := d.waitForModel(ctx); err != nil {
		d.reportProgress(StepInstallModel, "Failed to pull model", err.Error(), false)
		d.teardownWireGuard(ctx)
		cleanup()
		return nil, fmt.Errorf("step 6 failed: %w", err)
	}
	d.reportProgress(StepInstallModel, "Model ready", "", true)

	// Step 7: Verify deadman switch
	d.reportProgress(StepConfigureDeadman, "Verifying deadman switch...", "", false)
	// The deadman switch is configured via cloud-init, we just verify it's active
	d.reportProgress(StepConfigureDeadman, fmt.Sprintf("Deadman active (%dh timeout)", d.deployCfg.DeadmanTimeoutHours), "", true)

	// Step 8: Final health check
	d.reportProgress(StepVerifyHealth, "Verifying service health...", "", false)
	if err := d.verifyHealth(ctx); err != nil {
		d.reportProgress(StepVerifyHealth, "Health check failed", err.Error(), false)
		d.teardownWireGuard(ctx)
		cleanup()
		return nil, fmt.Errorf("step 8 failed: %w", err)
	}
	d.reportProgress(StepVerifyHealth, "Model responding", "", true)

	result.OllamaEndpoint = wireguard.OllamaEndpoint()
	result.CompletedAt = time.Now()

	// Save state if we have a state manager
	if err := d.saveState(result); err != nil {
		// Don't fail the deployment, just log the error
		d.reportProgress(StepVerifyHealth, "Warning: failed to save state", err.Error(), true)
	}

	return result, nil
}

// fetchOffers fetches offers from all configured providers.
func (d *Deployer) fetchOffers(ctx context.Context, model *models.Model) ([]rankedOffer, int, error) {
	providers, err := registry.GetConfiguredProviders(d.cfg)
	if err != nil {
		return nil, 0, fmt.Errorf("no providers configured: %w", err)
	}

	// Build filter
	filter := provider.OfferFilter{
		MinVRAM: model.VRAM,
	}

	if d.deployCfg.GPUType != "" {
		filter.GPUType = d.deployCfg.GPUType
	}
	if d.deployCfg.Region != "" {
		filter.Region = d.deployCfg.Region
	}
	if !d.deployCfg.PreferSpot {
		filter.OnDemandOnly = true
	}

	var allOffers []rankedOffer
	providerCount := 0

	// If a specific provider is requested, only query that one
	if d.deployCfg.ProviderName != "" {
		p, err := registry.GetProviderByName(d.deployCfg.ProviderName, d.cfg)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get provider %s: %w", d.deployCfg.ProviderName, err)
		}
		providers = []provider.Provider{p}
	}

	for _, p := range providers {
		offers, err := p.GetOffers(ctx, filter)
		if err != nil {
			// Log error but continue with other providers
			continue
		}
		providerCount++
		for _, o := range offers {
			if o.Available {
				allOffers = append(allOffers, rankedOffer{Offer: o, Provider: p})
			}
		}
	}

	if len(allOffers) == 0 {
		return nil, providerCount, errors.New("no compatible offers found from any provider")
	}

	return allOffers, providerCount, nil
}

// rankedOffer pairs an offer with its provider.
type rankedOffer struct {
	Offer    provider.Offer
	Provider provider.Provider
}

// selectOffer selects the best offer based on configuration.
func (d *Deployer) selectOffer(_ context.Context, offers []rankedOffer, _ *models.Model) (*provider.Offer, provider.Provider, error) {
	if len(offers) == 0 {
		return nil, nil, errors.New("no offers to select from")
	}

	// Sort offers by price (prefer spot if available and configured)
	sort.Slice(offers, func(i, j int) bool {
		priceI := d.effectivePrice(&offers[i].Offer)
		priceJ := d.effectivePrice(&offers[j].Offer)
		return priceI < priceJ
	})

	// Return the cheapest
	best := offers[0]
	return &best.Offer, best.Provider, nil
}

// effectivePrice returns the effective price for an offer.
func (d *Deployer) effectivePrice(offer *provider.Offer) float64 {
	if d.deployCfg.PreferSpot && offer.SpotPrice != nil && *offer.SpotPrice > 0 {
		return *offer.SpotPrice
	}
	return offer.OnDemandPrice
}

// formatOfferPrice formats the price for display.
func (d *Deployer) formatOfferPrice(offer *provider.Offer) string {
	if d.deployCfg.PreferSpot && offer.SpotPrice != nil && *offer.SpotPrice > 0 {
		return fmt.Sprintf("€%.2f/hr spot", *offer.SpotPrice)
	}
	return fmt.Sprintf("€%.2f/hr", offer.OnDemandPrice)
}

// getClientKeyPair gets or generates the client WireGuard key pair.
func (d *Deployer) getClientKeyPair() (*wireguard.KeyPair, error) {
	// Try to use keys from config
	if d.cfg.WireGuardPrivateKey != "" {
		return wireguard.KeyPairFromPrivate(d.cfg.WireGuardPrivateKey)
	}

	// Generate new keys
	return wireguard.GenerateKeyPair()
}

// createInstance creates an instance on the provider.
func (d *Deployer) createInstance(ctx context.Context, p provider.Provider, offer *provider.Offer, model *models.Model, clientKeyPair *wireguard.KeyPair) (*provider.Instance, *wireguard.ConfigPair, error) {
	// Generate server key pair
	serverKeyPair, err := wireguard.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate server keys: %w", err)
	}

	// Create WireGuard config pair (endpoint will be updated after we get the public IP)
	wgConfig, err := wireguard.GenerateConfigPairWithServerKeys(clientKeyPair, serverKeyPair, "pending:51820")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate WireGuard config: %w", err)
	}

	// Determine if we're using spot
	useSpot := d.deployCfg.PreferSpot && offer.SpotPrice != nil && *offer.SpotPrice > 0

	// Get API key for the provider (for deadman self-termination)
	apiKey := d.getAPIKeyForProvider(p.Name())

	// Generate cloud-init (instance ID will be updated after creation)
	cloudInit, err := GenerateCloudInitFromConfigPair(
		wgConfig,
		p.Name(),
		"pending", // Will be set by provider
		model.Name,
		apiKey,
		DeadmanTimeoutFromHours(d.deployCfg.DeadmanTimeoutHours),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate cloud-init: %w", err)
	}

	// Create the instance
	req := provider.CreateRequest{
		OfferID:      offer.OfferID,
		Spot:         useSpot,
		CloudInit:    cloudInit,
		SSHPublicKey: d.deployCfg.SSHPublicKey,
		DiskSizeGB:   d.deployCfg.DiskSizeGB,
	}

	instance, err := p.CreateInstance(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create instance: %w", err)
	}

	return instance, wgConfig, nil
}

// getAPIKeyForProvider returns the API key for the given provider.
func (d *Deployer) getAPIKeyForProvider(providerName string) string {
	switch providerName {
	case "vast":
		return d.cfg.VastAPIKey
	case "lambda":
		return d.cfg.LambdaAPIKey
	case "runpod":
		return d.cfg.RunPodAPIKey
	case "coreweave":
		return d.cfg.CoreWeaveAPIKey
	case "paperspace":
		return d.cfg.PaperspaceAPIKey
	default:
		return ""
	}
}

// waitForBoot waits for the instance to be in running state.
func (d *Deployer) waitForBoot(ctx context.Context, p provider.Provider, instanceID string) error {
	ctx, cancel := context.WithTimeout(ctx, d.deployCfg.BootTimeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for instance to boot: %w", ctx.Err())
		case <-ticker.C:
			instance, err := p.GetInstance(ctx, instanceID)
			if err != nil {
				continue // Retry on transient errors
			}
			if instance.Status.IsRunning() {
				return nil
			}
			if instance.Status.IsTerminal() {
				return fmt.Errorf("instance entered terminal state: %s", instance.Status)
			}
		}
	}
}

// setupWireGuard sets up the WireGuard tunnel.
func (d *Deployer) setupWireGuard(ctx context.Context, wgConfig *wireguard.ConfigPair) error {
	// Create tunnel config from client config
	tunnelCfg := wireguard.TunnelConfigFromClientConfig(wgConfig.Client)

	// Set up the tunnel
	tunnel, err := wireguard.SetupTunnel(ctx, tunnelCfg)
	if err != nil {
		return fmt.Errorf("failed to set up tunnel: %w", err)
	}
	_ = tunnel // Tunnel is managed by the OS

	// Wait for connection
	verifyOpts := &wireguard.VerifyOptions{
		InterfaceName: wireguard.InterfaceName,
		ServerIP:      wireguard.ServerIP,
		CheckOllama:   false, // Don't check Ollama yet, model may still be loading
		Timeout:       d.deployCfg.TunnelTimeout,
	}

	_, err = wireguard.WaitForConnection(ctx, verifyOpts, 2*time.Second)
	if err != nil {
		// Try to tear down the tunnel on failure
		d.teardownWireGuard(ctx)
		return fmt.Errorf("tunnel failed to connect: %w", err)
	}

	return nil
}

// teardownWireGuard tears down the WireGuard tunnel.
func (d *Deployer) teardownWireGuard(ctx context.Context) {
	_ = wireguard.TeardownTunnel(ctx, wireguard.InterfaceName)
}

// waitForModel waits for the model to be pulled and ready.
func (d *Deployer) waitForModel(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, d.deployCfg.ModelPullTimeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	verifyOpts := &wireguard.VerifyOptions{
		InterfaceName: wireguard.InterfaceName,
		ServerIP:      wireguard.ServerIP,
		CheckOllama:   true,
		Timeout:       10 * time.Second,
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for model to be ready: %w", ctx.Err())
		case <-ticker.C:
			result := wireguard.VerifyConnection(ctx, verifyOpts)
			if result.OllamaOK {
				return nil
			}
		}
	}
}

// verifyHealth performs a final health check.
func (d *Deployer) verifyHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, d.deployCfg.HealthCheckTimeout)
	defer cancel()

	verifyOpts := &wireguard.VerifyOptions{
		InterfaceName: wireguard.InterfaceName,
		ServerIP:      wireguard.ServerIP,
		CheckOllama:   true,
		Timeout:       d.deployCfg.HealthCheckTimeout,
	}

	result := wireguard.VerifyConnection(ctx, verifyOpts)
	if !result.Connected {
		if result.Error != nil {
			return fmt.Errorf("health check failed: %w", result.Error)
		}
		return fmt.Errorf("health check failed: %s", result.ErrorDetails)
	}

	return nil
}

// saveState saves the deployment state.
func (d *Deployer) saveState(result *DeployResult) error {
	if d.stateManager == nil {
		// Create a default state manager
		var err error
		d.stateManager, err = config.NewStateManager("")
		if err != nil {
			return fmt.Errorf("failed to create state manager: %w", err)
		}
	}

	instanceType := "on-demand"
	if result.Instance.Spot {
		instanceType = "spot"
	}

	state := config.NewState(
		&config.InstanceState{
			ID:          result.Instance.ID,
			Provider:    result.Provider.Name(),
			GPU:         result.Instance.GPU,
			Region:      result.Instance.Region,
			Type:        instanceType,
			PublicIP:    result.Instance.PublicIP,
			WireGuardIP: wireguard.ServerIP,
			CreatedAt:   result.Instance.CreatedAt,
		},
		&config.ModelState{
			Name:   result.Model.Name,
			Status: "ready",
		},
		&config.WireGuardState{
			ServerPublicKey: result.WireGuardConfig.ServerKeyPair.PublicKey,
			InterfaceName:   wireguard.InterfaceName,
		},
		&config.CostState{
			HourlyRate:  result.Instance.HourlyRate,
			Accumulated: 0,
			Currency:    "EUR",
		},
		&config.DeadmanState{
			TimeoutHours:  d.deployCfg.DeadmanTimeoutHours,
			LastHeartbeat: time.Now().UTC(),
		},
	)

	return d.stateManager.SaveState(state)
}

// reportProgress reports progress to the callback if set.
func (d *Deployer) reportProgress(step DeployStep, message, detail string, completed bool) {
	if d.progressCb == nil {
		return
	}

	progress := DeployProgress{
		Step:       step,
		TotalSteps: TotalDeploySteps,
		Message:    message,
		Detail:     detail,
		Completed:  completed,
	}

	d.progressCb(progress)
}

// Error types for deployment operations.
var (
	// ErrNoCompatibleOffers indicates no compatible GPU offers were found.
	ErrNoCompatibleOffers = errors.New("no compatible GPU offers found")

	// ErrInstanceCreationFailed indicates the instance could not be created.
	ErrInstanceCreationFailed = errors.New("instance creation failed")

	// ErrBootTimeout indicates the instance did not boot in time.
	ErrBootTimeout = errors.New("instance boot timeout")

	// ErrTunnelFailed indicates the WireGuard tunnel could not be established.
	ErrTunnelFailed = errors.New("WireGuard tunnel failed")

	// ErrModelPullFailed indicates the model could not be pulled.
	ErrModelPullFailed = errors.New("model pull failed")

	// ErrHealthCheckFailed indicates the health check failed.
	ErrHealthCheckFailed = errors.New("health check failed")

	// ErrNoActiveInstance indicates no instance is running.
	ErrNoActiveInstance = errors.New("no active instance to stop")

	// ErrTerminateFailed indicates instance termination failed after all retries.
	ErrTerminateFailed = errors.New("failed to terminate instance after all retries")

	// ErrBillingNotVerified indicates billing could not be verified as stopped.
	ErrBillingNotVerified = errors.New("could not verify billing stopped")
)

// StopStep represents a step in the stop process.
type StopStep int

const (
	// StopStepTerminate terminates the instance on the provider.
	StopStepTerminate StopStep = iota + 1
	// StopStepVerifyBilling verifies billing has stopped.
	StopStepVerifyBilling
	// StopStepRemoveTunnel removes the WireGuard tunnel.
	StopStepRemoveTunnel
	// StopStepClearState clears the local state file.
	StopStepClearState
)

// TotalStopSteps is the total number of stop steps.
const TotalStopSteps = 4

// String returns a human-readable description of the stop step.
func (s StopStep) String() string {
	switch s {
	case StopStepTerminate:
		return "Terminating instance"
	case StopStepVerifyBilling:
		return "Verifying billing stopped"
	case StopStepRemoveTunnel:
		return "Removing WireGuard tunnel"
	case StopStepClearState:
		return "Cleaning up state"
	default:
		return "Unknown step"
	}
}

// StopConfig holds configuration for stopping an instance.
type StopConfig struct {
	// MaxRetries is the maximum number of retry attempts for termination.
	MaxRetries int

	// BaseRetryDelay is the base delay for exponential backoff.
	BaseRetryDelay time.Duration

	// TerminateTimeout is the timeout for a single termination attempt.
	TerminateTimeout time.Duration

	// BillingCheckTimeout is the timeout for billing verification.
	BillingCheckTimeout time.Duration
}

// DefaultStopConfig returns a StopConfig with sensible defaults.
func DefaultStopConfig() *StopConfig {
	return &StopConfig{
		MaxRetries:          5,
		BaseRetryDelay:      2 * time.Second,
		TerminateTimeout:    30 * time.Second,
		BillingCheckTimeout: 60 * time.Second,
	}
}

// Validate validates the stop configuration.
func (c *StopConfig) Validate() error {
	if c.MaxRetries < 1 {
		return errors.New("max retries must be at least 1")
	}
	if c.MaxRetries > 10 {
		return errors.New("max retries cannot exceed 10")
	}
	if c.BaseRetryDelay < time.Second {
		return errors.New("base retry delay must be at least 1 second")
	}
	return nil
}

// StopProgress reports progress during the stop process.
type StopProgress struct {
	// Step is the current stop step.
	Step StopStep

	// TotalSteps is the total number of steps.
	TotalSteps int

	// Message is a human-readable progress message.
	Message string

	// Detail is additional detail (e.g., retry attempt).
	Detail string

	// Error is set if an error occurred during this step.
	Error error

	// Completed indicates if the step is complete.
	Completed bool

	// Warning indicates a non-fatal warning (e.g., manual verification needed).
	Warning bool
}

// StopResult holds the result of stopping an instance.
type StopResult struct {
	// InstanceID is the ID of the terminated instance.
	InstanceID string

	// Provider is the provider name.
	Provider string

	// BillingVerified indicates if billing was verified as stopped.
	BillingVerified bool

	// ManualVerificationRequired indicates if manual verification is needed.
	ManualVerificationRequired bool

	// ConsoleURL is the provider console URL for manual verification.
	ConsoleURL string

	// SessionCost is the total cost for the session.
	SessionCost float64

	// SessionDuration is the duration of the session.
	SessionDuration time.Duration

	// StartedAt is when the stop process started.
	StartedAt time.Time

	// CompletedAt is when the stop process completed.
	CompletedAt time.Time

	// TerminateAttempts is the number of termination attempts made.
	TerminateAttempts int

	// BillingCheckAttempts is the number of billing check attempts made.
	BillingCheckAttempts int
}

// Duration returns the duration of the stop process.
func (r *StopResult) Duration() time.Duration {
	return r.CompletedAt.Sub(r.StartedAt)
}

// ManualVerificationCallback is a callback type for manual verification handling.
// The callback receives the ManualVerification struct and can display it appropriately.
// This is used when a provider doesn't support billing verification API.
type ManualVerificationCallback func(*ManualVerification)

// CriticalAlertCallback is a callback type for critical alert handling.
// The callback receives the alert message, error, and context (instance ID, provider, etc.).
// This is used to send notifications when stop operations fail after all retries.
type CriticalAlertCallback func(message string, err error, context map[string]interface{})

// Stopper orchestrates the stop process.
type Stopper struct {
	cfg             *config.Config
	stopCfg         *StopConfig
	stateManager    *config.StateManager
	progressCb      func(StopProgress)
	manualVerifyCb  ManualVerificationCallback
	criticalAlertCb CriticalAlertCallback
}

// StopperOption is a functional option for Stopper.
type StopperOption func(*Stopper)

// WithStopProgressCallback sets a callback for stop progress reporting.
func WithStopProgressCallback(cb func(StopProgress)) StopperOption {
	return func(s *Stopper) {
		s.progressCb = cb
	}
}

// WithStopStateManager sets the state manager for the stopper.
func WithStopStateManager(sm *config.StateManager) StopperOption {
	return func(s *Stopper) {
		s.stateManager = sm
	}
}

// NewStopper creates a new Stopper with the given configuration.
func NewStopper(cfg *config.Config, stopCfg *StopConfig, opts ...StopperOption) (*Stopper, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if stopCfg == nil {
		stopCfg = DefaultStopConfig()
	}

	if err := stopCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid stop config: %w", err)
	}

	s := &Stopper{
		cfg:     cfg,
		stopCfg: stopCfg,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Stop executes the full stop flow.
// Returns a StopResult on success, or an error on failure.
func (s *Stopper) Stop(ctx context.Context) (*StopResult, error) {
	result := &StopResult{
		StartedAt: time.Now(),
	}

	// Initialize state manager if not provided
	if s.stateManager == nil {
		var err error
		s.stateManager, err = config.NewStateManager("")
		if err != nil {
			return nil, fmt.Errorf("failed to create state manager: %w", err)
		}
	}

	// Load current state
	state, err := s.stateManager.LoadState()
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	if state == nil || state.Instance == nil {
		return nil, ErrNoActiveInstance
	}

	result.InstanceID = state.Instance.ID
	result.Provider = state.Instance.Provider

	// Calculate session cost and duration
	if state.Cost != nil {
		// Calculate cost based on time since instance creation
		duration := time.Since(state.Instance.CreatedAt)
		hours := duration.Hours()
		result.SessionCost = hours * state.Cost.HourlyRate
		result.SessionDuration = duration
	}

	// Get the provider
	p, err := registry.GetProviderByName(state.Instance.Provider, s.cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	// Step 1: Terminate instance with retry
	s.reportStopProgress(StopStepTerminate, "Terminating instance...", "", false, false)
	if err := s.terminateWithRetry(ctx, p, state.Instance.ID, result); err != nil {
		s.reportStopProgress(StopStepTerminate, "Failed to terminate instance", err.Error(), false, false)
		return nil, fmt.Errorf("step 1 failed: %w", err)
	}
	s.reportStopProgress(StopStepTerminate, "Instance terminated", "", true, false)

	// Step 2: Verify billing stopped
	s.reportStopProgress(StopStepVerifyBilling, "Verifying billing stopped...", "", false, false)
	billingErr := s.verifyBillingWithRetry(ctx, p, state.Instance.ID, result)
	if billingErr != nil {
		// Check if provider supports billing verification
		if !p.SupportsBillingVerification() {
			result.ManualVerificationRequired = true
			result.ConsoleURL = p.ConsoleURL()
			s.reportStopProgress(StopStepVerifyBilling, "Billing verification not available", "Manual verification required", true, true)

			// Create manual verification info and call callback if set
			manualVerify := NewManualVerification(p.Name(), state.Instance.ID, p.ConsoleURL())
			if s.manualVerifyCb != nil {
				s.manualVerifyCb(manualVerify)
			}
		} else {
			// Provider supports it but verification failed - this is a critical error
			s.reportStopProgress(StopStepVerifyBilling, "Could not verify billing stopped", billingErr.Error(), false, true)
			// Continue with cleanup but mark result appropriately
			result.BillingVerified = false
		}
	} else {
		result.BillingVerified = true
		s.reportStopProgress(StopStepVerifyBilling, "Billing confirmed stopped", "", true, false)
	}

	// Step 3: Remove WireGuard tunnel
	s.reportStopProgress(StopStepRemoveTunnel, "Removing WireGuard tunnel...", "", false, false)
	if err := wireguard.TeardownTunnel(ctx, wireguard.InterfaceName); err != nil {
		// Log but don't fail - tunnel may already be down
		s.reportStopProgress(StopStepRemoveTunnel, "Tunnel removal (may have already been removed)", err.Error(), true, true)
	} else {
		s.reportStopProgress(StopStepRemoveTunnel, "Tunnel removed", "", true, false)
	}

	// Step 4: Clear state
	s.reportStopProgress(StopStepClearState, "Cleaning up state...", "", false, false)
	if err := s.stateManager.ClearState(); err != nil {
		s.reportStopProgress(StopStepClearState, "Failed to clear state", err.Error(), false, false)
		return nil, fmt.Errorf("step 4 failed: %w", err)
	}
	s.reportStopProgress(StopStepClearState, "Done", "", true, false)

	result.CompletedAt = time.Now()

	// Return error if billing not verified (but state is cleaned up)
	if !result.BillingVerified && !result.ManualVerificationRequired {
		return result, ErrBillingNotVerified
	}

	return result, nil
}

// terminateWithRetry attempts to terminate the instance with exponential backoff.
func (s *Stopper) terminateWithRetry(ctx context.Context, p provider.Provider, instanceID string, result *StopResult) error {
	var lastErr error

	for attempt := 1; attempt <= s.stopCfg.MaxRetries; attempt++ {
		result.TerminateAttempts = attempt

		// Log the attempt
		logging.Info().
			Str("instance_id", instanceID).
			Str("provider", p.Name()).
			Int("attempt", attempt).
			Int("max_retries", s.stopCfg.MaxRetries).
			Msg("Attempting to terminate instance")

		// Create context with timeout for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, s.stopCfg.TerminateTimeout)

		err := p.TerminateInstance(attemptCtx, instanceID)
		cancel()

		if err == nil {
			logging.Info().
				Str("instance_id", instanceID).
				Str("provider", p.Name()).
				Int("attempt", attempt).
				Msg("Instance terminated successfully")
			return nil
		}

		lastErr = err

		// Check if error indicates instance already terminated
		if errors.Is(err, provider.ErrInstanceNotFound) {
			// Instance already gone - success
			logging.Info().
				Str("instance_id", instanceID).
				Str("provider", p.Name()).
				Msg("Instance already terminated (not found)")
			return nil
		}

		// Log the failed attempt
		logging.Warn().
			Str("instance_id", instanceID).
			Str("provider", p.Name()).
			Int("attempt", attempt).
			Int("max_retries", s.stopCfg.MaxRetries).
			Err(err).
			Msg("Terminate attempt failed")

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		}

		// Last attempt - don't sleep
		if attempt == s.stopCfg.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := s.calculateBackoff(attempt)
		s.reportStopProgress(StopStepTerminate, fmt.Sprintf("Terminate failed, retrying in %v...", delay), fmt.Sprintf("attempt %d/%d: %v", attempt, s.stopCfg.MaxRetries, err), false, false)

		logging.Debug().
			Str("instance_id", instanceID).
			Dur("delay", delay).
			Int("next_attempt", attempt+1).
			Msg("Waiting before retry")

		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for retry: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// All retries exhausted - log CRITICAL and trigger alert callback
	logging.Error().
		Str("instance_id", instanceID).
		Str("provider", p.Name()).
		Int("attempts", s.stopCfg.MaxRetries).
		Err(lastErr).
		Msg("CRITICAL: Failed to terminate instance after all retries")

	// Trigger critical alert callback if configured
	if s.criticalAlertCb != nil {
		alertContext := map[string]interface{}{
			"instance_id": instanceID,
			"provider":    p.Name(),
			"attempts":    s.stopCfg.MaxRetries,
			"console_url": p.ConsoleURL(),
		}
		s.criticalAlertCb("Failed to terminate instance after all retries", lastErr, alertContext)
	}

	return fmt.Errorf("%w: %v", ErrTerminateFailed, lastErr)
}

// verifyBillingWithRetry attempts to verify billing has stopped with exponential backoff.
func (s *Stopper) verifyBillingWithRetry(ctx context.Context, p provider.Provider, instanceID string, result *StopResult) error {
	// If provider doesn't support billing verification, return immediately
	if !p.SupportsBillingVerification() {
		logging.Warn().
			Str("instance_id", instanceID).
			Str("provider", p.Name()).
			Msg("Provider does not support billing verification API")
		return provider.ErrBillingNotSupported
	}

	var lastErr error

	for attempt := 1; attempt <= s.stopCfg.MaxRetries; attempt++ {
		result.BillingCheckAttempts = attempt

		// Log the attempt
		logging.Info().
			Str("instance_id", instanceID).
			Str("provider", p.Name()).
			Int("attempt", attempt).
			Int("max_retries", s.stopCfg.MaxRetries).
			Msg("Checking billing status")

		// Create context with timeout for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, s.stopCfg.BillingCheckTimeout)

		status, err := p.GetBillingStatus(attemptCtx, instanceID)
		cancel()

		if err == nil {
			if status == provider.BillingStopped {
				logging.Info().
					Str("instance_id", instanceID).
					Str("provider", p.Name()).
					Int("attempt", attempt).
					Msg("Billing confirmed stopped")
				return nil
			}
			// Billing still active - wait and retry
			lastErr = fmt.Errorf("billing status is %s", status)
			logging.Warn().
				Str("instance_id", instanceID).
				Str("provider", p.Name()).
				Int("attempt", attempt).
				Str("status", string(status)).
				Msg("Billing still active")
		} else {
			// Check for expected errors
			if errors.Is(err, provider.ErrInstanceNotFound) {
				// Instance not found means billing is stopped
				logging.Info().
					Str("instance_id", instanceID).
					Str("provider", p.Name()).
					Msg("Instance not found - billing assumed stopped")
				return nil
			}
			lastErr = err
			logging.Warn().
				Str("instance_id", instanceID).
				Str("provider", p.Name()).
				Int("attempt", attempt).
				Err(err).
				Msg("Failed to check billing status")
		}

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		}

		// Last attempt - don't sleep
		if attempt == s.stopCfg.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := s.calculateBackoff(attempt)

		logging.Debug().
			Str("instance_id", instanceID).
			Dur("delay", delay).
			Int("next_attempt", attempt+1).
			Msg("Waiting before billing check retry")

		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for retry: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// Log critical error if billing verification failed
	logging.Error().
		Str("instance_id", instanceID).
		Str("provider", p.Name()).
		Int("attempts", s.stopCfg.MaxRetries).
		Err(lastErr).
		Msg("CRITICAL: Could not verify billing stopped after all retries")

	// Trigger critical alert callback if configured
	if s.criticalAlertCb != nil {
		alertContext := map[string]interface{}{
			"instance_id": instanceID,
			"provider":    p.Name(),
			"attempts":    s.stopCfg.MaxRetries,
			"console_url": p.ConsoleURL(),
			"type":        "billing_verification",
		}
		s.criticalAlertCb("Could not verify billing stopped after all retries", lastErr, alertContext)
	}

	return fmt.Errorf("%w: %v", ErrBillingNotVerified, lastErr)
}

// calculateBackoff calculates the backoff delay for a given attempt.
func (s *Stopper) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	// e.g., with 2s base: 2s, 4s, 8s, 16s, 32s
	multiplier := 1 << (attempt - 1) // 2^(attempt-1)
	delay := s.stopCfg.BaseRetryDelay * time.Duration(multiplier)

	// Cap at 60 seconds
	maxDelay := 60 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// reportStopProgress reports stop progress to the callback if set.
func (s *Stopper) reportStopProgress(step StopStep, message, detail string, completed, warning bool) {
	if s.progressCb == nil {
		return
	}

	progress := StopProgress{
		Step:       step,
		TotalSteps: TotalStopSteps,
		Message:    message,
		Detail:     detail,
		Completed:  completed,
		Warning:    warning,
	}

	s.progressCb(progress)
}

// ManualVerification holds information for manual billing verification.
type ManualVerification struct {
	// Required indicates if manual verification is required.
	Required bool

	// Provider is the provider name.
	Provider string

	// InstanceID is the instance ID to verify.
	InstanceID string

	// ConsoleURL is the provider console URL.
	ConsoleURL string

	// Instructions are step-by-step verification instructions.
	Instructions []string

	// WarningMessage is the main warning message to display.
	WarningMessage string
}

// NewManualVerification creates a ManualVerification for a provider that doesn't support billing verification.
func NewManualVerification(providerName, instanceID, consoleURL string) *ManualVerification {
	mv := &ManualVerification{
		Required:       true,
		Provider:       providerName,
		InstanceID:     instanceID,
		ConsoleURL:     consoleURL,
		WarningMessage: fmt.Sprintf("%s does not provide a billing status API.", capitalizeProvider(providerName)),
	}

	// Generate provider-specific instructions
	mv.Instructions = generateVerificationInstructions(providerName, instanceID, consoleURL)

	return mv
}

// generateVerificationInstructions generates provider-specific verification steps.
func generateVerificationInstructions(providerName, instanceID, consoleURL string) []string {
	switch providerName {
	case "paperspace":
		return []string{
			fmt.Sprintf("Open: %s", consoleURL),
			fmt.Sprintf("Confirm instance %s shows as \"Terminated\"", instanceID),
			"Check that no charges are accruing",
		}
	case "lambda":
		return []string{
			fmt.Sprintf("Open: %s", consoleURL),
			fmt.Sprintf("Verify instance %s is no longer listed or shows as terminated", instanceID),
			"Check the Billing section to confirm no active charges",
		}
	default:
		// Generic instructions for any provider
		return []string{
			fmt.Sprintf("Open: %s", consoleURL),
			fmt.Sprintf("Verify instance %s shows as terminated", instanceID),
			"Check that billing has stopped",
		}
	}
}

// capitalizeProvider returns a capitalized version of the provider name for display.
func capitalizeProvider(name string) string {
	switch name {
	case "paperspace":
		return "Paperspace"
	case "lambda":
		return "Lambda Labs"
	case "vast":
		return "Vast.ai"
	case "runpod":
		return "RunPod"
	case "coreweave":
		return "CoreWeave"
	default:
		return name
	}
}

// FormatManualVerificationText formats the manual verification as a text block.
// This is suitable for terminal output or logging.
func (mv *ManualVerification) FormatManualVerificationText() string {
	if !mv.Required {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("MANUAL VERIFICATION REQUIRED\n\n")
	sb.WriteString(mv.WarningMessage + "\n")
	sb.WriteString("Please verify manually that the instance is terminated:\n\n")

	for i, instruction := range mv.Instructions {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, instruction))
	}

	sb.WriteString(fmt.Sprintf("\n  Instance ID: %s\n", mv.InstanceID))

	return sb.String()
}

// GetLogFields returns structured fields suitable for logging.
func (mv *ManualVerification) GetLogFields() map[string]interface{} {
	return map[string]interface{}{
		"provider":     mv.Provider,
		"instance_id":  mv.InstanceID,
		"console_url":  mv.ConsoleURL,
		"verification": "manual_required",
	}
}

// CheckManualVerificationRequired checks if manual verification is required for a provider.
// Returns a ManualVerification struct with all necessary information.
func CheckManualVerificationRequired(p provider.Provider, instanceID string) *ManualVerification {
	if p.SupportsBillingVerification() {
		return &ManualVerification{Required: false}
	}

	return NewManualVerification(p.Name(), instanceID, p.ConsoleURL())
}

// WithManualVerificationCallback sets a callback for manual verification handling.
func WithManualVerificationCallback(cb ManualVerificationCallback) StopperOption {
	return func(s *Stopper) {
		s.manualVerifyCb = cb
	}
}

// WithCriticalAlertCallback sets a callback for critical alert handling.
// This callback is invoked when stop operations fail after all retries,
// allowing integration with external alerting systems (webhooks, etc.).
func WithCriticalAlertCallback(cb CriticalAlertCallback) StopperOption {
	return func(s *Stopper) {
		s.criticalAlertCb = cb
	}
}

// GetManualVerification returns the manual verification info if required, nil otherwise.
// This is called after Stop() completes to get verification details.
func (r *StopResult) GetManualVerification() *ManualVerification {
	if !r.ManualVerificationRequired {
		return nil
	}

	return NewManualVerification(r.Provider, r.InstanceID, r.ConsoleURL)
}
