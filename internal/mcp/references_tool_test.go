// Package mcp_test — TDD tests for the references MCP tool.
package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ─── referencesFakeBackend ────────────────────────────────────────────────────

// referencesFakeBackend supplies two services with real source content and a
// symbol whose name equals the query, so all three assertions can be driven
// from a single backend.
type referencesFakeBackend struct {
	extendedFakeBackend
	sourcesByService map[string][]core.SourceFile
}

func (r *referencesFakeBackend) Sources(_ context.Context, svc string) ([]core.SourceFile, error) {
	return r.sourcesByService[svc], nil
}

// compile-time check.
var _ mcp.QueryInterface = (*referencesFakeBackend)(nil)

// newReferencesSrv returns a Server backed by a two-service fake that contains
// the query token "FindMe" in multiple places.
func newReferencesSrv() (*mcp.Server, string) {
	const query = "FindMe"

	srcA1 := core.SourceFile{
		Path: "pkg/alpha.go",
		// Line 1: no match; Line 2: match; Line 3: match
		Content: []byte("package alpha\n\nfunc FindMe() {}\n// also FindMe here\n"),
		Lang:    "go",
	}
	srcA2 := core.SourceFile{
		Path:    "pkg/other.go",
		Content: []byte("package alpha\n// unrelated\n"),
		Lang:    "go",
	}
	srcB1 := core.SourceFile{
		Path:    "docs/guide.md",
		Content: []byte("# Guide\n\nSee FindMe for details.\n"),
		Lang:    "md",
	}

	be := &referencesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{
			nodes: []core.SymbolNode{
				{
					Service:  "svcA",
					Package:  "alpha",
					Name:     query, // exact match → definition
					Kind:     core.KindGoFunc,
					FilePath: "pkg/alpha.go",
					Line:     3,
					Tier:     core.TierVerified,
				},
				{
					Service:  "svcA",
					Package:  "alpha",
					Name:     "Other",
					Kind:     core.KindGoFunc,
					FilePath: "pkg/other.go",
					Line:     2,
					Tier:     core.TierVerified,
				},
			},
			services: []string{"svcA", "svcB"},
		},
		sourcesByService: map[string][]core.SourceFile{
			"svcA": {srcA1, srcA2},
			"svcB": {srcB1},
		},
	}

	return mcp.NewServer(be), query
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestReferences_OccurrencesAcrossServices(t *testing.T) {
	srv, query := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2000, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": query},
		},
	})
	p := xtraPayload(t, resp)

	occs, ok := p["occurrences"].([]any)
	if !ok {
		t.Fatalf("occurrences missing or wrong type: %v", p)
	}
	// svcA/pkg/alpha.go has 2 matches; svcB/docs/guide.md has 1 match → 3 total
	if len(occs) < 3 {
		t.Errorf("expected at least 3 occurrences across 2 services, got %d", len(occs))
	}

	// SB-1: tier and depth are ALWAYS lifted to the envelope (occurrences
	// are always pre-scan/depth 1); service is per-row here because the
	// search spans two services (not uniform), so it is NOT lifted.
	if tier, _ := p["occurrence_tier"].(string); tier != "pre-scan" {
		t.Errorf("occurrence_tier = %q, want pre-scan", tier)
	}
	if _, hasEnvelopeSvc := p["occurrence_service"]; hasEnvelopeSvc {
		t.Error("occurrence_service should not be set when occurrences span two services")
	}
	services := map[string]bool{}
	for _, raw := range occs {
		occ := raw.(map[string]any)
		svc, _ := occ["svc"].(string)
		if svc == "" {
			t.Error("occurrence missing per-row svc when the search spans services")
		}
		services[svc] = true
		if occ["loc"] == nil {
			t.Error("occurrence missing loc")
		}
	}
	if !services["svcA"] {
		t.Error("expected occurrences from svcA")
	}
	if !services["svcB"] {
		t.Error("expected occurrences from svcB")
	}
}

