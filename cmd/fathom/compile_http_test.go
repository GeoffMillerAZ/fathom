package main

// compile_http_test.go — EW-73 (G7) and EW-74 (G8): a spending
// `fathom compile` through the daemon sizes its client timeout from the
// service's own history, and the daemon dry run prints the same priced
// output band as the --store path. No model is called and no wall clock is
// waited on: the timeout is observed as a requested value.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"bytes"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestCompileClientTimeout_SizedFromHistory(t *testing.T) {
	cases := []struct {
		name       string
		sections   int
		perSection time.Duration // 0: the service has no history
		want       time.Duration
	}{
		{"history sizes it", 40, 30 * time.Second, 40 * 30 * time.Second * 3 / 2},
		{"no history uses the documented default per section", 40, 0, 40 * compileDefaultSectionDuration * 3 / 2},
		{"floor", 1, time.Second, compileClientTimeoutFloor},
		{"zero sections is the floor", 0, 30 * time.Second, compileClientTimeoutFloor},
		{"cap", 5000, time.Minute, compileClientTimeoutCap},
	}
	for _, tc := range cases {
		if got := compileClientTimeout(tc.sections, tc.perSection); got != tc.want {
			t.Errorf("%s: compileClientTimeout(%d, %v) = %v, want %v", tc.name, tc.sections, tc.perSection, got, tc.want)
		}
	}
	if compileClientTimeoutFloor != 120*time.Second {
		t.Errorf("the floor is the old fixed timeout, got %v", compileClientTimeoutFloor)
	}
}

