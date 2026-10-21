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
)

func assertCurrentWindowRefusal(t *testing.T, code int, stdout, stderr, wantReason string) {
	t.Helper()
	if code != 2 {
		t.Fatalf("window refusal exit %d, want 2; stdout=%s stderr=%s", code, stdout, stderr)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("window refusal JSON: %v; stdout=%s stderr=%s", err, stdout, stderr)
	}
	for _, key := range []string{"reports", "tasks", "summary", "pass", "proven"} {
		if _, ok := doc[key]; ok {
			t.Fatalf("window refusal exposed provisional %s: %s", key, stdout)
		}
	}
	var scope, freshness, state string
	_ = json.Unmarshal(doc["source_scope"], &scope)
	_ = json.Unmarshal(doc["current_worktree_freshness"], &freshness)
	_ = json.Unmarshal(doc["source_window_state"], &state)
	var failure struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(doc["error"], &failure); err != nil || scope != "current_worktree" || freshness != "unknown" || state != "refused" || failure.Code != "current_source_window_refused" || !strings.Contains(failure.Reason, wantReason) || !strings.Contains(stderr, wantReason) {
		t.Fatalf("incomplete window refusal: %s stderr=%s", stdout, stderr)
	}
}

func TestCurrentWindowRefusalJSONNeverPublishesReports(t *testing.T) {
	for _, phase := range []string{"begin", "advance", "close", "result"} {
		t.Run(phase, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := emitCurrentWindowRefusal(&stdout, &stderr, "svc", true, fmt.Errorf("current source window: %s failed", phase))
			assertCurrentWindowRefusal(t, code, stdout.String(), stderr.String(), phase+" failed")
		})
	}
}

func TestCurrentWindowExclusionManifestBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "excluded.json")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 1<<20+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSourceWindowExclusions(path); err == nil || !strings.Contains(err.Error(), "exceed 1 MiB") {
		t.Fatalf("unbounded exclusion manifest: %v", err)
	}
}

func TestCurrentWindowFallbackRequiresAbsentPublishedCapability(t *testing.T) {
	for _, tc := range []struct {
		name       string
		advertised bool
		begin      string
		wantErr    bool
		wantToken  string
	}{
		{"old daemon", false, "", false, ""},
		{"malformed new daemon", true, `{"service":"svc","summary":{}}`, true, ""},
		{"refused new daemon", true, `{"error":"seal changed"}`, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}
				if req.Method == "tools/list" {
					properties := map[string]any{}
					if tc.advertised {
						properties["source_window"] = map[string]any{"type": "object"}
					}
					json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"tools": []any{map[string]any{"name": "verify_plan", "inputSchema": map[string]any{"properties": properties}}}}})
					return
				}
				if tc.begin == `{"error":"seal changed"}` {
					fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"message":"seal changed"}}`)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"text": tc.begin}}}})
			}))
			defer srv.Close()
			token, err := beginCurrentWindow(srv.Client(), srv.URL, "svc")
			if (err != nil) != tc.wantErr || token != tc.wantToken {
				t.Fatalf("token=%q err=%v", token, err)
			}
		})
	}
}
