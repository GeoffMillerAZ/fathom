package mcp_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// verifyFakeBackend is a QueryInterface fake with a small fixture: a verified,
// tested symbol (DoWork), an untested one (Untested), an entrypoint (Router)
// that reaches DoWork, a contradicted-but-tested symbol (Conflicted), and the
// ADR-021 §3 O9 residue shape: RunAdapter dispatches to Complete on two
// receivers (AdapterA, AdapterB) — an ambiguous method name the syntax-only
// resolver refuses (no Calls edge is even attempted here), reachable ONLY
// through typedEdges.
type verifyFakeBackend struct {
	nodes      []core.SymbolNode
	contras    []core.Contradiction
	typedEdges []core.Edge
}

func newVerifyBackend() *verifyFakeBackend {
	return &verifyFakeBackend{
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "work", Name: "DoWork", Kind: core.KindGoFunc, FilePath: "work/do.go", Line: 10, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "TestDoWork", Kind: core.KindGoFunc, FilePath: "work/do_test.go", Line: 5, Calls: []string{"DoWork"}, Tier: core.TierVerified},
			{Service: "svc", Package: "api", Name: "Router", Kind: core.KindGoFunc, FilePath: "api/router.go", Line: 3, Calls: []string{"DoWork"}, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "Untested", Kind: core.KindGoFunc, FilePath: "work/untested.go", Line: 1, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "Conflicted", Kind: core.KindGoFunc, FilePath: "work/c.go", Line: 1, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "TestConflicted", Kind: core.KindGoFunc, FilePath: "work/c_test.go", Line: 1, Calls: []string{"Conflicted"}, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "RunAdapter", Kind: core.KindGoFunc, FilePath: "work/adapter.go", Line: 1, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "TestRunAdapter", Kind: core.KindGoFunc, FilePath: "work/adapter_test.go", Line: 1, Calls: []string{"RunAdapter"}, Tier: core.TierVerified},
			{Service: "svc", Package: "AdapterA", Name: "AdapterA.Complete", Kind: core.KindGoMethod, FilePath: "work/adaptera.go", Line: 1, Tier: core.TierVerified},
			{Service: "svc", Package: "AdapterB", Name: "AdapterB.Complete", Kind: core.KindGoMethod, FilePath: "work/adapterb.go", Line: 1, Tier: core.TierVerified},
		},
		contras: []core.Contradiction{{
			Symbol: "Conflicted",
			A:      core.Observation{SourceRef: "work/c.go"},
			B:      core.Observation{SourceRef: "docs/c.md"},
		}},
		typedEdges: []core.Edge{
			{FromService: "svc", FromFile: "work/adapter.go", FromSymbol: "svc::work/work.RunAdapter", ToSymbol: "svc::work/AdapterA.AdapterA.Complete", Kind: core.EdgeIfaceDispatch},
			{FromService: "svc", FromFile: "work/adapter.go", FromSymbol: "svc::work/work.RunAdapter", ToSymbol: "svc::work/AdapterB.AdapterB.Complete", Kind: core.EdgeIfaceDispatch},
		},
	}
}

func (f *verifyFakeBackend) QuerySymbols(_ context.Context, _ core.Query) (core.QueryResult, error) {
	return core.QueryResult{Contradictions: f.contras}, nil
}

func (f *verifyFakeBackend) FindCallers(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *verifyFakeBackend) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range f.nodes {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *verifyFakeBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return f.nodes, nil
}

func (f *verifyFakeBackend) Services(_ context.Context) ([]string, error) {
	return []string{"svc"}, nil
}

