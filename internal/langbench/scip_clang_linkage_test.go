package langbench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// linkageFixture is a scip-clang index of two test translation units that each define a
// file-local `static void close_cb(...)` (one symbol to scip-clang, O863), two headers
// that each define a `static inline` fast_min (one included by test/a.c through
// test/task.h, the other by test/b.c), an extern ext_fn defined in two documents (two
// platforms' copies, ruling 3's case), an extern b_helper only test/b.c defines, and a
// header test/run.h that refers to close_cb with no copy among the files it includes.
func linkageFixture(t *testing.T) []byte {
	t.Helper()
	const pre = "cxx . . $ "
	closeCB, fastMin, extFn, helper := pre+"close_cb(74ef0d3a3c5968a2).", pre+"fast_min(1111111111111111).", pre+"ext_fn(2222222222222222).", pre+"b_helper(3333333333333333)."
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{
			{RelativePath: "include/unix.h", Occurrences: []*scippb.Occurrence{scipCallsiteDef(fastMin, nil, 0, 18, 26)}},
			{RelativePath: "include/win.h", Occurrences: []*scippb.Occurrence{scipCallsiteDef(fastMin, nil, 0, 18, 26)}},
			{RelativePath: "test/a.c", Occurrences: []*scippb.Occurrence{
				scipCallsiteDef(closeCB, nil, 1, 12, 20),
				scipCallsiteRef(closeCB, 5, 10, 18),
				scipCallsiteRef(fastMin, 6, 2, 10),
				scipCallsiteRef(extFn, 7, 2, 8),
				scipCallsiteRef(helper, 8, 2, 10),
			}},
			{RelativePath: "test/b.c", Occurrences: []*scippb.Occurrence{
				scipCallsiteDef(closeCB, nil, 1, 12, 20),
				scipCallsiteDef(helper, nil, 3, 5, 13),
				scipCallsiteRef(closeCB, 5, 10, 18),
				scipCallsiteRef(fastMin, 6, 2, 10),
			}},
			{RelativePath: "src/x.c", Occurrences: []*scippb.Occurrence{scipCallsiteDef(extFn, nil, 0, 5, 11)}},
			{RelativePath: "src/y.c", Occurrences: []*scippb.Occurrence{scipCallsiteDef(extFn, nil, 0, 5, 11)}},
			{RelativePath: "test/run.h", Occurrences: []*scippb.Occurrence{scipCallsiteRef(closeCB, 0, 30, 38)}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestScoreSCIPClangInternalLinkage (owner ruling M3, O863, K19; C11 6.2.2p3): a file-scope
// function or object declared `static` has internal linkage, so in each translation unit
// the identifier names that unit's own definition, and another unit's same-named copy is
// a different entity. scip-clang 0.4.0 gives every copy one symbol, and K25 ruling 3 drew
// a reference to every one of them: every libuv test file reached every other through
// close_cb. A reference to such a symbol resolves to the definition in the referring
// document's own translation unit: the referring document itself (no cross-file edge),
// or a header its includes reach (the build's preprocessor oracle). The pairs ruling 3
// would draw to another unit's copy that nothing else draws are counted
// (scip_file_edges_internal_linkage) and disclosed beside every_definer's. An extern
// function defined in two documents keeps ruling 3, and so does a reference whose
// includes reach no defining copy (a header naming its includer's static).
func TestScoreSCIPClangInternalLinkage(t *testing.T) {
	raw := linkageFixture(t)
	opts := SCIPOptionsFor("c", nil)
	if opts.ClangLinkage == nil {
		t.Fatal("C's option set carries no linkage rule")
	}
	for _, d := range []struct{ path, name string }{
		{"test/a.c", "close_cb"}, {"test/b.c", "close_cb"}, {"include/unix.h", "fast_min"}, {"include/win.h", "fast_min"},
	} {
		opts.ClangLinkage.FileLocal(d.path, d.name)
	}
	opts.ClangLinkage.Include("test/a.c", "test/task.h")
	opts.ClangLinkage.Include("test/task.h", "include/unix.h")
	opts.ClangLinkage.Include("test/b.c", "include/win.h")
	opts.ClangLinkage.Include("test/a.c", "test/run.h")
	res, err := ScoreSCIPWith(raw, "s", nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	notes := opts.ClangLinkage.Finish("libuv/libuv", res.FileEdges, res.Counts)
	want := map[[2]string]bool{
		{"test/a.c", "include/unix.h"}: true, // fast_min: the copy test/task.h's include reaches
		{"test/b.c", "include/win.h"}:  true, // fast_min: test/b.c's own include
		{"test/a.c", "src/x.c"}:        true, // ext_fn: extern, every definer (ruling 3)
		{"test/a.c", "src/y.c"}:        true,
		{"test/a.c", "test/b.c"}:       true, // b_helper: extern, its one definer
		{"test/run.h", "test/a.c"}:     true, // close_cb: no copy run.h reaches, so ruling 3
		{"test/run.h", "test/b.c"}:     true,
	}
	if len(res.FileEdges) != len(want) {
		t.Errorf("file edges %v, want %v", res.FileEdges, want)
	}
	for e := range want {
		if !res.FileEdges[e] {
			t.Errorf("missing file edge %v", e)
		}
	}
	for _, e := range [][2]string{{"test/b.c", "test/a.c"}, {"test/a.c", "include/win.h"}, {"test/b.c", "include/unix.h"}} {
		if res.FileEdges[e] {
			t.Errorf("file edge %v drawn to another translation unit's copy", e)
		}
	}
	// Ruling 3 would add test/a.c -> test/b.c (drawn by b_helper anyway), test/b.c ->
	// test/a.c, test/a.c -> include/win.h and test/b.c -> include/unix.h: three left out.
	if got := res.Counts[CntScipFileEdgesInternalLinkage]; got != 3 {
		t.Errorf("%s = %d, want 3", CntScipFileEdgesInternalLinkage, got)
	}
	// A later definer still draws its pair under the rule: ext_fn's src/y.c, fast_min's
	// include/win.h for test/b.c and run.h's test/b.c.
	if got := res.Counts[CntScipFileEdgesEveryDefiner]; got != 3 {
		t.Errorf("%s = %d, want 3", CntScipFileEdgesEveryDefiner, got)
	}
	if len(notes) != 1 {
		t.Fatalf("notes %q, want one", notes)
	}
	for _, s := range []string{"libuv/libuv: 3 file-edge truth pairs", CntScipFileEdgesInternalLinkage, "owner ruling M3", "O863", "6.2.2", "clang-ast-defs", "2 pairs"} {
		if !strings.Contains(notes[0], s) {
			t.Errorf("note %q lacks %q", notes[0], s)
		}
	}

	// Without the compiler's linkage, and for every dialect but C, ruling 3 stands.
	for _, o := range []SCIPOptions{SCIPOptionsFor("c", nil), SCIPOptionsFor("cpp", nil), {}} {
		r, err := ScoreSCIPWith(raw, "s", nil, o)
		if err != nil {
			t.Fatal(err)
		}
		if !r.FileEdges[[2]string{"test/b.c", "test/a.c"}] || !r.FileEdges[[2]string{"test/a.c", "include/win.h"}] || r.Counts[CntScipFileEdgesInternalLinkage] != 0 {
			t.Errorf("no linkage known: every definer, nothing left out: %v %v", r.FileEdges, r.Counts)
		}
	}
	if SCIPOptionsFor("cpp", nil).ClangLinkage != nil {
		t.Error("the linkage rule is C's alone this wave")
	}
	var none *ClangLinkage
	if n := none.Finish("x", res.FileEdges, map[string]int{}); n != nil {
		t.Errorf("no rule: no note, got %q", n)
	}
}

// TestClangLinkageRead: the rule's facts come from the reading's own oracle files, the
// preprocessor oracle's `imports` (each #include and the subject file it entered) and its
// clang-ast-defs companion's `internal_linkage` (the compiler AST's file-scope definitions
// with internal linkage). A companion without them leaves ruling 3 standing, and the note
// says so.
func TestClangLinkageRead(t *testing.T) {
	dir := t.TempDir()
	pre := filepath.Join(dir, "libuv-preproc.json")
	companion := func(p string) string { return strings.TrimSuffix(p, ".json") + ".ranges.json" }
	write := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(pre, `{"tool":"clang-preprocessor","imports":[{"path":"test/a.c","line":1,"to_path":"test/task.h"},{"path":"test/task.h","line":3,"to_path":"include/unix.h"},{"path":"test/b.c","line":1,"to_path":"include/win.h"},{"path":"test/a.c","line":2,"to_path":"test/run.h"}]}`)
	write(companion(pre), `{"tool":"clang-ast","ranges":[],"internal_linkage":[{"path":"include/unix.h","name":"fast_min","line":1},{"path":"include/win.h","name":"fast_min","line":1},{"path":"test/a.c","name":"close_cb","line":2},{"path":"test/b.c","name":"close_cb","line":2}]}`)
	l := SCIPOptionsFor("c", nil).ClangLinkage
	if err := l.Read([]string{pre}, companion); err != nil {
		t.Fatal(err)
	}
	opts := SCIPOptions{ClangLinkage: l}
	res, err := ScoreSCIPWith(linkageFixture(t), "s", nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	l.Finish("libuv/libuv", res.FileEdges, res.Counts)
	if res.FileEdges[[2]string{"test/b.c", "test/a.c"}] || !res.FileEdges[[2]string{"test/a.c", "include/unix.h"}] || res.Counts[CntScipFileEdgesInternalLinkage] != 3 {
		t.Errorf("read from the oracle files: %v %v", res.FileEdges, res.Counts)
	}

	// A companion written before the oracle reported linkage (no internal_linkage).
	old := filepath.Join(dir, "curl-preproc.json")
	write(old, `{"imports":[]}`)
	write(companion(old), `{"tool":"clang-ast","ranges":[]}`)
	l2 := SCIPOptionsFor("c", nil).ClangLinkage
	if err := l2.Read([]string{old}, companion); err != nil {
		t.Fatal(err)
	}
	r2, err := ScoreSCIPWith(linkageFixture(t), "s", nil, SCIPOptions{ClangLinkage: l2})
	if err != nil {
		t.Fatal(err)
	}
	notes := l2.Finish("curl/curl", r2.FileEdges, r2.Counts)
	if !r2.FileEdges[[2]string{"test/b.c", "test/a.c"}] || len(notes) != 1 || !strings.Contains(notes[0], "no internal_linkage") {
		t.Errorf("an oracle with no linkage: ruling 3 stands and the note says so: %v %q", r2.FileEdges, notes)
	}
	if err := SCIPOptionsFor("c", nil).ClangLinkage.Read([]string{filepath.Join(dir, "missing.json")}, companion); err == nil {
		t.Error("a missing oracle file is an error")
	}
}

// TestClangFileScope: only a file-scope function or object has linkage of its own (C11
// 6.2.2): a scip-clang symbol of one term or function descriptor. A member (`uv_loop_s#data.`),
// a type, a namespace, a file, a macro or a local is none.
func TestClangFileScope(t *testing.T) {
	for sym, want := range map[string]string{
		"cxx . . $ close_cb(74ef0d3a3c5968a2).": "close_cb",
		"cxx . . $ close_cb_called.":            "close_cb_called",
		"cxx . . $ uv_loop_s#data.":             "",
		"cxx . . $ uv_loop_s#":                  "",
		"cxx . . $ fmt/f(1111111111111111).":    "",
		"cxx . . $ `<file>/src/a.c`/":           "",
		"cxx . . $ `src/inet.c:25:9`!":          "",
		"local 3":                               "",
	} {
		if got, ok := clangFileScope(sym); got != want || ok != (want != "") {
			t.Errorf("clangFileScope(%q) = %q, %v; want %q", sym, got, ok, want)
		}
	}
}
