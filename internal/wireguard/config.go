// Package wireguard provides WireGuard key generation and configuration management.
package wireguard

import (
	"bytes"
	"fmt"
	"text/template"
)

// Network constants for WireGuard tunnel.
// Uses 10.13.37.0/24 network as specified in PRD Section 4.1.
const (
	// DefaultListenPort is the WireGuard UDP port on the server.
	DefaultListenPort = 51820
	// ClientAddress is the WireGuard IP for the local client.
	ClientAddress = "10.13.37.2/24"
	// ClientIP is the client IP without CIDR notation.
	ClientIP = "10.13.37.2"
	// ServerAddress is the WireGuard IP for the remote server.
	ServerAddress = "10.13.37.1/24"
	// ServerIP is the server IP without CIDR notation.
	ServerIP = "10.13.37.1"
	// ClientAllowedIPs is the CIDR range allowed for the client (server perspective).
	ClientAllowedIPs = "10.13.37.2/32"
	// ServerAllowedIPs is the CIDR range allowed for the server (client perspective).
	ServerAllowedIPs = "10.13.37.1/32"
	// DefaultKeepalive is the persistent keepalive interval in seconds.
	DefaultKeepalive = 25
	// InterfaceName is the WireGuard interface name used by spinup.
	InterfaceName = "wg-spinup"
)

// ClientConfig holds the configuration for generating a client-side WireGuard config.
type ClientConfig struct {
	// ClientPrivateKey is the base64-encoded private key for the local machine.
	ClientPrivateKey string
	// ServerPublicKey is the base64-encoded public key of the server.
	ServerPublicKey string
	// ServerEndpoint is the public IP:port of the server (e.g., "1.2.3.4:51820").
	ServerEndpoint string
	// ClientAddress is the WireGuard IP for the client (default: 10.13.37.2/24).
	ClientAddress string
	// ServerAllowedIPs is the CIDR range for the server (default: 10.13.37.1/32).
	ServerAllowedIPs string
	// PersistentKeepalive is the keepalive interval in seconds (default: 25).
	PersistentKeepalive int
}

// ServerConfig holds the configuration for generating a server-side WireGuard config.
type ServerConfig struct {
	// ServerPrivateKey is the base64-encoded private key for the server.
	ServerPrivateKey string
	// ClientPublicKey is the base64-encoded public key of the client.
	ClientPublicKey string
	// ListenPort is the UDP port for WireGuard (default: 51820).
	ListenPort int
	// ServerAddress is the WireGuard IP for the server (default: 10.13.37.1/24).
	ServerAddress string
	// ClientAllowedIPs is the CIDR range for the client (default: 10.13.37.2/32).
	ClientAllowedIPs string
}

// ConfigPair holds both client and server configurations for a WireGuard tunnel.
type ConfigPair struct {
	// Client is the client-side configuration.
	Client *ClientConfig
	// Server is the server-side configuration.
	Server *ServerConfig
	// ClientKeyPair contains the client's key pair.
	ClientKeyPair *KeyPair
	// ServerKeyPair contains the server's key pair.
	ServerKeyPair *KeyPair
}

// clientConfigTemplate is the template for client-side WireGuard configuration (INI format).
const clientConfigTemplate = `[Interface]
PrivateKey = {{ .ClientPrivateKey }}
Address = {{ .ClientAddress }}

[Peer]
PublicKey = {{ .ServerPublicKey }}
Endpoint = {{ .ServerEndpoint }}
AllowedIPs = {{ .ServerAllowedIPs }}
PersistentKeepalive = {{ .PersistentKeepalive }}
`

// serverConfigTemplate is the template for server-side WireGuard configuration (INI format).
const serverConfigTemplate = `[Interface]
PrivateKey = {{ .ServerPrivateKey }}
Address = {{ .ServerAddress }}
ListenPort = {{ .ListenPort }}

[Peer]
PublicKey = {{ .ClientPublicKey }}
AllowedIPs = {{ .ClientAllowedIPs }}
`

