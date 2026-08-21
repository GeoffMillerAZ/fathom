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

// TestAnsweredBeatCarriesDisplacedCount pins IT-6: the answered beat must
// carry the obligations-cap displaced count the hook already renders into
// the payload (T5-2/O14), so obligations-cap pressure becomes a trendable
// number in the beat log — a COUNT only, never the evicted rules themselves.
func TestAnsweredBeatCarriesDisplacedCount(t *testing.T) {
	dir := t.TempDir()
	absFile := filepath.Join(dir, "internal", "query", "engine.go")

	beatLog := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)

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
				map[string]any{"rule": "Sort all outputs.", "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed", "layer": "obligations"},
			},
			"displaced": 3,
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

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, absFile, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}

	beats, err := readRetroBeats(beatLog)
	if err != nil {
		t.Fatalf("reading beat log: %v", err)
	}
	found := false
	for _, b := range beats {
		if b.Channel == "edit-hook" && b.Kind == "answered" && b.Meta["emitted"] == "true" {
			found = true
			if b.Meta["displaced"] != "3" {
				t.Errorf("answered beat meta[displaced] = %q, want %q; meta = %+v", b.Meta["displaced"], "3", b.Meta)
			}
		}
	}
	if !found {
		t.Fatal("no answered/emitted=true beat found in log")
	}
}
