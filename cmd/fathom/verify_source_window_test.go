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

	"github.com/geoffmilleraz/fathom/internal/verify"
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

// The pending receipt is budgeted at collection. The closed result must use
// that same dispatch budget or the handler can recut its tasks after close.
func TestCurrentWindowFinalResultsPreserveCollectedReceipts(t *testing.T) {
	f := newConsoleFixtureOpts(t, consoleFixtureOpts{Rooted: true})
	hc := &http.Client{Timeout: 5 * time.Second}
	const service = "payment-service"
	token, err := beginCurrentWindow(hc, f.ts.URL, service)
	if err != nil || token == "" {
		t.Fatalf("begin: token=%q err=%v", token, err)
	}
	defer abortCurrentWindow(hc, f.ts.URL, service, token)

	planOf := func(prefix string, count, titleBytes int) ([]byte, verify.Plan) {
		var b strings.Builder
		b.WriteString("# Window report\n\n")
		for i := 0; i < count; i++ {
			fmt.Fprintf(&b, "- [x] %s row %d %s {id: %s-%04d, symbol: Missing%d}\n", prefix, i, strings.Repeat("x", titleBytes), prefix, i, i)
		}
		md := []byte(b.String())
		return md, verify.ParseMarkdownPlan(service, md)
	}
	type collected struct {
		md   []byte
		plan verify.Plan
		ids  []string
	}
	var reports []collected
	seenReceipts := make(map[string]bool)
	for _, spec := range []struct {
		prefix            string
		count, titleBytes int
		wantContinuations bool
	}{
		{"small", 3, 20, false},
		{"medium", 40, 500, false},
		{"large", 100, 20000, true},
	} {
		md, plan := planOf(spec.prefix, spec.count, spec.titleBytes)
		ids, err := collectVerifyWindow(hc, f.ts.URL, service, "", md, plan, nil, token)
		if err != nil {
			t.Fatalf("collect %s: %v", spec.prefix, err)
		}
		if len(ids) == 0 || spec.wantContinuations != (len(ids) > 1) {
			t.Fatalf("collect %s: %d receipts, want continuations=%v", spec.prefix, len(ids), spec.wantContinuations)
		}
		for _, id := range ids {
			if seenReceipts[id] {
				t.Fatalf("duplicate collected receipt %s", id)
			}
			seenReceipts[id] = true
		}
		reports = append(reports, collected{md, plan, ids})
	}
	if err := closeCurrentWindow(hc, f.ts.URL, service, token); err != nil {
		t.Fatal(err)
	}
	for _, report := range reports {
		got, err := finalVerifyWindow(hc, f.ts.URL, service, report.md, report.plan, token, report.ids)
		if err != nil {
			t.Fatalf("final %s: %v", report.plan.Tasks[0].ID, err)
		}
		if len(report.plan.Tasks) == 40 {
			encoded, err := json.Marshal(got)
			if err != nil || len(encoded) <= 16000 {
				t.Fatalf("medium final report = %d bytes, err %v; need a result above the agent default", len(encoded), err)
			}
		}
		if len(got.Tasks) != len(report.plan.Tasks) {
			t.Fatalf("final %s: %d of %d tasks", report.plan.Tasks[0].ID, len(got.Tasks), len(report.plan.Tasks))
		}
		seen := make(map[string]int)
		for _, task := range got.Tasks {
			seen[task.TaskID]++
		}
		for _, task := range report.plan.Tasks {
			if seen[task.ID] != 1 {
				t.Errorf("final %s: task %s appears %d times", report.plan.Tasks[0].ID, task.ID, seen[task.ID])
			}
		}
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
