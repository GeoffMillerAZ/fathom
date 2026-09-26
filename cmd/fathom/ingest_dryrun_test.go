package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// fetchCountingConnector wraps a LocalConnector and counts Fetch calls.
type fetchCountingConnector struct {
	*connector.LocalConnector
	fetches atomic.Int64
}

func (c *fetchCountingConnector) Fetch(ctx context.Context, ref core.SourceRef) ([]byte, error) {
	c.fetches.Add(1)
	return c.LocalConnector.Fetch(ctx, ref)
}

// RT-70 (docs/design/responsible-tenant.md §4.2): `fathom ingest --dry-run
// --stats` measures what an ingest would discover — files, bytes, what is
// over the read cap, what the default cache list prunes — from stats alone:
// it never reads a file's content and never opens the store.
func TestIngestDryRunStatsReadsNoContent(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("a.go", "package a\n") // 10 bytes
	mustWrite(".cache/x.js", "cached\n")
	big := filepath.Join(dir, "big.js")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(3 << 20); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	// Unreadable: only a stat can size it.
	if err := os.Chmod(big, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(big, 0o644) })

	conn := &fetchCountingConnector{LocalConnector: connector.NewLocalConnector()}
	var out bytes.Buffer
	if code := runIngestDryRunStats(context.Background(), conn, core.ServiceRef{ID: "s", Path: dir}, &out, true); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if n := conn.fetches.Load(); n != 0 {
		t.Fatalf("the dry run fetched %d file(s); it must stat only", n)
	}
	var got ingestDryRunStats
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse %q: %v", out.String(), err)
	}
	if got.Files != 2 || got.Bytes != 10+3<<20 {
		t.Fatalf("files=%d bytes=%d, want 2 and %d", got.Files, got.Bytes, 10+3<<20)
	}
	if got.OversizeFiles != 1 || got.OversizeBytes != 3<<20 {
		t.Fatalf("oversize files=%d bytes=%d, want 1 and %d", got.OversizeFiles, got.OversizeBytes, 3<<20)
	}
	if !reflect.DeepEqual(got.DefaultIgnoredDirs, []string{".cache"}) {
		t.Fatalf("default_ignored_dirs = %v, want [.cache]", got.DefaultIgnoredDirs)
	}
	if !got.DryRun {
		t.Fatal("dry_run must be true")
	}

	// Through the verb: nothing is written, not even the store file.
	storePath := filepath.Join(t.TempDir(), "never.sqlite")
	var stdout, stderr bytes.Buffer
	if code := cmdIngest([]string{"--service", "s", "--path", dir, "--store", storePath, "--dry-run", "--stats"}, &stdout, &stderr); code != 0 {
		t.Fatalf("cmdIngest --dry-run exit %d: %s", code, stderr.String())
	}
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("a dry run touched the store (%v)", err)
	}
	if !strings.Contains(stdout.String(), "oversize:") || !strings.Contains(stdout.String(), ".cache") {
		t.Fatalf("text summary misses oversize or the pruned dir:\n%s", stdout.String())
	}
}
