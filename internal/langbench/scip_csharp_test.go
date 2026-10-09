package langbench

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestSCIPOptionsForCSharpOracleBlind (O464, LB-F175): C# is scored with the
// oracle-blind exclusion on when the source can be read, off when it cannot.
func TestSCIPOptionsForCSharpOracleBlind(t *testing.T) {
	if opts := SCIPOptionsFor("csharp", func(string) []byte { return nil }); !opts.OracleBlindSites || opts.Source == nil {
		t.Errorf("csharp with a source: OracleBlindSites %v, Source set %v, want both", opts.OracleBlindSites, opts.Source != nil)
	}
	if opts := SCIPOptionsFor("csharp", nil); opts.OracleBlindSites {
		t.Error("csharp without a source: OracleBlindSites must stay off (no written site can be read)")
	}
}

// TestScoreSCIPCSharpOracleBlindSites (O464, O271): scip-dotnet writes no
// occurrence for the name of a generic method called with explicit type
// arguments (`Workloads.FuncAsync<int>()`, `Make<List<int>>(2)`), so a fathom
// pair whose every written site is `name<...>(` with nothing of the index
// there is one the oracle cannot judge: left out of both L2 counts and
// counted. A site the index did write keeps its pair scored, and a pair with
// no written site found stays. scip-dotnet carries no enclosing_range: the
// mapped fathom definition's lines stand in (D5).
func TestScoreSCIPCSharpOracleBlindSites(t *testing.T) {
	const pre = "scip-dotnet nuget . . App/"
	src := "class C {\n" + // 0
		"  void Run(Registry reg) {\n" + // 1
		"    reg.Add(1);\n" + // 2
		"    Workloads.FuncAsync<int>();\n" + // 3
		"    Make<List<int>>(2);\n" + // 4
		"    reg.Touch<int>();\n" + // 5
		"  }\n" + // 6
		"}\n" // 7
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "0.2.14"}},
		Documents: []*scippb.Document{{RelativePath: "C.cs", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef(pre+"C#", nil, 0, 6, 7),
			scipCallsiteDef(pre+"C#Run().", nil, 1, 7, 10),
			scipCallsiteRef(pre+"Registry#", 1, 11, 19),
			scipCallsiteRef(pre+"Registry#Add().", 2, 8, 11), // reg.Add(1): judged, a truth pair
			scipCallsiteRef(pre+"Workloads#", 3, 4, 13),      // the receiver's type, never the method
			scipCallsiteRef("scip-dotnet nuget System . System/Int32#", 3, 24, 27),
			scipCallsiteRef("scip-dotnet nuget System . System/Collections/Generic/List`1#", 4, 9, 13),
			scipCallsiteRef(pre+"Registry#Touch().", 5, 8, 13), // the index did write this generic call
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "C", Kind: "CSharpClass", FilePath: "C.cs", Line: 1, EndLine: 8},
		{Service: "s", Name: "C.Run", Kind: "CSharpMethod", FilePath: "C.cs", Line: 2, EndLine: 7,
			Calls: []string{"reg.Add", "Workloads.FuncAsync", "Make", "reg.Touch", "Unwritten"}},
	}
	opts := SCIPOptionsFor("csharp", func(string) []byte { return []byte(src) })
	res, err := ScoreSCIPWith(raw, "s", nodes, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, blind := range []string{"FuncAsync", "Make"} {
		if res.CallFound[CallPair{"C.cs", "Run", blind}] {
			t.Errorf("(Run, %s): written only as `%s<...>(` with no occurrence there, yet still scored", blind, blind)
		}
	}
	if p := (CallPair{"C.cs", "Run", "Add"}); !res.CallTruth[p] || !res.CallFound[p] {
		t.Errorf("(Run, Add): want a found truth pair (truth %v, found %v)", res.CallTruth[p], res.CallFound[p])
	}
	if p := (CallPair{"C.cs", "Run", "Touch"}); !res.CallTruth[p] || !res.CallFound[p] {
		t.Errorf("(Run, Touch): the index wrote `Touch<int>(`, want a found truth pair (truth %v, found %v)", res.CallTruth[p], res.CallFound[p])
	}
	if !res.CallFound[CallPair{"C.cs", "Run", "Unwritten"}] {
		t.Error("(Run, Unwritten): no written site was found, so it must stay scored")
	}
	if res.Counts[CntCallsOracleBlind] != 2 {
		t.Errorf("oracle-blind = %d, want 2 (FuncAsync, Make)", res.Counts[CntCallsOracleBlind])
	}
	if res.Counts[CntCallFathom] != len(res.CallFound) {
		t.Errorf("call_fathom %d != scored pairs %d", res.Counts[CntCallFathom], len(res.CallFound))
	}
}

