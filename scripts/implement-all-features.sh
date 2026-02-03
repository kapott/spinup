#!/bin/bash
#
# implement-all-features.sh
#
# Runs implement-one-feature.sh in a loop until all features are complete
# or a blocker is encountered.
#
# Usage: ./scripts/implement-all-features.sh [OPTIONS]
#
# Options:
#   --max-features N    Stop after implementing N features (default: unlimited)
#   --delay SECONDS     Delay between features (default: 5)
#   --dry-run           Show what would be done without running Claude
#   --resume            Resume from where we left off (default behavior)
#

set -e

# Configuration
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="$PROJECT_DIR/scripts"
FEATURES_FILE="$PROJECT_DIR/FEATURES.md"
LOG_DIR="$PROJECT_DIR/logs"
SUMMARY_LOG="$LOG_DIR/implementation-summary.log"

# Defaults
MAX_FEATURES=0  # 0 = unlimited
DELAY=5
DRY_RUN=false

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --max-features)
            MAX_FEATURES="$2"
            shift 2
            ;;
        --delay)
            DELAY="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --resume)
            # Default behavior, just skip
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --max-features N    Stop after N features (default: unlimited)"
            echo "  --delay SECONDS     Delay between features (default: 5)"
            echo "  --dry-run           Show plan without running"
            echo "  --resume            Resume from current state (default)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Ensure log directory exists
mkdir -p "$LOG_DIR"

# Print header
echo -e "${BLUE}"
echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║                                                               ║"
echo "║     continueplz - Automated Feature Implementation Loop       ║"
echo "║                                                               ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Show current status
TOTAL=$(grep -cE "^### F[0-9]+:" "$FEATURES_FILE" 2>/dev/null) || TOTAL=64
COMPLETED=$(grep -c "^\*\*Status\*\*: \`\[x\]\`" "$FEATURES_FILE" 2>/dev/null) || COMPLETED=0
IN_PROGRESS=$(grep -c "^\*\*Status\*\*: \`\[~\]\`" "$FEATURES_FILE" 2>/dev/null) || IN_PROGRESS=0
BLOCKED=$(grep -c "^\*\*Status\*\*: \`\[!\]\`" "$FEATURES_FILE" 2>/dev/null) || BLOCKED=0
REMAINING=$((TOTAL - COMPLETED))

echo "Current Status:"
echo -e "  Total features:    $TOTAL"
echo -e "  Completed:         ${GREEN}$COMPLETED${NC}"
echo -e "  In progress:       ${YELLOW}$IN_PROGRESS${NC}"
echo -e "  Blocked:           ${RED}$BLOCKED${NC}"
echo -e "  Remaining:         $REMAINING"
echo ""

if [[ $MAX_FEATURES -gt 0 ]]; then
    echo -e "Will implement up to ${YELLOW}$MAX_FEATURES${NC} features"
else
    echo -e "Will implement ${YELLOW}all remaining${NC} features"
fi
echo -e "Delay between features: ${DELAY}s"
echo ""

if [[ "$DRY_RUN" == "true" ]]; then
    echo -e "${YELLOW}DRY RUN MODE - No changes will be made${NC}"
    echo ""
    echo "Features that would be implemented:"
    grep -B1 "^\*\*Status\*\*: \`\[ \]\`" "$FEATURES_FILE" | grep "^### F" | head -${MAX_FEATURES:-20}
    exit 0
fi

# Confirmation
echo -e "${YELLOW}Starting in 5 seconds... (Ctrl+C to cancel)${NC}"
sleep 5

# Initialize summary log
echo "=== Implementation Run ===" >> "$SUMMARY_LOG"
echo "Started: $(date)" >> "$SUMMARY_LOG"
echo "Max features: $MAX_FEATURES" >> "$SUMMARY_LOG"
echo "---" >> "$SUMMARY_LOG"

# Counters
FEATURES_DONE=0
FEATURES_FAILED=0
START_TIME=$(date +%s)

# Main loop
while true; do
    # Check if we've hit the max
    if [[ $MAX_FEATURES -gt 0 && $FEATURES_DONE -ge $MAX_FEATURES ]]; then
        echo ""
        echo -e "${GREEN}Reached maximum feature count ($MAX_FEATURES)${NC}"
        break
    fi

    # Run the single feature script
    echo ""
    echo -e "${BLUE}━━━ Feature $((FEATURES_DONE + 1)) ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    if "$SCRIPT_DIR/implement-one-feature.sh"; then
        FEATURES_DONE=$((FEATURES_DONE + 1))
        echo "$FEATURES_DONE features completed" >> "$SUMMARY_LOG"

        # Delay before next feature
        if [[ $DELAY -gt 0 ]]; then
            echo ""
            echo -e "${YELLOW}Waiting ${DELAY}s before next feature...${NC}"
            sleep $DELAY
        fi
    else
        EXIT_CODE=$?

        # Exit code 1 with "All features" message means we're done
        if [[ $EXIT_CODE -eq 1 ]]; then
            # Check if it's completion or failure
            REMAINING_NOW=$(grep -c "^\*\*Status\*\*: \`\[ \]\`" "$FEATURES_FILE" 2>/dev/null) || REMAINING_NOW=0

            if [[ $REMAINING_NOW -eq 0 ]]; then
                echo ""
                echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
                echo -e "${GREEN}  ALL FEATURES COMPLETED!${NC}"
                echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
                break
            else
                # Something went wrong
                FEATURES_FAILED=$((FEATURES_FAILED + 1))
                echo ""
                echo -e "${RED}Feature implementation failed or blocked${NC}"
                echo "Check the logs for details"

                # If too many failures, stop
                if [[ $FEATURES_FAILED -ge 3 ]]; then
                    echo -e "${RED}Too many failures, stopping${NC}"
                    break
                fi

                # Try to continue with next feature
                echo "Attempting to continue with next feature..."
                sleep $DELAY
            fi
        else
            echo -e "${RED}Unexpected error (exit code: $EXIT_CODE)${NC}"
            break
        fi
    fi
done

# Final summary
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
DURATION_MIN=$((DURATION / 60))
DURATION_SEC=$((DURATION % 60))

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Implementation Summary${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "  Features implemented: ${GREEN}$FEATURES_DONE${NC}"
echo -e "  Features failed:      ${RED}$FEATURES_FAILED${NC}"
echo -e "  Duration:             ${DURATION_MIN}m ${DURATION_SEC}s"
echo ""

# Update summary log
echo "---" >> "$SUMMARY_LOG"
echo "Ended: $(date)" >> "$SUMMARY_LOG"
echo "Features implemented: $FEATURES_DONE" >> "$SUMMARY_LOG"
echo "Features failed: $FEATURES_FAILED" >> "$SUMMARY_LOG"
echo "Duration: ${DURATION_MIN}m ${DURATION_SEC}s" >> "$SUMMARY_LOG"
echo "" >> "$SUMMARY_LOG"

# Final status
FINAL_COMPLETED=$(grep -c "^\*\*Status\*\*: \`\[x\]\`" "$FEATURES_FILE" 2>/dev/null) || FINAL_COMPLETED=0
FINAL_PROGRESS=$((FINAL_COMPLETED * 100 / TOTAL))
echo -e "  Project progress:     ${GREEN}$FINAL_COMPLETED${NC}/$TOTAL ($FINAL_PROGRESS%)"
echo ""
echo "Logs saved to: $LOG_DIR/"
echo ""
