package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const maxHistory = 5

// Config holds the persisted TUI configuration.
type Config struct {
	URL     string        `json:"url"`
	History []HistoryItem `json:"history,omitempty"`
}

// HistoryItem is a single past connection.
type HistoryItem struct {
	URL      string    `json:"url"`
	LastUsed time.Time `json:"last_used"`
}

// configPath returns the path to .robin/config.json in the current directory.
func configPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".robin", "config.json"), nil
}

// Load reads the config file. Returns an empty Config (not an error) if the
// file does not exist yet (first run).
func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil // first run — no config yet
	}
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Save writes cfg to disk, creating the directory if needed.
func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// AddHistory prepends url to cfg.History, deduplicates, and caps at maxHistory.
// It also sets cfg.URL = url.
func AddHistory(cfg *Config, url string) {
	cfg.URL = url

	// Remove any existing entry for this URL so it moves to the front.
	filtered := cfg.History[:0]
	for _, h := range cfg.History {
		if h.URL != url {
			filtered = append(filtered, h)
		}
	}

	cfg.History = append([]HistoryItem{{URL: url, LastUsed: time.Now()}}, filtered...)

	if len(cfg.History) > maxHistory {
		cfg.History = cfg.History[:maxHistory]
	}
}
