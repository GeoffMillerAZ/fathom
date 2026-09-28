package docindex

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestDocDriftMovedTarget pins KR-7's core comparison (design §6): an open doc_links row
// whose target's CURRENT content hash no longer matches what was frozen at
// link time is reported drifted — attributed to the most recent commit
// that touched the target's own file, when one is known; a link whose
// target vanished entirely (no longer resolves at all) is drifted too,
// never silently treated as "still fine"; an unchanged link produces no
// drift at all; and §11's own rule holds structurally — DriftedDocLink
// carries no "wrong" verdict field, only what moved and (maybe) when.
func TestDocDriftMovedTarget(t *testing.T) {
	links := []store.DocLinkView{
		{UnitID: "u1", Path: "docs/design/x.md", HeadingPath: "X > A", TargetKind: "symbol", Target: "Foo", Line: 5, LinkTier: "exact", TargetHashAtLink: "h-foo-v1"},
		{UnitID: "u2", Path: "docs/design/x.md", HeadingPath: "X > B", TargetKind: "path", Target: "a/b.go", Line: 9, LinkTier: "exact", TargetHashAtLink: "h-path-v1"},
		{UnitID: "u3", Path: "docs/design/y.md", HeadingPath: "Y", TargetKind: "symbol", Target: "Gone", Line: 2, LinkTier: "exact", TargetHashAtLink: "h-gone-v1"},
		{UnitID: "u4", Path: "docs/design/y.md", HeadingPath: "Y", TargetKind: "key", Target: "route:GET /health", Line: 3, LinkTier: "exact", TargetHashAtLink: "h-key-v1"},
	}

	current := func(kind, target string) TargetResolution {
		switch {
		case kind == "symbol" && target == "Foo":
			return TargetResolution{Exists: true, Hash: "h-foo-v1", File: "internal/pkg/foo.go"} // unchanged
		case kind == "path" && target == "a/b.go":
			return TargetResolution{Exists: true, Hash: "h-path-v2", File: "a/b.go"} // changed
		case kind == "symbol" && target == "Gone":
			return TargetResolution{Exists: false} // vanished
		case kind == "key" && target == "route:GET /health":
			return TargetResolution{Exists: true, Hash: "h-key-v1", File: ""} // unchanged, no single file
		}
		t.Fatalf("current: unexpected target %s:%s", kind, target)
		return TargetResolution{}
	}
	commitForPath := func(path string) (string, string, bool) {
		if path == "a/b.go" {
			return "deadbeef", "2026-09-27T00:00:00Z", true
		}
		return "", "", false
	}

	drifted := docDrift(links, current, commitForPath)

	if len(drifted) != 2 {
		t.Fatalf("docDrift = %+v, want 2 drifted (a/b.go changed, Gone vanished)", drifted)
	}

	byTarget := make(map[string]DriftedDocLink, len(drifted))
	for _, d := range drifted {
		byTarget[d.Target] = d
	}

	ab, ok := byTarget["a/b.go"]
	if !ok {
		t.Fatalf("docDrift missing a/b.go: %+v", drifted)
	}
	if ab.LinkedHash != "h-path-v1" || ab.CurrentHash != "h-path-v2" {
		t.Errorf("a/b.go hashes = linked %q current %q, want h-path-v1 / h-path-v2", ab.LinkedHash, ab.CurrentHash)
	}
	if ab.ChangedCommit != "deadbeef" || ab.ChangedAt != "2026-09-27T00:00:00Z" {
		t.Errorf("a/b.go commit attribution = %q at %q, want deadbeef at 2026-09-27T00:00:00Z", ab.ChangedCommit, ab.ChangedAt)
	}

	gone, ok := byTarget["Gone"]
	if !ok {
		t.Fatalf("docDrift missing Gone (vanished target): %+v", drifted)
	}
	if gone.ChangedCommit != "" {
		t.Errorf("Gone: ChangedCommit = %q, want empty (no file known for a vanished symbol in this fixture)", gone.ChangedCommit)
	}

	// Unchanged symbol and unchanged key: no drift reported at all.
	if _, ok := byTarget["Foo"]; ok {
		t.Errorf("docDrift reported unchanged Foo as drifted: %+v", drifted)
	}
	if _, ok := byTarget["route:GET /health"]; ok {
		t.Errorf("docDrift reported unchanged key as drifted: %+v", drifted)
	}

	// Deterministic order: path, then line.
	if drifted[0].Path != "docs/design/x.md" || drifted[1].Path != "docs/design/y.md" {
		t.Errorf("docDrift order = %s, %s, want x.md before y.md", drifted[0].Path, drifted[1].Path)
	}
}

// TestDocDriftEmpty pins that no open links produces no drift, not a nil
// panic on an empty slice.
func TestDocDriftEmpty(t *testing.T) {
	if got := docDrift(nil, func(string, string) TargetResolution { return TargetResolution{} }, nil); got != nil {
		t.Fatalf("docDrift(nil) = %+v, want nil", got)
	}
}
