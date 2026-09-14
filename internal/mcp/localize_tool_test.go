// Package mcp_test — localize (VB-26, vision 29 Wave G): end-to-end tests
// over a real (fake) backend exercising every input shape, the budget
// gate, the response shape, and determinism.
package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// localizeFakeBackend is a small, self-contained QueryInterface +
// obligationsBackend + storeWatermarker fake purpose-built for localize's
// own tests: real per-name FindSymbol filtering, real TypedEdges, a real
// on-disk svcPath so the search-based hops actually run against real
// files, and a governed file for the obligations-boost test.
type localizeFakeBackend struct {
	nodes       []core.SymbolNode
	typedEdges  []core.Edge
	svcPath     string
	indexCommit string
	governed    map[string]semantic.Obligation // file -> the one rule it carries
	// notIndexed makes ServiceMeta report found=false — an unknown or
	// never-ingested service, for the ADR-014 execute-and-flag test.
	notIndexed bool
}

func (f *localizeFakeBackend) QuerySymbols(context.Context, core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (f *localizeFakeBackend) FindCallers(context.Context, string, string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *localizeFakeBackend) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range f.nodes {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *localizeFakeBackend) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	return f.nodes, nil
}
func (f *localizeFakeBackend) Services(context.Context) ([]string, error) { return nil, nil }
func (f *localizeFakeBackend) Sources(context.Context, string) ([]core.SourceFile, error) {
	return nil, nil
}
func (f *localizeFakeBackend) IngestService(context.Context, string) error { return nil }
func (f *localizeFakeBackend) ServiceMeta(context.Context, string) (string, string, bool, error) {
	if f.notIndexed {
		return "", "", false, nil
	}
	return f.svcPath, f.indexCommit, true, nil
}

func (f *localizeFakeBackend) ListCommits(context.Context, string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

func (f *localizeFakeBackend) TypedEdges(context.Context, string) ([]core.Edge, error) {
	return f.typedEdges, nil
}

// StoreWatermark makes this fake eligible for the BP-28 graph memo.
func (f *localizeFakeBackend) StoreWatermark(context.Context, string) (string, error) {
	return "watermark-1", nil
}

func (f *localizeFakeBackend) Obligations(_ context.Context, _, file string, _ int) (semantic.Obligations, error) {
	if o, ok := f.governed[file]; ok {
		return semantic.Obligations{Rules: []semantic.Obligation{o}}, nil
	}
	return semantic.Obligations{}, nil
}

func (f *localizeFakeBackend) ObligationsIndexStale(context.Context, string, string) bool {
	return false
}

var _ mcp.QueryInterface = (*localizeFakeBackend)(nil)

// localizeFixture builds the shared fixture every test below runs
// against: Balance (ledger.go) is called by Handler (handler.go) and
// TestBalance (handler_test.go), both typed-confirmed edges — so a
// stack-frame or find_symbol seed on Balance always has real callers to
// expand into via graph proximity. ledger.go and handler.go are real
// files on disk (under t.TempDir()) with content matching the index, so
// the search-based hops (literal/term seeds) find real hits too; ledger.go
// carries a governing rule for the obligations-boost test.
func localizeFixture(t *testing.T) *localizeFakeBackend {
	t.Helper()
	root := t.TempDir()
	ledgerSrc := "package ledger\n\nfunc Balance(accountID string) (int64, error) {\n\tif accountID == \"\" {\n\t\treturn 0, errMissingAccount\n\t}\n\treturn 0, nil\n}\n"
	handlerSrc := "package ledger\n\nfunc Handler() {\n\tBalance(\"x\")\n}\n"
	testSrc := "package ledger\n\nfunc TestBalance(t *T) {\n\tBalance(\"x\")\n}\n"
	for name, content := range map[string]string{
		"ledger.go": ledgerSrc, "handler.go": handlerSrc, "handler_test.go": testSrc,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing fixture %s: %v", name, err)
		}
	}

	nodes := []core.SymbolNode{
		{Service: "svc", Package: "ledger", Name: "Balance", Kind: core.KindGoFunc, FilePath: "ledger.go", Line: 3},
		{Service: "svc", Package: "ledger", Name: "Handler", Kind: core.KindGoFunc, FilePath: "handler.go", Line: 3},
		{Service: "svc", Package: "ledger", Name: "TestBalance", Kind: core.KindGoFunc, FilePath: "handler_test.go", Line: 3},
	}
	byName := map[string]string{}
	for _, n := range nodes {
		byName[n.Name] = n.Key()
	}
	return &localizeFakeBackend{
		nodes:       nodes,
		svcPath:     root,
		indexCommit: "commit-abc123",
		typedEdges: []core.Edge{
			{FromService: "svc", FromSymbol: byName["Handler"], ToSymbol: byName["Balance"], Kind: core.EdgeDirect},
			{FromService: "svc", FromSymbol: byName["TestBalance"], ToSymbol: byName["Balance"], Kind: core.EdgeDirect},
		},
		governed: map[string]semantic.Obligation{
			"ledger.go": {Rule: "validate accountID before use", Src: "internal/CLAUDE.md#L5-L6"},
		},
	}
}

