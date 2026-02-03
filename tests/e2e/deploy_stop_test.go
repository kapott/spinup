// Package e2e provides end-to-end tests for the continueplz application.
// These tests validate full workflows from user perspective.
package e2e

import (
	"testing"
	"time"

	"github.com/tmeurs/continueplz/internal/config"
	"github.com/tmeurs/continueplz/internal/deploy"
	"github.com/tmeurs/continueplz/internal/provider"
	"github.com/tmeurs/continueplz/internal/provider/mock"
	"github.com/tmeurs/continueplz/tests/integration"
)

// TestFullDeployStopCycle tests the complete lifecycle:
// 1. Setup environment (init equivalent)
// 2. Deploy an instance
// 3. Verify instance is running and state is correct
// 4. Stop the instance
// 5. Verify instance is terminated and state is cleared
func TestFullDeployStopCycle(t *testing.T) {
	integration.SkipIfShort(t)

	// Create test environment with mock provider
	env := integration.NewTestEnv(t,
		integration.WithTimeout(60*time.Second),
		integration.WithMockOffers(integration.DefaultOffers()),
	)
	defer env.Cleanup()

	env.RequireMock()

	// Get the mock provider and context
	mockProvider := env.MockProvider
	ctx, cancel := env.Context()
	defer cancel()

	// Step 1: Verify initial state - no active instance
	t.Run("InitialState_NoActiveInstance", func(t *testing.T) {
		state, err := env.StateManager.LoadState()
		env.AssertNoError(err, "Loading initial state")
		env.AssertTrue(state == nil, "Initial state should be nil (no active instance)")
	})

	// Step 2: Simulate deployment by creating an instance and saving state
	var createdInstance *provider.Instance
	t.Run("Deploy_CreateInstance", func(t *testing.T) {
		// Get an available offer
		offers, err := mockProvider.GetOffers(ctx, provider.OfferFilter{})
		env.AssertNoError(err, "Getting offers")
		env.AssertTrue(len(offers) > 0, "Should have at least one offer")

		// Create instance
		req := provider.CreateRequest{
			OfferID:    offers[0].OfferID,
			Spot:       false,
			CloudInit:  "#!/bin/bash\necho 'hello'",
			DiskSizeGB: 100,
		}

		instance, err := mockProvider.CreateInstance(ctx, req)
		env.AssertNoError(err, "Creating instance")
		env.AssertTrue(instance != nil, "Instance should not be nil")
		env.AssertTrue(instance.ID != "", "Instance ID should not be empty")
		env.AssertEqual(instance.Status, provider.InstanceStatusRunning, "Instance should be running")

		createdInstance = instance

		// Verify mock recorded the call
		env.AssertTrue(len(mockProvider.CreateInstanceCalls) == 1, "Should have recorded one CreateInstance call")
	})

	// Step 3: Save state (simulating what the deploy command does)
	t.Run("Deploy_SaveState", func(t *testing.T) {
		state := config.NewState(
			&config.InstanceState{
				ID:          createdInstance.ID,
				Provider:    createdInstance.Provider,
				GPU:         createdInstance.GPU,
				Region:      createdInstance.Region,
				Type:        "on-demand",
				PublicIP:    createdInstance.PublicIP,
				WireGuardIP: "10.100.0.1",
				CreatedAt:   createdInstance.CreatedAt,
			},
			&config.ModelState{
				Name:   "qwen2.5-coder:7b",
				Status: "ready",
			},
			&config.WireGuardState{
				ServerPublicKey: "test-server-pubkey",
				InterfaceName:   "wg-continueplz",
			},
			&config.CostState{
				HourlyRate:  createdInstance.HourlyRate,
				Accumulated: 0,
				Currency:    "EUR",
			},
			&config.DeadmanState{
				TimeoutHours:  10,
				LastHeartbeat: time.Now().UTC(),
			},
		)

		err := env.StateManager.SaveState(state)
		env.AssertNoError(err, "Saving state after deployment")
	})

	// Step 4: Verify state is correctly saved
	t.Run("Deploy_VerifyState", func(t *testing.T) {
		state, err := env.StateManager.LoadState()
		env.AssertNoError(err, "Loading state after deployment")
		env.AssertTrue(state != nil, "State should exist after deployment")
		env.AssertTrue(state.Instance != nil, "Instance state should exist")
		env.AssertEqual(state.Instance.ID, createdInstance.ID, "Instance ID should match")
		env.AssertEqual(state.Instance.Provider, "mock", "Provider should be mock")
		env.AssertTrue(state.Model != nil, "Model state should exist")
		env.AssertEqual(state.Model.Name, "qwen2.5-coder:7b", "Model name should match")
		env.AssertEqual(state.Model.Status, "ready", "Model status should be ready")
		env.AssertTrue(state.WireGuard != nil, "WireGuard state should exist")
		env.AssertTrue(state.Cost != nil, "Cost state should exist")
		env.AssertTrue(state.Deadman != nil, "Deadman state should exist")
	})

	// Step 5: Verify instance is still running on provider
	t.Run("Verify_InstanceRunning", func(t *testing.T) {
		instance, err := mockProvider.GetInstance(ctx, createdInstance.ID)
		env.AssertNoError(err, "Getting instance")
		env.AssertEqual(instance.Status, provider.InstanceStatusRunning, "Instance should still be running")

		// Verify billing status
		billingStatus, err := mockProvider.GetBillingStatus(ctx, createdInstance.ID)
		env.AssertNoError(err, "Getting billing status")
		env.AssertEqual(billingStatus, provider.BillingActive, "Billing should be active")
	})

	// Step 6: Stop/terminate the instance
	t.Run("Stop_TerminateInstance", func(t *testing.T) {
		err := mockProvider.TerminateInstance(ctx, createdInstance.ID)
		env.AssertNoError(err, "Terminating instance")

		// Verify mock recorded the call
		env.AssertTrue(len(mockProvider.TerminateInstanceCalls) == 1, "Should have recorded one TerminateInstance call")
	})

	// Step 7: Verify instance is terminated
	t.Run("Stop_VerifyTerminated", func(t *testing.T) {
		instance, err := mockProvider.GetInstance(ctx, createdInstance.ID)
		env.AssertNoError(err, "Getting terminated instance")
		env.AssertEqual(instance.Status, provider.InstanceStatusTerminated, "Instance should be terminated")

		// Verify billing stopped
		billingStatus, err := mockProvider.GetBillingStatus(ctx, createdInstance.ID)
		env.AssertNoError(err, "Getting billing status after termination")
		env.AssertEqual(billingStatus, provider.BillingStopped, "Billing should be stopped")
	})

	// Step 8: Clear state (simulating what the stop command does)
	t.Run("Stop_ClearState", func(t *testing.T) {
		err := env.StateManager.ClearState()
		env.AssertNoError(err, "Clearing state after stop")
	})

	// Step 9: Verify state is cleared
	t.Run("Stop_VerifyStateClear", func(t *testing.T) {
		state, err := env.StateManager.LoadState()
		env.AssertNoError(err, "Loading state after clear")
		env.AssertTrue(state == nil, "State should be nil after stop")
	})
}

