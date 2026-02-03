# session-end

Properly close a Claude Code session by updating tracking files and summarizing progress.

## Trigger
User invokes `/session-end` or indicates they're done for now.

## Instructions

1. **Summarize work done this session**:
   - List features completed
   - List features partially completed
   - List files created/modified

2. **Update FEATURES.md**:
   - Mark completed features as `[x]`
   - Mark in-progress features as `[~]`
   - Update progress table at bottom

3. **Update MEMORY.md**:

   a. Update "Current Project Status" section:
   ```markdown
   **Overall Progress**: X/64 features (Y%)
   **Current Phase**: [Phase name]
   **Last Updated**: [Today's date]

   ### Last Completed Feature
   [Feature ID]: [Feature name]

   ### Current/Next Feature to Work On
   [Feature ID]: [Feature name]
   ```

   b. Add session log entry:
   ```markdown
   ### Session N - [Date]
   **Claude Model**: [Model name]
   **Duration**: [Approximate duration]
   **Work Done**:
   - Completed F00X: [Description]
   - Started F00Y: [Description]
   - Fixed bug in [file]

   **Notes**:
   - [Any important observations]
   - [Decisions made]
   ```

   c. Update other sections if needed:
   - Add any new blockers
   - Document technical decisions
   - Note any code patterns established
   - Update files created list

4. **Verify all changes saved**:
   ```bash
   git status
   ```

5. **Optionally create commit** (if user requested):
   - Stage changed files
   - Create descriptive commit message

## Output Format

```
## Session Complete

### Work Summary
- **Features Completed**: F001, F002
- **Features In Progress**: F003 (80% done)
- **Files Created**: 5
- **Files Modified**: 3

### Progress Update
- Before: 0/64 (0%)
- After: 2/64 (3.1%)

### Next Session Should
1. Complete F003 (remaining: implement validation)
2. Start F004

### Files Updated
- [x] FEATURES.md - marked F001, F002 complete
- [x] MEMORY.md - added session log, updated status

### Git Status
```
modified:   FEATURES.md
modified:   MEMORY.md
new file:   cmd/continueplz/main.go
new file:   internal/config/config.go
```

Would you like me to commit these changes?
```
