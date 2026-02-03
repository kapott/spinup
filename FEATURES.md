# continueplz Feature Breakdown

> **Purpose**: This document breaks down the continueplz project into small, atomic features that can each be completed in a single Claude Code context window. Each feature has clear acceptance criteria and dependencies.

## Status Legend

- `[ ]` - Not started
- `[~]` - In progress
- `[x]` - Completed
- `[!]` - Blocked

---

## Phase 1: Project Foundation

### F001: Initialize Go Module and Project Structure
**Status**: `[x]`
**Dependencies**: None
**Estimated files**: 5-8

**Description**: Create the basic Go module and directory structure as specified in the PRD.

**Tasks**:
1. Initialize Go module with `go mod init github.com/tmeurs/continueplz`
2. Create directory structure:
   ```
   cmd/continueplz/
   internal/config/
   internal/provider/
   internal/models/
   internal/wireguard/
   internal/deploy/
   internal/ui/
   internal/alert/
   internal/logging/
   pkg/api/
   templates/
   ```
3. Create placeholder `main.go` with minimal CLI
4. Create `Makefile` with build targets
5. Create `.example.env` from PRD template

**Acceptance Criteria**:
- [x] `go build ./cmd/continueplz` succeeds
- [x] `./continueplz --version` outputs version string
- [x] Directory structure matches PRD Section 9.1

---

### F002: Setup Cobra CLI Framework
**Status**: `[x]`
**Dependencies**: F001
**Estimated files**: 2-3

**Description**: Set up the Cobra CLI framework with all command definitions and flags.

**Tasks**:
1. Add Cobra to dependencies
2. Create root command with all global flags from PRD Section 3.3
3. Create subcommand stubs: `init`, `status`
4. Wire up `--version`, `--help` flags

**Acceptance Criteria**:
- [x] `continueplz --help` shows all flags from PRD
- [x] `continueplz init --help` works
- [x] `continueplz status --help` works

---

### F003: Implement Structured Logging
**Status**: `[x]`
**Dependencies**: F001
**Estimated files**: 2

**Description**: Implement the logging infrastructure using zerolog.

**Tasks**:
1. Create `internal/logging/logger.go`
2. Implement log levels: ERROR, WARN, INFO, DEBUG
3. Implement log format from PRD Section 7.2
4. Implement file logging with rotation (7 days)
5. Wire up `-v` and `-vv` flags for verbose output

**Acceptance Criteria**:
- [x] Logs write to `continueplz.log`
- [x] Log format matches PRD Section 7.2
- [x] `-v` shows INFO+ to stderr
- [x] `-vv` shows DEBUG+ to stderr

---

### F004: Configuration Loading from .env
**Status**: `[x]`
**Dependencies**: F001
**Estimated files**: 2

**Description**: Implement configuration loading from .env file.

**Tasks**:
1. Create `internal/config/config.go`
2. Add godotenv dependency
3. Define Config struct with all fields from PRD Section 4.3
4. Implement `LoadConfig()` function
5. Implement validation for required fields
6. Ensure file permissions check (0600)

**Acceptance Criteria**:
- [x] Config loads from `.env` file
- [x] Missing required fields return clear error
- [x] File permissions warning if not 0600

---

### F005: State File Management
**Status**: `[x]`
**Dependencies**: F001
**Estimated files**: 2

**Description**: Implement state file reading/writing/validation.

**Tasks**:
1. Create `internal/config/state.go`
2. Define State struct matching PRD Section 6.1
3. Implement `LoadState()`, `SaveState()`, `ClearState()`
4. Implement state file locking to prevent race conditions
5. Handle missing/corrupt state file gracefully

**Acceptance Criteria**:
- [x] State saves to `.continueplz.state`
- [x] State JSON matches PRD Section 6.1 format
- [x] Corrupt state file handled gracefully with warning

---

## Phase 2: Core Types and Interfaces

### F006: Define Provider Interface
**Status**: `[x]`
**Dependencies**: F001
**Estimated files**: 1

**Description**: Define the Provider interface and related types.

**Tasks**:
1. Create `internal/provider/provider.go`
2. Define `Provider` interface from PRD Section 9.2
3. Define `Offer`, `CreateRequest`, `Instance`, `BillingStatus` types
4. Define `OfferFilter` for filtering offers

**Acceptance Criteria**:
- [x] All types from PRD Section 9.2 defined
- [x] Interface methods match PRD specification

---

### F007: Create Model Registry
**Status**: `[x]`
**Dependencies**: F001
**Estimated files**: 1

**Description**: Implement the model registry with VRAM requirements.

