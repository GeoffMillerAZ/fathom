package main

// memscavenge.go — O108 (.fathom/observations.md): the always-on daemon's
// memory grew ~0.9 GB/hour over 9.4h until swap filled. Extensive synthetic
// reproduction (docs/plans/daemon-footprint.md) COULD NOT reproduce
// unbounded growth in any of the named candidates — the watermark-keyed
// call-graph memo (internal/analysis/callgraph), the typed-edge pass
// (internal/fusion/typedcalls.go), store handles, or goroutines all stayed
// bounded under sequential AND concurrent churn at a scale well beyond the
// incident's own 47-refreshes-in-9.4h. What every synthetic run DID show,
// without exception, is the one signature the incident itself recorded:
// go_memstats_heap_sys_bytes (what the Go runtime has reserved from the OS)
// is a high-water mark that never decreases, even while
// go_memstats_heap_inuse_bytes (live reachable objects) correctly
// oscillates and plateaus after each churn burst — exactly "12 GB (11 GB
// compressed) MEM, 623 MB RSS, the rest paged out": memory that was
// touched once (a large graph build, a burst of concurrent misses) and is
// no longer live, but that Go's own background scavenger has not yet
// returned to the OS, so macOS holds it as inactive/compressed rather than
// free. A 24/7 daemon accumulates these peaks all day; a short-lived
// process or a periodically-restarted one never notices.
//
// The fix does not depend on ever finding a specific leaking allocation:
// it bounds the WINDOW during which any one peak can inflate the daemon's
// OS-visible footprint, by asking the runtime to give freed memory back on
// a fixed cadence instead of waiting for Go's own lazy, low-priority
// scavenger. debug.FreeOSMemory forces a full GC cycle and immediately
// returns every free span it can to the OS (madvise), which is exactly
// what turns "inactive, compressed, still counted against this process"
// pages back into memory macOS can reuse elsewhere.

import (
	"context"
	"fmt"
	"os"
	"runtime/metrics"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// defaultScavengeInterval bounds how long any single memory peak can
// inflate the daemon's OS-visible footprint before the next scavenge pass
// gives it back. 15 minutes is frequent enough that a 9-hour shift sees
// dozens of chances to shed a peak, and infrequent enough that
// FreeOSMemory's own GC-and-return cost (a full GC cycle) never competes
// meaningfully with request traffic.
const defaultScavengeInterval = 15 * time.Minute

// scavengeIntervalFromEnv resolves the scavenge cadence: FATHOM_SCAVENGE_INTERVAL
// (a Go duration string, e.g. "5m") when set and valid, else defaultScavengeInterval.
// An invalid value is ignored (falls back), never a startup error — this is
// an operational tuning knob, not a correctness-critical setting.
func scavengeIntervalFromEnv() time.Duration {
	if v := os.Getenv("FATHOM_SCAVENGE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultScavengeInterval
}

// scavengeDisabled lets an operator switch periodic scavenging off without
// a rebuild — e.g. to A/B the daemon's footprint with and without it, or
// because a deployment already handles memory pressure another way.
func scavengeDisabled() bool {
	return strings.EqualFold(os.Getenv("FATHOM_SCAVENGE"), "off")
}

// runMemoryScavenger calls free (production: debug.FreeOSMemory) once per
// interval until ctx is done, logging each pass to log. interval <= 0 is a
// no-op (never spins a busy loop). Safe to run in its own goroutine for
// the life of the process; ctx cancellation is a testability seam — the
// production caller never cancels its own Background() context, so the
// goroutine runs for as long as the daemon does.
//
// RT-45 (design §13.3; DG-4 before it): the line is self-contained evidence
// of what a pass did. It reads total−released (runtime/metrics: what the Go
// runtime holds from the OS, minus what it has already returned) and the
// process's phys_footprint before and after free(), and names the bytes
// released. DG-4's heap_sys/heap_inuse pair came from a stop-the-world
// ReadMemStats, and heap_sys is a high-water mark that never falls — the
// wrong instrument (hazard heap-sys-is-the-wrong-instrument). read nil =
// readScavengeMem. The governor logs the same line on its events
// (governor.scavenge).
func runMemoryScavenger(ctx context.Context, interval time.Duration, log *dlog.Logger, free func(), read func() scavengeMem) {
	if interval <= 0 {
		return
	}
	if read == nil {
		read = readScavengeMem
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			before := read()
			free()
			after := read()
			log.Info(scavengeLine("tick", before, after),
				"total_minus_released_before_bytes", before.TotalMinusReleased,
				"total_minus_released_after_bytes", after.TotalMinusReleased,
				"phys_footprint_before_bytes", before.Footprint,
				"phys_footprint_after_bytes", after.Footprint,
			)
		}
	}
}

// scavengeMem is one scavenge reading: runtime/metrics total−released and
// the process's phys_footprint; -1 = unknown (off darwin, where no
// footprint reader is wired, the line says "phys_footprint unknown").
type scavengeMem struct {
	TotalMinusReleased int64
	Footprint          int64
}

// readScavengeMem reads total−released from runtime/metrics (no
// stop-the-world) and the process footprint.
func readScavengeMem() scavengeMem {
	s := [2]metrics.Sample{{Name: "/memory/classes/total:bytes"}, {Name: "/memory/classes/heap/released:bytes"}}
	metrics.Read(s[:])
	m := scavengeMem{TotalMinusReleased: tenancy.Unknown, Footprint: tenancy.Unknown}
	if s[0].Value.Kind() == metrics.KindUint64 && s[1].Value.Kind() == metrics.KindUint64 {
		m.TotalMinusReleased = int64(s[0].Value.Uint64()) - int64(s[1].Value.Uint64())
	}
	if fp, ok := processPhysFootprintBytes(); ok {
		m.Footprint = fp
	}
	return m
}

// scavengeLine is the one scavenge line the tick and the governor's events
// share: "scavenge (<event>): released X — total−released A → B,
// phys_footprint C → D". released is the footprint's fall (the OS-visible
// effect), else total−released's.
func scavengeLine(event string, before, after scavengeMem) string {
	released := tenancy.Unknown
	switch {
	case before.Footprint >= 0 && after.Footprint >= 0:
		released = max(before.Footprint-after.Footprint, 0)
	case before.TotalMinusReleased >= 0 && after.TotalMinusReleased >= 0:
		released = max(before.TotalMinusReleased-after.TotalMinusReleased, 0)
	}
	return fmt.Sprintf("scavenge (%s): released %s — total−released %s → %s, phys_footprint %s → %s",
		event, tenancy.FormatBytes(released),
		tenancy.FormatBytes(before.TotalMinusReleased), tenancy.FormatBytes(after.TotalMinusReleased),
		tenancy.FormatBytes(before.Footprint), tenancy.FormatBytes(after.Footprint))
}
