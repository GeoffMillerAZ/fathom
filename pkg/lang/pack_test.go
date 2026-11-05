package lang_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// samplePack is the pack the tests and the CLI read: a doc dialect that claims
// *.widget.json by glob and a tags dialect over the thrift grammar.
const samplePack = "langtest/testdata/samplepack"

// sampleFS is the sample pack as a mutable in-memory filesystem, with its
// goldens left out.
func sampleFS(t *testing.T) fstest.MapFS {
	t.Helper()
	out := fstest.MapFS{}
	root := os.DirFS(samplePack)
	err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(p, "/testdata/") {
			return err
		}
		b, err := fs.ReadFile(root, p)
		out[p] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func packReason(t *testing.T, err error) (reason, dialect string) {
	t.Helper()
	var pe *lang.PackError
	if !errors.As(err, &pe) {
		t.Fatalf("error %v is not a *lang.PackError", err)
	}
	return pe.Reason, pe.Dialect
}

func TestWithDialectsReadsPack(t *testing.T) {
	r := newReader(t, lang.WithDialects(os.DirFS(samplePack)))

	w := extract(t, r, "ui/clock.widget.json", `{"name":"Clock","kind":"timer","uses":["Battery"]}`)
	if w.Dialect != "widget-spec" || w.Skipped != "" || w.Level != lang.LevelUnmeasured {
		t.Fatalf("widget: dialect %q skipped %q level %q", w.Dialect, w.Skipped, w.Level)
	}
	if len(w.Symbols) != 1 || w.Symbols[0].Kind != "WidgetDef" || w.Symbols[0].Canonical != "resource" || w.Symbols[0].Name != "Clock" {
		t.Fatalf("widget symbols: %+v", w.Symbols)
	}

	s := extract(t, r, "idl/shop.thr", "struct Order {\n  1: i32 id\n}\n")
	if s.Dialect != "thrift-lite" || len(s.Symbols) != 1 || s.Symbols[0].Canonical != "struct" || s.Symbols[0].Kind != "ThriftLiteStruct" {
		t.Fatalf("thrift-lite: %+v", s)
	}

	// The pack adds dialects and removes none: a built-in file reads as before,
	// and a plain .json file the pack does not claim stays with the generic reader.
	if res := extract(t, r, "idl/shop.thrift", "struct Order {\n  1: i32 id\n}\n"); res.Dialect != "thrift" {
		t.Fatalf("built-in thrift: %q", res.Dialect)
	}
	plain := extract(t, newReader(t), "data/other.json", `{"name":"Clock"}`)
	if got := extract(t, r, "data/other.json", `{"name":"Clock"}`); got.Dialect != plain.Dialect || got.Skipped != plain.Skipped || len(got.Symbols) != len(plain.Symbols) {
		t.Fatalf("a .json file the pack does not claim changed: %+v vs %+v", got, plain)
	}

	infos := map[string]lang.DialectInfo{}
	for _, d := range r.Dialects() {
		infos[d.ID] = d
	}
	for id, engine := range map[string]string{"widget-spec": "doc", "thrift-lite": "tags"} {
		d, ok := infos[id]
		if !ok || d.Engine != engine || d.Level != lang.LevelUnmeasured || d.Grade != "" || len(d.Matches) == 0 {
			t.Errorf("Dialects()[%s] = %+v", id, d)
		}
	}
	if len(r.Dialects()) != len(newReader(t).Dialects())+2 {
		t.Errorf("the pack added %d dialects, want 2", len(r.Dialects())-len(newReader(t).Dialects()))
	}
	prev := ""
	for _, d := range r.Dialects() {
		if d.ID < prev {
			t.Fatalf("Dialects() is not sorted by id at %s", d.ID)
		}
		prev = d.ID
	}
}

func TestPackDoesNotTakeBuiltInFiles(t *testing.T) {
	// A glob that matches a file a specific built-in dialect reads: the built-in
	// claim is tried first, so the pack never sees package.json.
	fsys := sampleFS(t)
	fsys["dialects/widget-spec/dialect.json"].Data = []byte(strings.Replace(string(fsys["dialects/widget-spec/dialect.json"].Data), `"**/*.widget.json"`, `"**/*.widget.json", "**/package.json"`, 1))
	r := newReader(t, lang.WithDialects(fsys))
	res := extract(t, r, "web/package.json", `{"name":"web","version":"1.0.0"}`)
	if res.Dialect != "package-json" {
		t.Fatalf("package.json read by %q, want the built-in package-json", res.Dialect)
	}
}

// builtInOwned are files a built-in dialect owns by extension. No pack or Go
// extractor reads one, whatever its glob or basename claims (design section 6).
var builtInOwned = []struct{ path, src string }{
	{"main.tf", tfSrc},
	{"a/a.go", "package a\n\nfunc F() {}\n"},
	{"a.py", "def f():\n    pass\n"},
	{"README.md", "# Title\n"},
	{"src/a.go", "package a\n\nfunc G() {}\n"},
	{"src/a.py", "def g():\n    pass\n"},
	{"x.widget.py", "def w():\n    pass\n"},
	{"x.widget.go", "package a\n\nfunc W() {}\n"},
	{"x.widget.tf", tfSrc},
	{"infra/main.tf", tfSrc},
}

func TestPackNeverTakesBuiltInOwnedFiles(t *testing.T) {
	plain := newReader(t)
	for _, claim := range []struct{ name, globs, basenames string }{
		{"everything", `["**/*"]`, `[]`},
		{"a directory", `["src/**"]`, `[]`},
		{"a bracketed extension", `["**/*.t[f]"]`, `[]`},
		{"a middle segment", `["**/*.widget.*"]`, `[]`},
		{"a basename", `["**/*.widget.json"]`, `["main.tf"]`},
	} {
		t.Run(claim.name, func(t *testing.T) {
			fsys := sampleFS(t)
			d := string(fsys["dialects/widget-spec/dialect.json"].Data)
			d = strings.Replace(d, `"globs": ["**/*.widget.json"]`, `"globs": `+claim.globs, 1)
			d = strings.Replace(d, `"basenames": []`, `"basenames": `+claim.basenames, 1)
			fsys["dialects/widget-spec/dialect.json"].Data = []byte(d)
			m := lang.Match{ID: "aaa-rows", Globs: []string{"**/*"}, Kinds: map[string]string{"RowDef": "function"}}
			readers := map[string]*lang.Reader{
				"pack":      newReader(t, lang.WithDialects(fsys)),
				"extractor": newReader(t, lang.WithExtractor(m, &rowExtractor{})),
			}
			for who, r := range readers {
				for _, f := range builtInOwned {
					want, got := extract(t, plain, f.path, f.src), extract(t, r, f.path, f.src)
					if got.Dialect != want.Dialect || got.Skipped != want.Skipped || len(got.Symbols) != len(want.Symbols) || got.Level != want.Level {
						t.Errorf("%s: %s read as %q level %q skipped %q with %d symbols; the built-ins read it as %q level %q with %d", who, f.path, got.Dialect, got.Level, got.Skipped, len(got.Symbols), want.Dialect, want.Level, len(want.Symbols))
					}
				}
			}
			// The claim still works on a file no built-in owns.
			if got := extract(t, readers["extractor"], "notes/a.rowdata", "def Main\n"); got.Dialect != "aaa-rows" && claim.name == "everything" {
				t.Errorf("the extractor's own glob read nothing: %+v", got)
			}
		})
	}
}

func TestPackDialectsAreTriedInIDOrder(t *testing.T) {
	fsys := sampleFS(t)
	for _, id := range []string{"zeta-spec", "alpha-spec"} {
		fsys["dialects/"+id+"/dialect.json"] = &fstest.MapFile{Data: []byte(strings.NewReplacer("widget-spec", id, "WidgetDef", "Dup_"+id, "**/*.widget.json", "**/*.dup.json").Replace(string(fsys["dialects/widget-spec/dialect.json"].Data)))}
		fsys["dialects/"+id+"/doc.json"] = &fstest.MapFile{Data: []byte(strings.NewReplacer("WidgetDef", "Dup_"+id, "**/*.widget.json", "**/*.dup.json").Replace(string(fsys["dialects/widget-spec/doc.json"].Data)))}
	}
	r := newReader(t, lang.WithDialects(fsys))
	for i := 0; i < 3; i++ {
		if res := extract(t, r, "x/a.dup.json", `{"name":"A"}`); res.Dialect != "alpha-spec" {
			t.Fatalf("file two pack dialects claim is read by %q, want alpha-spec (id order)", res.Dialect)
		}
	}
}

func TestPackRefusedWhole(t *testing.T) {
	setFile := func(fsys fstest.MapFS, name, from, to string) {
		f := fsys[name]
		if f == nil || !strings.Contains(string(f.Data), from) {
			panic("test setup: " + name + " lacks " + from)
		}
		f.Data = []byte(strings.Replace(string(f.Data), from, to, 1))
	}
	thrift := "dialects/thrift-lite/dialect.json"
	widget := "dialects/widget-spec/dialect.json"
	cases := []struct {
		name    string
		reason  string
		dialect string
		mutate  func(fsys fstest.MapFS) fs.FS
	}{
		{"no dialect", lang.PackEmpty, "", func(fstest.MapFS) fs.FS { return fstest.MapFS{"README": &fstest.MapFile{Data: []byte("x")}} }},
		{"bad json", lang.PackManifest, "thrift-lite", func(f fstest.MapFS) fs.FS { f[thrift].Data = []byte("{"); return f }},
		{"id is not its directory", lang.PackManifest, "thrift-lite2", func(f fstest.MapFS) fs.FS {
			setFile(f, thrift, `"id": "thrift-lite"`, `"id": "thrift-lite2"`)
			return f
		}},
		{"kind is not language or tooling", lang.PackManifest, "widget-spec", func(f fstest.MapFS) fs.FS { setFile(f, widget, `"kind": "tooling"`, `"kind": "toy"`); return f }},
		{"canonical kind outside the closed list", lang.PackManifest, "thrift-lite", func(f fstest.MapFS) fs.FS {
			setFile(f, thrift, `"canonical": "struct"`, `"canonical": "thing"`)
			return f
		}},
		{"required kind not declared", lang.PackManifest, "thrift-lite", func(f fstest.MapFS) fs.FS {
			setFile(f, thrift, `"required_kinds": ["struct", "service", "function"]`, `"required_kinds": ["struct", "nope"]`)
			return f
		}},
		{"missing query file", lang.PackManifest, "thrift-lite", func(f fstest.MapFS) fs.FS { delete(f, "dialects/thrift-lite/tags.scm"); return f }},
		{"missing doc spec", lang.PackManifest, "widget-spec", func(f fstest.MapFS) fs.FS { delete(f, "dialects/widget-spec/doc.json"); return f }},
		{"source kind a built-in emits", lang.PackConflict, "thrift-lite", func(f fstest.MapFS) fs.FS { setFile(f, thrift, `"ThriftLiteStruct"`, `"ThriftStruct"`); return f }},
		{"extension a built-in dialect claims", lang.PackConflict, "thrift-lite", func(f fstest.MapFS) fs.FS { setFile(f, thrift, `[".thr"]`, `[".thrift"]`); return f }},
		{"basename a built-in dialect claims", lang.PackConflict, "widget-spec", func(f fstest.MapFS) fs.FS {
			setFile(f, widget, `"basenames": []`, `"basenames": ["package.json"]`)
			return f
		}},
		{"two pack dialects claim an extension", lang.PackConflict, "thrift-twin", func(f fstest.MapFS) fs.FS {
			f["dialects/thrift-twin/dialect.json"] = &fstest.MapFile{Data: []byte(strings.NewReplacer(`"thrift-lite"`, `"thrift-twin"`, "ThriftLite", "ThriftTwin").Replace(string(f[thrift].Data)))}
			f["dialects/thrift-twin/tags.scm"] = f["dialects/thrift-lite/tags.scm"]
			return f
		}},
		{"id of the native Go dialect", lang.PackConflict, "go", func(f fstest.MapFS) fs.FS {
			f["dialects/go/dialect.json"] = &fstest.MapFile{Data: []byte(strings.NewReplacer(`"thrift-lite"`, `"go"`, "ThriftLite", "GoLite", `[".thr"]`, `[".gol"]`).Replace(string(f[thrift].Data)))}
			f["dialects/go/tags.scm"] = f["dialects/thrift-lite/tags.scm"]
			return f
		}},
		{"native engine", lang.PackEngine, "thrift-lite", func(f fstest.MapFS) fs.FS { setFile(f, thrift, `"engine": "tags"`, `"engine": "native"`); return f }},
		{"tags+doc engine", lang.PackEngine, "thrift-lite", func(f fstest.MapFS) fs.FS { setFile(f, thrift, `"engine": "tags"`, `"engine": "tags+doc"`); return f }},
		{"injections query", lang.PackInjections, "thrift-lite", func(f fstest.MapFS) fs.FS {
			setFile(f, thrift, `"tags": "tags.scm"`, `"tags": "tags.scm", "injections": "inj.scm"`)
			f["dialects/thrift-lite/inj.scm"] = &fstest.MapFile{Data: []byte("(struct_definition) @injection.content\n")}
			return f
		}},
		{"grammar fathom does not embed", lang.PackGrammar, "thrift-lite", func(f fstest.MapFS) fs.FS { setFile(f, thrift, `"blob": "thrift"`, `"blob": "klingon"`); return f }},
		{"entry point not in the grammar", lang.PackGrammar, "thrift-lite", func(f fstest.MapFS) fs.FS {
			setFile(f, thrift, `"tree_sitter_thrift"`, `"tree_sitter_nothing"`)
			return f
		}},
		{"query that does not compile", lang.PackQuery, "thrift-lite", func(f fstest.MapFS) fs.FS {
			f["dialects/thrift-lite/tags.scm"] = &fstest.MapFile{Data: []byte("(no_such_node_type) @name @definition.struct\n")}
			return f
		}},
		{"query syntax error", lang.PackQuery, "thrift-lite", func(f fstest.MapFS) fs.FS {
			f["dialects/thrift-lite/tags.scm"] = &fstest.MapFile{Data: []byte("(struct_definition type: (identifier) @name\n")}
			return f
		}},
		{"container", lang.PackContainer, "widget-spec", func(f fstest.MapFS) fs.FS {
			setFile(f, widget, `"sniff": null`, `"sniff": null, "container": {"format": "opc-zip", "extensions": [".wgt"]}`)
			setFile(f, widget, `"extensions": []`, `"extensions": [".wgt"]`)
			return f
		}},
		{"claims nothing", lang.PackClaims, "widget-spec", func(f fstest.MapFS) fs.FS {
			setFile(f, widget, `"globs": ["**/*.widget.json"]`, `"globs": []`)
			return f
		}},
		{"claims by shebang", lang.PackClaims, "widget-spec", func(f fstest.MapFS) fs.FS { setFile(f, widget, `"shebangs": []`, `"shebangs": ["node"]`); return f }},
		{"extension the router reads generically", lang.PackRoute, "widget-spec", func(f fstest.MapFS) fs.FS {
			setFile(f, widget, `"extensions": []`, `"extensions": [".json"]`)
			return f
		}},
		{"extension the router owns, no dialect declares", lang.PackRoute, "thrift-lite", func(f fstest.MapFS) fs.FS { setFile(f, thrift, `[".thr"]`, `[".go"]`); return f }},
		{"glob ending in an owned extension", lang.PackRoute, "widget-spec", func(f fstest.MapFS) fs.FS { setFile(f, widget, `"**/*.widget.json"`, `"**/*.go"`); return f }},
		{"glob that is not a pattern", lang.PackClaims, "widget-spec", func(f fstest.MapFS) fs.FS {
			setFile(f, widget, `"**/*.widget.json"`, `"[abc"`)
			return f
		}},
		{"tags query with no pattern", lang.PackQuery, "thrift-lite", func(f fstest.MapFS) fs.FS {
			f["dialects/thrift-lite/tags.scm"] = &fstest.MapFile{Data: []byte("; nothing here\n\n")}
			return f
		}},
		{"capture of a kind the manifest does not declare", lang.PackQuery, "thrift-lite", func(f fstest.MapFS) fs.FS {
			f["dialects/thrift-lite/tags.scm"] = &fstest.MapFile{Data: []byte("(struct_definition type: (identifier) @name) @definition.nope\n")}
			return f
		}},
		{"variant tags query that does not compile", lang.PackQuery, "thrift-lite", func(f fstest.MapFS) fs.FS {
			setFile(f, thrift, `"variants": []`, `"variants": [{"id": "alt", "entry_point": "tree_sitter_thrift", "extensions": [".thy"], "queries": {"tags": "alt.scm"}}]`)
			f["dialects/thrift-lite/alt.scm"] = &fstest.MapFile{Data: []byte("(no_such_node_type) @name @definition.struct\n")}
			return f
		}},
		{"variant tags query without an id", lang.PackManifest, "thrift-lite", func(f fstest.MapFS) fs.FS {
			setFile(f, thrift, `"variants": []`, `"variants": [{"id": "", "entry_point": "tree_sitter_thrift", "extensions": [".thy"], "queries": {"tags": "alt.scm"}}]`)
			f["dialects/thrift-lite/alt.scm"] = &fstest.MapFile{Data: []byte("(struct_definition type: (identifier) @name) @definition.struct\n")}
			return f
		}},
		{"manifest misspelled", lang.PackManifest, "zz-typo", func(f fstest.MapFS) fs.FS {
			f["dialects/zz-typo/dialect.jsn"] = f[thrift]
			return f
		}},
		{"file the manifest does not name", lang.PackManifest, "thrift-lite", func(f fstest.MapFS) fs.FS {
			f["dialects/thrift-lite/tag.scm"] = f["dialects/thrift-lite/tags.scm"]
			return f
		}},
		{"file beside the dialects", lang.PackManifest, "", func(f fstest.MapFS) fs.FS {
			f["dialects/README.md"] = &fstest.MapFile{Data: []byte("x")}
			return f
		}},
		{"sniff on an owned extension", lang.PackRoute, "widget-spec", func(f fstest.MapFS) fs.FS {
			setFile(f, widget, `"sniff": null`, `"sniff": {"extensions": [".go"], "pattern": "^widget", "priority": 5}`)
			return f
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := lang.New(lang.WithDialects(tc.mutate(sampleFS(t))))
			if err == nil {
				r.Close()
				t.Fatal("New accepted the pack")
			}
			if r != nil {
				t.Fatalf("New returned a Reader with its error %v", err)
			}
			reason, dialect := packReason(t, err)
			if reason != tc.reason || dialect != tc.dialect {
				t.Fatalf("refused as (%s, %s), want (%s, %s): %v", reason, dialect, tc.reason, tc.dialect, err)
			}
			if !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("error text %q lacks its reason", err)
			}
		})
	}
}

