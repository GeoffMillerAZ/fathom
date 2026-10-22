package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func reviewBudgetResult(t *testing.T, findings int) json.RawMessage {
	t.Helper()
	rules := make([]semantic.Obligation, findings)
	for i := range rules {
		rules[i] = semantic.Obligation{Rule: fmt.Sprintf("rule %03d: %s", i, strings.Repeat("detail <&\"\n界 ", 45)), Src: "pkg/a.go#L1-L1"}
	}
	backend := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
		obligations:         map[string]semantic.Obligations{"pkg/a.go": {Rules: rules}},
	}
	req, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := mcp.NewServer(backend).HandleRequest(context.Background(), req)
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
	want, err := os.ReadFile("testdata/review_diff_small.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("small result differs from output captured at base")
	}
}

func TestReviewDiffWholeResultFitsBudget(t *testing.T) {
	raw := reviewBudgetResult(t, 100)
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
	if p["details_complete"] != false || p["elided"] == nil {
		t.Fatal("missing explicit omissions")
	}
	fields := p["elided"].(map[string]any)["fields"].(map[string]any)
	omission := fields["/dimensions/obligations/findings"].(map[string]any)
	if omission["total"] != float64(100) || omission["returned"] != float64(0) || omission["unit"] != "items" {
		t.Fatalf("original findings omission = %v", omission)
	}
	if next := reviewBudgetResult(t, 100); !bytes.Equal(raw, next) {
		t.Fatal("compaction is nondeterministic")
	}
}
