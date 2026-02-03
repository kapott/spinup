// Package paperspace implements the Paperspace provider for GPU instance management.
// Paperspace uses a REST API and does NOT support billing verification.
package paperspace

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
	// baseURL is the Paperspace API base URL.
	baseURL = "https://api.paperspace.io"

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

	// consoleURL is the Paperspace web console URL.
	consoleURL = "https://console.paperspace.com/"
)

// Client is the Paperspace API client.
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

// NewClient creates a new Paperspace API client.
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
	return "paperspace"
}

// ConsoleURL returns the Paperspace web console URL.
// This is used for manual verification since Paperspace doesn't have a billing API.
func (c *Client) ConsoleURL() string {
	return consoleURL
}

// SupportsBillingVerification returns false as Paperspace does NOT have a billing API.
// Users must manually verify that billing has stopped via the console.
func (c *Client) SupportsBillingVerification() bool {
	return false
}

// apiError represents an error response from the Paperspace API.
type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Name    string `json:"name"`
	Status  int    `json:"status"`
}

// request makes an HTTP request to the Paperspace API with retry logic.
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

		// Set headers - Paperspace uses x-api-key header for authentication
		req.Header.Set("Accept", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
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
		return provider.NewProviderError("service_unavailable", "Paperspace service temporarily unavailable", fmt.Errorf("%s", message))
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

// paperspaceTemplate represents a machine template (GPU type) available on Paperspace.
type paperspaceTemplate struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Label        string  `json:"label"`
	GPUType      string  `json:"gpuType"`
	GPUCount     int     `json:"gpuCount"`
	RAM          int     `json:"ram"`           // RAM in GB
	VRAM         int     `json:"vram"`          // GPU VRAM in GB
	CPUCount     int     `json:"cpuCount"`
	HourlyRate   float64 `json:"hourlyRate"`    // Price per hour in USD
	Available    bool    `json:"available"`
	Region       string  `json:"region"`
	Description  string  `json:"description"`
}

// paperspaceTemplatesResponse represents the response from the templates endpoint.
type paperspaceTemplatesResponse []paperspaceTemplate

// GetOffers returns available GPU offers matching the filter criteria.
// Paperspace does NOT support spot instances - all instances are on-demand only.
func (c *Client) GetOffers(ctx context.Context, filter provider.OfferFilter) ([]provider.Offer, error) {
	// If spot-only filter is set, return empty list immediately
	// Paperspace does not support spot instances
	if filter.SpotOnly {
		return []provider.Offer{}, nil
	}

	// Get available machine templates/types
	var templates paperspaceTemplatesResponse
	if err := c.request(ctx, http.MethodGet, "/templates/getTemplates", nil, &templates); err != nil {
		return nil, err
	}

	// Convert Paperspace templates to standard Offer type
	offers := make([]provider.Offer, 0)
	for _, tmpl := range templates {
		// Skip non-GPU templates
		if tmpl.GPUType == "" || tmpl.GPUCount == 0 {
			continue
		}

		// Normalize GPU name
		gpu := normalizeGPUName(tmpl.GPUType)
		vram := tmpl.VRAM

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

		// Apply max price filter
		if filter.MaxHourlyPrice > 0 && tmpl.HourlyRate > filter.MaxHourlyPrice {
			continue
		}

		// Apply region filter
		normalizedRegion := normalizeRegion(tmpl.Region)
		if filter.Region != "" && !regionMatches(normalizedRegion, filter.Region) {
			continue
		}

		// Skip unavailable templates
		if !tmpl.Available {
			continue
		}

		// Create offer - Paperspace does NOT support spot instances
		offer := provider.Offer{
			OfferID:       tmpl.ID,
			Provider:      "paperspace",
			GPU:           gpu,
			VRAM:          vram,
			Region:        normalizedRegion,
			OnDemandPrice: tmpl.HourlyRate,
			SpotPrice:     nil, // No spot pricing on Paperspace
			StoragePrice:  0,   // Storage billed separately
			EgressPrice:   0,   // Standard egress
			Available:     tmpl.Available,
		}

		offers = append(offers, offer)
	}

	return offers, nil
}

