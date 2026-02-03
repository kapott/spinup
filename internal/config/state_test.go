// Package config provides configuration and state management for continueplz.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestNewStateManager tests the StateManager constructor.
func TestNewStateManager(t *testing.T) {
	t.Run("with empty dir uses cwd", func(t *testing.T) {
		sm, err := NewStateManager("")
		if err != nil {
			t.Fatalf("NewStateManager failed: %v", err)
		}

		cwd, _ := os.Getwd()
		if sm.stateDir != cwd {
			t.Errorf("expected stateDir=%q, got %q", cwd, sm.stateDir)
		}
	})

	t.Run("with custom dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		sm, err := NewStateManager(tmpDir)
		if err != nil {
			t.Fatalf("NewStateManager failed: %v", err)
		}

		if sm.stateDir != tmpDir {
			t.Errorf("expected stateDir=%q, got %q", tmpDir, sm.stateDir)
		}
	})
}

// TestStateSaveLoadCycle tests the full save/load cycle.
func TestStateSaveLoadCycle(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)

	// Create a complete state
	state := &State{
		Version: StateVersion,
		Instance: &InstanceState{
			ID:          "inst-123",
			Provider:    "vast",
			GPU:         "A100-40GB",
			Region:      "EU-West",
			Type:        "spot",
			PublicIP:    "1.2.3.4",
			WireGuardIP: "10.0.0.1",
			CreatedAt:   now,
		},
		Model: &ModelState{
			Name:   "qwen2.5-coder:7b",
			Status: "ready",
		},
		WireGuard: &WireGuardState{
			ServerPublicKey: "abc123publickey",
			InterfaceName:   "wg0",
		},
		Cost: &CostState{
			HourlyRate:  0.50,
			Accumulated: 1.25,
			Currency:    "EUR",
		},
		Deadman: &DeadmanState{
			TimeoutHours:  24,
			LastHeartbeat: now,
		},
	}

	// Save state
	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Verify file exists
	statePath := filepath.Join(tmpDir, StateFileName)
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("state file was not created")
	}

	// Load state back
	loaded, err := sm.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Verify loaded state matches saved state
	if loaded.Version != state.Version {
		t.Errorf("Version: expected %d, got %d", state.Version, loaded.Version)
	}

	// Instance checks
	if loaded.Instance == nil {
		t.Fatal("Instance is nil after load")
	}
	if loaded.Instance.ID != state.Instance.ID {
		t.Errorf("Instance.ID: expected %q, got %q", state.Instance.ID, loaded.Instance.ID)
	}
	if loaded.Instance.Provider != state.Instance.Provider {
		t.Errorf("Instance.Provider: expected %q, got %q", state.Instance.Provider, loaded.Instance.Provider)
	}
	if loaded.Instance.GPU != state.Instance.GPU {
		t.Errorf("Instance.GPU: expected %q, got %q", state.Instance.GPU, loaded.Instance.GPU)
	}
	if loaded.Instance.Region != state.Instance.Region {
		t.Errorf("Instance.Region: expected %q, got %q", state.Instance.Region, loaded.Instance.Region)
	}
	if loaded.Instance.Type != state.Instance.Type {
		t.Errorf("Instance.Type: expected %q, got %q", state.Instance.Type, loaded.Instance.Type)
	}
	if loaded.Instance.PublicIP != state.Instance.PublicIP {
		t.Errorf("Instance.PublicIP: expected %q, got %q", state.Instance.PublicIP, loaded.Instance.PublicIP)
	}
	if loaded.Instance.WireGuardIP != state.Instance.WireGuardIP {
		t.Errorf("Instance.WireGuardIP: expected %q, got %q", state.Instance.WireGuardIP, loaded.Instance.WireGuardIP)
	}
	if !loaded.Instance.CreatedAt.Equal(state.Instance.CreatedAt) {
		t.Errorf("Instance.CreatedAt: expected %v, got %v", state.Instance.CreatedAt, loaded.Instance.CreatedAt)
	}

	// Model checks
	if loaded.Model == nil {
		t.Fatal("Model is nil after load")
	}
	if loaded.Model.Name != state.Model.Name {
		t.Errorf("Model.Name: expected %q, got %q", state.Model.Name, loaded.Model.Name)
	}
	if loaded.Model.Status != state.Model.Status {
		t.Errorf("Model.Status: expected %q, got %q", state.Model.Status, loaded.Model.Status)
	}

	// WireGuard checks
	if loaded.WireGuard == nil {
		t.Fatal("WireGuard is nil after load")
	}
	if loaded.WireGuard.ServerPublicKey != state.WireGuard.ServerPublicKey {
		t.Errorf("WireGuard.ServerPublicKey: expected %q, got %q", state.WireGuard.ServerPublicKey, loaded.WireGuard.ServerPublicKey)
	}
	if loaded.WireGuard.InterfaceName != state.WireGuard.InterfaceName {
		t.Errorf("WireGuard.InterfaceName: expected %q, got %q", state.WireGuard.InterfaceName, loaded.WireGuard.InterfaceName)
	}

	// Cost checks
	if loaded.Cost == nil {
		t.Fatal("Cost is nil after load")
	}
	if loaded.Cost.HourlyRate != state.Cost.HourlyRate {
		t.Errorf("Cost.HourlyRate: expected %f, got %f", state.Cost.HourlyRate, loaded.Cost.HourlyRate)
	}
	if loaded.Cost.Accumulated != state.Cost.Accumulated {
		t.Errorf("Cost.Accumulated: expected %f, got %f", state.Cost.Accumulated, loaded.Cost.Accumulated)
	}
	if loaded.Cost.Currency != state.Cost.Currency {
		t.Errorf("Cost.Currency: expected %q, got %q", state.Cost.Currency, loaded.Cost.Currency)
	}

	// Deadman checks
	if loaded.Deadman == nil {
		t.Fatal("Deadman is nil after load")
	}
	if loaded.Deadman.TimeoutHours != state.Deadman.TimeoutHours {
		t.Errorf("Deadman.TimeoutHours: expected %d, got %d", state.Deadman.TimeoutHours, loaded.Deadman.TimeoutHours)
	}
	if !loaded.Deadman.LastHeartbeat.Equal(state.Deadman.LastHeartbeat) {
		t.Errorf("Deadman.LastHeartbeat: expected %v, got %v", state.Deadman.LastHeartbeat, loaded.Deadman.LastHeartbeat)
	}
}

