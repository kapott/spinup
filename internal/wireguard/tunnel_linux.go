//go:build linux

// Package wireguard provides WireGuard tunnel management for Linux systems.
package wireguard

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// TunnelConfig holds the configuration needed to set up a WireGuard tunnel.
type TunnelConfig struct {
	// InterfaceName is the name of the WireGuard interface (default: "wg-continueplz").
	InterfaceName string
	// ClientPrivateKey is the base64-encoded private key for the local machine.
	ClientPrivateKey string
	// ServerPublicKey is the base64-encoded public key of the server.
	ServerPublicKey string
	// ServerEndpoint is the public IP:port of the server (e.g., "1.2.3.4:51820").
	ServerEndpoint string
	// ClientAddress is the WireGuard IP for the client (default: "10.13.37.2/24").
	ClientAddress string
	// ServerAllowedIPs is the CIDR range for the server (default: "10.13.37.1/32").
	ServerAllowedIPs string
	// PersistentKeepalive is the keepalive interval in seconds (default: 25).
	PersistentKeepalive int
}

// Tunnel represents an active WireGuard tunnel.
type Tunnel struct {
	// InterfaceName is the name of the WireGuard interface.
	InterfaceName string
	// Config is the configuration used to create this tunnel.
	Config *TunnelConfig
}

// TunnelError represents a WireGuard tunnel operation error.
type TunnelError struct {
	Op      string // Operation that failed (e.g., "create", "configure", "teardown")
	Message string
	Cause   error
}

func (e *TunnelError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("wireguard tunnel %s failed: %s: %v", e.Op, e.Message, e.Cause)
	}
	return fmt.Sprintf("wireguard tunnel %s failed: %s", e.Op, e.Message)
}

func (e *TunnelError) Unwrap() error {
	return e.Cause
}

// ErrRootRequired is returned when root privileges are required but not available.
var ErrRootRequired = &TunnelError{
	Op:      "check",
	Message: "root privileges required for WireGuard tunnel operations",
}

// ErrInterfaceExists is returned when the interface already exists.
var ErrInterfaceExists = &TunnelError{
	Op:      "create",
	Message: "interface already exists",
}

// ErrInterfaceNotFound is returned when the interface does not exist.
var ErrInterfaceNotFound = &TunnelError{
	Op:      "teardown",
	Message: "interface not found",
}

// NewTunnelConfig creates a TunnelConfig with default values.
// The caller must set ClientPrivateKey, ServerPublicKey, and ServerEndpoint.
func NewTunnelConfig() *TunnelConfig {
	return &TunnelConfig{
		InterfaceName:       InterfaceName,
		ClientAddress:       ClientAddress,
		ServerAllowedIPs:    ServerAllowedIPs,
		PersistentKeepalive: DefaultKeepalive,
	}
}

// TunnelConfigFromClientConfig creates a TunnelConfig from a ClientConfig.
// This is a convenience function for setting up a tunnel from an existing config.
func TunnelConfigFromClientConfig(cfg *ClientConfig) *TunnelConfig {
	return &TunnelConfig{
		InterfaceName:       InterfaceName,
		ClientPrivateKey:    cfg.ClientPrivateKey,
		ServerPublicKey:     cfg.ServerPublicKey,
		ServerEndpoint:      cfg.ServerEndpoint,
		ClientAddress:       cfg.ClientAddress,
		ServerAllowedIPs:    cfg.ServerAllowedIPs,
		PersistentKeepalive: cfg.PersistentKeepalive,
	}
}

