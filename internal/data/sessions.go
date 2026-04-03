package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// ReadSessionRegistry reads all ~/.claude/sessions/*.json files.
func ReadSessionRegistry(dir string) ([]SessionRegistry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var registries []SessionRegistry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var reg SessionRegistry
		if err := json.Unmarshal(data, &reg); err != nil {
			continue
		}
		registries = append(registries, reg)
	}
	return registries, nil
}

// IsPIDAlive checks if a process with the given PID exists.
func IsPIDAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

// MergeSessions combines registry, state, and bridge data into a unified view.
func MergeSessions(registries []SessionRegistry, states map[int]SessionState, bridges []BridgeData) []SessionInfo {
	// Index bridges by session ID
	bridgeBySession := make(map[string]BridgeData)
	for _, b := range bridges {
		existing, ok := bridgeBySession[b.SessionID]
		if !ok || b.Timestamp > existing.Timestamp {
			bridgeBySession[b.SessionID] = b
		}
	}

	var sessions []SessionInfo
	for _, reg := range registries {
		info := SessionInfo{
			PID:       reg.PID,
			SessionID: reg.SessionID,
			Cwd:       reg.Cwd,
			StartedAt: time.UnixMilli(reg.StartedAt),
		}

		// Name: use session name, fall back to last path component of cwd
		if reg.Name != "" {
			info.Name = reg.Name
		} else {
			info.Name = filepath.Base(reg.Cwd)
		}

		// PID liveness
		info.Alive = IsPIDAlive(reg.PID)

		// Status: check PID liveness, then state file, then bridge recency
		if !info.Alive {
			info.Status = StatusZombie
			info.ZombieSince = time.Now()
		} else if state, ok := states[reg.PID]; ok {
			switch state.State {
			case "working":
				info.Status = StatusWorking
			case "waiting":
				info.Status = StatusWaiting
			case "blocked":
				info.Status = StatusBlocked
			default:
				info.Status = StatusIdle
			}
		} else if !info.LastBridge.IsZero() && time.Since(info.LastBridge) < 10*time.Second {
			// No state file, but bridge was recently updated - session is active
			info.Status = StatusWorking
		} else {
			info.Status = StatusIdle
		}

		// Latency and model from bridge file
		if b, ok := bridgeBySession[reg.SessionID]; ok {
			info.LastBridge = time.Unix(b.Timestamp, 0)
			info.Model = b.Model
		}

		sessions = append(sessions, info)
	}

	return sessions
}

// SessionHexID returns a short hex string from a PID, e.g. 44946 -> "0xAF92".
func SessionHexID(pid int) string {
	return fmt.Sprintf("0x%04X", pid&0xFFFF)
}

// FormatLatency returns a human-readable latency string.
func FormatLatency(lastBridge time.Time) string {
	if lastBridge.IsZero() {
		return "INF"
	}
	d := time.Since(lastBridge)
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	default:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
}
