package main

import (
	"fmt"
	"os"

	"github.com/aaltwesthuis/claude-monitor/internal/config"
	"github.com/aaltwesthuis/claude-monitor/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(
		tui.NewModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "claude-monitor v%s: %v\n", config.Version, err)
		os.Exit(1)
	}
}
