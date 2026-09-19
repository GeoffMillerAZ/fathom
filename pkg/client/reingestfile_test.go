package client_test

// reingestfile_test.go — SU-5/SU-6 (docs/plans/substrate.md §A, ADR-041
// decisions 3 and 4): Client.ReingestFile is the pkg/client-level wiring
// the daemon's POST /refresh-file route (cmd/fathom) and the pre-edit hook
// both call through.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestClient_ReingestFile_ReplacesOneFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte("package main\n\nfunc Original() int { return 1 }\n"), 0o644); err != nil {
		t.Fatalf("write target.go: %v", err)
	}

	conn := connector.NewLocalConnector()
	ms := store.NewMemoryStore()
	c := client.New(conn, ms)
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "reingest-client-svc", Path: dir}); err != nil {
		t.Fatalf("initial Ingest: %v", err)
	}

	if err := os.WriteFile(target, []byte("package main\n\nfunc Changed() int { return 2 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite target.go: %v", err)
	}

	report, err := c.ReingestFile(ctx, "reingest-client-svc", "target.go")
	if err != nil {
		t.Fatalf("ReingestFile: %v", err)
	}
	if !report.Refreshed || report.Symbols != 1 {
		t.Fatalf("report = %+v, want Refreshed=true Symbols=1", report)
	}

	obs, err := c.Observations(ctx, "reingest-client-svc")
	if err != nil {
		t.Fatalf("Observations: %v", err)
	}
	found := false
	for _, o := range obs {
		if o.Symbol == "Original" {
			t.Fatalf("Original still present after ReingestFile: %+v", obs)
		}
		if o.Symbol == "Changed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Changed missing after ReingestFile: %+v", obs)
	}
}

func TestClient_ReingestFile_UnknownServiceErrors(t *testing.T) {
	conn := connector.NewLocalConnector()
	ms := store.NewMemoryStore()
	c := client.New(conn, ms)
	t.Cleanup(func() { _ = c.Close() })

	if _, err := c.ReingestFile(context.Background(), "never-ingested", "x.go"); err == nil {
		t.Fatal("expected an error for a service with no recorded ServiceMeta, got nil")
	}
}
