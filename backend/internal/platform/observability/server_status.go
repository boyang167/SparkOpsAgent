package observability

import (
	"os"
	"runtime"
	"time"
)

var processStartedAt = time.Now()

type ServerStatus struct {
	Hostname     string  `json:"hostname"`
	GoVersion    string  `json:"go_version"`
	GoOS         string  `json:"go_os"`
	GoArch       string  `json:"go_arch"`
	NumCPU       int     `json:"num_cpu"`
	Goroutines   int     `json:"goroutines"`
	UptimeSec    int64   `json:"uptime_sec"`
	MemAllocMB   float64 `json:"mem_alloc_mb"`
	MemSysMB     float64 `json:"mem_sys_mb"`
	HeapInuseMB  float64 `json:"heap_inuse_mb"`
	HeapObjects  uint64  `json:"heap_objects"`
	GCPauseMs    float64 `json:"gc_pause_ms"`
	CollectedAt  string  `json:"collected_at"`
}

func GetServerStatus() ServerStatus {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	hostname, _ := os.Hostname()
	return ServerStatus{
		Hostname:    hostname,
		GoVersion:   runtime.Version(),
		GoOS:        runtime.GOOS,
		GoArch:      runtime.GOARCH,
		NumCPU:      runtime.NumCPU(),
		Goroutines:  runtime.NumGoroutine(),
		UptimeSec:   int64(time.Since(processStartedAt).Seconds()),
		MemAllocMB:  round2(float64(mem.Alloc) / 1024 / 1024),
		MemSysMB:    round2(float64(mem.Sys) / 1024 / 1024),
		HeapInuseMB: round2(float64(mem.HeapInuse) / 1024 / 1024),
		HeapObjects: mem.HeapObjects,
		GCPauseMs:   round2(float64(mem.PauseTotalNs) / 1e6),
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
