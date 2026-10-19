package fusion

// mdxref_test.go — the markdown dialect's references through the doc-xref
// resolver (LB-Q-markdown): a link is a file edge from the section holding it
// to the target's first heading, a fragment an edge to the heading it slugs to.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func mdFixtureFiles(t *testing.T) []DocFile {
	t.Helper()
	x := extractor.NewMarkdownExtractor()
	var files []DocFile
	for _, fx := range []struct{ file, as string }{
		{"basic.md", "docs/basic.md"}, {"guide.md", "docs/guide.md"}, {"page.mdx", "docs/page.mdx"},
	} {
		src, err := os.ReadFile(filepath.Join("..", "extractor", "dialects", "markdown", "testdata", fx.file))
		if err != nil {
			t.Fatal(err)
		}
		r, err := x.ExtractDoc("svc", fx.as, src)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, DocFile{Path: fx.as, Result: r})
	}
	return files
}

func TestMarkdownLinksResolveToFilesAndHeadings(t *testing.T) {
	type edge struct{ from, fromFile, to, toFile string }
	got := map[edge]bool{}
	for _, x := range ResolveDocXrefs(mdFixtureFiles(t)) {
		got[edge{x.From, x.FromFile, x.To, x.ToFile}] = true
	}
	for _, w := range []edge{
		// basic.md line 3: [guide](guide.md) -> the root; guide.md#install-steps -> the first such heading
		{"Title", "docs/basic.md", "Guide", "docs/guide.md"},
		{"Title", "docs/basic.md", "Install Steps", "docs/guide.md"},
		// guide.md: [Basic](basic.md) from a section (#install-steps within the file draws no edge)
		{"Install Steps", "docs/guide.md", "Title", "docs/basic.md"},
		// page.mdx: ./basic.md#install-steps
		{"Page {#custom-id}", "docs/page.mdx", "Title", "docs/basic.md"},
		{"Page {#custom-id}", "docs/page.mdx", "Install steps", "docs/basic.md"},
	} {
		if !got[w] {
			t.Errorf("missing edge %+v; have %v", w, got)
		}
	}
	for e := range got {
		if e.fromFile == e.toFile && e.from == e.to {
			t.Errorf("self edge %+v", e)
		}
	}
}

// A heading with an explicit {#id} answers to it; a fragment no heading has resolves to nothing.
func TestMarkdownExplicitIDAndMissingAnchor(t *testing.T) {
	x := extractor.NewMarkdownExtractor()
	a, _ := x.ExtractDoc("svc", "a.md", []byte("# A\n\n[x](b.md#custom-id) [y](b.md#nope)\n"))
	b, _ := x.ExtractDoc("svc", "b.md", []byte("# B\n\n## Sub {#custom-id}\n"))
	var to []string
	for _, e := range ResolveDocXrefs([]DocFile{{Path: "a.md", Result: a}, {Path: "b.md", Result: b}}) {
		to = append(to, e.To)
	}
	want := map[string]bool{"B": true, "Sub {#custom-id}": true}
	if len(to) != 2 || !want[to[0]] || !want[to[1]] {
		t.Errorf("resolved to %q, want the root B (both links) and the explicit-id heading", to)
	}
}
