package client_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestObligationsIndexStaleSkipsOversize is RT-26 (design §4.3): the
// index-stale check read the whole edited file on every obligations call,
// whatever its size. It now stats first: a file over the 2 MiB hot-path
// cap is never read, is not reported stale (so no hook refreshes it), and
// the check names the skip — "not checked", never a silent "fresh" (I38).
// A file under the cap keeps the exact pre-RT-26 comparison.
func TestObligationsIndexStaleSkipsOversize(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	big := filepath.Join(dir, "big.md")
	small := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(big, []byte("# big\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(small, []byte("# R\n\n- Keep tests green.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	// big.md grows past the cap after the ingest: its stored fingerprint
	// no longer matches, and the pre-RT-26 check read all of it to say so.
	if err := os.WriteFile(big, bytes.Repeat([]byte("x"), int(client.HotPathReadCap)+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(small, []byte("# R\n\n- Keep tests green.\n- New rule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stop := client.ObserveReadsUnder(dir)
	stale, unchecked := c.ObligationsIndexCheck(ctx, "svc", "big.md")
	staleBool := c.ObligationsIndexStale(ctx, "svc", "big.md")
	smallStale, smallUnchecked := c.ObligationsIndexCheck(ctx, "svc", "CLAUDE.md")
	reads := stop()

	if stale || staleBool {
		t.Errorf("oversize file reported stale (check=%v, bool=%v); want not stale — it was never read, so nothing may be claimed or refreshed", stale, staleBool)
	}
	if !strings.Contains(unchecked, "read cap") || !strings.Contains(unchecked, "not refreshed") {
		t.Errorf("unchecked = %q, want it to name the read cap and that the file was not refreshed", unchecked)
	}
	for _, r := range reads {
		if r == big {
			t.Errorf("the oversize file was opened for reading: reads=%v", reads)
		}
	}
	if !smallStale || smallUnchecked != "" {
		t.Errorf("small edited file: stale=%v unchecked=%q, want stale with no skip note", smallStale, smallUnchecked)
	}
	if len(reads) != 1 || reads[0] != small {
		t.Errorf("reads = %v, want exactly the small file, read through the capped reader", reads)
	}
}
