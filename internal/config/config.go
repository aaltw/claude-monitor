package config

import (
	"os"
	"path/filepath"
	"time"
)

const (
	Version    = "1.0.0"
	VersionTag = "V1.0-STABLE"

	MinTermWidth  = 120
	MinTermHeight = 30

	PollBridge    = 2 * time.Second
	PollState     = 1 * time.Second
	PollRegistry  = 5 * time.Second
	PollPID       = 10 * time.Second
	TickAnimation = 200 * time.Millisecond

	RingBufferSize    = 360
	BurnRateWindowMin = 15

	StaleThreshold     = 5 * time.Minute
	ZombieGracePeriod  = 30 * time.Second
	ZombieCleanupDelay = 60 * time.Second
)

var (
	BridgeDir   = os.TempDir()
	SessionsDir = filepath.Join(homeDir(), ".claude", "sessions")
)

func BridgePattern() string {
	return filepath.Join(BridgeDir, "claude-monitor-*.json")
}

func StatePattern() string {
	return filepath.Join(BridgeDir, "claude-session-state-*.json")
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "/tmp"
	}
	return h
}
