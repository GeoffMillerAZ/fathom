package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

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
		tenancy.SafeModeFlag().Store(false)
		tenancy.StartWarmup(time.Time{}, 0)
		tenancy.SetQuarantined(nil)
	})
	cases := []struct {
		name   string
		set    func()
		reason string
	}{
		{"shed", func() { tenancy.PublishLevel(tenancy.LevelShed) }, "deferred (shed)"},
		{"refuse", func() { tenancy.PublishLevel(tenancy.LevelRefuse) }, "deferred (refuse)"},
		{"safe mode", func() { tenancy.SafeModeFlag().Store(true) }, "deferred (safe mode)"},
		{"warm-up", func() { tenancy.StartWarmup(now.Add(-30*time.Second), 120*time.Second) }, "deferred (warm-up"},
		{"quarantine", func() {
			tenancy.SetQuarantined([]tenancy.Quarantine{{Service: "svc", Reason: tenancy.QuarantineReasonIngest}})
		}, "deferred (quarantined: exceeded budget during ingest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tenancy.PublishLevel(tenancy.LevelNormal)
			tenancy.SafeModeFlag().Store(false)
			tenancy.StartWarmup(time.Time{}, 0)
			tenancy.SetQuarantined(nil)
			tc.set()

			fake := &refreshFake{path: "/repo", indexed: "old"}
			ar := newAR(fake, "new")
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

			// The gate lifts: the same drift is re-ingested, with no note.
			tenancy.PublishLevel(tenancy.LevelNormal)
			tenancy.SafeModeFlag().Store(false)
			tenancy.StartWarmup(time.Time{}, 0)
			tenancy.SetQuarantined(nil)
			ctx2, notes2 := withRefreshNotes(context.Background())
			ar.ensure(ctx2, "svc")
			if fake.ingests != 1 {
				t.Fatalf("ingests after the gate lifted = %d, want 1", fake.ingests)
			}
			if r := notes2.render(); r != nil {
				t.Fatalf("refresh notes after a normal ensure = %v, want none", r)
			}
		})
	}

	t.Run("warm-up ends", func(t *testing.T) {
		tenancy.StartWarmup(now.Add(-121*time.Second), 120*time.Second)
		fake := &refreshFake{path: "/repo", indexed: "old"}
		ar := newAR(fake, "new")
		ar.now = func() time.Time { return now }
		ar.ensure(context.Background(), "svc")
		if fake.ingests != 1 {
			t.Fatalf("ingests after warm-up ended = %d, want 1", fake.ingests)
		}
	})

	t.Run("envelope", func(t *testing.T) {
		tenancy.StartWarmup(time.Time{}, 0)
		tenancy.PublishLevel(tenancy.LevelShed)
		defer tenancy.PublishLevel(tenancy.LevelNormal)
		fake := &refreshFake{path: "/repo", indexed: "old"}
		srv := NewServer(newAR(fake, "new"))
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

// ingestStateFake reports an ingest_state row for the service.
type ingestStateFake struct {
	refreshFake
	state string
	pid   int
}

func (f *ingestStateFake) IngestState(_ context.Context, _ string) (string, int, bool, error) {
	return f.state, f.pid, f.state != "", nil
}

// TestEnsureReingestsInterruptedIngest pins RT-109's read half (design §6):
// an in_progress row written by another process (a killed or halted
// ingest — dead or foreign pid) means the index is not the one its
// watermark describes, so ensure re-ingests even when HEAD equals the
// watermark; one written by this process, or a complete row, does not.
func TestEnsureReingestsInterruptedIngest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state string
		pid   int
		want  int
	}{
		{"foreign in_progress", "in_progress", os.Getpid() + 1, 1},
		{"own in_progress", "in_progress", os.Getpid(), 0},
		{"complete", "complete", os.Getpid() + 1, 0},
		{"no row", "", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &ingestStateFake{refreshFake: refreshFake{path: "/repo", indexed: "same"}, state: tc.state, pid: tc.pid}
			ar := newAR(fake, "same")
			ar.ensure(context.Background(), "svc")
			ar.ensure(context.Background(), "svc")
			if fake.ingests != tc.want {
				t.Fatalf("ingests = %d, want %d", fake.ingests, tc.want)
			}
		})
	}
}
