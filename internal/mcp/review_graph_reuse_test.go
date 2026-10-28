package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	riskreview "github.com/geoffmilleraz/fathom/internal/review"
)

// review_graph_reuse_test.go pins NP-W3: review_diff's test_impact,
// resolution, layers and receipt sections read the impact dimension's own
// graph and symbol snapshot (review.Input.ImpactGraph, Input.Nodes) — one
// graph build per cold review — and a review whose graph could not be
// loaded names the failure once and never retries it in the same call.

var reviewGraphSections = []string{"test_impact", "resolution", "layers", "receipt", "receipt_sarif", "receipt_check"}

func reviewDiffCall(t *testing.T, s *Server, service string, changed ...string) map[string]any {
	t.Helper()
	files := make([]any, len(changed))
	for i, f := range changed {
		files[i] = f
	}
	return callTool(t, s, "review_diff", map[string]any{"service": service, "changed_files": files})
}

// TestReviewDiffColdBuildsOneGraph: one cold review starts exactly one
// graph build (Both) on a store-identified backend, and loads the symbols
// once on an uncached one — not the Syntax and Resolved graphs and the two
// further snapshot loads the section loaders used to add.
func TestReviewDiffColdBuildsOneGraph(t *testing.T) {
	t.Run("store-identified", func(t *testing.T) {
		b := &reviewMemoBackend{}
		b.nodes = []core.SymbolNode{
			{Service: "svc", Package: "pkg", Name: "Target", FilePath: "pkg/target.go", Line: 1},
			{Service: "svc", Package: "pkg", Name: "Caller", FilePath: "pkg/caller.go", Line: 1},
		}
		b.posts = []core.Edge{{FromSymbol: b.nodes[1].Key(), ToSymbol: b.nodes[0].Key(), Kind: core.EdgeDirect}}
		before := reviewGraphMisses()
		reviewDiffCall(t, NewServer(b), "svc", "pkg/target.go")
		if got := reviewGraphMisses() - before; got != 1 {
			t.Errorf("cold review graph builds=%d, want 1", got)
		}
		if b.loads != 1 {
			t.Errorf("cold review symbol loads=%d, want 1", b.loads)
		}
	})
	t.Run("uncached", func(t *testing.T) {
		b := newReviewFallbackBackend()
		reviewDiffCall(t, NewServer(b), "svc", "pkg/target.go")
		if b.loads != 1 {
			t.Errorf("uncached review symbol loads=%d, want 1", b.loads)
		}
	})
}

// reviewTypedErrBackend is the mixed resolution fixture whose typed-edge
// read fails with a plain error: the impact dimension degrades to the
// syntax graph (memo.GraphForTyped) and so does every section.
type reviewTypedErrBackend struct{ *multiSvcBackend }

func (reviewTypedErrBackend) TypedEdges(context.Context, string) ([]core.Edge, error) {
	return nil, errors.New("typed unavailable")
}

func resolutionFixtureBackend() *multiSvcBackend {
	a, b, c, d, testFoo := resolutionFixtureNodes()
	return &multiSvcBackend{svcs: map[string]*svcFixture{
		"svc": {
			nodes:       []core.SymbolNode{a, b, c, d, testFoo},
			typedEdges:  []core.Edge{{FromService: "svc", FromSymbol: a.Key(), ToSymbol: b.Key(), Kind: core.EdgeDirect}},
			indexCommit: "fixturecommit123",
		},
	}}
}

