package langbench

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScipBareNameJava pins scip-java's naming (LB-F71): a constructor,
// written `<init>`, is named as its class — which is also how new Foo(...)
// is written as a call — and every other leaf reads as it does for any
// producer.
func TestScipBareNameJava(t *testing.T) {
	const p = "semanticdb maven maven/com.google.guava/guava 33.7.1-jre "
	for sym, want := range map[string]string{
		p + "com/google/common/hash/MessageDigestHashFunction#`<init>`().":     "MessageDigestHashFunction",
		p + "com/google/common/hash/MessageDigestHashFunction#`<init>`(+1).":   "MessageDigestHashFunction",
		p + "com/google/common/graph/MapRetrievalCache#CacheEntry#`<init>`().": "CacheEntry",
		p + "com/google/common/base/Preconditions#checkNotNull().":             "checkNotNull",
		p + "com/google/common/base/Preconditions#checkNotNull(+1).":           "checkNotNull",
		p + "com/google/common/base/Optional#":                                 "Optional",
		p + "com/google/common/base/Optional#absent().":                        "absent",
		p + "com/google/common/collect/ImmutableList#EMPTY.":                   "EMPTY",
		p + "com/google/common/hash/BloomFilter#Strategy#`<init>`().":          "Strategy",
		p + "com/google/common/base/Absent#`<clinit>`().":                      "<clinit>",
	} {
		if got, _ := scipBareName(sym); got != want {
			t.Errorf("scipBareName(%q) = %q, want %q", sym, got, want)
		}
	}
	if _, callable := scipBareName(p + "com/google/common/base/Optional#`<init>`()."); !callable {
		t.Error("a constructor is callable: new Foo() is a call to it")
	}
	// A symbol of another producer keeps its leaf: `<init>` is not renamed outside scip-java's.
	if got, _ := scipBareName("scip-other pkg v 1 a/Foo#`<init>`()."); got != "<init>" {
		t.Errorf("another producer's `<init>` = %q, want it untouched", got)
	}
}

