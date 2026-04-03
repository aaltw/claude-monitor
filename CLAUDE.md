# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Go TUI dashboard for monitoring Claude Code API usage and session status. Built with bubbletea (Elm architecture) + lipgloss. Reads bridge files and session state from `/tmp/` and `~/.claude/sessions/` to display real-time usage bars, burn rate calculations, and active session status.

## Commands

```bash
go build -o claude-monitor ./cmd/monitor/   # Build
./claude-monitor                             # Run
go test ./...                                # Run all tests
go test ./internal/data/ -run TestCalcBurn   # Run single test
go vet ./...                                 # Lint
```

## Architecture

```
cmd/monitor/main.go          Entry point, initializes bubbletea program
internal/
  config/config.go            Constants: polling intervals, thresholds, paths
  data/                       Data layer (no TUI dependencies)
    types.go                  Shared structs: BridgeData, SessionInfo, UsageSnapshot, BurnRate
    bridge.go                 Reads /tmp/claude-monitor-*.json, extracts usage snapshots
    sessions.go               Reads ~/.claude/sessions/*.json, PID liveness checks, merges state
    ringbuffer.go             Circular buffer with linear regression for burn rate calculation
  tui/
    model.go                  Root bubbletea Model with 5 concurrent pollers + animation tick
    keymap.go                 Key bindings (q=quit, r=refresh, s=sort)
    theme/theme.go            Catppuccin Mocha palette, gradient interpolation, lipgloss styles
    components/               Stateless rendering functions
      usage.go                Usage bars with gradient fill (left 2/3)
      burnrate.go             Burn rate panel with time-to-exhaustion (right 1/3)
      sessions.go             Session table with status chips and sorting
      footer.go               Version, timestamp, uptime
```

**Data flow**: Bridge files -> `ReadBridgeFiles()` -> `LatestUsage()` -> `RingBuffer` -> `CalcBurnRate()` -> TUI render

**Data sources**:
- `/tmp/claude-monitor-{sessionId}.json` - written by statusline binary (usage/rate limits)
- `/tmp/claude-session-state-{pid}.json` - written by tmux hooks (session state)
- `~/.claude/sessions/{pid}.json` - persistent session registry

## Key Design Decisions

- All configuration is hardcoded constants in `config.go` (zero-setup design)
- Components are pure rendering functions, not bubbletea sub-models
- Ring buffer uses linear regression over 15-minute window for burn rate; 360-slot capacity
- Zombie sessions get 30s grace period before marking, 60s before cleanup
- Data older than 5 minutes is marked stale
- Theme uses "Editorial Terminalism" (Catppuccin Mocha) with gradient thresholds at 50/70/90%

## Dependencies

- `charmbracelet/bubbletea` - TUI framework
- `charmbracelet/lipgloss` - Terminal styling/layout
- `lucasb-eyer/go-colorful` - Color interpolation for gradients
