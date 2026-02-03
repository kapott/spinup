// Package vast implements the Vast.ai provider for GPU instance management.
package vast

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

	"github.com/tmeurs/spinup/internal/provider"
)

const (
	// baseURL is the Vast.ai API base URL.
	baseURL = "https://console.vast.ai/api/v0"

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

	// consoleURL is the Vast.ai web console URL.
	consoleURL = "https://console.vast.ai/"
)

// Client is the Vast.ai API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string

	// rateLimiter tracks rate limiting state.
	rateLimiter *rateLimiter
}

// rateLimiter implements simple rate limiting tracking.
type rateLimiter struct {
	mu            sync.Mutex
	lastRequest   time.Time
	minInterval   time.Duration
	retryAfter    time.Time
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

// NewClient creates a new Vast.ai API client.
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
	return "vast"
}

// ConsoleURL returns the Vast.ai web console URL.
func (c *Client) ConsoleURL() string {
	return consoleURL
}

// SupportsBillingVerification returns true as Vast.ai has billing APIs.
func (c *Client) SupportsBillingVerification() bool {
	return true
}

// apiError represents an error response from the Vast.ai API.
type apiError struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Msg     string `json:"msg"`
}

// request makes an HTTP request to the Vast.ai API with retry logic.
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

		// Set headers
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

	errMsg := apiErr.Error
	if errMsg == "" {
		errMsg = apiErr.Msg
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
		return provider.NewProviderError("service_unavailable", "Vast.ai service temporarily unavailable", fmt.Errorf("%s", message))
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

// vastOffer represents an offer from the Vast.ai API.
type vastOffer struct {
	ID             int     `json:"id"`
	MachineID      int     `json:"machine_id"`
	GPUName        string  `json:"gpu_name"`
	NumGPUs        int     `json:"num_gpus"`
	GPURam         float64 `json:"gpu_ram"`         // GPU VRAM in GB
	DphTotal       float64 `json:"dph_total"`       // On-demand price per hour
	MinBid         float64 `json:"min_bid"`         // Minimum spot bid price
	StorageCost    float64 `json:"storage_cost"`    // Storage cost per GB per month
	InetUpCost     float64 `json:"inet_up_cost"`    // Upload cost per GB
	InetDownCost   float64 `json:"inet_down_cost"`  // Download cost per GB
	Geolocation    string  `json:"geolocation"`     // Location (country code or region)
	Rentable       bool    `json:"rentable"`        // Is available for rent
	Verified       bool    `json:"verified"`        // Is verified host
	Reliability    float64 `json:"reliability2"`    // Host reliability score
	CudaMaxGood    float64 `json:"cuda_max_good"`   // Max CUDA version
	HostID         int     `json:"host_id"`
	BundleID       int     `json:"bundle_id"`
	CPUCores       int     `json:"cpu_cores"`
	CPURam         float64 `json:"cpu_ram"`         // CPU RAM in GB
	DiskSpace      float64 `json:"disk_space"`      // Disk space in GB
}

// vastSearchRequest represents the search request body for Vast.ai API.
type vastSearchRequest struct {
	Limit    int                    `json:"limit,omitempty"`
	Type     string                 `json:"type,omitempty"`      // "on-demand", "bid", "reserved"
	Order    [][]interface{}        `json:"order,omitempty"`     // Sort order
	Verified map[string]interface{} `json:"verified,omitempty"`
	Rentable map[string]interface{} `json:"rentable,omitempty"`
	GPUName  map[string]interface{} `json:"gpu_name,omitempty"`
	GPURam   map[string]interface{} `json:"gpu_ram,omitempty"`
	NumGPUs  map[string]interface{} `json:"num_gpus,omitempty"`
	DphTotal map[string]interface{} `json:"dph_total,omitempty"`
}

// vastSearchResponse represents the response from the Vast.ai offers API.
type vastSearchResponse struct {
	Offers []vastOffer `json:"offers"`
}

// GetOffers returns available GPU offers matching the filter criteria.
func (c *Client) GetOffers(ctx context.Context, filter provider.OfferFilter) ([]provider.Offer, error) {
	// Build request body with filters
	req := vastSearchRequest{
		Limit: 100, // Reasonable default
		Order: [][]interface{}{{"dph_total", "asc"}}, // Sort by price ascending
		// Only show rentable (available) offers
		Rentable: map[string]interface{}{"eq": true},
		// Prefer verified hosts for reliability
		Verified: map[string]interface{}{"eq": true},
		// We want single-GPU instances
		NumGPUs: map[string]interface{}{"eq": 1},
	}

	// Apply GPU type filter
	if filter.GPUType != "" {
		gpuName := normalizeGPUNameForVast(filter.GPUType)
		req.GPUName = map[string]interface{}{"eq": gpuName}
	}

	// Apply minimum VRAM filter
	if filter.MinVRAM > 0 {
		req.GPURam = map[string]interface{}{"gte": float64(filter.MinVRAM)}
	}

	// Apply max price filter
	if filter.MaxHourlyPrice > 0 {
		req.DphTotal = map[string]interface{}{"lte": filter.MaxHourlyPrice}
	}

	// Make API request
	var resp vastSearchResponse
	if err := c.request(ctx, http.MethodPost, "/bundles/", req, &resp); err != nil {
		return nil, err
	}

	// Convert Vast.ai offers to standard Offer type
	offers := make([]provider.Offer, 0, len(resp.Offers))
	for _, vo := range resp.Offers {
		offer := convertVastOffer(vo)

		// Apply additional filters that the API doesn't support directly
		if !applyLocalFilters(offer, filter) {
			continue
		}

		offers = append(offers, offer)
	}

	return offers, nil
}

// convertVastOffer converts a Vast.ai offer to the standard Offer type.
func convertVastOffer(vo vastOffer) provider.Offer {
	// Convert storage cost from per-month to per-hour
	// Vast.ai reports storage cost per GB per month
	storagePerHour := vo.StorageCost / (30 * 24)

	// Build the offer
	offer := provider.Offer{
		OfferID:       fmt.Sprintf("%d", vo.ID),
		Provider:      "vast",
		GPU:           normalizeGPUNameFromVast(vo.GPUName),
		VRAM:          int(vo.GPURam),
		Region:        normalizeRegion(vo.Geolocation),
		OnDemandPrice: vo.DphTotal,
		StoragePrice:  storagePerHour,
		EgressPrice:   vo.InetDownCost, // Egress is download from provider
		Available:     vo.Rentable,
	}

	// Set spot price if bidding is available (min_bid > 0)
	// Spot price is the minimum bid required
	if vo.MinBid > 0 {
		spotPrice := vo.MinBid
		offer.SpotPrice = &spotPrice
	}

	return offer
}

// normalizeGPUNameForVast converts our standard GPU name to Vast.ai's naming convention.
func normalizeGPUNameForVast(gpuType string) string {
	// Map our GPU names to Vast.ai's naming
	switch gpuType {
	case "A100-40GB":
		return "A100"
	case "A100-80GB":
		return "A100_80GB"
	case "H100-80GB":
		return "H100"
	case "A6000":
		return "RTX_A6000"
	default:
		return gpuType
	}
}

// normalizeGPUNameFromVast converts Vast.ai's GPU name to our standard naming.
func normalizeGPUNameFromVast(vastName string) string {
	// Vast.ai uses various naming conventions
	switch vastName {
	case "A100", "A100_40GB", "A100 40GB", "NVIDIA A100 40GB":
		return "A100 40GB"
	case "A100_80GB", "A100 80GB", "NVIDIA A100 80GB", "A100_PCIE_80GB", "A100_SXM4_80GB":
		return "A100 80GB"
	case "H100", "H100_80GB", "H100 80GB", "NVIDIA H100", "H100_SXM5", "H100_PCIE":
		return "H100 80GB"
	case "RTX_A6000", "A6000", "RTX A6000", "NVIDIA RTX A6000":
		return "A6000 48GB"
	default:
		// Return as-is for unknown GPUs
		return vastName
	}
}

// normalizeRegion converts Vast.ai geolocation to a standardized region name.
func normalizeRegion(geolocation string) string {
	// Vast.ai typically returns country codes or descriptive locations
	// We normalize these to more user-friendly region names
	switch geolocation {
	case "US", "US-East", "US-West", "USA":
		return "US"
	case "EU", "Europe", "DE", "NL", "FR", "UK", "GB":
		return "EU"
	case "SE":
		return "EU-North"
	case "CA":
		return "CA"
	case "AU":
		return "AU"
	case "JP":
		return "AP"
	case "SG":
		return "AP"
	default:
		if geolocation == "" {
			return "Unknown"
		}
		return geolocation
	}
}

// applyLocalFilters applies filters that the Vast.ai API doesn't support directly.
func applyLocalFilters(offer provider.Offer, filter provider.OfferFilter) bool {
	// Filter by region
	if filter.Region != "" {
		if offer.Region != filter.Region && !regionMatches(offer.Region, filter.Region) {
			return false
		}
	}

	// Filter by spot availability
	if filter.SpotOnly && offer.SpotPrice == nil {
		return false
	}

	// Filter by on-demand only (no spot)
	if filter.OnDemandOnly && offer.SpotPrice != nil {
		// On-demand only doesn't mean spot isn't available,
		// it means we want to filter to offers that are definitely available on-demand
		// All offers have on-demand pricing, so this filter is essentially a no-op
		// unless we interpret it as "don't show spot prices"
	}

	// Filter by availability
	if !offer.Available {
		return false
	}

	return true
}

// regionMatches checks if a region matches a filter (with some flexibility).
func regionMatches(region, filter string) bool {
	// Exact match
	if region == filter {
		return true
	}

	// EU regions
	if filter == "EU" || filter == "eu" || filter == "eu-west" || filter == "EU-West" {
		return region == "EU" || region == "EU-West" || region == "EU-North"
	}

	// US regions
	if filter == "US" || filter == "us" || filter == "us-east" || filter == "US-East" || filter == "us-west" || filter == "US-West" {
		return region == "US" || region == "US-East" || region == "US-West"
	}

	// Asia-Pacific
	if filter == "AP" || filter == "ap" || filter == "asia" {
		return region == "AP" || region == "JP" || region == "SG"
	}

	return false
}

// vastCreateRequest represents the request body for creating a Vast.ai instance.
type vastCreateRequest struct {
	// Image is the Docker image to use.
	Image string `json:"image"`
	// Disk is the disk size in GB.
	Disk int `json:"disk,omitempty"`
	// Runtype specifies the launch mode (ssh, jupyter, args, etc.).
	Runtype string `json:"runtype,omitempty"`
	// Price is the bid price for spot instances (nil for on-demand).
	Price *float64 `json:"price,omitempty"`
	// Onstart is the cloud-init script to run on startup.
	Onstart string `json:"onstart,omitempty"`
	// Env is the environment variables in Docker format.
	Env string `json:"env,omitempty"`
	// Label is a custom instance name.
	Label string `json:"label,omitempty"`
}

// vastCreateResponse represents the response from creating a Vast.ai instance.
type vastCreateResponse struct {
	Success     bool   `json:"success"`
	NewContract int    `json:"new_contract"`
	Error       string `json:"error,omitempty"`
	Msg         string `json:"msg,omitempty"`
}

// vastInstance represents an instance from the Vast.ai API.
type vastInstance struct {
	ID            int     `json:"id"`
	ActualStatus  string  `json:"actual_status"`   // "running", "created", "loading", "exited", etc.
	CurState      string  `json:"cur_state"`       // Current machine contract state
	NextState     string  `json:"next_state"`      // Next state if transitioning
	SSHHost       string  `json:"ssh_host"`        // SSH connection host
	SSHPort       int     `json:"ssh_port"`        // SSH connection port
	PublicIPAddr  string  `json:"public_ipaddr"`   // Public IP address
	GPUName       string  `json:"gpu_name"`        // GPU model
	GPURam        float64 `json:"gpu_totalram"`    // GPU RAM in MB (total)
	NumGPUs       int     `json:"num_gpus"`        // Number of GPUs
	DphTotal      float64 `json:"dph_total"`       // Dollars per hour
	StartDate     float64 `json:"start_date"`      // Unix timestamp of launch
	EndDate       float64 `json:"end_date"`        // Unix timestamp of termination (0 if still running)
	Geolocation   string  `json:"geolocation"`     // Location
	IsBid         bool    `json:"is_bid"`          // Is spot (bid) instance
	Label         string  `json:"label"`           // Instance label
	StatusMsg     string  `json:"status_msg"`      // Status message
	ImageUUID     string  `json:"image_uuid"`      // Docker image UUID
}

// CreateInstance creates a new GPU instance with the given configuration.
func (c *Client) CreateInstance(ctx context.Context, req provider.CreateRequest) (*provider.Instance, error) {
	if req.OfferID == "" {
		return nil, provider.NewProviderError("invalid_request", "offer ID is required", nil)
	}

	// Build the create request
	// Vast.ai uses the PUT /asks/{id}/ endpoint to accept an offer
	createReq := vastCreateRequest{
		// Use a base image with SSH access
		Image:   "vastai/base-image:latest",
		Runtype: "ssh",
		Label:   "spinup",
	}

	// Set disk size (default to 100GB if not specified)
	if req.DiskSizeGB > 0 {
		createReq.Disk = req.DiskSizeGB
	} else {
		createReq.Disk = 100
	}

	// Set spot bid price if requesting spot instance
	if req.Spot {
		// For spot, we need to set a price (bid)
		// We'll use the min_bid from the offer, which should be passed separately
		// For now, setting a reasonable default that allows acceptance
		// The caller should have verified spot is available
		price := 0.0 // Setting 0 means use the minimum bid price
		createReq.Price = &price
	}

	// Inject cloud-init script via the onstart parameter
	// Vast.ai runs this script when the container starts
	if req.CloudInit != "" {
		createReq.Onstart = req.CloudInit
	}

	// Set SSH public key via environment variable if provided
	if req.SSHPublicKey != "" {
		createReq.Env = fmt.Sprintf("-e SSH_PUBLIC_KEY=%s", req.SSHPublicKey)
	}

	// Create the instance by accepting the offer
	var resp vastCreateResponse
	path := fmt.Sprintf("/asks/%s/", req.OfferID)
	if err := c.request(ctx, http.MethodPut, path, createReq, &resp); err != nil {
		// Check for specific error conditions
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" {
				return nil, provider.ErrOfferNotFound.Wrap(fmt.Errorf("offer %s not found or no longer available", req.OfferID))
			}
		}
		return nil, err
	}

	if !resp.Success {
		errMsg := resp.Error
		if errMsg == "" {
			errMsg = resp.Msg
		}
		if errMsg == "" {
			errMsg = "failed to create instance"
		}
		return nil, provider.NewProviderError("create_failed", errMsg, nil)
	}

	// Get the instance details
	instanceID := fmt.Sprintf("%d", resp.NewContract)
	instance, err := c.GetInstance(ctx, instanceID)
	if err != nil {
		// Instance was created but we couldn't get details
		// Return a minimal instance with the ID
		return &provider.Instance{
			ID:       instanceID,
			Provider: "vast",
			Status:   provider.InstanceStatusCreating,
			Spot:     req.Spot,
		}, nil
	}

	return instance, nil
}

