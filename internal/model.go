package server

import "sync"

type CpuStat struct {
	Name              string   `json:"name"`
	LogicCores        int      `json:"logic_cores"`
	PhysicalCores     int      `json:"physical_cores"`
	Usage             float64  `json:"usage_percent"`
	Temperature       *float64 `json:"temperature_c"`
	MemoryUsedPercent float64  `json:"memory_used_percent"`
	TotalMemory       uint64   `json:"total_memory"`
	FreeMemory        uint64   `json:"free_memory"`
	BytesSent         uint64   `json:"bytes_sent"`
	BytesRecv         uint64   `json:"bytes_recv"`
	DiskTotal         uint64   `json:"disk_total"`
	DiskUsed          uint64   `json:"disk_used"`
	DiskFree          uint64   `json:"disk_free"`
	DiskUsedPercent   float64  `json:"disk_used_percent"`
}

type Metrics struct {
	CpuName   string
	CpuLcores int
	CpuPcores int
	Mutex     sync.Mutex
	CpuUsage  float64
	CpuTemp   *float64
	Memory
	Net
	Disk
}
type Memory struct {
	UsedPercent float64
	Total       uint64
	Free        uint64
}

type Net struct {
	BytesSent uint64
	BytesRecv uint64
}

type Disk struct {
	DiskTotal       uint64
	DiskUsed        uint64
	DiskFree        uint64
	DiskUsedPercent float64
}
