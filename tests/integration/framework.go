// Package integration provides a framework for integration tests.
// Integration tests can be run against the mock provider for CI/CD
// or against real providers with API keys for end-to-end validation.
package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tmeurs/spinup/internal/config"
	"github.com/tmeurs/spinup/internal/provider"
	"github.com/tmeurs/spinup/internal/provider/mock"
)

// TestEnv holds the test environment configuration.
type TestEnv struct {
	// Config is the loaded configuration (from .env or environment).
	Config *config.Config

	// StateManager manages state file during tests.
	StateManager *config.StateManager

	// StateFile is the path to the test state file.
	StateFile string

	// MockProvider is the mock provider for isolated tests.
	MockProvider *mock.Provider

	// RealProviders contains any configured real providers.
	RealProviders map[string]provider.Provider

	// UseMock indicates whether to use the mock provider.
	UseMock bool

	// Timeout is the default timeout for operations.
	Timeout time.Duration

	// T is the testing.T instance for the current test.
	T *testing.T
}

// DefaultOffers returns a set of default mock offers for testing.
func DefaultOffers() []provider.Offer {
	spotPrice := 0.80
	return []provider.Offer{
		{
			OfferID:       "mock-offer-1",
			Provider:      "mock",
			GPU:           "A100 40GB",
			VRAM:          40,
			Region:        "EU-West",
			SpotPrice:     &spotPrice,
			OnDemandPrice: 1.50,
			StoragePrice:  0.0001,
			EgressPrice:   0.01,
			Available:     true,
		},
		{
			OfferID:       "mock-offer-2",
			Provider:      "mock",
			GPU:           "A100 80GB",
			VRAM:          80,
			Region:        "EU-West",
			SpotPrice:     nil,
			OnDemandPrice: 2.50,
			StoragePrice:  0.0001,
			EgressPrice:   0.01,
			Available:     true,
		},
		{
			OfferID:       "mock-offer-3",
			Provider:      "mock",
			GPU:           "A6000 48GB",
			VRAM:          48,
			Region:        "US-East",
			SpotPrice:     &spotPrice,
			OnDemandPrice: 1.00,
			StoragePrice:  0.0001,
			EgressPrice:   0.01,
			Available:     true,
		},
	}
}

// Option is a functional option for configuring TestEnv.
type Option func(*TestEnv)

// WithTimeout sets a custom timeout for operations.
func WithTimeout(d time.Duration) Option {
	return func(e *TestEnv) {
		e.Timeout = d
	}
}

// WithMockOffers configures the mock provider with specific offers.
func WithMockOffers(offers []provider.Offer) Option {
	return func(e *TestEnv) {
		if e.MockProvider != nil {
			e.MockProvider.SetOffers(offers)
		}
	}
}

// WithRealProviders forces use of real providers instead of mock.
func WithRealProviders() Option {
	return func(e *TestEnv) {
		e.UseMock = false
	}
}

// NewTestEnv creates a new test environment for integration tests.
// By default, it uses the mock provider for isolated testing.
// Set INTEGRATION_USE_REAL_PROVIDERS=true to test against real providers.
func NewTestEnv(t *testing.T, opts ...Option) *TestEnv {
	t.Helper()

	env := &TestEnv{
		T:             t,
		UseMock:       os.Getenv("INTEGRATION_USE_REAL_PROVIDERS") != "true",
		RealProviders: make(map[string]provider.Provider),
		Timeout:       30 * time.Second,
	}

	// Apply options first (in case they change UseMock)
	for _, opt := range opts {
		opt(env)
	}

	// Create a temporary state file for tests
	tmpDir := t.TempDir()
	env.StateFile = tmpDir + "/.spinup.state"

	var err error
	env.StateManager, err = config.NewStateManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	// Create a minimal config for tests
	env.Config = &config.Config{
		DefaultTier:          "small",
		DeadmanTimeoutHours:  10,
		PreferSpot:           true,
		WireGuardPrivateKey:  "test-private-key",
		WireGuardPublicKey:   "test-public-key",
	}

	if env.UseMock {
		// Create mock provider with default offers
		env.MockProvider = mock.New(
			mock.WithName("mock"),
			mock.WithOffers(DefaultOffers()),
			mock.WithBillingVerificationSupport(true),
		)
	} else {
		// Try to load real configuration
		cfg, _, err := config.LoadConfig("")
		if err == nil {
			env.Config = cfg
			// Load real providers would happen here via registry
			// but for safety, we don't auto-create instances
		}
	}

	// Apply options again to allow overriding defaults
	for _, opt := range opts {
		opt(env)
	}

	return env
}

// GetProvider returns the provider to use for tests.
// Returns the mock provider if UseMock is true, otherwise returns nil
// (real providers should be loaded separately for safety).
func (e *TestEnv) GetProvider() provider.Provider {
	if e.UseMock {
		return e.MockProvider
	}
	return nil
}

