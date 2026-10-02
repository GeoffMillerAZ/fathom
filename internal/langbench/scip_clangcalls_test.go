package langbench

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScoreSCIPClangCompilerSites (LB-F116): the build's own compiler is a
// tier-A oracle of C++ call sites (calibration/oracles/clang-calls). Against a
// scip-clang index it settles the classes the index cannot:
//   - a call whose callee depends on a template parameter (`buf.size()` on a
//     `B&`) has no callee before instantiation: fathom's pair is left out of
//     both counts and counted (dependent);
//   - a call the compiler resolves that the index holds no reference for
//     (`count(buf)`, a function template's specialization), and a constructor
//     call written `T(` (scored against the index's class reference there), are
//     calls of the truth, counted;
//   - a reference the index writes at a function's declaration
//     (`int close(int fd);`, written like a call) is no call: the compiler
//     declares the function there; left out of the truth and counted;
//   - a functional cast (`size_t(n)`) is no call either way: fathom's pair stays
//     its own error;
//   - a call written only on lines the build's preprocessor discards (`_fileno`
//     under `#ifdef _WIN32`) is in neither the index nor the compiler: left out
//     of both counts and counted, as a definition on such a line is; a
//     function's own name in its declarator is no call site of fathom's
//     (st.clear's only call, `data_.clear()`, is dependent);
//   - a definition a macro writes (gtest's TEST(suite, name) defines
//     suite_name_Test::TestBody over the macro's name) is named by what is
//     written there, as fathom names it, so the calls in its body are measured;
//   - a definition of fathom's that the index holds no occurrence in (the index
//     did not index that code) is scored against the compiler's sites alone.
func TestScoreSCIPClangCompilerSites(t *testing.T) {
	src := "int g(int);\n" + // 0
		"template <typename B> int run(B& buf) {\n" + // 1
		"  int n = buf.size();\n" + // 2
		"  n += g(1);\n" + // 3
		"  n += count(buf);\n" + // 4
		"  auto s = string_view(p, n);\n" + // 5
		"  return size_t(n);\n" + // 6
		"}\n" + // 7
		"namespace t {\n" + // 8
		"int close(int fd);\n" + // 9
		"}\n" + // 10
		"TEST(suite, name) {\n" + // 11
		"  g(2);\n" + // 12
		"}\n" + // 13
		"template <typename C> struct fm<int, C> {\n" + // 14
		"  int parse(C& c) { return g(3); }\n" + // 15
		"};\n" + // 16
		"void plat() {\n" + // 17
		"#ifdef _WIN32\n" + // 18
		"  _fileno(f);\n" + // 19
		"#endif\n" + // 20
		"}\n" + // 21
		"template <class T> struct st { void clear() { data_.clear(); } T data_; };\n" // 22
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: "m.cc", Occurrences: []*scippb.Occurrence{
			scipCallsiteRef("cxx . . $ g(1111111111111111).", 0, 4, 5), // a declaration, outside any definition
			scipCallsiteDef("cxx . . $ run(2222222222222222).", nil, 1, 26, 29),
			scipCallsiteRef("cxx . . $ g(1111111111111111).", 3, 7, 8),
			scipCallsiteRef("cxx . . $ string_view#", 5, 11, 22),
			scipCallsiteDef("cxx . . $ t/", nil, 8, 10, 11),
			scipCallsiteRef("cxx . . $ t/close(3333333333333333).", 9, 4, 9),
			scipCallsiteDef("cxx . . $ suite_name_Test#", nil, 11, 0, 4),
			scipCallsiteDef("cxx . . $ suite_name_Test#TestBody(4444444444444444).", nil, 11, 0, 4),
			scipCallsiteRef("cxx . . $ g(1111111111111111).", 12, 2, 3),
			scipCallsiteDef("cxx . . $ plat(5555555555555555).", nil, 17, 5, 9),
			scipCallsiteDef("cxx . . $ st#", nil, 22, 26, 28),
			scipCallsiteDef("cxx . . $ st#clear(6666666666666666).", nil, 22, 36, 41),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "run", Kind: "CppFunc", FilePath: "m.cc", Line: 2, EndLine: 8, Calls: []string{"size", "g", "count", "string_view", "size_t"}},
		{Service: "s", Name: "t", Kind: "CppNamespace", FilePath: "m.cc", Line: 9, EndLine: 11},
		{Service: "s", Name: "TEST", Kind: "CppFunc", FilePath: "m.cc", Line: 12, EndLine: 14, Calls: []string{"g"}},
		{Service: "s", Name: "fm", Kind: "CppStruct", FilePath: "m.cc", Line: 15, EndLine: 17},
		{Service: "s", Name: "fm.parse", Kind: "CppFunc", FilePath: "m.cc", Line: 16, EndLine: 16, Calls: []string{"g"}},
		{Service: "s", Name: "plat", Kind: "CppFunc", FilePath: "m.cc", Line: 18, EndLine: 22, Calls: []string{"_fileno"}},
		{Service: "s", Name: "st", Kind: "CppStruct", FilePath: "m.cc", Line: 23, EndLine: 23},
		{Service: "s", Name: "st.clear", Kind: "CppFunc", FilePath: "m.cc", Line: 23, EndLine: 23, Calls: []string{"clear"}},
	}
	opts := SCIPOptionsFor("cpp", func(string) []byte { return []byte(src) })
	opts.ClangSites = map[string][]ClangSite{"m.cc": {
		{Line: 3, Col: 15, Name: "size", Class: ClangDependent},
		{Line: 4, Col: 8, Name: "g", Class: ClangResolved},
		{Line: 5, Col: 8, Name: "count", Class: ClangResolved},
		{Line: 6, Col: 12, Name: "string_view", Class: ClangConstruct},
		{Line: 7, Col: 10, Name: "size_t", Class: ClangCast},
		{Line: 13, Col: 3, Name: "g", Class: ClangResolved},
		{Line: 16, Col: 28, Name: "g", Class: ClangResolved},
		{Line: 10, Col: 5, Name: "close", Class: ClangDeclaration},
		{Line: 23, Col: 37, Name: "clear", Class: ClangDeclaration},
		{Line: 23, Col: 53, Name: "clear", Class: ClangDependent},
	}}
	opts.ActiveLines = map[string][][2]int{"m.cc": {{1, 19}, {21, 23}}}
	res, err := ScoreSCIPWith(raw, "s", nodes, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[CallPair]bool{
		{"m.cc", "run", "g"}: true, {"m.cc", "run", "count"}: true, {"m.cc", "run", "string_view"}: true,
		{"m.cc", "TEST", "g"}: true, {"m.cc", "parse", "g"}: true,
	}
	if !reflect.DeepEqual(res.CallTruth, want) {
		t.Errorf("call truth = %v, want %v", res.CallTruth, want)
	}
	wantFound := map[CallPair]bool{
		{"m.cc", "run", "g"}: true, {"m.cc", "run", "count"}: true, {"m.cc", "run", "string_view"}: true, {"m.cc", "run", "size_t"}: true,
		{"m.cc", "TEST", "g"}: true, {"m.cc", "parse", "g"}: true,
	}
	if !reflect.DeepEqual(res.CallFound, wantFound) {
		t.Errorf("fathom pairs = %v, want %v (the dependent size() left out)", res.CallFound, wantFound)
	}
	for k, want := range map[string]int{
		CntCallTruth: 5, CntCallTruthFound: 5, CntCallFathom: 6, CntCallFathomInTrue: 5,
		CntClangCallsDependent: 2, CntCallsInactive: 1, CntClangCallsAdded: 2, CntClangConstructs: 1, CntClangNotCalls: 1,
		CntScipDefsMacroNamed: 1, CntClangBlindDefs: 1,
	} {
		if got := res.Counts[k]; got != want {
			t.Errorf("%s = %d, want %d", k, got, want)
		}
	}
}

