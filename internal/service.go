package server

import (
	"context"
	"log"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/sensors"
)

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
