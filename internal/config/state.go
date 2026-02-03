// Package config provides configuration and state management for continueplz.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// StateFileName is the name of the state file.
	StateFileName = ".continueplz.state"

	// StateVersion is the current version of the state file format.
	StateVersion = 1
)

var (
	// ErrNoActiveInstance is returned when there is no active instance in state.
	ErrNoActiveInstance = errors.New("no active instance in state")

	// ErrStateLocked is returned when the state file is locked by another process.
	ErrStateLocked = errors.New("state file is locked by another process")

	// ErrStateCorrupt is returned when the state file is corrupt.
	ErrStateCorrupt = errors.New("state file is corrupt")

	// stateMutex provides process-level synchronization for state operations.
	stateMutex sync.Mutex
)

// State represents the complete state of a continueplz session.
// Matches PRD Section 6.1 format.
type State struct {
	Version   int             `json:"version"`
	Instance  *InstanceState  `json:"instance,omitempty"`
	Model     *ModelState     `json:"model,omitempty"`
	WireGuard *WireGuardState `json:"wireguard,omitempty"`
	Cost      *CostState      `json:"cost,omitempty"`
	Deadman   *DeadmanState   `json:"deadman,omitempty"`
}

// InstanceState represents the state of a running instance.
type InstanceState struct {
	ID          string    `json:"id"`
	Provider    string    `json:"provider"`
	GPU         string    `json:"gpu"`
	Region      string    `json:"region"`
	Type        string    `json:"type"` // "spot" or "on-demand"
	PublicIP    string    `json:"public_ip"`
	WireGuardIP string    `json:"wireguard_ip"`
	CreatedAt   time.Time `json:"created_at"`
}

// ModelState represents the state of the deployed model.
type ModelState struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "loading", "ready", "error"
}

// WireGuardState represents the WireGuard tunnel state.
type WireGuardState struct {
	ServerPublicKey string `json:"server_public_key"`
	InterfaceName   string `json:"interface_name"`
}

// CostState tracks cost information for the current session.
type CostState struct {
	HourlyRate  float64 `json:"hourly_rate"`
	Accumulated float64 `json:"accumulated"`
	Currency    string  `json:"currency"`
}

