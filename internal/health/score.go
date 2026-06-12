package health

import (
	"math"

	server "Pusher/internal"
)

// Score holds per-component and overall health scores.
// All values are in the range [0, 100]; higher means healthier.
type Score struct {
	Overall float64 `json:"overall"`
	CPU     float64 `json:"cpu"`
	Memory  float64 `json:"memory"`
	Disk    float64 `json:"disk"`
	Temp    float64 `json:"temp"`
}

// Compute derives a health score from a live metrics snapshot.
//
// Weights: CPU 40%, Memory 30%, Disk 20%, Temperature 10%.
//
// Temperature model:
//   - 40°C → 100 pts (baseline idle)
//   - 80°C →   0 pts (critical)
//   - nil temp → 100 pts (sensor unavailable; no penalty)
func Compute(stat server.CpuStat) Score {
	cpu := clamp(100 - stat.Usage)
	mem := clamp(100 - stat.MemoryUsedPercent)
	dsk := clamp(100 - stat.DiskUsedPercent)

	tmp := 100.0
	if stat.Temperature != nil {
		tmp = clamp(100 - math.Max(0, (*stat.Temperature-40)*2.5))
	}

	overall := cpu*0.40 + mem*0.30 + dsk*0.20 + tmp*0.10
	return Score{
		Overall: round1(overall),
		CPU:     round1(cpu),
		Memory:  round1(mem),
		Disk:    round1(dsk),
		Temp:    round1(tmp),
	}
}

func clamp(v float64) float64 {
	return math.Max(0, math.Min(100, v))
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