// TestDeployStopCycle_SpotInstance tests the cycle with a spot instance.
func TestDeployStopCycle_SpotInstance(t *testing.T) {
	integration.SkipIfShort(t)

	env := integration.NewTestEnv(t,
		integration.WithTimeout(60*time.Second),
	)
	defer env.Cleanup()

	env.RequireMock()

	// Load spot_available fixture
	env.LoadFixture("spot_available")

	mockProvider := env.MockProvider
	ctx, cancel := env.Context()
	defer cancel()

	// Get spot offer
	offers, err := mockProvider.GetOffers(ctx, provider.OfferFilter{SpotOnly: true})
	env.AssertNoError(err, "Getting spot offers")
	env.AssertTrue(len(offers) > 0, "Should have spot offers")
	env.AssertTrue(offers[0].SpotPrice != nil, "Offer should have spot price")

	// Create spot instance
	req := provider.CreateRequest{
		OfferID:    offers[0].OfferID,
		Spot:       true,
		CloudInit:  "#!/bin/bash\necho 'spot'",
		DiskSizeGB: 100,
	}

	instance, err := mockProvider.CreateInstance(ctx, req)
	env.AssertNoError(err, "Creating spot instance")
	env.AssertTrue(instance.Spot, "Instance should be spot")

	// Save state
	state := config.NewState(
		&config.InstanceState{
			ID:          instance.ID,
			Provider:    instance.Provider,
			GPU:         instance.GPU,
			Region:      instance.Region,
			Type:        "spot",
			PublicIP:    instance.PublicIP,
			WireGuardIP: "10.100.0.1",
			CreatedAt:   instance.CreatedAt,
		},
		&config.ModelState{
			Name:   "qwen2.5-coder:7b",
			Status: "ready",
		},
		nil, nil, nil,
	)

	err = env.StateManager.SaveState(state)
	env.AssertNoError(err, "Saving spot instance state")

	// Verify state shows spot
	loadedState, err := env.StateManager.LoadState()
	env.AssertNoError(err, "Loading spot state")
	env.AssertEqual(loadedState.Instance.Type, "spot", "State should show spot type")

	// Terminate
	err = mockProvider.TerminateInstance(ctx, instance.ID)
	env.AssertNoError(err, "Terminating spot instance")

	// Clear state
	err = env.StateManager.ClearState()
	env.AssertNoError(err, "Clearing spot state")

	// Verify cleared
	finalState, err := env.StateManager.LoadState()
	env.AssertNoError(err, "Loading final state")
	env.AssertTrue(finalState == nil, "State should be nil after clear")
}

