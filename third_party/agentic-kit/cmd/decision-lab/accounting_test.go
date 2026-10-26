package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func twoQuestionCase(t *testing.T) (Pack, Input, Capture, Manifest) {
	t.Helper()
	p := fixture[Pack](t, "review-triage.pack.json")
	q := p.Questions[0]
	q.ID = "triage2"
	p.Questions = append(p.Questions, q)
	in := fixture[Input](t, "case-1.input.json")
	pr, err := prepared(p, in)
	if err != nil {
		t.Fatal(err)
	}
	c := fixture[Capture](t, "case-1.capture.json")
	c.PackDigest = pr.PackDigest
	c.InputDigest = pr.InputDigest
	c.RequestKey = pr.RequestKey
	a := c.Response.Answers["triage"]
	a.ID = "triage2"
	c.Response.Answers["triage2"] = a
	m := Manifest{Version: "decision-lab.manifest/1", DatasetKind: "synthetic", PackDigest: pr.PackDigest, Cases: []ExpectedCase{{Input: in, IndependentUnitID: "unit-1", Labels: map[string]ExpectedLabel{"triage": {Choice: "inspect", Provenance: decision.ProvenanceFrontier}, "triage2": {Choice: "inspect", Provenance: decision.ProvenanceFrontier}}}}}
	return p, in, c, m
}

func TestMixedQuestionReuseAndFreshReading(t *testing.T) {
	p, in, c, m := twoQuestionCase(t)
	c.AnswerCacheHits = []string{"triage"}
	c.AttemptCount = 1
	if r, err := check(p, in, c); err != nil || r.Status != "usable" {
		t.Fatalf("mixed reuse rejected: %+v %v", r, err)
	}
	r, err := study(p, m, []Capture{c})
	if err != nil {
		t.Fatal(err)
	}
	if r.AnswerCacheHits != 1 || r.DeclaredAttempts != 1 || r.UsageUnreportedCaptures != 1 || r.Classes["triage/inspect"].Reading.N != 0 || r.Classes["triage2/inspect"].Reading.N != 1 {
		t.Fatalf("wrong per-question N: %+v", r)
	}
	c.AnswerCacheHits = []string{"triage", "triage"}
	if _, err := check(p, in, c); err == nil {
		t.Fatal("duplicate cache ID accepted")
	}
	c.AnswerCacheHits = []string{"foreign"}
	if _, err := check(p, in, c); err == nil {
		t.Fatal("foreign cache ID accepted")
	}
	c.AnswerCacheHits = []string{"triage"}
	c.AttemptCount = 0
	if _, err := check(p, in, c); err == nil {
		t.Fatal("fresh answer without attempt accepted")
	}
	c.AnswerCacheHits = []string{"triage2"}
	c.AttemptCount = 1
	delete(c.Response.Answers, "triage2")
	c.Response.Invalid["triage2"] = "invalid"
	if _, err := check(p, in, c); err == nil {
		t.Fatal("cached ID naming invalid outcome accepted")
	}
}

func TestPartialBatchExitAndAbstentionAccounting(t *testing.T) {
	p, in, c, m := twoQuestionCase(t)
	a := c.Response.Answers["triage2"]
	a.Choice = "unknown"
	a.Probabilities = map[string]float64{"inspect": .1, "defer": .1, "unknown": .8}
	c.Response.Answers["triage2"] = a
	cr, err := check(p, in, c)
	if err != nil || cr.Status != "unusable" || cr.Reason != "partial" || cr.Outcomes["triage"] != "answered" || cr.Outcomes["triage2"] != "unknown" {
		t.Fatalf("partial batch: %+v %v", cr, err)
	}
	if cr.ObservedAnswers["triage"].Choice != "inspect" || cr.ObservedAnswers["triage2"].Choice != "unknown" || cr.ObservedAnswers["triage"].Confidence != decision.Confidence(c.Response.Answers["triage"].Probabilities) {
		t.Fatalf("validated answer details missing: %+v", cr)
	}
	dir := t.TempDir()
	write := func(name string, v any) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, canonical(v), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	var out bytes.Buffer
	if code := run([]string{"check", "--pack", write("pack.json", p), "--input", write("input.json", in), "--capture", write("capture.json", c), "--format", "json"}, &out); code != 2 {
		t.Fatalf("partial exit %d: %s", code, out.String())
	}
	r, err := study(p, m, []Capture{c})
	if err != nil {
		t.Fatal(err)
	}
	if r.FalseUnfavorable != 0 || r.AbstainedExpectedFavorable != 1 || r.Unknown != 1 || r.Classes["triage2/unknown"].Reading.N != 0 {
		t.Fatalf("unknown counted as false negative: %+v", r)
	}
}

