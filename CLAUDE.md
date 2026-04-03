# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Real-time dashboard for monitoring Claude Code API usage, sessions, and rate limits. Two modes: terminal TUI (bubbletea) and web dashboard (HTTP + WebSocket with Chart.js). Reads bridge files written by a statusline binary and session state from tmux hooks.

## Commands

```bash
go build -o claude-monitor ./cmd/monitor/   # Build
./claude-monitor                             # Run TUI
./claude-monitor web                         # Run web dashboard (localhost:3000)
./claude-monitor web -p 8080                 # Custom port
./claude-monitor web --dev                   # Serve static files from disk (hot reload)
go test ./...                                # Run all tests
go test ./internal/data/ -run TestCalcBurn   # Run single test
go test ./internal/web/ -v                   # Run web package tests
go vet ./...                                 # Lint
```

Statusline binary (separate Go module):
```bash
cd scripts/statusline && go build -o statusline .   # Build statusline
```

## Architecture

```
cmd/monitor/main.go          Entry point: TUI (default) or web subcommand
internal/
  config/config.go            Constants: polling intervals, thresholds, paths
  data/                       Data layer (shared by TUI and web, no UI dependencies)
    types.go                  Shared structs: BridgeData, SessionInfo, UsageSnapshot, BurnRate
    bridge.go                 Reads bridge files + state files, extracts usage snapshots
    sessions.go               Reads session registry, PID checks, merges all data sources
    ringbuffer.go             Circular buffer with linear regression for burn rate
  tui/                        Terminal UI (bubbletea + lipgloss)
    model.go                  Root Model with 5 pollers + animation tick
    keymap.go                 Key bindings (q=quit, r=refresh, s=sort)
    theme/theme.go            Catppuccin Mocha palette, gradient interpolation
    components/               Stateless rendering functions (usage, burnrate, sessions, footer)
  web/                        Web dashboard server
    server.go                 HTTP server, WebSocket handler, tmux focus API
    hub.go                    WebSocket client registry + broadcast
    poller.go                 Data polling loop, state/history/event message assembly
    history.go                Ring buffer for chart data backfill (720 points)
    messages.go               JSON message types for WebSocket protocol
web/
  embed.go                    go:embed for static files (package webfs)
  static/                     Frontend: index.html, style.css, app.js (Chart.js CDN)
scripts/
  statusline/                 Separate Go module — reads Claude Code status JSON from stdin,
                              writes bridge files to $TMPDIR
  hooks/                      Claude Code hook scripts for session state tracking
  com.aaltw.claude-monitor.plist   macOS launchd agent for running as background service
```

**Data flow**: Statusline binary writes bridge files -> `internal/data/` reads them -> TUI renders or web server pushes via WebSocket

**Data sources** (all in `$TMPDIR`):
- `claude-monitor-{sessionId}.json` — written by statusline binary (usage, rate limits, tokens, model)
- `claude-session-state-{pid}.json` — written by tmux hooks (session working/idle/waiting/blocked state)
- `~/.claude/sessions/{pid}.json` — Claude Code's own session registry

**WebSocket protocol** (server → client):
- `state` — full dashboard state every 2s (usage, burn rate, sessions, per-model breakdown)
- `history` — chart data point every 5s (burn rate, tokens over time)
- `event` — session state changes in real-time

## Key Design Decisions

- `internal/data/` has zero UI dependencies — shared by both TUI and web server
- TUI components are pure rendering functions, not bubbletea sub-models
- Web static files embedded via `go:embed` in `web/embed.go` (package `webfs`), with `--dev` flag for disk serving
- Ring buffer uses linear regression over 15-minute window for burn rate
- Session status inferred from bridge file recency (< 10s = working) when no state file exists
- Tokens summed across all bridge files for accurate velocity tracking
- Tmux focus API (`/api/tmux/focus/{pid}`) walks process tree with `pgrep -P` to find the right pane, then `switch-client` + `select-window` + `select-pane`
- Go module path: `github.com/aaltw/claude-monitor`

## Dependencies

- `charmbracelet/bubbletea` — TUI framework
- `charmbracelet/lipgloss` — Terminal styling/layout
- `lucasb-eyer/go-colorful` — Color interpolation for gradients
- `nhooyr.io/websocket` — WebSocket server
- Chart.js (CDN) — Frontend charts
