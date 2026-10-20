package projectrules_test

// FD-30 (docs/plans/fathom-dir.md): a call_direction layer rule in
// .fathom/guard.cue and the legacy .fathom/rules/archconformance.cue give
// the same verdicts, because Load hands both to the one archconformance
// engine.

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
)

// fauxtownEdges extracts the fauxtown ledger and payment services and adds
// one synthetic call each way, so the fixture has a ledger -> payment and a
// payment -> ledger edge (its real source has no cross-service call).
func fauxtownEdges(t *testing.T) []core.Edge {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(here), "..", "connector", "testdata", "fauxtown")
	ge := extractor.NewGoExtractor()
	var nodes []core.SymbolNode
	for _, rel := range []string{"ledger-service/ledger.go", "ledger-service/errors.go", "payment-service/payment.go", "payment-service/errors.go"} {
		src, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read fixture %s: %v", rel, err)
		}
		ns, err := ge.Extract("fauxtown", rel, src)
		if err != nil {
			t.Fatalf("extract %s: %v", rel, err)
		}
		nodes = append(nodes, ns...)
	}
	nodes = append(nodes,
		core.SymbolNode{Package: "ledger", Name: "SyntheticCaller", Kind: core.KindGoFunc, FilePath: "ledger-service/synthetic.go", Calls: []string{"payment.Charge"}},
		core.SymbolNode{Package: "payment", Name: "SyntheticCaller", Kind: core.KindGoFunc, FilePath: "payment-service/synthetic.go", Calls: []string{"ledger.Post"}},
	)
	return archconformance.EdgesFromNodes(nodes)
}

func writeGuard(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guard.cue"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCallDirectionMatchesArchconformanceOnFixture(t *testing.T) {
	edges := fauxtownEdges(t)

	legacy := t.TempDir()
	writeRuleFile(t, legacy, `rules: [
	{from_pkg: "ledger", to_pkg: "payment", allow: false},
	{from_pkg: "payment", to_pkg: "ledger", allow: false},
	{from_pkg: "payment", to_pkg: "ledger", allow: true},
]
`)
	modern := t.TempDir()
	writeGuard(t, modern, `layer_rules: [
	{id: "ledger-not-payment", kind: "call_direction", sentence: "ledger never calls payment.", from: ["led*"], to: ["payment"], allow: false},
	{id: "payment-not-ledger", kind: "call_direction", sentence: "payment never calls ledger.", from: ["payment"], to: ["ledger"], allow: false},
	{id: "payment-may-ledger", kind: "call_direction", sentence: "payment may call ledger.", from: ["payment"], to: ["ledger"], allow: true},
]
`)

	want := mustCheck(t, legacy, edges)
	got := mustCheck(t, modern, edges)
	if len(want) == 0 {
		t.Fatal("fixture produced no violation through archconformance.cue; the comparison proves nothing")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("verdicts differ\n legacy: %+v\n layer_rules: %+v", want, got)
	}
}

func mustCheck(t *testing.T, root string, edges []core.Edge) []archconformance.Violation {
	t.Helper()
	res, err := projectrules.Load(root)
	if err != nil {
		t.Fatalf("Load(%s): %v", root, err)
	}
	if !res.Found || res.Source != projectrules.SourceProject {
		t.Fatalf("Load(%s): Found=%v Source=%q, want a loaded project pack", root, res.Found, res.Source)
	}
	var opts []archconformance.Option
	if res.DenyByDefault {
		opts = append(opts, archconformance.DenyByDefault())
	}
	return archconformance.Check(edges, res.Rules, opts...)
}

// Both files at once: the legacy pack and the layer rules both apply, and
// the legacy file's deny_by_default still governs.
func TestLoadMergesLayerRulesWithArchconformanceFile(t *testing.T) {
	root := t.TempDir()
	writeRuleFile(t, root, `rules: [{from_pkg: "a", to_pkg: "b", allow: false}]`)
	writeGuard(t, root, `layer_rules: [{id: "c-not-d", kind: "call_direction", sentence: "s", from: ["c"], to: ["d"]}]`)
	res, err := projectrules.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rules) != 2 || res.Rules[0].FromPkg != "a" || res.Rules[1].FromGlobs[0] != "c" {
		t.Fatalf("rules = %+v, want the legacy rule then the layer rule", res.Rules)
	}
}

func TestEquivalentLayerRulesGroupsByTarget(t *testing.T) {
	got := projectrules.EquivalentLayerRules([]archconformance.Rule{
		{FromPkg: "a", ToPkg: "x"}, {FromPkg: "b", ToPkg: "x"}, {FromPkg: "c", ToPkg: "y", Allow: true},
	})
	want := []string{
		`{kind: "call_direction", from: ["a", "b"], to: ["x"], allow: false}`,
		`{kind: "call_direction", from: ["c"], to: ["y"], allow: true}`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
