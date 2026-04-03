package web

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aaltw/claude-monitor/internal/config"
	"github.com/aaltw/claude-monitor/internal/data"
)

// Poller reads data files and pushes messages to the hub.
type Poller struct {
	hub        *Hub
	history    *HistoryBuffer
	ringBuffer *data.RingBuffer
	prevStatus map[int]string
	stop       chan struct{}
}

func NewPoller(hub *Hub, history *HistoryBuffer) *Poller {
	return &Poller{
		hub:        hub,
		history:    history,
		ringBuffer: data.NewRingBuffer(config.RingBufferSize),
		prevStatus: make(map[int]string),
		stop:       make(chan struct{}),
	}
}

func (p *Poller) Run() {
	stateTicker := time.NewTicker(config.PollBridge)
	historyTicker := time.NewTicker(5 * time.Second)
	defer stateTicker.Stop()
	defer historyTicker.Stop()

	p.pollAndBroadcastState()

	for {
		select {
		case <-stateTicker.C:
			p.pollAndBroadcastState()
		case <-historyTicker.C:
			p.pollAndBroadcastHistory()
		case <-p.stop:
			return
		}
	}
}

func (p *Poller) Stop() { close(p.stop) }

func (p *Poller) pollAndBroadcastState() {
	bridges, _ := data.ReadBridgeFiles(config.BridgeDir)
	states, _ := data.ReadStateFiles(config.BridgeDir)
	registries, _ := data.ReadSessionRegistry(config.SessionsDir)

	usage := data.LatestUsage(bridges)
	sessions := data.MergeSessions(registries, states, bridges)

	if usage.HasData {
		p.ringBuffer.Add(data.Observation{
			Timestamp:   time.Now(),
			FiveHourPct: usage.FiveHour.UsedPercentage,
			SevenDayPct: usage.SevenDay.UsedPercentage,
			TotalTokens: usage.TotalTokens,
		})
	}
	burnRate := p.ringBuffer.CalcBurnRate(time.Duration(config.BurnRateWindowMin) * time.Minute)

	events := DetectStateChanges(p.prevStatus, sessions)
	for _, evt := range events {
		if evtJSON, err := json.Marshal(evt); err == nil {
			p.hub.Broadcast(evtJSON)
		}
	}

	for _, s := range sessions {
		p.prevStatus[s.PID] = statusString(s.Status)
	}

	msg := BuildStateMsg(usage, burnRate, sessions, bridges)
	if msgJSON, err := json.Marshal(msg); err == nil {
		p.hub.Broadcast(msgJSON)
	} else {
		log.Printf("web: marshal state: %v", err)
	}
}

func (p *Poller) pollAndBroadcastHistory() {
	bridges, _ := data.ReadBridgeFiles(config.BridgeDir)
	usage := data.LatestUsage(bridges)
	burnRate := p.ringBuffer.CalcBurnRate(time.Duration(config.BurnRateWindowMin) * time.Minute)

	msg := BuildHistoryMsg(usage, burnRate, bridges)
	p.history.Add(msg)

	if msgJSON, err := json.Marshal(msg); err == nil {
		p.hub.Broadcast(msgJSON)
	}
}

func (p *Poller) HistoryEntries() []HistoryMsg {
	return p.history.Entries()
}

// BuildStateMsg assembles a StateMsg from data layer types.
func BuildStateMsg(usage data.UsageSnapshot, burnRate data.BurnRate, sessions []data.SessionInfo, bridges []data.BridgeData) StateMsg {
	msg := StateMsg{Type: "state"}

	msg.Usage = UsageMsg{
		HasData:     usage.HasData,
		IsStale:     usage.IsStale,
		TotalTokens: usage.TotalTokens,
	}
	if usage.HasData {
		msg.Usage.FiveHour = WindowMsg{
			UsedPct:  usage.FiveHour.UsedPercentage,
			ResetsAt: usage.FiveHour.ResetsAt.Format(time.RFC3339),
			Severity: usage.FiveHour.Severity,
		}
		msg.Usage.SevenDay = WindowMsg{
			UsedPct:  usage.SevenDay.UsedPercentage,
			ResetsAt: usage.SevenDay.ResetsAt.Format(time.RFC3339),
			Severity: usage.SevenDay.Severity,
		}
	}

	var tteMinutes float64
	if burnRate.TimeToExhaust > 0 {
		tteMinutes = burnRate.TimeToExhaust.Minutes()
	}
	msg.BurnRate = BurnRateMsg{
		HasData:       burnRate.HasData,
		PctPerHour:    burnRate.PercentPerHour,
		TokensPerHour: burnRate.TokensPerHour,
		TTEMinutes:    tteMinutes,
	}

	for _, s := range sessions {
		msg.Sessions = append(msg.Sessions, SessionMsg{
			PID:     s.PID,
			HexID:   data.SessionHexID(s.PID),
			Name:    s.Name,
			Model:   s.Model,
			Status:  statusString(s.Status),
			Latency: data.FormatLatency(s.LastBridge),
			Cwd:     s.Cwd,
		})
	}

	modelTokens := make(map[string]int)
	for _, b := range bridges {
		if b.Model != "" {
			modelTokens[b.Model] += b.Tokens.Total()
		}
	}
	totalAllModels := 0
	for _, t := range modelTokens {
		totalAllModels += t
	}
	msg.Models = make(map[string]ModelMsg)
	for model, tokens := range modelTokens {
		pct := 0.0
		if totalAllModels > 0 {
			pct = float64(tokens) / float64(totalAllModels) * 100
		}
		msg.Models[model] = ModelMsg{TotalTokens: tokens, Pct: pct}
	}

	return msg
}

// BuildHistoryMsg assembles a HistoryMsg from current data.
func BuildHistoryMsg(usage data.UsageSnapshot, burnRate data.BurnRate, bridges []data.BridgeData) HistoryMsg {
	msg := HistoryMsg{
		Type:      "history",
		Timestamp: time.Now(),
	}
	if usage.HasData {
		msg.FiveHourPct = usage.FiveHour.UsedPercentage
		msg.SevenDayPct = usage.SevenDay.UsedPercentage
		msg.TotalTokens = usage.TotalTokens
	}
	if burnRate.HasData {
		msg.BurnRatePct = burnRate.PercentPerHour
	}
	msg.TokensByModel = make(map[string]int)
	for _, b := range bridges {
		if b.Model != "" {
			msg.TokensByModel[b.Model] += b.Tokens.Total()
		}
	}
	return msg
}

// DetectStateChanges compares previous and current session states, returns events.
func DetectStateChanges(prev map[int]string, current []data.SessionInfo) []EventMsg {
	var events []EventMsg
	for _, s := range current {
		currStatus := statusString(s.Status)
		prevStatus, existed := prev[s.PID]
		if !existed {
			events = append(events, EventMsg{
				Type: "event", Timestamp: time.Now(), PID: s.PID,
				Session: s.Name, Model: s.Model,
				Action: "session_started", Detail: "session started",
			})
		} else if prevStatus != currStatus {
			events = append(events, EventMsg{
				Type: "event", Timestamp: time.Now(), PID: s.PID,
				Session: s.Name, Model: s.Model,
				Action: "state_change", Detail: fmt.Sprintf("%s → %s", prevStatus, currStatus),
			})
		}
	}
	return events
}

func statusString(s data.SessionStatus) string {
	return strings.ToLower(s.String())
}