func TestBadDialectRefusesTheGoodOnesToo(t *testing.T) {
	fsys := sampleFS(t)
	fsys["dialects/zz-bad/dialect.json"] = &fstest.MapFile{Data: []byte(strings.NewReplacer(`"widget-spec"`, `"zz-bad"`, "WidgetDef", "BadDef", `"**/*.widget.json"`, `"**/*.bad.json"`, `"canonical": "resource"`, `"canonical": "nonsense"`).Replace(string(fsys["dialects/widget-spec/dialect.json"].Data)))}
	fsys["dialects/zz-bad/doc.json"] = fsys["dialects/widget-spec/doc.json"]
	if _, err := lang.New(lang.WithDialects(fsys)); err == nil {
		t.Fatal("New accepted a pack with one bad dialect")
	} else if reason, dialect := packReason(t, err); reason != lang.PackManifest || dialect != "zz-bad" {
		t.Fatalf("refused as (%s, %s): %v", reason, dialect, err)
	}
	// Without the bad dialect the same pack loads, so the refusal left nothing behind.
	delete(fsys, "dialects/zz-bad/dialect.json")
	delete(fsys, "dialects/zz-bad/doc.json")
	r := newReader(t, lang.WithDialects(fsys))
	if res := extract(t, r, "a.widget.json", `{"name":"A"}`); res.Dialect != "widget-spec" {
		t.Fatalf("good pack: %q", res.Dialect)
	}
}