func (f *verifyFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return nil, nil
}
func (f *verifyFakeBackend) IngestService(_ context.Context, _ string) error { return nil }
func (f *verifyFakeBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

func (f *verifyFakeBackend) ListCommits(_ context.Context, _ string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

func (f *verifyFakeBackend) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) {
	return f.typedEdges, nil
}

func TestVerifyPlan_Tool(t *testing.T) {
	srv := mcp.NewServer(newVerifyBackend())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3001, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service": "svc",
				"concern": "work",
				"tasks": []any{
					map[string]any{"id": "T-1", "title": "do work", "symbol": "DoWork"},
					map[string]any{"id": "T-2", "title": "untested", "symbol": "Untested"},
					map[string]any{"id": "T-3", "title": "ghost", "symbol": "Ghost"},
					map[string]any{"id": "T-4", "title": "wired", "symbol": "DoWork", "reachable_from": "Router"},
					map[string]any{"id": "T-5", "title": "conflict", "symbol": "Conflicted"},
					// T-6: claimed done — should surface claimed=true in output.
					map[string]any{"id": "T-6", "title": "claimed done", "symbol": "DoWork", "claimed": true},
					// T-7: skipped — should surface skipped=true in output.
					map[string]any{"id": "T-7", "title": "skipped item", "symbol": "DoWork", "skipped": true},
				},
			},
		},
	})
	p := xtraPayload(t, resp)

	// Index tasks by id.
	byID := map[string]map[string]any{}
	for _, ti := range p["tasks"].([]any) {
		tm := ti.(map[string]any)
		byID[tm["id"].(string)] = tm
	}

	want := map[string][2]string{
		"T-1": {"pass", "verified"},
		"T-2": {"fail", "present-but-untested"},
		"T-3": {"fail", "claimed-but-absent"},
		"T-4": {"pass", "verified"},
		"T-5": {"fail", "contradicted"},
		"T-6": {"pass", "verified"},
		"T-7": {"pass", "verified"},
	}
	for id, exp := range want {
		tm, ok := byID[id]
		if !ok {
			t.Fatalf("task %s missing from result", id)
		}
		if got := tm["status"].(string); got != exp[0] {
			t.Errorf("%s: status = %q, want %q", id, got, exp[0])
		}
		if got := tm["label"].(string); got != exp[1] {
			t.Errorf("%s: label = %q, want %q", id, got, exp[1])
		}
	}

	// T-6 must emit claimed=true; T-7 must emit skipped=true.
	if byID["T-6"]["claimed"] != true {
		t.Errorf("T-6 should have claimed=true in output, got %v", byID["T-6"]["claimed"])
	}
	if byID["T-7"]["skipped"] != true {
		t.Errorf("T-7 should have skipped=true in output, got %v", byID["T-7"]["skipped"])
	}
	// T-1 has no claimed/skipped set — neither key should be present.
	if _, ok := byID["T-1"]["claimed"]; ok {
		t.Errorf("T-1 should not have 'claimed' key when false")
	}

	// runtime_pass is unknown when no attested run can be read — this fake
	// holds no store (TC-29: never a fabricated verdict; the daemon-mode
	// read is TestVerifyPlan_Tool_RuntimePassReadsAttestedRun).
	t1 := byID["T-1"]
	var sawRuntimeUnknown bool
	for _, ci := range t1["criteria"].([]any) {
		c := ci.(map[string]any)
		if c["name"] == "runtime_pass" && c["status"] == "unknown" {
			sawRuntimeUnknown = true
		}
	}
	if !sawRuntimeUnknown {
		t.Error("T-1 must report runtime_pass=unknown")
	}

	summary := p["summary"].(map[string]any)
	if summary["total"].(float64) != 7 {
		t.Errorf("summary.total = %v, want 7", summary["total"])
	}
}

// runtimeStoreBackend is verifyFakeBackend with a store holding attested
// runs: the store-backed backend shape the daemon serves verify_plan from.
type runtimeStoreBackend struct {
	*verifyFakeBackend
	st store.Store
}

func (b runtimeStoreBackend) Store() store.Store { return b.st }