func TestReferences_DefinitionsEntry(t *testing.T) {
	srv, query := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2001, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": query},
		},
	})
	p := xtraPayload(t, resp)

	defs, ok := p["definitions"].([]any)
	if !ok {
		t.Fatalf("definitions missing or wrong type: %v", p)
	}
	if len(defs) == 0 {
		t.Fatal("expected at least one definition entry for exact-name match")
	}
	// The symbol "FindMe" in svcA must appear.
	found := false
	for _, raw := range defs {
		def := raw.(map[string]any)
		if def["symbol"] == query && def["service"] == "svcA" {
			found = true
			if def["file"] == nil {
				t.Error("definition missing file")
			}
			if def["tier"] == nil {
				t.Error("definition missing tier")
			}
		}
	}
	if !found {
		t.Errorf("definition for symbol %q in svcA not found in %v", query, defs)
	}
}

func TestReferences_MaxCap_Truncated(t *testing.T) {
	srv, query := newReferencesSrv()
	// Set max=1 so the first occurrence fills the cap.
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2002, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": query, "max": 1},
		},
	})
	p := xtraPayload(t, resp)

	occs, ok := p["occurrences"].([]any)
	if !ok {
		t.Fatalf("occurrences missing: %v", p)
	}
	if len(occs) > 1 {
		t.Errorf("expected at most 1 occurrence (cap=1), got %d", len(occs))
	}
	truncated, _ := p["truncated"].(bool)
	if !truncated {
		t.Error("expected truncated=true when cap is exceeded")
	}
}

func TestReferences_ServiceFilter(t *testing.T) {
	srv, query := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2003, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": query, "services": []string{"svcB"}},
		},
	})
	p := xtraPayload(t, resp)

	occs, ok := p["occurrences"].([]any)
	if !ok {
		t.Fatalf("occurrences missing: %v", p)
	}
	// SB-1: a single-service search lifts service to the envelope instead
	// of repeating it per row.
	if svc, _ := p["occurrence_service"].(string); svc != "svcB" {
		t.Errorf("occurrence_service = %q, want svcB", svc)
	}
	for _, raw := range occs {
		occ := raw.(map[string]any)
		if _, hasSvc := occ["svc"]; hasSvc {
			t.Errorf("expected no per-row svc when the search stays within one service, got %v", occ["svc"])
		}
		loc, _ := occ["loc"].(string)
		if !strings.HasPrefix(loc, "docs/guide.md:") {
			t.Errorf("expected only svcB (docs/guide.md) occurrences, got loc=%q", loc)
		}
	}
}

func TestReferences_MissingQuery(t *testing.T) {
	srv, _ := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2004, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("expected params error code, got %v", code)
	}
}

func TestReferences_BadRegex(t *testing.T) {
	srv, _ := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2005, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": "[invalid(", "regex": true},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("expected params error code for bad regex, got %v", code)
	}
}

func TestReferences_WordMatch(t *testing.T) {
	// "FindMe" should match but "FindMeNot" should not when word=true
	f := core.SourceFile{
		Path:    "pkg/x.go",
		Content: []byte("FindMe is here\nFindMeNot is also here\n"),
		Lang:    "go",
	}
	be := &referencesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{
			nodes:    nil,
			services: []string{"svc"},
		},
		sourcesByService: map[string][]core.SourceFile{
			"svc": {f},
		},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2006, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": "FindMe", "word": true},
		},
	})
	p := xtraPayload(t, resp)
	occs, ok := p["occurrences"].([]any)
	if !ok {
		t.Fatalf("occurrences missing: %v", p)
	}
	// Only line 1 should match.
	if len(occs) != 1 {
		t.Errorf("word match: expected 1 occurrence (FindMe), got %d: %v", len(occs), occs)
	}
	if len(occs) == 1 {
		occ := occs[0].(map[string]any)
		if loc, _ := occ["loc"].(string); loc != "pkg/x.go:1" {
			t.Errorf("expected loc pkg/x.go:1, got %v", occ["loc"])
		}
	}
}

