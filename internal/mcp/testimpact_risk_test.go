package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestTestImpactRiskRank proves TQ-5 (docs/plans/test-intelligence.md;
// design doc §3 T1): test_impact with rank:"risk" serves the risk-weighted
// coverage list through the existing tool (no new tool) — no symbol
// needed, each row carrying its five cited factors, test code never
// ranked — while a plain call still requires a symbol and an unknown rank
// is refused.
func TestTestImpactRiskRank(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/risk\n\ngo 1.22\n")
	write("pkg/a.go", "package pkg\n\nfunc A() { B() }\n\nfunc B() { C() }\n\nfunc C() {}\n")
	write("pkg/a_test.go", "package pkg\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { A() }\n")
	write("docs/intent/incidents.md", "# Incidents\n\n### INC-1 2026-09-01 C broke\nWhat: pkg/a.go lost a call\n")

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := t.Context()
	if err := c.IngestServiceAt(ctx, "svc", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}
	srv := NewServer(c)

	payload := callTool(t, srv, "test_impact", map[string]any{"service": "svc", "rank": "risk", "limit": 5})
	risk, ok := payload["risk"].(map[string]any)
	if !ok {
		t.Fatalf("rank:risk payload has no risk block: %v", payload)
	}
	rows, _ := risk["rows"].([]any)
	if len(rows) == 0 {
		t.Fatalf("no risk rows: %v", risk)
	}
	if tests, ok := payload["tests"].([]any); !ok || len(tests) != 0 {
		t.Errorf("tests = %v, want present and empty on rank:risk", payload["tests"])
	}
	top := rows[0].(map[string]any)
	topSym, _ := top["symbol"].(string)
	if !strings.HasSuffix(topSym, "pkg.C") {
		t.Errorf("top risk symbol = %v, want pkg.C (most transitive callers)", topSym)
	}
	for _, r := range rows {
		row := r.(map[string]any)
		if sym, _ := row["symbol"].(string); strings.Contains(sym, "Test") {
			t.Errorf("test code ranked: %s", sym)
		}
		factors, _ := row["factors"].([]any)
		if len(factors) != 5 {
			t.Fatalf("%v: %d factors, want 5", row["symbol"], len(factors))
		}
		for _, f := range factors {
			if cite, _ := f.(map[string]any)["cite"].(string); cite == "" {
				t.Errorf("%v: factor without citation: %v", row["symbol"], f)
			}
		}
	}
	if s, _ := payload["summary"].(string); topSym == "" || !strings.Contains(s, topSym) {
		t.Errorf("summary = %q, want it to name the top symbol", s)
	}

	if _, rpcErr := callToolErr(srv, "test_impact", map[string]any{"service": "svc"}); rpcErr == "" {
		t.Error("plain test_impact without a symbol must still be refused")
	}
	if _, rpcErr := callToolErr(srv, "test_impact", map[string]any{"service": "svc", "rank": "speed"}); !strings.Contains(rpcErr, "unknown rank") {
		t.Errorf("unknown rank error = %q", rpcErr)
	}
}

// callToolErr calls a tool and returns its raw response and the JSON-RPC
// error message ("" when the call succeeded).
func callToolErr(srv *Server, name string, args map[string]any) (string, string) {
	raw, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	resp, _ := srv.HandleRequest(context.Background(), raw)
	var env struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(resp, &env)
	if env.Error == nil {
		return string(resp), ""
	}
	return string(resp), env.Error.Message
}
