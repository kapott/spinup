package deploy

import (
	"strings"
	"testing"

	"github.com/tmeurs/continueplz/internal/wireguard"
)

func TestNewCloudInitParams(t *testing.T) {
	params := NewCloudInitParams()

	if params.WireGuard.ListenPort != wireguard.DefaultListenPort {
		t.Errorf("expected default listen port %d, got %d", wireguard.DefaultListenPort, params.WireGuard.ListenPort)
	}
	if params.WireGuard.ServerAddress != wireguard.ServerAddress {
		t.Errorf("expected default server address %s, got %s", wireguard.ServerAddress, params.WireGuard.ServerAddress)
	}
	if params.WireGuard.ClientAllowedIPs != wireguard.ClientAllowedIPs {
		t.Errorf("expected default client allowed IPs %s, got %s", wireguard.ClientAllowedIPs, params.WireGuard.ClientAllowedIPs)
	}
	if params.Deadman.TimeoutSeconds != 36000 {
		t.Errorf("expected default deadman timeout 36000, got %d", params.Deadman.TimeoutSeconds)
	}
}

func TestCloudInitParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  *CloudInitParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid params",
			params: &CloudInitParams{
				WireGuard: WireGuardParams{
					ServerPrivateKey: "test-private-key",
					ClientPublicKey:  "test-public-key",
				},
				Provider: "vast",
				Model:    "qwen2.5-coder:32b",
			},
			wantErr: false,
		},
		{
			name: "missing server private key",
			params: &CloudInitParams{
				WireGuard: WireGuardParams{
					ClientPublicKey: "test-public-key",
				},
				Provider: "vast",
				Model:    "qwen2.5-coder:32b",
			},
			wantErr: true,
			errMsg:  "server private key is required",
		},
		{
			name: "missing client public key",
			params: &CloudInitParams{
				WireGuard: WireGuardParams{
					ServerPrivateKey: "test-private-key",
				},
				Provider: "vast",
				Model:    "qwen2.5-coder:32b",
			},
			wantErr: true,
			errMsg:  "client public key is required",
		},
		{
			name: "missing provider",
			params: &CloudInitParams{
				WireGuard: WireGuardParams{
					ServerPrivateKey: "test-private-key",
					ClientPublicKey:  "test-public-key",
				},
				Model: "qwen2.5-coder:32b",
			},
			wantErr: true,
			errMsg:  "provider is required",
		},
		{
			name: "missing model",
			params: &CloudInitParams{
				WireGuard: WireGuardParams{
					ServerPrivateKey: "test-private-key",
					ClientPublicKey:  "test-public-key",
				},
				Provider: "vast",
			},
			wantErr: true,
			errMsg:  "model is required",
		},
		{
			name: "invalid provider",
			params: &CloudInitParams{
				WireGuard: WireGuardParams{
					ServerPrivateKey: "test-private-key",
					ClientPublicKey:  "test-public-key",
				},
				Provider: "unknown",
				Model:    "qwen2.5-coder:32b",
			},
			wantErr: true,
			errMsg:  "unknown provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %v", tt.errMsg, err)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestGenerateCloudInit(t *testing.T) {
	params := &CloudInitParams{
		WireGuard: WireGuardParams{
			ServerPrivateKey: "test-server-private-key",
			ClientPublicKey:  "test-client-public-key",
			ListenPort:       51820,
			ServerAddress:    "10.13.37.1/24",
			ClientAllowedIPs: "10.13.37.2/32",
		},
		Deadman: DeadmanParams{
			TimeoutSeconds: 36000,
		},
		Provider:   "vast",
		InstanceID: "12345678",
		Model:      "qwen2.5-coder:32b",
		APIKey:     "test-api-key",
	}

	result, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that the output starts with #cloud-config
	if !strings.HasPrefix(result, "#cloud-config") {
		t.Error("expected output to start with #cloud-config")
	}

	// Check that WireGuard config is included
	if !strings.Contains(result, "PrivateKey = test-server-private-key") {
		t.Error("expected WireGuard server private key in output")
	}
	if !strings.Contains(result, "PublicKey = test-client-public-key") {
		t.Error("expected WireGuard client public key in output")
	}
	if !strings.Contains(result, "Address = 10.13.37.1/24") {
		t.Error("expected WireGuard server address in output")
	}
	if !strings.Contains(result, "ListenPort = 51820") {
		t.Error("expected WireGuard listen port in output")
	}

	// Check deadman configuration
	if !strings.Contains(result, "TIMEOUT_SECONDS=36000") {
		t.Error("expected deadman timeout in output")
	}
	if !strings.Contains(result, "PROVIDER=vast") {
		t.Error("expected provider in output")
	}
	if !strings.Contains(result, "INSTANCE_ID=12345678") {
		t.Error("expected instance ID in output")
	}

	// Check model pull command
	if !strings.Contains(result, "docker exec ollama ollama pull qwen2.5-coder:32b") {
		t.Error("expected model pull command in output")
	}

	// Check Ollama binding to WireGuard IP
	if !strings.Contains(result, "-p 10.13.37.1:11434:11434") {
		t.Error("expected Ollama to be bound to WireGuard IP")
	}

	// Check firewall rules
	if !strings.Contains(result, "ufw allow 51820/udp") {
		t.Error("expected WireGuard firewall rule in output")
	}
	if !strings.Contains(result, "ufw allow in on wg0 to any port 11434") {
		t.Error("expected Ollama firewall rule in output")
	}
	if !strings.Contains(result, "ufw allow in on wg0 to any port 22") {
		t.Error("expected SSH firewall rule in output")
	}

	// Check ready signal
	if !strings.Contains(result, "touch /tmp/continueplz-ready") {
		t.Error("expected ready signal in output")
	}
}

