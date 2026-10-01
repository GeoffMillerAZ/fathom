package langbench

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

func scipCallsiteDef(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
	return &scippb.Occurrence{Symbol: sym, Range: rng, SymbolRoles: int32(scippb.SymbolRole_Definition), EnclosingRange: enc}
}

func scipCallsiteRef(sym string, rng ...int32) *scippb.Occurrence {
	return &scippb.Occurrence{Symbol: sym, Range: rng}
}

// TestScoreSCIPPythonCallSites (LB-F65, O260): scip-python's vocabulary read
// at the call site. L2 is call sites: a function or method referenced without
// a call (passed as a value, a decorator applied by name) is not one and is
// counted unwritten; a class, an attribute or a local the index resolves at a
// written call (`name(`) is one — Python constructs by calling the class, and
// scip-python names the class (`Foo#`) there, never a constructor.
func TestScoreSCIPPythonCallSites(t *testing.T) {
	const pre = "scip-python python app 0.1 m/"
	src := "class Foo:\n" + // 0
		"    def bar(self):\n" + // 1
		"        return 1\n" + // 2
		"\n" + // 3
		"def run(cb):\n" + // 4
		"    f = Foo()\n" + // 5
		"    register(Foo.bar)\n" + // 6
		"    f.bar()\n" + // 7
		"    f.handler(1)\n" + // 8
		"    cb()\n" + // 9
		"    x: Foo = f\n" // 10
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-python", Version: "0.6.6"}},
		Documents: []*scippb.Document{{RelativePath: "m.py", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef(pre+"Foo#", []int32{0, 0, 2, 16}, 0, 6, 9),
			scipCallsiteDef(pre+"Foo#bar().", []int32{1, 4, 2, 16}, 1, 8, 11),
			scipCallsiteDef(pre+"run().", []int32{4, 0, 10, 14}, 4, 4, 7),
			scipCallsiteDef("local 0", nil, 4, 8, 10),
			scipCallsiteRef(pre+"Foo#", 5, 8, 11),         // Foo(): a call
			scipCallsiteRef(pre+"register().", 6, 4, 12),  // register(...): a call
			scipCallsiteRef(pre+"Foo#bar().", 6, 17, 20),  // Foo.bar as a value: no call
			scipCallsiteRef(pre+"Foo#bar().", 7, 6, 9),    // f.bar(): a call
			scipCallsiteRef(pre+"Foo#handler.", 8, 6, 13), // an attribute called
			scipCallsiteRef("local 0", 9, 4, 6),           // a parameter called
			scipCallsiteRef(pre+"Foo#", 10, 7, 10),        // an annotation: no call
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "Foo", Kind: "PythonClass", FilePath: "m.py", Line: 1, EndLine: 3},
		{Service: "s", Name: "Foo.bar", Kind: "PythonFunc", FilePath: "m.py", Line: 2, EndLine: 3},
		{Service: "s", Name: "run", Kind: "PythonFunc", FilePath: "m.py", Line: 5, EndLine: 11, Calls: []string{"Foo", "register", "f.bar", "f.handler", "cb"}},
	}
	plain, err := ScoreSCIP(raw, "s", nodes)
	if err != nil {
		t.Fatal(err)
	}
	if !plain.CallTruth[CallPair{"m.py", "run", "bar"}] || plain.CallTruth[CallPair{"m.py", "run", "Foo"}] {
		t.Fatalf("zero options changed: %v", plain.CallTruth)
	}
	res, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("python", func(string) []byte { return []byte(src) }))
	if err != nil {
		t.Fatal(err)
	}
	want := map[CallPair]bool{
		{"m.py", "run", "Foo"}: true, {"m.py", "run", "register"}: true, {"m.py", "run", "bar"}: true,
		{"m.py", "run", "handler"}: true, {"m.py", "run", "cb"}: true,
	}
	if !reflect.DeepEqual(res.CallTruth, want) {
		t.Errorf("call truth = %v, want %v", res.CallTruth, want)
	}
	if res.Counts[CntScipCallsUnwritten] != 1 {
		t.Errorf("unwritten = %d, want 1 (Foo.bar passed as a value)", res.Counts[CntScipCallsUnwritten])
	}
	if res.Counts[CntScipCalledNames] != 3 {
		t.Errorf("called names = %d, want 3 (the class, the attribute, the parameter)", res.Counts[CntScipCalledNames])
	}
}

// TestScoreSCIPGoCallSites (LB-F65, O260): scip-go writes an interface's
// method as a term (`Getter#Get.`), so a call through an interface, like a
// call of a func-typed field, is a term reference written as a call; it is a
// call site. A callable referenced as a value stays in the truth (Go's
// extractor records function values, VS-6), and a term never called (a
// constant argument) is no call.
func TestScoreSCIPGoCallSites(t *testing.T) {
	const pre = "scip-go gomod example.com/app v1 `example.com/app`/"
	src := "package app\n" + // 0
		"func Use(g Getter, h handlerT) {\n" + // 1
		"\tg.Get(StatusOK)\n" + // 2
		"\th.fn()\n" + // 3
		"\tRegister(Use)\n" + // 4
		"}\n" // 5
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-go", Version: "0.2.7"}},
		Documents: []*scippb.Document{{RelativePath: "app.go", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef(pre+"Use().", []int32{1, 0, 5, 1}, 1, 5, 8),
			scipCallsiteRef(pre+"Getter#Get.", 2, 3, 6),
			scipCallsiteRef(pre+"StatusOK.", 2, 7, 15),
			scipCallsiteRef(pre+"handlerT#fn.", 3, 3, 5),
			scipCallsiteRef(pre+"Register().", 4, 1, 9),
			scipCallsiteRef(pre+"Use().", 4, 10, 13),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{{Service: "s", Name: "Use", Kind: "GoFunc", FilePath: "app.go", Line: 2, EndLine: 6}}
	res, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("go", func(string) []byte { return []byte(src) }))
	if err != nil {
		t.Fatal(err)
	}
	want := map[CallPair]bool{
		{"app.go", "Use", "Get"}: true, {"app.go", "Use", "fn"}: true,
		{"app.go", "Use", "Register"}: true, {"app.go", "Use", "Use"}: true,
	}
	if !reflect.DeepEqual(res.CallTruth, want) {
		t.Errorf("call truth = %v, want %v", res.CallTruth, want)
	}
	if res.Counts[CntScipCallsUnwritten] != 0 || res.Counts[CntScipCalledNames] != 2 {
		t.Errorf("unwritten %d called names %d, want 0 and 2", res.Counts[CntScipCallsUnwritten], res.Counts[CntScipCalledNames])
	}
}
