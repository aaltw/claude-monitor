package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"nhooyr.io/websocket"
)

// Server is the web dashboard HTTP server.
type Server struct {
	hub      *Hub
	poller   *Poller
	history  *HistoryBuffer
	mux      *http.ServeMux
	addr     string
	dev      bool
	staticFS fs.FS
}

// NewServer creates a new web server.
func NewServer(addr string, dev bool, staticFS fs.FS) *Server {
	hub := NewHub()
	history := NewHistoryBuffer(720) // 1 hour at 5s intervals
	poller := NewPoller(hub, history)

	s := &Server{
		hub:      hub,
		poller:   poller,
		history:  history,
		mux:      http.NewServeMux(),
		addr:     addr,
		dev:      dev,
		staticFS: staticFS,
	}

	s.mux.HandleFunc("/ws", s.handleWS)
	s.mux.HandleFunc("/api/tmux/focus/", s.handleTmuxFocus)
	s.mux.Handle("/", s.staticHandler())

	return s
}

// Run starts the hub, poller, and HTTP server.
func (s *Server) Run() error {
	go s.hub.Run()
	go s.poller.Run()

	log.Printf("claude-monitor web dashboard: http://%s", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) staticHandler() http.Handler {
	if s.dev {
		log.Println("dev mode: serving web/static/ from disk")
		return http.FileServer(http.Dir("web/static"))
	}
	return http.FileServer(http.FS(s.staticFS))
}

func (s *Server) handleTmuxFocus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pidStr := strings.TrimPrefix(r.URL.Path, "/api/tmux/focus/")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "reason": "invalid pid"})
		return
	}

	paneID, err := findTmuxPane(pid)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "reason": "session not in tmux"})
		return
	}

	// Switch tmux client to the target session, window, and pane
	sessionOut, _ := exec.Command("tmux", "display-message", "-t", paneID, "-p", "#{session_name}").Output()
	if sessionName := strings.TrimSpace(string(sessionOut)); sessionName != "" {
		exec.Command("tmux", "switch-client", "-t", sessionName).Run()
	}
	exec.Command("tmux", "select-window", "-t", paneID).Run()
	exec.Command("tmux", "select-pane", "-t", paneID).Run()

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pane": paneID})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// findTmuxPane finds the tmux pane that contains the given PID.
func findTmuxPane(targetPID int) (string, error) {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F", "#{pane_id} #{pane_pid}").Output()
	if err != nil {
		return "", fmt.Errorf("tmux list-panes: %w", err)
	}

	target := strconv.Itoa(targetPID)

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		paneID := parts[0]
		panePID := parts[1]

		if panePID == target {
			return paneID, nil
		}
		if hasDescendant(panePID, target, 0) {
			return paneID, nil
		}
	}

	return "", fmt.Errorf("no pane found for PID %d", targetPID)
}

// hasDescendant recursively checks if targetPID is a descendant of parentPID using pgrep.
func hasDescendant(parentPID, targetPID string, depth int) bool {
	if depth > 10 {
		return false
	}

	out, err := exec.Command("pgrep", "-P", parentPID).Output()
	if err != nil {
		return false
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		child := strings.TrimSpace(line)
		if child == "" {
			continue
		}
		if child == targetPID {
			return true
		}
		if hasDescendant(child, targetPID, depth+1) {
			return true
		}
	}
	return false
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
	})
	if err != nil {
		log.Printf("ws accept: %v", err)
		return
	}
	defer conn.CloseNow()

	// Send history backfill
	entries := s.poller.HistoryEntries()
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		if err := conn.Write(r.Context(), websocket.MessageText, data); err != nil {
			return
		}
	}

	// Register client channel
	ch := make(chan []byte, 64)
	s.hub.Register(ch)
	defer s.hub.Unregister(ch)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Read loop: handles ping/pong and detects disconnect.
	// nhooyr.io/websocket v1 Write is safe for concurrent use from multiple goroutines.
	go func() {
		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				cancel()
				return
			}
			if string(msg) == `{"type":"ping"}` {
				conn.Write(ctx, websocket.MessageText, []byte(`{"type":"pong"}`))
			}
		}
	}()

	// Write loop
	for {
		select {
		case msg := <-ch:
			if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
