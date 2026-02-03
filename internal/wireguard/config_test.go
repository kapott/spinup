package wireguard

import (
	"strings"
	"testing"
)

func TestGenerateClientConfig(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate server key pair: %v", err)
	}

	cfg := &ClientConfig{
		ClientPrivateKey:    clientKeyPair.PrivateKey,
		ServerPublicKey:     serverKeyPair.PublicKey,
		ServerEndpoint:      "1.2.3.4:51820",
		ClientAddress:       ClientAddress,
		ServerAllowedIPs:    ServerAllowedIPs,
		PersistentKeepalive: DefaultKeepalive,
	}

	config, err := GenerateClientConfig(cfg)
	if err != nil {
		t.Fatalf("failed to generate client config: %v", err)
	}

	// Verify config contains expected sections
	if !strings.Contains(config, "[Interface]") {
		t.Error("config missing [Interface] section")
	}
	if !strings.Contains(config, "[Peer]") {
		t.Error("config missing [Peer] section")
	}
	if !strings.Contains(config, "PrivateKey = "+clientKeyPair.PrivateKey) {
		t.Error("config missing client private key")
	}
	if !strings.Contains(config, "PublicKey = "+serverKeyPair.PublicKey) {
		t.Error("config missing server public key")
	}
	if !strings.Contains(config, "Address = 10.13.37.2/24") {
		t.Error("config missing client address")
	}
	if !strings.Contains(config, "Endpoint = 1.2.3.4:51820") {
		t.Error("config missing server endpoint")
	}
	if !strings.Contains(config, "AllowedIPs = 10.13.37.1/32") {
		t.Error("config missing allowed IPs")
	}
	if !strings.Contains(config, "PersistentKeepalive = 25") {
		t.Error("config missing persistent keepalive")
	}
}

func TestGenerateClientConfig_RequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *ClientConfig
		wantErr string
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: "client config is nil",
		},
		{
			name: "missing client private key",
			cfg: &ClientConfig{
				ServerPublicKey: "some-key",
				ServerEndpoint:  "1.2.3.4:51820",
			},
			wantErr: "client private key is required",
		},
		{
			name: "missing server public key",
			cfg: &ClientConfig{
				ClientPrivateKey: "some-key",
				ServerEndpoint:   "1.2.3.4:51820",
			},
			wantErr: "server public key is required",
		},
		{
			name: "missing server endpoint",
			cfg: &ClientConfig{
				ClientPrivateKey: "some-key",
				ServerPublicKey:  "other-key",
			},
			wantErr: "server endpoint is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateClientConfig(tt.cfg)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestGenerateServerConfig(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate server key pair: %v", err)
	}

	cfg := &ServerConfig{
		ServerPrivateKey: serverKeyPair.PrivateKey,
		ClientPublicKey:  clientKeyPair.PublicKey,
		ListenPort:       DefaultListenPort,
		ServerAddress:    ServerAddress,
		ClientAllowedIPs: ClientAllowedIPs,
	}

	config, err := GenerateServerConfig(cfg)
	if err != nil {
		t.Fatalf("failed to generate server config: %v", err)
	}

	// Verify config contains expected sections
	if !strings.Contains(config, "[Interface]") {
		t.Error("config missing [Interface] section")
	}
	if !strings.Contains(config, "[Peer]") {
		t.Error("config missing [Peer] section")
	}
	if !strings.Contains(config, "PrivateKey = "+serverKeyPair.PrivateKey) {
		t.Error("config missing server private key")
	}
	if !strings.Contains(config, "PublicKey = "+clientKeyPair.PublicKey) {
		t.Error("config missing client public key")
	}
	if !strings.Contains(config, "Address = 10.13.37.1/24") {
		t.Error("config missing server address")
	}
	if !strings.Contains(config, "ListenPort = 51820") {
		t.Error("config missing listen port")
	}
	if !strings.Contains(config, "AllowedIPs = 10.13.37.2/32") {
		t.Error("config missing allowed IPs")
	}
}