// TestDeployStopCycle_StateConsistency tests that state is correctly managed
// through various error scenarios and recovery.
func TestDeployStopCycle_StateConsistency(t *testing.T) {
	integration.SkipIfShort(t)

	env := integration.NewTestEnv(t,
		integration.WithTimeout(60*time.Second),
	)
	defer env.Cleanup()

	env.RequireMock()

	mockProvider := env.MockProvider
	ctx, cancel := env.Context()
	defer cancel()

	// Create instance
	offers, err := mockProvider.GetOffers(ctx, provider.OfferFilter{})
	env.AssertNoError(err, "Getting offers")

	req := provider.CreateRequest{
		OfferID:    offers[0].OfferID,
		Spot:       false,
		CloudInit:  "#!/bin/bash\necho 'test'",
		DiskSizeGB: 100,
	}

	instance, err := mockProvider.CreateInstance(ctx, req)
	env.AssertNoError(err, "Creating instance")

	// Save initial state
	state := config.NewState(
		&config.InstanceState{
			ID:        instance.ID,
			Provider:  instance.Provider,
			GPU:       instance.GPU,
			Region:    instance.Region,
			Type:      "on-demand",
			CreatedAt: instance.CreatedAt,
		},
		&config.ModelState{
			Name:   "test-model",
			Status: "loading",
		},
		nil, nil, nil,
	)
	err = env.StateManager.SaveState(state)
	env.AssertNoError(err, "Saving initial state")

	// Update model status to ready (simulating model pull completion)
	loadedState, err := env.StateManager.LoadState()
	env.AssertNoError(err, "Loading state for update")

	loadedState.Model.Status = "ready"
	err = env.StateManager.SaveState(loadedState)
	env.AssertNoError(err, "Saving updated state")

	// Verify update persisted
	verifyState, err := env.StateManager.LoadState()
	env.AssertNoError(err, "Loading verified state")
	env.AssertEqual(verifyState.Model.Status, "ready", "Model status should be updated")

	// Multiple load/save cycles should be consistent
	for i := 0; i < 5; i++ {
		s, err := env.StateManager.LoadState()
		env.AssertNoError(err, "Load cycle iteration")
		env.AssertEqual(s.Instance.ID, instance.ID, "Instance ID should be consistent")
		env.AssertEqual(s.Model.Status, "ready", "Model status should be consistent")
	}

	// Cleanup
	err = mockProvider.TerminateInstance(ctx, instance.ID)
	env.AssertNoError(err, "Cleanup termination")

	err = env.StateManager.ClearState()
	env.AssertNoError(err, "Cleanup state clear")
}

