package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func reviewBudgetResult(t *testing.T, findings int) json.RawMessage {
	return reviewBudgetResultWithDetail(t, findings, true, 45)
}

func reviewBudgetResultWithCap(t *testing.T, findings int, capped bool) json.RawMessage {
	return reviewBudgetResultWithDetail(t, findings, capped, 1)
}

func reviewBudgetResultWithDetail(t *testing.T, findings int, capped bool, repeats int) json.RawMessage {
	t.Helper()
	rules := make([]semantic.Obligation, findings)
	for i := range rules {
		rules[i] = semantic.Obligation{Rule: fmt.Sprintf("rule %03d: %s", i, strings.Repeat("detail <&\"\n界 ", repeats)), Src: "pkg/a.go#L1-L1"}
	}
	backend := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
		obligations:         map[string]semantic.Obligations{"pkg/a.go": {Rules: rules}},
	}
	req, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if !capped {
		ctx = mcp.WithoutReviewBudgetForTest(ctx)
	}
	raw, ok := mcp.NewServer(backend).HandleRequest(ctx, req)
	if !ok {
		t.Fatal("no response")
	}
	var wire struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Error) != 0 {
		t.Fatalf("review_diff error: %s", wire.Error)
	}
	return wire.Result
}

func TestReviewDiffSmallResultByteIdentical(t *testing.T) {
	got := reviewBudgetResult(t, 0)
	if len(got) > 16000 {
		t.Fatalf("small fixture is %d bytes", len(got))
	}
	want := reviewBudgetResultWithCap(t, 0, false)
	if !bytes.Equal(got, want) {
		t.Fatal("small result differs with whole-result cap enabled")
	}
}

