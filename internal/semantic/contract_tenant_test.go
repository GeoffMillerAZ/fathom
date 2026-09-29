package semantic

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// fullLoadCountingStore counts whole-service observation loads
// (QueryObservations with no concern) — the read the obligations hot path
// must never make (RT-174).
type fullLoadCountingStore struct {
	store.Store
	fullLoads atomic.Int64
}

func (s *fullLoadCountingStore) QueryObservations(ctx context.Context, serviceID, concernID string) ([]core.Observation, error) {
	if concernID == "" {
		s.fullLoads.Add(1)
	}
	return s.Store.QueryObservations(ctx, serviceID, concernID)
}

// TestObligationsSymbolOnlyContractNeverLoadsService is RT-174: a served
// claims-role claim citing a symbol-only plan row ({id, symbol} with no
// package) resolved that symbol's file by loading every observation in
// the service, on every file-scoped obligations call — the default rules
// layer included, since claims-role claims are scoped before the layer
// split. It now resolves only the cited names, and still binds the row to
// its symbol's defining file.
func TestObligationsSymbolOnlyContractNeverLoadsService(t *testing.T) {
	ctx := context.Background()
	base := bpStore(t)
	obs := []core.Observation{
		{ServiceID: "svc", SourceRef: "internal/semantic/serve.go:40", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "ObligationsFor", Package: "internal/semantic", Line: 40},
		{ServiceID: "svc", SourceRef: "internal/query/engine.go:9", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Engine", Package: "internal/query", Line: 9},
	}
	if err := base.PutObservations(ctx, obs); err != nil {
		t.Fatal(err)
	}
	seedRoleClaim(t, base, "sym-1", "bp-claims", "Row sym-1 binds to its symbol's file.",
		[]string{"**"}, core.ClaimFresh, "docs/plans/sym.md#L3-L3",
		"- [ ] work {id: sym-1, symbol: ObligationsFor}")
	st := &fullLoadCountingStore{Store: base}

	got, err := ObligationsForLayer(ctx, st, "svc", "internal/semantic/serve.go", 0, ObligationLayerAll)
	if err != nil {
		t.Fatalf("ObligationsForLayer: %v", err)
	}
	if len(got.Rules) != 1 {
		t.Errorf("symbol's own file: got %d rules, want the symbol-bound row: %+v", len(got.Rules), got.Rules)
	}
	got, err = ObligationsForLayer(ctx, st, "svc", "internal/query/engine.go", 0, ObligationLayerAll)
	if err != nil {
		t.Fatalf("ObligationsForLayer: %v", err)
	}
	if len(got.Rules) != 0 {
		t.Errorf("another file: got %d rules, want none: %+v", len(got.Rules), got.Rules)
	}
	if _, err := ObligationsFor(ctx, st, "svc", "internal/semantic/serve.go", 0); err != nil {
		t.Fatal(err)
	}
	if n := st.fullLoads.Load(); n != 0 {
		t.Errorf("obligations loaded the whole service's observations %d times; want 0", n)
	}
}

// TestCitedTextResolverStatsBeforeReading (RT-174): the legacy-citation
// re-read read the whole cited file before checking it against
// citedFileMaxBytes. It now stats first and never opens a file that is not
// regular or is over the limit — a FIFO at the cited path would block a
// read-first resolver forever.
func TestCitedTextResolverStatsBeforeReading(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "docs", "plans", "fifo.md"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	big := make([]byte, citedFileMaxBytes+1)
	for i := range big {
		big[i] = '\n'
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "plans", "big.md"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "plans", "ok.md"), []byte("a\n- [ ] x {id: A-1, package: pkg}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolve := citedTextResolver(root, nil)
	done := make(chan [3]string, 1)
	go func() {
		done <- [3]string{
			resolve("docs/plans/fifo.md#L1-L1", "c1"),
			resolve("docs/plans/big.md#L1-L1", "c2"),
			resolve("docs/plans/ok.md#L2-L2", "c3"),
		}
	}()
	select {
	case got := <-done:
		if got[0] != "" || got[1] != "" {
			t.Errorf("non-regular or oversize citation resolved to %q / %q, want \"\"", got[0], got[1])
		}
		if got[2] != "- [ ] x {id: A-1, package: pkg}" {
			t.Errorf("regular citation = %q", got[2])
		}
	case <-time.After(waitSafety):
		t.Fatal("citedTextResolver blocked reading a FIFO: it reads before it stats")
	}
}