// TestStateLoadNonExistent tests loading when no state file exists.
func TestStateLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	state, err := sm.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if state != nil {
		t.Error("expected nil state when file doesn't exist")
	}
}

// TestStateLoadEmptyFile tests loading when state file is empty.
func TestStateLoadEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// Create empty state file
	statePath := filepath.Join(tmpDir, StateFileName)
	if err := os.WriteFile(statePath, []byte{}, 0600); err != nil {
		t.Fatalf("failed to create empty state file: %v", err)
	}

	state, err := sm.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if state != nil {
		t.Error("expected nil state for empty file")
	}
}

// TestStateCorruptHandling tests handling of corrupt state files.
func TestStateCorruptHandling(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"invalid json", "not valid json {{{"},
		{"truncated json", `{"version": 1, "instance": {`},
		{"wrong type", `{"version": "not a number"}`},
		{"array instead of object", `[1, 2, 3]`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			sm, err := NewStateManager(tmpDir)
			if err != nil {
				t.Fatalf("NewStateManager failed: %v", err)
			}

			// Write corrupt content
			statePath := filepath.Join(tmpDir, StateFileName)
			if err := os.WriteFile(statePath, []byte(tc.content), 0600); err != nil {
				t.Fatalf("failed to write corrupt state: %v", err)
			}

			_, err = sm.LoadState()
			if err == nil {
				t.Error("expected error for corrupt state")
			}
			if !errors.Is(err, ErrStateCorrupt) {
				t.Errorf("expected ErrStateCorrupt, got: %v", err)
			}
		})
	}
}

// TestStateClearState tests clearing the state file.
func TestStateClearState(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// Save initial state
	state := &State{
		Version: StateVersion,
		Instance: &InstanceState{
			ID:       "test-id",
			Provider: "test",
		},
	}
	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Verify file exists
	statePath := filepath.Join(tmpDir, StateFileName)
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("state file was not created")
	}

	// Clear state
	if err := sm.ClearState(); err != nil {
		t.Fatalf("ClearState failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("state file still exists after ClearState")
	}

	// Verify LoadState returns nil
	loaded, err := sm.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil state after ClearState")
	}
}

// TestStateClearNonExistent tests clearing when no state exists.
func TestStateClearNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// Clear non-existent state should not error
	if err := sm.ClearState(); err != nil {
		t.Errorf("ClearState on non-existent file failed: %v", err)
	}
}

// TestStateSaveNil tests that saving nil state fails.
func TestStateSaveNil(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	err = sm.SaveState(nil)
	if err == nil {
		t.Error("expected error when saving nil state")
	}
}

