package tenancy

import (
	"errors"
	"testing"
	"time"
)

// A process that stepped down from safe mode (a mode safe→normal row) and
// later exits over budget ran under normal rules: that exit counts once,
// it does not stop the daemon on its own (§2.3).
func TestBreakerSteppedDownSafeModeOverBudgetDoesNotStop(t *testing.T) {
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	st := NewLifecycleRow(RowStart, t0, 42, "v")
	st.Mode = ModeSafe
	md := NewLifecycleRow(RowMode, t0.Add(31*time.Minute), 42, "v")
	md.From, md.To = ModeSafe, ModeNormal
	ex := NewLifecycleRow(RowExit, t0.Add(50*time.Minute), 42, "v")
	ex.Reason = ExitOverBudget
	v := EvaluateBreaker([]LifecycleRow{st, md, ex}, t0.Add(51*time.Minute), BreakerSettings{})
	if v.Action != BreakerRun || v.Counted != 1 {
		t.Fatalf("a stepped-down process's over-budget exit: %+v, want run with 1 counted", v)
	}
	// Still in safe mode at the exit: stop.
	v = EvaluateBreaker([]LifecycleRow{st, ex}, t0.Add(51*time.Minute), BreakerSettings{})
	if v.Action != BreakerStop {
		t.Fatalf("a safe-mode process's over-budget exit: %+v, want stop", v)
	}
}

// A same-label duplicate start (port held, exit 0) must not hide the live
// incumbent's later unclean death: every unclosed start is examined.
func TestUncleanPredecessorsSeesPastADuplicateStart(t *testing.T) {
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	a := NewLifecycleRow(RowStart, t0, 1, "v")
	b := NewLifecycleRow(RowStart, t0.Add(time.Minute), 2, "v")
	bx := NewLifecycleRow(RowExit, t0.Add(time.Minute), 2, "v")
	bx.Reason, bx.Code = ExitPortHeld, IntPtr(0)
	rows := []LifecycleRow{a, b, bx}
	if p, ok := UncleanPredecessor(rows, 3); !ok || p.PID != 1 {
		t.Fatalf("pid 1 died with no exit row: got %+v %v", p, ok)
	}
	c := NewLifecycleRow(RowStart, t0.Add(2*time.Minute), 4, "v") // a newer unclosed start (alive, say)
	got := UncleanPredecessors(append(rows, c), 3)
	if len(got) != 2 || got[0].PID != 4 || got[1].PID != 1 {
		t.Fatalf("UncleanPredecessors = %+v, want pids [4 1] (newest first)", got)
	}
	if got := UncleanPredecessors(append(rows, c), 4); len(got) != 1 || got[0].PID != 1 {
		t.Fatalf("self excluded: %+v", got)
	}
}

// A start that failed with a non-zero code (a store that opens and is
// wrong, a module that does not mount) is counted: launchd relaunches it
// every throttle interval, the crash loop the breaker exists to stop.
func TestBreakerCountsFailedStarts(t *testing.T) {
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	var rows []LifecycleRow
	for i := 0; i < 5; i++ {
		at := t0.Add(time.Duration(i) * time.Minute)
		rows = append(rows, NewLifecycleRow(RowStart, at, 10+i, "v"))
		ex := NewLifecycleRow(RowExit, at.Add(time.Second), 10+i, "v")
		ex.Reason, ex.Code = ExitStartFailed, IntPtr(1)
		rows = append(rows, ex)
		if i == 2 {
			if v := EvaluateBreaker(rows, at.Add(2*time.Second), BreakerSettings{}); v.Action != BreakerSafe {
				t.Fatalf("3 failed starts: %+v, want safe", v)
			}
		}
	}
	if v := EvaluateBreaker(rows, t0.Add(10*time.Minute), BreakerSettings{}); v.Action != BreakerStop {
		t.Fatalf("5 failed starts: %+v, want stop", v)
	}
	ok := NewLifecycleRow(RowExit, t0, 99, "v")
	ok.Reason, ok.Code = ExitStartFailed, IntPtr(0)
	if countedExit(ok, nil) {
		t.Fatal("a start_failed exit with code 0 counted")
	}
}

// A write that fails with SQLITE_FULL or SQLITE_IOERR (an ingest, a
// refresh) is store distress exactly as a failed read is (§2.1): on a full
// disk the writes fail first.
func TestNoteStoreErrorIsDistress(t *testing.T) {
	t.Cleanup(ResetLifecycleState)
	ResetLifecycleState()
	now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	if kind := NoteStoreError(errors.New("ingest svc: constraint failed"), now); kind != "" {
		t.Fatalf("a non-distress error named %q", kind)
	}
	if _, on := StoreDistressNow(now); on {
		t.Fatal("a non-distress error set distress")
	}
	if kind := NoteStoreError(errors.New("fusion: store: database or disk is full (13)"), now); kind != "disk full (SQLITE_FULL)" {
		t.Fatalf("SQLITE_FULL named %q", kind)
	}
	if why, on := StoreDistressNow(now.Add(time.Minute)); !on || why == "" {
		t.Fatal("a write's SQLITE_FULL did not set distress")
	}
	if StoreDistressKind(errors.New("disk I/O error")) != "disk I/O error (SQLITE_IOERR)" {
		t.Fatal("IOERR not named")
	}
	if NoteStoreError(nil, now) != "" {
		t.Fatal("nil named")
	}
}
