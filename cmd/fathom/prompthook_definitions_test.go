package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// definitionsDaemon answers find_symbol from defs (name -> count, one
// definition at internal/x/<name>.go:10 as a func) and counts every request.
func definitionsDaemon(t *testing.T, defs map[string]int, calls *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					Name string `json:"name"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		inner := "{}"
		switch req.Params.Name {
		case "find_symbol":
			n := defs[req.Params.Arguments.Name]
			syms := []any{}
			for i := 0; i < n; i++ {
				syms = append(syms, map[string]any{"name": req.Params.Arguments.Name, "kind": "func", "file_path": "internal/x/" + req.Params.Arguments.Name + ".go", "line": 10 + i})
			}
			inner = mustJSON(map[string]any{"symbols": syms, "matched_via": "exact"})
		case "list_services":
			inner = mustJSON(map[string]any{"services": []any{}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": inner}}}})
	}))
}

func runPromptDefinitions(t *testing.T, on bool, defs map[string]int, prompt string) (string, int64) {
	t.Helper()
	dir := t.TempDir()
	if on {
		writeRepoFile(t, dir, ".fathom/config.toml", "[hooks]\nprompt_definitions = true\n")
	}
	// The production budget is 8 s; a loaded one-CPU race run must not
	// overrun it on a success path.
	origBudget := promptHookDeadlineBudget
	promptHookDeadlineBudget = time.Minute
	defer func() { promptHookDeadlineBudget = origBudget }()
	var calls atomic.Int64
	srv := definitionsDaemon(t, defs, &calls)
	defer srv.Close()
	promptHookStdin(t, dir, prompt)
	var out, errBuf bytes.Buffer
	if code := runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	return out.String(), calls.Load()
}

func TestPromptDefinitionsLineNamesResolvedIdentifiersOnly(t *testing.T) {
	defs := map[string]int{"alphaFunc": 1, "betaFunc": 3}
	prompt := "check `alphaFunc` and `betaFunc` and `ghostFunc`"

	t.Run("off by default", func(t *testing.T) {
		out, _ := runPromptDefinitions(t, false, defs, prompt)
		if strings.Contains(out, "defined:") {
			t.Fatalf("flag off but line present: %s", out)
		}
	})
	t.Run("only resolved, N definitions, no extra calls", func(t *testing.T) {
		out, onCalls := runPromptDefinitions(t, true, defs, prompt)
		_, offCalls := runPromptDefinitions(t, false, defs, prompt)
		if !strings.Contains(out, "alphaFunc func internal/x/alphaFunc.go:10") {
			t.Errorf("missing alpha definition: %s", out)
		}
		if !strings.Contains(out, "betaFunc (3 definitions)") || strings.Contains(out, "betaFunc func") {
			t.Errorf("betaFunc must read 3 definitions, none chosen: %s", out)
		}
		if strings.Contains(out, "ghostFunc") {
			t.Errorf("unresolved identifier named: %s", out)
		}
		if onCalls != offCalls {
			t.Errorf("flag on made %d daemon calls, off made %d: the line adds none", onCalls, offCalls)
		}
	})
	t.Run("silent when nothing resolves", func(t *testing.T) {
		out, _ := runPromptDefinitions(t, true, map[string]int{}, prompt)
		if strings.Contains(out, "defined:") {
			t.Fatalf("nothing resolved but line present: %s", out)
		}
	})
}

func TestPromptDefinitionsLineCapsAtFiveNames(t *testing.T) {
	var defs []promptDefinition
	for i := 0; i < 8; i++ {
		defs = append(defs, promptDefinition{Name: fmt.Sprintf("N%d", i), Kind: "func", File: "a.go", Line: i + 1, Count: 1})
	}
	got := promptDefinitionsLine(defs, false)
	if n := strings.Count(got, "a.go:"); n != 5 {
		t.Errorf("named %d, want 5: %s", n, got)
	}
	if promptDefinitionsLine(nil, false) != "" {
		t.Error("nothing resolved must be silent")
	}
	if !strings.Contains(promptDefinitionsLine(nil, true), "not computed (deadline)") {
		t.Error("a deadline miss must be named")
	}
}
