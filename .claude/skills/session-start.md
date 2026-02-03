# session-start

Initialize a new Claude Code session by reading project state and determining next steps.

## Trigger
User invokes `/session-start` or starts a new session asking to continue work.

## Instructions

1. **Read MEMORY.md** to understand:
   - Current project status
   - Last completed feature
   - Current/next feature to work on
   - Active blockers
   - Technical decisions made
   - Known issues

2. **Read FEATURES.md** to understand:
   - Full feature list
   - Current progress (check status markers)
   - Dependencies between features

3. **Verify actual code state**:
   ```bash
   # Check what files exist
   find . -name "*.go" -type f | head -20

   # Check if project builds
   go build ./... 2>&1 | head -10

   # Check test status
   go test ./... 2>&1 | tail -5
   ```

4. **Reconcile any discrepancies**:
   - If MEMORY.md says F005 done but files don't exist, investigate
   - If code exists that isn't tracked, update FEATURES.md

5. **Determine next action**:
   - Find lowest-numbered incomplete feature with completed dependencies
   - Verify no blockers prevent working on it
   - Prepare to implement

6. **Report status to user**

## Output Format

```
## Session Started

### Project Status
- **Overall Progress**: X/64 features (Y%)
- **Current Phase**: [Phase name]
- **Last Session**: [Date and what was done]

### Code Health
- Build: [PASS/FAIL]
- Tests: [X passed, Y failed, Z skipped]
- Lint: [PASS/FAIL/Not configured]

### Ready to Work On
**[Feature ID]: [Feature Name]**
- Dependencies: All satisfied
- Estimated files: N
- [Brief description]

### Blockers
[None / List of blockers]

### Quick Commands
- `/feature-implement [ID]` - Start implementing the next feature
- `/go-test` - Run tests
- `/go-lint` - Check code style

Shall I proceed with implementing [Feature ID]?
```
