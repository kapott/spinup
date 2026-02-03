// Package lambda implements the Lambda Labs provider for GPU instance management.
package lambda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/tmeurs/continueplz/internal/provider"
)

const (
	// baseURL is the Lambda Labs API base URL.
	baseURL = "https://cloud.lambdalabs.com/api/v1"

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

	// consoleURL is the Lambda Labs web console URL.
	consoleURL = "https://cloud.lambdalabs.com/"
)

// Client is the Lambda Labs API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string

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

// WithBaseURL sets a custom base URL (useful for testing).
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// NewClient creates a new Lambda Labs API client.
func NewClient(apiKey string, opts ...ClientOption) (*Client, error) {
	if apiKey == "" {
		return nil, provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("API key is required"))
	}

	c := &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
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
	return "lambda"
}

// ConsoleURL returns the Lambda Labs web console URL.
func (c *Client) ConsoleURL() string {
	return consoleURL
}

// SupportsBillingVerification returns true as Lambda Labs has billing APIs.
func (c *Client) SupportsBillingVerification() bool {
	return true
}

// apiError represents an error response from the Lambda Labs API.
type apiError struct {
	Error struct {
		Code       string `json:"code"`
		Message    string `json:"message"`
		Suggestion string `json:"suggestion,omitempty"`
	} `json:"error"`
}

// request makes an HTTP request to the Lambda Labs API with retry logic.
func (c *Client) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	// Wait for rate limiter
	if err := c.waitForRateLimit(ctx); err != nil {
		return err
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return provider.NewProviderError("request_encode_failed", "failed to encode request body", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	url := c.baseURL + path

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return provider.NewProviderError("context_cancelled", "request cancelled", ctx.Err())
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return provider.NewProviderError("request_create_failed", "failed to create request", err)
		}

		// Set headers - Lambda Labs uses Basic Auth with API key as username
		req.Header.Set("Accept", "application/json")
		req.SetBasicAuth(c.apiKey, "")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
			// Reset body reader for retry
			if bodyBytes, ok := body.([]byte); ok {
				bodyReader = bytes.NewReader(bodyBytes)
			} else {
				bodyBytes, _ := json.Marshal(body)
				bodyReader = bytes.NewReader(bodyBytes)
			}
			req.Body = io.NopCloser(bodyReader)
		}

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
		return c.parseAPIError(resp.StatusCode, bodyBytes)
	}

	// Decode successful response
	if result != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, result); err != nil {
			return provider.NewProviderError("response_decode_failed", "failed to decode response", err)
		}
	}

	return nil
}

// parseAPIError parses an API error response.
func (c *Client) parseAPIError(statusCode int, body []byte) error {
	var apiErr apiError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		// If we can't parse the error, use the raw body
		return c.statusCodeToError(statusCode, string(body))
	}

	errMsg := apiErr.Error.Message
	if errMsg == "" {
		errMsg = "unknown error"
	}

	return c.statusCodeToError(statusCode, errMsg)
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
		return provider.NewProviderError("service_unavailable", "Lambda Labs service temporarily unavailable", fmt.Errorf("%s", message))
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

// lambdaInstanceType represents an instance type from the Lambda Labs API.
type lambdaInstanceType struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	PriceCentsPerHour int    `json:"price_cents_per_hour"`
	Specs             struct {
		VCPUs   int `json:"vcpus"`
		MemGB   int `json:"memory_gib"`
		Storage int `json:"storage_gib"`
		GPUs    int `json:"gpus"`
	} `json:"specs"`
}

// lambdaInstanceTypesResponse represents the response from the instance types endpoint.
type lambdaInstanceTypesResponse struct {
	Data map[string]lambdaInstanceType `json:"data"`
}

// lambdaAvailabilityResponse represents the response from the instance types availability endpoint.
type lambdaAvailabilityResponse struct {
	Data map[string]struct {
		InstanceTypes map[string]struct {
			Available bool `json:"available"`
		} `json:"instance_types"`
	} `json:"data"`
}

