package connector_test

// fetchcap_test.go — RT-69 (docs/design/responsible-tenant.md §4.2): the
// local connector never reads a file over the 2 MiB cap and honours ctx in
// discovery and fetch. The 2026-09-26 incident's pass-2 held app-kit's
// 3.5 GiB of files over 2 MiB in memory only to discard them unextracted.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestFetch_NeverReadsOversize(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "small.go"), "package x\n")
	big := filepath.Join(root, "big.js")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	// Sparse: 3 MiB of apparent size, no real bytes written.
	if err := f.Truncate(3 << 20); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	// Unreadable: a read attempt fails with EACCES, so an OversizeError (not
	// a permission error) proves the connector decided from the stat alone.
	if err := os.Chmod(big, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(big, 0o644) })

	c := connector.NewLocalConnector()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "svc", Path: root}
	if _, err := c.Discover(ctx, svc); err != nil {
		t.Fatal(err)
	}

	data, err := c.Fetch(ctx, core.SourceRef{Service: "svc", Path: "big.js"})
	var over *connector.OversizeError
	if !errors.As(err, &over) {
		t.Fatalf("Fetch(big.js) = %d bytes, err %v; want *OversizeError decided from stat", len(data), err)
	}
	if over.Size != 3<<20 || over.Cap != connector.MaxFetchSize {
		t.Fatalf("OversizeError = %+v, want size %d cap %d", over, 3<<20, connector.MaxFetchSize)
	}
	if data != nil {
		t.Fatalf("an oversize fetch returned %d bytes; it must return none", len(data))
	}

	got, err := c.Fetch(ctx, core.SourceRef{Service: "svc", Path: "small.go"})
	if err != nil || string(got) != "package x\n" {
		t.Fatalf("Fetch(small.go) = %q, %v", got, err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := c.Fetch(cancelled, core.SourceRef{Service: "svc", Path: "small.go"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch under a cancelled ctx = %v, want context.Canceled", err)
	}
	if _, err := c.Discover(cancelled, svc); !errors.Is(err, context.Canceled) {
		t.Fatalf("Discover under a cancelled ctx = %v, want context.Canceled", err)
	}
}
