// Package mock provides a mock implementation of the Provider interface for testing.
package mock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tmeurs/spinup/internal/provider"
)

// Provider is a mock implementation of the provider.Provider interface.
// It allows configuring responses and simulating errors and delays for testing.
type Provider struct {
	mu sync.Mutex

	// Configuration
	name                        string
	consoleURL                  string
	supportsBillingVerification bool

	// Response configuration
	offers    []provider.Offer
	instances map[string]*provider.Instance
	nextID    int

	// Error injection
	getOffersError        error
	createInstanceError   error
	getInstanceError      error
	terminateInstanceError error
	getBillingStatusError error
	validateAPIKeyError   error

	// Delay injection (simulates slow API responses)
	getOffersDelay        time.Duration
	createInstanceDelay   time.Duration
	getInstanceDelay      time.Duration
	terminateInstanceDelay time.Duration
	getBillingStatusDelay time.Duration

	// Billing status override
	billingStatusOverride *provider.BillingStatus

	// Account info for validation
	accountInfo *provider.AccountInfo

	// Call tracking for assertions
	GetOffersCalls        []GetOffersCall
	CreateInstanceCalls   []CreateInstanceCall
	GetInstanceCalls      []GetInstanceCall
	TerminateInstanceCalls []TerminateInstanceCall
	GetBillingStatusCalls []GetBillingStatusCall
	ValidateAPIKeyCalls   int
}

// GetOffersCall records a call to GetOffers.
type GetOffersCall struct {
	Filter provider.OfferFilter
}

// CreateInstanceCall records a call to CreateInstance.
type CreateInstanceCall struct {
	Request provider.CreateRequest
}

// GetInstanceCall records a call to GetInstance.
type GetInstanceCall struct {
	ID string
}

// TerminateInstanceCall records a call to TerminateInstance.
type TerminateInstanceCall struct {
	ID string
}

// GetBillingStatusCall records a call to GetBillingStatus.
type GetBillingStatusCall struct {
	ID string
}

// Option is a functional option for configuring the mock provider.
type Option func(*Provider)

