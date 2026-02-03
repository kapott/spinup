//go:build darwin

// Package wireguard provides WireGuard tunnel management for macOS systems.
package wireguard

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
)

// TunnelConfig holds the configuration needed to set up a WireGuard tunnel.
type TunnelConfig struct {
	// InterfaceName is the name of the WireGuard interface (default: "wg-continueplz").
	// On macOS, this is typically a utun interface (e.g., utun5).
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
	// InterfaceName is the name of the WireGuard interface (e.g., utun5).
	InterfaceName string
	// Config is the configuration used to create this tunnel.
	Config *TunnelConfig
	// wireguardGoCmd is the wireguard-go process if running in userspace mode.
	wireguardGoCmd *exec.Cmd
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

// ErrWireGuardGoNotFound is returned when wireguard-go is not installed.
var ErrWireGuardGoNotFound = &TunnelError{
	Op:      "check",
	Message: "wireguard-go not found in PATH; install via 'brew install wireguard-go' or 'brew install wireguard-tools'",
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
// Returns true if we have root privileges or sudo is available without password.
func checkRootOrSudo() bool {
	if checkRoot() {
		return true
	}
	// Check if sudo is available and we can use it without password
	cmd := exec.Command("sudo", "-n", "true")
	return cmd.Run() == nil
}

// findWireGuardGo searches for the wireguard-go binary.
// It checks common installation paths and the system PATH.
func findWireGuardGo() (string, error) {
	// Check common paths on macOS
	commonPaths := []string{
		"/opt/homebrew/bin/wireguard-go", // Apple Silicon Homebrew
		"/usr/local/bin/wireguard-go",    // Intel Homebrew or manual install
		"/usr/bin/wireguard-go",          // System install
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Check system PATH
	path, err := exec.LookPath("wireguard-go")
	if err == nil {
		return path, nil
	}

	return "", ErrWireGuardGoNotFound
}

// findWgTool searches for the wg command-line tool.
func findWgTool() (string, error) {
	// Check common paths on macOS
	commonPaths := []string{
		"/opt/homebrew/bin/wg", // Apple Silicon Homebrew
		"/usr/local/bin/wg",    // Intel Homebrew
		"/usr/bin/wg",          // System install
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Check system PATH
	path, err := exec.LookPath("wg")
	if err == nil {
		return path, nil
	}

	return "", &TunnelError{
		Op:      "check",
		Message: "wg tool not found in PATH; install via 'brew install wireguard-tools'",
	}
}

// SetupTunnel creates and configures a WireGuard tunnel on macOS.
// This function requires root privileges or sudo access.
// It uses wireguard-go for userspace WireGuard implementation.
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

	// Find wireguard-go
	wgGoPath, err := findWireGuardGo()
	if err != nil {
		return nil, err
	}

	// Find wg tool for configuration
	wgPath, err := findWgTool()
	if err != nil {
		return nil, err
	}

	// Check if an interface with our name already has a running wireguard-go
	// On macOS, wireguard-go creates utun interfaces dynamically
	existingIface := findExistingInterface(cfg.InterfaceName)
	if existingIface != "" {
		return nil, &TunnelError{
			Op:      "create",
			Message: fmt.Sprintf("interface %s already exists", existingIface),
		}
	}

	// Start wireguard-go to create a utun interface
	actualInterface, wgCmd, err := startWireGuardGo(ctx, cfg.InterfaceName, wgGoPath, usesSudo)
	if err != nil {
		return nil, err
	}

	// Create tunnel object early so we can clean up on failure
	tunnel := &Tunnel{
		InterfaceName:  actualInterface,
		Config:         cfg,
		wireguardGoCmd: wgCmd,
	}

	// Configure the WireGuard interface using wg tool
	if err := configureWireGuardDarwin(ctx, actualInterface, cfg, wgPath, usesSudo); err != nil {
		// Clean up on failure
		_ = tunnel.Teardown(ctx)
		return nil, err
	}

	// Assign IP address to the interface
	if err := assignAddressDarwin(ctx, actualInterface, cfg.ClientAddress, usesSudo); err != nil {
		// Clean up on failure
		_ = tunnel.Teardown(ctx)
		return nil, err
	}

	// Bring the interface up
	if err := bringInterfaceUpDarwin(ctx, actualInterface, usesSudo); err != nil {
		// Clean up on failure
		_ = tunnel.Teardown(ctx)
		return nil, err
	}

	// Add route to server
	if err := addRouteDarwin(ctx, actualInterface, cfg.ServerAllowedIPs, usesSudo); err != nil {
		// Route might already exist, log but don't fail
	}

	return tunnel, nil
}

// TeardownTunnel removes a WireGuard tunnel on macOS.
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

	// On macOS, we need to find the actual utun interface
	actualInterface := findExistingInterface(interfaceName)
	if actualInterface == "" {
		// Interface doesn't exist, nothing to do
		return nil
	}

	// Try to kill wireguard-go process by looking at socket file
	killWireGuardGoProcess(actualInterface)

	// Delete the route if it exists
	_ = removeRouteDarwin(ctx, actualInterface, ServerAllowedIPs, usesSudo)

	// The interface should be removed when wireguard-go exits
	// But we can also try to bring it down
	_ = bringInterfaceDownDarwin(ctx, actualInterface, usesSudo)

	return nil
}

// Teardown removes this tunnel.
func (t *Tunnel) Teardown(ctx context.Context) error {
	if t.wireguardGoCmd != nil && t.wireguardGoCmd.Process != nil {
		// Kill the wireguard-go process
		_ = t.wireguardGoCmd.Process.Kill()
		_ = t.wireguardGoCmd.Wait()
	}
	return TeardownTunnel(ctx, t.InterfaceName)
}

// findExistingInterface searches for an existing WireGuard interface.
// Returns the actual interface name (e.g., utun5) or empty string if not found.
func findExistingInterface(name string) string {
	// Check for socket file in /var/run/wireguard/
	socketPath := filepath.Join("/var/run/wireguard", name+".sock")
	if _, err := os.Stat(socketPath); err == nil {
		// Socket exists, find the corresponding utun interface
		// Read the socket name to get the interface
		return findUtunForSocket(name)
	}

	// Also check if interface directly matches (utun*)
	if strings.HasPrefix(name, "utun") {
		if interfaceExists(name) {
			return name
		}
	}

	return ""
}

// findUtunForSocket attempts to find the utun interface for a named socket.
func findUtunForSocket(name string) string {
	// Try to use wgctrl to find the device
	wgClient, err := wgctrl.New()
	if err != nil {
		return ""
	}
	defer wgClient.Close()

	devices, err := wgClient.Devices()
	if err != nil {
		return ""
	}

	for _, device := range devices {
		if device.Name == name || strings.HasPrefix(device.Name, "utun") {
			return device.Name
		}
	}

	return ""
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
func runCommand(ctx context.Context, usesSudo bool, name string, args ...string) ([]byte, error) {
	var cmd *exec.Cmd
	if usesSudo {
		sudoArgs := append([]string{name}, args...)
		cmd = exec.CommandContext(ctx, "sudo", sudoArgs...)
	} else {
		cmd = exec.CommandContext(ctx, name, args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, &TunnelError{
			Op:      "exec",
			Message: fmt.Sprintf("command '%s %s' failed: %s", name, strings.Join(args, " "), string(output)),
			Cause:   err,
		}
	}
	return output, nil
}

// startWireGuardGo starts the wireguard-go process and returns the interface name.
func startWireGuardGo(ctx context.Context, name string, wgGoPath string, usesSudo bool) (string, *exec.Cmd, error) {
	// Create a socket directory if it doesn't exist
	socketDir := "/var/run/wireguard"
	if usesSudo {
		_, _ = runCommand(ctx, true, "mkdir", "-p", socketDir)
	} else {
		_ = os.MkdirAll(socketDir, 0755)
	}

	// Start wireguard-go with the interface name
	// wireguard-go on macOS creates a utun interface and names it based on the argument
	var cmd *exec.Cmd
	if usesSudo {
		cmd = exec.CommandContext(ctx, "sudo", wgGoPath, name)
	} else {
		cmd = exec.CommandContext(ctx, wgGoPath, name)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return "", nil, &TunnelError{
			Op:      "create",
			Message: "failed to start wireguard-go",
			Cause:   err,
		}
	}

	// Wait a moment for the interface to be created
	time.Sleep(500 * time.Millisecond)

	// Find the actual interface name (utun*)
	actualInterface := waitForInterface(name, 5*time.Second)
	if actualInterface == "" {
		// Kill the process if we couldn't find the interface
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return "", nil, &TunnelError{
			Op:      "create",
			Message: "wireguard-go started but interface was not created",
		}
	}

	return actualInterface, cmd, nil
}

// waitForInterface waits for a WireGuard interface to appear.
func waitForInterface(name string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Try wgctrl first
		wgClient, err := wgctrl.New()
		if err == nil {
			devices, err := wgClient.Devices()
			if err == nil {
				for _, device := range devices {
					// On macOS, wireguard-go creates utun interfaces
					// The device name matches what was passed to wireguard-go
					if device.Name == name || strings.HasPrefix(device.Name, "utun") {
						wgClient.Close()
						return device.Name
					}
				}
			}
			wgClient.Close()
		}

		// Also check for socket file
		socketPath := filepath.Join("/var/run/wireguard", name+".sock")
		if _, err := os.Stat(socketPath); err == nil {
			// Socket exists, interface should be ready
			// Try to find the utun name again
			wgClient, err := wgctrl.New()
			if err == nil {
				devices, _ := wgClient.Devices()
				wgClient.Close()
				for _, device := range devices {
					return device.Name
				}
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
	return ""
}

// configureWireGuardDarwin configures the WireGuard interface using the wg tool.
func configureWireGuardDarwin(ctx context.Context, interfaceName string, cfg *TunnelConfig, wgPath string, usesSudo bool) error {
	// Create a temporary config file
	configContent := fmt.Sprintf(`[Interface]
PrivateKey = %s

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = %d
`, cfg.ClientPrivateKey, cfg.ServerPublicKey, cfg.ServerEndpoint, cfg.ServerAllowedIPs, cfg.PersistentKeepalive)

	// Write to a temporary file
	tmpFile, err := os.CreateTemp("", "wg-config-*.conf")
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to create temporary config file",
			Cause:   err,
		}
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(configContent); err != nil {
		tmpFile.Close()
		return &TunnelError{
			Op:      "configure",
			Message: "failed to write config file",
			Cause:   err,
		}
	}
	tmpFile.Close()

	// Set proper permissions
	if err := os.Chmod(tmpPath, 0600); err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to set config file permissions",
			Cause:   err,
		}
	}

	// Apply configuration using wg setconf
	_, err = runCommand(ctx, usesSudo, wgPath, "setconf", interfaceName, tmpPath)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to configure WireGuard device",
			Cause:   err,
		}
	}

	return nil
}

// assignAddressDarwin assigns an IP address to an interface on macOS.
func assignAddressDarwin(ctx context.Context, name, address string, usesSudo bool) error {
	// Parse the address to get IP and netmask
	ip, ipNet, err := net.ParseCIDR(address)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to parse client address",
			Cause:   err,
		}
	}

	// Calculate the network address and mask
	maskSize, _ := ipNet.Mask.Size()
	netmaskStr := fmt.Sprintf("%d.%d.%d.%d",
		ipNet.Mask[0], ipNet.Mask[1], ipNet.Mask[2], ipNet.Mask[3])

	// On macOS, use ifconfig to set the address
	// ifconfig utunX inet 10.13.37.2 10.13.37.1 netmask 255.255.255.0
	// The format is: ifconfig <interface> inet <local-addr> <dest-addr> netmask <mask>
	destIP := ServerIP // Point-to-point destination
	if maskSize == 32 {
		// For /32, destination is same as source
		destIP = ip.String()
	}

	_, err = runCommand(ctx, usesSudo, "ifconfig", name, "inet", ip.String(), destIP, "netmask", netmaskStr)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: fmt.Sprintf("failed to assign address %s to interface %s", address, name),
			Cause:   err,
		}
	}
	return nil
}

