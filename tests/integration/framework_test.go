package integration

import (
	"context"
	"testing"
	"time"

	"github.com/tmeurs/spinup/internal/provider"
)

func TestNewTestEnv(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	if env.T != t {
		t.Error("TestEnv.T should be the test instance")
	}

	if env.StateManager == nil {
		t.Error("StateManager should not be nil")
	}

	if env.Config == nil {
		t.Error("Config should not be nil")
	}

	if !env.UseMock {
		t.Error("UseMock should be true by default")
	}

	if env.MockProvider == nil {
		t.Error("MockProvider should not be nil when UseMock is true")
	}
}

func TestTestEnv_GetProvider(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	p := env.GetProvider()
	if p == nil {
		t.Fatal("GetProvider should return mock provider")
	}

	if p.Name() != "mock" {
		t.Errorf("Provider name should be 'mock', got %s", p.Name())
	}
}

func TestTestEnv_Context(t *testing.T) {
	env := NewTestEnv(t, WithTimeout(5*time.Second))
	defer env.Cleanup()

	ctx, cancel := env.Context()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Context should have a deadline")
	}

	remaining := time.Until(deadline)
	if remaining > 5*time.Second || remaining < 4*time.Second {
		t.Errorf("Context deadline should be around 5s, got %v", remaining)
	}
}

func TestTestEnv_MockOperations(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	ctx, cancel := env.Context()
	defer cancel()

	p := env.GetProvider()

	// Test GetOffers with default offers
	offers, err := p.GetOffers(ctx, provider.OfferFilter{})
	if err != nil {
		t.Fatalf("GetOffers failed: %v", err)
	}
	if len(offers) == 0 {
		t.Error("Default offers should not be empty")
	}

	// Test CreateInstance
	instance, err := p.CreateInstance(ctx, provider.CreateRequest{
		OfferID:    offers[0].OfferID,
		Spot:       false,
		DiskSizeGB: 100,
	})
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	if instance == nil {
		t.Fatal("Instance should not be nil")
	}
	if instance.Status != provider.InstanceStatusRunning {
		t.Errorf("Instance status should be running, got %s", instance.Status)
	}

	// Test GetInstance
	retrieved, err := p.GetInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if retrieved.ID != instance.ID {
		t.Error("Retrieved instance ID should match")
	}

	// Test TerminateInstance
	err = p.TerminateInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("TerminateInstance failed: %v", err)
	}

	// Verify terminated
	terminated, err := p.GetInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("GetInstance after terminate failed: %v", err)
	}
	if terminated.Status != provider.InstanceStatusTerminated {
		t.Errorf("Instance status should be terminated, got %s", terminated.Status)
	}
}

func TestTestEnv_SetupMockError(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	ctx, cancel := env.Context()
	defer cancel()

	// Setup error
	env.SetupMockError("GetOffers", provider.ErrAuthenticationFailed)

	p := env.GetProvider()
	_, err := p.GetOffers(ctx, provider.OfferFilter{})
	if err == nil {
		t.Error("Expected error from GetOffers")
	}

	// Reset and verify error is cleared
	env.ResetMock()
	offers, err := p.GetOffers(ctx, provider.OfferFilter{})
	if err != nil {
		t.Errorf("Expected no error after reset: %v", err)
	}
	if len(offers) == 0 {
		t.Error("Should have default offers after reset")
	}
}

func TestTestEnv_SetupMockDelay(t *testing.T) {
	env := NewTestEnv(t, WithTimeout(10*time.Second))
	defer env.Cleanup()

	// Setup a short delay
	env.SetupMockDelay("GetOffers", 100*time.Millisecond)

	ctx, cancel := env.Context()
	defer cancel()

	p := env.GetProvider()

	start := time.Now()
	_, err := p.GetOffers(ctx, provider.OfferFilter{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("GetOffers failed: %v", err)
	}

	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected at least 100ms delay, got %v", elapsed)
	}
}

func TestTestEnv_LoadFixture(t *testing.T) {
	tests := []struct {
		name          string
		fixture       string
		expectedCount int
	}{
		{"empty", "empty", 0},
		{"single_offer", "single_offer", 1},
		{"multi_region", "multi_region", 3},
		{"spot_available", "spot_available", 1},
		{"high_vram", "high_vram", 2},
		{"expensive", "expensive", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewTestEnv(t)
			defer env.Cleanup()

			env.LoadFixture(tt.fixture)

			ctx, cancel := env.Context()
			defer cancel()

			p := env.GetProvider()
			offers, err := p.GetOffers(ctx, provider.OfferFilter{})
			if err != nil {
				t.Fatalf("GetOffers failed: %v", err)
			}

			if len(offers) != tt.expectedCount {
				t.Errorf("Expected %d offers, got %d", tt.expectedCount, len(offers))
			}
		})
	}
}

func TestTestEnv_Assertions(t *testing.T) {
	// These tests verify the assertion helpers don't panic on success
	env := NewTestEnv(t)
	defer env.Cleanup()

	env.AssertNoError(nil, "nil error")
	env.AssertTrue(true, "true condition")
	env.AssertFalse(false, "false condition")
	env.AssertEqual(1, 1, "equal values")
	env.AssertEqual("foo", "foo", "equal strings")
}

func TestRunTests(t *testing.T) {
	var setupCalled, runCalled, cleanupCalled bool

	tests := []TestRun{
		{
			Name: "test_case",
			Setup: func(env *TestEnv) {
				setupCalled = true
			},
			Run: func(env *TestEnv) {
				runCalled = true
			},
			Cleanup: func(env *TestEnv) {
				cleanupCalled = true
			},
		},
	}

	RunTests(t, tests)

	if !setupCalled {
		t.Error("Setup should have been called")
	}
	if !runCalled {
		t.Error("Run should have been called")
	}
	if !cleanupCalled {
		t.Error("Cleanup should have been called")
	}
}

func TestProviderTestSuite(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	p := env.GetProvider()
	ProviderTestSuite(t, p)
}

func TestDefaultOffers(t *testing.T) {
	offers := DefaultOffers()
	if len(offers) == 0 {
		t.Error("DefaultOffers should return offers")
	}

	// Verify structure
	for _, o := range offers {
		if o.OfferID == "" {
			t.Error("Offer should have an ID")
		}
		if o.GPU == "" {
			t.Error("Offer should have a GPU type")
		}
		if o.VRAM == 0 {
			t.Error("Offer should have VRAM")
		}
		if o.OnDemandPrice == 0 {
			t.Error("Offer should have a price")
		}
	}
}

func TestStandardFixtures(t *testing.T) {
	fixtures := StandardFixtures()

	expectedFixtures := []string{"empty", "single_offer", "multi_region", "spot_available", "high_vram", "expensive"}
	for _, name := range expectedFixtures {
		if _, ok := fixtures[name]; !ok {
			t.Errorf("Missing expected fixture: %s", name)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	env := NewTestEnv(t, WithTimeout(100*time.Millisecond))
	defer env.Cleanup()

	// Setup a long delay
	env.SetupMockDelay("GetOffers", 5*time.Second)

	ctx, cancel := env.Context()
	defer cancel()

	p := env.GetProvider()

	_, err := p.GetOffers(ctx, provider.OfferFilter{})
	if err == nil {
		t.Error("Expected context cancellation error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}
