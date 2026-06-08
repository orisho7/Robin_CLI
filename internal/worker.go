package server

import (
	"log"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

func CpuUsage(wg *sync.WaitGroup, m *Metrics) {

	GetcpuTemp(m)

	percent, err := cpu.Percent(100*time.Millisecond, false)
	if err != nil {
		log.Println(err)
	}

	m.Mutex.Lock()
	m.CpuUsage = percent[0]

	m.Mutex.Unlock()

	// Usage

}
