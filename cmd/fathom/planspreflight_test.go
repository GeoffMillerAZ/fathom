package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

// preflightEnv isolates the ledgers a preflight reads and writes.
func preflightTestEnv(t *testing.T) (ledger string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(dir, "verify.jsonl"))
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(dir, "sessions.jsonl"))
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", filepath.Join(dir, "rules.jsonl"))
	ledger = filepath.Join(dir, "preflight.jsonl")
	t.Setenv("FATHOM_PREFLIGHT_LEDGER", ledger)
	return ledger
}

type scoreFake struct {
	name  string
	calls int
	score float64
}

func (f *scoreFake) Name() string { return f.name }
func (f *scoreFake) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	f.calls++
	id := req.Questions[0].ID
	return decision.Response{Provider: f.name, Model: "m-1", Answers: map[string]decision.Answer{id: {ID: id, Kind: decision.KindScore, Score: f.score, Confidence: 1}}}, nil
}

// WI-17 / WI-18: `plans context --preflight` prints the row's profile and
// the advisory tier from the rule table; the decision port's Score only on
// --score and only with a provider and the site allowed.
func TestPlansContextPreflight(t *testing.T) {
	preflightTestEnv(t)
	storePath := seedPlansFixtureStore(t)
	pf := writeTempPlan(t, "# C\n\n- [ ] A row {id: T-1, symbol: validateCharge, weight: 2}\n- [ ] Ghost {id: T-2, symbol: NoSuchThingXYZ}\n")

	var out, errBuf bytes.Buffer
	if code := run([]string{"plans", "context", pf, "--row", "T-1", "--service", "payment-service", "--store", storePath}, &out, &errBuf); code != 0 || strings.Contains(out.String(), "preflight") {
		t.Fatalf("without --preflight the pack is unchanged: exit %d\n%s", code, out.String())
	}

	out.Reset()
	if code := run([]string{"plans", "context", pf, "--row", "T-1", "--service", "payment-service", "--store", storePath, "--preflight"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{
		"preflight (advisory: fathom recommends and the harness decides",
		"blast radius: 1 transitive caller",
		"plan weight 2",
		"not evaluated T7",
		"decision port score: not asked",
		"fathom plans outcome " + pf + " --row T-1 --service payment-service",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "recommended tier: light") {
		t.Errorf("unmeasured rules never certify a light tier:\n%s", got)
	}

	out.Reset()
	run([]string{"plans", "context", pf, "--row", "T-2", "--service", "payment-service", "--store", storePath, "--preflight"}, &out, &errBuf)
	if !strings.Contains(out.String(), `does not resolve in the index`) {
		t.Errorf("an anchor that does not resolve is named:\n%s", out.String())
	}

	out.Reset()
	run([]string{"plans", "context", pf, "--row", "T-1", "--service", "payment-service", "--preflight"}, &out, &errBuf)
	if !strings.Contains(out.String(), "needs direct mode") {
		t.Errorf("the daemon path names why it prints no preflight:\n%s", out.String())
	}
}

func TestPlansContextPreflightScore(t *testing.T) {
	preflightTestEnv(t)
	storePath := seedPlansFixtureStore(t)
	pf := writeTempPlan(t, "# C\n\n- [ ] A row {id: T-1, symbol: validateCharge}\n")
	saved := preflightOpenPorts
	t.Cleanup(func() { preflightOpenPorts = saved })

	args := []string{"plans", "context", pf, "--row", "T-1", "--service", "payment-service", "--store", storePath, "--score"}
	fake := &scoreFake{name: "fake", score: 1.4}
	var log bytes.Buffer
	preflightOpenPorts = func(io.Writer) *decide.Ports {
		return &decide.Ports{Decider: fake, Cfg: config.Decide{Provider: "llm", Sites: []string{decide.SitePreflight}}, Recorder: decide.NewRecorder(&log, time.Now, "test")}
	}
	var out, errBuf bytes.Buffer
	if code := run(args, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	if fake.calls != 1 || !strings.Contains(out.String(), "decision port score: standard") || !strings.Contains(out.String(), "--score-tier standard --score-provider fake") {
		t.Fatalf("one call, the Score shown, and carried into the outcome command: calls %d\n%s", fake.calls, out.String())
	}
	if !strings.Contains(log.String(), `"site":"preflight"`) {
		t.Fatalf("the ruling is logged: %q", log.String())
	}

	// No provider: nothing is asked, and the reason is printed.
	none := &scoreFake{name: "none"}
	preflightOpenPorts = func(io.Writer) *decide.Ports {
		return &decide.Ports{Decider: none, Cfg: config.Decide{Provider: "none", Sites: []string{decide.SitePreflight}}}
	}
	out.Reset()
	run(args, &out, &errBuf)
	if none.calls != 0 || !strings.Contains(out.String(), "decision port score: not shown — [decide] has no provider") {
		t.Fatalf("provider none: no call, named: calls %d\n%s", none.calls, out.String())
	}
}

// WI-19: `plans outcome` appends one identity-keyed line and refuses what
// it cannot record.
func TestPlansOutcome(t *testing.T) {
	ledger := preflightTestEnv(t)
	pf := writeTempPlan(t, "- [ ] A row {id: T-1}\n")
	var out, errBuf bytes.Buffer
	code := run([]string{
		"plans", "outcome", pf, "--row", "T-1", "--service", "svc", "--result", "proven", "--harness", "claude-code", "--model", "model-a", "--effort", "high",
		"--tier", "standard", "--score-tier", "heavy", "--score-provider", "fake", "--first-attempt", "true",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	raw, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	var o console.PreflightOutcome
	if err := json.Unmarshal(bytes.TrimSpace(raw), &o); err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	if o.Agent != "claude-code|model-a|high" || o.Result != "proven" || o.Tier != console.TierStandard || o.ScoreTier != console.TierHeavy || o.FirstAttempt == nil || !*o.FirstAttempt {
		t.Fatalf("the ledger row: %+v", o)
	}

	// A person is withheld, never keyed.
	errBuf.Reset()
	out.Reset()
	if code := run([]string{"plans", "outcome", pf, "--row", "T-2", "--service", "svc", "--result", "unproven", "--model", "Jane <jane@example.com>"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	if raw, _ := os.ReadFile(ledger); strings.Contains(string(raw), "jane") || strings.Contains(out.String(), "jane") {
		t.Fatalf("a person's email is never recorded or echoed:\n%s\n%s", raw, out.String())
	}

	before, _ := os.ReadFile(ledger)
	for name, args := range map[string][]string{
		"no result":        {"plans", "outcome", pf, "--row", "T-1", "--service", "svc"},
		"an unknown state": {"plans", "outcome", pf, "--row", "T-1", "--service", "svc", "--result", "maybe"},
		"an unknown tier":  {"plans", "outcome", pf, "--row", "T-1", "--service", "svc", "--result", "proven", "--tier", "gigantic"},
		"a bad first":      {"plans", "outcome", pf, "--row", "T-1", "--service", "svc", "--result", "proven", "--first-attempt", "perhaps"},
	} {
		if code := run(args, &out, &errBuf); code == 0 {
			t.Errorf("%s must fail", name)
		}
	}
	if after, _ := os.ReadFile(ledger); string(after) != string(before) {
		t.Fatal("a refused outcome writes nothing")
	}
}

// WI-17: the plan row page shows the row's preflight — profile, advisory
// tier with the rules that fired, calibration — and never asks the
// decision port or offers a control that starts, assigns or routes work.
func TestConsolePlanRowPreflight(t *testing.T) {
	t.Setenv("FATHOM_PREFLIGHT_LEDGER", filepath.Join(t.TempDir(), "preflight.jsonl"))
	f := newConsoleFixture(t)
	code, _, page := f.get(t, "/plans/proof-service/settlement/rows/S-2")
	if code != http.StatusOK {
		t.Fatalf("row page = %d", code)
	}
	mustContain(t, "S-2 preflight", page, `id="row-preflight"`, "Task preflight", "data-preflight-tier", "data-preflight-advisory",
		"the harness decides", "Decision port Score: not asked here", "uncalibrated: no outcome is recorded yet",
		`data-console-copy="fathom plans context docs/plans/settlement.md --row S-2 --service proof-service`) // the verb chip names the CLI that prints it
	start := strings.Index(page, `id="row-preflight"`)
	panel := page[start : start+strings.Index(page[start:], "data-preflight-advisory")+200]
	if strings.Contains(panel, "<form") || strings.Contains(strings.ToLower(panel), "dispatch") {
		t.Error("the preflight panel offers no control that starts, assigns or routes work")
	}
	if regexp.MustCompile(`\b(bg|text|border)-(red|green|blue|gray|slate|zinc)-\d`).MatchString(panel) || strings.Contains(panel, "dark:") {
		t.Error("role utilities only (cmd/fathom/CLAUDE.md)")
	}

	env := f.pageJSON(t, "/plans/proof-service/settlement/rows/S-2")
	var m planRowModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatalf("model: %v", err)
	}
	if m.Preflight == nil {
		t.Fatalf("the JSON model carries the preflight (or says why not): %q", m.PreflightErr)
	}
	if !m.Preflight.Recommendation.Advisory || m.Preflight.Profile.Row.ID != "S-2" || m.Preflight.Score.State != console.StateAbsent {
		t.Fatalf("advisory, for this row, no Score: %+v", m.Preflight)
	}

	_, _, frag := f.get(t, "/plans/proof-service/settlement/rows/S-2?format=fragment")
	if strings.Contains(frag, "row-preflight") {
		t.Error("the drawer's fragment stays the row's evidence; the preflight is the page's")
	}
}