**Tasks**:
1. Create `internal/models/registry.go`
2. Define `Model` struct with Name, Params, VRAM, Quality, Tier
3. Populate `ModelRegistry` with all models from PRD Section 3.4
4. Implement `GetModelByName()`, `GetModelsByTier()`, `GetCompatibleModels(vram int)`

**Acceptance Criteria**:
- [x] All 11 models from PRD defined (PRD Section 3.4 actually has 11 models)
- [x] `GetCompatibleModels(40)` returns correct models for A100-40GB
- [x] Tiers correctly categorized

---

### F008: Create GPU Registry
**Status**: `[x]`
**Dependencies**: F001
**Estimated files**: 1 (extend F007 file)

**Description**: Implement the GPU registry with provider compatibility.

**Tasks**:
1. Add to `internal/models/registry.go`
2. Define `GPU` struct with Name, VRAM, Providers
3. Populate `GPURegistry` with all GPUs from PRD Section 3.4
4. Implement `GetGPUByName()`, `GetGPUsForProvider()`, `IsModelCompatible(gpu, model)`

**Acceptance Criteria**:
- [x] All 4 GPU types from PRD defined
- [x] Provider compatibility matches PRD
- [x] Compatibility check works correctly

---

## Phase 3: Provider Implementations

### F009: Vast.ai Provider - API Client Basics
**Status**: `[x]`
**Dependencies**: F006
**Estimated files**: 2

**Description**: Implement basic Vast.ai API client (auth, error handling).

**Tasks**:
1. Create `internal/provider/vast/client.go`
2. Implement HTTP client with authentication
3. Implement base API request method with retry logic
4. Implement error parsing and handling
5. Add rate limiting consideration

**Acceptance Criteria**:
- [x] Client authenticates with API key
- [x] API errors parsed into Go errors
- [x] Retry logic with exponential backoff

---

### F010: Vast.ai Provider - GetOffers
**Status**: `[x]`
**Dependencies**: F009
**Estimated files**: 1 (extend F009)

**Description**: Implement GetOffers for Vast.ai.

**Tasks**:
1. Implement `GetOffers(ctx, filter)` method
2. Parse Vast.ai offers into standard `Offer` type
3. Handle spot vs on-demand pricing
4. Filter by GPU type, region, availability

**Acceptance Criteria**:
- [x] Returns list of Offers with correct pricing
- [x] Spot prices correctly populated (nil if unavailable)
- [x] Filters work correctly

---

### F011: Vast.ai Provider - Instance Lifecycle
**Status**: `[x]`
**Dependencies**: F010
**Estimated files**: 1 (extend F009)

**Description**: Implement instance create/get/terminate for Vast.ai.

**Tasks**:
1. Implement `CreateInstance(ctx, req)` method
2. Implement `GetInstance(ctx, id)` method
3. Implement `TerminateInstance(ctx, id)` method
4. Handle cloud-init injection

**Acceptance Criteria**:
- [x] Can create instance with cloud-init
- [x] Can query instance status
- [x] Can terminate instance

---

### F012: Vast.ai Provider - Billing Verification
**Status**: `[x]`
**Dependencies**: F011
**Estimated files**: 1 (extend F009)

**Description**: Implement billing status check for Vast.ai.

**Tasks**:
1. Implement `SupportsBillingVerification()` - returns true
2. Implement `GetBillingStatus(ctx, id)` method
3. Implement `ConsoleURL()` method
4. Complete the Provider interface for Vast.ai

**Acceptance Criteria**:
- [x] Billing status correctly reported
- [x] Console URL returns correct Vast.ai URL
- [x] Provider interface fully implemented

---

### F013: Lambda Labs Provider - Full Implementation
**Status**: `[x]`
**Dependencies**: F006
**Estimated files**: 2

**Description**: Implement complete Lambda Labs provider.

**Tasks**:
1. Create `internal/provider/lambda/client.go`
2. Implement all Provider interface methods
3. Note: Lambda Labs does not support spot instances
4. Implement proper error handling for Lambda API

**Acceptance Criteria**:
- [x] GetOffers returns on-demand pricing only
- [x] Instance lifecycle works
- [x] Billing verification implemented (if supported)

---

### F014: RunPod Provider - Full Implementation
**Status**: `[x]`
**Dependencies**: F006
**Estimated files**: 2

**Description**: Implement complete RunPod provider (GraphQL API).

**Tasks**:
1. Create `internal/provider/runpod/client.go`
2. Implement GraphQL client for RunPod
3. Implement all Provider interface methods
4. Handle spot instance support

**Acceptance Criteria**:
- [x] GraphQL queries work correctly
- [x] Spot pricing supported
- [x] Instance lifecycle works

---

### F015: CoreWeave Provider - Full Implementation
**Status**: `[x]`
**Dependencies**: F006
**Estimated files**: 2

