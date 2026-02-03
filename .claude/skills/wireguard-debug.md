# wireguard-debug

Diagnose WireGuard tunnel issues and suggest fixes.

## Trigger
User invokes `/wireguard-debug` or reports WireGuard connectivity issues.

## Instructions

1. **Check WireGuard installation**:
   ```bash
   which wg wg-quick
   wg --version
   ```

2. **List active interfaces**:
   ```bash
   sudo wg show
   ```

3. **Check interface status**:
   ```bash
   ip link show wg-spinup 2>/dev/null || echo "Interface not found"
   # macOS alternative:
   ifconfig utun* 2>/dev/null
   ```

4. **Verify handshake**:
   ```bash
   sudo wg show wg-spinup latest-handshakes
   ```
   - If no handshake in last 3 minutes, connection likely failed

5. **Check routing**:
   ```bash
   ip route | grep 10.13.37
   # macOS:
   netstat -rn | grep 10.13.37
   ```

6. **Test connectivity**:
   ```bash
   ping -c 3 10.13.37.1
   ```

7. **Check endpoint reachability**:
   ```bash
   nc -zvu [ENDPOINT_IP] 51820
   ```

8. **Analyze config file**:
   - Verify private/public key format
   - Check allowed IPs
   - Verify endpoint format

9. **Common issues to check**:
   - Firewall blocking UDP 51820
   - NAT traversal issues (PersistentKeepalive needed)
   - Key mismatch between client/server
   - Incorrect AllowedIPs
   - Interface already exists from previous run

## Output Format

```
## WireGuard Diagnostic Report

### Status
- Interface: wg-spinup [UP/DOWN/MISSING]
- Handshake: [TIME] ago / Never
- Transfer: RX [bytes] / TX [bytes]

### Connectivity
- Local endpoint: 10.13.37.2 [OK/FAIL]
- Remote endpoint: 10.13.37.1 [OK/FAIL]
- UDP 51820: [OPEN/BLOCKED/UNKNOWN]

### Issues Found
1. [Issue description]
   - Cause: [explanation]
   - Fix: [command or action]

### Suggested Actions
1. [First action to try]
2. [Second action if first fails]
```
