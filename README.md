# continueplz

Ephemeral GPU instances for code-assist LLMs. Spin up a GPU in the cloud, deploy your preferred coding model via Ollama, connect over WireGuard, and forget about cleanup.

## Features

- Price comparison across 5 cloud GPU providers
- Automatic spot instance selection for cost savings
- Secure WireGuard tunnel to the instance
- Deadman switch for guaranteed cleanup
- Interactive TUI and scriptable JSON output

## Supported Providers

| Provider | Spot | Billing API | Console |
|----------|------|-------------|---------|
| Vast.ai | Yes | Yes | https://console.vast.ai |
| Lambda Labs | No | Yes | https://cloud.lambdalabs.com |
| RunPod | Yes | Yes | https://www.runpod.io/console |
| CoreWeave | Yes | Yes | https://cloud.coreweave.com |
| Paperspace | No | No | https://console.paperspace.com |

## Supported GPUs

| GPU | VRAM | Providers |
|-----|------|-----------|
| A6000 | 48GB | Vast.ai, RunPod |
| A100-40GB | 40GB | Vast.ai, Lambda, RunPod, CoreWeave, Paperspace |
| A100-80GB | 80GB | Vast.ai, Lambda, RunPod, CoreWeave, Paperspace |
| H100-80GB | 80GB | Lambda, CoreWeave |

## Supported Models

### Small Tier (16-24GB VRAM)
- qwen2.5-coder:7b
- deepseek-coder:6.7b
- codellama:7b
- starcoder2:7b

### Medium Tier (24-48GB VRAM)
- qwen2.5-coder:14b
- qwen2.5-coder:32b
- deepseek-coder:33b
- codellama:34b

### Large Tier (80GB+ VRAM)
- codellama:70b
- qwen2.5-coder:72b
- deepseek-coder-v2:236b

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/kapott/continueplz.git
cd continueplz

# Build
make build

# Or install to GOPATH/bin
make install
```

### Pre-built Binaries

Download from the releases page for your platform:
- `continueplz-linux-amd64`
- `continueplz-linux-arm64`
- `continueplz-darwin-amd64`
- `continueplz-darwin-arm64`

### Requirements

- Go 1.21+ (for building from source)
- WireGuard installed on your system
- At least one cloud provider API key

## Quick Start

1. Copy the example configuration:
   ```bash
   cp .example.env .env
   chmod 600 .env
   ```

2. Add at least one provider API key to `.env`:
   ```bash
   # Edit .env and add your API key
   VAST_API_KEY=your-api-key-here
   ```

3. Run continueplz:
   ```bash
   # Interactive mode (TUI)
   ./continueplz

   # Or quick deploy with cheapest option
   ./continueplz --cheapest --model qwen2.5-coder:32b
   ```

4. Connect your IDE to the Ollama endpoint shown (e.g., `http://10.13.37.2:11434`)

5. When done, stop the instance:
   ```bash
   ./continueplz --stop
   ```

## Usage

### Interactive Mode

```bash
continueplz
```

Launches the TUI where you can:
- Compare prices across providers
- Select GPU and model
- Monitor instance status
- Stop the instance

### Non-Interactive Mode

```bash
# Deploy cheapest option for a model
continueplz --cheapest --model qwen2.5-coder:32b

# Force specific provider
continueplz --cheapest --provider vast --model qwen2.5-coder:32b

# Force specific GPU
continueplz --cheapest --gpu a100-80 --model qwen2.5-coder:72b

# Prefer on-demand over spot
continueplz --cheapest --on-demand --model qwen2.5-coder:32b

# Set custom timeout (default: 10h)
continueplz --cheapest --timeout 4h --model qwen2.5-coder:32b
```

### Check Status

```bash
# Human-readable output
continueplz status

# JSON output for scripting
continueplz status --output json
```

### Stop Instance

```bash
continueplz --stop
```

