// Package runpod implements the RunPod provider for GPU instance management.
// RunPod uses a GraphQL API and supports both spot and on-demand instances.
package runpod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tmeurs/continueplz/internal/provider"
)

const (
	// graphqlURL is the RunPod GraphQL API endpoint.
	graphqlURL = "https://api.runpod.io/graphql"

	// defaultTimeout is the default HTTP timeout.
	defaultTimeout = 30 * time.Second

	// maxRetries is the maximum number of retry attempts.
	maxRetries = 5

	// baseRetryDelay is the base delay for exponential backoff.
	baseRetryDelay = 2 * time.Second

	// maxRetryDelay caps the exponential backoff.
	maxRetryDelay = 60 * time.Second

	// rateLimitDelay is the delay when rate limited.
	rateLimitDelay = 5 * time.Second

	// consoleURL is the RunPod web console URL.
	consoleURL = "https://www.runpod.io/console/pods"
)

// Client is the RunPod API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
	graphqlURL string

	// rateLimiter tracks rate limiting state.
	rateLimiter *rateLimiter
}

// rateLimiter implements simple rate limiting tracking.
type rateLimiter struct {
	mu          sync.Mutex
	lastRequest time.Time
	minInterval time.Duration
	retryAfter  time.Time
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithGraphQLURL sets a custom GraphQL URL (useful for testing).
func WithGraphQLURL(url string) ClientOption {
	return func(c *Client) {
		c.graphqlURL = url
	}
}

// NewClient creates a new RunPod API client.
func NewClient(apiKey string, opts ...ClientOption) (*Client, error) {
	if apiKey == "" {
		return nil, provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("API key is required"))
	}

	c := &Client{
		apiKey:     apiKey,
		graphqlURL: graphqlURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		rateLimiter: &rateLimiter{
			minInterval: 100 * time.Millisecond, // Basic rate limiting: 10 req/sec
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Name returns the provider name.
func (c *Client) Name() string {
	return "runpod"
}

// ConsoleURL returns the RunPod web console URL.
func (c *Client) ConsoleURL() string {
	return consoleURL
}

// SupportsBillingVerification returns true as RunPod has billing APIs.
func (c *Client) SupportsBillingVerification() bool {
	return true
}

// graphQLRequest represents a GraphQL request.
type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// graphQLResponse represents a generic GraphQL response.
type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors,omitempty"`
}

// graphQLError represents a GraphQL error.
type graphQLError struct {
	Message    string `json:"message"`
	Path       []interface{} `json:"path,omitempty"`
	Extensions struct {
		Code string `json:"code,omitempty"`
	} `json:"extensions,omitempty"`
}

// query executes a GraphQL query with retry logic.
func (c *Client) query(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	// Wait for rate limiter
	if err := c.waitForRateLimit(ctx); err != nil {
		return err
	}

	reqBody := graphQLRequest{
		Query:     query,
		Variables: variables,
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return provider.NewProviderError("context_cancelled", "request cancelled", ctx.Err())
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return provider.NewProviderError("request_encode_failed", "failed to encode request body", err)
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return provider.NewProviderError("request_create_failed", "failed to create request", err)
		}

		// Set headers
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = provider.NewProviderError("request_failed", "HTTP request failed", err)
			c.sleep(ctx, c.calculateBackoff(attempt))
			continue
		}

		// Process response
		processErr := c.processResponse(resp, result)
		if processErr == nil {
			return nil
		}

		// Check if we should retry
		if shouldRetry(processErr, resp.StatusCode) {
			lastErr = processErr

			// Handle rate limiting
			if resp.StatusCode == http.StatusTooManyRequests {
				c.handleRateLimitResponse(resp)
				c.sleep(ctx, rateLimitDelay)
			} else {
				c.sleep(ctx, c.calculateBackoff(attempt))
			}
			continue
		}

		return processErr
	}

	return lastErr
}