**Description**: Implement complete CoreWeave provider (Kubernetes API).

**Tasks**:
1. Create `internal/provider/coreweave/client.go`
2. Implement Kubernetes API client
3. Implement all Provider interface methods
4. Handle spot instance support

**Acceptance Criteria**:
- [x] Kubernetes API interaction works
- [x] Instance lifecycle works
- [x] Proper error handling

---

### F016: Paperspace Provider - Full Implementation
**Status**: `[x]`
**Dependencies**: F006
**Estimated files**: 2

**Description**: Implement complete Paperspace provider.

**Tasks**:
1. Create `internal/provider/paperspace/client.go`
2. Implement all Provider interface methods
3. Note: Paperspace does NOT support billing verification API
4. Implement `SupportsBillingVerification()` to return false

**Acceptance Criteria**:
- [x] Instance lifecycle works
- [x] `SupportsBillingVerification()` returns false
- [x] Manual verification flow supported

---

### F017: Provider Registry and Factory
**Status**: `[x]`
**Dependencies**: F009, F013, F014, F015, F016
**Estimated files**: 1

**Description**: Create provider registry for easy provider access.

**Tasks**:
1. Create `internal/provider/registry/registry.go` (in subpackage to avoid import cycle)
2. Implement `NewProvider(name string, config Config)` factory
3. Implement `GetAllProviders(config)` for price comparison
4. Implement `GetConfiguredProviders(config)` (only those with API keys)

**Acceptance Criteria**:
- [x] Factory creates correct provider by name
- [x] Only providers with valid API keys returned
- [x] Error for unknown provider name

---

## Phase 4: WireGuard

### F018: WireGuard Key Generation
**Status**: `[x]`
**Dependencies**: F001
**Estimated files**: 1

**Description**: Implement WireGuard key pair generation.

**Tasks**:
1. Create `internal/wireguard/keys.go`
2. Implement `GenerateKeyPair()` function
3. Implement `PublicKeyFromPrivate()` function
4. Use golang.zx2c4.com/wireguard/wgctrl/wgtypes

**Acceptance Criteria**:
- [x] Valid WireGuard key pairs generated
- [x] Public key derivation works correctly

---

### F019: WireGuard Config Generation
**Status**: `[x]`
**Dependencies**: F018
**Estimated files**: 2

**Description**: Generate WireGuard configuration files.

**Tasks**:
1. Create `internal/wireguard/config.go`
2. Create `templates/wireguard.conf.tmpl`
3. Implement `GenerateClientConfig()` for local machine
4. Implement `GenerateServerConfig()` for cloud-init injection
5. Use IP range 10.13.37.0/24 as per PRD

**Acceptance Criteria**:
- [x] Client config matches PRD Section 4.1
- [x] Server config matches PRD Section 4.1
- [x] Template renders correctly

---

### F020: WireGuard Tunnel Setup - Linux
**Status**: `[x]`
**Dependencies**: F019
**Estimated files**: 1

**Description**: Implement WireGuard tunnel setup for Linux.

**Tasks**:
1. Create `internal/wireguard/tunnel_linux.go`
2. Implement `SetupTunnel()` using wgctrl
3. Implement interface creation and configuration
4. Handle sudo/root requirements gracefully

**Acceptance Criteria**:
- [x] Tunnel interface created (`wg-continueplz`)
- [x] Routes configured correctly
- [x] Works without requiring manual sudo

---

### F021: WireGuard Tunnel Setup - macOS
**Status**: `[x]`
**Dependencies**: F019
**Estimated files**: 1

**Description**: Implement WireGuard tunnel setup for macOS.

**Tasks**:
1. Create `internal/wireguard/tunnel_darwin.go`
2. Handle macOS-specific WireGuard setup (userspace or system)
3. Use wireguard-go or system WireGuard
4. Handle permissions correctly

**Acceptance Criteria**:
- [x] Tunnel works on macOS
- [x] Handles both Intel and Apple Silicon

---

### F022: WireGuard Tunnel Teardown
**Status**: `[x]`
**Dependencies**: F020, F021
**Estimated files**: 1 (extend F020/F021)

**Description**: Implement WireGuard tunnel teardown.

**Tasks**:
1. Implement `TeardownTunnel()` function
2. Remove interface and routes
3. Handle already-torn-down case gracefully
4. Platform-specific implementations

**Acceptance Criteria**:
- [x] Interface removed cleanly
- [x] No errors if interface doesn't exist
- [x] Works on both Linux and macOS

---

### F023: WireGuard Connection Verification
**Status**: `[x]`
**Dependencies**: F020, F021
**Estimated files**: 1

**Description**: Implement WireGuard connection health check.

