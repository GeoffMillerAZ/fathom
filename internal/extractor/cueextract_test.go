package extractor_test

import (
	"os"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const cueFixture = `package config

// #Service defines a Kubernetes service.
#Service: {
  name: string
  port: int & >0
}

// #Deployment defines a deployment spec.
#Deployment: {
  replicas: int | *1
  image: string
}

// Not a definition — should be a plain CUEField.
someValue: "hello"
`

func TestCUEExtractor_Basic(t *testing.T) {
	x := extractor.NewCUEExtractor()
	nodes, err := x.Extract("svc", "schema.cue", []byte(cueFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("#Service", func(t *testing.T) {
		n, ok := byName["#Service"]
		if !ok {
			t.Fatal("expected symbol #Service")
		}
		if n.Kind != core.KindCUEDefinition {
			t.Errorf("Kind: got %q, want KindCUEDefinition", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("#Deployment", func(t *testing.T) {
		n, ok := byName["#Deployment"]
		if !ok {
			t.Fatal("expected symbol #Deployment")
		}
		if n.Kind != core.KindCUEDefinition {
			t.Errorf("Kind: got %q, want KindCUEDefinition", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
		// #Service's struct literal spans multiple lines: EndLine must be
		// the closing brace's line, strictly after Line.
		if n.EndLine <= n.Line {
			t.Errorf("EndLine: got %d, want > Line (%d)", n.EndLine, n.Line)
		}
	})

	t.Run("someValue_is_KindCUEField", func(t *testing.T) {
		n, ok := byName["someValue"]
		if !ok {
			t.Fatal("expected symbol someValue (non-definition top-level field)")
		}
		if n.Kind != core.KindCUEField {
			t.Errorf("someValue Kind: got %q, want KindCUEField", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("someValue Tier: got %q, want TierVerified", n.Tier)
		}
		// Single-line field: EndLine equals Line.
		if n.EndLine != n.Line {
			t.Errorf("someValue EndLine: got %d, want == Line (%d)", n.EndLine, n.Line)
		}
	})

	// LT-4: nested fields are now visible with dotted names — #Service and
	// #Deployment each have two child fields, so the previously-invisible
	// #Service.name / #Service.port / #Deployment.replicas /
	// #Deployment.image are all present alongside the 3 top-level fields.
	t.Run("nested_dotted_fields", func(t *testing.T) {
		for _, want := range []string{"#Service.name", "#Service.port", "#Deployment.replicas", "#Deployment.image"} {
			n, ok := byName[want]
			if !ok {
				t.Errorf("expected nested symbol %q", want)
				continue
			}
			if n.Kind != core.KindCUEField {
				t.Errorf("%s Kind: got %q, want KindCUEField", want, n.Kind)
			}
		}
	})

	// LT-1/ADR-021 §1: Package is read from the file's `package config`
	// clause, not derived from the filename ("schema").
	t.Run("package_from_clause", func(t *testing.T) {
		if n, ok := byName["#Service"]; ok && n.Package != "config" {
			t.Errorf("Package = %q, want %q (from the package clause)", n.Package, "config")
		}
	})

	if len(nodes) != 7 {
		t.Errorf("expected 7 nodes (3 top-level + 4 nested), got %d", len(nodes))
	}
}

// TestCUEExtractor_DepthCap verifies that struct-literal recursion stops at
// cueMaxDepth (6): a field 7 levels deep is never emitted, but the field at
// exactly the cap (depth 6) is.
func TestCUEExtractor_DepthCap(t *testing.T) {
	const deepFixture = `package deep

a: b: c: d: e: f: g: int
`
	x := extractor.NewCUEExtractor()
	nodes, err := x.Extract("svc", "deep.cue", []byte(deepFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = true
	}

	for _, want := range []string{"a", "a.b", "a.b.c", "a.b.c.d", "a.b.c.d.e", "a.b.c.d.e.f"} {
		if !byName[want] {
			t.Errorf("expected symbol %q within the depth cap, not found", want)
		}
	}
	if byName["a.b.c.d.e.f.g"] {
		t.Error("a.b.c.d.e.f.g exceeds cueMaxDepth (6) and must not be emitted")
	}
	if len(nodes) != 6 {
		t.Errorf("expected exactly 6 nodes (depth 1..6), got %d: %v", len(nodes), nodes)
	}
}

// TestCUEExtractor_Imports verifies that import declarations are captured as
// KindCUEImport reference observations.
func TestCUEExtractor_Imports(t *testing.T) {
	const importFixture = `package withimports

import (
	"list"
	"strings"
)

value: list.Contains([1, 2, 3], 1)
`
	x := extractor.NewCUEExtractor()
	nodes, err := x.Extract("svc", "withimports.cue", []byte(importFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	var imports []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindCUEImport {
			imports = append(imports, n)
		}
	}
	if len(imports) != 2 {
		t.Fatalf("expected 2 import nodes, got %d: %v", len(imports), imports)
	}
	names := map[string]bool{}
	for _, n := range imports {
		names[n.Name] = true
		if n.Tier != core.TierVerified {
			t.Errorf("import %q Tier = %q, want TierVerified", n.Name, n.Tier)
		}
		if n.Package != "withimports" {
			t.Errorf("import %q Package = %q, want %q", n.Name, n.Package, "withimports")
		}
	}
	for _, want := range []string{"list", "strings"} {
		if !names[want] {
			t.Errorf("expected import %q, got %v", want, names)
		}
	}
}

// TestCUEExtractor_MalformedImportNoPanic is the crash-class regression for
// the import-capture path: `import 42` parses to a partial AST whose
// ImportSpec has an empty Path.Value, and literal.Unquote("") panics with
// index-out-of-range. There is no recover() anywhere in fusion/mcp/cmd, so
// without the guard a single mid-edit save of a .cue file crashes `fathom
// ingest` and the stdio server outright. Extract must not panic and must
// still extract the rest of the file.
func TestCUEExtractor_MalformedImportNoPanic(t *testing.T) {
	const malformed = `package broken

import 42

good: "value"
`
	x := extractor.NewCUEExtractor()
	nodes, err := x.Extract("svc", "broken.cue", []byte(malformed))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var sawGood bool
	for _, n := range nodes {
		if n.Kind == core.KindCUEImport && n.Name == "" {
			t.Errorf("malformed import emitted an empty-named import node: %+v", n)
		}
		if n.Name == "good" {
			sawGood = true
		}
	}
	if !sawGood {
		t.Error("the well-formed remainder of the file was not extracted")
	}
}

// TestCUEExtractor_PackageFallback verifies that a file with NO package
// clause falls back to the filename-derived package (pkgFromPath), preserving
// the old behavior for package-less data fragments.
func TestCUEExtractor_PackageFallback(t *testing.T) {
	const noPackageFixture = `value: 42
`
	x := extractor.NewCUEExtractor()
	nodes, err := x.Extract("svc", "fragment.cue", []byte(noPackageFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	if nodes[0].Package != "fragment" {
		t.Errorf("Package = %q, want %q (filename fallback)", nodes[0].Package, "fragment")
	}
}

// TestCUEExtractor_RealConfigSchemas_SamePackage is the exact regression the
// grounding audit (wf_d0588a59) reproduced: internal/config/config_schema.cue
// and schema/config/schema.cue both declare `package config` but have
// different filenames. The old filename-derived heuristic reported two
// different, both-wrong Package values ("config_schema" and "schema"); reading
// the actual package clause reports "config" for both.
func TestCUEExtractor_RealConfigSchemas_SamePackage(t *testing.T) {
	paths := []string{
		"../config/config_schema.cue",
		"../../schema/config/schema.cue",
	}
	x := extractor.NewCUEExtractor()
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", p, err)
		}
		nodes, err := x.Extract("svc", p, src)
		if err != nil {
			t.Fatalf("Extract(%q): %v", p, err)
		}
		if len(nodes) == 0 {
			t.Fatalf("Extract(%q): no nodes", p)
		}
		for _, n := range nodes {
			if n.Package != "config" {
				t.Errorf("%s: symbol %q Package = %q, want %q", p, n.Name, n.Package, "config")
			}
		}
	}
}

// TestCUEExtractor_GoldenLayerSchema is the golden test against the REAL
// schema/semantic/schema.cue (ADR-021 §1 / LT-4): a representative sample of
// #Layer's nested field tree, sorted, must all be present now that struct
// literals recurse.
func TestCUEExtractor_GoldenLayerSchema(t *testing.T) {
	const path = "../../schema/semantic/schema.cue"
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	x := extractor.NewCUEExtractor()
	nodes, err := x.Extract("svc", path, src)
	if err != nil {
		t.Fatalf("Extract(%q): %v", path, err)
	}

	names := make([]string, 0, len(nodes))
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		names = append(names, n.Name)
		byName[n.Name] = n
	}
	sort.Strings(names)

	want := []string{
		"#Layer",
		"#Layer.budgets",
		"#Layer.budgets.max_claims_per_file",
		"#Layer.budgets.max_usd_per_run",
		"#Layer.corpus",
		"#Layer.corpus.include",
		"#Layer.name",
		"#Layer.serving",
		"#Layer.serving.drift_tracked",
		"#Layer.tier_profile",
	}
	for _, w := range want {
		if _, ok := byName[w]; !ok {
			t.Errorf("golden field tree: missing %q (got sorted names: %v)", w, names)
		}
	}
	if n, ok := byName["#Layer"]; ok && n.Package != "semantic" {
		t.Errorf("#Layer Package = %q, want %q", n.Package, "semantic")
	}
}

func TestCUEExtractor_LineNumbers(t *testing.T) {
	x := extractor.NewCUEExtractor()
	nodes, err := x.Extract("svc", "schema.cue", []byte(cueFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// #Service is on line 4 (after package line, blank, comment).
	if n, ok := byName["#Service"]; ok {
		if n.Line != 4 {
			t.Errorf("#Service line: got %d, want 4", n.Line)
		}
	}
}

func TestCUEExtractor_Determinism(t *testing.T) {
	x := extractor.NewCUEExtractor()
	n1, err := x.Extract("svc", "schema.cue", []byte(cueFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "schema.cue", []byte(cueFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Line != n2[i].Line {
			t.Errorf("non-deterministic at index %d: %+v vs %+v", i, n1[i], n2[i])
		}
	}
}

// cueRefFixture (VS-3) exercises unification, field type references, and
// list element types all within one package.
const cueRefFixture = `package kit

#Block: {
	name: string
}

#Profile: {
	kind: string
}

// unification: #Kit -> #Block
#Kit: #Block & {
	extra: string
}

// field type reference: profile -> #Profile
profile: #Profile

// list element type: uses -> #Block
uses: [...#Block]
`

// cueRefCrossPkgFixture (VS-3d) exercises a cross-package selector
// reference: #Wrapper -> kit.#Block.
const cueRefCrossPkgFixture = `package other

import "example.com/kit"

#Wrapper: kit.#Block & {
	label: string
}
`

// TestCUEReferenceEdgesEmitted is VS-3's row test (docs/plans/verify-spine.md):
// the CUE extractor must emit reference edges for all four reference forms
// instead of discarding them, landing in SymbolNode.Calls exactly like
// goextract.go's collectCalls — so find_callers/references work for CUE.
func TestCUEReferenceEdgesEmitted(t *testing.T) {
	x := extractor.NewCUEExtractor()

	nodes, err := x.Extract("svc", "kit/kit.cue", []byte(cueRefFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	hasCall := func(t *testing.T, n core.SymbolNode, want string) {
		t.Helper()
		for _, c := range n.Calls {
			if c == want {
				return
			}
		}
		t.Errorf("%s.Calls: want %q in %v", n.Name, want, n.Calls)
	}

	t.Run("unification", func(t *testing.T) {
		n, ok := byName["#Kit"]
		if !ok {
			t.Fatal("expected symbol #Kit")
		}
		hasCall(t, n, core.SymbolKey("svc", "kit", "#Block"))
	})

	t.Run("field_type_reference", func(t *testing.T) {
		n, ok := byName["profile"]
		if !ok {
			t.Fatal("expected symbol profile")
		}
		hasCall(t, n, core.SymbolKey("svc", "kit", "#Profile"))
	})

	t.Run("list_element_type", func(t *testing.T) {
		n, ok := byName["uses"]
		if !ok {
			t.Fatal("expected symbol uses")
		}
		hasCall(t, n, core.SymbolKey("svc", "kit", "#Block"))
	})

	t.Run("cross_package_import", func(t *testing.T) {
		nodes, err := x.Extract("svc", "other/other.cue", []byte(cueRefCrossPkgFixture))
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		var wrapper core.SymbolNode
		found := false
		for _, n := range nodes {
			if n.Name == "#Wrapper" {
				wrapper = n
				found = true
			}
		}
		if !found {
			t.Fatal("expected symbol #Wrapper")
		}
		hasCall(t, wrapper, "kit.#Block")
	})
}

// cueTestdataFixture references #Kit (defined elsewhere in the "kit"
// package) from a testdata/*.cue path.
const cueTestdataFixture = `package kit

myKit: #Kit & {
	extra: "value"
}
`

// TestCUETestdataUnificationCountsAsReaches is VS-4's row test: a
// testdata/*.cue fixture that unifies/references a definition must produce
// an edge of the SAME KIND (a Test-prefixed caller in Calls-derived edges)
// that internal/analysis/callgraph.TestImpact's reverse walk already
// recognizes for Go tests — so reachable_from/test_reaches is satisfiable for
// a CUE plan row with no new grammar, and with NO change to the reaches
// machinery itself.
func TestCUETestdataUnificationCountsAsReaches(t *testing.T) {
	x := extractor.NewCUEExtractor()

	t.Run("testdata_path_synthesizes_Test_caller", func(t *testing.T) {
		nodes, err := x.Extract("svc", "kit/testdata/fixture.cue", []byte(cueTestdataFixture))
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		var synth core.SymbolNode
		found := false
		for _, n := range nodes {
			if n.Name == "TestmyKit" {
				synth = n
				found = true
			}
		}
		if !found {
			var names []string
			for _, n := range nodes {
				names = append(names, n.Name)
			}
			t.Fatalf("expected synthesized symbol %q (got %v)", "TestmyKit", names)
		}
		wantCall := "#Kit"
		callFound := false
		for _, c := range synth.Calls {
			if c == wantCall {
				callFound = true
			}
		}
		if !callFound {
			t.Errorf("TestmyKit.Calls: want %q in %v", wantCall, synth.Calls)
		}
	})

	t.Run("non_testdata_path_synthesizes_nothing", func(t *testing.T) {
		nodes, err := x.Extract("svc", "kit/fixture.cue", []byte(cueTestdataFixture))
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		for _, n := range nodes {
			if n.Name == "TestmyKit" {
				t.Errorf("non-testdata path must not synthesize a Test-prefixed caller: %+v", n)
			}
		}
	})
}
