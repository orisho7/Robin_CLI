# TUI UX Improvements

## Original Ideas (from audio note)

1. **Interactive URL Prompt on Startup** — If `ROBIN_URL` is not set, prompt the user interactively instead of silently defaulting to localhost.
2. **Persistent Config (`.env` / config file)** — Save the entered URL so it is auto-loaded on next run.

---

## Enhanced Plan (to be implemented)

### 1. Config File at `~/.robin/config.json`
Store URL and recent history as structured JSON, not a `.env` file.
- Proper Go struct, `encoding/json` serialization.
- Location: `os.UserHomeDir() + "/.robin/config.json"`.
- Fields: `url`, `history []string` (last 5 URLs used).

### 2. Full-Screen Styled Setup Wizard (Bubble Tea)
A styled first-run screen built as its own Bubble Tea `Model` using `charmbracelet/bubbles/textinput`:
- Shows the Robin ASCII banner.
- Shows a styled text input: `Backend URL:` pre-filled with `http://localhost:8080`.
- Shows recent history (if any) with numbers the user can press to select instantly.
- Enter confirms, Esc quits.
- After confirming URL, runs a live connection test (`GET /pusher`) with a spinner.

### 3. Connection Test Before Launching Dashboard
After the URL is entered or loaded from config:
- Make a test HTTP request to the backend with a 5-second timeout.
- If it fails, show a styled error with retry/edit options — do not launch the broken dashboard.
- If it succeeds, save the URL to config and transition into the main TUI.

### 4. `--setup` / `-s` Flag
A CLI flag that forces the setup wizard even if a saved URL exists. Useful for switching between servers.

### 5. Priority Order for URL Resolution
```
1. --url flag (explicit override, highest priority)
2. ROBIN_URL env var (CI/scripted use)
3. ~/.robin/config.json (saved from last run)
4. Interactive setup wizard (first run / no config)
5. http://localhost:8080 (absolute fallback if wizard is skipped)
```

### 6. Connection History in Setup Screen
In the setup wizard, show the last 5 used URLs as quick-select options:
```
 Recent connections:
  [1] http://192.168.1.10:8080  (3 hours ago)
  [2] https://xyz.loca.lt       (yesterday)
  [3] http://localhost:8080     (last week)
```
Pressing `1`–`5` fills the input field instantly.

---

## Files to Create / Modify

| File | Change |
|---|---|
| `internal/config/config.go` | [NEW] Config struct, Load(), Save(), AddHistory() |
| `internal/tui/setup.go` | [NEW] Setup wizard Bubble Tea model |
| `cmd/tui/main.go` | [MODIFY] Add flag parsing, config loading, setup flow |
| `go.mod` | [MODIFY] Add `charmbracelet/bubbles` for textinput |
