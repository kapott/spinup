// Package deploy provides deployment orchestration for continueplz.
package deploy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tmeurs/continueplz/internal/config"
	"github.com/tmeurs/continueplz/internal/provider"
	"github.com/tmeurs/continueplz/internal/provider/registry"
)

// ReconcileResult holds the result of state reconciliation.
type ReconcileResult struct {
	// StateValid indicates if the local state matches provider reality.
	StateValid bool

	// StateCleaned indicates if stale state was detected and cleaned.
	StateCleaned bool

	// Warning is set if a warning was generated during reconciliation.
	Warning string

	// InstanceStatus is the status from the provider (if available).
	InstanceStatus provider.InstanceStatus

	// Details provides additional context about the reconciliation.
	Details string
}

// ReconcileMismatchType represents the type of state mismatch detected.
type ReconcileMismatchType int

const (
	// MismatchNone indicates no mismatch was detected.
	MismatchNone ReconcileMismatchType = iota

	// MismatchInstanceNotFound indicates the instance doesn't exist at the provider.
	MismatchInstanceNotFound

	// MismatchInstanceTerminated indicates the instance was terminated externally.
	MismatchInstanceTerminated

	// MismatchProviderUnavailable indicates the provider API was unavailable.
	MismatchProviderUnavailable
)

// String returns a human-readable description of the mismatch type.
func (m ReconcileMismatchType) String() string {
	switch m {
	case MismatchNone:
		return "no mismatch"
	case MismatchInstanceNotFound:
		return "instance not found"
	case MismatchInstanceTerminated:
		return "instance terminated externally"
	case MismatchProviderUnavailable:
		return "provider unavailable"
	default:
		return "unknown"
	}
}

// ReconcileWarning represents a warning generated during state reconciliation.
type ReconcileWarning struct {
	// MismatchType describes what kind of mismatch was detected.
	MismatchType ReconcileMismatchType

	// InstanceID is the instance ID from local state.
	InstanceID string

	// Provider is the provider name from local state.
	Provider string

	// Message is a user-friendly warning message.
	Message string

	// Reasons lists possible reasons for the mismatch.
	Reasons []string
}

// FormatWarning formats the warning as a multi-line string for display.
func (w *ReconcileWarning) FormatWarning() string {
	if w == nil || w.MismatchType == MismatchNone {
		return ""
	}

	s := fmt.Sprintf("WARNING: %s\n", w.Message)
	s += fmt.Sprintf("  Instance: %s\n", w.InstanceID)
	s += fmt.Sprintf("  Provider: %s\n", w.Provider)
	if len(w.Reasons) > 0 {
		s += "  Possible reasons:\n"
		for _, reason := range w.Reasons {
			s += fmt.Sprintf("    - %s\n", reason)
		}
	}
	s += "\nCleaning up local state..."
	return s
}

// ReconcileOptions configures the state reconciliation behavior.
type ReconcileOptions struct {
	// Timeout is the maximum time to wait for provider API response.
	Timeout time.Duration

	// AutoCleanup determines if stale state should be automatically cleaned.
	AutoCleanup bool

	// WarningCallback is called when a warning is generated.
	// If nil, warnings are ignored.
	WarningCallback func(*ReconcileWarning)
}

// DefaultReconcileOptions returns ReconcileOptions with sensible defaults.
func DefaultReconcileOptions() *ReconcileOptions {
	return &ReconcileOptions{
		Timeout:     30 * time.Second,
		AutoCleanup: true,
	}
}

// Reconciler handles state reconciliation between local state and providers.
type Reconciler struct {
	cfg          *config.Config
	stateManager *config.StateManager
	opts         *ReconcileOptions
}

// NewReconciler creates a new Reconciler with the given configuration.
func NewReconciler(cfg *config.Config, stateManager *config.StateManager, opts *ReconcileOptions) (*Reconciler, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if stateManager == nil {
		return nil, errors.New("state manager is required")
	}
	if opts == nil {
		opts = DefaultReconcileOptions()
	}

	return &Reconciler{
		cfg:          cfg,
		stateManager: stateManager,
		opts:         opts,
	}, nil
}

