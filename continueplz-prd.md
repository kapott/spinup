# Product Requirements Document: continueplz

**Version:** 1.0
**Author:** Claude (voor Claude Code implementatie)
**Date:** 2026-02-02
**Status:** Draft

---

## 1. Executive Summary

**continueplz** is een command-line tool geschreven in Go die ontwikkelaars in staat stelt om met één commando een beveiligde, ephemeral GPU-instance op te spinnen met een code-assist LLM. De tool vergelijkt real-time prijzen van meerdere cloud GPU providers, deploy het gekozen model, zet een WireGuard tunnel op voor veilige connectie, en ruimt alles gegarandeerd op wanneer gewenst.

### Kernprincipes

1. **Sovereignty** - Jij controleert de dataflow, het model draait op dedicated hardware
2. **Kostenbewust** - Real-time prijsvergelijking, spot vs on-demand, per-seconde billing awareness
3. **Betrouwbaar** - Stop mag niet falen, deadman switch als backup, alerting bij problemen
4. **Eenvoudig** - Één binary, één commando, klaar

---

## 2. User Stories

### 2.1 Primary User Stories

**US-1: Quick Start met Goedkoopste Optie**
```
Als developer
Wil ik met één commando de goedkoopste GPU-instance starten met mijn gewenste model
Zodat ik direct aan de slag kan zonder prijzen te vergelijken
```
Acceptance: `continueplz --cheapest --model=qwen2.5-coder:32b` start instance en geeft IP terug

**US-2: Interactieve Provider/Model Selectie**
```
Als developer
Wil ik een overzichtelijk menu zien met prijzen en modellen
Zodat ik een geïnformeerde keuze kan maken
```
Acceptance: `continueplz` zonder args toont TUI met provider/model selectie

**US-3: Gegarandeerd Stoppen**
```
Als developer
Wil ik zeker weten dat mijn instance stopt en billing eindigt
Zodat ik geen onverwachte kosten krijg
```
Acceptance: `continueplz` (als instance draait) stopt alles, bevestigt dat billing gestopt is

**US-4: Cronjob Automation**
```
Als developer
Wil ik de tool in een cronjob kunnen draaien
Zodat mijn instance automatisch start om 08:30 en stopt om 17:00
```
Acceptance: Non-interactive mode met exit codes en logging geschikt voor cron

**US-5: Eerste Configuratie**
```
Als nieuwe gebruiker
Wil ik een guided setup voor API keys en voorkeuren
Zodat ik snel aan de slag kan
```
Acceptance: `continueplz init` begeleidt door credential setup

---

## 3. Functional Requirements

### 3.1 Providers

De tool MOET de volgende providers ondersteunen:

| Provider | Priority | API Type | Spot Support |
|----------|----------|----------|--------------|
| Vast.ai | P0 | REST API | Ja |
| Lambda Labs | P0 | REST API | Nee |
| RunPod | P0 | GraphQL | Ja |
| CoreWeave | P1 | Kubernetes API | Ja |
| Paperspace | P1 | REST API | Nee |

**Per provider MOET de tool kunnen:**
- Real-time prijzen ophalen (compute, egress, storage apart)
- Beschikbare GPU types en regions ophalen
- Instance starten met cloud-init/startup script
- Instance status opvragen
- Instance termineren
- Billing status verifiëren (waar API dit ondersteunt)

### 3.2 Commands

#### `continueplz init`

Interactieve setup wizard:

```
$ continueplz init

╭─────────────────────────────────────────────────────────╮
│                   continueplz setup                      │
╰─────────────────────────────────────────────────────────╯

Let's configure your GPU providers.

? Which providers do you want to configure?
  [x] Vast.ai
  [x] Lambda Labs
  [x] RunPod
  [ ] CoreWeave
  [ ] Paperspace

? Vast.ai API Key: ****************************************
  ✓ Valid - Account: user@example.com, Balance: $50.00

? Lambda Labs API Key: ****************************************
  ✓ Valid - Account: user@example.com

? RunPod API Key: ****************************************
  ✓ Valid - Account: user@example.com, Balance: $25.00

? WireGuard private key (leave empty to generate): 
  ✓ Generated new keypair

? Default model tier:
  ○ small  (7-14B params, faster, cheaper)
  ● medium (32B params, balanced)
  ○ large  (70B+ params, best quality, expensive)

? Deadman switch timeout (hours): 10

✓ Configuration saved to .env
✓ State file initialized

Run 'continueplz' to start your first instance!
```

**Output:** Creëert `.env` bestand en initialiseert state.

#### `continueplz` (geen args, geen actieve instance)

Toont interactieve TUI:

