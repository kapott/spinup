// Package cli provides the Cobra CLI commands for continueplz.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tmeurs/continueplz/internal/config"
	"github.com/tmeurs/continueplz/internal/deploy"
	"github.com/tmeurs/continueplz/internal/logging"
)

// RunCheapestDeploy executes the --cheapest non-interactive deployment flow.
// It outputs progress in the PRD format:
//
//	[1/8] Fetching prices from providers...
//	      ✓ vast.ai: 3 offers
//	[2/8] Selecting cheapest compatible option...
//	      ✓ Selected: vast.ai A100 40GB EU-West @ €0.65/hr spot
//
// ...and so on.
// When --output=json is set, it outputs JSON only at the end.
func RunCheapestDeploy(modelName, providerName, gpuType, regionName string, preferSpot bool, timeoutStr string) error {
	log := logging.Get()
	jsonOutput := IsJSONOutput()

	// Print header (skip in JSON mode)
	if !jsonOutput {
		fmt.Printf("\ncontinueplz %s - Starting deployment\n\n", Version)
	}

	// Set up context with cancellation on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		if !jsonOutput {
			fmt.Println("\n\nInterrupted. Cleaning up...")
		}
		cancel()
	}()

	// Load configuration
	cfg, warnings, err := config.LoadConfig("")
	if err != nil {
		if jsonOutput {
			PrintJSONError(fmt.Errorf("failed to load config: %w", err))
		}
		return fmt.Errorf("failed to load config: %w", err)
	}
	for _, w := range warnings {
		log.Warn().Msg(w)
	}

	// Validate that at least one provider is configured
	if !cfg.HasAnyProvider() {
		err := fmt.Errorf("no providers configured - run 'continueplz init' first or set API keys in .env")
		if jsonOutput {
			PrintJSONError(err)
		}
		return err
	}

	// Parse timeout
	deadmanHours := parseTimeout(timeoutStr)

	// Create deploy configuration
	deployCfg := deploy.DefaultDeployConfig()
	deployCfg.Model = modelName
	deployCfg.PreferSpot = preferSpot
	deployCfg.ProviderName = providerName
	deployCfg.GPUType = gpuType
	deployCfg.Region = regionName
	deployCfg.DeadmanTimeoutHours = deadmanHours

	// Create state manager
	stateManager, err := config.NewStateManager("")
	if err != nil {
		if jsonOutput {
			PrintJSONError(fmt.Errorf("failed to create state manager: %w", err))
		}
		return fmt.Errorf("failed to create state manager: %w", err)
	}

	// Check if there's already an active instance
	existingState, _ := stateManager.LoadState()
	if existingState != nil && existingState.Instance != nil {
		err := fmt.Errorf("an instance is already running (ID: %s). Use --stop first", existingState.Instance.ID)
		if jsonOutput {
			PrintJSONError(err)
		}
		return err
	}

	// Create deployer with progress callback (skip in JSON mode)
	var progressCb func(deploy.DeployProgress)
	if !jsonOutput {
		progressCb = cheapestProgressCallback
	}
	deployer, err := deploy.NewDeployer(cfg, deployCfg,
		deploy.WithProgressCallback(progressCb),
		deploy.WithStateManager(stateManager),
	)
	if err != nil {
		if jsonOutput {
			PrintJSONError(fmt.Errorf("failed to create deployer: %w", err))
		}
		return fmt.Errorf("failed to create deployer: %w", err)
	}

	// Run deployment
	log.Info().
		Str("model", modelName).
		Str("provider", providerName).
		Str("gpu", gpuType).
		Str("region", regionName).
		Bool("spot", preferSpot).
		Msg("Starting deployment")

	result, err := deployer.Deploy(ctx)
	if err != nil {
		if jsonOutput {
			PrintJSONError(err)
		}
		return err
	}

	// Print success summary
	if jsonOutput {
		printDeploymentSummaryJSON(result, deadmanHours)
	} else {
		printDeploymentSummary(result)
	}

	return nil
}

// cheapestProgressCallback handles progress updates from the deployer.
// It formats output to match the PRD format.
func cheapestProgressCallback(progress deploy.DeployProgress) {
	stepNum := int(progress.Step)
	totalSteps := progress.TotalSteps

	if progress.Completed {
		// Print completed step with checkmark
		fmt.Printf("[%d/%d] %s\n", stepNum, totalSteps, progress.Step.String())
		fmt.Printf("      ✓ %s\n\n", progress.Message)
	} else if progress.Error != nil {
		// Print error
		fmt.Printf("[%d/%d] %s\n", stepNum, totalSteps, progress.Step.String())
		fmt.Printf("      ✗ %s: %s\n\n", progress.Message, progress.Error.Error())
	} else {
		// Print in-progress step
		fmt.Printf("[%d/%d] %s\n", stepNum, totalSteps, progress.Step.String())
		if progress.Detail != "" {
			fmt.Printf("      ⋯ %s (%s)\n", progress.Message, progress.Detail)
		} else {
			fmt.Printf("      ⋯ %s\n", progress.Message)
		}
	}
}