// TestVerifyPlan_Tool_RuntimePassReadsAttestedRun is TC-29 in daemon mode:
// verify_plan reads the newest attested run of a row's certifying test
// from the backend's store — Pass or Fail with its commit and age — and the
// row's status never moves (I8).
func TestVerifyPlan_Tool_RuntimePassReadsAttestedRun(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	sha := "89abcdef0123456789abcdef0123456789abcdef"
	if err := st.PutTestRuns(ctx, "svc", []store.TestRun{
		{RunDigest: "sha256:0badc0de0badc0de", Suite: "unit", StartedAt: "2026-10-10T00:00:00Z", Commit: sha, Cases: []store.TestCase{
			{CaseID: "1", Name: "TestDoWork", Status: "pass", Attempts: 1, TestSymbolKey: "svc::work/work.TestDoWork"},
			{CaseID: "2", Name: "TestConflicted", Status: "fail", Attempts: 1, TestSymbolKey: "svc::work/work.TestConflicted"},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	srv := mcp.NewServer(runtimeStoreBackend{newVerifyBackend(), st})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3002, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service": "svc",
				"tasks": []any{
					map[string]any{"id": "T-1", "title": "do work", "symbol": "DoWork", "reachable_from": "TestDoWork"},
					map[string]any{"id": "T-2", "title": "untested", "symbol": "Untested"},
					map[string]any{"id": "T-3", "title": "conflicted", "symbol": "Conflicted", "allow_contradiction": true},
				},
			},
		},
	})
	p := xtraPayload(t, resp)
	got := map[string]map[string]any{}
	status := map[string]string{}
	for _, ti := range p["tasks"].([]any) {
		tm := ti.(map[string]any)
		id := tm["id"].(string)
		status[id] = tm["status"].(string)
		for _, ci := range tm["criteria"].([]any) {
			if c := ci.(map[string]any); c["name"] == "runtime_pass" {
				got[id] = c
			}
		}
	}
	if c := got["T-1"]; c["status"] != "pass" || !strings.Contains(c["detail"].(string), "89abcdef0123") || !strings.Contains(c["detail"].(string), "old") {
		t.Errorf("T-1 runtime_pass = %v, want pass with the run's commit and age", c)
	}
	if c := got["T-2"]; c["status"] != "unknown" {
		t.Errorf("T-2 runtime_pass = %v, want unknown (no certifying test)", c)
	}
	if c := got["T-3"]; c["status"] != "fail" || status["T-3"] != "pass" {
		t.Errorf("T-3 runtime_pass = %v with row %s, want fail on a passing row (informational)", c, status["T-3"])
	}
}

// TestVerifyPlan_Tool_TypedEdgeDispatch is the CT-4 residue-flip proof
// (ADR-021 §3), exercised through the real verify_plan MCP tool end to end
// (not just the internal/verify unit test): a claim on AdapterA.Complete —
// reachable ONLY via a typed interface-dispatch edge, never via the
// syntax-only graph (RunAdapter has no Calls entry naming "Complete" at
// all, mirroring the ambiguous-method-name refusal graph.go:218-240 always
// gave) — now verifies Pass, with the dispatch modality named in the detail.
func TestVerifyPlan_Tool_TypedEdgeDispatch(t *testing.T) {
	srv := mcp.NewServer(newVerifyBackend())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3002, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service": "svc",
				"tasks": []any{
					map[string]any{"id": "T-1", "title": "adapter residue", "symbol": "AdapterA.Complete", "claimed": true},
				},
			},
		},
	})
	p := xtraPayload(t, resp)
	tasks := p["tasks"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(tasks))
	}
	tm := tasks[0].(map[string]any)
	if got := tm["status"].(string); got != "pass" {
		t.Fatalf("status = %q, want pass: %+v", got, tm)
	}
	if got := tm["label"].(string); got != "verified" {
		t.Errorf("label = %q, want verified", got)
	}
	var detail string
	for _, ci := range tm["criteria"].([]any) {
		c := ci.(map[string]any)
		if c["name"] == "test_reaches" {
			detail = c["detail"].(string)
		}
	}
	if detail == "" {
		t.Fatal("no test_reaches criterion in output")
	}
	if !strings.Contains(detail, "reached via interface dispatch") {
		t.Errorf("test_reaches detail = %q, want it to mention interface dispatch", detail)
	}
	if !strings.Contains(detail, "2 implementer") {
		t.Errorf("test_reaches detail = %q, want it to mention 2 implementers", detail)
	}
	if !strings.Contains(detail, "TestRunAdapter") {
		t.Errorf("test_reaches detail = %q, want it to name the reaching test", detail)
	}
}

// TestVerifyPlan_ReusesWatermarkedCallGraphMemo is O33's fix, pinned: before
// it, verify_plan rebuilt its call graph (callgraph.Build/AddTypedEdges)
// from scratch on every call — profiled at 13-25s on this repository's
// ~11.5k symbols (everything else in the handler combined: under 200ms) —
// even though the watermark-keyed memo (BP-28, I21) already existed and
// served the nav tools. A second verify_plan call at an unchanged watermark
// must now HIT that same cache instead of building again.
func TestVerifyPlan_ReusesWatermarkedCallGraphMemo(t *testing.T) {
	srv := mcp.NewServer(newNavFake())
	args := map[string]any{
		"service": "svc",
		"tasks":   []any{map[string]any{"id": "R-1", "symbol": "Target"}},
	}

	callTool(t, srv, "verify_plan", args)
	_, hits1, misses1 := srv.GraphCacheStatsForTest()
	if misses1 == 0 {
		t.Fatal("first call should have missed the cache (nothing built yet) — the memo isn't wired at all")
	}

	callTool(t, srv, "verify_plan", args)
	_, hits2, misses2 := srv.GraphCacheStatsForTest()
	if hits2 <= hits1 {
		t.Errorf("second call at the same watermark did not hit the memo (hits %d -> %d): verify_plan is still rebuilding its own graph", hits1, hits2)
	}
	if misses2 != misses1 {
		t.Errorf("second call at the same watermark rebuilt the graph (misses %d -> %d) — O33 regression", misses1, misses2)
	}
}

