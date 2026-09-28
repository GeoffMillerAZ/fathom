package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// RT-172 review finding 4: a call the refuse rung cancelled answers with
// the named outcome — a JSON-RPC error whose data.outcome is
// "abandoned-by-governor" — and its call-log row records that outcome,
// never a normal-looking answer computed under a cancelled context.
func TestAbandonedByGovernorCallIsNamedInResponseAndLog(t *testing.T) {
	for _, logged := range []bool{false, true} {
		var buf bytes.Buffer
		var srv *mcp.Server
		backend := &extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}}
		if logged {
			srv = mcp.NewServerWithLogger(backend, mcp.NewCallLogger(&buf))
		} else {
			srv = mcp.NewServer(backend)
		}
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(tenancy.ErrAbandonedByGovernor)
		raw, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]any{"name": "repo_map", "arguments": map[string]any{"service": "svc"}},
		})
		resp, ok := srv.HandleRequest(ctx, raw)
		if !ok {
			t.Fatal("no response")
		}
		var got struct {
			Result any `json:"result"`
			Error  *struct {
				Message string         `json:"message"`
				Data    map[string]any `json:"data"`
			} `json:"error"`
		}
		if err := json.Unmarshal(resp, &got); err != nil {
			t.Fatal(err)
		}
		if got.Error == nil || got.Result != nil {
			t.Fatalf("logged=%v: abandoned call answered %s, want a JSON-RPC error", logged, resp)
		}
		if got.Error.Data["outcome"] != "abandoned-by-governor" || !strings.Contains(got.Error.Message, "abandoned-by-governor") {
			t.Fatalf("logged=%v: error %+v lacks the abandoned-by-governor outcome", logged, got.Error)
		}
		if logged {
			var row map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &row); err != nil {
				t.Fatalf("call log row: %v (%q)", err, buf.String())
			}
			if row["outcome"] != "abandoned-by-governor" {
				t.Fatalf("call log row %v lacks outcome abandoned-by-governor", row)
			}
		}
	}

	// A plain cancel (no governor cause) is not relabelled.
	srv := mcp.NewServer(&extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	raw, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": "repo_map", "arguments": map[string]any{"service": "svc"}},
	})
	resp, _ := srv.HandleRequest(ctx, raw)
	if strings.Contains(string(resp), "abandoned-by-governor") {
		t.Fatalf("a non-governor cancel was labelled abandoned-by-governor: %s", resp)
	}
}