// TestCSharpGenericWrittenSites (O464): the written-site match reads a C#
// generic method's explicit type arguments between its name and `(`, nested
// ones included, and a comparison or a lambda arrow is no call.
func TestCSharpGenericWrittenSites(t *testing.T) {
	cases := []struct {
		line   string
		callee string
		want   int
	}{
		{"x.Handle<InvalidOperationException>();", "Handle", 1},
		{"JsonConvert.DeserializeObject<IList<decimal>>(s);", "DeserializeObject", 1},
		{"Pair<System.Exception, string>(5);", "Pair", 1},
		{"Make<(int, string)[]>(x);", "Make", 1},
		{"if (Count < Max) return;", "Count", 0},
		{"Func<int, int> f = n => Sq(n);", "Func", 0},
	}
	for _, tc := range cases {
		lines := [][]byte{[]byte(tc.line)}
		if got := len(tsWrittenCallSites(lines, 1, 1, tc.callee)); got != tc.want {
			t.Errorf("%q: %d written sites of %s, want %d", tc.line, got, tc.callee, tc.want)
		}
	}
}

// TestCSharpSCIPNames (LB-F286): scip-dotnet's descriptor leaf in the name the
// dialect gives the same definition: a constructor (`.ctor`) or static
// constructor (`.cctor`) is its class, as Java's `<init>` is; an explicit
// interface implementation (`N.IFoo<T>.M`) is its member's own name; an
// indexer (`this[]`) is `this`, as the Roslyn dump names it; a type's arity
// suffix (`List`1`) is not part of its name. Any other producer is untouched.
func TestCSharpSCIPNames(t *testing.T) {
	const pre = "scip-dotnet nuget . . App/"
	cases := []struct {
		sym      string
		name     string
		callable bool
	}{
		{pre + "Widget#`.ctor`().", "Widget", true},
		{pre + "Widget#`.ctor`(+1).", "Widget", true},
		{pre + "Outer#Inner#`.cctor`().", "Inner", true},
		{pre + "AsyncPolicy#`Polly.IAsyncPolicy.WithPolicyKey`().", "WithPolicyKey", true},
		{pre + "Context#`System.Collections.Generic.ICollection<System.Collections.Generic.KeyValuePair<System.String,System.Object>>.IsReadOnly`.", "IsReadOnly", false},
		{pre + "JArray#`this[]`.", "this", false},
		{"scip-dotnet nuget System . System/Collections/Generic/List`1#", "List", false},
		{pre + "Widget#Render().", "Render", true},
		{"semanticdb maven . . a/Widget#`.ctor`().", ".ctor", true},
	}
	for _, tc := range cases {
		if name, callable := scipBareName(tc.sym); name != tc.name || callable != tc.callable {
			t.Errorf("scipBareName(%q) = %q, %v; want %q, %v", tc.sym, name, callable, tc.name, tc.callable)
		}
	}
}

