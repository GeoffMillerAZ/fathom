package guardspec

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"golang.org/x/tools/go/packages"
)

// fauxtownNodes extracts the REAL ledger-service and payment-service Go
// files from the fauxtown fixture (internal/connector/testdata/fauxtown —
// CLAUDE.md's "prefer golden tests against it") through the actual Go
// extractor, so the golden tests below run against genuinely fathom-
// indexed data, not a hand-typed stand-in. Neither service imports the
// other in the fixture's real source — every "satisfied" case below is a
// real, unmodified reading; every "violated" case adds one or two
// hand-built synthetic core.SymbolNode entries (documented at each call
// site) because the checked-in fixture has no real cross-package call to
// exercise the violated path — fauxtown's services must stay untouched
// (other packages' golden counts depend on their exact file contents).
func fauxtownNodes(t *testing.T) []core.SymbolNode {
	t.Helper()
	root := repoRoot(t)
	fx := filepath.Join(root, "internal", "connector", "testdata", "fauxtown")
	files := []string{
		"ledger-service/ledger.go",
		"ledger-service/errors.go",
		"payment-service/payment.go",
		"payment-service/errors.go",
	}
	ge := extractor.NewGoExtractor()
	var nodes []core.SymbolNode
	for _, rel := range files {
		src, err := os.ReadFile(filepath.Join(fx, rel))
		if err != nil {
			t.Fatalf("read fixture %s: %v", rel, err)
		}
		ns, err := ge.Extract("fauxtown", rel, src)
		if err != nil {
			t.Fatalf("extract %s: %v", rel, err)
		}
		nodes = append(nodes, ns...)
	}
	return nodes
}

// syntheticCall fabricates one func-node call edge from fromPkg's file
// (in fromDir) to a callee in toPkg — the synthetic augmentation golden
// "violated" cases layer on top of the real fauxtown extraction (see
// fauxtownNodes' doc comment).
func syntheticCall(fromPkg, fromDir, callee string) core.SymbolNode {
	return core.SymbolNode{
		Package:  fromPkg,
		Name:     "SyntheticCaller",
		Kind:     core.KindGoFunc,
		FilePath: fromDir + "/synthetic.go",
		Calls:    []string{callee},
	}
}

func TestEvaluateLayerRule_ImportDirection(t *testing.T) {
	base := fauxtownNodes(t)
	rule := LayerRule{
		ID: "ledger-not-payment", Kind: ImportDirection, Sentence: "s", Severity: SeverityError,
		From: []string{"ledger-service"}, To: []string{"payment-service"},
	}

	// Satisfied: real fauxtown data — ledger never calls payment.
	idx := BuildIndexFromSymbols(base, nil)
	if got := EvaluateLayerRule(rule, idx); len(got) != 0 {
		t.Fatalf("satisfied case: got %d violations, want 0: %+v", len(got), got)
	}

	// Violated: synthetic ledger -> payment call.
	withViolation := append(append([]core.SymbolNode{}, base...),
		syntheticCall("ledger", "ledger-service", "payment.Charge"))
	idx = BuildIndexFromSymbols(withViolation, nil)
	got := EvaluateLayerRule(rule, idx)
	if len(got) != 1 {
		t.Fatalf("violated case: got %d violations, want 1: %+v", len(got), got)
	}
	if got[0].From != "ledger-service" || got[0].To != "payment-service" {
		t.Errorf("violation = %+v", got[0])
	}
}

func TestEvaluateLayerRule_NoCycles(t *testing.T) {
	base := fauxtownNodes(t)
	rule := LayerRule{
		ID: "no-service-cycles", Kind: NoCycles, Sentence: "s", Severity: SeverityError,
		Scope: []string{"*-service"},
	}

	// Satisfied: real fauxtown data has no calls between the two services.
	idx := BuildIndexFromSymbols(base, nil)
	if got := EvaluateLayerRule(rule, idx); len(got) != 0 {
		t.Fatalf("satisfied case: got %d violations, want 0: %+v", len(got), got)
	}

	// Violated: synthetic ledger -> payment AND payment -> ledger calls
	// form a 2-cycle within scope.
	withCycle := append(
		append([]core.SymbolNode{}, base...),
		syntheticCall("ledger", "ledger-service", "payment.Charge"),
		syntheticCall("payment", "payment-service", "ledger.Post"),
	)
	idx = BuildIndexFromSymbols(withCycle, nil)
	got := EvaluateLayerRule(rule, idx)
	if len(got) != 1 {
		t.Fatalf("violated case: got %d violations, want 1: %+v", len(got), got)
	}
	if got[0].Kind != NoCycles {
		t.Errorf("violation kind = %q", got[0].Kind)
	}
}