// fakeCompileDaemon answers compile_layer: the dry run reports 40
// sections and 30000 ms per section; the spending call records that it ran.
func fakeCompileDaemon(t *testing.T, spent *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Arguments struct {
					DryRun bool `json:"dry_run"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		res := map[string]any{"service": "svc", "layer": "obligations", "dry_run": req.Params.Arguments.DryRun,
			"sections_total": 40, "sections_compiled": 40, "tokens_in": 100, "tokens_out": 0, "cost_usd": 0, "duration_ms": 1}
		if req.Params.Arguments.DryRun {
			res["est_section_ms"] = 30000
		} else {
			*spent++
		}
		inner, _ := json.Marshal(res)
		out, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []map[string]any{{"type": "text", "text": string(inner)}}}})
		w.Write(out)
	}))
}

func TestCompileViaHTTP_SpendingCompileSizesItsTimeoutFromTheEstimate(t *testing.T) {
	var spent int
	ts := fakeCompileDaemon(t, &spent)
	defer ts.Close()
	var asked []time.Duration
	hc := func(d time.Duration) *http.Client { asked = append(asked, d); return ts.Client() }

	var stdout, stderr bytes.Buffer
	if code := compileViaHTTPWith(hc, ts.URL, "svc", "obligations", false, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if spent != 1 {
		t.Fatalf("spending calls = %d, want 1", spent)
	}
	want := 40 * 30 * time.Second * 3 / 2
	if len(asked) != 2 || asked[len(asked)-1] != want {
		t.Fatalf("client timeouts asked = %v, want the spending call sized %v", asked, want)
	}
	if asked[0] > compileClientTimeoutFloor {
		t.Fatalf("the sizing dry run must use the floor, asked %v", asked[0])
	}
}

func TestCompileViaHTTP_DryRunKeepsTheFloorTimeout(t *testing.T) {
	var spent int
	ts := fakeCompileDaemon(t, &spent)
	defer ts.Close()
	var asked []time.Duration
	hc := func(d time.Duration) *http.Client { asked = append(asked, d); return ts.Client() }
	var stdout, stderr bytes.Buffer
	if code := compileViaHTTPWith(hc, ts.URL, "svc", "obligations", true, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if len(asked) != 1 || asked[0] != compileClientTimeoutFloor || spent != 0 {
		t.Fatalf("asked %v spent %d, want one floor-sized dry run", asked, spent)
	}
}

// daemonDryRun runs a real daemon-shaped chain (mcp server over a client on
// a real ingested store) and returns what `fathom compile` prints for a dry
// run through it.
func daemonDryRun(t *testing.T, history []core.CompileRun, priced bool) string {
	t.Helper()
	storePath, _ := newGuardTestStore(t)
	if len(history) > 0 {
		st, err := store.Open(storePath)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range history {
			if err := st.RecordCompileRun(context.Background(), r); err != nil {
				t.Fatal(err)
			}
		}
		st.Close()
	}
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	if priced {
		c.WithLLM(nil, defaultCompileModel, knownModelInRatePerKTok, knownModelOutRatePerKTok)
	}
	srv := mcp.NewServer(c)
	h := newHTTPHandlerLogged(srv, c, "127.0.0.1:0", nil, testDiscardLogger())
	ts := httptest.NewServer(newDaemonServer(h, daemonServerOpts{}).Handler)
	t.Cleanup(ts.Close)
	var stdout, stderr bytes.Buffer
	if code := compileViaHTTP(ts.URL, "svc", "obligations", true, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	return stdout.String()
}

var tokensOutBand = regexp.MustCompile(`tokens_out:\s+unknown before a run — est\. (\d+)-(\d+)`)

func TestCompileViaHTTP_DryRunPrintsNonZeroBandFromHistory(t *testing.T) {
	isolateConfigDirs(t)
	hist := []core.CompileRun{{
		RunID: "h1", ServiceID: "svc", Layer: "obligations",
		TokensIn: 1000, TokensOut: 6700, RanAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}}
	out := daemonDryRun(t, hist, true)
	m := tokensOutBand.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no tokens_out band in:\n%s", out)
	}
	lo, _ := strconv.Atoi(m[1])
	hi, _ := strconv.Atoi(m[2])
	if lo <= 0 || hi < lo {
		t.Fatalf("band %d-%d is not a non-zero band:\n%s", lo, hi, out)
	}
	if !strings.Contains(out, "x6.7") || !strings.Contains(out, "service history") {
		t.Errorf("output must name the history ratio and basis:\n%s", out)
	}
	if strings.Contains(out, "cost_usd:          0") || strings.Contains(out, "est. 0.00-0.00") {
		t.Errorf("a non-empty run must not print a zero cost:\n%s", out)
	}
	if !strings.Contains(out, "cost_usd:          est. ") {
		t.Errorf("priced daemon must print an est. cost band:\n%s", out)
	}
}

func TestCompileViaHTTP_DryRunNoHistoryPrintsDefaultBand(t *testing.T) {
	isolateConfigDirs(t)
	out := daemonDryRun(t, nil, true)
	m := tokensOutBand.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no tokens_out band in:\n%s", out)
	}
	lo, _ := strconv.Atoi(m[1])
	hi, _ := strconv.Atoi(m[2])
	if lo <= 0 || hi != lo*4 {
		t.Fatalf("band %d-%d is not the documented default x2-x8:\n%s", lo, hi, out)
	}
	if !strings.Contains(out, "default band") {
		t.Errorf("output must say the default band was used:\n%s", out)
	}
}

func TestCompileViaHTTP_UnpricedDaemonPrintsUnpricedNotZero(t *testing.T) {
	isolateConfigDirs(t)
	out := daemonDryRun(t, nil, false)
	if !strings.Contains(out, "cost_usd:          unpriced") {
		t.Errorf("an unpriced daemon must print unpriced:\n%s", out)
	}
	if strings.Contains(out, "cost_usd:          0") || strings.Contains(out, "0.00-0.00") {
		t.Errorf("never a zero cost:\n%s", out)
	}
}

// The --store path reads the same history (EW-74): a fixture store whose
// completed runs ran at 6.7x yields 6.7x.
func TestCmdCompile_StoreDryRunUsesServiceHistoryRatio(t *testing.T) {
	isolateConfigDirs(t)
	clearProfileEnv(t)
	storePath, _ := newGuardTestStore(t)
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordCompileRun(context.Background(), core.CompileRun{
		RunID: "h1", ServiceID: "svc", Layer: "obligations",
		TokensIn: 1000, TokensOut: 6700, RanAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	st.Close()
	var stdout, stderr bytes.Buffer
	if code := cmdCompile([]string{"--service", "svc", "--layer", "obligations", "--store", storePath, "--dry-run"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "input x6.7") || !strings.Contains(out, "service history") {
		t.Errorf("--store dry run must use the 6.7 history ratio:\n%s", out)
	}
}