// serverCloudInitTemplate is the template for server-side WireGuard config in cloud-init YAML format.
const serverCloudInitTemplate = `# WireGuard configuration for spinup
# Generated for cloud-init injection
wireguard:
  interfaces:
    wg0:
      private_key: {{ .ServerPrivateKey }}
      listen_port: {{ .ListenPort }}
      addresses:
        - {{ .ServerAddress }}
      peers:
        - public_key: {{ .ClientPublicKey }}
          allowed_ips:
            - {{ .ClientAllowedIPs }}
`

// NewClientConfig creates a ClientConfig with default values.
// The caller must set ClientPrivateKey, ServerPublicKey, and ServerEndpoint.
func NewClientConfig() *ClientConfig {
	return &ClientConfig{
		ClientAddress:       ClientAddress,
		ServerAllowedIPs:    ServerAllowedIPs,
		PersistentKeepalive: DefaultKeepalive,
	}
}

// NewServerConfig creates a ServerConfig with default values.
// The caller must set ServerPrivateKey and ClientPublicKey.
func NewServerConfig() *ServerConfig {
	return &ServerConfig{
		ListenPort:       DefaultListenPort,
		ServerAddress:    ServerAddress,
		ClientAllowedIPs: ClientAllowedIPs,
	}
}

// GenerateClientConfig generates a WireGuard client configuration string.
// The output is in INI format suitable for wg-quick or WireGuard tools.
func GenerateClientConfig(cfg *ClientConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("client config is nil")
	}
	if cfg.ClientPrivateKey == "" {
		return "", fmt.Errorf("client private key is required")
	}
	if cfg.ServerPublicKey == "" {
		return "", fmt.Errorf("server public key is required")
	}
	if cfg.ServerEndpoint == "" {
		return "", fmt.Errorf("server endpoint is required")
	}

	// Apply defaults if not set
	if cfg.ClientAddress == "" {
		cfg.ClientAddress = ClientAddress
	}
	if cfg.ServerAllowedIPs == "" {
		cfg.ServerAllowedIPs = ServerAllowedIPs
	}
	if cfg.PersistentKeepalive == 0 {
		cfg.PersistentKeepalive = DefaultKeepalive
	}

	tmpl, err := template.New("client").Parse(clientConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse client config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("failed to execute client config template: %w", err)
	}

	return buf.String(), nil
}

// GenerateServerConfig generates a WireGuard server configuration string.
// The output is in INI format suitable for wg-quick or WireGuard tools.
func GenerateServerConfig(cfg *ServerConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("server config is nil")
	}
	if cfg.ServerPrivateKey == "" {
		return "", fmt.Errorf("server private key is required")
	}
	if cfg.ClientPublicKey == "" {
		return "", fmt.Errorf("client public key is required")
	}

	// Apply defaults if not set
	if cfg.ListenPort == 0 {
		cfg.ListenPort = DefaultListenPort
	}
	if cfg.ServerAddress == "" {
		cfg.ServerAddress = ServerAddress
	}
	if cfg.ClientAllowedIPs == "" {
		cfg.ClientAllowedIPs = ClientAllowedIPs
	}

	tmpl, err := template.New("server").Parse(serverConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse server config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("failed to execute server config template: %w", err)
	}

	return buf.String(), nil
}

// GenerateServerCloudInit generates a WireGuard server configuration in cloud-init YAML format.
// This is suitable for injection into cloud-init scripts for instance bootstrap.
func GenerateServerCloudInit(cfg *ServerConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("server config is nil")
	}
	if cfg.ServerPrivateKey == "" {
		return "", fmt.Errorf("server private key is required")
	}
	if cfg.ClientPublicKey == "" {
		return "", fmt.Errorf("client public key is required")
	}

	// Apply defaults if not set
	if cfg.ListenPort == 0 {
		cfg.ListenPort = DefaultListenPort
	}
	if cfg.ServerAddress == "" {
		cfg.ServerAddress = ServerAddress
	}
	if cfg.ClientAllowedIPs == "" {
		cfg.ClientAllowedIPs = ClientAllowedIPs
	}

	tmpl, err := template.New("serverCloudInit").Parse(serverCloudInitTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse server cloud-init template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("failed to execute server cloud-init template: %w", err)
	}

	return buf.String(), nil
}

