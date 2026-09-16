package semantic

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// obligationsInclude loads the obligations layer's corpus include list from
// the manifest (schema/semantic/schema.cue via LoadLayer) — the corpus_test
// fixtures exercise SelectCorpus against the SAME globs production compiles
// use (M1-2: no hand-synced ObligationsInclude Go var exists anymore).
func obligationsInclude(t *testing.T) []string {
	t.Helper()
	def, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(%s): %v", LayerObligations, err)
	}
	return def.CorpusInclude
}

// obligationsCorpus is obligationsInclude's sibling: it also returns the
// layer's CorpusExclude list, for tests (like
// TestSelectCorpus_ExcludesTestdataFixtures below) that need the exclude
// side of the SAME production manifest, not a hand-written glob.
func obligationsCorpus(t *testing.T) (include, exclude []string) {
	t.Helper()
	def, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(%s): %v", LayerObligations, err)
	}
	return def.CorpusInclude, def.CorpusExclude
}

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

	got := SelectCorpus(files, obligationsInclude(t), nil)

	// Design essays (docs/design/**) are deliberately excluded — binding
	// documents only (see the obligations layer's corpus.include in
	// schema/semantic/schema.cue).
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
	got := SelectCorpus(nil, obligationsInclude(t), nil)
	if got != nil {
		t.Errorf("nil input → %v, want nil", got)
	}
}

func TestSelectCorpus_NoneMatch(t *testing.T) {
	files := []core.SourceFile{
		{Path: "internal/core/claims.go"},
		{Path: "README.md"},
	}
	got := SelectCorpus(files, obligationsInclude(t), nil)
	if len(got) != 0 {
		t.Errorf("want empty, got %v", paths(got))
	}
}

// TestSelectCorpus_Exclude is LC-6's motivating case (docs/plans/layer-
// catalog.md): spec-bindings' corpus is docs/plans/** EXCLUDING README.md —
// CorpusExclude was decoded from the manifest but never wired into any
// SelectCorpus call before this. An exclude glob removes a file even when an
// include glob also matches it.
func TestSelectCorpus_Exclude(t *testing.T) {
	files := []core.SourceFile{
		{Path: "docs/plans/README.md"},
		{Path: "docs/plans/layer-catalog.md"},
		{Path: "docs/plans/config-experience.md"},
	}
	got := SelectCorpus(files, []string{"docs/plans/**"}, []string{"docs/plans/README.md"})
	want := []string{"docs/plans/config-experience.md", "docs/plans/layer-catalog.md"}
	if len(got) != len(want) {
		t.Fatalf("SelectCorpus with exclude = %v, want %v", paths(got), want)
	}
	for i, f := range got {
		if f.Path != want[i] {
			t.Errorf("got[%d].Path = %q, want %q", i, f.Path, want[i])
		}
	}
}

func TestSelectCorpus_ExcludeEmptyIsNoOp(t *testing.T) {
	got := SelectCorpus([]core.SourceFile{{Path: "ADR.md"}}, []string{"ADR.md"}, nil)
	if len(got) != 1 || got[0].Path != "ADR.md" {
		t.Errorf("SelectCorpus with nil exclude = %v, want [ADR.md]", paths(got))
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
	include := obligationsInclude(t)
	a := SelectCorpus(files, include, nil)
	b := SelectCorpus(files, include, nil)
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

// TestSelectCorpus_ExcludesTestdataFixtures is O54's follow-up (.fathom/
// observations.md O54 addendum): the obligations compile corpus walked
// internal/custodian/testdata/codefacts/fixture/CLAUDE.md (a fixture
// authored to exercise internal/custodian's OWN tests, never a real
// governing document) as if it were a nested guide — it sat at position 10
// of 66 sections in a real compile run, and its testdata-sibling ADR.md at
// 19. The obligations layer's corpus.exclude must drop any path with a
// `testdata` path segment, at any depth, while still selecting a genuine
// nested guide (internal/CLAUDE.md) that the "**/CLAUDE.md" include glob
// is meant to reach.
func TestSelectCorpus_ExcludesTestdataFixtures(t *testing.T) {
	files := []core.SourceFile{
		{Path: "internal/custodian/testdata/codefacts/fixture/CLAUDE.md"},
		{Path: "internal/custodian/testdata/codefacts/fixture/ADR.md"},
		{Path: "internal/CLAUDE.md"},
		{Path: "CLAUDE.md"},
	}
	include, exclude := obligationsCorpus(t)
	got := SelectCorpus(files, include, exclude)

	for _, f := range got {
		if f.Path == "internal/custodian/testdata/codefacts/fixture/CLAUDE.md" ||
			f.Path == "internal/custodian/testdata/codefacts/fixture/ADR.md" {
			t.Errorf("SelectCorpus included a testdata fixture path %q — want it excluded (O54 addendum)", f.Path)
		}
	}

	wantPresent := []string{"internal/CLAUDE.md", "CLAUDE.md"}
	for _, want := range wantPresent {
		found := false
		for _, f := range got {
			if f.Path == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SelectCorpus dropped real nested guide %q, got %v", want, paths(got))
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
