package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// TestNavLoadNoteNamesADeclinedBuild (RT-191, §5 "No doomed flights",
// I38): a graph-backed call whose deadline could not hold the build is
// told so — no build was started — never "context canceled" or an empty
// index.
func TestNavLoadNoteNamesADeclinedBuild(t *testing.T) {
	for _, err := range []error{memo.ErrWouldNotFinish, fmt.Errorf("product reach: %w", memo.ErrWouldNotFinish)} {
		note := navLoadNote("svc", err)
		if !strings.Contains(note, "would not finish inside this call's deadline") || !strings.Contains(note, "no build was started") {
			t.Errorf("navLoadNote(%v) = %q, want the declined build named", err, note)
		}
	}
	if note := navLoadNote("svc", context.Canceled); strings.Contains(note, "no build was started") {
		t.Errorf("a cancelled load reads as declined: %q", note)
	}
}

// TestVerifyPlanAnswersADeclinedBuildAsItsStagePartial (RT-191): a
// verify_plan whose deadline cannot hold the evidence stage's build
// answers at once with the stage-building partial, worded as a declined
// build (nothing keeps running), never an internal error and never the
// typed-edge graph fallback.
func TestVerifyPlanAnswersADeclinedBuildAsItsStagePartial(t *testing.T) {
	b := &coldGraphBackend{
		storeID: "rt191-" + t.Name(),
		mark:    "w1",
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "work", Name: "Run", Kind: core.KindGoFunc, FilePath: "work/run.go", Line: 1, Tier: core.TierVerified},
		},
		typedReads: map[string]int{},
	}
	srv := NewServer(b)
	// Under half the snapshot's expected cost, with no build of this
	// service measured yet: the stage's first flight is declined.
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "verify_plan", "arguments": map[string]any{
			"service": "svc",
			"tasks":   []any{map[string]any{"id": "R-1", "symbol": "Run", "claimed": true}},
		}}})
	resp, ok := srv.HandleRequest(ctx, raw)
	if !ok {
		t.Fatal("no response")
	}
	var out rpcResp
	if err := json.Unmarshal(resp, &out); err != nil {
		t.Fatalf("%v: %s", err, resp)
	}
	if out.Error != nil {
		t.Fatalf("a declined stage is an error instead of a partial: %+v", out.Error)
	}
	content, _ := out.Result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("no content: %+v", out.Result)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &m); err != nil {
		t.Fatal(err)
	}
	p, _ := m["partial"].(string)
	if !strings.Contains(p, "would not finish inside this call's deadline") || strings.Contains(p, "keeps running") {
		t.Errorf("partial = %q, want the declined build named and no build promised", p)
	}
	if tasks, _ := m["tasks"].([]any); len(tasks) != 0 {
		t.Errorf("rows verified without the stage: %+v", tasks)
	}
	b.mu.Lock()
	n := b.typedReads["w1"]
	b.mu.Unlock()
	if n != 0 {
		t.Errorf("the typed edges were read %d times for a declined stage", n)
	}
}