func TestGenerateCloudInit_NilParams(t *testing.T) {
	_, err := GenerateCloudInit(nil)
	if err == nil {
		t.Error("expected error for nil params")
	}
}

func TestGenerateCloudInit_InvalidParams(t *testing.T) {
	params := &CloudInitParams{
		WireGuard: WireGuardParams{
			// Missing required fields
		},
	}

	_, err := GenerateCloudInit(params)
	if err == nil {
		t.Error("expected error for invalid params")
	}
}

func TestGenerateCloudInit_AllProviders(t *testing.T) {
	providers := []string{"vast", "lambda", "runpod", "coreweave", "paperspace"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			params := &CloudInitParams{
				WireGuard: WireGuardParams{
					ServerPrivateKey: "test-server-private-key",
					ClientPublicKey:  "test-client-public-key",
				},
				Deadman: DeadmanParams{
					TimeoutSeconds: 36000,
				},
				Provider:   provider,
				InstanceID: "test-instance",
				Model:      "qwen2.5-coder:32b",
				APIKey:     "test-api-key",
			}

			result, err := GenerateCloudInit(params)
			if err != nil {
				t.Fatalf("unexpected error for provider %s: %v", provider, err)
			}

			// Verify provider-specific termination command is present
			if !strings.Contains(result, "Deadman triggered") {
				t.Errorf("expected deadman trigger message for provider %s", provider)
			}

			// Verify the output is valid YAML (starts with #cloud-config)
			if !strings.HasPrefix(result, "#cloud-config") {
				t.Errorf("expected output to start with #cloud-config for provider %s", provider)
			}
		})
	}
}

func TestGenerateCloudInit_VastSpecific(t *testing.T) {
	params := &CloudInitParams{
		WireGuard: WireGuardParams{
			ServerPrivateKey: "test-key",
			ClientPublicKey:  "test-key",
		},
		Provider:   "vast",
		InstanceID: "12345",
		Model:      "test-model",
		APIKey:     "vast-api-key",
	}

	result, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "console.vast.ai/api/v0/instances") {
		t.Error("expected Vast.ai API URL in output")
	}
	if !strings.Contains(result, "Authorization: Bearer vast-api-key") {
		t.Error("expected Vast.ai API key in output")
	}
}