func TestGenerateServerConfig_RequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *ServerConfig
		wantErr string
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: "server config is nil",
		},
		{
			name: "missing server private key",
			cfg: &ServerConfig{
				ClientPublicKey: "some-key",
			},
			wantErr: "server private key is required",
		},
		{
			name: "missing client public key",
			cfg: &ServerConfig{
				ServerPrivateKey: "some-key",
			},
			wantErr: "client public key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateServerConfig(tt.cfg)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestGenerateServerCloudInit(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate server key pair: %v", err)
	}

	cfg := &ServerConfig{
		ServerPrivateKey: serverKeyPair.PrivateKey,
		ClientPublicKey:  clientKeyPair.PublicKey,
		ListenPort:       DefaultListenPort,
		ServerAddress:    ServerAddress,
		ClientAllowedIPs: ClientAllowedIPs,
	}

	config, err := GenerateServerCloudInit(cfg)
	if err != nil {
		t.Fatalf("failed to generate server cloud-init config: %v", err)
	}

	// Verify cloud-init YAML format
	if !strings.Contains(config, "wireguard:") {
		t.Error("config missing wireguard: section")
	}
	if !strings.Contains(config, "interfaces:") {
		t.Error("config missing interfaces: section")
	}
	if !strings.Contains(config, "wg0:") {
		t.Error("config missing wg0: interface")
	}
	if !strings.Contains(config, "private_key: "+serverKeyPair.PrivateKey) {
		t.Error("config missing server private key")
	}
	if !strings.Contains(config, "public_key: "+clientKeyPair.PublicKey) {
		t.Error("config missing client public key")
	}
	if !strings.Contains(config, "listen_port: 51820") {
		t.Error("config missing listen port")
	}
	if !strings.Contains(config, "- 10.13.37.1/24") {
		t.Error("config missing server address")
	}
	if !strings.Contains(config, "- 10.13.37.2/32") {
		t.Error("config missing client allowed IPs")
	}
}

func TestGenerateConfigPair(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverEndpoint := "203.0.113.50:51820"

	configPair, err := GenerateConfigPair(clientKeyPair, serverEndpoint)
	if err != nil {
		t.Fatalf("failed to generate config pair: %v", err)
	}

	// Verify client config
	if configPair.Client.ClientPrivateKey != clientKeyPair.PrivateKey {
		t.Error("client config has wrong private key")
	}
	if configPair.Client.ServerEndpoint != serverEndpoint {
		t.Error("client config has wrong server endpoint")
	}
	if configPair.Client.ServerPublicKey == "" {
		t.Error("client config missing server public key")
	}

	// Verify server config
	if configPair.Server.ClientPublicKey != clientKeyPair.PublicKey {
		t.Error("server config has wrong client public key")
	}
	if configPair.Server.ServerPrivateKey == "" {
		t.Error("server config missing private key")
	}

	// Verify server public key in client config matches server private key derived public key
	derivedPublicKey, err := PublicKeyFromPrivate(configPair.Server.ServerPrivateKey)
	if err != nil {
		t.Fatalf("failed to derive public key: %v", err)
	}
	if configPair.Client.ServerPublicKey != derivedPublicKey {
		t.Error("server public key mismatch")
	}

	// Test rendering
	clientConfig, err := configPair.RenderClientConfig()
	if err != nil {
		t.Fatalf("failed to render client config: %v", err)
	}
	if clientConfig == "" {
		t.Error("rendered client config is empty")
	}

	serverConfig, err := configPair.RenderServerConfig()
	if err != nil {
		t.Fatalf("failed to render server config: %v", err)
	}
	if serverConfig == "" {
		t.Error("rendered server config is empty")
	}

	cloudInitConfig, err := configPair.RenderServerCloudInit()
	if err != nil {
		t.Fatalf("failed to render cloud-init config: %v", err)
	}
	if cloudInitConfig == "" {
		t.Error("rendered cloud-init config is empty")
	}
}

