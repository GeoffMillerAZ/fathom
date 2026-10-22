package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// TestEditHookDisclosesSupersededWithheld pins RS-2's tail line: rules the
// daemon withheld because they cite a fully superseded decision record are
// counted under the served rules, never dropped silently (I7, I35).
func TestEditHookDisclosesSupersededWithheld(t *testing.T) {
	dir := t.TempDir()
	absFile := filepath.Join(dir, "internal", "store", "backend.go")
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Arguments struct {
					File string `json:"file"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		cannedPayload := map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    req.Params.Arguments.File,
			"rules": []any{
				map[string]any{"rule": "Refuse a .duckdb path loudly.", "src": "ADR.md#L4-L4", "tier": "doc-claimed", "layer": "obligations"},
			},
			"superseded_withheld": 2,
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	hookStdin(t, fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, absFile, dir))
	var out, errBuf bytes.Buffer
	if code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "2 rule(s) citing a superseded decision withheld") {
		t.Errorf("hook output does not disclose the withhold:\n%s", out.String())
	}
}
