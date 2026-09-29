package main

// prompthook_findings_test.go — FL-21 (docs/vision/30-findings-loop.md
// §3.4): the prompt hook's "open findings: N across the files this prompt
// names" line.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// layerAwarePromptDaemon serves obligations per (file, layer) — unlike
// fakePromptDaemon (keyed by file only), this fake actually respects the
// "layer" argument the real server does, so a test can prove finding-kind
// rows reach ONLY the layer=all count call and never leak into the
// layer="" rules-merge call.
func layerAwarePromptDaemon(t *testing.T, byFileAndLayer map[string]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					File  string `json:"file"`
					Layer string `json:"layer"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "obligations":
			layer := req.Params.Arguments.Layer
			if layer == "" {
				layer = "rules"
			}
			payload, ok := byFileAndLayer[req.Params.Arguments.File][layer]
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

func TestPromptHook_FindingsCountLine(t *testing.T) {
	srv := layerAwarePromptDaemon(t, map[string]map[string]any{
		"internal/auth/token.go": {
			"rules": map[string]any{
				"service": "svc", "file": "internal/auth/token.go",
				"rules": []any{
					map[string]any{"rule": "Secrets must never be logged.", "src": "docs/spec.md#L1-L2", "tier": "verified"},
				},
			},
			"all": map[string]any{
				"service": "svc", "file": "internal/auth/token.go",
				"rules": []any{
					map[string]any{"rule": "Secrets must never be logged.", "src": "docs/spec.md#L1-L2", "tier": "verified"},
					map[string]any{
						"rule": "Do not log API keys.", "src": "internal/auth/token.go#L1-L2", "tier": "sme-stated",
						"kind": "finding", "rule_id": "secrets-in-logs",
					},
				},
			},
		},
	})
	defer srv.Close()

	dir := touchTree(t, t.TempDir(), "internal/auth/token.go") // EW-21: a prompt's path must exist to count
	hookInput := fmt.Sprintf(`{"prompt":"fix internal/auth/token.go","cwd":%q}`, dir)
	deadline := time.Now().Add(promptHookDeadlineBudget)
	text, _, ok := renderPromptObligations(srv.URL, dir, "fix internal/auth/token.go", "sess", deadline)
	_ = hookInput
	if !ok {
		t.Fatal("renderPromptObligations returned ok=false")
	}
	if !strings.Contains(text, "Secrets must never be logged.") {
		t.Errorf("governing rule must still render, got %q", text)
	}
	if strings.Contains(text, "Do not log API keys.") {
		t.Errorf("the finding's rule text must never leak into the rules bullet list, got %q", text)
	}
	if !strings.Contains(text, "open findings: 1 across the files this prompt names") {
		t.Errorf("findings count line must render, got %q", text)
	}
}

func TestPromptHook_FindingsCountLine_ZeroFindingsSilent(t *testing.T) {
	srv := layerAwarePromptDaemon(t, map[string]map[string]any{
		"a.go": {
			"rules": map[string]any{"service": "svc", "file": "a.go", "rules": []any{
				map[string]any{"rule": "R", "src": "s#L1-L2", "tier": "verified"},
			}},
			"all": map[string]any{"service": "svc", "file": "a.go", "rules": []any{
				map[string]any{"rule": "R", "src": "s#L1-L2", "tier": "verified"},
			}},
		},
	})
	defer srv.Close()
	dir := touchTree(t, t.TempDir(), "a.go") // EW-21: a prompt's path must exist to count
	deadline := time.Now().Add(promptHookDeadlineBudget)
	text, _, ok := renderPromptObligations(srv.URL, dir, "fix a.go", "sess", deadline)
	if !ok {
		t.Fatal("renderPromptObligations returned ok=false")
	}
	if strings.Contains(text, "open findings") {
		t.Errorf("zero findings must render no count line, got %q", text)
	}
}

func TestCountFindingsAcrossFiles_DeadlineAlreadySpent(t *testing.T) {
	n, ok := countFindingsAcrossFiles("http://unused.invalid", "svc", []string{"a.go"}, time.Now().Add(-time.Second))
	if ok || n != 0 {
		t.Errorf("countFindingsAcrossFiles = (%d,%v), want (0,false) past the deadline", n, ok)
	}
}