```
╭─────────────────────────────────────────────────────────────────────────────╮
│                              continueplz                                     │
│                         GPU Code Assistant Launcher                          │
╰─────────────────────────────────────────────────────────────────────────────╯

┌─ Available Configurations ───────────────────────────────────────────────────┐
│                                                                              │
│  Provider      GPU            Region      Spot/hr   OnDemand/hr   Day Est.  │
│  ─────────────────────────────────────────────────────────────────────────── │
│  vast.ai       A100 40GB      EU-West     €0.65     €0.95         €5.20 *   │
│  vast.ai       A100 80GB      US-East     €0.89     €1.35         €7.12     │
│  lambda        A100 40GB      US-West     -         €1.10         €8.80     │
│  runpod        A100 40GB      EU-Central  €0.75     €1.29         €6.00     │
│  runpod        A6000 48GB     US-East     €0.45     €0.79         €3.60     │
│  paperspace    A100 80GB      US-East     -         €1.89         €15.12    │
│                                                                              │
│  * Currently selected                                            [↑↓] Navigate│
│                                                                  [Enter] Select│
│                                                                  [q] Quit     │
└──────────────────────────────────────────────────────────────────────────────┘

┌─ Model Selection ────────────────────────────────────────────────────────────┐
│                                                                              │
│  Model                      Size    VRAM     Quality   Compatible GPUs       │
│  ───────────────────────────────────────────────────────────────────────────│
│  qwen2.5-coder:7b          7B      ~8GB     ★★★☆☆    A6000, A100 40/80     │
│  qwen2.5-coder:14b         14B     ~16GB    ★★★★☆    A6000, A100 40/80     │
│  qwen2.5-coder:32b         32B     ~35GB    ★★★★★    A100 40/80            │ *
│  deepseek-coder:33b        33B     ~36GB    ★★★★★    A100 40/80            │
│  codellama:70b-q4          70B     ~40GB    ★★★★★    A100 80               │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘

┌─ Cost Breakdown (Selected: vast.ai A100 40GB Spot + qwen2.5-coder:32b) ──────┐
│                                                                              │
│  Compute:  €0.65/hr  ×  8hr  =  €5.20                                       │
│  Storage:  €0.05/hr  ×  8hr  =  €0.40  (100GB model storage)                │
│  Egress:   ~€0.00         (WireGuard tunnel, minimal)                        │
│  ─────────────────────────────────────────────────────────────────────────── │
│  Estimated daily total:     €5.60                                           │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘

                    [Enter] Deploy    [r] Refresh Prices    [q] Quit
```

#### `continueplz` (geen args, instance actief)

Als er een actieve instance is:

```
╭─────────────────────────────────────────────────────────────────────────────╮
│                              continueplz                                     │
│                            Instance Active                                   │
╰─────────────────────────────────────────────────────────────────────────────╯

┌─ Current Instance ───────────────────────────────────────────────────────────┐
│                                                                              │
│  Provider:     vast.ai                                                       │
│  GPU:          A100 40GB                                                     │
│  Region:       EU-West                                                       │
│  Instance ID:  12345678                                                      │
│  Status:       ● Running                                                     │
│                                                                              │
│  Model:        qwen2.5-coder:32b                                            │
│  Model Status: ● Loaded and ready                                           │
│                                                                              │
│  Started:      08:32:15 (4h 27m ago)                                        │
│  Deadman:      Active (kills in 5h 33m if no heartbeat)                     │
│                                                                              │
│  WireGuard:    ● Connected                                                   │
│  Endpoint:     10.13.37.2:11434                                             │
│                                                                              │
│  Current cost: €2.93                                                        │
│  Projected:    €5.60 (at 17:00)                                             │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘

┌─ Quick Actions ──────────────────────────────────────────────────────────────┐
│                                                                              │
│  [s] Stop instance and cleanup                                              │
│  [t] Test connection (send ping to model)                                   │
│  [l] View logs                                                              │
│  [q] Quit (instance keeps running)                                          │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Bij [s] Stop:**

```
╭─────────────────────────────────────────────────────────────────────────────╮
│                           Stopping Instance                                  │
╰─────────────────────────────────────────────────────────────────────────────╯

  [✓] Terminating instance 12345678 on vast.ai...
  [✓] Instance terminated
  [✓] Verifying billing stopped...
  [✓] Billing confirmed stopped
  [✓] Removing WireGuard tunnel...
  [✓] Tunnel removed
  [✓] Cleaning up state...
  [✓] Done

Total session cost: €2.93
Session duration: 4h 28m

Press any key to exit...
```

**Bij Stop zonder billing verificatie API (bijv. sommige providers):**

```
╭─────────────────────────────────────────────────────────────────────────────╮
│                           Stopping Instance                                  │
╰─────────────────────────────────────────────────────────────────────────────╯

  [✓] Terminating instance 12345678 on paperspace...
  [✓] Instance terminated
  [⚠] Billing verification not available for this provider
  [✓] Removing WireGuard tunnel...
  [✓] Tunnel removed
  [✓] Cleaning up state...
  [✓] Done

╭─────────────────────────────────────────────────────────────────────────────╮
│  ⚠️  MANUAL VERIFICATION REQUIRED                                            │
│                                                                              │
│  Paperspace does not provide a billing status API.                          │
│  Please verify manually that the instance is terminated:                    │
│                                                                              │
│  1. Open: https://console.paperspace.com/machines                           │
│  2. Confirm instance 12345678 shows as "Terminated"                         │
│  3. Check that no charges are accruing                                      │
│                                                                              │
│  Instance ID: 12345678                                                      │
│                                                                              │
╰─────────────────────────────────────────────────────────────────────────────╯

Total session cost: €2.93 (estimated)
Session duration: 4h 28m

