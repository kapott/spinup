// Package cli provides the Cobra CLI commands for continueplz.
package cli

import (
	"encoding/json"
	"fmt"
	"time"
)

// OutputFormat represents the output format for commands.
type OutputFormat string

const (
	OutputFormatText OutputFormat = "text"
	OutputFormatJSON OutputFormat = "json"
)

// GetOutputFormat returns the current output format from the global flag.
func GetOutputFormat() OutputFormat {
	if output == "json" {
		return OutputFormatJSON
	}
	return OutputFormatText
}

// IsJSONOutput returns true if JSON output mode is enabled.
func IsJSONOutput() bool {
	return output == "json"
}

// DeployOutput represents the JSON output structure for deploy command.
// Matches PRD Section 3.2 JSON format.
type DeployOutput struct {
	Status   string              `json:"status"` // "deploying", "ready", "error"
	Instance *DeployInstanceInfo `json:"instance,omitempty"`
	Model    string              `json:"model,omitempty"`
	Endpoint *EndpointInfo       `json:"endpoint,omitempty"`
	Cost     *CostInfo           `json:"cost,omitempty"`
	Deadman  *DeadmanInfo        `json:"deadman,omitempty"`
	Error    string              `json:"error,omitempty"`
	Duration string              `json:"duration,omitempty"`
}

// DeployInstanceInfo contains instance information for deploy output.
type DeployInstanceInfo struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	GPU      string `json:"gpu"`
	Region   string `json:"region"`
	Type     string `json:"type"` // "spot" or "on-demand"
	PublicIP string `json:"public_ip,omitempty"`
}

// EndpointInfo contains endpoint information for output.
type EndpointInfo struct {
	WireGuardIP string `json:"wireguard_ip"`
	Port        int    `json:"port"`
	URL         string `json:"url"`
}

// CostInfo contains cost information for output.
type CostInfo struct {
	Hourly      float64 `json:"hourly"`
	Accumulated float64 `json:"accumulated,omitempty"`
	Current     float64 `json:"current,omitempty"`
	Currency    string  `json:"currency"`
}

// DeadmanInfo contains deadman switch information for output.
type DeadmanInfo struct {
	Active           bool   `json:"active"`
	TimeoutHours     int    `json:"timeout_hours,omitempty"`
	RemainingSeconds int    `json:"remaining_seconds,omitempty"`
	RemainingHours   int    `json:"remaining_hours,omitempty"`
	Remaining        string `json:"remaining,omitempty"`
}

// StopOutput represents the JSON output structure for stop command.
type StopOutput struct {
	Status                     string   `json:"status"` // "stopped", "error", "manual_verification_required"
	InstanceID                 string   `json:"instance_id,omitempty"`
	Provider                   string   `json:"provider,omitempty"`
	BillingVerified            bool     `json:"billing_verified"`
	ManualVerificationRequired bool     `json:"manual_verification_required,omitempty"`
	ConsoleURL                 string   `json:"console_url,omitempty"`
	SessionCost                float64  `json:"session_cost,omitempty"`
	SessionDuration            string   `json:"session_duration,omitempty"`
	SessionDurationSeconds     int      `json:"session_duration_seconds,omitempty"`
	Error                      string   `json:"error,omitempty"`
}

// PrintJSON marshals and prints a value as JSON.
func PrintJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		// Fallback error output
		errOut := map[string]string{
			"status": "error",
			"error":  fmt.Sprintf("failed to marshal JSON: %v", err),
		}
		data, _ = json.MarshalIndent(errOut, "", "  ")
	}
	fmt.Println(string(data))
}

// PrintJSONError prints an error in JSON format.
func PrintJSONError(err error) {
	output := struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}{
		Status: "error",
		Error:  err.Error(),
	}
	PrintJSON(output)
}

// FormatDurationSeconds returns the duration as seconds.
func FormatDurationSeconds(d time.Duration) int {
	return int(d.Seconds())
}
