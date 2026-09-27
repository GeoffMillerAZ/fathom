package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestEnsureSkippedAtShedSafeModeWarmupQuarantine pins RT-150 (design §7.1,
// §2.1, §2.3): ensure() runs no inline ingest at shed and above, in safe
// mode, during warm-up, or for a quarantined service — the read answers
// from the index as it stands and says why: the response envelope carries
// `refresh: deferred (<reason>)` for the service. Once the gate lifts, the
// same drift is re-ingested.
func TestEnsureSkippedAtShedSafeModeWarmupQuarantine(t *testing.T) {
	now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	t.Cleanup(func() {
		tenancy.PublishLevel(tenancy.LevelNormal)
		tenancy.ResetLifecycleStateForTest()
		tenancy.SetQuarantined(nil)
	})
	cases := []struct {
		name   string
		set    func()
		reason string
	}{
		{"shed", func() { tenancy.PublishLevel(tenancy.LevelShed) }, "deferred (shed)"},
		{"refuse", func() { tenancy.PublishLevel(tenancy.LevelRefuse) }, "deferred (refuse)"},
		{"safe mode", func() { tenancy.SetPausedSafeMode(true) }, "deferred (safe mode)"},
		{"warm-up", func() { tenancy.SetWarmup(now.Add(-30*time.Second), 120*time.Second) }, "deferred (warm-up"},
		{"quarantine", func() {
			tenancy.SetQuarantined([]tenancy.Quarantine{{Service: "svc", Reason: tenancy.QuarantineReasonIngest}})
		}, "deferred (quarantined: exceeded budget during ingest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tenancy.PublishLevel(tenancy.LevelNormal)
			tenancy.ResetLifecycleStateForTest()
			tenancy.SetQuarantined(nil)
			tc.set()

			fake := &refreshFake{path: "/repo", indexed: "old"}
			ar := newAR(t, fake, "new")
			ar.now = func() time.Time { return now }
			ctx, notes := withRefreshNotes(context.Background())
			if _, err := ar.FindSymbol(ctx, "svc", "X"); err != nil {
				t.Fatal(err)
			}
			if fake.ingests != 0 {
				t.Fatalf("ingests while gated by %s = %d, want 0", tc.name, fake.ingests)
			}
			got := notes.render()["svc"]
			if !strings.HasPrefix(got, tc.reason) {
				t.Fatalf("refresh note = %q, want it to start %q", got, tc.reason)
			}

			// The gate lifts: the same drift is re-ingested; the read that
			// asked for it says it was served stale while the refresh is
			// queued (RT-99), never "deferred".
			tenancy.PublishLevel(tenancy.LevelNormal)
			tenancy.ResetLifecycleStateForTest()
			tenancy.SetQuarantined(nil)
			ctx2, notes2 := withRefreshNotes(context.Background())
			ensureSettled(t, ar, ctx2, "svc")
			if fake.ingests != 1 {
				t.Fatalf("ingests after the gate lifted = %d, want 1", fake.ingests)
			}
			if r := notes2.render()["svc"]; strings.HasPrefix(r, "deferred") || !strings.Contains(r, "index_stale") {
				t.Fatalf("refresh note after a normal ensure = %q, want queued/refreshing with index_stale", r)
			}
		})
	}

	t.Run("warm-up ends", func(t *testing.T) {
		tenancy.SetWarmup(now.Add(-121*time.Second), 120*time.Second)
		fake := &refreshFake{path: "/repo", indexed: "old"}
		ar := newAR(t, fake, "new")
		ar.now = func() time.Time { return now }
		ensureSettled(t, ar, context.Background(), "svc")
		if fake.ingests != 1 {
			t.Fatalf("ingests after warm-up ended = %d, want 1", fake.ingests)
		}
	})

	t.Run("envelope", func(t *testing.T) {
		tenancy.ResetLifecycleStateForTest()
		tenancy.PublishLevel(tenancy.LevelShed)
		defer tenancy.PublishLevel(tenancy.LevelNormal)
		fake := &refreshFake{path: "/repo", indexed: "old"}
		srv := NewServer(newAR(t, fake, "new"))
		params, _ := json.Marshal(map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "X"}})
		res, rpcErr := srv.handleToolsCallWithCtx(context.Background(), params)
		if rpcErr != nil {
			t.Fatalf("find_symbol: %v", rpcErr.Message)
		}
		env, _ := res.(map[string]any)
		refresh, _ := env["refresh"].(map[string]string)
		if !strings.HasPrefix(refresh["svc"], "deferred (shed)") {
			t.Fatalf("envelope refresh = %v, want svc deferred (shed)", env["refresh"])
		}
	})
}

// ingestStateFake reports an ingest_state row for the service; noMeta
// makes ServiceMeta answer not-found (a first ingest killed before its
// final commit recorded any).
type ingestStateFake struct {
	refreshFake
	row    store.IngestState
	noMeta bool
	paths  []string // the corpus roots headFn was asked about
}