func TestWithDialectsMisuse(t *testing.T) {
	if _, err := lang.New(lang.WithDialects(nil)); err == nil {
		t.Error("WithDialects(nil) accepted")
	}
	if _, err := lang.New(lang.WithDialects(os.DirFS(samplePack)), lang.WithDialects(os.DirFS(samplePack))); err == nil {
		t.Error("WithDialects twice accepted")
	}
}

func TestPackQueriesRunUnderTheEngineBudget(t *testing.T) {
	// A pack query runs on the engine's one executor, so the parse and query
	// deadline that bounds a built-in dialect bounds it. Shrunk to nothing, the
	// deadline fires on a pack file as it does on a built-in one.
	r := newReader(t, lang.WithDialects(os.DirFS(samplePack)))
	restore := shrinkParseTimeout()
	defer restore()
	src := strings.Repeat("struct S {\n  1: i32 a\n}\n", 2000)
	res := extract(t, r, "big.thr", src)
	faulted := false
	for _, n := range res.Notes {
		faulted = faulted || strings.Contains(n, "parse and query deadline (ParseTimeout)")
	}
	if !faulted || !res.Incomplete {
		t.Fatalf("a nanosecond deadline did not stop the pack query: %d symbols, notes %v", len(res.Symbols), res.Notes)
	}
}

