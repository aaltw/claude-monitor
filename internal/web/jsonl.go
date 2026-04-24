package web

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aaltw/claude-monitor/internal/proxy"
)

// Claude Code writes one JSONL per session under
// ~/.claude/projects/<slug>/<sessionId>.jsonl. Each assistant turn is a line
// whose message.usage carries the full token accounting. Tailing these files
// with byte-offset resume gives the same data the reverse-proxy produced —
// without requiring ANTHROPIC_BASE_URL rerouting. Reference:
// ryoppippi/ccusage + hoangsonww/Claude-Code-Agent-Monitor.

// Configuration defaults for the tailer.
const (
	jsonlPollInterval   = 4 * time.Second
	jsonlActivityWindow = 30 * time.Minute // skip files older than this
	jsonlMaxTurnHistory = 200
)

type jsonlLine struct {
	Type      string    `json:"type"`
	SessionID string    `json:"sessionId"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Model string     `json:"model"`
		Usage jsonlUsage `json:"usage"`
	} `json:"message"`
}

type jsonlUsage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

func (u jsonlUsage) totalInput() int {
	return u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// fileCursor remembers where we left off reading a single jsonl file.
type fileCursor struct {
	path      string
	size      int64
	offset    int64
	sessionID string
	mtime     time.Time
}

// sessionState aggregates every assistant turn we've seen for one session.
type sessionState struct {
	sessionID    string
	projectSlug  string
	path         string
	model        string
	turnCount    int
	lastSeen     time.Time
	lastUsage    jsonlUsage
	turns        []turnEntry
}

type turnEntry struct {
	TurnNumber    int       `json:"turn"`
	Timestamp     time.Time `json:"ts"`
	Input         int       `json:"input"`
	CacheCreation int       `json:"cache_creation"`
	CacheRead     int       `json:"cache_read"`
	Output        int       `json:"output"`
}

// JSONLTailer watches ~/.claude/projects/*/*.jsonl and broadcasts
// context-composition snapshots derived from per-turn usage data.
type JSONLTailer struct {
	hub         *Hub
	projectsDir string
	stop        chan struct{}

	mu       sync.RWMutex
	cursors  map[string]*fileCursor
	sessions map[string]*sessionState
	latest   []byte
	present  bool
}

// NewJSONLTailer returns a tailer rooted at ~/.claude/projects (or $CLAUDE_CONFIG_DIR/projects if set).
func NewJSONLTailer(hub *Hub) *JSONLTailer {
	base, err := os.UserHomeDir()
	if err != nil {
		base = "/"
	}
	dir := filepath.Join(base, ".claude", "projects")
	if cfg := os.Getenv("CLAUDE_CONFIG_DIR"); cfg != "" {
		dir = filepath.Join(cfg, "projects")
	}
	return &JSONLTailer{
		hub:         hub,
		projectsDir: dir,
		stop:        make(chan struct{}),
		cursors:     make(map[string]*fileCursor),
		sessions:    make(map[string]*sessionState),
	}
}

func (t *JSONLTailer) Run() {
	if _, err := os.Stat(t.projectsDir); err != nil {
		log.Printf("jsonl: projects dir unavailable (%v) — tailer disabled", err)
		return
	}
	log.Printf("jsonl: tailing %s (poll %s, activity window %s)", t.projectsDir, jsonlPollInterval, jsonlActivityWindow)

	tick := time.NewTicker(jsonlPollInterval)
	defer tick.Stop()

	t.scan()
	for {
		select {
		case <-tick.C:
			t.scan()
		case <-t.stop:
			return
		}
	}
}

func (t *JSONLTailer) Stop() { close(t.stop) }

// Latest returns the most recently broadcast snapshot for new WS backfill.
func (t *JSONLTailer) Latest() ([]byte, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.present {
		return nil, false
	}
	return t.latest, true
}

// scan walks the projects tree, reads only files modified within the
// activity window, and parses any bytes appended since the last cursor.
func (t *JSONLTailer) scan() {
	entries, err := filepath.Glob(filepath.Join(t.projectsDir, "*", "*.jsonl"))
	if err != nil {
		log.Printf("jsonl: glob: %v", err)
		return
	}
	cutoff := time.Now().Add(-jsonlActivityWindow)
	var touched bool

	for _, path := range entries {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			continue
		}

		t.mu.Lock()
		cur, ok := t.cursors[path]
		if !ok {
			cur = &fileCursor{path: path, sessionID: sessionIDFromPath(path)}
			t.cursors[path] = cur
			// Seed cursor: jump to end of file so we only stream *new* data.
			// For the focus session we need at least one turn of history —
			// so instead, if the file has grown recently, rewind to start.
			cur.offset = 0
		}
		t.mu.Unlock()

		if info.Size() == cur.size && info.ModTime().Equal(cur.mtime) {
			continue
		}
		if info.Size() < cur.offset {
			// Log file was truncated/rotated — restart from beginning.
			cur.offset = 0
		}

		added, err := t.readNewLines(cur, info.Size())
		if err != nil {
			log.Printf("jsonl: read %s: %v", path, err)
			continue
		}
		cur.size = info.Size()
		cur.mtime = info.ModTime()
		if added > 0 {
			touched = true
		}
	}

	t.pruneStaleSessions(cutoff)

	if touched {
		if msg := t.buildSnapshot(); msg != nil {
			t.broadcast(msg)
		}
	}
}

func (t *JSONLTailer) readNewLines(cur *fileCursor, size int64) (int, error) {
	f, err := os.Open(cur.path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if _, err := f.Seek(cur.offset, io.SeekStart); err != nil {
		return 0, err
	}

	reader := bufio.NewReaderSize(f, 64*1024)
	added := 0
	var lastCompleteOffset int64 = cur.offset

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 && (err == nil || err == io.EOF) {
			// Only commit offset past a line that ended with '\n'.
			ended := len(line) > 0 && line[len(line)-1] == '\n'
			if ended {
				if t.processLine(cur, line) {
					added++
				}
				lastCompleteOffset += int64(len(line))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return added, err
		}
	}
	cur.offset = lastCompleteOffset
	_ = size
	return added, nil
}

// processLine updates session state for one JSONL line. Returns true if the
// line advanced any user-visible counter (i.e. an assistant turn with real
// usage). Non-assistant / synthetic / zero-usage lines are still consumed
// but don't flip the flag.
func (t *JSONLTailer) processLine(cur *fileCursor, line []byte) bool {
	var ll jsonlLine
	if err := json.Unmarshal(line, &ll); err != nil {
		return false
	}
	if ll.Type != "assistant" {
		return false
	}
	u := ll.Message.Usage
	if u.totalInput() == 0 && u.OutputTokens == 0 {
		return false
	}
	// "<synthetic>" model = Claude Code's canned non-API responses (e.g. login).
	if ll.Message.Model == "" || ll.Message.Model == "<synthetic>" {
		return false
	}

	sid := ll.SessionID
	if sid == "" {
		sid = cur.sessionID
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.sessions[sid]
	if !ok {
		s = &sessionState{
			sessionID:   sid,
			projectSlug: filepath.Base(filepath.Dir(cur.path)),
			path:        cur.path,
		}
		t.sessions[sid] = s
	}
	s.model = ll.Message.Model
	s.turnCount++
	s.lastSeen = ll.Timestamp
	s.lastUsage = u
	s.turns = append(s.turns, turnEntry{
		TurnNumber:    s.turnCount,
		Timestamp:     ll.Timestamp,
		Input:         u.InputTokens,
		CacheCreation: u.CacheCreationInputTokens,
		CacheRead:     u.CacheReadInputTokens,
		Output:        u.OutputTokens,
	})
	if len(s.turns) > jsonlMaxTurnHistory {
		s.turns = s.turns[len(s.turns)-jsonlMaxTurnHistory:]
	}
	return true
}

func (t *JSONLTailer) pruneStaleSessions(cutoff time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, s := range t.sessions {
		if s.lastSeen.Before(cutoff) {
			delete(t.sessions, id)
		}
	}
	for path, c := range t.cursors {
		if !c.mtime.IsZero() && c.mtime.Before(cutoff) {
			delete(t.cursors, path)
		}
	}
}

// buildSnapshot picks the most-recently-updated session as the focus and
// serialises a JSONLContextMsg.
func (t *JSONLTailer) buildSnapshot() []byte {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.sessions) == 0 {
		return nil
	}

	var focus *sessionState
	for _, s := range t.sessions {
		if focus == nil || s.lastSeen.After(focus.lastSeen) {
			focus = s
		}
	}
	if focus == nil {
		return nil
	}

	u := focus.lastUsage
	totalInput := u.totalInput()
	windowSize := proxy.LookupContextWindowSize(focus.model, false)
	usedPct := 0.0
	if windowSize > 0 {
		usedPct = float64(totalInput) / float64(windowSize) * 100
	}
	cacheHit := 0.0
	if denom := u.CacheReadInputTokens + u.CacheCreationInputTokens; denom > 0 {
		cacheHit = float64(u.CacheReadInputTokens) / float64(denom) * 100
	}
	var deltaInput int
	if n := len(focus.turns); n >= 2 {
		prev := focus.turns[n-2]
		cur := focus.turns[n-1]
		deltaInput = (cur.Input + cur.CacheCreation + cur.CacheRead) -
			(prev.Input + prev.CacheCreation + prev.CacheRead)
	}

	msg := JSONLContextMsg{
		Type:              "jsonl_context",
		Timestamp:         time.Now().UTC(),
		SessionID:         focus.sessionID,
		ProjectSlug:       humanProjectSlug(focus.projectSlug),
		Model:             focus.model,
		TurnNumber:        focus.turnCount,
		InputTokens:       u.InputTokens,
		CacheCreation:     u.CacheCreationInputTokens,
		CacheRead:         u.CacheReadInputTokens,
		OutputTokens:      u.OutputTokens,
		TotalInputTokens:  totalInput,
		ContextWindowSize: windowSize,
		UsedPct:           usedPct,
		CacheHitRatio:     cacheHit,
		InputDelta:        deltaInput,
		LastTurnAt:        focus.lastSeen,
		ActiveSessions:    len(t.sessions),
		Turns:             append([]turnEntry(nil), focus.turns...),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("jsonl: marshal: %v", err)
		return nil
	}
	return data
}

func (t *JSONLTailer) broadcast(data []byte) {
	t.mu.Lock()
	t.latest = data
	t.present = true
	t.mu.Unlock()
	t.hub.Broadcast(data)
}

// sessionIDFromPath extracts the session UUID from <slug>/<uuid>.jsonl.
func sessionIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".jsonl")
}

// humanProjectSlug turns "-Users-aaltwesthuis-Sources-foo-bar" into
// "foo/bar". Claude Code encodes paths by replacing "/" with "-" and
// prefixing the dir. We just take the last couple of segments for display.
func humanProjectSlug(slug string) string {
	s := strings.TrimPrefix(slug, "-")
	parts := strings.Split(s, "-")
	if len(parts) == 0 {
		return slug
	}
	if len(parts) <= 2 {
		return strings.Join(parts, "/")
	}
	return strings.Join(parts[len(parts)-2:], "/")
}
