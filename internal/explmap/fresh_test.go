package explmap

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// fakeSource is an in-memory Source: current files, a symbol table, and the
// files as they were at a commit.
type fakeSource struct {
	files   map[string]string
	symbols map[string]span
	filed   map[string]string // commit + "\x00" + path -> content
	checks  int
}

type span struct {
	path       string
	start, end int
}

func (f *fakeSource) Lines(path string, start, end int) (string, bool) {
	f.checks++
	c, ok := f.files[path]
	if !ok {
		return "", false
	}
	return core.LinesText([]byte(c), start, end)
}

func (f *fakeSource) Symbol(key string) (string, int, int, bool) {
	s, ok := f.symbols[key]
	return s.path, s.start, s.end, ok
}

func (f *fakeSource) Filed(path, commit string, start, end int) (string, bool) {
	c, ok := f.filed[commit+"\x00"+path]
	if !ok {
		return "", false
	}
	return core.LinesText([]byte(c), start, end)
}

const fileV1 = "package x\n\nfunc A() int {\n\treturn 1\n}\n\nfunc B() {}\n"

// filedEntry files an entry over path's lines at commit "c1" the way the CLI
// does: the hash of the span's text.
func filedEntry(path string, start, end int, symbol, content string) Entry {
	text, _ := core.LinesText([]byte(content), start, end)
	e := ent(path, start, end, HashText(text), "why")
	e.Symbol = symbol
	e.Commit = "c1"
	if symbol != "" {
		e.Anchor = symbol
	}
	return e
}

// TestResolveEntryFlagsStaleWithDiffSize pins XM-5: every entry is re-resolved
// at read time. An unchanged span is fresh; a symbol whose text only moved is
// re-resolved to its new span; anything else is stale with the size of its
// diff since filing, and a stale entry is never served as fresh.
func TestResolveEntryFlagsStaleWithDiffSize(t *testing.T) {
	src := &fakeSource{
		files:   map[string]string{"x.go": fileV1},
		symbols: map[string]span{"x.A": {"x.go", 3, 5}},
		filed:   map[string]string{"c1\x00x.go": fileV1},
	}
	byKey := filedEntry("x.go", 3, 5, "x.A", fileV1)
	bySpan := filedEntry("x.go", 3, 5, "", fileV1)

	if f := resolveEntryFresh(byKey, src); f.State != Fresh || f.Anchor != "x.go#L3-L5" {
		t.Fatalf("unchanged = %+v, want fresh at x.go#L3-L5", f)
	}

	// Two lines inserted above: the span drifted, the symbol's text did not.
	moved := "package x\n\n// A doc.\n// More.\nfunc A() int {\n\treturn 1\n}\n\nfunc B() {}\n"
	src.files["x.go"] = moved
	src.symbols["x.A"] = span{"x.go", 5, 7}
	f := resolveEntryFresh(byKey, src)
	if f.State != ReResolved || f.Anchor != "x.go#L5-L7" || f.DiffSize() != 0 {
		t.Fatalf("moved symbol = %+v, want re-resolved to x.go#L5-L7 with no diff", f)
	}
	// A span anchor has no key to re-resolve by: the drift is stale.
	if f := resolveEntryFresh(bySpan, src); f.State != Stale || !f.DiffKnown || f.DiffSize() == 0 {
		t.Errorf("drifted span = %+v, want stale with a known, non-zero diff", f)
	}

	// The body changed: one line replaced is one removed and one added.
	changed := "package x\n\nfunc A() int {\n\treturn 2\n}\n\nfunc B() {}\n"
	src.files["x.go"] = changed
	src.symbols["x.A"] = span{"x.go", 3, 5}
	f = resolveEntryFresh(byKey, src)
	if f.State != Stale || !f.DiffKnown || f.Added != 1 || f.Removed != 1 {
		t.Fatalf("changed body = %+v, want stale, +1 -1", f)
	}
	if f.Anchor != "x.go#L3-L5" || !strings.Contains(f.Note, "changed") {
		t.Errorf("changed body anchor/note = %q / %q", f.Anchor, f.Note)
	}

	// The symbol is gone: stale, every filed line removed.
	delete(src.symbols, "x.A")
	src.files["x.go"] = "package x\n"
	f = resolveEntryFresh(byKey, src)
	if f.State != Stale || f.Removed != 3 || f.Added != 0 || !strings.Contains(f.Note, "no longer resolves") {
		t.Errorf("gone symbol = %+v, want stale -3 naming the symbol", f)
	}

	// The file is gone.
	delete(src.files, "x.go")
	f = resolveEntryFresh(bySpan, src)
	if f.State != Stale || f.Removed != 3 || !strings.Contains(f.Note, "gone") {
		t.Errorf("gone file = %+v, want stale -3", f)
	}

	// The text at filing cannot be recovered: still stale, the size unknown,
	// and said so rather than guessed.
	src.files["x.go"] = changed
	noBase := bySpan
	noBase.Commit = "c-unknown"
	f = resolveEntryFresh(noBase, src)
	if f.State != Stale || f.DiffKnown || f.DiffSize() != -1 || !strings.Contains(f.Note, "size unknown") {
		t.Errorf("unrecoverable base = %+v, want stale with the size unknown", f)
	}
	// A base whose hash disagrees with the entry (filed from a dirty tree)
	// is not trusted as the diff base either.
	src.filed["c2\x00x.go"] = "package x\n\nfunc A() int {\n\treturn 9\n}\n"
	dirty := bySpan
	dirty.Commit = "c2"
	if f := resolveEntryFresh(dirty, src); f.DiffKnown {
		t.Errorf("a base that does not hash to the entry was used for the diff: %+v", f)
	}
}

func TestLineDiffCounts(t *testing.T) {
	cases := []struct {
		a, b         string
		added, remov int
	}{
		{"x\ny\nz", "x\ny\nz", 0, 0},
		{"x\ny\nz", "x\nz", 0, 1},
		{"x\nz", "x\ny\nz", 1, 0},
		{"a\nb", "c\nd", 2, 2},
		{"", "a", 1, 1},
	}
	for _, c := range cases {
		add, rem := lineDiff(c.a, c.b)
		if add != c.added || rem != c.remov {
			t.Errorf("lineDiff(%q, %q) = +%d -%d, want +%d -%d", c.a, c.b, add, rem, c.added, c.remov)
		}
	}
}
