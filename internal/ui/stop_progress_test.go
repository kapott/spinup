package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmeurs/spinup/internal/deploy"
)

func TestNewStopProgressModel(t *testing.T) {
	m := NewStopProgressModel()

	// Check that all steps are initialized
	if len(m.steps) != deploy.TotalStopSteps {
		t.Errorf("expected %d steps, got %d", deploy.TotalStopSteps, len(m.steps))
	}

	// Check all steps start as pending
	for i, step := range m.steps {
		if step.State != StopStepStatePending {
			t.Errorf("step %d should be pending, got %v", i, step.State)
		}
	}

	// Check initial state
	if m.currentStep != -1 {
		t.Errorf("expected currentStep -1, got %d", m.currentStep)
	}
	if m.completed {
		t.Error("model should not be completed initially")
	}
	if m.failed {
		t.Error("model should not be failed initially")
	}
}

func TestStopProgressModel_Update_ProgressUpdate(t *testing.T) {
	m := NewStopProgressModel()

	// Simulate step 1 starting
	progress := deploy.StopProgress{
		Step:       deploy.StopStepTerminate,
		TotalSteps: deploy.TotalStopSteps,
		Message:    "Terminating instance...",
		Completed:  false,
	}

	msg := StopProgressUpdateMsg{Progress: progress}
	m, _ = m.Update(msg)

	if m.steps[0].State != StopStepStateInProgress {
		t.Errorf("step 0 should be in progress, got %v", m.steps[0].State)
	}
	if m.currentStep != 0 {
		t.Errorf("expected currentStep 0, got %d", m.currentStep)
	}
}

func TestStopProgressModel_Update_StepCompleted(t *testing.T) {
	m := NewStopProgressModel()

	// Start step 1
	m, _ = m.Update(StopProgressUpdateMsg{
		Progress: deploy.StopProgress{
			Step:      deploy.StopStepTerminate,
			Message:   "Terminating instance...",
			Completed: false,
		},
	})

	// Complete step 1
	m, _ = m.Update(StopProgressUpdateMsg{
		Progress: deploy.StopProgress{
			Step:      deploy.StopStepTerminate,
			Message:   "Instance terminated",
			Completed: true,
		},
	})

	if m.steps[0].State != StopStepStateCompleted {
		t.Errorf("step 0 should be completed, got %v", m.steps[0].State)
	}
}

func TestStopProgressModel_Update_StepWarning(t *testing.T) {
	m := NewStopProgressModel()

	// Complete step with warning
	m, _ = m.Update(StopProgressUpdateMsg{
		Progress: deploy.StopProgress{
			Step:      deploy.StopStepVerifyBilling,
			Message:   "Billing verification not available",
			Detail:    "Manual verification required",
			Completed: true,
			Warning:   true,
		},
	})

	if m.steps[1].State != StopStepStateWarning {
		t.Errorf("step 1 should have warning state, got %v", m.steps[1].State)
	}
}

func TestStopProgressModel_Update_Complete(t *testing.T) {
	m := NewStopProgressModel()

	result := &StopProgressResult{
		InstanceID:      "test-123",
		Provider:        "vast",
		SessionCost:     2.93,
		SessionDuration: 4*time.Hour + 28*time.Minute,
	}

	m, _ = m.Update(StopProgressCompleteMsg{Result: result})

	if !m.completed {
		t.Error("model should be completed")
	}
	if m.result == nil {
		t.Error("result should not be nil")
	}
	if m.result.SessionCost != 2.93 {
		t.Errorf("expected session cost 2.93, got %f", m.result.SessionCost)
	}
	if !m.waitingForKey {
		t.Error("should be waiting for key after completion")
	}
}

func TestStopProgressModel_Update_Error(t *testing.T) {
	m := NewStopProgressModel()
	m.currentStep = 0

	err := deploy.ErrTerminateFailed
	m, _ = m.Update(StopProgressErrorMsg{Err: err})

	if !m.failed {
		t.Error("model should be failed")
	}
	if m.err != err {
		t.Errorf("expected error %v, got %v", err, m.err)
	}
	if !m.waitingForKey {
		t.Error("should be waiting for key after error")
	}
}

func TestStopProgressModel_Update_ManualVerification(t *testing.T) {
	m := NewStopProgressModel()

	verification := deploy.NewManualVerification("paperspace", "test-123", "https://console.paperspace.com")
	m, _ = m.Update(StopManualVerificationMsg{Verification: verification})

	if m.manualVerification == nil {
		t.Error("manual verification should not be nil")
	}
	if m.manualVerification.Provider != "paperspace" {
		t.Errorf("expected provider paperspace, got %s", m.manualVerification.Provider)
	}
}

func TestStopProgressModel_View_Basic(t *testing.T) {
	m := NewStopProgressModel()
	m.SetDimensions(80, 24)

	view := m.View()

	// Check that the view contains the title
	if view == "" {
		t.Error("view should not be empty")
	}
}