Press any key to exit...
```

#### `continueplz --cheapest --model=<model>`

Non-interactive deployment:

```
$ continueplz --cheapest --model=qwen2.5-coder:32b

continueplz v1.0.0 - Starting deployment

[1/8] Fetching prices from 5 providers...
      ✓ vast.ai: 3 offers
      ✓ lambda: 2 offers  
      ✓ runpod: 4 offers
      ✓ coreweave: 1 offer
      ✓ paperspace: 2 offers

[2/8] Selecting cheapest compatible option...
      ✓ Selected: vast.ai A100 40GB EU-West @ €0.65/hr spot

[3/8] Creating instance...
      ✓ Instance 12345678 created

[4/8] Waiting for instance to boot...
      ⠋ Booting... (23s)
      ✓ Instance running

[5/8] Configuring WireGuard tunnel...
      ✓ Tunnel configured
      ✓ Connection verified

[6/8] Installing Ollama and pulling model...
      ⠋ Pulling qwen2.5-coder:32b... 45%
      ✓ Model ready

[7/8] Configuring deadman switch...
      ✓ Deadman active (10h timeout)

[8/8] Verifying service health...
      ✓ Model responding

═══════════════════════════════════════════════════════════════════════════════

  ✓ READY

  Provider:    vast.ai (spot)
  GPU:         A100 40GB
  Model:       qwen2.5-coder:32b
  
  Endpoint:    10.13.37.2:11434
  
  Cost:        €0.65/hr (€5.20/8hr day)
  
  Stop with:   continueplz --stop
               or: continueplz (interactive)

═══════════════════════════════════════════════════════════════════════════════
```

**Exit code:** 0 on success, non-zero on failure

#### `continueplz --stop`

Non-interactive stop:

```
$ continueplz --stop

continueplz v1.0.0 - Stopping instance

[1/4] Terminating instance 12345678...
      ✓ Terminated

[2/4] Verifying billing stopped...
      ✓ Confirmed

[3/4] Removing WireGuard tunnel...
      ✓ Removed

[4/4] Cleaning state...
      ✓ Done

Session cost: €2.93
Duration: 4h 28m
```

**Exit code:** 0 on success, 1 on failure (met ERROR in log)

#### `continueplz status`

```
$ continueplz status

continueplz v1.0.0 - Status

Instance:     ● Active
Provider:     vast.ai
GPU:          A100 40GB
Model:        qwen2.5-coder:32b
Endpoint:     10.13.37.2:11434
Running:      4h 28m
Cost so far:  €2.93
Deadman:      5h 32m remaining

# Of als er geen instance is:

Instance:     ○ None active
```

#### `continueplz --output=json`

Alle commands ondersteunen JSON output:

```json
{
  "status": "ready",
  "instance": {
    "id": "12345678",
    "provider": "vast.ai",
    "gpu": "A100 40GB",
    "region": "EU-West",
    "type": "spot"
  },
  "model": "qwen2.5-coder:32b",
  "endpoint": {
    "wireguard_ip": "10.13.37.2",
    "port": 11434,
    "url": "http://10.13.37.2:11434"
  },
  "cost": {
    "hourly": 0.65,
    "current": 2.93,
    "currency": "EUR"
  },
  "deadman": {
    "active": true,
    "remaining_seconds": 19920
  }
}
```

### 3.3 CLI Flags (Volledig)

```
continueplz - Ephemeral GPU Code Assistant

Usage:
  continueplz [flags]
  continueplz [command]

Commands:
  init        Configure providers and generate .env
  status      Show current instance status
  
Flags:
  --cheapest              Select cheapest compatible provider/GPU automatically
  --provider=<name>       Force specific provider (vast, lambda, runpod, coreweave, paperspace)
  --gpu=<type>            Force specific GPU type (a100-40, a100-80, a6000, h100)
  --model=<name>          Model to deploy (e.g., qwen2.5-coder:32b)
  --tier=<size>           Model tier: small, medium, large (default: medium)
  --spot                  Prefer spot instances (default: true)
  --on-demand             Force on-demand instances
  --region=<region>       Preferred region (eu-west, us-east, etc.)
  --stop                  Stop running instance
  --output=<format>       Output format: text, json (default: text)
  --timeout=<duration>    Deadman switch timeout (default: 10h)
  --yes, -y               Skip confirmations
  --verbose, -v           Verbose logging (-vv for debug)
  --version               Show version
  --help, -h              Show help

Examples:
  continueplz                                    # Interactive TUI
  continueplz --cheapest --model=qwen2.5-coder:32b
  continueplz --provider=lambda --gpu=a100-80 --model=codellama:70b
  continueplz --stop
  continueplz status --output=json