// validate validates the TunnelConfig.
func (tc *TunnelConfig) validate() error {
	if tc.InterfaceName == "" {
		return &TunnelError{Op: "validate", Message: "interface name is required"}
	}
	if tc.ClientPrivateKey == "" {
		return &TunnelError{Op: "validate", Message: "client private key is required"}
	}
	if tc.ServerPublicKey == "" {
		return &TunnelError{Op: "validate", Message: "server public key is required"}
	}
	if tc.ServerEndpoint == "" {
		return &TunnelError{Op: "validate", Message: "server endpoint is required"}
	}
	if tc.ClientAddress == "" {
		tc.ClientAddress = ClientAddress
	}
	if tc.ServerAllowedIPs == "" {
		tc.ServerAllowedIPs = ServerAllowedIPs
	}
	if tc.PersistentKeepalive == 0 {
		tc.PersistentKeepalive = DefaultKeepalive
	}
	return nil
}

// checkRoot checks if the current process has root privileges.
func checkRoot() bool {
	return os.Geteuid() == 0
}

// checkRootOrSudo checks if we can execute privileged operations.
// Returns true if we have root privileges or sudo is available.
func checkRootOrSudo() bool {
	if checkRoot() {
		return true
	}
	// Check if sudo is available and we can use it without password
	// This is a best-effort check; actual command may still fail
	cmd := exec.Command("sudo", "-n", "true")
	return cmd.Run() == nil
}

// SetupTunnel creates and configures a WireGuard tunnel on Linux.
// This function requires root privileges or sudo access.
// It creates the WireGuard interface, assigns an IP address, and configures the peer.
func SetupTunnel(ctx context.Context, cfg *TunnelConfig) (*Tunnel, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Check for root privileges
	usesSudo := false
	if !checkRoot() {
		if !checkRootOrSudo() {
			return nil, ErrRootRequired
		}
		usesSudo = true
	}

	// Check if interface already exists
	if interfaceExists(cfg.InterfaceName) {
		return nil, &TunnelError{
			Op:      "create",
			Message: fmt.Sprintf("interface %s already exists", cfg.InterfaceName),
		}
	}

	// Create the WireGuard interface using ip link
	if err := createInterface(ctx, cfg.InterfaceName, usesSudo); err != nil {
		return nil, err
	}

	// Configure the WireGuard interface
	if err := configureWireGuard(ctx, cfg); err != nil {
		// Clean up on failure
		_ = deleteInterface(ctx, cfg.InterfaceName, usesSudo)
		return nil, err
	}

	// Assign IP address to the interface
	if err := assignAddress(ctx, cfg.InterfaceName, cfg.ClientAddress, usesSudo); err != nil {
		// Clean up on failure
		_ = deleteInterface(ctx, cfg.InterfaceName, usesSudo)
		return nil, err
	}

	// Bring the interface up
	if err := bringInterfaceUp(ctx, cfg.InterfaceName, usesSudo); err != nil {
		// Clean up on failure
		_ = deleteInterface(ctx, cfg.InterfaceName, usesSudo)
		return nil, err
	}

	// Add route to server
	if err := addRoute(ctx, cfg.InterfaceName, cfg.ServerAllowedIPs, usesSudo); err != nil {
		// Route might already exist, log but don't fail
		// This is a best-effort operation
	}

	return &Tunnel{
		InterfaceName: cfg.InterfaceName,
		Config:        cfg,
	}, nil
}

// TeardownTunnel removes a WireGuard tunnel.
// This function is idempotent - it succeeds even if the tunnel doesn't exist.
func TeardownTunnel(ctx context.Context, interfaceName string) error {
	if interfaceName == "" {
		interfaceName = InterfaceName
	}

	// Check for root privileges
	usesSudo := false
	if !checkRoot() {
		if !checkRootOrSudo() {
			return ErrRootRequired
		}
		usesSudo = true
	}

	// Check if interface exists
	if !interfaceExists(interfaceName) {
		// Interface doesn't exist, nothing to do
		return nil
	}

	// Delete the interface
	return deleteInterface(ctx, interfaceName, usesSudo)
}

// Teardown removes this tunnel.
func (t *Tunnel) Teardown(ctx context.Context) error {
	return TeardownTunnel(ctx, t.InterfaceName)
}