// TestReferencesOccurrenceUnder100Bytes (SB-1, docs/plans/strengths-bench.md;
// O31/O32): one rendered occurrence — the compact wire shape
// renderReferenceOccurrence produces — never exceeds 100 marshaled bytes,
// even when the source line it cites is far longer than that.
func TestReferencesOccurrenceUnder100Bytes(t *testing.T) {
	longLine := "func (s *someRatherLongTypeName) SomeRatherLongMethodName(ctx context.Context, arg1 string, arg2 int) (result string, err error) {"
	f := core.SourceFile{
		Path:    "internal/analysis/references/references.go",
		Content: []byte(longLine + "\nFindMe\n"),
		Lang:    "go",
	}
	be := &referencesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		sourcesByService:    map[string][]core.SourceFile{"svc": {f}},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2020, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": "FindMe"},
		},
	})
	p := xtraPayload(t, resp)
	occs, ok := p["occurrences"].([]any)
	if !ok || len(occs) == 0 {
		t.Fatalf("occurrences missing or empty: %v", p)
	}
	for _, raw := range occs {
		b, err := json.Marshal(raw)
		if err != nil {
			t.Fatalf("marshal occurrence: %v", err)
		}
		if len(b) > 100 {
			t.Errorf("occurrence %s is %d bytes, want <= 100", b, len(b))
		}
	}
}

// TestReferences_RefusesNonSymbolQuery (SB-2, O31): a query that is not an
// identifier, a dotted path, or a declared regex is refused, naming the
// three accepted forms — the O31 junk query ("func (s *") that kept
// exploring must never reach the source scan again.
func TestReferences_RefusesNonSymbolQuery(t *testing.T) {
	srv, _ := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2021, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": "func (s *"},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("expected params error code, got %v", code)
	}
	msg, _ := errObj["message"].(string)
	for _, form := range []string{"identifier", "dotted path", "regular expression"} {
		if !strings.Contains(msg, form) {
			t.Errorf("refusal message %q does not name accepted form %q", msg, form)
		}
	}
}

// TestReferences_DottedPathAndRegexAccepted (SB-2): the other two accepted
// forms are never refused.
func TestReferences_DottedPathAndRegexAccepted(t *testing.T) {
	srv, _ := newReferencesSrv()
	for _, args := range []map[string]any{
		{"query": "pkg.Sym"},
		{"query": "Type.Method"},
		{"query": "Find.*", "regex": true},
	} {
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 2022, Method: "tools/call",
			Params: map[string]any{"name": "references", "arguments": args},
		})
		if resp["error"] != nil {
			t.Errorf("query %v unexpectedly refused: %v", args, resp["error"])
		}
	}
}