// TestDeployStopCycle_IdempotentTermination tests that termination is idempotent.
func TestDeployStopCycle_IdempotentTermination(t *testing.T) {
	integration.SkipIfShort(t)

	env := integration.NewTestEnv(t,
		integration.WithTimeout(60*time.Second),
	)
	defer env.Cleanup()

	env.RequireMock()

	mockProvider := env.MockProvider
	ctx, cancel := env.Context()
	defer cancel()

	// Create instance
	offers, err := mockProvider.GetOffers(ctx, provider.OfferFilter{})
	env.AssertNoError(err, "Getting offers")

	req := provider.CreateRequest{
		OfferID:    offers[0].OfferID,
		Spot:       false,
		CloudInit:  "#!/bin/bash\necho 'idempotent'",
		DiskSizeGB: 100,
	}

	instance, err := mockProvider.CreateInstance(ctx, req)
	env.AssertNoError(err, "Creating instance")

	// Terminate multiple times - should all succeed (idempotent)
	for i := 0; i < 3; i++ {
		err = mockProvider.TerminateInstance(ctx, instance.ID)
		env.AssertNoError(err, "Termination call should be idempotent")
	}

	// Verify instance is terminated
	terminatedInstance, err := mockProvider.GetInstance(ctx, instance.ID)
	env.AssertNoError(err, "Getting terminated instance")
	env.AssertEqual(terminatedInstance.Status, provider.InstanceStatusTerminated, "Instance should be terminated")

	// Terminating non-existent instance should also succeed (idempotent)
	err = mockProvider.TerminateInstance(ctx, "non-existent-id")
	env.AssertNoError(err, "Terminating non-existent instance should succeed")
}

// TestDeployStopCycle_BillingVerification tests billing verification flow.
func TestDeployStopCycle_BillingVerification(t *testing.T) {
	integration.SkipIfShort(t)

	env := integration.NewTestEnv(t,
		integration.WithTimeout(60*time.Second),
	)
	defer env.Cleanup()

	env.RequireMock()

	mockProvider := env.MockProvider
	ctx, cancel := env.Context()
	defer cancel()

	// Create and start instance
	offers, err := mockProvider.GetOffers(ctx, provider.OfferFilter{})
	env.AssertNoError(err, "Getting offers")

	req := provider.CreateRequest{
		OfferID:    offers[0].OfferID,
		Spot:       false,
		CloudInit:  "#!/bin/bash\necho 'billing'",
		DiskSizeGB: 100,
	}

	instance, err := mockProvider.CreateInstance(ctx, req)
	env.AssertNoError(err, "Creating instance")

	// Verify billing is active while running
	billingStatus, err := mockProvider.GetBillingStatus(ctx, instance.ID)
	env.AssertNoError(err, "Getting billing status while running")
	env.AssertEqual(billingStatus, provider.BillingActive, "Billing should be active")

	// Terminate
	err = mockProvider.TerminateInstance(ctx, instance.ID)
	env.AssertNoError(err, "Terminating instance")

	// Verify billing stopped
	billingStatus, err = mockProvider.GetBillingStatus(ctx, instance.ID)
	env.AssertNoError(err, "Getting billing status after termination")
	env.AssertEqual(billingStatus, provider.BillingStopped, "Billing should be stopped")

	// Verify supports billing verification flag
	env.AssertTrue(mockProvider.SupportsBillingVerification(), "Mock should support billing verification")
}

