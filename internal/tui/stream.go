package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	server "Pusher/internal"
	"Pusher/internal/process"
)

// StreamMetrics connects to the SSE endpoint and streams metrics into a channel.
// It automatically reconnects if the connection drops or the server is unavailable.
// It exits cleanly when the provided context is cancelled.
func StreamMetrics(ctx context.Context, targetURL string, ch chan<- server.CpuStat) {
	endpoint := targetURL + "/pusher"
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		// Watch context and close response body on cancellation to unblock Scan()
		bodyClosed := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				resp.Body.Close()
			case <-bodyClosed:
			}
		}()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				jsonStr := strings.TrimPrefix(line, "data: ")
				jsonStr = strings.TrimSpace(jsonStr)

				var stat server.CpuStat
				if err := json.Unmarshal([]byte(jsonStr), &stat); err == nil {
					select {
					case <-ctx.Done():
						close(bodyClosed)
						resp.Body.Close()
						return
					case ch <- stat:
					}
				}
			}
		}
		close(bodyClosed)
		resp.Body.Close()

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// StreamProcesses connects to the /processes SSE endpoint and streams process stats.
func StreamProcesses(ctx context.Context, targetURL string, ch chan<- []process.ProcessStat) {
	endpoint := targetURL + "/processes"
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		bodyClosed := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				resp.Body.Close()
			case <-bodyClosed:
			}
		}()

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				jsonStr := strings.TrimPrefix(line, "data: ")
				jsonStr = strings.TrimSpace(jsonStr)

				var procs []process.ProcessStat
				if err := json.Unmarshal([]byte(jsonStr), &procs); err == nil {
					select {
					case <-ctx.Done():
						close(bodyClosed)
						resp.Body.Close()
						return
					case ch <- procs:
					}
				}
			}
		}
		close(bodyClosed)
		resp.Body.Close()

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

