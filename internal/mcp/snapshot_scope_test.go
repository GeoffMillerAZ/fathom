package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestSnapshotSeesSuppressionChangeAndDiscloses (RT-75 with SP-2/SP-3): a
// cached symbol snapshot never serves a row a suppression written since
// hides — the suppression is part of the snapshot's key — and every read
// of a filtered snapshot, the cache hit included, still carries the
// disclosure of what it hides.
func TestSnapshotSeesSuppressionChangeAndDiscloses(t *testing.T) {
	supPath := filepath.Join(t.TempDir(), "suppressions.jsonl")
	t.Setenv(scope.EnvVar, supPath)
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0).UTC()
	if err := st.PutObservations(ctx, []core.Observation{
		{ServiceID: "svc", SourceRef: "a.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Open", IngestedAt: now},
		{ServiceID: "svc", SourceRef: "secret/b.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Hidden", IngestedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	_ = st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", IndexedCommit: "c", IndexedAt: 1})
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	if nodes, err := serviceSymbols(ctx, c, "svc"); err != nil || len(nodes) != 2 {
		t.Fatalf("before any suppression: %d nodes, %v", len(nodes), err)
	}
	if _, err := scope.AppendSuppression(supPath, scope.Suppression{Service: "svc", Pattern: "secret/**", Reason: "test", CreatedAt: time.Now().UTC(), CreatedBy: scope.OriginCLI}); err != nil {
		t.Fatal(err)
	}
	for i := range 2 { // a miss, then a hit
		cctx, col := scope.WithCollector(ctx)
		nodes, err := serviceSymbols(cctx, c, "svc")
		if err != nil || len(nodes) != 1 || nodes[0].Name != "Open" {
			t.Fatalf("read %d after the suppression: %+v, %v", i, nodes, err)
		}
		if d := col.For("svc"); d == nil || d.Count != 1 {
			t.Fatalf("read %d: disclosure = %+v, want the one hidden row named", i, d)
		}
	}
	_ = os.Remove(supPath)
	if nodes, _ := serviceSymbols(ctx, c, "svc"); len(nodes) != 2 {
		t.Fatalf("after the suppression was removed: %d nodes", len(nodes))
	}
}