// interfaceExists checks if a network interface exists.
func interfaceExists(name string) bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range interfaces {
		if iface.Name == name {
			return true
		}
	}
	return false
}

// runCommand runs a command with optional sudo.
func runCommand(ctx context.Context, usesSudo bool, name string, args ...string) error {
	var cmd *exec.Cmd
	if usesSudo {
		sudoArgs := append([]string{name}, args...)
		cmd = exec.CommandContext(ctx, "sudo", sudoArgs...)
	} else {
		cmd = exec.CommandContext(ctx, name, args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &TunnelError{
			Op:      "exec",
			Message: fmt.Sprintf("command '%s %s' failed: %s", name, strings.Join(args, " "), string(output)),
			Cause:   err,
		}
	}
	return nil
}

// createInterface creates a WireGuard interface using ip link.
func createInterface(ctx context.Context, name string, usesSudo bool) error {
	err := runCommand(ctx, usesSudo, "ip", "link", "add", "dev", name, "type", "wireguard")
	if err != nil {
		return &TunnelError{
			Op:      "create",
			Message: fmt.Sprintf("failed to create interface %s", name),
			Cause:   err,
		}
	}
	return nil
}

// deleteInterface deletes a network interface.
func deleteInterface(ctx context.Context, name string, usesSudo bool) error {
	err := runCommand(ctx, usesSudo, "ip", "link", "delete", "dev", name)
	if err != nil {
		return &TunnelError{
			Op:      "delete",
			Message: fmt.Sprintf("failed to delete interface %s", name),
			Cause:   err,
		}
	}
	return nil
}

// assignAddress assigns an IP address to an interface.
func assignAddress(ctx context.Context, name, address string, usesSudo bool) error {
	err := runCommand(ctx, usesSudo, "ip", "address", "add", "dev", name, address)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: fmt.Sprintf("failed to assign address %s to interface %s", address, name),
			Cause:   err,
		}
	}
	return nil
}

// bringInterfaceUp brings a network interface up.
func bringInterfaceUp(ctx context.Context, name string, usesSudo bool) error {
	err := runCommand(ctx, usesSudo, "ip", "link", "set", "up", "dev", name)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: fmt.Sprintf("failed to bring up interface %s", name),
			Cause:   err,
		}
	}
	return nil
}

// addRoute adds a route through the WireGuard interface.
func addRoute(ctx context.Context, name, cidr string, usesSudo bool) error {
	// Route is automatically added by WireGuard based on AllowedIPs, but we add it explicitly
	// to ensure it's in place
	err := runCommand(ctx, usesSudo, "ip", "route", "add", cidr, "dev", name)
	if err != nil {
		// Route might already exist, which is fine
		return &TunnelError{
			Op:      "configure",
			Message: fmt.Sprintf("failed to add route %s via interface %s", cidr, name),
			Cause:   err,
		}
	}
	return nil
}

// configureWireGuard configures the WireGuard interface using wgctrl.
func configureWireGuard(ctx context.Context, cfg *TunnelConfig) error {
	// Parse the private key
	privateKey, err := wgtypes.ParseKey(cfg.ClientPrivateKey)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to parse private key",
			Cause:   err,
		}
	}

	// Parse the server public key
	serverPublicKey, err := wgtypes.ParseKey(cfg.ServerPublicKey)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to parse server public key",
			Cause:   err,
		}
	}

	// Parse the server endpoint
	endpoint, err := parseEndpoint(cfg.ServerEndpoint)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to parse server endpoint",
			Cause:   err,
		}
	}

	// Parse the allowed IPs
	_, allowedIPNet, err := net.ParseCIDR(cfg.ServerAllowedIPs)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to parse allowed IPs",
			Cause:   err,
		}
	}

	// Create wgctrl client
	wgClient, err := wgctrl.New()
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to create wgctrl client",
			Cause:   err,
		}
	}
	defer wgClient.Close()

	// Configure the WireGuard interface
	keepalive := time.Duration(cfg.PersistentKeepalive) * time.Second
	wgCfg := wgtypes.Config{
		PrivateKey: &privateKey,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   serverPublicKey,
				Endpoint:                    endpoint,
				AllowedIPs:                  []net.IPNet{*allowedIPNet},
				PersistentKeepaliveInterval: &keepalive,
			},
		},
	}

	if err := wgClient.ConfigureDevice(cfg.InterfaceName, wgCfg); err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to configure WireGuard device",
			Cause:   err,
		}
	}

	return nil
}

