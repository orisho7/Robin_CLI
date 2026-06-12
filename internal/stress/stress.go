package stress

import (
	"context"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Type identifies which system resource to stress.
type Type int

const (
	TypeCPU    Type = iota // saturate CPU cores with math computation
	TypeMemory             // allocate and continually touch large memory blocks
	TypeDisk               // write and delete temporary files in a tight loop
)

// Config parameterizes a stress run.
type Config struct {
	Kind     Type
	Workers  int           // number of concurrent goroutines
	Duration time.Duration // test stops automatically after this duration
}

// Status describes the state of the currently active stress test.
type Status struct {
	Active    bool      `json:"active"`
	Kind      string    `json:"kind"`
	StartedAt time.Time `json:"started_at"`
	EndsAt    time.Time `json:"ends_at"`
}

var (
	running    int32 // atomic flag: 0 = idle, 1 = active
	statMu     sync.Mutex
	stat       Status
	cancelTest context.CancelFunc
)

// ErrAlreadyRunning is returned by Run when a test is already in progress.
var ErrAlreadyRunning = &stressError{"a stress test is already running"}

type stressError struct{ msg string }

func (e *stressError) Error() string { return e.msg }

// IsRunning reports whether a stress test is currently active.
func IsRunning() bool { return atomic.LoadInt32(&running) == 1 }

// Current returns a snapshot of the active stress test state.
func Current() Status {
	statMu.Lock()
	defer statMu.Unlock()
	return stat
}

// Stop cancels any currently active stress test.
func Stop() {
	statMu.Lock()
	defer statMu.Unlock()
	if cancelTest != nil {
		cancelTest()
		cancelTest = nil
	}
}

// Run starts a stress test in the background.
// Returns ErrAlreadyRunning if a test is already active.
// The test stops after cfg.Duration or when ctx is cancelled, whichever is first.
func Run(ctx context.Context, cfg Config) error {
	if !atomic.CompareAndSwapInt32(&running, 0, 1) {
		return ErrAlreadyRunning
	}

	now := time.Now()
	statMu.Lock()
	stat = Status{
		Active:    true,
		Kind:      kindName(cfg.Kind),
		StartedAt: now,
		EndsAt:    now.Add(cfg.Duration),
	}
	statMu.Unlock()

	// context.WithTimeout bounds the test duration regardless of caller context.
	runCtx, cancel := context.WithTimeout(ctx, cfg.Duration)

	statMu.Lock()
	cancelTest = cancel
	statMu.Unlock()

	go func() {
		defer func() {
			cancel()
			atomic.StoreInt32(&running, 0)
			statMu.Lock()
			stat = Status{}
			cancelTest = nil
			statMu.Unlock()
		}()

		var wg sync.WaitGroup
		for i := 0; i < cfg.Workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				switch cfg.Kind {
				case TypeCPU:
					runCPU(runCtx)
				case TypeMemory:
					runMemory(runCtx)
				case TypeDisk:
					runDisk(runCtx)
				}
			}()
		}
		wg.Wait()
	}()

	return nil
}

// runCPU saturates a CPU core with tight floating-point computation.
// Uses a recurrence that prevents the compiler from optimising the loop away.
func runCPU(ctx context.Context) {
	x := 1.0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			x = math.Sqrt(x*x + 1.0)
			if x > 1e15 {
				x = 1.0
			}
		}
	}
}

// runMemory allocates a 64 MiB buffer per worker and continually writes to
// every page to prevent the GC from collecting the allocation.
func runMemory(ctx context.Context) {
	const blockSize = 64 * 1024 * 1024 // 64 MiB per worker
	buf := make([]byte, blockSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Touch every 4-KiB page to ensure physical memory is actually used.
			for i := 0; i < len(buf); i += 4096 {
				buf[i]++
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// runDisk writes 1 MiB temporary files and removes them immediately,
// generating sustained sequential write + metadata pressure.
func runDisk(ctx context.Context) {
	payload := make([]byte, 1*1024*1024) // 1 MiB payload per iteration
	for i := range payload {
		payload[i] = byte(i & 0xFF)
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
			f, err := os.CreateTemp("", "robin_stress_*.tmp")
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			f.Write(payload)
			f.Close()
			os.Remove(f.Name())
		}
	}
}

func kindName(t Type) string {
	switch t {
	case TypeCPU:
		return "CPU"
	case TypeMemory:
		return "Memory"
	case TypeDisk:
		return "Disk"
	default:
		return "Unknown"
	}
}