func TestGenerateCloudInit_LambdaSpecific(t *testing.T) {
	params := &CloudInitParams{
		WireGuard: WireGuardParams{
			ServerPrivateKey: "test-key",
			ClientPublicKey:  "test-key",
		},
		Provider:   "lambda",
		InstanceID: "12345",
		Model:      "test-model",
		APIKey:     "lambda-api-key",
	}

	result, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "cloud.lambdalabs.com/api/v1/instance-operations/terminate") {
		t.Error("expected Lambda Labs API URL in output")
	}
}

func TestGenerateCloudInit_RunPodSpecific(t *testing.T) {
	params := &CloudInitParams{
		WireGuard: WireGuardParams{
			ServerPrivateKey: "test-key",
			ClientPublicKey:  "test-key",
		},
		Provider:   "runpod",
		InstanceID: "12345",
		Model:      "test-model",
		APIKey:     "runpod-api-key",
	}

	result, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "api.runpod.io/graphql") {
		t.Error("expected RunPod GraphQL API URL in output")
	}
	if !strings.Contains(result, "podTerminate") {
		t.Error("expected RunPod podTerminate mutation in output")
	}
}

func TestGenerateCloudInit_PaperspaceSpecific(t *testing.T) {
	params := &CloudInitParams{
		WireGuard: WireGuardParams{
			ServerPrivateKey: "test-key",
			ClientPublicKey:  "test-key",
		},
		Provider:   "paperspace",
		InstanceID: "12345",
		Model:      "test-model",
		APIKey:     "paperspace-api-key",
	}

	result, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "api.paperspace.io") {
		t.Error("expected Paperspace API URL in output")
	}
	if !strings.Contains(result, "x-api-key: paperspace-api-key") {
		t.Error("expected Paperspace API key header in output")
	}
}

func TestGenerateCloudInitFromConfigPair(t *testing.T) {
	serverKeyPair := &wireguard.KeyPair{
		PrivateKey: "server-private-key",
		PublicKey:  "server-public-key",
	}
	clientKeyPair := &wireguard.KeyPair{
		PrivateKey: "client-private-key",
		PublicKey:  "client-public-key",
	}

	configPair := &wireguard.ConfigPair{
		Server: &wireguard.ServerConfig{
			ServerPrivateKey: serverKeyPair.PrivateKey,
			ClientPublicKey:  clientKeyPair.PublicKey,
			ListenPort:       51820,
			ServerAddress:    "10.13.37.1/24",
			ClientAllowedIPs: "10.13.37.2/32",
		},
		ServerKeyPair: serverKeyPair,
		ClientKeyPair: clientKeyPair,
	}

	result, err := GenerateCloudInitFromConfigPair(configPair, "vast", "12345", "qwen2.5-coder:32b", "api-key", 36000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "PrivateKey = server-private-key") {
		t.Error("expected server private key in output")
	}
	if !strings.Contains(result, "PublicKey = client-public-key") {
		t.Error("expected client public key in output")
	}
}

func TestGenerateCloudInitFromConfigPair_NilConfigPair(t *testing.T) {
	_, err := GenerateCloudInitFromConfigPair(nil, "vast", "12345", "model", "key", 36000)
	if err == nil {
		t.Error("expected error for nil config pair")
	}
}

func TestGenerateCloudInitFromConfigPair_NilServerConfig(t *testing.T) {
	configPair := &wireguard.ConfigPair{
		Server: nil,
	}

	_, err := GenerateCloudInitFromConfigPair(configPair, "vast", "12345", "model", "key", 36000)
	if err == nil {
		t.Error("expected error for nil server config")
	}
}