// bringInterfaceUpDarwin brings a network interface up on macOS.
func bringInterfaceUpDarwin(ctx context.Context, name string, usesSudo bool) error {
	_, err := runCommand(ctx, usesSudo, "ifconfig", name, "up")
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: fmt.Sprintf("failed to bring up interface %s", name),
			Cause:   err,
		}
	}
	return nil
}

// bringInterfaceDownDarwin brings a network interface down on macOS.
func bringInterfaceDownDarwin(ctx context.Context, name string, usesSudo bool) error {
	_, err := runCommand(ctx, usesSudo, "ifconfig", name, "down")
	return err
}

// addRouteDarwin adds a route through the WireGuard interface on macOS.
func addRouteDarwin(ctx context.Context, name, cidr string, usesSudo bool) error {
	// Parse the CIDR to get the network address
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: "failed to parse CIDR for route",
			Cause:   err,
		}
	}

	// On macOS, use route add -net <network>/<prefix> -interface <iface>
	maskSize, _ := ipNet.Mask.Size()
	_, err = runCommand(ctx, usesSudo, "route", "add", "-net",
		fmt.Sprintf("%s/%d", ipNet.IP.String(), maskSize),
		"-interface", name)
	if err != nil {
		return &TunnelError{
			Op:      "configure",
			Message: fmt.Sprintf("failed to add route %s via interface %s", cidr, name),
			Cause:   err,
		}
	}
	return nil
}

