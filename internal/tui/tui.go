package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	server "Pusher/internal"
	"Pusher/internal/alert"
	"Pusher/internal/config"
	"Pusher/internal/health"
	"Pusher/internal/history"
	"Pusher/internal/process"
	"Pusher/internal/stress"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Styles ---
var (
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#04B575")
	alertColor     = lipgloss.Color("#E02E3E")
	textMuted      = lipgloss.Color("#767676")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(primaryColor).
			Padding(0, 2).
			MarginBottom(1)

	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(textMuted).
			PaddingRight(2).
			PaddingLeft(2).
			MarginRight(2)

	activeChoiceStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				PaddingLeft(1).
				PaddingRight(2).
				PaddingTop(1).
				PaddingBottom(1).
				BorderStyle(lipgloss.NormalBorder()).
				BorderLeft(true).
				BorderForeground(primaryColor)

	inactiveChoiceStyle = lipgloss.NewStyle().
				Foreground(textMuted).
				PaddingLeft(2).
				PaddingRight(2).
				PaddingTop(1).
				PaddingBottom(1)

	contentStyle = lipgloss.NewStyle().PaddingTop(1).PaddingLeft(2)

	statLabelStyle = lipgloss.NewStyle().Foreground(textMuted).Width(14)
	statValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Bold(true)

	bannerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(2).
			MarginLeft(2)
)

const banner = `
    ____        __    _           
   / __ \____  / /_  (_)___      
  / /_/ / __ \/ __ \/ / __ \     
 / _, _/ /_/ / /_/ / / / / /    
/_/ |_|\____/_.___/_/_/ /_/  
`

// --- Formatters ---
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func progressBar(percent float64) string {
	width := 30
	filled := int((percent / 100.0) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	empty := width - filled

	barColor := secondaryColor
	if percent > 85 {
		barColor = alertColor
	}

	bar := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("░", empty))
	return fmt.Sprintf("%s %5.1f%%", bar, percent)
}

// --- Bubble Tea Logic ---

type metricsMsg server.CpuStat

// connLostMsg is fired when no metric has arrived within the heartbeat window,
// meaning the backend has likely gone away. The TUI will enter RECONNECTING state.
type connLostMsg struct{}

// reconnectTickMsg is fired periodically while disconnected to re-probe the backend.
type reconnectTickMsg struct{}

type model struct {
	// original fields — untouched
	metrics server.CpuStat
	choices []string
	cursor  int
	ch      chan server.CpuStat
	processCh chan []process.ProcessStat

	// analytics: computed locally on every metrics tick
	hist        *history.RingBuffer
	alertEngine *alert.Engine
	healthScore health.Score
	alerts      []alert.Event

	// process inspector: refreshed every 2 seconds via pollProcessCmd
	processes      []process.ProcessStat
	processSortKey string
	processSortAsc bool


	// stress testing: controlled via backend API
	stressStatus stress.Status

	// target backend URL
	targetURL string

	// connection state: true after first successful metric received
	connected   bool
	connectedAt time.Time
	lastMetricAt time.Time // updated on every metricsMsg; used for heartbeat

	// url configuration and editing
	urlInput     textinput.Model
	probing      bool
	probeErr     string
	streamCtx    context.Context
	streamCancel context.CancelFunc
}

// heartbeatWindow is how long to wait for a metric before declaring the connection lost.
const heartbeatWindow = 5 * time.Second

// reconnectInterval is how long to wait between re-probe attempts while disconnected.
const reconnectInterval = 3 * time.Second

func waitForActivity(ch chan server.CpuStat) tea.Cmd {
	return func() tea.Msg {
		select {
		case stat, ok := <-ch:
			if !ok {
				return connLostMsg{}
			}
			return metricsMsg(stat)
		case <-time.After(heartbeatWindow):
			// No metric arrived in time — backend may be down.
			return connLostMsg{}
		}
	}
}

