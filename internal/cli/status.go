package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tmeurs/continueplz/internal/config"
)

// StatusOutput represents the JSON output structure for status command.
// Matches PRD Section 3.2 JSON format.
type StatusOutput struct {
	Status   string               `json:"status"` // "ready", "loading", "none_active"
	Instance *StatusInstanceInfo  `json:"instance,omitempty"`
	Model    string               `json:"model,omitempty"`
	Endpoint *StatusEndpointInfo  `json:"endpoint,omitempty"`
	Cost     *StatusCostInfo      `json:"cost,omitempty"`
	Deadman  *StatusDeadmanInfo   `json:"deadman,omitempty"`
}

// StatusInstanceInfo contains instance information for status output.
type StatusInstanceInfo struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	GPU      string `json:"gpu"`
	Region   string `json:"region"`
	Type     string `json:"type"` // "spot" or "on-demand"
}

// StatusEndpointInfo contains endpoint information for status output.
type StatusEndpointInfo struct {
	WireGuardIP string `json:"wireguard_ip"`
	Port        int    `json:"port"`
	URL         string `json:"url"`
}

// StatusCostInfo contains cost information for status output.
type StatusCostInfo struct {
	Hourly      float64 `json:"hourly"`
	Accumulated float64 `json:"accumulated"`
	Currency    string  `json:"currency"`
}

// StatusDeadmanInfo contains deadman switch information for status output.
type StatusDeadmanInfo struct {
	TimeoutHours   int    `json:"timeout_hours"`
	RemainingHours int    `json:"remaining_hours"`
	RemainingStr   string `json:"remaining"`
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current instance status",
	Long: `Display the current status of your GPU instance.

If an instance is running, this will show:
- Provider and GPU information
- Model being served
- Endpoint for API access
- Running time and cost
- Deadman switch timer

If no instance is running, it will indicate that.

Use --output=json for machine-readable output.`,
	Run: runStatusCmd,
}

func runStatusCmd(cmd *cobra.Command, args []string) {
	outputFormat, _ := cmd.Flags().GetString("output")

	// Load state
	stateManager, err := config.NewStateManager("")
	if err != nil {
		if outputFormat == "json" {
			printStatusError(err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: failed to initialize state manager: %v\n", err)
		}
		os.Exit(1)
	}

	state, err := stateManager.LoadState()
	if err != nil {
		if outputFormat == "json" {
			printStatusError(err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: failed to load state: %v\n", err)
		}
		os.Exit(1)
	}

	// Check if there's an active instance
	if state == nil || state.Instance == nil {
		printNoActiveInstance(outputFormat)
		return
	}

	// Print status for active instance
	if outputFormat == "json" {
		printStatusJSON(state)
	} else {
		printStatusText(state)
	}
}

// printNoActiveInstance prints the "no active instance" message.
func printNoActiveInstance(outputFormat string) {
	if outputFormat == "json" {
		output := StatusOutput{
			Status: "none_active",
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Text format
	fmt.Printf("continueplz %s - Status\n", Version)
	fmt.Println("")
	fmt.Println("Instance:     ○ None active")
	fmt.Println("")
	fmt.Println("Run 'continueplz' to start a new instance.")
}

// printStatusJSON prints status in JSON format per PRD Section 3.2.
func printStatusJSON(state *config.State) {
	output := StatusOutput{
		Status: getStatusFromState(state),
	}

	// Instance info
	if state.Instance != nil {
		output.Instance = &StatusInstanceInfo{
			ID:       state.Instance.ID,
			Provider: state.Instance.Provider,
			GPU:      state.Instance.GPU,
			Region:   state.Instance.Region,
			Type:     state.Instance.Type,
		}
	}

	// Model info
	if state.Model != nil {
		output.Model = state.Model.Name
	}

	// Endpoint info
	if state.Instance != nil && state.Instance.WireGuardIP != "" {
		output.Endpoint = &StatusEndpointInfo{
			WireGuardIP: state.Instance.WireGuardIP,
			Port:        11434, // Ollama default port
			URL:         fmt.Sprintf("http://%s:11434", state.Instance.WireGuardIP),
		}
	}

	// Cost info
	if state.Cost != nil {
		accumulated := state.CalculateAccumulatedCost()
		output.Cost = &StatusCostInfo{
			Hourly:      state.Cost.HourlyRate,
			Accumulated: accumulated,
			Currency:    state.Cost.Currency,
		}
	}

	// Deadman info
	if state.Deadman != nil {
		remaining := calculateDeadmanRemaining(state.Deadman)
		output.Deadman = &StatusDeadmanInfo{
			TimeoutHours:   state.Deadman.TimeoutHours,
			RemainingHours: int(remaining.Hours()),
			RemainingStr:   formatDuration(remaining),
		}
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(data))
}

// printStatusText prints status in text format per PRD Section 3.2.
func printStatusText(state *config.State) {
	fmt.Printf("continueplz %s - Status\n", Version)
	fmt.Println("")

	// Instance status
	fmt.Printf("Instance:     ● Active\n")

	// Provider
	if state.Instance != nil {
		fmt.Printf("Provider:     %s\n", state.Instance.Provider)
		fmt.Printf("GPU:          %s\n", state.Instance.GPU)
	}

	// Model
	if state.Model != nil {
		fmt.Printf("Model:        %s\n", state.Model.Name)
	}

	// Endpoint
	if state.Instance != nil && state.Instance.WireGuardIP != "" {
		fmt.Printf("Endpoint:     %s:11434\n", state.Instance.WireGuardIP)
	}

	// Running time
	if state.Instance != nil {
		duration := state.Instance.Duration()
		fmt.Printf("Running:      %s\n", formatDuration(duration))
	}

	// Cost
	if state.Cost != nil {
		accumulated := state.CalculateAccumulatedCost()
		fmt.Printf("Cost so far:  %s%.2f\n", getCurrencySymbol(state.Cost.Currency), accumulated)
	}

	// Deadman
	if state.Deadman != nil {
		remaining := calculateDeadmanRemaining(state.Deadman)
		if remaining > 0 {
			fmt.Printf("Deadman:      %s remaining\n", formatDuration(remaining))
		} else {
			fmt.Printf("Deadman:      Expired\n")
		}
	}
}

// printStatusError prints an error in JSON format.
func printStatusError(err error) {
	output := struct {
		Error  string `json:"error"`
		Status string `json:"status"`
	}{
		Error:  err.Error(),
		Status: "error",
	}
	data, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(data))
}

// getStatusFromState determines the status string based on state.
func getStatusFromState(state *config.State) string {
	if state == nil || state.Instance == nil {
		return "none_active"
	}
	if state.Model != nil {
		switch state.Model.Status {
		case "loading":
			return "loading"
		case "ready":
			return "ready"
		case "error":
			return "error"
		}
	}
	return "running"
}

// calculateDeadmanRemaining calculates the time remaining before deadman triggers.
func calculateDeadmanRemaining(deadman *config.DeadmanState) time.Duration {
	if deadman == nil {
		return 0
	}
	deadline := deadman.LastHeartbeat.Add(time.Duration(deadman.TimeoutHours) * time.Hour)
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// getCurrencySymbol returns the symbol for a currency code.
func getCurrencySymbol(currency string) string {
	switch currency {
	case "EUR":
		return "€"
	case "USD":
		return "$"
	case "GBP":
		return "£"
	default:
		return currency + " "
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Status command inherits --output flag from root, but we can also set it locally
	statusCmd.Flags().String("output", "text", "Output format: text, json")
}
