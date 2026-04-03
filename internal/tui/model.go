package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aaltw/claude-monitor/internal/config"
	"github.com/aaltw/claude-monitor/internal/data"
	"github.com/aaltw/claude-monitor/internal/tui/components"
	"github.com/aaltw/claude-monitor/internal/tui/theme"
)

// ── Messages ────────────────────────────────────────────────────────────────

type tickMsg time.Time
type bridgeTickMsg time.Time
type stateTickMsg time.Time
type registryTickMsg time.Time
type pidTickMsg time.Time

// ── Model ───────────────────────────────────────────────────────────────────

type Model struct {
	width     int
	height    int
	ready     bool
	startTime time.Time

	// Data
	usage      data.UsageSnapshot
	burnRate   data.BurnRate
	sessions   []data.SessionInfo
	ringBuffer *data.RingBuffer

	// UI state
	animTick  int
	sortOrder data.SortOrder

	// Zombie tracking: PID -> time first detected dead
	zombies map[int]time.Time
}

func NewModel() Model {
	return Model{
		startTime:  time.Now(),
		ringBuffer: data.NewRingBuffer(config.RingBufferSize),
		zombies:    make(map[int]time.Time),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickAnimation(),
		tickBridge(),
		tickState(),
		tickRegistry(),
		tickPID(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		result, cmd := handleKey(msg, &m)
		return result, cmd

	case tickMsg:
		m.animTick++
		return m, tickAnimation()

	case bridgeTickMsg:
		m.refreshBridge()
		return m, tickBridge()

	case stateTickMsg:
		m.refreshState()
		return m, tickState()

	case registryTickMsg:
		m.refreshRegistry()
		return m, tickRegistry()

	case pidTickMsg:
		m.refreshPIDs()
		return m, tickPID()
	}

	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return ""
	}

	if m.width < config.MinTermWidth || m.height < config.MinTermHeight {
		msg := theme.SizeWarning.Width(m.width).Height(m.height).Render(
			"TERMINAL TOO SMALL -- REQUIRES 120+ COLUMNS, 30+ ROWS",
		)
		return msg
	}

	// Zone 1: Usage (2/3) + Burn Rate (1/3)
	usageWidth := m.width * 2 / 3
	burnWidth := m.width - usageWidth

	usagePanel := components.RenderUsagePanel(m.usage, usageWidth, m.usage.IsStale, m.animTick)
	burnPanel := components.RenderBurnRatePanel(m.burnRate, burnWidth, 0, m.animTick)

	topSection := lipgloss.JoinHorizontal(lipgloss.Top, usagePanel, burnPanel)

	// Zone 2: Session table
	sessionTable := components.RenderSessionTable(m.sessions, m.width, m.sortOrder, m.animTick)

	// Zone 3: Footer
	footer := components.RenderFooter(m.startTime, m.width)

	// Compose
	content := lipgloss.JoinVertical(lipgloss.Left,
		topSection,
		sessionTable,
		footer,
	)

	// Fill remaining space with base background
	return lipgloss.NewStyle().
		Background(theme.ColSurface()).
		Width(m.width).
		Height(m.height).
		Render(content)
}

// ── Data refresh methods ────────────────────────────────────────────────────

func (m *Model) refreshBridge() {
	bridges, err := data.ReadBridgeFiles(config.BridgeDir)
	if err != nil {
		return
	}
	m.usage = data.LatestUsage(bridges)

	// Feed ring buffer
	if m.usage.HasData {
		m.ringBuffer.Add(data.Observation{
			Timestamp:   time.Now(),
			FiveHourPct: m.usage.FiveHour.UsedPercentage,
			SevenDayPct: m.usage.SevenDay.UsedPercentage,
			TotalTokens: m.usage.TotalTokens,
		})
		m.burnRate = m.ringBuffer.CalcBurnRate(time.Duration(config.BurnRateWindowMin) * time.Minute)
	}
}

func (m *Model) refreshState() {
	states, err := data.ReadStateFiles(config.BridgeDir)
	if err != nil {
		return
	}

	// Update session statuses from state files
	for i, sess := range m.sessions {
		if sess.Status == data.StatusZombie {
			continue
		}
		if state, ok := states[sess.PID]; ok {
			switch state.State {
			case "working":
				m.sessions[i].Status = data.StatusWorking
			case "waiting":
				m.sessions[i].Status = data.StatusWaiting
			case "blocked":
				m.sessions[i].Status = data.StatusBlocked
			default:
				m.sessions[i].Status = data.StatusIdle
			}
		}
	}
}

func (m *Model) refreshRegistry() {
	registries, err := data.ReadSessionRegistry(config.SessionsDir)
	if err != nil {
		return
	}

	states, _ := data.ReadStateFiles(config.BridgeDir)
	bridges, _ := data.ReadBridgeFiles(config.BridgeDir)

	newSessions := data.MergeSessions(registries, states, bridges)

	// Apply zombie tracking
	for i, sess := range newSessions {
		if sess.Status == data.StatusZombie {
			if since, ok := m.zombies[sess.PID]; ok {
				newSessions[i].ZombieSince = since
			} else {
				m.zombies[sess.PID] = time.Now()
				newSessions[i].ZombieSince = time.Now()
			}
		} else {
			delete(m.zombies, sess.PID)
		}
	}

	// Filter out zombies past cleanup delay
	var filtered []data.SessionInfo
	for _, sess := range newSessions {
		if sess.Status == data.StatusZombie && !sess.ZombieSince.IsZero() {
			if time.Since(sess.ZombieSince) > config.ZombieCleanupDelay {
				delete(m.zombies, sess.PID)
				continue
			}
		}
		filtered = append(filtered, sess)
	}

	m.sessions = filtered
}

func (m *Model) refreshPIDs() {
	for i, sess := range m.sessions {
		if sess.Status == data.StatusZombie {
			continue
		}
		if !data.IsPIDAlive(sess.PID) {
			m.sessions[i].Status = data.StatusZombie
			m.sessions[i].Alive = false
			if _, ok := m.zombies[sess.PID]; !ok {
				m.zombies[sess.PID] = time.Now()
				m.sessions[i].ZombieSince = time.Now()
			}
		}
	}
}

func (m *Model) forceRefresh() tea.Cmd {
	m.refreshBridge()
	m.refreshState()
	m.refreshRegistry()
	m.refreshPIDs()
	return nil
}

// ── Tick commands ───────────────────────────────────────────────────────────

func tickAnimation() tea.Cmd {
	return tea.Tick(config.TickAnimation, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func tickBridge() tea.Cmd {
	return tea.Tick(config.PollBridge, func(t time.Time) tea.Msg {
		return bridgeTickMsg(t)
	})
}

func tickState() tea.Cmd {
	return tea.Tick(config.PollState, func(t time.Time) tea.Msg {
		return stateTickMsg(t)
	})
}

func tickRegistry() tea.Cmd {
	return tea.Tick(config.PollRegistry, func(t time.Time) tea.Msg {
		return registryTickMsg(t)
	})
}

func tickPID() tea.Cmd {
	return tea.Tick(config.PollPID, func(t time.Time) tea.Msg {
		return pidTickMsg(t)
	})
}
