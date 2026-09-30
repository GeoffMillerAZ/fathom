package main

// claim_learn_test.go — GL-13 (docs/plans/governance-learning.md, design
// §6): TestLearningPromotesAndRetires is the row's proof anchor
// (reachable_from). It reaches semantic.applyLearning through the verb,
// `fathom claim learn`, over a real store, the rule-outcome log the hooks
// write and a `.fathom/learning.cue` that overrides one threshold.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

type claimLearnFixture struct {
	storePath, root, logPath string
	now                      time.Time
}

type claimLearnSeed struct {
	id        string
	lifecycle core.ClaimLifecycle
}

type claimLearnRow struct {
	ago          time.Duration
	claim, event string
	session      string
}

func newClaimLearnFixture(t *testing.T, service string, claims []claimLearnSeed, rows []claimLearnRow) claimLearnFixture {
	t.Helper()
	f := claimLearnFixture{
		storePath: filepath.Join(t.TempDir(), "learn.sqlite"),
		root:      t.TempDir(),
		logPath:   filepath.Join(t.TempDir(), "rule-outcomes.jsonl"),
		now:       time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC),
	}
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", f.logPath)
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(t.TempDir(), "intent-ledger.jsonl"))
	orig := learningNow
	learningNow = func() time.Time { return f.now }
	t.Cleanup(func() { learningNow = orig })

	// .fathom/learning.cue lowers promotion to two corroborations; the
	// other four thresholds are the binary's defaults.
	if err := os.MkdirAll(filepath.Join(f.root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(f.root, ".fathom", "learning.cue"), "promote_corroborations: 2\n")

	st, err := store.Open(f.storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: service, Path: f.root}); err != nil {
		t.Fatal(err)
	}
	minted := f.now.AddDate(-1, 0, 0)
	var cs []core.SemanticClaim
	for _, c := range claims {
		cs = append(cs, core.SemanticClaim{
			ServiceID: service, ClaimID: c.id, Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
			Subject: c.id + "#s", Rule: "rule " + c.id, Scope: []string{"**"}, Tier: core.TierDocClaimed,
			Status: core.ClaimFresh, CompiledAt: minted, ValidFrom: minted, Lifecycle: c.lifecycle,
		})
	}
	if err := st.PutClaims(ctx, cs, nil); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, `{"schema_version":1,"ts":%q,"rule_id":%q,"event":%q,"session":%q}`+"\n",
			f.now.Add(-r.ago).Format(time.RFC3339Nano), r.claim, r.event, r.session)
	}
	mustWrite(t, f.logPath, b.String())
	return f
}

func (f claimLearnFixture) run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(append([]string{"claim", "learn", "--store", f.storePath}, args...), &out, &errb)
	return out.String(), errb.String(), code
}

func (f claimLearnFixture) claims(t *testing.T, service string) map[string]core.SemanticClaim {
	t.Helper()
	st, err := store.Open(f.storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cs, err := st.ClaimsForService(context.Background(), service, "")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]core.SemanticClaim{}
	for _, c := range cs {
		out[c.ClaimID] = c
	}
	return out
}

const learnDay = 24 * time.Hour

