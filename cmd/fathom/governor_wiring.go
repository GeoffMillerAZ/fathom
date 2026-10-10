package main

// governor_wiring.go — what serve-http's governor acts on (design §2.1,
// RT-34): the caches each rung drops and the background work shed pauses.
// Before wave C's internal/memo, shed drops the console and fleet memos and
// keeps the graph and symbol caches; refuse drops every cache.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// backgroundPaused is true at shed and above: the console feeders skip
// their polls (auto-refresh reads the published level itself, in ensure).
var backgroundPaused atomic.Bool

// reset forgets the gather caches.
func (c *projectsCaches) reset() {
	c.mu.Lock()
	c.ledgerKey, c.runs, c.ledgerRd = "", nil, console.SourceRead{}
	c.typedKey, c.typed, c.facts = "", nil, nil
	c.mu.Unlock()
}

// consoleMemoDrops are the console's derived page caches (shed).
func consoleMemoDrops() {
	projectWiringMemo.reset()
	projectIndexMemo.reset()
	projectActivityMemo.reset()
	intentAuditMemo.reset()
	consoleGatherCache.reset()
	consoleProjectsCache.Lock()
	consoleProjectsCache.key, consoleProjectsCache.read = "", projectsRead{}
	consoleProjectsCache.Unlock()
	obsCache.Lock()
	dropObsCacheLocked()
	obsCache.Unlock()
	// The tails' folds and read positions and the beat index's offsets
	// stay: they are small next to the logs, and dropping them would make
	// the next read re-read whole logs while memory is tight. Only the
	// index's cached session lines are freed.
	hooksBeatIndex.dropLines()
}

// servingLadderActions wires the rungs to the serving daemon's caches.
func servingLadderActions(srv *mcp.Server, c *client.Client, fleet *fleetAdapter) ladderActions {
	return ladderActions{
		dropShed: []namedDrop{
			// §5: the memo registry trims by LRU to half its budget at
			// shed and stays under it while shed; only refuse drops it.
			{"memo registry trim", func() { memo.Trim(memo.ReasonShed, 0) }},
			{"console memos", consoleMemoDrops},
			{"fleet memos", c.DropCaches},
		},
		dropRefuse: []namedDrop{
			{"memo registry", func() { memo.Drop(memo.ReasonRefuse) }},
			{"graph and navigation caches", srv.DropCaches},
			{"obligations memos", c.DropHotPathMemos},
			{"ingest stat records", c.DropIngestStats},
			// LB-F15: idle grammar modules (up to 64 MiB each) close now;
			// a module leased mid-extraction closes at its release.
			{"idle grammar modules", tsengine.CloseIdleParsers},
		},
		dropIdle: []namedDrop{
			{"memo registry", func() { memo.Drop(memo.ReasonIdle) }},
			{"idle grammar modules", tsengine.CloseIdleParsers},
		},
		pauseBackground: func(p bool) { backgroundPaused.Store(p) },
	}
}

// startGovernor resolves nothing itself: it builds the serving daemon's
// governor over the platform host, loads the quarantine from the lifecycle
// ledger (its diagnostic captures beside it), and starts sampling; observe
// (may be nil) sees every sample and the level after it — store distress
// and the breaker's safe mode ride on it (RT-41, RT-39). stop ends the
// sampling goroutine, unpublishes the governor and the level (serve-http
// defers it: an in-process serve-http that returns leaves nothing running).
func startGovernor(b tenancy.Budget, notes []tenancy.Note, storePath, ledger string, actions ladderActions, flog *dlog.Logger, observe func(*tenancy.Sample, tenancy.Level)) (g *governor, stop func()) {
	dir := ""
	if ledger != "" {
		dir = filepath.Dir(ledger)
	}
	pid := os.Getpid()
	// §5: the memo registry's byte budget is the cache fraction of the
	// tenancy budget; at shed it holds ShedTrim of it.
	memo.Default.SetBudget(b.Caches, b.CacheShedTrim)
	flight := &flightRecorder{}
	g = newGovernor(governorConfig{
		budget:       b,
		notes:        notes,
		host:         newTenancyHost(filepath.Dir(storePath)),
		now:          time.Now,
		inflight:     tenancy.DefaultInFlight,
		ledger:       ledger,
		pid:          pid,
		version:      Version,
		logf:         func(m string) { flog.Warn(m) },
		actions:      actions,
		freeOSMemory: debug.FreeOSMemory,
		killChildren: func() int { return killChildTree(pid) },
		captureDiag:  diagCapturer(filepath.Join(dir, "diag"), diagSources{inflight: tenancy.DefaultInFlight, now: time.Now, flight: flight}),
		flight:       flight,
		exit:         os.Exit,
		observe:      observe,
	})
	if dir == "" {
		g.captureDiag = nil
	} else if ring, err := tenancy.OpenRing(filepath.Join(dir, tenancy.RingFileName)); err == nil {
		g.ring = ring
		if err := ring.WriteMeta(tenancy.RingMeta{PID: pid, Managed: b.Managed, StartedAt: time.Now(), Budget: b.Budget, Shed: b.Shed, Refuse: b.Refuse, Exit: b.Exit, ExitEnabled: b.ExitEnabled}); err != nil {
			flog.Warn(fmt.Sprintf("tenancy: resource ring header not written (%v)", err))
		}
		if days, derr := tenancy.OpenDayLog(filepath.Join(dir, tenancy.DaysFileName)); derr == nil {
			g.days = days
		} else {
			flog.Warn(fmt.Sprintf("tenancy: day summaries unavailable (%v) — `fathom resources --days` will read none for this daemon", derr))
		}
	} else {
		flog.Warn(fmt.Sprintf("tenancy: resource ring unavailable (%v) — `fathom resources` and doctor will read no history for this daemon", err))
	}
	if err := g.loadQuarantine(); err != nil {
		flog.Warn(fmt.Sprintf("tenancy: quarantine unreadable (%v) — the lifecycle ledger %s could not be read; no service is treated as quarantined", err, ledger))
	}
	activeGovernor.Store(g)
	ctx, cancel := context.WithCancel(context.Background())
	//fathom:tenancy-allow bare-go the governor's sampler, one per process, stopped by its cancel
	go g.run(ctx)
	return g, func() {
		cancel()
		flight.stop()
		if activeGovernor.CompareAndSwap(g, nil) {
			tenancy.PublishLevel(tenancy.LevelNormal)
		}
		g.mu.Lock()
		if c, ok := g.ring.(interface{ Close() error }); ok {
			_ = c.Close()
		}
		g.ring = nil
		if g.days != nil {
			_ = g.days.Flush(g.now(), true)
			g.days = nil
		}
		g.mu.Unlock()
	}
}

// tenancyStartupLine names the budget and its derivation (P22).
func tenancyStartupLine(b tenancy.Budget, notes []tenancy.Note) string {
	how := ""
	for _, n := range notes {
		if n.Key == "budget" {
			how = n.Derivation
		}
	}
	exit := tenancy.FormatBytes(b.Exit)
	if !b.ExitEnabled {
		exit += " (exit rung disabled)"
	}
	return fmt.Sprintf("tenancy: budget %s (%s) · shed %s · refuse %s · exit %s · sampled every %s, %d CPUs",
		tenancy.FormatBytes(b.Budget), how, tenancy.FormatBytes(b.Shed), tenancy.FormatBytes(b.Refuse), exit, tenancy.SampleEvery, runtime.NumCPU())
}
