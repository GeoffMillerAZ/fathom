package main

// diag_capture.go — automatic diagnostic capture at threshold crossings
// (docs/design/responsible-tenant.md §13.4, RT-122). The governor calls a
// capture on the first crossing of each power of two of its own footprint
// (1, 2, 4, 8 GiB), on entry to refuse, on pressure critical and at the
// exit rung. A capture is a directory ~/.fathom/daemon/<id>/diag/<ts>-<trigger>/
// holding heap.pb.gz, goroutine.txt (goroutines labelled tool/origin by the
// MCP dispatch wrapper's pprof.Do), inflight.json, metrics.txt (with
// fathom_gc_cpu_fraction) and trace.out from the flight recorder the
// governor starts at shed. The exit rung writes only the heap profile and
// inflight.json: it must finish within a second. Retention: 10 captures and
// 200 MiB, oldest pruned first (pruneDiagCaptures).

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// diagTriggerExit is the exit rung's trigger: heap and in-flight only.
const diagTriggerExit = "exit"

// flightWindowBytes is the flight recorder's window (§13.4: 8 MiB).
const flightWindowBytes = 8 << 20

// diagSources is what a capture reads besides the runtime's profiles.
type diagSources struct {
	inflight *tenancy.InFlight
	now      func() time.Time
	// metrics renders metrics.txt (nil: the governor's and admission's
	// lines as /metrics serves them, without the gate's).
	metrics func() string
	// flight is the flight recorder (nil or not recording: no trace.out).
	flight *flightRecorder
}

// captureDiagnostics writes one capture for trigger under dir and returns
// its directory ("" when nothing could be written). Every file is best
// effort: a failed profile leaves the others.
func captureDiagnostics(dir, trigger string, src diagSources) string {
	if dir == "" {
		return ""
	}
	if src.now == nil {
		src.now = time.Now
	}
	if src.inflight == nil {
		src.inflight = tenancy.DefaultInFlight
	}
	d := filepath.Join(dir, src.now().UTC().Format("20060102T150405Z")+"-"+trigger)
	if err := os.MkdirAll(d, 0o700); err != nil {
		return ""
	}
	writeDiagFile(d, "heap.pb.gz", func(w io.Writer) error { return pprof.Lookup("heap").WriteTo(w, 0) })
	if b, err := json.MarshalIndent(src.inflight.Snapshot(), "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(d, "inflight.json"), b, 0o600)
	}
	if trigger != diagTriggerExit {
		// debug=1 prints each goroutine group's pprof labels (tool, origin).
		writeDiagFile(d, "goroutine.txt", func(w io.Writer) error { return pprof.Lookup("goroutine").WriteTo(w, 1) })
		m := ""
		if src.metrics != nil {
			m = src.metrics()
		} else {
			m = governorMetricLines()
		}
		if !strings.Contains(m, "fathom_gc_cpu_fraction") {
			m += "# fathom_gc_cpu_fraction: no governor sample in this process\n"
		}
		_ = os.WriteFile(filepath.Join(d, "metrics.txt"), []byte(m), 0o600)
		if src.flight != nil && src.flight.recording() {
			writeDiagFile(d, "trace.out", func(w io.Writer) error { return src.flight.writeTo(w) })
		}
	}
	pruneDiagCaptures(dir, diagKeep, diagKeepBytes)
	return d
}

func writeDiagFile(dir, name string, write func(io.Writer) error) {
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	if err := write(f); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return
	}
	_ = f.Close()
}

// diagCapturer is the governor's captureDiag for a daemon whose captures
// live under dir.
func diagCapturer(dir string, src diagSources) func(trigger string) string {
	return func(trigger string) string { return captureDiagnostics(dir, trigger, src) }
}

// flightRecorder wraps runtime/trace's flight recorder: started at shed,
// stopped back at normal, so trace.out holds the seconds before a refuse
// or pressure capture. Only one may run per process.
type flightRecorder struct {
	mu sync.Mutex
	fr *trace.FlightRecorder
}

func (f *flightRecorder) start() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fr != nil && f.fr.Enabled() {
		return nil
	}
	f.fr = trace.NewFlightRecorder(trace.FlightRecorderConfig{MaxBytes: flightWindowBytes})
	if err := f.fr.Start(); err != nil {
		f.fr = nil
		return err
	}
	return nil
}

func (f *flightRecorder) stop() {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fr != nil && f.fr.Enabled() {
		f.fr.Stop()
	}
	f.fr = nil
}

func (f *flightRecorder) recording() bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fr != nil && f.fr.Enabled()
}

func (f *flightRecorder) writeTo(w io.Writer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fr == nil {
		return nil
	}
	_, err := f.fr.WriteTo(w)
	return err
}