// removeRouteDarwin removes a route on macOS.
func removeRouteDarwin(ctx context.Context, name, cidr string, usesSudo bool) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	maskSize, _ := ipNet.Mask.Size()
	_, err = runCommand(ctx, usesSudo, "route", "delete", "-net",
		fmt.Sprintf("%s/%d", ipNet.IP.String(), maskSize),
		"-interface", name)
	return err
}

// killWireGuardGoProcess attempts to kill the wireguard-go process for an interface.
func killWireGuardGoProcess(interfaceName string) {
	// Try to find and kill by socket file
	socketPath := filepath.Join("/var/run/wireguard", interfaceName+".sock")

	// Try to find process using lsof
	output, err := exec.Command("lsof", socketPath).Output()
	if err == nil && len(output) > 0 {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines[1:] { // Skip header
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				pid := fields[1]
				pidInt, err := strconv.Atoi(pid)
				if err == nil {
					proc, err := os.FindProcess(pidInt)
					if err == nil {
						_ = proc.Kill()
					}
				}
			}
		}
	}

	// Also try to remove the socket file
	_ = os.Remove(socketPath)
}

// GetTunnelStatus returns information about an existing tunnel.
func GetTunnelStatus(ctx context.Context, interfaceName string) (*TunnelStatus, error) {
	if interfaceName == "" {
		interfaceName = InterfaceName
	}

	// Find the actual interface (might be utun*)
	actualInterface := findExistingInterface(interfaceName)
	if actualInterface == "" {
		// Try the name directly
		if !interfaceExists(interfaceName) {
			return nil, ErrInterfaceNotFound
		}
		actualInterface = interfaceName
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
	device, err := wgClient.Device(actualInterface)
	if err != nil {
		return nil, &TunnelError{
			Op:      "status",
			Message: "failed to get device info",
			Cause:   err,
		}
	}

	status := &TunnelStatus{
		InterfaceName: actualInterface,
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