// processResponse processes the HTTP response and decodes the result.
func (c *Client) processResponse(resp *http.Response, result interface{}) error {
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return provider.NewProviderError("response_read_failed", "failed to read response body", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return c.parseHTTPError(resp.StatusCode, bodyBytes)
	}

	// Parse GraphQL response
	var gqlResp graphQLResponse
	if err := json.Unmarshal(bodyBytes, &gqlResp); err != nil {
		return provider.NewProviderError("response_decode_failed", "failed to decode GraphQL response", err)
	}

	// Check for GraphQL errors
	if len(gqlResp.Errors) > 0 {
		return c.parseGraphQLErrors(gqlResp.Errors)
	}

	// Decode the data field into the result
	if result != nil && len(gqlResp.Data) > 0 {
		if err := json.Unmarshal(gqlResp.Data, result); err != nil {
			return provider.NewProviderError("response_decode_failed", "failed to decode response data", err)
		}
	}

	return nil
}

// parseHTTPError parses an HTTP error response.
func (c *Client) parseHTTPError(statusCode int, body []byte) error {
	// Try to parse as GraphQL error first
	var gqlResp graphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err == nil && len(gqlResp.Errors) > 0 {
		return c.parseGraphQLErrors(gqlResp.Errors)
	}

	return c.statusCodeToError(statusCode, string(body))
}

// parseGraphQLErrors converts GraphQL errors to a provider error.
func (c *Client) parseGraphQLErrors(errors []graphQLError) error {
	if len(errors) == 0 {
		return nil
	}

	errMsg := errors[0].Message
	errCode := errors[0].Extensions.Code

	// Map common error codes
	switch errCode {
	case "UNAUTHENTICATED", "FORBIDDEN":
		return provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("%s", errMsg))
	case "NOT_FOUND":
		return provider.ErrInstanceNotFound.Wrap(fmt.Errorf("%s", errMsg))
	case "RATE_LIMITED":
		return provider.ErrRateLimited.Wrap(fmt.Errorf("%s", errMsg))
	}

	// Check message for common patterns
	lowerMsg := strings.ToLower(errMsg)
	if strings.Contains(lowerMsg, "not found") || strings.Contains(lowerMsg, "does not exist") {
		return provider.ErrInstanceNotFound.Wrap(fmt.Errorf("%s", errMsg))
	}
	if strings.Contains(lowerMsg, "unauthorized") || strings.Contains(lowerMsg, "authentication") {
		return provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("%s", errMsg))
	}
	if strings.Contains(lowerMsg, "capacity") || strings.Contains(lowerMsg, "available") {
		return provider.ErrInsufficientCapacity.Wrap(fmt.Errorf("%s", errMsg))
	}

	return provider.NewProviderError("graphql_error", errMsg, nil)
}

// statusCodeToError converts an HTTP status code to a provider error.
func (c *Client) statusCodeToError(statusCode int, message string) error {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("%s", message))
	case http.StatusNotFound:
		return provider.ErrInstanceNotFound.Wrap(fmt.Errorf("%s", message))
	case http.StatusTooManyRequests:
		return provider.ErrRateLimited.Wrap(fmt.Errorf("%s", message))
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return provider.NewProviderError("service_unavailable", "RunPod service temporarily unavailable", fmt.Errorf("%s", message))
	default:
		return provider.NewProviderError("api_error", fmt.Sprintf("API error (HTTP %d): %s", statusCode, message), nil)
	}
}

// shouldRetry determines if a request should be retried.
func shouldRetry(err error, statusCode int) bool {
	// Retry on rate limiting
	if statusCode == http.StatusTooManyRequests {
		return true
	}

	// Retry on server errors
	if statusCode >= 500 {
		return true
	}

	// Retry on network errors (check if it's a temporary error)
	if provErr, ok := err.(*provider.ProviderError); ok {
		switch provErr.Code {
		case "request_failed", "service_unavailable":
			return true
		}
	}

	return false
}

