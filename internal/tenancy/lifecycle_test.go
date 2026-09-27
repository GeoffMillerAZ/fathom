package tenancy

import (
	"strings"
	"testing"
	"time"
)

var lt0 = time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)

func at(d time.Duration) string { return lt0.Add(d).Format(time.RFC3339Nano) }

func startRow(pid int, d time.Duration, mode string) LifecycleRow {
	return LifecycleRow{SchemaVersion: 1, Row: RowStart, At: at(d), PID: pid, Mode: mode}
}

func exitRow(pid int, d time.Duration, reason string, code int) LifecycleRow {
	return LifecycleRow{SchemaVersion: 1, Row: RowExit, At: at(d), PID: pid, Reason: reason, Code: &code}
}

// TestLifecycle_UncleanPredecessorNamed (RT-37, §2.3): a start without a
// following exit (or adopt) names its predecessor unclean at the next
// start, with jetsam or resource-report evidence for that pid when found;
// a clean predecessor, a live one, or this process itself is not named.
func TestLifecycle_UncleanPredecessorNamed(t *testing.T) {
	rows := []LifecycleRow{
		startRow(100, 0, ModeNormal),
		exitRow(100, time.Minute, ExitDrain, 0),
		startRow(200, 2*time.Minute, ModeNormal),
	}
	pred, ok := UncleanPredecessor(rows, 300)
	if !ok || pred.PID != 200 {
		t.Fatalf("UncleanPredecessor = %+v, %v; want pid 200", pred, ok)
	}

	jetsam := EvidenceFile{Name: "JetsamEvent-2026-09-26-210500.ips", Content: []byte(`{"bug_type":"298"}
{
  "largestProcess" : "fathom",
  "processes" : [ {"name":"other","pid":17}, {"name":"fathom","pid":200} ]
}`)}
	diag := EvidenceFile{Name: "fathom_2026-09-26-210400_host.diag", Content: []byte("Command:          fathom\nPID:              200\n\nEvent:            disk writes\nAction taken:     none\n")}
	other := EvidenceFile{Name: "fathom_2026-09-26-210300_host.diag", Content: []byte("Command: fathom\nPID:              999\nEvent: disk writes\n")}
	ex := SynthesizeUncleanExit(pred, rows, []EvidenceFile{other, jetsam, diag}, lt0.Add(10*time.Minute), "v-test")
	if ex.Row != RowExit || ex.Reason != ExitUnclean || ex.PID != 200 || ex.Code != nil {
		t.Fatalf("synthesized row = %+v", ex)
	}
	if ex.NotCounted != "" {
		t.Fatalf("an unexplained unclean death is counted, got not_counted %q", ex.NotCounted)
	}
	joined := strings.Join(ex.Evidence, "; ")
	if !strings.Contains(joined, "jetsam: largestProcess") || !strings.Contains(joined, "resource: disk writes") {
		t.Fatalf("evidence = %q, want jetsam largestProcess and resource disk writes", joined)
	}
	if strings.Contains(joined, "999") || strings.Count(joined, "resource:") != 1 {
		t.Fatalf("evidence names another pid's report: %q", joined)
	}
	if !strings.Contains(ex.Detail, "pid 200") || !strings.Contains(ex.Detail, "no exit row") {
		t.Fatalf("detail = %q", ex.Detail)
	}

	// Closed by an exit or adopt row: not unclean.
	for _, closer := range []LifecycleRow{exitRow(200, 3*time.Minute, ExitOverBudget, 70), {Row: RowAdopt, At: at(3 * time.Minute), PID: 200}} {
		if p, ok := UncleanPredecessor(append(append([]LifecycleRow(nil), rows...), closer), 300); ok {
			t.Fatalf("closed by %s, still unclean: %+v", closer.Row, p)
		}
	}
	// This process's own start row is never its predecessor; the older
	// unclosed start before it still is.
	if p, ok := UncleanPredecessor(append(append([]LifecycleRow(nil), rows...), startRow(300, 4*time.Minute, ModeNormal)), 300); !ok || p.PID != 200 {
		t.Fatalf("with its own start row last: %+v %v, want pid 200 (never 300)", p, ok)
	}
	// An empty ledger has no predecessor.
	if _, ok := UncleanPredecessor(nil, 300); ok {
		t.Fatal("empty ledger named a predecessor")
	}
	// A kill-switch start (no start row) leaves the ledger as it was.
	if _, ok := UncleanPredecessor([]LifecycleRow{startRow(1, 0, ModeNormal), exitRow(1, time.Second, ExitIdle, 0)}, 2); ok {
		t.Fatal("clean ledger named a predecessor")
	}
	// Evidence scanning is bounded to MaxEvidenceFiles.
	many := make([]EvidenceFile, MaxEvidenceFiles+5)
	for i := range many {
		many[i] = diag
	}
	if got := SynthesizeUncleanExit(pred, rows, many, lt0.Add(10*time.Minute), "v"); len(got.Evidence) != 1 {
		t.Fatalf("duplicate evidence is folded, got %v", got.Evidence)
	}
}

