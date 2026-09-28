package certify

// portfolio.go — TQ-6 and TQ-7 (docs/plans/test-intelligence.md; design
// doc docs/design/test-intelligence.md §3 T2): the test portfolio. Every
// test is classified by what it certifies and what it statically reaches:
//
//   - certifying: carries at least one live certificate;
//   - reaching_governed: no live certificate, reaches a symbol in a file a
//     governing rule scopes (TQ-3 severity above "none");
//   - orphan: no live certificate, reaches nothing governed and nothing
//     central (centrality below the service's 20th percentile);
//   - reaching: everything else (reaches central code, certifies nothing).
//
// Redundancy is a flag beside the class: the tests with exactly the same
// reach set and the same certificate targets (exact set equality; a
// near-duplicate is never claimed). The critical targets without a
// certificate — the top decile of T1 risk with no live certificate on
// their file — are listed beside the tests as gaps.
//
// Reach is static (the honesty boundary of test_reaches): a test that
// reaches code has not been shown to pass over it. Reach sets come from
// one forward walk over the call graph's SCC condensation
// (callgraph.ForwardReach), never a BFS per test.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestClass is a test's portfolio class.
type TestClass string

const (
	ClassCertifying       TestClass = "certifying"
	ClassReachingGoverned TestClass = "reaching_governed"
	ClassOrphan           TestClass = "orphan"
	ClassReaching         TestClass = "reaching"
)

// PortfolioCert is one certificate a test carries.
type PortfolioCert struct {
	Target   string `json:"target"` // "<kind>:<id>"
	Strength string `json:"strength"`
	State    string `json:"state"`
}

// PortfolioTest is one classified test.
type PortfolioTest struct {
	Test      string          `json:"test"` // canonical key
	Name      string          `json:"name"`
	File      string          `json:"file"`
	Line      int             `json:"line,omitempty"`
	Class     TestClass       `json:"class"`
	Certifies []PortfolioCert `json:"certifies,omitempty"`
	// Reaches counts the symbols the test statically reaches;
	// ReachesGoverned those in a governed file.
	Reaches         int `json:"reaches"`
	ReachesGoverned int `json:"reaches_governed"`
	// TopReach is the most central symbol the test reaches ("" when it
	// reaches nothing).
	TopReach string `json:"top_reach,omitempty"`
	// RedundantWith names the other tests with the same reach set and the
	// same certificate targets, sorted.
	RedundantWith []string `json:"redundant_with,omitempty"`
	// Purpose is the test's purpose line (TQ-7, testPurpose).
	Purpose string `json:"purpose"`
}

// CriticalGap is a top-decile T1 risk symbol with no live certificate.
type CriticalGap struct {
	Symbol string  `json:"symbol"`
	File   string  `json:"file"`
	Risk   float64 `json:"risk"`
}

// Portfolio is ClassifyTests' answer.
type Portfolio struct {
	Tests     []PortfolioTest   `json:"tests"`
	Counts    map[TestClass]int `json:"counts"`
	Redundant int               `json:"redundant"`
	Gaps      []CriticalGap     `json:"gaps"`
	// CentralityP20 is the service's 20th-percentile centrality, the
	// orphan threshold.
	CentralityP20 float64  `json:"centrality_p20"`
	Watermark     string   `json:"watermark,omitempty"`
	Unread        []string `json:"unread,omitempty"`
}

// isTestFunc is the test-entry convention callgraph.TestImpact uses (a
// Test* name), narrowed to test files so a production TestHelper type is
// not a test.
func isTestFunc(n core.SymbolNode) bool {
	return strings.HasPrefix(n.Name, "Test") && isTestFile(n.FilePath)
}

