package deploy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tmeurs/spinup/internal/config"
)

func TestReconcileMismatchType_String(t *testing.T) {
	tests := []struct {
		mismatch ReconcileMismatchType
		want     string
	}{
		{MismatchNone, "no mismatch"},
		{MismatchInstanceNotFound, "instance not found"},
		{MismatchInstanceTerminated, "instance terminated externally"},
		{MismatchProviderUnavailable, "provider unavailable"},
		{ReconcileMismatchType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.mismatch.String()
			if got != tt.want {
				t.Errorf("ReconcileMismatchType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReconcileWarning_FormatWarning(t *testing.T) {
	// Test nil warning
	var nilWarning *ReconcileWarning
	if got := nilWarning.FormatWarning(); got != "" {
		t.Errorf("nil warning.FormatWarning() = %q, want empty string", got)
	}

	// Test no mismatch
	noMismatch := &ReconcileWarning{MismatchType: MismatchNone}
	if got := noMismatch.FormatWarning(); got != "" {
		t.Errorf("no mismatch warning.FormatWarning() = %q, want empty string", got)
	}

	// Test actual warning
	warning := &ReconcileWarning{
		MismatchType: MismatchInstanceNotFound,
		InstanceID:   "test-123",
		Provider:     "vast",
		Message:      "Instance not found",
		Reasons:      []string{"Terminated externally", "Spot interruption"},
	}

	formatted := warning.FormatWarning()
	if formatted == "" {
		t.Error("expected non-empty formatted warning")
	}
	if !contains(formatted, "Instance not found") {
		t.Error("formatted warning should contain message")
	}
	if !contains(formatted, "test-123") {
		t.Error("formatted warning should contain instance ID")
	}
	if !contains(formatted, "vast") {
		t.Error("formatted warning should contain provider")
	}
	if !contains(formatted, "Terminated externally") {
		t.Error("formatted warning should contain reasons")
	}
}

func TestDefaultReconcileOptions(t *testing.T) {
	opts := DefaultReconcileOptions()

	if opts == nil {
		t.Fatal("DefaultReconcileOptions() returned nil")
	}

	if opts.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", opts.Timeout)
	}

	if !opts.AutoCleanup {
		t.Error("AutoCleanup should default to true")
	}
}

func TestNewReconciler(t *testing.T) {
	cfg := &config.Config{VastAPIKey: "test-key"}
	tmpDir := t.TempDir()
	sm, err := config.NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	tests := []struct {
		name    string
		cfg     *config.Config
		sm      *config.StateManager
		opts    *ReconcileOptions
		wantErr bool
	}{
		{
			name:    "valid with all params",
			cfg:     cfg,
			sm:      sm,
			opts:    DefaultReconcileOptions(),
			wantErr: false,
		},
		{
			name:    "valid with nil opts",
			cfg:     cfg,
			sm:      sm,
			opts:    nil,
			wantErr: false,
		},
		{
			name:    "nil config",
			cfg:     nil,
			sm:      sm,
			opts:    nil,
			wantErr: true,
		},
		{
			name:    "nil state manager",
			cfg:     cfg,
			sm:      nil,
			opts:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewReconciler(tt.cfg, tt.sm, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewReconciler() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && r == nil {
				t.Error("NewReconciler() returned nil with no error")
			}
		})
	}
}

func TestReconciler_ReconcileState_NoState(t *testing.T) {
	cfg := &config.Config{VastAPIKey: "test-key"}
	tmpDir := t.TempDir()
	sm, err := config.NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	reconciler, err := NewReconciler(cfg, sm, nil)
	if err != nil {
		t.Fatalf("NewReconciler failed: %v", err)
	}

	ctx := context.Background()
	result, err := reconciler.ReconcileState(ctx)
	if err != nil {
		t.Fatalf("ReconcileState failed: %v", err)
	}

	if !result.StateValid {
		t.Error("StateValid should be true when no state exists")
	}
	if result.StateCleaned {
		t.Error("StateCleaned should be false when no state exists")
	}
	if result.Warning != "" {
		t.Errorf("Warning should be empty, got: %s", result.Warning)
	}
}

func TestReconciler_ReconcileState_NoInstance(t *testing.T) {
	cfg := &config.Config{VastAPIKey: "test-key"}
	tmpDir := t.TempDir()
	sm, err := config.NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// Create state with no instance
	state := &config.State{
		Version:  1,
		Instance: nil,
	}
	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	reconciler, err := NewReconciler(cfg, sm, nil)
	if err != nil {
		t.Fatalf("NewReconciler failed: %v", err)
	}

	ctx := context.Background()
	result, err := reconciler.ReconcileState(ctx)
	if err != nil {
		t.Fatalf("ReconcileState failed: %v", err)
	}

	if !result.StateValid {
		t.Error("StateValid should be true when no instance in state")
	}
}

func TestReconciler_ReconcileState_ProviderNotConfigured(t *testing.T) {
	// Config without vast API key
	cfg := &config.Config{}
	tmpDir := t.TempDir()
	sm, err := config.NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// Create state with instance on unconfigured provider
	state := &config.State{
		Version: 1,
		Instance: &config.InstanceState{
			ID:       "test-123",
			Provider: "vast",
		},
	}
	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	reconciler, err := NewReconciler(cfg, sm, nil)
	if err != nil {
		t.Fatalf("NewReconciler failed: %v", err)
	}

	ctx := context.Background()
	result, err := reconciler.ReconcileState(ctx)
	if err != nil {
		t.Fatalf("ReconcileState failed: %v", err)
	}

	// Should return valid (keep state) but with warning
	if !result.StateValid {
		t.Error("StateValid should be true when provider not configured (keep state)")
	}
	if result.Warning == "" {
		t.Error("Warning should be set when provider not configured")
	}
	if result.StateCleaned {
		t.Error("StateCleaned should be false when provider not configured")
	}
}

func TestReconcileState_ConvenienceFunction(t *testing.T) {
	cfg := &config.Config{VastAPIKey: "test-key"}
	tmpDir := t.TempDir()
	sm, err := config.NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	ctx := context.Background()
	result, err := ReconcileState(ctx, cfg, sm, nil)
	if err != nil {
		t.Fatalf("ReconcileState failed: %v", err)
	}

	if !result.StateValid {
		t.Error("StateValid should be true when no state exists")
	}
}

func TestReconcileState_WithWarningCallback(t *testing.T) {
	cfg := &config.Config{}
	tmpDir := t.TempDir()
	sm, err := config.NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// Create state with instance on unconfigured provider
	state := &config.State{
		Version: 1,
		Instance: &config.InstanceState{
			ID:       "test-123",
			Provider: "vast",
		},
	}
	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	var callbackCalled bool
	callback := func(w *ReconcileWarning) {
		callbackCalled = true
	}

	ctx := context.Background()
	_, err = ReconcileState(ctx, cfg, sm, callback)
	if err != nil {
		t.Fatalf("ReconcileState failed: %v", err)
	}

	// Callback should not be called for provider-not-configured case
	// (that's not a mismatch, just can't verify)
	if callbackCalled {
		t.Error("Callback should not be called for provider-not-configured case")
	}
}

func TestReconciler_HandleMismatch_InstanceNotFound(t *testing.T) {
	cfg := &config.Config{VastAPIKey: "test-key"}
	tmpDir := t.TempDir()
	sm, err := config.NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// Create state file directly for this test
	state := &config.State{
		Version: 1,
		Instance: &config.InstanceState{
			ID:       "test-123",
			Provider: "vast",
		},
	}
	stateData, _ := json.MarshalIndent(state, "", "  ")
	statePath := filepath.Join(tmpDir, config.StateFileName)
	if err := os.WriteFile(statePath, stateData, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	var warningReceived *ReconcileWarning
	opts := &ReconcileOptions{
		Timeout:     30 * time.Second,
		AutoCleanup: true,
		WarningCallback: func(w *ReconcileWarning) {
			warningReceived = w
		},
	}

	reconciler, err := NewReconciler(cfg, sm, opts)
	if err != nil {
		t.Fatalf("NewReconciler failed: %v", err)
	}

	ctx := context.Background()
	result, err := reconciler.handleMismatch(ctx, state, MismatchInstanceNotFound)
	if err != nil {
		t.Fatalf("handleMismatch failed: %v", err)
	}

	// State should be cleaned
	if !result.StateCleaned {
		t.Error("StateCleaned should be true")
	}

	// State should be marked invalid
	if result.StateValid {
		t.Error("StateValid should be false")
	}

	// Warning callback should have been called
	if warningReceived == nil {
		t.Error("Warning callback should have been called")
	} else {
		if warningReceived.MismatchType != MismatchInstanceNotFound {
			t.Errorf("MismatchType = %v, want %v", warningReceived.MismatchType, MismatchInstanceNotFound)
		}
		if warningReceived.InstanceID != "test-123" {
			t.Errorf("InstanceID = %q, want %q", warningReceived.InstanceID, "test-123")
		}
	}

	// Verify state file was actually removed
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("State file should have been removed")
	}
}

func TestReconciler_HandleMismatch_InstanceTerminated(t *testing.T) {
	cfg := &config.Config{VastAPIKey: "test-key"}
	tmpDir := t.TempDir()
	sm, err := config.NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	state := &config.State{
		Version: 1,
		Instance: &config.InstanceState{
			ID:       "term-456",
			Provider: "lambda",
		},
	}
	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	var warningReceived *ReconcileWarning
	opts := &ReconcileOptions{
		Timeout:     30 * time.Second,
		AutoCleanup: true,
		WarningCallback: func(w *ReconcileWarning) {
			warningReceived = w
		},
	}

	reconciler, err := NewReconciler(cfg, sm, opts)
	if err != nil {
		t.Fatalf("NewReconciler failed: %v", err)
	}

	ctx := context.Background()
	result, err := reconciler.handleMismatch(ctx, state, MismatchInstanceTerminated)
	if err != nil {
		t.Fatalf("handleMismatch failed: %v", err)
	}

	if !result.StateCleaned {
		t.Error("StateCleaned should be true")
	}

	if warningReceived == nil {
		t.Fatal("Warning callback should have been called")
	}

	if warningReceived.MismatchType != MismatchInstanceTerminated {
		t.Errorf("MismatchType = %v, want %v", warningReceived.MismatchType, MismatchInstanceTerminated)
	}

	// Check warning message contains relevant info
	if !contains(warningReceived.Message, "terminated") {
		t.Error("Warning message should mention termination")
	}
}

func TestReconciler_HandleMismatch_NoAutoCleanup(t *testing.T) {
	cfg := &config.Config{VastAPIKey: "test-key"}
	tmpDir := t.TempDir()
	sm, err := config.NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	state := &config.State{
		Version: 1,
		Instance: &config.InstanceState{
			ID:       "no-clean-789",
			Provider: "runpod",
		},
	}
	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	opts := &ReconcileOptions{
		Timeout:     30 * time.Second,
		AutoCleanup: false, // Disabled
	}

	reconciler, err := NewReconciler(cfg, sm, opts)
	if err != nil {
		t.Fatalf("NewReconciler failed: %v", err)
	}

	ctx := context.Background()
	result, err := reconciler.handleMismatch(ctx, state, MismatchInstanceNotFound)
	if err != nil {
		t.Fatalf("handleMismatch failed: %v", err)
	}

	// State should NOT be cleaned
	if result.StateCleaned {
		t.Error("StateCleaned should be false when AutoCleanup is disabled")
	}

	// But state should still be marked invalid
	if result.StateValid {
		t.Error("StateValid should be false")
	}

	// Verify state file still exists
	statePath := filepath.Join(tmpDir, config.StateFileName)
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("State file should still exist when AutoCleanup is disabled")
	}
}

func TestErrorTypes(t *testing.T) {
	// Just verify error variables are defined
	if ErrReconcileFailed == nil {
		t.Error("ErrReconcileFailed should not be nil")
	}
	if ErrStateCleanupFailed == nil {
		t.Error("ErrStateCleanupFailed should not be nil")
	}
}

// helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