func TestGenerateConfigPair_ValidationErrors(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	tests := []struct {
		name           string
		clientKeyPair  *KeyPair
		serverEndpoint string
		wantErr        string
	}{
		{
			name:           "nil client key pair",
			clientKeyPair:  nil,
			serverEndpoint: "1.2.3.4:51820",
			wantErr:        "client key pair is required",
		},
		{
			name:           "empty server endpoint",
			clientKeyPair:  clientKeyPair,
			serverEndpoint: "",
			wantErr:        "server endpoint is required",
		},
		{
			name: "empty private key",
			clientKeyPair: &KeyPair{
				PrivateKey: "",
				PublicKey:  clientKeyPair.PublicKey,
			},
			serverEndpoint: "1.2.3.4:51820",
			wantErr:        "client key pair must have both private and public keys",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateConfigPair(tt.clientKeyPair, tt.serverEndpoint)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// Verify PRD-specified IP range
	if ClientAddress != "10.13.37.2/24" {
		t.Errorf("ClientAddress should be 10.13.37.2/24, got %s", ClientAddress)
	}
	if ServerAddress != "10.13.37.1/24" {
		t.Errorf("ServerAddress should be 10.13.37.1/24, got %s", ServerAddress)
	}
	if ClientIP != "10.13.37.2" {
		t.Errorf("ClientIP should be 10.13.37.2, got %s", ClientIP)
	}
	if ServerIP != "10.13.37.1" {
		t.Errorf("ServerIP should be 10.13.37.1, got %s", ServerIP)
	}
	if DefaultListenPort != 51820 {
		t.Errorf("DefaultListenPort should be 51820, got %d", DefaultListenPort)
	}
	if DefaultKeepalive != 25 {
		t.Errorf("DefaultKeepalive should be 25, got %d", DefaultKeepalive)
	}
}

func TestOllamaEndpoint(t *testing.T) {
	endpoint := OllamaEndpoint()
	expected := "http://10.13.37.1:11434"
	if endpoint != expected {
		t.Errorf("OllamaEndpoint() = %s, want %s", endpoint, expected)
	}
}

func TestNewClientConfig_Defaults(t *testing.T) {
	cfg := NewClientConfig()

	if cfg.ClientAddress != ClientAddress {
		t.Errorf("ClientAddress = %s, want %s", cfg.ClientAddress, ClientAddress)
	}
	if cfg.ServerAllowedIPs != ServerAllowedIPs {
		t.Errorf("ServerAllowedIPs = %s, want %s", cfg.ServerAllowedIPs, ServerAllowedIPs)
	}
	if cfg.PersistentKeepalive != DefaultKeepalive {
		t.Errorf("PersistentKeepalive = %d, want %d", cfg.PersistentKeepalive, DefaultKeepalive)
	}
}

func TestNewServerConfig_Defaults(t *testing.T) {
	cfg := NewServerConfig()

	if cfg.ListenPort != DefaultListenPort {
		t.Errorf("ListenPort = %d, want %d", cfg.ListenPort, DefaultListenPort)
	}
	if cfg.ServerAddress != ServerAddress {
		t.Errorf("ServerAddress = %s, want %s", cfg.ServerAddress, ServerAddress)
	}
	if cfg.ClientAllowedIPs != ClientAllowedIPs {
		t.Errorf("ClientAllowedIPs = %s, want %s", cfg.ClientAllowedIPs, ClientAllowedIPs)
	}
}

func TestGenerateConfigPairWithServerKeys(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate server key pair: %v", err)
	}

	serverEndpoint := "192.168.1.100:51820"

	configPair, err := GenerateConfigPairWithServerKeys(clientKeyPair, serverKeyPair, serverEndpoint)
	if err != nil {
		t.Fatalf("failed to generate config pair: %v", err)
	}

	// Verify keys are as provided
	if configPair.Client.ClientPrivateKey != clientKeyPair.PrivateKey {
		t.Error("client private key mismatch")
	}
	if configPair.Client.ServerPublicKey != serverKeyPair.PublicKey {
		t.Error("server public key mismatch in client config")
	}
	if configPair.Server.ServerPrivateKey != serverKeyPair.PrivateKey {
		t.Error("server private key mismatch")
	}
	if configPair.Server.ClientPublicKey != clientKeyPair.PublicKey {
		t.Error("client public key mismatch in server config")
	}
}

// TestClientConfigValidSyntax verifies that generated client configs have valid WireGuard INI syntax.
func TestClientConfigValidSyntax(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate server key pair: %v", err)
	}

	cfg := &ClientConfig{
		ClientPrivateKey:    clientKeyPair.PrivateKey,
		ServerPublicKey:     serverKeyPair.PublicKey,
		ServerEndpoint:      "198.51.100.1:51820",
		ClientAddress:       ClientAddress,
		ServerAllowedIPs:    ServerAllowedIPs,
		PersistentKeepalive: DefaultKeepalive,
	}

	config, err := GenerateClientConfig(cfg)
	if err != nil {
		t.Fatalf("failed to generate client config: %v", err)
	}

	// Validate INI format structure
	lines := strings.Split(config, "\n")
	var foundInterface, foundPeer bool
	var inInterfaceSection, inPeerSection bool
	interfaceKeys := make(map[string]bool)
	peerKeys := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "[Interface]" {
			foundInterface = true
			inInterfaceSection = true
			inPeerSection = false
			continue
		}
		if line == "[Peer]" {
			foundPeer = true
			inPeerSection = true
			inInterfaceSection = false
			continue
		}
		// Check key=value format
		if inInterfaceSection || inPeerSection {
			parts := strings.SplitN(line, " = ", 2)
			if len(parts) != 2 {
				t.Errorf("invalid key-value format: %s", line)
				continue
			}
			key := parts[0]
			value := parts[1]
			if key == "" || value == "" {
				t.Errorf("empty key or value in line: %s", line)
			}
			if inInterfaceSection {
				interfaceKeys[key] = true
			}
			if inPeerSection {
				peerKeys[key] = true
			}
		}
	}

	if !foundInterface {
		t.Error("missing [Interface] section")
	}
	if !foundPeer {
		t.Error("missing [Peer] section")
	}

	// Verify required Interface keys
	requiredInterfaceKeys := []string{"PrivateKey", "Address"}
	for _, key := range requiredInterfaceKeys {
		if !interfaceKeys[key] {
			t.Errorf("missing required Interface key: %s", key)
		}
	}

	// Verify required Peer keys
	requiredPeerKeys := []string{"PublicKey", "Endpoint", "AllowedIPs", "PersistentKeepalive"}
	for _, key := range requiredPeerKeys {
		if !peerKeys[key] {
			t.Errorf("missing required Peer key: %s", key)
		}
	}
}

