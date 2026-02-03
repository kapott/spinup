// Package registry provides a factory and registry for cloud GPU providers.
// It allows creating providers by name and retrieving all configured providers.
package registry

import (
	"fmt"

	"github.com/tmeurs/continueplz/internal/config"
	"github.com/tmeurs/continueplz/internal/provider"
	"github.com/tmeurs/continueplz/internal/provider/coreweave"
	"github.com/tmeurs/continueplz/internal/provider/lambda"
	"github.com/tmeurs/continueplz/internal/provider/paperspace"
	"github.com/tmeurs/continueplz/internal/provider/runpod"
	"github.com/tmeurs/continueplz/internal/provider/vast"
)

// ProviderName constants for supported providers.
const (
	ProviderVast       = "vast"
	ProviderLambda     = "lambda"
	ProviderRunPod     = "runpod"
	ProviderCoreWeave  = "coreweave"
	ProviderPaperspace = "paperspace"
)

// AllProviderNames returns all supported provider names in priority order.
func AllProviderNames() []string {
	return []string{
		ProviderVast,
		ProviderLambda,
		ProviderRunPod,
		ProviderCoreWeave,
		ProviderPaperspace,
	}
}

// ErrUnknownProvider indicates an unknown provider name was requested.
var ErrUnknownProvider = &provider.ProviderError{Code: "unknown_provider", Message: "unknown provider"}

// NewProvider creates a new Provider instance for the given provider name.
// The provider is configured using the API key from the Config.
// Returns ErrUnknownProvider if the provider name is not recognized.
// Returns provider.ErrAuthenticationFailed if the provider's API key is not configured.
func NewProvider(name string, cfg *config.Config) (provider.Provider, error) {
	switch name {
	case ProviderVast:
		if cfg.VastAPIKey == "" {
			return nil, provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("VAST_API_KEY not configured"))
		}
		return vast.NewClient(cfg.VastAPIKey)

	case ProviderLambda:
		if cfg.LambdaAPIKey == "" {
			return nil, provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("LAMBDA_API_KEY not configured"))
		}
		return lambda.NewClient(cfg.LambdaAPIKey)

	case ProviderRunPod:
		if cfg.RunPodAPIKey == "" {
			return nil, provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("RUNPOD_API_KEY not configured"))
		}
		return runpod.NewClient(cfg.RunPodAPIKey)

	case ProviderCoreWeave:
		if cfg.CoreWeaveAPIKey == "" {
			return nil, provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("COREWEAVE_API_KEY not configured"))
		}
		return coreweave.NewClient(cfg.CoreWeaveAPIKey)

	case ProviderPaperspace:
		if cfg.PaperspaceAPIKey == "" {
			return nil, provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("PAPERSPACE_API_KEY not configured"))
		}
		return paperspace.NewClient(cfg.PaperspaceAPIKey)

	default:
		return nil, ErrUnknownProvider.Wrap(fmt.Errorf("provider %q not recognized", name))
	}
}

// GetAllProviders returns Provider instances for all known providers that have
// API keys configured. This is used for price comparison across all available providers.
// Providers without configured API keys are silently skipped.
func GetAllProviders(cfg *config.Config) ([]provider.Provider, error) {
	return GetConfiguredProviders(cfg)
}

// GetConfiguredProviders returns Provider instances only for providers
// that have valid API keys configured in the Config.
// Returns an error if no providers are configured.
func GetConfiguredProviders(cfg *config.Config) ([]provider.Provider, error) {
	var providers []provider.Provider

	// Check each provider in priority order
	if cfg.VastAPIKey != "" {
		p, err := vast.NewClient(cfg.VastAPIKey)
		if err == nil {
			providers = append(providers, p)
		}
	}

	if cfg.LambdaAPIKey != "" {
		p, err := lambda.NewClient(cfg.LambdaAPIKey)
		if err == nil {
			providers = append(providers, p)
		}
	}

	if cfg.RunPodAPIKey != "" {
		p, err := runpod.NewClient(cfg.RunPodAPIKey)
		if err == nil {
			providers = append(providers, p)
		}
	}

	if cfg.CoreWeaveAPIKey != "" {
		p, err := coreweave.NewClient(cfg.CoreWeaveAPIKey)
		if err == nil {
			providers = append(providers, p)
		}
	}

	if cfg.PaperspaceAPIKey != "" {
		p, err := paperspace.NewClient(cfg.PaperspaceAPIKey)
		if err == nil {
			providers = append(providers, p)
		}
	}

	if len(providers) == 0 {
		return nil, config.ErrNoProviderConfigured
	}

	return providers, nil
}

// IsValidProviderName checks if the given name is a valid provider name.
func IsValidProviderName(name string) bool {
	switch name {
	case ProviderVast, ProviderLambda, ProviderRunPod, ProviderCoreWeave, ProviderPaperspace:
		return true
	default:
		return false
	}
}

// GetProviderAPIKeyEnvVar returns the environment variable name for a provider's API key.
func GetProviderAPIKeyEnvVar(name string) string {
	switch name {
	case ProviderVast:
		return "VAST_API_KEY"
	case ProviderLambda:
		return "LAMBDA_API_KEY"
	case ProviderRunPod:
		return "RUNPOD_API_KEY"
	case ProviderCoreWeave:
		return "COREWEAVE_API_KEY"
	case ProviderPaperspace:
		return "PAPERSPACE_API_KEY"
	default:
		return ""
	}
}

// GetProviderByName returns a single configured provider by name.
// This is a convenience wrapper around NewProvider.
func GetProviderByName(name string, cfg *config.Config) (provider.Provider, error) {
	return NewProvider(name, cfg)
}
