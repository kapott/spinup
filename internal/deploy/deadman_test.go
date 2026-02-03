package deploy

import (
	"strings"
	"testing"
	"time"
)

func TestNewDeadmanConfig(t *testing.T) {
	cfg := NewDeadmanConfig()

	if cfg.TimeoutSeconds != 36000 {
		t.Errorf("expected default timeout 36000, got %d", cfg.TimeoutSeconds)
	}
	if cfg.HeartbeatFile != DefaultHeartbeatFile {
		t.Errorf("expected default heartbeat file %s, got %s", DefaultHeartbeatFile, cfg.HeartbeatFile)
	}
	if cfg.CheckIntervalSeconds != 60 {
		t.Errorf("expected default check interval 60, got %d", cfg.CheckIntervalSeconds)
	}
}

func TestNewDeadmanConfigWithTimeout(t *testing.T) {
	cfg := NewDeadmanConfigWithTimeout(5 * time.Hour)

	if cfg.TimeoutSeconds != 18000 {
		t.Errorf("expected timeout 18000, got %d", cfg.TimeoutSeconds)
	}
}

func TestDeadmanConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *DeadmanConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			cfg:     NewDeadmanConfig(),
			wantErr: false,
		},
		{
			name: "timeout too short",
			cfg: &DeadmanConfig{
				TimeoutSeconds:       1800, // 30 minutes, less than 1 hour minimum
				HeartbeatFile:        DefaultHeartbeatFile,
				CheckIntervalSeconds: 60,
			},
			wantErr: true,
			errMsg:  "at least",
		},
		{
			name: "timeout too long",
			cfg: &DeadmanConfig{
				TimeoutSeconds:       300000, // > 72 hours
				HeartbeatFile:        DefaultHeartbeatFile,
				CheckIntervalSeconds: 60,
			},
			wantErr: true,
			errMsg:  "exceed",
		},
		{
			name: "negative timeout",
			cfg: &DeadmanConfig{
				TimeoutSeconds:       -1,
				HeartbeatFile:        DefaultHeartbeatFile,
				CheckIntervalSeconds: 60,
			},
			wantErr: true,
			errMsg:  "positive",
		},
		{
			name: "empty heartbeat file",
			cfg: &DeadmanConfig{
				TimeoutSeconds:       36000,
				HeartbeatFile:        "",
				CheckIntervalSeconds: 60,
			},
			wantErr: true,
			errMsg:  "heartbeat file",
		},
		{
			name: "negative check interval",
			cfg: &DeadmanConfig{
				TimeoutSeconds:       36000,
				HeartbeatFile:        DefaultHeartbeatFile,
				CheckIntervalSeconds: -1,
			},
			wantErr: true,
			errMsg:  "check interval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %v", tt.errMsg, err)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestDeadmanConfig_Timeout(t *testing.T) {
	cfg := &DeadmanConfig{TimeoutSeconds: 7200}
	if cfg.Timeout() != 2*time.Hour {
		t.Errorf("expected 2h, got %v", cfg.Timeout())
	}
}

func TestDeadmanConfig_TimeoutHours(t *testing.T) {
	tests := []struct {
		seconds int
		hours   int
	}{
		{3600, 1},
		{7200, 2},
		{36000, 10},
		{3700, 1}, // rounds down
		{0, 0},
	}

	for _, tt := range tests {
		cfg := &DeadmanConfig{TimeoutSeconds: tt.seconds}
		if cfg.TimeoutHours() != tt.hours {
			t.Errorf("TimeoutHours(%d) = %d, want %d", tt.seconds, cfg.TimeoutHours(), tt.hours)
		}
	}
}