// calculateBackoff calculates the backoff delay for a given attempt.
func (c *Client) calculateBackoff(attempt int) time.Duration {
	delay := time.Duration(float64(baseRetryDelay) * math.Pow(2, float64(attempt-1)))
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	return delay
}

// sleep waits for the given duration, respecting context cancellation.
func (c *Client) sleep(ctx context.Context, duration time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
}

// waitForRateLimit waits if necessary to respect rate limits.
func (c *Client) waitForRateLimit(ctx context.Context) error {
	c.rateLimiter.mu.Lock()

	// Check if we need to wait for a retry-after
	if !c.rateLimiter.retryAfter.IsZero() && time.Now().Before(c.rateLimiter.retryAfter) {
		waitDuration := time.Until(c.rateLimiter.retryAfter)
		c.rateLimiter.mu.Unlock()

		select {
		case <-ctx.Done():
			return provider.NewProviderError("context_cancelled", "request cancelled while waiting for rate limit", ctx.Err())
		case <-time.After(waitDuration):
		}
		return nil
	}

	// Ensure minimum interval between requests
	if !c.rateLimiter.lastRequest.IsZero() {
		elapsed := time.Since(c.rateLimiter.lastRequest)
		if elapsed < c.rateLimiter.minInterval {
			waitDuration := c.rateLimiter.minInterval - elapsed
			c.rateLimiter.mu.Unlock()

			select {
			case <-ctx.Done():
				return provider.NewProviderError("context_cancelled", "request cancelled while waiting", ctx.Err())
			case <-time.After(waitDuration):
			}

			c.rateLimiter.mu.Lock()
		}
	}

	c.rateLimiter.lastRequest = time.Now()
	c.rateLimiter.mu.Unlock()
	return nil
}

// handleRateLimitResponse updates rate limiter state from a rate limit response.
func (c *Client) handleRateLimitResponse(resp *http.Response) {
	c.rateLimiter.mu.Lock()
	defer c.rateLimiter.mu.Unlock()

	// Check for Retry-After header
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter != "" {
		// Try to parse as seconds
		if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
			c.rateLimiter.retryAfter = time.Now().Add(seconds)
			return
		}
		// Try to parse as HTTP date
		if t, err := http.ParseTime(retryAfter); err == nil {
			c.rateLimiter.retryAfter = t
			return
		}
	}

	// Default to rate limit delay
	c.rateLimiter.retryAfter = time.Now().Add(rateLimitDelay)
}

// GraphQL queries for RunPod API

const queryGpuTypes = `
query GpuTypes {
  gpuTypes {
    id
    displayName
    memoryInGb
    secureCloud
    communityCloud
    lowestPrice(input: { gpuCount: 1 }) {
      minimumBidPrice
      uninterruptablePrice
    }
  }
}
`

const queryAvailableGpus = `
query AvailableGpus($input: GpuLowestPriceInput) {
  gpuTypes {
    id
    displayName
    memoryInGb
    secureCloud
    communityCloud
    lowestPrice(input: $input) {
      minimumBidPrice
      uninterruptablePrice
      stockStatus
      countAvailable
    }
  }
}
`

const mutationCreatePod = `
mutation CreatePod($input: PodFindAndDeployOnDemandInput!) {
  podFindAndDeployOnDemand(input: $input) {
    id
    name
    desiredStatus
    machineId
    machine {
      gpuDisplayName
      location
    }
  }
}
`

const mutationCreateSpotPod = `
mutation CreateSpotPod($input: PodRentInterruptableInput!) {
  podRentInterruptable(input: $input) {
    id
    name
    desiredStatus
    machineId
    machine {
      gpuDisplayName
      location
    }
  }
}
`

