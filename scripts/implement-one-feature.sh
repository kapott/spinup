#!/bin/bash
#
# implement-one-feature.sh
#
# Starts a fresh Claude Code instance, implements exactly one feature,
# updates tracking files, and exits.
#
# Usage: ./scripts/implement-one-feature.sh
# Loop:  while ./scripts/implement-one-feature.sh; do echo "Next..."; done
#

set -e

# Configuration
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FEATURES_FILE="$PROJECT_DIR/FEATURES.md"
MEMORY_FILE="$PROJECT_DIR/MEMORY.md"
LOG_DIR="$PROJECT_DIR/logs"
TIMESTAMP=$(date +"%Y%m%d-%H%M%S")
LOG_FILE="$LOG_DIR/feature-$TIMESTAMP.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Ensure log directory exists
mkdir -p "$LOG_DIR"

echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}  continueplz - Automated Feature Implementation${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
echo ""

# Check if FEATURES.md exists
if [[ ! -f "$FEATURES_FILE" ]]; then
    echo -e "${RED}ERROR: FEATURES.md not found at $FEATURES_FILE${NC}"
    exit 1
fi

# Find the next feature to implement
# Look for features marked as [ ] (not started) whose dependencies are complete
# This is a simple grep - Claude will do the actual dependency checking
NEXT_FEATURE=$(grep -E "^### F[0-9]+:" "$FEATURES_FILE" | while read -r line; do
    # Extract feature ID
    FEATURE_ID=$(echo "$line" | grep -oE "F[0-9]+")

    # Check if this feature is not started (has [ ] status on next line)
    STATUS=$(grep -A1 "^### $FEATURE_ID:" "$FEATURES_FILE" | grep -oE "\[.\]" | head -1)

    if [[ "$STATUS" == "[ ]" ]]; then
        echo "$FEATURE_ID"
        break
    fi
done)

# Check if all features are complete
if [[ -z "$NEXT_FEATURE" ]]; then
    # Double-check by counting incomplete features
    INCOMPLETE=$(grep -c "^\*\*Status\*\*: \`\[ \]\`" "$FEATURES_FILE" 2>/dev/null) || INCOMPLETE=0

    if [[ "$INCOMPLETE" == "0" ]]; then
        echo -e "${GREEN}✓ All features have been implemented!${NC}"
        echo ""
        echo "Project complete. Check FEATURES.md for final status."
        exit 1  # Exit with 1 to stop the while loop
    fi

    # There might be features but with complex status
    NEXT_FEATURE=$(grep -B1 "^\*\*Status\*\*: \`\[ \]\`" "$FEATURES_FILE" | grep -oE "F[0-9]+" | head -1)
fi

if [[ -z "$NEXT_FEATURE" ]]; then
    echo -e "${GREEN}✓ No more features to implement!${NC}"
    exit 1
fi

echo -e "Next feature: ${GREEN}$NEXT_FEATURE${NC}"
echo "Log file: $LOG_FILE"
echo ""

# Count progress
TOTAL_FEATURES=$(grep -cE "^### F[0-9]+:" "$FEATURES_FILE" 2>/dev/null) || TOTAL_FEATURES=64
COMPLETED_FEATURES=$(grep -c "^\*\*Status\*\*: \`\[x\]\`" "$FEATURES_FILE" 2>/dev/null) || COMPLETED_FEATURES=0
PROGRESS=$((COMPLETED_FEATURES * 100 / TOTAL_FEATURES))

echo -e "Progress: ${GREEN}$COMPLETED_FEATURES${NC}/$TOTAL_FEATURES ($PROGRESS%)"
echo ""

# Create the prompt for Claude Code
PROMPT="You are starting a fresh session to implement exactly ONE feature for the continueplz project.

CRITICAL INSTRUCTIONS:
1. Read MEMORY.md to understand current project state
2. Read FEATURES.md to find feature $NEXT_FEATURE
3. Implement ONLY feature $NEXT_FEATURE - nothing else
4. After implementing, update:
   - FEATURES.md: Mark $NEXT_FEATURE as [x] complete (or [~] if partially done)
   - MEMORY.md: Add session log entry with what was done
5. Verify the code compiles: go build ./... (if Go files exist)
6. Then EXIT immediately - do not ask what to do next

IMPORTANT:
- Implement the feature completely according to its acceptance criteria
- If blocked by missing dependencies, mark as [!] and document in MEMORY.md
- Do NOT start any other features
- Do NOT wait for user input after completing
- EXIT when done

Begin now. Implement feature $NEXT_FEATURE."

# Run Claude Code with the prompt
echo -e "${YELLOW}Starting Claude Code...${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Log the start
echo "=== Feature Implementation Log ===" > "$LOG_FILE"
echo "Feature: $NEXT_FEATURE" >> "$LOG_FILE"
echo "Started: $(date)" >> "$LOG_FILE"
echo "---" >> "$LOG_FILE"

# Run Claude Code
# --print: outputs the conversation
# --dangerously-skip-permissions: allows file operations without prompts (use with caution)
# Pipe to tee to show output and log it
cd "$PROJECT_DIR"

if command -v claude &> /dev/null; then
    # Claude Code CLI exists
    claude --print --dangerously-skip-permissions "$PROMPT" 2>&1 | tee -a "$LOG_FILE"
    EXIT_CODE=${PIPESTATUS[0]}
else
    echo -e "${RED}ERROR: 'claude' command not found${NC}"
    echo "Please install Claude Code CLI first."
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Log completion
echo "---" >> "$LOG_FILE"
echo "Ended: $(date)" >> "$LOG_FILE"
echo "Exit code: $EXIT_CODE" >> "$LOG_FILE"

# Check if feature was completed
NEW_STATUS=$(grep -A1 "^### $NEXT_FEATURE:" "$FEATURES_FILE" | grep -oE "\[.\]" | head -1)

if [[ "$NEW_STATUS" == "[x]" ]]; then
    echo -e "${GREEN}✓ Feature $NEXT_FEATURE completed successfully${NC}"
elif [[ "$NEW_STATUS" == "[~]" ]]; then
    echo -e "${YELLOW}~ Feature $NEXT_FEATURE partially completed${NC}"
elif [[ "$NEW_STATUS" == "[!]" ]]; then
    echo -e "${RED}! Feature $NEXT_FEATURE is blocked${NC}"
    echo "Check MEMORY.md for blocker details"
    exit 1  # Stop the loop if blocked
else
    echo -e "${YELLOW}? Feature $NEXT_FEATURE status unclear (was: $NEW_STATUS)${NC}"
fi

echo ""
echo "Log saved to: $LOG_FILE"
echo ""

# Return success to continue the loop
exit 0
