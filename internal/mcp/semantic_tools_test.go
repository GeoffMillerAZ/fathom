package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ─── extendedBackend wraps fakeBackend with optional semantic methods ─────────

// semanticFakeBackend adds obligationsBackend + compileBackend + claimsSummaryBackend
// to fakeBackend via composition, for semantic tool tests.
type semanticFakeBackend struct {
	fakeBackend
	obligations      semantic.Obligations
	obligationsErr   error
	indexStale       bool
	compileRun       core.CompileRun
	compileErr       error
	claimsSummary    map[string]any
	claimsSummaryErr error
	gotCompileLayer  string // records the layer name CompileLayer was called with

	// gotObligationsFile records the `file` arg the LAST Obligations call
	// carried — used to prove rootObligations() preferred RootObligations
	// (this fake implements rootObligationsBackend) over degrading to the
	// literal-probe fallback, which would have called Obligations with
	// rootObligationsProbe ("AGENTS.md").
	gotObligationsFile string

	// ClaimFeedback (D2, docs/plans/expert-depth.md): records the call and
	// returns feedbackErr.
	feedbackErr        error
	gotFeedbackService string
	gotFeedbackClaimID string
	gotFeedbackAction  string
	gotFeedbackNote    string
	gotFeedbackAuthor  string

	// ObligationsAsOf (SU-19, ADR-041 decision 7): records the resolved
	// as_of instant the last call carried.
	obligationsAsOf    semantic.Obligations
	obligationsAsOfErr error
	gotAsOf            time.Time
}

func (s *semanticFakeBackend) ObligationsAsOf(_ context.Context, _, _ string, _ int, asOf time.Time) (semantic.Obligations, error) {
	s.gotAsOf = asOf
	return s.obligationsAsOf, s.obligationsAsOfErr
}

func (s *semanticFakeBackend) ClaimFeedback(_ context.Context, service, claimID, action, note, author string) error {
	s.gotFeedbackService, s.gotFeedbackClaimID, s.gotFeedbackAction, s.gotFeedbackNote, s.gotFeedbackAuthor = service, claimID, action, note, author
	return s.feedbackErr
}

func (s *semanticFakeBackend) Obligations(_ context.Context, service, file string, _ int) (semantic.Obligations, error) {
	s.gotObligationsFile = file
	return s.obligations, s.obligationsErr
}

func (s *semanticFakeBackend) ObligationsIndexStale(_ context.Context, _, _ string) bool {
	return s.indexStale
}

// RootObligations makes semanticFakeBackend satisfy rootObligationsBackend
// too (every real backend does) — returns the same fixed s.obligations a
// file-scoped call would, so existing root-scope tests observe identical
// payloads regardless of which path served them.
func (s *semanticFakeBackend) RootObligations(_ context.Context, _ string, _ int) (semantic.Obligations, error) {
	return s.obligations, s.obligationsErr
}

func (s *semanticFakeBackend) CompileLayer(_ context.Context, _, layer string, dryRun bool) (core.CompileRun, error) {
	s.gotCompileLayer = layer
	run := s.compileRun
	run.DryRun = dryRun
	return run, s.compileErr
}

func (s *semanticFakeBackend) ClaimsSummary(_ context.Context, _ string) (map[string]any, error) {
	return s.claimsSummary, s.claimsSummaryErr
}

// ─── obligations: empty rules + not stale → compact payload ──────────────────

func TestObligationsTool_EmptyClean(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc", File: "pkg/foo.go"},
		indexStale:  false,
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "pkg/foo.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if v, _ := payload["index_stale"].(bool); v {
		t.Error("index_stale should be absent/false for a clean file")
	}
	if rules, ok := payload["rules"]; ok && rules != nil {
		t.Errorf("rules should be absent for zero-obligation file, got %v", rules)
	}
}

// ─── obligations: rules + stale flag ────────────────────────────────────────

