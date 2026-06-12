package tui

import (
	"fmt"
	"strings"

	server "Pusher/internal"

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
			MarginRight(2)

	activeChoiceStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				PaddingLeft(1).
				BorderStyle(lipgloss.NormalBorder()).
				BorderLeft(true).
				BorderForeground(primaryColor)

	inactiveChoiceStyle = lipgloss.NewStyle().
				Foreground(textMuted).
				PaddingLeft(2)

	contentStyle = lipgloss.NewStyle().Width(60)

	statLabelStyle = lipgloss.NewStyle().Foreground(textMuted).Width(15)
	statValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Bold(true)

	bannerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)
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

type model struct {
	metrics server.CpuStat
	choices []string
	cursor  int
	ch      chan server.CpuStat
}

func waitForActivity(ch chan server.CpuStat) tea.Cmd {
	return func() tea.Msg {
		return metricsMsg(<-ch)
	}
}

func (m model) Init() tea.Cmd {
	// Start the SSE listener
	go StreamMetrics(m.ch)
	return waitForActivity(m.ch)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			if m.choices[m.cursor] == "Quit" {
				return m, tea.Quit
			}
		}
	case metricsMsg:
		m.metrics = server.CpuStat(msg)
		return m, waitForActivity(m.ch)
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

	// Content
	var content strings.Builder
	if m.metrics.Name == "" {
		content.WriteString("Waiting for metrics... (Ensure backend is running on :8080)")
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
		}
	}

	app := lipgloss.JoinHorizontal(lipgloss.Top,
		sidebarStyle.Render(sidebar.String()),
		contentStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		bannerStyle.Render(banner),
		app,
		"\n"+lipgloss.NewStyle().Foreground(textMuted).Render("Use ↑/↓ to navigate • Press 'q' or select Quit to exit"),
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
	s := m.renderRow("CPU Usage:", progressBar(m.metrics.Usage))
	s += m.renderRow("Memory Usage:", progressBar(m.metrics.MemoryUsedPercent))
	s += m.renderRow("Disk Usage:", progressBar(m.metrics.DiskUsedPercent))
	s += "\n"
	s += m.renderRow("Net Sent:", formatBytes(m.metrics.BytesSent))
	s += m.renderRow("Net Recv:", formatBytes(m.metrics.BytesRecv))
	return s
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

// Start initializes and runs the Bubble Tea program
func Start() error {
	m := model{
		choices: []string{"Dashboard", "CPU Details", "Memory & Disk", "Network", "Quit"},
		ch:      make(chan server.CpuStat),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
