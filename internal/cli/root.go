// Package cli provides the Cobra CLI commands for continueplz.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tmeurs/continueplz/internal/logging"
)

// Version information set at build time
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Global flags
var (
	cheapest   bool
	provider   string
	gpu        string
	model      string
	tier       string
	spot       bool
	onDemand   bool
	region     string
	stop       bool
	output     string
	timeout    string
	yes        bool
	verbose    int
)

// showVersion tracks if --version was requested
var showVersion bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "continueplz",
	Short: "Ephemeral GPU Code Assistant",
	Long: `continueplz - Ephemeral GPU instances for code-assist LLMs

A CLI tool that spins up ephemeral GPU instances with code-assist LLMs.
It compares prices across cloud GPU providers, deploys models via Ollama,
sets up WireGuard tunnels, and guarantees cleanup.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize logging with verbosity from command line flags
		// verbose is a count: 0 = default, 1 = -v, 2+ = -vv
		cfg := logging.Config{
			LogFile:       "continueplz.log",
			Verbosity:     verbose,
			ConsoleOutput: verbose > 0, // Enable console output if any verbosity is set
		}
		if err := logging.Init(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize logging: %v\n", err)
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Close the logger to ensure all logs are flushed
		logging.Close()
	},
	Run: func(cmd *cobra.Command, args []string) {
		log := logging.Get()
		jsonOutput := IsJSONOutput()

		// Handle --version flag
		if showVersion {
			if jsonOutput {
				PrintJSON(map[string]string{
					"version": Version,
					"commit":  Commit,
					"date":    Date,
				})
			} else {
				fmt.Printf("continueplz %s (commit: %s, built: %s)\n", Version, Commit, Date)
			}
			return
		}

		// If --stop flag is set, stop the instance
		if stop {
			err := RunStop()
			if err != nil {
				log.Error().Err(err).Msg("Stop failed")
				if !jsonOutput {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				}
				os.Exit(1)
			}
			return
		}

		// If --cheapest flag is set, do non-interactive deployment
		if cheapest {
			// Determine spot preference: --on-demand overrides --spot
			preferSpot := spot && !onDemand
			err := RunCheapestDeploy(model, provider, gpu, region, preferSpot, timeout)
			if err != nil {
				log.Error().Err(err).Msg("Deployment failed")
				if !jsonOutput {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				}
				os.Exit(1)
			}
			return
		}

		// JSON output mode is incompatible with interactive TUI
		if jsonOutput {
			PrintJSONError(fmt.Errorf("--output=json requires --cheapest, --stop, or status subcommand"))
			os.Exit(1)
		}

		// Default behavior: interactive TUI
		// If no instance is running, shows deployment flow
		// If instance is running, shows status view with actions
		log.Info().Msg("Launching continueplz")
		_, err := CheckAndRunInteractive()
		if err != nil {
			log.Error().Err(err).Msg("Interactive mode failed")
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Deployment flags
	rootCmd.Flags().BoolVar(&cheapest, "cheapest", false, "Select cheapest compatible provider/GPU automatically")
	rootCmd.Flags().StringVar(&provider, "provider", "", "Force specific provider (vast, lambda, runpod, coreweave, paperspace)")
	rootCmd.Flags().StringVar(&gpu, "gpu", "", "Force specific GPU type (a100-40, a100-80, a6000, h100)")
	rootCmd.Flags().StringVar(&model, "model", "qwen2.5-coder:32b", "Model to deploy (e.g., qwen2.5-coder:32b)")
	rootCmd.Flags().StringVar(&tier, "tier", "medium", "Model tier: small, medium, large")
	rootCmd.Flags().BoolVar(&spot, "spot", true, "Prefer spot instances")
	rootCmd.Flags().BoolVar(&onDemand, "on-demand", false, "Force on-demand instances")
	rootCmd.Flags().StringVar(&region, "region", "", "Preferred region (eu-west, us-east, etc.)")

	// Control flags
	rootCmd.Flags().BoolVar(&stop, "stop", false, "Stop running instance")

	// Output flags
	rootCmd.Flags().StringVar(&output, "output", "text", "Output format: text, json")
	rootCmd.Flags().StringVar(&timeout, "timeout", "10h", "Deadman switch timeout")

	// Convenience flags
	rootCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmations")
	rootCmd.Flags().CountVarP(&verbose, "verbose", "v", "Verbose logging (-v for info, -vv for debug)")

	// Version flag
	rootCmd.Flags().BoolVar(&showVersion, "version", false, "Show version information")
}

// SetVersion sets the version information for the version command
func SetVersion(version, commit, date string) {
	Version = version
	Commit = commit
	Date = date
}