// composedReviewSections is the graph sections exactly as review_diff
// composed them before NP-W3: over loadNavGraphsErr's Both graph and
// snapshot nodes, through the same helpers the handler calls.
func composedReviewSections(t *testing.T, b QueryInterface, service string, changed []string) map[string]any {
	t.Helper()
	ctx := context.Background()
	in, _, rpcErr := buildReviewInput(ctx, b, service, changed, "", "", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	v := review.Run(in)
	ng, err := loadNavGraphsErr(ctx, b, service)
	if err != nil {
		t.Fatal(err)
	}
	hops, _ := serviceRefHopsInto(ctx, b, service, changed)
	risk := reviewDiffReceipt(ng.Both, ng.Nodes, in, v, hops, riskreview.RiskTableFor(in.Root))
	run := receipt.Run{Dir: in.Root, Rev: indexCommitFor(ctx, b, service), Service: service, Nodes: ng.Nodes,
		RepositoryURIAt: receipt.ConfiguredRepositoryURI}
	sarif, check := review.ChangeRiskReceipt(ctx, "fathom review_diff", run, risk)
	return map[string]any{
		"test_impact":   impact.TestGapOn(ng.Both, ng.Nodes, changed).Payload(),
		"resolution":    resolutionFromBlastBuckets(blastRadiusByDepth(ng.Both, ng.Nodes, changed)),
		"layers":        layerDiffByBlastRadius(ng.Both, ng.Nodes, changed, hops),
		"receipt":       riskReceiptPayload(risk),
		"receipt_sarif": json.RawMessage(sarif),
		"receipt_check": json.RawMessage(check),
	}
}

// reviewSectionJSON is one section as wire JSON with citation_status
// removed: citation binding reads the source tree, not the graph, and is
// pinned by the citation tests.
func reviewSectionJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	var strip func(any)
	strip = func(x any) {
		switch x := x.(type) {
		case map[string]any:
			delete(x, "citation_status")
			for _, e := range x {
				strip(e)
			}
		case []any:
			for _, e := range x {
				strip(e)
			}
		}
	}
	strip(generic)
	out, _ := json.Marshal(generic)
	return string(out)
}

// TestReviewDiffGraphSectionsMatchNavComposition pins the success output:
// the handler's graph sections equal the ones composed over the navigation
// graphs, on a mixed typed/heuristic fixture and on a typed-read failure.
func TestReviewDiffGraphSectionsMatchNavComposition(t *testing.T) {
	for name, backend := range map[string]func() QueryInterface{
		"mixed-tiers":       func() QueryInterface { return resolutionFixtureBackend() },
		"typed-read-failed": func() QueryInterface { return reviewTypedErrBackend{resolutionFixtureBackend()} },
	} {
		t.Run(name, func(t *testing.T) {
			changed := []string{"pkg/c.go"}
			got := reviewDiffCall(t, NewServer(backend()), "svc", changed...)
			want := composedReviewSections(t, backend(), "svc", changed)
			for _, section := range reviewGraphSections {
				if got[section] == nil {
					t.Fatalf("%s missing: %v", section, got)
				}
				if g, w := reviewSectionJSON(t, got[section]), reviewSectionJSON(t, want[section]); g != w {
					t.Errorf("%s differs from the nav-graph composition:\n got %s\nwant %s", section, g, w)
				}
			}
		})
	}
}

// reviewPatchBackend is the mixed fixture behind a store identity, with the
// review_diff claim store: its findings-persistence read patches the
// service's package in the memo (an incremental refresh landing mid-review,
// same scope), adding a caller of the changed file's symbol.
type reviewPatchBackend struct {
	*multiSvcBackend
	testStore
	patch func()
}

func (*reviewPatchBackend) StoreWatermark(context.Context, string) (string, error) { return "w1", nil }
func (b *reviewPatchBackend) ClaimsForService(_ context.Context, _, layer string) ([]core.SemanticClaim, error) {
	if layer == review.ClaimsLayer && b.patch != nil {
		b.patch()
		b.patch = nil
	}
	return nil, nil
}
func (*reviewPatchBackend) PutClaims(context.Context, []core.SemanticClaim, []core.ClaimLink) error {
	return nil
}
func (*reviewPatchBackend) SetClaimStatus(context.Context, string, string, core.ClaimStatus) error {
	return nil
}
func (*reviewPatchBackend) SourceFingerprint(context.Context, string, string) (string, bool, error) {
	return "", false, nil
}
func (*reviewPatchBackend) SetClaimValidTo(context.Context, string, string, time.Time) error {
	return nil
}
func (*reviewPatchBackend) WriteSupersedes(context.Context, string, string, string, string) error {
	return nil
}

