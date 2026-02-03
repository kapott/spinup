package deploy

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tmeurs/spinup/internal/config"
	"github.com/tmeurs/spinup/internal/provider"
)

func TestDefaultDeployConfig(t *testing.T) {
	cfg := DefaultDeployConfig()

	if cfg == nil {
		t.Fatal("DefaultDeployConfig returned nil")
	}

	if !cfg.PreferSpot {
		t.Error("expected PreferSpot to be true by default")
	}

	if cfg.DeadmanTimeoutHours != 10 {
		t.Errorf("expected DeadmanTimeoutHours to be 10, got %d", cfg.DeadmanTimeoutHours)
	}

	if cfg.BootTimeout != 5*time.Minute {
		t.Errorf("expected BootTimeout to be 5m, got %v", cfg.BootTimeout)
	}

	if cfg.ModelPullTimeout != 15*time.Minute {
		t.Errorf("expected ModelPullTimeout to be 15m, got %v", cfg.ModelPullTimeout)
	}

	if cfg.TunnelTimeout != 2*time.Minute {
		t.Errorf("expected TunnelTimeout to be 2m, got %v", cfg.TunnelTimeout)
	}

	if cfg.HealthCheckTimeout != 30*time.Second {
		t.Errorf("expected HealthCheckTimeout to be 30s, got %v", cfg.HealthCheckTimeout)
	}

	if cfg.DiskSizeGB != 100 {
		t.Errorf("expected DiskSizeGB to be 100, got %d", cfg.DiskSizeGB)
	}
}

func TestDeployConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *DeployConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty model",
			config:  &DeployConfig{},
			wantErr: true,
			errMsg:  "model name is required",
		},
		{
			name: "invalid model",
			config: &DeployConfig{
				Model: "nonexistent-model:123b",
			},
			wantErr: true,
			errMsg:  "invalid model",
		},
		{
			name: "valid model with low deadman timeout",
			config: &DeployConfig{
				Model:               "qwen2.5-coder:32b",
				DeadmanTimeoutHours: 0,
			},
			wantErr: true,
			errMsg:  "deadman timeout must be at least 1 hour",
		},
		{
			name: "valid model with high deadman timeout",
			config: &DeployConfig{
				Model:               "qwen2.5-coder:32b",
				DeadmanTimeoutHours: 100,
			},
			wantErr: true,
			errMsg:  "deadman timeout cannot exceed 72 hours",
		},
		{
			name: "valid model with low disk size",
			config: &DeployConfig{
				Model:               "qwen2.5-coder:32b",
				DeadmanTimeoutHours: 10,
				DiskSizeGB:          30,
			},
			wantErr: true,
			errMsg:  "disk size must be at least 50GB",
		},
		{
			name: "valid config",
			config: &DeployConfig{
				Model:               "qwen2.5-coder:32b",
				DeadmanTimeoutHours: 10,
				DiskSizeGB:          100,
			},
			wantErr: false,
		},
		{
			name: "valid config with minimum values",
			config: &DeployConfig{
				Model:               "qwen2.5-coder:7b",
				DeadmanTimeoutHours: 1,
				DiskSizeGB:          50,
			},
			wantErr: false,
		},
		{
			name: "valid config with maximum values",
			config: &DeployConfig{
				Model:               "qwen2.5-coder:72b",
				DeadmanTimeoutHours: 72,
				DiskSizeGB:          500,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestDeployStep_String(t *testing.T) {
	tests := []struct {
		step DeployStep
		want string
	}{
		{StepFetchPrices, "Fetching prices from providers"},
		{StepSelectOffer, "Selecting best option"},
		{StepCreateInstance, "Creating instance"},
		{StepWaitBoot, "Waiting for instance to boot"},
		{StepConfigureWireGuard, "Configuring WireGuard tunnel"},
		{StepInstallModel, "Installing Ollama and pulling model"},
		{StepConfigureDeadman, "Configuring deadman switch"},
		{StepVerifyHealth, "Verifying service health"},
		{DeployStep(0), "Unknown step"},
		{DeployStep(100), "Unknown step"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.step.String(); got != tt.want {
				t.Errorf("DeployStep.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTotalDeploySteps(t *testing.T) {
	if TotalDeploySteps != 8 {
		t.Errorf("TotalDeploySteps = %d, want 8", TotalDeploySteps)
	}
}

func TestDeployResult_Duration(t *testing.T) {
	result := &DeployResult{
		StartedAt:   time.Now().Add(-5 * time.Minute),
		CompletedAt: time.Now(),
	}

	duration := result.Duration()
	if duration < 4*time.Minute || duration > 6*time.Minute {
		t.Errorf("expected duration around 5 minutes, got %v", duration)
	}
}

func TestNewDeployer_NilConfig(t *testing.T) {
	_, err := NewDeployer(nil, nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestNewDeployer_NilDeployConfig(t *testing.T) {
	// This would need a valid config.Config, so we test the validation path
	// by providing invalid deploy config
	_, err := NewDeployer(nil, &DeployConfig{})
	if err == nil {
		t.Error("expected error")
	}
}

func TestEffectivePrice(t *testing.T) {
	spotPrice := 0.65
	onDemandPrice := 0.95

	tests := []struct {
		name       string
		preferSpot bool
		spotPrice  *float64
		onDemand   float64
		want       float64
	}{
		{
			name:       "prefer spot with spot available",
			preferSpot: true,
			spotPrice:  &spotPrice,
			onDemand:   onDemandPrice,
			want:       spotPrice,
		},
		{
			name:       "prefer spot but no spot available",
			preferSpot: true,
			spotPrice:  nil,
			onDemand:   onDemandPrice,
			want:       onDemandPrice,
		},
		{
			name:       "prefer on-demand with spot available",
			preferSpot: false,
			spotPrice:  &spotPrice,
			onDemand:   onDemandPrice,
			want:       onDemandPrice,
		},
		{
			name:       "prefer on-demand no spot available",
			preferSpot: false,
			spotPrice:  nil,
			onDemand:   onDemandPrice,
			want:       onDemandPrice,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Deployer{
				deployCfg: &DeployConfig{
					PreferSpot: tt.preferSpot,
				},
			}
			offer := &provider.Offer{
				SpotPrice:     tt.spotPrice,
				OnDemandPrice: tt.onDemand,
			}
			got := d.effectivePrice(offer)
			if got != tt.want {
				t.Errorf("effectivePrice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatOfferPrice(t *testing.T) {
	spotPrice := 0.65
	onDemandPrice := 0.95

	tests := []struct {
		name       string
		preferSpot bool
		spotPrice  *float64
		onDemand   float64
		want       string
	}{
		{
			name:       "spot price",
			preferSpot: true,
			spotPrice:  &spotPrice,
			onDemand:   onDemandPrice,
			want:       "€0.65/hr spot",
		},
		{
			name:       "on-demand price",
			preferSpot: false,
			spotPrice:  &spotPrice,
			onDemand:   onDemandPrice,
			want:       "€0.95/hr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Deployer{
				deployCfg: &DeployConfig{
					PreferSpot: tt.preferSpot,
				},
			}
			offer := &provider.Offer{
				SpotPrice:     tt.spotPrice,
				OnDemandPrice: tt.onDemand,
			}
			got := d.formatOfferPrice(offer)
			if got != tt.want {
				t.Errorf("formatOfferPrice() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeployProgress(t *testing.T) {
	progress := DeployProgress{
		Step:       StepFetchPrices,
		TotalSteps: TotalDeploySteps,
		Message:    "Test message",
		Detail:     "Test detail",
		Completed:  true,
	}

	if progress.Step != StepFetchPrices {
		t.Errorf("expected step StepFetchPrices, got %v", progress.Step)
	}
	if progress.TotalSteps != 8 {
		t.Errorf("expected 8 total steps, got %d", progress.TotalSteps)
	}
	if progress.Message != "Test message" {
		t.Errorf("expected 'Test message', got %q", progress.Message)
	}
	if progress.Detail != "Test detail" {
		t.Errorf("expected 'Test detail', got %q", progress.Detail)
	}
	if !progress.Completed {
		t.Error("expected Completed to be true")
	}
}

func TestWithProgressCallback(t *testing.T) {
	var called bool
	cb := func(p DeployProgress) {
		called = true
	}

	d := &Deployer{}
	opt := WithProgressCallback(cb)
	opt(d)

	if d.progressCb == nil {
		t.Error("expected progressCb to be set")
	}

	d.progressCb(DeployProgress{})
	if !called {
		t.Error("expected callback to be called")
	}
}

func TestErrorVariables(t *testing.T) {
	if ErrNoCompatibleOffers.Error() != "no compatible GPU offers found" {
		t.Error("ErrNoCompatibleOffers has wrong message")
	}
	if ErrInstanceCreationFailed.Error() != "instance creation failed" {
		t.Error("ErrInstanceCreationFailed has wrong message")
	}
	if ErrBootTimeout.Error() != "instance boot timeout" {
		t.Error("ErrBootTimeout has wrong message")
	}
	if ErrTunnelFailed.Error() != "WireGuard tunnel failed" {
		t.Error("ErrTunnelFailed has wrong message")
	}
	if ErrModelPullFailed.Error() != "model pull failed" {
		t.Error("ErrModelPullFailed has wrong message")
	}
	if ErrHealthCheckFailed.Error() != "health check failed" {
		t.Error("ErrHealthCheckFailed has wrong message")
	}
}

// ============================================================================
// Stop Flow Tests
// ============================================================================

func TestDefaultStopConfig(t *testing.T) {
	cfg := DefaultStopConfig()

	if cfg == nil {
		t.Fatal("DefaultStopConfig returned nil")
	}

	if cfg.MaxRetries != 5 {
		t.Errorf("expected MaxRetries to be 5, got %d", cfg.MaxRetries)
	}

	if cfg.BaseRetryDelay != 2*time.Second {
		t.Errorf("expected BaseRetryDelay to be 2s, got %v", cfg.BaseRetryDelay)
	}

	if cfg.TerminateTimeout != 30*time.Second {
		t.Errorf("expected TerminateTimeout to be 30s, got %v", cfg.TerminateTimeout)
	}

	if cfg.BillingCheckTimeout != 60*time.Second {
		t.Errorf("expected BillingCheckTimeout to be 60s, got %v", cfg.BillingCheckTimeout)
	}
}

func TestStopConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *StopConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &StopConfig{
				MaxRetries:          5,
				BaseRetryDelay:      2 * time.Second,
				TerminateTimeout:    30 * time.Second,
				BillingCheckTimeout: 60 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "max retries too low",
			config: &StopConfig{
				MaxRetries:     0,
				BaseRetryDelay: 2 * time.Second,
			},
			wantErr: true,
			errMsg:  "max retries must be at least 1",
		},
		{
			name: "max retries too high",
			config: &StopConfig{
				MaxRetries:     15,
				BaseRetryDelay: 2 * time.Second,
			},
			wantErr: true,
			errMsg:  "max retries cannot exceed 10",
		},
		{
			name: "base retry delay too short",
			config: &StopConfig{
				MaxRetries:     5,
				BaseRetryDelay: 500 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "base retry delay must be at least 1 second",
		},
		{
			name: "minimum valid config",
			config: &StopConfig{
				MaxRetries:     1,
				BaseRetryDelay: 1 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "maximum valid config",
			config: &StopConfig{
				MaxRetries:     10,
				BaseRetryDelay: 30 * time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestStopStep_String(t *testing.T) {
	tests := []struct {
		step StopStep
		want string
	}{
		{StopStepTerminate, "Terminating instance"},
		{StopStepVerifyBilling, "Verifying billing stopped"},
		{StopStepRemoveTunnel, "Removing WireGuard tunnel"},
		{StopStepClearState, "Cleaning up state"},
		{StopStep(0), "Unknown step"},
		{StopStep(100), "Unknown step"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.step.String(); got != tt.want {
				t.Errorf("StopStep.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTotalStopSteps(t *testing.T) {
	if TotalStopSteps != 4 {
		t.Errorf("TotalStopSteps = %d, want 4", TotalStopSteps)
	}
}

func TestStopResult_Duration(t *testing.T) {
	result := &StopResult{
		StartedAt:   time.Now().Add(-30 * time.Second),
		CompletedAt: time.Now(),
	}

	duration := result.Duration()
	if duration < 29*time.Second || duration > 31*time.Second {
		t.Errorf("expected duration around 30 seconds, got %v", duration)
	}
}

func TestNewStopper_NilConfig(t *testing.T) {
	_, err := NewStopper(nil, nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
	if err.Error() != "config is required" {
		t.Errorf("expected 'config is required', got %q", err.Error())
	}
}

func TestNewStopper_NilStopConfig(t *testing.T) {
	// Create a minimal config
	cfg := &config.Config{
		VastAPIKey: "test-key",
	}

	stopper, err := NewStopper(cfg, nil)
	if err != nil {
		t.Errorf("expected no error with nil stop config, got %v", err)
	}
	if stopper == nil {
		t.Error("expected stopper to be created")
	}
}

func TestNewStopper_InvalidStopConfig(t *testing.T) {
	cfg := &config.Config{
		VastAPIKey: "test-key",
	}
	stopCfg := &StopConfig{
		MaxRetries:     0, // Invalid
		BaseRetryDelay: 2 * time.Second,
	}

	_, err := NewStopper(cfg, stopCfg)
	if err == nil {
		t.Error("expected error for invalid stop config")
	}
}

func TestWithStopProgressCallback(t *testing.T) {
	var called bool
	cb := func(p StopProgress) {
		called = true
	}

	s := &Stopper{}
	opt := WithStopProgressCallback(cb)
	opt(s)

	if s.progressCb == nil {
		t.Error("expected progressCb to be set")
	}

	s.progressCb(StopProgress{})
	if !called {
		t.Error("expected callback to be called")
	}
}

func TestStopProgress(t *testing.T) {
	progress := StopProgress{
		Step:       StopStepTerminate,
		TotalSteps: TotalStopSteps,
		Message:    "Test message",
		Detail:     "Test detail",
		Completed:  true,
		Warning:    false,
	}

	if progress.Step != StopStepTerminate {
		t.Errorf("expected step StopStepTerminate, got %v", progress.Step)
	}
	if progress.TotalSteps != 4 {
		t.Errorf("expected 4 total steps, got %d", progress.TotalSteps)
	}
	if progress.Message != "Test message" {
		t.Errorf("expected 'Test message', got %q", progress.Message)
	}
	if progress.Detail != "Test detail" {
		t.Errorf("expected 'Test detail', got %q", progress.Detail)
	}
	if !progress.Completed {
		t.Error("expected Completed to be true")
	}
	if progress.Warning {
		t.Error("expected Warning to be false")
	}
}

func TestStopper_calculateBackoff(t *testing.T) {
	s := &Stopper{
		stopCfg: &StopConfig{
			BaseRetryDelay: 2 * time.Second,
		},
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 2 * time.Second},   // 2 * 2^0 = 2s
		{2, 4 * time.Second},   // 2 * 2^1 = 4s
		{3, 8 * time.Second},   // 2 * 2^2 = 8s
		{4, 16 * time.Second},  // 2 * 2^3 = 16s
		{5, 32 * time.Second},  // 2 * 2^4 = 32s
		{6, 60 * time.Second},  // 2 * 2^5 = 64s, capped to 60s
		{10, 60 * time.Second}, // Would be huge, capped to 60s
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			got := s.calculateBackoff(tt.attempt)
			if got != tt.want {
				t.Errorf("calculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestStopErrorVariables(t *testing.T) {
	if ErrNoActiveInstance.Error() != "no active instance to stop" {
		t.Error("ErrNoActiveInstance has wrong message")
	}
	if ErrTerminateFailed.Error() != "failed to terminate instance after all retries" {
		t.Error("ErrTerminateFailed has wrong message")
	}
	if ErrBillingNotVerified.Error() != "could not verify billing stopped" {
		t.Error("ErrBillingNotVerified has wrong message")
	}
}

func TestStopResult_Fields(t *testing.T) {
	result := &StopResult{
		InstanceID:                 "test-instance-123",
		Provider:                   "vast",
		BillingVerified:            true,
		ManualVerificationRequired: false,
		ConsoleURL:                 "https://console.vast.ai/",
		SessionCost:                2.93,
		SessionDuration:            4*time.Hour + 28*time.Minute,
		TerminateAttempts:          1,
		BillingCheckAttempts:       2,
	}

	if result.InstanceID != "test-instance-123" {
		t.Errorf("expected instance ID 'test-instance-123', got %q", result.InstanceID)
	}
	if result.Provider != "vast" {
		t.Errorf("expected provider 'vast', got %q", result.Provider)
	}
	if !result.BillingVerified {
		t.Error("expected BillingVerified to be true")
	}
	if result.ManualVerificationRequired {
		t.Error("expected ManualVerificationRequired to be false")
	}
	if result.SessionCost != 2.93 {
		t.Errorf("expected SessionCost 2.93, got %f", result.SessionCost)
	}
	if result.TerminateAttempts != 1 {
		t.Errorf("expected TerminateAttempts 1, got %d", result.TerminateAttempts)
	}
	if result.BillingCheckAttempts != 2 {
		t.Errorf("expected BillingCheckAttempts 2, got %d", result.BillingCheckAttempts)
	}
}

// ============================================================================
// Manual Verification Flow Tests (F029)
// ============================================================================

func TestNewManualVerification(t *testing.T) {
	mv := NewManualVerification("paperspace", "instance-123", "https://console.paperspace.com/")

	if !mv.Required {
		t.Error("expected Required to be true")
	}
	if mv.Provider != "paperspace" {
		t.Errorf("expected provider 'paperspace', got %q", mv.Provider)
	}
	if mv.InstanceID != "instance-123" {
		t.Errorf("expected instance ID 'instance-123', got %q", mv.InstanceID)
	}
	if mv.ConsoleURL != "https://console.paperspace.com/" {
		t.Errorf("expected console URL 'https://console.paperspace.com/', got %q", mv.ConsoleURL)
	}
	if len(mv.Instructions) == 0 {
		t.Error("expected instructions to be populated")
	}
	if mv.WarningMessage == "" {
		t.Error("expected warning message to be populated")
	}
}

func TestManualVerification_PaperspaceInstructions(t *testing.T) {
	mv := NewManualVerification("paperspace", "ps-123", "https://console.paperspace.com/machines")

	// Check instructions are provider-specific
	found := false
	for _, instruction := range mv.Instructions {
		if strings.Contains(instruction, "ps-123") && strings.Contains(instruction, "Terminated") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected instructions to contain instance ID and status verification")
	}

	// Check warning message mentions Paperspace
	if !strings.Contains(mv.WarningMessage, "Paperspace") {
		t.Error("expected warning message to mention Paperspace")
	}
}

func TestManualVerification_LambdaInstructions(t *testing.T) {
	mv := NewManualVerification("lambda", "lambda-456", "https://cloud.lambdalabs.com/")

	// Check instructions mention billing
	foundBilling := false
	for _, instruction := range mv.Instructions {
		if strings.Contains(strings.ToLower(instruction), "billing") {
			foundBilling = true
			break
		}
	}
	if !foundBilling {
		t.Error("expected Lambda instructions to mention billing check")
	}

	// Check warning message mentions Lambda Labs
	if !strings.Contains(mv.WarningMessage, "Lambda Labs") {
		t.Error("expected warning message to mention Lambda Labs")
	}
}

func TestManualVerification_GenericProvider(t *testing.T) {
	mv := NewManualVerification("unknown-provider", "inst-789", "https://example.com/console")

	// Should still work with generic instructions
	if !mv.Required {
		t.Error("expected Required to be true for unknown provider")
	}
	if len(mv.Instructions) == 0 {
		t.Error("expected generic instructions to be populated")
	}
}

func TestManualVerification_FormatManualVerificationText(t *testing.T) {
	mv := NewManualVerification("paperspace", "ps-test-123", "https://console.paperspace.com/")

	text := mv.FormatManualVerificationText()

	// Check key elements are present
	if !strings.Contains(text, "MANUAL VERIFICATION REQUIRED") {
		t.Error("expected text to contain header")
	}
	if !strings.Contains(text, "ps-test-123") {
		t.Error("expected text to contain instance ID")
	}
	if !strings.Contains(text, "Paperspace") {
		t.Error("expected text to mention Paperspace")
	}
	if !strings.Contains(text, "1.") {
		t.Error("expected text to contain numbered instructions")
	}
}

func TestManualVerification_FormatManualVerificationText_NotRequired(t *testing.T) {
	mv := &ManualVerification{Required: false}

	text := mv.FormatManualVerificationText()
	if text != "" {
		t.Errorf("expected empty string for non-required verification, got %q", text)
	}
}

func TestManualVerification_GetLogFields(t *testing.T) {
	mv := NewManualVerification("vast", "vast-instance", "https://console.vast.ai/")

	fields := mv.GetLogFields()

	if fields["provider"] != "vast" {
		t.Errorf("expected provider 'vast', got %v", fields["provider"])
	}
	if fields["instance_id"] != "vast-instance" {
		t.Errorf("expected instance_id 'vast-instance', got %v", fields["instance_id"])
	}
	if fields["console_url"] != "https://console.vast.ai/" {
		t.Errorf("expected console_url 'https://console.vast.ai/', got %v", fields["console_url"])
	}
	if fields["verification"] != "manual_required" {
		t.Errorf("expected verification 'manual_required', got %v", fields["verification"])
	}
}

func TestCapitalizeProvider(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"paperspace", "Paperspace"},
		{"lambda", "Lambda Labs"},
		{"vast", "Vast.ai"},
		{"runpod", "RunPod"},
		{"coreweave", "CoreWeave"},
		{"unknown", "unknown"},
	}

	for _, tc := range testCases {
		result := capitalizeProvider(tc.input)
		if result != tc.expected {
			t.Errorf("capitalizeProvider(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestStopResult_GetManualVerification(t *testing.T) {
	// Test when manual verification is required
	result := &StopResult{
		ManualVerificationRequired: true,
		Provider:                   "paperspace",
		InstanceID:                 "ps-123",
		ConsoleURL:                 "https://console.paperspace.com/",
	}

	mv := result.GetManualVerification()
	if mv == nil {
		t.Fatal("expected ManualVerification to be returned")
	}
	if !mv.Required {
		t.Error("expected Required to be true")
	}
	if mv.Provider != "paperspace" {
		t.Errorf("expected provider 'paperspace', got %q", mv.Provider)
	}
}

func TestStopResult_GetManualVerification_NotRequired(t *testing.T) {
	result := &StopResult{
		ManualVerificationRequired: false,
		BillingVerified:            true,
	}

	mv := result.GetManualVerification()
	if mv != nil {
		t.Error("expected nil when manual verification not required")
	}
}

func TestWithManualVerificationCallback(t *testing.T) {
	cfg := &config.Config{
		VastAPIKey: "test-key",
	}

	var receivedMV *ManualVerification
	callback := func(mv *ManualVerification) {
		receivedMV = mv
	}

	stopper, err := NewStopper(cfg, nil, WithManualVerificationCallback(callback))
	if err != nil {
		t.Fatalf("NewStopper failed: %v", err)
	}

	if stopper.manualVerifyCb == nil {
		t.Error("expected manualVerifyCb to be set")
	}

	// Test the callback is callable and works
	testMV := NewManualVerification("paperspace", "test-id", "https://console.paperspace.com/")
	stopper.manualVerifyCb(testMV)

	if receivedMV == nil {
		t.Error("expected callback to be called and set receivedMV")
	}
	if receivedMV != testMV {
		t.Error("expected receivedMV to equal testMV")
	}
}

func TestManualVerification_AllProviders(t *testing.T) {
	// Test all supported providers generate valid manual verification
	providers := []struct {
		name       string
		consoleURL string
	}{
		{"paperspace", "https://console.paperspace.com/"},
		{"lambda", "https://cloud.lambdalabs.com/"},
		{"vast", "https://console.vast.ai/"},
		{"runpod", "https://www.runpod.io/console/"},
		{"coreweave", "https://cloud.coreweave.com/"},
	}

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			mv := NewManualVerification(p.name, "test-id", p.consoleURL)

			if !mv.Required {
				t.Errorf("%s: expected Required to be true", p.name)
			}
			if mv.Provider != p.name {
				t.Errorf("%s: expected provider %q, got %q", p.name, p.name, mv.Provider)
			}
			if len(mv.Instructions) == 0 {
				t.Errorf("%s: expected instructions to be populated", p.name)
			}
			if mv.WarningMessage == "" {
				t.Errorf("%s: expected warning message to be populated", p.name)
			}

			// Verify FormatManualVerificationText doesn't panic
			text := mv.FormatManualVerificationText()
			if text == "" {
				t.Errorf("%s: expected non-empty formatted text", p.name)
			}

			// Verify GetLogFields doesn't panic
			fields := mv.GetLogFields()
			if len(fields) == 0 {
				t.Errorf("%s: expected non-empty log fields", p.name)
			}
		})
	}
}

func TestGenerateVerificationInstructions(t *testing.T) {
	// Test that instructions are generated for each provider
	testCases := []struct {
		provider   string
		instanceID string
		consoleURL string
		wantLen    int // minimum number of instructions
	}{
		{"paperspace", "ps-123", "https://console.paperspace.com/", 3},
		{"lambda", "lambda-456", "https://cloud.lambdalabs.com/", 3},
		{"unknown", "unknown-789", "https://example.com/", 3},
	}

	for _, tc := range testCases {
		t.Run(tc.provider, func(t *testing.T) {
			instructions := generateVerificationInstructions(tc.provider, tc.instanceID, tc.consoleURL)

			if len(instructions) < tc.wantLen {
				t.Errorf("%s: expected at least %d instructions, got %d", tc.provider, tc.wantLen, len(instructions))
			}

			// Check that console URL is included in instructions
			foundURL := false
			for _, inst := range instructions {
				if strings.Contains(inst, tc.consoleURL) {
					foundURL = true
					break
				}
			}
			if !foundURL {
				t.Errorf("%s: expected console URL in instructions", tc.provider)
			}

			// Check that instance ID is included in instructions
			foundID := false
			for _, inst := range instructions {
				if strings.Contains(inst, tc.instanceID) {
					foundID = true
					break
				}
			}
			if !foundID {
				t.Errorf("%s: expected instance ID in instructions", tc.provider)
			}
		})
	}
}

// ============================================================================
// F050: Critical Alert Callback Tests
// ============================================================================

func TestWithCriticalAlertCallback(t *testing.T) {
	cfg := &config.Config{
		VastAPIKey: "test-key",
	}

	var receivedMessage string
	var receivedErr error
	var receivedContext map[string]interface{}

	callback := func(message string, err error, ctx map[string]interface{}) {
		receivedMessage = message
		receivedErr = err
		receivedContext = ctx
	}

	stopper, err := NewStopper(cfg, nil, WithCriticalAlertCallback(callback))
	if err != nil {
		t.Fatalf("NewStopper failed: %v", err)
	}

	if stopper.criticalAlertCb == nil {
		t.Error("expected criticalAlertCb to be set")
	}

	// Test the callback is callable and works
	testErr := fmt.Errorf("test error")
	testCtx := map[string]interface{}{
		"instance_id": "test-123",
		"provider":    "vast",
	}
	stopper.criticalAlertCb("Test message", testErr, testCtx)

	if receivedMessage != "Test message" {
		t.Errorf("expected message 'Test message', got %q", receivedMessage)
	}
	if receivedErr != testErr {
		t.Errorf("expected error to match, got %v", receivedErr)
	}
	if receivedContext["instance_id"] != "test-123" {
		t.Errorf("expected context instance_id 'test-123', got %v", receivedContext["instance_id"])
	}
	if receivedContext["provider"] != "vast" {
		t.Errorf("expected context provider 'vast', got %v", receivedContext["provider"])
	}
}

func TestCriticalAlertCallbackType(t *testing.T) {
	// Test that the CriticalAlertCallback type signature is correct
	var callback CriticalAlertCallback = func(message string, err error, ctx map[string]interface{}) {
		// Verify parameters are accessible
		_ = message
		_ = err
		_ = ctx["test"]
	}

	// Should compile and be callable
	callback("test", nil, nil)
	callback("test", fmt.Errorf("error"), map[string]interface{}{"key": "value"})
}

func TestStopper_CriticalAlertCallbackNotSet(t *testing.T) {
	// Test that stopper works fine without a critical alert callback
	cfg := &config.Config{
		VastAPIKey: "test-key",
	}

	stopper, err := NewStopper(cfg, nil)
	if err != nil {
		t.Fatalf("NewStopper failed: %v", err)
	}

	if stopper.criticalAlertCb != nil {
		t.Error("expected criticalAlertCb to be nil when not set")
	}
}

func TestStopper_MultipleCriticalAlertCallbacks(t *testing.T) {
	cfg := &config.Config{
		VastAPIKey: "test-key",
	}

	var callCount int
	callback1 := func(message string, err error, ctx map[string]interface{}) {
		callCount++
	}
	callback2 := func(message string, err error, ctx map[string]interface{}) {
		callCount += 10
	}

	// Only the last callback should be set (functional options pattern)
	stopper, err := NewStopper(cfg, nil,
		WithCriticalAlertCallback(callback1),
		WithCriticalAlertCallback(callback2),
	)
	if err != nil {
		t.Fatalf("NewStopper failed: %v", err)
	}

	stopper.criticalAlertCb("test", nil, nil)

	// Only callback2 should have been called (callCount += 10)
	if callCount != 10 {
		t.Errorf("expected callCount to be 10 (only last callback), got %d", callCount)
	}
}
