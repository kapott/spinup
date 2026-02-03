# Integration Tests

This directory contains the integration test framework and tests for continueplz.

## Overview

Integration tests validate the full flow of operations across multiple components.
They can run in two modes:

1. **Mock Mode** (default): Uses the mock provider for isolated, fast, reliable testing
2. **Real Provider Mode**: Tests against actual cloud GPU providers (requires API keys)

## Running Integration Tests

### Run with Mock Provider (Default)

```bash
# Run all integration tests
go test ./tests/integration/...

# Run with verbose output
go test -v ./tests/integration/...

# Run a specific test
go test -v ./tests/integration/... -run TestName
```

### Run with Real Providers

To test against real cloud providers, set `INTEGRATION_USE_REAL_PROVIDERS=true` and provide API keys:

```bash
# Set environment variables
export INTEGRATION_USE_REAL_PROVIDERS=true
export VAST_API_KEY=your-api-key
export LAMBDA_API_KEY=your-api-key
# ... etc

# Run tests
go test -v ./tests/integration/...
```

Or using a `.env` file:

```bash
# Create a test .env file
cp .example.env .env.test
# Edit .env.test with your API keys

# Source and run
source .env.test
INTEGRATION_USE_REAL_PROVIDERS=true go test -v ./tests/integration/...
```

## Test Framework

### TestEnv

The `TestEnv` struct provides a configured test environment:

```go
func TestMyFeature(t *testing.T) {
    env := integration.NewTestEnv(t)
    defer env.Cleanup()

    ctx, cancel := env.Context()
    defer cancel()

    provider := env.GetProvider()
    // ... run test
}
```

### Options

Configure the test environment with functional options:

```go
env := integration.NewTestEnv(t,
    integration.WithTimeout(60*time.Second),
    integration.WithMockOffers(customOffers),
    integration.WithRealProviders(),
)
```

### Fixtures

Pre-defined test fixtures for common scenarios:

```go
env.LoadFixture("single_offer")   // One basic offer
env.LoadFixture("multi_region")   // Offers in multiple regions
env.LoadFixture("spot_available") // Offers with spot pricing
env.LoadFixture("high_vram")      // 80GB+ VRAM offers
env.LoadFixture("expensive")      // High-priced offers
env.LoadFixture("empty")          // No offers
```

### Table-Driven Tests

Use `RunTests` for table-driven testing:

```go
tests := []integration.TestRun{
    {
        Name: "basic_test",
        Setup: func(env *integration.TestEnv) {
            env.LoadFixture("single_offer")
        },
        Run: func(env *integration.TestEnv) {
            // Test logic
        },
    },
}
integration.RunTests(t, tests)
```

### Error Injection

For testing error handling:

```go
env.SetupMockError("CreateInstance", provider.ErrInsufficientCapacity)
env.SetupMockDelay("GetOffers", 5*time.Second)
```

### Provider Test Suite

Run standard provider tests:

```go
func TestVastProvider(t *testing.T) {
    integration.SkipIfNoRealProviders(t)

    cfg, _ := config.LoadConfig("")
    p, _ := vast.NewClient(cfg.VastAPIKey)

    integration.ProviderTestSuite(t, p)
}
```

## Test Files

- `framework.go` - Test framework and helpers
- `framework_test.go` - Tests for the framework itself
- `deploy_test.go` - Deployment flow integration tests (future)
- `stop_test.go` - Stop/cleanup integration tests (future)

## Writing New Tests

1. Use `NewTestEnv` to create a test environment
2. Always call `defer env.Cleanup()`
3. Use `env.Context()` for operations that need timeouts
4. Use fixtures for common scenarios
5. Use `env.RequireMock()` or `env.RequireRealProvider()` for mode-specific tests
6. Use assertion helpers: `env.AssertNoError()`, `env.AssertEqual()`, etc.

Example:

```go
func TestDeployFlow(t *testing.T) {
    integration.SkipIfShort(t)

    env := integration.NewTestEnv(t)
    defer env.Cleanup()

    env.LoadFixture("spot_available")
    ctx, cancel := env.Context()
    defer cancel()

    p := env.GetProvider()
    env.AssertTrue(p != nil, "Provider should not be nil")

    offers, err := p.GetOffers(ctx, provider.OfferFilter{MinVRAM: 40})
    env.AssertNoError(err, "GetOffers should succeed")
    env.AssertTrue(len(offers) > 0, "Should have offers")
}
```

## CI/CD Integration

Integration tests run in mock mode by default, suitable for CI:

```yaml
# GitHub Actions example
- name: Run Integration Tests
  run: go test -v ./tests/integration/...
```

For periodic real-provider testing:

```yaml
- name: Run Real Provider Tests
  env:
    INTEGRATION_USE_REAL_PROVIDERS: true
    VAST_API_KEY: ${{ secrets.VAST_API_KEY }}
  run: go test -v ./tests/integration/... -timeout 5m
```

## Safety Notes

- Real provider tests may incur costs
- Always clean up resources in test teardown
- Use short timeouts to prevent runaway instances
- Never commit API keys to the repository
