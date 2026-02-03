package mock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tmeurs/continueplz/internal/provider"
)

func TestProvider_Name(t *testing.T) {
	p := New()
	if got := p.Name(); got != "mock" {
		t.Errorf("Name() = %q, want %q", got, "mock")
	}

	p = New(WithName("custom"))
	if got := p.Name(); got != "custom" {
		t.Errorf("Name() = %q, want %q", got, "custom")
	}
}

func TestProvider_ConsoleURL(t *testing.T) {
	p := New()
	if got := p.ConsoleURL(); got != "https://mock.example.com/console" {
		t.Errorf("ConsoleURL() = %q, want default", got)
	}

	p = New(WithConsoleURL("https://custom.example.com"))
	if got := p.ConsoleURL(); got != "https://custom.example.com" {
		t.Errorf("ConsoleURL() = %q, want custom", got)
	}
}

func TestProvider_SupportsBillingVerification(t *testing.T) {
	p := New()
	if !p.SupportsBillingVerification() {
		t.Error("SupportsBillingVerification() = false, want true")
	}

	p = New(WithBillingVerificationSupport(false))
	if p.SupportsBillingVerification() {
		t.Error("SupportsBillingVerification() = true, want false")
	}
}

func TestProvider_GetOffers(t *testing.T) {
	spotPrice := 0.50
	offers := []provider.Offer{
		{OfferID: "offer1", GPU: "A100 40GB", VRAM: 40, Region: "EU-West", OnDemandPrice: 1.00, SpotPrice: &spotPrice, Available: true},
		{OfferID: "offer2", GPU: "A6000 48GB", VRAM: 48, Region: "US-East", OnDemandPrice: 0.80, Available: true},
		{OfferID: "offer3", GPU: "A100 40GB", VRAM: 40, Region: "EU-West", OnDemandPrice: 1.20, Available: false}, // unavailable
	}

	p := New(WithOffers(offers))
	ctx := context.Background()

	t.Run("no filter returns available offers", func(t *testing.T) {
		got, err := p.GetOffers(ctx, provider.OfferFilter{})
		if err != nil {
			t.Fatalf("GetOffers() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("GetOffers() returned %d offers, want 2", len(got))
		}
	})

	t.Run("filter by GPU type", func(t *testing.T) {
		got, err := p.GetOffers(ctx, provider.OfferFilter{GPUType: "A100 40GB"})
		if err != nil {
			t.Fatalf("GetOffers() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("GetOffers() returned %d offers, want 1", len(got))
		}
	})

	t.Run("filter by region", func(t *testing.T) {
		got, err := p.GetOffers(ctx, provider.OfferFilter{Region: "US-East"})
		if err != nil {
			t.Fatalf("GetOffers() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("GetOffers() returned %d offers, want 1", len(got))
		}
	})

	t.Run("filter by spot only", func(t *testing.T) {
		got, err := p.GetOffers(ctx, provider.OfferFilter{SpotOnly: true})
		if err != nil {
			t.Fatalf("GetOffers() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("GetOffers() returned %d offers, want 1", len(got))
		}
	})

	t.Run("call tracking", func(t *testing.T) {
		p.Reset()
		_, _ = p.GetOffers(ctx, provider.OfferFilter{GPUType: "A100 40GB"})
		if len(p.GetOffersCalls) != 1 {
			t.Errorf("GetOffersCalls = %d, want 1", len(p.GetOffersCalls))
		}
		if p.GetOffersCalls[0].Filter.GPUType != "A100 40GB" {
			t.Errorf("Filter.GPUType = %q, want A100 40GB", p.GetOffersCalls[0].Filter.GPUType)
		}
	})
}

func TestProvider_GetOffers_Error(t *testing.T) {
	expectedErr := errors.New("network error")
	p := New(WithGetOffersError(expectedErr))

	_, err := p.GetOffers(context.Background(), provider.OfferFilter{})
	if err != expectedErr {
		t.Errorf("GetOffers() error = %v, want %v", err, expectedErr)
	}
}

func TestProvider_GetOffers_Delay(t *testing.T) {
	p := New(WithGetOffersDelay(50 * time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := p.GetOffers(ctx, provider.OfferFilter{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("GetOffers() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestProvider_CreateInstance(t *testing.T) {
	spotPrice := 0.50
	offers := []provider.Offer{
		{OfferID: "offer1", GPU: "A100 40GB", VRAM: 40, Region: "EU-West", OnDemandPrice: 1.00, SpotPrice: &spotPrice, Available: true},
	}

	p := New(WithOffers(offers))
	ctx := context.Background()

	t.Run("create on-demand instance", func(t *testing.T) {
		instance, err := p.CreateInstance(ctx, provider.CreateRequest{
			OfferID:   "offer1",
			Spot:      false,
			CloudInit: "#!/bin/bash\necho hello",
		})
		if err != nil {
			t.Fatalf("CreateInstance() error = %v", err)
		}
		if instance.Status != provider.InstanceStatusRunning {
			t.Errorf("Status = %v, want Running", instance.Status)
		}
		if instance.GPU != "A100 40GB" {
			t.Errorf("GPU = %q, want A100 40GB", instance.GPU)
		}
		if instance.HourlyRate != 1.00 {
			t.Errorf("HourlyRate = %f, want 1.00", instance.HourlyRate)
		}
	})

	t.Run("create spot instance", func(t *testing.T) {
		instance, err := p.CreateInstance(ctx, provider.CreateRequest{
			OfferID: "offer1",
			Spot:    true,
		})
		if err != nil {
			t.Fatalf("CreateInstance() error = %v", err)
		}
		if instance.HourlyRate != 0.50 {
			t.Errorf("HourlyRate = %f, want 0.50", instance.HourlyRate)
		}
		if !instance.Spot {
			t.Error("Spot = false, want true")
		}
	})

	t.Run("offer not found", func(t *testing.T) {
		_, err := p.CreateInstance(ctx, provider.CreateRequest{
			OfferID: "nonexistent",
		})
		if !errors.Is(err, provider.ErrOfferNotFound) {
			t.Errorf("CreateInstance() error = %v, want ErrOfferNotFound", err)
		}
	})
}

func TestProvider_CreateInstance_SpotNotAvailable(t *testing.T) {
	offers := []provider.Offer{
		{OfferID: "offer1", GPU: "A100 40GB", OnDemandPrice: 1.00, Available: true}, // no spot price
	}

	p := New(WithOffers(offers))

	_, err := p.CreateInstance(context.Background(), provider.CreateRequest{
		OfferID: "offer1",
		Spot:    true,
	})
	if !errors.Is(err, provider.ErrSpotNotAvailable) {
		t.Errorf("CreateInstance() error = %v, want ErrSpotNotAvailable", err)
	}
}

func TestProvider_GetInstance(t *testing.T) {
	spotPrice := 0.50
	offers := []provider.Offer{
		{OfferID: "offer1", GPU: "A100 40GB", VRAM: 40, Region: "EU-West", OnDemandPrice: 1.00, SpotPrice: &spotPrice, Available: true},
	}

	p := New(WithOffers(offers))
	ctx := context.Background()

	// Create an instance first
	created, _ := p.CreateInstance(ctx, provider.CreateRequest{OfferID: "offer1"})

	t.Run("get existing instance", func(t *testing.T) {
		instance, err := p.GetInstance(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetInstance() error = %v", err)
		}
		if instance.ID != created.ID {
			t.Errorf("ID = %q, want %q", instance.ID, created.ID)
		}
	})

	t.Run("instance not found", func(t *testing.T) {
		_, err := p.GetInstance(ctx, "nonexistent")
		if !errors.Is(err, provider.ErrInstanceNotFound) {
			t.Errorf("GetInstance() error = %v, want ErrInstanceNotFound", err)
		}
	})
}

func TestProvider_TerminateInstance(t *testing.T) {
	spotPrice := 0.50
	offers := []provider.Offer{
		{OfferID: "offer1", GPU: "A100 40GB", VRAM: 40, OnDemandPrice: 1.00, SpotPrice: &spotPrice, Available: true},
	}

	p := New(WithOffers(offers))
	ctx := context.Background()

	// Create an instance
	created, _ := p.CreateInstance(ctx, provider.CreateRequest{OfferID: "offer1"})

	t.Run("terminate existing instance", func(t *testing.T) {
		err := p.TerminateInstance(ctx, created.ID)
		if err != nil {
			t.Fatalf("TerminateInstance() error = %v", err)
		}

		// Verify status changed
		instance, _ := p.GetInstance(ctx, created.ID)
		if instance.Status != provider.InstanceStatusTerminated {
			t.Errorf("Status = %v, want Terminated", instance.Status)
		}
	})

	t.Run("terminate idempotent for nonexistent", func(t *testing.T) {
		err := p.TerminateInstance(ctx, "nonexistent")
		if err != nil {
			t.Errorf("TerminateInstance() error = %v, want nil (idempotent)", err)
		}
	})
}

func TestProvider_GetBillingStatus(t *testing.T) {
	spotPrice := 0.50
	offers := []provider.Offer{
		{OfferID: "offer1", GPU: "A100 40GB", OnDemandPrice: 1.00, SpotPrice: &spotPrice, Available: true},
	}

	p := New(WithOffers(offers))
	ctx := context.Background()

	// Create an instance
	created, _ := p.CreateInstance(ctx, provider.CreateRequest{OfferID: "offer1"})

	t.Run("running instance is billing active", func(t *testing.T) {
		status, err := p.GetBillingStatus(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetBillingStatus() error = %v", err)
		}
		if status != provider.BillingActive {
			t.Errorf("BillingStatus = %v, want Active", status)
		}
	})

	t.Run("terminated instance is billing stopped", func(t *testing.T) {
		_ = p.TerminateInstance(ctx, created.ID)
		status, err := p.GetBillingStatus(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetBillingStatus() error = %v", err)
		}
		if status != provider.BillingStopped {
			t.Errorf("BillingStatus = %v, want Stopped", status)
		}
	})

	t.Run("nonexistent instance is billing stopped", func(t *testing.T) {
		status, err := p.GetBillingStatus(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("GetBillingStatus() error = %v", err)
		}
		if status != provider.BillingStopped {
			t.Errorf("BillingStatus = %v, want Stopped", status)
		}
	})
}

func TestProvider_GetBillingStatus_Override(t *testing.T) {
	p := New(WithBillingStatusOverride(provider.BillingUnknown))

	status, err := p.GetBillingStatus(context.Background(), "any")
	if err != nil {
		t.Fatalf("GetBillingStatus() error = %v", err)
	}
	if status != provider.BillingUnknown {
		t.Errorf("BillingStatus = %v, want Unknown (override)", status)
	}
}

func TestProvider_ValidateAPIKey(t *testing.T) {
	t.Run("default success", func(t *testing.T) {
		p := New()
		info, err := p.ValidateAPIKey(context.Background())
		if err != nil {
			t.Fatalf("ValidateAPIKey() error = %v", err)
		}
		if !info.Valid {
			t.Error("Valid = false, want true")
		}
	})

	t.Run("with custom account info", func(t *testing.T) {
		balance := 100.0
		p := New(WithAccountInfo(&provider.AccountInfo{
			Valid:           true,
			Email:           "custom@example.com",
			Balance:         &balance,
			BalanceCurrency: "USD",
		}))

		info, err := p.ValidateAPIKey(context.Background())
		if err != nil {
			t.Fatalf("ValidateAPIKey() error = %v", err)
		}
		if info.Email != "custom@example.com" {
			t.Errorf("Email = %q, want custom@example.com", info.Email)
		}
		if info.Balance == nil || *info.Balance != 100.0 {
			t.Errorf("Balance = %v, want 100.0", info.Balance)
		}
	})

	t.Run("with error", func(t *testing.T) {
		p := New(WithValidateAPIKeyError(provider.ErrAuthenticationFailed))
		_, err := p.ValidateAPIKey(context.Background())
		if !errors.Is(err, provider.ErrAuthenticationFailed) {
			t.Errorf("ValidateAPIKey() error = %v, want ErrAuthenticationFailed", err)
		}
	})

	t.Run("call tracking", func(t *testing.T) {
		p := New()
		_, _ = p.ValidateAPIKey(context.Background())
		_, _ = p.ValidateAPIKey(context.Background())
		if p.ValidateAPIKeyCalls != 2 {
			t.Errorf("ValidateAPIKeyCalls = %d, want 2", p.ValidateAPIKeyCalls)
		}
	})
}

func TestProvider_SetError(t *testing.T) {
	p := New()
	ctx := context.Background()
	testErr := errors.New("injected error")

	p.SetError("GetOffers", testErr)
	_, err := p.GetOffers(ctx, provider.OfferFilter{})
	if err != testErr {
		t.Errorf("GetOffers() error = %v, want injected error", err)
	}

	p.SetError("GetOffers", nil) // clear error
	_, err = p.GetOffers(ctx, provider.OfferFilter{})
	if err != nil {
		t.Errorf("GetOffers() error = %v after clearing, want nil", err)
	}
}

func TestProvider_SetInstanceStatus(t *testing.T) {
	spotPrice := 0.50
	offers := []provider.Offer{
		{OfferID: "offer1", GPU: "A100 40GB", OnDemandPrice: 1.00, SpotPrice: &spotPrice, Available: true},
	}

	p := New(WithOffers(offers))
	ctx := context.Background()

	// Create an instance
	created, _ := p.CreateInstance(ctx, provider.CreateRequest{OfferID: "offer1"})

	// Change status
	err := p.SetInstanceStatus(created.ID, provider.InstanceStatusError)
	if err != nil {
		t.Fatalf("SetInstanceStatus() error = %v", err)
	}

	// Verify
	instance, _ := p.GetInstance(ctx, created.ID)
	if instance.Status != provider.InstanceStatusError {
		t.Errorf("Status = %v, want Error", instance.Status)
	}
}

func TestProvider_AddInstance(t *testing.T) {
	p := New()
	ctx := context.Background()

	// Add instance directly
	p.AddInstance(&provider.Instance{
		ID:         "external-123",
		Provider:   "mock",
		Status:     provider.InstanceStatusRunning,
		GPU:        "H100 80GB",
		HourlyRate: 3.50,
	})

	// Retrieve it
	instance, err := p.GetInstance(ctx, "external-123")
	if err != nil {
		t.Fatalf("GetInstance() error = %v", err)
	}
	if instance.GPU != "H100 80GB" {
		t.Errorf("GPU = %q, want H100 80GB", instance.GPU)
	}
}

func TestProvider_Reset(t *testing.T) {
	spotPrice := 0.50
	offers := []provider.Offer{
		{OfferID: "offer1", GPU: "A100 40GB", OnDemandPrice: 1.00, SpotPrice: &spotPrice, Available: true},
	}

	p := New(WithOffers(offers))
	ctx := context.Background()

	// Do some operations
	_, _ = p.GetOffers(ctx, provider.OfferFilter{})
	_, _ = p.CreateInstance(ctx, provider.CreateRequest{OfferID: "offer1"})
	p.SetError("GetOffers", errors.New("error"))
	p.SetDelay("GetInstance", time.Second)

	// Reset
	p.Reset()

	// Verify state cleared
	if len(p.GetOffersCalls) != 0 {
		t.Error("GetOffersCalls not cleared")
	}
	if len(p.CreateInstanceCalls) != 0 {
		t.Error("CreateInstanceCalls not cleared")
	}

	// Verify errors cleared
	_, err := p.GetOffers(ctx, provider.OfferFilter{})
	if err != nil {
		t.Error("Error not cleared after reset")
	}
}

func TestProvider_ImplementsInterface(t *testing.T) {
	// This test verifies at compile time that Provider implements provider.Provider
	var _ provider.Provider = (*Provider)(nil)
}
