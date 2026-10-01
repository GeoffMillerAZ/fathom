package main

// assess_test.go — DO-3 / IR-3 / IR-4 (docs/design/assess-lanes.md,
// ADR-046). Every provider here is a scripted fake: no test reaches a
// network or spends a token.

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/assess"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

const (
	assessDiffSentinel       = "DIFF-SENTINEL-4c1e"
	assessTranscriptSentinel = "TRANSCRIPT-SENTINEL-7f3a"
)

// assessGitAt runs git in root with both author and committer dates pinned.
func assessGitAt(t *testing.T, root string, at time.Time, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
	stamp := at.UTC().Format(time.RFC3339)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func assessWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type assessFixture struct {
	root, sid, ledger string
	now               time.Time
}

// newAssessFixture builds a repository with one session's unit of work:
// an old seed commit OUTSIDE the window, beats bounding the window, a
// commit inside it that flips a plan row, a verify-ledger row, a session
// ledger row, and a transcript on disk carrying a sentinel a transcript
// miner would find. Every ledger is redirected into temp dirs.
func newAssessFixture(t *testing.T) assessFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// cmdAssess sets these two for isolation; t.Setenv restores whatever
	// was here at cleanup, so the mutation never leaks into other tests.
	t.Setenv("FATHOM_CHANNELS", "")
	t.Setenv(assess.SpendLabelEnv, "")
	t.Setenv("FATHOM_STATE_DIR", t.TempDir())

	now := time.Now().UTC().Truncate(time.Second)
	root := t.TempDir()
	sid := "sess-assess-1"
	assessGitAt(t, root, now.Add(-2*time.Hour), "init", "-q")
	assessWrite(t, root, "docs/plans/p.md", "# p\n\n- [ ] widget exists {id: W-1}\n- [ ] gadget exists {id: W-2}\n")
	assessWrite(t, root, "a.go", "package a\n")
	assessWrite(t, root, "b.go", "package a\n\nfunc B() {}\n")
	assessWrite(t, root, "c.go", "package a\n")
	assessGitAt(t, root, now.Add(-2*time.Hour), "add", "-A")
	assessGitAt(t, root, now.Add(-2*time.Hour), "commit", "-q", "-m", "seed OUTSIDE-WINDOW-SENTINEL")

	// The unit's work, committed inside the window.
	assessWrite(t, root, "w.go", "package a\n\n// "+assessDiffSentinel+"\nfunc Widget() {}\n")
	assessWrite(t, root, "a.go", "package a\n\nimport _ \"net/http\"\n")
	assessWrite(t, root, "docs/plans/p.md", "# p\n\n- [x] widget exists {id: W-1}\n- [ ] gadget exists {id: W-2}\n")
	assessGitAt(t, root, now.Add(-90*time.Second), "add", "-A")
	assessGitAt(t, root, now.Add(-90*time.Second), "commit", "-q", "-m", "feat: widget")
	// b.go modified and never committed; c.go untouched.
	assessWrite(t, root, "b.go", "package a\n\nfunc B() error { return nil }\n")

	hooks := filepath.Join(t.TempDir(), "hooks.jsonl")
	lines := []string{
		retroBeatLine(now.Add(-3*time.Minute), "edit-hook", "answered", map[string]string{
			"emitted": "true", "phase": "pre", "target": "file", "session": sid, "file": filepath.Join(root, "a.go"), "project_dir": root,
		}),
		retroBeatLine(now.Add(-2*time.Minute), "edit-hook", "answered", map[string]string{
			"emitted": "true", "phase": "post", "session": sid, "file": filepath.Join(root, "b.go"), "project_dir": root,
		}),
		retroBeatLine(now.Add(-110*time.Second), "edit-hook", "answered", map[string]string{
			"emitted": "true", "phase": "pre", "target": "file", "session": sid, "file": filepath.Join(root, "c.go"), "project_dir": root,
		}),
		retroBeatLine(now.Add(-100*time.Second), "edit-hook", "answered", map[string]string{
			"emitted": "true", "phase": "post", "project_dir": root,
		}), // pre-IR-1: no stamps
		retroBeatLine(now.Add(-30*time.Second), "discovery", "grep", map[string]string{"session": sid, "project_dir": root}),
	}
	if err := os.WriteFile(hooks, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_HOOKS_LOG", hooks)

	vl := filepath.Join(t.TempDir(), "verify-ledger.jsonl")
	vrow, _ := json.Marshal(verifyLedgerRow{SchemaVersion: 2, At: now.Add(-60 * time.Second).Format(time.RFC3339), RepoRoot: root,
		Plan: "docs/plans/p.md", Claimed: 1, Verified: 1, Origin: "cli"})
	if err := os.WriteFile(vl, append(vrow, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_VERIFY_LEDGER", vl)

	sl := filepath.Join(t.TempDir(), "session-ledger.jsonl")
	zero := 0
	srow, _ := json.Marshal(sessionLedgerEntry{SchemaVersion: 2, SessionID: sid, Repo: "fixture", At: now.Format(time.RFC3339), VerifyCatches: &zero, TokensNote: "fixture"})
	if err := os.WriteFile(sl, append(srow, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_SESSION_LEDGER", sl)

	// Where a transcript miner would look — assess must never read it.
	assessWrite(t, home, filepath.Join(".claude", "projects", "fixture", sid+".jsonl"),
		`{"type":"assistant","message":{"content":"`+assessTranscriptSentinel+`"}}`+"\n")

	ledger := filepath.Join(t.TempDir(), "assess-ledger.jsonl")
	t.Setenv("FATHOM_ASSESS_LEDGER", ledger)
	return assessFixture{root: root, sid: sid, ledger: ledger, now: now}
}

func (f assessFixture) enable(t *testing.T) {
	t.Helper()
	assessWrite(t, f.root, ".fathom/capabilities.cue", "capabilities: {\n\tassess: on: true\n}\n")
}

// fakeAssessPorts swaps the decision-port opener for a scripted decider and
// records the environment the port was opened under (the isolation order).
type fakeAssessPorts struct {
	mock        *decision.MockDecider
	cfg         config.Decide
	openedEnv   map[string]string
	openedCalls int
}

func installFakeAssessPorts(t *testing.T, f *fakeAssessPorts) {
	t.Helper()
	orig := assessOpenPorts
	assessOpenPorts = func(string, io.Writer) *decide.Ports {
		f.openedCalls++
		f.openedEnv = map[string]string{
			"FATHOM_CHANNELS":    os.Getenv("FATHOM_CHANNELS"),
			assess.SpendLabelEnv: os.Getenv(assess.SpendLabelEnv),
		}
		var d decision.Decider = decision.None{}
		if f.mock != nil {
			d = f.mock
		}
		return &decide.Ports{Decider: d, Cfg: f.cfg}
	}
	t.Cleanup(func() { assessOpenPorts = orig })
}

func unitAnswers(model string) decision.Response {
	return decision.Response{
		Provider: "llm", Model: model,
		Answers: map[string]decision.Answer{
			"coherent":        {ID: "coherent", Kind: decision.KindPredicate, Noul: 0.96, Confidence: 0.96},
			"claims_backed":   {ID: "claims_backed", Kind: decision.KindPredicate, Noul: 0.93, Confidence: 0.93},
			"tests_accompany": {ID: "tests_accompany", Kind: decision.KindPredicate, Noul: 0.2, Confidence: 0.8},
			"quality":         {ID: "quality", Kind: decision.KindChoice, Choice: "adequate", Probabilities: map[string]float64{"adequate": 0.9, "solid": 0.1}, Confidence: 0.875},
		},
		Usage: decision.Usage{InputTokens: 1200, OutputTokens: 80},
	}
}

func readAssessLedger(t *testing.T, p string) []assess.Row {
	t.Helper()
	rows, err := assess.ReadRows(p)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// DO-3: the benchmark lane judges the unit's work products on disk — the
// commit's diff, the plan row it flipped, the gate result — and NEVER the
// transcript; the run is explicit, typed, isolated, and reports its own
// per-run token cost.
func TestCmdAssess_JudgesWorkProductsNeverTranscripts(t *testing.T) {
	f := newAssessFixture(t)
	f.enable(t)
	fake := &fakeAssessPorts{
		mock: decision.NewMockDecider("llm", decision.MockEntry{Response: unitAnswers(defaultCompileModel)}),
		cfg:  config.Decide{Provider: "llm", Sites: decide.AllSites},
	}
	installFakeAssessPorts(t, fake)

	var out, errb bytes.Buffer
	code := cmdAssess([]string{"--session", f.sid, "--project", f.root}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, out.String(), errb.String())
	}
	calls := fake.mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("Decide called %d times, want exactly one batched call", len(calls))
	}
	state := calls[0].State
	for _, want := range []string{assessDiffSentinel, "flipped to done", "W-1", "claimed 1, verified 1, failing 0", "session verify catches: 0"} {
		if !strings.Contains(state, want) {
			t.Errorf("the judged state is missing the work product %q:\n%s", want, state)
		}
	}
	for _, never := range []string{assessTranscriptSentinel, "OUTSIDE-WINDOW-SENTINEL", "func B() error"} {
		if strings.Contains(state, never) {
			t.Errorf("the judged state carries %q — only the unit's COMMITTED work products may be sent", never)
		}
	}
	for _, q := range calls[0].Questions {
		if q.Kind != decision.KindPredicate && q.Kind != decision.KindChoice {
			t.Errorf("question %s is %q: every judgment is a typed noul or choice", q.ID, q.Kind)
		}
	}

	// IR-4 order: the isolation environment is in place BEFORE the port
	// opens, so a CLI provider's judge child inherits it.
	if fake.openedEnv["FATHOM_CHANNELS"] != "off" || fake.openedEnv[assess.SpendLabelEnv] != assess.LabelUnit {
		t.Errorf("port opened under env %v, want FATHOM_CHANNELS=off and %s=%s", fake.openedEnv, assess.SpendLabelEnv, assess.LabelUnit)
	}

	s := out.String()
	for _, want := range []string{
		"cost: 1,200 in / 80 out tokens",
		"$0.0016",
		"uncalibrated",
		"coherent", "verdict",
		"tests_accompany", "leaning",
		assess.IsolationRegistration,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}

	rows := readAssessLedger(t, f.ledger)
	if len(rows) != 1 {
		t.Fatalf("assess ledger rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.SpendLabel != assess.LabelUnit || r.Lane != assess.LaneUnit || r.Isolation != assess.IsolationRegistration {
		t.Errorf("row labels = %+v", r)
	}
	if r.TokensIn == nil || *r.TokensIn != 1200 || r.CostUSD == nil || *r.CostUSD != 0.0016 {
		t.Errorf("row spend = in %v cost %v", r.TokensIn, r.CostUSD)
	}
	if r.Calibrated {
		t.Error("v1 has no calibrated question set; a row must never claim calibration")
	}
	if r.QuestionSet != assess.UnitQuestionSetDigest() || r.Proceed != 2 || r.Confirm != 2 {
		t.Errorf("row verdict tallies = %+v", r)
	}

	// The session ledger is read, never written: judging a unit never adds
	// to that unit's figures (IR-4 rule 4).
	data, _ := os.ReadFile(os.Getenv("FATHOM_SESSION_LEDGER"))
	if strings.Count(string(data), "\n") != 1 {
		t.Errorf("assess wrote to the session ledger:\n%s", data)
	}
}

// A unit that landed nothing has nothing on disk to judge: no call, no
// spend, exit 0, said plainly.
func TestCmdAssess_NothingLandedSpendsNothing(t *testing.T) {
	f := newAssessFixture(t)
	f.enable(t)
	fake := &fakeAssessPorts{mock: decision.NewMockDecider("llm"), cfg: config.Decide{Provider: "llm", Sites: decide.AllSites}}
	installFakeAssessPorts(t, fake)
	// A different session whose window holds no commit.
	hooks := os.Getenv("FATHOM_HOOKS_LOG")
	extra := retroBeatLine(f.now.Add(-20*time.Hour), "discovery", "grep", map[string]string{"session": "sess-idle", "project_dir": f.root})
	fh, _ := os.OpenFile(hooks, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = fh.WriteString(extra + "\n")
	fh.Close()

	var out, errb bytes.Buffer
	if code := cmdAssess([]string{"--session", "sess-idle", "--project", f.root}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if len(fake.mock.Calls()) != 0 || fake.openedCalls != 0 {
		t.Errorf("a unit with no work products opened the port (%d) or called it (%d)", fake.openedCalls, len(fake.mock.Calls()))
	}
	if !strings.Contains(out.String(), "nothing on disk to judge") {
		t.Errorf("output = %s", out.String())
	}
}

// The capability is consent to spend: default off refuses before any port
// opens, and a malformed capabilities file fails CLOSED — the opposite of
// the safety capabilities, because an unreadable file is never consent.
// This is assess's case in TestEverySwitchableCapabilityHasAnEnforcementCase.
//
// Only the last case discriminates fail-closed from fail-open: with the
// user scope ON, a fail-open read of a malformed project file would resolve
// to the user's ON and spend. (With no user setting, fail-open and
// fail-closed both land on the default off.)
func TestSwitchOffAssessRefusesAndFailsClosed(t *testing.T) {
	const malformed = "capabilities: {\n\tassess: on?: true\n}\n"
	for _, tc := range []struct {
		name, user, cfg string
	}{
		{"default off (no config at all)", "", ""},
		{"explicitly off", "", "capabilities: {\n\tassess: on: false\n}\n"},
		{"malformed config fails closed", "", malformed},
		{"malformed project file fails closed over a user-scope ON", "capabilities: {\n\tassess: on: true\n}\n", malformed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAssessFixture(t)
			if tc.user != "" {
				assessWrite(t, os.Getenv("HOME"), ".fathom/capabilities.cue", tc.user)
			}
			if tc.cfg != "" {
				assessWrite(t, f.root, ".fathom/capabilities.cue", tc.cfg)
			}
			fake := &fakeAssessPorts{mock: decision.NewMockDecider("llm", decision.MockEntry{Response: unitAnswers(defaultCompileModel)}), cfg: config.Decide{Provider: "llm", Sites: decide.AllSites}}
			installFakeAssessPorts(t, fake)
			var out, errb bytes.Buffer
			if code := cmdAssess([]string{"--session", f.sid, "--project", f.root}, &out, &errb); code == 0 {
				t.Fatalf("exit 0 with assess not switched on:\n%s", out.String())
			}
			if fake.openedCalls != 0 || len(fake.mock.Calls()) != 0 {
				t.Error("the decision port was opened or called without consent")
			}
			if !strings.Contains(errb.String(), "fathom capabilities set assess on") {
				t.Errorf("refusal must name the switch: %s", errb.String())
			}
			if tc.cfg == malformed && !strings.Contains(errb.String(), "fails CLOSED") {
				t.Errorf("a malformed capabilities file must be refused as unreadable, not read as off: %s", errb.String())
			}
			if rows := readAssessLedger(t, f.ledger); len(rows) != 0 {
				t.Errorf("a refused run wrote %d ledger rows", len(rows))
			}
			if os.Getenv("FATHOM_CHANNELS") == "off" {
				t.Error("a refused run still flipped the isolation env — it must change nothing")
			}
		})
	}
}

// The provider and the site are the egress arm (I14, ADR-042): provider
// none refuses with the remedy, and a narrowed [decide] sites list that
// omits assess refuses by name. Neither calls anything.
func TestCmdAssess_ProviderNoneAndNarrowedSiteRefuse(t *testing.T) {
	t.Run("provider none", func(t *testing.T) {
		f := newAssessFixture(t)
		f.enable(t)
		fake := &fakeAssessPorts{cfg: config.Decide{Provider: "none", Sites: decide.AllSites}}
		installFakeAssessPorts(t, fake)
		var out, errb bytes.Buffer
		if code := cmdAssess([]string{"--session", f.sid, "--project", f.root}, &out, &errb); code == 0 {
			t.Fatal("exit 0 with no decision provider")
		}
		if !strings.Contains(errb.String(), "[decide] provider") {
			t.Errorf("refusal must name the [decide] provider remedy: %s", errb.String())
		}
	})
	t.Run("site narrowed away", func(t *testing.T) {
		f := newAssessFixture(t)
		f.enable(t)
		mock := decision.NewMockDecider("llm", decision.MockEntry{Response: unitAnswers(defaultCompileModel)})
		fake := &fakeAssessPorts{mock: mock, cfg: config.Decide{Provider: "llm", Sites: []string{decide.SiteRoute}}}
		installFakeAssessPorts(t, fake)
		var out, errb bytes.Buffer
		if code := cmdAssess([]string{"--session", f.sid, "--project", f.root}, &out, &errb); code == 0 {
			t.Fatal("exit 0 with assess outside [decide] sites")
		}
		if len(mock.Calls()) != 0 || !strings.Contains(errb.String(), "sites") {
			t.Errorf("calls=%d stderr=%s", len(mock.Calls()), errb.String())
		}
	})
}

// --dry-run shows exactly what would be sent and spends nothing — no
// capability, no provider, no ledger row needed.
func TestCmdAssess_DryRunShowsRequestAndSpendsNothing(t *testing.T) {
	f := newAssessFixture(t)
	fake := &fakeAssessPorts{}
	installFakeAssessPorts(t, fake)
	var out, errb bytes.Buffer
	if code := cmdAssess([]string{"--session", f.sid, "--project", f.root, "--dry-run"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if fake.openedCalls != 0 {
		t.Error("a dry run opened the decision port")
	}
	s := out.String()
	for _, want := range []string{assessDiffSentinel, "coherent", "estimated", "tokens", "question set " + assess.UnitQuestionSetDigest(), "nothing was sent"} {
		if !strings.Contains(s, want) {
			t.Errorf("dry run missing %q:\n%s", want, s)
		}
	}
	if rows := readAssessLedger(t, f.ledger); len(rows) != 0 {
		t.Errorf("a dry run wrote %d ledger rows", len(rows))
	}
}

// The pre-flight token cap refuses before any port opens and names the
// estimate.
func TestCmdAssess_TokenCapRefusesBeforeTheCall(t *testing.T) {
	f := newAssessFixture(t)
	f.enable(t)
	fake := &fakeAssessPorts{mock: decision.NewMockDecider("llm"), cfg: config.Decide{Provider: "llm", Sites: decide.AllSites}}
	installFakeAssessPorts(t, fake)
	var out, errb bytes.Buffer
	if code := cmdAssess([]string{"--session", f.sid, "--project", f.root, "--max-tokens", "10"}, &out, &errb); code == 0 {
		t.Fatal("exit 0 over the token cap")
	}
	if fake.openedCalls != 0 || !strings.Contains(errb.String(), "--max-tokens") {
		t.Errorf("opened=%d stderr=%s", fake.openedCalls, errb.String())
	}
}

// A provider failure fabricates no verdict: the error is recorded on the
// ledger, the exit is 2 (operational), and the output says nothing was
// judged.
func TestCmdAssess_ProviderErrorRecordsNoVerdict(t *testing.T) {
	f := newAssessFixture(t)
	f.enable(t)
	mock := decision.NewMockDecider("llm", decision.MockEntry{Err: errors.New("provider unreachable")})
	installFakeAssessPorts(t, &fakeAssessPorts{mock: mock, cfg: config.Decide{Provider: "llm", Sites: decide.AllSites}})
	var out, errb bytes.Buffer
	if code := cmdAssess([]string{"--session", f.sid, "--project", f.root}, &out, &errb); code != 2 {
		t.Fatalf("exit %d, want 2 (operational)", code)
	}
	rows := readAssessLedger(t, f.ledger)
	if len(rows) != 1 || rows[0].Error == "" || len(rows[0].Verdicts) != 0 || rows[0].Proceed != 0 {
		t.Fatalf("ledger = %+v", rows)
	}
	if !strings.Contains(errb.String(), "nothing was judged") {
		t.Errorf("stderr = %s", errb.String())
	}
}

// IR-3: the intervention lane is capability-gated (default off) and sends
// ONLY the rows mechanics cannot decide — a committed or
// modified-uncommitted file after a firing — each with its on-disk
// guidance and provenance; an untouched file and an unstamped firing are
// named, never asked; the per-run cost is in the output.
func TestCmdAssessInterventions_CapabilityGatedUndecidableOnly(t *testing.T) {
	origG := assessGuidance
	assessGuidance = func(root, rel, channel string) (string, string, bool) {
		switch rel {
		case "a.go":
			return "never import net/http in package a", "guard note in .fathom/guard.cue at assess time", true
		case "b.go":
			return "return errors, never swallow them", "obligations served for the file at assess time", true
		}
		return "", "", false
	}
	t.Cleanup(func() { assessGuidance = origG })

	t.Run("default off refuses", func(t *testing.T) {
		f := newAssessFixture(t)
		mock := decision.NewMockDecider("llm")
		installFakeAssessPorts(t, &fakeAssessPorts{mock: mock, cfg: config.Decide{Provider: "llm", Sites: decide.AllSites}})
		var out, errb bytes.Buffer
		if code := cmdAssess([]string{"--interventions", "--session", f.sid, "--project", f.root}, &out, &errb); code == 0 {
			t.Fatal("the intervention lane ran with its capability off")
		}
		if len(mock.Calls()) != 0 {
			t.Error("Decide called without consent")
		}
	})

	t.Run("on: judges the undecidable rows only", func(t *testing.T) {
		f := newAssessFixture(t)
		f.enable(t)
		mock := decision.NewMockDecider("llm", decision.MockEntry{Response: decision.Response{
			Provider: "llm", Model: defaultCompileModel,
			Answers: map[string]decision.Answer{
				"F01": {ID: "F01", Kind: decision.KindChoice, Choice: "departed", Probabilities: map[string]float64{"departed": 0.95, "complied": 0.05}, Confidence: 0.9333},
				"F02": {ID: "F02", Kind: decision.KindChoice, Choice: "complied", Probabilities: map[string]float64{"complied": 0.6, "cannot_tell": 0.4}, Confidence: 0.4667},
			},
			Usage: decision.Usage{InputTokens: 900, OutputTokens: 40},
		}})
		fake := &fakeAssessPorts{mock: mock, cfg: config.Decide{Provider: "llm", Sites: decide.AllSites}}
		installFakeAssessPorts(t, fake)

		var out, errb bytes.Buffer
		if code := cmdAssess([]string{"--interventions", "--session", f.sid, "--project", f.root}, &out, &errb); code != 0 {
			t.Fatalf("exit %d\n%s\n%s", code, out.String(), errb.String())
		}
		calls := mock.Calls()
		if len(calls) != 1 {
			t.Fatalf("Decide calls = %d", len(calls))
		}
		qs := calls[0].Questions
		if len(qs) != 2 {
			t.Fatalf("questions = %d, want 2 (a.go committed, b.go modified-uncommitted)", len(qs))
		}
		if !strings.Contains(qs[0].Instructions, "a.go") || !strings.Contains(qs[1].Instructions, "b.go") {
			t.Errorf("questions not about a.go then b.go: %q / %q", qs[0].Instructions, qs[1].Instructions)
		}
		state := calls[0].State
		for _, want := range []string{"never import net/http in package a", "guard note in .fathom/guard.cue", "net/http", "func B() error"} {
			if !strings.Contains(state, want) {
				t.Errorf("state missing %q:\n%s", want, state)
			}
		}
		if strings.Contains(state, "c.go") || strings.Contains(state, assessTranscriptSentinel) {
			t.Errorf("state carries an untouched file or the transcript:\n%s", state)
		}
		if fake.openedEnv[assess.SpendLabelEnv] != assess.LabelInterventions {
			t.Errorf("spend label at open = %q", fake.openedEnv[assess.SpendLabelEnv])
		}
		s := out.String()
		for _, want := range []string{"c.go", "not judgeable from disk", "unjoinable", "cost: 900 in / 40 out tokens", "departed", "verdict", "undecided"} {
			if !strings.Contains(s, want) {
				t.Errorf("output missing %q:\n%s", want, s)
			}
		}
		rows := readAssessLedger(t, f.ledger)
		if len(rows) != 1 || rows[0].SpendLabel != assess.LabelInterventions || rows[0].Lane != assess.LaneInterventions {
			t.Fatalf("ledger = %+v", rows)
		}
	})
}

// `fathom assess spend` totals the assess ledger through the compile spend
// meter's aggregation, grouped by spend label, and says when a total is a
// lower bound because a run's model had no price.
func TestCmdAssessSpend_TotalsThroughTheSpendMeter(t *testing.T) {
	p := filepath.Join(t.TempDir(), "assess-ledger.jsonl")
	t.Setenv("FATHOM_ASSESS_LEDGER", p)
	in, out := 1000, 100
	usd := 0.0015
	now := time.Now().UTC()
	for _, r := range []assess.Row{
		{At: now.Add(-time.Hour).Format(time.RFC3339), RunID: "a", SpendLabel: assess.LabelUnit, Model: defaultCompileModel, TokensIn: &in, TokensOut: &out, CostUSD: &usd},
		{At: now.Add(-time.Hour).Format(time.RFC3339), RunID: "b", SpendLabel: assess.LabelInterventions, Model: "jev-1.13", TokensIn: &in, TokensOut: &out, CostNote: "cost unknown"},
	} {
		if err := assess.AppendRow(p, r); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := cmdAssess([]string{"spend"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	s := stdout.String()
	for _, want := range []string{"assess ", "assess-interventions", "runs=1", "$0.0015", "1 run(s) with no known price", "lower bound"} {
		if !strings.Contains(s, want) {
			t.Errorf("spend output missing %q:\n%s", want, s)
		}
	}
}

// A run whose cost is unknown is never printed as a zero cost (design §1):
// a failed call may still have been billed, and a provider that reported
// no usage contributes neither tokens nor cost. Each group and the total
// say so, and each kind of unknown is named with what it does and does not
// contribute.
func TestCmdAssessSpend_UnknownCostIsNeverPrintedAsZero(t *testing.T) {
	p := filepath.Join(t.TempDir(), "assess-ledger.jsonl")
	t.Setenv("FATHOM_ASSESS_LEDGER", p)
	now := time.Now().UTC()
	at := now.Add(-time.Hour).Format(time.RFC3339)
	in, out := 1000, 100
	usd := 0.0015
	for _, r := range []assess.Row{
		{At: at, RunID: "ok", SpendLabel: assess.LabelUnit, Model: defaultCompileModel, TokensIn: &in, TokensOut: &out, CostUSD: &usd},
		{At: at, RunID: "failed", SpendLabel: assess.LabelUnit, Error: "context deadline exceeded", TokensNote: "unknown: the call failed", CostNote: "unknown: the call failed"},
		{At: at, RunID: "nousage", SpendLabel: assess.LabelInterventions, TokensNote: "not reported by the provider", CostNote: "unknown: the provider reported no token usage"},
	} {
		if err := assess.AppendRow(p, r); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := cmdAssess([]string{"spend"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	s := stdout.String()
	if strings.Contains(s, "cost=$0.0000") {
		t.Errorf("an unknown cost printed as a zero:\n%s", s)
	}
	for _, want := range []string{
		"cost≥$0.0015 (1 of 2 run(s) unknown)", // the unit group: one priced, one failed
		"cost=unknown",                         // the interventions group: nothing known
		"total: runs=3 cost≥$0.0015",
		"1 failed run(s)", "may still have been billed",
		"1 run(s) whose provider reported no token usage", "neither tokens nor cost",
		"lower bound",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("spend output missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "contribute tokens but no cost") {
		t.Errorf("no run here has tokens without a price — that line misdescribes them:\n%s", s)
	}

	stdout.Reset()
	if code := cmdAssess([]string{"spend", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var js struct {
		CostComplete    bool `json:"cost_complete"`
		UnknownCostRuns int  `json:"unknown_cost_runs"`
		FailedRuns      int  `json:"failed_runs"`
		NoUsageRuns     int  `json:"no_usage_runs"`
		UnpricedRuns    int  `json:"unpriced_runs"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &js); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	if js.CostComplete || js.UnknownCostRuns != 2 || js.FailedRuns != 1 || js.NoUsageRuns != 1 || js.UnpricedRuns != 0 {
		t.Errorf("json cost accounting = %+v", js)
	}
}

// IR-4, pre-registered 2026-09-25 before any assess run: judging spend never
// enters an arm unit, a thread-bet unit, or the flywheel's admissible arm
// rows — counted and named where it is excluded — and the Stop hook stamps
// the spend label onto any row written inside a judge child.
func TestAssessSpendNeverEntersArmOrBetUnits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "arm.json"))
	ledger := filepath.Join(t.TempDir(), "session-ledger.jsonl")
	t.Setenv("FATHOM_SESSION_LEDGER", ledger)
	now := time.Now().UTC()

	ip := func(n int) *int { return &n }
	rows := []sessionLedgerEntry{
		{SchemaVersion: 2, SessionID: "s1", Arm: "on", At: now.Add(-3 * time.Hour).Format(time.RFC3339), TokensIn: ip(1000), TokensOut: ip(100), TurnCount: ip(20), SessionSource: "startup", BriefInjected: true, FirstEditTurn: ip(4)},
		{SchemaVersion: 2, SessionID: "s2", Arm: "on", At: now.Add(-2 * time.Hour).Format(time.RFC3339), TokensIn: ip(2000), TokensOut: ip(200), TurnCount: ip(20), SessionSource: "resume", FirstEditTurn: ip(6)},
		// A judge child's row that slipped past FATHOM_CHANNELS=off: its
		// tokens are the cost of judging, not of working.
		{SchemaVersion: 2, SessionID: "s3", Arm: "on", SpendLabel: assess.LabelUnit, At: now.Add(-time.Hour).Format(time.RFC3339), TokensIn: ip(900000), TokensOut: ip(9000), TurnCount: ip(12), SessionSource: "startup", BriefInjected: true, FirstEditTurn: ip(1)},
	}
	var buf bytes.Buffer
	for _, r := range rows {
		b, _ := json.Marshal(r)
		buf.Write(append(b, '\n'))
	}
	if err := os.WriteFile(ledger, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	imp := buildImpactModel(now)
	if imp.TokensOnSessions != 2 {
		t.Errorf("ON arm units = %d, want 2 — the judge row must not be a unit", imp.TokensOnSessions)
	}
	wantMed, _ := medianInt([]int{1100 / 20, 2200 / 20})
	if imp.TokensOnMedian == nil || *imp.TokensOnMedian != wantMed {
		t.Errorf("ON median = %v, want %d over the two working sessions — judging spend leaked into the arm figure", imp.TokensOnMedian, wantMed)
	}
	if imp.IsolatedSpendRows != 1 || !strings.Contains(imp.IsolatedSpendNote, "assess") {
		t.Errorf("isolated rows = %d note %q — the exclusion must be counted and named", imp.IsolatedSpendRows, imp.IsolatedSpendNote)
	}

	bet := threadBetCheck(dedupedLedgerRows())
	if bet.FreshUnits != 1 || bet.IsolatedSpendRows != 1 {
		t.Errorf("bet fresh units = %d isolated = %d, want 1 and 1", bet.FreshUnits, bet.IsolatedSpendRows)
	}
	// Counted AND named: every surface that renders the bet says a judge
	// row was excluded — the verdict detail (which the flywheel prints) and
	// the brief's continuity section, whose brief-byte sum is no longer
	// "all sessions".
	if !strings.Contains(bet.Detail, "judging spend") || !strings.Contains(bet.IsolatedSpendNote, "assess") {
		t.Errorf("bet detail %q / note %q — the exclusion must be named, not only counted", bet.Detail, bet.IsolatedSpendNote)
	}
	threads := filepath.Join(t.TempDir(), "threads.jsonl")
	ti, _ := json.Marshal(threadItem{SchemaVersion: 1, ID: "t1", Thread: "main", Kind: "note", Text: "open work", FiledAt: now.Format(time.RFC3339)})
	if err := os.WriteFile(threads, append(ti, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_THREADS", threads)
	if ev := checkLoopContinuity().Evidence; !strings.Contains(ev, "judging spend") {
		t.Errorf("flywheel continuity evidence = %q — the bet's exclusion must be named there too", ev)
	}
	md := renderBriefMD(observatoryModel{})
	sec6 := md[strings.Index(md, "## 6 "):]
	if strings.Contains(sec6, "across all sessions") || !strings.Contains(sec6, "Excluded as judging spend: 1") {
		t.Errorf("brief section 6 must name the excluded judge row beside a sum that no longer covers all sessions:\n%s", sec6)
	}

	// The flywheel's arm loop: an OFF arm with one clean OFF row and one
	// judge row stamped off-and-full — only the first is admissible.
	st := armState{SchemaVersion: 1, Arm: "off", SetAt: now.Add(-5 * time.Hour).Format(time.RFC3339)}
	sb, _ := json.Marshal(st)
	if err := os.WriteFile(os.Getenv("FATHOM_ARM_STATE"), sb, 0o600); err != nil {
		t.Fatal(err)
	}
	lrows := []ledgerRow{
		{At: now.Add(-time.Hour).Format(time.RFC3339), Arm: "off", ArmCoverage: "full"},
		{At: now.Add(-time.Hour).Format(time.RFC3339), Arm: "off", ArmCoverage: "full", SpendLabel: assess.LabelUnit},
	}
	c := checkLoopArm(lrows, obsImpact{}, now)
	if !strings.Contains(c.Evidence, "1 admissible") || !strings.Contains(c.Evidence, "judging spend") {
		t.Errorf("flywheel arm evidence = %q — the judge row must be excluded and named", c.Evidence)
	}

	// The Stop hook stamps the label from the environment a judge child
	// inherits.
	if err := os.WriteFile(ledger, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(assess.SpendLabelEnv, assess.LabelInterventions)
	recordSessionLedgerEntry("claude-code", "s4", "", t.TempDir(), now, nil)
	got := dedupedLedgerRows()
	if len(got) != 1 || got[0].SpendLabel != assess.LabelInterventions {
		t.Fatalf("stop-hook row = %+v, want spend_label %q", got, assess.LabelInterventions)
	}
	if !spendIsolatedRow(got[0]) {
		t.Error("a labelled row is not isolated")
	}
}

// The egress allowlist that governs a run is the ASSESSED repository's,
// never the caller's working directory's (I14, ADR-044 decision 8: a site
// not listed makes no call at all). Repo R narrows [decide] sites to
// forbid the lane; `fathom assess --project R` run from another directory —
// one whose own config would allow it — must still refuse before anything
// reaches the provider. This drives the REAL port opener: the user config
// names a claude-cli profile whose binary is a local capture script (no
// model), so a leak is a file on disk, not a spend.
func TestCmdAssess_ProjectSitesGovernFromAForeignCwd(t *testing.T) {
	f := newAssessFixture(t)
	clearProfileEnv(t)
	t.Setenv("FATHOM_CALL_LOG", filepath.Join(t.TempDir(), "calls.jsonl"))
	home := os.Getenv("HOME")

	capture := filepath.Join(t.TempDir(), "captured-stdin")
	bin := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ncat > '"+capture+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	assessWrite(t, home, ".fathom/config.toml",
		"[decide]\nprovider = \"llm\"\nprofile = \"judge\"\n\n[profiles.judge]\nprovider = \"claude-cli\"\nbin = \""+bin+"\"\n")
	assessWrite(t, home, ".fathom/capabilities.cue", "capabilities: {\n\tassess: on: true\n}\n")
	// The assessed repository forbids the lane.
	assessWrite(t, f.root, ".fathom/config.toml", "[decide]\nsites = [\"route\"]\n")
	// The caller's cwd is a different repository whose config would allow it.
	foreign := t.TempDir()
	assessWrite(t, foreign, ".fathom/config.toml", "[decide]\nsites = [\"route\", \"assess\"]\n")
	t.Chdir(foreign)

	var out, errb bytes.Buffer
	code := cmdAssess([]string{"--session", f.sid, "--project", f.root}, &out, &errb)
	if data, err := os.ReadFile(capture); err == nil {
		t.Fatalf("the judge received %d bytes although %s's [decide] sites forbids assess (leaked %v)",
			len(data), f.root, strings.Contains(string(data), assessDiffSentinel))
	}
	if code != 1 || !strings.Contains(errb.String(), "[decide] sites") {
		t.Fatalf("exit %d, want 1 with the site refusal\nstdout:\n%s\nstderr:\n%s", code, out.String(), errb.String())
	}
	if rows := readAssessLedger(t, f.ledger); len(rows) != 0 {
		t.Errorf("a refused run wrote %d ledger rows", len(rows))
	}
}

// IR-3 asks only about a change it can SHOW: the state holds F's diff
// inside the window (design §1). The retro's outcome is wider than that
// diff — `committed` counts commits made after the window, and
// `modified-uncommitted` includes an untracked new file, which `git diff
// HEAD` never shows. A firing whose windowed diff is empty is named
// unjudgeable and never asked; an untracked new file is diffed against
// /dev/null so its content is the evidence.
func TestCmdAssessInterventions_EmptyWindowDiffIsNeverAsked(t *testing.T) {
	f := newAssessFixture(t)
	origG := assessGuidance
	assessGuidance = func(root, rel, channel string) (string, string, bool) {
		return "keep package a free of network imports", "guard note in .fathom/guard.cue at assess time", true
	}
	t.Cleanup(func() { assessGuidance = origG })

	// A past session, five hours ago: a guard fired on a.go (committed only
	// later, outside this window) and on n.go (created, never committed).
	const newFileSentinel = "NEWFILE-SENTINEL-91d2"
	assessWrite(t, f.root, "n.go", "package a\n\n// "+newFileSentinel+"\n")
	old := "sess-old"
	at := f.now.Add(-5 * time.Hour)
	hooks := os.Getenv("FATHOM_HOOKS_LOG")
	fh, err := os.OpenFile(hooks, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"a.go", "n.go"} {
		_, _ = fh.WriteString(retroBeatLine(at, "edit-hook", "answered", map[string]string{
			"emitted": "true", "phase": "pre", "target": "file", "session": old, "file": filepath.Join(f.root, rel), "project_dir": f.root,
		}) + "\n")
	}
	fh.Close()

	var out, errb bytes.Buffer
	if code := cmdAssess([]string{"--interventions", "--dry-run", "--session", old, "--project", f.root}, &out, &errb); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out.String(), errb.String())
	}
	s := out.String()
	if strings.Contains(s, "did the change to a.go") {
		t.Errorf("a.go was asked with an empty windowed diff (its commits fall outside the window):\n%s", s)
	}
	if !strings.Contains(s, "a.go") || !strings.Contains(s, "no change to the file inside the session window") {
		t.Errorf("the empty-diff row must be named unjudgeable, not dropped:\n%s", s)
	}
	if !strings.Contains(s, "did the change to n.go") || !strings.Contains(s, newFileSentinel) {
		t.Errorf("an untracked new file must be asked with its content as the diff:\n%s", s)
	}
}

// The --json envelope mirrors the ledger row, so it carries the row's
// schema version — never a 0 no ledger row has. The dry run's envelope has
// the same row shape and the same version.
func TestCmdAssess_JSONCarriesTheRowSchemaVersion(t *testing.T) {
	f := newAssessFixture(t)
	f.enable(t)
	installFakeAssessPorts(t, &fakeAssessPorts{
		mock: decision.NewMockDecider("llm", decision.MockEntry{Response: unitAnswers(defaultCompileModel)}),
		cfg:  config.Decide{Provider: "llm", Sites: decide.AllSites},
	})
	for _, args := range [][]string{
		{"--session", f.sid, "--project", f.root, "--json"},
		{"--session", f.sid, "--project", f.root, "--json", "--dry-run"},
	} {
		var out, errb bytes.Buffer
		if code := cmdAssess(args, &out, &errb); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, errb.String())
		}
		var got struct {
			SchemaVersion int `json:"schema_version"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out.String())
		}
		if got.SchemaVersion != assess.RowSchemaVersion {
			t.Errorf("%v: schema_version = %d, want %d (the ledger row's)", args, got.SchemaVersion, assess.RowSchemaVersion)
		}
	}
	if rows := readAssessLedger(t, f.ledger); len(rows) != 1 || rows[0].SchemaVersion != assess.RowSchemaVersion {
		t.Fatalf("ledger = %+v", rows)
	}
}

// DLP before egress (design §1) covers what this lane actually sends: raw
// commit patches. A credential committed inside the window leaves as a
// labelled placeholder, and the dry run — which shows exactly what a live
// run sends — prints the redaction count (text and JSON).
func TestCmdAssess_DLPRedactsCommittedCredentialsBeforeEgress(t *testing.T) {
	const aws, gh = "AKIAIOSFODNN7EXAMPLE", "ghp_0123456789abcdefghijklmnopqrstuvwxyzAB"
	f := newAssessFixture(t)
	assessWrite(t, f.root, "k.go", "package a\n\nconst k = \""+aws+"\"\nconst g = \""+gh+"\"\n")
	assessGitAt(t, f.root, f.now.Add(-80*time.Second), "add", "k.go")
	assessGitAt(t, f.root, f.now.Add(-80*time.Second), "commit", "-q", "-m", "feat: keys")

	var out, errb bytes.Buffer
	if code := cmdAssess([]string{"--session", f.sid, "--project", f.root, "--dry-run"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	s := out.String()
	if strings.Contains(s, aws) || strings.Contains(s, gh) {
		t.Fatalf("a committed credential would leave the machine unredacted:\n%s", s)
	}
	for _, want := range []string{"[REDACTED:AWS_ACCESS_KEY_ID]", "[REDACTED:GITHUB_TOKEN]", "redacted 2 span(s)"} {
		if !strings.Contains(s, want) {
			t.Errorf("dry run missing %q:\n%s", want, s)
		}
	}

	out.Reset()
	if code := cmdAssess([]string{"--session", f.sid, "--project", f.root, "--dry-run", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var js struct {
		Redactions *int   `json:"redactions"`
		State      string `json:"state"`
	}
	if err := json.Unmarshal(out.Bytes(), &js); err != nil {
		t.Fatal(err)
	}
	if js.Redactions == nil || *js.Redactions != 2 || strings.Contains(js.State, aws) {
		t.Errorf("dry-run JSON redactions = %v (want 2), state leaks = %v", js.Redactions, strings.Contains(js.State, aws))
	}
}
