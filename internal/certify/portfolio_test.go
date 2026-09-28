package certify

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// portfolioFixture: Hub calls Leaf; A and B call Hub; Governed sits in a
// file a fresh obligation governs. Tests: TestCert (reaches A, one live
// certificate), TestGov (reaches Governed), TestDupA/TestDupB (both reach
// exactly Leaf), TestOrphan (reaches nothing).
func portfolioFixture(t *testing.T) (store.Store, func(file, name string) string) {
	t.Helper()
	ctx := context.Background()
	const svc = "svc"
	st := store.NewMemoryStore()
	key := func(file, name string) string { return core.SymbolKey(svc, core.QualifiedPackage(file, "pkg"), name) }
	ob := func(file, name string, calls ...string) core.Observation {
		return core.Observation{ServiceID: svc, SourceRef: file, SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified,
			Symbol: name, Package: "pkg", Claim: "func " + name + "()", Calls: calls, IngestedAt: time.Now()}
	}
	obs := []core.Observation{
		ob("pkg/leaf.go", "Leaf"),
		ob("pkg/core.go", "Hub", key("pkg/leaf.go", "Leaf")),
		ob("pkg/core.go", "A", key("pkg/core.go", "Hub")),
		ob("pkg/core.go", "B", key("pkg/core.go", "Hub")),
		ob("pkg/gov.go", "Governed"),
		ob("pkg/x_test.go", "TestCert", key("pkg/core.go", "A")),
		ob("pkg/x_test.go", "TestGov", key("pkg/gov.go", "Governed")),
		ob("pkg/x_test.go", "TestDupA", key("pkg/leaf.go", "Leaf")),
		ob("pkg/x_test.go", "TestDupB", key("pkg/leaf.go", "Leaf")),
		ob("pkg/x_test.go", "TestOrphan"),
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatal(err)
	}
	// Centrality as the ingest stores it (TQ-2).
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	counts := callgraph.TransitiveCallerCounts(callgraph.Build(nodes))
	max := 0
	for _, c := range counts {
		if c > max {
			max = c
		}
	}
	var rows []store.SymbolCentrality
	for _, n := range nodes {
		c := counts[n.Key()]
		rows = append(rows, store.SymbolCentrality{Symbol: n.Key(), File: n.FilePath, Callers: c, Centrality: float64(c) / float64(max)})
	}
	if err := st.SetSymbolCentrality(ctx, svc, "wm", rows); err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(Meta{Test: "TestCert", Strength: "asserts", State: string(StateLive),
		TestSymbolKey: key("pkg/x_test.go", "TestCert"), TargetKind: "invariant", TargetID: "I1", TestFile: "pkg/x_test.go"})
	if err := st.PutClaims(ctx, []core.SemanticClaim{
		{ClaimID: "cert1", ServiceID: svc, Layer: Layer, Kind: Kind, Subject: "invariant:I1", Rule: "r",
			Scope: []string{"pkg/core.go"}, Status: core.ClaimFresh, Meta: meta},
		{ClaimID: "obl1", ServiceID: svc, Layer: "obligations", Kind: "obligation", Subject: "CLAUDE.md#gov", Rule: "r",
			Scope: []string{"pkg/gov.go"}, Status: core.ClaimFresh},
	}, nil); err != nil {
		t.Fatal(err)
	}
	return st, key
}

// TestClassifyTestsPortfolio proves TQ-6 (docs/plans/test-intelligence.md;
// design doc §3 T2): every test is classified certifying (a live
// certificate), reaching governed code (no certificate), orphan (reaches
// nothing governed and nothing at or above the 20th centrality
// percentile) or reaching; redundant tests are named by exact equality of
// reach set and certificate targets; and the top-decile T1 risk symbols
// without a live certificate are listed beside the tests.
func TestClassifyTestsPortfolio(t *testing.T) {
	st, key := portfolioFixture(t)
	p, err := ClassifyTests(context.Background(), st, "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]TestClass{}
	byName := map[string]PortfolioTest{}
	for _, pt := range p.Tests {
		got[pt.Name] = pt.Class
		byName[pt.Name] = pt
	}
	want := map[string]TestClass{
		"TestCert":   ClassCertifying,
		"TestGov":    ClassReachingGoverned,
		"TestDupA":   ClassReaching,
		"TestDupB":   ClassReaching,
		"TestOrphan": ClassOrphan,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("classes = %v, want %v", got, want)
	}
	if r := byName["TestDupA"].RedundantWith; !reflect.DeepEqual(r, []string{key("pkg/x_test.go", "TestDupB")}) {
		t.Errorf("TestDupA redundant with %v, want [TestDupB]", r)
	}
	if r := byName["TestCert"].RedundantWith; len(r) != 0 {
		t.Errorf("TestCert redundant with %v, want none", r)
	}
	if n := byName["TestCert"].Reaches; n != 3 {
		t.Errorf("TestCert reaches %d, want 3 (A, Hub, Leaf)", n)
	}
	if p.Counts[ClassCertifying] != 1 || p.Counts[ClassOrphan] != 1 || p.Counts[ClassReaching] != 2 || p.Redundant != 2 {
		t.Errorf("counts = %v redundant = %d", p.Counts, p.Redundant)
	}
	// Leaf carries the most callers and no live certificate covers its
	// file: the one top-decile gap.
	if len(p.Gaps) != 1 || p.Gaps[0].Symbol != key("pkg/leaf.go", "Leaf") {
		t.Errorf("gaps = %+v, want [Leaf]", p.Gaps)
	}
	// Deterministic order: by test key.
	for i := 1; i < len(p.Tests); i++ {
		if p.Tests[i-1].Test >= p.Tests[i].Test {
			t.Fatalf("tests not sorted: %s before %s", p.Tests[i-1].Test, p.Tests[i].Test)
		}
	}
}

// TestTestPurposeLine proves TQ-7: each test's purpose line says what it
// certifies at which strength and state, or what it reaches and that it
// certifies nothing.
func TestTestPurposeLine(t *testing.T) {
	st, _ := portfolioFixture(t)
	p, err := ClassifyTests(context.Background(), st, "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	purpose := map[string]string{}
	for _, pt := range p.Tests {
		purpose[pt.Name] = pt.Purpose
		if pt.Purpose != testPurpose(pt) {
			t.Errorf("%s: Purpose %q is not testPurpose's %q", pt.Name, pt.Purpose, testPurpose(pt))
		}
	}
	if got := purpose["TestCert"]; got != "certifies invariant:I1 (asserts, live)" {
		t.Errorf("TestCert purpose = %q", got)
	}
	if got := purpose["TestOrphan"]; got != "reaches nothing indexed, certifies nothing" {
		t.Errorf("TestOrphan purpose = %q", got)
	}
	if got := purpose["TestGov"]; !strings.HasPrefix(got, "reaches ") || !strings.Contains(got, "Governed") || !strings.HasSuffix(got, ", certifies nothing") {
		t.Errorf("TestGov purpose = %q", got)
	}
	multi := testPurpose(PortfolioTest{Reaches: 4, TopReach: "svc::pkg.Hub"})
	if multi != "reaches svc::pkg.Hub and 3 more, certifies nothing" {
		t.Errorf("multi-reach purpose = %q", multi)
	}
}
