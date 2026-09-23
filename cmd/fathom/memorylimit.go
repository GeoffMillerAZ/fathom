package main

// memorylimit.go — W-N (docs/plans/daemon-footprint.md): applies
// [daemon].memory_limit at daemon startup as a SOFT limit
// (runtime/debug.SetMemoryLimit) — the runtime GCs and scavenges harder as
// total memory use approaches it, but never kills the process, unlike a
// hard OS/container limit. Companion to memscavenge.go's periodic
// FreeOSMemory: that bounds how long an idle peak's OS-visible footprint
// survives; this bounds the runtime's own eagerness to grow the heap in
// the first place as usage climbs toward the ceiling.
//
// defaultMemoryLimit ("3GiB") is generous headroom above any legitimate
// single-service typed-edge pass (production evidence: a 390-package
// service peaked well under this) while still catching a runaway before it
// fills swap the way O108's incident did.

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/geoffmilleraz/fathom/internal/config"
)

const defaultMemoryLimit = "3GiB"

// memoryLimitFromEnvOrConfig resolves the memory-limit string serve-http
// applies: FATHOM_MEMORY_LIMIT (an env var, not a config key) ALWAYS wins
// outright over configured ([daemon].memory_limit) — the same
// env-wins-over-config precedence documented at runServeHTTP's own doc
// comment (flags > env > project > user > defaults) and applied ad hoc for
// FATHOM_CALL_LOG (calllogidentity.go, defaultpaths.go). configured empty
// (unset at every config layer) falls back to defaultMemoryLimit.
func memoryLimitFromEnvOrConfig(configured string) string {
	if v := os.Getenv("FATHOM_MEMORY_LIMIT"); v != "" {
		return v
	}
	return firstNonEmpty(configured, defaultMemoryLimit)
}

// applyMemoryLimit resolves and applies the daemon's soft memory limit,
// printing the startup line runServeHTTP emits either way. Returns a
// non-nil error only when the resolved string fails ParseMemoryLimit
// (shape-valid TOML can still be semantically wrong — e.g. "0GiB" — and an
// env override is never CUE-validated at all, so this is the one gate
// every source must pass through before SetMemoryLimit is ever called).
// setLimit is injectable (production: debug.SetMemoryLimit) so tests never
// have to mutate this process's own real runtime limit.
func applyMemoryLimit(configured string, stdout io.Writer, setLimit func(int64) int64) error {
	resolved := memoryLimitFromEnvOrConfig(configured)
	bytes, off, err := config.ParseMemoryLimit(resolved)
	if err != nil {
		return fmt.Errorf("[daemon].memory_limit / FATHOM_MEMORY_LIMIT: %w", err)
	}
	if off {
		fmt.Fprintln(stdout, "serve-http: memory limit off")
		return nil
	}
	setLimit(bytes)
	fmt.Fprintf(stdout, "serve-http: memory limit %s (soft, runtime.SetMemoryLimit)\n", formatMemoryLimit(bytes))
	return nil
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