// localizeCall is a small round-trip helper: calls "localize" with args
// and returns the decoded result payload.
func localizeCall(t *testing.T, srv *mcp.Server, args map[string]any) map[string]any {
	t.Helper()
	return callTool(t, srv, "localize", args)
}

func localizeCandidateFiles(t *testing.T, got map[string]any) []string {
	t.Helper()
	cands, ok := got["candidates"].([]any)
	if !ok {
		t.Fatalf("candidates missing or wrong type: %v (%T)", got["candidates"], got["candidates"])
	}
	var files []string
	for _, c := range cands {
		files = append(files, c.(map[string]any)["file"].(string))
	}
	return files
}

// ─── seeds from each input shape ───────────────────────────────────────────

func TestLocalize_StackTraceSeedsResolveAndExpand(t *testing.T) {
	be := localizeFixture(t)
	srv := mcp.NewServer(be)
	got := localizeCall(t, srv, map[string]any{
		"service": "svc", "stack_trace": "panic: at ledger.go:5 in Balance",
	})
	files := localizeCandidateFiles(t, got)
	if !containsStr(files, "ledger.go") {
		t.Errorf("expected ledger.go among candidates (the stack frame's own file), got %v", files)
	}
	// Graph proximity: Balance's callers (Handler, TestBalance) must show
	// up too, via the queued proximity hop.
	if !containsStr(files, "handler.go") || !containsStr(files, "handler_test.go") {
		t.Errorf("expected graph-proximity candidates handler.go and handler_test.go, got %v", files)
	}
}

func TestLocalize_FailingTestSeedsSymbolAndItsCallees(t *testing.T) {
	be := localizeFixture(t)
	srv := mcp.NewServer(be)
	got := localizeCall(t, srv, map[string]any{"service": "svc", "failing_test": "TestBalance"})
	files := localizeCandidateFiles(t, got)
	if !containsStr(files, "handler_test.go") {
		t.Errorf("expected handler_test.go (TestBalance's own file), got %v", files)
	}
	if !containsStr(files, "ledger.go") {
		t.Errorf("expected ledger.go (Balance, TestBalance's callee via graph proximity), got %v", files)
	}
}

// TestLocalize_FailingTestStripsRunnerFormatting is the review's own
// fix-first item: go test runner formatting around a failing test name
// must not change the candidate set — only the bare name matters.
func TestLocalize_FailingTestStripsRunnerFormatting(t *testing.T) {
	baseline := sortedUnique(localizeCandidateFiles(t, localizeCall(t, mcp.NewServer(localizeFixture(t)), map[string]any{
		"service": "svc", "failing_test": "TestBalance",
	})))

	shapes := []string{
		"--- FAIL: TestBalance (0.00s)",
		"=== RUN   TestBalance/sub",
		"FAIL: TestBalance",
		"TestBalance/subcase",
	}
	for _, shape := range shapes {
		t.Run(shape, func(t *testing.T) {
			got := sortedUnique(localizeCandidateFiles(t, localizeCall(t, mcp.NewServer(localizeFixture(t)), map[string]any{
				"service": "svc", "failing_test": shape,
			})))
			if !reflect.DeepEqual(got, baseline) {
				t.Errorf("candidate files for %q = %v, want the same set as bare TestBalance: %v", shape, got, baseline)
			}
		})
	}
}

