package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	server "Pusher/internal"
	"Pusher/internal/health"
	"Pusher/internal/stress"
)

// healthHandler returns the current system health score as JSON.
// GET /health
func healthHandler(w http.ResponseWriter, r *http.Request) {
	cacheMu.RLock()
	stat := cachedStat
	cacheMu.RUnlock()

	score := health.Compute(stat)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(score); err != nil {
		log.Printf("health encode: %v", err)
	}
}

// processesSSEHandler streams the top processes by CPU usage as SSE.
// GET /processes
func processesSSEHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Cache-Control", "no-cache")

	rc := http.NewResponseController(w)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	sendProcesses := func() error {
		processMu.RLock()
		procs := cachedProcesses
		processMu.RUnlock()

		if len(procs) == 0 {
			return nil
		}
		data, err := json.Marshal(procs)
		if err != nil {
			return nil
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		return rc.Flush()
	}

	// Send immediately on connect
	if err := sendProcesses(); err != nil {
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if err := sendProcesses(); err != nil {
				return
			}
		}
	}
}

// alertsSSEHandler streams alert events as SSE.
// It reads from the shared global cache updated by startSystemMonitor.
// GET /alerts
func alertsSSEHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Cache-Control", "no-cache")

	rc := http.NewResponseController(w)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			cacheMu.RLock()
			events := cachedAlerts
			cacheMu.RUnlock()

			for _, ev := range events {
				data, err := json.Marshal(ev)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
			}
			if len(events) > 0 {
				if err := rc.Flush(); err != nil {
					log.Printf("alerts flush: %v", err)
					return
				}
			}
		}
	}
}


// stressRequest is the POST /stress request body.
type stressRequest struct {
	Kind            string `json:"type"`            // "cpu" | "memory" | "disk"
	Workers         int    `json:"workers"`          // goroutine count; defaults to 4
	DurationSeconds int    `json:"duration_seconds"` // auto-stop duration; defaults to 30
}

// stressHandler dispatches GET (status) and POST (start) for stress testing.
// GET  /stress  → current status
// POST /stress  → start a stress test
func stressHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		stressStatusHandler(w, r)
	case http.MethodPost:
		startStressHandler(w, r)
	case http.MethodDelete:
		stress.Stop()
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// stressStatusHandler returns the current stress test status as JSON.
// GET /stress/status
func stressStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stress.Current()); err != nil {
		log.Printf("stress status encode: %v", err)
	}
}

func startStressHandler(w http.ResponseWriter, r *http.Request) {
	var req stressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Workers <= 0 {
		req.Workers = 4
	}
	if req.DurationSeconds <= 0 {
		req.DurationSeconds = 30
	}

	var kind stress.Type
	switch req.Kind {
	case "memory":
		kind = stress.TypeMemory
	case "disk":
		kind = stress.TypeDisk
	default:
		kind = stress.TypeCPU
	}

	if err := stress.Run(context.Background(), stress.Config{
		Kind:     kind,
		Workers:  req.Workers,
		Duration: time.Duration(req.DurationSeconds) * time.Second,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// collectSnapshot assembles a full CpuStat snapshot using the existing
// service layer functions. The CPU measurement blocks for 100ms.
// Called on-demand per request; not shared across handlers.
func collectSnapshot() server.CpuStat {
	var (
		m  server.Metrics
		wg sync.WaitGroup
	)

	server.GetMemoryUsage(&m)
	server.GetNetUsage(&m)
	server.GetDiskUsage(&m)
	server.GetCpuName(context.Background(), &m)
	server.GetcpuTemp(&m)
	server.GetCpuLCores(context.Background(), &m)
	server.GetCpuPCores(context.Background(), &m)

	wg.Add(1)
	go func() {
		server.CpuUsage(&wg, &m)
		wg.Done()
	}()
	wg.Wait()

	return server.CpuStat{
		Name:              m.CpuName,
		LogicCores:        m.CpuLcores,
		PhysicalCores:     m.CpuPcores,
		Usage:             m.CpuUsage,
		Temperature:       m.CpuTemp,
		MemoryUsedPercent: m.UsedPercent,
		TotalMemory:       m.Total,
		FreeMemory:        m.Free,
		BytesSent:         m.BytesSent,
		BytesRecv:         m.BytesRecv,
		DiskTotal:         m.DiskTotal,
		DiskUsed:          m.DiskUsed,
		DiskFree:          m.DiskFree,
		DiskUsedPercent:   m.DiskUsedPercent,
	}
}
