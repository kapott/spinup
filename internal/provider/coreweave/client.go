// Package coreweave implements the CoreWeave provider for GPU instance management.
// CoreWeave uses a Kubernetes-based API for managing GPU instances.
package coreweave

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

	"github.com/tmeurs/spinup/internal/provider"
)

const (
	// baseURL is the CoreWeave API base URL.
	// CoreWeave uses a REST API that wraps their Kubernetes infrastructure.
	baseURL = "https://api.coreweave.com/v1"

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

	// consoleURL is the CoreWeave web console URL.
	consoleURL = "https://cloud.coreweave.com/"
)

// Client is the CoreWeave API client.
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

// NewClient creates a new CoreWeave API client.
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
	return "coreweave"
}

// ConsoleURL returns the CoreWeave web console URL.
func (c *Client) ConsoleURL() string {
	return consoleURL
}

// SupportsBillingVerification returns true as CoreWeave has billing APIs.
func (c *Client) SupportsBillingVerification() bool {
	return true
}

// apiError represents an error response from the CoreWeave API.
type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code"`
	Status  int    `json:"status"`
}

// request makes an HTTP request to the CoreWeave API with retry logic.
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

		// Set headers - CoreWeave uses Bearer token authentication
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
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

	errMsg := apiErr.Message
	if errMsg == "" {
		errMsg = apiErr.Error
	}
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
		return provider.NewProviderError("service_unavailable", "CoreWeave service temporarily unavailable", fmt.Errorf("%s", message))
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

// coreweaveGPUType represents a GPU type available on CoreWeave.
type coreweaveGPUType struct {
	Name         string  `json:"name"`
	VRAM         int     `json:"vram_gb"`
	OnDemandRate float64 `json:"on_demand_rate_per_hour"`
	SpotRate     float64 `json:"spot_rate_per_hour"`
	Available    bool    `json:"available"`
	Regions      []struct {
		Name         string `json:"name"`
		Available    bool   `json:"available"`
		SpotCapacity bool   `json:"spot_capacity"`
	} `json:"regions"`
}

// coreweaveGPUTypesResponse represents the response from the GPU types endpoint.
type coreweaveGPUTypesResponse struct {
	Data []coreweaveGPUType `json:"data"`
}

// GetOffers returns available GPU offers matching the filter criteria.
// CoreWeave supports both spot and on-demand instances.
func (c *Client) GetOffers(ctx context.Context, filter provider.OfferFilter) ([]provider.Offer, error) {
	// Get GPU types with pricing
	var gpuResp coreweaveGPUTypesResponse
	if err := c.request(ctx, http.MethodGet, "/gpu-types", nil, &gpuResp); err != nil {
		return nil, err
	}

	// Convert CoreWeave GPU types to standard Offer type
	offers := make([]provider.Offer, 0)
	for _, gpuType := range gpuResp.Data {
		// Normalize GPU name
		gpu := normalizeGPUName(gpuType.Name)
		vram := gpuType.VRAM

		// Apply GPU type filter
		if filter.GPUType != "" {
			normalizedFilter := normalizeGPUNameForFilter(filter.GPUType)
			normalizedGPU := normalizeGPUNameForFilter(gpu)
			if normalizedFilter != normalizedGPU {
				continue
			}
		}

		// Apply VRAM filter
		if filter.MinVRAM > 0 && vram < filter.MinVRAM {
			continue
		}

		// Apply max price filter (check on-demand price)
		if filter.MaxHourlyPrice > 0 && gpuType.OnDemandRate > filter.MaxHourlyPrice {
			continue
		}

		// Create offers for each region
		for _, region := range gpuType.Regions {
			if !region.Available {
				continue
			}

			// Apply region filter
			normalizedRegion := normalizeRegion(region.Name)
			if filter.Region != "" && !regionMatches(normalizedRegion, filter.Region) {
				continue
			}

			// Check spot availability
			hasSpot := region.SpotCapacity && gpuType.SpotRate > 0
			if filter.SpotOnly && !hasSpot {
				continue
			}

			// Create offer
			offer := provider.Offer{
				OfferID:       fmt.Sprintf("%s@%s", gpuType.Name, region.Name),
				Provider:      "coreweave",
				GPU:           gpu,
				VRAM:          vram,
				Region:        normalizedRegion,
				OnDemandPrice: gpuType.OnDemandRate,
				StoragePrice:  0, // Storage billed separately in CoreWeave
				EgressPrice:   0, // Standard egress
				Available:     region.Available,
			}

			// Set spot price if available
			if hasSpot {
				spotPrice := gpuType.SpotRate
				offer.SpotPrice = &spotPrice
			}

			offers = append(offers, offer)
		}
	}

	return offers, nil
}

