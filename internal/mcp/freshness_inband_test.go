package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// O1449 (2): the refresh disclosure rides inside the result an agent reads
// (the JSON text and structuredContent), not only in the out-of-band
// envelope key. The old key stays, so the envelope is backward compatible.
func TestToolResultCarriesFreshnessInBand(t *testing.T) {
	tenancy.ResetLifecycleStateForTest()
	tenancy.PublishLevel(tenancy.LevelShed)
	t.Cleanup(func() {
		tenancy.PublishLevel(tenancy.LevelNormal)
		tenancy.ResetLifecycleStateForTest()
	})
	fake := &refreshFake{path: "/repo", indexed: "old"}
	srv := NewServer(newAR(t, fake, "new"))
	params, _ := json.Marshal(map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "X"}})
	res, rpcErr := srv.handleToolsCallWithCtx(context.Background(), params)
	if rpcErr != nil {
		t.Fatalf("find_symbol: %v", rpcErr.Message)
	}
	env, _ := res.(map[string]any)
	if r, _ := env["refresh"].(map[string]string); !strings.HasPrefix(r["svc"], "deferred (shed)") {
		t.Fatalf("out-of-band refresh key lost: %v", env["refresh"])
	}
	content, _ := env["content"].([]any)
	if len(content) == 0 {
		t.Fatal("no content block")
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	var inText map[string]any
	if err := json.Unmarshal([]byte(text), &inText); err != nil {
		t.Fatalf("text block is not a JSON object: %v: %q", err, text)
	}
	fresh, _ := inText["freshness"].(map[string]any)
	if s, _ := fresh["svc"].(string); !strings.HasPrefix(s, "deferred (shed)") {
		t.Fatalf("text block freshness = %v, want svc deferred (shed)", inText["freshness"])
	}
	sc, _ := json.Marshal(env["structuredContent"])
	var inSC map[string]any
	if err := json.Unmarshal(sc, &inSC); err != nil || inSC["freshness"] == nil {
		t.Fatalf("structuredContent freshness missing: %s", sc)
	}
}