// scheduleReconnect returns a Cmd that fires reconnectTickMsg after reconnectInterval.
func scheduleReconnect() tea.Cmd {
	return tea.Tick(reconnectInterval, func(_ time.Time) tea.Msg {
		return reconnectTickMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		probeCmd(m.targetURL),
		pollStressCmd(),
		waitForActivity(m.ch),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		isOnChangeURL := m.choices[m.cursor] == "Change URL"

		if isOnChangeURL && !m.probing {
			switch msg.String() {
			case "enter":
				newURL := strings.TrimSpace(m.urlInput.Value())
				if newURL == "" {
					newURL = "http://localhost:8080"
				}
				m.probing = true
				m.probeErr = ""
				return m, probeCmd(newURL)
			case "up", "down", "ctrl+c", "esc":
				// Let navigation, escape, or interrupt keys fall through
			default:
				var cmd tea.Cmd
				m.urlInput, cmd = m.urlInput.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			if m.choices[m.cursor] == "Change URL" {
				m.urlInput.Focus()
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
			if m.choices[m.cursor] == "Change URL" {
				m.urlInput.Focus()
			}
		case "enter":
			if m.choices[m.cursor] == "Quit" {
				return m, tea.Quit
			}
		case "c", "m", "d":
			// Stress test controls — only active on the Stress Test tab.
			if m.choices[m.cursor] == "Stress Test" && !m.stressStatus.Active {
				kind := "cpu"
				if msg.String() == "m" {
					kind = "memory"
				} else if msg.String() == "d" {
					kind = "disk"
				}
				return m, startStressCmd(m.targetURL, kind)
			}
				case "x":
			// Stop an active stress test on the Stress Test tab.
			if m.choices[m.cursor] == "Stress Test" && m.stressStatus.Active {
				return m, stopStressCmd(m.targetURL)
			}
		case "s":
			if m.choices[m.cursor] == "Processes" {
				if m.processSortKey == "cpu" {
					m.processSortKey = "memory"
				} else {
					m.processSortKey = "cpu"
				}
				m.sortProcesses()
				return m, nil
			}
		case "o":
			if m.choices[m.cursor] == "Processes" {
				m.processSortAsc = !m.processSortAsc
				m.sortProcesses()
				return m, nil
			}
		}

	case connLostMsg:
		// Only act if we haven't received a metric recently (avoids false positives
		// when a new stream was just started and data hasn't flowed yet).
		if m.connected && time.Since(m.lastMetricAt) >= heartbeatWindow {
			m.connected = false
		}
		// Drain any stale data in the channel so the new stream has a clean slate.
		for len(m.ch) > 0 {
			<-m.ch
		}
		// Re-probe after a short delay instead of hammering the server.
		return m, scheduleReconnect()

	case reconnectTickMsg:
		if !m.connected && !m.probing {
			m.probing = true
			m.probeErr = ""
			// Cancel the existing stream before starting a fresh one.
			if m.streamCancel != nil {
				m.streamCancel()
			}
			return m, probeCmd(m.targetURL)
		}
		// Already probing or reconnected — just keep the heartbeat alive.
		return m, waitForActivity(m.ch)

	case metricsMsg:
		m.metrics = server.CpuStat(msg)
		m.lastMetricAt = time.Now()
		if !m.connected {
			m.connected = true
			m.connectedAt = time.Now()
		}
		// Update analytics synchronously — no allocations beyond the new slice.
		m.hist.Record(history.Snapshot{Timestamp: time.Now(), Stat: m.metrics})
		m.healthScore = health.Compute(m.metrics)
		if newAlerts := m.alertEngine.Evaluate(m.metrics); len(newAlerts) > 0 {
			m.alerts = append(newAlerts, m.alerts...)
			if len(m.alerts) > 100 {
				m.alerts = m.alerts[:100]
			}
		}
		return m, waitForActivity(m.ch)

	case processMsg:
		if msg != nil {
			m.processes = []process.ProcessStat(msg)
			m.sortProcesses()
		}
		return m, waitForProcesses(m.processCh)

	case stressTickMsg:
		return m, fetchStressStatus(m.targetURL)

	case stressMsg:
		m.stressStatus = stress.Status(msg)
		return m, pollStressCmd()

	case probeResultMsg:
		m.probing = false
		if msg.ok {
			// Persist history only when this was a manual URL change (urlInput differs from targetURL).
			if m.urlInput.Value() != "" && m.urlInput.Value() != m.targetURL {
				cfg, err := config.Load()
				if err == nil {
					config.AddHistory(&cfg, m.urlInput.Value())
					_ = config.Save(cfg)
				}
				m.targetURL = m.urlInput.Value()
			}

			if m.streamCancel != nil {
				m.streamCancel()
			}

			ctx, cancel := context.WithCancel(context.Background())
			m.streamCtx = ctx
			m.streamCancel = cancel
			m.connected = false
			m.connectedAt = time.Time{}
			m.lastMetricAt = time.Time{}
			m.metrics = server.CpuStat{} // reset metrics

			m.cursor = 0 // back to Dashboard

			return m, startStreamCmd(m.streamCtx, m.targetURL, m.ch, m.processCh)
		}
		// Probe failed — keep the error visible but schedule an automatic retry
		// so the TUI reconnects as soon as the backend comes back up.
		m.probeErr = msg.err
		return m, scheduleReconnect()


	}
	return m, nil
}

func (m model) View() string {
	// Sidebar
	var sidebar strings.Builder
	for i, choice := range m.choices {
		if m.cursor == i {
			sidebar.WriteString(activeChoiceStyle.Render(choice) + "\n")
		} else {
			sidebar.WriteString(inactiveChoiceStyle.Render(choice) + "\n")
		}
	}

	// Connection status indicator
	var connStatus string
	if m.connected {
		uptime := time.Since(m.connectedAt).Round(time.Second)
		connStatus = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Render("● LIVE") +
			lipgloss.NewStyle().Foreground(textMuted).Render(fmt.Sprintf("  %s  uptime %s", m.targetURL, uptime))
	} else if m.probing {
		connStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4A261")).Bold(true).Render("↺ RECONNECTING") +
			lipgloss.NewStyle().Foreground(textMuted).Render(fmt.Sprintf("  %s", m.targetURL))
	} else {
		connStatus = lipgloss.NewStyle().Foreground(alertColor).Bold(true).Render("○ CONNECTING") +
			lipgloss.NewStyle().Foreground(textMuted).Render(fmt.Sprintf("  %s", m.targetURL))
	}

	// Content
	var content strings.Builder
	if m.metrics.Name == "" && m.choices[m.cursor] != "Change URL" {
		var waitMsg string
		if m.probing {
			waitMsg = fmt.Sprintf("↺  Reconnecting to %s — retrying automatically...", m.targetURL)
		} else if m.probeErr != "" {
			waitMsg = fmt.Sprintf("○  %s  Retrying in %ds...", m.probeErr, int(reconnectInterval.Seconds()))
		} else {
			waitMsg = fmt.Sprintf("○  Connecting to %s...", m.targetURL)
		}
		content.WriteString(
			lipgloss.NewStyle().Foreground(textMuted).Italic(true).Render(waitMsg),
		)
	} else {
		switch m.choices[m.cursor] {
		case "Dashboard":
			content.WriteString(m.renderDashboard())
		case "CPU Details":
			content.WriteString(m.renderCPU())
		case "Memory & Disk":
			content.WriteString(m.renderStorage())
		case "Network":
			content.WriteString(m.renderNetwork())
		case "Processes":
			content.WriteString(renderProcesses(m.processes, m.processSortKey, m.processSortAsc))
		case "Alerts":
			content.WriteString(renderAlerts(m.alerts))
		case "History":
			content.WriteString(renderHistory(m.hist))
		case "Health Score":
			content.WriteString(renderHealthScore(m.healthScore))
		case "Stress Test":
			content.WriteString(renderStressTest(m.stressStatus))
		case "Change URL":
			content.WriteString(m.renderChangeURL())
		}
	}

	app := lipgloss.JoinHorizontal(lipgloss.Top,
		sidebarStyle.Render(sidebar.String()),
		contentStyle.Render(content.String()),
	)

	statusBar := lipgloss.JoinHorizontal(lipgloss.Left,
		connStatus,
		lipgloss.NewStyle().Foreground(textMuted).Render("  │  ↑↓/jk navigate  │  q quit"),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		bannerStyle.Render(banner),
		app,
		"\n"+statusBar,
	)
}

// --- View Helpers ---

func (m model) renderRow(label, value string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		statLabelStyle.Render(label),
		statValueStyle.Render(value),
	) + "\n"
}

func (m model) renderDashboard() string {
	var s strings.Builder

	// 1. Quick Metrics
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("SYSTEM OVERVIEW") + "\n")
	s.WriteString(lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("─", 50)) + "\n")
	
	healthColor := secondaryColor
	if m.healthScore.Overall < 40 {
		healthColor = alertColor
	} else if m.healthScore.Overall < 80 {
		healthColor = lipgloss.Color("#F4A261")
	}
	healthStr := "Waiting..."
	if m.healthScore.Overall > 0 {
		healthStr = fmt.Sprintf("%.1f / 100", m.healthScore.Overall)
	}
	s.WriteString(m.renderRow("Health:", lipgloss.NewStyle().Foreground(healthColor).Bold(true).Render(healthStr)))
	s.WriteString(m.renderRow("CPU Usage:", progressBar(m.metrics.Usage)))
	s.WriteString(m.renderRow("RAM Usage:", progressBar(m.metrics.MemoryUsedPercent)))
	s.WriteString(m.renderRow("Disk Usage:", progressBar(m.metrics.DiskUsedPercent)))
	s.WriteString(m.renderRow("Network:", fmt.Sprintf("%s ↑ / %s ↓", formatBytes(m.metrics.BytesSent), formatBytes(m.metrics.BytesRecv))))
	s.WriteString("\n")

	// 2. Processes (Top 2)
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("TOP PROCESSES") + "\n")
	s.WriteString(lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("─", 50)) + "\n")
	procs := m.processes
	if len(procs) > 2 {
		procs = procs[:2]
	}
	if len(procs) == 0 {
		s.WriteString(lipgloss.NewStyle().Foreground(textMuted).Italic(true).Render("  No process data yet...") + "\n")
	} else {
		for _, p := range procs {
			name := p.Name
			if len(name) > 15 {
				name = name[:12] + "..."
			}
			s.WriteString(fmt.Sprintf("  %-15s %5.1f%% CPU   %8s MEM\n", name, p.CPUPercent, formatBytes(p.MemRSS)))
		}
	}
	s.WriteString("\n")

	// 3. Alerts (Last 2)
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("RECENT ALERTS") + "\n")
	s.WriteString(lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("─", 50)) + "\n")
	alerts := m.alerts
	if len(alerts) > 2 {
		alerts = alerts[:2]
	}
	if len(alerts) == 0 {
		s.WriteString(lipgloss.NewStyle().Foreground(secondaryColor).Render("  ✓ No recent alerts") + "\n")
	} else {
		for _, ev := range alerts {
			c := lipgloss.Color("#F4A261")
			if ev.Severity == alert.SeverityCritical {
				c = alertColor
			}
			badge := lipgloss.NewStyle().Bold(true).Foreground(c).Render(fmt.Sprintf("[%s]", ev.Severity))
			s.WriteString(fmt.Sprintf("  %s %s %s\n", badge, lipgloss.NewStyle().Foreground(textMuted).Render(ev.FiredAt.Format("15:04:05")), ev.Message))
		}
	}
	s.WriteString("\n")

	return s.String()
}

