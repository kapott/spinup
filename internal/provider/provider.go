// Package provider defines the interface and types for cloud GPU providers.
package provider

import (
	"context"
	"time"
)

// Provider defines the interface that all cloud GPU providers must implement.
// Each provider (Vast.ai, Lambda Labs, RunPod, CoreWeave, Paperspace) implements
// this interface to enable unified instance management and price comparison.
type Provider interface {
	// Name returns the provider's identifier (e.g., "vast", "lambda", "runpod").
	Name() string

	// GetOffers returns available GPU offers matching the filter criteria.
	// This is used for price comparison across providers.
	GetOffers(ctx context.Context, filter OfferFilter) ([]Offer, error)

	// CreateInstance creates a new GPU instance with the given configuration.
	// The cloud-init script is injected via CreateRequest.CloudInit.
	CreateInstance(ctx context.Context, req CreateRequest) (*Instance, error)

	// GetInstance retrieves the current status of an instance by ID.
	// Returns nil if the instance doesn't exist.
	GetInstance(ctx context.Context, id string) (*Instance, error)

	// TerminateInstance terminates an instance by ID.
	// This should be idempotent - terminating an already-terminated instance should not error.
	TerminateInstance(ctx context.Context, id string) error

	// SupportsBillingVerification returns true if the provider has an API
	// to verify that billing has stopped after instance termination.
	// Providers like Paperspace don't have this, requiring manual verification.
	SupportsBillingVerification() bool

	// GetBillingStatus returns the billing status for an instance.
	// Only valid if SupportsBillingVerification() returns true.
	GetBillingStatus(ctx context.Context, id string) (BillingStatus, error)

	// ConsoleURL returns the URL to the provider's web console.
	// Used for manual verification when billing API is not available.
	ConsoleURL() string

	// ValidateAPIKey validates the API key and returns account information.
	// This is used during setup to verify credentials before saving them.
	// Returns an error if the API key is invalid or the API is unreachable.
	ValidateAPIKey(ctx context.Context) (*AccountInfo, error)
}

// AccountInfo contains account information returned from API key validation.
type AccountInfo struct {
	// Email is the account email (if available from the provider).
	Email string

	// Username is the account username (if available from the provider).
	Username string

	// Balance is the current account balance in the provider's currency.
	// Nil if balance information is not available.
	Balance *float64

	// BalanceCurrency is the currency of the balance (e.g., "USD", "EUR").
	BalanceCurrency string

	// AccountID is the provider's internal account identifier.
	AccountID string

	// Valid indicates the API key is valid and the account is in good standing.
	Valid bool
}

// BillingStatus represents the billing state of an instance.
type BillingStatus string

const (
	// BillingActive indicates the instance is being billed.
	BillingActive BillingStatus = "active"

	// BillingStopped indicates billing has stopped for the instance.
	BillingStopped BillingStatus = "stopped"

	// BillingUnknown indicates the billing status could not be determined.
	BillingUnknown BillingStatus = "unknown"
)

// OfferFilter specifies criteria for filtering GPU offers.
type OfferFilter struct {
	// GPUType filters by specific GPU type (e.g., "A100-40GB", "A6000").
	// Empty string means no filter.
	GPUType string

	// MinVRAM filters offers with at least this much VRAM in GB.
	MinVRAM int

	// Region filters by geographic region (e.g., "eu-west", "us-east").
	// Empty string means no filter.
	Region string

	// SpotOnly if true, only returns offers with spot pricing available.
	SpotOnly bool

	// OnDemandOnly if true, excludes spot offers.
	OnDemandOnly bool

	// MaxHourlyPrice filters offers with hourly price at or below this value.
	// Zero means no price limit.
	MaxHourlyPrice float64
}

// Offer represents a GPU instance offering from a provider.
type Offer struct {
	// OfferID is the provider-specific identifier for this offer.
	// Used when creating an instance.
	OfferID string

	// Provider is the provider name (e.g., "vast", "lambda").
	Provider string

	// GPU is the GPU type (e.g., "A100 40GB", "A6000 48GB").
	GPU string

	// VRAM is the GPU memory in GB.
	VRAM int

	// Region is the geographic region (e.g., "EU-West", "US-East").
	Region string

	// SpotPrice is the hourly price for spot instances in EUR.
	// Nil if spot is not available for this offer.
	SpotPrice *float64

	// OnDemandPrice is the hourly price for on-demand instances in EUR.
	OnDemandPrice float64

	// StoragePrice is the price per GB per hour for storage in EUR.
	StoragePrice float64

	// EgressPrice is the price per GB for egress traffic in EUR.
	EgressPrice float64

	// Available indicates if this offer is currently available.
	Available bool
}

