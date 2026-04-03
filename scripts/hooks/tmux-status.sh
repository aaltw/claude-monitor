#!/usr/bin/env bash
# Claude Code hook: set status icons on tmux window names
# Icons: ○ (idle/stopped), ◐ (waiting for input/notification)
# Usage: claude-tmux-status.sh [stop|notification|pretooluse|restore]

set -euo pipefail

STATE_DIR="${HOME}/.claude/.tmux-pane-state"
mkdir -p "$STATE_DIR"

# Get the pane where Claude is running (not necessarily the active pane)
get_pane_id() {
  echo "${TMUX_PANE:-$(tmux display-message -p '#{pane_id}' 2>/dev/null)}"
}

state_file() {
  local pane_id="$1"
  echo "${STATE_DIR}/${pane_id//%/}.json"
}

save_state() {
  local pane_id="$1" original_name="$2" auto_rename="$3"
  printf '{"pane_id":"%s","original_name":"%s","auto_rename":"%s"}\n' \
    "$pane_id" "$original_name" "$auto_rename" > "$(state_file "$pane_id")"
}

load_original_name() {
  local sf
  sf="$(state_file "$1")"
  if [[ -f "$sf" ]]; then
    jq -r '.original_name' "$sf" 2>/dev/null || true
  fi
}

load_auto_rename() {
  local sf
  sf="$(state_file "$1")"
  if [[ -f "$sf" ]]; then
    jq -r '.auto_rename' "$sf" 2>/dev/null || true
  fi
}

get_window_name() {
  tmux display-message -p -t "$1" '#{window_name}' 2>/dev/null
}

get_auto_rename() {
  local out
  out="$(tmux show-options -t "$1" automatic-rename 2>/dev/null)" || true
  [[ "$out" == *"on"* ]] && echo "on" || echo "off"
}

strip_icon() {
  local name="$1"
  # Remove any existing icon prefix
  name="${name#○ }"
  name="${name#◐ }"
  name="${name#● }"
  echo "$name"
}

set_icon() {
  local pane_id="$1" icon="$2"

  local current_name
  current_name="$(get_window_name "$pane_id")" || return 0

  # Get or save original name
  local original_name
  original_name="$(load_original_name "$pane_id")"
  if [[ -z "$original_name" ]]; then
    original_name="$(strip_icon "$current_name")"
    local auto_rename
    auto_rename="$(get_auto_rename "$pane_id")"
    save_state "$pane_id" "$original_name" "$auto_rename"
  fi

  # Disable auto-rename and set the icon
  tmux set-option -t "$pane_id" automatic-rename off 2>/dev/null || true
  tmux rename-window -t "$pane_id" "${icon} ${original_name}" 2>/dev/null || true
}

restore() {
  local pane_id="$1"
  local sf
  sf="$(state_file "$pane_id")"
  [[ -f "$sf" ]] || return 0

  local original_name auto_rename
  original_name="$(load_original_name "$pane_id")"
  auto_rename="$(load_auto_rename "$pane_id")"

  tmux rename-window -t "$pane_id" "$original_name" 2>/dev/null || true
  tmux set-option -t "$pane_id" automatic-rename "${auto_rename:-on}" 2>/dev/null || true
  rm -f "$sf"
}

# Cleanup state files for panes that no longer exist
cleanup_stale() {
  local existing_panes
  existing_panes="$(tmux list-panes -a -F '#{pane_id}' 2>/dev/null)" || return 0
  for sf in "$STATE_DIR"/*.json; do
    [[ -f "$sf" ]] || continue
    local pane_id
    pane_id="%$(basename "$sf" .json)"
    if ! echo "$existing_panes" | grep -qF "$pane_id"; then
      rm -f "$sf"
    fi
  done
}

# Write session state for claude-monitor TUI
write_monitor_state() {
  local event="$1" state="$2"
  local pid="${PPID:-$$}"
  local session_id=""

  # Try to get session_id from environment
  if [[ -n "${CLAUDE_SESSION_ID:-}" ]]; then
    session_id="$CLAUDE_SESSION_ID"
  fi

  local state_file="${TMPDIR:-/tmp}/claude-session-state-${pid}.json"
  printf '{"pid":%d,"session_id":"%s","state":"%s","updated_at":%d,"event":"%s"}\n' \
    "$pid" "$session_id" "$state" "$(date +%s)" "$event" > "$state_file"
}

# Main
pane_id="$(get_pane_id)"
[[ -n "$pane_id" ]] || exit 0

case "${1:-}" in
  stop)
    set_icon "$pane_id" "○"
    write_monitor_state "stop" "idle"
    ;;
  notification)
    set_icon "$pane_id" "◐"
    # Check if this is a permission prompt
    input=""
    input="$(cat)" 2>/dev/null || true
    if echo "$input" | grep -qiE 'permission|allow|approve|deny|trust'; then
      write_monitor_state "notification" "blocked"
    else
      write_monitor_state "notification" "waiting"
    fi
    ;;
  pretooluse)
    set_icon "$pane_id" "◐"
    write_monitor_state "pretooluse" "working"
    ;;
  restore)      restore "$pane_id" ;;
  cleanup)      cleanup_stale ;;
  *)            echo "Usage: $0 [stop|notification|pretooluse|restore|cleanup]" >&2; exit 1 ;;
esac