func TestObligationsTool_WithRules(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service: "svc",
			File:    "internal/mcp/tools.go",
			Rules: []semantic.Obligation{
				{Rule: "Sort outputs.", Src: "CLAUDE.md#L10-L10", Tier: "doc-claimed"},
			},
			Watermark: "abc123",
		},
		indexStale: true,
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "internal/mcp/tools.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)

	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatalf("expected rules, got %v", payload["rules"])
	}
	if stale, _ := payload["index_stale"].(bool); !stale {
		t.Error("expected index_stale:true")
	}
	if wm, _ := payload["compiled_at_watermark"].(string); wm == "" {
		t.Error("expected compiled_at_watermark")
	}
}

// O109: renderObligationReceipt built its own response map and never read
// obl.RankedBy/obl.RankNote, and no rendered rule carried relevance — a
// live client saw none of these on the wire even though semantic.
// Obligations carried them correctly. This pins the fix directly against
// the map-building function (the fake backend hands renderObligationReceipt
// a pre-built Obligations exactly like a real backend would after DP-7's
// rerank ran).
func TestObligationsTool_RankedByAndRelevanceOnWire(t *testing.T) {
	relevance := 0.83
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service: "svc",
			File:    "internal/mcp/tools.go",
			Rules: []semantic.Obligation{
				{Rule: "Sort outputs.", Src: "CLAUDE.md#L10-L10", Tier: "doc-claimed", Relevance: &relevance},
			},
			RankedBy: "llm",
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "internal/mcp/tools.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if payload["ranked_by"] != "llm" {
		t.Errorf(`payload["ranked_by"] = %v, want "llm"`, payload["ranked_by"])
	}
	if _, present := payload["rank_note"]; present {
		t.Errorf("rank_note should be omitted when empty, got %v", payload["rank_note"])
	}
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %v", payload["rules"])
	}
	rule, _ := rules[0].(map[string]any)
	if got, _ := rule["relevance"].(float64); got != relevance {
		t.Errorf("rule relevance = %v, want %v", rule["relevance"], relevance)
	}
}

// TestObligationsTool_RankedByNoneAndRankNoteOnWire pins the OTHER two wire
// states DP-8 promises: ranked_by:"none" is always present (never simply
// absent — see semantic.Obligations.RankedBy's own doc comment), and
// rank_note surfaces when a configured Decider's own call failed.
func TestObligationsTool_RankedByNoneAndRankNoteOnWire(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service:  "svc",
			File:     "internal/mcp/tools.go",
			RankedBy: "none",
			RankNote: "decision: no provider configured",
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "internal/mcp/tools.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if payload["ranked_by"] != "none" {
		t.Errorf(`payload["ranked_by"] = %v, want "none" (always present, never absent)`, payload["ranked_by"])
	}
	if payload["rank_note"] != "decision: no provider configured" {
		t.Errorf(`payload["rank_note"] = %v, want the note`, payload["rank_note"])
	}
}

// ─── obligations: as_of (SU-19, ADR-041 decision 7) ──────────────────────────

func TestObligationsTool_AsOf(t *testing.T) {
	fake := &semanticFakeBackend{
		obligationsAsOf: semantic.Obligations{
			Service: "svc", File: "pkg/foo.go",
			Rules: []semantic.Obligation{{Rule: "Old rule.", Src: "CLAUDE.md#L1-L1", Tier: "doc-claimed"}},
			AsOf:  "2026-02-01T00:00:00Z",
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "pkg/foo.go", "as_of": "2026-02-01T00:00:00Z"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if got, _ := payload["as_of"].(string); got != "2026-02-01T00:00:00Z" {
		t.Errorf("as_of = %q, want the resolved RFC3339 instant", got)
	}
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("expected 1 rule from the as_of path, got %v", payload["rules"])
	}
	want := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if !fake.gotAsOf.Equal(want) {
		t.Errorf("backend saw asOf=%v, want %v", fake.gotAsOf, want)
	}
}

// TestObligationsTool_AsOf_RequiresFile pins that a root-scope as_of call is
// refused rather than silently answering the live root-scope question.
func TestObligationsTool_AsOf_RequiresFile(t *testing.T) {
	fake := &semanticFakeBackend{}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "as_of": "2026-02-01T00:00:00Z"},
		},
	})
	if resp["error"] == nil {
		t.Fatal("expected an error for a root-scope as_of call")
	}
}

