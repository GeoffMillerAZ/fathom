package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func TestPushPreflightsAllRefsBeforeJudgment(t *testing.T) {
	dir, base := committedPolicy(t)
	fixtureWrite(t, dir, "AGENTS.md", "Use declared rules.\n")
	good := fixtureCommit(t, dir)
	fixtureWrite(t, dir, "data.json", "invalid")
	bad := fixtureCommit(t, dir)
	rows := fmt.Sprintf("refs/heads/good %s refs/heads/good %s\nrefs/heads/bad %s refs/heads/bad %s\n", good, base, bad, base)
	calls := 0
	model := func(context.Context, policy, string, string, snapshot, modelOptions) modelResult {
		calls++
		return modelResult{}
	}
	for _, observe := range []bool{false, true} {
		args := []string{"pre-push", "--policy-ref", base, "--format", "json", "--allow-remote"}
		if observe {
			args = append(args, "--observe")
		}
		rc, out, stderr := invoke(t, dir, rows, model, args...)
		want := 1
		if observe {
			want = 0
		}
		if rc != want || calls != 0 {
			t.Fatalf("observe=%v rc=%d calls=%d out=%s err=%s", observe, rc, calls, out, stderr)
		}
		var reports []report
		if json.Unmarshal([]byte(out), &reports) != nil || len(reports) != 2 || reports[0].Model.Reason != "push-structural-preflight-failed" {
			t.Fatal(out)
		}
	}
}

func TestInvocationSharesOneBatchBudgetAcrossRefs(t *testing.T) {
	dir, base := committedPolicy(t)
	fixtureWrite(t, dir, "AGENTS.md", "Use declared rules.\n")
	one := fixtureCommit(t, dir)
	fixtureWrite(t, dir, "AGENTS.md", "Use declared rules and inspect evidence.\n")
	two := fixtureCommit(t, dir)
	rows := fmt.Sprintf("refs/heads/one %s refs/heads/one %s\nrefs/heads/two %s refs/heads/two %s\n", one, base, two, base)
	dispatches := 0
	model := func(_ context.Context, p policy, _, _ string, _ snapshot, opts modelOptions) modelResult {
		if opts.Budget == nil {
			t.Fatal("CLI omitted invocation budget")
		}
		ordinal, ok := opts.Budget.reserve()
		if ok {
			dispatches++
			return modelResult{Provider: p.Model.Provider, RemoteAttempted: true, BatchOrdinal: ordinal, BatchLimit: opts.Budget.limit}
		}
		return modelResult{Provider: p.Model.Provider, Reason: "batch-budget-exhausted", BatchLimit: opts.Budget.limit}
	}
	rc, out, stderr := invoke(t, dir, rows, model, "pre-push", "--policy-ref", base, "--format", "json")
	if rc != 0 || dispatches != 1 {
		t.Fatalf("rc=%d dispatches=%d out=%s err=%s", rc, dispatches, out, stderr)
	}
	var reports []report
	if json.Unmarshal([]byte(out), &reports) != nil || !reports[0].Model.RemoteAttempted || reports[1].Model.RemoteAttempted || reports[1].Model.Reason != "batch-budget-exhausted" {
		t.Fatal(out)
	}
}

