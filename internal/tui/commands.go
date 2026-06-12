package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"Pusher/internal/process"
	"Pusher/internal/stress"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Message types for async TUI updates ---

// processTickMsg triggers a background process list fetch.
type processTickMsg struct{}

// stressTickMsg triggers a local stress status read.
type stressTickMsg struct{}

// processMsg carries a fresh process snapshot back into the event loop.
type processMsg []process.ProcessStat

// stressMsg carries a stress status update back into the event loop.
type stressMsg stress.Status

// --- Polling commands ---

// pollProcessCmd returns a Cmd that fires processTickMsg after 5 seconds.
// pollStressCmd returns a Cmd that fires stressTickMsg after 2 seconds.
func pollStressCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return stressTickMsg{}
	})
}

// waitForProcesses reads the next process list from the SSE channel.
func waitForProcesses(ch <-chan []process.ProcessStat) tea.Cmd {
	return func() tea.Msg {
		procs, ok := <-ch
		if !ok {
			return nil
		}
		return processMsg(procs)
	}
}

// fetchStressStatus fetches the active stress status from the backend.
func fetchStressStatus(targetURL string) tea.Cmd {
	return func() tea.Msg {
		if targetURL == "" {
			return stressMsg{}
		}
		resp, err := http.Get(targetURL + "/stress/status")
		if err != nil {
			return stressMsg{}
		}
		defer resp.Body.Close()
		var status stress.Status
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			return stressMsg{}
		}
		return stressMsg(status)
	}
}

// startStressCmd sends a POST request to the backend to start a stress test.
func startStressCmd(targetURL string, kind string) tea.Cmd {
	return func() tea.Msg {
		if targetURL == "" {
			return nil
		}
		reqBody := fmt.Sprintf(`{"type": "%s", "workers": 4, "duration_seconds": 30}`, kind)
		req, err := http.NewRequest("POST", targetURL+"/stress", strings.NewReader(reqBody))
		if err != nil {
			return nil
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
		return nil
	}
}

// stopStressCmd sends a DELETE request to the backend to stop a stress test.
func stopStressCmd(targetURL string) tea.Cmd {
	return func() tea.Msg {
		if targetURL == "" {
			return nil
		}
		req, err := http.NewRequest("DELETE", targetURL+"/stress", nil)
		if err != nil {
			return nil
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
		return nil
	}
}
