package server

import (
	"context"
	"log"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/sensors"
)

func GetNetUsage(m *Metrics) {
	counters, err := net.IOCounters(false)
	if err != nil {
		log.Println(err)
		return
	}

	if len(counters) > 0 {
		m.Net.BytesSent = counters[0].BytesSent
		m.Net.BytesRecv = counters[0].BytesRecv
	}
}

func GetCpuName(ctx context.Context, m *Metrics) {
	// Name and basic info
	info, err := cpu.Info()
	if err != nil {
		log.Println(err)
	}

	if len(info) > 0 {
		m.CpuName = info[0].ModelName
	}

}
func GetDiskUsage(m *Metrics) {
	// Use "/" for Linux/macOS or "C:" for Windows
	usage, err := disk.Usage("/")
	if err != nil {
		log.Printf("Disk Error: %v", err)
		return
	}

	m.Disk.DiskTotal = usage.Total
	m.Disk.DiskUsed = usage.Used
	m.Disk.DiskFree = usage.Free
	m.Disk.DiskUsedPercent = usage.UsedPercent
}
func GetMemoryUsage(m *Metrics) {
	v, _ := mem.VirtualMemory()

	// almost every return value is a struct
	m.Memory.Free = v.Free
	m.Memory.Total = v.Total
	m.Memory.UsedPercent = v.UsedPercent
	// convert to JSON. String() is also implemented
}

func GetCpuLCores(ctx context.Context, m *Metrics) {
	// Cores
	logicalCores, _ := cpu.Counts(true)
	m.CpuLcores = logicalCores

}
func GetCpuPCores(ctx context.Context, m *Metrics) {
	// Cores
	physicalCores, _ := cpu.Counts(false)
	m.CpuPcores = physicalCores
}

func GetcpuTemp(m *Metrics) {
	temps, _ := sensors.SensorsTemperatures()
	if len(temps) == 0 {
		return
	}

	maxVal := 0.0
	for _, t := range temps {
		if t.Temperature > maxVal {
			maxVal = t.Temperature
		}
	}
	m.Mutex.Lock()
	m.CpuTemp = &maxVal
	m.Mutex.Unlock()
}
