package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/aaltwesthuis/claude-monitor/internal/config"
	"github.com/aaltwesthuis/claude-monitor/internal/tui"
)

// RenderFooter renders the bottom status bar.
func RenderFooter(startTime time.Time, width int) string {
	now := time.Now()

	// Left: version + timestamp
	left := tui.LabelMD.Render(
		fmt.Sprintf("CLAUDE_MONITOR_V%s // %s", config.Version, now.Format("2006-01-02T15:04:05Z")),
	)

	// Center: version tag
	center := tui.LabelMD.Render(config.VersionTag)

	// Right: uptime
	uptime := formatUptime(time.Since(startTime))
	right := tui.LabelMD.Render(fmt.Sprintf("UPTIME: %s", uptime))

	// Layout: left --- center --- right
	leftW := lipgloss.Width(left)
	centerW := lipgloss.Width(center)
	rightW := lipgloss.Width(right)

	gapTotal := width - leftW - centerW - rightW
	if gapTotal < 2 {
		gapTotal = 2
	}
	gapLeft := gapTotal / 2
	gapRight := gapTotal - gapLeft

	return lipgloss.NewStyle().
		Background(tui.ColSurfaceContainer()).
		Width(width).
		Render(
			left +
				strings.Repeat(" ", gapLeft) +
				center +
				strings.Repeat(" ", gapRight) +
				right,
		)
}

func formatUptime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