func sortedUnique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func TestLocalize_ErrorSeedsFrameAndQuotedLiteral(t *testing.T) {
	be := localizeFixture(t)
	srv := mcp.NewServer(be)
	got := localizeCall(t, srv, map[string]any{
		"service": "svc", "error": `ledger: validation failed at ledger.go:5: "errMissingAccount"`,
	})
	files := localizeCandidateFiles(t, got)
	if !containsStr(files, "ledger.go") {
		t.Errorf("expected ledger.go (both the stack frame and the quoted literal name it), got %v", files)
	}
	// The literal hop must actually have searched and found something.
	hops, _ := got["hops"].([]any)
	sawSearch := false
	for _, h := range hops {
		row := h.(map[string]any)
		if row["tool"] == "search" && row["arg"] == "errMissingAccount" {
			sawSearch = true
			if added, _ := row["added"].(float64); added < 1 {
				t.Errorf("search hop for errMissingAccount added %v, want >= 1", row["added"])
			}
		}
	}
	if !sawSearch {
		t.Errorf("expected a logged search hop for the quoted literal, got hops=%v", hops)
	}
}

func TestLocalize_ConcernSeedsSignificantTerms(t *testing.T) {
	be := localizeFixture(t)
	srv := mcp.NewServer(be)
	got := localizeCall(t, srv, map[string]any{
		"service": "svc", "concern": "how does balance validation reject an empty account",
	})
	files := localizeCandidateFiles(t, got)
	if !containsStr(files, "ledger.go") {
		t.Errorf("expected ledger.go (the term \"balance\" matches Balance's own declaration), got %v", files)
	}
}

// ─── budget stops the loop ─────────────────────────────────────────────────

func TestLocalize_BudgetStopsTheLoop(t *testing.T) {
	be := localizeFixture(t)
	srv := mcp.NewServer(be)
	got := localizeCall(t, srv, map[string]any{
		"service": "svc", "stack_trace": "at ledger.go:5 in Balance",
		"budget": 1,
	})
	budget, ok := got["budget"].(map[string]any)
	if !ok {
		t.Fatalf("budget missing or wrong type: %v", got["budget"])
	}
	if allowed, _ := budget["allowed"].(float64); allowed != 1 {
		t.Errorf("budget.allowed = %v, want 1", budget["allowed"])
	}
	used, _ := budget["used"].(float64)
	if used > 1 {
		t.Errorf("budget.used = %v, must never exceed budget.allowed (1)", budget["used"])
	}
	hops, _ := got["hops"].([]any)
	if len(hops) > 1 {
		t.Errorf("expected at most 1 logged hop under budget 1, got %d: %v", len(hops), hops)
	}
}

func TestLocalize_BudgetDefaultAndClamp(t *testing.T) {
	be := localizeFixture(t)
	srv := mcp.NewServer(be)

	got := localizeCall(t, srv, map[string]any{"service": "svc", "concern": "nothing matches anything here"})
	if allowed, _ := got["budget"].(map[string]any)["allowed"].(float64); allowed != 12 {
		t.Errorf("default budget.allowed = %v, want 12", allowed)
	}

	got = localizeCall(t, srv, map[string]any{"service": "svc", "concern": "nothing matches anything here", "budget": 1000})
	if allowed, _ := got["budget"].(map[string]any)["allowed"].(float64); allowed != 40 {
		t.Errorf("clamped budget.allowed = %v, want 40 (the max)", allowed)
	}
}

// ─── the diagnosis object, never a bare list ───────────────────────────────

func TestLocalize_ResponseShape(t *testing.T) {
	be := localizeFixture(t)
	srv := mcp.NewServer(be)
	got := localizeCall(t, srv, map[string]any{"service": "svc", "stack_trace": "at ledger.go:5 in Balance"})

	for _, key := range []string{"candidates", "hops", "budget"} {
		if _, ok := got[key]; !ok {
			t.Errorf("response missing %q: %v", key, got)
		}
	}
	if got["index_commit"] != "commit-abc123" {
		t.Errorf("index_commit = %v, want commit-abc123", got["index_commit"])
	}
	cands := got["candidates"].([]any)
	if len(cands) == 0 {
		t.Fatal("expected at least one candidate")
	}
	first := cands[0].(map[string]any)
	for _, key := range []string{"file", "symbol", "score", "tier", "why"} {
		if _, ok := first[key]; !ok {
			t.Errorf("candidate missing %q: %v", key, first)
		}
	}
	why, ok := first["why"].([]any)
	if !ok || len(why) == 0 {
		t.Errorf("candidate %v carries no citation in why", first)
	}
	if tier, _ := first["tier"].(string); tier != "typed" && tier != "heuristic" {
		t.Errorf("candidate tier = %v, want typed or heuristic", first["tier"])
	}

	hops := got["hops"].([]any)
	if len(hops) == 0 {
		t.Fatal("expected at least one logged hop")
	}
	for _, h := range hops {
		row := h.(map[string]any)
		for _, key := range []string{"n", "tool", "arg", "added"} {
			if _, ok := row[key]; !ok {
				t.Errorf("hop missing %q: %v", key, row)
			}
		}
	}
	// A recency hop is logged even though nothing is wired for it (VB-26:
	// "else skip with a note").
	sawRecency := false
	for _, h := range hops {
		if h.(map[string]any)["tool"] == "recency" {
			sawRecency = true
		}
	}
	if !sawRecency {
		t.Error("expected a logged recency hop (skip-with-a-note)")
	}
}

