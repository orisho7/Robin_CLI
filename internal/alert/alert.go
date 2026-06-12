package alert

import (
	"fmt"
	"sync"
	"time"

	server "Pusher/internal"
)

// Severity classifies alert urgency.
type Severity string

const (
	SeverityWarn     Severity = "WARN"
	SeverityCritical Severity = "CRITICAL"
)

// Rule is a single threshold-based alert condition.
type Rule struct {
	Metric    string   // "cpu" | "memory" | "disk"
	Threshold float64  // percentage at which the rule fires
	Severity  Severity
}

// Event is a single fired alert instance.
type Event struct {
	Metric   string    `json:"metric"`
	Value    float64   `json:"value"`
	Severity Severity  `json:"severity"`
	FiredAt  time.Time `json:"fired_at"`
	Message  string    `json:"message"`
}

const ringCap = 100

// Engine evaluates rules against incoming metric snapshots and stores
// recent events in a fixed-size ring buffer. Safe for concurrent use.
type Engine struct {
	rules []Rule
	buf   [ringCap]Event
	head  int
	count int
	mu    sync.RWMutex
}

// DefaultRules returns a production-sensible baseline set of thresholds.
func DefaultRules() []Rule {
	return []Rule{
		{Metric: "cpu", Threshold: 85, Severity: SeverityWarn},
		{Metric: "cpu", Threshold: 95, Severity: SeverityCritical},
		{Metric: "memory", Threshold: 90, Severity: SeverityWarn},
		{Metric: "memory", Threshold: 98, Severity: SeverityCritical},
		{Metric: "disk", Threshold: 90, Severity: SeverityWarn},
		{Metric: "disk", Threshold: 98, Severity: SeverityCritical},
	}
}

// NewEngine creates an Engine with the provided rules.
func NewEngine(rules []Rule) *Engine {
	return &Engine{rules: rules}
}

// Evaluate checks stat against every rule and returns the newly fired events.
// Events are also recorded in the internal ring buffer for later retrieval.
// Note: every rule that matches fires on every call (no deduplication).
// Callers that want rate-limiting should gate calls externally.
func (e *Engine) Evaluate(stat server.CpuStat) []Event {
	values := map[string]float64{
		"cpu":    stat.Usage,
		"memory": stat.MemoryUsedPercent,
		"disk":   stat.DiskUsedPercent,
	}

	var fired []Event
	for _, r := range e.rules {
		v, ok := values[r.Metric]
		if !ok || v < r.Threshold {
			continue
		}
		ev := Event{
			Metric:   r.Metric,
			Value:    v,
			Severity: r.Severity,
			FiredAt:  time.Now(),
			Message:  fmt.Sprintf("%s %.1f%% ≥ threshold %.1f%%", r.Metric, v, r.Threshold),
		}
		fired = append(fired, ev)
		e.push(ev)
	}
	return fired
}

// Recent returns up to n most recent events, newest first.
func (e *Engine) Recent(n int) []Event {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if n > e.count {
		n = e.count
	}
	out := make([]Event, n)
	for i := 0; i < n; i++ {
		idx := ((e.head - 1 - i) + ringCap) % ringCap
		out[i] = e.buf[idx]
	}
	return out
}

func (e *Engine) push(ev Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buf[e.head] = ev
	e.head = (e.head + 1) % ringCap
	if e.count < ringCap {
		e.count++
	}
}