// TestScoreSCIPCSharpCallSites (LB-F286, O590): scip-dotnet 0.2.14 writes a
// constructor (`.ctor`) reference only where an attribute is applied
// (`[Obsolete("x")]`), which calls nothing where it is written: no call site,
// left out of the truth and counted. `new X(..)` it writes as a reference to
// the type X: a type reference written `new X(`, `new X<T>(` or `new X {` is a
// call to X, counted. A `.ctor` definition maps to the dialect's constructor.
func TestScoreSCIPCSharpCallSites(t *testing.T) {
	const pre = "scip-dotnet nuget . . App/"
	src := "[Obsolete(\"x\")]\n" + // 0
		"class C {\n" + // 1
		"  public C() { }\n" + // 2
		"  void Run(Registry reg) {\n" + // 3
		"    var w = new Widget(1);\n" + // 4
		"    var g = new Box<int>();\n" + // 5
		"    var h = new App.Holder { N = 1 };\n" + // 6
		"    Registry.Create(w);\n" + // 7
		"  }\n" + // 8
		"}\n" // 9
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "0.2.14"}},
		Documents: []*scippb.Document{{RelativePath: "C.cs", Occurrences: []*scippb.Occurrence{
			scipCallsiteRef("scip-dotnet nuget System . System/ObsoleteAttribute#`.ctor`(+1).", 0, 1, 9),
			scipCallsiteDef(pre+"C#", nil, 1, 6, 7),
			scipCallsiteDef(pre+"C#`.ctor`().", nil, 2, 9, 10),
			scipCallsiteDef(pre+"C#Run().", nil, 3, 7, 10),
			scipCallsiteRef(pre+"Registry#", 3, 11, 19),
			scipCallsiteRef(pre+"Widget#", 4, 16, 22),
			scipCallsiteRef(pre+"Box#", 5, 16, 19),
			scipCallsiteRef("scip-dotnet nuget . . App/", 6, 16, 19),
			scipCallsiteRef(pre+"Holder#", 6, 20, 26),
			scipCallsiteRef(pre+"Registry#", 7, 4, 12),
			scipCallsiteRef(pre+"Registry#Create().", 7, 13, 19),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "C", Kind: "CSharpClass", FilePath: "C.cs", Line: 2, EndLine: 10},
		{Service: "s", Name: "C.C", Kind: "CSharpMethod", FilePath: "C.cs", Line: 3, EndLine: 3},
		{Service: "s", Name: "C.Run", Kind: "CSharpMethod", FilePath: "C.cs", Line: 4, EndLine: 9,
			Calls: []string{"Widget", "Box", "App.Holder", "Registry.Create"}},
	}
	res, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("csharp", func(string) []byte { return []byte(src) }))
	if err != nil {
		t.Fatal(err)
	}
	for _, callee := range []string{"Widget", "Box", "Holder", "Create"} {
		if p := (CallPair{"C.cs", "Run", callee}); !res.CallTruth[p] || !res.CallFound[p] {
			t.Errorf("(Run, %s): want a found truth pair (truth %v, found %v)", callee, res.CallTruth[p], res.CallFound[p])
		}
	}
	for p := range res.CallTruth {
		if p.Callee == "ObsoleteAttribute" || p.Callee == "Obsolete" || p.Callee == ".ctor" {
			t.Errorf("%v: an attribute's constructor reference is no call site", p)
		}
		if p.Callee == "Registry" {
			t.Errorf("%v: a type written as a receiver is no construction", p)
		}
	}
	if got := res.Counts[CntScipCSharpCtorNotCalls]; got != 1 {
		t.Errorf("%s = %d, want 1 (the attribute)", CntScipCSharpCtorNotCalls, got)
	}
	if got := res.Counts[CntScipCSharpConstructions]; got != 3 {
		t.Errorf("%s = %d, want 3 (Widget, Box, Holder)", CntScipCSharpConstructions, got)
	}
	if res.Counts[CntScipDefsMapped] != res.Counts[CntScipDefs] {
		t.Errorf("def_mapping %d of %d: the `.ctor` definition maps to the dialect's constructor C.C", res.Counts[CntScipDefsMapped], res.Counts[CntScipDefs])
	}
}