// TestServerConfigValidSyntax verifies that generated server configs have valid WireGuard INI syntax.
func TestServerConfigValidSyntax(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate server key pair: %v", err)
	}

	cfg := &ServerConfig{
		ServerPrivateKey: serverKeyPair.PrivateKey,
		ClientPublicKey:  clientKeyPair.PublicKey,
		ListenPort:       DefaultListenPort,
		ServerAddress:    ServerAddress,
		ClientAllowedIPs: ClientAllowedIPs,
	}

	config, err := GenerateServerConfig(cfg)
	if err != nil {
		t.Fatalf("failed to generate server config: %v", err)
	}

	// Validate INI format structure
	lines := strings.Split(config, "\n")
	var foundInterface, foundPeer bool
	var inInterfaceSection, inPeerSection bool
	interfaceKeys := make(map[string]bool)
	peerKeys := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "[Interface]" {
			foundInterface = true
			inInterfaceSection = true
			inPeerSection = false
			continue
		}
		if line == "[Peer]" {
			foundPeer = true
			inPeerSection = true
			inInterfaceSection = false
			continue
		}
		// Check key=value format
		if inInterfaceSection || inPeerSection {
			parts := strings.SplitN(line, " = ", 2)
			if len(parts) != 2 {
				t.Errorf("invalid key-value format: %s", line)
				continue
			}
			key := parts[0]
			value := parts[1]
			if key == "" || value == "" {
				t.Errorf("empty key or value in line: %s", line)
			}
			if inInterfaceSection {
				interfaceKeys[key] = true
			}
			if inPeerSection {
				peerKeys[key] = true
			}
		}
	}

	if !foundInterface {
		t.Error("missing [Interface] section")
	}
	if !foundPeer {
		t.Error("missing [Peer] section")
	}

	// Verify required Interface keys for server
	requiredInterfaceKeys := []string{"PrivateKey", "Address", "ListenPort"}
	for _, key := range requiredInterfaceKeys {
		if !interfaceKeys[key] {
			t.Errorf("missing required Interface key: %s", key)
		}
	}

	// Verify required Peer keys
	requiredPeerKeys := []string{"PublicKey", "AllowedIPs"}
	for _, key := range requiredPeerKeys {
		if !peerKeys[key] {
			t.Errorf("missing required Peer key: %s", key)
		}
	}
}

