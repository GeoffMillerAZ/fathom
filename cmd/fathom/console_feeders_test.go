package main

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestFeedersIdleWithoutSubscribers (RT-119, design §14): the event
// feeders start on the first /events subscriber and stop when the last one
// leaves — an unwatched console polls nothing — a restart publishes what
// changed while they were stopped (their read positions are kept, never
// re-primed), and the store poll reads only the compile runs newer than its
// cursor.
func TestFeedersIdleWithoutSubscribers(t *testing.T) {
	ctx := t.Context()
	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "store.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	dir := t.TempDir()
	verifyLedger := filepath.Join(dir, "verify.jsonl")
	mustWrite(t, verifyLedger, "")
	bus := console.NewEventBus(console.BusOptions{})
	src := newConsoleEventSources(bus, c, verifyLedger, filepath.Join(dir, "sessions.jsonl"))
	fs := newFeederSwitch(src)
	bus.OnSubscribers(fs.subscribers)
	if fs.running() {
		t.Fatal("feeders run with no subscriber")
	}

	sub, _, err := bus.Subscribe(0)
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, fs.running)
	sub2, _, _ := bus.Subscribe(0)
	sub.Close()
	if !fs.running() {
		t.Fatal("feeders stopped while a subscriber remains")
	}
	sub2.Close()
	waitUntil(t, func() bool { return !fs.running() && fs.stoppedCleanly() })

	// While stopped, the verify ledger grows; a new subscriber is told.
	mustWrite(t, verifyLedger, `{"service":"pay","plan":"docs/plans/p.md","failing":0}`+"\n")
	sub3, _, _ := bus.Subscribe(bus.LastID())
	defer sub3.Close()
	select {
	case e := <-sub3.C():
		if e.Kind != console.EventVerify || e.Service != "pay" {
			t.Fatalf("after a restart = %+v, want the verify row appended while stopped", e)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a change made while the feeders were stopped was never published")
	}

	// The compile cursor: only runs recorded since the last poll are read,
	// in the order they were recorded — a long compile that started first
	// and finished last (its ran_at is its start) is still published.
	at := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	rec := func(id string, when time.Time) {
		t.Helper()
		if err := c.Store().RecordCompileRun(ctx, core.CompileRun{RunID: id, ServiceID: "pay", Layer: "adr-rules", RanAt: when}); err != nil {
			t.Fatal(err)
		}
	}
	src2 := newConsoleEventSources(console.NewEventBus(console.BusOptions{}), c, "", "")
	rec("run-1", at)
	src2.pollStore(ctx) // baseline: nothing published
	base := src2.compileCursor
	if base == 0 {
		t.Fatal("the baseline poll left the cursor at zero")
	}
	rec("run-2", at) // same instant: still new
	rec("run-3", at.Add(time.Second))
	src2.pollStore(ctx)
	if evs := src2.bus.LastID(); evs != 2 {
		t.Fatalf("%d compile events, want run-2 and run-3", evs)
	}
	rec("run-long", at.Add(-time.Hour)) // started before run-3, recorded after
	src2.pollStore(ctx)
	if evs := src2.bus.LastID(); evs != 3 {
		t.Fatalf("%d compile events; the run that started earlier but finished later was missed", evs)
	}
	src2.pollStore(ctx)
	if src2.bus.LastID() != 3 {
		t.Error("an unchanged store published again")
	}
	refs, err := compileRunRefsAfter(ctx, c, base)
	if err != nil || len(refs) != 3 || refs[0].RunID != "run-2" || refs[2].RunID != "run-long" {
		t.Errorf("runs after the baseline = %+v, %v; want run-2, run-3, run-long in record order", refs, err)
	}
}

// TestSourceGenerationCatchesUpWithoutSubscribers (RT-119): with no
// /events subscriber the feeders are off, but a page reading a panel
// source's generation first folds what the ledgers gained (a JSON or
// headless reader is never served a memo keyed on a generation the
// feeders stopped advancing); and a navigation's close-and-reopen of
// /events does not run another full-fleet store poll inside
// consoleStorePoll.
func TestSourceGenerationCatchesUpWithoutSubscribers(t *testing.T) {
	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "store.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	dir := t.TempDir()
	verifyLedger := filepath.Join(dir, "verify.jsonl")
	mustWrite(t, verifyLedger, "")
	bus := console.NewEventBus(console.BusOptions{})
	src := newConsoleEventSources(bus, c, verifyLedger, filepath.Join(dir, "sessions.jsonl"))
	clock := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	var clockMu sync.Mutex
	src.now = func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return clock }
	advance := func(d time.Duration) { clockMu.Lock(); clock = clock.Add(d); clockMu.Unlock() }
	fs := newFeederSwitch(src)
	bus.OnSubscribers(fs.subscribers)
	deps := &consoleDeps{bus: bus, feeders: fs}

	if g := deps.sourceGeneration(t.Context(), "plans:pay"); g != 0 {
		t.Fatalf("generation before any change = %d", g)
	}
	mustWrite(t, verifyLedger, `{"service":"pay","plan":"docs/plans/p.md","failing":0}`+"\n")
	advance(consoleLedgerPoll)
	if g := deps.sourceGeneration(t.Context(), "plans:pay"); g == 0 {
		t.Fatal("a verify appended while no /events subscriber listened left the plans generation unmoved: the page's memo would serve the old audit")
	}

	// Navigation: /events closes and reopens; the store is polled once
	// per consoleStorePoll, not on every restart.
	polls := func() int { src.pollMu.Lock(); defer src.pollMu.Unlock(); return src.storePolls }
	for i := 0; i < 3; i++ {
		sub, _, err := bus.Subscribe(bus.LastID())
		if err != nil {
			t.Fatal(err)
		}
		waitUntil(t, fs.running)
		sub.Close()
		waitUntil(t, func() bool { return !fs.running() && fs.stoppedCleanly() })
	}
	if n := polls(); n > 1 {
		t.Fatalf("three navigations inside consoleStorePoll ran %d full store polls; want at most one", n)
	}
}