// GetOffers returns available GPU offers matching the filter criteria.
// Lambda Labs does NOT support spot instances - all prices are on-demand.
func (c *Client) GetOffers(ctx context.Context, filter provider.OfferFilter) ([]provider.Offer, error) {
	// Lambda Labs has spot-only filter check - immediately return empty if spot only is requested
	if filter.SpotOnly {
		return []provider.Offer{}, nil
	}

	// Get instance types with pricing
	var typesResp lambdaInstanceTypesResponse
	if err := c.request(ctx, http.MethodGet, "/instance-types", nil, &typesResp); err != nil {
		return nil, err
	}

	// Get availability information
	var availResp lambdaAvailabilityResponse
	if err := c.request(ctx, http.MethodGet, "/instance-types", nil, &availResp); err != nil {
		// If availability check fails, continue without it
		availResp = lambdaAvailabilityResponse{}
	}

	// Convert Lambda Labs instance types to standard Offer type
	offers := make([]provider.Offer, 0)
	for typeName, instanceType := range typesResp.Data {
		// Convert price from cents to dollars
		pricePerHour := float64(instanceType.PriceCentsPerHour) / 100.0

		// Parse GPU info from instance type name
		gpu, vram := parseGPUFromInstanceType(typeName, instanceType.Description)

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

		// Apply max price filter
		if filter.MaxHourlyPrice > 0 && pricePerHour > filter.MaxHourlyPrice {
			continue
		}

		// Check availability across regions
		for regionName, regionData := range availResp.Data {
			// Apply region filter
			normalizedRegion := normalizeRegion(regionName)
			if filter.Region != "" && !regionMatches(normalizedRegion, filter.Region) {
				continue
			}

			// Check if this instance type is available in this region
			instanceAvail, exists := regionData.InstanceTypes[typeName]
			available := exists && instanceAvail.Available

			// Create offer
			offer := provider.Offer{
				OfferID:       fmt.Sprintf("%s@%s", typeName, regionName),
				Provider:      "lambda",
				GPU:           gpu,
				VRAM:          vram,
				Region:        normalizedRegion,
				OnDemandPrice: pricePerHour,
				SpotPrice:     nil, // Lambda Labs does not support spot instances
				StoragePrice:  0,   // Storage included in Lambda Labs
				EgressPrice:   0,   // Egress typically free on Lambda Labs
				Available:     available,
			}

			// Apply availability filter
			if !offer.Available {
				continue
			}

			offers = append(offers, offer)
		}
	}

	return offers, nil
}

// parseGPUFromInstanceType extracts GPU name and VRAM from Lambda Labs instance type.
func parseGPUFromInstanceType(typeName, description string) (string, int) {
	// Lambda Labs instance types follow patterns like:
	// gpu_1x_a100, gpu_8x_a100, gpu_1x_a100_sxm4, gpu_1x_h100_pcie, etc.
	switch {
	case contains(typeName, "a100") && contains(typeName, "sxm"):
		// A100 SXM (80GB variant)
		return "A100 80GB", 80
	case contains(typeName, "a100"):
		// A100 (40GB standard)
		return "A100 40GB", 40
	case contains(typeName, "h100"):
		// H100 (80GB)
		return "H100 80GB", 80
	case contains(typeName, "a6000"):
		return "A6000 48GB", 48
	case contains(typeName, "rtx6000"):
		return "RTX 6000", 24
	case contains(typeName, "a10"):
		return "A10", 24
	default:
		// Parse from description if available
		if contains(description, "A100") && contains(description, "80") {
			return "A100 80GB", 80
		}
		if contains(description, "A100") {
			return "A100 40GB", 40
		}
		if contains(description, "H100") {
			return "H100 80GB", 80
		}
		// Default to generic GPU
		return typeName, 0
	}
}

// contains checks if s contains substr (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsLower(s, substr))
}