// javaAnonFixture is one Java file and scip-java's index of it: the
// anonymous class's method `invoke` is `local 1` (kind Method, display name
// invoke, with an enclosing_range), its parameter `local 2`.
func javaAnonFixture(t *testing.T) []byte {
	t.Helper()
	const pre = "semanticdb maven . . retrofit2/"
	// class Retrofit {                                   // 0
	//   Object create(Class<?> service) {                // 1
	//     return Proxy.newProxyInstance(                 // 2
	//         new InvocationHandler() {                  // 3
	//           @Override                                // 4
	//           public Object invoke(Method method) {    // 5
	//             return method.getDeclaringClass();     // 6
	//           }                                        // 7
	//         });                                        // 8
	//   }                                                // 9
	// }                                                  // 10
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-java", Version: "0.12.3"}},
		Documents: []*scippb.Document{{RelativePath: "Retrofit.java",
			Symbols: []*scippb.SymbolInformation{
				{Symbol: "local 1", Kind: scippb.SymbolInformation_Method, DisplayName: "invoke"},
				{Symbol: "local 2", DisplayName: "method"},
			},
			Occurrences: []*scippb.Occurrence{
				scipCallsiteDef(pre+"Retrofit#", []int32{0, 0, 10, 1}, 0, 6, 14),
				scipCallsiteDef(pre+"Retrofit#create().", []int32{1, 2, 9, 3}, 1, 9, 15),
				scipCallsiteRef("semanticdb maven jdk 21 java/lang/reflect/Proxy#newProxyInstance().", 2, 17, 33),
				scipCallsiteRef("semanticdb maven jdk 21 java/lang/reflect/InvocationHandler#", 3, 12, 29),
				scipCallsiteDef("local 1", []int32{4, 10, 7, 11}, 5, 24, 30),
				scipCallsiteDef("local 2", []int32{5, 31, 5, 44}, 5, 38, 44),
				scipCallsiteRef("local 2", 6, 19, 25),
				scipCallsiteRef("semanticdb maven jdk 21 java/lang/reflect/Method#getDeclaringClass().", 6, 26, 43),
			}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestScoreSCIPJavaAnonymousClassMethods (O458): scip-java names an
// anonymous class's method `local N`; the call truth maps such a local, a
// Method with an enclosing_range whose name is written at its range, to the
// extractor's same-named definition inside it, so a call in its body is the
// method's, as the dialect and the L1 oracle attribute it. An extractor that
// defines no such method keeps the calls on the enclosing named method, as
// before; the local is never counted among the index's definitions.
func TestScoreSCIPJavaAnonymousClassMethods(t *testing.T) {
	raw := javaAnonFixture(t)
	dialect := []core.SymbolNode{
		{Service: "s", Name: "Retrofit", Kind: "JavaClass", FilePath: "Retrofit.java", Line: 1, EndLine: 11},
		{Service: "s", Name: "Retrofit.create", Kind: "JavaMethod", FilePath: "Retrofit.java", Line: 2, EndLine: 10,
			Calls: []string{"newProxyInstance", "InvocationHandler"}},
		{Service: "s", Name: "invoke", Kind: "JavaMethod", FilePath: "Retrofit.java", Line: 6, EndLine: 8,
			Calls: []string{"getDeclaringClass"}},
	}
	res, err := ScoreSCIPWith(raw, "s", dialect, SCIPOptionsFor("java", nil))
	if err != nil {
		t.Fatal(err)
	}
	want := map[CallPair]bool{
		{"Retrofit.java", "create", "newProxyInstance"}:  true,
		{"Retrofit.java", "invoke", "getDeclaringClass"}: true,
	}
	if !reflect.DeepEqual(res.CallTruth, want) {
		t.Errorf("dialect: call truth = %v, want %v", res.CallTruth, want)
	}
	if res.Counts[CntScipJavaLocalMethods] != 1 {
		t.Errorf("dialect: local methods mapped = %d, want 1", res.Counts[CntScipJavaLocalMethods])
	}
	if res.Counts[CntScipDefs] != 2 || res.Counts[CntScipDefsMapped] != 2 {
		t.Errorf("dialect: scip defs %d mapped %d, want 2 and 2 (a local is no index definition)", res.Counts[CntScipDefs], res.Counts[CntScipDefsMapped])
	}

	incumbent := []core.SymbolNode{
		{Service: "s", Name: "Retrofit", Kind: "JavaClass", FilePath: "Retrofit.java", Line: 1, EndLine: 11},
		{Service: "s", Name: "Retrofit.create", Kind: "JavaMethod", FilePath: "Retrofit.java", Line: 2, EndLine: 10,
			Calls: []string{"newProxyInstance", "getDeclaringClass"}},
	}
	res, err = ScoreSCIPWith(raw, "s", incumbent, SCIPOptionsFor("java", nil))
	if err != nil {
		t.Fatal(err)
	}
	want = map[CallPair]bool{
		{"Retrofit.java", "create", "newProxyInstance"}:  true,
		{"Retrofit.java", "create", "getDeclaringClass"}: true,
	}
	if !reflect.DeepEqual(res.CallTruth, want) {
		t.Errorf("incumbent: call truth = %v, want %v (no anonymous method defined: the enclosing method's)", res.CallTruth, want)
	}
	if res.Counts[CntScipJavaLocalMethods] != 0 {
		t.Errorf("incumbent: local methods mapped = %d, want 0", res.Counts[CntScipJavaLocalMethods])
	}
}

// TestScoreSCIPJavaLocalNeedsMethodKind (O458): a local that is not a
// method (a variable, a parameter) is never a caller, whatever is defined
// near it, and another producer's locals are untouched.
func TestScoreSCIPJavaLocalNeedsMethodKind(t *testing.T) {
	raw := javaAnonFixture(t)
	nodes := []core.SymbolNode{
		{Service: "s", Name: "Retrofit.create", Kind: "JavaMethod", FilePath: "Retrofit.java", Line: 2, EndLine: 10,
			Calls: []string{"getDeclaringClass"}},
		{Service: "s", Name: "method", Kind: "JavaMethod", FilePath: "Retrofit.java", Line: 6, EndLine: 6},
	}
	res, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("java", nil))
	if err != nil {
		t.Fatal(err)
	}
	if !res.CallTruth[CallPair{"Retrofit.java", "create", "getDeclaringClass"}] {
		t.Errorf("a parameter local became a caller: truth %v", res.CallTruth)
	}
	idx := &scippb.Index{}
	if err := proto.Unmarshal(raw, idx); err != nil {
		t.Fatal(err)
	}
	idx.Metadata.ToolInfo.Name = "scip-other"
	other, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes = append(nodes, core.SymbolNode{Service: "s", Name: "invoke", Kind: "JavaMethod", FilePath: "Retrofit.java", Line: 6, EndLine: 8})
	if res, err = ScoreSCIPWith(other, "s", nodes, SCIPOptionsFor("java", nil)); err != nil {
		t.Fatal(err)
	}
	if res.Counts[CntScipJavaLocalMethods] != 0 || !res.CallTruth[CallPair{"Retrofit.java", "create", "getDeclaringClass"}] {
		t.Errorf("another producer's local was mapped: truth %v", res.CallTruth)
	}
}