func TestTypedFailuresAndAttempts(t *testing.T) {
	p := fixture[Pack](t, "review-triage.pack.json")
	base := fixture[Input](t, "case-1.input.json")
	causes := []string{"no-provider", "timeout", "not-authorized", "budget-exhausted", "transport-error"}
	attempts := []int{0, 2, 0, 0, 1}
	m := Manifest{Version: "decision-lab.manifest/1", DatasetKind: "observed", PackDigest: digest(canonical(p))}
	captures := []Capture{}
	for i, cause := range causes {
		in := base
		in.CaseID = "failure-" + cause
		in.SubjectID = in.CaseID
		in.CapturedState += cause
		pr, err := prepared(p, in)
		if err != nil {
			t.Fatal(err)
		}
		m.Cases = append(m.Cases, ExpectedCase{Input: in, IndependentUnitID: "unit-" + cause, Labels: map[string]ExpectedLabel{"triage": {Choice: "inspect", Provenance: decision.ProvenanceFrontier}}})
		captures = append(captures, Capture{Version: "decision-lab.capture/1", CaseID: in.CaseID, PackDigest: pr.PackDigest, InputDigest: pr.InputDigest, RequestKey: pr.RequestKey, AttemptCount: attempts[i], AnswerCacheHits: []string{}, Cause: cause})
	}
	r, err := study(p, m, captures)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "complete" || r.ExpectedCases != 5 || r.Failed != 5 || r.DeclaredAttempts != 3 || r.FailedAttempts != 3 || r.Classes["triage/inspect"].Reading.N != 0 {
		t.Fatalf("typed failures lost: %+v", r)
	}
	if r.UsageUnreportedCaptures != 2 {
		t.Fatalf("missing unreported usage count: %+v", r)
	}
	for _, cause := range causes {
		if r.CauseCounts[cause] != 1 {
			t.Fatalf("lost cause %s", cause)
		}
	}
	captures[0].AttemptCount = 1
	if _, err := check(p, m.Cases[0].Input, captures[0]); err == nil {
		t.Fatal("no-provider attempt accepted")
	}
	captures[1].AttemptCount = 0
	if _, err := check(p, m.Cases[1].Input, captures[1]); err == nil {
		t.Fatal("timeout without attempt accepted")
	}
	captures[1].AttemptCount = 33
	if _, err := check(p, m.Cases[1].Input, captures[1]); err == nil {
		t.Fatal("attempt count over bound accepted")
	}
}

func TestRetrievalFixtureStudy(t *testing.T) {
	p := fixture[Pack](t, "retrieval-support.pack.json")
	m := fixture[Manifest](t, "retrieval-support.manifest.json")
	c := fixture[[]Capture](t, "retrieval-support.captures.json")
	r, err := study(p, m, c)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "complete" || r.ExpectedCases != 7 || r.Answered != 5 || r.Unknown != 2 || r.FalseFavorable != 1 || r.FalseUnfavorable != 0 || r.AbstainedExpectedFavorable != 1 || r.UsageUnreportedCaptures != 7 {
		t.Fatalf("retrieval diagnostics: %+v", r)
	}
	for _, cls := range []string{"supported", "contradicted", "mixed", "unsupported", "unknown"} {
		if r.Classes["support/"+cls].Expected == 0 {
			t.Fatalf("missing expected class %s", cls)
		}
	}
}