const queryPod = `
query Pod($input: PodFilter!) {
  pod(input: $input) {
    id
    name
    desiredStatus
    imageName
    machineId
    machine {
      gpuDisplayName
      location
    }
    runtime {
      uptimeInSeconds
      ports {
        ip
        isIpPublic
        privatePort
        publicPort
      }
      gpus {
        id
        gpuUtilPercent
        memoryUtilPercent
      }
    }
    costPerHr
    gpuCount
    volumeInGb
  }
}
`

const queryMyPods = `
query MyPods {
  myself {
    pods {
      id
      name
      desiredStatus
      imageName
      machineId
      machine {
        gpuDisplayName
        location
      }
      runtime {
        uptimeInSeconds
        ports {
          ip
          isIpPublic
          privatePort
          publicPort
        }
      }
      costPerHr
      gpuCount
      volumeInGb
    }
  }
}
`

const mutationTerminatePod = `
mutation TerminatePod($input: PodTerminateInput!) {
  podTerminate(input: $input)
}
`

// RunPod API response types

type gpuTypesResponse struct {
	GpuTypes []gpuType `json:"gpuTypes"`
}

type gpuType struct {
	ID             string       `json:"id"`
	DisplayName    string       `json:"displayName"`
	MemoryInGb     int          `json:"memoryInGb"`
	SecureCloud    bool         `json:"secureCloud"`
	CommunityCloud bool         `json:"communityCloud"`
	LowestPrice    *lowestPrice `json:"lowestPrice"`
}

type lowestPrice struct {
	MinimumBidPrice      float64 `json:"minimumBidPrice"`
	UninterruptablePrice float64 `json:"uninterruptablePrice"`
	StockStatus          string  `json:"stockStatus"`
	CountAvailable       int     `json:"countAvailable"`
}

type podResponse struct {
	Pod *runpodPod `json:"pod"`
}

type myPodsResponse struct {
	Myself struct {
		Pods []runpodPod `json:"pods"`
	} `json:"myself"`
}

type runpodPod struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	DesiredStatus string  `json:"desiredStatus"`
	ImageName     string  `json:"imageName"`
	MachineID     string  `json:"machineId"`
	Machine       *machine `json:"machine"`
	Runtime       *runtime `json:"runtime"`
	CostPerHr     float64 `json:"costPerHr"`
	GPUCount      int     `json:"gpuCount"`
	VolumeInGb    int     `json:"volumeInGb"`
}

type machine struct {
	GpuDisplayName string `json:"gpuDisplayName"`
	Location       string `json:"location"`
}

type runtime struct {
	UptimeInSeconds int    `json:"uptimeInSeconds"`
	Ports           []port `json:"ports"`
	Gpus            []gpu  `json:"gpus"`
}

type port struct {
	IP          string `json:"ip"`
	IsIPPublic  bool   `json:"isIpPublic"`
	PrivatePort int    `json:"privatePort"`
	PublicPort  int    `json:"publicPort"`
}

type gpu struct {
	ID               string `json:"id"`
	GpuUtilPercent   int    `json:"gpuUtilPercent"`
	MemoryUtilPercent int    `json:"memoryUtilPercent"`
}

type createPodResponse struct {
	PodFindAndDeployOnDemand *runpodPod `json:"podFindAndDeployOnDemand"`
}

type createSpotPodResponse struct {
	PodRentInterruptable *runpodPod `json:"podRentInterruptable"`
}

type terminatePodResponse struct {
	PodTerminate string `json:"podTerminate"`
}

