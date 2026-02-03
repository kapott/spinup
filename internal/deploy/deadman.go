// Package deploy provides deployment orchestration for continueplz.
package deploy

import (
	"fmt"
	"strings"
	"time"
)

// DeadmanConfig represents the configuration for the deadman switch.
// The deadman switch automatically terminates the instance if no heartbeat
// is received within the timeout period.
type DeadmanConfig struct {
	// TimeoutSeconds is the deadman switch timeout in seconds.
	// If no heartbeat is received within this time, the instance self-terminates.
	// Default is 36000 seconds (10 hours).
	TimeoutSeconds int

	// HeartbeatFile is the path to the heartbeat file on the instance.
	// The client must periodically touch this file to prevent termination.
	// Default is /tmp/continueplz-heartbeat
	HeartbeatFile string

	// CheckIntervalSeconds is how often the deadman script checks the heartbeat file.
	// Default is 60 seconds.
	CheckIntervalSeconds int
}

// DeadmanStatus represents the current status of the deadman switch.
type DeadmanStatus struct {
	// Active indicates whether the deadman switch is active.
	Active bool

	// TimeoutSeconds is the configured timeout in seconds.
	TimeoutSeconds int

	// RemainingSeconds is the estimated time until termination.
	// This is based on the last known heartbeat time.
	RemainingSeconds int

	// LastHeartbeat is the last known heartbeat time.
	LastHeartbeat time.Time
}

// ProviderTerminationInfo contains provider-specific termination information
// used by the deadman switch to self-terminate the instance.
type ProviderTerminationInfo struct {
	// Provider is the cloud provider name.
	Provider string

	// InstanceID is the provider-specific instance identifier.
	InstanceID string

	// APIKey is the API key used for self-termination.
	APIKey string

	// APIURL is the base URL for the provider's API.
	APIURL string

	// HTTPMethod is the HTTP method to use (DELETE, POST, etc.)
	HTTPMethod string

	// AuthHeader is the authentication header name.
	AuthHeader string

	// AuthValue is the authentication header value format.
	AuthValue string

	// ContentType is the content type for POST requests.
	ContentType string

	// Body is the request body for POST requests (template with ${INSTANCE_ID} placeholder).
	Body string
}

// Constants for deadman switch configuration.
const (
	// DefaultDeadmanTimeout is the default deadman timeout (10 hours).
	DefaultDeadmanTimeout = 10 * time.Hour

	// DefaultHeartbeatFile is the default heartbeat file path on the instance.
	DefaultHeartbeatFile = "/tmp/continueplz-heartbeat"

	// DefaultCheckInterval is the default check interval (60 seconds).
	DefaultCheckInterval = 60 * time.Second

	// MinDeadmanTimeout is the minimum allowed deadman timeout (1 hour).
	MinDeadmanTimeout = 1 * time.Hour

	// MaxDeadmanTimeout is the maximum allowed deadman timeout (72 hours).
	MaxDeadmanTimeout = 72 * time.Hour
)

// NewDeadmanConfig creates a new DeadmanConfig with default values.
func NewDeadmanConfig() *DeadmanConfig {
	return &DeadmanConfig{
		TimeoutSeconds:       int(DefaultDeadmanTimeout.Seconds()),
		HeartbeatFile:        DefaultHeartbeatFile,
		CheckIntervalSeconds: int(DefaultCheckInterval.Seconds()),
	}
}

// NewDeadmanConfigWithTimeout creates a new DeadmanConfig with the specified timeout.
func NewDeadmanConfigWithTimeout(timeout time.Duration) *DeadmanConfig {
	cfg := NewDeadmanConfig()
	cfg.TimeoutSeconds = int(timeout.Seconds())
	return cfg
}

// Validate validates the DeadmanConfig.
func (c *DeadmanConfig) Validate() error {
	if c.TimeoutSeconds <= 0 {
		return fmt.Errorf("deadman timeout must be positive")
	}
	if c.TimeoutSeconds < int(MinDeadmanTimeout.Seconds()) {
		return fmt.Errorf("deadman timeout must be at least %v", MinDeadmanTimeout)
	}
	if c.TimeoutSeconds > int(MaxDeadmanTimeout.Seconds()) {
		return fmt.Errorf("deadman timeout must not exceed %v", MaxDeadmanTimeout)
	}
	if c.HeartbeatFile == "" {
		return fmt.Errorf("heartbeat file path is required")
	}
	if c.CheckIntervalSeconds <= 0 {
		return fmt.Errorf("check interval must be positive")
	}
	return nil
}

// Timeout returns the timeout as a time.Duration.
func (c *DeadmanConfig) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// TimeoutHours returns the timeout in hours (rounded down).
func (c *DeadmanConfig) TimeoutHours() int {
	return c.TimeoutSeconds / 3600
}