```

### 3.4 Model Registry

De tool MOET een ingebouwde registry hebben van coding models met hun VRAM requirements:

```go
var ModelRegistry = []Model{
    // Small tier (7-14B)
    {Name: "qwen2.5-coder:7b", Params: "7B", VRAM: 8, Quality: 3, Tier: "small"},
    {Name: "deepseek-coder:6.7b", Params: "6.7B", VRAM: 8, Quality: 3, Tier: "small"},
    {Name: "codellama:7b", Params: "7B", VRAM: 8, Quality: 2, Tier: "small"},
    {Name: "starcoder2:7b", Params: "7B", VRAM: 8, Quality: 3, Tier: "small"},
    
    // Medium tier (14-35B)
    {Name: "qwen2.5-coder:14b", Params: "14B", VRAM: 16, Quality: 4, Tier: "medium"},
    {Name: "qwen2.5-coder:32b", Params: "32B", VRAM: 35, Quality: 5, Tier: "medium"},
    {Name: "deepseek-coder:33b", Params: "33B", VRAM: 36, Quality: 5, Tier: "medium"},
    {Name: "codellama:34b", Params: "34B", VRAM: 36, Quality: 4, Tier: "medium"},
    
    // Large tier (70B+)
    {Name: "codellama:70b", Params: "70B", VRAM: 40, Quality: 5, Tier: "large"},
    {Name: "qwen2.5-coder:72b", Params: "72B", VRAM: 45, Quality: 5, Tier: "large"},
    {Name: "deepseek-coder-v2:236b", Params: "236B", VRAM: 120, Quality: 5, Tier: "large"},
}

var GPURegistry = []GPU{
    {Name: "A6000", VRAM: 48, Providers: []string{"vast", "runpod"}},
    {Name: "A100-40GB", VRAM: 40, Providers: []string{"vast", "lambda", "runpod", "coreweave", "paperspace"}},
    {Name: "A100-80GB", VRAM: 80, Providers: []string{"vast", "lambda", "runpod", "coreweave", "paperspace"}},
    {Name: "H100-80GB", VRAM: 80, Providers: []string{"lambda", "coreweave"}},
}
```

---

## 4. Security Requirements

### 4.1 WireGuard Tunnel

**Setup flow:**

1. Bij `init`: Genereer WireGuard keypair, sla private key op in `.env`
2. Bij deploy: 
   - Genereer per-instance server keypair
   - Inject public key in cloud-init
   - Instance start WireGuard met client public key
   - Tool configureert lokale WireGuard interface
3. Bij stop: Verwijder lokale WireGuard interface

**WireGuard configuratie op instance (cloud-init):**

```yaml
# Gegenereerd door continueplz
wireguard:
  interfaces:
    wg0:
      private_key: ${SERVER_PRIVATE_KEY}
      listen_port: 51820
      addresses:
        - 10.13.37.1/24
      peers:
        - public_key: ${CLIENT_PUBLIC_KEY}
          allowed_ips:
            - 10.13.37.2/32
```

**Lokale WireGuard configuratie:**

```ini
[Interface]
PrivateKey = ${CLIENT_PRIVATE_KEY}
Address = 10.13.37.2/24

[Peer]
PublicKey = ${SERVER_PUBLIC_KEY}
Endpoint = ${INSTANCE_PUBLIC_IP}:51820
AllowedIPs = 10.13.37.1/32
PersistentKeepalive = 25
```

### 4.2 Firewall Rules

Instance MOET alleen accepteren:
- WireGuard (UDP 51820) van anywhere (nodig voor NAT traversal)
- SSH (TCP 22) alleen via WireGuard interface (10.13.37.0/24)
- Ollama API (TCP 11434) alleen via WireGuard interface

### 4.3 Credential Storage

`.env` bestand:

```bash
# continueplz configuration
# Generated by: continueplz init
# Date: 2026-02-02

# Provider API Keys
VAST_API_KEY=vast_xxxxxxxxxxxxxxxxxxxx
LAMBDA_API_KEY=lambda_xxxxxxxxxxxxxxxxxxxx
RUNPOD_API_KEY=runpod_xxxxxxxxxxxxxxxxxxxx
COREWEAVE_API_KEY=coreweave_xxxxxxxxxxxxxxxxxxxx
PAPERSPACE_API_KEY=paperspace_xxxxxxxxxxxxxxxxxxxx

# WireGuard
WIREGUARD_PRIVATE_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx=
WIREGUARD_PUBLIC_KEY=yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy=

# Preferences
DEFAULT_TIER=medium
DEFAULT_REGION=eu-west
PREFER_SPOT=true
DEADMAN_TIMEOUT_HOURS=10

# Billing alerts (optional)
ALERT_WEBHOOK_URL=https://hooks.slack.com/services/xxx
DAILY_BUDGET_EUR=20
```

File permissions MOETEN 0600 zijn.

---

## 5. Reliability Requirements

### 5.1 Deadman Switch

**Implementatie op instance (cloud-init):**

```bash
#!/bin/bash
# deadman.sh - Runs on instance

TIMEOUT_SECONDS=${DEADMAN_TIMEOUT:-36000}  # 10 hours default
HEARTBEAT_FILE=/tmp/continueplz-heartbeat

# Create initial heartbeat
touch $HEARTBEAT_FILE

