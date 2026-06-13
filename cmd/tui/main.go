package main

import (
	"flag"
	"fmt"
	"os"

	"Pusher/internal/config"
	"Pusher/internal/tui"
)

func main() {
	// --- Flags ---
	var (
		flagURL   = flag.String("url", "", "Robin backend URL (overrides all other sources)")
		flagSetup = flag.Bool("setup", false, "Force the setup wizard even if a config file exists")
	)
	flag.Parse()

	// --- URL resolution (priority order) ---
	//
	//  1. --url flag        (explicit override, highest priority)
	//  2. ROBIN_URL env var (CI / scripted use)
	//  3. .robin/config.json (saved from a previous run)
	//  4. Interactive setup wizard (first run or --setup)
	//  5. http://localhost:8080 (absolute fallback)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v\n", err)
	}

	var finalURL string

	switch {
	case *flagURL != "":
		finalURL = *flagURL

	case os.Getenv("ROBIN_URL") != "":
		finalURL = os.Getenv("ROBIN_URL")

	case !*flagSetup && cfg.URL != "":
		// Saved config exists and --setup was not requested — use it directly.
		finalURL = cfg.URL

	default:
		// First run or --setup: show the wizard.
		url, ok := tui.RunSetup(cfg)
		if !ok {
			fmt.Println("Exiting.")
			os.Exit(0)
		}
		finalURL = url
	}

	// Persist the chosen URL (only if it came from the wizard or flags, not env).
	if os.Getenv("ROBIN_URL") == "" {
		config.AddHistory(&cfg, finalURL)
		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save config: %v\n", err)
		}
	}

	// Launch the dashboard.
	if err := tui.Start(finalURL); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