// TestStateVersionAutoSet tests that version is auto-set if missing.
func TestStateVersionAutoSet(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// Save state with version=0
	state := &State{
		Version: 0, // Should be auto-set to StateVersion
		Instance: &InstanceState{
			ID:       "test",
			Provider: "test",
		},
	}

	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := sm.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if loaded.Version != StateVersion {
		t.Errorf("expected version=%d, got %d", StateVersion, loaded.Version)
	}
}

// TestHasActiveInstance tests the HasActiveInstance helper.
func TestHasActiveInstance(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// No state file
	has, err := sm.HasActiveInstance()
	if err != nil {
		t.Fatalf("HasActiveInstance failed: %v", err)
	}
	if has {
		t.Error("expected false when no state file")
	}

	// State with no instance
	if err := sm.SaveState(&State{Version: StateVersion}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}
	has, err = sm.HasActiveInstance()
	if err != nil {
		t.Fatalf("HasActiveInstance failed: %v", err)
	}
	if has {
		t.Error("expected false when state has no instance")
	}

	// State with instance
	if err := sm.SaveState(&State{
		Version:  StateVersion,
		Instance: &InstanceState{ID: "test"},
	}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}
	has, err = sm.HasActiveInstance()
	if err != nil {
		t.Fatalf("HasActiveInstance failed: %v", err)
	}
	if !has {
		t.Error("expected true when state has instance")
	}
}