func (m model) renderCPU() string {
	s := m.renderRow("Model:", m.metrics.Name)
	s += m.renderRow("Physical Cores:", fmt.Sprintf("%d", m.metrics.PhysicalCores))
	s += m.renderRow("Logical Cores:", fmt.Sprintf("%d", m.metrics.LogicCores))
	s += m.renderRow("Usage:", progressBar(m.metrics.Usage))
	if m.metrics.Temperature != nil {
		s += m.renderRow("Temperature:", fmt.Sprintf("%.1f °C", *m.metrics.Temperature))
	}
	return s
}

func (m model) renderStorage() string {
	s := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("--- Memory ---") + "\n"
	s += m.renderRow("Total:", formatBytes(m.metrics.TotalMemory))
	s += m.renderRow("Used:", formatBytes(m.metrics.TotalMemory-m.metrics.FreeMemory))
	s += m.renderRow("Free:", formatBytes(m.metrics.FreeMemory))
	s += m.renderRow("Usage:", progressBar(m.metrics.MemoryUsedPercent))

	s += "\n" + lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("--- Disk ---") + "\n"
	s += m.renderRow("Total:", formatBytes(m.metrics.DiskTotal))
	s += m.renderRow("Used:", formatBytes(m.metrics.DiskUsed))
	s += m.renderRow("Free:", formatBytes(m.metrics.DiskFree))
	s += m.renderRow("Usage:", progressBar(m.metrics.DiskUsedPercent))
	return s
}