// TestObligationsTool_AsOf_BackendUnsupported pins the degrade for a backend
// that omits ObligationsAsOf (fakeBackend alone).
func TestObligationsTool_AsOf_BackendUnsupported(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "pkg/foo.go", "as_of": "2026-02-01T00:00:00Z"},
		},
	})
	if resp["error"] == nil {
		t.Fatal("expected an error when the backend does not support as_of")
	}
}

// ─── obligations: `path` is an alias for `file` (wave P3 / O82) ──────────────

// TestObligationsTool_PathAliasForFile pins the defect an operator hit live:
// calling obligations with `path` instead of `file` silently served
// root-scope rules only (rootScope := p.File == "" read true because the
// caller's argument landed in a field the handler never looked at). `path`
// must resolve exactly like `file` — same rules, same non-root-scope
// behavior — never a silently different scope.
func TestObligationsTool_PathAliasForFile(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service: "svc",
			File:    "internal/mcp/tools.go",
			Rules: []semantic.Obligation{
				{Rule: "Sort outputs.", Src: "CLAUDE.md#L10-L10", Tier: "doc-claimed"},
			},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "path": "internal/mcp/tools.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if fake.gotObligationsFile != "internal/mcp/tools.go" {
		t.Fatalf("obligations(path=...) resolved file=%q, want the path argument forwarded — path must alias file", fake.gotObligationsFile)
	}
	payload := extractPayload(t, resp)
	if root, _ := payload["root"].(bool); root {
		t.Error("obligations(path=...) answered root:true — path was silently ignored and fell back to root-scope")
	}
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatalf("expected rules for path-addressed file, got %v", payload["rules"])
	}
}

// TestObligationsTool_FileWinsOverPath: when a caller (mistakenly) sends
// both, `file` is authoritative — `path` is purely a fallback alias, never
// an override.
func TestObligationsTool_FileWinsOverPath(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc", File: "a.go"},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "a.go", "path": "b.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if fake.gotObligationsFile != "a.go" {
		t.Fatalf("obligations(file=a.go, path=b.go) resolved file=%q, want %q (file wins)", fake.gotObligationsFile, "a.go")
	}
}

// ─── obligations: backend missing optional interface → ErrCodeInvalid ────────

func TestObligationsTool_BackendUnsupported(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "foo.go"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeInvalid) {
		t.Errorf("code = %v, want ErrCodeInvalid (%d)", code, mcp.ErrCodeInvalid)
	}
}

// ─── obligations: no file → root-scope answer (RC-22, docs/vision/28-receipts.md §3.5) ───

// TestObligationsTool_NoFile_ReturnsRootScope proves a fileless call is a
// documented shape, not "obligations: file required" — RC-22 replaces the
// error with root:true, a note steering a caller who wants file-scoped
// rules toward the file argument, and the same rules/watermark/displaced
// payload a file-scoped call gets (the fake ignores its file argument
// entirely, so this exercises the WIRING — no-file no longer errors and
// carries the root markers — not internal/semantic's own glob matching,
// which is out of the MCP tool's layer).
func TestObligationsTool_NoFile_ReturnsRootScope(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service: "svc",
			File:    "CLAUDE.md",
			Rules:   []semantic.Obligation{{Rule: "Sort outputs.", Src: "CLAUDE.md#L5-L5", Tier: "doc-claimed", Layer: "obligations"}},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 10, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error on a fileless call: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if v, _ := payload["root"].(bool); !v {
		t.Errorf("expected root:true, got %v", payload["root"])
	}
	if note, _ := payload["note"].(string); note == "" {
		t.Error("expected a non-empty note steering toward file for file-scoped rules")
	}
	if f, has := payload["file"]; has && f != "" {
		t.Errorf("expected no/empty file on a root-scope answer, got %v", f)
	}
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("expected the root-scope rules to come through, got %v", payload["rules"])
	}
}