// normalizeGPUName normalizes CoreWeave GPU names to standard format.
func normalizeGPUName(name string) string {
	nameLower := strings.ToLower(name)
	switch {
	case strings.Contains(nameLower, "a100") && (strings.Contains(nameLower, "80") || strings.Contains(nameLower, "sxm")):
		return "A100 80GB"
	case strings.Contains(nameLower, "a100"):
		return "A100 40GB"
	case strings.Contains(nameLower, "h100"):
		return "H100 80GB"
	case strings.Contains(nameLower, "a6000"):
		return "A6000 48GB"
	case strings.Contains(nameLower, "a40"):
		return "A40 48GB"
	case strings.Contains(nameLower, "rtx"):
		if strings.Contains(nameLower, "4090") {
			return "RTX 4090"
		}
		if strings.Contains(nameLower, "3090") {
			return "RTX 3090"
		}
		return name
	default:
		return name
	}
}

// normalizeGPUNameForFilter normalizes GPU names for filtering comparison.
func normalizeGPUNameForFilter(name string) string {
	lower := strings.ToLower(name)
	// Remove common separators and normalize
	lower = strings.ReplaceAll(lower, " ", "")
	lower = strings.ReplaceAll(lower, "-", "")
	switch {
	case strings.Contains(lower, "a100") && (strings.Contains(lower, "80") || strings.Contains(lower, "sxm")):
		return "a10080gb"
	case strings.Contains(lower, "a100"):
		return "a10040gb"
	case strings.Contains(lower, "h100"):
		return "h10080gb"
	case strings.Contains(lower, "a6000"):
		return "a6000"
	default:
		return lower
	}
}