// Context returns a context with the configured timeout.
func (e *TestEnv) Context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), e.Timeout)
}

// RequireMock skips the test if not using mock provider.
func (e *TestEnv) RequireMock() {
	e.T.Helper()
	if !e.UseMock {
		e.T.Skip("Skipping: test requires mock provider")
	}
}

// RequireRealProvider skips the test if no real providers are configured.
func (e *TestEnv) RequireRealProvider(name string) provider.Provider {
	e.T.Helper()
	if e.UseMock {
		e.T.Skip("Skipping: test requires real provider")
	}
	p, ok := e.RealProviders[name]
	if !ok {
		e.T.Skipf("Skipping: provider %s not configured", name)
	}
	return p
}

// SetupMockError configures the mock to return an error for an operation.
func (e *TestEnv) SetupMockError(operation string, err error) {
	e.T.Helper()
	if e.MockProvider == nil {
		e.T.Fatal("SetupMockError called but mock provider is nil")
	}
	e.MockProvider.SetError(operation, err)
}

// SetupMockDelay configures the mock to add a delay for an operation.
func (e *TestEnv) SetupMockDelay(operation string, delay time.Duration) {
	e.T.Helper()
	if e.MockProvider == nil {
		e.T.Fatal("SetupMockDelay called but mock provider is nil")
	}
	e.MockProvider.SetDelay(operation, delay)
}

// ResetMock resets the mock provider to its default state.
func (e *TestEnv) ResetMock() {
	e.T.Helper()
	if e.MockProvider != nil {
		e.MockProvider.Reset()
		e.MockProvider.SetOffers(DefaultOffers())
	}
}

// Cleanup performs cleanup after tests.
func (e *TestEnv) Cleanup() {
	e.T.Helper()
	// Clear any test state
	if e.StateManager != nil {
		_ = e.StateManager.ClearState()
	}
}

// AssertNoError fails the test if err is not nil.
func (e *TestEnv) AssertNoError(err error, msg string) {
	e.T.Helper()
	if err != nil {
		e.T.Fatalf("%s: %v", msg, err)
	}
}

// AssertError fails the test if err is nil.
func (e *TestEnv) AssertError(err error, msg string) {
	e.T.Helper()
	if err == nil {
		e.T.Fatalf("%s: expected error but got nil", msg)
	}
}

// AssertEqual fails the test if got != want.
func (e *TestEnv) AssertEqual(got, want interface{}, msg string) {
	e.T.Helper()
	if got != want {
		e.T.Fatalf("%s: got %v, want %v", msg, got, want)
	}
}

// AssertTrue fails the test if condition is false.
func (e *TestEnv) AssertTrue(condition bool, msg string) {
	e.T.Helper()
	if !condition {
		e.T.Fatalf("%s: expected true", msg)
	}
}

// AssertFalse fails the test if condition is true.
func (e *TestEnv) AssertFalse(condition bool, msg string) {
	e.T.Helper()
	if condition {
		e.T.Fatalf("%s: expected false", msg)
	}
}

// Fixture represents test fixtures for integration tests.
type Fixture struct {
	// Name is the fixture name for identification.
	Name string

	// Offers are pre-configured provider offers.
	Offers []provider.Offer

	// Instances are pre-created instances.
	Instances []*provider.Instance

	// Config overrides for this fixture.
	Config *config.Config

	// StatePresets are state configurations to pre-load.
	StatePresets []*config.State
}

// StandardFixtures returns a map of standard test fixtures.
func StandardFixtures() map[string]*Fixture {
	spotPrice := 0.80
	highPrice := 5.00

	return map[string]*Fixture{
		"empty": {
			Name:   "empty",
			Offers: []provider.Offer{},
		},
		"single_offer": {
			Name: "single_offer",
			Offers: []provider.Offer{
				{
					OfferID:       "single-1",
					Provider:      "mock",
					GPU:           "A100 40GB",
					VRAM:          40,
					Region:        "EU-West",
					OnDemandPrice: 1.50,
					Available:     true,
				},
			},
		},
		"multi_region": {
			Name: "multi_region",
			Offers: []provider.Offer{
				{
					OfferID:       "eu-offer",
					Provider:      "mock",
					GPU:           "A100 40GB",
					VRAM:          40,
					Region:        "EU-West",
					OnDemandPrice: 1.50,
					Available:     true,
				},
				{
					OfferID:       "us-offer",
					Provider:      "mock",
					GPU:           "A100 40GB",
					VRAM:          40,
					Region:        "US-East",
					OnDemandPrice: 1.40,
					Available:     true,
				},
				{
					OfferID:       "asia-offer",
					Provider:      "mock",
					GPU:           "A100 40GB",
					VRAM:          40,
					Region:        "Asia-Pacific",
					OnDemandPrice: 1.60,
					Available:     true,
				},
			},
		},
		"spot_available": {
			Name: "spot_available",
			Offers: []provider.Offer{
				{
					OfferID:       "spot-offer",
					Provider:      "mock",
					GPU:           "A100 40GB",
					VRAM:          40,
					Region:        "EU-West",
					SpotPrice:     &spotPrice,
					OnDemandPrice: 1.50,
					Available:     true,
				},
			},
		},
		"high_vram": {
			Name: "high_vram",
			Offers: []provider.Offer{
				{
					OfferID:       "a100-80",
					Provider:      "mock",
					GPU:           "A100 80GB",
					VRAM:          80,
					Region:        "EU-West",
					OnDemandPrice: 2.50,
					Available:     true,
				},
				{
					OfferID:       "h100",
					Provider:      "mock",
					GPU:           "H100 80GB",
					VRAM:          80,
					Region:        "EU-West",
					OnDemandPrice: 4.00,
					Available:     true,
				},
			},
		},
		"expensive": {
			Name: "expensive",
			Offers: []provider.Offer{
				{
					OfferID:       "expensive-1",
					Provider:      "mock",
					GPU:           "H100 80GB",
					VRAM:          80,
					Region:        "EU-West",
					SpotPrice:     &highPrice,
					OnDemandPrice: 10.00,
					Available:     true,
				},
			},
		},
	}
}

