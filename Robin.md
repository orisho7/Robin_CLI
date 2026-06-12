“Robin”, a lightweight real-time observability agent and CLI dashboard for Linux systems. The goal is to evolve it from a simple CPU monitor into a full system monitoring and control tool using SSE (Server-Sent Events) as the primary transport layer.

Design the system with a clear layered roadmap. Prioritize features by importance and architectural value, not implementation difficulty.

Tier 1 — Core Product (Must Have)

Implement these first as the foundation of the system:

System Metrics Collection
CPU utilization
Memory usage
Disk usage
Network RX/TX throughput
CLI Dashboard (TUI-style output)
Live updating terminal interface
Compact system overview panel
Real-time refresh from SSE stream
Process Inspector
Top CPU-consuming processes
Top memory-consuming processes
Basic per-process breakdown
Alert Engine
Threshold-based alerts
CPU, memory, disk thresholds
Persistent alert events streamed to CLI
Historical Metrics Storage
In-memory rolling buffer (5 min minimum)
Ability to query recent trends from CLI
Tier 2 — Differentiating Features

These features turn Robin into a useful operational tool:

System Health Score
Single computed score (0–100)
Derived from CPU, memory, disk, temperature
Stress Testing Module
CPU stress generator with configurable load
Memory stress simulation
Disk I/O pressure simulation
Used for system validation under load
Service Monitoring
Track system services (nginx, postgres, redis, etc.)
Show health status per service
Log Monitoring
Tail system logs (/var/log/syslog, auth logs)
Detect error spikes and crash patterns