// ClassifyTests builds service's test portfolio. corpusRoot is the
// service's indexed path ("" degrades governance to claims alone).
func ClassifyTests(ctx context.Context, st store.Store, service, corpusRoot string) (Portfolio, error) {
	nodes, err := serviceSymbols(ctx, st, service)
	if err != nil {
		return Portfolio{}, fmt.Errorf("ClassifyTests: symbols: %w", err)
	}
	typed, err := st.TypedEdges(ctx, service)
	if err != nil {
		return Portfolio{}, fmt.Errorf("ClassifyTests: typed edges: %w", err)
	}
	watermark, cent, err := st.SymbolCentrality(ctx, service)
	if err != nil {
		return Portfolio{}, fmt.Errorf("ClassifyTests: centrality: %w", err)
	}
	claims, err := st.ClaimsForService(ctx, service, "")
	if err != nil {
		return Portfolio{}, fmt.Errorf("ClassifyTests: claims: %w", err)
	}
	risk, err := RiskWeightedCoverage(ctx, st, service, corpusRoot, RiskOptions{Limit: MaxRiskLimit})
	if err != nil {
		return Portfolio{}, fmt.Errorf("ClassifyTests: risk: %w", err)
	}

	// Tests, deduplicated by key, sorted.
	testNode := map[string]core.SymbolNode{}
	fileOf := map[string]string{}
	for _, n := range nodes {
		k := n.Key()
		if _, ok := fileOf[k]; !ok {
			fileOf[k] = n.FilePath
		}
		if isTestFunc(n) {
			if _, ok := testNode[k]; !ok {
				testNode[k] = n
			}
		}
	}
	testKeys := make([]string, 0, len(testNode))
	for k := range testNode {
		testKeys = append(testKeys, k)
	}
	sort.Strings(testKeys)

	// Centrality and its 20th percentile over non-test symbols.
	centOf := make(map[string]float64, len(cent))
	var prod []float64
	for _, r := range cent {
		centOf[r.Symbol] = r.Centrality
		if !isTestSymbol(r.Symbol, r.File) {
			prod = append(prod, r.Centrality)
		}
	}
	p20 := percentile(prod, 0.2)

	// Governed files: severity above "none" (TQ-3).
	var files []string
	for _, f := range fileOf {
		files = append(files, f)
	}
	sort.Strings(files)
	gov := riskInputs(files, loadGuards(corpusRoot), claims, nil)

	// Certificates per test key; live certificate scope per file.
	certs := map[string][]PortfolioCert{}
	liveFiles := map[string]bool{}
	for _, c := range claims {
		if c.Layer != Layer || len(c.Meta) == 0 {
			continue
		}
		var m Meta
		if json.Unmarshal(c.Meta, &m) != nil {
			continue
		}
		k := m.TestSymbolKey
		if k == "" {
			k = m.Test
		}
		certs[k] = append(certs[k], PortfolioCert{Target: c.Subject, Strength: m.Strength, State: m.State})
		if State(m.State) == StateLive {
			for _, f := range c.Scope {
				liveFiles[f] = true
			}
		}
	}

	reach := callgraph.ForwardReach(callgraph.BuildBoth(nodes, typed), testKeys)
	out := Portfolio{Counts: map[TestClass]int{}, CentralityP20: p20, Watermark: watermark, Unread: risk.Unread}
	groups := map[string][]string{} // reach fingerprint + cert targets -> test keys
	for _, k := range testKeys {
		n := testNode[k]
		pt := PortfolioTest{Test: k, Name: n.Name, File: n.FilePath, Line: n.Line}
		pt.Certifies = certs[k]
		if len(pt.Certifies) == 0 {
			pt.Certifies = certs[n.Name] // a certificate that names only the bare test
		}
		sort.Slice(pt.Certifies, func(i, j int) bool { return pt.Certifies[i].Target < pt.Certifies[j].Target })
		live, central := false, false
		for _, c := range pt.Certifies {
			if State(c.State) == StateLive {
				live = true
			}
		}
		best := -1.0
		reach.Each(k, func(sym string) {
			pt.Reaches++
			if gov[fileOf[sym]].Severity > severityNone {
				pt.ReachesGoverned++
			}
			c := centOf[sym]
			if c >= p20 {
				central = true
			}
			if c > best || (c == best && sym < pt.TopReach) {
				best, pt.TopReach = c, sym
			}
		})
		switch {
		case live:
			pt.Class = ClassCertifying
		case pt.ReachesGoverned > 0:
			pt.Class = ClassReachingGoverned
		case !central:
			pt.Class = ClassOrphan
		default:
			pt.Class = ClassReaching
		}
		targets := make([]string, 0, len(pt.Certifies))
		for _, c := range pt.Certifies {
			targets = append(targets, c.Target)
		}
		sig := reach.Fingerprint(k) + "\x00" + strings.Join(targets, "\x00")
		groups[sig] = append(groups[sig], k)
		out.Tests = append(out.Tests, pt)
	}
	for i := range out.Tests {
		pt := &out.Tests[i]
		targets := make([]string, 0, len(pt.Certifies))
		for _, c := range pt.Certifies {
			targets = append(targets, c.Target)
		}
		sig := reach.Fingerprint(pt.Test) + "\x00" + strings.Join(targets, "\x00")
		for _, other := range groups[sig] {
			if other != pt.Test {
				pt.RedundantWith = append(pt.RedundantWith, other)
			}
		}
		if len(pt.RedundantWith) > 0 {
			out.Redundant++
		}
		pt.Purpose = testPurpose(*pt)
		out.Counts[pt.Class]++
	}

	// Critical targets without a certificate: the top decile of T1 risk.
	decile := int(math.Ceil(float64(risk.Considered) / 10))
	for i, r := range risk.Rows {
		if i >= decile {
			break
		}
		if r.Risk <= 0 || liveFiles[r.File] {
			continue
		}
		out.Gaps = append(out.Gaps, CriticalGap{Symbol: r.Symbol, File: r.File, Risk: r.Risk})
	}
	return out, nil
}

// testPurpose is a test's purpose line (TQ-7, design doc §3 T2): what it
// certifies, at which strength and state, or what it reaches and that it
// certifies nothing.
func testPurpose(t PortfolioTest) string {
	if len(t.Certifies) > 0 {
		parts := make([]string, 0, len(t.Certifies))
		for _, c := range t.Certifies {
			parts = append(parts, fmt.Sprintf("%s (%s, %s)", c.Target, c.Strength, c.State))
		}
		return "certifies " + strings.Join(parts, "; ")
	}
	switch {
	case t.Reaches == 0 || t.TopReach == "":
		return "reaches nothing indexed, certifies nothing"
	case t.Reaches == 1:
		return fmt.Sprintf("reaches %s, certifies nothing", t.TopReach)
	}
	return fmt.Sprintf("reaches %s and %d more, certifies nothing", t.TopReach, t.Reaches-1)
}

// percentile returns the q-quantile of vals (nearest rank, lower); 0 for
// none.
func percentile(vals []float64, q float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	return s[int(q*float64(len(s)-1))]
}
