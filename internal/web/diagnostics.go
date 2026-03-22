package web

import (
	"net/http"
	"runtime"
	"runtime/metrics"
	"time"

	"go-port-forward/internal/models"
	"go-port-forward/pkg/pool"
)

func (h *handler) diagnostics(w http.ResponseWriter, _ *http.Request) {
	if h.mgr == nil {
		fail(w, http.StatusServiceUnavailable, "forward manager is unavailable")
		return
	}
	mgrDiag, err := h.mgr.Diagnostics()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, models.DiagnosticsResponse{
		Timestamp: time.Now(),
		Runtime:   collectRuntimeDiagnostics(),
		Pool: models.PoolDiagnostics{
			Running: pool.Running(),
			Free:    pool.Free(),
			Cap:     pool.Cap(),
		},
		Manager: *mgrDiag,
	})
}

func collectRuntimeDiagnostics() models.RuntimeDiagnostics {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	d := models.RuntimeDiagnostics{
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocBytes: mem.HeapAlloc,
		HeapInuseBytes: mem.HeapInuse,
		HeapObjects:    mem.HeapObjects,
		NumGC:          mem.NumGC,
		PauseTotalNs:   mem.PauseTotalNs,
	}
	if mem.LastGC > 0 {
		d.LastGC = time.Unix(0, int64(mem.LastGC))
	}

	samples := []metrics.Sample{
		{Name: "/sched/goroutines/running:goroutines"},
		{Name: "/sched/goroutines/runnable:goroutines"},
		{Name: "/sched/goroutines/waiting:goroutines"},
		{Name: "/sched/goroutines/not-in-go:goroutines"},
		{Name: "/sched/threads/total:threads"},
	}
	metrics.Read(samples)
	d.GoroutinesRunning = sampleUint64(samples[0])
	d.GoroutinesRunnable = sampleUint64(samples[1])
	d.GoroutinesWaiting = sampleUint64(samples[2])
	d.GoroutinesSyscall = sampleUint64(samples[3])
	d.ThreadsLive = sampleUint64(samples[4])
	return d
}

func sampleUint64(sample metrics.Sample) uint64 {
	if sample.Value.Kind() == metrics.KindUint64 {
		return sample.Value.Uint64()
	}
	return 0
}
