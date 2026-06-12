package main

import (
	"fmt"
	"os"

	"Pusher/internal/tui"
)

func main() {
	if err := tui.Start(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
