package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aaltw/claude-monitor/internal/config"
	"github.com/aaltw/claude-monitor/internal/tui"
	"github.com/aaltw/claude-monitor/internal/web"
	webfs "github.com/aaltw/claude-monitor/web"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "web" {
		runWeb(os.Args[2:])
		return
	}
	runTUI()
}

func runTUI() {
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

func runWeb(args []string) {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	port := fs.Int("p", 3000, "HTTP server port")
	dev := fs.Bool("dev", false, "serve static files from disk (hot reload)")
	fs.Parse(args)

	staticFS, err := webfs.FS()
	if err != nil {
		log.Fatalf("embedded static files: %v", err)
	}

	addr := fmt.Sprintf(":%d", *port)
	srv := web.NewServer(addr, *dev, staticFS)
	if err := srv.Run(); err != nil {
		log.Fatalf("web server: %v", err)
	}
}
