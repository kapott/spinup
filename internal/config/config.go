// Package config handles loading configuration from .env files and environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config represents the application configuration loaded from .env file.
type Config struct {
	// Provider API Keys (at least one required for operation)
	VastAPIKey       string
	LambdaAPIKey     string
	RunPodAPIKey     string
	CoreWeaveAPIKey  string
	PaperspaceAPIKey string

	// WireGuard configuration
	WireGuardPrivateKey string
	WireGuardPublicKey  string

	// User preferences
	DefaultTier          string // small, medium, large
	DefaultRegion        string
	PreferSpot           bool
	DeadmanTimeoutHours  int

	// Alerting (optional)
	AlertWebhookURL string
	DailyBudgetEUR  float64
}

// DefaultEnvPath is the default path for the .env file.
const DefaultEnvPath = ".env"

// ErrNoConfigFile indicates the .env file was not found.
var ErrNoConfigFile = errors.New("configuration file not found")

// ErrInsecurePermissions indicates the .env file has insecure permissions.
var ErrInsecurePermissions = errors.New("configuration file has insecure permissions (should be 0600)")

// ErrNoProviderConfigured indicates no provider API key was configured.
var ErrNoProviderConfigured = errors.New("at least one provider API key must be configured")

// LoadConfig loads configuration from the specified .env file path.
// If path is empty, it uses DefaultEnvPath.
// Returns the config and any warnings (e.g., permission issues) as a slice of strings.
func LoadConfig(path string) (*Config, []string, error) {
	if path == "" {
		path = DefaultEnvPath
	}

	var warnings []string

	// Check if file exists
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%w: %s", ErrNoConfigFile, absPath)
		}
		return nil, nil, fmt.Errorf("failed to stat config file: %w", err)
	}

	// Check file permissions (Unix only)
	mode := info.Mode().Perm()
	if mode != 0o600 {
		warnings = append(warnings, fmt.Sprintf(
			"config file %s has permissions %04o, should be 0600 for security",
			absPath, mode,
		))
	}

	// Load the .env file
	if err := godotenv.Load(absPath); err != nil {
		return nil, nil, fmt.Errorf("failed to load config file: %w", err)
	}

	cfg := &Config{}
	if err := cfg.loadFromEnv(); err != nil {
		return nil, warnings, err
	}

	return cfg, warnings, nil
}

// LoadConfigFromEnv loads configuration directly from environment variables
// without reading a .env file. Useful for containerized deployments.
func LoadConfigFromEnv() (*Config, error) {
	cfg := &Config{}
	if err := cfg.loadFromEnv(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadFromEnv populates the Config from environment variables.
func (c *Config) loadFromEnv() error {
	// Provider API Keys
	c.VastAPIKey = os.Getenv("VAST_API_KEY")
	c.LambdaAPIKey = os.Getenv("LAMBDA_API_KEY")
	c.RunPodAPIKey = os.Getenv("RUNPOD_API_KEY")
	c.CoreWeaveAPIKey = os.Getenv("COREWEAVE_API_KEY")
	c.PaperspaceAPIKey = os.Getenv("PAPERSPACE_API_KEY")

	// WireGuard
	c.WireGuardPrivateKey = os.Getenv("WIREGUARD_PRIVATE_KEY")
	c.WireGuardPublicKey = os.Getenv("WIREGUARD_PUBLIC_KEY")

	// Preferences with defaults
	c.DefaultTier = getEnvWithDefault("DEFAULT_TIER", "medium")
	c.DefaultRegion = getEnvWithDefault("DEFAULT_REGION", "eu-west")
	c.PreferSpot = getEnvBool("PREFER_SPOT", true)
	c.DeadmanTimeoutHours = getEnvInt("DEADMAN_TIMEOUT_HOURS", 10)

	// Alerting
	c.AlertWebhookURL = os.Getenv("ALERT_WEBHOOK_URL")
	c.DailyBudgetEUR = getEnvFloat("DAILY_BUDGET_EUR", 20.0)

	return nil
}

// Validate checks if the configuration is valid for operation.
func (c *Config) Validate() error {
	if !c.HasAnyProvider() {
		return ErrNoProviderConfigured
	}

	// Validate tier
	tier := strings.ToLower(c.DefaultTier)
	if tier != "small" && tier != "medium" && tier != "large" {
		return fmt.Errorf("invalid DEFAULT_TIER: %q (must be small, medium, or large)", c.DefaultTier)
	}

	// Validate deadman timeout
	if c.DeadmanTimeoutHours < 1 {
		return fmt.Errorf("DEADMAN_TIMEOUT_HOURS must be at least 1, got %d", c.DeadmanTimeoutHours)
	}
	if c.DeadmanTimeoutHours > 168 { // 1 week max
		return fmt.Errorf("DEADMAN_TIMEOUT_HOURS too large: %d (max 168 = 1 week)", c.DeadmanTimeoutHours)
	}

	// Validate daily budget
	if c.DailyBudgetEUR < 0 {
		return fmt.Errorf("DAILY_BUDGET_EUR cannot be negative: %.2f", c.DailyBudgetEUR)
	}

	return nil
}

// HasAnyProvider returns true if at least one provider API key is configured.
func (c *Config) HasAnyProvider() bool {
	return c.VastAPIKey != "" ||
		c.LambdaAPIKey != "" ||
		c.RunPodAPIKey != "" ||
		c.CoreWeaveAPIKey != "" ||
		c.PaperspaceAPIKey != ""
}

// ConfiguredProviders returns a list of provider names that have API keys configured.
func (c *Config) ConfiguredProviders() []string {
	var providers []string
	if c.VastAPIKey != "" {
		providers = append(providers, "vast")
	}
	if c.LambdaAPIKey != "" {
		providers = append(providers, "lambda")
	}
	if c.RunPodAPIKey != "" {
		providers = append(providers, "runpod")
	}
	if c.CoreWeaveAPIKey != "" {
		providers = append(providers, "coreweave")
	}
	if c.PaperspaceAPIKey != "" {
		providers = append(providers, "paperspace")
	}
	return providers
}

// HasWireGuardKeys returns true if WireGuard keys are configured.
func (c *Config) HasWireGuardKeys() bool {
	return c.WireGuardPrivateKey != "" && c.WireGuardPublicKey != ""
}

// Helper functions for environment variable parsing

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "1" || value == "yes"
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return defaultValue
}
