// decisionrerank_wire_test.go — DP-20/O109: a wire-level test for the
// obligations re-rank (DP-7/DP-8, ADR-042 decision 2). Every existing
// internal/mcp obligations test drives a hand-built fakeBackend that
// returns a canned semantic.Obligations{} directly — none of them ever
// exercises the REAL pkg/client.Client -> semantic.ObligationsForLayerOpts
// -> rerankWithinTiers path, so a break anywhere along that chain (or in
// how the wire renders its output) is invisible to the existing suite.
// This file builds the real Client over a real (in-memory) store, wires a
// decision.MockDecider through WithDecision exactly as cmd/fathom/
// servehttp.go does, and drives the obligations tool through the actual
// JSON-RPC surface (mcp.NewServer + roundtrip) — the same path a real
// client speaks over POST /rpc/agent. Two scenarios: the happy path (claims
// eligible, the Decider actually gets called) and O109's own unresolved
// case (an active Decider but nothing eligible in scope, where the rerank
// correctly never fires at all).
package mcp_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const rerankWireTestFile = "pkg/rerank/target.go"

// seedWireClaim puts one exact-file-tier, obligations-layer claim directly
// into st — the same minimal shape pkg/client's own semantic_test.go uses
// (PutClaims with no compile_runs row), so no Ingest/CompileLayer call is
// needed to exercise the serve-time rerank.
func seedWireClaim(t *testing.T, st store.Store, id, rule string) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
		Subject: "docs/g.md#" + id, Rule: rule, Scope: []string{rerankWireTestFile},
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: core.ClaimFresh,
		CompiledAt: time.Unix(1, 0).UTC(),
	}
	links := []core.ClaimLink{
		{ClaimID: id, Rel: core.RelCites, TargetRef: "docs/g.md#L1-L2", TargetHash: "h"},
		{ClaimID: id, Rel: core.RelScopes, TargetRef: rerankWireTestFile},
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seedWireClaim(%s): %v", id, err)
	}
}

// mockNoulResponse builds a decision.Response answering each id in answers
// with its given noul probability — mirrors internal/semantic/
// decisionrank_test.go's own helper of the same shape (unexported there,
// so this package needs its own copy).
func mockNoulResponse(provider string, answers map[string]float64) decision.Response {
	out := decision.Response{Provider: provider, Answers: make(map[string]decision.Answer, len(answers))}
	for id, noul := range answers {
		out.Answers[id] = decision.Answer{ID: id, Kind: decision.KindPredicate, Noul: noul}
	}
	return out
}

// TestObligationsWire_RerankReachesTheJSONRPCEnvelope is DP-20's
// reachable_from (O109): with a real Client (real store, real
// semantic.ObligationsForLayerOpts) wired to a decision.MockDecider via
// WithDecision — exactly as cmd/fathom/servehttp.go wires the daemon — and
// two eligible rules in the SAME governing-scope tier (both scoped to the
// same file, so rerankWithinTiers has something to permute), driving the
// obligations tool over the real JSON-RPC surface must: (1) actually call
// the Decider (MockDecider.Calls() non-empty); (2) the wire response must
// carry ranked_by: "mock", not "none"; (3) at least one served rule must
// carry relevance; (4) the decisions log (decide.Recorder) must receive a
// "site":"obligations" row. Before this test existed, DP-6/DP-8/DP-7's own
// reachable_from tests only asserted this against internal/semantic
// directly or against a fake MCP backend that never calls through to it —
// so all three could report [x] while a real client saw none of it
// (O109's live finding).
func TestObligationsWire_RerankReachesTheJSONRPCEnvelope(t *testing.T) {
	st := store.NewMemoryStore()
	seedWireClaim(t, st, "wire-1", "wire-1: requires review before any change.")
	seedWireClaim(t, st, "wire-2", "wire-2: requires review before any change too.")

	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	mock := decision.NewMockDecider("mock", decision.MockEntry{
		Response: mockNoulResponse("mock", map[string]float64{
			"wire-1": 0.55,
			"wire-2": 0.95,
		}),
	})
	var log bytes.Buffer
	rec := decide.NewRecorder(&log, func() time.Time { return time.Unix(2, 0).UTC() }, "test")
	c.WithDecision(mock, config.Decide{Provider: "llm", RankWithinTier: true, CompileThreshold: 0.5, Sites: []string{decide.SiteObligationsRerank}}, rec)

	srv := mcp.NewServer(c)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": rerankWireTestFile},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("obligations tool error: %v", resp["error"])
	}

	if calls := mock.Calls(); len(calls) == 0 {
		t.Error("MockDecider.Calls() is empty: the obligations tool never called the Decider over the real JSON-RPC path")
	}

	payload := extractPayload(t, resp)
	if payload["ranked_by"] != "mock" {
		t.Errorf(`payload["ranked_by"] = %v, want "mock"`, payload["ranked_by"])
	}

	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatalf("payload[\"rules\"] missing or empty: %v", payload)
	}
	sawRelevance := false
	for _, r := range rules {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := rm["relevance"]; ok {
			sawRelevance = true
		}
	}
	if !sawRelevance {
		t.Errorf("no served rule carries relevance on the wire: %v", rules)
	}

	if !strings.Contains(log.String(), `"site":"obligations"`) {
		t.Errorf("decisions log carries no obligations-site row: %s", log.String())
	}
}

// TestObligationsWire_NoEligibleClaimsNamesWhyOnWire is O109's own
// unresolved caveat, closed: the live probe against internal/semantic/
// serve.go and cmd/fathom/servehttp.go saw zero decisions logged and a
// byte-identical rule order between provider "llm" and "none", and could
// not tell "the rerank never fired for these two files" apart from "it
// fired and changed nothing" — because rerankWithinTiers is only reached
// AFTER matched is built, and obligationsCore returns early when matched
// is empty (nothing in scope to ask the Decider about at all). With an
// active Decider (RankWithinTier true, Decider non-nil) and NO claim
// seeded for the queried file, the mock must never be called (nothing to
// ask), ranked_by must still read "none" (no rerank occurred — RankedBy's
// own contract is unchanged), and rank_note must now name why, rather
// than looking identical to "no provider configured at all".
func TestObligationsWire_NoEligibleClaimsNamesWhyOnWire(t *testing.T) {
	st := store.NewMemoryStore() // no claims seeded at all

	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	mock := decision.NewMockDecider("mock")
	var log bytes.Buffer
	rec := decide.NewRecorder(&log, func() time.Time { return time.Unix(2, 0).UTC() }, "test")
	c.WithDecision(mock, config.Decide{Provider: "llm", RankWithinTier: true, CompileThreshold: 0.5, Sites: []string{decide.SiteObligationsRerank}}, rec)

	srv := mcp.NewServer(c)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": rerankWireTestFile},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("obligations tool error: %v", resp["error"])
	}

	if calls := mock.Calls(); len(calls) != 0 {
		t.Errorf("MockDecider.Calls() = %d, want 0 (nothing eligible to ask about)", len(calls))
	}

	payload := extractPayload(t, resp)
	if payload["ranked_by"] != "none" {
		t.Errorf(`payload["ranked_by"] = %v, want "none" (no rerank occurred)`, payload["ranked_by"])
	}
	if payload["rank_note"] != "no eligible compiled rules in scope to rerank" {
		t.Errorf(`payload["rank_note"] = %v, want a note explaining why ranked_by is "none"`, payload["rank_note"])
	}
	if strings.Contains(log.String(), `"site":"obligations"`) {
		t.Errorf("decisions log carries an obligations-site row despite nothing being asked: %s", log.String())
	}
}