while true; do
    sleep 60
    
    # Check if heartbeat file was updated in last TIMEOUT_SECONDS
    if [ $(($(date +%s) - $(stat -c %Y $HEARTBEAT_FILE))) -gt $TIMEOUT_SECONDS ]; then
        echo "Deadman switch triggered - no heartbeat for ${TIMEOUT_SECONDS}s"
        
        # Self-terminate based on provider
        if [ -f /etc/vast-instance ]; then
            curl -X DELETE "https://console.vast.ai/api/v0/instances/${INSTANCE_ID}/" \
                -H "Authorization: Bearer ${VAST_API_KEY}"
        elif [ -f /etc/lambda-instance ]; then
            curl -X POST "https://cloud.lambdalabs.com/api/v1/instance-operations/terminate" \
                -H "Authorization: Bearer ${LAMBDA_API_KEY}" \
                -d '{"instance_ids": ["'${INSTANCE_ID}'"]}'
        # ... etc for other providers
        fi
        
        # Fallback: shutdown anyway
        shutdown -h now
    fi
done
```

**Client-side heartbeat:**

De tool MOET elke 5 minuten een heartbeat sturen zolang de instance actief is in state.

### 5.2 Stop Guarantees

**Stop sequence met retry:**

```go
func (c *Client) Stop() error {
    maxRetries := 5
    baseDelay := 2 * time.Second
    
    for attempt := 1; attempt <= maxRetries; attempt++ {
        err := c.provider.Terminate(c.instanceID)
        if err == nil {
            break
        }
        
        delay := baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
        c.log.Error("Terminate failed, retrying", 
            "attempt", attempt, 
            "delay", delay,
            "error", err)
        
        time.Sleep(delay)
    }
    
    // Verify billing stopped
    for attempt := 1; attempt <= maxRetries; attempt++ {
        status, err := c.provider.GetBillingStatus(c.instanceID)
        if err == nil && status == "stopped" {
            return nil
        }
        
        delay := baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
        time.Sleep(delay)
    }
    
    // Check of provider billing verificatie ondersteunt
    if !c.provider.SupportsBillingVerification() {
        // Manual verification required
        c.log.Warn("Provider does not support billing verification API",
            "instance_id", c.instanceID,
            "provider", c.provider.Name())
        
        c.ui.ShowManualVerificationRequired(c.provider.Name(), c.instanceID, c.provider.ConsoleURL())
        return nil // Not an error, but user must verify manually
    }
    
    // CRITICAL: Als we hier komen, is billing mogelijk nog actief
    c.log.Error("CRITICAL: Could not verify billing stopped",
        "instance_id", c.instanceID,
        "provider", c.provider.Name())
    
    // Alert sturen
    c.sendAlert(AlertCritical, "Billing may still be active for instance "+c.instanceID)
    
    return ErrBillingNotVerified
}
```

### 5.3 Spot Instance Interruption Handling

Als een spot instance interrupted wordt:

1. Provider stuurt termination notice (meestal 30-120 seconden)
2. Instance detecteert dit via metadata service
3. Instance stuurt signal naar client via WireGuard
4. Client ontvangt interrupt, toont in TUI:

```
╭─────────────────────────────────────────────────────────────────────────────╮
│  ⚠️  SPOT INSTANCE INTERRUPTED                                               │
│                                                                              │
│  Your spot instance was reclaimed by the provider.                          │
│  This is normal for spot instances.                                         │
│                                                                              │
│  Session cost: €2.15                                                        │
│  Duration: 3h 18m                                                           │
│                                                                              │
│  [r] Restart with new instance                                              │
│  [q] Quit                                                                   │
│                                                                              │
╰─────────────────────────────────────────────────────────────────────────────╯
```

Bij `[r]`: Terug naar provider/model selectie scherm.

---

## 6. State Management

### 6.1 State File

Locatie: `.continueplz.state` (zelfde directory als binary)

```json
{
  "version": 1,
  "instance": {
    "id": "12345678",
    "provider": "vast",
    "gpu": "A100-40GB",
    "region": "eu-west",
    "type": "spot",
    "public_ip": "203.0.113.42",
    "wireguard_ip": "10.13.37.1",
    "created_at": "2026-02-02T08:32:15Z"
  },
  "model": {
    "name": "qwen2.5-coder:32b",
    "status": "ready"
  },
  "wireguard": {
    "server_public_key": "xxxx",
    "interface_name": "wg-continueplz"
  },
  "cost": {
    "hourly_rate": 0.65,
    "accumulated": 2.93,
    "currency": "EUR"
  },
  "deadman": {
    "timeout_hours": 10,
    "last_heartbeat": "2026-02-02T13:00:15Z"
  }
}
```

### 6.2 State Reconciliation

Bij elke operatie:

```go
func (c *Client) reconcileState() error {
    // 1. Laad lokale state
    localState, err := c.loadState()
    if err != nil {
        return err
    }
    
    // 2. Als er een instance in state is, verifieer bij provider
    if localState.Instance != nil {
        remoteStatus, err := c.provider.GetInstance(localState.Instance.ID)
        
        if err != nil || remoteStatus == nil {
            // Instance bestaat niet meer bij provider
            c.log.Warn("Local state has instance but provider reports it doesn't exist",
                "instance_id", localState.Instance.ID)
            
            // Toon waarschuwing aan gebruiker
            c.ui.ShowWarning("State mismatch detected",
                "Local state shows an active instance, but the provider reports it no longer exists.\n"+
                "This could mean:\n"+
                "- The instance was terminated externally\n"+
                "- A spot instance was interrupted\n"+
                "- The provider API is temporarily unavailable\n\n"+
                "Cleaning up local state...")
            
            // Cleanup lokale state
            c.clearState()
            return nil
        }
        
        if remoteStatus.Status == "terminated" {
            c.log.Info("Instance was terminated externally, cleaning up state")
            c.clearState()
        }
    }
    
    return nil
}
```

---

## 7. Logging

### 7.1 Log Levels

| Level | Gebruik |
|-------|---------|
| ERROR | Kritieke fouten, stop failures, billing issues |
| WARN | State mismatches, retries, degraded service |
| INFO | Normale operaties, milestones |
| DEBUG | API calls, responses, timings |

### 7.2 Log Format

```
2026-02-02T08:32:15.123Z INFO  Starting deployment provider=vast model=qwen2.5-coder:32b
2026-02-02T08:32:16.456Z DEBUG API request method=POST url=https://console.vast.ai/api/v0/asks/12345/
2026-02-02T08:32:17.789Z INFO  Instance created instance_id=12345678
2026-02-02T08:33:45.012Z ERROR Terminate failed, retrying attempt=1 error="connection timeout"
```

### 7.3 Log File

Locatie: `continueplz.log` (zelfde directory als binary)

Rotatie: Nieuwe file per dag, keep 7 dagen.

### 7.4 Verbose Mode

- `-v`: Toont INFO en hoger naar stderr
- `-vv`: Toont DEBUG en hoger naar stderr

---

## 8. Alerting

### 8.1 Alert Types

| Type | Trigger | Action |
|------|---------|--------|
| CRITICAL | Stop failed, billing not verified | Webhook + ERROR log + TUI flash |
| ERROR | API failures after retries | ERROR log + TUI notification |
| WARN | Spot interruption, state mismatch | WARN log + TUI notification |
| INFO | Session start/stop | INFO log |

### 8.2 Webhook Format

```json
{
  "level": "CRITICAL",
  "message": "Could not verify billing stopped",
  "timestamp": "2026-02-02T17:00:15Z",
  "context": {
    "instance_id": "12345678",
    "provider": "vast.ai",
    "action": "stop"
  }
}
```

### 8.3 TUI Alert Display

Bij CRITICAL errors:

```
╭─────────────────────────────────────────────────────────────────────────────╮
│  🔴 CRITICAL ERROR 🔴                                                        │
│                                                                              │
│  Could not verify that billing has stopped for instance 12345678.           │
│                                                                              │
│  IMMEDIATE ACTION REQUIRED:                                                  │
│  1. Log into vast.ai console                                                │
│  2. Verify instance 12345678 is terminated                                  │
│  3. If still running, terminate manually                                    │
│                                                                              │
│  Error details logged to: continueplz.log                                   │
│                                                                              │
╰─────────────────────────────────────────────────────────────────────────────╯
```

De border MOET rood knipperen (ANSI escape codes).

---

## 9. Technical Architecture

### 9.1 Project Structure

```
continueplz/
├── cmd/
│   └── continueplz/
│       └── main.go           # Entry point
├── internal/
│   ├── config/
│   │   ├── config.go         # Config loading from .env
│   │   └── state.go          # State file management
│   ├── provider/
│   │   ├── provider.go       # Provider interface
│   │   ├── vast/
│   │   │   └── vast.go       # Vast.ai implementation
│   │   ├── lambda/
│   │   │   └── lambda.go     # Lambda Labs implementation
│   │   ├── runpod/
│   │   │   └── runpod.go     # RunPod implementation
│   │   ├── coreweave/
│   │   │   └── coreweave.go  # CoreWeave implementation
│   │   └── paperspace/
│   │       └── paperspace.go # Paperspace implementation
│   ├── models/
│   │   └── registry.go       # Model and GPU registry
│   ├── wireguard/
│   │   ├── wireguard.go      # WireGuard setup/teardown
│   │   └── config.go         # Config generation
│   ├── deploy/
│   │   ├── deploy.go         # Deployment orchestration
│   │   ├── cloudinit.go      # Cloud-init generation
│   │   └── deadman.go        # Deadman switch logic
│   ├── ui/
│   │   ├── tui.go            # Main TUI (bubbletea)
│   │   ├── provider_select.go
│   │   ├── model_select.go
│   │   ├── deploy_progress.go
│   │   ├── status.go
│   │   └── alerts.go         # Alert display
│   ├── alert/
│   │   └── webhook.go        # Webhook notifications
│   └── logging/
│       └── logger.go         # Structured logging
├── pkg/
│   └── api/
│       └── ollama.go         # Ollama API client
├── templates/
│   ├── cloud-init.yaml.tmpl
│   └── wireguard.conf.tmpl
├── .example.env
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 9.2 Key Interfaces

