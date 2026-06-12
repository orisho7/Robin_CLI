package history

import (
	"sync"
	"time"

	server "Pusher/internal"
)

// Snapshot is a timestamped metric sample.
type Snapshot struct {
	Timestamp time.Time
	Stat      server.CpuStat
}

// RingBuffer is a fixed-capacity circular buffer of Snapshots.
// At one sample per second, a capacity of 300 covers a 5-minute window.
// All methods are safe for concurrent use.
type RingBuffer struct {
	buf  []Snapshot
	cap  int
	head int
	size int
	mu   sync.RWMutex
}

// NewRingBuffer returns a RingBuffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buf: make([]Snapshot, capacity),
		cap: capacity,
	}
}

// Record appends a snapshot, evicting the oldest entry when the buffer is full.
func (r *RingBuffer) Record(s Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = s
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
}

// Last returns up to n most recent snapshots in chronological order (oldest first).
func (r *RingBuffer) Last(n int) []Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n > r.size {
		n = r.size
	}
	if n == 0 {
		return nil
	}
	out := make([]Snapshot, n)
	start := ((r.head - n) + r.cap) % r.cap
	for i := 0; i < n; i++ {
		out[i] = r.buf[(start+i)%r.cap]
	}
	return out
}

// TrendCPU returns CPU usage percentage values for the last n samples.
func (r *RingBuffer) TrendCPU(n int) []float64 {
	return trend(r, n, func(s Snapshot) float64 { return s.Stat.Usage })
}

// TrendMemory returns memory usage percentage values for the last n samples.
func (r *RingBuffer) TrendMemory(n int) []float64 {
	return trend(r, n, func(s Snapshot) float64 { return s.Stat.MemoryUsedPercent })
}

// TrendDisk returns disk usage percentage values for the last n samples.
func (r *RingBuffer) TrendDisk(n int) []float64 {
	return trend(r, n, func(s Snapshot) float64 { return s.Stat.DiskUsedPercent })
}

func trend(r *RingBuffer, n int, fn func(Snapshot) float64) []float64 {
	snaps := r.Last(n)
	out := make([]float64, len(snaps))
	for i, s := range snaps {
		out[i] = fn(s)
	}
	return out
}