// RemainingTime calculates the remaining time until deadman triggers
// based on the last heartbeat time.
func (c *DeadmanConfig) RemainingTime(lastHeartbeat time.Time) time.Duration {
	if lastHeartbeat.IsZero() {
		return 0
	}
	elapsed := time.Since(lastHeartbeat)
	remaining := c.Timeout() - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsExpired returns true if the deadman should have triggered based on
// the last heartbeat time.
func (c *DeadmanConfig) IsExpired(lastHeartbeat time.Time) bool {
	return c.RemainingTime(lastHeartbeat) == 0
}

// GetTerminationInfo returns the provider-specific termination information
// for the deadman switch self-termination.
func GetTerminationInfo(provider, instanceID, apiKey string) (*ProviderTerminationInfo, error) {
	provider = strings.ToLower(provider)

	switch provider {
	case "vast":
		return &ProviderTerminationInfo{
			Provider:   "vast",
			InstanceID: instanceID,
			APIKey:     apiKey,
			APIURL:     "https://console.vast.ai/api/v0/instances/" + instanceID + "/",
			HTTPMethod: "DELETE",
			AuthHeader: "Authorization",
			AuthValue:  "Bearer " + apiKey,
		}, nil

	case "lambda":
		return &ProviderTerminationInfo{
			Provider:    "lambda",
			InstanceID:  instanceID,
			APIKey:      apiKey,
			APIURL:      "https://cloud.lambdalabs.com/api/v1/instance-operations/terminate",
			HTTPMethod:  "POST",
			AuthHeader:  "Authorization",
			AuthValue:   "Bearer " + apiKey,
			ContentType: "application/json",
			Body:        `{"instance_ids": ["` + instanceID + `"]}`,
		}, nil

	case "runpod":
		return &ProviderTerminationInfo{
			Provider:    "runpod",
			InstanceID:  instanceID,
			APIKey:      apiKey,
			APIURL:      "https://api.runpod.io/graphql",
			HTTPMethod:  "POST",
			AuthHeader:  "Authorization",
			AuthValue:   "Bearer " + apiKey,
			ContentType: "application/json",
			Body:        `{"query":"mutation { podTerminate(input: {podId: \"` + instanceID + `\"}) { id } }"}`,
		}, nil

	case "coreweave":
		return &ProviderTerminationInfo{
			Provider:   "coreweave",
			InstanceID: instanceID,
			APIKey:     apiKey,
			APIURL:     "https://api.coreweave.com/v1/instances/" + instanceID,
			HTTPMethod: "DELETE",
			AuthHeader: "Authorization",
			AuthValue:  "Bearer " + apiKey,
		}, nil

	case "paperspace":
		return &ProviderTerminationInfo{
			Provider:   "paperspace",
			InstanceID: instanceID,
			APIKey:     apiKey,
			APIURL:     "https://api.paperspace.io/machines/" + instanceID + "/destroyMachine",
			HTTPMethod: "POST",
			AuthHeader: "x-api-key",
			AuthValue:  apiKey,
		}, nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// GenerateCurlCommand generates the curl command for self-termination
// based on the provider termination info.
func (info *ProviderTerminationInfo) GenerateCurlCommand() string {
	var cmd strings.Builder
	cmd.WriteString("curl -X ")
	cmd.WriteString(info.HTTPMethod)
	cmd.WriteString(" \"")
	cmd.WriteString(info.APIURL)
	cmd.WriteString("\"")

	// Add auth header
	cmd.WriteString(" -H \"")
	cmd.WriteString(info.AuthHeader)
	cmd.WriteString(": ")
	cmd.WriteString(info.AuthValue)
	cmd.WriteString("\"")

	// Add content type if specified
	if info.ContentType != "" {
		cmd.WriteString(" -H \"Content-Type: ")
		cmd.WriteString(info.ContentType)
		cmd.WriteString("\"")
	}

	// Add body if specified
	if info.Body != "" {
		cmd.WriteString(" -d '")
		cmd.WriteString(info.Body)
		cmd.WriteString("'")
	}

	return cmd.String()
}

// ValidateProvider validates that the provider is known and supported.
func ValidateProvider(provider string) error {
	validProviders := map[string]bool{
		"vast":       true,
		"lambda":     true,
		"runpod":     true,
		"coreweave":  true,
		"paperspace": true,
	}
	if !validProviders[strings.ToLower(provider)] {
		return fmt.Errorf("unknown provider: %s (valid: vast, lambda, runpod, coreweave, paperspace)", provider)
	}
	return nil
}

// SupportedProviders returns a list of all supported providers.
func SupportedProviders() []string {
	return []string{"vast", "lambda", "runpod", "coreweave", "paperspace"}
}

// NewDeadmanStatus creates a DeadmanStatus from the configuration and last heartbeat.
func NewDeadmanStatus(cfg *DeadmanConfig, lastHeartbeat time.Time) *DeadmanStatus {
	if cfg == nil {
		return &DeadmanStatus{Active: false}
	}

	remaining := cfg.RemainingTime(lastHeartbeat)
	return &DeadmanStatus{
		Active:           true,
		TimeoutSeconds:   cfg.TimeoutSeconds,
		RemainingSeconds: int(remaining.Seconds()),
		LastHeartbeat:    lastHeartbeat,
	}
}

// FormatRemaining returns a human-readable string for the remaining time.
func (s *DeadmanStatus) FormatRemaining() string {
	if !s.Active {
		return "inactive"
	}
	if s.RemainingSeconds <= 0 {
		return "expired"
	}

	d := time.Duration(s.RemainingSeconds) * time.Second
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm remaining", hours, minutes)
	}
	return fmt.Sprintf("%dm remaining", minutes)
}

// IsExpired returns true if the deadman has triggered or will trigger soon.
func (s *DeadmanStatus) IsExpired() bool {
	return s.Active && s.RemainingSeconds <= 0
}

// IsWarning returns true if the remaining time is below the warning threshold (1 hour).
func (s *DeadmanStatus) IsWarning() bool {
	return s.Active && s.RemainingSeconds > 0 && s.RemainingSeconds < 3600
}