func TestBoundaryCannotPassWithNoRequiredPolicy(t *testing.T) {
	dir, _ := committedPolicy(t)
	p, err := decodePolicy(defaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	for i := range p.Rules {
		p.Rules[i].Boundaries = []string{"pre-commit"}
	}
	for i := range p.Model.Sites {
		p.Model.Sites[i].Boundaries = []string{"ci"}
	} // shadow-only is observation
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	fixtureWrite(t, dir, ".quality-gate/policy.json", string(b))
	base := fixtureCommit(t, dir)
	rc, out, stderr := invoke(t, dir, "", offlineModel, "check", "--boundary", "ci", "--base", base, "--revision", base, "--policy-ref", base, "--format", "json")
	if rc != 2 || !strings.Contains(out, "boundary has no required rules or hold sites") {
		t.Fatalf("rc=%d %s %s", rc, out, stderr)
	}
	rc, out, _ = invoke(t, dir, "", offlineModel, "check", "--boundary", "ci", "--base", base, "--revision", base, "--policy-ref", base, "--observe", "--format", "json")
	if rc != 0 || !strings.Contains(out, `"state": "observed"`) {
		t.Fatalf("observe: %d %s", rc, out)
	}
}

func TestCoverageReportsUnselectedPathsAndApplicability(t *testing.T) {
	dir, _ := committedPolicy(t)
	fixtureWrite(t, dir, "notes.txt", "Outside the default content selectors.\n")
	fixtureGit(t, dir, nil, "add", "notes.txt")
	rc, out, stderr := invoke(t, dir, "", offlineModel, "check", "--boundary", "pre-commit", "--staged", "--format", "json")
	if rc != 0 {
		t.Fatalf("%d %s %s", rc, out, stderr)
	}
	var r report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Coverage.RequiredRules) != 2 || len(r.Coverage.UnselectedPaths) != 1 || r.Coverage.UnselectedPaths[0] != "notes.txt" {
		t.Fatal(out)
	}
	for _, rule := range r.Rules {
		if rule.State != "not-applicable" || rule.Reason != "no-selected-changes" {
			t.Fatal(out)
		}
	}
}

func TestShadowOnlyObserveCollectsWithoutEnforcing(t *testing.T) {
	dir, _ := committedPolicy(t)
	p, _ := decodePolicy(defaultPolicy)
	p.Rules = []rule{}
	p.Model.Sites[0].Select = selector{Prefixes: []string{"data.json"}}
	b, _ := json.Marshal(p)
	fixtureWrite(t, dir, ".quality-gate/policy.json", string(b))
	base := fixtureCommit(t, dir)
	fixtureWrite(t, dir, "data.json", `{"changed":true}`)
	fixtureGit(t, dir, nil, "add", "data.json")
	calls := 0
	model := func(_ context.Context, p policy, _, _ string, _ snapshot, _ modelOptions) modelResult {
		calls++
		return modelResult{Provider: p.Model.Provider, Sites: []siteResult{{ID: p.Model.Sites[0].ID, Mode: "shadow", Outcome: "concern"}}}
	}
	rc, _, _ := invoke(t, dir, "", model, "check", "--boundary", "pre-commit", "--staged")
	if rc != 2 || calls != 0 {
		t.Fatal("shadow-only enforcement dispatched", rc, calls)
	}
	rc, out, stderr := invoke(t, dir, "", model, "check", "--boundary", "pre-commit", "--staged", "--observe", "--format", "json")
	if rc != 0 || calls != 1 || !strings.Contains(out, `"outcome": "concern"`) {
		t.Fatalf("check observation not collected: %d %d %s %s", rc, calls, out, stderr)
	}
	rev := fixtureCommit(t, dir)
	row := fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", rev, base)
	calls = 0
	rc, _, _ = invoke(t, dir, row, model, "pre-push", "--policy-ref", base)
	if rc != 2 || calls != 0 {
		t.Fatal("shadow-only push enforcement dispatched", rc, calls)
	}
	rc, out, stderr = invoke(t, dir, row, model, "pre-push", "--policy-ref", base, "--observe", "--format", "json")
	if rc != 0 || calls != 1 || !strings.Contains(out, `"outcome": "concern"`) {
		t.Fatalf("push observation not collected: %d %d %s %s", rc, calls, out, stderr)
	}
	deletion := fmt.Sprintf("(delete) %s refs/heads/main %s\n", strings.Repeat("0", 40), rev)
	rc, out, _ = invoke(t, dir, deletion, model, "pre-push", "--policy-ref", base, "--format", "json")
	if rc != 2 || !strings.Contains(out, noRequiredBoundaryReason) {
		t.Fatalf("deletion bypassed empty boundary: %d %s", rc, out)
	}
}

