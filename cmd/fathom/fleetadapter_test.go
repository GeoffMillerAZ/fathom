package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// countingConnector is a connector.Connector fake that counts Discover
// calls per service — the live filesystem walk DG-18's cache exists to
// avoid paying on every read.
type countingConnector struct {
	mu       sync.Mutex
	discover map[string]int
	refs     map[string][]core.SourceRef
	content  map[string][]byte
}

func newCountingConnector() *countingConnector {
	return &countingConnector{
		discover: make(map[string]int),
		refs:     make(map[string][]core.SourceRef),
		content:  make(map[string][]byte),
	}
}

func (c *countingConnector) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.discover[svc.ID]++
	return c.refs[svc.ID], nil
}

func (c *countingConnector) Fetch(_ context.Context, ref core.SourceRef) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.content[ref.Path], nil
}

func (c *countingConnector) Watermark(_ context.Context, _ core.ServiceRef) (string, error) {
	return "", nil
}

func (c *countingConnector) discoverCalls(service string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.discover[service]
}

// addSwiftFile registers one unsupported-language (".swift", per
// internal/connector.unsupportedLangExtMap) source file for service so a
// successful Discover/Fetch round trip has something to classify.
func (c *countingConnector) addSwiftFile(service, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refs[service] = append(c.refs[service], core.SourceRef{Service: service, Path: path, Kind: core.SourceKind("unclassified")})
	c.content[path] = []byte("import Foundation\n")
}

func newTestFleetAdapter(conn *countingConnector, st store.Store) *fleetAdapter {
	c := client.New(conn, st)
	return &fleetAdapter{c: c, log: dlog.New(io.Discard, dlog.LevelDebug, time.Now)}
}

// TestFleetAdapter_UnsupportedLanguagesFor_MissingCorpusSkipsRewalk pins
// DG-3/DG-18's root-cause fix: a service whose recorded corpus root no
// longer exists is detected with a single os.Stat and never reaches
// client.Sources (connector.Discover) at all — no re-walk that can only
// ever lstat-fail. RootMissing reports it structurally, with a remedy
// naming how to drop the stale record.
func TestFleetAdapter_UnsupportedLanguagesFor_MissingCorpusSkipsRewalk(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{
		ServiceID:     "demo",
		Path:          "/does/not/exist/anywhere",
		IndexedCommit: "abc123",
		IndexedAt:     1,
	}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	conn := newCountingConnector()
	fa := newTestFleetAdapter(conn, st)

	if got := fa.UnsupportedLanguagesFor("demo"); got != nil {
		t.Errorf("UnsupportedLanguagesFor(missing root) = %v, want nil", got)
	}
	if n := conn.discoverCalls("demo"); n != 0 {
		t.Errorf("Discover called %d times for a missing root, want 0 (no re-walk)", n)
	}

	missing, remedy := fa.RootMissing("demo")
	if !missing {
		t.Fatal("RootMissing(demo) = false, want true")
	}
	if !strings.Contains(remedy, "remove_service") || !strings.Contains(remedy, "fathom fleet") {
		t.Errorf("remedy %q does not name remove_service / fathom fleet", remedy)
	}
}

// TestFleetAdapter_UnsupportedLanguagesFor_CallsDiscoverAtMostOncePerServiceWatermark
// is DG-18's own contract test: list_services over N services calls
// Discover at most once per (service, watermark) across repeated calls —
// the fix for list_services being the fleet's most-called MCP tool (~31k
// calls measured, p95 4.6s, max 182s) purely because this re-walked every
// service's source tree on every read.
func TestFleetAdapter_UnsupportedLanguagesFor_CallsDiscoverAtMostOncePerServiceWatermark(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	conn := newCountingConnector()
	root := t.TempDir()

	const n = 5
	services := make([]string, n)
	for i := 0; i < n; i++ {
		svc := "svc-" + string(rune('a'+i))
		services[i] = svc
		if err := st.SetServiceMeta(ctx, store.ServiceMeta{
			ServiceID:     svc,
			Path:          root,
			IndexedCommit: "commit-" + svc,
			IndexedAt:     1,
		}); err != nil {
			t.Fatalf("SetServiceMeta(%s): %v", svc, err)
		}
		conn.addSwiftFile(svc, svc+"/main.swift")
	}
	fa := newTestFleetAdapter(conn, st)

	// Simulate 20 repeated list_services-style reads.
	const reads = 20
	for r := 0; r < reads; r++ {
		for _, svc := range services {
			got := fa.UnsupportedLanguagesFor(svc)
			if got["swift"] != 1 {
				t.Fatalf("read %d: UnsupportedLanguagesFor(%s) = %v, want {swift:1}", r, svc, got)
			}
		}
	}

	for _, svc := range services {
		if calls := conn.discoverCalls(svc); calls != 1 {
			t.Errorf("Discover called %d times for %s across %d reads, want at most 1 per watermark", calls, svc, reads)
		}
	}
}

// TestFleetAdapter_UnsupportedLanguagesFor_RecomputesOnWatermarkChange pins
// that the cache is keyed on watermark, not pinned forever: once a fresh
// ingest changes the service's StoreWatermark (a new IndexedCommit), the
// next read recomputes exactly once, not on every subsequent read.
func TestFleetAdapter_UnsupportedLanguagesFor_RecomputesOnWatermarkChange(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	conn := newCountingConnector()
	root := t.TempDir()
	meta := store.ServiceMeta{ServiceID: "demo", Path: root, IndexedCommit: "v1", IndexedAt: 1}
	if err := st.SetServiceMeta(ctx, meta); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	conn.addSwiftFile("demo", "demo/main.swift")
	fa := newTestFleetAdapter(conn, st)

	fa.UnsupportedLanguagesFor("demo")
	fa.UnsupportedLanguagesFor("demo")
	if calls := conn.discoverCalls("demo"); calls != 1 {
		t.Fatalf("Discover called %d times before any re-ingest, want 1", calls)
	}

	// A fresh ingest replaces service_meta with a new IndexedCommit — the
	// watermark moves.
	meta.IndexedCommit = "v2"
	meta.IndexedAt = 2
	if err := st.SetServiceMeta(ctx, meta); err != nil {
		t.Fatalf("SetServiceMeta (re-ingest): %v", err)
	}

	fa.UnsupportedLanguagesFor("demo")
	fa.UnsupportedLanguagesFor("demo")
	if calls := conn.discoverCalls("demo"); calls != 2 {
		t.Errorf("Discover called %d times after a watermark change plus repeats, want 2 (one recompute, then cached)", calls)
	}
}