// TestScoreSCIPCSharpCompilerSites (LB-F287, O591): the compiler's semantic
// model (calibration/oracles/roslyn calls mode, tier A) settles what the
// index's references cannot: a call it binds is in the truth whether or not
// the index wrote an occurrence there (a generic method called with explicit
// type arguments, O271; a delegate-typed parameter invoked); a fathom pair
// whose every written site the compiler binds to nothing, or which every
// compiled configuration discards (`#if NEVER`), is left out of both counts
// and counted, not as oracle-blind.
func TestScoreSCIPCSharpCompilerSites(t *testing.T) {
	const pre = "scip-dotnet nuget . . App/"
	src := "class C {\n" + // 1 (1-based)
		"  void Run(Action<int> cb) {\n" + // 2
		"    Policy.Handle<int>();\n" + // 3
		"    cb(2);\n" + // 4
		"#if NEVER\n" + // 5
		"    Gone();\n" + // 6
		"#endif\n" + // 7
		"    Dyn();\n" + // 8
		"    Lost<int>();\n" + // 9
		"  }\n" + // 10
		"}\n" // 11
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "0.2.14"}},
		Documents: []*scippb.Document{{RelativePath: "C.cs", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef(pre+"C#", nil, 0, 6, 7),
			scipCallsiteDef(pre+"C#Run().", nil, 1, 7, 10),
			scipCallsiteRef(pre+"Policy#", 2, 4, 10),
			scipCallsiteRef(pre+"C#Run().(cb)", 3, 4, 6),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "C", Kind: "CSharpClass", FilePath: "C.cs", Line: 1, EndLine: 11},
		{Service: "s", Name: "C.Run", Kind: "CSharpMethod", FilePath: "C.cs", Line: 2, EndLine: 10,
			Calls: []string{"Policy.Handle", "cb", "Gone", "Dyn", "Lost"}},
	}
	opts := SCIPOptionsFor("csharp", func(string) []byte { return []byte(src) })
	opts.CSharp = &CSharpCompiler{
		Sites: map[string][]ClangSite{"C.cs": {
			{Line: 3, Col: 12, Name: "Handle", Class: CSharpResolved},
			{Line: 4, Col: 5, Name: "cb", Class: CSharpObject},
			{Line: 8, Col: 5, Name: "Dyn", Class: CSharpUnresolved},
		}},
		Active: map[string][][2]int{"C.cs": {{1, 5}, {7, 11}}},
	}
	res, err := ScoreSCIPWith(raw, "s", nodes, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, callee := range []string{"Handle", "cb"} {
		if p := (CallPair{"C.cs", "Run", callee}); !res.CallTruth[p] || !res.CallFound[p] {
			t.Errorf("(Run, %s): the compiler binds it, want a found truth pair (truth %v, found %v)", callee, res.CallTruth[p], res.CallFound[p])
		}
	}
	for _, callee := range []string{"Gone", "Dyn", "Lost"} {
		if res.CallFound[CallPair{"C.cs", "Run", callee}] {
			t.Errorf("(Run, %s): unjudged, yet scored", callee)
		}
	}
	want := map[string]int{CntCSharpCallsAdded: 2, CntCallsInactive: 1, CntCSharpCallsUnresolved: 1, CntCallsOracleBlind: 1}
	for k, v := range want {
		if res.Counts[k] != v {
			t.Errorf("%s = %d, want %d", k, res.Counts[k], v)
		}
	}
}

// TestScoreSCIPCSharpStandInDefs (LB-F287, O591): scip-dotnet writes no
// definition for a conversion operator (only its parameter), so the calls in
// its body are attributed to the class around it; a fathom definition that
// makes calls, with no definition of the index at its name's line, stands in
// for itself as the caller, as clang's index-blind definitions do (LB-F116).
// A definition the index does define is never doubled.
func TestScoreSCIPCSharpStandInDefs(t *testing.T) {
	const pre = "scip-dotnet nuget . . App/"
	src := "class Money {\n" + // 0
		"  public static implicit operator bool(Money m) => Check(m);\n" + // 1
		"  static bool Check(Money m) => Other();\n" + // 2
		"}\n" // 3
	col := int32(strings.Index("  public static implicit operator bool(Money m) => Check(m);", "Check"))
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "0.2.14"}},
		Documents: []*scippb.Document{{RelativePath: "M.cs", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef(pre+"Money#", nil, 0, 6, 11),
			scipCallsiteDef(pre+"Money#op_Implicit().(m)", nil, 1, 46, 47),
			scipCallsiteRef(pre+"Money#Check().", 1, col, col+5),
			scipCallsiteDef(pre+"Money#Check().", nil, 2, 14, 19),
			scipCallsiteRef(pre+"Money#Other().", 2, 32, 37),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "Money", Kind: "CSharpClass", FilePath: "M.cs", Line: 1, EndLine: 4},
		{Service: "s", Name: "Money.bool", Kind: "CSharpMethod", FilePath: "M.cs", Line: 2, EndLine: 2, Calls: []string{"Check"}},
		{Service: "s", Name: "Money.Check", Kind: "CSharpMethod", FilePath: "M.cs", Line: 3, EndLine: 3, Calls: []string{"Other"}},
	}
	res, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("csharp", func(string) []byte { return []byte(src) }))
	if err != nil {
		t.Fatal(err)
	}
	if p := (CallPair{"M.cs", "bool", "Check"}); !res.CallTruth[p] || !res.CallFound[p] {
		t.Errorf("(bool, Check): the operator stands in for itself, want a found truth pair (truth %v, found %v)", res.CallTruth[p], res.CallFound[p])
	}
	if res.CallTruth[CallPair{"M.cs", "Money", "Check"}] {
		t.Error("(Money, Check): the call is the operator's, not its class's")
	}
	if p := (CallPair{"M.cs", "Check", "Other"}); !res.CallTruth[p] || !res.CallFound[p] {
		t.Errorf("(Check, Other): want a found truth pair (truth %v, found %v)", res.CallTruth[p], res.CallFound[p])
	}
	if got := res.Counts[CntCSharpStandInDefs]; got != 1 {
		t.Errorf("%s = %d, want 1 (the operator; Check is the index's)", CntCSharpStandInDefs, got)
	}
}