// TestScoreSCIPClangCompilerSitesOff: with no compiler sites the scoring is the
// index's alone, as before (no rule above applies, no count is written).
func TestScoreSCIPClangCompilerSitesOff(t *testing.T) {
	src := "void run() {\n  g(1);\n  size_t(2);\n}\n"
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: "m.cc", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef("cxx . . $ run(2222222222222222).", nil, 0, 5, 8),
			scipCallsiteRef("cxx . . $ g(1111111111111111).", 1, 2, 3),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{{Service: "s", Name: "run", Kind: "CppFunc", FilePath: "m.cc", Line: 1, EndLine: 4, Calls: []string{"g", "size_t"}}}
	res, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("cpp", func(string) []byte { return []byte(src) }))
	if err != nil {
		t.Fatal(err)
	}
	if want := (map[CallPair]bool{{"m.cc", "run", "g"}: true}); !reflect.DeepEqual(res.CallTruth, want) {
		t.Errorf("call truth = %v, want %v", res.CallTruth, want)
	}
	if res.Counts[CntCallFathom] != 2 || res.Counts[CntCallFathomInTrue] != 1 {
		t.Errorf("fathom pairs %d (in truth %d), want 2 (1)", res.Counts[CntCallFathom], res.Counts[CntCallFathomInTrue])
	}
	for _, k := range []string{CntClangCallsDependent, CntClangCallsAdded, CntClangConstructs, CntClangNotCalls, CntScipDefsMacroNamed, CntClangBlindDefs} {
		if _, ok := res.Counts[k]; ok {
			t.Errorf("%s written with no compiler sites", k)
		}
	}
}
