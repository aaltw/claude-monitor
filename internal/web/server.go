package web

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"

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

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
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

	// Read loop (handles ping/pong and detects disconnect)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

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
