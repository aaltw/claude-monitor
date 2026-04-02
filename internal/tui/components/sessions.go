package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/aaltwesthuis/claude-monitor/internal/data"
	"github.com/aaltwesthuis/claude-monitor/internal/tui/theme"
)

// RenderSessionTable renders the full-width active process matrix.
func RenderSessionTable(sessions []data.SessionInfo, width int, sortOrder data.SortOrder, animTick int) string {
	bg := theme.SurfaceContainer.Width(width)

	var b strings.Builder

	// Sort sessions
	sortSessions(sessions, sortOrder)

	// Compute load status
	loadStatus, loadColor := computeLoad(sessions)

	// Header
	icon := theme.PrimaryText.Render("≡")
	title := lipgloss.NewStyle().Foreground(theme.ColPrimary()).Bold(true).Render("  " + theme.LetterSpace("ACTIVE_PROCESS_MATRIX"))
	total := theme.LabelMD.Render(fmt.Sprintf("● TOTAL: %02d", len(sessions)))
	load := lipgloss.NewStyle().Foreground(loadColor).Render(fmt.Sprintf("● LOAD: %s", loadStatus))

	rightHeader := total + "    " + load
	headerLine := lipgloss.JoinHorizontal(lipgloss.Top,
		icon+title,
		strings.Repeat(" ", max(1, width-4-lipgloss.Width(icon+title)-lipgloss.Width(rightHeader))),
		rightHeader,
	)
	b.WriteString(headerLine + "\n\n")

	// Column headers
	colWidths := columnWidths(width - 4)
	headers := []string{"SESSION_ID", "TASK_KERNEL", "LATENCY", "STATUS_BIT"}
	var headerCells []string
	for i, h := range headers {
		cell := theme.LabelMD.Width(colWidths[i]).Render(theme.LetterSpace(h))
		headerCells = append(headerCells, cell)
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, headerCells...) + "\n")

	// Empty state
	if len(sessions) == 0 {
		emptyMsg := theme.LabelMD.Render("N O   A C T I V E   S E S S I O N S")
		b.WriteString("\n" + lipgloss.Place(width-4, 3, lipgloss.Center, lipgloss.Center, emptyMsg) + "\n")
		return bg.Padding(1, 2).Render(b.String())
	}

	// Rows
	for _, sess := range sessions {
		b.WriteString("\n")
		b.WriteString(renderSessionRow(sess, colWidths, animTick))
	}

	return bg.Padding(1, 2).Render(b.String())
}

func renderSessionRow(sess data.SessionInfo, colWidths [4]int, animTick int) string {
	// SESSION_ID
	hexID := data.SessionHexID(sess.PID)
	idStyle := lipgloss.NewStyle().Foreground(theme.ColPrimary()).Width(colWidths[0])
	if sess.Status == data.StatusZombie {
		idStyle = idStyle.Foreground(theme.ColOutlineVariant())
	}
	idCell := idStyle.Render(hexID)

	// TASK_KERNEL
	nameStyle := lipgloss.NewStyle().Foreground(theme.ColOnSurface()).Width(colWidths[1])
	if sess.Status == data.StatusZombie {
		nameStyle = nameStyle.Foreground(theme.ColOutlineVariant()).Italic(true)
	}
	nameCell := nameStyle.Render(sess.Name)

	// LATENCY
	latency := data.FormatLatency(sess.LastBridge)
	latencyStyle := lipgloss.NewStyle().Foreground(theme.ColOnSurfaceVariant()).Width(colWidths[2])
	if sess.Status == data.StatusZombie {
		latencyStyle = latencyStyle.Foreground(theme.ColOutlineVariant())
		latency = "--"
	}
	latencyCell := latencyStyle.Render(latency)

	// STATUS_BIT
	statusCell := renderStatusChip(sess.Status, colWidths[3], animTick)

	return lipgloss.JoinHorizontal(lipgloss.Top, idCell, nameCell, latencyCell, statusCell)
}

func renderStatusChip(status data.SessionStatus, width, animTick int) string {
	style := lipgloss.NewStyle().Width(width)

	switch status {
	case data.StatusWorking:
		frame := theme.SpinnerFrames[animTick%len(theme.SpinnerFrames)]
		return style.Render(theme.StatusWorkingStyle.Render(frame + " WORKING"))

	case data.StatusWaiting:
		if animTick%10 < 5 {
			return style.Render(theme.StatusWaitingStyle.Render("● WAITING"))
		}
		return style.Render(theme.StatusWaitingDimStyle.Render("● WAITING"))

	case data.StatusBlocked:
		// Faster pulse: 3 ticks on, 3 off (600ms cycle at 200ms tick)
		if animTick%6 < 3 {
			return style.Render(theme.StatusBlockedStyle.Render("⚠ BLOCKED"))
		}
		return style.Render(theme.StatusBlockedDimStyle.Render("⚠ BLOCKED"))

	case data.StatusZombie:
		return style.Render(theme.StatusZombieStyle.Render("ZOMBIE_STATE"))

	default: // Idle
		return style.Render(theme.StatusIdleStyle.Render("○ IDLE"))
	}
}

func columnWidths(totalWidth int) [4]int {
	// SESSION_ID: 15, TASK_KERNEL: flexible, LATENCY: 12, STATUS_BIT: 18
	idW := 15
	latencyW := 12
	statusW := 18
	nameW := totalWidth - idW - latencyW - statusW
	if nameW < 20 {
		nameW = 20
	}
	return [4]int{idW, nameW, latencyW, statusW}
}

func computeLoad(sessions []data.SessionInfo) (string, lipgloss.Color) {
	hasBlocked := false
	hasWaiting := false
	for _, s := range sessions {
		if s.Status == data.StatusBlocked {
			hasBlocked = true
		}
		if s.Status == data.StatusWaiting {
			hasWaiting = true
		}
	}
	if hasBlocked {
		return "CRITICAL", theme.ColRed()
	}
	if hasWaiting {
		return "ATTENTION", theme.ColPeach()
	}
	return "NOMINAL", theme.ColGreen()
}

func sortSessions(sessions []data.SessionInfo, order data.SortOrder) {
	sort.Slice(sessions, func(i, j int) bool {
		switch order {
		case data.SortByStatus:
			if sessions[i].Status != sessions[j].Status {
				return sessions[i].Status > sessions[j].Status // blocked first
			}
			return sessions[i].Name < sessions[j].Name
		case data.SortByLatency:
			if sessions[i].LastBridge.IsZero() && !sessions[j].LastBridge.IsZero() {
				return false
			}
			if !sessions[i].LastBridge.IsZero() && sessions[j].LastBridge.IsZero() {
				return true
			}
			return sessions[i].LastBridge.After(sessions[j].LastBridge)
		default: // SortByName
			return sessions[i].Name < sessions[j].Name
		}
	})
}
