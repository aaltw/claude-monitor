package components

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/aaltwesthuis/claude-monitor/internal/data"
	"github.com/aaltwesthuis/claude-monitor/internal/tui/theme"
)

// RenderUsagePanel renders the left 2/3 usage statistics panel.
func RenderUsagePanel(usage data.UsageSnapshot, width int, isStale bool, animTick int) string {
	bg := theme.SurfaceContainer.Width(width)

	if !usage.HasData {
		noData := theme.LabelMD.Render("N O   D A T A")
		content := lipgloss.Place(width-4, 8, lipgloss.Center, lipgloss.Center, noData)
		return bg.Padding(1, 2).Render(content)
	}

	var b strings.Builder

	// Header line
	header := theme.LabelMD.Render(theme.LetterSpace("COMPUTE METRICS"))
	staleIndicator := ""
	if isStale {
		if animTick%10 < 5 {
			staleIndicator = theme.StatusWaitingStyle.Render("  [STALE]")
		} else {
			staleIndicator = theme.StatusWaitingDimStyle.Render("  [STALE]")
		}
	}
	hostname, _ := hostnameShort()
	nodeLabel := theme.LabelMD.Render(fmt.Sprintf("NODE: %s", hostname))

	headerLine := lipgloss.JoinHorizontal(lipgloss.Top,
		header+staleIndicator,
		strings.Repeat(" ", max(1, width-4-lipgloss.Width(header+staleIndicator)-lipgloss.Width(nodeLabel))),
		nodeLabel,
	)
	b.WriteString(headerLine + "\n")

	// Subheader
	b.WriteString(theme.HeadlineMD.Render("USAGE_STATISTICS") + "\n\n")

	// 5H bar
	b.WriteString(renderBar("5H WINDOW USAGE", usage.FiveHour, width-4))
	b.WriteString("\n")

	// 7D bar
	b.WriteString(renderBar("7D WINDOW USAGE", usage.SevenDay, width-4))

	return bg.Padding(1, 2).Render(b.String())
}

func renderBar(label string, w data.WindowUsage, barWidth int) string {
	var b strings.Builder

	// Colored square indicator
	gradColor := theme.GradientColor(w.UsedPercentage)
	square := lipgloss.NewStyle().Foreground(gradColor).Render("■")

	// Label line
	labelText := theme.LabelMD.Render(fmt.Sprintf("%s %s", square, theme.LetterSpace(label)))
	resetTime := theme.LabelMD.Render(fmt.Sprintf("RESETS_AT: %s", w.ResetsAt.Format("15:04")))

	labelLine := lipgloss.JoinHorizontal(lipgloss.Top,
		labelText,
		strings.Repeat(" ", max(1, barWidth-lipgloss.Width(labelText)-lipgloss.Width(resetTime))),
		resetTime,
	)
	b.WriteString(labelLine + "\n")

	// Progress bar
	fillWidth := barWidth - 2 // padding
	if fillWidth < 10 {
		fillWidth = 10
	}
	filled := int(w.UsedPercentage / 100.0 * float64(fillWidth))
	if filled > fillWidth {
		filled = fillWidth
	}
	if filled < 0 {
		filled = 0
	}

	var bar strings.Builder
	for i := 0; i < filled; i++ {
		pctAtPos := float64(i) / float64(fillWidth) * 100.0
		c := theme.GradientColor(pctAtPos)
		bar.WriteString(lipgloss.NewStyle().Foreground(c).Render("█"))
	}
	emptyColor := lipgloss.Color("#1a1a2e")
	for i := filled; i < fillWidth; i++ {
		bar.WriteString(lipgloss.NewStyle().Foreground(emptyColor).Render("░"))
	}

	// Overlay text centered on bar
	overlayText := fmt.Sprintf("%d%% %s", int(w.UsedPercentage), w.Severity)
	barStr := bar.String()

	// Build the bar line with overlay
	barLine := lipgloss.NewStyle().Width(barWidth).Render(barStr)

	// Place overlay text
	overlayStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#e6e3fa")).
		Bold(true)
	overlay := overlayStyle.Render(overlayText)

	// Center the overlay on the bar
	overlayPos := (barWidth - lipgloss.Width(overlay)) / 2
	if overlayPos < 0 {
		overlayPos = 0
	}

	// Rebuild with overlay -- for terminal, we just show bar + label below or inline
	// Since true overlay isn't possible, show percentage at the end
	pctLabel := lipgloss.NewStyle().
		Foreground(gradColor).
		Bold(true).
		Render(fmt.Sprintf(" %d%% %s", int(w.UsedPercentage), w.Severity))

	_ = barLine
	_ = overlay
	_ = overlayPos
	_ = overlayText

	b.WriteString(bar.String() + pctLabel + "\n")

	return b.String()
}

func hostnameShort() (string, error) {
	h, err := os.Hostname()
	if err != nil {
		return "LOCAL", nil
	}
	parts := strings.SplitN(h, ".", 2)
	return strings.ToUpper(parts[0]), nil
}