// TestObligationsTool_NoFile_PrefersRootObligationsBackend proves the MCP
// wiring for the MAJOR finding: when the backend implements
// rootObligationsBackend (every real one does), rootObligations() must use
// it directly rather than degrading to the literal-probe fallback — the
// fallback would call Obligations with rootObligationsProbe ("AGENTS.md"),
// which this test asserts never happens.
func TestObligationsTool_NoFile_PrefersRootObligationsBackend(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Rules: []semantic.Obligation{{Rule: "Repo-wide code rule.", Src: "CLAUDE.md#L1-L1", Tier: "doc-claimed", Layer: "obligations"}},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 12, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if fake.gotObligationsFile != "" {
		t.Errorf("Obligations called with file %q — rootObligations should have preferred RootObligations, never the literal-probe fallback, for a backend that implements it", fake.gotObligationsFile)
	}
	payload := extractPayload(t, resp)
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("expected the RootObligations rule to come through, got %v", payload["rules"])
	}
}

// probeOnlyBackend implements obligationsBackend but deliberately NOT
// rootObligationsBackend — proves rootObligations() still degrades to the
// literal-probe fallback for a backend with no better answer (an older
// module, a minimal test double).
type probeOnlyBackend struct {
	fakeBackend
	obligations semantic.Obligations
	gotFile     string // records the `file` arg Obligations was called with
}

func (p *probeOnlyBackend) Obligations(_ context.Context, _, file string, _ int) (semantic.Obligations, error) {
	p.gotFile = file
	return p.obligations, nil
}

func (p *probeOnlyBackend) ObligationsIndexStale(_ context.Context, _, _ string) bool { return false }

// TestObligationsTool_NoFile_DegradesToProbeWithoutRootObligationsBackend
// proves the fallback branch itself: a backend lacking rootObligationsBackend
// still gets a root-scope answer, via the literal probe path.
func TestObligationsTool_NoFile_DegradesToProbeWithoutRootObligationsBackend(t *testing.T) {
	const wantProbe = "AGENTS.md" // mcp.rootObligationsProbe's documented value (ADR-059)
	fake := &probeOnlyBackend{obligations: semantic.Obligations{Service: "svc", File: wantProbe}}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 13, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if fake.gotFile != wantProbe {
		t.Errorf("Obligations called with file %q, want the literal probe path %q", fake.gotFile, wantProbe)
	}
	payload := extractPayload(t, resp)
	if v, _ := payload["root"].(bool); !v {
		t.Errorf("expected root:true, got %v", payload["root"])
	}
}

// TestObligationsTool_NoFileNoAction_BackendUnsupported keeps the existing
// "backend does not serve semantic layers" refusal for a fileless call too
// — RC-22 documents the shape, it does not require an obligationsBackend
// that was never there.
func TestObligationsTool_NoFileNoAction_BackendUnsupported(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 11, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeInvalid) {
		t.Errorf("code = %v, want ErrCodeInvalid (%d)", code, mcp.ErrCodeInvalid)
	}
}

// ─── obligations: feedback action (D2, docs/plans/expert-depth.md) ───────────
// The obligations tool doubles as the refute/corroborate write path — the
// already-hot channel, not a new tool.

