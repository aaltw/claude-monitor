# Design Spec: claude-monitor v1.0

## 1. Problem Statement

Running multiple Claude Code sessions simultaneously on a Max plan leads to two recurring problems:

1. **Usage blindness**: The 5-hour and 7-day rolling window limits evaporate without warning, especially with Dispatch threads, chat sessions, and multiple CLI sessions competing for quota. There is no persistent, ambient display of current usage or burn rate projection.

2. **Session awareness**: With multiple named sessions across tmux panes, it's easy to lose track of which sessions need input, which are blocked on permission prompts, and which are actively working. Missed permission prompts silently stall entire workflows.

## 2. Solution

A persistent Go TUI application (`claude-monitor`) that runs in a dedicated Ghostty/tmux pane, providing real-time ambient awareness of Claude Code usage limits and session status.

The visual language follows the **"Editorial Terminalism"** design system: a borderless, tonal-layered interface using 24-bit true color with the Catppuccin Mocha palette on a deep nocturnal foundation.

## 3. Design System: Terminal Adaptation

The "Editorial Terminalism" design system is built for web/CSS. The following table maps each principle to terminal constraints:

| Design System Concept | Terminal Adaptation |
|---|---|
| **No-border rule** | No box-drawing characters. Sections separated by background color shifts (`lipgloss.NewStyle().Background()`) and vertical whitespace. |
| **Surface hierarchy** | Lipgloss background tiers: `#0d0d1c` (base), `#18182a` (container), `#2a2a42` (bright/elevated). |
| **Glassmorphism** | Not achievable in terminal. Elevated sections use `surface-bright` background with generous padding instead. |
| **Space Grotesk / Editorial typography** | Terminal uses the configured monospace font. Editorial feel preserved through: ALL_CAPS labels with spaced letter tracking (literal spaces between characters), bold rendering for emphasis, and size hierarchy via whitespace allocation. |
| **Asymmetric 2/3 + 1/3 layout** | `lipgloss.Place()` / `lipgloss.JoinHorizontal()` with explicit column widths. |
| **Display LG numbers** | Bold + bright color single-line text for burn rate values. No figlet rendering. |
| **Status Glow chips** | Colored Unicode dot (`●`) with label text. Pulsing effect via bubbletea tick-based brightness alternation. |
| **Ghost borders** | If separation is ever needed beyond tonal shifts, use `#474658` at effective low contrast (dim attribute). |
| **Shadows / depth** | Simulated by placing lighter-background blocks on darker backgrounds. The contrast difference implies elevation. |

## 4. Color Palette (24-bit)

All colors are 24-bit hex values rendered via lipgloss true color support.

### Surfaces

| Token | Hex | Usage |
|---|---|---|
| `surface` | `#0d0d1c` | Base background, the infinite canvas |
| `surface-container` | `#18182a` | Section backgrounds (usage panel, session table area) |
| `surface-bright` | `#2a2a42` | Elevated panels (burn rate analysis), interactive elements |

### Content

| Token | Hex | Usage |
|---|---|---|
| `on-surface` | `#e6e3fa` | Primary text |
| `on-surface-variant` | `#c9c5dd` | Secondary text, labels, metadata |
| `outline-variant` | `#474658` | Ghost borders (if absolutely needed, low opacity simulation) |

### Accents

| Token | Hex | Usage |
|---|---|---|
| `primary` | `#d1abfd` | Accent, headers, section titles |
| `secondary` | `#8cb7fe` | Working status, focus indicators |
| `green` | `#a6e3a1` | Low usage (healthy), success |
| `yellow` | `#f9e2af` | Medium usage |
| `peach` | `#fab387` | Warnings, amber/waiting status |
| `red` | `#f38ba8` | Critical usage, blocked status, errors |

### Progress Bar Gradient

The progress bars use smooth 24-bit color interpolation across these thresholds:

- 0-50%: `#a6e3a1` (green)
- 50-70%: `#f9e2af` (yellow)
- 70-90%: `#fab387` (peach/orange)
- 90-100%: `#f38ba8` (red)

Interpolation between thresholds is linear in RGB space using `go-colorful`.

## 5. Layout

