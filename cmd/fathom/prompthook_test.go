package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakePromptDaemon serves obligations (canned per exact file) and
// find_symbol (canned per exact name) from small maps — everything else
// gets an empty-but-well-formed RPC result rather than an error, matching
// the daemon's own fail-open shape for list_services et al.
func fakePromptDaemon(t *testing.T, obligationsByFile map[string]any, symbolFiles map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					File string `json:"file"`
					Name string `json:"name"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "obligations":
			payload, ok := obligationsByFile[req.Params.Arguments.File]
			if !ok {
				payload = map[string]any{"service": "svc", "file": req.Params.Arguments.File}
			}
			inner = mustJSON(payload)
		case "find_symbol":
			if fp, ok := symbolFiles[req.Params.Arguments.Name]; ok {
				inner = mustJSON(map[string]any{"symbols": []any{map[string]any{"name": req.Params.Arguments.Name, "file_path": fp}}, "matched_via": "exact"})
			} else {
				inner = mustJSON(map[string]any{"symbols": []any{}, "matched_via": "none"})
			}
		case "list_services":
			inner = mustJSON(map[string]any{"services": []any{}})
		default:
			inner = "{}"
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": inner},
			}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// fakeSlowPromptDaemon is fakePromptDaemon plus a per-file artificial
// delay on the "obligations" call — lets a test make ONE candidate file
// resolve fast and another hang well past a shortened deadline, so the
// deadline (not luck) is what bounds the hook's total wall-clock time.
func fakeSlowPromptDaemon(t *testing.T, obligationsByFile map[string]any, delayByFile map[string]time.Duration) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					File string `json:"file"`
					Name string `json:"name"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name == "obligations" {
			if d, ok := delayByFile[req.Params.Arguments.File]; ok {
				// A zero delay never answers on its own: it holds until the
				// client gives up (its own Timeout fired), so the hook
				// returning means the shared deadline fired (EW-7).
				var answer <-chan time.Time
				if d > 0 {
					answer = time.After(d)
				}
				select {
				case <-answer:
				case <-r.Context().Done(): // the client gave up (its own Timeout fired): stop holding the connection open
					return
				}
			}
		}
		var inner string
		switch req.Params.Name {
		case "obligations":
			payload, ok := obligationsByFile[req.Params.Arguments.File]
			if !ok {
				payload = map[string]any{"service": "svc", "file": req.Params.Arguments.File}
			}
			inner = mustJSON(payload)
		case "list_services":
			inner = mustJSON(map[string]any{"services": []any{}})
		default:
			inner = "{}"
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": inner},
			}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// TestPromptHook_SlowDaemonReturnsWithinBudget pins the review finding on
// O57 item 4/5(a): prompt-hook must share ONE deadline across every RPC a
// call makes, not hand each its own fresh timeout — a prompt naming
// several files could otherwise spend several multiples of a single
// call's budget against the hook's own wall-clock limit. Two candidate
// files: one the fake daemon answers immediately, one it holds well past
// a deliberately shortened deadline. The hook must still return quickly
// (bounded by the shortened deadline, not the daemon's artificial delay),
// serve the file that DID resolve, and say so resolved partially.
func TestPromptHook_SlowDaemonReturnsWithinBudget(t *testing.T) {
	orig := promptHookDeadlineBudget
	promptHookDeadlineBudget = 300 * time.Millisecond
	defer func() { promptHookDeadlineBudget = orig }()

	dir := t.TempDir()
	srv := fakeSlowPromptDaemon(t, map[string]any{
		"internal/store/backend.go": map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    "internal/store/backend.go",
			"rules": []any{
				map[string]any{
					"rule": "internal/store stays pure Go — the DuckDB backend was deleted outright (ADR-025); never reintroduce CGO or DuckDB there.",
					"src":  "ADR.md#L814-L814", "tier": "doc-claimed", "layer": "adr-rules",
				},
			},
		},
		"internal/mcp/tools.go": map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    "internal/mcp/tools.go",
			"rules":   []any{map[string]any{"rule": "Register every tool in the mcp registry.", "src": "internal/CLAUDE.md#L4-L4", "tier": "doc-claimed", "layer": "obligations"}},
		},
	}, map[string]time.Duration{
		"internal/mcp/tools.go": 0, // never answers: the 300ms deadline must end it, not block the hook
	})
	defer srv.Close()

	promptHookStdin(t, dir, "please update internal/store/backend.go and internal/mcp/tools.go together")

	var out, errBuf bytes.Buffer
	var code int
	returnsUnaided(t, "prompt-hook against a daemon that never answers for one file", func() {
		code = runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	})

	if code != 0 {
		t.Fatalf("prompt-hook exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "ADR-025") {
		t.Fatalf("output missing the fast file's rule: %s", out.String())
	}
	if !strings.Contains(out.String(), "partial: 1 of 2 resolved") {
		t.Fatalf("output missing the partial-resolution note: %s", out.String())
	}
}

