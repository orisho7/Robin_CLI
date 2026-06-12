package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	server "Pusher/internal"
	"Pusher/internal/alert"
	"Pusher/internal/history"
	"Pusher/internal/process"
	"Pusher/internal/tunnel"
)

// Package-level singletons shared by all route handlers.
var (
	alertEngine *alert.Engine
	histBuffer  *history.RingBuffer

	// Global cache for low CPU usage
	cachedStat      server.CpuStat
	cachedAlerts    []alert.Event
	cacheMu         sync.RWMutex
	cachedProcesses []process.ProcessStat
	processMu       sync.RWMutex
)

func main() {
	// Initialize analytics and monitoring subsystems
	alertEngine = alert.NewEngine(alert.DefaultRules())
	histBuffer = history.NewRingBuffer(300) // 5-minute rolling window at 1 sample/s

	// Start the central system monitor
	go startSystemMonitor()
	go startProcessMonitor()

	// Original routes
	http.HandleFunc("/pusher", sseHandler)
	http.HandleFunc("/hello", handlehello)

	// New feature routes
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/alerts", alertsSSEHandler)
	http.HandleFunc("/processes", processesSSEHandler)
	http.HandleFunc("/stress", stressHandler)
	http.HandleFunc("/stress/status", stressStatusHandler)

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	// Interactively ask for tunnel if .env doesn't exist (first run)
	tunnelEnabled := false
	customURL := ""

	envData, err := os.ReadFile(".env")
	if err == nil {
		content := string(envData)
		if strings.Contains(content, "ROBIN_TUNNEL=1") {
			tunnelEnabled = true
		}
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "CUSTOM_URL=") {
				customURL = strings.TrimSpace(strings.SplitN(line, "=", 2)[1])
			}
		}
	} else if os.Getenv("ROBIN_TUNNEL") == "1" {
		tunnelEnabled = true
	} else {
		fmt.Print("\n\033[1;36mDo you want to enable a public LocalTunnel URL for remote access? (y/N):\033[0m ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))

		envContent := ""
		if answer == "y" || answer == "yes" {
			tunnelEnabled = true
			envContent = "ROBIN_TUNNEL=1\n"
		} else {
			envContent = "ROBIN_TUNNEL=0\n"
		}
		envContent += "# To use a custom domain (e.g. Nginx proxy), uncomment and set the line below:\n"
		envContent += "# CUSTOM_URL=https://monitor.yourdomain.com\n"
		os.WriteFile(".env", []byte(envContent), 0644)
		fmt.Println()
	}

	fmt.Println("\n\033[1;32m=== ROBIN BACKEND AGENT ===\033[0m")
	fmt.Println("Server listening on :8080\n")

	if customURL != "" {
		fmt.Printf("\033[1;34mCustom URL:\033[0m %s\n", customURL)
		fmt.Println("\nRun the TUI on another machine and enter this URL:")
		fmt.Printf("  \033[1m%s\033[0m\n\n", customURL)
	} else if tunnelEnabled {
		fmt.Println("\033[1;34mLocal:\033[0m       http://localhost:8080")
		go tunnel.Start(serverCtx, 8080, func(publicURL string) {
			fmt.Printf("\033[1;34mTunnel:\033[0m      %s\n", publicURL)
			fmt.Println("\nRun the TUI on another machine and enter this URL:")
			fmt.Printf("  \033[1m%s\033[0m\n\n", publicURL)
		})
	} else {
		fmt.Println("\033[1;34mLocal:\033[0m       http://localhost:8080\n")
		fmt.Println("\033[90m(Tip: You can edit the .env file to enable LocalTunnel or set a CUSTOM_URL)\033[0m\n")
	}

	log.Fatal(http.ListenAndServe(":8080", nil))
}


func handlehello(w http.ResponseWriter, r *http.Request) {
	wc, err := w.Write([]byte("Hello World!"))
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(wc)
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Cache-Control", "no-cache")

	rc := http.NewResponseController(w)
	t := time.NewTicker(time.Second)
	defer t.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-t.C:
			cacheMu.RLock()
			stat := cachedStat
			cacheMu.RUnlock()

			if stat.Name == "" {
				continue // wait until first tick is ready
			}

			data, err := json.Marshal(stat)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}

// startSystemMonitor runs in the background and collects system metrics exactly
// once per second. This prevents multiple SSE clients from causing duplicate
// CPU-intensive system calls.
func startSystemMonitor() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		stat := collectSnapshot()
		events := alertEngine.Evaluate(stat)

		cacheMu.Lock()
		cachedStat = stat
		cachedAlerts = events
		cacheMu.Unlock()

		histBuffer.Record(history.Snapshot{Timestamp: time.Now(), Stat: stat})
	}
}

// startProcessMonitor runs in the background and collects process lists every 5 seconds.
func startProcessMonitor() {
	update := func() {
		cpuProcs, err1 := process.TopByCPU(30)
		memProcs, err2 := process.TopByMemory(30)
		if err1 == nil && err2 == nil {
			// Merge and deduplicate by PID
			merged := make(map[int32]process.ProcessStat)
			for _, p := range cpuProcs {
				merged[p.PID] = p
			}
			for _, p := range memProcs {
				merged[p.PID] = p
			}

			procs := make([]process.ProcessStat, 0, len(merged))
			for _, p := range merged {
				procs = append(procs, p)
			}

			processMu.Lock()
			cachedProcesses = procs
			processMu.Unlock()
		}
	}

	update()

	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		update()
	}
}
