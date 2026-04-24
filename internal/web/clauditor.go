package web

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// clauditorStatus mirrors `clauditor status --json`.
type clauditorStatus struct {
	Session        string  `json:"session"`
	Model          string  `json:"model"`
	Turns          int     `json:"turns"`
	CacheStatus    string  `json:"cacheStatus"`
	CacheRatio     float64 `json:"cacheRatio"`
	ContextPercent float64 `json:"contextPercent"`
	ContextTokens  int     `json:"contextTokens"`
	LoopDetected   bool    `json:"loopDetected"`
	Cost           float64 `json:"cost"`
	SavedByCache   float64 `json:"savedByCache"`
}

// clauditorSession mirrors one entry in `clauditor sessions --json`.
type clauditorSession struct {
	Label       string    `json:"label"`
	Model       string    `json:"model"`
	Turns       int       `json:"turns"`
	LastUpdated time.Time `json:"lastUpdated"`
	CacheStatus string    `json:"cacheStatus"`
	CacheRatio  float64   `json:"cacheRatio"`
	Cost        float64   `json:"cost"`
	SpikeTurns  int       `json:"spikeTurns"`
}

// clauditorImpact mirrors `clauditor impact --json`.
type clauditorImpact struct {
	FirstSeen           string         `json:"firstSeen"`
	LastUpdated         string         `json:"lastUpdated"`
	SessionsMonitored   int            `json:"sessionsMonitored"`
	TotalTurnsMonitored int            `json:"totalTurnsMonitored"`
	HealthySessionPct   int            `json:"healthySessionPct"`
	AvgCacheRatio       float64        `json:"avgCacheRatio"`
	Detected            map[string]int `json:"detected"`
}

// ClauditorPoller runs the clauditor CLI periodically and broadcasts results.
type ClauditorPoller struct {
	hub      *Hub
	binary   string
	stop     chan struct{}
	mu       sync.RWMutex
	latest   []byte
	present  bool
}

// NewClauditorPoller resolves the clauditor binary (PATH first, then ~/.nvm
// fallbacks). Returns a poller that produces no messages if the binary is
// unavailable, so the rest of the server keeps working.
func NewClauditorPoller(hub *Hub) *ClauditorPoller {
	return &ClauditorPoller{
		hub:     hub,
		binary:  resolveClauditorBinary(),
		stop:    make(chan struct{}),
		present: false,
	}
}

// Run polls sessions+status every 30s and impact every 10min. Each cycle
// merges the three into a single ClauditorMsg broadcast on the hub.
func (p *ClauditorPoller) Run() {
	if p.binary == "" {
		log.Printf("clauditor: binary not found on PATH — skipping clauditor poller")
		return
	}
	log.Printf("clauditor: using binary %s", p.binary)

	// clauditor CLI does a full jsonl scan per invocation (~10s+ on large
	// histories), so poll at human-meaningful cadence, not real-time.
	ticker := time.NewTicker(30 * time.Second)
	impactTicker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	defer impactTicker.Stop()

	var lastImpact *clauditorImpact

	// Prime immediately.
	lastImpact = p.readImpact()
	p.runCycle(lastImpact)

	for {
		select {
		case <-ticker.C:
			p.runCycle(lastImpact)
		case <-impactTicker.C:
			if imp := p.readImpact(); imp != nil {
				lastImpact = imp
			}
		case <-p.stop:
			return
		}
	}
}

func (p *ClauditorPoller) runCycle(im *clauditorImpact) {
	sessions := p.readSessions()
	status := p.readStatus()
	if msg := p.buildMsg(sessions, status, im); msg != nil {
		p.broadcast(msg)
	}
}

func (p *ClauditorPoller) Stop() { close(p.stop) }

// Latest returns the most recent serialized ClauditorMsg. Empty slice and
// false if nothing has been produced yet. Used to backfill new WS clients.
func (p *ClauditorPoller) Latest() ([]byte, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.present {
		return nil, false
	}
	return p.latest, true
}

func (p *ClauditorPoller) broadcast(data []byte) {
	p.mu.Lock()
	p.latest = data
	p.present = true
	p.mu.Unlock()
	p.hub.Broadcast(data)
}