### Requirements

- **Minimum terminal size**: 120 columns wide. If the terminal is smaller, display a centered message: `"TERMINAL TOO NARROW -- REQUIRES 120+ COLUMNS"` and do not render the dashboard.
- **Minimum rows**: 30. Same behavior if too short.

### Structure

The layout has three major zones, stacked vertically:

```
+===========================================================================+
|                                                                           |
|   USAGE_STATISTICS (2/3 width)      |   BURN_RATE_ANALYSIS (1/3 width)   |
|   - 5H window bar + label           |   - Rate %/hour                    |
|   - 7D window bar + label           |   - Token velocity                 |
|                                      |   - Time to exhaustion             |
|                                                                           |
+---------------------------------------------------------------------------+
|                                                                           |
|   ACTIVE_PROCESS_MATRIX (full width)                                      |
|   - Session table rows                                                    |
|                                                                           |
+---------------------------------------------------------------------------+
|   Footer: version // timestamp  /  version-tag                            |
+===========================================================================+
```

(The `+---+` borders above are for diagram clarity only. The actual UI uses NO borders.)

### Zone 1: Usage Statistics + Burn Rate (top)

**Left panel (2/3 width)** -- background: `surface-container`

- Header: `COMPUTE METRICS` in `label-md` style (uppercase, `on-surface-variant`, letter-spaced) on the left. Machine identifier on the right (hostname or "NODE: <hostname>").
- Subheader: `USAGE_STATISTICS` in `headline-md` style (bold, `on-surface`, larger visual weight via padding).
- Two usage bars, each consisting of:
  - Label line: `[colored square] 5H WINDOW USAGE` (left) + `RESETS_AT: HH:MM` (right), in `label-md` style.
  - Bar line: Block characters (`█` for filled, `░` for empty) spanning the available width. Color follows the gradient based on current percentage. Percentage + severity label overlaid on the bar center (e.g., `25% LOAD`, `85% CRITICAL`).
  - Severity labels: `NOMINAL` (0-50%), `ELEVATED` (50-70%), `HIGH` (70-90%), `CRITICAL` (90-100%).
- Vertical spacing between bars: 1 blank line.

**Right panel (1/3 width)** -- background: `surface-bright` (elevated)

- Header: flame icon + `BURN_RATE_ANALYSIS` in `label-md` style, `primary` color.
- Three data rows, vertically stacked with spacing:
  1. `RATE_P_HOUR` label (`on-surface-variant`, uppercase) + value in bold `on-surface` (e.g., `4.2%`).
  2. `TOKEN_VELOCITY` label + value in bold `green` (e.g., `12k t/h`).
  3. `TIME_TO_EXHAUSTION` block: Nested on `surface-container` background within the elevated panel. Shows `LIMIT IN ~2H 15M` in bold, or `SAFE` if burn rate projects no exhaustion within the window. **Pulses** (alternates brightness) when under 30 minutes remaining.

### Zone 2: Active Process Matrix (middle, full width)

Background: `surface-container`

- Header line: hamburger icon + `ACTIVE_PROCESS_MATRIX` (left, `primary` color, uppercase) + `TOTAL: NN` and `LOAD: NOMINAL` status chips (right).
- Column headers: `SESSION_ID`, `TASK_KERNEL`, `LATENCY`, `STATUS_BIT` -- in `label-md` style, `on-surface-variant`, uppercase.
- **No divider lines between rows.** Rows are separated by 1 blank line of vertical spacing.
- Each row:
  - `SESSION_ID`: Short hex derived from PID (e.g., PID 44946 -> `0xAF92`). Color: `primary`.
  - `TASK_KERNEL`: Session `name` if set, otherwise last path component of `cwd`. Color: `on-surface`.
  - `LATENCY`: Time since last bridge file update (e.g., `12ms`, `342ms`, `2s`, `INF`). Color: `on-surface-variant`. Shows `INF` if no bridge data exists.
  - `STATUS_BIT`: Colored status chip:
    - `WORKING`: `secondary` (`#8cb7fe`) dot + text. Spinning icon animation (`⟳` cycling frames).
    - `WAITING`: `peach` (`#fab387`) dot + text. Pulsing brightness (1s cycle).
    - `BLOCKED`: `red` (`#f38ba8`) dot + text. Pulsing brightness (1s cycle, faster).
    - `ZOMBIE_STATE`: `on-surface-variant` at reduced brightness. No animation. Shown for sessions whose PID is dead.