func TestDeadmanConfig_RemainingTime(t *testing.T) {
	cfg := &DeadmanConfig{TimeoutSeconds: 3600} // 1 hour

	// Test with recent heartbeat
	recent := time.Now().Add(-30 * time.Minute)
	remaining := cfg.RemainingTime(recent)
	if remaining < 29*time.Minute || remaining > 31*time.Minute {
		t.Errorf("expected ~30m remaining, got %v", remaining)
	}

	// Test with expired heartbeat
	expired := time.Now().Add(-2 * time.Hour)
	remaining = cfg.RemainingTime(expired)
	if remaining != 0 {
		t.Errorf("expected 0 remaining for expired heartbeat, got %v", remaining)
	}

	// Test with zero heartbeat
	remaining = cfg.RemainingTime(time.Time{})
	if remaining != 0 {
		t.Errorf("expected 0 remaining for zero heartbeat, got %v", remaining)
	}
}

func TestDeadmanConfig_IsExpired(t *testing.T) {
	cfg := &DeadmanConfig{TimeoutSeconds: 3600} // 1 hour

	// Not expired
	recent := time.Now().Add(-30 * time.Minute)
	if cfg.IsExpired(recent) {
		t.Error("expected not expired for recent heartbeat")
	}

	// Expired
	expired := time.Now().Add(-2 * time.Hour)
	if !cfg.IsExpired(expired) {
		t.Error("expected expired for old heartbeat")
	}

	// Zero time (expired)
	if !cfg.IsExpired(time.Time{}) {
		t.Error("expected expired for zero heartbeat")
	}
}

func TestGetTerminationInfo(t *testing.T) {
	providers := []struct {
		name       string
		instanceID string
		apiKey     string
		wantMethod string
		wantURL    string
	}{
		{"vast", "123", "key", "DELETE", "console.vast.ai"},
		{"lambda", "456", "key", "POST", "lambdalabs.com"},
		{"runpod", "789", "key", "POST", "runpod.io/graphql"},
		{"coreweave", "abc", "key", "DELETE", "coreweave.com"},
		{"paperspace", "def", "key", "POST", "paperspace.io"},
	}

	for _, tt := range providers {
		t.Run(tt.name, func(t *testing.T) {
			info, err := GetTerminationInfo(tt.name, tt.instanceID, tt.apiKey)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.HTTPMethod != tt.wantMethod {
				t.Errorf("expected method %s, got %s", tt.wantMethod, info.HTTPMethod)
			}
			if !strings.Contains(info.APIURL, tt.wantURL) {
				t.Errorf("expected URL containing %s, got %s", tt.wantURL, info.APIURL)
			}
			if info.InstanceID != tt.instanceID {
				t.Errorf("expected instance ID %s, got %s", tt.instanceID, info.InstanceID)
			}
		})
	}
}

