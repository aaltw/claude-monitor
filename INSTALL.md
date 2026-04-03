# INSTALL.md

This file is written for an LLM (Claude Code, Cursor, etc.) to follow step-by-step to install claude-monitor on a user's machine. Every command is exact and copy-pasteable.

**Important for LLM installers:** Each step's commands should be run as a single chained command (using `&&`) from the repo root directory to avoid working directory issues between tool calls.

## Prerequisites

- **Go 1.26+** — verify with `go version`
- **tmux** — required for session state tracking; the monitor itself runs without it but sessions will show as Idle
- **Claude Code CLI** — verify with `claude --version`
- **jq** — used by hook scripts; verify with `jq --version`

## Step 1: Clone and build

```bash
mkdir -p ~/.local/share && git clone https://github.com/aaltw/claude-monitor.git ~/.local/share/claude-monitor && cd ~/.local/share/claude-monitor && go build -o claude-monitor ./cmd/monitor/
```

Verify the binary was built: `ls -la ~/.local/share/claude-monitor/claude-monitor`

## Step 2: Build the statusline bridge

The statusline binary feeds usage data from Claude Code into claude-monitor. It reads Claude Code's status JSON from stdin and writes bridge files to `$TMPDIR`. It is a separate Go module in `scripts/statusline/`.

```bash
cd ~/.local/share/claude-monitor/scripts/statusline && go build -o statusline . && mkdir -p ~/.claude && cp statusline ~/.claude/statusline-bin
```

Verify: `echo 'not json' | ~/.claude/statusline-bin` should output `statusline: invalid JSON input` to stderr.

## Step 3: Install hook scripts

Copy the hook scripts that report session state to claude-monitor:

```bash
mkdir -p ~/.claude/hooks && cp ~/.local/share/claude-monitor/scripts/hooks/tmux-status.sh ~/.claude/hooks/claude-tmux-status.sh && cp ~/.local/share/claude-monitor/scripts/hooks/context-monitor.sh ~/.claude/hooks/context-monitor.sh && chmod +x ~/.claude/hooks/claude-tmux-status.sh ~/.claude/hooks/context-monitor.sh
```

## Step 4: Configure Claude Code settings

Read `~/.claude/settings.json` if it exists. You need to add/merge two things:

### 4a: Add the statusLine key

Add this top-level key (it does not conflict with any existing key):

```json
"statusLine": {
  "type": "command",
  "command": "~/.claude/statusline-bin"
}
```

### 4b: Add hook entries

The `hooks` key contains arrays for each event type. If the user already has hooks, **append** the new entries to the existing arrays. If the event type doesn't exist yet, create it.

Add these hook entries:

**PreToolUse** — append to the `PreToolUse` array:
```json
{
  "matcher": "",
  "hooks": [
    {
      "type": "command",
      "command": "~/.claude/hooks/claude-tmux-status.sh pretooluse",
      "async": true
    }
  ]
}
```

**PostToolUse** — append to the `PostToolUse` array:
```json
{
  "matcher": "",
  "hooks": [
    {
      "type": "command",
      "command": "~/.claude/hooks/context-monitor.sh",
      "async": true
    }
  ]
}
```

**Stop** — append to the `Stop` array:
```json
{
  "matcher": "",
  "hooks": [
    {
      "type": "command",
      "command": "~/.claude/hooks/claude-tmux-status.sh stop"
    }
  ]
}
```

**Notification** — append to the `Notification` array:
```json
{
  "matcher": "",
  "hooks": [
    {
      "type": "command",
      "command": "~/.claude/hooks/claude-tmux-status.sh notification"
    }
  ]
}
```

### Merge algorithm for LLMs

If `~/.claude/settings.json` does not exist, create it with the full config. If it exists:

1. Read the existing JSON
2. Set `statusLine` key (overwrites if present, which is fine)
3. For each hook event type (`PreToolUse`, `PostToolUse`, `Stop`, `Notification`):
   - If `hooks[eventType]` doesn't exist, create it as an array with the new entry
   - If it exists, append the new entry to the array (do not deduplicate — Claude Code handles multiple matchers)
4. Write the result back

## Step 5: Install the binary (optional)

To make `claude-monitor` available system-wide:

```bash
sudo cp ~/.local/share/claude-monitor/claude-monitor /usr/local/bin/
```

Or symlink it: `ln -sf ~/.local/share/claude-monitor/claude-monitor ~/.local/bin/claude-monitor` (ensure `~/.local/bin` is on `$PATH`).

## Step 6: Run

### TUI mode (terminal dashboard)

```bash
claude-monitor
```

Keybindings: `q` quit, `r` force refresh, `s` cycle sort order (name, status, latency).

Requires a terminal with at least 120 columns and 30 rows.

### Web mode (browser dashboard)

```bash
claude-monitor web
```

Opens on `http://localhost:3000`. Flags:
- `-p 8080` — custom port
- `--dev` — serve static files from disk (for development)

## Verification

After installation, start a Claude Code session in tmux and verify:

1. Bridge files appear: `ls $TMPDIR/claude-monitor-*.json` — should show at least one file after Claude Code does some work
2. Session state files appear: `ls $TMPDIR/claude-session-state-*.json` — should appear after Claude Code runs a tool
3. TUI shows data: run `claude-monitor` in a 120+ column terminal — usage bars and session table should populate
4. Web shows data: run `claude-monitor web` and open `http://localhost:3000` — dashboard should show live data

## Troubleshooting

### No usage data showing

The statusline binary must be configured in `~/.claude/settings.json` under `statusLine`. Restart Claude Code after changing settings.

### Sessions show as "Idle" even when working

The tmux status hook writes state files using `$PPID` to identify the Claude process. This only works when Claude Code runs inside tmux. Sessions in standalone terminals or IDEs will show as Idle but their latency and usage data still works.

### Latency shows "INF"

The bridge file for that session hasn't been written yet. This happens if the session hasn't made any API calls since the statusline was configured. Use Claude Code normally and the data will appear.

### Web dashboard shows "Disconnected"

The WebSocket connection dropped. The dashboard auto-reconnects every 3 seconds. If it persists, check that `claude-monitor web` is still running.

## Architecture overview

```
Claude Code session
    │
    ├── statusline-bin (stdin JSON → $TMPDIR/claude-monitor-{session}.json)
    │
    ├── tmux-status.sh hook (writes $TMPDIR/claude-session-state-{pid}.json)
    │
    └── context-monitor.sh hook (context usage warnings)

claude-monitor
    │
    ├── Reads bridge files + session state + ~/.claude/sessions/
    │
    ├── TUI mode (bubbletea) — terminal dashboard
    │
    └── Web mode (HTTP + WebSocket) — browser dashboard
        ├── /ws — real-time data push
        ├── /api/tmux/focus/{pid} — click to switch tmux pane
        └── Static files (embedded or --dev from disk)
```