func TestObligationsFeedback_Refute(t *testing.T) {
	fake := &semanticFakeBackend{}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 10, Method: "tools/call",
		Params: map[string]any{
			"name": "obligations",
			"arguments": map[string]any{
				"service": "svc", "action": "refute", "claim_id": "cl-1",
				"note": "already fixed upstream", "author": "alice",
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if payload["claim_id"] != "cl-1" || payload["action"] != "refute" {
		t.Errorf("payload = %v", payload)
	}
	if fake.gotFeedbackService != "svc" || fake.gotFeedbackClaimID != "cl-1" ||
		fake.gotFeedbackAction != "refute" || fake.gotFeedbackNote != "already fixed upstream" ||
		fake.gotFeedbackAuthor != "alice" {
		t.Errorf("backend not called with expected args: %+v", fake)
	}
}

func TestObligationsFeedback_Corroborate(t *testing.T) {
	fake := &semanticFakeBackend{}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 11, Method: "tools/call",
		Params: map[string]any{
			"name": "obligations",
			"arguments": map[string]any{
				"service": "svc", "action": "corroborate", "claim_id": "cl-2",
				"note": "confirmed in review",
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if fake.gotFeedbackAction != "corroborate" {
		t.Errorf("action = %q, want corroborate", fake.gotFeedbackAction)
	}
}

func TestObligationsFeedback_InvalidAction(t *testing.T) {
	fake := &semanticFakeBackend{}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 12, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "action": "delete", "claim_id": "cl-3", "note": "x"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want ErrCodeParams (%d)", code, mcp.ErrCodeParams)
	}
}

func TestObligationsFeedback_MissingClaimID(t *testing.T) {
	fake := &semanticFakeBackend{}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 13, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "action": "refute", "note": "x"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want ErrCodeParams (%d)", code, mcp.ErrCodeParams)
	}
}

func TestObligationsFeedback_MissingNote(t *testing.T) {
	fake := &semanticFakeBackend{}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 14, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "action": "refute", "claim_id": "cl-4"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want ErrCodeParams (%d)", code, mcp.ErrCodeParams)
	}
}

func TestObligationsFeedback_BackendUnsupported(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 15, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "action": "refute", "claim_id": "cl-5", "note": "x"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeInvalid) {
		t.Errorf("code = %v, want ErrCodeInvalid (%d)", code, mcp.ErrCodeInvalid)
	}
}

func TestObligationsFeedback_BackendError(t *testing.T) {
	fake := &semanticFakeBackend{feedbackErr: fmt.Errorf("claim %q not found", "cl-6")}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 16, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "action": "refute", "claim_id": "cl-6", "note": "x"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeInternal) {
		t.Errorf("code = %v, want ErrCodeInternal (%d)", code, mcp.ErrCodeInternal)
	}
}

// ─── obligations IS on companion surface ─────────────────────────────────────

func TestObligationsOnCompanionSurface(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "foo.go"},
		},
	})
	// Should dispatch (not surface-gated), even though this backend lacks the
	// optional interface (returns ErrCodeInvalid, not ErrCodeNotFound).
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		// No error means it dispatched fine (e.g. backed by semanticFakeBackend).
		return
	}
	if code := errObj["code"].(float64); code == float64(mcp.ErrCodeNotFound) {
		t.Errorf("obligations should be on companion surface but got surface-gate error: %v", errObj)
	}
}

// ─── compile_layer off the companion surface: execute-and-flag ────────────────

