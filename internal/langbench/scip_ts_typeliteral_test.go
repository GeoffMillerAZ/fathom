package langbench

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestTSAnonymousTypeLiteralMember (owner ruling S3, K19, LB-F511) reads
// scip-typescript 0.4.0's own symbols (a fixture indexed in wave lf7): a member
// whose innermost `typeLiteral<n>:` is a type argument's, a parameter's, a
// variable's, a member's or a type parameter's type literal is anonymous; one
// directly under a type alias (`Own#typeLiteral0:`) is the alias's own body.
func TestTSAnonymousTypeLiteralMember(t *testing.T) {
	const pre = "scip-typescript npm fixture-tl 1.0.0 src/`a.tsx`/"
	for sym, want := range map[string]bool{
		pre + "ButtonProps#Props:typeLiteral2:disabled.":    true,
		pre + "ButtonFn().(`{ value }`)typeLiteral7:value.": true,
		pre + "anno.typeLiteral8:a.":                        true,
		pre + "anno.typeLiteral8:run().":                    true,
		pre + "Iface#field.typeLiteral3:inner.":             true,
		pre + "Iface#m().typeLiteral4:ret.":                 true,
		pre + "generic().[T]typeLiteral12:k.":               true,
		pre + "C#p.typeLiteral13:q.":                        true,
		pre + "Own#typeLiteral0:nested.typeLiteral1:deep.":  true,
		"scip-typescript npm @headlessui/react 2.2.10 src/components/button/`button.tsx`/ButtonProps#Props:typeLiteral1:disabled.": true,
		"scip-typescript npm my  pkg 1.0.0 `a.ts`/x.typeLiteral0:y.":                                                               true, // a double space escapes a space in the package name
		pre + "Own#typeLiteral0:open.":           false, // a type alias's own body
		pre + "Own#typeLiteral0:close().":        false,
		pre + "Own#typeLiteral0:nested.":         false,
		pre + "Iface#field.":                     false,
		pre + "anno.":                            false,
		pre + "typeLiteral0:":                    false, // an object literal's property `typeLiteral0:` names no member after it
		pre + "mode0:":                           false,
		pre + "`typeLiteral0:x`.":                false, // escaped text is a name, not a descriptor
		"local 7":                                false,
		"scip-go gomod x v1 `x`/typeLiteral0:y.": false,
		pre + "broken(":                          false,
	} {
		if got := tsAnonymousTypeLiteralMember(sym); got != want {
			t.Errorf("tsAnonymousTypeLiteralMember(%q) = %v, want %v", sym, got, want)
		}
	}
}