func promptHookStdin(t *testing.T, cwd, prompt string) {
	t.Helper()
	body := fmt.Sprintf(`{"session_id":"s1","cwd":%q,"hook_event_name":"UserPromptSubmit","prompt":%q}`, cwd, prompt)
	hookStdin(t, body)
}

// TestPromptHook_PathNameServesGoverningRule pins the required O57
// item 4/5 reproduction: a prompt naming internal/store/backend.go serves
// ADR-025's rule (internal/store stays pure Go — the DuckDB backend was
// deleted outright) before any tool has touched the file.
func TestPromptHook_PathNameServesGoverningRule(t *testing.T) {
	dir := t.TempDir()
	srv := fakePromptDaemon(t, map[string]any{
		"internal/store/backend.go": map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    "internal/store/backend.go",
			"rules": []any{
				map[string]any{
					"rule": "internal/store stays pure Go — the DuckDB backend was deleted outright (ADR-025); never reintroduce CGO or DuckDB there.",
					"src":  "ADR.md#L814-L814", "tier": "doc-claimed", "layer": "adr-rules",
				},
			},
		},
	}, nil)
	defer srv.Close()

	promptHookStdin(t, dir, "please refactor internal/store/backend.go to add a caching layer")

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("prompt-hook exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "ADR-025") {
		t.Fatalf("output missing ADR-025's rule: %s", out.String())
	}
	if !strings.Contains(out.String(), "internal/store/backend.go") {
		t.Fatalf("output does not name the file the prompt named: %s", out.String())
	}
}

// TestPromptHook_SilentWhenNothingResolves: a prompt naming nothing
// path-, package- or symbol-shaped produces no output.
func TestPromptHook_SilentWhenNothingResolves(t *testing.T) {
	dir := t.TempDir()
	srv := fakePromptDaemon(t, nil, nil)
	defer srv.Close()

	promptHookStdin(t, dir, "please make the tests pass and fix the bug")

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("prompt-hook exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for a prompt naming nothing resolvable, got %q", out.String())
	}
}

// TestPromptHook_SilentOnEmptyStdin mirrors edit-hook's own silence
// contract on unparseable/empty input.
func TestPromptHook_SilentOnEmptyStdin(t *testing.T) {
	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.WriteString("{}")
	w.Close()
	var out, errBuf bytes.Buffer
	code := run([]string{"prompt-hook"}, &out, &errBuf)
	os.Stdin = origStdin
	r.Close()
	if code != 0 {
		t.Fatalf("prompt-hook exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("empty prompt: expected no output, got %q", out.String())
	}
}

// TestPromptHook_ResolvesBareSymbolViaFindSymbol: a prompt naming a bare
// CamelCase symbol (no path) resolves through find_symbol before
// obligations is queried for the file it lands in.
func TestPromptHook_ResolvesBareSymbolViaFindSymbol(t *testing.T) {
	dir := t.TempDir()
	srv := fakePromptDaemon(t, map[string]any{
		"internal/mcp/tools.go": map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    "internal/mcp/tools.go",
			"rules": []any{
				map[string]any{"rule": "Register every tool in the mcp registry.", "src": "internal/CLAUDE.md#L4-L4", "tier": "doc-claimed", "layer": "obligations"},
			},
		},
	}, map[string]string{
		"registerTool": "internal/mcp/tools.go",
	})
	defer srv.Close()

	promptHookStdin(t, dir, "add a case to registerTool for the new verb")

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("prompt-hook exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Register every tool in the mcp registry.") {
		t.Fatalf("output missing the rule resolved via find_symbol: %s", out.String())
	}
}
