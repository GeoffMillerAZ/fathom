package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestVerifyViaHTTP_WholePlanInOneCall (O1014, O1169): `fathom verify`
// and `fathom plans status` render the verdicts themselves and read up to
// verifyPlanReadLimit, so they declare a dispatch budget sized to it and a
// 768-row plan answers in ONE verify_plan call through a real serve-http
// handler. Before, the agent default (16,000 B) kept a few dozen rows per
// answer and the rest took a follow-up call each, every one rebuilding
// the call graph on a large index (54 calls, 665 s for language-breadth).
func TestVerifyViaHTTP_WholePlanInOneCall(t *testing.T) {
	f := newConsoleFixture(t)
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rpc" {
			calls.Add(1)
		}
		f.ts.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)

	const rows = 800
	var md strings.Builder
	md.WriteString("# A plan as long as language-breadth\n\n")
	for i := 1; i <= rows; i++ {
		fmt.Fprintf(&md, "- [x] row %d names a symbol the index lacks {id: R-%04d, symbol: NoSuchSymbol%d}\n", i, i, i)
	}
	plan := verify.ParseMarkdownPlan("payment-service", []byte(md.String()))
	if len(plan.Tasks) != rows {
		t.Fatalf("fixture plan has %d rows", len(plan.Tasks))
	}
	hc := &http.Client{Timeout: 60 * time.Second}
	rep, err := verifyViaHTTP(hc, ts.URL, "payment-service", "", []byte(md.String()), plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("verify_plan calls = %d, want the whole plan in 1", got)
	}
	if len(rep.Tasks) != rows || rep.Summary.Total != rows {
		t.Fatalf("report = %d tasks, summary %+v; want every row's verdict", len(rep.Tasks), rep.Summary)
	}
	for _, v := range rep.Tasks {
		if v.Label == "verdict-elided" {
			t.Fatalf("row %s never arrived: %+v", v.TaskID, v)
		}
	}
}

// TestVerifyViaHTTP_OverlongAnswerFallsBackToDefault: an answer past
// verifyPlanReadLimit is never parsed cut; the call is asked again at the
// daemon's default budget, and the follow-up calls complete the plan.
func TestVerifyViaHTTP_OverlongAnswerFallsBackToDefault(t *testing.T) {
	inner, calls := elidingVerifyDaemon(t, 7, nil)
	declared := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(mcp.DeclaredBudgetHeader) != "" {
			declared++
			_, _ = w.Write([]byte(strings.Repeat(" ", verifyPlanReadLimit+1)))
			return
		}
		inner.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	var md strings.Builder
	for i := 1; i <= 25; i++ {
		fmt.Fprintf(&md, "- [x] row %d {id: R-%02d, symbol: Sym%d}\n", i, i, i)
	}
	plan := verify.ParseMarkdownPlan("svc", []byte(md.String()))
	rep, err := verifyViaHTTP(&http.Client{Timeout: 10 * time.Second}, ts.URL, "svc", "", []byte(md.String()), plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Summary.Total != 25 || rep.Summary.Pass != 25 {
		t.Fatalf("summary %+v; want every row through the default budget's follow-ups", rep.Summary)
	}
	if declared == 0 || *calls < 4 {
		t.Errorf("declared calls %d, default calls %d; want each overlong answer retried at the default", declared, *calls)
	}
}