func (f *ingestStateFake) IngestState(_ context.Context, _ string) (store.IngestState, bool, error) {
	return f.row, f.row.State != "", nil
}

func (f *ingestStateFake) ServiceMeta(ctx context.Context, svc string) (string, string, bool, error) {
	if f.noMeta {
		return "", "", false, nil
	}
	return f.refreshFake.ServiceMeta(ctx, svc)
}

// ownerProbe is an injected liveness answer for the row's pid.
func ownerProbe(alive bool) func(int, time.Time) bool {
	return func(int, time.Time) bool { return alive }
}

// TestEnsureReingestsInterruptedIngest pins RT-109's read half (design §6):
// an in_progress row whose owner is dead or foreign — a killed or halted
// ingest in another process — means the index is not the one its watermark
// describes, so ensure re-ingests even when HEAD equals the watermark. A
// row this process wrote, a complete row, and an in_progress row whose
// owner is still running (a live concurrent ingest; review finding 3) do
// not.
func TestEnsureReingestsInterruptedIngest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state string
		pid   int
		alive bool
		want  int
	}{
		{"foreign in_progress, owner dead", "in_progress", os.Getpid() + 1, false, 1},
		{"foreign in_progress, owner still running", "in_progress", os.Getpid() + 1, true, 0},
		{"own in_progress", "in_progress", os.Getpid(), false, 0},
		{"complete", "complete", os.Getpid() + 1, false, 0},
		{"no row", "", 0, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &ingestStateFake{refreshFake: refreshFake{path: "/repo", indexed: "same"}, row: store.IngestState{ServiceID: "svc", State: tc.state, PID: tc.pid, StartedAt: 1}}
			ar := newAR(t, fake, "same")
			ar.ownerAlive = ownerProbe(tc.alive)
			ensureSettled(t, ar, context.Background(), "svc")
			ensureSettled(t, ar, context.Background(), "svc")
			if fake.ingests != tc.want {
				t.Fatalf("ingests = %d, want %d", fake.ingests, tc.want)
			}
		})
	}
}

// TestEnsureSeenDoesNotHideInterruptedIngest pins review finding 2 on
// RT-109: this daemon refreshed svc to H (seen[svc] = H); another process
// then began an ingest at H and was killed. The debounce must not hide it:
// the half-written index is re-ingested although HEAD never moved.
func TestEnsureSeenDoesNotHideInterruptedIngest(t *testing.T) {
	fake := &ingestStateFake{refreshFake: refreshFake{path: "/repo", indexed: "old"}}
	ar := newAR(t, fake, "new")
	ar.ownerAlive = ownerProbe(false)
	ensureSettled(t, ar, context.Background(), "svc")
	if fake.ingests != 1 {
		t.Fatalf("ingests after the drift = %d, want 1", fake.ingests)
	}
	fake.indexed = "new"
	fake.row = store.IngestState{ServiceID: "svc", State: store.IngestInProgress, PID: os.Getpid() + 1, StartedAt: 2, TargetCommit: "new", Path: "/repo"}
	ensureSettled(t, ar, context.Background(), "svc")
	if fake.ingests != 2 {
		t.Fatalf("ingests after a foreign ingest at the seen HEAD was killed = %d, want 2", fake.ingests)
	}
}

// TestEnsureResumesKilledFirstIngest pins review finding 1 on RT-109: a
// first ingest killed before its final commit left no service_meta, only an
// in_progress row naming the corpus root; ensure re-ingests from that root.
func TestEnsureResumesKilledFirstIngest(t *testing.T) {
	fake := &ingestStateFake{noMeta: true, row: store.IngestState{ServiceID: "svc", State: store.IngestInProgress, PID: os.Getpid() + 1, StartedAt: 1, TargetCommit: "h1", Path: "/new/repo"}}
	ar := newAR(t, fake, "h1")
	ar.headFn = func(p string) string { fake.paths = append(fake.paths, p); return "h1" }
	ar.ownerAlive = ownerProbe(false)
	ensureSettled(t, ar, context.Background(), "svc")
	if fake.ingests != 1 {
		t.Fatalf("ingests for a killed first ingest = %d, want 1", fake.ingests)
	}
	if len(fake.paths) == 0 || fake.paths[0] != "/new/repo" {
		t.Fatalf("drift read corpus roots %v, want /new/repo", fake.paths)
	}
	// No row, no meta: nothing to refresh.
	fake2 := &ingestStateFake{noMeta: true}
	ar2 := newAR(t, fake2, "h1")
	ensureSettled(t, ar2, context.Background(), "svc")
	if fake2.ingests != 0 {
		t.Fatalf("ingests for an unknown service = %d, want 0", fake2.ingests)
	}
}