// New creates a new mock provider with the given options.
func New(opts ...Option) *Provider {
	p := &Provider{
		name:                        "mock",
		consoleURL:                  "https://mock.example.com/console",
		supportsBillingVerification: true,
		instances:                   make(map[string]*provider.Instance),
		nextID:                      1000,
		accountInfo: &provider.AccountInfo{
			Valid:    true,
			Email:    "test@example.com",
			Username: "testuser",
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// WithName sets the provider name.
func WithName(name string) Option {
	return func(p *Provider) {
		p.name = name
	}
}

// WithConsoleURL sets the console URL.
func WithConsoleURL(url string) Option {
	return func(p *Provider) {
		p.consoleURL = url
	}
}

// WithBillingVerificationSupport sets whether billing verification is supported.
func WithBillingVerificationSupport(supported bool) Option {
	return func(p *Provider) {
		p.supportsBillingVerification = supported
	}
}

// WithOffers sets the offers that GetOffers will return.
func WithOffers(offers []provider.Offer) Option {
	return func(p *Provider) {
		p.offers = offers
	}
}

// WithGetOffersError sets an error to return from GetOffers.
func WithGetOffersError(err error) Option {
	return func(p *Provider) {
		p.getOffersError = err
	}
}

// WithCreateInstanceError sets an error to return from CreateInstance.
func WithCreateInstanceError(err error) Option {
	return func(p *Provider) {
		p.createInstanceError = err
	}
}

// WithGetInstanceError sets an error to return from GetInstance.
func WithGetInstanceError(err error) Option {
	return func(p *Provider) {
		p.getInstanceError = err
	}
}

// WithTerminateInstanceError sets an error to return from TerminateInstance.
func WithTerminateInstanceError(err error) Option {
	return func(p *Provider) {
		p.terminateInstanceError = err
	}
}

// WithGetBillingStatusError sets an error to return from GetBillingStatus.
func WithGetBillingStatusError(err error) Option {
	return func(p *Provider) {
		p.getBillingStatusError = err
	}
}

// WithValidateAPIKeyError sets an error to return from ValidateAPIKey.
func WithValidateAPIKeyError(err error) Option {
	return func(p *Provider) {
		p.validateAPIKeyError = err
	}
}

// WithGetOffersDelay sets a delay for GetOffers.
func WithGetOffersDelay(d time.Duration) Option {
	return func(p *Provider) {
		p.getOffersDelay = d
	}
}

// WithCreateInstanceDelay sets a delay for CreateInstance.
func WithCreateInstanceDelay(d time.Duration) Option {
	return func(p *Provider) {
		p.createInstanceDelay = d
	}
}

// WithGetInstanceDelay sets a delay for GetInstance.
func WithGetInstanceDelay(d time.Duration) Option {
	return func(p *Provider) {
		p.getInstanceDelay = d
	}
}

// WithTerminateInstanceDelay sets a delay for TerminateInstance.
func WithTerminateInstanceDelay(d time.Duration) Option {
	return func(p *Provider) {
		p.terminateInstanceDelay = d
	}
}

// WithGetBillingStatusDelay sets a delay for GetBillingStatus.
func WithGetBillingStatusDelay(d time.Duration) Option {
	return func(p *Provider) {
		p.getBillingStatusDelay = d
	}
}

// WithBillingStatusOverride sets a fixed billing status to return.
func WithBillingStatusOverride(status provider.BillingStatus) Option {
	return func(p *Provider) {
		p.billingStatusOverride = &status
	}
}

// WithAccountInfo sets the account info for ValidateAPIKey.
func WithAccountInfo(info *provider.AccountInfo) Option {
	return func(p *Provider) {
		p.accountInfo = info
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return p.name
}

// ConsoleURL returns the console URL.
func (p *Provider) ConsoleURL() string {
	return p.consoleURL
}

// SupportsBillingVerification returns whether billing verification is supported.
func (p *Provider) SupportsBillingVerification() bool {
	return p.supportsBillingVerification
}

// GetOffers returns configured offers, applying filters.
func (p *Provider) GetOffers(ctx context.Context, filter provider.OfferFilter) ([]provider.Offer, error) {
	p.mu.Lock()
	p.GetOffersCalls = append(p.GetOffersCalls, GetOffersCall{Filter: filter})
	offers := p.offers
	delay := p.getOffersDelay
	err := p.getOffersError
	p.mu.Unlock()

	// Apply delay
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err != nil {
		return nil, err
	}

	// Apply filters
	var filtered []provider.Offer
	for _, offer := range offers {
		if !matchesFilter(offer, filter) {
			continue
		}
		filtered = append(filtered, offer)
	}

	return filtered, nil
}

// matchesFilter checks if an offer matches the given filter.
func matchesFilter(offer provider.Offer, filter provider.OfferFilter) bool {
	if filter.GPUType != "" && offer.GPU != filter.GPUType {
		return false
	}
	if filter.MinVRAM > 0 && offer.VRAM < filter.MinVRAM {
		return false
	}
	if filter.Region != "" && offer.Region != filter.Region {
		return false
	}
	if filter.SpotOnly && offer.SpotPrice == nil {
		return false
	}
	if filter.OnDemandOnly && offer.SpotPrice != nil {
		return false
	}
	if filter.MaxHourlyPrice > 0 && offer.OnDemandPrice > filter.MaxHourlyPrice {
		return false
	}
	if !offer.Available {
		return false
	}
	return true
}

// CreateInstance creates a mock instance.
func (p *Provider) CreateInstance(ctx context.Context, req provider.CreateRequest) (*provider.Instance, error) {
	p.mu.Lock()
	p.CreateInstanceCalls = append(p.CreateInstanceCalls, CreateInstanceCall{Request: req})
	delay := p.createInstanceDelay
	err := p.createInstanceError
	p.mu.Unlock()

	// Apply delay
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err != nil {
		return nil, err
	}

	// Find the offer to get GPU info
	p.mu.Lock()
	var offer *provider.Offer
	for i := range p.offers {
		if p.offers[i].OfferID == req.OfferID {
			offer = &p.offers[i]
			break
		}
	}

	if offer == nil {
		p.mu.Unlock()
		return nil, provider.ErrOfferNotFound
	}

	// Check spot availability
	if req.Spot && offer.SpotPrice == nil {
		p.mu.Unlock()
		return nil, provider.ErrSpotNotAvailable
	}

	// Create the instance
	id := fmt.Sprintf("mock-%d", p.nextID)
	p.nextID++

	hourlyRate := offer.OnDemandPrice
	if req.Spot && offer.SpotPrice != nil {
		hourlyRate = *offer.SpotPrice
	}

	instance := &provider.Instance{
		ID:         id,
		Provider:   p.name,
		Status:     provider.InstanceStatusRunning,
		PublicIP:   fmt.Sprintf("10.0.0.%d", p.nextID%256),
		GPU:        offer.GPU,
		Region:     offer.Region,
		Spot:       req.Spot,
		CreatedAt:  time.Now(),
		HourlyRate: hourlyRate,
	}

	p.instances[id] = instance
	p.mu.Unlock()

	// Return a copy to prevent external modification
	instanceCopy := *instance
	return &instanceCopy, nil
}

// GetInstance returns a mock instance by ID.
func (p *Provider) GetInstance(ctx context.Context, id string) (*provider.Instance, error) {
	p.mu.Lock()
	p.GetInstanceCalls = append(p.GetInstanceCalls, GetInstanceCall{ID: id})
	delay := p.getInstanceDelay
	err := p.getInstanceError
	instance, exists := p.instances[id]
	p.mu.Unlock()

	// Apply delay
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, provider.ErrInstanceNotFound
	}

	// Return a copy
	instanceCopy := *instance
	return &instanceCopy, nil
}

// TerminateInstance terminates a mock instance.
func (p *Provider) TerminateInstance(ctx context.Context, id string) error {
	p.mu.Lock()
	p.TerminateInstanceCalls = append(p.TerminateInstanceCalls, TerminateInstanceCall{ID: id})
	delay := p.terminateInstanceDelay
	err := p.terminateInstanceError
	instance, exists := p.instances[id]
	p.mu.Unlock()

	// Apply delay
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err != nil {
		return err
	}

	// Idempotent: don't error if already terminated or doesn't exist
	if !exists {
		return nil
	}

	// Mark as terminated
	p.mu.Lock()
	instance.Status = provider.InstanceStatusTerminated
	p.mu.Unlock()

	return nil
}

// GetBillingStatus returns the billing status for an instance.
func (p *Provider) GetBillingStatus(ctx context.Context, id string) (provider.BillingStatus, error) {
	p.mu.Lock()
	p.GetBillingStatusCalls = append(p.GetBillingStatusCalls, GetBillingStatusCall{ID: id})
	delay := p.getBillingStatusDelay
	err := p.getBillingStatusError
	override := p.billingStatusOverride
	instance, exists := p.instances[id]
	p.mu.Unlock()

	// Apply delay
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return provider.BillingUnknown, ctx.Err()
		}
	}

	if err != nil {
		return provider.BillingUnknown, err
	}

	// Use override if set
	if override != nil {
		return *override, nil
	}

	// Derive from instance status
	if !exists {
		return provider.BillingStopped, nil
	}

	switch instance.Status {
	case provider.InstanceStatusRunning, provider.InstanceStatusCreating:
		return provider.BillingActive, nil
	case provider.InstanceStatusTerminated:
		return provider.BillingStopped, nil
	default:
		return provider.BillingUnknown, nil
	}
}

// ValidateAPIKey validates the API key (mock always succeeds unless error is configured).
func (p *Provider) ValidateAPIKey(ctx context.Context) (*provider.AccountInfo, error) {
	p.mu.Lock()
	p.ValidateAPIKeyCalls++
	err := p.validateAPIKeyError
	info := p.accountInfo
	p.mu.Unlock()

	if err != nil {
		return nil, err
	}

	if info == nil {
		return &provider.AccountInfo{Valid: true}, nil
	}

	// Return a copy
	infoCopy := *info
	return &infoCopy, nil
}

// SetOffers sets the offers at runtime (useful for test scenarios).
func (p *Provider) SetOffers(offers []provider.Offer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.offers = offers
}

// SetError sets an error for a specific operation at runtime.
func (p *Provider) SetError(operation string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch operation {
	case "GetOffers":
		p.getOffersError = err
	case "CreateInstance":
		p.createInstanceError = err
	case "GetInstance":
		p.getInstanceError = err
	case "TerminateInstance":
		p.terminateInstanceError = err
	case "GetBillingStatus":
		p.getBillingStatusError = err
	case "ValidateAPIKey":
		p.validateAPIKeyError = err
	}
}

// SetDelay sets a delay for a specific operation at runtime.
func (p *Provider) SetDelay(operation string, delay time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch operation {
	case "GetOffers":
		p.getOffersDelay = delay
	case "CreateInstance":
		p.createInstanceDelay = delay
	case "GetInstance":
		p.getInstanceDelay = delay
	case "TerminateInstance":
		p.terminateInstanceDelay = delay
	case "GetBillingStatus":
		p.getBillingStatusDelay = delay
	}
}

// SetInstanceStatus updates the status of an existing instance (for testing transitions).
func (p *Provider) SetInstanceStatus(id string, status provider.InstanceStatus) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	instance, exists := p.instances[id]
	if !exists {
		return provider.ErrInstanceNotFound
	}

	instance.Status = status
	return nil
}

// AddInstance adds an instance directly (for setting up test scenarios).
func (p *Provider) AddInstance(instance *provider.Instance) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.instances[instance.ID] = instance
}

// Reset clears all state and call tracking.
func (p *Provider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.instances = make(map[string]*provider.Instance)
	p.GetOffersCalls = nil
	p.CreateInstanceCalls = nil
	p.GetInstanceCalls = nil
	p.TerminateInstanceCalls = nil
	p.GetBillingStatusCalls = nil
	p.ValidateAPIKeyCalls = 0

	// Clear errors
	p.getOffersError = nil
	p.createInstanceError = nil
	p.getInstanceError = nil
	p.terminateInstanceError = nil
	p.getBillingStatusError = nil
	p.validateAPIKeyError = nil

	// Clear delays
	p.getOffersDelay = 0
	p.createInstanceDelay = 0
	p.getInstanceDelay = 0
	p.terminateInstanceDelay = 0
	p.getBillingStatusDelay = 0

	// Clear overrides
	p.billingStatusOverride = nil
}

// Ensure Provider implements the provider.Provider interface.
var _ provider.Provider = (*Provider)(nil)