// TestAllParametersSubstituted verifies that all template placeholders are properly substituted.
func TestAllParametersSubstituted(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate server key pair: %v", err)
	}

	serverEndpoint := "203.0.113.100:51820"

	// Test client config
	clientCfg := &ClientConfig{
		ClientPrivateKey:    clientKeyPair.PrivateKey,
		ServerPublicKey:     serverKeyPair.PublicKey,
		ServerEndpoint:      serverEndpoint,
		ClientAddress:       ClientAddress,
		ServerAllowedIPs:    ServerAllowedIPs,
		PersistentKeepalive: DefaultKeepalive,
	}

	clientConfig, err := GenerateClientConfig(clientCfg)
	if err != nil {
		t.Fatalf("failed to generate client config: %v", err)
	}

	// Check no template placeholders remain ({{ ... }})
	if strings.Contains(clientConfig, "{{") || strings.Contains(clientConfig, "}}") {
		t.Error("client config contains unsubstituted template placeholders")
	}

	// Verify exact substitutions
	if !strings.Contains(clientConfig, clientKeyPair.PrivateKey) {
		t.Error("client private key not substituted")
	}
	if !strings.Contains(clientConfig, serverKeyPair.PublicKey) {
		t.Error("server public key not substituted")
	}
	if !strings.Contains(clientConfig, serverEndpoint) {
		t.Error("server endpoint not substituted")
	}
	if !strings.Contains(clientConfig, ClientAddress) {
		t.Error("client address not substituted")
	}
	if !strings.Contains(clientConfig, ServerAllowedIPs) {
		t.Error("server allowed IPs not substituted")
	}
	if !strings.Contains(clientConfig, "25") {
		t.Error("persistent keepalive not substituted")
	}

	// Test server config
	serverCfg := &ServerConfig{
		ServerPrivateKey: serverKeyPair.PrivateKey,
		ClientPublicKey:  clientKeyPair.PublicKey,
		ListenPort:       DefaultListenPort,
		ServerAddress:    ServerAddress,
		ClientAllowedIPs: ClientAllowedIPs,
	}

	serverConfig, err := GenerateServerConfig(serverCfg)
	if err != nil {
		t.Fatalf("failed to generate server config: %v", err)
	}

	// Check no template placeholders remain
	if strings.Contains(serverConfig, "{{") || strings.Contains(serverConfig, "}}") {
		t.Error("server config contains unsubstituted template placeholders")
	}

	// Verify exact substitutions
	if !strings.Contains(serverConfig, serverKeyPair.PrivateKey) {
		t.Error("server private key not substituted")
	}
	if !strings.Contains(serverConfig, clientKeyPair.PublicKey) {
		t.Error("client public key not substituted")
	}
	if !strings.Contains(serverConfig, "51820") {
		t.Error("listen port not substituted")
	}
	if !strings.Contains(serverConfig, ServerAddress) {
		t.Error("server address not substituted")
	}
	if !strings.Contains(serverConfig, ClientAllowedIPs) {
		t.Error("client allowed IPs not substituted")
	}

	// Test cloud-init config
	cloudInitConfig, err := GenerateServerCloudInit(serverCfg)
	if err != nil {
		t.Fatalf("failed to generate cloud-init config: %v", err)
	}

	// Check no template placeholders remain
	if strings.Contains(cloudInitConfig, "{{") || strings.Contains(cloudInitConfig, "}}") {
		t.Error("cloud-init config contains unsubstituted template placeholders")
	}
}

// TestConfigWithSpecialCharacters verifies configs handle special endpoint formats.
func TestConfigWithSpecialCharacters(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate server key pair: %v", err)
	}

	tests := []struct {
		name           string
		serverEndpoint string
	}{
		{"IPv4 with default port", "192.0.2.1:51820"},
		{"IPv4 with custom port", "192.0.2.1:12345"},
		{"IPv6", "[2001:db8::1]:51820"},
		{"hostname", "vpn.example.com:51820"},
		{"hostname with custom port", "vpn.example.com:65535"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ClientConfig{
				ClientPrivateKey:    clientKeyPair.PrivateKey,
				ServerPublicKey:     serverKeyPair.PublicKey,
				ServerEndpoint:      tt.serverEndpoint,
				ClientAddress:       ClientAddress,
				ServerAllowedIPs:    ServerAllowedIPs,
				PersistentKeepalive: DefaultKeepalive,
			}

			config, err := GenerateClientConfig(cfg)
			if err != nil {
				t.Fatalf("failed to generate client config: %v", err)
			}

			if !strings.Contains(config, "Endpoint = "+tt.serverEndpoint) {
				t.Errorf("endpoint not properly substituted, expected %s", tt.serverEndpoint)
			}
		})
	}
}

