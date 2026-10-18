package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// bigVerifyPlan is an n-row Markdown plan over newVerifyBackend's symbols:
// passing, failing and absent rows, so every verdict carries criteria.
func bigVerifyPlan(n int) string {
	syms := []string{"DoWork", "Untested", "Ghost", "Conflicted"}
	var b strings.Builder
	b.WriteString("# Big plan\n\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "- [x] row %d of a plan as long as language-breadth {id: R-%04d, symbol: %s}\n", i, i, syms[i%len(syms)])
	}
	return b.String()
}

// verifyPlanAnswer is one verify_plan tools/call under ctx: the text
// block's bytes and its decoded payload.
func verifyPlanAnswer(t *testing.T, srv *mcp.Server, ctx context.Context, md string) (string, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "verify_plan", "arguments": map[string]any{"service": "svc", "plan_markdown": md}},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := srv.HandleRequest(ctx, raw)
	if !ok {
		t.Fatal("no response")
	}
	var env struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(resp, &env); err != nil || env.Error != nil || len(env.Result.Content) == 0 {
		t.Fatalf("verify_plan answer = %.300s (err %v)", resp, err)
	}
	text := env.Result.Content[0].Text
	var p map[string]any
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		t.Fatal(err)
	}
	return text, p
}

// TestVerifyPlan_DeclaredBudgetAnswersWholePlan (O1014, O1169): a machine
// caller that declares a dispatch budget (X-Fathom-Max-Bytes, capped at
// MaxDeclaredBudget) gets a 768-row plan's every verdict in one answer;
// without the declaration the answer is cut to the agent default and says
// so, as before.
func TestVerifyPlan_DeclaredBudgetAnswersWholePlan(t *testing.T) {
	const rows = 800
	srv := mcp.NewServer(newVerifyBackend())
	md := bigVerifyPlan(rows)

	_, whole := verifyPlanAnswer(t, srv, mcp.WithDeclaredBudget(context.Background(), 1<<20), md)
	if n := len(whole["tasks"].([]any)); n != rows || whole["elided"] != nil {
		t.Fatalf("declared 1 MiB: %d of %d rows, elided %v; want the whole plan in one answer", n, rows, whole["elided"])
	}

	_, cut := verifyPlanAnswer(t, srv, context.Background(), md)
	kept := len(cut["tasks"].([]any))
	el, _ := cut["elided"].(map[string]any)
	if kept >= rows || el == nil || int(el["count"].(float64)) != rows-kept {
		t.Fatalf("undeclared: %d of %d rows, elided %v; want the agent default's cut, disclosed", kept, rows, el)
	}

	// The cap is finite: a declaration past it is held to it.
	if got := mcp.DeclaredBudgetForTest(mcp.WithDeclaredBudget(context.Background(), 1<<30)); got != mcp.MaxDeclaredBudget {
		t.Errorf("declared 1 GiB reads as %d, want the %d cap", got, mcp.MaxDeclaredBudget)
	}
	for _, n := range []int{0, -5} {
		if got := mcp.DeclaredBudgetForTest(mcp.WithDeclaredBudget(context.Background(), n)); got != 0 {
			t.Errorf("declared %d reads as %d, want none", n, got)
		}
	}
}

// TestVerifyPlan_AgentSurfaceDefaultAnswerUnchanged pins the agent
// surfaces' default: with no declaration, /rpc/agent and /rpc/extended
// answer byte for byte the agent default's cut of the whole answer — the
// dispatch budget still protects an agent's context.
func TestVerifyPlan_AgentSurfaceDefaultAnswerUnchanged(t *testing.T) {
	srv := mcp.NewServer(newVerifyBackend())
	md := bigVerifyPlan(800)
	_, whole := verifyPlanAnswer(t, srv, mcp.WithDeclaredBudget(context.Background(), mcp.MaxDeclaredBudget), md)
	want, err := json.Marshal(mcp.BudgetedResultForTest(whole, mcp.DefaultCharBudgetForTest))
	if err != nil {
		t.Fatal(err)
	}
	for _, surface := range []string{"agent", "extended"} {
		got, _ := verifyPlanAnswer(t, srv, mcp.WithRequestScope(context.Background(), surface, true), md)
		if got != string(want) {
			t.Errorf("/rpc/%s default answer (%d B) differs from the agent default's cut (%d B)", surface, len(got), len(want))
		}
	}
}
