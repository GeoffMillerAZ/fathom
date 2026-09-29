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
	fetches  int
	refs     map[string][]core.SourceRef
	content  map[string][]byte
	// gate, when set, blocks Discover until closed — to hold concurrent
	// cold callers inside the walk together.
	gate chan struct{}
	// honourCtx, when set, makes Discover fail with ctx.Err() once its ctx
	// has ended, as the local connector's walk does.
	honourCtx bool
}

func newCountingConnector() *countingConnector {
	return &countingConnector{
		discover: make(map[string]int),
		refs:     make(map[string][]core.SourceRef),
		content:  make(map[string][]byte),
	}
}

func (c *countingConnector) Discover(ctx context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	c.mu.Lock()
	c.discover[svc.ID]++
	gate := c.gate
	refs := c.refs[svc.ID]
	honour := c.honourCtx
	c.mu.Unlock()
	if gate != nil {
		<-gate
	}
	if honour && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return refs, nil
}

func (c *countingConnector) Fetch(_ context.Context, ref core.SourceRef) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fetches++
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

// addZigFile registers one unsupported-language (".zig", per
// internal/connector.unsupportedLangExtMap) source file for service so a
// successful Discover/Fetch round trip has something to classify.
func (c *countingConnector) addZigFile(service, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refs[service] = append(c.refs[service], core.SourceRef{Service: service, Path: path, Kind: core.SourceKind("unclassified")})
	c.content[path] = []byte("const std = @import(\"std\");\n")
}

func newTestFleetAdapter(conn *countingConnector, st store.Store) *fleetAdapter {
	c := client.New(conn, st)
	return newFleetAdapter(c, dlog.New(io.Discard, dlog.LevelDebug, time.Now))
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
	fa.SetLog(dlog.New(&buf, dlog.LevelDebug, time.Now))

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
	fa.SetLog(dlog.New(&buf, dlog.LevelDebug, time.Now))

	fa.SymbolsFor("demo")
	line := buf.String()
	if strings.Contains(strings.ToUpper(line), "CANTOPEN") || strings.Contains(line, "store-locked") {
		t.Errorf("log line %q wrongly tagged a generic error as store-locked/CANTOPEN", line)
	}
	if !strings.Contains(line, "disk full") {
		t.Errorf("log line %q lost the underlying error text", line)
	}
}

// 2026-09-26 incident: list_services answered "which languages here are
// unsupported" through client.Sources, which FETCHES every file's contents.
// On a cold cache every concurrent caller read every repository into memory
// at once — the daemon reached ~30 GB in minutes and the operator
// force-quit it. The answer depends on paths only, so a read must never
// fetch a file, and concurrent cold callers share one walk per service.
// TestFleetAdapter_UnsupportedLanguagesFor_NeverReadsFileContents (RT-79):
// unsupported languages come from the service's summary row — derived at
// first use from the fingerprinted paths by NAME — never from a walk or a
// fetch; repeated reads at one watermark derive it once.
func TestFleetAdapter_UnsupportedLanguagesFor_NeverReadsFileContents(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	conn := newCountingConnector()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "demo", Path: t.TempDir(), IndexedCommit: "c1", IndexedAt: 1}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"demo/a.zig", "demo/b.zig", "demo/c.go"} {
		_ = st.SetFingerprint(ctx, "demo", p, "h")
	}
	conn.addZigFile("demo", "demo/a.zig")
	fa := newTestFleetAdapter(conn, st)
	for range 5 {
		if got := fa.UnsupportedLanguagesFor("demo"); got["zig"] != 2 || len(got) != 1 {
			t.Fatalf("UnsupportedLanguagesFor = %v, want {zig:2}", got)
		}
	}
	conn.mu.Lock()
	fetches := conn.fetches
	conn.mu.Unlock()
	if fetches != 0 || conn.discoverCalls("demo") != 0 {
		t.Fatalf("a list_services read walked (%d) or fetched (%d); it reads the summary row only", conn.discoverCalls("demo"), fetches)
	}
	sum, _, _ := st.ServiceSummary(ctx, "demo")
	if sum.Source != store.SummaryFromBackfill {
		t.Fatalf("summary = %+v, want one backfilled row", sum)
	}
}