func TestDeadmanTimeoutFromHours(t *testing.T) {
	tests := []struct {
		hours    int
		expected int
	}{
		{1, 3600},
		{10, 36000},
		{24, 86400},
		{0, 0},
	}

	for _, tt := range tests {
		result := DeadmanTimeoutFromHours(tt.hours)
		if result != tt.expected {
			t.Errorf("DeadmanTimeoutFromHours(%d) = %d, want %d", tt.hours, result, tt.expected)
		}
	}
}

func TestDefaultDeadmanTimeoutSeconds(t *testing.T) {
	if DefaultDeadmanTimeoutSeconds != 36000 {
		t.Errorf("expected DefaultDeadmanTimeoutSeconds to be 36000, got %d", DefaultDeadmanTimeoutSeconds)
	}
}

func TestCloudInitParamsFromServerConfig(t *testing.T) {
	serverCfg := &wireguard.ServerConfig{
		ServerPrivateKey: "server-key",
		ClientPublicKey:  "client-key",
		ListenPort:       51821,
		ServerAddress:    "10.13.37.10/24",
		ClientAllowedIPs: "10.13.37.20/32",
	}

	params := CloudInitParamsFromServerConfig(serverCfg, "vast", "instance-123", "model", "api-key", 7200)

	if params.WireGuard.ServerPrivateKey != "server-key" {
		t.Error("expected server private key to be set")
	}
	if params.WireGuard.ClientPublicKey != "client-key" {
		t.Error("expected client public key to be set")
	}
	if params.WireGuard.ListenPort != 51821 {
		t.Error("expected custom listen port to be set")
	}
	if params.WireGuard.ServerAddress != "10.13.37.10/24" {
		t.Error("expected custom server address to be set")
	}
	if params.WireGuard.ClientAllowedIPs != "10.13.37.20/32" {
		t.Error("expected custom client allowed IPs to be set")
	}
	if params.Provider != "vast" {
		t.Error("expected provider to be set")
	}
	if params.InstanceID != "instance-123" {
		t.Error("expected instance ID to be set")
	}
	if params.Model != "model" {
		t.Error("expected model to be set")
	}
	if params.APIKey != "api-key" {
		t.Error("expected API key to be set")
	}
	if params.Deadman.TimeoutSeconds != 7200 {
		t.Error("expected custom deadman timeout to be set")
	}
}

func TestCloudInitParamsFromServerConfig_NilServerConfig(t *testing.T) {
	params := CloudInitParamsFromServerConfig(nil, "vast", "instance-123", "model", "api-key", 7200)

	// Should still have defaults set
	if params.WireGuard.ListenPort != wireguard.DefaultListenPort {
		t.Error("expected default listen port when server config is nil")
	}
	if params.Provider != "vast" {
		t.Error("expected provider to be set even when server config is nil")
	}
}

func TestGenerateCloudInit_ProviderNormalization(t *testing.T) {
	// Test that provider names are normalized to lowercase
	params := &CloudInitParams{
		WireGuard: WireGuardParams{
			ServerPrivateKey: "test-key",
			ClientPublicKey:  "test-key",
		},
		Provider:   "VAST",
		InstanceID: "12345",
		Model:      "test-model",
		APIKey:     "api-key",
	}

	result, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Provider should be normalized to lowercase
	if !strings.Contains(result, "PROVIDER=vast") {
		t.Error("expected provider to be normalized to lowercase")
	}
}

func TestGenerateCloudInit_ValidYAML(t *testing.T) {
	params := &CloudInitParams{
		WireGuard: WireGuardParams{
			ServerPrivateKey: "test-key",
			ClientPublicKey:  "test-key",
		},
		Provider:   "vast",
		InstanceID: "12345",
		Model:      "qwen2.5-coder:32b",
		APIKey:     "api-key",
	}

	result, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for key YAML sections
	requiredSections := []string{
		"packages:",
		"write_files:",
		"runcmd:",
		"package_update: true",
		"package_upgrade: false",
	}

	for _, section := range requiredSections {
		if !strings.Contains(result, section) {
			t.Errorf("expected cloud-init output to contain %q", section)
		}
	}
}