**Tasks**:
1. Add `VerifyConnection()` to `internal/wireguard/`
2. Ping remote endpoint through tunnel
3. Check handshake status
4. Return detailed error on failure

**Acceptance Criteria**:
- [x] Can verify tunnel is established
- [x] Detects connection failures
- [x] Returns actionable error messages

---

## Phase 5: Deployment Orchestration

### F024: Cloud-init Template Generation
**Status**: `[x]`
**Dependencies**: F018, F007
**Estimated files**: 2

**Description**: Generate cloud-init configuration for instances.

**Tasks**:
1. Create `internal/deploy/cloudinit.go`
2. Create `templates/cloud-init.yaml.tmpl` from PRD Section 10.2
3. Implement `GenerateCloudInit(params)` function
4. Include WireGuard setup, Ollama, deadman switch, firewall rules

**Acceptance Criteria**:
- [x] Template matches PRD Section 10.2
- [x] Variables correctly substituted
- [x] Valid YAML output

---

### F025: Deadman Switch Implementation - Server Side
**Status**: `[x]`
**Dependencies**: F024
**Estimated files**: 1 (part of cloud-init template)

**Description**: Implement server-side deadman switch logic.

**Tasks**:
1. Verify deadman.sh script in cloud-init template
2. Implement provider-specific self-termination calls
3. Configure systemd service for deadman
4. Handle timeout configuration

**Acceptance Criteria**:
- [x] Deadman script included in cloud-init
- [x] Self-termination works for each provider
- [x] Timeout configurable via parameter

---

### F026: Heartbeat Client Implementation
**Status**: `[x]`
**Dependencies**: F023
**Estimated files**: 2

**Description**: Implement client-side heartbeat sender.

**Tasks**:
1. Create `internal/deploy/heartbeat.go`
2. Implement heartbeat goroutine (sends every 5 minutes)
3. Touch heartbeat file via SSH or API
4. Handle connection failures gracefully

**Acceptance Criteria**:
- [x] Heartbeat sent every 5 minutes
- [x] Heartbeat updates `/tmp/continueplz-heartbeat` on instance
- [x] Failures logged but don't crash client

---

### F027: Deploy Orchestration - Create Flow
**Status**: `[x]`
**Dependencies**: F017, F024, F023, F026
**Estimated files**: 2

**Description**: Implement the deployment orchestration logic.

**Tasks**:
1. Create `internal/deploy/deploy.go`
2. Implement step-by-step deployment as per PRD Section 3.2 (`--cheapest`)
3. Steps: fetch prices, select, create instance, wait boot, configure WG, install Ollama, configure deadman, verify health
4. Implement progress reporting callbacks

**Acceptance Criteria**:
- [x] Full deployment flow works
- [x] Progress reported at each step
- [x] Failures handled with cleanup

---

### F028: Deploy Orchestration - Stop Flow
**Status**: `[x]`
**Dependencies**: F027
**Estimated files**: 1 (extend F027)

**Description**: Implement the stop orchestration with retry logic.

**Tasks**:
1. Implement stop flow from PRD Section 5.2
2. Implement retry with exponential backoff (5 retries)
3. Verify billing stopped (if supported)
4. Clean up WireGuard tunnel
5. Clear state

**Acceptance Criteria**:
- [x] Stop with retry works
- [x] Billing verification attempted
- [x] State cleaned up
- [x] Exit code 1 on failure

---

### F029: Deploy Orchestration - Manual Verification Flow
**Status**: `[x]`
**Dependencies**: F028
**Estimated files**: 1 (extend F027)

**Description**: Implement manual verification flow for providers without billing API.

**Tasks**:
1. Check `SupportsBillingVerification()` result
2. If false, trigger manual verification UI
3. Log WARN about manual verification needed
4. Include console URL in output

**Acceptance Criteria**:
- [x] Manual verification shown for Paperspace
- [x] Console URL displayed
- [x] Clear instructions provided

---

## Phase 6: TUI Components

### F030: TUI Framework Setup
**Status**: `[x]`
**Dependencies**: F002
**Estimated files**: 2

**Description**: Set up Bubbletea TUI framework.

**Tasks**:
1. Create `internal/ui/tui.go`
2. Add bubbletea, lipgloss, bubbles dependencies
3. Create base TUI model and update loop
4. Implement common styles matching PRD aesthetic

**Acceptance Criteria**:
- [x] TUI launches and displays
- [x] Keyboard input handled
- [x] Clean quit with 'q'

---

### F031: TUI - Provider Selection View
**Status**: `[x]`
**Dependencies**: F030, F017
**Estimated files**: 1

**Description**: Implement provider/GPU selection table.