## Command Reference

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--cheapest` | false | Select cheapest compatible provider/GPU automatically |
| `--provider` | - | Force specific provider (vast, lambda, runpod, coreweave, paperspace) |
| `--gpu` | - | Force specific GPU type (a6000, a100-40, a100-80, h100) |
| `--model` | qwen2.5-coder:32b | Model to deploy |
| `--tier` | medium | Model tier: small, medium, large |
| `--spot` | true | Prefer spot instances |
| `--on-demand` | false | Force on-demand instances |
| `--region` | - | Preferred region (eu-west, us-east, etc.) |
| `--stop` | false | Stop running instance |
| `--output` | text | Output format: text, json |
| `--timeout` | 10h | Deadman switch timeout |
| `-y, --yes` | false | Skip confirmations |
| `-v` | - | Verbose logging (INFO level) |
| `-vv` | - | Debug logging (DEBUG level) |
| `--version` | - | Show version information |

### Commands

| Command | Description |
|---------|-------------|
| `continueplz` | Interactive TUI (default) |
| `continueplz init` | Configuration wizard |
| `continueplz status` | Show current instance status |

## Configuration

Configuration is loaded from `.env` in the current directory.

```bash
# Provider API Keys (at least one required)
VAST_API_KEY=
LAMBDA_API_KEY=
RUNPOD_API_KEY=
COREWEAVE_API_KEY=
PAPERSPACE_API_KEY=

# WireGuard (leave empty to auto-generate)
WIREGUARD_PRIVATE_KEY=
WIREGUARD_PUBLIC_KEY=

# Preferences
DEFAULT_TIER=medium          # small, medium, large
DEFAULT_REGION=eu-west       # eu-west, us-east, us-west, etc.
PREFER_SPOT=true             # true/false
DEADMAN_TIMEOUT_HOURS=10     # Hours before auto-termination

# Alerting (optional)
ALERT_WEBHOOK_URL=           # Slack/Discord webhook
DAILY_BUDGET_EUR=20          # Daily spend warning threshold
```

**Important:** Set file permissions to 0600:
```bash
chmod 600 .env
```

## Provider Setup

### Vast.ai

1. Create account at https://vast.ai
2. Go to Account > API Keys
3. Generate new API key
4. Add to `.env`: `VAST_API_KEY=your-key`

### Lambda Labs

1. Create account at https://lambdalabs.com
2. Go to https://cloud.lambdalabs.com/api-keys
3. Generate new API key
4. Add to `.env`: `LAMBDA_API_KEY=your-key`

### RunPod

1. Create account at https://runpod.io
2. Go to Settings > API Keys
3. Generate new API key with Pod permissions
4. Add to `.env`: `RUNPOD_API_KEY=your-key`

### CoreWeave

1. Create account at https://cloud.coreweave.com
2. Generate API credentials from the console
3. Add to `.env`: `COREWEAVE_API_KEY=your-key`

### Paperspace

1. Create account at https://paperspace.com
2. Go to Team Settings > API Keys
3. Generate new API key
4. Add to `.env`: `PAPERSPACE_API_KEY=your-key`

**Note:** Paperspace does not support billing verification via API. You must manually verify instance termination in their console.

## State File

continueplz maintains state in `.continueplz.state` in the current directory. This file tracks:
- Active instance details
- WireGuard connection info
- Cost accumulation
- Deadman switch heartbeat

Do not edit this file manually. Do not commit it to version control.

## Deadman Switch

The deadman switch ensures instances are terminated even if continueplz crashes or loses connection. The instance will auto-terminate after the configured timeout (default: 10 hours) without a heartbeat.

Configure with:
- `--timeout` flag: `continueplz --cheapest --timeout 4h`
- Environment variable: `DEADMAN_TIMEOUT_HOURS=4`

## Development

```bash
# Build
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Lint
make lint

# Format code
make fmt

# Build for all platforms
make build-all

# Create release
make release
```

## License

MIT
