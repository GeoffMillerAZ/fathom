package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestDerivedBackfillLoopDerivesEveryService pins RT-160's daemon half:
// after start-up the serving daemon derives each service's summary and
// call index once, from stored rows, reading no file.
func TestDerivedBackfillLoopDerivesEveryService(t *testing.T) {
	ctx := context.Background()
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "b.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, svc := range []string{"a", "b"} {
		_ = st.PutObservations(ctx, []core.Observation{{ServiceID: svc, SourceRef: "x.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "X", Calls: []string{"p.Y"}}})
		_ = st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: svc, IndexedCommit: "c", IndexedAt: 1})
	}
	conn := newCountingConnector()
	c := client.New(conn, st)
	var lines []string
	runDerivedBackfill(ctx, c, func(m string) { lines = append(lines, m) })
	for _, svc := range []string{"a", "b"} {
		ds, _ := st.DerivedState(ctx, svc)
		if !ds.SummaryCurrent || !ds.CallsIndexed {
			t.Fatalf("%s not derived: %+v (log %v)", svc, ds, lines)
		}
	}
	if conn.discoverCalls("a")+conn.discoverCalls("b") != 0 || conn.fetches != 0 {
		t.Fatal("the backfill walked or read a corpus")
	}
	// A second pass writes nothing.
	lines = nil
	runDerivedBackfill(ctx, c, func(m string) { lines = append(lines, m) })
	if len(lines) != 0 {
		t.Fatalf("second pass wrote: %v", lines)
	}
}