**Tasks**:
1. Create `internal/ui/provider_select.go`
2. Display table with Provider, GPU, Region, Spot/hr, OnDemand/hr, Day Est.
3. Implement keyboard navigation (↑↓)
4. Highlight selected row
5. Implement [r] refresh prices, [q] quit

**Acceptance Criteria**:
- [x] Table matches PRD Section 3.2 layout
- [x] Navigation works
- [x] Selection indicated

---

### F032: TUI - Model Selection View
**Status**: `[x]`
**Dependencies**: F030, F007
**Estimated files**: 1

**Description**: Implement model selection table.

**Tasks**:
1. Create `internal/ui/model_select.go`
2. Display table with Model, Size, VRAM, Quality stars, Compatible GPUs
3. Filter to show only compatible models for selected GPU
4. Keyboard navigation

**Acceptance Criteria**:
- [x] Table matches PRD Section 3.2 layout
- [x] Only compatible models shown
- [x] Quality stars displayed

---

### F033: TUI - Cost Breakdown View
**Status**: `[x]`
**Dependencies**: F031, F032
**Estimated files**: 1

**Description**: Implement cost breakdown panel.

**Tasks**:
1. Create `internal/ui/cost_breakdown.go`
2. Display Compute, Storage, Egress costs
3. Calculate estimated daily total
4. Update when selection changes

**Acceptance Criteria**:
- [x] Breakdown matches PRD Section 3.2 layout
- [x] Totals calculated correctly
- [x] Updates reactively

---

### F034: TUI - Deploy Progress View
**Status**: `[x]`
**Dependencies**: F030, F027
**Estimated files**: 1

**Description**: Implement deployment progress display.

**Tasks**:
1. Create `internal/ui/deploy_progress.go`
2. Show step-by-step progress with spinners
3. Show checkmarks for completed steps
4. Handle errors with X marks

**Acceptance Criteria**:
- [x] Progress matches PRD Section 3.2 (`--cheapest` output)
- [x] Spinners animate during active steps
- [x] Final summary displayed

---

### F035: TUI - Instance Status View
**Status**: `[x]`
**Dependencies**: F030, F005
**Estimated files**: 1

**Description**: Implement active instance status display.

**Tasks**:
1. Create `internal/ui/status.go`
2. Display all instance info from PRD Section 3.2 (active instance)
3. Show running time, cost, deadman timer
4. Implement quick actions: [s]top, [t]est, [l]ogs, [q]uit

**Acceptance Criteria**:
- [x] Status matches PRD Section 3.2 layout
- [x] Live updates (cost, time)
- [x] Actions trigger correct flows

---

### F036: TUI - Stop Progress View
**Status**: `[x]`
**Dependencies**: F030, F028
**Estimated files**: 1

**Description**: Implement stop progress display.

**Tasks**:
1. Create `internal/ui/stop_progress.go`
2. Show step-by-step stop progress
3. Show session summary at end
4. Handle manual verification display

**Acceptance Criteria**:
- [x] Progress matches PRD Section 3.2 (stop output)
- [x] Manual verification warning shown when needed
- [x] Session cost and duration displayed

---

### F037: TUI - Alert Display
**Status**: `[x]`
**Dependencies**: F030
**Estimated files**: 1

**Description**: Implement alert display components.

**Tasks**:
1. Create `internal/ui/alerts.go`
2. Implement CRITICAL alert with red flashing border
3. Implement WARNING alert display
4. Implement INFO notification

**Acceptance Criteria**:
- [x] CRITICAL alert matches PRD Section 8.3
- [x] Red border flashes (ANSI codes)
- [x] Clear action instructions displayed

---

### F038: TUI - Spot Interruption Alert
**Status**: `[x]`
**Dependencies**: F037
**Estimated files**: 1 (extend F037)

**Description**: Implement spot interruption handling UI.

**Tasks**:
1. Add spot interruption alert view
2. Display session cost and duration
3. Offer [r]estart or [q]uit options
4. Handle restart flow

**Acceptance Criteria**:
- [x] Alert matches PRD Section 5.3 layout
- [x] Restart returns to provider selection
- [x] Quit exits cleanly

---

## Phase 7: Commands

### F039: Init Command - Provider Selection
**Status**: `[x]`
**Dependencies**: F030, F004
**Estimated files**: 1

**Description**: Implement init command provider selection.

**Tasks**:
1. Create `internal/ui/init_wizard.go`
2. Implement multi-select for providers
3. Display as per PRD Section 3.2 (`init` output)

**Acceptance Criteria**:
- [x] Provider selection works
- [x] Visual matches PRD

---

### F040: Init Command - API Key Input and Validation
**Status**: `[x]`
**Dependencies**: F039, F017
**Estimated files**: 1 (extend F039)

**Description**: Implement API key input and validation.