// GetOffers returns available GPU offers matching the filter criteria.
func (c *Client) GetOffers(ctx context.Context, filter provider.OfferFilter) ([]provider.Offer, error) {
	// Query GPU types with pricing
	var resp gpuTypesResponse
	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"gpuCount": 1,
		},
	}

	if err := c.query(ctx, queryAvailableGpus, variables, &resp); err != nil {
		return nil, err
	}

	// Convert RunPod GPU types to standard Offer type
	offers := make([]provider.Offer, 0)
	for _, gpuType := range resp.GpuTypes {
		// Skip if no pricing available
		if gpuType.LowestPrice == nil {
			continue
		}

		// Skip if not available
		if gpuType.LowestPrice.StockStatus == "unavailable" || gpuType.LowestPrice.CountAvailable == 0 {
			continue
		}

		// Normalize GPU name
		gpu, vram := normalizeGPUFromRunPod(gpuType.DisplayName, gpuType.MemoryInGb)

		// Apply GPU type filter
		if filter.GPUType != "" {
			normalizedFilter := normalizeGPUName(filter.GPUType)
			normalizedGPU := normalizeGPUName(gpu)
			if normalizedFilter != normalizedGPU {
				continue
			}
		}

		// Apply VRAM filter
		if filter.MinVRAM > 0 && vram < filter.MinVRAM {
			continue
		}

		// Get prices
		onDemandPrice := gpuType.LowestPrice.UninterruptablePrice
		spotPrice := gpuType.LowestPrice.MinimumBidPrice

		// Apply max price filter
		if filter.MaxHourlyPrice > 0 {
			if filter.SpotOnly && spotPrice > filter.MaxHourlyPrice {
				continue
			}
			if !filter.SpotOnly && onDemandPrice > filter.MaxHourlyPrice {
				continue
			}
		}

		// Apply spot filter
		if filter.SpotOnly && spotPrice == 0 {
			continue
		}

		// Determine region from cloud type
		region := "Global"
		if gpuType.SecureCloud {
			region = "Secure Cloud"
		} else if gpuType.CommunityCloud {
			region = "Community Cloud"
		}

		// Apply region filter (RunPod doesn't have traditional regions)
		if filter.Region != "" && !regionMatches(region, filter.Region) {
			continue
		}

		// Create offer
		offer := provider.Offer{
			OfferID:       gpuType.ID,
			Provider:      "runpod",
			GPU:           gpu,
			VRAM:          vram,
			Region:        region,
			OnDemandPrice: onDemandPrice,
			StoragePrice:  0,   // Storage is typically included or billed separately
			EgressPrice:   0,   // Egress pricing varies
			Available:     true,
		}

		// Set spot price if available
		if spotPrice > 0 {
			offer.SpotPrice = &spotPrice
		}

		offers = append(offers, offer)
	}

	return offers, nil
}

// normalizeGPUFromRunPod converts RunPod GPU name to standardized format.
func normalizeGPUFromRunPod(displayName string, memoryGb int) (string, int) {
	lower := strings.ToLower(displayName)

	switch {
	case strings.Contains(lower, "a100") && memoryGb >= 80:
		return "A100 80GB", 80
	case strings.Contains(lower, "a100") && memoryGb >= 40:
		return "A100 40GB", 40
	case strings.Contains(lower, "a100"):
		// Default A100 based on memory
		if memoryGb >= 80 {
			return "A100 80GB", 80
		}
		return "A100 40GB", 40
	case strings.Contains(lower, "h100"):
		return "H100 80GB", 80
	case strings.Contains(lower, "a6000"):
		return "A6000 48GB", 48
	case strings.Contains(lower, "rtx 4090"):
		return "RTX 4090", 24
	case strings.Contains(lower, "rtx 3090"):
		return "RTX 3090", 24
	case strings.Contains(lower, "rtx 3080"):
		return "RTX 3080", 12
	default:
		// Return as-is with memory
		if memoryGb > 0 {
			return fmt.Sprintf("%s %dGB", displayName, memoryGb), memoryGb
		}
		return displayName, memoryGb
	}
}

// normalizeGPUName normalizes GPU names for comparison.
func normalizeGPUName(name string) string {
	lower := strings.ToLower(name)
	// Remove common separators and normalize
	switch {
	case strings.Contains(lower, "a100") && (strings.Contains(lower, "80") || strings.Contains(lower, "sxm")):
		return "a100-80gb"
	case strings.Contains(lower, "a100"):
		return "a100-40gb"
	case strings.Contains(lower, "h100"):
		return "h100-80gb"
	case strings.Contains(lower, "a6000"):
		return "a6000"
	default:
		return strings.ReplaceAll(lower, " ", "-")
	}
}