// TestDeployStopCycle_NoBillingVerification tests flow when provider doesn't support billing verification.
func TestDeployStopCycle_NoBillingVerification(t *testing.T) {
	integration.SkipIfShort(t)

	env := integration.NewTestEnv(t,
		integration.WithTimeout(60*time.Second),
	)
	defer env.Cleanup()

	// Create a mock provider that doesn't support billing verification
	noBillingMock := mock.New(
		mock.WithName("no-billing-provider"),
		mock.WithOffers(integration.DefaultOffers()),
		mock.WithBillingVerificationSupport(false),
	)

	ctx, cancel := env.Context()
	defer cancel()

	// Verify it doesn't support billing verification
	env.AssertFalse(noBillingMock.SupportsBillingVerification(), "Should not support billing verification")

	// Create instance
	offers, err := noBillingMock.GetOffers(ctx, provider.OfferFilter{})
	env.AssertNoError(err, "Getting offers")

	req := provider.CreateRequest{
		OfferID:    offers[0].OfferID,
		Spot:       false,
		CloudInit:  "#!/bin/bash\necho 'no-billing'",
		DiskSizeGB: 100,
	}

	instance, err := noBillingMock.CreateInstance(ctx, req)
	env.AssertNoError(err, "Creating instance")

	// Terminate
	err = noBillingMock.TerminateInstance(ctx, instance.ID)
	env.AssertNoError(err, "Terminating instance")

	// Manual verification would be required in real scenario
	// The deploy.CheckManualVerificationRequired would return Required: true
	manualVerify := deploy.CheckManualVerificationRequired(noBillingMock, instance.ID)
	env.AssertTrue(manualVerify.Required, "Manual verification should be required")
	env.AssertEqual(manualVerify.Provider, "no-billing-provider", "Provider name should match")
}

// TestDeployStopCycle_MultipleInstances tests that state correctly tracks only one instance.
func TestDeployStopCycle_MultipleInstances(t *testing.T) {
	integration.SkipIfShort(t)

	env := integration.NewTestEnv(t,
		integration.WithTimeout(60*time.Second),
	)
	defer env.Cleanup()

	env.RequireMock()

	mockProvider := env.MockProvider
	ctx, cancel := env.Context()
	defer cancel()

	// Create first instance
	offers, err := mockProvider.GetOffers(ctx, provider.OfferFilter{})
	env.AssertNoError(err, "Getting offers")

	req1 := provider.CreateRequest{
		OfferID:    offers[0].OfferID,
		Spot:       false,
		CloudInit:  "#!/bin/bash\necho 'first'",
		DiskSizeGB: 100,
	}

	instance1, err := mockProvider.CreateInstance(ctx, req1)
	env.AssertNoError(err, "Creating first instance")

	// Save state for first instance
	state1 := config.NewState(
		&config.InstanceState{
			ID:        instance1.ID,
			Provider:  instance1.Provider,
			GPU:       instance1.GPU,
			Region:    instance1.Region,
			Type:      "on-demand",
			CreatedAt: instance1.CreatedAt,
		},
		nil, nil, nil, nil,
	)
	err = env.StateManager.SaveState(state1)
	env.AssertNoError(err, "Saving first instance state")

	// Verify state shows first instance
	loadedState, err := env.StateManager.LoadState()
	env.AssertNoError(err, "Loading state")
	env.AssertEqual(loadedState.Instance.ID, instance1.ID, "State should show first instance")

	// Stop first instance
	err = mockProvider.TerminateInstance(ctx, instance1.ID)
	env.AssertNoError(err, "Terminating first instance")

	err = env.StateManager.ClearState()
	env.AssertNoError(err, "Clearing first instance state")

	// Create second instance
	req2 := provider.CreateRequest{
		OfferID:    offers[0].OfferID,
		Spot:       false,
		CloudInit:  "#!/bin/bash\necho 'second'",
		DiskSizeGB: 100,
	}

	instance2, err := mockProvider.CreateInstance(ctx, req2)
	env.AssertNoError(err, "Creating second instance")
	env.AssertTrue(instance1.ID != instance2.ID, "Second instance should have different ID")

	// Save state for second instance
	state2 := config.NewState(
		&config.InstanceState{
			ID:        instance2.ID,
			Provider:  instance2.Provider,
			GPU:       instance2.GPU,
			Region:    instance2.Region,
			Type:      "on-demand",
			CreatedAt: instance2.CreatedAt,
		},
		nil, nil, nil, nil,
	)
	err = env.StateManager.SaveState(state2)
	env.AssertNoError(err, "Saving second instance state")

	// Verify state shows second instance (not first)
	loadedState, err = env.StateManager.LoadState()
	env.AssertNoError(err, "Loading state for second instance")
	env.AssertEqual(loadedState.Instance.ID, instance2.ID, "State should show second instance")

	// Cleanup
	err = mockProvider.TerminateInstance(ctx, instance2.ID)
	env.AssertNoError(err, "Terminating second instance")

	err = env.StateManager.ClearState()
	env.AssertNoError(err, "Final state clear")
}