**Tasks**:
1. Prompt for API key for each selected provider
2. Validate key by making test API call
3. Show account info/balance on success
4. Show error on invalid key

**Acceptance Criteria**:
- [x] Keys validated against real API
- [x] Account info displayed
- [x] Invalid keys rejected with message

---

### F041: Init Command - WireGuard and Preferences
**Status**: `[x]`
**Dependencies**: F040, F018
**Estimated files**: 1 (extend F039)

**Description**: Implement WireGuard key and preferences setup.

**Tasks**:
1. Generate or accept existing WireGuard key
2. Prompt for default tier (small/medium/large)
3. Prompt for deadman timeout
4. Save to .env file with 0600 permissions

**Acceptance Criteria**:
- [x] WireGuard keys generated/stored
- [x] Preferences saved
- [x] File permissions correct

---

### F042: Interactive Mode - No Instance
**Status**: `[x]`
**Dependencies**: F031, F032, F033, F034
**Estimated files**: 1

**Description**: Wire up interactive mode when no instance is running.

**Tasks**:
1. Update root command to check state
2. If no instance: show provider/model selection TUI
3. On Enter: trigger deployment
4. Handle all keyboard shortcuts

**Acceptance Criteria**:
- [x] TUI launches when no args and no instance
- [x] Deploy triggers on selection
- [x] Prices refresh on [r]

---

### F043: Interactive Mode - Instance Active
**Status**: `[x]`
**Dependencies**: F035, F036
**Estimated files**: 1

**Description**: Wire up interactive mode when instance is running.

**Tasks**:
1. If instance active: show status TUI
2. Wire up [s]top, [t]est, [l]ogs, [q]uit
3. Handle stop flow with progress

**Acceptance Criteria**:
- [x] Status shown when instance active
- [x] Stop triggers correctly
- [x] Test pings model endpoint

---

### F044: --cheapest Flag Implementation
**Status**: `[x]`
**Dependencies**: F027, F007, F017
**Estimated files**: 1

**Description**: Implement --cheapest non-interactive deployment.

**Tasks**:
1. Parse --cheapest and --model flags
2. Fetch prices from all configured providers
3. Select cheapest compatible option
4. Run deployment flow
5. Output progress as per PRD Section 3.2

**Acceptance Criteria**:
- [x] Selects cheapest compatible GPU
- [x] Output matches PRD format
- [x] Exit code 0 on success

---

### F045: --stop Flag Implementation
**Status**: `[x]`
**Dependencies**: F028
**Estimated files**: 1

**Description**: Implement --stop non-interactive stop.

**Tasks**:
1. Parse --stop flag
2. Check for active instance in state
3. Run stop flow
4. Output progress as per PRD Section 3.2 (--stop)

**Acceptance Criteria**:
- [x] Stops running instance
- [x] Output matches PRD format
- [x] Exit code 1 on failure

---

### F046: Status Command
**Status**: `[x]`
**Dependencies**: F005
**Estimated files**: 1

**Description**: Implement status command.

**Tasks**:
1. Implement `status` subcommand
2. Read state and display status
3. Format as per PRD Section 3.2

**Acceptance Criteria**:
- [x] Shows instance info when active
- [x] Shows "None active" when no instance
- [x] Format matches PRD

---

### F047: JSON Output Mode
**Status**: `[x]`
**Dependencies**: F044, F045, F046
**Estimated files**: 2

**Description**: Implement --output=json for all commands.

**Tasks**:
1. Parse --output flag
2. Create JSON output structs matching PRD Section 3.2
3. Add JSON output to status, deploy, stop commands
4. Disable TUI/colors in JSON mode

**Acceptance Criteria**:
- [x] JSON matches PRD Section 3.2 format
- [x] Valid JSON output
- [x] No ANSI codes in JSON mode

---

## Phase 8: Reliability Features

### F048: State Reconciliation
**Status**: `[x]`
**Dependencies**: F005, F017
**Estimated files**: 1

**Description**: Implement state reconciliation from PRD Section 6.2.

**Tasks**:
1. Add `reconcileState()` to deploy package
2. On startup, verify local state against provider
3. Handle instance doesn't exist case
4. Handle instance terminated externally
5. Show warning and clean up state

**Acceptance Criteria**:
- [x] Stale state detected and cleaned
- [x] User warned about mismatch
- [x] State matches provider reality

---

### F049: Spot Interruption Detection
**Status**: `[x]`
**Dependencies**: F026
**Estimated files**: 2

**Description**: Implement spot interruption detection.

**Tasks**:
1. Create server-side interruption detector (cloud-init)
2. Monitor provider metadata service for termination notice
3. Signal client through WireGuard (or connection loss detection)
4. Trigger interruption UI on client