// TestGetInstance tests the GetInstance helper.
func TestGetInstance(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// No state file
	_, err = sm.GetInstance()
	if !errors.Is(err, ErrNoActiveInstance) {
		t.Errorf("expected ErrNoActiveInstance, got: %v", err)
	}

	// State with instance
	expected := &InstanceState{
		ID:       "inst-456",
		Provider: "vast",
		GPU:      "A100-80GB",
	}
	if err := sm.SaveState(&State{
		Version:  StateVersion,
		Instance: expected,
	}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	inst, err := sm.GetInstance()
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if inst.ID != expected.ID {
		t.Errorf("expected ID=%q, got %q", expected.ID, inst.ID)
	}
}

// TestUpdateCost tests the UpdateCost helper.
func TestUpdateCost(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// No state - should fail
	err = sm.UpdateCost(1.0)
	if !errors.Is(err, ErrNoActiveInstance) {
		t.Errorf("expected ErrNoActiveInstance, got: %v", err)
	}

	// State with cost
	if err := sm.SaveState(&State{
		Version:  StateVersion,
		Instance: &InstanceState{ID: "test"},
		Cost: &CostState{
			HourlyRate:  0.50,
			Accumulated: 0.0,
			Currency:    "EUR",
		},
	}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Update cost
	if err := sm.UpdateCost(2.50); err != nil {
		t.Fatalf("UpdateCost failed: %v", err)
	}

	// Verify update
	state, err := sm.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if state.Cost.Accumulated != 2.50 {
		t.Errorf("expected Accumulated=2.50, got %f", state.Cost.Accumulated)
	}
}

// TestUpdateHeartbeat tests the UpdateHeartbeat helper.
func TestUpdateHeartbeat(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// No state - should fail
	err = sm.UpdateHeartbeat()
	if !errors.Is(err, ErrNoActiveInstance) {
		t.Errorf("expected ErrNoActiveInstance, got: %v", err)
	}

	// State with deadman
	oldHeartbeat := time.Now().UTC().Add(-1 * time.Hour)
	if err := sm.SaveState(&State{
		Version:  StateVersion,
		Instance: &InstanceState{ID: "test"},
		Deadman: &DeadmanState{
			TimeoutHours:  24,
			LastHeartbeat: oldHeartbeat,
		},
	}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Update heartbeat
	beforeUpdate := time.Now().UTC()
	if err := sm.UpdateHeartbeat(); err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}
	afterUpdate := time.Now().UTC()

	// Verify update
	state, err := sm.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if state.Deadman.LastHeartbeat.Before(beforeUpdate) || state.Deadman.LastHeartbeat.After(afterUpdate) {
		t.Errorf("heartbeat not updated correctly: got %v, expected between %v and %v",
			state.Deadman.LastHeartbeat, beforeUpdate, afterUpdate)
	}
}

// TestUpdateModelStatus tests the UpdateModelStatus helper.
func TestUpdateModelStatus(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// No state - should fail
	err = sm.UpdateModelStatus("ready")
	if !errors.Is(err, ErrNoActiveInstance) {
		t.Errorf("expected ErrNoActiveInstance, got: %v", err)
	}

	// State with model
	if err := sm.SaveState(&State{
		Version:  StateVersion,
		Instance: &InstanceState{ID: "test"},
		Model: &ModelState{
			Name:   "qwen2.5-coder:7b",
			Status: "loading",
		},
	}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Update status
	if err := sm.UpdateModelStatus("ready"); err != nil {
		t.Fatalf("UpdateModelStatus failed: %v", err)
	}

	// Verify update
	state, err := sm.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if state.Model.Status != "ready" {
		t.Errorf("expected Status=%q, got %q", "ready", state.Model.Status)
	}
}

// TestNewState tests the NewState helper function.
func TestNewState(t *testing.T) {
	instance := &InstanceState{ID: "test"}
	model := &ModelState{Name: "test-model"}
	wireguard := &WireGuardState{InterfaceName: "wg0"}
	cost := &CostState{HourlyRate: 0.50}
	deadman := &DeadmanState{TimeoutHours: 24}

	state := NewState(instance, model, wireguard, cost, deadman)

	if state.Version != StateVersion {
		t.Errorf("expected Version=%d, got %d", StateVersion, state.Version)
	}
	if state.Instance != instance {
		t.Error("Instance not set correctly")
	}
	if state.Model != model {
		t.Error("Model not set correctly")
	}
	if state.WireGuard != wireguard {
		t.Error("WireGuard not set correctly")
	}
	if state.Cost != cost {
		t.Error("Cost not set correctly")
	}
	if state.Deadman != deadman {
		t.Error("Deadman not set correctly")
	}
}

// TestInstanceStateDuration tests the Duration helper method.
func TestInstanceStateDuration(t *testing.T) {
	t.Run("nil instance", func(t *testing.T) {
		var inst *InstanceState
		if inst.Duration() != 0 {
			t.Error("expected 0 duration for nil instance")
		}
	})

	t.Run("running instance", func(t *testing.T) {
		inst := &InstanceState{
			CreatedAt: time.Now().Add(-1 * time.Hour),
		}
		dur := inst.Duration()
		// Allow some tolerance for test execution time
		if dur < 59*time.Minute || dur > 61*time.Minute {
			t.Errorf("expected ~1 hour duration, got %v", dur)
		}
	})
}

// TestInstanceStateIsSpot tests the IsSpot helper method.
func TestInstanceStateIsSpot(t *testing.T) {
	t.Run("nil instance", func(t *testing.T) {
		var inst *InstanceState
		if inst.IsSpot() {
			t.Error("expected false for nil instance")
		}
	})

	t.Run("spot instance", func(t *testing.T) {
		inst := &InstanceState{Type: "spot"}
		if !inst.IsSpot() {
			t.Error("expected true for spot instance")
		}
	})

	t.Run("on-demand instance", func(t *testing.T) {
		inst := &InstanceState{Type: "on-demand"}
		if inst.IsSpot() {
			t.Error("expected false for on-demand instance")
		}
	})
}

// TestCalculateAccumulatedCost tests the CalculateAccumulatedCost method.
func TestCalculateAccumulatedCost(t *testing.T) {
	t.Run("nil state", func(t *testing.T) {
		var state *State
		if state.CalculateAccumulatedCost() != 0 {
			t.Error("expected 0 for nil state")
		}
	})

	t.Run("no instance", func(t *testing.T) {
		state := &State{Cost: &CostState{HourlyRate: 1.0}}
		if state.CalculateAccumulatedCost() != 0 {
			t.Error("expected 0 when no instance")
		}
	})

	t.Run("no cost", func(t *testing.T) {
		state := &State{Instance: &InstanceState{}}
		if state.CalculateAccumulatedCost() != 0 {
			t.Error("expected 0 when no cost")
		}
	})

	t.Run("running instance", func(t *testing.T) {
		state := &State{
			Instance: &InstanceState{
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
			Cost: &CostState{
				HourlyRate: 0.50,
			},
		}
		cost := state.CalculateAccumulatedCost()
		// 2 hours * 0.50 = 1.0, allow some tolerance
		if cost < 0.99 || cost > 1.01 {
			t.Errorf("expected ~1.0, got %f", cost)
		}
	})
}

// TestStateConcurrentAccess tests concurrent access to state operations.
func TestStateConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// Initialize state
	if err := sm.SaveState(&State{
		Version:  StateVersion,
		Instance: &InstanceState{ID: "test"},
		Cost: &CostState{
			HourlyRate:  0.50,
			Accumulated: 0.0,
			Currency:    "EUR",
		},
		Deadman: &DeadmanState{
			TimeoutHours:  24,
			LastHeartbeat: time.Now().UTC(),
		},
	}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	errChan := make(chan error, 100)
	numGoroutines := 10
	numOpsPerGoroutine := 10

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range numOpsPerGoroutine {
				// Mix of operations
				switch j % 4 {
				case 0:
					_, err := sm.LoadState()
					if err != nil {
						errChan <- err
					}
				case 1:
					_, err := sm.HasActiveInstance()
					if err != nil {
						errChan <- err
					}
				case 2:
					err := sm.UpdateCost(float64(id*numOpsPerGoroutine + j))
					if err != nil {
						errChan <- err
					}
				case 3:
					err := sm.UpdateHeartbeat()
					if err != nil {
						errChan <- err
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("concurrent operations had %d errors, first: %v", len(errors), errors[0])
	}
}

// TestStateAtomicWrite tests that writes are atomic (via temp file + rename).
func TestStateAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// Save state
	state := &State{
		Version: StateVersion,
		Instance: &InstanceState{
			ID:       "test-atomic",
			Provider: "vast",
		},
	}
	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Verify no temp file left behind
	tempPath := filepath.Join(tmpDir, StateFileName+".tmp")
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("temp file was left behind after save")
	}

	// Verify state file is valid JSON
	data, err := os.ReadFile(filepath.Join(tmpDir, StateFileName))
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Errorf("saved state is not valid JSON: %v", err)
	}
}

// TestStateFilePermissions tests that state files are created with correct permissions.
func TestStateFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	state := &State{
		Version:  StateVersion,
		Instance: &InstanceState{ID: "test"},
	}
	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	statePath := filepath.Join(tmpDir, StateFileName)
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("failed to stat state file: %v", err)
	}

	// Check permissions are 0600 (read/write for owner only)
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}
}

