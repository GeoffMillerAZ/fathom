package connector_test

// root_clobber_test.go — the waves C–F live proof's F1
// (docs/bench/readings/rt-waves-c-f-2026-09-27.md §6): a read on a service
// that is still being onboarded made the onboarding ingest fail for every
// file with "stat <service>/<path>: no such file or directory". The read
// has no service_meta path yet, so pkg/client resolves the corpus as the
// bare service ID and calls Discover with it; Discover recorded that root
// in the connector's process-wide per-service map BEFORE walking it, so
// the in-flight ingest's Stat and Fetch resolved every file against the
// service ID. A Discover that fails must leave the recorded root alone.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestFailedDiscoverKeepsRecordedRoot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := connector.NewLocalConnector()
	refs, err := c.Discover(ctx, core.ServiceRef{ID: "onboarding-svc", Path: root})
	if err != nil || len(refs) != 1 {
		t.Fatalf("Discover(root) = %v, %v; want one ref", refs, err)
	}

	// The concurrent read: no service_meta path yet, so the corpus is the
	// bare service ID — a directory that does not exist.
	if _, err := c.Discover(ctx, core.ServiceRef{ID: "onboarding-svc", Path: "onboarding-svc"}); err == nil {
		t.Fatal("Discover of a root that does not exist must fail")
	}

	if _, err := c.Stat(ctx, refs[0]); err != nil {
		t.Fatalf("Stat after a failed Discover resolved against the wrong root: %v", err)
	}
	if _, err := c.Fetch(ctx, refs[0]); err != nil {
		t.Fatalf("Fetch after a failed Discover resolved against the wrong root: %v", err)
	}
}