// containsLower checks if s contains substr (case-insensitive) using simple ASCII lowering.
func containsLower(s, substr string) bool {
	sl := toLower(s)
	subsl := toLower(substr)
	for i := 0; i <= len(sl)-len(subsl); i++ {
		if sl[i:i+len(subsl)] == subsl {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase (ASCII only).
func toLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// normalizeGPUName normalizes GPU names for comparison.
func normalizeGPUName(name string) string {
	lower := toLower(name)
	// Remove common separators and normalize
	switch {
	case contains(lower, "a100") && (contains(lower, "80") || contains(lower, "sxm")):
		return "a100-80gb"
	case contains(lower, "a100"):
		return "a100-40gb"
	case contains(lower, "h100"):
		return "h100-80gb"
	case contains(lower, "a6000"):
		return "a6000"
	default:
		return lower
	}
}

// normalizeRegion converts Lambda Labs region codes to standardized region names.
func normalizeRegion(region string) string {
	switch region {
	case "us-west-1", "us-west-2":
		return "US-West"
	case "us-east-1", "us-east-2":
		return "US-East"
	case "us-south-1":
		return "US-South"
	case "us-midwest-1":
		return "US-Midwest"
	case "europe-central-1", "eu-central-1":
		return "EU-Central"
	case "asia-northeast-1":
		return "AP-Northeast"
	case "asia-southeast-1":
		return "AP-Southeast"
	case "australia-southeast-1":
		return "AU"
	default:
		if region == "" {
			return "Unknown"
		}
		return region
	}
}

// regionMatches checks if a region matches a filter (with some flexibility).
func regionMatches(region, filter string) bool {
	// Exact match
	if region == filter {
		return true
	}

	filterLower := toLower(filter)
	regionLower := toLower(region)

	// US regions
	if filterLower == "us" || filterLower == "us-east" || filterLower == "us-west" {
		return contains(regionLower, "us")
	}

	// EU regions
	if filterLower == "eu" || filterLower == "eu-west" || filterLower == "eu-central" {
		return contains(regionLower, "eu")
	}

	// Asia-Pacific
	if filterLower == "ap" || filterLower == "asia" {
		return contains(regionLower, "ap") || contains(regionLower, "asia")
	}

	// Australia
	if filterLower == "au" || filterLower == "australia" {
		return contains(regionLower, "au")
	}

	return false
}

// lambdaLaunchRequest represents the request body for launching a Lambda Labs instance.
type lambdaLaunchRequest struct {
	RegionName       string   `json:"region_name"`
	InstanceTypeName string   `json:"instance_type_name"`
	SSHKeyNames      []string `json:"ssh_key_names"`
	FileSystemNames  []string `json:"file_system_names,omitempty"`
	Quantity         int      `json:"quantity,omitempty"`
	Name             string   `json:"name,omitempty"`
}

// lambdaLaunchResponse represents the response from launching a Lambda Labs instance.
type lambdaLaunchResponse struct {
	Data struct {
		InstanceIDs []string `json:"instance_ids"`
	} `json:"data"`
}

// lambdaInstance represents an instance from the Lambda Labs API.
type lambdaInstance struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	IP               string `json:"ip"`
	Status           string `json:"status"` // "booting", "active", "terminating", "terminated", "unhealthy"
	SSHKeyNames      []string `json:"ssh_key_names"`
	FileSystemNames  []string `json:"file_system_names"`
	Region           struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"region"`
	InstanceType struct {
		Name              string `json:"name"`
		Description       string `json:"description"`
		PriceCentsPerHour int    `json:"price_cents_per_hour"`
		Specs             struct {
			VCPUs   int `json:"vcpus"`
			MemGB   int `json:"memory_gib"`
			Storage int `json:"storage_gib"`
			GPUs    int `json:"gpus"`
		} `json:"specs"`
	} `json:"instance_type"`
	Hostname     string `json:"hostname"`
	JupyterToken string `json:"jupyter_token,omitempty"`
	JupyterURL   string `json:"jupyter_url,omitempty"`
}

// lambdaInstanceResponse represents the response from the instance details endpoint.
type lambdaInstanceResponse struct {
	Data lambdaInstance `json:"data"`
}

// CreateInstance creates a new GPU instance with the given configuration.
// Note: Lambda Labs requires SSH keys to be pre-registered. Cloud-init is not directly supported,
// but the startup script can be run after the instance boots via SSH.
func (c *Client) CreateInstance(ctx context.Context, req provider.CreateRequest) (*provider.Instance, error) {
	if req.OfferID == "" {
		return nil, provider.NewProviderError("invalid_request", "offer ID is required", nil)
	}

	// Lambda Labs does not support spot instances
	if req.Spot {
		return nil, provider.ErrSpotNotAvailable.Wrap(fmt.Errorf("Lambda Labs does not support spot instances"))
	}

	// Parse offer ID (format: instance_type@region)
	instanceType, region, err := parseOfferID(req.OfferID)
	if err != nil {
		return nil, provider.NewProviderError("invalid_request", "invalid offer ID format", err)
	}

	// Lambda Labs requires SSH keys to be pre-registered
	// For now, we'll attempt to create without SSH keys and handle the error
	// In production, the user would need to register their SSH key first via the API
	launchReq := lambdaLaunchRequest{
		RegionName:       region,
		InstanceTypeName: instanceType,
		SSHKeyNames:      []string{}, // TODO: Support SSH key registration
		Quantity:         1,
		Name:             "continueplz",
	}

	// If an SSH public key is provided, we'd need to register it first
	// Lambda Labs requires SSH keys to be pre-registered via their API
	if req.SSHPublicKey != "" {
		// TODO: Register SSH key via POST /ssh-keys
		// For now, we'll proceed without it and rely on the instance's default access
	}

	var launchResp lambdaLaunchResponse
	if err := c.request(ctx, http.MethodPost, "/instance-operations/launch", launchReq, &launchResp); err != nil {
		// Check for specific error conditions
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" || contains(provErr.Message, "not found") {
				return nil, provider.ErrOfferNotFound.Wrap(fmt.Errorf("instance type %s not available in region %s", instanceType, region))
			}
			if contains(provErr.Message, "capacity") || contains(provErr.Message, "available") {
				return nil, provider.ErrInsufficientCapacity.Wrap(fmt.Errorf("no capacity available for %s in %s", instanceType, region))
			}
		}
		return nil, err
	}

	if len(launchResp.Data.InstanceIDs) == 0 {
		return nil, provider.NewProviderError("create_failed", "no instance ID returned", nil)
	}

	instanceID := launchResp.Data.InstanceIDs[0]

	// Get instance details
	instance, err := c.GetInstance(ctx, instanceID)
	if err != nil {
		// Instance was created but we couldn't get details
		// Return a minimal instance with the ID
		return &provider.Instance{
			ID:       instanceID,
			Provider: "lambda",
			Status:   provider.InstanceStatusCreating,
			Spot:     false,
		}, nil
	}

	return instance, nil
}

// parseOfferID parses a Lambda Labs offer ID into instance type and region.
func parseOfferID(offerID string) (instanceType, region string, err error) {
	// Format: instance_type@region (e.g., "gpu_1x_a100@us-east-1")
	for i := len(offerID) - 1; i >= 0; i-- {
		if offerID[i] == '@' {
			return offerID[:i], offerID[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid offer ID format: %s (expected instance_type@region)", offerID)
}

// GetInstance retrieves the current status of an instance by ID.
func (c *Client) GetInstance(ctx context.Context, id string) (*provider.Instance, error) {
	if id == "" {
		return nil, provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	var resp lambdaInstanceResponse
	path := fmt.Sprintf("/instances/%s", id)
	if err := c.request(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	// Convert Lambda Labs instance to standard Instance type
	instance := convertLambdaInstance(resp.Data)
	return &instance, nil
}

// convertLambdaInstance converts a Lambda Labs instance to the standard Instance type.
func convertLambdaInstance(li lambdaInstance) provider.Instance {
	// Convert price from cents to dollars
	hourlyRate := float64(li.InstanceType.PriceCentsPerHour) / 100.0

	// Parse GPU info from instance type
	gpu, _ := parseGPUFromInstanceType(li.InstanceType.Name, li.InstanceType.Description)

	instance := provider.Instance{
		ID:         li.ID,
		Provider:   "lambda",
		Status:     mapLambdaStatus(li.Status),
		PublicIP:   li.IP,
		GPU:        gpu,
		Region:     normalizeRegion(li.Region.Name),
		Spot:       false, // Lambda Labs does not support spot instances
		HourlyRate: hourlyRate,
		// Lambda Labs doesn't provide created_at in the API response
		// CreatedAt will be zero time
	}

	// Use hostname as fallback for IP if not available
	if instance.PublicIP == "" && li.Hostname != "" {
		instance.PublicIP = li.Hostname
	}

	return instance
}

// mapLambdaStatus maps Lambda Labs status strings to our InstanceStatus type.
func mapLambdaStatus(status string) provider.InstanceStatus {
	switch status {
	case "active":
		return provider.InstanceStatusRunning
	case "booting":
		return provider.InstanceStatusCreating
	case "terminating":
		return provider.InstanceStatusStopping
	case "terminated":
		return provider.InstanceStatusTerminated
	case "unhealthy":
		return provider.InstanceStatusError
	default:
		if status == "" {
			return provider.InstanceStatusCreating
		}
		return provider.InstanceStatusError
	}
}

// lambdaTerminateRequest represents the request body for terminating Lambda Labs instances.
type lambdaTerminateRequest struct {
	InstanceIDs []string `json:"instance_ids"`
}

// lambdaTerminateResponse represents the response from terminating Lambda Labs instances.
type lambdaTerminateResponse struct {
	Data struct {
		TerminatedInstances []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"terminated_instances"`
	} `json:"data"`
}

// TerminateInstance terminates an instance by ID.
// This is idempotent - terminating an already-terminated instance does not error.
func (c *Client) TerminateInstance(ctx context.Context, id string) error {
	if id == "" {
		return provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	terminateReq := lambdaTerminateRequest{
		InstanceIDs: []string{id},
	}

	var resp lambdaTerminateResponse
	if err := c.request(ctx, http.MethodPost, "/instance-operations/terminate", terminateReq, &resp); err != nil {
		// Check if the error is "instance not found" - treat as success (idempotent)
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" || contains(provErr.Message, "not found") {
				// Instance already terminated or doesn't exist - that's fine
				return nil
			}
		}
		return err
	}

	return nil
}