```go
// provider/provider.go

type Provider interface {
    Name() string
    
    // Pricing
    GetOffers(ctx context.Context, filter OfferFilter) ([]Offer, error)
    
    // Instance lifecycle
    CreateInstance(ctx context.Context, req CreateRequest) (*Instance, error)
    GetInstance(ctx context.Context, id string) (*Instance, error)
    TerminateInstance(ctx context.Context, id string) error
    
    // Billing
    SupportsBillingVerification() bool           // Returns true if provider has billing status API
    GetBillingStatus(ctx context.Context, id string) (BillingStatus, error)
    ConsoleURL() string                          // URL to provider console for manual verification
}

type Offer struct {
    Provider      string
    GPU           string
    VRAM          int
    Region        string
    SpotPrice     *float64  // nil if not available
    OnDemandPrice float64
    StoragePrice  float64   // per GB/hr
    EgressPrice   float64   // per GB
    Available     bool
}

type CreateRequest struct {
    OfferID       string
    Spot          bool
    CloudInit     string
    SSHPublicKey  string
    DiskSizeGB    int
}

type Instance struct {
    ID            string
    Provider      string
    Status        string  // creating, running, terminated
    PublicIP      string
    GPU           string
    Region        string
    CreatedAt     time.Time
    HourlyRate    float64
}
```

