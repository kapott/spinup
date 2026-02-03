# go-lint

Run Go linting and formatting on the codebase, fixing issues automatically where possible.

## Trigger
User invokes `/go-lint` or asks to lint/format Go code.

## Instructions

1. **Check if Go tools are available**:
   ```bash
   which go gofmt
   ```

2. **Run gofmt to fix formatting**:
   ```bash
   gofmt -w -s ./...
   ```

3. **Run go vet for static analysis**:
   ```bash
   go vet ./...
   ```

4. **If golangci-lint is available, run it**:
   ```bash
   which golangci-lint && golangci-lint run --fix ./...
   ```

5. **Check for common issues**:
   - Unused imports
   - Unused variables
   - Missing error checks
   - Inefficient string concatenation

6. **Report findings**:
   - List files modified by formatting
   - List any issues that couldn't be auto-fixed
   - Suggest manual fixes if needed

## Output Format

```
## Go Lint Results

### Formatting
- [x] Fixed: path/to/file.go
- [x] Fixed: path/to/other.go

### Static Analysis
- No issues found

### Suggestions
- Consider adding error check at file.go:42
```
