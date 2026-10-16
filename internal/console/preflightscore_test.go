package console

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// fakeDecider counts its calls and answers as told.
type fakeDecider struct {
	name  string
	calls int
	req   decision.Request
	resp  decision.Response
	err   error
}

func (f *fakeDecider) Name() string { return f.name }
func (f *fakeDecider) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	f.calls++
	f.req = req
	return f.resp, f.err
}

func scoreAnswer(score float64) decision.Response {
	return decision.Response{
		Provider: "fake", Model: "fake-1", Latency: 40 * time.Millisecond,
		Usage:   decision.Usage{InputTokens: 120, OutputTokens: 3},
		Answers: map[string]decision.Answer{PreflightQuestionID: {ID: PreflightQuestionID, Kind: decision.KindScore, Score: score, Confidence: 1}},
	}
}

func portsFor(d decision.Decider, provider string, sites []string, log *bytes.Buffer) *decide.Ports {
	fixed := func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) }
	return &decide.Ports{Decider: d, Cfg: config.Decide{Provider: provider, Sites: sites}, Recorder: decide.NewRecorder(log, fixed, "test")}
}

// WI-18 (I42): the decision port's Score is asked only with [decide]
// configured and the preflight site allowed — otherwise no call is made and
// the Score is absent, saying why. With a provider it is a KindScore
// question over the profile's features alone, recorded in the decisions
// log, and shown with its calibration.
func TestPreflightScoreOnlyWithProvider(t *testing.T) {
	profile := DifficultyProfile(preflightInput())
	ctx := t.Context()

	if v := preflightScore(ctx, nil, profile); v.State != StateAbsent || !strings.Contains(v.Reason, "[decide]") {
		t.Fatalf("no port: absent, naming [decide]: %+v", v)
	}
	var log bytes.Buffer
	none := &fakeDecider{name: "none"}
	if v := preflightScore(ctx, portsFor(none, "none", []string{decide.SitePreflight}, &log), profile); v.State != StateAbsent || none.calls != 0 || log.Len() != 0 {
		t.Fatalf("provider none makes no call and writes no ruling (I42): %+v calls=%d log=%q", v, none.calls, log.String())
	}
	off := &fakeDecider{name: "fake"}
	if v := preflightScore(ctx, portsFor(off, "llm", []string{"route", "assess"}, &log), profile); v.State != StateAbsent || off.calls != 0 || !strings.Contains(v.Reason, decide.SitePreflight) {
		t.Fatalf("a site the allowlist omits sends nothing, and the reason names it: %+v calls=%d", v, off.calls)
	}

	ok := &fakeDecider{name: "fake", resp: scoreAnswer(1.4)}
	v := preflightScore(ctx, portsFor(ok, "llm", []string{decide.SitePreflight}, &log), profile)
	if v.State != StatePresent || v.V.Tier != TierStandard || v.V.Provider != "fake" || v.V.Model != "fake-1" || ok.calls != 1 {
		t.Fatalf("a provider and an allowed site: one call, the level rounds to its tier: %+v calls=%d", v, ok.calls)
	}
	if len(ok.req.Questions) != 1 || ok.req.Questions[0].Kind != decision.KindScore || len(ok.req.Questions[0].Levels) != len(preflightTiers) {
		t.Fatalf("one Score question over the tier ladder: %+v", ok.req.Questions)
	}
	if err := decision.Validate(ok.req); err != nil {
		t.Fatalf("the request is valid for every adapter: %v", err)
	}
	// Only the features go off-host: no row id, title, symbol or path.
	for _, leak := range []string{"X-1", "a row", "Thing", "internal/console", "docs/plans"} {
		if strings.Contains(ok.req.State, leak) {
			t.Fatalf("the state carries %q — only measured features may leave the machine:\n%s", leak, ok.req.State)
		}
	}
	if !strings.Contains(ok.req.State, "blast radius: transitive callers: 12") {
		t.Fatalf("the state lists the features:\n%s", ok.req.State)
	}
	if !strings.Contains(log.String(), `"site":"preflight"`) || !strings.Contains(log.String(), `"applied":false`) {
		t.Fatalf("the ruling is logged, never applied: %q", log.String())
	}

	for name, tc := range map[string]struct {
		score float64
		tier  Tier
	}{"low": {0.2, TierLight}, "high": {1.9, TierHeavy}, "top": {2, TierHeavy}} {
		d := &fakeDecider{name: "fake", resp: scoreAnswer(tc.score)}
		if got := preflightScore(ctx, portsFor(d, "llm", []string{decide.SitePreflight}, &log), profile); got.V.Tier != tc.tier {
			t.Fatalf("%s: %v -> %+v", name, tc.score, got)
		}
	}

	failing := &fakeDecider{name: "fake", err: errors.New("provider down")}
	var flog bytes.Buffer
	if v := preflightScore(ctx, portsFor(failing, "llm", []string{decide.SitePreflight}, &flog), profile); v.State != StateError || !strings.Contains(v.Reason, "provider down") || !strings.Contains(flog.String(), "provider down") {
		t.Fatalf("a failed call is an error value and a logged error row: %+v %q", v, flog.String())
	}
	invalid := &fakeDecider{name: "fake", resp: decision.Response{Provider: "fake", Invalid: map[string]string{PreflightQuestionID: "score out of range"}}}
	if v := preflightScore(ctx, portsFor(invalid, "llm", []string{decide.SitePreflight}, &log), profile); v.State != StateError || !strings.Contains(v.Reason, "score out of range") {
		t.Fatalf("an invalid answer is an error, not a tier: %+v", v)
	}
	unsure := scoreAnswer(1)
	a := unsure.Answers[PreflightQuestionID]
	a.Confidence = 0.3
	unsure.Answers[PreflightQuestionID] = a
	if v := preflightScore(ctx, portsFor(&fakeDecider{name: "fake", resp: unsure}, "llm", []string{decide.SitePreflight}, &log), profile); v.State != StateAbsent || !strings.Contains(v.Reason, "unsure") {
		t.Fatalf("an answer in the unsure band is shown as no answer (I42): %+v", v)
	}

	// Shown with its calibration: outcomes recorded against Score tiers.
	yes, no := true, false
	outcomes := []PreflightOutcome{
		{Row: "A", Result: ResultProven, FirstAttempt: &yes, Tier: TierStandard, ScoreTier: TierStandard},
		{Row: "B", Result: ResultProven, FirstAttempt: &no, Tier: TierStandard, ScoreTier: TierStandard},
		{Row: "C", Result: ResultUnproven, Tier: TierHeavy, ScoreTier: TierStandard},
	}
	with := Preflight(PreflightInput{Row: preflightRow()}, v, outcomes)
	if !with.ScoreCalibration.Known() || len(with.ScoreCalibration.V) != 1 || with.ScoreCalibration.V[0].Bucket != "standard" || with.ScoreCalibration.V[0].N != 3 || with.ScoreCalibration.V[0].Hits != 1 {
		t.Fatalf("the Score is shown with its calibration, one bucket per Score tier: %+v", with.ScoreCalibration)
	}
	if len(with.Calibration.RuleTable) != 2 {
		t.Fatalf("and the rule table's, per its own tier: %+v", with.Calibration.RuleTable)
	}
	without := Preflight(PreflightInput{Row: preflightRow()}, Value[ScoreReading]{}, outcomes)
	if without.Score.State != StateAbsent || without.ScoreCalibration.State != StateAbsent {
		t.Fatalf("no Score asked: no Score and no Score calibration: %+v %+v", without.Score, without.ScoreCalibration)
	}
	if _, err := json.Marshal(without); err != nil {
		t.Fatalf("the model marshals: %v", err)
	}
	if _, err := json.Marshal(with); err != nil {
		t.Fatalf("the model marshals with a Score: %v", err)
	}
}