func TestPushObservationRetainsEachRefOutcome(t *testing.T) {
	dir, base := committedPolicy(t)
	fixtureWrite(t, dir, "data.json", `{"ok":true}`)
	good := fixtureCommit(t, dir)
	fixtureWrite(t, dir, "data.json", "invalid")
	bad := fixtureCommit(t, dir)
	rows := fmt.Sprintf("refs/heads/good %s refs/heads/good %s\nrefs/heads/bad %s refs/heads/bad %s\n", good, base, bad, base)
	rc, out, stderr := invoke(t, dir, rows, offlineModel, "pre-push", "--policy-ref", base, "--observe", "--format", "json")
	var reports []report
	if rc != 0 || json.Unmarshal([]byte(out), &reports) != nil || len(reports) != 2 {
		t.Fatalf("%d %s %s", rc, out, stderr)
	}
	if reports[0].Reason != "enforcement exit would be 0" || reports[1].Reason != "enforcement exit would be 1" {
		t.Fatal("aggregate result misattributed to individual refs", reports)
	}
}

func TestPushPreflightsRequiredModelSourceBeforeAnyDispatch(t *testing.T) {
	dir, _ := committedPolicy(t)
	p, _ := decodePolicy(defaultPolicy)
	p.Model.Sites[0].Select = selector{Prefixes: []string{"src/"}}
	p.Model.Sites[0].Mode = "hold"
	b, _ := json.Marshal(p)
	fixtureWrite(t, dir, ".quality-gate/policy.json", string(b))
	base := fixtureCommit(t, dir)
	fixtureWrite(t, dir, "src/good.txt", "A regular source.\n")
	good := fixtureCommit(t, dir)
	fixtureWrite(t, dir, "src/binary.txt", string([]byte{0, 1, 2}))
	bad := fixtureCommit(t, dir)
	rows := fmt.Sprintf("refs/heads/good %s refs/heads/good %s\nrefs/heads/bad %s refs/heads/bad %s\n", good, base, bad, base)
	calls := 0
	model := func(context.Context, policy, string, string, snapshot, modelOptions) modelResult {
		calls++
		return modelResult{}
	}
	rc, out, stderr := invoke(t, dir, rows, model, "pre-push", "--policy-ref", base, "--format", "json", "--allow-remote")
	if rc != 2 || calls != 0 {
		t.Fatalf("required source unknown dispatched: %d %d %s %s", rc, calls, out, stderr)
	}
	var reports []report
	if json.Unmarshal([]byte(out), &reports) != nil || len(reports) != 2 || reports[0].State != "unknown" || !strings.Contains(reports[0].Reason, "skipped") {
		t.Fatal("unchecked required site claimed pass", out)
	}
}

func TestModelInputPreparationNeverDispatches(t *testing.T) {
	setModelEnv(t)
	p, s := modelFixture("hold")
	stub := &modelStub{response: stubAnswer("concern")}
	opts := modelOptions{PrepareOnly: true, AllowRemote: true, Budget: newModelBudget(p.Model)}
	r := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, stub)
	if stub.calls != 0 || opts.Budget.used != 0 || r.RemoteAttempted || len(r.Sites) != 1 || r.Sites[0].Outcome != "pending" || r.Sites[0].Held {
		t.Fatal("preflight performed judgment", r, stub.calls)
	}
	p.Model.MaxInputBytes = 1
	r = evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, stub)
	if stub.calls != 0 || len(r.Sites) != 1 || !r.Sites[0].Held || r.Sites[0].Reason != "input-limit" {
		t.Fatal("preflight omitted input bound", r, stub.calls)
	}
}

