package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPlansStatusGenerationScopeReachesVerifyPlan(t *testing.T) {
	var mu sync.Mutex
	var scopes []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/indexed" {
			_, _ = w.Write([]byte(`{"indexed":true}`))
			return
		}
		if r.URL.Path != "/rpc" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Method string `json:"method"`
			Params struct {
				Arguments struct {
					SourceScope string `json:"source_scope"`
				} `json:"arguments"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Method == "tools/list" {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"tools": []any{map[string]any{"name": "verify_plan", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"service": map[string]any{"type": "string"}}}}}}})
			return
		}
		mu.Lock()
		scopes = append(scopes, req.Params.Arguments.SourceScope)
		mu.Unlock()
		inner := `{"service":"svc","source_scope":"ingest_generation","current_worktree_freshness":"unknown","summary":{"total":1,"unknown":1},"tasks":[{"id":"G-1","title":"one","status":"unknown","label":"spec-generation-only","criteria":[{"name":"spec_covered","status":"unknown","detail":"ingest-generation refs; current worktree freshness unknown"}]}]}`
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []map[string]string{{"type": "text", "text": inner}}}})
	}))
	defer ts.Close()
	plan := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(plan, []byte("# Plan\n\n- [x] one {id: G-1, symbol: One, spec: [FR-1]}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := run([]string{"plans", "status", "--service", "svc", "--url", ts.URL, "--plan", plan, "--source-scope", "ingest_generation", "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("status exit %d: %s", code, errOut.String())
	}
	mu.Lock()
	if len(scopes) == 0 {
		mu.Unlock()
		t.Fatal("no verify_plan call")
	}
	for _, scope := range scopes {
		if scope != "ingest_generation" {
			mu.Unlock()
			t.Fatalf("verify_plan scope=%q, want ingest_generation", scope)
		}
	}
	mu.Unlock()
	var rows []struct {
		SourceScope              string `json:"source_scope"`
		CurrentWorktreeFreshness string `json:"current_worktree_freshness"`
		Proven                   int    `json:"proven"`
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0].SourceScope != "ingest_generation" || rows[0].CurrentWorktreeFreshness != "unknown" || rows[0].Proven != 0 {
		t.Fatalf("generation rollup=%s err=%v", out.String(), err)
	}
	out.Reset()
	errOut.Reset()
	if code := run([]string{"plans", "status", "--service", "svc", "--url", ts.URL, "--plan", plan, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("default status exit %d: %s", code, errOut.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if scopes[len(scopes)-1] != "" {
		t.Fatalf("default sent source_scope=%q", scopes[len(scopes)-1])
	}
}

func TestPlansStatusGenerationScopeRejectsInvalidAndDirectStore(t *testing.T) {
	for _, args := range [][]string{
		{"plans", "status", "--service", "svc", "--source-scope", "unknown"},
		{"plans", "status", "--service", "svc", "--source-scope", "ingest_generation", "--store", "fixture.sqlite"},
		{"plans", "status", "--service", "svc", "--source-scope", "ingest_generation", "--weighted", "--json"},
	} {
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code == 0 {
			t.Fatalf("invalid scope accepted: %v", args)
		}
	}
}