func TestCompileLayerNotOnCompanionSurface(t *testing.T) {
	fake := &semanticFakeBackend{
		compileRun: core.CompileRun{ServiceID: "svc", Layer: "obligations", SectionsTotal: 2, TokensIn: 500},
	}
	var buf bytes.Buffer
	srv := mcp.NewServerWithLogger(fake, mcp.NewCallLogger(&buf))
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)

	// compile_layer is off the companion allowlist (full surface only, per
	// docs/mcp-tools.md) but execute-and-flags rather than being rejected
	// (ADR-014 amendment): it runs, and the call log carries off_surface:true.
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("compile_layer off companion should execute-and-flag, got error: %v", resp["error"])
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry); err != nil {
		t.Fatalf("log line parse: %v (buf=%q)", err, buf.String())
	}
	if off, _ := entry["off_surface"].(bool); !off {
		t.Errorf("compile_layer off-surface call should log off_surface:true, got %v", entry)
	}

	// Hard gating stays available behind strictSurfaces for any future
	// multi-tenant shape.
	srv.SetStrictSurfaces(true)
	strict := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 6, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj, ok := strict["error"].(map[string]any)
	if !ok {
		t.Fatalf("compile_layer under strictSurfaces: want surface-gate error, got success")
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("compile_layer strictSurfaces gate: code = %v, want ErrCodeNotFound (%d)", code, mcp.ErrCodeNotFound)
	}
}

// ─── compile_layer: dry_run defaults true ────────────────────────────────────