// normalizeRegion converts CoreWeave region codes to standardized region names.
func normalizeRegion(region string) string {
	regionLower := strings.ToLower(region)
	switch {
	case strings.Contains(regionLower, "ord"), strings.Contains(regionLower, "chicago"):
		return "US-Central"
	case strings.Contains(regionLower, "lga"), strings.Contains(regionLower, "new york"), strings.Contains(regionLower, "nyc"):
		return "US-East"
	case strings.Contains(regionLower, "las"), strings.Contains(regionLower, "vegas"):
		return "US-West"
	case strings.Contains(regionLower, "ams"), strings.Contains(regionLower, "amsterdam"):
		return "EU-West"
	case strings.Contains(regionLower, "fra"), strings.Contains(regionLower, "frankfurt"):
		return "EU-Central"
	case strings.Contains(regionLower, "lon"), strings.Contains(regionLower, "london"):
		return "EU-West"
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

	filterLower := strings.ToLower(filter)
	regionLower := strings.ToLower(region)

	// US regions
	if filterLower == "us" || filterLower == "us-east" || filterLower == "us-west" || filterLower == "us-central" {
		return strings.Contains(regionLower, "us")
	}

	// EU regions
	if filterLower == "eu" || filterLower == "eu-west" || filterLower == "eu-central" {
		return strings.Contains(regionLower, "eu")
	}

	return false
}

// coreweaveCreateRequest represents the request body for creating a CoreWeave instance.
type coreweaveCreateRequest struct {
	Name        string `json:"name"`
	GPUType     string `json:"gpu_type"`
	Region      string `json:"region"`
	Spot        bool   `json:"spot"`
	DiskSizeGB  int    `json:"disk_size_gb,omitempty"`
	CloudInit   string `json:"cloud_init,omitempty"`
	SSHKey      string `json:"ssh_public_key,omitempty"`
	GPUCount    int    `json:"gpu_count,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// coreweaveCreateResponse represents the response from creating a CoreWeave instance.
type coreweaveCreateResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// coreweaveInstance represents an instance from the CoreWeave API.
type coreweaveInstance struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"` // "pending", "running", "stopping", "stopped", "terminated", "error"
	PublicIP    string    `json:"public_ip"`
	PrivateIP   string    `json:"private_ip"`
	GPUType     string    `json:"gpu_type"`
	GPUCount    int       `json:"gpu_count"`
	Region      string    `json:"region"`
	Spot        bool      `json:"spot"`
	HourlyRate  float64   `json:"hourly_rate"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	StoppedAt   time.Time `json:"stopped_at,omitempty"`
	DiskSizeGB  int       `json:"disk_size_gb"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// coreweaveInstanceResponse represents the response from the instance details endpoint.
type coreweaveInstanceResponse struct {
	Data coreweaveInstance `json:"data"`
}

// CreateInstance creates a new GPU instance with the given configuration.
func (c *Client) CreateInstance(ctx context.Context, req provider.CreateRequest) (*provider.Instance, error) {
	if req.OfferID == "" {
		return nil, provider.NewProviderError("invalid_request", "offer ID is required", nil)
	}

	// Parse offer ID (format: gpu_type@region)
	gpuType, region, err := parseOfferID(req.OfferID)
	if err != nil {
		return nil, provider.NewProviderError("invalid_request", "invalid offer ID format", err)
	}

	// Build the create request
	createReq := coreweaveCreateRequest{
		Name:     fmt.Sprintf("spinup-%d", time.Now().Unix()),
		GPUType:  gpuType,
		Region:   region,
		Spot:     req.Spot,
		GPUCount: 1,
		Labels: map[string]string{
			"app":        "spinup",
			"managed-by": "spinup",
		},
	}

	// Set disk size (default to 100GB if not specified)
	if req.DiskSizeGB > 0 {
		createReq.DiskSizeGB = req.DiskSizeGB
	} else {
		createReq.DiskSizeGB = 100
	}

	// Inject cloud-init if provided
	if req.CloudInit != "" {
		createReq.CloudInit = req.CloudInit
	}

	// Set SSH public key if provided
	if req.SSHPublicKey != "" {
		createReq.SSHKey = req.SSHPublicKey
	}

	// Make the create request
	var createResp coreweaveCreateResponse
	if err := c.request(ctx, http.MethodPost, "/instances", createReq, &createResp); err != nil {
		// Check for specific error conditions
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" || strings.Contains(provErr.Message, "not found") {
				return nil, provider.ErrOfferNotFound.Wrap(fmt.Errorf("GPU type %s not available in region %s", gpuType, region))
			}
			if strings.Contains(provErr.Message, "capacity") || strings.Contains(provErr.Message, "available") {
				return nil, provider.ErrInsufficientCapacity.Wrap(fmt.Errorf("no capacity available for %s in %s", gpuType, region))
			}
			if req.Spot && strings.Contains(provErr.Message, "spot") {
				return nil, provider.ErrSpotNotAvailable.Wrap(fmt.Errorf("spot instances not available for %s in %s", gpuType, region))
			}
		}
		return nil, err
	}

	if createResp.Error != "" {
		return nil, provider.NewProviderError("create_failed", createResp.Error, nil)
	}

	if createResp.ID == "" {
		return nil, provider.NewProviderError("create_failed", "no instance ID returned", nil)
	}

	// Get instance details
	instance, err := c.GetInstance(ctx, createResp.ID)
	if err != nil {
		// Instance was created but we couldn't get details
		// Return a minimal instance with the ID
		return &provider.Instance{
			ID:       createResp.ID,
			Provider: "coreweave",
			Status:   provider.InstanceStatusCreating,
			Spot:     req.Spot,
		}, nil
	}

	return instance, nil
}

// parseOfferID parses a CoreWeave offer ID into GPU type and region.
func parseOfferID(offerID string) (gpuType, region string, err error) {
	// Format: gpu_type@region (e.g., "A100_40GB@ord1")
	parts := strings.SplitN(offerID, "@", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid offer ID format: %s (expected gpu_type@region)", offerID)
	}
	return parts[0], parts[1], nil
}

