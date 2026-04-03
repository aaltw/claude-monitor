package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aaltwesthuis/claude-monitor/internal/config"
)

// ReadBridgeFiles reads all claude-monitor-*.json files from the given directory.
func ReadBridgeFiles(dir string) ([]BridgeData, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var bridges []BridgeData
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "claude-monitor-") || !strings.HasSuffix(name, ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		var bd BridgeData
		if err := json.Unmarshal(data, &bd); err != nil {
			continue // skip malformed
		}
		bridges = append(bridges, bd)
	}
	return bridges, nil
}

// ReadStateFiles reads all claude-session-state-*.json files from the given directory.
// Each state file is keyed by PID. Files are named claude-session-state-{pid}.json,
// so duplicate PIDs should not occur in practice.
func ReadStateFiles(dir string) (map[int]SessionState, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	states := make(map[int]SessionState)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "claude-session-state-") || !strings.HasSuffix(name, ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		var ss SessionState
		if err := json.Unmarshal(data, &ss); err != nil {
			continue
		}
		states[ss.PID] = ss
	}
	return states, nil
}

// LatestUsage returns the usage snapshot from the most recently updated bridge
// that has rate limit data.
func LatestUsage(bridges []BridgeData) UsageSnapshot {
	var best *BridgeData
	for i := range bridges {
		b := &bridges[i]
		if !b.HasRateLimits() {
			continue
		}
		if best == nil || b.Timestamp > best.Timestamp {
			best = b
		}
	}

	if best == nil {
		return UsageSnapshot{HasData: false}
	}

	// Sum tokens across all bridge files for accurate velocity tracking
	totalTokens := 0
	for i := range bridges {
		totalTokens += bridges[i].Tokens.Total()
	}

	lastUpdate := time.Unix(best.Timestamp, 0)
	isStale := time.Since(lastUpdate) > config.StaleThreshold

	return UsageSnapshot{
		FiveHour: WindowUsage{
			UsedPercentage: best.RateLimits.FiveHour.UsedPercentage,
			ResetsAt:       time.Unix(int64(best.RateLimits.FiveHour.ResetsAt), 0),
			Severity:       SeverityFor(best.RateLimits.FiveHour.UsedPercentage),
		},
		SevenDay: WindowUsage{
			UsedPercentage: best.RateLimits.SevenDay.UsedPercentage,
			ResetsAt:       time.Unix(int64(best.RateLimits.SevenDay.ResetsAt), 0),
			Severity:       SeverityFor(best.RateLimits.SevenDay.UsedPercentage),
		},
		TotalTokens: totalTokens,
		HasData:     true,
		IsStale:     isStale,
		LastUpdate:  lastUpdate,
	}
}
