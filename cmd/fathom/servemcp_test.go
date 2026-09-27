package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/memo"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// buildMCPInput constructs a finite stdin reader that sends initialize
// followed by tools/list (and nothing else), so Serve returns at EOF
// without blocking.
func buildMCPInput(t *testing.T, extras ...map[string]any) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	writeJSON := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("buildMCPInput marshal: %v", err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	writeJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"clientInfo":      map[string]any{"name": "test", "version": "0.1"},
			"capabilities":    map[string]any{},
		},
	})
	writeJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	})
	for _, extra := range extras {
		writeJSON(extra)
	}
	return buf
}

// parseAllResponses decodes all newline-delimited JSON objects from buf.
func parseAllResponses(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var results []map[string]any
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	for dec.More() {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatalf("parseAllResponses decode: %v (buf=%q)", err, buf.String())
		}
		results = append(results, m)
	}
	return results
}

// ─── serve-mcp: flag validation ─────────────────────────────────────────────

func TestServeMCPMissingStore(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runServeMCP([]string{}, buildMCPInput(t), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --store is missing")
	}
	if errBuf.String() == "" {
		t.Fatal("expected non-empty stderr message")
	}
}

func TestServeMCPUnknownMode(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var out, errBuf bytes.Buffer
	code := runServeMCP([]string{"--store", store, "--mode", "bogus"},
		buildMCPInput(t), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit for unknown --mode")
	}
}

// ─── serve-mcp: initialize response ─────────────────────────────────────────

func TestServeMCPInitialize(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var out, errBuf bytes.Buffer
	code := runServeMCP([]string{"--store", store},
		buildMCPInput(t), &out, &errBuf)
	if code != 0 {
		t.Fatalf("serve-mcp exited %d; stderr=%q", code, errBuf.String())
	}

	responses := parseAllResponses(t, &out)
	if len(responses) < 1 {
		t.Fatalf("expected at least one response; got 0 (stdout=%q)", out.String())
	}

	// First response must be initialize.
	initResp := responses[0]
	if initResp["error"] != nil {
		t.Fatalf("initialize error: %v", initResp["error"])
	}
	result, ok := initResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("initialize result missing: %v", initResp)
	}
	si, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("serverInfo missing in initialize result: %v", result)
	}
	if si["name"] != "fathom" {
		t.Errorf("serverInfo.name = %v, want fathom", si["name"])
	}
}

// ─── serve-mcp: tools/list contains rename_impact ───────────────────────────

func TestServeMCPToolsList(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var out, errBuf bytes.Buffer
	code := runServeMCP([]string{"--store", store},
		buildMCPInput(t), &out, &errBuf)
	if code != 0 {
		t.Fatalf("serve-mcp exited %d; stderr=%q", code, errBuf.String())
	}

	responses := parseAllResponses(t, &out)
	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses (init + tools/list); got %d", len(responses))
	}

	// Second response is tools/list.
	listResp := responses[1]
	if listResp["error"] != nil {
		t.Fatalf("tools/list error: %v", listResp["error"])
	}
	result, ok := listResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list result missing: %v", listResp)
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools missing or wrong type: %v", result)
	}
	if len(tools) == 0 {
		t.Fatal("tools/list returned empty tool list")
	}

	names := make([]string, 0, len(tools))
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("tool entry not a map: %v", raw)
		}
		name, _ := tool["name"].(string)
		names = append(names, name)
	}

	// rename_impact must be registered (analysis tool).
	found := false
	for _, n := range names {
		if n == "rename_impact" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("rename_impact not in tools/list; got: %s", strings.Join(names, ", "))
	}
}

// ─── serve-mcp: run() dispatch ───────────────────────────────────────────────

func TestRunServeMCPSubcommand(t *testing.T) {
	// Verify that "serve-mcp" is dispatched through the main run() function.
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var out, errBuf bytes.Buffer
	in := buildMCPInput(t)
	code := runWithStdin([]string{"serve-mcp", "--store", store}, in, &out, &errBuf)
	if code != 0 {
		t.Fatalf("run serve-mcp exited %d; stderr=%q stdout=%q", code, errBuf.String(), out.String())
	}
}

// TestServeMCPBoundsTheMemoRegistry (review finding, I51): serve-mcp runs
// the same MCP tools as serve-http over the same memo registry, so it
// takes the same cache budget (the cache fraction of its unmanaged
// tenancy budget) — with none the registry held every service it touched.
func TestServeMCPBoundsTheMemoRegistry(t *testing.T) {
	prev, _ := memo.Default.Budget()
	memo.Default.SetBudget(0, 0)
	t.Cleanup(func() { memo.Default.SetBudget(prev, 0) })
	t.Setenv("FATHOM_TENANCY_BUDGET", "1GiB")
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var out, errBuf bytes.Buffer
	if code := runServeMCP([]string{"--store", store}, buildMCPInput(t), &out, &errBuf); code != 0 {
		t.Fatalf("serve-mcp exited %d; stderr=%q", code, errBuf.String())
	}
	got, _ := memo.Default.Budget()
	if got <= 0 || got >= 1<<30 {
		t.Fatalf("memo budget = %d, want the cache fraction of the 1 GiB tenancy budget", got)
	}
}