func TestDuplicateExposureRejected(t *testing.T) {
	p := fixture[Pack](t, "review-triage.pack.json")
	first := fixture[Manifest](t, "review-triage.manifest.json").Cases[0]
	for _, conflicting := range []bool{false, true} {
		second := first
		second.Input.CaseID = "copy-case"
		second.Input.SubjectID = "copy-subject"
		second.Input.Revision = "r2"
		second.IndependentUnitID = "copy-unit"
		second.Labels = map[string]ExpectedLabel{"triage": first.Labels["triage"]}
		if conflicting {
			second.Labels["triage"] = ExpectedLabel{Choice: "defer", Provenance: decision.ProvenanceFrontier}
		}
		m := Manifest{Version: "decision-lab.manifest/1", DatasetKind: "synthetic", PackDigest: digest(canonical(p)), Cases: []ExpectedCase{first, second}}
		r, err := study(p, m, nil)
		if err == nil || r.Reason != "duplicate_exposure" || !reflect.DeepEqual(r.DuplicateExposureCaseIDs, []string{"case-1", "copy-case"}) {
			t.Fatalf("duplicate exposure accepted: %+v %v", r, err)
		}
		dir := t.TempDir()
		packPath := filepath.Join(dir, "pack.json")
		manifestPath := filepath.Join(dir, "manifest.json")
		capturesPath := filepath.Join(dir, "captures.json")
		os.WriteFile(packPath, canonical(p), 0600)
		os.WriteFile(manifestPath, canonical(m), 0600)
		os.WriteFile(capturesPath, []byte("[]"), 0600)
		var out bytes.Buffer
		if code := run([]string{"study", "--pack", packPath, "--manifest", manifestPath, "--captures", capturesPath, "--format", "json"}, &out); code != 2 {
			t.Fatalf("duplicate study exit %d", code)
		}
		var envelope EnvelopeResult
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil || envelope.Reason != "duplicate_exposure" {
			t.Fatalf("duplicate reason: %s %v", out.String(), err)
		}
	}
}

func TestMissingCoverageReportsValidatedIDs(t *testing.T) {
	p := fixture[Pack](t, "review-triage.pack.json")
	m := fixture[Manifest](t, "review-triage.manifest.json")
	c := fixture[[]Capture](t, "review-triage.captures.json")
	r, err := study(p, m, c[:1])
	if err == nil || r.Reason != "missing_cases" || !reflect.DeepEqual(r.MissingCaseIDs, []string{"case-2", "case-3"}) {
		t.Fatalf("missing case report: %+v %v", r, err)
	}
}

func TestConfidenceRangeAndProbabilityOrder(t *testing.T) {
	p := fixture[Pack](t, "review-triage.pack.json")
	c := fixture[Capture](t, "case-1.capture.json")
	a := c.Response.Answers["triage"]
	a.Probabilities = map[string]float64{"inspect": 1, "defer": 0, "unknown": 0}
	a.Confidence = 1 + 1e-13
	if err := checkAnswer(p.Questions[0], a); err == nil {
		t.Fatal("confidence above one accepted through epsilon")
	}
	a.Confidence = 1
	for i := 0; i < 500; i++ {
		if err := checkAnswer(p.Questions[0], a); err != nil {
			t.Fatalf("ordered valid distribution rejected: %v", err)
		}
	}
	a.Probabilities = map[string]float64{"inspect": 1.0 / 3, "defer": 1.0 / 3, "unknown": 1.0 / 3}
	a.Confidence = -1e-13
	if err := checkAnswer(p.Questions[0], a); err == nil {
		t.Fatal("confidence below zero accepted through epsilon")
	}
}