// GetInstance retrieves the current status of an instance by ID.
func (c *Client) GetInstance(ctx context.Context, id string) (*provider.Instance, error) {
	if id == "" {
		return nil, provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	// Vast.ai GET /instances/{id}/ returns the instance details
	var resp vastInstance
	path := fmt.Sprintf("/instances/%s/", id)
	if err := c.request(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	// Convert Vast.ai instance to standard Instance type
	instance := convertVastInstance(resp)
	return &instance, nil
}

// convertVastInstance converts a Vast.ai instance to the standard Instance type.
func convertVastInstance(vi vastInstance) provider.Instance {
	instance := provider.Instance{
		ID:         fmt.Sprintf("%d", vi.ID),
		Provider:   "vast",
		Status:     mapVastStatus(vi.ActualStatus, vi.CurState),
		PublicIP:   vi.PublicIPAddr,
		GPU:        normalizeGPUNameFromVast(vi.GPUName),
		Region:     normalizeRegion(vi.Geolocation),
		Spot:       vi.IsBid,
		HourlyRate: vi.DphTotal,
	}

	// Convert start date from Unix timestamp
	if vi.StartDate > 0 {
		instance.CreatedAt = time.Unix(int64(vi.StartDate), 0)
	}

	// If public IP is empty but we have SSH host, use that
	if instance.PublicIP == "" && vi.SSHHost != "" {
		instance.PublicIP = vi.SSHHost
	}

	return instance
}

// mapVastStatus maps Vast.ai status strings to our InstanceStatus type.
func mapVastStatus(actualStatus, curState string) provider.InstanceStatus {
	// actual_status values: "running", "created", "loading", "exited", "offline", etc.
	// cur_state values: "running", "stopped", etc.
	switch actualStatus {
	case "running":
		return provider.InstanceStatusRunning
	case "created", "loading", "starting":
		return provider.InstanceStatusCreating
	case "exited", "offline", "stopped":
		// Check cur_state for more context
		if curState == "stopped" || curState == "" {
			return provider.InstanceStatusTerminated
		}
		return provider.InstanceStatusStopping
	case "error":
		return provider.InstanceStatusError
	default:
		// Unknown status - check curState
		switch curState {
		case "running":
			return provider.InstanceStatusRunning
		case "stopped", "terminated":
			return provider.InstanceStatusTerminated
		default:
			// Default to creating if unknown
			if actualStatus == "" {
				return provider.InstanceStatusCreating
			}
			return provider.InstanceStatusError
		}
	}
}

// vastDeleteResponse represents the response from deleting a Vast.ai instance.
type vastDeleteResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg,omitempty"`
	Error   string `json:"error,omitempty"`
}

// TerminateInstance terminates an instance by ID.
// This is idempotent - terminating an already-terminated instance does not error.
func (c *Client) TerminateInstance(ctx context.Context, id string) error {
	if id == "" {
		return provider.NewProviderError("invalid_request", "instance ID is required", nil)
	}

	// Vast.ai DELETE /instances/{id}/ destroys the instance
	var resp vastDeleteResponse
	path := fmt.Sprintf("/instances/%s/", id)
	if err := c.request(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		// Check if the error is "instance not found" - treat as success (idempotent)
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "instance_not_found" {
				// Instance already terminated or doesn't exist - that's fine
				return nil
			}
		}
		return err
	}

	// Check response for success
	if !resp.Success {
		errMsg := resp.Error
		if errMsg == "" {
			errMsg = resp.Msg
		}
		if errMsg == "" {
			errMsg = "failed to terminate instance"
		}
		// Check if error indicates instance not found
		if errMsg == "Instance not found" || errMsg == "instance not found" {
			return nil // Idempotent - already terminated
		}
		return provider.NewProviderError("terminate_failed", errMsg, nil)
	}

	return nil
}