func TestStopProgressModel_View_WithSummary(t *testing.T) {
	m := NewStopProgressModel()
	m.SetDimensions(80, 24)
	m.completed = true
	m.waitingForKey = true
	m.result = &StopProgressResult{
		InstanceID:      "test-123",
		Provider:        "vast",
		SessionCost:     2.93,
		SessionDuration: 4*time.Hour + 28*time.Minute,
	}

	view := m.View()

	// Check that the view contains the session summary
	if view == "" {
		t.Error("view should not be empty")
	}
	// The view should contain cost info
	// Note: We can't easily check exact content due to styling
}

func TestStopProgressModel_Getters(t *testing.T) {
	m := NewStopProgressModel()

	if m.IsCompleted() {
		t.Error("should not be completed initially")
	}
	if m.IsFailed() {
		t.Error("should not be failed initially")
	}
	if m.Error() != nil {
		t.Error("should have no error initially")
	}
	if m.Result() != nil {
		t.Error("should have no result initially")
	}
	if m.ManualVerification() != nil {
		t.Error("should have no manual verification initially")
	}
	if m.IsWaitingForKey() {
		t.Error("should not be waiting for key initially")
	}
}

func TestResultFromStopResult(t *testing.T) {
	deployResult := &deploy.StopResult{
		InstanceID:                 "test-123",
		Provider:                   "vast",
		BillingVerified:            true,
		ManualVerificationRequired: false,
		SessionCost:                2.93,
		SessionDuration:            4*time.Hour + 28*time.Minute,
		StartedAt:                  time.Now().Add(-10 * time.Second),
		CompletedAt:                time.Now(),
	}

	result := ResultFromStopResult(deployResult)

	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.InstanceID != "test-123" {
		t.Errorf("expected instance ID test-123, got %s", result.InstanceID)
	}
	if result.Provider != "vast" {
		t.Errorf("expected provider vast, got %s", result.Provider)
	}
	if result.SessionCost != 2.93 {
		t.Errorf("expected session cost 2.93, got %f", result.SessionCost)
	}
	if result.BillingVerified != true {
		t.Error("billing should be verified")
	}
}

func TestResultFromStopResult_Nil(t *testing.T) {
	result := ResultFromStopResult(nil)
	if result != nil {
		t.Error("result should be nil for nil input")
	}
}

func TestUpdateStopProgress(t *testing.T) {
	progress := deploy.StopProgress{
		Step:    deploy.StopStepTerminate,
		Message: "Terminating...",
	}

	cmd := UpdateStopProgress(progress)
	msg := cmd()

	if update, ok := msg.(StopProgressUpdateMsg); ok {
		if update.Progress.Step != deploy.StopStepTerminate {
			t.Error("unexpected step in message")
		}
	} else {
		t.Error("expected StopProgressUpdateMsg")
	}
}

func TestCompleteStop(t *testing.T) {
	result := &StopProgressResult{InstanceID: "test"}
	cmd := CompleteStop(result)
	msg := cmd()

	if complete, ok := msg.(StopProgressCompleteMsg); ok {
		if complete.Result.InstanceID != "test" {
			t.Error("unexpected result in message")
		}
	} else {
		t.Error("expected StopProgressCompleteMsg")
	}
}

func TestFailStop(t *testing.T) {
	err := deploy.ErrTerminateFailed
	cmd := FailStop(err)
	msg := cmd()

	if fail, ok := msg.(StopProgressErrorMsg); ok {
		if fail.Err != err {
			t.Error("unexpected error in message")
		}
	} else {
		t.Error("expected StopProgressErrorMsg")
	}
}

func TestSetManualVerification(t *testing.T) {
	v := deploy.NewManualVerification("paperspace", "id", "url")
	cmd := SetManualVerification(v)
	msg := cmd()

	if mv, ok := msg.(StopManualVerificationMsg); ok {
		if mv.Verification.Provider != "paperspace" {
			t.Error("unexpected provider in message")
		}
	} else {
		t.Error("expected StopManualVerificationMsg")
	}
}

func TestMakeStopProgressCallback(t *testing.T) {
	// We can't fully test this without a real tea.Program,
	// but we can at least verify the function returns a non-nil callback
	// In a real test environment, we'd mock the tea.Program

	// Note: This is a compile-time type check essentially
	var _ func(deploy.StopProgress) = MakeStopProgressCallback(nil)
}

func TestMakeManualVerificationCallback(t *testing.T) {
	// Similar to above, this verifies the type signature
	var _ deploy.ManualVerificationCallback = MakeManualVerificationCallback(nil)
}

func TestStopProgressModel_KeyHandling(t *testing.T) {
	m := NewStopProgressModel()

	// Test q key when not waiting
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(keyMsg)
	if cmd == nil {
		t.Error("q key should trigger quit command")
	}
}

func TestStopProgressModel_KeyHandling_WaitingForKey(t *testing.T) {
	m := NewStopProgressModel()
	m.waitingForKey = true

	// Any key should quit when waiting
	keyMsg := tea.KeyMsg{Type: tea.KeySpace}
	_, cmd := m.Update(keyMsg)
	if cmd == nil {
		t.Error("any key should trigger quit when waiting")
	}
}
