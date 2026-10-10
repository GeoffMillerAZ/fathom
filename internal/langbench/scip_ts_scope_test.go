package langbench

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScoreSCIPDefScope (owner ruling L5, K19, LB-F467, O788): a scip-typescript
// definition in a document outside the reading's measured scope (a TypeScript
// declaration file beside JavaScript, eslint's lib/types/*.d.ts) is left out of
// def_mapping's denominator and counted (scip_defs_out_of_scope), the
// out-of-scope principle ClangScope (LB-F292) and the oracle side apply; the
// rule is TypeScript's and JavaScript's, it needs the scope lang-oracle hands
// over (nil scopes nothing), and typed_recall, the call truth and every other
// count are unchanged.
func TestScoreSCIPDefScope(t *testing.T) {
	const js = "scip-typescript npm app 1.0.0 lib/`a.js`/"
	const dts = "scip-typescript npm app 1.0.0 lib/types/`index.d.ts`/"
	srcs := map[string]string{
		"lib/a.js": "const types = require(\"./types\");\n" + // 0
			"function make(opts) {\n" + // 1
			"  return new types.Rule(opts);\n" + // 2
			"}\n" + // 3
			"module.exports = { make };\n", // 4
		"lib/types/index.d.ts": "export interface RuleOptions { strict: boolean }\n" + // 0
			"export class Rule {\n" + // 1
			"  constructor(opts: RuleOptions);\n" + // 2
			"}\n", // 3
	}
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-typescript", Version: "0.4.0"}},
		Documents: []*scippb.Document{
			{RelativePath: "lib/a.js", Occurrences: []*scippb.Occurrence{
				def(js+"types.", nil, 0, 6, 11),
				def(js+"make().", []int32{1, 0, 3, 1}, 1, 9, 13),
				ref(js+"types.", 2, 13, 18),
				ref(dts+"Rule#", 2, 19, 23),
			}},
			{RelativePath: "lib/types/index.d.ts", Occurrences: []*scippb.Occurrence{
				def(dts+"RuleOptions#", []int32{0, 0, 0, 48}, 0, 17, 28),
				def(dts+"RuleOptions#strict.", nil, 0, 31, 37),
				def(dts+"Rule#", []int32{1, 0, 3, 1}, 1, 13, 17),
				def(dts+"Rule#`<constructor>`().", []int32{2, 2, 2, 33}, 2, 2, 13),
				ref(dts+"RuleOptions#", 2, 20, 31),
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Package: "lib/a", Name: "types", Kind: "JSConst", FilePath: "lib/a.js", Line: 1, EndLine: 1},
		{Service: "s", Package: "lib/a", Name: "make", Kind: "JSFunc", FilePath: "lib/a.js", Line: 2, EndLine: 4},
	}
	read := func(p string) []byte { return []byte(srcs[p]) }
	scope := map[string]bool{"lib/a.js": true}

	unscoped, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("javascript", read))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"javascript", "typescript"} {
		opts := SCIPOptionsFor(d, read)
		if !opts.DefScoped {
			t.Fatalf("SCIPOptionsFor(%s) must scope def_mapping (ruling L5)", d)
		}
		opts.DefScope = scope
		scoped, err := ScoreSCIPWith(raw, "s", nodes, opts)
		if err != nil {
			t.Fatal(err)
		}
		if unscoped.Counts[CntScipDefs] != 6 || unscoped.Counts[CntScipDefsMapped] != 2 || unscoped.Counts[CntScipDefsOutOfScope] != 0 {
			t.Errorf("no scope handed over: defs %d mapped %d out %d, want 6 2 0", unscoped.Counts[CntScipDefs], unscoped.Counts[CntScipDefsMapped], unscoped.Counts[CntScipDefsOutOfScope])
		}
		if scoped.Counts[CntScipDefs] != 2 || scoped.Counts[CntScipDefsMapped] != 2 || scoped.Counts[CntScipDefsOutOfScope] != 4 {
			t.Errorf("%s scoped: defs %d mapped %d out %d, want 2 2 4", d, scoped.Counts[CntScipDefs], scoped.Counts[CntScipDefsMapped], scoped.Counts[CntScipDefsOutOfScope])
		}
		for _, k := range []string{CntScipEdges, CntScipEdgesTyped, CntCallTruth, CntCallFathom} {
			if unscoped.Counts[k] != scoped.Counts[k] {
				t.Errorf("%s: %s moved with the scope: %d -> %d; only def_mapping's denominator may", d, k, unscoped.Counts[k], scoped.Counts[k])
			}
		}
		if n := DefScopeNote("eslint", scoped.Counts); !strings.Contains(n, "4 definitions") || !strings.Contains(n, CntScipDefsOutOfScope) || !strings.Contains(n, "L5") {
			t.Errorf("DefScopeNote = %q, want the out-of-scope disclosure with its count and ruling", n)
		}
	}
	if n := DefScopeNote("eslint", unscoped.Counts); n != "" {
		t.Errorf("a subject the rule moved nothing for discloses %q", n)
	}
	for _, d := range []string{"go", "python", "rust", "java", "csharp", "php", "ruby", "dart", "c", "cpp"} {
		if SCIPOptionsFor(d, read).DefScoped {
			t.Errorf("SCIPOptionsFor(%s) scopes def_mapping: ruling L5 is TypeScript's and JavaScript's (K29)", d)
		}
	}
}

// TestManifestOwnsVariantExtensions (O749, ruling L5's commit, LB-F468): a
// dialect's measured files include its grammar variants' extensions, so the
// TypeScript reading measures .tsx (the tsx variant's documents are ingest's
// and scip-typescript's TypeScript); typescript is the only dialect with a
// variant, so no other dialect's measured file set changes.
func TestManifestOwnsVariantExtensions(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "extractor", "dialects", "*", "dialect.json"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no dialect manifests: %v", err)
	}
	var withVariants []string
	var ts, js Manifest
	for _, p := range dirs {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		m, err := ParseManifest(b)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, v := range m.Variants {
			if len(v.Extensions) > 0 {
				withVariants = append(withVariants, m.ID)
				break
			}
		}
		switch m.ID {
		case "typescript":
			ts = m
		case "javascript":
			js = m
		}
	}
	if !slices.Equal(withVariants, []string{"typescript"}) {
		t.Errorf("dialects with variant extensions = %v, want [typescript] alone (any other's measured file set would change: K29)", withVariants)
	}
	for path, want := range map[string]bool{"src/Button.tsx": true, "src/a.ts": true, "src/b.mts": true, "src/UPPER.TSX": true, "src/a.js": false, "src/a.jsx": false} {
		if got := ts.Owns(path); got != want {
			t.Errorf("typescript Owns(%q) = %v, want %v", path, got, want)
		}
	}
	if js.Owns("src/Button.tsx") || !js.Owns("src/a.jsx") {
		t.Errorf("javascript owns .tsx or not .jsx")
	}
}