func TestReviewDiffWholeResultFitsBudget(t *testing.T) {
	// The pre-cap handler budget leaves this fixture intact, yet dispatch's
	// two copies exceed the whole-result cap. This size assertion is red if
	// the finalizer is reverted, rather than passing due to legacy elision.
	uncapped := reviewBudgetResultWithCap(t, 100, false)
	var legacy struct {
		Structured map[string]any `json:"structuredContent"`
	}
	if err := json.Unmarshal(uncapped, &legacy); err != nil {
		t.Fatal(err)
	}
	legacyPayload, _ := json.Marshal(legacy.Structured)
	legacyBudgeted, _ := json.Marshal(mcp.BudgetedResultForTest(legacy.Structured, 16000))
	if !bytes.Equal(legacyPayload, legacyBudgeted) {
		t.Fatal("fixture must survive the base handler budget unchanged")
	}
	if len(uncapped) <= 16000 {
		t.Fatalf("base fixture only %d bytes, want over 16000", len(uncapped))
	}
	raw := reviewBudgetResultWithCap(t, 100, true)
	t.Logf("uncapped/base result: %d bytes; compacted result: %d bytes", len(uncapped), len(raw))
	if len(raw) > 16000 {
		t.Fatalf("complete serialized MCP ToolResult = %d bytes, cap 16000", len(raw))
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Structured json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || !bytes.Equal([]byte(result.Content[0].Text), result.Structured) {
		t.Fatal("payload copies differ")
	}
	var p map[string]any
	if err := json.Unmarshal(result.Structured, &p); err != nil {
		t.Fatal(err)
	}
	statuses, ok := p["dimension_status"].(map[string]any)
	if !ok || len(statuses) != 8 {
		t.Fatalf("all eight dimension statuses required: %v", p["dimension_status"])
	}
	// Ground all final states against the same generated service's small result.
	var small struct {
		Structured map[string]any `json:"structuredContent"`
	}
	if err := json.Unmarshal(reviewBudgetResult(t, 0), &small); err != nil {
		t.Fatal(err)
	}
	for _, item := range small.Structured["dimensions"].([]any) {
		dim := item.(map[string]any)
		name := dim["name"].(string)
		status := statuses[name].(map[string]any)
		wantCount := dim["count"]
		if name == "obligations" {
			wantCount = float64(100)
		}
		if status["checked"] != dim["checked"] || status["count"] != wantCount {
			t.Errorf("%s status = %v, want checked %v count %v", name, status, dim["checked"], wantCount)
		}
		if dim["checked"] == false && status["unchecked_reason"] != dim["reason"] {
			t.Errorf("%s unchecked reason lost", name)
		}
	}
	if len(raw) < 12000 {
		t.Errorf("compacted 100-finding result wastes budget: %d bytes", len(raw))
	}
	if receipt, ok := p["receipt"].(map[string]any); !ok || receipt["band"] == nil {
		t.Error("receipt.band lost")
	}
	for i, value := range legacy.Structured["dimensions"].([]any) {
		dim := value.(map[string]any)
		original := dim["findings"].([]any)
		if len(original) == 0 {
			continue
		}
		kept := p["dimensions"].([]any)[i].(map[string]any)["findings"].([]any)
		if len(kept) == 0 {
			t.Errorf("%s lost first finding", dim["name"])
			continue
		}
		a, _ := json.Marshal(original[0])
		b, _ := json.Marshal(kept[0])
		if !bytes.Equal(a, b) {
			t.Errorf("%s first finding changed", dim["name"])
		}
	}
	if p["details_complete"] != false || p["elided"] == nil {
		t.Fatal("missing explicit omissions")
	}
	fields := p["elided"].(map[string]any)["fields"].(map[string]any)
	omission := fields["/dimensions/obligations/findings"].(map[string]any)
	if omission["total"] != float64(100) || omission["returned"] != float64(len(p["dimensions"].([]any)[1].(map[string]any)["findings"].([]any))) || omission["unit"] != "items" {
		t.Fatalf("original findings omission = %v", omission)
	}
	if next := reviewBudgetResultWithCap(t, 100, true); !bytes.Equal(raw, next) {
		t.Fatal("compaction is nondeterministic")
	}
}

// Keep the original round-one long-message corpus as an additional control.
func TestReviewDiffWholeResultLongFindings(t *testing.T) {
	raw := reviewBudgetResult(t, 100)
	if len(raw) > 16000 {
		t.Fatalf("whole result %d bytes exceeds cap", len(raw))
	}
	var result struct {
		Structured struct {
			Statuses map[string]struct {
				Count int `json:"count"`
			} `json:"dimension_status"`
			Elided struct {
				Fields map[string]struct {
					Total int `json:"total"`
				} `json:"fields"`
			} `json:"elided"`
		} `json:"structuredContent"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Structured.Statuses) != 8 || result.Structured.Statuses["obligations"].Count != 100 {
		t.Fatal("round-one statuses/counts lost")
	}
	if result.Structured.Elided.Fields["/dimensions/obligations/findings"].Total != 100 {
		t.Fatal("round-one original count lost")
	}
}

func TestReviewDiffBudgetKeepsOrNamesExclusions(t *testing.T) {
	b := reviewRound3Fixture(t)
	b.sources["generated.lock"] = append([]byte("// Code generated. DO NOT EDIT.\n"), bytes.Repeat([]byte("x"), 3<<20)...)
	rules := make([]semantic.Obligation, 100)
	for i := range rules {
		rules[i] = semantic.Obligation{Rule: fmt.Sprintf("rule %03d: %s", i, strings.Repeat("detail <&\"\n界 ", 45)), Src: "spec.md#L1-L1"}
	}
	b.obligations = map[string]semantic.Obligations{"spec.md": {Rules: rules}}
	req, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc", "changed_files": []string{"spec.md", "generated.lock"}}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := mcp.NewServer(b).HandleRequest(context.Background(), req)
	if !ok {
		t.Fatal("no response")
	}
	var wire struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil || len(wire.Error) != 0 {
		t.Fatalf("review_diff failed: %v %s", err, wire.Error)
	}
	if len(wire.Result) > 16000 {
		t.Fatalf("serialized ToolResult = %d bytes, cap 16000", len(wire.Result))
	}
	var result struct {
		Structured map[string]any `json:"structuredContent"`
	}
	if err := json.Unmarshal(wire.Result, &result); err != nil {
		t.Fatal(err)
	}
	p := result.Structured
	if p["details_complete"] != false {
		t.Fatalf("fixture did not exceed the cap (result %d bytes)", len(wire.Result))
	}
	if list, _ := p["excluded"].([]any); len(list) > 0 {
		for _, e := range list {
			if m, _ := e.(map[string]any); m["path"] == "generated.lock" {
				return
			}
		}
		t.Fatalf("excluded present without generated.lock: %v", list)
	}
	elided, _ := p["elided"].(map[string]any)
	fields, _ := elided["fields"].(map[string]any)
	if _, named := fields["/excluded"]; !named {
		t.Fatalf("excluded neither kept nor named in elision: %v", p["elided"])
	}
}