// TestDeployStopCycle_ErrorRecovery tests error handling during the cycle.
func TestDeployStopCycle_ErrorRecovery(t *testing.T) {
	integration.SkipIfShort(t)

	env := integration.NewTestEnv(t,
		integration.WithTimeout(60*time.Second),
	)
	defer env.Cleanup()

	env.RequireMock()

	mockProvider := env.MockProvider
	ctx, cancel := env.Context()
	defer cancel()

	// Test 1: Error during instance creation
	t.Run("CreateInstanceError", func(t *testing.T) {
		// Configure mock to return error on create
		env.SetupMockError("CreateInstance", provider.ErrInsufficientCapacity)

		req := provider.CreateRequest{
			OfferID:    "mock-offer-1",
			Spot:       false,
			CloudInit:  "#!/bin/bash\necho 'error'",
			DiskSizeGB: 100,
		}

		_, err := mockProvider.CreateInstance(ctx, req)
		env.AssertError(err, "CreateInstance should fail when error is set")

		// State should not be saved on error
		state, err := env.StateManager.LoadState()
		env.AssertNoError(err, "Loading state after failed create")
		env.AssertTrue(state == nil, "State should be nil after failed create")

		// Reset mock
		env.ResetMock()
	})

	// Test 2: Error during termination (should still be able to retry)
	t.Run("TerminateInstanceError", func(t *testing.T) {
		// First create successfully
		offers, _ := mockProvider.GetOffers(ctx, provider.OfferFilter{})
		req := provider.CreateRequest{
			OfferID:    offers[0].OfferID,
			Spot:       false,
			CloudInit:  "#!/bin/bash\necho 'terminate-error'",
			DiskSizeGB: 100,
		}

		instance, err := mockProvider.CreateInstance(ctx, req)
		env.AssertNoError(err, "Creating instance for termination error test")

		// Configure mock to return error on first terminate
		env.SetupMockError("TerminateInstance", provider.ErrRateLimited)

		err = mockProvider.TerminateInstance(ctx, instance.ID)
		env.AssertError(err, "First terminate should fail")

		// Clear the error and retry
		env.SetupMockError("TerminateInstance", nil)

		err = mockProvider.TerminateInstance(ctx, instance.ID)
		env.AssertNoError(err, "Retry terminate should succeed")

		// Verify terminated
		terminatedInstance, err := mockProvider.GetInstance(ctx, instance.ID)
		env.AssertNoError(err, "Getting terminated instance")
		env.AssertEqual(terminatedInstance.Status, provider.InstanceStatusTerminated, "Should be terminated after retry")

		env.ResetMock()
	})
}
