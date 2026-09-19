package main

// edithook_pre_freshness_test.go — SU-6 (docs/plans/substrate.md §A,
// ADR-041 decisions 3/4): when the FIRST obligations fetch reports
// index_stale for the edited file, the pre-edit hook asks the daemon to
// refresh just that file (POST /refresh-file) before answering, and names
// what happened in the staleness line instead of a bare "index is behind".

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// rpcRequestName decodes a tools/call request's tool name.
func rpcRequestName(r *http.Request) string {
	var req struct {
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	_ = json.Unmarshal(body, &req)
	return req.Params.Name
}

func rpcTextResult(payload map[string]any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"result": map[string]any{"content": []any{
			map[string]any{"type": "text", "text": mustJSON(payload)},
		}},
	}
}

// TestEditHookPre_StaleIndex_RefreshSucceeds proves the happy path: a
// stale first answer triggers a refresh, the SECOND obligations answer
// (now fresh) is what the agent actually sees, and the staleness line
// says "refreshed", never "behind".
func TestEditHookPre_StaleIndex_RefreshSucceeds(t *testing.T) {
	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "Store schema is spec-governed."}]`)
	file := filepath.Join(dir, "internal", "store", "db.go")

	var obligationsCalls int32
	var refreshCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		if rpcRequestName(r) != "obligations" {
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{"rules": []any{}}))
			return
		}
		n := atomic.AddInt32(&obligationsCalls, 1)
		if n == 1 {
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{
				"service": strings.ToLower(filepath.Base(dir)),
				"file":    "internal/store/db.go",
				"rules": []any{
					map[string]any{"rule": "STALE: migrations are additive-only.", "src": "docs/spec.md#L1", "tier": "verified"},
				},
				"index_stale": true,
			}))
			return
		}
		json.NewEncoder(w).Encode(rpcTextResult(map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    "internal/store/db.go",
			"rules": []any{
				map[string]any{"rule": "FRESH: migrations are additive-only.", "src": "docs/spec.md#L1", "tier": "verified"},
			},
			"compiled_at_watermark": "wm-42",
		}))
	})
	mux.HandleFunc("/refresh-file", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&refreshCalls, 1)
		json.NewEncoder(w).Encode(refreshFileResponse{Refreshed: true, Symbols: 3, TypedEdgesStale: true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-fresh"}`, file, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	outStr := out.String()

	if atomic.LoadInt32(&refreshCalls) != 1 {
		t.Errorf("POST /refresh-file called %d time(s), want 1", refreshCalls)
	}
	if atomic.LoadInt32(&obligationsCalls) != 2 {
		t.Errorf("obligations fetched %d time(s), want 2 (stale, then re-fetched fresh)", obligationsCalls)
	}
	if !strings.Contains(outStr, "index refreshed for this file (typed edges as of wm-42)") {
		t.Errorf("expected the refreshed staleness line naming the watermark, got %s", outStr)
	}
	if strings.Contains(outStr, "STALE:") {
		t.Errorf("the STALE rule text must not reach the agent after a successful refresh: %s", outStr)
	}
	if !strings.Contains(outStr, "FRESH: migrations are additive-only.") {
		t.Errorf("expected the FRESH rule text after refresh, got %s", outStr)
	}
}

// TestEditHookPre_StaleIndex_RefreshFails proves the honest-degradation
// path: a refresh that fails leaves the ORIGINAL stale answer standing,
// with a staleness line naming the failure reason rather than pretending
// the index caught up.
func TestEditHookPre_StaleIndex_RefreshFails(t *testing.T) {
	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "Store schema is spec-governed."}]`)
	file := filepath.Join(dir, "internal", "store", "db.go")

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		if rpcRequestName(r) != "obligations" {
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{"rules": []any{}}))
			return
		}
		json.NewEncoder(w).Encode(rpcTextResult(map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    "internal/store/db.go",
			"rules": []any{
				map[string]any{"rule": "STALE: migrations are additive-only.", "src": "docs/spec.md#L1", "tier": "verified"},
			},
			"index_stale": true,
		}))
	})
	mux.HandleFunc("/refresh-file", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(refreshFileResponse{Error: "read target.go: permission denied"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-fail"}`, file, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	outStr := out.String()
	if !strings.Contains(outStr, "index is behind your working tree (refresh failed: read target.go: permission denied)") {
		t.Errorf("expected the refresh-failed staleness line, got %s", outStr)
	}
	if !strings.Contains(outStr, "STALE: migrations are additive-only.") {
		t.Errorf("the original (stale) rule must still be served when refresh fails: %s", outStr)
	}
}
