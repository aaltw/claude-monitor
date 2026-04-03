# Claude Monitor Web Dashboard

## Context

The TUI dashboard works well for ambient terminal monitoring, but has limitations: no charts, no historical data, no clickable interactions. A web dashboard serves the same data with richer visualization, accessible from any browser tab. Both modes coexist — `./claude-monitor` for TUI, `./claude-monitor web` for HTTP.

## Architecture

```
Bridge Files (/tmp/)          Session Files (~/.claude/sessions/)
  claude-monitor-*.json         *.json
  claude-session-state-*.json
        │                            │
        └──────────┬─────────────────┘
                   ▼
        internal/data/ (shared, unchanged)
               │
       ┌───────┴───────┐
       ▼               ▼
   internal/tui/   internal/web/
   (unchanged)     server.go   HTTP + WebSocket
                   hub.go      client broadcast
                   poller.go   data polling loop
                   history.go  ring buffer for charts
                       │
                  WebSocket /ws
                       │
                       ▼
                   web/ (static, embedded via go:embed)
                   index.html
                   app.js
                   style.css
```

### Entry point

`cmd/monitor/main.go` gains a `web` subcommand:

```
./claude-monitor           # TUI (default, unchanged)
./claude-monitor web       # HTTP server on :3000
./claude-monitor web -p 8080  # custom port
./claude-monitor web --dev    # serve from disk, not embed (hot reload)
```

### Static files

Separate files in `web/` directory, embedded in the binary via `go:embed`. In `--dev` mode, served from disk for hot reload during development.

- `web/index.html` — single-page dashboard (Tailwind CDN, Space Grotesk + JetBrains Mono from Google Fonts)
- `web/app.js` — WebSocket client, DOM updates, Chart.js integration
- `web/style.css` — Catppuccin Mocha theme, bento grid layout, animations

## WebSocket Protocol

Single WebSocket endpoint at `/ws`. Server pushes; client is read-only.

### Message types (server → client)

**`state`** — Full dashboard state, pushed on connect and every 2 seconds:

```json
{
  "type": "state",
  "usage": {
    "has_data": true,
    "is_stale": false,
    "five_hour": { "used_pct": 45.2, "resets_at": "2026-04-03T21:00:00Z", "severity": "Nominal" },
    "seven_day": { "used_pct": 12.1, "resets_at": "2026-04-04T11:00:00Z", "severity": "Nominal" },
    "total_tokens": 538785
  },
  "burn_rate": {
    "has_data": true,
    "pct_per_hour": 4.2,
    "tokens_per_hour": 12000,
    "tte_minutes": 135
  },
  "sessions": [
    {
      "pid": 86792,
      "hex_id": "0x5308",
      "name": "claude-monitor",
      "model": "Opus 4.6",
      "status": "working",
      "latency": "2s",
      "cwd": "/Users/aaltwesthuis/Sources/playground/claude-monitor"
    }
  ],
  "models": {
    "Opus 4.6": { "total_tokens": 487000, "pct": 72.1 },
    "Sonnet 4.6": { "total_tokens": 189000, "pct": 27.9 }
  }
}
```

**`history`** — Chart data point, pushed every 5 seconds:

```json
{
  "type": "history",
  "timestamp": "2026-04-03T19:14:22Z",
  "five_hour_pct": 45.2,
  "seven_day_pct": 12.1,
  "burn_rate_pct_per_hour": 4.2,
  "total_tokens": 538900,
  "tokens_by_model": { "Opus 4.6": 487200, "Sonnet 4.6": 189100 }
}
```

**`event`** — Session state change, pushed in real-time:

```json
{
  "type": "event",
  "timestamp": "2026-04-03T19:14:22Z",
  "pid": 86792,
  "session": "claude-monitor",
  "model": "Opus 4.6",
  "action": "state_change",
  "detail": "idle → working"
}
```

### Client → server

**`ping`** — Keepalive, server responds with `pong`.

## Go Server (`internal/web/`)

### `server.go`

- `http.ServeMux` with routes: `/` (static files), `/ws` (WebSocket upgrade)
- WebSocket via `gorilla/websocket` (or `nhooyr.io/websocket` for stdlib-friendliness)
- `--dev` flag: serve `web/` from disk; default: `http.FS(embeddedFS)`

### `hub.go`

- Central broadcast hub managing connected WebSocket clients
- `Register`/`Unregister` channels for client lifecycle
- `Broadcast` channel for pushing messages to all clients

### `poller.go`

- Single goroutine polling data layer on same intervals as TUI
- Reuses `data.ReadBridgeFiles`, `data.LatestUsage`, `data.MergeSessions`, `data.RingBuffer`
- Computes per-model token breakdown by grouping bridge files by `model` field
- Tracks previous session states to detect state changes → emits `event` messages
- Pushes `state` every 2s, `history` every 5s, `event` on change

### `history.go`

- In-memory ring buffer of 720 history points (1 hour at 5s intervals)
- Sent to new clients on connect so charts populate immediately

## Web Frontend (`web/`)

### Dashboard layout

Bento grid (CSS Grid, 12-column), matching the existing HTML mockup:

| Panel | Grid position | Content |
|-------|--------------|---------|
| Usage bars | col 1-8, row 1 | 5h + 7d progress bars with gradient fill, per-model token cards |
| Burn rate | col 9-12, row 1 | Rate/hour, velocity, TTE with severity colors |
| Burn rate chart | col 1-6, row 2 | Chart.js line chart, 60-min window, color by severity |
| Token timeline | col 7-12, row 2 | Chart.js stacked area: input, output, cache |
| Session table | col 1-12, row 3 | Clickable rows with model badge, status chips |
| Event log | col 1-12, row 4 | Live scrolling state changes |

### Charts

- **Library:** Chart.js via CDN (`<script src="https://cdn.jsdelivr.net/npm/chart.js">`)
- **Burn rate chart:** Line chart, Y-axis = %/hour, colored segments by severity threshold
- **Token timeline:** Stacked area, three datasets (input, output, cache), 60-min rolling window
- Both charts update in real-time from `history` WebSocket messages
- On connect, receive backfill of up to 720 history points

### Session table

- Model column with colored badge (Opus = mauve, Sonnet = blue)
- Session ID is a clickable link that opens a terminal link (OSC 8) to focus the tmux pane
- Status chips with background tint and icon matching TUI status styling

### Per-model breakdown

- Cards below usage bars showing token count and percentage per model
- Thin proportional bar under each card
- Updates in real-time from `state.models`

### Event log

- Scrolling list of session state changes, newest at top
- Each entry: timestamp, hex ID, session name, model tag, state transition
- Max 100 entries retained in DOM, older entries pruned

### Theme

Catppuccin Mocha palette (same as TUI), using:
- Space Grotesk for headings
- JetBrains Mono for data/labels
- Tailwind CSS via CDN for utilities

## Dependencies

New Go dependencies:
- `nhooyr.io/websocket` — WebSocket library (stdlib-friendly, no gorilla dependency)

Frontend (all CDN, no build step):
- Chart.js
- Tailwind CSS
- Google Fonts (Space Grotesk, JetBrains Mono)

## Verification

1. `go build ./cmd/monitor/` compiles successfully
2. `go test ./...` — all existing tests pass, new tests for hub and poller
3. `./claude-monitor web` starts HTTP server, browser shows dashboard
4. WebSocket connects and receives `state` messages every 2s
5. Charts populate with history on connect and update in real-time
6. Session table shows correct model, status, latency
7. Per-model token breakdown updates as sessions generate tokens
8. Event log shows state transitions in real-time
9. `./claude-monitor` TUI still works unchanged
