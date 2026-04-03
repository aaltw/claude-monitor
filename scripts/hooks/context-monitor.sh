#!/usr/bin/env bash
# Context Monitor — PostToolUse hook
# Reads context metrics from the statusline bridge file and injects
# warnings into the agent's conversation when context usage is high.
#
# The statusline writes metrics to /tmp/claude-ctx-{session_id}.json
# This hook reads those metrics after each tool use and warns the agent.
#
# Thresholds (remaining_percentage):
#   WARNING  (<= 35%): Agent should wrap up current task
#   CRITICAL (<= 25%): Agent should stop and inform user

set -euo pipefail

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // empty')
[ -z "$SESSION_ID" ] && exit 0

TMPDIR="${TMPDIR:-/tmp}"
METRICS="${TMPDIR}/claude-ctx-${SESSION_ID}.json"
[ ! -f "$METRICS" ] && exit 0

REMAINING=$(jq -r '.remaining_percentage // empty' "$METRICS")
USED=$(jq -r '.used_pct // empty' "$METRICS")
TIMESTAMP=$(jq -r '.timestamp // 0' "$METRICS")
NOW=$(date +%s)

# Ignore stale metrics (>60s old)
[ $(( NOW - TIMESTAMP )) -gt 60 ] && exit 0

# No warning needed
[ -z "$REMAINING" ] && exit 0
REMAINING_INT=$(printf '%.0f' "$REMAINING")
[ "$REMAINING_INT" -gt 35 ] && exit 0

# Debounce: at most one warning per 5 tool calls
WARN_FILE="${TMPDIR}/claude-ctx-${SESSION_ID}-warned.json"
CALLS_SINCE=999
LAST_LEVEL=""
if [ -f "$WARN_FILE" ]; then
  CALLS_SINCE=$(jq -r '.calls // 0' "$WARN_FILE")
  LAST_LEVEL=$(jq -r '.level // empty' "$WARN_FILE")
fi

CALLS_SINCE=$(( CALLS_SINCE + 1 ))

if [ "$REMAINING_INT" -le 25 ]; then
  LEVEL="critical"
else
  LEVEL="warning"
fi

# Fire immediately on first warning or severity escalation, debounce otherwise
ESCALATED=false
[ "$LEVEL" = "critical" ] && [ "$LAST_LEVEL" = "warning" ] && ESCALATED=true

if [ "$CALLS_SINCE" -lt 5 ] && [ "$ESCALATED" = "false" ]; then
  echo "{\"calls\": $CALLS_SINCE, \"level\": \"$LAST_LEVEL\"}" > "$WARN_FILE"
  exit 0
fi

# Reset debounce
echo "{\"calls\": 0, \"level\": \"$LEVEL\"}" > "$WARN_FILE"

# Build warning message
if [ "$LEVEL" = "critical" ]; then
  MSG="CONTEXT CRITICAL: Usage at ${USED}%. Remaining: ${REMAINING_INT}%. Context is nearly exhausted. Inform the user that context is low and ask how they want to proceed."
else
  MSG="CONTEXT WARNING: Usage at ${USED}%. Remaining: ${REMAINING_INT}%. Be aware that context is getting limited. Avoid unnecessary exploration or starting new complex work."
fi

# Inject into agent conversation
jq -n --arg msg "$MSG" '{hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: $msg}}'
