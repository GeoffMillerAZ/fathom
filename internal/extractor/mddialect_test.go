package extractor

// mddialect_test.go — goldens and contracts for the markdown native dialect
// (LB-Q-markdown, design §13.2). Goldens are hand-written from the fixtures:
// headings as symbols (unchanged from the retired heading-only extractor) and
// the cross-file references the extractor records whether or not fusion can
// resolve them (a relative link to a .md or .mdx file, and its anchor).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type mdGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type mdGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type mdGolden struct {
	Path      string           `json:"path"`
	Roots     []string         `json:"roots"`
	Symbols   []mdGoldenSymbol `json:"symbols"`
	CrossFile []mdGoldenCross  `json:"cross_file"`
}

func mdFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "markdown", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if ext := filepath.Ext(e.Name()); !e.IsDir() && (ext == ".md" || ext == ".mdx") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	if len(out) < 3 {
		t.Fatalf("%s: %d fixtures, want at least 3", dir, len(out))
	}
	return out
}

func TestMarkdownDialectGoldens(t *testing.T) {
	x := NewMarkdownExtractor()
	for _, fx := range mdFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(fx + ".golden.json")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want mdGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s.golden.json: %v", fx, err)
			}
			res, err := x.ExtractDoc("svc", want.Path, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			got := mdGolden{Path: want.Path, Roots: res.Roots, Symbols: []mdGoldenSymbol{}, CrossFile: []mdGoldenCross{}}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != want.Path {
					t.Fatalf("plumbing changed: service %q file %q", n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, mdGoldenSymbol{Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature, Calls: n.Calls, Tier: string(n.Tier)})
			}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, mdGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line})
			}
			if want.CrossFile == nil {
				want.CrossFile = []mdGoldenCross{}
			}
			if !reflect.DeepEqual(got.Roots, want.Roots) {
				t.Errorf("roots: got %q, want %q", got.Roots, want.Roots)
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("symbols\n got: %+v\nwant: %+v", got.Symbols, want.Symbols)
			}
			if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
				t.Errorf("cross_file\n got: %+v\nwant: %+v", got.CrossFile, want.CrossFile)
			}
			nodes, err := x.Extract("svc", want.Path, src)
			if err != nil || !reflect.DeepEqual(nodes, res.Nodes) {
				t.Errorf("Extract and ExtractDoc disagree on the nodes (err %v)", err)
			}
			again, _ := x.ExtractDoc("svc", want.Path, src)
			if !reflect.DeepEqual(again, res) {
				t.Errorf("ExtractDoc is not deterministic")
			}
		})
	}
}

// The native registry answers for the dialect, and gives the cross-file pass
// its DocResult.
func TestMarkdownDialectIsRegistered(t *testing.T) {
	e, ok := NativeDialectExtractor("markdown")
	if !ok {
		t.Fatal("NativeDialectExtractor(markdown): not registered")
	}
	if _, isDoc := e.(DocResulter); !isDoc {
		t.Error("the markdown extractor must give the cross-file pass its DocResult")
	}
}

// MarkdownHeadingSlugs is the anchor index: GitHub-style slugs, repeats
// suffixed -1, -2, an explicit {#id} winning.
func TestMarkdownHeadingSlugs(t *testing.T) {
	src := []byte("# A & B: c_d\n\n## Install Steps\n\n## Install Steps\n\n## Page {#custom-id}\n\n## Tail {/* #mdx-id */}\n\n### `code` and *em*\n")
	nodes, err := NewMarkdownExtractor().Extract("svc", "x.md", src)
	if err != nil {
		t.Fatal(err)
	}
	got := MarkdownHeadingSlugs(nodes)
	want := map[string]string{
		"a--b-c_d": "A & B: c_d", "install-steps": "Install Steps", "install-steps-1": "Install Steps",
		"custom-id": "Page {#custom-id}", "mdx-id": "Tail {/* #mdx-id */}",
	}
	for slug, name := range want {
		if got[slug] != name {
			t.Errorf("slug %q: got %q, want %q (all: %v)", slug, got[slug], name, got)
		}
	}
}

// A link target that leaves the repository, or names no Markdown file, or has
// a scheme, records no reference.
func TestMarkdownLinkTarget(t *testing.T) {
	cases := []struct {
		rel, dest, target, frag string
		ok                      bool
	}{
		{"docs/a.md", "b.md", "docs/b.md", "", true},
		{"docs/a.md", "../b.md#X%20y", "b.md", "x y", true},
		{"docs/a.md", "/b.mdx", "b.mdx", "", true},
		{"docs/a.md", "#top", "", "", false},
		{"docs/a.md", "a.md#top", "docs/a.md", "top", true},
		{"docs/a.md", "../../b.md", "", "", false},
		{"docs/a.md", "http://x/b.md", "", "", false},
		{"docs/a.md", "//x/b.md", "", "", false},
		{"docs/a.md", "mailto:a@b.md", "", "", false},
		{"docs/a.md", "b.png", "", "", false},
		{"docs/a.md", "sub/", "", "", false},
		{"docs/a.md", "", "", "", false},
	}
	for _, c := range cases {
		target, frag, ok := markdownLinkTarget(c.rel, c.dest)
		if target != c.target || frag != c.frag || ok != c.ok {
			t.Errorf("markdownLinkTarget(%q, %q) = %q, %q, %v; want %q, %q, %v", c.rel, c.dest, target, frag, ok, c.target, c.frag, c.ok)
		}
	}
}