func TestEvaluateLayerRule_Boundary(t *testing.T) {
	base := fauxtownNodes(t)
	rule := LayerRule{
		ID: "only-payment-imports-ledger", Kind: Boundary, Sentence: "s", Severity: SeverityError,
		Allow: []string{"payment-service"}, Guarded: []string{"ledger-service"},
	}

	// Satisfied: real fauxtown data — nobody imports ledger-service at all.
	idx := BuildIndexFromSymbols(base, nil)
	if got := EvaluateLayerRule(rule, idx); len(got) != 0 {
		t.Fatalf("satisfied case: got %d violations, want 0: %+v", len(got), got)
	}

	// Violated: synthetic worktree-service -> ledger-service call (worktree
	// is not on the allow list).
	withViolation := append(append([]core.SymbolNode{}, base...),
		syntheticCall("worktree", "worktree-service", "ledger.Post"))
	idx = BuildIndexFromSymbols(withViolation, nil)
	got := EvaluateLayerRule(rule, idx)
	if len(got) != 1 {
		t.Fatalf("violated case: got %d violations, want 1: %+v", len(got), got)
	}
	if got[0].From != "worktree-service" || got[0].To != "ledger-service" {
		t.Errorf("violation = %+v", got[0])
	}
}

func TestEvaluateLayerRule_CrossServiceDirection(t *testing.T) {
	rule := LayerRule{
		ID: "ledger-not-payment-fleet", Kind: CrossServiceDirection, Sentence: "s", Severity: SeverityError,
		FromService: "ledger-service", ToService: "payment-service",
	}

	// Satisfied: the fleet's typed edges name only the allowed direction.
	idx := LayerRuleIndex{CrossService: []CrossServiceEdge{
		{FromService: "payment-service", ToService: "ledger-service"},
	}}
	if got := EvaluateLayerRule(rule, idx); len(got) != 0 {
		t.Fatalf("satisfied case: got %d violations, want 0: %+v", len(got), got)
	}

	// Violated: the forbidden direction appears in the fleet's typed edges.
	idx = LayerRuleIndex{CrossService: []CrossServiceEdge{
		{FromService: "ledger-service", ToService: "payment-service"},
	}}
	got := EvaluateLayerRule(rule, idx)
	if len(got) != 1 {
		t.Fatalf("violated case: got %d violations, want 1: %+v", len(got), got)
	}
	if got[0].From != "ledger-service" || got[0].To != "payment-service" {
		t.Errorf("violation = %+v", got[0])
	}
}

// TestBuildIndexFromSymbols_AgreesWithGoTruth_OnFauxtown is the honest gap
// measurement the package doc comment promises: BuildIndexFromSymbols is
// CALL-derived (a package dependency visible only through a type
// reference, embed, or unused import is invisible to it — never a false
// violation, but a possible false negative). This copies fauxtown's real
// ledger/payment sources into a throwaway module and asks go/packages —
// the TRUE import graph, ground truth, no fathom index involved — whether
// either package imports the other. It does not (matching the call-derived
// "satisfied" result above), so on THIS fixture the gap costs nothing;
// internal/guardspec/layerrules.BuildIndex is the go/packages-backed
// producer that closes the gap generally, used by the generated test
// (`fathom rules gen-test`, VB-14) rather than live obligations serving.
func TestBuildIndexFromSymbols_AgreesWithGoTruth_OnFauxtown(t *testing.T) {
	root := repoRoot(t)
	fx := filepath.Join(root, "internal", "connector", "testdata", "fauxtown")
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module fauxtowngap\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	copyPkg := func(svcDir, pkgDir string, files ...string) {
		if err := os.MkdirAll(filepath.Join(tmp, pkgDir), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			data, err := os.ReadFile(filepath.Join(fx, svcDir, f))
			if err != nil {
				t.Fatalf("read %s/%s: %v", svcDir, f, err)
			}
			if err := os.WriteFile(filepath.Join(tmp, pkgDir, f), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	copyPkg("ledger-service", "ledger", "ledger.go", "errors.go")
	copyPkg("payment-service", "payment", "payment.go", "errors.go")

	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedImports,
		Dir:   tmp,
		Env:   append(os.Environ(), "GOWORK=off"),
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("packages.Load: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("loaded %d packages, want 2: %v", len(pkgs), pkgs)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].PkgPath < pkgs[j].PkgPath })
	for _, p := range pkgs {
		for imp := range p.Imports {
			if imp == "fauxtowngap/ledger" || imp == "fauxtowngap/payment" {
				t.Errorf("go/packages ground truth: %s unexpectedly imports %s — the call-derived satisfied case above is WRONG for this fixture", p.PkgPath, imp)
			}
		}
	}
}