func (p *ClauditorPoller) buildMsg(sessions []clauditorSession, st *clauditorStatus, im *clauditorImpact) []byte {
	if len(sessions) == 0 && st == nil {
		return nil
	}

	msg := ClauditorMsg{
		Type:      "clauditor",
		Timestamp: time.Now().UTC(),
		HasData:   true,
	}

	// Top 8 most-recent sessions. `clauditor sessions --json` already comes
	// sorted by lastUpdated desc, but trust nothing.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated.After(sessions[j].LastUpdated)
	})
	const topN = 8
	if len(sessions) > topN {
		sessions = sessions[:topN]
	}
	msg.Sessions = make([]ClauditorSessionMsg, 0, len(sessions))
	for _, s := range sessions {
		msg.Sessions = append(msg.Sessions, ClauditorSessionMsg{
			Label:       s.Label,
			Model:       s.Model,
			Turns:       s.Turns,
			LastUpdated: s.LastUpdated,
			CacheStatus: s.CacheStatus,
			CacheRatio:  s.CacheRatio,
			Cost:        s.Cost,
			SpikeTurns:  s.SpikeTurns,
		})
	}

	if st != nil {
		msg.Focus = &ClauditorFocusMsg{
			Session:        st.Session,
			Model:          st.Model,
			Turns:          st.Turns,
			CacheStatus:    st.CacheStatus,
			CacheRatio:     st.CacheRatio,
			ContextPercent: st.ContextPercent,
			ContextTokens:  st.ContextTokens,
			LoopDetected:   st.LoopDetected,
			Cost:           st.Cost,
			SavedByCache:   st.SavedByCache,
		}
	}

	if im != nil {
		msg.SessionsMonitored = im.SessionsMonitored
		msg.TotalTurnsMonitored = im.TotalTurnsMonitored
		msg.HealthySessionPct = im.HealthySessionPct
		msg.AvgCacheRatio = im.AvgCacheRatio
		msg.Detected = im.Detected
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("clauditor: marshal: %v", err)
		return nil
	}
	return data
}

func (p *ClauditorPoller) readSessions() []clauditorSession {
	out, err := runWithTimeout(p.binary, []string{"sessions", "--json"}, 30*time.Second)
	if err != nil {
		log.Printf("clauditor: sessions: %v", err)
		return nil
	}
	var ss []clauditorSession
	if err := json.Unmarshal(out, &ss); err != nil {
		log.Printf("clauditor: parse sessions: %v", err)
		return nil
	}
	return ss
}

func (p *ClauditorPoller) readStatus() *clauditorStatus {
	out, err := runWithTimeout(p.binary, []string{"status", "--json"}, 30*time.Second)
	if err != nil {
		log.Printf("clauditor: status: %v", err)
		return nil
	}
	var st clauditorStatus
	if err := json.Unmarshal(out, &st); err != nil {
		log.Printf("clauditor: parse status: %v", err)
		return nil
	}
	return &st
}

func (p *ClauditorPoller) readImpact() *clauditorImpact {
	out, err := runWithTimeout(p.binary, []string{"impact", "--json"}, 30*time.Second)
	if err != nil {
		log.Printf("clauditor: impact: %v", err)
		return nil
	}
	var im clauditorImpact
	if err := json.Unmarshal(out, &im); err != nil {
		log.Printf("clauditor: parse impact: %v", err)
		return nil
	}
	return &im
}

func runWithTimeout(binary string, args []string, d time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	// clauditor is a Node CLI with `#!/usr/bin/env node` shebang. When the
	// parent process (e.g. launchd) doesn't inherit nvm's PATH, `env node`
	// can't resolve and the script exits 127. Prepend the binary's own
	// directory to PATH so the sibling `node` shim is discoverable.
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.New("timed out")
		}
		return nil, err
	}
	return out, nil
}

// resolveClauditorBinary prefers PATH; falls back to the newest
// ~/.nvm/versions/node/*/bin/clauditor entry. Returns "" if nothing works.
func resolveClauditorBinary() string {
	if p, err := exec.LookPath("clauditor"); err == nil {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".nvm", "versions", "node", "*", "bin", "clauditor"))
	if len(matches) == 0 {
		return ""
	}
	// Prefer the most-recently-modified binary so upgrades win.
	sort.Slice(matches, func(i, j int) bool {
		ai, _ := os.Stat(matches[i])
		aj, _ := os.Stat(matches[j])
		if ai == nil || aj == nil {
			return false
		}
		return ai.ModTime().After(aj.ModTime())
	})
	return matches[0]
}
