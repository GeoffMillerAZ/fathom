package main

// memorylimit.go — the GC hint and GOMAXPROCS (docs/design/
// responsible-tenant.md §2.2, RT-36). runtime/debug.SetMemoryLimit is a
// SOFT limit: the runtime GCs and scavenges harder as total memory use
// approaches it and never kills the process. It is not protection — the
// daemon reached ~30 GB under a 3 GiB one (INC-23). The ladder (governor.go)
// bounds memory; the hint only keeps the heap from outrunning the exit
// mark, so it defaults to the exit mark (a limit at the budget would make
// the band between the refuse and exit marks a GC-thrash band). A managed
// daemon runs with GOMAXPROCS = max(2, NumCPU/2) unless GOMAXPROCS is in
// its env. [daemon].memory_limit / FATHOM_MEMORY_LIMIT migrate to the hint
// (internal/config.ResolveTenancyConfig).

import (
	"fmt"
	"io"
	"runtime/debug"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// applyMemoryLimit applies the budget's GC hint and, for a managed daemon,
// its GOMAXPROCS, printing the startup line. setLimit, setProcs and getenv
// are injectable (production: debug.SetMemoryLimit, runtime.GOMAXPROCS,
// os.Getenv) so tests never change this process's runtime.
func applyMemoryLimit(b tenancy.Budget, stdout io.Writer, setLimit func(int64) int64, setProcs func(int) int, getenv func(string) string) {
	hint := "gc hint off"
	if !b.GCHintOff && b.GCHint > 0 {
		setLimit(b.GCHint)
		how := "configured"
		switch b.GCHint {
		case b.Exit:
			how = "the exit mark"
		case b.Budget:
			how = "the budget"
		}
		hint = fmt.Sprintf("gc hint %s (%s; soft, runtime.SetMemoryLimit — the ladder bounds, not this)", formatMemoryLimit(b.GCHint), how)
	}
	procs := "GOMAXPROCS runtime default"
	if b.GOMAXPROCS > 0 {
		if v := getenv("GOMAXPROCS"); v != "" {
			procs = "GOMAXPROCS " + v + " (env)"
		} else {
			setProcs(b.GOMAXPROCS)
			procs = fmt.Sprintf("GOMAXPROCS %d", b.GOMAXPROCS)
		}
	}
	fmt.Fprintf(stdout, "serve-http: memory %s; %s\n", hint, procs)
}

// formatMemoryLimit renders bytes back into the largest IEC unit it
// divides evenly by (matching the startup line's "3 GiB" shape for the
// common round-number case), falling back to two-decimal GiB for a value
// that doesn't land on an exact unit boundary.
func formatMemoryLimit(bytes int64) string {
	units := []struct {
		suffix string
		size   int64
	}{
		{"TiB", 1 << 40},
		{"GiB", 1 << 30},
		{"MiB", 1 << 20},
		{"KiB", 1 << 10},
	}
	for _, u := range units {
		if bytes >= u.size && bytes%u.size == 0 {
			return fmt.Sprintf("%d %s", bytes/u.size, u.suffix)
		}
	}
	if bytes >= 1<<30 {
		return fmt.Sprintf("%.2f GiB", float64(bytes)/(1<<30))
	}
	return fmt.Sprintf("%d B", bytes)
}

// currentMemoryLimitBytes reads back the runtime's currently-set soft
// memory limit without changing it — debug.SetMemoryLimit(-1) (a negative
// input) returns the previous value and applies nothing (the Go 1.19+
// contract Task 2's own test instruction names). Used by /metrics'
// go_memory_limit_bytes and doctor's daemon_footprint row so both read the
// SAME live value the runtime is actually enforcing, not a cached copy of
// what serve-http applied at startup (which could differ if something else
// in-process — a test, a future feature — ever calls SetMemoryLimit
// again).
func currentMemoryLimitBytes() int64 {
	return debug.SetMemoryLimit(-1)
}

// memoryLimitMetricLine renders go_memory_limit_bytes for /metrics.
func memoryLimitMetricLine() string {
	return fmt.Sprintf("go_memory_limit_bytes %d\n", currentMemoryLimitBytes())
}

// unlimitedMemoryLimit reports whether bytes (as read back by
// currentMemoryLimitBytes, or parsed from /metrics' go_memory_limit_bytes)
// represents "no soft limit set" — the runtime's own sentinel is
// math.MaxInt64, but any value at that scale (an operator could in
// principle configure a multi-exabyte limit, though never meaningfully)
// reads the same way to a human.
func unlimitedMemoryLimit(bytes float64) bool {
	return bytes <= 0 || bytes >= 1e18
}