// printDeploymentSummary prints the final deployment summary.
func printDeploymentSummary(result *deploy.DeployResult) {
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Println("DEPLOYMENT COMPLETE")
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Printf("  Provider:    %s\n", result.Provider.Name())
	fmt.Printf("  GPU:         %s\n", result.SelectedOffer.GPU)
	fmt.Printf("  Region:      %s\n", result.SelectedOffer.Region)
	fmt.Printf("  Model:       %s\n", result.Model.Name)
	fmt.Printf("  Instance ID: %s\n", result.Instance.ID)
	fmt.Printf("  Public IP:   %s\n", result.Instance.PublicIP)

	// Print pricing
	priceType := "on-demand"
	hourlyRate := result.SelectedOffer.OnDemandPrice
	if result.Instance.Spot && result.SelectedOffer.SpotPrice != nil {
		priceType = "spot"
		hourlyRate = *result.SelectedOffer.SpotPrice
	}
	fmt.Printf("  Pricing:     €%.2f/hr (%s)\n", hourlyRate, priceType)

	fmt.Println()
	fmt.Println("CONNECT")
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Printf("  Ollama API:  %s\n", result.OllamaEndpoint)
	fmt.Println()
	fmt.Println("  Configure your editor to use:")
	fmt.Printf("    OLLAMA_HOST=%s\n", result.OllamaEndpoint)
	fmt.Println()
	fmt.Printf("  Deployment took %s\n", formatDuration(result.Duration()))
	fmt.Println()
	fmt.Println("To stop the instance and cleanup:")
	fmt.Println("  continueplz --stop")
	fmt.Println()
}

// formatDuration formats a duration for display.
// Handles hours for longer durations (used by status command).
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "0m"
	}
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}

// parseTimeout parses a timeout string like "10h" into hours.
// Defaults to 10 hours if parsing fails.
func parseTimeout(timeoutStr string) int {
	if timeoutStr == "" {
		return 10
	}

	// Try parsing as hours (e.g., "10h" or "10")
	var hours int
	n, err := fmt.Sscanf(timeoutStr, "%dh", &hours)
	if err == nil && n == 1 && hours > 0 {
		return hours
	}

	// Try parsing as plain number
	n, err = fmt.Sscanf(timeoutStr, "%d", &hours)
	if err == nil && n == 1 && hours > 0 {
		return hours
	}

	// Default to 10 hours
	return 10
}

// printDeploymentSummaryJSON prints the deployment summary in JSON format.
// Matches PRD Section 3.2 JSON format.
func printDeploymentSummaryJSON(result *deploy.DeployResult, deadmanHours int) {
	// Determine pricing type and rate
	instanceType := "on-demand"
	hourlyRate := result.SelectedOffer.OnDemandPrice
	if result.Instance.Spot && result.SelectedOffer.SpotPrice != nil {
		instanceType = "spot"
		hourlyRate = *result.SelectedOffer.SpotPrice
	}

	// Extract WireGuard IP from endpoint if available
	wireguardIP := ""
	if result.WireGuardConfig != nil && result.WireGuardConfig.Client != nil {
		// ClientAddress is in CIDR format (e.g., "10.13.37.2/24"), extract just the IP
		addr := result.WireGuardConfig.Client.ClientAddress
		if idx := len(addr) - 1; idx > 0 {
			for i := idx; i >= 0; i-- {
				if addr[i] == '/' {
					wireguardIP = addr[:i]
					break
				}
			}
			if wireguardIP == "" {
				wireguardIP = addr
			}
		}
	}

	output := DeployOutput{
		Status: "ready",
		Instance: &DeployInstanceInfo{
			ID:       result.Instance.ID,
			Provider: result.Provider.Name(),
			GPU:      result.SelectedOffer.GPU,
			Region:   result.SelectedOffer.Region,
			Type:     instanceType,
			PublicIP: result.Instance.PublicIP,
		},
		Model: result.Model.Name,
		Cost: &CostInfo{
			Hourly:   hourlyRate,
			Currency: "EUR",
		},
		Duration: formatDuration(result.Duration()),
	}

	// Add endpoint info if WireGuard is configured
	if wireguardIP != "" {
		output.Endpoint = &EndpointInfo{
			WireGuardIP: wireguardIP,
			Port:        11434,
			URL:         result.OllamaEndpoint,
		}
	}

	// Add deadman info
	remainingSeconds := deadmanHours * 3600
	output.Deadman = &DeadmanInfo{
		Active:           true,
		TimeoutHours:     deadmanHours,
		RemainingSeconds: remainingSeconds,
	}

	PrintJSON(output)
}
