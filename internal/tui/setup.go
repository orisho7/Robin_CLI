package tui

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"Pusher/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ----- Styles ---------------------------------------------------------------

var (
	setupBannerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				Padding(1, 4)

	setupBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 3).
			Width(60)

	setupLabelStyle  = lipgloss.NewStyle().Foreground(textMuted).Bold(true)
	setupOkStyle     = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
	setupErrStyle    = lipgloss.NewStyle().Foreground(alertColor).Bold(true)
	setupMutedStyle  = lipgloss.NewStyle().Foreground(textMuted)
	setupSelectStyle = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	setupHintStyle   = lipgloss.NewStyle().Foreground(textMuted).Italic(true)
)

const setupBanner = `
  ██████╗  ██████╗ ██████╗ ██╗███╗  ██╗
  ██╔══██╗██╔═══██╗██╔══██╗██║████╗ ██║
  ██████╔╝██║   ██║██████╔╝██║██╔██╗██║
  ██╔══██╗██║   ██║██╔══██╗██║██║╚████║
  ██║  ██║╚██████╔╝██████╔╝██║██║ ╚███║
  ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝╚═╝  ╚══╝
  System Observability Agent`

// ----- Message types --------------------------------------------------------

type probeResultMsg struct {
	ok  bool
	err string
}

// ----- Setup model ----------------------------------------------------------

// SetupModel is a standalone Bubble Tea model for the first-run / --setup flow.
// It collects the backend URL, optionally shows recent history, and runs a
// live connection probe before handing off to the main dashboard.
type SetupModel struct {
	input   textinput.Model
	history []config.HistoryItem

	// state machine
	probing bool
	probeOK bool
	probeErr string
	done    bool
	quit    bool

	// final URL selected by the user
	ChosenURL string
}

// NewSetupModel constructs the setup wizard pre-filled with the last used URL.
func NewSetupModel(cfg config.Config) SetupModel {
	ti := textinput.New()
	ti.Placeholder = "http://localhost:8080"
	ti.CharLimit = 256
	ti.Width = 50
	ti.Focus()

	if cfg.URL != "" {
		ti.SetValue(cfg.URL)
	}

	return SetupModel{
		input:   ti,
		history: cfg.History,
	}
}

func (m SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		// Quit
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
			m.quit = true
			return m, tea.Quit
		}

		// Confirm URL and start probing
		if msg.Type == tea.KeyEnter && !m.probing {
			url := strings.TrimSpace(m.input.Value())
			if url == "" {
				url = "http://localhost:8080"
			}
			m.ChosenURL = url
			m.probing = true
			m.probeErr = ""
			return m, probeCmd(url)
		}

		// Quick-select from history: keys 1–5
		if !m.probing && msg.Type == tea.KeyRunes {
			if len(msg.Runes) == 1 {
				r := msg.Runes[0]
				if r >= '1' && r <= '5' {
					idx := int(r-'1')
					if idx < len(m.history) {
						m.input.SetValue(m.history[idx].URL)
						return m, nil
					}
				}
			}
		}

	case probeResultMsg:
		m.probing = false
		if msg.ok {
			m.probeOK = true
			m.done = true
			return m, tea.Quit
		}
		m.probeErr = msg.err
	}

	// Delegate to the text input while not probing.
	if !m.probing {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m SetupModel) View() string {
	var b strings.Builder

	b.WriteString(setupBannerStyle.Render(setupBanner))
	b.WriteString("\n\n")

	var inner strings.Builder

	// URL input
	inner.WriteString(setupLabelStyle.Render("Backend URL") + "\n")
	inner.WriteString(m.input.View() + "\n\n")

	// Status line
	if m.probing {
		inner.WriteString(setupMutedStyle.Render("  Connecting...") + "\n\n")
	} else if m.probeErr != "" {
		inner.WriteString(setupErrStyle.Render("  ✕ "+m.probeErr) + "\n")
		inner.WriteString(setupHintStyle.Render("  Edit the URL and press Enter to retry.") + "\n\n")
	} else {
		inner.WriteString(setupHintStyle.Render("  Press Enter to connect  •  Esc to quit") + "\n\n")
	}

	// Recent history
	if len(m.history) > 0 {
		inner.WriteString(setupLabelStyle.Render("Recent connections") + "\n")
		for i, h := range m.history {
			if i >= 5 {
				break
			}
			age := formatAge(time.Since(h.LastUsed))
			inner.WriteString(fmt.Sprintf(
				"  %s  %-40s  %s\n",
				setupSelectStyle.Render(fmt.Sprintf("[%d]", i+1)),
				h.URL,
				setupMutedStyle.Render(age),
			))
		}
	}

	b.WriteString(setupBoxStyle.Render(inner.String()))
	b.WriteString("\n")
	return b.String()
}

// probeCmd attempts a GET /pusher against url with a 10s timeout.
func probeCmd(url string) tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url + "/pusher")
		if err != nil {
			return probeResultMsg{ok: false, err: err.Error()}
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return probeResultMsg{ok: true}
		}
		return probeResultMsg{ok: false, err: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
}

type bypassTransport struct {
	base http.RoundTripper
}

func (t *bypassTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Bypass-Tunnel-Reminder", "true")
	req.Header.Set("ngrok-skip-browser-warning", "true")
	return t.base.RoundTrip(req)
}

func init() {
	http.DefaultTransport = &bypassTransport{
		base: http.DefaultTransport,
	}
}

// RunSetup runs the setup wizard and returns the chosen URL.
// Returns ("", false) if the user quit.
func RunSetup(cfg config.Config) (string, bool) {
	m := NewSetupModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return "", false
	}
	final := result.(SetupModel)
	if final.quit || !final.done {
		return "", false
	}
	return final.ChosenURL, true
}

// formatAge returns a human-readable duration string.
func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
