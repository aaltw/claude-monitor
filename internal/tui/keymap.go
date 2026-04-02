package tui

import "github.com/charmbracelet/bubbletea"

func handleKey(msg tea.KeyMsg, m *Model) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return *m, tea.Quit
	case "r":
		return *m, m.forceRefresh()
	case "s":
		m.sortOrder = (m.sortOrder + 1) % 3
		return *m, nil
	}
	return *m, nil
}