// regionMatches checks if a region matches a filter (with some flexibility).
// RunPod uses "Secure Cloud" and "Community Cloud" rather than geographic regions.
func regionMatches(region, filter string) bool {
	// Exact match
	if strings.EqualFold(region, filter) {
		return true
	}

	filterLower := strings.ToLower(filter)
	regionLower := strings.ToLower(region)

	// Match secure/community cloud types
	if strings.Contains(filterLower, "secure") && strings.Contains(regionLower, "secure") {
		return true
	}
	if strings.Contains(filterLower, "community") && strings.Contains(regionLower, "community") {
		return true
	}

	// Global matches everything
	if regionLower == "global" {
		return true
	}

	return false
}

// CreateInstance creates a new GPU instance with the given configuration.
func (c *Client) CreateInstance(ctx context.Context, req provider.CreateRequest) (*provider.Instance, error) {
	if req.OfferID == "" {
		return nil, provider.NewProviderError("invalid_request", "offer ID is required", nil)
	}

	// Build the input for pod creation
	input := map[string]interface{}{
		"gpuTypeId":     req.OfferID,
		"gpuCount":      1,
		"volumeInGb":    req.DiskSizeGB,
		"containerDiskInGb": 20,
		"imageName":     "runpod/pytorch:latest", // Base image with CUDA support
		"name":          "continueplz",
	}

	// Set default disk size if not specified
	if req.DiskSizeGB == 0 {
		input["volumeInGb"] = 100
	}

	// Add cloud-init/startup command if provided
	if req.CloudInit != "" {
		// RunPod uses docker commands, so we wrap the cloud-init in a startup script
		input["dockerArgs"] = fmt.Sprintf("bash -c '%s'", escapeShellArg(req.CloudInit))
	}

	// Add SSH public key via environment variable if provided
	if req.SSHPublicKey != "" {
		input["env"] = []map[string]string{
			{"key": "PUBLIC_KEY", "value": req.SSHPublicKey},
		}
	}

	var pod *runpodPod
	var err error

	if req.Spot {
		// Create spot (interruptable) pod
		input["bidPerGpu"] = 0.0 // Use minimum bid price

		var resp createSpotPodResponse
		variables := map[string]interface{}{"input": input}
		if err = c.query(ctx, mutationCreateSpotPod, variables, &resp); err != nil {
			return nil, c.handleCreateError(err)
		}
		pod = resp.PodRentInterruptable
	} else {
		// Create on-demand pod
		var resp createPodResponse
		variables := map[string]interface{}{"input": input}
		if err = c.query(ctx, mutationCreatePod, variables, &resp); err != nil {
			return nil, c.handleCreateError(err)
		}
		pod = resp.PodFindAndDeployOnDemand
	}

	if pod == nil {
		return nil, provider.NewProviderError("create_failed", "no pod returned", nil)
	}

	// Convert to standard Instance type
	instance := convertRunPodInstance(*pod, req.Spot)
	return &instance, nil
}

// handleCreateError processes errors from create operations.
func (c *Client) handleCreateError(err error) error {
	if provErr, ok := err.(*provider.ProviderError); ok {
		lowerMsg := strings.ToLower(provErr.Message)
		if strings.Contains(lowerMsg, "not found") || strings.Contains(lowerMsg, "invalid gpu type") {
			return provider.ErrOfferNotFound.Wrap(fmt.Errorf("GPU type not available: %s", provErr.Message))
		}
		if strings.Contains(lowerMsg, "capacity") || strings.Contains(lowerMsg, "available") || strings.Contains(lowerMsg, "stock") {
			return provider.ErrInsufficientCapacity.Wrap(fmt.Errorf("no capacity available: %s", provErr.Message))
		}
	}
	return err
}