**Acceptance Criteria**:
- [x] Interruption detected within 30 seconds
- [x] Client notified
- [x] UI shows interruption alert

---

### F050: Stop Retry with Exponential Backoff
**Status**: `[x]`
**Dependencies**: F028
**Estimated files**: 1 (verify in F028)

**Description**: Ensure stop has proper retry logic.

**Tasks**:
1. Verify retry logic in stop flow
2. Implement exponential backoff (2s base, 5 retries)
3. Log each retry attempt
4. Final failure triggers CRITICAL alert

**Acceptance Criteria**:
- [x] 5 retries with exponential backoff
- [x] Each attempt logged
- [x] CRITICAL alert on final failure

---

## Phase 9: Alerting

### F051: Webhook Client
**Status**: `[x]`
**Dependencies**: F003
**Estimated files**: 1

**Description**: Implement webhook client for alerts.

**Tasks**:
1. Create `internal/alert/webhook.go`
2. Implement `SendAlert(level, message, context)` function
3. Format payload as per PRD Section 8.2
4. Handle network failures gracefully

**Acceptance Criteria**:
- [x] Webhook sends to configured URL
- [x] Payload matches PRD format
- [x] Failures don't crash app

---

### F052: Alert Dispatcher
**Status**: `[x]`
**Dependencies**: F051, F003
**Estimated files**: 1

**Description**: Implement alert dispatching logic.

**Tasks**:
1. Create alert dispatcher in `internal/alert/`
2. Route alerts based on level
3. CRITICAL: webhook + ERROR log + TUI
4. ERROR: ERROR log + TUI notification
5. WARN: WARN log + TUI notification

**Acceptance Criteria**:
- [x] Alerts routed correctly by level
- [x] Multiple destinations handled
- [x] TUI integration works

---

### F053: Budget Warnings
**Status**: `[x]`
**Dependencies**: F052, F004
**Estimated files**: 1

**Description**: Implement daily budget warning.

**Tasks**:
1. Read DAILY_BUDGET_EUR from config
2. Track accumulated cost in state
3. Warn when approaching budget (80%, 100%)
4. Alert via webhook and TUI

**Acceptance Criteria**:
- [x] Warning at 80% of budget
- [x] Alert at 100% of budget
- [x] Webhook sent

---

## Phase 10: Testing

### F054: Unit Tests - Provider Mocking
**Status**: `[x]`
**Dependencies**: F006
**Estimated files**: 2

**Description**: Create mock provider for testing.

**Tasks**:
1. Create `internal/provider/mock/mock.go`
2. Implement Provider interface with configurable responses
3. Add ability to simulate errors, delays

**Acceptance Criteria**:
- [x] Mock implements full Provider interface
- [x] Errors configurable
- [x] Useful for unit tests

---

### F055: Unit Tests - State Management
**Status**: `[x]`
**Dependencies**: F005
**Estimated files**: 1

**Description**: Write unit tests for state management.

**Tasks**:
1. Create `internal/config/state_test.go`
2. Test save/load cycle
3. Test corrupt file handling
4. Test concurrent access

**Acceptance Criteria**:
- [x] All state operations tested
- [x] Edge cases covered
- [x] Tests pass

---

### F056: Unit Tests - Cost Calculations
**Status**: `[x]`
**Dependencies**: F007, F033
**Estimated files**: 1

**Description**: Write unit tests for cost calculations.

**Tasks**:
1. Create cost calculation tests
2. Test hourly to daily conversion
3. Test accumulated cost tracking
4. Test multi-currency handling (if applicable)

**Acceptance Criteria**:
- [x] Calculations verified
- [x] Edge cases covered

---

### F057: Unit Tests - Model/GPU Compatibility
**Status**: `[x]`
**Dependencies**: F007, F008
**Estimated files**: 1

**Description**: Write unit tests for compatibility checks.

**Tasks**:
1. Create `internal/models/registry_test.go`
2. Test model-GPU compatibility
3. Test tier filtering
4. Test VRAM requirements

**Acceptance Criteria**:
- [x] All compatibility scenarios tested
- [x] Edge cases (exactly enough VRAM) covered

---

### F058: Unit Tests - WireGuard Config
**Status**: `[x]`
**Dependencies**: F019
**Estimated files**: 1

**Description**: Write unit tests for WireGuard config generation.

**Tasks**:
1. Create `internal/wireguard/config_test.go`
2. Test config generation
3. Verify valid WireGuard config syntax

**Acceptance Criteria**:
- [x] Generated configs are valid
- [x] All parameters substituted

---

### F059: Integration Test Framework
**Status**: `[x]`
**Dependencies**: F054
**Estimated files**: 2

**Description**: Set up integration test framework.

