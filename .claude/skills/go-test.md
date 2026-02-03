# go-test

Run Go tests with coverage analysis and provide detailed results.

## Trigger
User invokes `/go-test` or asks to run tests.

## Arguments
- `[package]` - Optional: specific package to test (e.g., `./internal/config/...`)
- `-v` - Verbose output
- `-cover` - Show coverage (default: on)

## Instructions

1. **Run tests with coverage**:
   ```bash
   go test -v -cover -coverprofile=coverage.out ./...
   ```

2. **If specific package requested**:
   ```bash
   go test -v -cover -coverprofile=coverage.out [package]
   ```

3. **Generate coverage report**:
   ```bash
   go tool cover -func=coverage.out
   ```

4. **Analyze results**:
   - Count passed/failed/skipped tests
   - Identify packages with low coverage (<60%)
   - Find untested functions

5. **If tests fail**:
   - Show the failing test output
   - Identify the assertion that failed
   - Suggest potential fixes

## Output Format

```
## Test Results

### Summary
- Total: 42 tests
- Passed: 40
- Failed: 2
- Skipped: 0

### Coverage
- Overall: 72.3%
- internal/config: 85.2%
- internal/provider: 68.1% (needs attention)
- internal/wireguard: 45.3% (low coverage)

### Failures
1. TestStateLoad (internal/config/state_test.go:45)
   - Expected: "running"
   - Got: "stopped"
   - Suggestion: Check state initialization in setUp()

### Uncovered Functions
- internal/provider/vast/client.go: handleRateLimit()
- internal/wireguard/tunnel.go: cleanupOrphanedInterface()
```