// TestFleetAdapter_UnsupportedLanguagesFor_DedupesRepeatedFailures pins
// that a persistently failing Sources call (a non-root-missing error) also
// collapses to one Discover attempt per watermark — DG-2's Dedupe means a
// human reading the log sees one line, not a burst; DG-18's cache means
// the daemon does not re-walk to produce even that one line more than
// once per watermark.
func TestFleetAdapter_UnsupportedLanguagesFor_DedupesRepeatedFailures(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	conn := newCountingConnector()
	root := t.TempDir()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{
		ServiceID: "flaky", Path: root, IndexedCommit: "v1", IndexedAt: 1,
	}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	// No files registered in conn.refs["flaky"] means Discover succeeds
	// with zero refs (not a Sources error) — to exercise the FAILURE path
	// instead, use a fake connector that always errors on Discover for
	// this one service.
	failing := &alwaysFailDiscoverConnector{countingConnector: conn, failService: "flaky"}
	c := client.New(failing, st)
	fa := &fleetAdapter{c: c, log: dlog.New(io.Discard, dlog.LevelDebug, time.Now)}

	for i := 0; i < 5; i++ {
		if got := fa.UnsupportedLanguagesFor("flaky"); got != nil {
			t.Errorf("call %d: got %v, want nil on a Sources failure", i, got)
		}
	}
	if calls := conn.discoverCalls("flaky"); calls != 1 {
		t.Errorf("Discover called %d times across 5 failing reads at one watermark, want 1", calls)
	}
}

// alwaysFailDiscoverConnector wraps countingConnector, returning an error
// from Discover for one named service (still counting the attempt) while
// delegating everything else.
type alwaysFailDiscoverConnector struct {
	*countingConnector
	failService string
}

func (c *alwaysFailDiscoverConnector) Discover(ctx context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	c.countingConnector.mu.Lock()
	c.countingConnector.discover[svc.ID]++
	c.countingConnector.mu.Unlock()
	if svc.ID == c.failService {
		return nil, errors.New("connector: simulated discover failure")
	}
	return c.countingConnector.refs[svc.ID], nil
}

// errInjectingStore wraps store.MemoryStore, overriding QueryObservations
// to return a caller-chosen error — used to simulate a raw SQLite
// CANTOPEN-shaped failure without a real corrupted database file.
type errInjectingStore struct {
	*store.MemoryStore
	queryObservationsErr error
}

func (s *errInjectingStore) QueryObservations(ctx context.Context, serviceID, concernID string) ([]core.Observation, error) {
	if s.queryObservationsErr != nil {
		return nil, s.queryObservationsErr
	}
	return s.MemoryStore.QueryObservations(ctx, serviceID, concernID)
}

// TestSymbolsFor_TagsCantopenDistinctly pins DG-17: a SQLITE_CANTOPEN-shaped
// store error is tagged as "store-locked (possibly transient...)" rather
// than logged identically to any other store failure.
func TestSymbolsFor_TagsCantopenDistinctly(t *testing.T) {
	cantopen := errors.New(`sqlite: unable to open database file (SQLITE_CANTOPEN)`)
	st := &errInjectingStore{MemoryStore: store.NewMemoryStore(), queryObservationsErr: cantopen}
	conn := newCountingConnector()
	fa := newTestFleetAdapter(conn, st)

	var buf strings.Builder
	fa.log = dlog.New(&buf, dlog.LevelDebug, time.Now)

	if got := fa.SymbolsFor("demo"); got != nil {
		t.Errorf("SymbolsFor on a store error = %v, want nil", got)
	}
	line := buf.String()
	if !strings.Contains(strings.ToUpper(line), "CANTOPEN") {
		t.Errorf("log line %q does not name CANTOPEN", line)
	}
	if !strings.Contains(line, "store-locked") {
		t.Errorf("log line %q does not tag the store-locked (transient) class distinctly", line)
	}
}

// TestSymbolsFor_GenericErrorNotTaggedCantopen pins the negative case: an
// ordinary store failure is NOT mislabeled as a transient CANTOPEN
// condition — classification only fires for the actual SQLite class.
func TestSymbolsFor_GenericErrorNotTaggedCantopen(t *testing.T) {
	generic := errors.New("boom: disk full")
	st := &errInjectingStore{MemoryStore: store.NewMemoryStore(), queryObservationsErr: generic}
	conn := newCountingConnector()
	fa := newTestFleetAdapter(conn, st)

	var buf strings.Builder
	fa.log = dlog.New(&buf, dlog.LevelDebug, time.Now)

	fa.SymbolsFor("demo")
	line := buf.String()
	if strings.Contains(strings.ToUpper(line), "CANTOPEN") || strings.Contains(line, "store-locked") {
		t.Errorf("log line %q wrongly tagged a generic error as store-locked/CANTOPEN", line)
	}
	if !strings.Contains(line, "disk full") {
		t.Errorf("log line %q lost the underlying error text", line)
	}
}