- Load indicator (header right): `NOMINAL` (all working/idle), `ATTENTION` (any waiting), `CRITICAL` (any blocked).

### Zone 3: Footer (bottom, single line)

Background: `surface` (base, blends with terminal)

- Left: `CLAUDE_MONITOR_V1.0.0 // <ISO timestamp>` in `label-md` style, `on-surface-variant`.
- Center: `V1.0-STABLE` version tag.
- Right: `UPTIME: <duration>` since monitor started.

## 6. Data Architecture

### 6.1 Bridge Files (Usage Data)

**Source**: The existing statusline Go binary (`~/.claude/statusline-bin`) already receives the full StatusLine JSON from Claude Code on every assistant turn. It currently writes `/tmp/claude-ctx-{sessionId}.json`.

**Change**: Extend it to also write `/tmp/claude-monitor-{sessionId}.json` with the following schema:

```json
{
  "session_id": "d2a364d3-0ff5-4275-bd44-e6d3f73fb0ee",
  "timestamp": 1774981063,
  "rate_limits": {
    "five_hour": {
      "used_percentage": 25.0,
      "resets_at": 1774999200
    },
    "seven_day": {
      "used_percentage": 85.0,
      "resets_at": 1775500800
    }
  },
  "tokens": {
    "input": 12345,
    "output": 6789,
    "cache_read": 1000,
    "cache_creation": 500,
    "total_input": 50000,
    "total_output": 25000
  },
  "model": "Claude Sonnet 4",
  "cwd": "/Users/aaltwesthuis/Sources/project"
}
```

**Note**: `rate_limits` fields are only present for Claude.ai subscriber sessions (Max plan). API-key sessions will not have them. The TUI must handle missing rate limit data gracefully (show `NO DATA` instead of bars).

### 6.2 Session State Files

**Source**: The existing `~/.claude/hooks/claude-tmux-status.sh` hook fires on `PreToolUse`, `Stop`, and `Notification` events.

**Change**: Extend it to also write `/tmp/claude-session-state-{pid}.json`:

```json
{
  "pid": 12345,
  "session_id": "d2a364d3-0ff5-4275-bd44-e6d3f73fb0ee",
  "state": "working",
  "updated_at": 1774981063,
  "event": "pretooluse"
}
```

State mapping:
- `pretooluse` event -> state `"working"`
- `stop` event -> state `"idle"`
- `notification` event -> state `"waiting"` (general notification) or `"blocked"` (if notification text contains permission-related keywords like "permission", "allow", "deny", "approve")

### 6.3 Session Registry

**Source**: `~/.claude/sessions/*.json` (existing, read-only).

Provides: `pid`, `sessionId`, `cwd`, `name`, `startedAt`, `kind`, `entrypoint`.

The TUI reads these to discover which sessions exist, then correlates with bridge files (by sessionId) and state files (by pid).

### 6.4 Burn Rate Ring Buffer

The TUI maintains an in-memory ring buffer of `(timestamp, five_hour_percentage, seven_day_percentage, total_tokens)` observations.

- **Buffer size**: 360 entries (covers 30 minutes at 5-second intervals, or 6 hours at 1-minute intervals).
- **Burn rate calculation**: Linear regression over the last N minutes of observations.
  - `rate_per_hour = slope * 3600` where slope is `delta_percentage / delta_seconds`.
  - Use a configurable window (default: last 15 minutes of observations) for the regression.
- **Time-to-exhaustion**: `(100 - current_percentage) / rate_per_hour` hours. Display as `~Xh Ym`. If rate is zero or negative, display `SAFE`.
- **Token velocity**: `delta_total_tokens / delta_seconds * 3600` over the same window.

### 6.5 Polling Intervals

