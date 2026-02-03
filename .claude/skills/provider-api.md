# provider-api

Research and implement a cloud GPU provider API client.

## Trigger
User invokes `/provider-api [provider-name]` or asks to implement a provider.

## Arguments
- `[provider-name]` - One of: vast, lambda, runpod, coreweave, paperspace

## Provider Reference

### Vast.ai
- **API Docs**: https://vast.ai/docs/api/introduction
- **API Type**: REST
- **Base URL**: https://console.vast.ai/api/v0
- **Auth**: Bearer token in header
- **Key Endpoints**:
  - `GET /bundles/` - List available offers
  - `POST /asks/{id}/` - Create instance
  - `GET /instances/{id}/` - Get instance status
  - `DELETE /instances/{id}/` - Terminate instance
- **Spot Support**: Yes
- **Billing API**: Yes (check instance cost)

### Lambda Labs
- **API Docs**: https://cloud.lambdalabs.com/api/v1/docs
- **API Type**: REST
- **Base URL**: https://cloud.lambdalabs.com/api/v1
- **Auth**: Bearer token in header
- **Key Endpoints**:
  - `GET /instance-types` - List GPU types and prices
  - `POST /instance-operations/launch` - Create instance
  - `GET /instances/{id}` - Get instance status
  - `POST /instance-operations/terminate` - Terminate instance
- **Spot Support**: No (on-demand only)
- **Billing API**: Limited

### RunPod
- **API Docs**: https://docs.runpod.io/reference/runpod-apis
- **API Type**: GraphQL
- **Base URL**: https://api.runpod.io/graphql
- **Auth**: API key in header
- **Key Operations**:
  - `query gpuTypes` - List GPU types
  - `mutation podRentInterruptable` - Create spot pod
  - `mutation podStop` - Stop pod
  - `mutation podTerminate` - Terminate pod
- **Spot Support**: Yes ("interruptable" pods)
- **Billing API**: Yes

### CoreWeave
- **API Docs**: https://docs.coreweave.com/
- **API Type**: Kubernetes API
- **Auth**: kubeconfig / service account
- **Key Resources**:
  - VirtualServer CRD for GPU instances
  - Standard Kubernetes pod/deployment
- **Spot Support**: Yes (spot node pools)
- **Billing API**: Limited

### Paperspace
- **API Docs**: https://docs.paperspace.com/core/api-reference/
- **API Type**: REST
- **Base URL**: https://api.paperspace.io
- **Auth**: API key in header
- **Key Endpoints**:
  - `GET /machines/getAvailability` - Check availability
  - `POST /machines/create` - Create machine
  - `GET /machines/show` - Get machine status
  - `POST /machines/destroy` - Destroy machine
- **Spot Support**: No
- **Billing API**: **NO** (requires manual verification!)

## Instructions

1. **Research the API**:
   - Read official documentation
   - Identify authentication method
   - Map endpoints to Provider interface methods

2. **Create client structure**:
   ```go
   type [Provider]Client struct {
       baseURL    string
       apiKey     string
       httpClient *http.Client
       logger     *zerolog.Logger
   }
   ```

3. **Implement interface methods**:
   - `Name() string`
   - `GetOffers(ctx, filter) ([]Offer, error)`
   - `CreateInstance(ctx, req) (*Instance, error)`
   - `GetInstance(ctx, id) (*Instance, error)`
   - `TerminateInstance(ctx, id) error`
   - `SupportsBillingVerification() bool`
   - `GetBillingStatus(ctx, id) (BillingStatus, error)`
   - `ConsoleURL() string`

4. **Handle provider quirks**:
   - Rate limiting
   - Pagination
   - Error response formats
   - Cloud-init injection method

5. **Create tests with mocked HTTP**:
   - Test successful responses
   - Test error handling
   - Test rate limiting behavior

## Output

Creates `internal/provider/[provider]/client.go` with full Provider implementation.