// normalizeGPUName normalizes Paperspace GPU names to standard format.
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
	case strings.Contains(nameLower, "v100"):
		return "V100 16GB"
	case strings.Contains(nameLower, "p100"):
		return "P100 16GB"
	case strings.Contains(nameLower, "rtx"):
		if strings.Contains(nameLower, "4000") {
			return "RTX 4000"
		}
		if strings.Contains(nameLower, "5000") {
			return "RTX 5000"
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

// normalizeRegion converts Paperspace region codes to standardized region names.
func normalizeRegion(region string) string {
	regionLower := strings.ToLower(region)
	switch {
	case strings.Contains(regionLower, "ny") || strings.Contains(regionLower, "new york") || strings.Contains(regionLower, "nyc"):
		return "US-East"
	case strings.Contains(regionLower, "ca") || strings.Contains(regionLower, "california") || strings.Contains(regionLower, "sf"):
		return "US-West"
	case strings.Contains(regionLower, "tx") || strings.Contains(regionLower, "texas") || strings.Contains(regionLower, "dallas"):
		return "US-Central"
	case strings.Contains(regionLower, "ams") || strings.Contains(regionLower, "amsterdam"):
		return "EU-West"
	case strings.Contains(regionLower, "lon") || strings.Contains(regionLower, "london"):
		return "EU-West"
	case strings.Contains(regionLower, "fra") || strings.Contains(regionLower, "frankfurt"):
		return "EU-Central"
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

// paperspaceCreateRequest represents the request body for creating a Paperspace machine.
type paperspaceCreateRequest struct {
	Region          string `json:"region"`
	MachineType     string `json:"machineType"`
	Size            int    `json:"size"`           // Disk size in GB
	BillingType     string `json:"billingType"`    // "hourly" or "monthly"
	MachineName     string `json:"machineName"`
	TemplateID      string `json:"templateId"`
	ScriptID        string `json:"scriptId,omitempty"`       // Startup script ID
	StartupScript   string `json:"startupScript,omitempty"`  // Inline startup script
	PublicIpType    string `json:"publicIpType,omitempty"`   // "static" or "dynamic"
	AssignPublicIp  bool   `json:"assignPublicIp"`
}

// paperspaceCreateResponse represents the response from creating a Paperspace machine.
type paperspaceCreateResponse struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	State           string    `json:"state"`
	OS              string    `json:"os"`
	RAM             string    `json:"ram"`
	CPUs            int       `json:"cpus"`
	GPU             string    `json:"gpu"`
	StorageTotal    string    `json:"storageTotal"`
	UsageRate       string    `json:"usageRate"`
	ShutdownTimeout *int      `json:"shutdownTimeoutInHours"`
	PublicIpAddress string    `json:"publicIpAddress"`
	PrivateIpAddress string   `json:"privateIpAddress"`
	Region          string    `json:"region"`
	MachineType     string    `json:"machineType"`
	DtCreated       time.Time `json:"dtCreated"`
	Error           string    `json:"error,omitempty"`
}

// CreateInstance creates a new GPU instance with the given configuration.
// Paperspace does NOT support spot instances - the Spot field in CreateRequest is ignored,
// but if explicitly requested, an error is returned.
func (c *Client) CreateInstance(ctx context.Context, req provider.CreateRequest) (*provider.Instance, error) {
	if req.OfferID == "" {
		return nil, provider.NewProviderError("invalid_request", "offer ID is required", nil)
	}

	// Paperspace does NOT support spot instances
	if req.Spot {
		return nil, provider.ErrSpotNotAvailable.Wrap(fmt.Errorf("Paperspace does not support spot instances"))
	}

	// Build the create request
	// The OfferID for Paperspace is the template ID
	createReq := paperspaceCreateRequest{
		TemplateID:     req.OfferID,
		MachineName:    fmt.Sprintf("spinup-%d", time.Now().Unix()),
		BillingType:    "hourly",
		AssignPublicIp: true,
		PublicIpType:   "dynamic",
	}

	// Set disk size (default to 100GB if not specified)
	if req.DiskSizeGB > 0 {
		createReq.Size = req.DiskSizeGB
	} else {
		createReq.Size = 100
	}

	// Inject startup script (cloud-init equivalent for Paperspace)
	if req.CloudInit != "" {
		createReq.StartupScript = req.CloudInit
	}

	// Make the create request
	var createResp paperspaceCreateResponse
	if err := c.request(ctx, http.MethodPost, "/machines/createSingleMachinePublic", createReq, &createResp); err != nil {
		// Check for specific error conditions
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" || strings.Contains(provErr.Message, "not found") {
				return nil, provider.ErrOfferNotFound.Wrap(fmt.Errorf("template %s not found", req.OfferID))
			}
			if strings.Contains(provErr.Message, "capacity") || strings.Contains(provErr.Message, "available") {
				return nil, provider.ErrInsufficientCapacity.Wrap(fmt.Errorf("no capacity available for template %s", req.OfferID))
			}
		}
		return nil, err
	}

	if createResp.Error != "" {
		return nil, provider.NewProviderError("create_failed", createResp.Error, nil)
	}

	if createResp.ID == "" {
		return nil, provider.NewProviderError("create_failed", "no machine ID returned", nil)
	}

	// Get instance details
	instance, err := c.GetInstance(ctx, createResp.ID)
	if err != nil {
		// Instance was created but we couldn't get details
		// Return a minimal instance with the ID
		return &provider.Instance{
			ID:       createResp.ID,
			Provider: "paperspace",
			Status:   provider.InstanceStatusCreating,
			Spot:     false, // Paperspace never has spot
		}, nil
	}

	return instance, nil
}