// ─── ranked by graph proximity, recency and open obligations ───────────────

func TestLocalize_ObligationsBoostGovernedCandidate(t *testing.T) {
	be := localizeFixture(t)
	srv := mcp.NewServer(be)
	got := localizeCall(t, srv, map[string]any{"service": "svc", "stack_trace": "at ledger.go:5 in Balance"})

	cands := got["candidates"].([]any)
	var ledgerRow map[string]any
	for _, c := range cands {
		row := c.(map[string]any)
		if row["file"] == "ledger.go" && row["symbol"] == "Balance" {
			ledgerRow = row
		}
	}
	if ledgerRow == nil {
		t.Fatalf("expected a ledger.go/Balance candidate, got %v", cands)
	}
	why, _ := ledgerRow["why"].([]any)
	sawGoverned := false
	for _, w := range why {
		if s, _ := w.(string); len(s) >= 8 && s[:8] == "governed" {
			sawGoverned = true
		}
	}
	if !sawGoverned {
		t.Errorf("expected the governed ledger.go/Balance candidate to carry a \"governed: ...\" citation, why=%v", why)
	}

	hops, _ := got["hops"].([]any)
	sawObligationsHop := false
	for _, h := range hops {
		row := h.(map[string]any)
		if row["tool"] == "obligations" && row["arg"] == "ledger.go" {
			sawObligationsHop = true
			if added, _ := row["added"].(float64); added != 1 {
				t.Errorf("obligations hop for ledger.go added = %v, want 1", row["added"])
			}
		}
	}
	if !sawObligationsHop {
		t.Errorf("expected a logged obligations hop for ledger.go, hops=%v", hops)
	}
}

// ─── determinism: two runs byte-identical ──────────────────────────────────

func TestLocalize_Deterministic(t *testing.T) {
	be := localizeFixture(t)
	args := map[string]any{"service": "svc", "stack_trace": "at ledger.go:5 in Balance, see also Handler(x)"}

	srv1 := mcp.NewServer(be)
	got1 := localizeCall(t, srv1, args)
	srv2 := mcp.NewServer(be)
	got2 := localizeCall(t, srv2, args)

	raw1, err := json.Marshal(got1)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw2, err := json.Marshal(got2)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw1) != string(raw2) {
		t.Errorf("localize is not deterministic:\n%s\nvs\n%s", raw1, raw2)
	}
}

// ─── ADR-014 execute-and-flag: an unindexed service, never a silent empty ──

func TestLocalize_UnknownServiceReturnsEmptyDiagnosisWithNote(t *testing.T) {
	be := localizeFixture(t)
	be.notIndexed = true
	srv := mcp.NewServer(be)
	got := localizeCall(t, srv, map[string]any{"service": "nope", "concern": "anything at all"})

	cands, ok := got["candidates"].([]any)
	if !ok || len(cands) != 0 {
		t.Errorf("expected an empty candidates array, got %v (%T)", got["candidates"], got["candidates"])
	}
	hops, ok := got["hops"].([]any)
	if !ok || len(hops) != 0 {
		t.Errorf("expected no hops attempted against an unindexed service, got %v", got["hops"])
	}
	note, _ := got["note"].(string)
	if note == "" {
		t.Fatal("expected a note naming the unindexed service, got none")
	}
	if !strings.Contains(note, "nope") {
		t.Errorf("note = %q, want it to name the unindexed service %q", note, "nope")
	}
}

// ─── input validation ───────────────────────────────────────────────────────

func TestLocalize_ExactlyOneInputRequired(t *testing.T) {
	be := localizeFixture(t)
	srv := mcp.NewServer(be)

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "localize", "arguments": map[string]any{"service": "svc"}},
	})
	if resp["error"] == nil {
		t.Error("expected an error when none of stack_trace/failing_test/error/concern is given")
	}

	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{"name": "localize", "arguments": map[string]any{
			"service": "svc", "stack_trace": "x.go:1", "concern": "y",
		}},
	})
	if resp["error"] == nil {
		t.Error("expected an error when more than one input shape is given")
	}
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