// ReconcileState verifies local state against the provider and cleans up stale state.
// This should be called at startup or before any operation that relies on state.
//
// Behavior:
//   - If no local state exists, returns immediately (valid state).
//   - If local state has an instance, queries the provider to verify it exists.
//   - If provider reports instance doesn't exist or is terminated, cleans up local state.
//   - Returns a warning if state mismatch was detected.
func (r *Reconciler) ReconcileState(ctx context.Context) (*ReconcileResult, error) {
	result := &ReconcileResult{
		StateValid: true,
	}

	// Load local state
	state, err := r.stateManager.LoadState()
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// No state or no instance - nothing to reconcile
	if state == nil || state.Instance == nil {
		result.Details = "no active instance in state"
		return result, nil
	}

	instanceID := state.Instance.ID
	providerName := state.Instance.Provider

	// Get provider client
	p, err := registry.GetProviderByName(providerName, r.cfg)
	if err != nil {
		// Cannot get provider - this could be a configuration issue
		// or the provider API key was removed
		result.Warning = fmt.Sprintf("cannot verify instance %s: provider %s not configured", instanceID, providerName)
		result.Details = "provider not configured, state kept"
		// Don't clean up state if we can't verify - user should handle this manually
		return result, nil
	}

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, r.opts.Timeout)
	defer cancel()

	// Query provider for instance status
	instance, err := p.GetInstance(checkCtx, instanceID)

	// Handle various error cases
	if err != nil {
		// Check if instance doesn't exist
		if errors.Is(err, provider.ErrInstanceNotFound) {
			return r.handleMismatch(ctx, state, MismatchInstanceNotFound)
		}

		// Provider API error - could be transient
		// Check if context was cancelled
		if checkCtx.Err() != nil {
			result.Warning = fmt.Sprintf("timeout checking instance %s status", instanceID)
			result.Details = "provider API timeout, state kept"
			return result, nil
		}

		// Other API error - don't clean up, could be transient
		result.Warning = fmt.Sprintf("error checking instance %s: %v", instanceID, err)
		result.Details = "provider API error, state kept"
		return result, nil
	}

	// Instance exists - check its status
	if instance.Status.IsTerminal() {
		return r.handleMismatch(ctx, state, MismatchInstanceTerminated)
	}

	// Instance exists and is not terminated - state is valid
	result.InstanceStatus = instance.Status
	result.Details = fmt.Sprintf("instance %s is %s", instanceID, instance.Status)
	return result, nil
}

// handleMismatch handles a detected state mismatch.
func (r *Reconciler) handleMismatch(_ context.Context, state *config.State, mismatchType ReconcileMismatchType) (*ReconcileResult, error) {
	result := &ReconcileResult{
		StateValid:   false,
		StateCleaned: false,
	}

	instanceID := state.Instance.ID
	providerName := state.Instance.Provider

	// Generate warning
	warning := &ReconcileWarning{
		MismatchType: mismatchType,
		InstanceID:   instanceID,
		Provider:     providerName,
	}

	switch mismatchType {
	case MismatchInstanceNotFound:
		warning.Message = "Local state has instance but provider reports it doesn't exist"
		warning.Reasons = []string{
			"The instance was terminated externally",
			"A spot instance was interrupted",
			"The provider API is returning stale data",
		}
	case MismatchInstanceTerminated:
		warning.Message = "Instance was terminated externally"
		warning.Reasons = []string{
			"The instance was manually terminated via provider console",
			"A spot instance was preempted",
			"The deadman switch triggered on the server",
		}
	}

	result.Warning = warning.FormatWarning()
	result.Details = mismatchType.String()

	// Call warning callback if set
	if r.opts.WarningCallback != nil {
		r.opts.WarningCallback(warning)
	}

	// Auto cleanup if enabled
	if r.opts.AutoCleanup {
		if err := r.stateManager.ClearState(); err != nil {
			return nil, fmt.Errorf("failed to clear stale state: %w", err)
		}
		result.StateCleaned = true
	}

	return result, nil
}

// ReconcileState is a convenience function that creates a Reconciler and performs reconciliation.
// This is the function mentioned in the PRD and FEATURES.md.
func ReconcileState(ctx context.Context, cfg *config.Config, stateManager *config.StateManager, warningCb func(*ReconcileWarning)) (*ReconcileResult, error) {
	opts := DefaultReconcileOptions()
	opts.WarningCallback = warningCb

	reconciler, err := NewReconciler(cfg, stateManager, opts)
	if err != nil {
		return nil, err
	}

	return reconciler.ReconcileState(ctx)
}

// MustReconcile performs state reconciliation and panics on error.
// This is useful for startup code where reconciliation errors are fatal.
func MustReconcile(ctx context.Context, cfg *config.Config, stateManager *config.StateManager, warningCb func(*ReconcileWarning)) *ReconcileResult {
	result, err := ReconcileState(ctx, cfg, stateManager, warningCb)
	if err != nil {
		panic(fmt.Sprintf("state reconciliation failed: %v", err))
	}
	return result
}

// Error types for reconciliation operations.
var (
	// ErrReconcileFailed indicates reconciliation could not be completed.
	ErrReconcileFailed = errors.New("state reconciliation failed")

	// ErrStateCleanupFailed indicates stale state could not be cleaned up.
	ErrStateCleanupFailed = errors.New("failed to clean up stale state")
)