### 9.3 Dependencies

```go
// go.mod
module github.com/yourusername/continueplz

go 1.22

require (
    github.com/charmbracelet/bubbletea v0.25.0    // TUI framework
    github.com/charmbracelet/lipgloss v0.9.1     // TUI styling
    github.com/charmbracelet/bubbles v0.18.0     // TUI components
    github.com/joho/godotenv v1.5.1              // .env loading
    github.com/spf13/cobra v1.8.0                // CLI framework
    github.com/rs/zerolog v1.32.0                // Structured logging
    golang.zx2c4.com/wireguard/wgctrl v0.0.0    // WireGuard control
    gopkg.in/yaml.v3 v3.0.1                      // YAML for cloud-init
)
```

---

## 10. File Templates

### 10.1 .example.env

```bash
# continueplz configuration
# Copy this file to .env and fill in your API keys
#
# Get your API keys from:
# - Vast.ai: https://cloud.vast.ai/account/
# - Lambda Labs: https://cloud.lambdalabs.com/api-keys
# - RunPod: https://www.runpod.io/console/user/settings
# - CoreWeave: https://cloud.coreweave.com/api-access
# - Paperspace: https://console.paperspace.com/settings/apikeys

# Provider API Keys (at least one required)
VAST_API_KEY=
LAMBDA_API_KEY=
RUNPOD_API_KEY=
COREWEAVE_API_KEY=
PAPERSPACE_API_KEY=

# WireGuard (leave empty to auto-generate on init)
WIREGUARD_PRIVATE_KEY=
WIREGUARD_PUBLIC_KEY=

# Preferences
DEFAULT_TIER=medium          # small, medium, large
DEFAULT_REGION=eu-west       # eu-west, us-east, us-west, etc.
PREFER_SPOT=true             # true/false
DEADMAN_TIMEOUT_HOURS=10     # Auto-terminate after this many hours without heartbeat

# Alerting (optional)
ALERT_WEBHOOK_URL=           # Slack/Discord webhook for critical alerts
DAILY_BUDGET_EUR=20          # Warn if daily spend exceeds this
```

### 10.2 Cloud-init Template

```yaml
#cloud-config
# Generated by continueplz - DO NOT EDIT

package_update: true
package_upgrade: false

packages:
  - docker.io
  - wireguard-tools
  - jq
  - curl

write_files:
  - path: /etc/wireguard/wg0.conf
    permissions: '0600'
    content: |
      [Interface]
      PrivateKey = {{ .WireGuard.ServerPrivateKey }}
      Address = 10.13.37.1/24
      ListenPort = 51820
      
      [Peer]
      PublicKey = {{ .WireGuard.ClientPublicKey }}
      AllowedIPs = 10.13.37.2/32
      
  - path: /usr/local/bin/deadman.sh
    permissions: '0755'
    content: |
      #!/bin/bash
      TIMEOUT_SECONDS={{ .Deadman.TimeoutSeconds }}
      HEARTBEAT_FILE=/tmp/continueplz-heartbeat
      PROVIDER={{ .Provider }}
      INSTANCE_ID={{ .InstanceID }}
      
      touch $HEARTBEAT_FILE
      
      while true; do
          sleep 60
          if [ $(($(date +%s) - $(stat -c %Y $HEARTBEAT_FILE))) -gt $TIMEOUT_SECONDS ]; then
              echo "Deadman triggered"
              {{ if eq .Provider "vast" }}
              curl -X DELETE "https://console.vast.ai/api/v0/instances/${INSTANCE_ID}/" \
                  -H "Authorization: Bearer {{ .APIKey }}"
              {{ else if eq .Provider "lambda" }}
              curl -X POST "https://cloud.lambdalabs.com/api/v1/instance-operations/terminate" \
                  -H "Authorization: Bearer {{ .APIKey }}" \
                  -d '{"instance_ids": ["'${INSTANCE_ID}'"]}'
              {{ end }}
              shutdown -h now
          fi
      done
      
  - path: /etc/systemd/system/deadman.service
    content: |
      [Unit]
      Description=Continueplz Deadman Switch
      After=network.target
      
      [Service]
      ExecStart=/usr/local/bin/deadman.sh
      Restart=always
      
      [Install]
      WantedBy=multi-user.target

  - path: /etc/continueplz-instance
    content: |
      PROVIDER={{ .Provider }}
      INSTANCE_ID={{ .InstanceID }}

runcmd:
  # Firewall
  - ufw default deny incoming
  - ufw default allow outgoing
  - ufw allow 51820/udp
  - ufw enable
  
  # WireGuard
  - systemctl enable wg-quick@wg0
  - systemctl start wg-quick@wg0
  
  # Allow Ollama only via WireGuard
  - ufw allow in on wg0 to any port 11434
  - ufw allow in on wg0 to any port 22
  
  # Docker setup
  - systemctl enable docker
  - systemctl start docker
  
  # NVIDIA Container Toolkit
  - curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  - curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
  - apt-get update
  - apt-get install -y nvidia-container-toolkit
  - nvidia-ctk runtime configure --runtime=docker
  - systemctl restart docker
  
  # Start Ollama
  - docker run -d --gpus all -v ollama:/root/.ollama -p 10.13.37.1:11434:11434 --name ollama --restart unless-stopped ollama/ollama
  
  # Wait and pull model
  - sleep 10
  - until curl -s http://10.13.37.1:11434/api/tags > /dev/null; do sleep 2; done
  - docker exec ollama ollama pull {{ .Model }}
  
  # Start deadman
  - systemctl enable deadman
  - systemctl start deadman
  
  # Signal ready
  - touch /tmp/continueplz-ready
```