// TestBreakerSafeModeThenStop (RT-38, §2.3): 3 counted exits in 30 min →
// safe mode; 5 in 60 min, or a safe-mode process exiting over budget →
// stop. Adoption, stop, store-wait, drain, the lifetime exits, an
// already-served port and a deploy-intent kickstart are not counted; an
// operator_reset starts the window over.
func TestBreakerSafeModeThenStop(t *testing.T) {
	cfg := BreakerSettings{}
	now := lt0.Add(40 * time.Minute)
	var rows []LifecycleRow
	add := func(pid int, d time.Duration, reason string, code int) {
		rows = append(rows, startRow(pid, d-time.Minute, ModeNormal), exitRow(pid, d, reason, code))
	}
	// Not counted, however many.
	add(1, 12*time.Minute, ExitDrain, 0)
	add(2, 13*time.Minute, ExitAdopt, 75)
	add(3, 14*time.Minute, ExitStop, 0)
	add(4, 15*time.Minute, ExitIdle, 0)
	add(5, 16*time.Minute, ExitPortHeld, 0) // already served by the same label
	rows = append(rows, LifecycleRow{Row: RowStoreWait, At: at(17 * time.Minute), Errno: "EPERM"})
	rows = append(rows, LifecycleRow{Row: RowDeploy, At: at(18 * time.Minute), ReplacedPID: 6})
	u := exitRow(6, 18*time.Minute+10*time.Second, ExitUnclean, 0)
	u.Code = nil
	rows = append(rows, startRow(6, 17*time.Minute, ModeNormal), u)
	if v := EvaluateBreaker(rows, now, cfg); v.Action != BreakerRun || v.Counted != 0 {
		t.Fatalf("uncounted exits moved the breaker: %+v", v)
	}

	// Two counted: still run.
	add(7, 20*time.Minute, ExitOverBudget, 70)
	add(8, 25*time.Minute, ExitPortHeld, 75)
	if v := EvaluateBreaker(rows, now, cfg); v.Action != BreakerRun || v.Counted != 2 {
		t.Fatalf("two counted: %+v", v)
	}
	// The third inside 30 min: safe mode, reason named.
	u9 := exitRow(9, 30*time.Minute, ExitUnclean, 0)
	u9.Code = nil
	rows = append(rows, startRow(9, 29*time.Minute, ModeNormal), u9)
	v := EvaluateBreaker(rows, now, cfg)
	if v.Action != BreakerSafe || v.CountedSafe != 3 {
		t.Fatalf("three counted in 30 min: %+v", v)
	}
	if !strings.Contains(v.Reason, "3 counted exits in 30m") || !strings.Contains(v.Reason, "over_budget") {
		t.Fatalf("reason = %q", v.Reason)
	}
	// The same three spread past 30 min are not safe mode.
	if v := EvaluateBreaker(rows, lt0.Add(55*time.Minute), cfg); v.Action != BreakerRun {
		t.Fatalf("exits older than the safe window: %+v", v)
	}

	// A safe-mode process that exits over budget: stop.
	stopRows := append(append([]LifecycleRow(nil), rows...), startRow(10, 31*time.Minute, ModeSafe), exitRow(10, 32*time.Minute, ExitOverBudget, 70))
	if v := EvaluateBreaker(stopRows, now, cfg); v.Action != BreakerStop || !strings.Contains(v.Reason, "safe mode") {
		t.Fatalf("safe-mode over-budget exit: %+v", v)
	}

	// Five in 60 min: stop.
	add(11, 33*time.Minute, ExitOverBudget, 70)
	add(12, 34*time.Minute, ExitOverBudget, 70)
	if v := EvaluateBreaker(rows, now, cfg); v.Action != BreakerStop || v.Counted != 5 || !strings.Contains(v.Reason, "5 counted exits in 1h0m") {
		t.Fatalf("five counted in 60 min: %+v", v)
	}

	// operator_reset (`fathom daemon start`) starts the window over.
	reset := append(append([]LifecycleRow(nil), rows...), LifecycleRow{Row: RowOperatorReset, At: at(35 * time.Minute)})
	if v := EvaluateBreaker(reset, now, cfg); v.Action != BreakerRun || v.Counted != 0 {
		t.Fatalf("after operator_reset: %+v", v)
	}

	// Configured thresholds are honoured.
	if v := EvaluateBreaker(rows, now, BreakerSettings{SafeAfter: 10, StopAfter: 10}); v.Action != BreakerRun {
		t.Fatalf("configured thresholds: %+v", v)
	}
}

