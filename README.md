# Robin

Robin is a lightweight, real-time observability agent and CLI dashboard for Linux systems. It streams live system metrics from a backend agent to an interactive terminal UI via Server-Sent Events (SSE), and supports monitoring both local and remote servers with zero-config public tunneling.

## Features

### Core Monitoring

- **Real-time Dashboard** — CPU, Memory, Disk and Network I/O at a glance
- **Process Inspector** — Top 15 processes by CPU with inline usage bars, refreshed every 2 seconds
- **Alert Engine** — Threshold-based alerts (CPU > 85%, Memory > 90%, etc.) with WARN/CRITICAL severity
- **Metric History** — 5-minute rolling buffer with ASCII sparklines for CPU, Memory, and Disk trends

### Operational Tools

- **Health Score** — Single 0–100 score weighted across CPU (40%), Memory (30%), Disk (20%), Temperature (10%)
- **Stress Test** — Pure-Go CPU, Memory, and Disk stressors to validate system behaviour under load

### Reliability & UX

- **Auto-Reconnect** — TUI reconnects to the backend automatically if it restarts
- **Setup Wizard** — On first run, a full-screen styled wizard prompts for the backend URL, tests connectivity, and saves the result
- **Persistent Config** — Backend URL saved to `.robin/config.json`; no need to type it again
- **Connection History** — Quick-select from last 5 used URLs with human-readable timestamps
- **LocalTunnel** — One-flag public HTTPS URL via `ROBIN_TUNNEL=1`

### Planned Features

- **Service Monitor** — `systemctl`-based health checks
- **Log Monitor** — Live remote streaming of system logs

---

## Architecture

```
┌─────────────────────────────────┐      SSE stream      ┌───────────────────────────┐
│   Backend Agent  (cmd/)         │ ──────────────────►  │   TUI Dashboard (cmd/tui/) │
│                                 │                       │                            │
│  main.go      — wires & starts  │  /pusher  metrics     │  internal/tui/tui.go       │
│  routes.go    — HTTP handlers   │  /alerts  events      │  internal/tui/setup.go     │
│                                 │                       │  internal/tui/views.go     │
│  internal/                      │                       │  internal/tui/stream.go    │
│    service.go — gopsutil calls  │◄──────────────────    │                            │
│    alert/     — rule engine     │  REST calls           └───────────────────────────┘
│    health/    — score formula   │  /health /processes
│    history/   — ring buffer     │  /stress
│    process/   — /proc reader    │
│    stress/    — load generator  │
│    tunnel/    — localtunnel     │
└─────────────────────────────────┘
```

---

## Getting Started

### Prerequisites

- Go 1.22 or higher
- Linux
- Node.js (optional — only needed for `ROBIN_TUNNEL=1` public tunneling)

### 1. Start the Backend Agent

```bash
cd cmd
go run .
```

> **Note:** Use `go run .` (not `go run main.go`) — the backend spans multiple files in the same `main` package.

The server starts on `:8080` and prints your LAN address and a tunnel hint:

```
Server starting on :8080
------------------------------------------------------
LAN:    http://192.168.1.10:8080
Tip: set ROBIN_TUNNEL=1 to expose a public tunnel URL
To monitor from another machine:
  ROBIN_URL=http://192.168.1.10:8080 go run .
------------------------------------------------------
```

### 2. Start the TUI Dashboard

```bash
cd cmd/tui
go run .
```

**First run:** A full-screen setup wizard appears, asks for the backend URL, and runs a live connection test. The URL is saved to `.robin/config.json` — next time the TUI starts instantly.

**Subsequent runs:** Connect immediately using the saved URL with no input needed.

---

## TUI Connection — URL Priority Order

The TUI resolves the backend URL in this order:

| Priority | Source                 | Example                               |
| -------- | ---------------------- | ------------------------------------- |
| 1        | `--url` flag           | `go run . --url http://10.0.0.5:8080` |
| 2        | `ROBIN_URL` env var    | `ROBIN_URL=http://... go run .`       |
| 3        | `.robin/config.json` | Saved from previous run               |
| 4        | Setup wizard           | First-run interactive prompt          |
| 5        | Fallback               | `http://localhost:8080`               |

To force the setup wizard (e.g., to switch servers):