// WI-19: an outcome — the model used, its effort, proven or not — is
// appended to a ledger, keyed by agent identity only (I58), and calibrates
// the preflight.
func TestRecordPreflightOutcome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "preflight.jsonl")
	fixed := func() time.Time { return time.Date(2026, 9, 28, 12, 30, 0, 0, time.UTC) }
	yes := true

	err := recordPreflightOutcome(path, PreflightOutcome{
		Service: "svc", Plan: "docs/plans/p.md", Row: "X-1", Harness: "claude-code", Model: "model-a", Effort: "high",
		Result: ResultProven, FirstAttempt: &yes, Tier: TierStandard, ScoreTier: TierHeavy, ScoreProvider: "fake",
	}, fixed)
	if err != nil {
		t.Fatal(err)
	}
	// A person's name or email never becomes a key (I58).
	if err := recordPreflightOutcome(path, PreflightOutcome{
		Service: "svc", Row: "X-2", Harness: "h", Model: "Jane Doe <jane@example.com>", Effort: "jane@example.com",
		Result: ResultUnproven, Tier: TierLight,
	}, fixed); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("one JSON line per outcome, appended: %q", raw)
	}
	if strings.Contains(string(raw), "jane") || strings.Contains(string(raw), "Jane") || strings.Contains(string(raw), "@") {
		t.Fatalf("a person is never recorded: %s", raw)
	}
	var first PreflightOutcome
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.At != "2026-09-28T12:30:00Z" || first.Agent != "claude-code|model-a|high" || first.Result != ResultProven || first.FirstAttempt == nil || !*first.FirstAttempt || first.TableVersion != TierRuleTableVersion {
		t.Fatalf("the row: %+v", first)
	}

	// Refusals write nothing.
	before, _ := os.ReadFile(path)
	for name, o := range map[string]PreflightOutcome{
		"no row":        {Service: "svc", Result: ResultProven},
		"no service":    {Row: "X", Result: ResultProven},
		"unknown state": {Service: "svc", Row: "X", Result: "maybe"},
		"unknown tier":  {Service: "svc", Row: "X", Result: ResultProven, Tier: "gigantic"},
	} {
		if err := recordPreflightOutcome(path, o, fixed); err == nil {
			t.Fatalf("%s must be refused", name)
		}
	}
	if err := recordPreflightOutcome("", PreflightOutcome{Service: "svc", Row: "X", Result: ResultProven}, fixed); err == nil {
		t.Fatal("no ledger path must be refused")
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatal("a refused outcome writes nothing")
	}

	// Reading is a bounded tail that skips a malformed line and names it.
	if err := os.WriteFile(path, append(raw, []byte("not json\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	got, read := PreflightOutcomes(t.Context(), path, 1<<20)
	if len(got) != 2 || read.State != StatePresent || !strings.Contains(read.Reason, "1 malformed") {
		t.Fatalf("read back: %d %+v", len(got), read)
	}
	if _, read := PreflightOutcomes(t.Context(), filepath.Join(t.TempDir(), "none.jsonl"), 1<<20); read.State != StateAbsent {
		t.Fatalf("no ledger yet is absent: %+v", read)
	}

	// Calibration: per recommended tier, how often a row was proven at
	// the first attempt — uncalibrated below the scorecard's n.
	cal := PreflightCalibrationOf(got)
	if cal.Outcomes != 2 || cal.Proven != 1 || cal.Unproven != 1 || cal.Calibrated {
		t.Fatalf("calibration: %+v", cal)
	}
	if !strings.Contains(cal.Note, "uncalibrated") {
		t.Fatalf("a small n says so: %q", cal.Note)
	}
	many := make([]PreflightOutcome, ScorecardMinN)
	for i := range many {
		many[i] = PreflightOutcome{Row: "R", Result: ResultProven, FirstAttempt: &yes, Tier: TierLight}
	}
	if c := PreflightCalibrationOf(many); !c.Calibrated || c.RuleTable[0].Hits != ScorecardMinN || c.RuleTable[0].Fraction != 1 {
		t.Fatalf("n reached: %+v", c)
	}
}

// The score and outcome code, like the recommendation, starts nothing:
// it may ask the decision port one question and append one ledger line.
func TestPreflightNeverDispatches(t *testing.T) {
	_, self, _, _ := runtime.Caller(0)
	for _, name := range []string{"preflight.go", "preflightscore.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(self), name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			if p := strings.Trim(imp.Path.Value, `"`); p == "os/exec" || p == "net/http" || p == "net" {
				t.Fatalf("%s imports %s", name, p)
			}
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				for _, bad := range []string{"dispatch", "assign", "route", "launch", "spawn"} {
					if strings.Contains(strings.ToLower(fn.Name.Name), bad) {
						t.Fatalf("%s declares %s", name, fn.Name.Name)
					}
				}
			}
		}
	}
}
