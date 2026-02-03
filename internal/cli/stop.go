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

// RunStop executes the --stop non-interactive stop flow.
// It outputs progress in the PRD format:
//
//	[1/4] Terminating instance 12345678...
//	      ✓ Terminated
//	[2/4] Verifying billing stopped...
//	      ✓ Confirmed
//	[3/4] Removing WireGuard tunnel...
//	      ✓ Removed
//	[4/4] Cleaning state...
//	      ✓ Done
//
//	Session cost: €2.93
//	Duration: 4h 28m
//
// When --output=json is set, it outputs JSON only at the end.
func RunStop() error {
	log := logging.Get()
	jsonOutput := IsJSONOutput()

	// Print header (skip in JSON mode)
	if !jsonOutput {
		fmt.Printf("\ncontinueplz %s - Stopping instance\n\n", Version)
	}

	// Set up context with cancellation on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		if !jsonOutput {
			fmt.Println("\n\nInterrupted. Cleanup may be incomplete - please verify manually.")
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

	// Create state manager
	stateManager, err := config.NewStateManager("")
	if err != nil {
		if jsonOutput {
			PrintJSONError(fmt.Errorf("failed to create state manager: %w", err))
		}
		return fmt.Errorf("failed to create state manager: %w", err)
	}

	// Check if there's an active instance
	existingState, err := stateManager.LoadState()
	if err != nil {
		if jsonOutput {
			PrintJSONError(fmt.Errorf("failed to load state: %w", err))
		}
		return fmt.Errorf("failed to load state: %w", err)
	}
	if existingState == nil || existingState.Instance == nil {
		if jsonOutput {
			output := StopOutput{
				Status: "no_active_instance",
			}
			PrintJSON(output)
		} else {
			fmt.Println("No active instance to stop.")
		}
		return nil
	}

	// Create stopper with progress callback (skip progress in JSON mode)
	var progressCb func(deploy.StopProgress)
	var manualVerifCb func(*deploy.ManualVerification)
	if !jsonOutput {
		progressCb = stopProgressCallback
		manualVerifCb = displayManualVerification
	}
	stopper, err := deploy.NewStopper(cfg, deploy.DefaultStopConfig(),
		deploy.WithStopProgressCallback(progressCb),
		deploy.WithStopStateManager(stateManager),
		deploy.WithManualVerificationCallback(manualVerifCb),
	)
	if err != nil {
		if jsonOutput {
			PrintJSONError(fmt.Errorf("failed to create stopper: %w", err))
		}
		return fmt.Errorf("failed to create stopper: %w", err)
	}

	// Run stop
	log.Info().
		Str("instance_id", existingState.Instance.ID).
		Str("provider", existingState.Instance.Provider).
		Msg("Stopping instance")

	result, err := stopper.Stop(ctx)
	if err != nil {
		// Check if we got a result despite the error (e.g., billing not verified)
		if result != nil {
			if jsonOutput {
				printStopSummaryJSON(result, err)
			} else {
				printStopSummary(result)
			}
		} else if jsonOutput {
			PrintJSONError(err)
		}
		return err
	}

	// Print success summary
	if jsonOutput {
		printStopSummaryJSON(result, nil)
	} else {
		printStopSummary(result)
	}

	return nil
}

// stopProgressCallback handles progress updates from the stopper.
// It formats output to match the PRD format.
func stopProgressCallback(progress deploy.StopProgress) {
	stepNum := int(progress.Step)
	totalSteps := progress.TotalSteps

	if progress.Completed {
		// Print completed step with checkmark or warning
		fmt.Printf("[%d/%d] %s\n", stepNum, totalSteps, progress.Step.String())
		if progress.Warning {
			fmt.Printf("      ⚠ %s\n\n", progress.Message)
		} else {
			fmt.Printf("      ✓ %s\n\n", progress.Message)
		}
	} else if progress.Error != nil {
		// Print error
		fmt.Printf("[%d/%d] %s\n", stepNum, totalSteps, progress.Step.String())
		fmt.Printf("      ✗ %s: %v\n\n", progress.Message, progress.Error)
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

// displayManualVerification displays manual verification information when required.
func displayManualVerification(mv *deploy.ManualVerification) {
	if mv == nil || !mv.Required {
		return
	}

	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Println("⚠ MANUAL VERIFICATION REQUIRED")
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println(mv.WarningMessage)
	fmt.Println()
	fmt.Println("Please verify manually that the instance is terminated:")
	fmt.Println()
	for i, instruction := range mv.Instructions {
		fmt.Printf("  %d. %s\n", i+1, instruction)
	}
	fmt.Println()
	fmt.Printf("  Instance ID: %s\n", mv.InstanceID)
	fmt.Println()
}

// printStopSummary prints the final stop summary.
func printStopSummary(result *deploy.StopResult) {
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Println("SESSION COMPLETE")
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Printf("  Session cost: €%.2f\n", result.SessionCost)
	fmt.Printf("  Duration:     %s\n", formatSessionDuration(result.SessionDuration))
	fmt.Println()

	// Show verification status
	if result.BillingVerified {
		fmt.Println("  Billing:      ✓ Confirmed stopped")
	} else if result.ManualVerificationRequired {
		fmt.Println("  Billing:      ⚠ Manual verification required")
	} else {
		fmt.Println("  Billing:      ✗ Could not verify")
	}
	fmt.Println()
}

// formatSessionDuration formats a duration for session display (e.g., "4h 28m").
func formatSessionDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// printStopSummaryJSON prints the stop summary in JSON format.
func printStopSummaryJSON(result *deploy.StopResult, stopErr error) {
	status := "stopped"
	if result.ManualVerificationRequired {
		status = "manual_verification_required"
	}
	if stopErr != nil && !result.ManualVerificationRequired {
		status = "error"
	}

	output := StopOutput{
		Status:                     status,
		InstanceID:                 result.InstanceID,
		Provider:                   result.Provider,
		BillingVerified:            result.BillingVerified,
		ManualVerificationRequired: result.ManualVerificationRequired,
		ConsoleURL:                 result.ConsoleURL,
		SessionCost:                result.SessionCost,
		SessionDuration:            formatSessionDuration(result.SessionDuration),
		SessionDurationSeconds:     int(result.SessionDuration.Seconds()),
	}

	if stopErr != nil {
		output.Error = stopErr.Error()
	}

	PrintJSON(output)
}
