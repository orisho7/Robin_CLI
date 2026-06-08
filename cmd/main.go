package main

import (
	server "Pusher/internal"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

func main() {
	http.HandleFunc("/pusher", sseHandler)
	http.HandleFunc("/hello", handlehello)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
func handlehello(w http.ResponseWriter, r *http.Request) {
	wc, err := w.Write([]byte("Hello World!"))
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(wc)

}

func sseHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Cache-Control", "no-cache")
	clientgone := r.Context().Done()

	rc := http.NewResponseController(w)
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-clientgone:
			fmt.Println("Disconnected")
			return

		case <-t.C:
			var stats server.CpuStat
			var m server.Metrics
			var wg sync.WaitGroup

			server.GetCpuName(context.Background(), &m)
			server.GetcpuTemp(&m)
			server.GetCpuPCores(context.Background(), &m)
			server.GetCpuLCores(context.Background(), &m)
			wg.Add(1)

			go func() {

				server.CpuUsage(&wg, &m)
				wg.Done()

			}()

			wg.Wait()

			stats.Name = m.CpuName
			stats.Usage = m.CpuUsage
			stats.LogicCores = m.CpuLcores
			stats.PhysicalCores = m.CpuPcores
			stats.Temperature = m.CpuTemp

			JsonRes, err := json.Marshal(stats)
			if err != nil {
				log.Fatal(err)

			}

			_, err = fmt.Printf("data:  %s\n\n", JsonRes)
			if _, err := fmt.Fprintf(w, "data:  %s\n\n", JsonRes); err != nil {
				log.Printf("Error encoding JSON: %v", err)
			}
			err = rc.Flush()
			if err != nil {
				log.Fatal(err)
			}
		}

	}
}