// TestDeployIntentClassifiesUncleanDeath (RT-156, §8.5): a deploy row that
// names the predecessor's pid, written after its start and within 60 s,
// makes its unclean death not counted; a deploy naming another pid, or
// older than 60 s, does not.
func TestDeployIntentClassifiesUncleanDeath(t *testing.T) {
	rows := []LifecycleRow{startRow(200, 0, ModeNormal), {Row: RowDeploy, At: at(5 * time.Minute), ReplacedPID: 200, BinaryHash: "abc"}}
	pred, ok := UncleanPredecessor(rows, 300)
	if !ok {
		t.Fatal("predecessor not found")
	}
	ex := SynthesizeUncleanExit(pred, rows, nil, lt0.Add(5*time.Minute+20*time.Second), "v")
	if ex.NotCounted == "" || !strings.Contains(ex.NotCounted, "deploy") {
		t.Fatalf("deploy-intent kickstart counted: %+v", ex)
	}
	if v := EvaluateBreaker(append(rows, ex), lt0.Add(6*time.Minute), BreakerSettings{SafeAfter: 1}); v.Counted != 0 {
		t.Fatalf("breaker counted a deploy kickstart: %+v", v)
	}
	late := SynthesizeUncleanExit(pred, rows, nil, lt0.Add(7*time.Minute), "v")
	if late.NotCounted != "" {
		t.Fatalf("a deploy row older than 60 s excused a death: %+v", late)
	}
	rows[1].ReplacedPID = 201
	if other := SynthesizeUncleanExit(pred, rows, nil, lt0.Add(5*time.Minute+20*time.Second), "v"); other.NotCounted != "" {
		t.Fatalf("a deploy for another pid excused a death: %+v", other)
	}
}

// TestSafeModeStepsDownAfter30MinNormal (§2.3): safe mode ends after 30
// min continuously at normal; any higher rung restarts the clock.
func TestSafeModeStepsDownAfter30MinNormal(t *testing.T) {
	tr := SafeModeTracker{Safe: true}
	if tr.Step(lt0, LevelNormal) {
		t.Fatal("left at once")
	}
	if tr.Step(lt0.Add(20*time.Minute), LevelShed) {
		t.Fatal("left while shed")
	}
	if tr.Step(lt0.Add(21*time.Minute), LevelNormal) || tr.Step(lt0.Add(50*time.Minute), LevelNormal) {
		t.Fatal("left before 30 min at normal since the shed")
	}
	if !tr.Step(lt0.Add(51*time.Minute), LevelNormal) || tr.Safe {
		t.Fatal("did not leave after 30 min at normal")
	}
	if tr.Step(lt0.Add(90*time.Minute), LevelNormal) {
		t.Fatal("a normal tracker left twice")
	}
}