func TestShadowSourceUnknownIsAdvisoryAndDoesNotDispatchItself(t *testing.T) {
	dir, _ := committedPolicy(t)
	p, _ := decodePolicy(defaultPolicy)
	p.Model.Provider = "openai"
	p.Model.MaxBatches = 2
	p.Model.Sites[0].ID = "source"
	p.Model.Sites[0].Select = selector{Prefixes: []string{"src/"}}
	p.Model.Sites[0].Mode = "shadow"
	t.Setenv(p.Model.ModelEnv, "pinned")
	t.Setenv(p.Model.ExpectedModelEnv, "pinned")
	b, _ := json.Marshal(p)
	fixtureWrite(t, dir, ".quality-gate/policy.json", string(b))
	base := fixtureCommit(t, dir)
	fixtureWrite(t, dir, "src/good.txt", "Regular input.\n")
	good := fixtureCommit(t, dir)
	fixtureWrite(t, dir, "src/bad.txt", string([]byte{0, 1, 2}))
	bad := fixtureCommit(t, dir)
	rows := fmt.Sprintf("refs/heads/good %s refs/heads/good %s\nrefs/heads/bad %s refs/heads/bad %s\n", good, base, bad, base)
	stub := &modelStub{response: stubAnswer("concern")}
	model := func(ctx context.Context, p policy, boundary, digest string, s snapshot, opts modelOptions) modelResult {
		return evaluateModelsWith(ctx, p, boundary, digest, s, opts, stub)
	}
	rc, out, stderr := invoke(t, dir, rows, model, "pre-push", "--policy-ref", base, "--format", "json", "--allow-remote")
	var reports []report
	if rc != 0 || stub.calls != 1 || json.Unmarshal([]byte(out), &reports) != nil || len(reports) != 2 {
		t.Fatalf("shadow advisory: %d calls=%d %s %s", rc, stub.calls, out, stderr)
	}
	if reports[0].Model.Sites[0].Outcome != "concern" || reports[1].Model.Sites[0].Outcome != "unknown" || reports[1].Model.Sites[0].Held || reports[1].Model.RemoteAttempted {
		t.Fatal(out)
	}
}