// paperspaceMachine represents a machine from the Paperspace API.
type paperspaceMachine struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	State            string    `json:"state"` // "off", "starting", "running", "stopping", "restarting"
	OS               string    `json:"os"`
	RAM              string    `json:"ram"`
	CPUs             int       `json:"cpus"`
	GPU              string    `json:"gpu"`
	StorageTotal     string    `json:"storageTotal"`
	StorageUsed      string    `json:"storageUsed"`
	UsageRate        string    `json:"usageRate"`
	PublicIpAddress  string    `json:"publicIpAddress"`
	PrivateIpAddress string    `json:"privateIpAddress"`
	Region           string    `json:"region"`
	MachineType      string    `json:"machineType"`
	DtCreated        time.Time `json:"dtCreated"`
	DtLastRun        *time.Time `json:"dtLastRun"`
	DtDeleted        *time.Time `json:"dtDeleted"`
}

// GetInstance retrieves the current status of an instance by ID.
func (c *Client) GetInstance(ctx context.Context, id string) (*provider.Instance, error) {
	if id == "" {
		return nil, provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	var machine paperspaceMachine
	path := fmt.Sprintf("/machines/getMachinePublic?machineId=%s", id)
	if err := c.request(ctx, http.MethodGet, path, nil, &machine); err != nil {
		return nil, err
	}

	// Convert Paperspace machine to standard Instance type
	instance := convertPaperspaceMachine(machine)
	return &instance, nil
}

// convertPaperspaceMachine converts a Paperspace machine to the standard Instance type.
func convertPaperspaceMachine(pm paperspaceMachine) provider.Instance {
	// Parse hourly rate from usageRate string (format: "$1.89/hr")
	hourlyRate := parseUsageRate(pm.UsageRate)

	instance := provider.Instance{
		ID:         pm.ID,
		Provider:   "paperspace",
		Status:     mapPaperspaceStatus(pm.State),
		PublicIP:   pm.PublicIpAddress,
		GPU:        normalizeGPUName(pm.GPU),
		Region:     normalizeRegion(pm.Region),
		Spot:       false, // Paperspace never has spot instances
		HourlyRate: hourlyRate,
		CreatedAt:  pm.DtCreated,
	}

	// Use private IP as fallback if public IP is not available
	if instance.PublicIP == "" && pm.PrivateIpAddress != "" {
		instance.PublicIP = pm.PrivateIpAddress
	}

	return instance
}

// parseUsageRate parses Paperspace's usage rate string (e.g., "$1.89/hr") to a float.
func parseUsageRate(rate string) float64 {
	// Remove currency symbol and unit
	rate = strings.TrimPrefix(rate, "$")
	rate = strings.TrimSuffix(rate, "/hr")
	rate = strings.TrimSuffix(rate, "/hour")
	rate = strings.TrimSpace(rate)

	var hourlyRate float64
	fmt.Sscanf(rate, "%f", &hourlyRate)
	return hourlyRate
}

// mapPaperspaceStatus maps Paperspace state strings to our InstanceStatus type.
func mapPaperspaceStatus(state string) provider.InstanceStatus {
	switch strings.ToLower(state) {
	case "running", "ready":
		return provider.InstanceStatusRunning
	case "starting", "provisioning", "pending":
		return provider.InstanceStatusCreating
	case "stopping", "shutting down":
		return provider.InstanceStatusStopping
	case "off", "stopped", "terminated", "deleted":
		return provider.InstanceStatusTerminated
	case "error", "failed":
		return provider.InstanceStatusError
	default:
		if state == "" {
			return provider.InstanceStatusCreating
		}
		return provider.InstanceStatusError
	}
}

// TerminateInstance terminates an instance by ID.
// This is idempotent - terminating an already-terminated instance does not error.
func (c *Client) TerminateInstance(ctx context.Context, id string) error {
	if id == "" {
		return provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	// Paperspace has separate "stop" and "destroy" endpoints
	// We use destroy to fully terminate and delete the machine
	path := fmt.Sprintf("/machines/%s/destroyMachine", id)
	if err := c.request(ctx, http.MethodPost, path, nil, nil); err != nil {
		// Check if the error is "machine not found" - treat as success (idempotent)
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" || strings.Contains(provErr.Message, "not found") {
				// Machine already terminated or doesn't exist - that's fine
				return nil
			}
		}
		return err
	}

	return nil
}

