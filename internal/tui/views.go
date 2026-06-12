package tui

import (
	"fmt"
	"strings"
	"time"

	"Pusher/internal/alert"
	"Pusher/internal/health"
	"Pusher/internal/history"
	"Pusher/internal/process"
	"Pusher/internal/stress"

	"github.com/charmbracelet/lipgloss"
)

// renderProcesses renders the process inspector tab.
func renderProcesses(procs []process.ProcessStat, sortKey string, sortAsc bool) string {
	sortName := "CPU"
	if sortKey == "memory" {
		sortName = "MEMORY"
	}
	orderName := "DESC"
	if sortAsc {
		orderName = "ASC"
	}

	s := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(
		fmt.Sprintf("TOP PROCESSES — BY %s (%s)", sortName, orderName),
	) + "\n"
	s += lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("─", 58)) + "\n"

	if len(procs) == 0 {
		return s + lipgloss.NewStyle().Foreground(textMuted).Italic(true).Render("Collecting process data...") + "\n"
	}

	// Truncate to top 15
	if len(procs) > 15 {
		procs = procs[:15]
	}

	s += lipgloss.NewStyle().Foreground(textMuted).Render(
		fmt.Sprintf("  %-7s %-18s %7s  %-14s %8s", "PID", "NAME", "CPU%", "", "MEM"),
	) + "\n"

	for _, p := range procs {
		name := p.Name
		if len(name) > 17 {
			name = name[:14] + "..."
		}

		cpuColor := secondaryColor
		if p.CPUPercent > 80 {
			cpuColor = alertColor
		} else if p.CPUPercent > 40 {
			cpuColor = lipgloss.Color("#F4A261")
		}

		// Mini inline CPU bar (14 chars wide)
		barWidth := 14
		filled := int(p.CPUPercent / 100.0 * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		miniBar := lipgloss.NewStyle().Foreground(cpuColor).Render(strings.Repeat("█", filled)) +
			lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("·", barWidth-filled))

		s += fmt.Sprintf("  %-7d %-18s ", p.PID, name)
		s += lipgloss.NewStyle().Foreground(cpuColor).Bold(true).Render(fmt.Sprintf("%6.1f%%", p.CPUPercent))
		s += "  " + miniBar
		s += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Render(fmt.Sprintf("%8s", formatBytes(p.MemRSS)))
		s += "\n"
	}
	s += "\n" + lipgloss.NewStyle().Foreground(textMuted).Render("  [s] toggle CPU/MEM sort  •  [o] toggle ASC/DESC order") + "\n"
	return s
}

// renderAlerts renders the alert event log tab.
func renderAlerts(events []alert.Event) string {
	s := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("ALERT EVENTS") + "\n"
	s += lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("─", 50)) + "\n"

	if len(events) == 0 {
		s += lipgloss.NewStyle().Foreground(secondaryColor).Render("✓  No alerts — all metrics within thresholds") + "\n"
		return s
	}

	for _, ev := range events {
		ts := ev.FiredAt.Format("15:04:05")

		badgeStyle := lipgloss.NewStyle().Bold(true)
		if ev.Severity == alert.SeverityCritical {
			badgeStyle = badgeStyle.Foreground(alertColor)
		} else {
			badgeStyle = badgeStyle.Foreground(lipgloss.Color("#F4A261"))
		}

		s += badgeStyle.Render(fmt.Sprintf("[%-8s]", ev.Severity))
		s += " " + lipgloss.NewStyle().Foreground(textMuted).Render(ts)
		s += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Render(ev.Message)
		s += "\n"
	}
	return s
}

// renderHistory renders the metric history tab with ASCII sparklines.
func renderHistory(buf *history.RingBuffer) string {
	s := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("METRIC HISTORY — LAST 60s") + "\n"
	s += lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("─", 50)) + "\n\n"

	const sparkW = 40
	cpuVals := buf.TrendCPU(sparkW)
	memVals := buf.TrendMemory(sparkW)
	dskVals := buf.TrendDisk(sparkW)

	label := lipgloss.NewStyle().Foreground(textMuted).Width(10)

	row := func(name string, vals []float64, c lipgloss.Color) string {
		avg := average(vals)
		spark := lipgloss.NewStyle().Foreground(c).Render(sparkline(vals, sparkW))
		avgStr := lipgloss.NewStyle().Foreground(textMuted).Render(fmt.Sprintf("  avg %.1f%%", avg))
		return label.Render(name) + spark + avgStr + "\n"
	}

	s += row("CPU", cpuVals, secondaryColor)
	s += row("Memory", memVals, lipgloss.Color("#7EC8E3"))
	s += row("Disk", dskVals, lipgloss.Color("#F4A261"))

	if len(cpuVals) == 0 {
		s += "\n" + lipgloss.NewStyle().Foreground(textMuted).Italic(true).Render("Waiting for first samples...") + "\n"
	}
	return s
}

