package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// review_cancel_postverdict_test.go pins GDEV-03's in-process half
// (consumer O21): a review_diff cancelled after review.Run — in findings
// persistence, citation binding, the service-ref hops, the SARIF anchor or
// an index_commit read — stops at that phase's boundary. No review read
// and no finding write starts after the cancellation, and no graph build
// starts for the call.

// reviewPostVerdictBackend is the mixed resolution fixture with a local
// source root, the review claim store, Services and source fingerprints,
// every read and write logged in call order. It cancels at the read whose
// position in the log is cancelAt (a phase is chosen by call order: the
// same method serves several phases). It has no IngestState: the citation
// wrapper's post-handler recheck is outside the review's phases. The log
// is locked: memo flights call the backend from their own goroutines.
type reviewPostVerdictBackend struct {
	*multiSvcBackend
	testStore
	mu       sync.Mutex
	log      []string
	after    []string
	cancelAt int
	cancel   context.CancelFunc
	misses   int64
	refs     map[string]store.ServiceRefs
}

func (b *reviewPostVerdictBackend) record(ctx context.Context, call string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ctx.Err() != nil {
		b.after = append(b.after, call)
	}
	b.log = append(b.log, call)
	if b.cancel != nil && len(b.log)-1 == b.cancelAt {
		b.cancel()
		b.misses = reviewGraphMisses()
	}
}

// calls is the call log and the calls made after cancellation.
func (b *reviewPostVerdictBackend) calls() (log, after []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.log), slices.Clone(b.after)
}

func (*reviewPostVerdictBackend) StoreWatermark(context.Context, string) (string, error) {
	return "w1", nil
}
func (b *reviewPostVerdictBackend) ListAllSymbols(ctx context.Context, service string) ([]core.SymbolNode, error) {
	b.record(ctx, "ListAllSymbols")
	return b.multiSvcBackend.ListAllSymbols(ctx, service)
}
func (b *reviewPostVerdictBackend) TypedEdges(ctx context.Context, service string) ([]core.Edge, error) {
	b.record(ctx, "TypedEdges")
	return b.multiSvcBackend.TypedEdges(ctx, service)
}
func (b *reviewPostVerdictBackend) ServiceMeta(ctx context.Context, service string) (string, string, bool, error) {
	b.record(ctx, "ServiceMeta")
	return b.multiSvcBackend.ServiceMeta(ctx, service)
}
func (b *reviewPostVerdictBackend) Services(ctx context.Context) ([]string, error) {
	b.record(ctx, "Services")
	return b.multiSvcBackend.Services(ctx)
}

// ServiceRefsFor serves a route the changed file registers and two calls
// into it from another service: two service_ref hops, so the receipt has
// two link hints whose citations are read after the verdict.
func (b *reviewPostVerdictBackend) ServiceRefsFor(ctx context.Context, service string) (store.ServiceRefs, bool, error) {
	b.record(ctx, "ServiceRefsFor")
	return b.refs[service], true, nil
}
func (b *reviewPostVerdictBackend) Sources(ctx context.Context, service string) ([]core.SourceFile, error) {
	b.record(ctx, "Sources")
	return b.multiSvcBackend.Sources(ctx, service)
}
func (b *reviewPostVerdictBackend) ScanSources(ctx context.Context, service string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	b.record(ctx, "ScanSources")
	return b.multiSvcBackend.ScanSources(ctx, service, bud, fn)
}
func (b *reviewPostVerdictBackend) SourceFingerprint(ctx context.Context, _, path string) (string, bool, error) {
	b.record(ctx, "SourceFingerprint:"+path)
	return "", false, nil
}
func (b *reviewPostVerdictBackend) ClaimsForService(ctx context.Context, _, layer string) ([]core.SemanticClaim, error) {
	b.record(ctx, "ClaimsForService:"+layer)
	if layer != review.ClaimsLayer {
		return nil, nil
	}
	// A served arch finding on the changed file that this run no longer
	// emits: reconciliation supersedes it, so the review writes.
	return []core.SemanticClaim{{ClaimID: "stale-arch", Layer: review.ClaimsLayer, PromptVersion: review.ClaimsPromptVersion,
		Status: core.ClaimFresh, Subject: review.DimArch + "#gone", Scope: []string{"pkg/c.go"}}}, nil
}
func (b *reviewPostVerdictBackend) PutClaims(ctx context.Context, _ []core.SemanticClaim, _ []core.ClaimLink) error {
	b.record(ctx, "write:PutClaims")
	return nil
}
func (b *reviewPostVerdictBackend) SetClaimStatus(ctx context.Context, _, _ string, _ core.ClaimStatus) error {
	b.record(ctx, "write:SetClaimStatus")
	return nil
}
func (b *reviewPostVerdictBackend) SetClaimValidTo(ctx context.Context, _, _ string, _ time.Time) error {
	b.record(ctx, "write:SetClaimValidTo")
	return nil
}
func (b *reviewPostVerdictBackend) WriteSupersedes(ctx context.Context, _, _, _, _ string) error {
	b.record(ctx, "write:WriteSupersedes")
	return nil
}