// LoadFixture loads a named fixture into the test environment.
func (e *TestEnv) LoadFixture(name string) *Fixture {
	e.T.Helper()
	fixtures := StandardFixtures()
	f, ok := fixtures[name]
	if !ok {
		e.T.Fatalf("Unknown fixture: %s", name)
	}

	// Always set the offers (even if empty, to clear any existing offers)
	if e.MockProvider != nil {
		e.MockProvider.SetOffers(f.Offers)
	}

	if f.Config != nil {
		e.Config = f.Config
	}

	return f
}

// TestRun represents a single test run for table-driven tests.
type TestRun struct {
	// Name is the test name for t.Run.
	Name string

	// Setup is called before the test.
	Setup func(*TestEnv)

	// Run is the test function.
	Run func(*TestEnv)

	// Cleanup is called after the test.
	Cleanup func(*TestEnv)

	// Skip is the reason to skip this test (empty means don't skip).
	Skip string

	// ExpectError indicates whether an error is expected.
	ExpectError bool
}

// RunTests runs a slice of table-driven tests.
func RunTests(t *testing.T, tests []TestRun, opts ...Option) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Skip != "" {
				t.Skip(tt.Skip)
			}

			env := NewTestEnv(t, opts...)
			defer env.Cleanup()

			if tt.Setup != nil {
				tt.Setup(env)
			}

			if tt.Cleanup != nil {
				defer tt.Cleanup(env)
			}

			tt.Run(env)
		})
	}
}

// ProviderTestSuite runs a standard suite of provider tests.
// This is useful for testing real providers with the same tests as mock.
func ProviderTestSuite(t *testing.T, p provider.Provider) {
	t.Helper()

	t.Run("Name", func(t *testing.T) {
		name := p.Name()
		if name == "" {
			t.Error("Provider name is empty")
		}
	})

	t.Run("ConsoleURL", func(t *testing.T) {
		url := p.ConsoleURL()
		if url == "" {
			t.Error("Console URL is empty")
		}
	})

	t.Run("GetOffers_EmptyFilter", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		offers, err := p.GetOffers(ctx, provider.OfferFilter{})
		if err != nil {
			// Some providers may return errors for empty filter
			t.Logf("GetOffers with empty filter: %v", err)
			return
		}

		t.Logf("Found %d offers", len(offers))
		for i, o := range offers {
			if i >= 5 {
				t.Logf("... and %d more", len(offers)-5)
				break
			}
			t.Logf("  %s: %s %s @ %.2f/hr", o.OfferID, o.GPU, o.Region, o.OnDemandPrice)
		}
	})

	t.Run("GetOffers_MinVRAM", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		offers, err := p.GetOffers(ctx, provider.OfferFilter{MinVRAM: 40})
		if err != nil {
			t.Logf("GetOffers with MinVRAM filter: %v", err)
			return
		}

		for _, o := range offers {
			if o.VRAM < 40 {
				t.Errorf("Offer %s has VRAM %d, expected >= 40", o.OfferID, o.VRAM)
			}
		}
	})

	t.Run("SupportsBillingVerification", func(t *testing.T) {
		supported := p.SupportsBillingVerification()
		t.Logf("Billing verification supported: %v", supported)
	})
}

// SkipIfShort skips integration tests when running with -short flag.
func SkipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
}

// SkipIfNoRealProviders skips if INTEGRATION_USE_REAL_PROVIDERS is not set.
func SkipIfNoRealProviders(t *testing.T) {
	t.Helper()
	if os.Getenv("INTEGRATION_USE_REAL_PROVIDERS") != "true" {
		t.Skip("Skipping: INTEGRATION_USE_REAL_PROVIDERS not set")
	}
}
