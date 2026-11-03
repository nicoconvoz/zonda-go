// Package diag measures the running program, only when ZONDAGO_DIAG=1: every long-lived goroutine carries a name the
// CPU profiler splits the cost by, a live profiler answers on 127.0.0.1:6060 (this computer only), and every 10 s a
// line in diag.txt sets the memory Go manages against the whole process (the difference is the C++ of the DLLs:
// WebRTC's audio processing, libvpx), with the process CPU.
package diag

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"time"
)

// Activo: the measurements are on only when asked for; off, Hilo is a plain call and nothing else runs.
var Activo = os.Getenv("ZONDAGO_DIAG") == "1"

// Nombrar names the goroutine it is called in (its first line), for the profiler; goroutines it starts inherit it.
func Nombrar(nombre string) {
	if Activo {
		pprof.SetGoroutineLabels(pprof.WithLabels(context.Background(), pprof.Labels("hilo", nombre)))
	}
}

// Hilo runs f with the goroutine's name as a profiler label ("hilo"); goroutines f starts inherit it.
func Hilo(nombre string, f func()) {
	if !Activo {
		f()
		return
	}
	pprof.Do(context.Background(), pprof.Labels("hilo", nombre), func(context.Context) { f() })
}

// cadaLinea: criterion, not a measurement: often enough to see memory grow minute by minute, cheap enough to leave
// running (one ReadMemStats and two system calls).
const cadaLinea = 10 * time.Second

// Arrancar opens the live profiler and the memory log in dir; nothing when not Activo.
func Arrancar(dir string) {
	if !Activo {
		return
	}
	// /zonda/nucleos?n=2 changes how many cores Go runs on, live: to measure what the scheduler costs in a call
	// without restarting it.
	http.HandleFunc("/zonda/nucleos", func(w http.ResponseWriter, r *http.Request) {
		if n, err := strconv.Atoi(r.URL.Query().Get("n")); err == nil && n > 0 {
			runtime.GOMAXPROCS(n)
		}
		fmt.Fprintln(w, runtime.GOMAXPROCS(0))
	})
	go http.ListenAndServe("127.0.0.1:6060", nil)
	go Hilo("diag", func() { diario(filepath.Join(dir, "diag.txt")) })
}

func diario(ruta string) {
	f, err := os.OpenFile(ruta, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "\n# %s  arranca (MB; cpu = %% de un núcleo en los últimos %v)\n", time.Now().Format("2006-01-02 15:04:05"), cadaLinea)
	fmt.Fprintln(f, "# hora      cpu   proceso  go-total  go-montón  fuera-de-go  gcs  goroutines  núcleos")
	prevCPU, prevT := cpuProceso(), time.Now()
	for range time.Tick(cadaLinea) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		cpu, t := cpuProceso(), time.Now()
		pct := float64(cpu-prevCPU) / float64(t.Sub(prevT)) * 100
		prevCPU, prevT = cpu, t
		priv := privadaProceso()
		mb := func(b uint64) float64 { return float64(b) / (1 << 20) }
		fuera := 0.0
		if priv > m.Sys {
			fuera = mb(priv - m.Sys)
		}
		fmt.Fprintf(f, "%s  %5.1f  %7.1f  %8.1f  %9.1f  %11.1f  %4d  %10d\n", t.Format("15:04:05"), pct, mb(priv), mb(m.Sys), mb(m.HeapInuse), fuera, m.NumGC, runtime.NumGoroutine())
	}
}
