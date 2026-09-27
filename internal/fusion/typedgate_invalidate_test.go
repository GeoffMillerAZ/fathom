package fusion

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestFingerprintWriteInvalidatesPassState is RT-104/RT-105 (design §7.2,
// "changed since the last successful pass"): whatever writes a pass input's
// new fingerprint — the one-file refresh, or an ingest that fails after its
// streamed writes — first removes that pass's success record, so the next
// whole ingest cannot mistake the input for unchanged and skip the pass.
func TestFingerprintWriteInvalidatesPassState(t *testing.T) {
	ctx := context.Background()

	t.Run("refresh-file then ingest", func(t *testing.T) {
		dir := gateFixture(t)
		st := store.NewMemoryStore()
		p := New(connector.NewLocalConnector(), st, Config{})
		gateIngest(t, p, dir)
		goBefore, cueBefore := p.TypedPassTotal(), p.CUEPassTotal()

		appendTo(t, filepath.Join(dir, "a.go"), "\nfunc C() int { return A() }\n")
		if _, err := p.ReingestFile(ctx, st, "gate", dir, "a.go"); err != nil {
			t.Fatal(err)
		}
		appendTo(t, filepath.Join(dir, "pkgone", "base.cue"), "\n#Later: #Base\n")
		if _, err := p.ReingestFile(ctx, st, "gate", dir, "pkgone/base.cue"); err != nil {
			t.Fatal(err)
		}
		rep := gateIngest(t, p, dir)
		if got := p.TypedPassTotal(); got != goBefore+1 || rep.TypedSkipped != "" {
			t.Fatalf("typed passes %d -> %d, skipped %q: a Go edit fingerprinted by /refresh-file was hidden from the gate", goBefore, got, rep.TypedSkipped)
		}
		if got := p.CUEPassTotal(); got != cueBefore+1 || rep.CUESkipped != "" {
			t.Fatalf("CUE passes %d -> %d, skipped %q: a .cue edit fingerprinted by /refresh-file was hidden from the gate", cueBefore, got, rep.CUESkipped)
		}

		// An unchanged file's one-file refresh changes nothing: the gate stands.
		if _, err := p.ReingestFile(ctx, st, "gate", dir, "a.go"); err != nil {
			t.Fatal(err)
		}
		rep = gateIngest(t, p, dir)
		if got := p.TypedPassTotal(); got != goBefore+1 || rep.TypedSkipped == "" {
			t.Fatalf("a no-op refresh of an unchanged Go file re-ran the typed pass (total %d)", got)
		}
	})

	t.Run("ingest fails after its streamed writes", func(t *testing.T) {
		dir := gateFixture(t)
		st := &failListRefsStore{Store: store.NewMemoryStore()}
		p := New(connector.NewLocalConnector(), st, Config{})
		gateIngest(t, p, dir)
		before := p.TypedPassTotal()

		appendTo(t, filepath.Join(dir, "a.go"), "\nfunc C() int { return A() }\n")
		st.failNext.Store(true)
		if _, err := p.Ingest(ctx, core.ServiceRef{ID: "gate", Path: dir}); err == nil {
			t.Fatal("the injected ListSourceRefs failure did not fail the ingest")
		}
		// A fresh pipeline (a restarted daemon) retries: no in-memory memo helps.
		p2 := New(connector.NewLocalConnector(), st, Config{})
		rep := gateIngest(t, p2, dir)
		if p2.TypedPassTotal() != 1 || rep.TypedSkipped != "" {
			t.Fatalf("retry after a failed ingest: typed passes %d, skipped %q; the Go change was lost (first pipeline ran %d)", p2.TypedPassTotal(), rep.TypedSkipped, before)
		}
	})
}

type failListRefsStore struct {
	store.Store
	failNext atomic.Bool
}

func (s *failListRefsStore) ListSourceRefs(ctx context.Context, serviceID string) ([]string, error) {
	if s.failNext.CompareAndSwap(true, false) {
		return nil, errors.New("injected list failure")
	}
	return s.Store.ListSourceRefs(ctx, serviceID)
}
