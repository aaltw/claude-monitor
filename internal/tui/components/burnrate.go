package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/aaltwesthuis/claude-monitor/internal/data"
	"github.com/aaltwesthuis/claude-monitor/internal/tui"
)

// RenderBurnRatePanel renders the right 1/3 burn rate analysis panel.
func RenderBurnRatePanel(rate data.BurnRate, width, height int, animTick int) string {
	bg := tui.SurfaceBright.Width(width).Height(height)

	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Foreground(tui.ColPrimary()).
		Render("🔥 " + tui.LetterSpace("BURN_RATE_ANALYSIS"))
	b.WriteString(header + "\n\n")

	if !rate.HasData {
		b.WriteString(tui.LabelMD.Render(tui.LetterSpace("RATE_P_HOUR")) + "\n")
		b.WriteString(tui.ValueBold.Render("--") + "\n\n")
		b.WriteString(tui.LabelMD.Render(tui.LetterSpace("TOKEN_VELOCITY")) + "\n")
		b.WriteString(tui.ValueBold.Render("--") + "\n\n")
		b.WriteString(tui.LabelMD.Render(tui.LetterSpace("TIME_TO_EXHAUSTION")) + "\n")
		b.WriteString(tui.ValueBold.Render("--") + "\n")

		return bg.Padding(1, 2).Render(b.String())
	}

	// Rate per hour
	b.WriteString(tui.LabelMD.Render(tui.LetterSpace("RATE_P_HOUR")) + "\n")
	rateStr := fmt.Sprintf("%.1f%%", rate.PercentPerHour)
	rateCol := rateColor(rate.PercentPerHour)
	b.WriteString(lipgloss.NewStyle().Foreground(rateCol).Bold(true).Render(rateStr) + "\n\n")

	// Token velocity
	b.WriteString(tui.LabelMD.Render(tui.LetterSpace("TOKEN_VELOCITY")) + "\n")
	tokenStr := formatTokenVelocity(rate.TokensPerHour) + "  t/h"
	b.WriteString(tui.ValueGreen.Render(tokenStr) + "\n\n")

	// Time to exhaustion
	tteLabel := tui.LabelMD.Render(tui.LetterSpace("TIME_TO_EXHAUSTION"))
	b.WriteString(tteLabel + "\n")

	tteBlock := renderTTEBlock(rate, width-6, animTick)
	b.WriteString(tteBlock + "\n")

	return bg.Padding(1, 2).Render(b.String())
}

func renderTTEBlock(rate data.BurnRate, width, animTick int) string {
	innerBg := lipgloss.NewStyle().
		Background(tui.ColSurfaceContainer()).
		Padding(0, 1)

	var text string
	var style lipgloss.Style

	if rate.TimeToExhaust <= 0 {
		text = "SAFE"
		style = lipgloss.NewStyle().Foreground(tui.ColGreen()).Bold(true)
	} else {
		text = "LIMIT IN " + formatDuration(rate.TimeToExhaust)
		// Pulse when under 30 minutes
		if rate.TimeToExhaust < 30*time.Minute {
			if animTick%10 < 5 {
				style = lipgloss.NewStyle().Foreground(tui.ColRed()).Bold(true)
			} else {
				style = lipgloss.NewStyle().Foreground(tui.ColPeach()).Bold(true)
			}
		} else {
			style = lipgloss.NewStyle().Foreground(tui.ColOnSurface()).Bold(true)
		}
	}

	return innerBg.Render(style.Render(text))
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "~0M"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("~%dH %02dM", hours, minutes)
	}
	return fmt.Sprintf("~%dM", minutes)
}

func formatTokenVelocity(tokPerHour float64) string {
	if tokPerHour >= 1_000_000 {
		return fmt.Sprintf("%.1fM", tokPerHour/1_000_000)
	}
	if tokPerHour >= 1_000 {
		return fmt.Sprintf("%.0fk", tokPerHour/1_000)
	}
	return fmt.Sprintf("%.0f", tokPerHour)
}

func rateColor(pctPerHour float64) lipgloss.Color {
	switch {
	case pctPerHour >= 20:
		return tui.ColRed()
	case pctPerHour >= 10:
		return tui.ColPeach()
	case pctPerHour >= 5:
		return tui.ColYellow()
	default:
		return tui.ColOnSurface()
	}
}
