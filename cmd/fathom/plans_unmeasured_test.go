package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// deadlineMsg is the daemon's answer for a call its deadline abandoned
// (internal/mcp/admission.go), the error O1161 saw end `plans status`.
const deadlineMsg = "abandoned: the call's deadline passed (the index-class server deadline or X-Fathom-Deadline)"

// unmeasuredDaemon answers /indexed and verify_plan like a daemon that
// verifies every row pass, except a plan whose markdown carries "# Slow":
// that call answers the deadline's RPC error.
func unmeasuredDaemon(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/indexed" {
			_, _ = w.Write([]byte(`{"indexed":true}`))
			return
		}
		if r.URL.Path != "/rpc" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Params struct {
				Arguments struct {
					PlanMarkdown string `json:"plan_markdown"`
				} `json:"arguments"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		md := req.Params.Arguments.PlanMarkdown
		if strings.Contains(md, "# Slow") {
			env, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "error": map[string]any{"code": -32000, "message": deadlineMsg}})
			_, _ = w.Write(env)
			return
		}
		plan := verify.ParseMarkdownPlan("svc", []byte(md))
		var tasks []map[string]any
		for _, tk := range plan.Tasks {
			tasks = append(tasks, map[string]any{"id": tk.ID, "title": tk.Title, "status": "pass", "label": "verified"})
		}
		inner, _ := json.Marshal(map[string]any{"service": "svc", "summary": map[string]int{"total": len(tasks), "pass": len(tasks)}, "tasks": tasks})
		env, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []map[string]string{{"type": "text", "text": string(inner)}}}})
		_, _ = w.Write(env)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// unmeasuredPlans writes a slow plan (its daemon verify misses the
// deadline) and a fast one: two claimed rows and one open.
func unmeasuredPlans(t *testing.T) (slow, fast string) {
	t.Helper()
	dir := t.TempDir()
	slow = filepath.Join(dir, "slow.md")
	fast = filepath.Join(dir, "fast.md")
	if err := os.WriteFile(slow, []byte("# Slow plan\n\n- [x] one {id: S-1, symbol: One}\n- [ ] two {id: S-2, symbol: Two}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fast, []byte("# Fast plan\n\n- [x] one {id: F-1, symbol: One}\n- [x] two {id: F-2, symbol: Two}\n- [ ] three {id: F-3, symbol: Three}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return slow, fast
}

// TestCmdPlansStatus_UnmeasuredPlanNeverFailsTheVerb pins O1161: one plan
// whose daemon verify misses its deadline once ended `plans status` with
// that error and no table. Its row now reads `not measured: <the error>`,
// every other plan reports, the summary counts the unmeasured plans, and
// the verb exits 2 (operational, docs/plans/README.md); --json carries the
// same field per plan.
func TestCmdPlansStatus_UnmeasuredPlanNeverFailsTheVerb(t *testing.T) {
	ts := unmeasuredDaemon(t)
	slow, fast := unmeasuredPlans(t)
	args := []string{"plans", "status", "--service", "svc", "--url", ts.URL, "--plan", slow, "--plan", fast}

	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != 2 {
		t.Fatalf("exit = %d, want 2 (one plan unmeasured)\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	var fastLine, slowLine string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "fast.md"):
			fastLine = line
		case strings.HasPrefix(line, "slow.md"):
			slowLine = line
		}
	}
	if f := strings.Fields(fastLine); len(f) < 4 || f[1] != "3" || f[2] != "2" {
		t.Errorf("measured plan row = %q, want total 3, proven 2", fastLine)
	}
	if !strings.Contains(slowLine, "not measured: ") || !strings.Contains(slowLine, "the call's deadline passed") {
		t.Errorf("unmeasured plan row = %q, want `not measured: <the error>`", slowLine)
	}
	if !strings.Contains(out, "not measured: 1 of 2 plans") {
		t.Errorf("summary does not count the unmeasured plan:\n%s", out)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(append(args, "--json"), &stdout, &stderr); code != 2 {
		t.Fatalf("--json exit = %d, want 2\nstderr:\n%s", code, stderr.String())
	}
	var rows []struct {
		Plan        string `json:"plan"`
		Total       int    `json:"total"`
		Proven      int    `json:"proven"`
		NotMeasured string `json:"not_measured"`
		Archivable  bool   `json:"archivable"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	if len(rows) != 2 {
		t.Fatalf("json rows = %d, want both plans", len(rows))
	}
	for _, r := range rows {
		switch filepath.Base(r.Plan) {
		case "fast.md":
			if r.NotMeasured != "" || r.Total != 3 || r.Proven != 2 {
				t.Errorf("measured plan json = %+v", r)
			}
		case "slow.md":
			if !strings.Contains(r.NotMeasured, "the call's deadline passed") || r.Archivable {
				t.Errorf("unmeasured plan json = %+v, want not_measured naming the error, never archivable", r)
			}
		}
	}
}

// TestCmdPlansStatus_AllMeasuredExitsZero: the daemon path with every plan
// verified still exits 0 and prints no not-measured summary.
func TestCmdPlansStatus_AllMeasuredExitsZero(t *testing.T) {
	ts := unmeasuredDaemon(t)
	_, fast := unmeasuredPlans(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"plans", "status", "--service", "svc", "--url", ts.URL, "--plan", fast}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "not measured") {
		t.Errorf("all-measured run names an unmeasured plan:\n%s", stdout.String())
	}
}

// TestCmdPlansNext_UnmeasuredPlanNeverFailsTheVerb: `plans next` shares
// computePlanRowsIn, so the same plan once ended it too. The measured
// plan's frontier prints, the unmeasured plan is named on stderr with the
// error, and the verb exits 2.
func TestCmdPlansNext_UnmeasuredPlanNeverFailsTheVerb(t *testing.T) {
	ts := unmeasuredDaemon(t)
	slow, fast := unmeasuredPlans(t)
	for _, extra := range [][]string{nil, {"--json"}} {
		args := append([]string{"plans", "next", "--service", "svc", "--url", ts.URL, "--root", t.TempDir(), "--plan", slow, "--plan", fast}, extra...)
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("%v exit = %d, want 2\nstdout:\n%s\nstderr:\n%s", extra, code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "F-3") {
			t.Errorf("%v: the measured plan's open row is missing:\n%s", extra, stdout.String())
		}
		if strings.Contains(stdout.String(), "S-2") {
			t.Errorf("%v: an unmeasured plan's row was reported as frontier:\n%s", extra, stdout.String())
		}
		if e := stderr.String(); !strings.Contains(e, "not measured: ") || !strings.Contains(e, "slow.md") || !strings.Contains(e, "the call's deadline passed") {
			t.Errorf("%v: stderr does not name the unmeasured plan and its error:\n%s", extra, e)
		}
	}
}

// TestComputePlanRows_UnmeasuredIsAnError: `plans archive` reads one plan
// through computePlanRows; a plan the daemon could not verify must stay an
// error there, never an empty plan that looks archivable.
func TestComputePlanRows_UnmeasuredIsAnError(t *testing.T) {
	ts := unmeasuredDaemon(t)
	slow, _ := unmeasuredPlans(t)
	if _, err := computePlanRows([]string{slow}, "svc", "", ts.URL); err == nil || !strings.Contains(err.Error(), "the call's deadline passed") {
		t.Fatalf("computePlanRows err = %v, want the daemon's error", err)
	}
}