**Tasks**:
1. Create `tests/integration/` directory
2. Set up test fixtures
3. Create helper functions for full flow tests
4. Document how to run with real API keys

**Acceptance Criteria**:
- [x] Framework in place
- [x] Can run against mock or real providers
- [x] Documentation exists

---

### F060: E2E Test - Full Deploy/Stop Cycle
**Status**: `[x]`
**Dependencies**: F059
**Estimated files**: 1

**Description**: Write E2E test for full cycle.

**Tasks**:
1. Create `tests/e2e/deploy_stop_test.go`
2. Test init -> deploy -> verify -> stop -> verify stopped
3. Run against mock provider initially

**Acceptance Criteria**:
- [x] Full cycle tested
- [x] State correctly managed throughout

---

## Phase 11: Polish and Release

### F061: Makefile Build Targets
**Status**: `[x]`
**Dependencies**: F001
**Estimated files**: 1

**Description**: Complete Makefile with all targets.

**Tasks**:
1. Add `build`, `test`, `lint`, `clean` targets
2. Add cross-compilation targets (Linux/macOS, amd64/arm64)
3. Add `release` target for versioned builds
4. Add `install` target

**Acceptance Criteria**:
- [x] All platforms build successfully
- [x] Tests run via make
- [x] Lint checks pass

---

### F062: Cross-Platform Builds
**Status**: `[x]`
**Dependencies**: F061
**Estimated files**: 1

**Description**: Verify cross-platform compilation.

**Tasks**:
1. Test Linux amd64 build
2. Test Linux arm64 build
3. Test macOS amd64 build
4. Test macOS arm64 build
5. Create build script/CI workflow

**Acceptance Criteria**:
- [x] All 4 platform builds succeed
- [x] Binaries work on target platforms

---

### F063: README Documentation
**Status**: `[x]`
**Dependencies**: All features
**Estimated files**: 1

**Description**: Write comprehensive README.

**Tasks**:
1. Create README.md
2. Document installation
3. Document quick start
4. Document all commands and flags
5. Document provider setup

**Acceptance Criteria**:
- [x] Installation clear
- [x] Quick start works when followed
- [x] All features documented

---

### F064: Ollama API Client
**Status**: `[x]`
**Dependencies**: F023
**Estimated files**: 1

**Description**: Implement Ollama API client for health checks.

**Tasks**:
1. Create `pkg/api/ollama.go`
2. Implement `Ping()` for health check
3. Implement `ListModels()` to verify model loaded
4. Implement `Generate()` for test query

**Acceptance Criteria**:
- [x] Can verify Ollama is responding
- [x] Can verify model is loaded
- [x] Timeout handling

---

---

## Dependency Graph Summary

```
F001 (Foundation)
├── F002 (Cobra CLI)
├── F003 (Logging)
├── F004 (Config)
├── F005 (State)
├── F006 (Provider Interface)
│   ├── F009-F012 (Vast.ai)
│   ├── F013 (Lambda)
│   ├── F014 (RunPod)
│   ├── F015 (CoreWeave)
│   └── F016 (Paperspace)
├── F007-F008 (Model/GPU Registry)
└── F018-F023 (WireGuard)

F017 (Provider Registry) ← F009-F016
F024-F029 (Deploy) ← F017, F018-F023
F030-F038 (TUI) ← F002, F005, F007
F039-F047 (Commands) ← TUI, Deploy
F048-F050 (Reliability) ← Commands
F051-F053 (Alerting) ← F003
F054-F060 (Testing) ← All
F061-F064 (Polish) ← All
```

---

## Progress Tracking

| Phase | Total | Done | Progress |
|-------|-------|------|----------|
| 1. Foundation | 5 | 5 | 100% |
| 2. Core Types | 3 | 3 | 100% |
| 3. Providers | 9 | 9 | 100% |
| 4. WireGuard | 6 | 6 | 100% |
| 5. Deployment | 6 | 6 | 100% |
| 6. TUI | 9 | 9 | 100% |
| 7. Commands | 9 | 9 | 100% |
| 8. Reliability | 3 | 3 | 100% |
| 9. Alerting | 3 | 3 | 100% |
| 10. Testing | 7 | 7 | 100% |
| 11. Polish | 4 | 4 | 100% |
| **TOTAL** | **64** | **64** | **100%** |

---

## How to Use This Document

1. **Starting a new session**: Read MEMORY.md first to understand current state
2. **Pick a feature**: Choose the lowest-numbered incomplete feature whose dependencies are complete
3. **Before starting**: Mark feature as `[~]` in progress
4. **After completing**: Mark feature as `[x]` complete and update MEMORY.md
5. **If blocked**: Mark as `[!]` and document blocker in MEMORY.md