// TestDefaultsApplied verifies that defaults are applied when fields are empty.
func TestDefaultsApplied(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate server key pair: %v", err)
	}

	// Client config with only required fields
	clientCfg := &ClientConfig{
		ClientPrivateKey: clientKeyPair.PrivateKey,
		ServerPublicKey:  serverKeyPair.PublicKey,
		ServerEndpoint:   "1.2.3.4:51820",
		// Leave optional fields empty
	}

	clientConfig, err := GenerateClientConfig(clientCfg)
	if err != nil {
		t.Fatalf("failed to generate client config: %v", err)
	}

	// Verify defaults were applied
	if !strings.Contains(clientConfig, "Address = "+ClientAddress) {
		t.Errorf("default client address not applied, expected %s", ClientAddress)
	}
	if !strings.Contains(clientConfig, "AllowedIPs = "+ServerAllowedIPs) {
		t.Errorf("default server allowed IPs not applied, expected %s", ServerAllowedIPs)
	}
	if !strings.Contains(clientConfig, "PersistentKeepalive = 25") {
		t.Error("default persistent keepalive not applied, expected 25")
	}

	// Server config with only required fields
	serverCfg := &ServerConfig{
		ServerPrivateKey: serverKeyPair.PrivateKey,
		ClientPublicKey:  clientKeyPair.PublicKey,
		// Leave optional fields empty
	}

	serverConfig, err := GenerateServerConfig(serverCfg)
	if err != nil {
		t.Fatalf("failed to generate server config: %v", err)
	}

	// Verify defaults were applied
	if !strings.Contains(serverConfig, "ListenPort = 51820") {
		t.Error("default listen port not applied, expected 51820")
	}
	if !strings.Contains(serverConfig, "Address = "+ServerAddress) {
		t.Errorf("default server address not applied, expected %s", ServerAddress)
	}
	if !strings.Contains(serverConfig, "AllowedIPs = "+ClientAllowedIPs) {
		t.Errorf("default client allowed IPs not applied, expected %s", ClientAllowedIPs)
	}
}

// TestCloudInitYAMLValidity verifies that cloud-init output is valid YAML structure.
func TestCloudInitYAMLValidity(t *testing.T) {
	clientKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate client key pair: %v", err)
	}

	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate server key pair: %v", err)
	}

	cfg := &ServerConfig{
		ServerPrivateKey: serverKeyPair.PrivateKey,
		ClientPublicKey:  clientKeyPair.PublicKey,
		ListenPort:       DefaultListenPort,
		ServerAddress:    ServerAddress,
		ClientAllowedIPs: ClientAllowedIPs,
	}

	config, err := GenerateServerCloudInit(cfg)
	if err != nil {
		t.Fatalf("failed to generate cloud-init config: %v", err)
	}

	// Verify YAML structure
	lines := strings.Split(config, "\n")
	foundWireguard := false
	foundInterfaces := false
	foundWg0 := false
	foundPrivateKey := false
	foundListenPort := false
	foundPeers := false
	foundPublicKey := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue // Skip comments
		}
		if trimmed == "wireguard:" {
			foundWireguard = true
		}
		if strings.Contains(line, "interfaces:") {
			foundInterfaces = true
		}
		if strings.Contains(line, "wg0:") {
			foundWg0 = true
		}
		if strings.Contains(line, "private_key:") && strings.Contains(line, serverKeyPair.PrivateKey) {
			foundPrivateKey = true
		}
		if strings.Contains(line, "listen_port: 51820") {
			foundListenPort = true
		}
		if strings.Contains(line, "peers:") {
			foundPeers = true
		}
		if strings.Contains(line, "public_key:") && strings.Contains(line, clientKeyPair.PublicKey) {
			foundPublicKey = true
		}
	}

	if !foundWireguard {
		t.Error("missing wireguard: section")
	}
	if !foundInterfaces {
		t.Error("missing interfaces: section")
	}
	if !foundWg0 {
		t.Error("missing wg0: interface")
	}
	if !foundPrivateKey {
		t.Error("missing or incorrect private_key")
	}
	if !foundListenPort {
		t.Error("missing or incorrect listen_port")
	}
	if !foundPeers {
		t.Error("missing peers: section")
	}
	if !foundPublicKey {
		t.Error("missing or incorrect public_key in peers")
	}
}
