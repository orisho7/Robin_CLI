package process

import (
	"runtime"
	"sort"

	goproc "github.com/shirou/gopsutil/v4/process"
)

// logicalCores is the number of logical CPUs visible to the process.
// Used to normalise gopsutil's per-core CPU% into a 0-100 range.
var logicalCores = float64(runtime.NumCPU())

// ProcessStat holds a snapshot of a single process's resource usage.
type ProcessStat struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemRSS     uint64  `json:"mem_rss_bytes"`
	MemPercent float32 `json:"mem_percent"`
}

// TopByCPU returns the top n processes sorted by CPU usage, descending.
// Processes that have exited mid-collection are silently skipped.
func TopByCPU(n int) ([]ProcessStat, error) {
	procs, err := collect()
	if err != nil {
		return nil, err
	}
	sort.Slice(procs, func(i, j int) bool {
		return procs[i].CPUPercent > procs[j].CPUPercent
	})
	return head(procs, n), nil
}

// TopByMemory returns the top n processes sorted by RSS memory, descending.
func TopByMemory(n int) ([]ProcessStat, error) {
	procs, err := collect()
	if err != nil {
		return nil, err
	}
	sort.Slice(procs, func(i, j int) bool {
		return procs[i].MemRSS > procs[j].MemRSS
	})
	return head(procs, n), nil
}

// collect reads all process entries from /proc and returns them as ProcessStat values.
// Individual per-process errors (race with process exit) are non-fatal and skipped.
func collect() ([]ProcessStat, error) {
	pids, err := goproc.Pids()
	if err != nil {
		return nil, err
	}

	stats := make([]ProcessStat, 0, len(pids))
	for _, pid := range pids {
		p, err := goproc.NewProcess(pid)
		if err != nil {
			continue // process may have exited between Pids() and now
		}

		name, _ := p.Name()
		cpu, _ := p.CPUPercent()
		cpu = cpu / logicalCores // normalise: gopsutil sums across all cores
		minfo, _ := p.MemoryInfo()
		mpct, _ := p.MemoryPercent()

		var rss uint64
		if minfo != nil {
			rss = minfo.RSS
		}

		stats = append(stats, ProcessStat{
			PID:        pid,
			Name:       name,
			CPUPercent: cpu,
			MemRSS:     rss,
			MemPercent: mpct,
		})
	}
	return stats, nil
}

func head(s []ProcessStat, n int) []ProcessStat {
	if n > len(s) {
		return s
	}
	return s[:n]
}