// TestVerifyPlan_VerdictOutcomeReachesCallLog is T5-7's real evidence
// (docs/plans/trust-and-tokenomics.md): the gate-catches instrument is the
// JSONL call log, not just the tool result — mirrors D1-2's
// TestBrief_ClaimsAttachedReachesCallLog (extra_tools_test.go) exactly,
// generalized from a scalar (claims_attached) to named counts
// (verdict_outcome). Covers both write paths: the top-level dispatch
// (loggedToolsCall, reading the field back out of the wrapped MCP content
// envelope) and the batch sub-call path (logSub, reading the raw handler
// result map).
func TestVerifyPlan_VerdictOutcomeReachesCallLog(t *testing.T) {
	// T-1 claimed+verified, T-2 claimed+failing, T-3 unclaimed (must not
	// count) — claimed:2 verified:1 failing:1 over exactly the tasks
	// internal/verify.Gate itself would gate on.
	tasks := []any{
		map[string]any{"id": "T-1", "title": "do work", "symbol": "DoWork", "claimed": true},
		map[string]any{"id": "T-2", "title": "untested", "symbol": "Untested", "claimed": true},
		map[string]any{"id": "T-3", "title": "ghost", "symbol": "Ghost"},
	}
	args := map[string]any{"service": "svc", "tasks": tasks}

	t.Run("top_level", func(t *testing.T) {
		var buf bytes.Buffer
		srv := mcp.NewServerWithLogger(newVerifyBackend(), mcp.NewCallLogger(&buf))
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 3101, Method: "tools/call",
			Params: map[string]any{"name": "verify_plan", "arguments": args},
		})
		p := xtraPayload(t, resp)
		vo, ok := p["verdict_outcome"].(map[string]any)
		if !ok {
			t.Fatalf("result payload missing verdict_outcome (fixture broken?): %v", p)
		}
		if vo["claimed"] != float64(2) || vo["verified"] != float64(1) || vo["failing"] != float64(1) {
			t.Fatalf("result verdict_outcome = %v, want claimed:2 verified:1 failing:1", vo)
		}

		entry := callLogEntryFor(t, &buf, "verify_plan", "")
		logged, ok := entry["verdict_outcome"].(map[string]any)
		if !ok {
			t.Fatalf("logged entry missing verdict_outcome: %v", entry)
		}
		if logged["claimed"] != float64(2) || logged["verified"] != float64(1) || logged["failing"] != float64(1) {
			t.Errorf("logged verdict_outcome = %v, want claimed:2 verified:1 failing:1", logged)
		}
	})

	// The "via_batch" subtest (verdict_outcome reaching the call log
	// through a batch sub-call, via:"batch") was retired: batch was
	// unregistered by SR-19 (surface reckoning, F-77).

	t.Run("no_claimed_tasks_omits_field", func(t *testing.T) {
		var buf bytes.Buffer
		srv := mcp.NewServerWithLogger(newVerifyBackend(), mcp.NewCallLogger(&buf))
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 3103, Method: "tools/call",
			Params: map[string]any{
				"name": "verify_plan",
				"arguments": map[string]any{
					"service": "svc",
					"tasks": []any{
						map[string]any{"id": "T-1", "title": "unclaimed", "symbol": "DoWork"},
					},
				},
			},
		})
		p := xtraPayload(t, resp)
		if _, present := p["verdict_outcome"]; present {
			t.Errorf("verdict_outcome should be absent with zero claimed tasks, got: %v", p["verdict_outcome"])
		}
		entry := callLogEntryFor(t, &buf, "verify_plan", "")
		if _, present := entry["verdict_outcome"]; present {
			t.Errorf("logged entry should have no verdict_outcome field, got: %v", entry)
		}
	})
}