// GetBillingStatus returns the billing status for an instance.
// For Lambda Labs, billing is determined by the instance's status:
// - If instance is "active" or "booting" → BillingActive
// - If instance is "terminating" → BillingActive (conservative)
// - If instance is "terminated" or not found → BillingStopped
// - If status cannot be determined → BillingUnknown
func (c *Client) GetBillingStatus(ctx context.Context, id string) (provider.BillingStatus, error) {
	if id == "" {
		return provider.BillingUnknown, provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	// Get instance details to check status
	instance, err := c.GetInstance(ctx, id)
	if err != nil {
		// Check if instance not found - that means billing has stopped
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" || contains(provErr.Message, "not found") {
				return provider.BillingStopped, nil
			}
		}
		// For other errors, we can't determine billing status
		return provider.BillingUnknown, err
	}

	// Map instance status to billing status
	switch instance.Status {
	case provider.InstanceStatusRunning, provider.InstanceStatusCreating:
		// Instance is active, billing is active
		return provider.BillingActive, nil
	case provider.InstanceStatusStopping:
		// Instance is stopping but may still be billed until fully stopped
		// Return active to be conservative
		return provider.BillingActive, nil
	case provider.InstanceStatusTerminated:
		// Instance is terminated, billing has stopped
		return provider.BillingStopped, nil
	case provider.InstanceStatusError:
		// Error state - billing status depends on whether the instance is actually running
		// Return unknown as we can't be sure
		return provider.BillingUnknown, nil
	default:
		return provider.BillingUnknown, nil
	}
}