func TestCompileLayerDryRunDefault(t *testing.T) {
	fake := &semanticFakeBackend{
		compileRun: core.CompileRun{
			ServiceID:     "svc",
			Layer:         "obligations",
			SectionsTotal: 2,
			TokensIn:      500,
			// F5: a budget-stopped run returns nil error — the tool result
			// must carry the stop, or it is indistinguishable from success.
			BudgetStopped: true,
			RefusalReason: "token total 700 exceeds layer \"obligations\" budget max_tokens_per_run=600",
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 6, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if dr, _ := payload["dry_run"].(bool); !dr {
		t.Errorf("dry_run should default true, got %v", payload["dry_run"])
	}
	if bs, _ := payload["budget_stopped"].(bool); !bs {
		t.Errorf("budget_stopped should surface true, got %v", payload["budget_stopped"])
	}
	if rr, _ := payload["refusal_reason"].(string); rr == "" {
		t.Errorf("refusal_reason should surface, got %v", payload["refusal_reason"])
	}
	if _, present := payload["refused"]; present {
		t.Errorf("refused should be omitted when false, got %v", payload["refused"])
	}
}

// TestCompileLayer_SectionsCarriedOnWire (AG-26): the compile_layer
// handler hand-copies named fields off core.CompileRun, so the carry counts
// are copied explicitly, and omitted when nothing carried.
func TestCompileLayer_SectionsCarriedOnWire(t *testing.T) {
	for _, tc := range []struct {
		carried, claims int
		present         bool
	}{{4, 9, true}, {0, 0, false}} {
		fake := &semanticFakeBackend{compileRun: core.CompileRun{
			ServiceID: "svc", Layer: "obligations", SectionsTotal: 5,
			SectionsCarried: tc.carried, ClaimsCarried: tc.claims,
		}}
		resp := roundtrip(t, mcp.NewServer(fake), request{
			JSONRPC: "2.0", ID: 7, Method: "tools/call",
			Params: map[string]any{"name": "compile_layer", "arguments": map[string]any{"service": "svc"}},
		})
		if resp["error"] != nil {
			t.Fatalf("unexpected error: %v", resp["error"])
		}
		payload := extractPayload(t, resp)
		sc, present := payload["sections_carried"]
		if present != tc.present {
			t.Errorf("carried=%d: sections_carried present = %v, want %v", tc.carried, present, tc.present)
		}
		if tc.present {
			if f, _ := sc.(float64); int(f) != tc.carried {
				t.Errorf("sections_carried = %v, want %d", sc, tc.carried)
			}
			if f, _ := payload["claims_carried"].(float64); int(f) != tc.claims {
				t.Errorf("claims_carried = %v, want %d", payload["claims_carried"], tc.claims)
			}
		}
	}
}

// TestCompileLayer_SectionsPrefilteredOnWire is O109/DP-6's wire pin: the
// compile_layer handler hand-copies named fields off core.CompileRun and
// had never copied SectionsPrefiltered/PrefilteredSections — a live
// --dry-run whose decision-port pre-filter genuinely ran and skipped
// sections reported nothing about it.
func TestCompileLayer_SectionsPrefilteredOnWire(t *testing.T) {
	fake := &semanticFakeBackend{
		compileRun: core.CompileRun{
			ServiceID:           "svc",
			Layer:               "obligations",
			SectionsTotal:       10,
			SectionsCompiled:    7,
			SectionsPrefiltered: 3,
			PrefilteredSections: []string{"docs/a.md#s1", "docs/a.md#s2", "docs/b.md#s1"},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 7, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if sp, _ := payload["sections_prefiltered"].(float64); sp != 3 {
		t.Errorf("sections_prefiltered = %v, want 3", payload["sections_prefiltered"])
	}
	names, ok := payload["prefiltered_sections"].([]any)
	if !ok || len(names) != 3 {
		t.Errorf("prefiltered_sections = %v, want 3 names", payload["prefiltered_sections"])
	}
}

// TestCompileLayer_SectionsPrefilteredOmittedWhenZero: a run with no
// Decider configured (or nothing skipped) stays byte-identical to before
// these fields existed.
func TestCompileLayer_SectionsPrefilteredOmittedWhenZero(t *testing.T) {
	fake := &semanticFakeBackend{
		compileRun: core.CompileRun{
			ServiceID:        "svc",
			Layer:            "obligations",
			SectionsTotal:    10,
			SectionsCompiled: 10,
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 8, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if _, present := payload["sections_prefiltered"]; present {
		t.Errorf("sections_prefiltered should be omitted when zero, got %v", payload["sections_prefiltered"])
	}
	if _, present := payload["prefiltered_sections"]; present {
		t.Errorf("prefiltered_sections should be omitted when empty, got %v", payload["prefiltered_sections"])
	}
}

// ─── compile_layer: wrong layer rejected ─────────────────────────────────────

// TestCompileLayerUnknownLayer asserts compile_layer surfaces the BACKEND's
// own rejection of an unknown layer name (LC-3, docs/plans/layer-catalog.md:
// the tool handler no longer pre-validates via the embedded-only
// semantic.LoadLayer — a project-local layer, rooted at the service's own
// path, is valid too, and only cb.CompileLayer can resolve that union). The
// fake backend simulates what pkg/client.CompileLayer really does for a name
// neither embedded nor present in any project's .fathom/layers/: return an
// error naming the layer.
func TestCompileLayerUnknownLayer(t *testing.T) {
	fake := &semanticFakeBackend{compileErr: fmt.Errorf("client.CompileLayer: load layer manifest: semantic: unknown layer %q", "hypotheses")}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 7, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc", "layer": "hypotheses"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for unknown layer, got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeInternal) {
		t.Errorf("unknown layer code = %v, want ErrCodeInternal (the backend's own resolution error)", code)
	}
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "hypotheses") {
		t.Errorf("error message = %q, want it to name the requested layer", msg)
	}
}

// ─── compile_layer: the requested layer name actually reaches the backend ────

// TestCompileLayerRoutesRequestedLayerName is the F7 regression (docs/plans/
// findings-tracer.md FT-1): compile_layer validated the requested layer name
// via semantic.LoadLayer but then discarded it, always compiling
// "obligations" regardless of what was asked for. The backend must now
// receive the exact layer the caller named.
func TestCompileLayerRoutesRequestedLayerName(t *testing.T) {
	fake := &semanticFakeBackend{compileRun: core.CompileRun{ServiceID: "svc", Layer: "findings"}}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc", "layer": "findings"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if fake.gotCompileLayer != "findings" {
		t.Errorf("backend received layer %q, want %q (F7 regression)", fake.gotCompileLayer, "findings")
	}
}

// ─── staleness_report appends layers when claimsSummaryBackend present ────────

func TestStalenessReportWithLayers(t *testing.T) {
	fake := &semanticFakeBackend{
		claimsSummary: map[string]any{
			"obligations": map[string]any{"fresh": 3},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 8, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("staleness_report error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	layers, ok := payload["layers"]
	if !ok {
		t.Fatalf("staleness_report: expected layers field, got %v", payload)
	}
	_ = layers
}

// ─── staleness_report: no layers when backend lacks interface ─────────────────

func TestStalenessReportWithoutLayers(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 9, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("staleness_report error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if _, ok := payload["layers"]; ok {
		t.Error("staleness_report: unexpected layers field on plain fakeBackend")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func extractPayload(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content missing: %v", result)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v (text=%q)", err, text)
	}
	return payload
}

// ─── regression O7: capabilities must survive the auto-refresh wrapper ───────

// TestSemanticTools_ThroughAutoRefreshWrapper exercises the semantic tools the
// way the DAEMON actually serves them — behind mcp.NewAutoRefresh. Interface
// embedding does not forward methods outside QueryInterface's method set, so
// without explicit forwards on the wrapper every semantic capability was
// silently stripped in production while direct-backend tests stayed green
// (and the edit-hook's silent-on-error contract hid it — dogfood O7).
func TestSemanticTools_ThroughAutoRefreshWrapper(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service: "svc", File: "pkg/foo.go",
			Rules: []semantic.Obligation{{Rule: "Sort outputs.", Src: "d.md#L1-L2", Tier: "doc-claimed"}},
		},
		compileRun:    core.CompileRun{ServiceID: "svc", Layer: "obligations", SectionsTotal: 3},
		claimsSummary: map[string]any{"obligations": map[string]any{"fresh": 1}},
	}
	srv := mcp.NewServer(mcp.NewAutoRefreshForTest(t, fake))

	// obligations
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "pkg/foo.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("obligations through wrapper: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if rules, ok := payload["rules"].([]any); !ok || len(rules) != 1 {
		t.Errorf("obligations rules through wrapper = %v", payload["rules"])
	}

	// compile_layer (dry run default)
	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("compile_layer through wrapper: %v", resp["error"])
	}

	// staleness_report layers section
	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("staleness_report through wrapper: %v", resp["error"])
	}
	payload = extractPayload(t, resp)
	if payload["layers"] == nil {
		t.Error("staleness_report through wrapper lost the layers summary")
	}
}

// ─── ingest tool: onboarding a new service by path ────────────────────────────

type ingestAtFake struct {
	fakeBackend
	gotService, gotPath string
}

func (f *ingestAtFake) IngestServiceAt(_ context.Context, service, path string) error {
	f.gotService, f.gotPath = service, path
	return nil
}

// TestIngestTool_WithPath_ThroughWrapper: the ingest tool's path argument must
// reach the backend through the auto-refresh wrapper (same O7 class as the
// semantic capabilities).
func TestIngestTool_WithPath_ThroughWrapper(t *testing.T) {
	fake := &ingestAtFake{}
	srv := mcp.NewServer(mcp.NewAutoRefreshForTest(t, fake))
	// IO-2: onboarding paths must exist and are canonicalized at the door,
	// so the fixture is a real directory (resolved through macOS's
	// /var→/private/var symlink to match what the handler forwards).
	dir := t.TempDir()
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": "newrepo", "path": dir},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("ingest with path through wrapper: %v", resp["error"])
	}
	if fake.gotService != "newrepo" || fake.gotPath != canonical {
		t.Errorf("backend got (%q, %q), want (newrepo, %q)", fake.gotService, fake.gotPath, canonical)
	}
}

// TestIngestTool_WithPath_Unsupported: a backend without the capability gets a
// clear error, not a silent re-ingest of the wrong directory.
func TestIngestTool_WithPath_Unsupported(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": "newrepo", "path": "/abs/corpus"},
		},
	})
	if resp["error"] == nil {
		t.Fatal("expected error for path-onboarding on an unsupported backend")
	}
}
