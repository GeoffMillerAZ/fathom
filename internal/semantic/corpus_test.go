package semantic

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestSelectCorpus(t *testing.T) {
	files := []core.SourceFile{
		{Path: "CLAUDE.md"},
		{Path: "ADR.md"},
		{Path: "docs/design/semantic-layers.md"},
		{Path: "docs/design/product-vision.md"},
		{Path: "docs/plans/README.md"},
		{Path: "internal/mcp/tools.go"},
		{Path: "pkg/client/client.go"},
		{Path: "README.md"},
		{Path: "docs/other.md"},
	}

	got := SelectCorpus(files, ObligationsInclude)

	// Design essays (docs/design/**) are deliberately excluded — binding
	// documents only (see ObligationsInclude).
	want := []string{
		"ADR.md",
		"CLAUDE.md",
		"docs/plans/README.md",
	}
	if len(got) != len(want) {
		t.Fatalf("SelectCorpus returned %d files, want %d: %v", len(got), len(want), paths(got))
	}
	for i, f := range got {
		if f.Path != want[i] {
			t.Errorf("got[%d].Path = %q, want %q", i, f.Path, want[i])
		}
	}
}

func TestSelectCorpus_Empty(t *testing.T) {
	got := SelectCorpus(nil, ObligationsInclude)
	if got != nil {
		t.Errorf("nil input → %v, want nil", got)
	}
}

func TestSelectCorpus_NoneMatch(t *testing.T) {
	files := []core.SourceFile{
		{Path: "internal/core/claims.go"},
		{Path: "README.md"},
	}
	got := SelectCorpus(files, ObligationsInclude)
	if len(got) != 0 {
		t.Errorf("want empty, got %v", paths(got))
	}
}

func TestSelectCorpus_Deterministic(t *testing.T) {
	// Unordered input; output must be sorted by path.
	files := []core.SourceFile{
		{Path: "docs/design/z.md"},
		{Path: "CLAUDE.md"},
		{Path: "docs/design/a.md"},
		{Path: "ADR.md"},
	}
	a := SelectCorpus(files, ObligationsInclude)
	b := SelectCorpus(files, ObligationsInclude)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: %v vs %v", paths(a), paths(b))
	}
	for i := range a {
		if a[i].Path != b[i].Path {
			t.Errorf("[%d] %q != %q", i, a[i].Path, b[i].Path)
		}
	}
	// Must be sorted.
	for i := 1; i < len(a); i++ {
		if a[i].Path < a[i-1].Path {
			t.Errorf("not sorted: %v", paths(a))
			break
		}
	}
}

func paths(files []core.SourceFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}
