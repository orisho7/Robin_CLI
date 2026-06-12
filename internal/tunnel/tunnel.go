package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// Start launches a LocalTunnel process via npx in the background and calls
// onURL exactly once with the public HTTPS URL when the tunnel is ready.
//
// The tunnel process is stopped automatically when ctx is cancelled (e.g. on
// server shutdown). If npx is not on PATH, a warning is logged and the function
// returns immediately so the server continues normally without tunneling.
//
// Typical usage:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	go tunnel.Start(ctx, 8080, func(url string) {
//	    log.Printf("Public tunnel URL: %s", url)
//	})
func Start(ctx context.Context, port int, onURL func(string)) {
	if _, err := exec.LookPath("npx"); err != nil {
		log.Println("[tunnel] npx not found on PATH — LocalTunnel unavailable.")
		log.Println("[tunnel] Install Node.js (https://nodejs.org) to enable public tunneling.")
		return
	}

	cmd := exec.CommandContext(ctx,
		"npx", "--yes", "localtunnel",
		"--port", fmt.Sprintf("%d", port),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[tunnel] stdout pipe: %v", err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("[tunnel] stderr pipe: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[tunnel] start failed: %v", err)
		return
	}
	log.Println("[tunnel] LocalTunnel starting... (this may take a few seconds)")

	// Drain stderr so the localtunnel process never blocks on a full pipe.
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			// Only surface lines that look useful (npx install noise is noisy).
			line := s.Text()
			if strings.Contains(line, "error") || strings.Contains(line, "warn") {
				log.Printf("[tunnel] %s", line)
			}
		}
	}()

	// localtunnel prints exactly one line to stdout once the tunnel is ready:
	// "your url is: https://<random>.loca.lt"
	urlFound := false
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if !urlFound {
			if idx := strings.Index(line, "https://"); idx != -1 {
				publicURL := strings.TrimSpace(line[idx:])
				// Strip any trailing punctuation localtunnel may append.
				publicURL = strings.TrimRight(publicURL, ".,;")
				urlFound = true
				onURL(publicURL)
			}
		}
		// Keep draining stdout so the process doesn't stall.
	}

	// cmd.Wait() is called implicitly when ctx is cancelled and the process exits.
	cmd.Wait()
}