// TestVerifyPlan_Tool_StructuredSkipReason is wave-N5's follow-up (O56): the
// Markdown parser has always accepted a `skip:` annotation key alongside
// `skip_test: true` (markdown.go, ann["skip"] -> Expect.SkipReason), but the
// structured `tasks[]` JSON schema exposed no equivalent field, so a
// structured caller's skip_test was ALWAYS refused as skip-without-reason —
// there was no way to supply a reason at all. This pins that a `skip` string
// field on a structured task now reaches Expect.SkipReason exactly as the
// Markdown path does: bare skip_test still fails skip-without-reason, and
// skip_test+skip passes with the reason in the detail.
func TestVerifyPlan_Tool_StructuredSkipReason(t *testing.T) {
	srv := mcp.NewServer(newVerifyBackend())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3201, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service": "svc",
				"tasks": []any{
					// Bare skip_test, no reason: still refused.
					map[string]any{"id": "T-BARE", "title": "bare skip", "symbol": "Untested", "skip_test": true},
					// skip_test plus a structured `skip` reason: now passes,
					// same as the Markdown `skip:` annotation would.
					map[string]any{"id": "T-REASONED", "title": "reasoned skip", "symbol": "Untested", "skip_test": true, "skip": "covered by TestUntestedIntegration in another package, not visible to this graph"},
				},
			},
		},
	})
	p := xtraPayload(t, resp)

	byID := map[string]map[string]any{}
	for _, ti := range p["tasks"].([]any) {
		tm := ti.(map[string]any)
		byID[tm["id"].(string)] = tm
	}

	criterionFor := func(t *testing.T, tm map[string]any, name string) map[string]any {
		for _, ci := range tm["criteria"].([]any) {
			c := ci.(map[string]any)
			if c["name"] == name {
				return c
			}
		}
		t.Fatalf("no %q criterion in %v", name, tm)
		return nil
	}

	bare := byID["T-BARE"]
	if got := bare["status"].(string); got != "fail" {
		t.Errorf("T-BARE: status = %q, want fail", got)
	}
	if c := criterionFor(t, bare, "skip_test"); c["status"] != "fail" || c["detail"] != "skip_test: true with no skip: reason — bare skips are refused" {
		t.Errorf("T-BARE: skip_test criterion = %v", c)
	}

	reasoned := byID["T-REASONED"]
	if got := reasoned["status"].(string); got != "pass" {
		t.Errorf("T-REASONED: status = %q, want pass", got)
	}
	if c := criterionFor(t, reasoned, "skip_test"); c["status"] != "pass" || !strings.Contains(c["detail"].(string), "covered by TestUntestedIntegration") {
		t.Errorf("T-REASONED: skip_test criterion = %v, want the structured skip reason in the detail", c)
	}
}

func (f *verifyFakeBackend) ScanSources(ctx context.Context, svc string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	files, err := f.Sources(ctx, svc)
	if err != nil {
		return corpus.Coverage{}, err
	}
	return corpus.ScanSourceFiles(ctx, files, bud, fn)
}

// stoppedScanVerifyBackend's tree scan stops after its first file (the
// 64 MiB / 5 s handler budget spent): the spec ref lives in a file the
// scan never reached.
type stoppedScanVerifyBackend struct {
	*verifyFakeBackend
}

func (b stoppedScanVerifyBackend) ScanSources(_ context.Context, _ string, _ corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	fn(core.SourceFile{Path: "a.go", Content: []byte("package a\n")})
	return corpus.Coverage{Scanned: 1, Total: 2, Stopped: corpus.StopBytes}, nil
}

// I38: a spec matrix built from a scan that stopped early is a lower
// bound — an "uncovered" spec may sit in an unscanned file — so the
// answer names the partial scan instead of reading as a complete one.
func TestVerifyPlan_PartialSpecScanIsNamed(t *testing.T) {
	srv := mcp.NewServer(stoppedScanVerifyBackend{newVerifyBackend()})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service": "svc",
				"tasks": []any{
					map[string]any{"id": "T-1", "title": "spec", "symbol": "DoWork", "spec_ids": []any{"SPEC-1"}},
				},
			},
		},
	})
	p := xtraPayload(t, resp)
	note, _ := p["partial"].(string)
	if !strings.Contains(note, "scanned 1/2 files: budget bytes") || !strings.Contains(note, "spec") {
		t.Fatalf("partial = %q, want the spec matrix's stopped scan named", note)
	}

	// A plan with no spec IDs never scans: no partial.
	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service": "svc",
				"tasks":   []any{map[string]any{"id": "T-1", "title": "do", "symbol": "DoWork"}},
			},
		},
	})
	if _, ok := xtraPayload(t, resp)["partial"]; ok {
		t.Fatal("partial set without a spec scan")
	}
}
