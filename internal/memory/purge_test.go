package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/filter"
)

// TestPurgeMatchingDeletesQuotingMemories is AM-12's half of SP-5: a
// memory whose file/path anchor sits inside the purged pattern is deleted
// — its free-form claim may QUOTE the purged content — while memories
// anchored elsewhere survive byte-for-byte.
func TestPurgeMatchingDeletesQuotingMemories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memories.jsonl")
	fs := NewFileStore(path)
	ctx := context.Background()

	quoting := Memory{
		ID: "m-secret", Service: "svc",
		Claim:   "the token constant is AKIA-NOT-A-REAL-KEY-0123456789",
		Anchors: []Anchor{{Kind: AnchorFile, Ref: "secrets/token.go"}},
	}
	globAnchored := Memory{
		ID: "m-glob", Service: "svc",
		Claim:   "everything under secrets/ is generated",
		Anchors: []Anchor{{Kind: AnchorPath, Ref: "secrets/**"}},
	}
	unrelated := Memory{
		ID: "m-keep", Service: "svc",
		Claim:   "keep.go is harmless",
		Anchors: []Anchor{{Kind: AnchorFile, Ref: "internal/keep.go"}, {Kind: AnchorSymbol, Ref: "Keep"}},
	}
	for _, m := range []Memory{quoting, globAnchored, unrelated} {
		if err := fs.Put(ctx, m); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	removed, err := fs.PurgeMatching(filter.New([]string{"secrets/**"}))
	if err != nil {
		t.Fatalf("PurgeMatching: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2 (the file-anchored and the glob-anchored memory)", removed)
	}

	left, err := fs.List(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0].ID != "m-keep" {
		t.Fatalf("surviving memories = %+v, want only m-keep", left)
	}

	// Ground truth on the raw file: the quoted canary is gone.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "AKIA-NOT-A-REAL-KEY") {
		t.Fatal("the memories file still quotes the purged content")
	}

	// Idempotent: a second purge removes nothing and does not error.
	if n, err := fs.PurgeMatching(filter.New([]string{"secrets/**"})); err != nil || n != 0 {
		t.Fatalf("second purge: n=%d err=%v", n, err)
	}
}

// TestPurgeMatchingMissingFileIsZero: a repo with no memories file purges
// zero memories with no error — absence is a fine answer here, the CLI
// layer is what discloses it.
func TestPurgeMatchingMissingFileIsZero(t *testing.T) {
	fs := NewFileStore(filepath.Join(t.TempDir(), "absent.jsonl"))
	n, err := fs.PurgeMatching(filter.New([]string{"**"}))
	if err != nil || n != 0 {
		t.Fatalf("got (%d, %v), want (0, nil)", n, err)
	}
}