func TestLearningPromotesAndRetires(t *testing.T) {
	const svc = "learnsvc"
	f := newClaimLearnFixture(t, svc,
		[]claimLearnSeed{
			{"cl-cand", core.LifecycleCandidate},
			{"cl-refuted", core.LifecycleActive},
			{"cl-never", ""}, // a compiled claim: no lifecycle recorded reads as active
			{"cl-served", core.LifecycleActive},
		},
		[]claimLearnRow{
			{200 * learnDay, "cl-served", "served", "s0"}, // the log reaches back over the 180-day window
			{10 * learnDay, "cl-served", "served", "s1"},
			{5 * learnDay, "cl-cand", "corroborated", "sess-a"},
			{4 * learnDay, "cl-cand", "corroborated", "sess-b"},
			{3 * learnDay, "cl-cand", "served", "sess-b"},
			{20 * learnDay, "cl-refuted", "refuted", "s1"},
			{19 * learnDay, "cl-refuted", "refuted", "s2"},
			{18 * learnDay, "cl-refuted", "refuted", "s3"},
			{17 * learnDay, "cl-refuted", "corroborated", "s4"},
		})

	// Dry run: every decision with its signals, nothing written.
	out, errb, code := f.run(t, "--service", svc)
	if code != 0 {
		t.Fatalf("claim learn: exit %d: %s", code, errb)
	}
	for _, want := range []string{
		"would: cl-cand candidate → active (2 corroborations from 2 distinct sessions, 0 refutes over 30d)",
		"would: cl-refuted active → retired (3 refutes > 1 corroborations)",
		"would: cl-never active → retired (never triggered for 180d)",
		"thresholds from " + filepath.Join(f.root, ".fathom", "learning.cue"),
		"dry run: nothing written",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "cl-served") {
		t.Errorf("a served active claim got a decision:\n%s", out)
	}
	if got := f.claims(t, svc); got["cl-cand"].Lifecycle != core.LifecycleCandidate || !got["cl-never"].ValidTo.IsZero() {
		t.Fatalf("a dry run wrote: %+v", got)
	}

	// --json carries the signals applyLearning measured.
	js, _, _ := f.run(t, "--service", svc, "--json")
	var rep learnReport
	if err := json.Unmarshal([]byte(js), &rep); err != nil {
		t.Fatalf("%v\n%s", err, js)
	}
	if len(rep.Services) != 1 || len(rep.Services[0].Decisions) != 3 {
		t.Fatalf("json report = %+v, want one service with three decisions", rep)
	}
	for _, d := range rep.Services[0].Decisions {
		if d.ClaimID == "cl-cand" && (d.Corroborations != 2 || d.DistinctSessions != 2) {
			t.Errorf("cl-cand signals = %+v", d)
		}
		if d.ClaimID == "cl-refuted" && (d.Refutes != 3 || d.Corroborations != 1) {
			t.Errorf("cl-refuted signals = %+v", d)
		}
	}

	// --apply: lifecycles persist, a retirement closes the validity window
	// at now and leaves status alone, and nothing is deleted.
	out, errb, code = f.run(t, "--service", svc, "--apply")
	if code != 0 {
		t.Fatalf("claim learn --apply: exit %d: %s", code, errb)
	}
	if !strings.Contains(out, "applied: cl-cand candidate → active") {
		t.Errorf("--apply output:\n%s", out)
	}
	got := f.claims(t, svc)
	if len(got) != 4 {
		t.Fatalf("claims after --apply = %d, want 4 (retirement never deletes)", len(got))
	}
	if got["cl-cand"].Lifecycle != core.LifecycleActive || !got["cl-cand"].ValidTo.IsZero() {
		t.Errorf("cl-cand = %+v, want active with an open window", got["cl-cand"])
	}
	for _, id := range []string{"cl-refuted", "cl-never"} {
		c := got[id]
		if c.Lifecycle != core.LifecycleRetired || !c.ValidTo.Equal(f.now) || c.Status != core.ClaimFresh {
			t.Errorf("%s = lifecycle %q valid_to %v status %q, want retired, closed at %v, status unchanged", id, c.Lifecycle, c.ValidTo, c.Status, f.now)
		}
	}
	if c := got["cl-served"]; c.Lifecycle != core.LifecycleActive || !c.ValidTo.IsZero() {
		t.Errorf("cl-served changed: %+v", c)
	}

	// A second pass decides nothing: retirement is terminal and the
	// promoted claim is served.
	out, _, _ = f.run(t, "--service", svc, "--apply")
	if !strings.Contains(out, "no promotion or retirement") {
		t.Errorf("second pass:\n%s", out)
	}
}

// TestLearningHoldsUnevidencedNeverTriggered: a log that reaches back 10
// days cannot say a rule went unserved for 180, so that retirement is held
// and named, never applied.
func TestLearningHoldsUnevidencedNeverTriggered(t *testing.T) {
	const svc = "learnsvc"
	f := newClaimLearnFixture(t, svc,
		[]claimLearnSeed{{"cl-never", core.LifecycleActive}, {"cl-other", core.LifecycleActive}},
		[]claimLearnRow{{10 * learnDay, "cl-other", "served", "s1"}})
	out, errb, code := f.run(t, "--service", svc, "--apply")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb)
	}
	if !strings.Contains(out, "cl-never → retired held: never triggered for 180d is not yet evidenced — the rule-outcome log reaches back 10 of the 180 days it reads") {
		t.Errorf("the unevidenced retirement is not held by name:\n%s", out)
	}
	if c := f.claims(t, svc)["cl-never"]; c.Lifecycle != core.LifecycleActive || !c.ValidTo.IsZero() {
		t.Errorf("a held retirement was applied: %+v", c)
	}
}