// DeadmanState tracks the deadman switch status.
type DeadmanState struct {
	TimeoutHours  int       `json:"timeout_hours"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// StateManager handles state file operations with locking.
type StateManager struct {
	stateDir string
	lockFile *os.File
}

// NewStateManager creates a new StateManager for the given directory.
// If stateDir is empty, it uses the current working directory.
func NewStateManager(stateDir string) (*StateManager, error) {
	if stateDir == "" {
		var err error
		stateDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	return &StateManager{
		stateDir: stateDir,
	}, nil
}

// statePath returns the full path to the state file.
func (m *StateManager) statePath() string {
	return filepath.Join(m.stateDir, StateFileName)
}

// lockPath returns the full path to the lock file.
func (m *StateManager) lockPath() string {
	return filepath.Join(m.stateDir, StateFileName+".lock")
}

// acquireLock acquires an exclusive file lock for state operations.
// This prevents race conditions between multiple processes.
func (m *StateManager) acquireLock() error {
	stateMutex.Lock()

	lockPath := m.lockPath()
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		stateMutex.Unlock()
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock
	if err := acquireFileLock(f); err != nil {
		f.Close()
		stateMutex.Unlock()
		return ErrStateLocked
	}

	m.lockFile = f
	return nil
}

// releaseLock releases the file lock.
func (m *StateManager) releaseLock() error {
	if m.lockFile == nil {
		stateMutex.Unlock()
		return nil
	}

	if err := releaseFileLock(m.lockFile); err != nil {
		m.lockFile.Close()
		m.lockFile = nil
		stateMutex.Unlock()
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if err := m.lockFile.Close(); err != nil {
		m.lockFile = nil
		stateMutex.Unlock()
		return fmt.Errorf("failed to close lock file: %w", err)
	}

	m.lockFile = nil
	stateMutex.Unlock()
	return nil
}

// LoadState loads the state from the state file.
// Returns nil state and no error if the file doesn't exist (no active session).
// Returns ErrStateCorrupt if the file exists but is not valid JSON.
func (m *StateManager) LoadState() (*State, error) {
	if err := m.acquireLock(); err != nil {
		return nil, err
	}
	defer m.releaseLock()

	return m.loadStateUnlocked()
}

// loadStateUnlocked loads state without acquiring lock (for internal use when already locked).
func (m *StateManager) loadStateUnlocked() (*State, error) {
	statePath := m.statePath()

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No state file means no active session - this is normal
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Empty file is treated as no state
	if len(data) == 0 {
		return nil, nil
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		// Return a wrapped error indicating corruption, but include the original error
		return nil, fmt.Errorf("%w: %v", ErrStateCorrupt, err)
	}

	// Validate version
	if state.Version == 0 {
		state.Version = StateVersion
	}

	return &state, nil
}

// SaveState saves the state to the state file.
// Creates the file if it doesn't exist, overwrites if it does.
func (m *StateManager) SaveState(state *State) error {
	if err := m.acquireLock(); err != nil {
		return err
	}
	defer m.releaseLock()

	return m.saveStateUnlocked(state)
}

// saveStateUnlocked saves state without acquiring lock (for internal use when already locked).
func (m *StateManager) saveStateUnlocked(state *State) error {
	if state == nil {
		return errors.New("cannot save nil state")
	}

	// Ensure version is set
	if state.Version == 0 {
		state.Version = StateVersion
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	statePath := m.statePath()

	// Write to temp file first, then rename for atomicity
	tempPath := statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tempPath, statePath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// ClearState removes the state file, indicating no active session.
func (m *StateManager) ClearState() error {
	if err := m.acquireLock(); err != nil {
		return err
	}
	defer m.releaseLock()

	statePath := m.statePath()

	if err := os.Remove(statePath); err != nil {
		if os.IsNotExist(err) {
			// Already cleared - not an error
			return nil
		}
		return fmt.Errorf("failed to remove state file: %w", err)
	}

	return nil
}

// HasActiveInstance returns true if there is an active instance in the state.
func (m *StateManager) HasActiveInstance() (bool, error) {
	state, err := m.LoadState()
	if err != nil {
		return false, err
	}
	return state != nil && state.Instance != nil, nil
}

// GetInstance returns the active instance state, or ErrNoActiveInstance if none.
func (m *StateManager) GetInstance() (*InstanceState, error) {
	state, err := m.LoadState()
	if err != nil {
		return nil, err
	}
	if state == nil || state.Instance == nil {
		return nil, ErrNoActiveInstance
	}
	return state.Instance, nil
}

// UpdateCost updates the accumulated cost in the state.
func (m *StateManager) UpdateCost(accumulated float64) error {
	if err := m.acquireLock(); err != nil {
		return err
	}
	defer m.releaseLock()

	state, err := m.loadStateUnlocked()
	if err != nil {
		return err
	}
	if state == nil || state.Cost == nil {
		return ErrNoActiveInstance
	}

	state.Cost.Accumulated = accumulated
	return m.saveStateUnlocked(state)
}

// UpdateHeartbeat updates the last heartbeat timestamp in the state.
func (m *StateManager) UpdateHeartbeat() error {
	if err := m.acquireLock(); err != nil {
		return err
	}
	defer m.releaseLock()

	state, err := m.loadStateUnlocked()
	if err != nil {
		return err
	}
	if state == nil || state.Deadman == nil {
		return ErrNoActiveInstance
	}

	state.Deadman.LastHeartbeat = time.Now().UTC()
	return m.saveStateUnlocked(state)
}

// UpdateModelStatus updates the model status in the state.
func (m *StateManager) UpdateModelStatus(status string) error {
	if err := m.acquireLock(); err != nil {
		return err
	}
	defer m.releaseLock()

	state, err := m.loadStateUnlocked()
	if err != nil {
		return err
	}
	if state == nil || state.Model == nil {
		return ErrNoActiveInstance
	}

	state.Model.Status = status
	return m.saveStateUnlocked(state)
}

// NewState creates a new State with the given instance information.
// This is a convenience function for starting a new session.
func NewState(instance *InstanceState, model *ModelState, wireguard *WireGuardState, cost *CostState, deadman *DeadmanState) *State {
	return &State{
		Version:   StateVersion,
		Instance:  instance,
		Model:     model,
		WireGuard: wireguard,
		Cost:      cost,
		Deadman:   deadman,
	}
}

// Duration returns how long the instance has been running.
func (s *InstanceState) Duration() time.Duration {
	if s == nil {
		return 0
	}
	return time.Since(s.CreatedAt)
}

// IsSpot returns true if the instance is a spot instance.
func (s *InstanceState) IsSpot() bool {
	return s != nil && s.Type == "spot"
}

// CalculateAccumulatedCost calculates the current accumulated cost based on hourly rate and duration.
func (s *State) CalculateAccumulatedCost() float64 {
	if s == nil || s.Instance == nil || s.Cost == nil {
		return 0
	}
	hours := s.Instance.Duration().Hours()
	return hours * s.Cost.HourlyRate
}