---

## 11. Testing Requirements

### 11.1 Unit Tests

- Provider API clients (mocked HTTP)
- State serialization/deserialization
- Cost calculations
- Model/GPU compatibility matching
- WireGuard config generation

### 11.2 Integration Tests

- Full deploy/stop cycle per provider (requires real API keys)
- WireGuard tunnel establishment
- Ollama connectivity
- Deadman switch trigger

### 11.3 E2E Tests

- `continueplz init` flow
- `continueplz --cheapest --model=X` flow
- `continueplz --stop` flow
- Spot interruption handling

---

## 12. Release Criteria

### 12.1 MVP (v0.1.0)

- [ ] `continueplz init` werkt
- [ ] Vast.ai provider volledig werkend
- [ ] Lambda Labs provider volledig werkend
- [ ] TUI voor provider/model selectie
- [ ] WireGuard tunnel setup (Linux + macOS)
- [ ] `--cheapest` flag
- [ ] `--stop` flag
- [ ] Basic deadman switch
- [ ] State management
- [ ] Logging naar file

### 12.2 v1.0.0

- [ ] Alle 5 providers werkend
- [ ] Spot interruption handling
- [ ] Webhook alerting
- [ ] JSON output
- [ ] Budget warnings
- [ ] Manual verification flow voor providers zonder billing API
- [ ] Full test coverage
- [ ] Cross-platform builds (Linux amd64/arm64, macOS amd64/arm64)
- [ ] Documentation

### 12.3 Platform Support Matrix

| Platform | Status | Notes |
|----------|--------|-------|
| Linux amd64 | ✓ Supported | Primary target |
| Linux arm64 | ✓ Supported | |
| macOS amd64 | ✓ Supported | Intel Macs |
| macOS arm64 | ✓ Supported | Apple Silicon |
| Windows | ✗ Not supported | Use WSL2 if needed |

---

## 13. Design Decisions

De volgende beslissingen zijn genomen:

1. **Cross-platform support:** Linux en macOS only
   - Windows is out of scope voor MVP en v1.0
   - WireGuard setup is significant anders op Windows
   - Gebruikers op Windows kunnen WSL2 gebruiken (niet officieel ondersteund)

2. **Billing verificatie:** Manual verification als fallback
   - Voor providers met billing status API: automatisch verifiëren
   - Voor providers zonder: toon duidelijke waarschuwing dat gebruiker handmatig moet controleren
   - Log altijd een WARN als automatische verificatie niet mogelijk is
   - TUI toont instructies voor handmatige verificatie met directe link naar provider console

3. **Multi-instance support:** Niet ondersteund
   - Eén instance per directory/configuratie
   - Simpeler state management
   - Duidelijker cost tracking
   - Als gebruiker meerdere instances wil: meerdere directories met eigen .env

---

## 14. Glossary

| Term | Definitie |
|------|-----------|
| Spot instance | Goedkope instance die provider kan onderbreken |
| On-demand | Instance met gegarandeerde beschikbaarheid |
| Deadman switch | Automatische termination als geen heartbeat |
| Heartbeat | Periodiek signaal dat client nog actief is |
| WireGuard | VPN protocol voor beveiligde tunnel |
| VRAM | Video RAM op GPU, bepaalt welke modellen passen |
| Tier | Model grootte categorie (small/medium/large) |

---

## Appendix A: Provider API Documentation Links

- Vast.ai: https://vast.ai/docs/api/introduction
- Lambda Labs: https://cloud.lambdalabs.com/api/v1/docs
- RunPod: https://docs.runpod.io/reference/runpod-apis
- CoreWeave: https://docs.coreweave.com/
- Paperspace: https://docs.paperspace.com/core/api-reference/

---

*End of PRD*