func TestPackReaderConcurrentAndDeterministic(t *testing.T) {
	r := newReader(t, lang.WithDialects(os.DirFS(samplePack)))
	src := []byte("struct Order {\n  1: i32 id\n}\nservice Shop {\n  void ping()\n}\n")
	want := extract(t, r, "idl/shop.thr", string(src))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := r.Extract(context.Background(), lang.File{Path: "idl/shop.thr", Src: src})
			if err != nil || len(got.Symbols) != len(want.Symbols) || got.Dialect != want.Dialect {
				t.Errorf("concurrent read: %v %+v", err, got)
			}
		}()
	}
	wg.Wait()
}

func TestPackVariantsKeepTheirOwnQueries(t *testing.T) {
	// An extension-keyed variant with a tags query of its own is compiled, and
	// reads its extension with that query, not with the dialect's.
	fsys := sampleFS(t)
	d := string(fsys["dialects/thrift-lite/dialect.json"].Data)
	d = strings.Replace(d, `"variants": []`, `"variants": [{"id": "alt", "entry_point": "tree_sitter_thrift", "extensions": [".thy"], "queries": {"tags": "alt.scm"}}]`, 1)
	fsys["dialects/thrift-lite/dialect.json"].Data = []byte(d)
	fsys["dialects/thrift-lite/alt.scm"] = &fstest.MapFile{Data: []byte("(struct_definition type: (identifier) @name) @definition.service\n")}
	r := newReader(t, lang.WithDialects(fsys))
	src := "struct Order {\n  1: i32 id\n}\n"
	for i := 0; i < 2; i++ { // twice, and in both orders, so the first compiled query cannot win by luck
		alt := extract(t, r, "a.thy", src)
		def := extract(t, r, "a.thr", src)
		if len(alt.Symbols) == 0 || alt.Symbols[0].Kind != "ThriftLiteService" {
			t.Fatalf("the variant's extension read %+v, want the variant's query (ThriftLiteService)", alt.Symbols)
		}
		if len(def.Symbols) == 0 || def.Symbols[0].Kind != "ThriftLiteStruct" {
			t.Fatalf("the dialect's extension read %+v, want the dialect's query (ThriftLiteStruct)", def.Symbols)
		}
	}
}

func TestExtractorGlobThatIsNotAPatternIsRefused(t *testing.T) {
	m := lang.Match{ID: "aaa-rows", Globs: []string{"[abc"}, Kinds: map[string]string{"RowDef": "function"}}
	_, err := lang.New(lang.WithExtractor(m, &rowExtractor{}))
	if reason, dialect := packReason(t, err); reason != lang.PackClaims || dialect != "aaa-rows" {
		t.Fatalf("refused as (%s, %s): %v", reason, dialect, err)
	}
}
