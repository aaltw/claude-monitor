package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aaltw/claude-monitor/internal/config"
	"github.com/aaltw/claude-monitor/internal/proxy"
	"github.com/aaltw/claude-monitor/internal/tui"
	"github.com/aaltw/claude-monitor/internal/web"
	webfs "github.com/aaltw/claude-monitor/web"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "web":
			runWeb(os.Args[2:])
			return
		case "proxy":
			runProxy(os.Args[2:])
			return
		}
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

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	srv := web.NewServer(addr, *dev, staticFS)
	if err := srv.Run(); err != nil {
		log.Fatalf("web server: %v", err)
	}
}

func runProxy(args []string) {
	fs := flag.NewFlagSet("proxy", flag.ExitOnError)
	port := fs.Int("p", 8080, "proxy listen port")
	webAddr := fs.String("web", "http://127.0.0.1:3000", "web server address for pushing context data")
	target := fs.String("target", "https://api.anthropic.com", "upstream API target")
	fs.Parse(args)

	p := proxy.NewProxy(*target, *webAddr)
	addr := fmt.Sprintf("127.0.0.1:%d", *port)

	log.Printf("claude-monitor proxy: %s -> %s (push to %s)", addr, *target, *webAddr)
	if err := http.ListenAndServe(addr, p.Handler()); err != nil {
		log.Fatalf("proxy: %v", err)
	}
}