// parseEndpoint parses a WireGuard endpoint string (host:port).
func parseEndpoint(endpoint string) (*net.UDPAddr, error) {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint format: %w", err)
	}

	// Resolve the host to an IP
	ips, err := net.LookupIP(host)
	if err != nil {
		// Try parsing as IP directly
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("failed to resolve endpoint host: %w", err)
		}
		ips = []net.IP{ip}
	}

	// Parse the port
	var portNum int
	_, err = fmt.Sscanf(port, "%d", &portNum)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	// Prefer IPv4
	var ip net.IP
	for _, addr := range ips {
		if v4 := addr.To4(); v4 != nil {
			ip = v4
			break
		}
	}
	if ip == nil && len(ips) > 0 {
		ip = ips[0]
	}
	if ip == nil {
		return nil, fmt.Errorf("no IP address found for endpoint")
	}

	return &net.UDPAddr{
		IP:   ip,
		Port: portNum,
	}, nil
}

// GetTunnelStatus returns information about an existing tunnel.
func GetTunnelStatus(ctx context.Context, interfaceName string) (*TunnelStatus, error) {
	if interfaceName == "" {
		interfaceName = InterfaceName
	}

	// Check if interface exists
	if !interfaceExists(interfaceName) {
		return nil, ErrInterfaceNotFound
	}

	// Create wgctrl client
	wgClient, err := wgctrl.New()
	if err != nil {
		return nil, &TunnelError{
			Op:      "status",
			Message: "failed to create wgctrl client",
			Cause:   err,
		}
	}
	defer wgClient.Close()

	// Get device info
	device, err := wgClient.Device(interfaceName)
	if err != nil {
		return nil, &TunnelError{
			Op:      "status",
			Message: "failed to get device info",
			Cause:   err,
		}
	}

	status := &TunnelStatus{
		InterfaceName: interfaceName,
		PublicKey:     device.PublicKey.String(),
	}

	// Get peer info
	if len(device.Peers) > 0 {
		peer := device.Peers[0]
		status.PeerPublicKey = peer.PublicKey.String()
		if peer.Endpoint != nil {
			status.PeerEndpoint = peer.Endpoint.String()
		}
		status.LastHandshake = peer.LastHandshakeTime
		status.RxBytes = peer.ReceiveBytes
		status.TxBytes = peer.TransmitBytes
		status.Connected = !peer.LastHandshakeTime.IsZero() &&
			time.Since(peer.LastHandshakeTime) < 3*time.Minute
	}

	return status, nil
}

// TunnelStatus represents the current status of a WireGuard tunnel.
type TunnelStatus struct {
	// InterfaceName is the name of the WireGuard interface.
	InterfaceName string
	// PublicKey is the public key of the local interface.
	PublicKey string
	// PeerPublicKey is the public key of the peer.
	PeerPublicKey string
	// PeerEndpoint is the endpoint address of the peer.
	PeerEndpoint string
	// LastHandshake is the time of the last successful handshake.
	LastHandshake time.Time
	// RxBytes is the number of bytes received.
	RxBytes int64
	// TxBytes is the number of bytes transmitted.
	TxBytes int64
	// Connected indicates if the tunnel appears to be connected (recent handshake).
	Connected bool
}

// IsConnected returns true if the tunnel appears to be connected.
func (ts *TunnelStatus) IsConnected() bool {
	return ts.Connected
}