// GetBillingStatus returns the billing status for an instance.
// IMPORTANT: Paperspace does NOT support billing verification via API.
// This method always returns ErrBillingNotSupported.
// Users must manually verify that billing has stopped via the Paperspace console.
func (c *Client) GetBillingStatus(ctx context.Context, id string) (provider.BillingStatus, error) {
	return provider.BillingUnknown, provider.ErrBillingNotSupported.Wrap(
		fmt.Errorf("Paperspace does not provide a billing status API - please verify manually at %s", consoleURL),
	)
}

// paperspaceUserResponse represents the response from the Paperspace user endpoint.
type paperspaceUserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	TeamID    string `json:"teamId"`
	TeamName  string `json:"teamName"`
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ValidateAPIKey validates the API key and returns account information.
// Note: Paperspace API doesn't expose balance information.
func (c *Client) ValidateAPIKey(ctx context.Context) (*provider.AccountInfo, error) {
	// Paperspace uses GET /users/getUser to get current user info
	var resp paperspaceUserResponse
	if err := c.request(ctx, http.MethodGet, "/users/getUser", nil, &resp); err != nil {
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

	// Construct username from first and last name
	username := strings.TrimSpace(resp.FirstName + " " + resp.LastName)
	if username == "" {
		username = resp.TeamName
	}

	return &provider.AccountInfo{
		Email:           resp.Email,
		Username:        username,
		Balance:         nil, // Paperspace doesn't expose balance via API
		BalanceCurrency: "",
		AccountID:       resp.ID,
		Valid:           true,
	}, nil
}