| Data source | Poll interval | Method |
|---|---|---|
| Bridge files (`/tmp/claude-monitor-*.json`) | 2 seconds | `os.ReadDir` + `os.ReadFile` |
| Session state files (`/tmp/claude-session-state-*.json`) | 1 second | `os.ReadDir` + `os.ReadFile` |
| Session registry (`~/.claude/sessions/*.json`) | 5 seconds | `os.ReadDir` + `os.ReadFile` |
| PID liveness check (`kill -0 pid`) | 10 seconds | `syscall.Kill(pid, 0)` |
| Animation ticks (pulsing, spinning) | 200 milliseconds | Bubbletea tick |

File watching (`fsnotify`) is an option but polling is simpler, more reliable across macOS edge cases, and the intervals are infrequent enough that the overhead is negligible.

## 7. Animations

### Pulsing Effect

For WAITING and BLOCKED status indicators, alternate between two brightness levels of the status color on a 1-second cycle (5 ticks at 200ms).

- **WAITING (peach)**: Alternate between `#fab387` (bright) and `#7d5944` (dim, ~50% brightness).
- **BLOCKED (red)**: Alternate between `#f38ba8` (bright) and `#7a4654` (dim), faster cycle (600ms).

### Spinning Effect

For WORKING status, cycle through spinner frames: `⠋`, `⠙`, `⠹`, `⠸`, `⠼`, `⠴`, `⠦`, `⠧`, `⠇`, `⠏` (braille dot spinner). One frame per 200ms tick.

### Time-to-Exhaustion Pulse

When time-to-exhaustion is under 30 minutes, the entire TIME_TO_EXHAUSTION block alternates between `red` text and `peach` text on a 1-second cycle.

### Stale Data Indicator

If no bridge file has been updated in >5 minutes (across all sessions), display `[STALE]` in `peach` next to `COMPUTE METRICS`, pulsing.

## 8. Session Lifecycle

```
Session appears in ~/.claude/sessions/*.json
    │
    ├── PID alive? ──── No ───> ZOMBIE_STATE (dimmed row, no status chip)
    │       │
    │      Yes
    │       │
    │       ├── State file exists? ── No ──> Show row, STATUS_BIT = "IDLE" (dim)
    │       │        │
    │       │       Yes
    │       │        │
    │       │        ├── state = "working" ──> WORKING (blue, spinner)
    │       │        ├── state = "idle" ────── IDLE (dim, no animation)
    │       │        ├── state = "waiting" ──> WAITING (amber, pulse)
    │       │        └── state = "blocked" ──> BLOCKED (red, pulse)
    │       │
    │       └── Bridge file exists? ── Yes ──> Show LATENCY (time since last update)
    │                                  No ───> LATENCY = "INF"
    │
    └── Session file removed (or PID dies) ──> Remove after 30s grace period
```

Zombie sessions are cleaned from the display after 60 seconds. The grace period prevents flickering during session restarts.

## 9. Project Structure

```
claude-monitor/
  cmd/
    monitor/
      main.go               # Entry point, arg parsing, bubbletea program init
  internal/
    tui/
      model.go              # Root bubbletea model: Init, Update, View
      styles.go             # All lipgloss styles, color constants, style constructors
      keymap.go             # Key bindings (q/ctrl-c to quit, etc.)
      components/
        usage.go            # Usage bars section rendering
        burnrate.go         # Burn rate panel rendering
        sessions.go         # Session table rendering
        footer.go           # Footer bar rendering
    data/
      bridge.go             # Read and parse /tmp/claude-monitor-*.json files
      sessions.go           # Read ~/.claude/sessions/*.json + state files, PID checks
      ringbuffer.go         # Time-series ring buffer for burn rate calculation
      types.go              # Shared data types (BridgeData, SessionState, etc.)
    config/
      config.go             # Paths, polling intervals, thresholds, version string
  go.mod
  go.sum
```

### Dependencies

| Package | Purpose |
|---|---|
| `github.com/charmbracelet/bubbletea` | TUI framework (model-update-view) |
| `github.com/charmbracelet/lipgloss` | Styling, layout, 24-bit color |
| `github.com/lucasb-eyer/go-colorful` | Color interpolation for gradient progress bars |

No other external dependencies. File I/O and polling use stdlib only.

