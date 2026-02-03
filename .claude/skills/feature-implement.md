# feature-implement

Implement a specific feature from FEATURES.md following project conventions.

## Trigger
User invokes `/feature-implement [feature-id]` or asks to implement a feature.

## Arguments
- `[feature-id]` - Feature ID from FEATURES.md (e.g., "F001", "F027")

## Instructions

1. **Read feature specification**:
   - Open FEATURES.md
   - Find the specified feature
   - Read description, tasks, and acceptance criteria
   - Check dependencies are complete

2. **Check current state**:
   - Read MEMORY.md for context
   - Verify prerequisite features are done
   - Check for any blockers noted

3. **Plan implementation**:
   - List files to create/modify
   - Identify imports needed
   - Consider edge cases

4. **Implement the feature**:
   - Follow Go best practices
   - Match existing code style
   - Add appropriate error handling
   - Include logging where appropriate

5. **Verify acceptance criteria**:
   - Go through each criterion
   - Test manually or write tests
   - Fix any issues

6. **Update tracking**:
   - Mark feature as `[x]` in FEATURES.md
   - Update MEMORY.md with:
     - Feature completed
     - Any decisions made
     - Any issues encountered
     - Files created/modified

## Code Standards for This Project

### Error Handling
```go
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

### Logging
```go
logger.Info().
    Str("instance_id", id).
    Str("provider", provider).
    Msg("Instance created")
```

### Context
- Always pass context.Context as first parameter
- Respect context cancellation

### Naming
- Files: snake_case.go
- Types: PascalCase
- Functions: PascalCase (exported), camelCase (private)
- Variables: camelCase

### Comments
- Package comments for every package
- Doc comments for exported types/functions
- No obvious comments ("// increment counter")

## Output Format

```
## Feature Implementation: [FEATURE_ID]

### Summary
[Brief description of what was implemented]

### Files Modified
- `path/to/file.go` - [what changed]
- `path/to/new_file.go` - [new file, purpose]

### Acceptance Criteria
- [x] Criterion 1 - verified by [how]
- [x] Criterion 2 - verified by [how]

### Decisions Made
- [Any technical decisions with rationale]

### Follow-up
- [Any tasks spawned or issues discovered]

### Testing
```bash
# Commands to verify the implementation
go build ./...
go test ./internal/[package]/...
```
```
