package client_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// summaryFixture ingests one Go file into a SQLite store at path and then
// deletes its summary row — a store written before RT-76.
func summaryFixture(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n\nfunc A() {}\n\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(t.TempDir(), "s.sqlite")
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	if _, err := c.Ingest(context.Background(), client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM service_summary`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	return dir, path
}

// TestServiceSummaryOnAReadOnlyMount (review finding, RT-79): a read-only
// mount cannot write the backfilled row, and list_services then answered
// symbol_count_error where it used to answer a count. The summary is
// derived without writing and served (it is still the store's count at
// its watermark).
func TestServiceSummaryOnAReadOnlyMount(t *testing.T) {
	_, path := summaryFixture(t)
	ro, err := store.MountReadOnly(path, store.LevelL0)
	if err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), ro)
	t.Cleanup(func() { _ = c.Close() })
	sum, err := c.ServiceSummary(context.Background(), "svc")
	if err != nil {
		t.Fatalf("read-only mount: %v; want the summary derived without a write", err)
	}
	if sum.SymbolCount < 2 {
		t.Fatalf("symbol count = %d, want at least 2", sum.SymbolCount)
	}
}

// TestServiceSummaryNeverDerivedMidIngest (review finding, RT-160): the
// ingest slot is per process, so a list_services in another process could
// backfill a service while an ingest of it is between commits, and write
// a mixed count under the old watermark as current. The store's own
// ingest_state is the cross-process signal: a service whose ingest is in
// progress (or was interrupted) is named absent, never half-counted.
func TestServiceSummaryNeverDerivedMidIngest(t *testing.T) {
	dir, path := summaryFixture(t)
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st.SetIngestState(ctx, store.IngestState{ServiceID: "svc", State: store.IngestInProgress, PID: 1, Path: dir}); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	t.Cleanup(func() { _ = c.Close() })
	_, err = c.ServiceSummary(ctx, "svc")
	if !errors.Is(err, client.ErrSummaryPending) || !strings.Contains(err.Error(), "ingest") {
		t.Fatalf("mid-ingest: err %v; want a named absence", err)
	}
	if _, ok, _ := st.ServiceSummary(ctx, "svc"); ok {
		t.Fatal("a summary row was written while the service's ingest was in progress")
	}
}
