# api-mock

Generate mock HTTP responses and mock implementations for provider APIs.

## Trigger
User invokes `/api-mock [provider]` or asks to create mocks for an API.

## Arguments
- `[provider]` - Provider name: vast, lambda, runpod, coreweave, paperspace, or "all"

## Instructions

1. **Identify the provider interface**:
   - Read `internal/provider/provider.go` for the interface definition
   - Identify all methods that need mocking

2. **Create mock struct**:
   ```go
   type MockProvider struct {
       // Configurable responses
       OffersResponse    []Offer
       OffersError       error
       CreateResponse    *Instance
       CreateError       error
       GetResponse       *Instance
       GetError          error
       TerminateError    error
       BillingStatus     BillingStatus
       BillingError      error

       // Call tracking
       CreateCalls       []CreateRequest
       TerminateCalls    []string
   }
   ```

3. **Implement all interface methods**:
   - Return configured responses
   - Track calls for verification
   - Support error injection

4. **Create fixture data**:
   - Sample offers with realistic pricing
   - Sample instance responses
   - Error scenarios

5. **Create helper functions**:
   ```go
   func NewMockProvider() *MockProvider
   func NewMockProviderWithOffers(offers []Offer) *MockProvider
   func NewMockProviderThatFails(err error) *MockProvider
   ```

6. **Generate test fixtures file**:
   - JSON fixtures for HTTP response mocking
   - Located in `testdata/` directory

## Output

Creates/updates:
- `internal/provider/mock/mock.go` - Mock implementation
- `internal/provider/mock/fixtures.go` - Test fixtures
- `testdata/[provider]/` - JSON response fixtures
