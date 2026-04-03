package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ── JSON input ──────────────────────────────────────────────────────────────

type StatusInput struct {
	Model         Model         `json:"model"`
	ContextWindow ContextWindow `json:"context_window"`
	Cost          Cost          `json:"cost"`
	Exceeds200k   bool          `json:"exceeds_200k_tokens"`
	Cwd           string        `json:"cwd"`
	SessionID     string        `json:"session_id"`
	RateLimits    struct {
		FiveHour struct {
			UsedPercentage float64 `json:"used_percentage"`
			ResetsAt       float64 `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay struct {
			UsedPercentage float64 `json:"used_percentage"`
			ResetsAt       float64 `json:"resets_at"`
		} `json:"seven_day"`
	} `json:"rate_limits"`
}

type Model struct {
	DisplayName string `json:"display_name"`
}

type ContextWindow struct {
	UsedPercentage      float64      `json:"used_percentage"`
	RemainingPercentage float64      `json:"remaining_percentage"`
	ContextWindowSize   int          `json:"context_window_size"`
	CurrentUsage        CurrentUsage `json:"current_usage"`
	TotalInputTokens    int          `json:"total_input_tokens"`
	TotalOutputTokens   int          `json:"total_output_tokens"`
}

type CurrentUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

type Cost struct {
	TotalLinesAdded   int `json:"total_lines_added"`
	TotalLinesRemoved int `json:"total_lines_removed"`
}

// ── Colors (Catppuccin Mocha) ───────────────────────────────────────────────

type Color struct{ R, G, B int }

func (c Color) FG() string { return fmt.Sprintf("\033[38;2;%d;%d;%dm", c.R, c.G, c.B) }
func (c Color) BG() string { return fmt.Sprintf("\033[48;2;%d;%d;%dm", c.R, c.G, c.B) }

var (
	base      = Color{30, 30, 46}
	surface0  = Color{49, 50, 68}
	surface1  = Color{69, 71, 90}
	blue      = Color{137, 180, 250}
	green     = Color{166, 227, 161}
	green80   = Color{133, 182, 129}
	greenDim  = Color{83, 113, 80}
	yellow    = Color{249, 226, 175}
	yellow80  = Color{199, 181, 140}
	yellowDim = Color{124, 113, 87}
	red       = Color{243, 139, 168}
	red80     = Color{194, 111, 134}
	redDim    = Color{121, 69, 84}
	pink      = Color{245, 194, 231}
	teal      = Color{148, 226, 213}
)

const (
	sepR  = "\ue0b0" // Powerline right arrow
	sepL  = "\ue0b2" // Powerline left arrow
	reset = "\033[0m"
)

// ── Segments ────────────────────────────────────────────────────────────────

type Segment struct {
	BG       Color
	Text     string
	NoSep    bool   // join without separator
	SepColor *Color // override separator fg color (defaults to BG)
}

func renderLeft(segs []Segment, b *strings.Builder) {
	for i, s := range segs {
		b.WriteString(s.BG.BG())
		b.WriteString(s.Text)
		if i+1 < len(segs) {
			b.WriteString(reset)
			b.WriteString(s.BG.FG())
			b.WriteString(segs[i+1].BG.BG())
			b.WriteString(sepR)
		} else {
			b.WriteString(reset)
			b.WriteString(s.BG.FG())
			b.WriteString(sepR)
			b.WriteString(reset)
		}
	}
}

func renderRight(segs []Segment, b *strings.Builder) {
	for i, s := range segs {
		if i == 0 {
			b.WriteString(reset)
			b.WriteString(s.BG.FG())
			b.WriteString(sepL)
		} else if s.NoSep {
			// continue without separator
		} else {
			sepFG := s.BG
			if s.SepColor != nil {
				sepFG = *s.SepColor
			}
			b.WriteString(sepFG.FG())
			b.WriteString(segs[i-1].BG.BG())
			b.WriteString(sepL)
		}
		b.WriteString(s.BG.BG())
		b.WriteString(s.Text)
	}
	b.WriteString(reset)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func fmtTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%5s", fmt.Sprintf("%.1fk", float64(n)/1000))
	}
	return fmt.Sprintf("%5d", n)
}

var ansiRE = regexp.MustCompile(`\033\[[0-9;]*m`)

func visibleWidth(s string) int {
	stripped := ansiRE.ReplaceAllString(s, "")
	return utf8.RuneCountInString(stripped)
}

func segmentsWidth(segs []Segment) int {
	w := 0
	for _, s := range segs {
		w += visibleWidth(s.Text)
	}
	w += len(segs) // separators
	return w
}

func termWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			return n
		}
	}
	if out, err := exec.Command("tmux", "display-message", "-p", "#{pane_width}").Output(); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && n > 0 {
			return n
		}
	}
	if out, err := exec.Command("tput", "cols").Output(); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && n > 0 {
			return n
		}
	}
	return 80
}

// ── Git ─────────────────────────────────────────────────────────────────────

type GitInfo struct {
	Branch string
	Dirty  string
	Ahead  int
	Behind int
}

const gitCachePath = "/tmp/claude-statusline-git-cache"

func getGitInfo(cwd string) GitInfo {
	// Check cache
	if info, err := os.Stat(gitCachePath); err == nil {
		if time.Since(info.ModTime()) < 5*time.Second {
			if data, err := os.ReadFile(gitCachePath); err == nil {
				lines := strings.Split(string(data), "\n")
				if len(lines) >= 4 {
					ahead, _ := strconv.Atoi(lines[2])
					behind, _ := strconv.Atoi(lines[3])
					return GitInfo{Branch: lines[0], Dirty: lines[1], Ahead: ahead, Behind: behind}
				}
			}
		}
	}

	gi := GitInfo{}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Branch
	if out, err := exec.Command("git", "-C", cwd, "--no-optional-locks", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		gi.Branch = strings.TrimSpace(string(out))
	}
	if gi.Branch == "" {
		writeGitCache(gi)
		return gi
	}

	// Dirty
	err1 := exec.Command("git", "-C", cwd, "--no-optional-locks", "diff", "--quiet").Run()
	err2 := exec.Command("git", "-C", cwd, "--no-optional-locks", "diff", "--cached", "--quiet").Run()
	if err1 != nil || err2 != nil {
		gi.Dirty = " ✦"
	}

	// Ahead/behind
	if out, err := exec.Command("git", "-C", cwd, "--no-optional-locks", "rev-list", "--count", "@{u}..HEAD").Output(); err == nil {
		gi.Ahead, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("git", "-C", cwd, "--no-optional-locks", "rev-list", "--count", "HEAD..@{u}").Output(); err == nil {
		gi.Behind, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}

	writeGitCache(gi)
	return gi
}

func writeGitCache(gi GitInfo) {
	data := fmt.Sprintf("%s\n%s\n%d\n%d\n", gi.Branch, gi.Dirty, gi.Ahead, gi.Behind)
	_ = os.WriteFile(gitCachePath, []byte(data), 0644)
}

func writeMonitorBridge(input StatusInput) {
	if input.SessionID == "" {
		return
	}
	bridgePath := filepath.Join(os.TempDir(), "claude-monitor-"+input.SessionID+".json")
	cacheT := input.ContextWindow.CurrentUsage.CacheReadInputTokens + input.ContextWindow.CurrentUsage.CacheCreationInputTokens
	_ = cacheT
	bridgeData, _ := json.Marshal(map[string]any{
		"session_id": input.SessionID,
		"timestamp":  time.Now().Unix(),
		"rate_limits": map[string]any{
			"five_hour": map[string]any{
				"used_percentage": input.RateLimits.FiveHour.UsedPercentage,
				"resets_at":       input.RateLimits.FiveHour.ResetsAt,
			},
			"seven_day": map[string]any{
				"used_percentage": input.RateLimits.SevenDay.UsedPercentage,
				"resets_at":       input.RateLimits.SevenDay.ResetsAt,
			},
		},
		"tokens": map[string]any{
			"input":          input.ContextWindow.CurrentUsage.InputTokens,
			"output":         input.ContextWindow.CurrentUsage.OutputTokens,
			"cache_read":     input.ContextWindow.CurrentUsage.CacheReadInputTokens,
			"cache_creation": input.ContextWindow.CurrentUsage.CacheCreationInputTokens,
			"total_input":    input.ContextWindow.TotalInputTokens,
			"total_output":   input.ContextWindow.TotalOutputTokens,
		},
		"model": input.Model.DisplayName,
		"cwd":   input.Cwd,
	})
	_ = os.WriteFile(bridgePath, bridgeData, 0644)
}

// ── Main ────────────────────────────────────────────────────────────────────

func main() {
	var input StatusInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintln(os.Stderr, "statusline: invalid JSON input")
		os.Exit(1)
	}

	// ── Model segment ───────────────────────────────────────────────────────
	shortModel := "Claude"
	if input.Model.DisplayName != "" {
		shortModel = strings.TrimPrefix(input.Model.DisplayName, "Claude ")
	}
	modelSeg := Segment{
		BG:   surface1,
		Text: blue.FG() + " 󰧑 " + shortModel + " ",
	}

	// ── Tokens segment ──────────────────────────────────────────────────────
	cacheT := input.ContextWindow.CurrentUsage.CacheReadInputTokens + input.ContextWindow.CurrentUsage.CacheCreationInputTokens
	tokensSeg := Segment{
		BG: surface0,
		Text: green.FG() + " ↑" + fmtTokens(input.ContextWindow.CurrentUsage.InputTokens) + "/" + fmtTokens(input.ContextWindow.TotalInputTokens) + " " +
			yellow.FG() + "↓" + fmtTokens(input.ContextWindow.CurrentUsage.OutputTokens) + "/" + fmtTokens(input.ContextWindow.TotalOutputTokens) + " " +
			teal.FG() + "~" + fmtTokens(cacheT) + " ",
	}

	// ── Context bar ─────────────────────────────────────────────────────────
	pct := int(input.ContextWindow.UsedPercentage)

	var ctxLabel string
	if input.ContextWindow.ContextWindowSize >= 1000000 {
		ctxLabel = fmt.Sprintf("%dM", input.ContextWindow.ContextWindowSize/1000000)
	} else if input.ContextWindow.ContextWindowSize > 0 {
		ctxLabel = fmt.Sprintf("%dk", input.ContextWindow.ContextWindowSize/1000)
	}

	var barColor, barFill, barDim Color
	if pct < 70 {
		barColor, barFill, barDim = green, green80, greenDim
	} else if pct < 90 {
		barColor, barFill, barDim = yellow, yellow80, yellowDim
	} else {
		barColor, barFill, barDim = red, red80, redDim
	}

	sizeSuffix := ""
	if ctxLabel != "" {
		sizeSuffix = "/" + ctxLabel
	}

	var ctxSeg, barSeg Segment
	if pct > 0 {
		filled := pct * 10 / 100
		if filled > 10 {
			filled = 10
		}
		var barStr strings.Builder
		for i := 0; i < filled; i++ {
			barStr.WriteString(barFill.FG() + barDim.BG() + "█")
		}
		for i := 0; i < 10-filled; i++ {
			barStr.WriteString(barDim.FG() + barDim.BG() + "▒")
		}
		ctxSeg = Segment{BG: surface1, Text: barColor.FG() + fmt.Sprintf(" %d%%%s ", pct, sizeSuffix)}
		barSeg = Segment{BG: barDim, Text: barStr.String() + barDim.FG() + "▒", SepColor: &barFill}
	} else {
		var barStr strings.Builder
		for i := 0; i < 10; i++ {
			barStr.WriteString(greenDim.FG() + greenDim.BG() + "▒")
		}
		ctxSeg = Segment{BG: surface1, Text: green.FG() + fmt.Sprintf(" --%% %s ", sizeSuffix)}
		gd := greenDim
		barSeg = Segment{BG: greenDim, Text: barStr.String() + greenDim.FG() + "▒", SepColor: &gd}
	}

	// ── Git segment ─────────────────────────────────────────────────────────
	gi := getGitInfo(input.Cwd)
	var leftSegs []Segment

	if gi.Branch != "" {
		label := gi.Branch + gi.Dirty
		if gi.Ahead > 0 {
			label += fmt.Sprintf(" ↑%d", gi.Ahead)
		}
		if gi.Behind > 0 {
			label += fmt.Sprintf(" ↓%d", gi.Behind)
		}
		leftSegs = append(leftSegs, Segment{BG: surface1, Text: pink.FG() + "  " + label + " "})
	}

	// ── Lines changed segment ───────────────────────────────────────────────
	if input.Cost.TotalLinesAdded > 0 || input.Cost.TotalLinesRemoved > 0 {
		leftSegs = append(leftSegs, Segment{
			BG:   surface0,
			Text: green.FG() + fmt.Sprintf("  +%d ", input.Cost.TotalLinesAdded) + red.FG() + fmt.Sprintf("-%d ", input.Cost.TotalLinesRemoved),
		})
	}

	// ── Compact warning ─────────────────────────────────────────────────────
	if pct >= 85 {
		leftSegs = append(leftSegs, Segment{BG: red, Text: base.FG() + " ⚡ COMPACT "})
	}

	// ── Assemble right segments ─────────────────────────────────────────────
	rightSegs := []Segment{ctxSeg, barSeg, tokensSeg, modelSeg}

	// ── Layout ──────────────────────────────────────────────────────────────
	tw := termWidth() - 6 // nerd font icon margin
	leftW := segmentsWidth(leftSegs)
	rightW := segmentsWidth(rightSegs)

	var out strings.Builder
	out.WriteString(reset) // Clear any ANSI state from Claude Code

	if leftW+rightW+2 <= tw {
		// Single line
		if len(leftSegs) > 0 {
			renderLeft(leftSegs, &out)
		}
		gap := tw - leftW - rightW
		if gap < 1 {
			gap = 1
		}
		out.WriteString(strings.Repeat(" ", gap))
		renderRight(rightSegs, &out)
	} else {
		// Two lines
		if len(leftSegs) > 0 {
			renderLeft(leftSegs, &out)
		}
		out.WriteString("\n")
		gap := tw - rightW
		if gap < 0 {
			gap = 0
		}
		out.WriteString(strings.Repeat(" ", gap))
		renderRight(rightSegs, &out)
	}
	out.WriteString("\n")

	os.Stdout.WriteString(out.String())

	// Write context metrics bridge file for the context-monitor PostToolUse hook.
	// The hook reads this to inject agent-facing warnings when context is low.
	if input.SessionID != "" {
		bridgePath := filepath.Join(os.TempDir(), "claude-ctx-"+input.SessionID+".json")
		bridgeData, _ := json.Marshal(map[string]any{
			"session_id":           input.SessionID,
			"remaining_percentage": input.ContextWindow.RemainingPercentage,
			"used_pct":             pct,
			"timestamp":            time.Now().Unix(),
		})
		_ = os.WriteFile(bridgePath, bridgeData, 0644)
	}

	writeMonitorBridge(input)
}