func TestRequiredModelReceiptCannotDisappearOrForgeClear(t *testing.T) {
	dir, commit := committedPolicy(t)
	fixtureWrite(t, dir, "data.json", `{"changed":true}`)
	fixtureGit(t, dir, nil, "add", "data.json")
	p, _ := decodePolicy(defaultPolicy)
	site := modelSite{ID: "required", Version: "1", Question: "Is there a concrete concern?", Boundaries: []string{"pre-commit"}, Select: selector{Prefixes: []string{"data.json"}}, Mode: "hold", MinConfidence: .8}
	p.Model.Sites = []modelSite{site}
	p.Model.Provider = "openai"
	p.Model.ExpectedModelEnv = "QUALITY_EXCELLENCE_EXPECTED"
	t.Setenv(p.Model.ExpectedModelEnv, "pinned")
	s, err := (gitSource{Dir: dir}).staged(context.Background(), p.Limits)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(defaultPolicy)
	digest := hex.EncodeToString(h[:])
	confidence := .99
	clear := siteResult{ID: "required", Mode: "hold", Outcome: "clear", Confidence: &confidence, Calibration: "pass"}
	for _, tc := range []struct {
		name  string
		sites []siteResult
	}{
		{"omitted", nil},
		{"duplicate", []siteResult{clear, clear}},
		{"forged-clear-without-calibration", []siteResult{clear}},
		{"missing-confidence", []siteResult{{ID: "required", Mode: "hold", Outcome: "clear", Calibration: "pass"}}},
		{"silenced-unknown", []siteResult{{ID: "required", Mode: "hold", Outcome: "unknown", Held: false}}},
		{"silenced-concern", []siteResult{{ID: "required", Mode: "hold", Outcome: "concern", Held: false}}},
		{"wrong-mode", []siteResult{{ID: "required", Mode: "shadow", Outcome: "clear", Confidence: &confidence, Calibration: "pass"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := func(context.Context, policy, string, string, snapshot, modelOptions) modelResult {
				return modelResult{Provider: "openai", Model: "pinned", Sites: tc.sites}
			}
			_, code := assess(context.Background(), gitSource{Dir: dir}, p, options{boundary: "pre-commit", staged: true, policyRef: "HEAD"}, commit, digest, s, nil, model)
			if code != 2 {
				t.Fatalf("missing/forged required receipt returned %d", code)
			}
		})
	}
	// The combiner also accepts a genuinely bound clear using the same contract.
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	c := &calibration{Site: p.ID + "/pre-commit/required", Provider: "openai", ResolvedModel: "pinned", ValidUntil: now.Add(time.Hour), MinPerClass: 1, MinN: 30, MinAccuracy: .9, MinWilsonLo: .9}
	for i := 0; i < 120; i++ {
		c.Cases = append(c.Cases, labeledCase{ID: fmt.Sprintf("clear-%d", i), Expected: "clear", Actual: "clear", Confidence: .99, Provenance: "human"})
	}
	for _, label := range []string{"concern", "insufficient"} {
		c.Cases = append(c.Cases, labeledCase{ID: label, Expected: label, Actual: label, Confidence: .99, Provenance: "human"})
	}
	p.Model.Sites[0].Calibration = c
	c.QuestionSHA256 = siteDigest(p.Model.Sites[0], p.Model.Sites...)
	model := func(context.Context, policy, string, string, snapshot, modelOptions) modelResult {
		return modelResult{Provider: "openai", Model: "pinned", Sites: []siteResult{clear}}
	}
	_, code := assess(context.Background(), gitSource{Dir: dir}, p, options{boundary: "pre-commit", staged: true, policyRef: "HEAD", now: func() time.Time { return now }}, commit, digest, s, nil, model)
	if code != 0 {
		t.Fatalf("bound clear refused: %d", code)
	}
}

func TestPolicyBatchLimitBounds(t *testing.T) {
	for _, n := range []int{0, -1, 33} {
		data := strings.Replace(string(defaultPolicy), `"max_batches": 1`, fmt.Sprintf(`"max_batches": %d`, n), 1)
		if _, err := decodePolicy([]byte(data)); err == nil {
			t.Fatalf("accepted max_batches=%d", n)
		}
	}
	data := strings.Replace(string(defaultPolicy), `    "max_batches": 1,`+"\n", "", 1)
	p, err := decodePolicy([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := newModelBudget(p.Model); b.limit != 1 {
		t.Fatal("omitted batch limit is not one")
	}
}

func TestModelCohortOrderingMatchesRecipe(t *testing.T) {
	setModelEnv(t)
	p, s := modelFixture("shadow")
	b := p.Model.Sites[0]
	b.ID = "aaa"
	p.Model.Sites = append(p.Model.Sites, b)
	stub := &modelStub{response: decision.Response{Provider: "openai", Model: "pinned"}}
	evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true}, stub)
	if len(stub.request.Questions) != 2 || stub.request.Questions[0].ID != "aaa" {
		t.Fatal("request cohort not canonical", stub.request.Questions)
	}
	first := decision.RequestKey(stub.request)
	p.Model.Sites[0], p.Model.Sites[1] = p.Model.Sites[1], p.Model.Sites[0]
	evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true}, stub)
	if decision.RequestKey(stub.request) != first {
		t.Fatal("site reorder changed effective request under same canonical recipe")
	}
}

func TestModelSourceFailureReasonIsDeterministic(t *testing.T) {
	p, s := modelFixture("hold")
	s.Changes = []change{
		{Path: "src/z", AfterOID: "z", AfterMode: "120000", After: []byte("symlink")},
		{Path: "src/a", AfterOID: "a", AfterMode: "100644", After: []byte{0}},
	}
	for i := 0; i < 30; i++ {
		r := evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{}, nil)
		if len(r.Sites) != 1 || r.Sites[0].Reason != "binary-source" {
			t.Fatal("unstable source failure", r)
		}
	}
}