// CreateRequest contains the parameters for creating a new instance.
type CreateRequest struct {
	// OfferID is the provider-specific offer identifier from Offer.OfferID.
	OfferID string

	// Spot if true, creates a spot instance (if available).
	// If spot is not available, returns an error.
	Spot bool

	// CloudInit is the cloud-init YAML configuration to inject.
	// This sets up WireGuard, Ollama, and the deadman switch.
	CloudInit string

	// SSHPublicKey is the SSH public key to add to the instance.
	// Used for emergency access if WireGuard fails.
	SSHPublicKey string

	// DiskSizeGB is the disk size in GB.
	// Should be large enough for the model (typically 100GB+).
	DiskSizeGB int
}

// Instance represents a running or terminated GPU instance.
type Instance struct {
	// ID is the provider-specific instance identifier.
	ID string

	// Provider is the provider name (e.g., "vast", "lambda").
	Provider string

	// Status is the instance status.
	Status InstanceStatus

	// PublicIP is the instance's public IP address.
	// Empty until the instance is running.
	PublicIP string

	// GPU is the GPU type (e.g., "A100 40GB").
	GPU string

	// Region is the geographic region.
	Region string

	// Spot indicates if this is a spot instance.
	Spot bool

	// CreatedAt is when the instance was created.
	CreatedAt time.Time

	// HourlyRate is the hourly cost in EUR.
	HourlyRate float64
}

// InstanceStatus represents the lifecycle status of an instance.
type InstanceStatus string

const (
	// InstanceStatusCreating indicates the instance is being created.
	InstanceStatusCreating InstanceStatus = "creating"

	// InstanceStatusRunning indicates the instance is running.
	InstanceStatusRunning InstanceStatus = "running"

	// InstanceStatusStopping indicates the instance is being stopped.
	InstanceStatusStopping InstanceStatus = "stopping"

	// InstanceStatusTerminated indicates the instance has been terminated.
	InstanceStatusTerminated InstanceStatus = "terminated"

	// InstanceStatusError indicates the instance encountered an error.
	InstanceStatusError InstanceStatus = "error"
)

// IsRunning returns true if the instance is in a running state.
func (s InstanceStatus) IsRunning() bool {
	return s == InstanceStatusRunning
}

// IsTerminal returns true if the instance is in a terminal state
// (terminated or error).
func (s InstanceStatus) IsTerminal() bool {
	return s == InstanceStatusTerminated || s == InstanceStatusError
}

// Error types for provider operations.
var (
	// ErrOfferNotFound indicates the requested offer doesn't exist.
	ErrOfferNotFound = &ProviderError{Code: "offer_not_found", Message: "offer not found"}

	// ErrInstanceNotFound indicates the requested instance doesn't exist.
	ErrInstanceNotFound = &ProviderError{Code: "instance_not_found", Message: "instance not found"}

	// ErrSpotNotAvailable indicates spot pricing is not available for the offer.
	ErrSpotNotAvailable = &ProviderError{Code: "spot_not_available", Message: "spot pricing not available for this offer"}

	// ErrInsufficientCapacity indicates the provider has no available capacity.
	ErrInsufficientCapacity = &ProviderError{Code: "insufficient_capacity", Message: "insufficient capacity at provider"}

	// ErrAuthenticationFailed indicates the API key is invalid.
	ErrAuthenticationFailed = &ProviderError{Code: "authentication_failed", Message: "authentication failed - check API key"}

	// ErrRateLimited indicates the provider rate limited the request.
	ErrRateLimited = &ProviderError{Code: "rate_limited", Message: "rate limited by provider"}

	// ErrBillingNotSupported indicates billing verification is not available.
	ErrBillingNotSupported = &ProviderError{Code: "billing_not_supported", Message: "billing verification not supported by this provider"}
)

// ProviderError represents an error from a provider operation.
type ProviderError struct {
	Code    string // Machine-readable error code
	Message string // Human-readable error message
	Cause   error  // Underlying error, if any
}

func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// Wrap returns a new ProviderError with the same code and message but with a cause.
func (e *ProviderError) Wrap(cause error) *ProviderError {
	return &ProviderError{
		Code:    e.Code,
		Message: e.Message,
		Cause:   cause,
	}
}

// NewProviderError creates a new ProviderError with the given code and message.
func NewProviderError(code, message string, cause error) *ProviderError {
	return &ProviderError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
