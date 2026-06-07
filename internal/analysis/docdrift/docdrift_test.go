package docdrift_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/docdrift"
)

const goodMD = `# Overview

Use ` + "`" + `ValidateToken` + "`" + ` to authenticate requests.

See [auth guide](docs/auth.md) for details.
`

const driftMD = `# Overview

Use ` + "`" + `OldFunction` + "`" + ` which was removed.

See [old guide](docs/old.md) for details.

Also ` + "`" + `internal/auth/token.go` + "`" + ` was refactored.
`

const fencedMD = "# Title\n\n```go\nOldFunction()\n```\n\nDone.\n"

const linkMD = `# Links

[valid link](docs/valid.md)
[broken link](docs/gone.md)
[external](https://example.com)
`

func TestAnalyze_NoSymbols(t *testing.T) {
	drifts := docdrift.Analyze("doc.md", goodMD,
		map[string]bool{"ValidateToken": true},
		map[string]bool{"docs/auth.md": true},
	)
	if len(drifts) != 0 {
		t.Fatalf("expected no drift, got %v", drifts)
	}
}

func TestAnalyze_MissingSymbol(t *testing.T) {
	drifts := docdrift.Analyze("doc.md", driftMD,
		map[string]bool{},
		map[string]bool{},
	)
	// OldFunction, docs/old.md, internal/auth/token.go should be flagged
	refs := make(map[string]bool)
	for _, d := range drifts {
		refs[d.Ref] = true
	}
	if !refs["OldFunction"] {
		t.Error("expected OldFunction to be flagged")
	}
}

func TestAnalyze_MissingFile(t *testing.T) {
	drifts := docdrift.Analyze("doc.md", driftMD,
		map[string]bool{},
		map[string]bool{},
	)
	refs := make(map[string]bool)
	for _, d := range drifts {
		refs[d.Ref] = true
	}
	if !refs["docs/old.md"] {
		t.Error("expected docs/old.md to be flagged")
	}
}

func TestAnalyze_FencedCodeIgnored(t *testing.T) {
	// OldFunction is inside a fenced block — should not be flagged.
	drifts := docdrift.Analyze("doc.md", fencedMD,
		map[string]bool{},
		map[string]bool{},
	)
	for _, d := range drifts {
		if d.Ref == "OldFunction" {
			t.Errorf("OldFunction inside fenced block should not be flagged, got %v", d)
		}
	}
}

func TestAnalyze_LinkTargets(t *testing.T) {
	drifts := docdrift.Analyze("doc.md", linkMD,
		map[string]bool{},
		map[string]bool{"docs/valid.md": true},
	)
	refs := make(map[string]bool)
	for _, d := range drifts {
		refs[d.Ref] = true
	}
	if !refs["docs/gone.md"] {
		t.Error("expected docs/gone.md to be flagged")
	}
	if refs["docs/valid.md"] {
		t.Error("docs/valid.md should not be flagged (it exists)")
	}
	// External links should not be flagged.
	for _, d := range drifts {
		if d.Ref == "https://example.com" {
			t.Error("external URL should not be flagged")
		}
	}
}

func TestAnalyze_LineNumbers(t *testing.T) {
	drifts := docdrift.Analyze("doc.md", driftMD,
		map[string]bool{},
		map[string]bool{},
	)
	for _, d := range drifts {
		if d.Line <= 0 {
			t.Errorf("invalid line number %d for ref %q", d.Line, d.Ref)
		}
	}
}

func TestAnalyze_FileField(t *testing.T) {
	md := "Use `Ghost` here.\n"
	drifts := docdrift.Analyze("myfile.md", md, map[string]bool{}, map[string]bool{})
	if len(drifts) != 1 {
		t.Fatalf("expected 1 drift, got %d", len(drifts))
	}
	if drifts[0].File != "myfile.md" {
		t.Errorf("expected file=myfile.md, got %q", drifts[0].File)
	}
}

func TestAnalyze_Sorted(t *testing.T) {
	md := "Use `B` and `A` here.\n"
	drifts := docdrift.Analyze("doc.md", md, map[string]bool{}, map[string]bool{})
	for i := 1; i < len(drifts); i++ {
		if drifts[i].Line < drifts[i-1].Line {
			t.Errorf("not sorted by line")
		}
		if drifts[i].Line == drifts[i-1].Line && drifts[i].Ref < drifts[i-1].Ref {
			t.Errorf("not sorted by ref within same line")
		}
	}
}

func TestAnalyze_Determinism(t *testing.T) {
	var prev []docdrift.Drift
	for i := 0; i < 5; i++ {
		d := docdrift.Analyze("doc.md", driftMD,
			map[string]bool{},
			map[string]bool{},
		)
		if prev != nil {
			if len(d) != len(prev) {
				t.Fatalf("non-deterministic length at iteration %d", i)
			}
			for j := range d {
				if d[j] != prev[j] {
					t.Fatalf("non-deterministic output at iteration %d element %d", i, j)
				}
			}
		}
		prev = d
	}
}

func TestAnalyze_Empty(t *testing.T) {
	drifts := docdrift.Analyze("doc.md", "", map[string]bool{}, map[string]bool{})
	if len(drifts) != 0 {
		t.Fatalf("expected no drifts for empty content, got %v", drifts)
	}
}

func TestAnalyze_KnownSymbolNotFlagged(t *testing.T) {
	md := "Use `KnownFunc` to do things.\n"
	drifts := docdrift.Analyze("doc.md", md,
		map[string]bool{"KnownFunc": true},
		map[string]bool{},
	)
	if len(drifts) != 0 {
		t.Fatalf("known symbol should not be flagged, got %v", drifts)
	}
}
