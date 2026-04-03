# INSTALL.md

This file is written for an LLM (Claude Code, Cursor, etc.) to follow step-by-step to install claude-monitor on a user's machine. Every command is exact and copy-pasteable.

## Prerequisites

- **Go 1.21+** — verify with `go version`
- **tmux** — verify with `tmux -V`
- **Claude Code CLI** — verify with `claude --version`
- **jq** — verify with `jq --version`

## Step 1: Clone and build

```bash
git clone https://github.com/aaltw/claude-monitor.git
cd claude-monitor
go build -o claude-monitor ./cmd/monitor/
```

Verify: `./claude-monitor --help` or just `./claude-monitor` (press `q` to quit the TUI).

## Step 2: Build the statusline bridge

The statusline binary is what feeds usage data from Claude Code into claude-monitor. It reads Claude Code's status JSON from stdin and writes bridge files to `$TMPDIR`.

```bash
cd scripts/statusline
go build -o statusline .
cp statusline ~/.claude/statusline-bin
cd ../..
```

Verify: `echo '{}' | ~/.claude/statusline-bin` should output `statusline: invalid JSON input` (expected — it needs real Claude Code status JSON).

## Step 3: Install hook scripts

Copy the hook scripts that report session state to claude-monitor:

```bash
mkdir -p ~/.claude/hooks
cp scripts/hooks/tmux-status.sh ~/.claude/hooks/claude-tmux-status.sh
cp scripts/hooks/context-monitor.sh ~/.claude/hooks/context-monitor.sh
chmod +x ~/.claude/hooks/claude-tmux-status.sh
chmod +x ~/.claude/hooks/context-monitor.sh
```

## Step 4: Configure Claude Code settings

Add the following to `~/.claude/settings.json`. If the file already exists, merge these keys into it. If it doesn't exist, create it with this content:

```json
{
  "statusLine": {
    "type": "command",
    "command": "~/.claude/statusline-bin"
  },
  "hooks": {
    "PreToolUse": [
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
    ],
    "PostToolUse": [
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
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/claude-tmux-status.sh stop"
          }
        ]
      }
    ],
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/claude-tmux-status.sh notification"
          }
        ]
      }
    ]
  }
}
```

**Important:** If the user already has hooks configured, the new hook entries must be **appended** to the existing arrays, not replace them. Read the existing file first, merge, then write.

## Step 5: Install the binary (optional)

To make `claude-monitor` available system-wide:

```bash
cp claude-monitor /usr/local/bin/
```

Or add the build directory to `$PATH`.

## Step 6: Run

### TUI mode (terminal dashboard)

```bash
claude-monitor
```

Keybindings: `q` quit, `r` force refresh, `s` cycle sort order.

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
3. TUI shows data: run `claude-monitor` — usage bars and session table should populate
4. Web shows data: run `claude-monitor web` and open `http://localhost:3000` — dashboard should show live data

## Troubleshooting

### No usage data showing

The statusline binary must be configured in `~/.claude/settings.json` under `statusLine`. Restart Claude Code after changing settings.

### Sessions show as "Idle" even when working

The tmux status hook (`claude-tmux-status.sh`) writes state files using `$PPID` to identify the Claude process. This only works when Claude Code runs inside tmux. Sessions outside tmux will show as Idle.

### Latency shows "INF"

The bridge file for that session hasn't been written yet. This happens if the session hasn't made any API calls since the statusline was configured. Use Claude Code normally and the data will appear.

### Web dashboard shows "Disconnected"

The WebSocket connection dropped. The dashboard auto-reconnects every 3 seconds. If it persists, check that `claude-monitor web` is still running.

## Architecture overview

```
Claude Code session
    │
    ├── statusline-bin (stdin JSON → /tmp/claude-monitor-{session}.json)
    │
    ├── tmux-status.sh hook (writes /tmp/claude-session-state-{pid}.json)
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
