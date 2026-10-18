package main

// verify_operational_test.go — O1162: a strict shortfall whose every unproven
// claimed row could not be read (graph fallback, unanswered verdict) is
// operational: exit 2. A genuine failing claim among them keeps exit 1.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeVerdict struct {
	ID       string
	Status   string
	Label    string
	Criteria []map[string]string
}

// verdictDaemon answers verify_plan with the verdicts given; ids absent from
// verdicts are never answered (the CLI marks them verdict-elided).
func verdictDaemon(t *testing.T, verdicts []fakeVerdict) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r) // no indexed route: the CLI falls back to list_services
			return
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"list_services"`) {
			inner, _ := json.Marshal(map[string]any{"services": []map[string]any{{"service": "svc", "symbol_count": 5}}})
			env, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []map[string]string{{"type": "text", "text": string(inner)}}}})
			_, _ = w.Write(env)
			return
		}
		var tasks []map[string]any
		for _, v := range verdicts {
			tasks = append(tasks, map[string]any{"id": v.ID, "title": v.ID, "status": v.Status, "label": v.Label, "criteria": v.Criteria})
		}
		inner, _ := json.Marshal(map[string]any{"service": "svc", "summary": map[string]int{"total": len(tasks)}, "tasks": tasks})
		env, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []map[string]string{{"type": "text", "text": string(inner)}}}})
		_, _ = w.Write(env)
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

func verifyAgainstFake(t *testing.T, plan string, verdicts []fakeVerdict) (int, string) {
	t.Helper()
	dir := t.TempDir()
	pf := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(pf, []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	url := verdictDaemon(t, verdicts)
	code, so, se := runInDir(t, dir, []string{"verify", "--service", "svc", "--plan", pf, "--url", url, "--strict", "--ledger", filepath.Join(dir, "l.jsonl")})
	return code, so + se
}

var graphFallbackCriterion = []map[string]string{{
	"name": "edge_wired", "status": "unknown",
	"detail": "Main does not reach Run (typed call graph unavailable: typed edges: refused) [graph-fallback]",
}}

const operationalPlan = "- [x] wired {id: R-1, symbol: Run, reachable_from: Main}\n- [x] also wired {id: R-2, symbol: Run2, reachable_from: Main}\n- [x] absent {id: R-3, symbol: Ghost}\n"

func TestVerifyOperational_GraphFallbackOnlyExits2(t *testing.T) {
	code, out := verifyAgainstFake(t, operationalPlan[:strings.Index(operationalPlan, "- [x] absent")], []fakeVerdict{
		{"R-1", "unknown", "graph-fallback", graphFallbackCriterion},
		{"R-2", "unknown", "graph-fallback", graphFallbackCriterion},
	})
	if code != 2 {
		t.Fatalf("only graph-fallback rows unproven: want 2, got %d\n%s", code, out)
	}
	for _, want := range []string{"graph-fallback:", "typed edges: refused", "2 claimed row(s) held back", "--store"} {
		if !strings.Contains(out, want) {
			t.Errorf("aggregate line missing %q:\n%s", want, out)
		}
	}
}

func TestVerifyOperational_FallbackPlusGenuineFailureExits1(t *testing.T) {
	code, out := verifyAgainstFake(t, operationalPlan, []fakeVerdict{
		{"R-1", "unknown", "graph-fallback", graphFallbackCriterion},
		{"R-2", "unknown", "graph-fallback", graphFallbackCriterion},
		{"R-3", "fail", "claimed-but-absent", []map[string]string{{"name": "symbol_present", "status": "fail", "detail": "\"Ghost\" not found in the index"}}},
	})
	if code != 1 {
		t.Fatalf("a genuine failing row among them: want 1, got %d\n%s", code, out)
	}
}

func TestVerifyOperational_VerdictElidedOnlyExits2(t *testing.T) {
	code, out := verifyAgainstFake(t, "- [x] lost {id: R-1, symbol: Run}\n", nil)
	if code != 2 {
		t.Fatalf("only verdict-elided rows unproven: want 2, got %d\n%s", code, out)
	}
}
