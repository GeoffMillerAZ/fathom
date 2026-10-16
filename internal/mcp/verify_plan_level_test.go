package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestVerifyPlanPartialNamesTenancyLevel pins EW-79 (O1073, I38): while the
// daemon is at shed or refuse the graph build may never finish, so
// verify_plan's deadline partial names the level and the budget knob
// instead of promising the build "is still building"; at normal the partial
// is unchanged.
func TestVerifyPlanPartialNamesTenancyLevel(t *testing.T) {
	gate := make(chan struct{})
	t.Cleanup(func() {
		close(gate)
		tenancy.PublishLevel(tenancy.LevelNormal)
	})
	b := &coldGraphBackend{
		storeID: "ew79-" + t.Name(),
		mark:    "w1",
		gate:    gate, // the typed edges never arrive while the test runs
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "work", Name: "Run", Kind: core.KindGoFunc, FilePath: "work/run.go", Line: 1, Tier: core.TierVerified},
		},
		typedReads: map[string]int{},
	}
	srv := NewServer(b)
	partial := func(t *testing.T) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
			t.Fatalf("verify_plan error instead of a partial: %+v", out.Error)
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
		return p
	}

	tenancy.PublishLevel(tenancy.LevelNormal)
	if p := partial(t); !strings.Contains(p, "still building") || strings.Contains(p, "tenancy level") {
		t.Errorf("level normal: the partial must name the build in flight and no level, got %q", p)
	}
	for _, lvl := range []tenancy.Level{tenancy.LevelShed, tenancy.LevelRefuse} {
		tenancy.PublishLevel(lvl)
		p := partial(t)
		if strings.Contains(p, "is still building") {
			t.Errorf("level %s: the partial promises a build that may never finish: %q", lvl, p)
		}
		if !strings.Contains(p, "tenancy level "+lvl.String()) || !strings.Contains(p, "FATHOM_TENANCY_BUDGET") || !strings.Contains(p, "[tenancy]") {
			t.Errorf("level %s: the partial must name the level and the budget knob, got %q", lvl, p)
		}
	}
}