func newReviewPostVerdictBackend(t *testing.T) *reviewPostVerdictBackend {
	t.Helper()
	root := t.TempDir()
	b := &reviewPostVerdictBackend{multiSvcBackend: resolutionFixtureBackend(), cancelAt: -1}
	fx := b.svcs["svc"]
	fx.path = root
	fx.sources = map[string][]byte{}
	for _, n := range fx.nodes {
		fx.sources[n.FilePath] = []byte("package pkg\n\nfunc " + n.Name + "() {}\n")
	}
	// Two changed route files with no indexed symbol: their link hints are
	// the first citations of those files, so binding them reads.
	b.svcs["other"] = &svcFixture{}
	b.refs = map[string]store.ServiceRefs{}
	for _, name := range []string{"one", "two"} {
		file := "pkg/route_" + name + ".go"
		route := []byte("package pkg\n\nimport \"net/http\"\n\nfunc init() { http.HandleFunc(\"GET /" + name + "\", nil) }\n")
		call := []byte("package client\n\nimport \"net/http\"\n\nfunc Fetch() { http.Get(\"http://svc/" + name + "\") }\n")
		fx.sources[file], fx.sources["client/"+name+".go"] = route, call
		svcRefs, otherRefs := b.refs["svc"], b.refs["other"]
		svcRefs.Routes = append(svcRefs.Routes, serviceref.ExtractFile(file, route).Routes...)
		otherRefs.Calls = append(otherRefs.Calls, serviceref.ExtractFile("client/"+name+".go", call).Calls...)
		b.refs["svc"], b.refs["other"] = svcRefs, otherRefs
	}
	for _, dir := range []string{"pkg", "client"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A rule pack the fixture keeps, so arch_conformance is checked.
	if err := os.MkdirAll(filepath.Join(root, ".fathom", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fathom", "rules", "archconformance.cue"), []byte("rules: [{from_pkg: \"api\", to_pkg: \"store\", allow: false}]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for path, src := range fx.sources {
		if err := os.WriteFile(filepath.Join(root, path), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The indexed commit is the root's HEAD, so the SARIF anchor resolves
	// it and reads the stored fingerprints.
	citationGit(t, root, "init", "-q")
	citationGit(t, root, "add", "-A")
	citationGit(t, root, "commit", "-qm", "fixture")
	fx.indexCommit = citationGit(t, root, "rev-parse", "HEAD")
	return b
}

var reviewPostVerdictRequest = []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"review_diff","arguments":{"service":"svc","changed_files":["pkg/c.go","pkg/route_one.go","pkg/route_two.go"]}}}`)

// reviewPostVerdictPhases finds each post-verdict phase's first read in an
// uncancelled review's call log. Persistence opens with the review claims
// read. Citation binding follows the last finding write, one ServiceMeta
// and SourceFingerprint per cited file. The first index_commit read is the
// ServiceMeta just before Services, then the hops read every service's
// refs; that phase cancels at its last read. A cancellation inside it
// (during Services or an earlier service's refs) still reads the remaining
// services' refs: serviceRefHopsInto and serviceref.BuildFromStore check
// no ctx between reads, and the memo starts a flight for a caller that has
// already left. Those sites are outside the review's own phase boundaries.
// Each receipt link hint binds a route file no earlier phase cited; the
// second index_commit read follows the last hint, and the SARIF anchor's
// stored fingerprints follow it. The citation phases cancel at a file's
// last read: between two files, and after the last one. A cancellation
// inside one file's citation stream (citation_stream.go) finishes that
// file's identity reads; that stream is the citation-freshness contract's.
func reviewPostVerdictPhases(t *testing.T, log []string) map[string]int {
	t.Helper()
	lastWrite := -1
	for i, call := range log {
		if strings.HasPrefix(call, "write:") {
			lastWrite = i
		}
	}
	at := func(call string) int { return slices.Index(log, call) }
	services := at("Services")
	lastRefs := services
	for lastRefs+1 < len(log) && log[lastRefs+1] == "ServiceRefsFor" {
		lastRefs++
	}
	hintOne, hintTwo := at("SourceFingerprint:pkg/route_one.go"), at("SourceFingerprint:pkg/route_two.go")
	phases := map[string]int{
		"persistence":         at("ClaimsForService:" + review.ClaimsLayer),
		"citations":           lastWrite + 2,
		"citations-last-file": services - 2,
		"index-commit":        services - 1,
		"service-ref-hops":    lastRefs,
		"link-hints":          hintOne,
		"link-hints-last":     hintTwo,
		"sarif-index-commit":  hintTwo + 1,
		"sarif-anchor":        hintTwo + 2,
	}
	want := map[string]string{
		"persistence": "ClaimsForService:" + review.ClaimsLayer, "citations": "SourceFingerprint:pkg/a.go", "citations-last-file": "SourceFingerprint:pkg/a_test.go",
		"index-commit": "ServiceMeta", "service-ref-hops": "ServiceRefsFor", "link-hints": "SourceFingerprint:pkg/route_one.go", "link-hints-last": "SourceFingerprint:pkg/route_two.go",
		"sarif-index-commit": "ServiceMeta", "sarif-anchor": "SourceFingerprint:pkg/c.go",
	}
	if lastWrite < phases["persistence"] || !(phases["citations"] < phases["citations-last-file"]) || !(services < hintOne && hintOne < hintTwo) {
		t.Fatalf("unexpected review call log: %v", log)
	}
	for phase, i := range phases {
		if i < 0 || i >= len(log) || log[i] != want[phase] {
			t.Fatalf("phase %s: read %d is not %s in %v", phase, i, want[phase], log)
		}
	}
	return phases
}

func TestReviewDiffCancelledAfterVerdictStopsAtPhaseBoundary(t *testing.T) {
	testReviewDiffStopsAfterVerdict(t, false)
}

func TestReviewDiffDeadlineAfterVerdictStopsAtPhaseBoundary(t *testing.T) {
	testReviewDiffStopsAfterVerdict(t, true)
}

func testReviewDiffStopsAfterVerdict(t *testing.T, deadline bool) {
	t.Helper()
	probe := newReviewPostVerdictBackend(t)
	raw, ok := NewServer(probe).HandleRequest(context.Background(), reviewPostVerdictRequest)
	if !ok || strings.Contains(string(raw), `"error":`) {
		t.Fatalf("uncancelled review: %s", raw)
	}
	probeLog, _ := probe.calls()
	phases := reviewPostVerdictPhases(t, probeLog)
	t.Logf("review call log: %v", probeLog)
	for phase, at := range phases {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var requestContext context.Context = ctx
			want := "canceled"
			if deadline {
				requestContext = reviewDeadlineContext{ctx}
				want = "deadline exceeded"
			}
			b := newReviewPostVerdictBackend(t)
			b.cancelAt, b.cancel = at, cancel
			raw, ok := NewServer(b).HandleRequest(requestContext, reviewPostVerdictRequest)
			if !ok {
				t.Fatal("no response")
			}
			var env struct {
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				t.Fatal(err)
			}
			if env.Error == nil || !strings.Contains(env.Error.Message, want) {
				t.Fatalf("cancellation in %s not named %q: %s", phase, want, raw)
			}
			log, after := b.calls()
			if len(log) <= at || log[at] != probeLog[at] {
				t.Fatalf("call order diverged from the probe at %d: %v", at, log)
			}
			for _, call := range after {
				if strings.HasPrefix(call, "write:") {
					t.Errorf("finding write after cancellation in %s: %s", phase, call)
				}
			}
			if len(after) > 0 {
				t.Errorf("review reads after cancellation in %s: %v", phase, after)
			}
			if got := reviewGraphMisses() - b.misses; got != 0 {
				t.Errorf("graph builds after cancellation in %s: %d", phase, got)
			}
		})
	}
}