## 10. Changes to Existing Infrastructure

### 10.1 Statusline Binary (`~/.claude/statusline-src/main.go`)

**Scope**: ~30 lines of Go code.

1. Add `RateLimits` struct to `StatusInput`:
   ```
   RateLimits struct {
       FiveHour struct {
           UsedPercentage float64 `json:"used_percentage"`
           ResetsAt       float64 `json:"resets_at"`
       } `json:"five_hour"`
       SevenDay struct {
           UsedPercentage float64 `json:"used_percentage"`
           ResetsAt       float64 `json:"resets_at"`
       } `json:"seven_day"`
   } `json:"rate_limits"`
   ```

2. Add `writeMonitorBridge(input StatusInput)` function that writes `/tmp/claude-monitor-{sessionId}.json` with rate limits, token data, model, and cwd.

3. Call `writeMonitorBridge()` at the end of `main()`, alongside the existing `writeContextBridge()`.

### 10.2 Tmux Status Hook (`~/.claude/hooks/claude-tmux-status.sh`)

**Scope**: ~15 lines of bash.

Add a function that writes `/tmp/claude-session-state-{pid}.json` based on the hook event type. Called from each existing event handler (`pretooluse`, `stop`, `notification`).

The `notification` handler inspects the notification JSON (passed via stdin). Claude Code notifications include a `message` field. If the message contains any of the substrings `"permission"`, `"allow"`, `"approve"`, `"deny"`, or `"trust"`, the state is set to `"blocked"`. Otherwise, the state is set to `"waiting"`. The `stop` event (which fires when Claude Code finishes a turn and waits for user input) sets state to `"idle"`.

### 10.3 No Changes Required

- `~/.claude/settings.json` -- no changes needed (statusline command path unchanged).
- `~/.claude/sessions/*.json` -- read-only, no modifications.
- `~/.claude/hooks/context-monitor.sh` -- independent, no changes.

## 11. Key Bindings

| Key | Action |
|---|---|
| `q` / `Ctrl+C` | Quit |
| `r` | Force refresh all data sources |
| `s` | Cycle sort order for session table (by name, by status, by latency) |

Minimal interaction -- this is an ambient display, not an interactive tool.

## 12. Edge Cases

| Scenario | Behavior |
|---|---|
| No sessions running | Show empty state: `NO ACTIVE SESSIONS` in center of process matrix |
| No bridge data (fresh start, no sessions have emitted statusline data yet) | Usage bars show `NO DATA` label, burn rate shows `--` |
| Rate limits not available (API key session, not Max plan) | Usage bars show `NOT AVAILABLE (API SESSION)` |
| Bridge file from a dead session | Ignored after PID liveness check fails. Data still used for rate limits (most recent wins). |
| Multiple sessions with rate limit data | Use the **most recently updated** bridge file's rate limit data (it's global, not per-session). |
| Terminal resized below 120 columns | Clear screen, show centered warning message. Resume normal rendering when resized back. |
| `/tmp` files cleaned by OS | Graceful degradation -- show stale indicator, empty states. No crashes. |
| Rapid session start/stop | 30-second grace period before removing zombie sessions from display. |

## 13. Future Considerations (Out of Scope for v1.0)

- Historical usage graphs (sparklines over time)
- Sound/notification alerts when usage exceeds thresholds
- Integration with the physical LED matrix (shared data source)
- Configuration file for custom thresholds, colors, polling intervals
- Multi-machine monitoring (SSH tunnel / network bridge files)

## 14. Success Criteria

1. The TUI launches, detects terminal size, and renders the full layout without visual artifacts.
2. Usage bars update within 5 seconds of a Claude Code session emitting statusline data.
3. Session status changes (working/waiting/blocked) are reflected within 2 seconds.
4. Burn rate calculation stabilizes after ~2 minutes of observations and provides reasonable time-to-exhaustion estimates.
5. Animations (pulsing, spinning) render smoothly at 5fps (200ms ticks).
6. The application uses negligible CPU when idle (polling, not busy-waiting).
7. No crashes or panics on missing files, dead PIDs, malformed JSON, or terminal resize.