// lambdaSSHKeysResponse represents the response from the Lambda Labs SSH keys endpoint.
type lambdaSSHKeysResponse struct {
	Data []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		PublicKey string `json:"public_key"`
	} `json:"data"`
}

// ValidateAPIKey validates the API key and returns account information.
// Lambda Labs doesn't have a dedicated user info endpoint, so we validate by listing SSH keys.
func (c *Client) ValidateAPIKey(ctx context.Context) (*provider.AccountInfo, error) {
	// Lambda Labs doesn't have a direct "me" endpoint like other providers.
	// We validate the API key by attempting to list SSH keys, which requires auth.
	var resp lambdaSSHKeysResponse
	if err := c.request(ctx, http.MethodGet, "/ssh-keys", nil, &resp); err != nil {
		// Check for authentication errors
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "authentication_failed" {
				return &provider.AccountInfo{Valid: false}, provider.ErrAuthenticationFailed.Wrap(err)
			}
		}
		return nil, err
	}

	// If we successfully fetched SSH keys, the API key is valid.
	// Lambda Labs doesn't expose email/balance via API, so we return minimal info.
	return &provider.AccountInfo{
		Email:           "", // Not available via Lambda Labs API
		Username:        "", // Not available via Lambda Labs API
		Balance:         nil, // Balance not available via API
		BalanceCurrency: "",
		AccountID:       "", // Not available
		Valid:           true,
	}, nil
}
