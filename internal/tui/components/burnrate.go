package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/aaltw/claude-monitor/internal/data"
	"github.com/aaltw/claude-monitor/internal/tui/theme"
)

// RenderBurnRatePanel renders the right 1/3 burn rate analysis panel.
func RenderBurnRatePanel(rate data.BurnRate, width, height int, animTick int) string {
	bg := theme.SurfaceBright.Width(width).Height(height)

	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Foreground(theme.ColPrimary()).
		Render("Burn Rate Analysis")
	b.WriteString(header + "\n\n")

	if !rate.HasData {
		b.WriteString(theme.LabelMD.Render("Rate/Hour") + "\n")
		b.WriteString(theme.ValueBold.Render("--") + "\n\n")
		b.WriteString(theme.LabelMD.Render("Token Velocity") + "\n")
		b.WriteString(theme.ValueBold.Render("--") + "\n\n")
		b.WriteString(theme.LabelMD.Render("Time to Exhaustion") + "\n")
		b.WriteString(theme.ValueBold.Render("--") + "\n")

		return bg.Padding(1, 2).Render(b.String())
	}

	// Rate per hour
	b.WriteString(theme.LabelMD.Render("Rate/Hour") + "\n")
	rateStr := fmt.Sprintf("%.1f%%", rate.PercentPerHour)
	rateCol := rateColor(rate.PercentPerHour)
	b.WriteString(lipgloss.NewStyle().Foreground(rateCol).Bold(true).Render(rateStr) + "\n\n")

	// Token velocity
	b.WriteString(theme.LabelMD.Render("Token Velocity") + "\n")
	tokenStr := formatTokenVelocity(rate.TokensPerHour) + "  t/h"
	b.WriteString(theme.ValueGreen.Render(tokenStr) + "\n\n")

	// Time to exhaustion
	tteLabel := theme.LabelMD.Render("Time to Exhaustion")
	b.WriteString(tteLabel + "\n")

	tteBlock := renderTTEBlock(rate, width-6, animTick)
	b.WriteString(tteBlock + "\n")

	return bg.Padding(1, 2).Render(b.String())
}

func renderTTEBlock(rate data.BurnRate, width, animTick int) string {
	innerBg := lipgloss.NewStyle().
		Background(theme.ColSurfaceContainer()).
		Padding(0, 1)

	var text string
	var style lipgloss.Style

	if rate.TimeToExhaust <= 0 {
		text = "Safe"
		style = lipgloss.NewStyle().Foreground(theme.ColGreen()).Bold(true)
	} else {
		text = "Limit in " + formatDuration(rate.TimeToExhaust)
		// Pulse when under 30 minutes
		if rate.TimeToExhaust < 30*time.Minute {
			if animTick%10 < 5 {
				style = lipgloss.NewStyle().Foreground(theme.ColRed()).Bold(true)
			} else {
				style = lipgloss.NewStyle().Foreground(theme.ColPeach()).Bold(true)
			}
		} else {
			style = lipgloss.NewStyle().Foreground(theme.ColOnSurface()).Bold(true)
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
		return theme.ColRed()
	case pctPerHour >= 10:
		return theme.ColPeach()
	case pctPerHour >= 5:
		return theme.ColYellow()
	default:
		return theme.ColOnSurface()
	}
}
