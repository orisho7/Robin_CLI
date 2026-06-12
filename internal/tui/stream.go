package tui

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"

	server "Pusher/internal"
)

// StreamMetrics connects to the SSE endpoint and streams metrics into a channel
func StreamMetrics(ch chan<- server.CpuStat) {
	resp, err := http.Get("http://localhost:8080/pusher")
	if err != nil {
		return // Ideally, we'd pass this error back to the UI
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			jsonStr := strings.TrimPrefix(line, "data: ")
			jsonStr = strings.TrimSpace(jsonStr)

			var stat server.CpuStat
			if err := json.Unmarshal([]byte(jsonStr), &stat); err == nil {
				ch <- stat
			}
		}
	}
}
