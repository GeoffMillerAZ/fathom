package mcp_test

// obligations_claimid_test.go — SR-61 (surface reckoning F-27/U4, design §6)
// cut-back: the claim id is a 64-hex-character identity, and putting it on
// every rule of every obligations call measured +10-13% bytes on the L1
// replay (internal/indexbench testdata/l1-replay.json) — paid by every
// connected agent for a field only the edit hook reads. The id now rides
// only on the edit-hook path (origin "edit-hook"); the agent-facing payload
// is byte-identical to a payload that never carried the field, so the
// per-call cost of claim_id to an agent is 0% (the row's bar is <= 3%).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

const claimIDFixture = "9f2b6c1d0e7a4b58a3c1d2e4f60718293a4b5c6d7e8f90112233445566778899"

func claimIDRules(withIDs bool) []semantic.Obligation {
	rules := []semantic.Obligation{
		{Rule: "Register every extractor in fusion.New.", Src: "CLAUDE.md#L5-L5", Tier: "doc-claimed", Layer: "obligations", ClaimID: claimIDFixture},
		{Rule: "Determinism is a contract.", Src: "CLAUDE.md#L6-L6", Tier: "doc-claimed", Layer: "obligations", ClaimID: claimIDFixture[:32] + strings.Repeat("a", 32)},
	}
	if !withIDs {
		for i := range rules {
			rules[i].ClaimID = ""
		}
	}
	return rules
}

// obligationsText calls the obligations tool through the real dispatcher
// under ctx and returns the text block the caller reads.
func obligationsText(t *testing.T, ctx context.Context, srv *mcp.Server) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "obligations", "arguments": map[string]any{
			"service": "svc", "file": "internal/extractor/goextract.go", "include_snippets": false,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := srv.HandleRequest(ctx, raw)
	if !ok {
		t.Fatal("no response")
	}
	var env struct {
		Error  json.RawMessage `json:"error"`
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Error) > 0 || len(env.Result.Content) == 0 {
		t.Fatalf("tool error or empty result: %s", resp)
	}
	return env.Result.Content[0].Text
}

func TestObligationsClaimIDAddsNothingToAgentPayload(t *testing.T) {
	with, _ := snippetServer(t, claimIDRules(true))
	without, _ := snippetServer(t, claimIDRules(false))

	got := obligationsText(t, context.Background(), with)
	base := obligationsText(t, context.Background(), without)
	if strings.Contains(got, "claim_id") || strings.Contains(got, claimIDFixture) {
		t.Fatalf("the agent-facing obligations payload carries a claim id:\n%s", got)
	}
	// The row's bar: claim_id adds <= 3% bytes per call. It adds none.
	if over := float64(len(got))/float64(len(base)) - 1; over > 0.03 {
		t.Errorf("claim_id inflates the agent payload by %.1f%% (%d vs %d bytes), budget 3%%", 100*over, len(got), len(base))
	}
	if got != base {
		t.Errorf("agent payload differs when the backend knows claim ids:\n got: %s\nbase: %s", got, base)
	}
}

func TestObligationsClaimIDRidesTheEditHookPath(t *testing.T) {
	srv, _ := snippetServer(t, claimIDRules(true))
	ctx := mcp.WithOrigin(context.Background(), mcp.OriginEditHook)

	var payload struct {
		Rules []struct {
			ClaimID string `json:"claim_id"`
		} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(obligationsText(t, ctx, srv)), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(payload.Rules))
	}
	if payload.Rules[0].ClaimID != claimIDFixture {
		t.Errorf("edit-hook rules[0].claim_id = %q, want the full claim id %q", payload.Rules[0].ClaimID, claimIDFixture)
	}
	if payload.Rules[1].ClaimID == "" {
		t.Error("edit-hook rules[1].claim_id is empty; the hook keys every served rule by it")
	}
}

func TestObligationsClaimIDOtherHooksDoNotGetIt(t *testing.T) {
	// The hook origins that never read the id (the session primer, the
	// prompt and stop hooks) are agent-shaped: only the edit hook pays.
	srv, _ := snippetServer(t, claimIDRules(true))
	for _, origin := range []string{"session-hook", "prompt-hook", "prime", "doctor", "console"} {
		got := obligationsText(t, mcp.WithOrigin(context.Background(), origin), srv)
		if strings.Contains(got, "claim_id") {
			t.Errorf("origin %q was served claim ids:\n%s", origin, got)
		}
	}
}
