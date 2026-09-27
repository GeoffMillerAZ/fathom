package fusion_test

// allfailed_incomplete_test.go — F2 of the waves C–F live proof
// (docs/bench/readings/rt-waves-c-f-2026-09-27.md §6): an ingest in which
// every discovered file failed to read, over a service with no evidence to
// protect (a first ingest), was recorded complete at HEAD — service_meta
// indexed_commit = HEAD, ingest_state = complete, 0 fingerprints — so
// auto-refresh saw no drift and served an empty index as fresh. RT-109: the
// watermark advances only at the final commit, and an ingest that indexed
// nothing is not one.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngestAllReadsFailedNeverRecordsComplete(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	root := t.TempDir()
	const head = "0123456789abcdef0123456789abcdef01234567"
	refs := []core.SourceRef{goRef("a.go"), goRef("b.go")}

	wedged := &readErrConn{refs: refs, fail: map[string]error{"a.go": eperm("a.go"), "b.go": eperm("b.go")}}
	p := newHonestyPipeline(wedged, st)
	p.WithGitHead(func(string) string { return head })
	report, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root})

	var rfe *fusion.ReadFailuresError
	if !errors.As(err, &rfe) || !rfe.Incomplete {
		t.Fatalf("Ingest error = %v, want an Incomplete *fusion.ReadFailuresError", err)
	}
	if report.ReadFailed != 2 {
		t.Fatalf("ReadFailed = %d, want 2", report.ReadFailed)
	}
	if !strings.Contains(rfe.Error(), "not recorded complete") || !strings.Contains(rfe.Error(), "a.go") {
		t.Fatalf("the failure must be named: %q", rfe.Error())
	}
	if meta, found, _ := st.GetServiceMeta(ctx, "svc"); found && meta.IndexedCommit != "" {
		t.Fatalf("an ingest that indexed nothing advanced the watermark to %q", meta.IndexedCommit)
	}
	row, found, _ := st.GetIngestState(ctx, "svc")
	if !found || row.State != store.IngestInProgress {
		t.Fatalf("ingest_state = %+v (found %v), want in_progress: the ingest must stay retriable", row, found)
	}
	if row.Path == "" {
		t.Fatal("the in_progress row must name the corpus root, so the next process can resume it")
	}

	// The retry is not stat-skipped: every file is read and the ingest
	// commits once the reader recovers.
	good := &readErrConn{refs: refs, content: map[string][]byte{"a.go": []byte(goSrc), "b.go": []byte(goSrc)}}
	p2 := newHonestyPipeline(good, st)
	p2.WithGitHead(func(string) string { return head })
	report, err = p2.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root})
	if err != nil || report.Extracted != 2 {
		t.Fatalf("retry = extracted %d, err %v; want 2 files extracted", report.Extracted, err)
	}
	if meta, _, _ := st.GetServiceMeta(ctx, "svc"); meta.IndexedCommit != head {
		t.Fatalf("after a good retry the watermark = %q, want %q", meta.IndexedCommit, head)
	}
	if row, _, _ := st.GetIngestState(ctx, "svc"); row.State != store.IngestComplete {
		t.Fatalf("after a good retry ingest_state = %q, want complete", row.State)
	}
}

// An ingest that read at least one file is honest progress (IT-15): it
// still commits — only an ingest that indexed nothing is held back.
func TestIngestPartialReadFailureStillCommits(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	conn := &readErrConn{
		refs:    []core.SourceRef{goRef("a.go"), goRef("b.go")},
		content: map[string][]byte{"a.go": []byte(goSrc)},
		fail:    map[string]error{"b.go": eperm("b.go")},
	}
	p := newHonestyPipeline(conn, st)
	p.WithGitHead(func(string) string { return "feed" })
	_, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: t.TempDir()})
	var rfe *fusion.ReadFailuresError
	if !errors.As(err, &rfe) || rfe.Incomplete || rfe.Refused {
		t.Fatalf("Ingest error = %v, want a degraded (not refused, not incomplete) *fusion.ReadFailuresError", err)
	}
	if row, _, _ := st.GetIngestState(ctx, "svc"); row.State != store.IngestComplete {
		t.Fatalf("ingest_state = %q, want complete", row.State)
	}
}