func TestReferences_InToolsList(t *testing.T) {
	srv, _ := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2010, Method: "tools/list", Params: map[string]any{},
	})
	if resp["error"] != nil {
		t.Fatalf("tools/list error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	tools := result["tools"].([]any)
	found := false
	for _, raw := range tools {
		if raw.(map[string]any)["name"] == "references" {
			found = true
			break
		}
	}
	if !found {
		t.Error("references tool not found in tools/list")
	}
}

// ─── SB-45: exact mode ────────────────────────────────────────────────────────

// TestReferencesExactModeDefault: a service whose typed pass produced edges
// defaults `exact` to true — occurrences the typed pass could not confirm
// (a syntax-only call, a doc mention) are dropped, and the response says so.
func TestReferencesExactModeDefault(t *testing.T) {
	srv := mcp.NewServer(newGraphBackend())
	payload := callTool(t, srv, "references", map[string]any{"query": "Target", "word": true})

	if payload["exact"] != true {
		t.Fatalf("exact = %v, want true (the service has typed edges)", payload["exact"])
	}
	if payload["exact_available"] != true {
		t.Errorf("exact_available = %v, want true — nothing here lacked typed reference sites", payload["exact_available"])
	}
	if _, ok := payload["degraded"]; ok {
		t.Errorf("degraded = %v, want absent — nothing degraded", payload["degraded"])
	}
	occs := payload["occurrences"].([]any)
	if len(occs) != 1 {
		t.Fatalf("occurrences = %v, want exactly the one typed-confirmed mention (b.go:12)", occs)
	}
	if loc := occs[0].(map[string]any)["loc"]; loc != "b.go:12" {
		t.Errorf("loc = %v, want b.go:12 (Direct2, typed edge to Target)", loc)
	}
	if c := occs[0].(map[string]any)["c"]; c != "r" {
		t.Errorf("c = %v, want r (resolved)", c)
	}
}

// TestReferencesExactFalseRestoresEveryOccurrence: exact: false turns the
// filter off entirely, restoring the pre-SB-45 answer.
func TestReferencesExactFalseRestoresEveryOccurrence(t *testing.T) {
	srv := mcp.NewServer(newGraphBackend())
	payload := callTool(t, srv, "references", map[string]any{"query": "Target", "word": true, "exact": false})
	if payload["exact"] != false {
		t.Errorf("exact = %v, want false", payload["exact"])
	}
	if _, ok := payload["exact_available"]; ok {
		t.Errorf("exact_available = %v, want absent when exact is false", payload["exact_available"])
	}
	occs := payload["occurrences"].([]any)
	if len(occs) != 3 {
		t.Fatalf("occurrences = %v, want all 3 mentions (exact filtering off)", occs)
	}
}

// TestReferencesExactIncludeHeuristicAddsBack: include_heuristic: true adds
// the syntax-only occurrences back beside the exact ones without turning
// exact mode off.
func TestReferencesExactIncludeHeuristicAddsBack(t *testing.T) {
	srv := mcp.NewServer(newGraphBackend())
	payload := callTool(t, srv, "references", map[string]any{"query": "Target", "word": true, "include_heuristic": true})
	if payload["exact"] != true {
		t.Errorf("exact = %v, want true — include_heuristic adds occurrences, it does not disable exact mode", payload["exact"])
	}
	occs := payload["occurrences"].([]any)
	if len(occs) != 3 {
		t.Fatalf("occurrences = %v, want all 3 mentions (include_heuristic restores them)", occs)
	}
}

// referencesExactMultiFake is two independent services for
// TestReferencesExactFallsBackForUnconfirmableService: svcA has a typed
// edge confirming its one mention; svcB has an occurrence of the same query
// but no typed edges and no indexed definition of it at all — the shape of
// "the store holds no typed reference sites for this symbol" (SB-45).
type referencesExactMultiFake struct {
	nodesByService   map[string][]core.SymbolNode
	sourcesByService map[string][]core.SourceFile
	typedByService   map[string][]core.Edge
}

func (f *referencesExactMultiFake) QuerySymbols(context.Context, core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (f *referencesExactMultiFake) FindCallers(context.Context, string, string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *referencesExactMultiFake) FindSymbol(context.Context, string, string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *referencesExactMultiFake) ListAllSymbols(_ context.Context, svc string) ([]core.SymbolNode, error) {
	return f.nodesByService[svc], nil
}

func (f *referencesExactMultiFake) Services(context.Context) ([]string, error) {
	return []string{"svcA", "svcB"}, nil
}

func (f *referencesExactMultiFake) Sources(_ context.Context, svc string) ([]core.SourceFile, error) {
	return f.sourcesByService[svc], nil
}
func (f *referencesExactMultiFake) IngestService(context.Context, string) error { return nil }
func (f *referencesExactMultiFake) ServiceMeta(context.Context, string) (string, string, bool, error) {
	return "", "", false, nil
}

func (f *referencesExactMultiFake) ListCommits(context.Context, string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

func (f *referencesExactMultiFake) TypedEdges(_ context.Context, svc string) ([]core.Edge, error) {
	return f.typedByService[svc], nil
}

var _ mcp.QueryInterface = (*referencesExactMultiFake)(nil)

func newReferencesExactMultiFake() *referencesExactMultiFake {
	targetA := core.SymbolNode{Service: "svcA", Package: "p", Name: "Target", Kind: core.KindGoFunc, FilePath: "p/target.go", Line: 3, Tier: core.TierVerified}
	callerA := core.SymbolNode{Service: "svcA", Package: "p", Name: "Caller", Kind: core.KindGoFunc, FilePath: "p/caller.go", Line: 3, Tier: core.TierVerified}
	return &referencesExactMultiFake{
		nodesByService: map[string][]core.SymbolNode{
			"svcA": {targetA, callerA},
			"svcB": {}, // no indexed definition of "Target" at all
		},
		sourcesByService: map[string][]core.SourceFile{
			"svcA": {{Path: "p/caller.go", Content: []byte("package p\n\nfunc Caller() {\n\tTarget()\n}\n")}},
			"svcB": {{Path: "docs/guide.md", Content: []byte("See Target for details.\n")}},
		},
		typedByService: map[string][]core.Edge{
			"svcA": {{FromService: "svcA", FromSymbol: callerA.Key(), ToSymbol: targetA.Key(), Kind: core.EdgeDirect}},
			// svcB: no typed edges at all.
		},
	}
}

// TestReferencesExactFallsBackForUnconfirmableService: exact defaults true
// because svcA has typed edges; svcB has none and no indexed definition of
// the query either, so it cannot be filtered — it falls back to its
// unfiltered occurrence, is named in degraded, and the response reports
// exact_available: false even though svcA answered at full precision.
func TestReferencesExactFallsBackForUnconfirmableService(t *testing.T) {
	srv := mcp.NewServer(newReferencesExactMultiFake())
	payload := callTool(t, srv, "references", map[string]any{"query": "Target", "word": true})

	if payload["exact"] != true {
		t.Fatalf("exact = %v, want true (svcA has typed edges)", payload["exact"])
	}
	if payload["exact_available"] != false {
		t.Errorf("exact_available = %v, want false — svcB had no typed reference sites", payload["exact_available"])
	}
	degraded, ok := payload["degraded"].([]any)
	if !ok || len(degraded) != 1 {
		t.Fatalf("degraded = %v, want exactly one entry for svcB", payload["degraded"])
	}
	d := degraded[0].(map[string]any)
	if d["service"] != "svcB" {
		t.Errorf("degraded[0].service = %v, want svcB", d["service"])
	}
	if reason, _ := d["reason"].(string); !strings.Contains(reason, "Target") {
		t.Errorf("degraded[0].reason = %q, want it to name the query", reason)
	}

	occs := payload["occurrences"].([]any)
	if len(occs) != 2 {
		t.Fatalf("occurrences = %v, want svcA's confirmed mention plus svcB's unfiltered fallback", occs)
	}
	var sawSvcAConfirmed, sawSvcBFallback bool
	for _, raw := range occs {
		o := raw.(map[string]any)
		switch o["loc"] {
		case "p/caller.go:4":
			sawSvcAConfirmed = o["c"] == "r"
		case "docs/guide.md:1":
			sawSvcBFallback = o["c"] == "h"
		}
	}
	if !sawSvcAConfirmed {
		t.Errorf("occurrences = %v, want svcA's p/caller.go:4 resolved", occs)
	}
	if !sawSvcBFallback {
		t.Errorf("occurrences = %v, want svcB's docs/guide.md:1 present (fallback) and heuristic", occs)
	}
}

// ─── O36: a service with an unreadable root must degrade, never abort ────────

// referencesUnreadableFake is a two-service backend where svcBad's
// discovery calls (ListAllSymbols, Sources) fail — the shape O36 reports:
// a stale service whose corpus root has vanished — and svcGood answers
// normally. references must still answer for svcGood, naming svcBad in
// degraded, instead of failing the whole call (ADR-014 execute-and-flag).
type referencesUnreadableFake struct {
	goodNodes   []core.SymbolNode
	goodSources []core.SourceFile
	listErr     error
	sourcesErr  error
}

func (f *referencesUnreadableFake) QuerySymbols(context.Context, core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (f *referencesUnreadableFake) FindCallers(context.Context, string, string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *referencesUnreadableFake) FindSymbol(context.Context, string, string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *referencesUnreadableFake) ListAllSymbols(_ context.Context, svc string) ([]core.SymbolNode, error) {
	if svc == "svcBad" {
		return nil, f.listErr
	}
	return f.goodNodes, nil
}

func (f *referencesUnreadableFake) Services(context.Context) ([]string, error) {
	return []string{"svcBad", "svcGood"}, nil
}

func (f *referencesUnreadableFake) Sources(_ context.Context, svc string) ([]core.SourceFile, error) {
	if svc == "svcBad" {
		return nil, f.sourcesErr
	}
	return f.goodSources, nil
}
func (f *referencesUnreadableFake) IngestService(context.Context, string) error { return nil }
func (f *referencesUnreadableFake) ServiceMeta(context.Context, string) (string, string, bool, error) {
	return "", "", false, nil
}

func (f *referencesUnreadableFake) ListCommits(context.Context, string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

func (f *referencesUnreadableFake) TypedEdges(context.Context, string) ([]core.Edge, error) {
	return nil, nil
}

var _ mcp.QueryInterface = (*referencesUnreadableFake)(nil)

// TestReferencesDegradesUnreadableServiceRoot is O36: `references` must not
// fail the whole fleet-wide call when one service's root has vanished — it
// skips that service, names it in `degraded`, and still answers for every
// other one.
func TestReferencesDegradesUnreadableServiceRoot(t *testing.T) {
	f := &referencesUnreadableFake{
		goodNodes:   []core.SymbolNode{{Service: "svcGood", Package: "p", Name: "HooksWired", Kind: core.KindGoFunc, FilePath: "p/hooks.go", Line: 1, Tier: core.TierVerified}},
		goodSources: []core.SourceFile{{Path: "p/hooks.go", Content: []byte("package p\n\nfunc HooksWired() {}\n")}},
		listErr:     fmt.Errorf(`sources demo: client.Sources discover "demo": lstat /scratchpad/adopt2/demo: no such file or directory`),
	}
	srv := mcp.NewServer(f)
	payload := callTool(t, srv, "references", map[string]any{"query": "HooksWired"})

	if _, hasErr := payload["error"]; hasErr {
		t.Fatalf("references returned an error for the whole call: %v", payload)
	}
	degraded, ok := payload["degraded"].([]any)
	if !ok || len(degraded) != 1 {
		t.Fatalf("degraded = %v, want exactly one entry for svcBad", payload["degraded"])
	}
	d := degraded[0].(map[string]any)
	if d["service"] != "svcBad" {
		t.Errorf("degraded[0].service = %v, want svcBad", d["service"])
	}
	if reason, _ := d["reason"].(string); !strings.Contains(reason, "no such file or directory") {
		t.Errorf("degraded[0].reason = %q, want the underlying error named", reason)
	}
	defs := payload["definitions"].([]any)
	if len(defs) != 1 || defs[0].(map[string]any)["symbol"] != "HooksWired" {
		t.Errorf("definitions = %v, want svcGood's HooksWired still answered", defs)
	}
}

// TestReferencesDegradesUnreadableSourcesOnly: ListAllSymbols succeeds but
// Sources fails for one service (the O36 repro's own shape) — same
// doctrine, occurrences degrade for that service without losing its
// definitions or any other service's answer.
func TestReferencesDegradesUnreadableSourcesOnly(t *testing.T) {
	f := &referencesUnreadableFake{
		goodNodes:   []core.SymbolNode{{Service: "svcGood", Package: "p", Name: "HooksWired", Kind: core.KindGoFunc, FilePath: "p/hooks.go", Line: 1, Tier: core.TierVerified}},
		goodSources: []core.SourceFile{{Path: "p/hooks.go", Content: []byte("package p\n\nfunc HooksWired() {}\n")}},
		sourcesErr:  fmt.Errorf(`client.Sources discover "svcBad": lstat /scratchpad/adopt2/demo: no such file or directory`),
	}
	srv := mcp.NewServer(f)
	payload := callTool(t, srv, "references", map[string]any{"query": "HooksWired"})

	if _, hasErr := payload["error"]; hasErr {
		t.Fatalf("references returned an error for the whole call: %v", payload)
	}
	degraded, ok := payload["degraded"].([]any)
	if !ok || len(degraded) != 1 || degraded[0].(map[string]any)["service"] != "svcBad" {
		t.Fatalf("degraded = %v, want exactly one entry for svcBad", payload["degraded"])
	}
	occs := payload["occurrences"].([]any)
	if len(occs) != 1 {
		t.Errorf("occurrences = %v, want svcGood's one occurrence", occs)
	}
}