func (m model) renderNetwork() string {
	s := m.renderRow("Bytes Sent:", formatBytes(m.metrics.BytesSent))
	s += m.renderRow("Bytes Received:", formatBytes(m.metrics.BytesRecv))
	return s
}

// Start initializes and runs the Bubble Tea program with all Robin features.
func Start(targetURL string) error {
	ti := textinput.New()
	ti.Placeholder = "http://localhost:8080"
	ti.CharLimit = 256
	ti.Width = 40
	ti.SetValue(targetURL)

	streamCtx, streamCancel := context.WithCancel(context.Background())

	m := model{
		choices: []string{
			"Dashboard", "CPU Details", "Memory & Disk", "Network",
			"Processes", "Alerts", "History", "Health Score",
			"Stress Test", "Change URL", "Quit",
		},
		ch:             make(chan server.CpuStat),
		processCh:      make(chan []process.ProcessStat),
		hist:           history.NewRingBuffer(300),
		alertEngine:    alert.NewEngine(alert.DefaultRules()),
		targetURL:      targetURL,
		urlInput:       ti,
		streamCtx:      streamCtx,
		streamCancel:   streamCancel,
		processSortKey: "cpu",
		processSortAsc: false,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func startStreamCmd(ctx context.Context, targetURL string, ch chan server.CpuStat, processCh chan []process.ProcessStat) tea.Cmd {
	launch := func() tea.Msg {
		go StreamMetrics(ctx, targetURL, ch)
		go StreamProcesses(ctx, targetURL, processCh)
		return nil
	}
	// Batch: start the goroutines, then immediately seed both channel-wait loops.
	// Without the two waitFor* commands, the BubbleTea event loop never reads
	// from the channels and the dashboard stays blank indefinitely.
	return tea.Batch(launch, waitForActivity(ch), waitForProcesses(processCh))
}

func (m model) renderChangeURL() string {
	var s strings.Builder

	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("CHANGE BACKEND URL") + "\n")
	s.WriteString(lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("─", 50)) + "\n\n")

	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Bold(true).Render("Backend URL:") + "\n")
	s.WriteString(m.urlInput.View() + "\n\n")

	if m.probing {
		s.WriteString(lipgloss.NewStyle().Foreground(primaryColor).Render("  Connecting / Probing...") + "\n")
	} else if m.probeErr != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(alertColor).Bold(true).Render("  ✕ "+m.probeErr) + "\n")
		s.WriteString(lipgloss.NewStyle().Foreground(textMuted).Italic(true).Render("  Please verify the address and ensure the agent is running.") + "\n")
	} else {
		s.WriteString(lipgloss.NewStyle().Foreground(textMuted).Italic(true).Render("  Press Enter to test connection and save  •  ↑/↓ to navigate away") + "\n")
	}

	return s.String()
}

func (m *model) sortProcesses() {
	if len(m.processes) == 0 {
		return
	}
	sort.Slice(m.processes, func(i, j int) bool {
		if m.processSortKey == "memory" {
			if m.processSortAsc {
				return m.processes[i].MemRSS < m.processes[j].MemRSS
			}
			return m.processes[i].MemRSS > m.processes[j].MemRSS
		}
		// Default: CPU
		if m.processSortAsc {
			return m.processes[i].CPUPercent < m.processes[j].CPUPercent
		}
		return m.processes[i].CPUPercent > m.processes[j].CPUPercent
	})
}
