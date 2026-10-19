package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestStopHookVerify_EmptyPartialOneCall (O1304) pins the Stop hook's wiring:
// stopHookVerify itself, against a daemon whose verify_plan answers a stage
// partial with every row elided, sends verify_plan once. The unit test
// (TestStopHookSkipsFollowUpAfterEmptyPartial) drives the verify loop
// directly, so it stays green if the hook goes back to verifyViaHTTP.
func TestStopHookVerify_EmptyPartialOneCall(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify-ledger.jsonl"))
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	plans := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plans, "p.md")
	if err := os.WriteFile(planPath, []byte("- [ ] One {id: T-1, symbol: A}\n- [ ] Two {id: T-2, symbol: B}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(planPath, []byte("- [x] One {id: T-1, symbol: A}\n- [x] Two {id: T-2, symbol: B}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var verifies atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "list_services":
			inner = fmt.Sprintf(`{"services":[{"service":%q,"symbol_count":10}]}`, svc)
		case "verify_plan":
			verifies.Add(1)
			inner = `{"service":"` + svc + `","summary":{"total":2},"tasks":[],
			  "elided":{"count":2,"of":2},
			  "partial":"the call graph for this index state is still building"}`
		default:
			http.Error(w, "unknown tool", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}})
	}))
	defer srv.Close()

	stopHookVerify(dir, &http.Client{Timeout: 2 * time.Second}, srv.URL)
	if n := verifies.Load(); n != 1 {
		t.Fatalf("the Stop hook sent verify_plan %d times for an empty stage partial, want 1", n)
	}
}