// escapeShellArg escapes a string for use in a shell command.
func escapeShellArg(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}

// GetInstance retrieves the current status of an instance by ID.
func (c *Client) GetInstance(ctx context.Context, id string) (*provider.Instance, error) {
	if id == "" {
		return nil, provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	var resp podResponse
	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"podId": id,
		},
	}

	if err := c.query(ctx, queryPod, variables, &resp); err != nil {
		return nil, err
	}

	if resp.Pod == nil {
		return nil, provider.ErrInstanceNotFound.Wrap(fmt.Errorf("pod %s not found", id))
	}

	// Determine if this is a spot instance by checking the desired status
	// RunPod uses "RUNNING" for on-demand and "RUNNING" for spot, so we can't easily tell
	// We'll default to false and let the caller track this
	instance := convertRunPodInstance(*resp.Pod, false)
	return &instance, nil
}

// convertRunPodInstance converts a RunPod pod to the standard Instance type.
func convertRunPodInstance(pod runpodPod, spot bool) provider.Instance {
	instance := provider.Instance{
		ID:         pod.ID,
		Provider:   "runpod",
		Status:     mapRunPodStatus(pod.DesiredStatus),
		Spot:       spot,
		HourlyRate: pod.CostPerHr,
	}

	// Extract GPU info
	if pod.Machine != nil {
		instance.GPU, _ = normalizeGPUFromRunPod(pod.Machine.GpuDisplayName, 0)
		instance.Region = normalizeRunPodRegion(pod.Machine.Location)
	}

	// Extract public IP from runtime ports
	if pod.Runtime != nil {
		for _, p := range pod.Runtime.Ports {
			if p.IsIPPublic && p.IP != "" {
				instance.PublicIP = p.IP
				break
			}
		}

		// Calculate created time from uptime
		if pod.Runtime.UptimeInSeconds > 0 {
			instance.CreatedAt = time.Now().Add(-time.Duration(pod.Runtime.UptimeInSeconds) * time.Second)
		}
	}

	return instance
}

// mapRunPodStatus maps RunPod status strings to our InstanceStatus type.
func mapRunPodStatus(desiredStatus string) provider.InstanceStatus {
	switch strings.ToUpper(desiredStatus) {
	case "RUNNING":
		return provider.InstanceStatusRunning
	case "EXITED", "TERMINATED", "DEAD":
		return provider.InstanceStatusTerminated
	case "CREATED", "STARTING", "PENDING":
		return provider.InstanceStatusCreating
	case "STOPPING":
		return provider.InstanceStatusStopping
	case "ERROR", "FAILED":
		return provider.InstanceStatusError
	default:
		if desiredStatus == "" {
			return provider.InstanceStatusCreating
		}
		return provider.InstanceStatusError
	}
}

// normalizeRunPodRegion normalizes RunPod location to a region string.
func normalizeRunPodRegion(location string) string {
	if location == "" {
		return "Unknown"
	}

	lower := strings.ToLower(location)

	// Common location patterns
	switch {
	case strings.Contains(lower, "us") || strings.Contains(lower, "america"):
		if strings.Contains(lower, "east") {
			return "US-East"
		}
		if strings.Contains(lower, "west") {
			return "US-West"
		}
		return "US"
	case strings.Contains(lower, "eu") || strings.Contains(lower, "europe"):
		return "EU"
	case strings.Contains(lower, "asia"):
		return "AP"
	case strings.Contains(lower, "canada") || strings.Contains(lower, "ca"):
		return "CA"
	default:
		return location
	}
}

