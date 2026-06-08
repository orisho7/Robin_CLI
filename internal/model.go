package server

import "sync"

type CpuStat struct {
	Name          string   `json:"name"`
	LogicCores    int      `json:"logic_cores"`
	PhysicalCores int      `json:"physical_cores"`
	Usage         float64  `json:"usage_percent"`
	Temperature   *float64 `json:"temperature_c"`
}

type Metrics struct {
	CpuName   string
	CpuLcores int
	CpuPcores int
	Mutex     sync.Mutex
	CpuUsage  float64
	CpuTemp   *float64
}
