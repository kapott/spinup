#!/bin/bash
#
# check-progress.sh
#
# Quick status check for the continueplz project
#

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FEATURES_FILE="$PROJECT_DIR/FEATURES.md"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo ""
echo -e "${BLUE}continueplz - Project Progress${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Count features by status (grep -c returns 1 on no match but still outputs 0)
TOTAL=$(grep -cE "^### F[0-9]+:" "$FEATURES_FILE" 2>/dev/null) || TOTAL=64
COMPLETED=$(grep -c "^\*\*Status\*\*: \`\[x\]\`" "$FEATURES_FILE" 2>/dev/null) || COMPLETED=0
IN_PROGRESS=$(grep -c "^\*\*Status\*\*: \`\[~\]\`" "$FEATURES_FILE" 2>/dev/null) || IN_PROGRESS=0
BLOCKED=$(grep -c "^\*\*Status\*\*: \`\[!\]\`" "$FEATURES_FILE" 2>/dev/null) || BLOCKED=0
NOT_STARTED=$(grep -c "^\*\*Status\*\*: \`\[ \]\`" "$FEATURES_FILE" 2>/dev/null) || NOT_STARTED=$TOTAL

# Calculate percentages
COMPLETED_PCT=$((COMPLETED * 100 / TOTAL))

# Progress bar
BAR_WIDTH=40
FILLED=$((COMPLETED_PCT * BAR_WIDTH / 100))
EMPTY=$((BAR_WIDTH - FILLED))
BAR=$(printf "%${FILLED}s" | tr ' ' '█')$(printf "%${EMPTY}s" | tr ' ' '░')

echo -e "Progress: [${GREEN}${BAR}${NC}] $COMPLETED_PCT%"
echo ""
echo -e "  ${GREEN}✓ Completed:${NC}     $COMPLETED"
echo -e "  ${YELLOW}~ In Progress:${NC}   $IN_PROGRESS"
echo -e "  ${RED}! Blocked:${NC}       $BLOCKED"
echo -e "    Not Started:   $NOT_STARTED"
echo -e "  ─────────────────"
echo -e "    Total:         $TOTAL"
echo ""

# Show current/next feature
if [[ $IN_PROGRESS -gt 0 ]]; then
    echo -e "${YELLOW}Currently in progress:${NC}"
    grep -B1 "^\*\*Status\*\*: \`\[~\]\`" "$FEATURES_FILE" | grep "^### F" | sed 's/^### /  /'
    echo ""
fi

if [[ $BLOCKED -gt 0 ]]; then
    echo -e "${RED}Blocked features:${NC}"
    grep -B1 "^\*\*Status\*\*: \`\[!\]\`" "$FEATURES_FILE" | grep "^### F" | sed 's/^### /  /'
    echo ""
fi

# Show next feature to work on
NEXT=$(grep -B1 "^\*\*Status\*\*: \`\[ \]\`" "$FEATURES_FILE" | grep "^### F" | head -1 | sed 's/^### //')
if [[ -n "$NEXT" ]]; then
    echo -e "${BLUE}Next feature:${NC}"
    echo -e "  $NEXT"
    echo ""
fi

# Phase breakdown
echo -e "${BLUE}Phase Breakdown:${NC}"
echo ""
printf "  %-20s %s\n" "Phase" "Progress"
echo "  ────────────────────────────────"

# Count features per phase using feature number ranges
count_phase_progress() {
    local start=$1
    local end=$2
    local phase_total=0
    local phase_done=0

    for i in $(seq "$start" "$end"); do
        fnum=$(printf "F%03d" "$i")
        if grep -q "^### $fnum:" "$FEATURES_FILE" 2>/dev/null; then
            phase_total=$((phase_total + 1))
            if grep -A1 "^### $fnum:" "$FEATURES_FILE" | grep -q '\[x\]' 2>/dev/null; then
                phase_done=$((phase_done + 1))
            fi
        fi
    done
    echo "$phase_done/$phase_total"
}

printf "  %-20s %s\n" "Foundation" "$(count_phase_progress 1 5)"
printf "  %-20s %s\n" "Core Types" "$(count_phase_progress 6 8)"
printf "  %-20s %s\n" "Providers" "$(count_phase_progress 9 17)"
printf "  %-20s %s\n" "WireGuard" "$(count_phase_progress 18 23)"
printf "  %-20s %s\n" "Deployment" "$(count_phase_progress 24 29)"
printf "  %-20s %s\n" "TUI" "$(count_phase_progress 30 38)"
printf "  %-20s %s\n" "Commands" "$(count_phase_progress 39 47)"
printf "  %-20s %s\n" "Reliability" "$(count_phase_progress 48 50)"
printf "  %-20s %s\n" "Alerting" "$(count_phase_progress 51 53)"
printf "  %-20s %s\n" "Testing" "$(count_phase_progress 54 60)"
printf "  %-20s %s\n" "Polish" "$(count_phase_progress 61 64)"

echo ""
echo "Run './scripts/implement-one-feature.sh' to implement next feature"
echo "Run './scripts/implement-all-features.sh' to implement all features"
echo ""