// TestStateJSONFormat tests that the JSON format matches PRD Section 6.1.
func TestStateJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	state := &State{
		Version: 1,
		Instance: &InstanceState{
			ID:          "inst-123",
			Provider:    "vast",
			GPU:         "A100-40GB",
			Region:      "EU-West",
			Type:        "spot",
			PublicIP:    "1.2.3.4",
			WireGuardIP: "10.0.0.1",
			CreatedAt:   now,
		},
		Model: &ModelState{
			Name:   "qwen2.5-coder:7b",
			Status: "ready",
		},
		WireGuard: &WireGuardState{
			ServerPublicKey: "serverkey123",
			InterfaceName:   "wg0",
		},
		Cost: &CostState{
			HourlyRate:  0.50,
			Accumulated: 1.25,
			Currency:    "EUR",
		},
		Deadman: &DeadmanState{
			TimeoutHours:  24,
			LastHeartbeat: now,
		},
	}

	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, StateFileName))
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	// Verify expected JSON structure
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Check top-level keys
	expectedKeys := []string{"version", "instance", "model", "wireguard", "cost", "deadman"}
	for _, key := range expectedKeys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing expected key: %q", key)
		}
	}

	// Check instance keys
	inst, ok := parsed["instance"].(map[string]any)
	if !ok {
		t.Fatal("instance is not an object")
	}
	instanceKeys := []string{"id", "provider", "gpu", "region", "type", "public_ip", "wireguard_ip", "created_at"}
	for _, key := range instanceKeys {
		if _, ok := inst[key]; !ok {
			t.Errorf("instance missing key: %q", key)
		}
	}
}

// TestStatePartialState tests saving/loading state with some fields nil.
func TestStatePartialState(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// State with only instance and model
	state := &State{
		Version: StateVersion,
		Instance: &InstanceState{
			ID:       "partial-test",
			Provider: "lambda",
		},
		Model: &ModelState{
			Name:   "deepseek-coder:6.7b",
			Status: "loading",
		},
		// WireGuard, Cost, Deadman are nil
	}

	if err := sm.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := sm.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if loaded.Instance == nil {
		t.Fatal("Instance should not be nil")
	}
	if loaded.Model == nil {
		t.Fatal("Model should not be nil")
	}
	if loaded.WireGuard != nil {
		t.Error("WireGuard should be nil")
	}
	if loaded.Cost != nil {
		t.Error("Cost should be nil")
	}
	if loaded.Deadman != nil {
		t.Error("Deadman should be nil")
	}
}