func TestGetTerminationInfo_UnknownProvider(t *testing.T) {
	_, err := GetTerminationInfo("unknown", "123", "key")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestGetTerminationInfo_CaseInsensitive(t *testing.T) {
	_, err := GetTerminationInfo("VAST", "123", "key")
	if err != nil {
		t.Errorf("expected case-insensitive lookup, got error: %v", err)
	}
}

func TestProviderTerminationInfo_GenerateCurlCommand(t *testing.T) {
	tests := []struct {
		name        string
		info        *ProviderTerminationInfo
		wantContain []string
	}{
		{
			name: "DELETE request",
			info: &ProviderTerminationInfo{
				APIURL:     "https://api.example.com/instance/123",
				HTTPMethod: "DELETE",
				AuthHeader: "Authorization",
				AuthValue:  "Bearer token",
			},
			wantContain: []string{"curl -X DELETE", "https://api.example.com/instance/123", "Authorization: Bearer token"},
		},
		{
			name: "POST with body",
			info: &ProviderTerminationInfo{
				APIURL:      "https://api.example.com/terminate",
				HTTPMethod:  "POST",
				AuthHeader:  "Authorization",
				AuthValue:   "Bearer token",
				ContentType: "application/json",
				Body:        `{"id":"123"}`,
			},
			wantContain: []string{"curl -X POST", "Content-Type: application/json", `{"id":"123"}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.info.GenerateCurlCommand()
			for _, want := range tt.wantContain {
				if !strings.Contains(cmd, want) {
					t.Errorf("expected curl command to contain %q, got %s", want, cmd)
				}
			}
		})
	}
}

func TestValidateProvider(t *testing.T) {
	// Valid providers
	for _, p := range []string{"vast", "lambda", "runpod", "coreweave", "paperspace", "VAST", "Lambda"} {
		if err := ValidateProvider(p); err != nil {
			t.Errorf("ValidateProvider(%s) returned error: %v", p, err)
		}
	}

	// Invalid provider
	if err := ValidateProvider("unknown"); err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()
	expected := []string{"vast", "lambda", "runpod", "coreweave", "paperspace"}

	if len(providers) != len(expected) {
		t.Errorf("expected %d providers, got %d", len(expected), len(providers))
	}

	for _, e := range expected {
		found := false
		for _, p := range providers {
			if p == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected provider %s in list", e)
		}
	}
}

func TestNewDeadmanStatus(t *testing.T) {
	cfg := &DeadmanConfig{TimeoutSeconds: 3600}
	heartbeat := time.Now().Add(-30 * time.Minute)

	status := NewDeadmanStatus(cfg, heartbeat)

	if !status.Active {
		t.Error("expected status to be active")
	}
	if status.TimeoutSeconds != 3600 {
		t.Errorf("expected timeout 3600, got %d", status.TimeoutSeconds)
	}
	// Remaining should be approximately 30 minutes (1800 seconds)
	if status.RemainingSeconds < 1700 || status.RemainingSeconds > 1900 {
		t.Errorf("expected ~1800 remaining seconds, got %d", status.RemainingSeconds)
	}
}

func TestNewDeadmanStatus_NilConfig(t *testing.T) {
	status := NewDeadmanStatus(nil, time.Now())
	if status.Active {
		t.Error("expected status to be inactive when config is nil")
	}
}

func TestDeadmanStatus_FormatRemaining(t *testing.T) {
	tests := []struct {
		status   *DeadmanStatus
		wantPart string
	}{
		{
			status:   &DeadmanStatus{Active: false},
			wantPart: "inactive",
		},
		{
			status:   &DeadmanStatus{Active: true, RemainingSeconds: 0},
			wantPart: "expired",
		},
		{
			status:   &DeadmanStatus{Active: true, RemainingSeconds: 7200},
			wantPart: "2h",
		},
		{
			status:   &DeadmanStatus{Active: true, RemainingSeconds: 600},
			wantPart: "10m",
		},
	}

	for _, tt := range tests {
		result := tt.status.FormatRemaining()
		if !strings.Contains(result, tt.wantPart) {
			t.Errorf("FormatRemaining() = %q, want containing %q", result, tt.wantPart)
		}
	}
}

func TestDeadmanStatus_IsExpired(t *testing.T) {
	// Not expired
	status := &DeadmanStatus{Active: true, RemainingSeconds: 100}
	if status.IsExpired() {
		t.Error("expected not expired")
	}

	// Expired
	status = &DeadmanStatus{Active: true, RemainingSeconds: 0}
	if !status.IsExpired() {
		t.Error("expected expired")
	}

	// Inactive (not expired, just not active)
	status = &DeadmanStatus{Active: false, RemainingSeconds: 0}
	if status.IsExpired() {
		t.Error("expected not expired when inactive")
	}
}

func TestDeadmanStatus_IsWarning(t *testing.T) {
	// Not warning (plenty of time)
	status := &DeadmanStatus{Active: true, RemainingSeconds: 7200}
	if status.IsWarning() {
		t.Error("expected not warning with 2h remaining")
	}

	// Warning (under 1 hour)
	status = &DeadmanStatus{Active: true, RemainingSeconds: 1800}
	if !status.IsWarning() {
		t.Error("expected warning with 30m remaining")
	}

	// Expired (not warning, it's past warning)
	status = &DeadmanStatus{Active: true, RemainingSeconds: 0}
	if status.IsWarning() {
		t.Error("expected not warning when expired")
	}

	// Inactive
	status = &DeadmanStatus{Active: false, RemainingSeconds: 1800}
	if status.IsWarning() {
		t.Error("expected not warning when inactive")
	}
}

func TestDeadmanConstants(t *testing.T) {
	if DefaultDeadmanTimeout != 10*time.Hour {
		t.Errorf("expected DefaultDeadmanTimeout to be 10h, got %v", DefaultDeadmanTimeout)
	}
	if DefaultHeartbeatFile != "/tmp/spinup-heartbeat" {
		t.Errorf("expected DefaultHeartbeatFile to be /tmp/spinup-heartbeat, got %s", DefaultHeartbeatFile)
	}
	if DefaultCheckInterval != 60*time.Second {
		t.Errorf("expected DefaultCheckInterval to be 60s, got %v", DefaultCheckInterval)
	}
	if MinDeadmanTimeout != 1*time.Hour {
		t.Errorf("expected MinDeadmanTimeout to be 1h, got %v", MinDeadmanTimeout)
	}
	if MaxDeadmanTimeout != 72*time.Hour {
		t.Errorf("expected MaxDeadmanTimeout to be 72h, got %v", MaxDeadmanTimeout)
	}
}

func TestGetTerminationInfo_VastDetails(t *testing.T) {
	info, _ := GetTerminationInfo("vast", "12345", "my-api-key")

	if info.APIURL != "https://console.vast.ai/api/v0/instances/12345/" {
		t.Errorf("unexpected Vast.ai API URL: %s", info.APIURL)
	}
	if info.HTTPMethod != "DELETE" {
		t.Errorf("expected DELETE method for Vast.ai, got %s", info.HTTPMethod)
	}
	if info.AuthValue != "Bearer my-api-key" {
		t.Errorf("expected Bearer auth for Vast.ai, got %s", info.AuthValue)
	}
}

func TestGetTerminationInfo_LambdaDetails(t *testing.T) {
	info, _ := GetTerminationInfo("lambda", "12345", "my-api-key")

	if !strings.Contains(info.APIURL, "instance-operations/terminate") {
		t.Errorf("unexpected Lambda API URL: %s", info.APIURL)
	}
	if info.ContentType != "application/json" {
		t.Errorf("expected application/json for Lambda, got %s", info.ContentType)
	}
	if !strings.Contains(info.Body, "12345") {
		t.Errorf("expected instance ID in body, got %s", info.Body)
	}
}

func TestGetTerminationInfo_RunPodDetails(t *testing.T) {
	info, _ := GetTerminationInfo("runpod", "12345", "my-api-key")

	if !strings.Contains(info.APIURL, "graphql") {
		t.Errorf("expected GraphQL URL for RunPod, got %s", info.APIURL)
	}
	if !strings.Contains(info.Body, "podTerminate") {
		t.Errorf("expected podTerminate mutation in body, got %s", info.Body)
	}
	if !strings.Contains(info.Body, "12345") {
		t.Errorf("expected instance ID in body, got %s", info.Body)
	}
}

func TestGetTerminationInfo_PaperspaceDetails(t *testing.T) {
	info, _ := GetTerminationInfo("paperspace", "12345", "my-api-key")

	if !strings.Contains(info.APIURL, "destroyMachine") {
		t.Errorf("unexpected Paperspace API URL: %s", info.APIURL)
	}
	if info.AuthHeader != "x-api-key" {
		t.Errorf("expected x-api-key header for Paperspace, got %s", info.AuthHeader)
	}
	if info.AuthValue != "my-api-key" {
		t.Errorf("expected direct API key for Paperspace auth, got %s", info.AuthValue)
	}
}