// TestScoreSCIPTypeScriptTypeLiteralMembers (owner ruling S3, K19, LB-F511):
// def_mapping leaves a member of an anonymous type literal out of its
// denominator, counted (scip_defs_type_literal) and disclosed, the ones the
// dialect names as well (a method signature) out of the numerator alike
// (scip_defs_type_literal_named); a type alias's own members stay; typed_recall
// and every other count are unchanged; and the rule is TypeScript's alone.
func TestScoreSCIPTypeScriptTypeLiteralMembers(t *testing.T) {
	const pre = "scip-typescript npm app 1.0.0 src/`a.tsx`/"
	src := "type Props<T, P> = T & P;\n" + // 0
		"export type Own = { open: boolean; close(): void; nested: { deep: number } };\n" + // 1
		"export type ButtonProps = Props<string, { disabled?: boolean }>;\n" + // 2
		"export function ButtonFn({ value }: { value?: boolean }) { return value; }\n" + // 3
		"export let anno: { a: number; run(): void } = { a: 1, run() {} };\n" + // 4
		"export function use(o: ButtonProps) { return o.disabled && anno.a; }\n" // 5
	def := func(sym string, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-typescript", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: "src/a.tsx", Occurrences: []*scippb.Occurrence{
			def(pre+"Props#", 0, 5, 10),
			def(pre+"Own#", 1, 12, 15),
			def(pre+"Own#typeLiteral0:open.", 1, 20, 24),
			def(pre+"Own#typeLiteral0:close().", 1, 35, 40),
			def(pre+"Own#typeLiteral0:nested.", 1, 50, 56),
			def(pre+"Own#typeLiteral0:nested.typeLiteral1:deep.", 1, 60, 64),
			def(pre+"ButtonProps#", 2, 12, 23),
			def(pre+"ButtonProps#Props:typeLiteral2:disabled.", 2, 42, 50),
			def(pre+"ButtonFn().", 3, 16, 24),
			def(pre+"ButtonFn().(`{ value }`)", 3, 25, 34),
			def(pre+"ButtonFn().(`{ value }`)typeLiteral3:value.", 3, 39, 44),
			def(pre+"anno.", 4, 11, 15),
			def(pre+"anno.typeLiteral4:a.", 4, 19, 20),
			def(pre+"anno.typeLiteral4:run().", 4, 30, 33),
			def(pre+"use().", 5, 16, 19),
			ref(pre+"ButtonProps#", 5, 23, 34),
			ref(pre+"ButtonProps#Props:typeLiteral2:disabled.", 5, 47, 55),
			ref(pre+"anno.", 5, 59, 63),
			ref(pre+"anno.typeLiteral4:a.", 5, 64, 65),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	node := func(name, kind string, line int) core.SymbolNode {
		return core.SymbolNode{Service: "s", Package: "src/a", Name: name, Kind: core.SourceKind(kind), FilePath: "src/a.tsx", Line: line, EndLine: line}
	}
	nodes := []core.SymbolNode{
		node("Props", "TSType", 1), node("Own", "TSType", 2), node("Own.open", "JSField", 2), node("Own.close", "JSMethod", 2),
		node("Own.nested", "JSField", 2), node("ButtonProps", "TSType", 3), node("ButtonFn", "JSFunc", 4),
		node("anno", "JSConst", 5), node("run", "JSMethod", 5), node("use", "JSFunc", 6),
	}
	read := func(string) []byte { return []byte(src) }
	ts, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("typescript", read))
	if err != nil {
		t.Fatal(err)
	}
	js, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("javascript", read))
	if err != nil {
		t.Fatal(err)
	}
	if got := [4]int{ts.Counts[CntScipDefs], ts.Counts[CntScipDefsMapped], ts.Counts[CntScipDefsTypeLiteral], ts.Counts[CntScipDefsTypeLiteralNamed]}; got != [4]int{9, 9, 5, 1} {
		t.Errorf("typescript: defs, mapped, type_literal, type_literal_named = %v, want [9 9 5 1] (deep, disabled, value, a and run out; run named)", got)
	}
	if got := [4]int{js.Counts[CntScipDefs], js.Counts[CntScipDefsMapped], js.Counts[CntScipDefsTypeLiteral], js.Counts[CntScipDefsTypeLiteralNamed]}; got != [4]int{14, 10, 0, 0} {
		t.Errorf("javascript (out of the rule's scope): defs, mapped, type_literal, type_literal_named = %v, want [14 10 0 0]", got)
	}
	for _, k := range []string{CntScipEdges, CntScipEdgesTyped, CntCallTruth, CntCallFathom} {
		if ts.Counts[k] != js.Counts[k] {
			t.Errorf("%s moved with the rule: %d -> %d; only def_mapping may", k, js.Counts[k], ts.Counts[k])
		}
	}
	found := false
	for _, n := range SCIPRuleNotes("app", "scip-typescript", ts.Counts) {
		if strings.Contains(n, "5 members of an anonymous type literal") && strings.Contains(n, CntScipDefsTypeLiteral) && strings.Contains(n, "1 of them") && strings.Contains(n, "S3") {
			found = true
		}
	}
	if !found {
		t.Errorf("SCIPRuleNotes = %q, want the type-literal disclosure with its counts and ruling", SCIPRuleNotes("app", "scip-typescript", ts.Counts))
	}
	for _, n := range SCIPRuleNotes("app", "scip-typescript", js.Counts) {
		if strings.Contains(n, CntScipDefsTypeLiteral) {
			t.Errorf("a reading the rule moved nothing for discloses %q", n)
		}
	}
}