// TestWarmupLimitsHeavySlots (RT-40, §2.3): for warmup after any start,
// tree, fleet and ingest get one slot each, point and index are
// unchanged, and no inline ingest is allowed; after it, the slots return.
func TestWarmupLimitsHeavySlots(t *testing.T) {
	slots := map[Class]int{ClassPoint: 64, ClassIndex: 8, ClassTree: 2, ClassFleet: 1, ClassIngest: 1, ClassEgress: 2}
	got := WarmupSlots(slots, lt0, lt0.Add(30*time.Second), 0)
	want := map[Class]int{ClassPoint: 64, ClassIndex: 8, ClassTree: 1, ClassFleet: 1, ClassIngest: 1, ClassEgress: 2}
	for c, n := range want {
		if got[c] != n {
			t.Errorf("warm-up slots[%s] = %d, want %d", c, got[c], n)
		}
	}
	if slots[ClassTree] != 2 {
		t.Fatal("WarmupSlots mutated its input")
	}
	after := WarmupSlots(slots, lt0, lt0.Add(DefaultWarmup), 0)
	if after[ClassTree] != 2 {
		t.Fatalf("after warm-up tree slots = %d, want 2", after[ClassTree])
	}
	if custom := WarmupSlots(slots, lt0, lt0.Add(150*time.Second), 5*time.Minute); custom[ClassTree] != 1 {
		t.Fatal("a configured warm-up is honoured")
	}

	defer resetLifecycleState()
	SetWarmup(lt0, 0)
	if ok, why := InlineIngestAllowed(lt0.Add(time.Minute)); ok || !strings.Contains(why, "warm-up") {
		t.Fatalf("inline ingest during warm-up: %v %q", ok, why)
	}
	if ok, _ := InlineIngestAllowed(lt0.Add(DefaultWarmup + time.Second)); !ok {
		t.Fatal("inline ingest after warm-up refused")
	}
	SetSafeMode(&SafeModeInfo{Since: lt0, Reason: "3 counted exits"})
	if ok, why := InlineIngestAllowed(lt0.Add(time.Hour)); ok || !strings.Contains(why, "safe mode") {
		t.Fatalf("inline ingest in safe mode: %v %q", ok, why)
	}
	SetSafeMode(nil)
	SetPausedSafeMode(true)
	if _, on := SafeModeNow(); !on {
		t.Fatal("paused.json is safe mode")
	}
	SetPausedSafeMode(false)
	NoteStoreDistress("disk full (SQLITE_FULL)", lt0.Add(time.Hour))
	if ok, why := InlineIngestAllowed(lt0.Add(time.Hour)); ok || !strings.Contains(why, "store distress") {
		t.Fatalf("inline ingest in store distress: %v %q", ok, why)
	}
}

// TestStoreDistressHoldsAndClears (RT-41, §2.1): a store error holds
// distress for StoreDistressHold; a low store volume holds it while low.
func TestStoreDistressHoldsAndClears(t *testing.T) {
	defer resetLifecycleState()
	if _, on := StoreDistressNow(lt0); on {
		t.Fatal("distress without cause")
	}
	NoteStoreDistress("disk I/O error (SQLITE_IOERR)", lt0)
	if why, on := StoreDistressNow(lt0.Add(time.Minute)); !on || !strings.Contains(why, "SQLITE_IOERR") {
		t.Fatalf("distress after an IOERR: %v %q", on, why)
	}
	if _, on := StoreDistressNow(lt0.Add(StoreDistressHold + time.Second)); on {
		t.Fatal("an old store error still holds distress")
	}
	NoteStoreVolumeFree(1*GiB, 2*GiB, lt0)
	if why, on := StoreDistressNow(lt0.Add(time.Hour)); !on || !strings.Contains(why, "store volume") {
		t.Fatalf("low store volume: %v %q", on, why)
	}
	NoteStoreVolumeFree(3*GiB, 2*GiB, lt0.Add(time.Hour))
	if _, on := StoreDistressNow(lt0.Add(time.Hour)); on {
		t.Fatal("a recovered volume still holds distress")
	}
	NoteStoreVolumeFree(Unknown, 2*GiB, lt0)
	if _, on := StoreDistressNow(lt0.Add(time.Hour)); on {
		t.Fatal("an unknown volume reading is never a trigger")
	}
}
