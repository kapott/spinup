# CLAUDE.md - Instructions for Claude Code

> This file contains project-specific instructions for Claude Code sessions working on continueplz.

## Project Overview

**continueplz** is a CLI tool written in Go that spins up ephemeral GPU instances with code-assist LLMs. It compares prices across cloud GPU providers, deploys models via Ollama, sets up WireGuard tunnels, and guarantees cleanup.

## Critical Files to Read First

1. **MEMORY.md** - Current project state, what was done, what's next
2. **FEATURES.md** - Complete feature breakdown with status tracking
3. **continueplz-prd.md** - Full product requirements (reference as needed)

## Session Workflow

### Starting a Session

```
1. ALWAYS read MEMORY.md first
2. Check "Current/Next Feature to Work On" section
3. Verify dependencies are complete in FEATURES.md
4. Begin implementation
```

Or use: `/session-start`

### During Development

1. **Pick the lowest-numbered incomplete feature** whose dependencies are done
2. **Mark feature as `[~]`** (in progress) in FEATURES.md before starting
3. **Implement following the feature's tasks and acceptance criteria**
4. **Test your work** - code should compile, tests should pass
5. **Mark feature as `[x]`** (complete) when done

### Ending a Session

Before ending, you MUST:

1. Update FEATURES.md with completed/in-progress status
2. Update MEMORY.md with:
   - New session log entry
   - Updated "Current Project Status" section
   - Any decisions made or blockers found
3. List files created/modified

Or use: `/session-end`

## Available Skills

| Skill | When to Use |
|-------|-------------|
| `/session-start` | Beginning of every session |
| `/session-end` | End of every session |
| `/feature-implement F001` | Implement a specific feature |
| `/go-lint` | After writing Go code |
| `/go-test` | To run tests with coverage |
| `/api-mock [provider]` | When implementing provider tests |
| `/tui-scaffold [name]` | When creating TUI components |
| `/provider-api [name]` | When implementing a provider client |
| `/security-review` | Before completing security-sensitive features |
| `/cross-compile` | When building releases |

## Code Standards

### Go Conventions
- **Error handling**: Always wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- **Logging**: Use zerolog with structured fields
- **Context**: Pass `context.Context` as first parameter
- **Naming**: Files `snake_case.go`, types `PascalCase`, private funcs `camelCase`

### Project Structure
```
cmd/continueplz/     - Entry point
internal/
  config/            - Configuration and state
  provider/          - Cloud provider implementations
    vast/
    lambda/
    runpod/
    coreweave/
    paperspace/
    mock/            - Mock for testing
  models/            - Model and GPU registry
  wireguard/         - WireGuard setup/teardown
  deploy/            - Deployment orchestration
  ui/                - TUI components (Bubbletea)
  alert/             - Webhook notifications
  logging/           - Structured logging
pkg/api/             - Ollama API client
templates/           - Cloud-init and WireGuard templates
tests/
  integration/
  e2e/
```

### Dependencies (from PRD)
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - TUI styling
- `github.com/charmbracelet/bubbles` - TUI components
- `github.com/joho/godotenv` - .env loading
- `github.com/spf13/cobra` - CLI framework
- `github.com/rs/zerolog` - Structured logging
- `golang.zx2c4.com/wireguard/wgctrl` - WireGuard control

## Feature Implementation Order

Features are numbered F001-F064. Always:
1. Check dependencies in FEATURES.md
2. Implement in order when possible
3. Don't skip ahead unless blocked

### Phase Priority
1. **Foundation (F001-F005)** - Must be done first
2. **Core Types (F006-F008)** - Needed for providers
3. **Providers (F009-F017)** - Can parallelize per provider
4. **WireGuard (F018-F023)** - Needed for deployment
5. **Deployment (F024-F029)** - Core functionality
6. **TUI (F030-F038)** - User interface
7. **Commands (F039-F047)** - Wire everything together
8. **Reliability (F048-F050)** - Production hardening
9. **Alerting (F051-F053)** - Notifications
10. **Testing (F054-F060)** - Quality assurance
11. **Polish (F061-F064)** - Release prep

## Important Reminders

- **Never commit .env** - Contains API keys
- **Never commit .continueplz.state** - Contains instance data
- **PRD is in Dutch** - Code and comments should be in English
- **Paperspace has no billing API** - Requires manual verification flow
- **WireGuard differs by OS** - Separate implementations for Linux/macOS
- **RunPod uses GraphQL** - Different from other REST providers

## Testing Commands

```bash
# Build
go build ./cmd/continueplz

# Test all
go test ./...

# Test with coverage
go test -cover -coverprofile=coverage.out ./...

# Lint (if installed)
golangci-lint run ./...
```

## Quick Reference

### Check Project State
```bash
# What files exist
find . -name "*.go" -type f

# Does it build
go build ./...

# Do tests pass
go test ./...
```

### Git Workflow
- Commit after completing features (if user requests)
- Use descriptive commit messages
- Never force push

## Automation Scripts

The project includes scripts for automated feature implementation:

```bash
# Check current progress
./scripts/check-progress.sh

# Implement exactly one feature (for use in loops)
./scripts/implement-one-feature.sh

# Implement all remaining features automatically
./scripts/implement-all-features.sh

# Implement with limits
./scripts/implement-all-features.sh --max-features 5
./scripts/implement-all-features.sh --delay 10
```

When invoked via these scripts, you will receive a focused prompt to implement exactly one feature. Follow it precisely:
1. Implement only the specified feature
2. Update tracking files
3. Exit immediately when done

## When Stuck

1. Re-read the feature description in FEATURES.md
2. Check the PRD section for that component
3. Look at existing code for patterns
4. Document the blocker in MEMORY.md
5. Ask the user for clarification