// GetBillingStatus returns the billing status for an instance.
// For Vast.ai, billing is determined by the instance's actual_status:
// - If instance is "running", "created", or "loading" → BillingActive
// - If instance is "exited", "offline", "stopped", or terminated → BillingStopped
// - If instance is not found → BillingStopped
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
			if provErr.Code == "instance_not_found" {
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

// vastUserResponse represents the response from the Vast.ai user/current endpoint.
type vastUserResponse struct {
	ID           int     `json:"id"`
	Username     string  `json:"username"`
	Email        string  `json:"email"`
	Credit       float64 `json:"credit"`        // Account credit/balance in USD
	CreditPlus   float64 `json:"credit_plus"`   // Additional credit
	IsVerified   bool    `json:"verified"`      // Email verified
	SSHKey       string  `json:"ssh_key"`       // SSH public key
	APIKeyName   string  `json:"api_key_name"`  // Name of the API key used
	Success      bool    `json:"success"`       // API call success indicator
	Error        string  `json:"error"`         // Error message if any
}

// ValidateAPIKey validates the API key and returns account information.
func (c *Client) ValidateAPIKey(ctx context.Context) (*provider.AccountInfo, error) {
	// Vast.ai uses GET /users/current/ to retrieve current user info
	var resp vastUserResponse
	if err := c.request(ctx, http.MethodGet, "/users/current/", nil, &resp); err != nil {
		// Check for authentication errors
		if provErr, ok := err.(*provider.ProviderError); ok {
			if provErr.Code == "authentication_failed" {
				return &provider.AccountInfo{Valid: false}, provider.ErrAuthenticationFailed.Wrap(err)
			}
		}
		return nil, err
	}

	// Check if response indicates an error
	if resp.Error != "" {
		return &provider.AccountInfo{Valid: false}, provider.ErrAuthenticationFailed.Wrap(fmt.Errorf("API error: %s", resp.Error))
	}

	// Calculate total balance (credit + credit_plus)
	balance := resp.Credit + resp.CreditPlus

	return &provider.AccountInfo{
		Email:           resp.Email,
		Username:        resp.Username,
		Balance:         &balance,
		BalanceCurrency: "USD",
		AccountID:       fmt.Sprintf("%d", resp.ID),
		Valid:           true,
	}, nil
}