```bash
go run . --setup
```

---

## TUI Navigation

| Key            | Action                                               |
| -------------- | ---------------------------------------------------- |
| `↑` / `k`      | Move up the sidebar                                  |
| `↓` / `j`      | Move down the sidebar                                |
| `q` / `Ctrl+C` | Quit                                                 |
| `c`            | Start CPU stress test _(Stress Test tab only)_       |
| `m`            | Start Memory stress test _(Stress Test tab only)_    |
| `d`            | Start Disk stress test _(Stress Test tab only)_      |
| `x`            | Stop the active stress test _(Stress Test tab only)_ |

---

## Remote Monitoring

### Direct LAN Connection

Copy the URL printed by the agent at startup and enter it in the TUI setup wizard.

### Via LocalTunnel (Zero-Config Public URL)

On its first run, the backend agent will interactively ask if you want to enable a public LocalTunnel URL.

```
Do you want to enable a public LocalTunnel URL for remote access? (y/N): y

Server starting on :8080
------------------------------------------------------
LAN:    http://192.168.1.10:8080
------------------------------------------------------
Tunnel: https://funny-name-xyz.loca.lt
------------------------------------------------------
Run the TUI on another machine and enter this URL:
  https://funny-name-xyz.loca.lt
```

Your choice is saved in a local `.env` file alongside the executable so you won't be asked again.
To change it later, simply edit or delete the `.env` file.

> LocalTunnel URLs are ephemeral. For a stable URL, use Nginx.

### Via Nginx Reverse Proxy

The critical Nginx settings for SSE are `proxy_buffering off` and `chunked_transfer_encoding off` — without these the stream will stall.

```nginx
server {
    listen 80;
    server_name monitor.yourdomain.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;

        # Required for Server-Sent Events
        proxy_set_header Connection '';
        proxy_http_version 1.1;
        chunked_transfer_encoding off;
        proxy_buffering off;
        proxy_cache off;
    }
}
```

---

## API Reference

All endpoints are served by the backend agent on `:8080`.

| Method | Path             | Type | Description                          |
| ------ | ---------------- | ---- | ------------------------------------ |
| `GET`  | `/pusher`        | SSE  | Live `CpuStat` JSON, 1 sample/second |
| `GET`  | `/health`        | JSON | Current health score (0–100)         |
| `GET`  | `/alerts`        | SSE  | Fired alert events                   |
| `GET`  | `/processes`     | JSON | Top 20 processes by CPU              |
| `GET`  | `/services`      | JSON | Status of tracked systemctl services |
| `GET`  | `/logs`          | SSE  | Live `/var/log/syslog` tail          |
| `GET`  | `/stress/status` | JSON | Active stress test state             |
| `POST` | `/stress`        | JSON | Start a stress test                  |

**POST `/stress` body:**

```json
{ "type": "cpu", "workers": 4, "duration_seconds": 30 }
```

`type` accepts `cpu`, `memory`, or `disk`.

---

## Project Structure

```
Pusher/
├── cmd/
│   ├── main.go          # Entry point: registers routes, starts server, optional tunnel
│   ├── routes.go        # HTTP handlers for all feature endpoints
│   └── tui/
│       └── main.go      # TUI entry point: flag parsing, config loading, setup flow
└── internal/
    ├── model.go          # Core data types: CpuStat, Metrics
    ├── service.go        # gopsutil-based metric collection
    ├── alert/            # Threshold rule engine + ring buffer of fired events
    ├── config/           # .robin/config.json read/write + connection history
    ├── health/           # Pure-function health score computation
    ├── history/          # Fixed-capacity ring buffer for sparkline trends
    ├── process/          # /proc-based process list sorted by CPU or memory
    ├── stress/           # Pure-Go CPU / Memory / Disk load generators
    ├── tunnel/           # LocalTunnel subprocess manager (ROBIN_TUNNEL=1)
    └── tui/
        ├── tui.go        # Bubble Tea model, Update loop, layout
        ├── setup.go      # First-run setup wizard with connection probe
        ├── views.go      # Per-tab render functions and sparkline helpers
        ├── commands.go   # Tea commands: poll timers, fetch funcs, message types
        └── stream.go     # SSE client with automatic reconnection
```