// renderHealthScore renders the overall and per-component health score tab.
func renderHealthScore(score health.Score) string {
	s := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("SYSTEM HEALTH SCORE") + "\n"
	s += lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("─", 50)) + "\n\n"

	if score.Overall == 0 {
		return s + lipgloss.NewStyle().Foreground(textMuted).Italic(true).Render("Waiting for first sample...") + "\n"
	}

	scoreColor := secondaryColor
	label := "EXCELLENT"
	switch {
	case score.Overall < 40:
		scoreColor = alertColor
		label = "CRITICAL"
	case score.Overall < 60:
		scoreColor = lipgloss.Color("#E07020")
		label = "POOR"
	case score.Overall < 80:
		scoreColor = lipgloss.Color("#F4A261")
		label = "FAIR"
	}

	s += "   " + lipgloss.NewStyle().Bold(true).Foreground(scoreColor).
		Render(fmt.Sprintf("%.1f / 100", score.Overall)) + "\n"
	s += "   " + lipgloss.NewStyle().Foreground(scoreColor).Render(label) + "\n\n"

	comp := func(name string, val float64) string {
		c := secondaryColor
		if val < 40 {
			c = alertColor
		} else if val < 60 {
			c = lipgloss.Color("#F4A261")
		}
		// Small visual bar (20 chars) for each component
		barWidth := 20
		filled := int(val / 100.0 * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("█", filled)) +
			lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("░", barWidth-filled))
		return lipgloss.NewStyle().Foreground(textMuted).Width(14).Render(name) +
			lipgloss.NewStyle().Foreground(c).Bold(true).Width(7).Render(fmt.Sprintf("%.1f", val)) +
			" " + bar + "\n"
	}

	s += comp("CPU Score:", score.CPU)
	s += comp("Memory Score:", score.Memory)
	s += comp("Disk Score:", score.Disk)
	s += comp("Temp Score:", score.Temp)
	return s
}


// renderStressTest renders the stress test control tab.
func renderStressTest(st stress.Status) string {
	s := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("STRESS TEST") + "\n"
	s += lipgloss.NewStyle().Foreground(textMuted).Render(strings.Repeat("─", 50)) + "\n\n"

	if !st.Active {
		s += lipgloss.NewStyle().Foreground(secondaryColor).Render("Status:  IDLE") + "\n\n"
		s += lipgloss.NewStyle().Foreground(textMuted).Render("Controls:") + "\n"
		s += "  " + lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("[c]") +
			"  Start CPU stress    (4 goroutines × 30s)\n"
		s += "  " + lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("[m]") +
			"  Start Memory stress (64 MiB/worker × 30s)\n"
		s += "  " + lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("[d]") +
			"  Start Disk I/O stress\n"
		return s
	}

	elapsed := time.Since(st.StartedAt)
	total := st.EndsAt.Sub(st.StartedAt)
	remaining := time.Until(st.EndsAt)
	if remaining < 0 {
		remaining = 0
	}
	pct := 0.0
	if total > 0 {
		pct = float64(elapsed) / float64(total) * 100
	}

	s += lipgloss.NewStyle().Foreground(alertColor).Bold(true).Render("Status:  RUNNING — "+st.Kind) + "\n"
	s += lipgloss.NewStyle().Foreground(textMuted).Render(fmt.Sprintf("Started:   %s", st.StartedAt.Format("15:04:05"))) + "\n"
	s += lipgloss.NewStyle().Foreground(textMuted).Render(fmt.Sprintf("Ends:      %s", st.EndsAt.Format("15:04:05"))) + "\n"
	s += lipgloss.NewStyle().Foreground(textMuted).Render(fmt.Sprintf("Remaining: %ds", int(remaining.Seconds()))) + "\n"
	s += "\n" + progressBar(pct) + "\n\n"
	s += "  " + lipgloss.NewStyle().Foreground(alertColor).Bold(true).Render("[x]") +
		"  Stop stress test\n"
	return s
}

// --- Shared helpers ---

// sparkline converts a slice of 0–100 percentage values into a string of
// Unicode block characters (▁–█), left-padded with spaces if shorter than width.
func sparkline(values []float64, width int) string {
	blocks := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	if len(values) == 0 {
		return strings.Repeat("─", width)
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}
	var sb strings.Builder
	// Left-pad with spaces so older data aligns to the right.
	for i := len(values); i < width; i++ {
		sb.WriteString(" ")
	}
	for _, v := range values {
		idx := int(v / 100.0 * float64(len(blocks)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		sb.WriteString(blocks[idx])
	}
	return sb.String()
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