// TerminateInstance terminates an instance by ID.
// This is idempotent - terminating an already-terminated instance does not error.
func (c *Client) TerminateInstance(ctx context.Context, id string) error {
	if id == "" {
		return provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	var resp terminatePodResponse
	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"podId": id,
		},
	}

	if err := c.query(ctx, mutationTerminatePod, variables, &resp); err != nil {
		// Check if the error is "not found" - treat as success (idempotent)
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" || strings.Contains(strings.ToLower(provErr.Message), "not found") {
				// Pod already terminated or doesn't exist - that's fine
				return nil
			}
		}
		return err
	}

	return nil
}

// GetBillingStatus returns the billing status for an instance.
// For RunPod, billing is determined by the pod's status:
// - If pod is "RUNNING" → BillingActive
// - If pod is "EXITED", "TERMINATED", or not found → BillingStopped
// - If status cannot be determined → BillingUnknown
func (c *Client) GetBillingStatus(ctx context.Context, id string) (provider.BillingStatus, error) {
	if id == "" {
		return provider.BillingUnknown, provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	// Get pod details to check status
	instance, err := c.GetInstance(ctx, id)
	if err != nil {
		// Check if pod not found - that means billing has stopped
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" || strings.Contains(strings.ToLower(provErr.Message), "not found") {
				return provider.BillingStopped, nil
			}
		}
		// For other errors, we can't determine billing status
		return provider.BillingUnknown, err
	}

	// Map instance status to billing status
	switch instance.Status {
	case provider.InstanceStatusRunning, provider.InstanceStatusCreating:
		// Pod is active, billing is active
		return provider.BillingActive, nil
	case provider.InstanceStatusStopping:
		// Pod is stopping but may still be billed until fully stopped
		// Return active to be conservative
		return provider.BillingActive, nil
	case provider.InstanceStatusTerminated:
		// Pod is terminated, billing has stopped
		return provider.BillingStopped, nil
	case provider.InstanceStatusError:
		// Error state - billing status depends on whether the pod is actually running
		// Return unknown as we can't be sure
		return provider.BillingUnknown, nil
	default:
		return provider.BillingUnknown, nil
	}
}

// GraphQL query to get user info (myself).
const queryMyself = `
query {
	myself {
		id
		email
		currentSpendPerHr
		machineQuota
		referralEarned
		signedTermsOfService
		serverBalance
		creditAlertThreshold
		notifyPodsStale
		notifyPodsGeneral
	}
}
`

// myselfResponse represents the response from the myself query.
type myselfResponse struct {
	Myself *struct {
		ID                   string  `json:"id"`
		Email                string  `json:"email"`
		CurrentSpendPerHr    float64 `json:"currentSpendPerHr"`
		MachineQuota         int     `json:"machineQuota"`
		ReferralEarned       float64 `json:"referralEarned"`
		SignedTermsOfService bool    `json:"signedTermsOfService"`
		ServerBalance        float64 `json:"serverBalance"`
		CreditAlertThreshold float64 `json:"creditAlertThreshold"`
		NotifyPodsStale      bool    `json:"notifyPodsStale"`
		NotifyPodsGeneral    bool    `json:"notifyPodsGeneral"`
	} `json:"myself"`
}

// ValidateAPIKey validates the API key and returns account information.
func (c *Client) ValidateAPIKey(ctx context.Context) (*provider.AccountInfo, error) {
	// RunPod uses GraphQL query "myself" to get current user info
	var resp myselfResponse
	if err := c.query(ctx, queryMyself, nil, &resp); err != nil {
		// Check for authentication errors
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "authentication_failed" {
				return &provider.AccountInfo{Valid: false}, provider.ErrAuthenticationFailed.Wrap(err)
			}
		}
		return nil, err
	}

	if resp.Myself == nil {
		return &provider.AccountInfo{Valid: false}, provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("unable to fetch user info"))
	}

	balance := resp.Myself.ServerBalance

	return &provider.AccountInfo{
		Email:           resp.Myself.Email,
		Username:        "", // RunPod doesn't have usernames
		Balance:         &balance,
		BalanceCurrency: "USD",
		AccountID:       resp.Myself.ID,
		Valid:           true,
	}, nil
}