// GenerateConfigPair generates a complete WireGuard configuration pair for both client and server.
// This generates new server keys and uses the provided client keys.
// The serverEndpoint is the public IP:port of the server instance.
func GenerateConfigPair(clientKeyPair *KeyPair, serverEndpoint string) (*ConfigPair, error) {
	if clientKeyPair == nil {
		return nil, fmt.Errorf("client key pair is required")
	}
	if clientKeyPair.PrivateKey == "" || clientKeyPair.PublicKey == "" {
		return nil, fmt.Errorf("client key pair must have both private and public keys")
	}
	if serverEndpoint == "" {
		return nil, fmt.Errorf("server endpoint is required")
	}

	// Generate server key pair
	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate server key pair: %w", err)
	}

	clientCfg := &ClientConfig{
		ClientPrivateKey:    clientKeyPair.PrivateKey,
		ServerPublicKey:     serverKeyPair.PublicKey,
		ServerEndpoint:      serverEndpoint,
		ClientAddress:       ClientAddress,
		ServerAllowedIPs:    ServerAllowedIPs,
		PersistentKeepalive: DefaultKeepalive,
	}

	serverCfg := &ServerConfig{
		ServerPrivateKey: serverKeyPair.PrivateKey,
		ClientPublicKey:  clientKeyPair.PublicKey,
		ListenPort:       DefaultListenPort,
		ServerAddress:    ServerAddress,
		ClientAllowedIPs: ClientAllowedIPs,
	}

	return &ConfigPair{
		Client:        clientCfg,
		Server:        serverCfg,
		ClientKeyPair: clientKeyPair,
		ServerKeyPair: serverKeyPair,
	}, nil
}

// GenerateConfigPairWithServerKeys generates a complete WireGuard configuration pair using
// provided client and server key pairs. This is useful when you want to control all keys.
func GenerateConfigPairWithServerKeys(clientKeyPair, serverKeyPair *KeyPair, serverEndpoint string) (*ConfigPair, error) {
	if clientKeyPair == nil {
		return nil, fmt.Errorf("client key pair is required")
	}
	if serverKeyPair == nil {
		return nil, fmt.Errorf("server key pair is required")
	}
	if clientKeyPair.PrivateKey == "" || clientKeyPair.PublicKey == "" {
		return nil, fmt.Errorf("client key pair must have both private and public keys")
	}
	if serverKeyPair.PrivateKey == "" || serverKeyPair.PublicKey == "" {
		return nil, fmt.Errorf("server key pair must have both private and public keys")
	}
	if serverEndpoint == "" {
		return nil, fmt.Errorf("server endpoint is required")
	}

	clientCfg := &ClientConfig{
		ClientPrivateKey:    clientKeyPair.PrivateKey,
		ServerPublicKey:     serverKeyPair.PublicKey,
		ServerEndpoint:      serverEndpoint,
		ClientAddress:       ClientAddress,
		ServerAllowedIPs:    ServerAllowedIPs,
		PersistentKeepalive: DefaultKeepalive,
	}

	serverCfg := &ServerConfig{
		ServerPrivateKey: serverKeyPair.PrivateKey,
		ClientPublicKey:  clientKeyPair.PublicKey,
		ListenPort:       DefaultListenPort,
		ServerAddress:    ServerAddress,
		ClientAllowedIPs: ClientAllowedIPs,
	}

	return &ConfigPair{
		Client:        clientCfg,
		Server:        serverCfg,
		ClientKeyPair: clientKeyPair,
		ServerKeyPair: serverKeyPair,
	}, nil
}

// RenderClientConfig renders the client configuration from a ConfigPair.
func (cp *ConfigPair) RenderClientConfig() (string, error) {
	return GenerateClientConfig(cp.Client)
}

// RenderServerConfig renders the server configuration from a ConfigPair (INI format).
func (cp *ConfigPair) RenderServerConfig() (string, error) {
	return GenerateServerConfig(cp.Server)
}

// RenderServerCloudInit renders the server configuration from a ConfigPair (cloud-init YAML format).
func (cp *ConfigPair) RenderServerCloudInit() (string, error) {
	return GenerateServerCloudInit(cp.Server)
}

// OllamaEndpoint returns the Ollama API endpoint URL through the WireGuard tunnel.
func OllamaEndpoint() string {
	return fmt.Sprintf("http://%s:11434", ServerIP)
}
