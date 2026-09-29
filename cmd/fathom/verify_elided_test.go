package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// elidingVerifyDaemon answers verify_plan like a daemon whose dispatch
// budget keeps only the first keep tasks (sorted by id) and discloses the
// rest in an elided block; a positive refuse stops answering rows whose
// id is in it (a verdict that never arrives). calls counts the requests.
func elidingVerifyDaemon(t *testing.T, keep int, refuse map[string]bool) (*httptest.Server, *int) {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
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
		plan := verify.ParseMarkdownPlan("svc", []byte(req.Params.Arguments.PlanMarkdown))
		type task struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
			Label  string `json:"label"`
		}
		var tasks []task
		for _, tk := range plan.Tasks {
			if refuse[tk.ID] {
				continue
			}
			tasks = append(tasks, task{ID: tk.ID, Title: tk.Title, Status: "pass", Label: "verified"})
		}
		sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
		out := map[string]any{"service": "svc", "summary": map[string]int{"total": len(plan.Tasks), "pass": len(tasks)}}
		if len(tasks) > keep {
			out["elided"] = map[string]any{"count": len(tasks) - keep, "of": len(tasks), "ordering": "as returned", "how_to_get_more": "narrow the query"}
			tasks = tasks[:keep]
		}
		out["tasks"] = tasks
		inner, _ := json.Marshal(out)
		env, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []map[string]string{{"type": "text", "text": string(inner)}}}})
		_, _ = w.Write(env)
	}))
	t.Cleanup(ts.Close)
	return ts, &calls
}

// TestVerifyViaHTTP_CompletesElidedAnswers pins the daemon-mode gate's
// completeness: the daemon's dispatch budget (SU-3) cuts a long plan's
// verify_plan answer to what fits, and verifyViaHTTP once reported only the
// rows that fit — `fathom verify` against the daemon checked 20 of the
// observatory plan's 59 rows and passed. The rest now arrive in follow-up
// calls carrying only the missing rows (positional md-NNN ids kept), and a
// row whose verdict never arrives is Unknown with the reason, never left
// out of the report the gate reads.
func TestVerifyViaHTTP_CompletesElidedAnswers(t *testing.T) {
	var md strings.Builder
	md.WriteString("# Long plan\n\n")
	for i := 1; i <= 23; i++ {
		fmt.Fprintf(&md, "- [x] row %d {id: R-%02d, symbol: Sym%d}\n", i, i, i)
	}
	md.WriteString("- [x] an unnamed row with a positional id {symbol: Anon}\n")
	md.WriteString("- [ ] another unnamed row {symbol: Anon2}\n")
	plan := verify.ParseMarkdownPlan("svc", []byte(md.String()))
	if len(plan.Tasks) != 25 {
		t.Fatalf("fixture plan has %d rows", len(plan.Tasks))
	}
	hc := &http.Client{Timeout: 10 * time.Second}

	ts, calls := elidingVerifyDaemon(t, 7, nil)
	rep, err := verifyViaHTTP(hc, ts.URL, "svc", "", []byte(md.String()), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Tasks) != 25 || rep.Summary.Total != 25 || rep.Summary.Pass != 25 {
		t.Fatalf("report = %d tasks, summary %+v; want every row's verdict", len(rep.Tasks), rep.Summary)
	}
	ids := map[string]bool{}
	for _, v := range rep.Tasks {
		if ids[v.TaskID] {
			t.Errorf("row %s reported twice", v.TaskID)
		}
		ids[v.TaskID] = true
		if v.Status != verify.Pass {
			t.Errorf("row %s = %s", v.TaskID, v.Status)
		}
	}
	for _, id := range []string{"md-024", "md-025", "R-23"} {
		if !ids[id] {
			t.Errorf("row %s missing — positional ids must survive the follow-up calls", id)
		}
	}
	if *calls < 4 {
		t.Errorf("verify_plan calls = %d, want the first answer plus follow-ups", *calls)
	}

	// A verdict that never arrives is Unknown with the reason, and the gate
	// reads it (strict fails it) — never a silently ungated claim.
	ts, _ = elidingVerifyDaemon(t, 7, map[string]bool{"R-20": true})
	rep, err = verifyViaHTTP(hc, ts.URL, "svc", "", []byte(md.String()), plan)
	if err != nil {
		t.Fatal(err)
	}
	var lost *verify.TaskVerdict
	for i := range rep.Tasks {
		if rep.Tasks[i].TaskID == "R-20" {
			lost = &rep.Tasks[i]
		}
	}
	if lost == nil || lost.Status != verify.Unknown || lost.Label != "verdict-elided" || !lost.Claimed {
		t.Fatalf("a verdict that never arrived = %+v", lost)
	}
	if gr := verify.Gate(rep, true); len(gr.Failures) != 1 {
		t.Errorf("strict gate failures = %v, want the unverified claim", gr.Failures)
	}

	// An answer nothing was cut from keeps the daemon's own summary.
	ts, calls = elidingVerifyDaemon(t, 100, nil)
	rep, err = verifyViaHTTP(hc, ts.URL, "svc", "", []byte(md.String()), plan)
	if err != nil || len(rep.Tasks) != 25 || *calls != 1 {
		t.Errorf("uncut answer = %d tasks in %d calls (%v)", len(rep.Tasks), *calls, err)
	}
}

// TestVerifyWithCaller_BuildingAnswerIsFollowedUp: a daemon whose call graph
// is still building answers verify_plan with every row elided and a named
// partial (EW-22, O130); the gate's caller asks again and gets the rows from
// the build the first call left running, and a row that still never
// arrives is Unknown naming why.
func TestVerifyWithCaller_BuildingAnswerIsFollowedUp(t *testing.T) {
	md := []byte("# p\n\n- [x] one {id: A-1, symbol: One}\n- [x] two {id: A-2, symbol: Two}\n")
	plan := verify.ParseMarkdownPlan("svc", md)
	const building = "the call graph for this index state is still building"
	answer := func(ready bool) httpVerifyResult {
		var hvr httpVerifyResult
		hvr.Service = "svc"
		if !ready {
			hvr.Partial = building
			hvr.Elided = &struct {
				Count int `json:"count"`
				Of    int `json:"of"`
			}{Count: 2, Of: 2}
			return hvr
		}
		for _, id := range []string{"A-1", "A-2"} {
			hvr.Tasks = append(hvr.Tasks, httpVerifyTask{ID: id, Status: "pass", Label: "verified"})
		}
		hvr.Summary.Total, hvr.Summary.Pass = 2, 2
		return hvr
	}

	calls := 0
	rep, err := verifyWithCaller(func([]byte) (httpVerifyResult, error) {
		calls++
		return answer(calls > 1), nil
	}, md, plan)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || rep.Summary.Pass != 2 || len(rep.Tasks) != 2 {
		t.Fatalf("calls %d, report %+v: the building answer must be followed up", calls, rep)
	}

	rep, err = verifyWithCaller(func([]byte) (httpVerifyResult, error) { return answer(false), nil }, md, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range rep.Tasks {
		if v.Status != verify.Unknown || len(v.Criteria) != 1 || !strings.Contains(v.Criteria[0].Detail, building) {
			t.Fatalf("a row the build never answered = %+v, want Unknown naming the build", v)
		}
	}
}