// TestScoreSCIPCSharpFileEdges (LB-F287, O592): a reference to a symbol two
// documents define (a C# partial type; scip-dotnet also gives a generic type
// and its non-generic namesake one symbol) names no one file, so it draws no
// file edge, counted; the compiler's own cross-file edges (every target
// framework, generic type arguments and generic method calls included) join
// the truth from the documents the index holds, counted.
func TestScoreSCIPCSharpFileEdges(t *testing.T) {
	const pre = "scip-dotnet nuget . . App/"
	def := func(sym string, line int32) *scippb.Occurrence {
		return scipCallsiteDef(sym, nil, line, 6, 9)
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "0.2.14"}},
		Documents: []*scippb.Document{
			{RelativePath: "A.cs", Occurrences: []*scippb.Occurrence{def(pre+"Policy#", 0)}},
			{RelativePath: "B.cs", Occurrences: []*scippb.Occurrence{def(pre+"Policy#", 0), def(pre+"Policy#Bulkhead().", 1)}},
			{RelativePath: "D.cs", Occurrences: []*scippb.Occurrence{def(pre+"Single#", 0)}},
			{RelativePath: "C.cs", Occurrences: []*scippb.Occurrence{
				def(pre+"User#", 0),
				scipCallsiteRef(pre+"Policy#", 1, 4, 10),
				scipCallsiteRef(pre+"Policy#Bulkhead().", 1, 11, 19),
				scipCallsiteRef(pre+"Single#", 2, 4, 10),
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	opts := SCIPOptionsFor("csharp", func(string) []byte { return nil })
	opts.CSharp = &CSharpCompiler{FileEdges: [][2]string{{"C.cs", "E.cs"}, {"C.cs", "D.cs"}, {"Linked.cs", "D.cs"}}}
	res, err := ScoreSCIPWith(raw, "s", nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[[2]string]bool{{"C.cs", "B.cs"}: true, {"C.cs", "D.cs"}: true, {"C.cs", "E.cs"}: true}
	if len(res.FileEdges) != len(want) {
		t.Errorf("file edges %v, want %v", res.FileEdges, want)
	}
	for e := range want {
		if !res.FileEdges[e] {
			t.Errorf("missing file edge %v (have %v)", e, res.FileEdges)
		}
	}
	if res.FileEdges[[2]string{"C.cs", "A.cs"}] {
		t.Error("C.cs -> A.cs: a reference to the partial Policy names no one file")
	}
	if got := res.Counts[CntScipCSharpFileEdgesMultiDefined]; got != 1 {
		t.Errorf("%s = %d, want 1 (C.cs -> A.cs)", CntScipCSharpFileEdgesMultiDefined, got)
	}
	if got := res.Counts[CntCSharpFileEdgesAdded]; got != 1 {
		t.Errorf("%s = %d, want 1 (C.cs -> E.cs; Linked.cs is no document of the index)", CntCSharpFileEdgesAdded, got)
	}
	// Any other producer keeps the first definer's edge.
	if r2, err := ScoreSCIPWith(raw, "s", nil, SCIPOptions{}); err != nil || !r2.FileEdges[[2]string{"C.cs", "A.cs"}] {
		t.Errorf("without the C# rule the first definer's edge stays: %v %v", r2.FileEdges, err)
	}
}