// TestReviewDiffGraphSectionsAgreeWithImpactAcrossPatch: a same-scope
// package patch landing after the verdict does not make resolution, layers,
// test_impact or the receipt disagree with the impact dimension (VB-19:
// resolution sums the impact dimension's own by_depth counts).
func TestReviewDiffGraphSectionsAgreeWithImpactAcrossPatch(t *testing.T) {
	b := &reviewPatchBackend{multiSvcBackend: resolutionFixtureBackend()}
	fx := b.svcs["svc"]
	added := core.SymbolNode{Service: "svc", Package: "pkg", Name: "E", Kind: core.KindGoFunc, FilePath: "pkg/e.go", Line: 1, Calls: []string{"C"}}
	b.patch = func() {
		patched := append(append([]core.SymbolNode(nil), fx.nodes...), added)
		memo.PatchPackage(navScope(context.Background(), b, "svc"), "pkg", patched, fx.typedEdges)
	}
	p := reviewDiffCall(t, NewServer(b), "svc", "pkg/c.go")
	if b.patch != nil {
		t.Fatal("findings persistence never read the claims: the patch did not land")
	}
	impactCount := -1
	for _, raw := range p["dimensions"].([]any) {
		d := raw.(map[string]any)
		if d["name"] == review.DimImpact {
			impactCount = int(d["count"].(float64))
		}
	}
	typed, heuristic := resolutionOf(t, p)
	if impactCount != 4 || typed+heuristic != impactCount {
		t.Errorf("resolution {typed:%d heuristic:%d} disagrees with the impact dimension's %d finding(s)", typed, heuristic, impactCount)
	}
	for _, section := range reviewGraphSections {
		if s := reviewSectionJSON(t, p[section]); strings.Contains(s, "pkg/e.go") || strings.Contains(s, added.Key()) {
			t.Errorf("%s reads the patched graph the verdict never saw: %s", section, s)
		}
	}
}

// TestReviewDiffGraphFailureNamedOnceWithoutRetry (REV-3): when the impact
// graph cannot be loaded, the review keeps its verdict on the direct
// typed-edge fallback, names the failure in partial and in test_impact, and
// makes no second snapshot or graph attempt in the same call (ADR-047's
// 2026-10-19 amendment: a retry meets the same or a shorter deadline).
func TestReviewDiffGraphFailureNamedOnceWithoutRetry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		failAt int
		err    error
	}{
		{"typed-admission", 1, memo.ErrWouldNotFinish},
		{"graph-build", 2, errors.New("graph typed load failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newReviewFallbackBackend()
			b.failAt, b.failure = tc.failAt, tc.err
			p := reviewDiffCall(t, NewServer(b), "svc", "pkg/target.go")
			if b.reads != tc.failAt+1 {
				t.Errorf("typed reads=%d, want %d ending at the direct fallback", b.reads, tc.failAt+1)
			}
			if b.loads != 1 {
				t.Errorf("symbol loads=%d, want 1", b.loads)
			}
			if want := navLoadError("review_diff", "svc", tc.err).Message; p["partial"] != want {
				t.Errorf("partial = %v, want %q", p["partial"], want)
			}
			for _, field := range []string{"resolution", "layers", "receipt", "receipt_sarif", "receipt_check"} {
				if p[field] != nil {
					t.Errorf("%s present without the impact graph", field)
				}
			}
			ti, _ := p["test_impact"].(map[string]any)
			reason := ti["reason"]
			if ti["checked"] != false || reason != navLoadNote("svc", tc.err) {
				t.Errorf("test_impact = %v, want checked:false naming the load failure", ti)
			}
			if strings.Contains(fmt.Sprint(reason), "no symbols indexed") {
				t.Errorf("an indexed service's load failure read as an empty index: %v", reason)
			}
		})
	}
}