// GetInstance retrieves the current status of an instance by ID.
func (c *Client) GetInstance(ctx context.Context, id string) (*provider.Instance, error) {
	if id == "" {
		return nil, provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	var resp coreweaveInstanceResponse
	path := fmt.Sprintf("/instances/%s", id)
	if err := c.request(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	// Convert CoreWeave instance to standard Instance type
	instance := convertCoreweaveInstance(resp.Data)
	return &instance, nil
}

// convertCoreweaveInstance converts a CoreWeave instance to the standard Instance type.
func convertCoreweaveInstance(ci coreweaveInstance) provider.Instance {
	instance := provider.Instance{
		ID:         ci.ID,
		Provider:   "coreweave",
		Status:     mapCoreweaveStatus(ci.Status),
		PublicIP:   ci.PublicIP,
		GPU:        normalizeGPUName(ci.GPUType),
		Region:     normalizeRegion(ci.Region),
		Spot:       ci.Spot,
		HourlyRate: ci.HourlyRate,
		CreatedAt:  ci.CreatedAt,
	}

	// Use private IP as fallback if public IP is not available
	if instance.PublicIP == "" && ci.PrivateIP != "" {
		instance.PublicIP = ci.PrivateIP
	}

	return instance
}

// mapCoreweaveStatus maps CoreWeave status strings to our InstanceStatus type.
func mapCoreweaveStatus(status string) provider.InstanceStatus {
	switch strings.ToLower(status) {
	case "running", "active":
		return provider.InstanceStatusRunning
	case "pending", "creating", "starting", "provisioning":
		return provider.InstanceStatusCreating
	case "stopping", "terminating":
		return provider.InstanceStatusStopping
	case "stopped", "terminated", "deleted":
		return provider.InstanceStatusTerminated
	case "error", "failed":
		return provider.InstanceStatusError
	default:
		if status == "" {
			return provider.InstanceStatusCreating
		}
		return provider.InstanceStatusError
	}
}

// coreweaveDeleteResponse represents the response from deleting a CoreWeave instance.
type coreweaveDeleteResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// TerminateInstance terminates an instance by ID.
// This is idempotent - terminating an already-terminated instance does not error.
func (c *Client) TerminateInstance(ctx context.Context, id string) error {
	if id == "" {
		return provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	var resp coreweaveDeleteResponse
	path := fmt.Sprintf("/instances/%s", id)
	if err := c.request(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		// Check if the error is "instance not found" - treat as success (idempotent)
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" || strings.Contains(provErr.Message, "not found") {
				// Instance already terminated or doesn't exist - that's fine
				return nil
			}
		}
		return err
	}

	// Check response for errors (but be lenient - not found means already terminated)
	if resp.Error != "" && !strings.Contains(strings.ToLower(resp.Error), "not found") {
		return provider.NewProviderError("terminate_failed", resp.Error, nil)
	}

	return nil
}

// GetBillingStatus returns the billing status for an instance.
// For CoreWeave, billing is determined by the instance's status:
// - If instance is "running" or "pending" → BillingActive
// - If instance is "stopping" → BillingActive (conservative)
// - If instance is "stopped", "terminated", or not found → BillingStopped
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
			if provErr.Code == "instance_not_found" || strings.Contains(provErr.Message, "not found") {
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

// coreweaveUserResponse represents the response from the CoreWeave user endpoint.
type coreweaveUserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	OrgName   string `json:"org_name"`
	OrgID     string `json:"org_id"`
	Namespace string `json:"namespace"`
	Error     string `json:"error,omitempty"`
}

// ValidateAPIKey validates the API key and returns account information.
func (c *Client) ValidateAPIKey(ctx context.Context) (*provider.AccountInfo, error) {
	// CoreWeave uses GET /user or /me endpoint to retrieve current user info
	// The exact endpoint may vary - we try /user first, then /me
	var resp coreweaveUserResponse
	if err := c.request(ctx, http.MethodGet, "/user", nil, &resp); err != nil {
		// Check for authentication errors
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "authentication_failed" {
				return &provider.AccountInfo{Valid: false}, provider.ErrAuthenticationFailed.Wrap(err)
			}
		}
		return nil, err
	}

	if resp.Error != "" {
		return &provider.AccountInfo{Valid: false}, provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("API error: %s", resp.Error))
	}

	// CoreWeave is Kubernetes-based and doesn't expose balance via REST API
	// Account ID is typically the org ID or namespace
	accountID := resp.OrgID
	if accountID == "" {
		accountID = resp.Namespace
	}

	return &provider.AccountInfo{
		Email:           resp.Email,
		Username:        resp.OrgName,
		Balance:         nil, // CoreWeave doesn't expose balance via API
		BalanceCurrency: "",
		AccountID:       accountID,
		Valid:           true,
	}, nil
}
