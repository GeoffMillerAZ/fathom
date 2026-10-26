package main

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func fixture[T any](t *testing.T, name string) T {
	t.Helper()
	var v T
	if _, err := readJSON(filepath.Join("fixtures", name), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFixtureStudy(t *testing.T) {
	if got := decision.Confidence(map[string]float64{"inspect": .8, "defer": .1, "unknown": .1}); math.Abs(got-.7) > 1e-12 {
		t.Fatalf("confidence recipe drift: %v", got)
	}
	p := fixture[Pack](t, "review-triage.pack.json")
	m := fixture[Manifest](t, "review-triage.manifest.json")
	c := fixture[[]Capture](t, "review-triage.captures.json")
	r, err := study(p, m, c)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "complete" || r.ExpectedCases != 3 || r.FalseFavorable != 1 || r.Unknown != 1 || r.Classes["triage/inspect"].Reading.N != 2 {
		t.Fatalf("unexpected report: %+v", r)
	}
	if r.Classes["triage/unknown"].Reading.N != 0 {
		t.Fatal("abstention counted as fresh calibration")
	}
	c[0].AnswerCacheHits = []string{"triage"}
	c[0].AttemptCount = 0
	r, err = study(p, m, c)
	if err != nil {
		t.Fatal(err)
	}
	if r.AnswerCacheHits != 1 || r.Classes["triage/inspect"].Reading.N != 1 {
		t.Fatal("answer reuse inflated fresh N")
	}
	if again, err := study(p, m, c); err != nil || again.Classes["triage/inspect"].Reading.N != 1 {
		t.Fatal("repeat study appended trial")
	}
}

func TestBindingAndCoverage(t *testing.T) {
	p := fixture[Pack](t, "review-triage.pack.json")
	m := fixture[Manifest](t, "review-triage.manifest.json")
	c := fixture[[]Capture](t, "review-triage.captures.json")
	in := m.Cases[0].Input
	mutations := []struct {
		name   string
		change func(*Pack, *Input, *Capture)
	}{
		{"question", func(p *Pack, _ *Input, _ *Capture) { p.Questions[0].Instructions += " changed" }},
		{"order", func(p *Pack, _ *Input, _ *Capture) {
			p.Questions[0].Options[0], p.Questions[0].Options[1] = p.Questions[0].Options[1], p.Questions[0].Options[0]
		}},
		{"cohort", func(p *Pack, _ *Input, _ *Capture) { p.PolicyEpoch = "policy-v2" }},
		{"model", func(p *Pack, _ *Input, _ *Capture) { p.ResolvedModel = "fixture-model-2" }},
		{"input-state", func(_ *Pack, i *Input, _ *Capture) { i.CapturedState += " changed" }},
		{"input-revision", func(_ *Pack, i *Input, _ *Capture) { i.Revision = "r2" }},
		{"case", func(_ *Pack, _ *Input, c *Capture) { c.CaseID = "other" }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			pp := p
			pp.Questions = append([]PackQuestion(nil), p.Questions...)
			pp.Questions[0].Options = append([]PackOption(nil), p.Questions[0].Options...)
			ii := in
			cc := c[0]
			tc.change(&pp, &ii, &cc)
			if _, err := check(pp, ii, cc); err == nil {
				t.Fatal("binding drift accepted")
			}
		})
	}
	if _, err := study(p, m, c[:2]); err == nil {
		t.Fatal("missing expected case accepted")
	}
	dup := append(append([]Capture{}, c...), c[0])
	if _, err := study(p, m, dup); err == nil {
		t.Fatal("duplicate capture accepted")
	}
	foreign := append([]Capture{}, c...)
	foreign[2].CaseID = "foreign"
	if _, err := study(p, m, foreign); err == nil {
		t.Fatal("foreign capture accepted")
	}
	bad := m
	bad.Cases = append([]ExpectedCase{}, m.Cases...)
	bad.Cases[2].Labels = map[string]ExpectedLabel{}
	if _, err := study(p, bad, c); err == nil {
		t.Fatal("missing independent label accepted")
	}
}

func TestPackContract(t *testing.T) {
	p := fixture[Pack](t, "review-triage.pack.json")
	bad := p
	bad.ConfidenceRecipe = "decision.Confidence/2"
	if err := bad.validate(); err == nil {
		t.Fatal("unknown confidence recipe accepted")
	}
	bad = p
	bad.Questions = append([]PackQuestion(nil), p.Questions...)
	bad.Questions[0].Instructions = " \t "
	if err := bad.validate(); err == nil {
		t.Fatal("blank instructions accepted")
	}
	bad = p
	bad.Questions = append([]PackQuestion(nil), p.Questions...)
	bad.Questions[0].Options = append([]PackOption(nil), p.Questions[0].Options...)
	bad.Questions[0].Options[2].Criteria = " "
	if err := bad.validate(); err == nil {
		t.Fatal("blank unknown criteria accepted")
	}
	bad = p
	bad.Questions = append([]PackQuestion(nil), p.Questions...)
	bad.Questions[0].FavorableOptions = nil
	if err := bad.validate(); err == nil {
		t.Fatal("missing favorable mapping accepted")
	}
	bad = p
	bad.Questions = append([]PackQuestion(nil), p.Questions...)
	bad.Questions[0].Options = bad.Questions[0].Options[:2]
	if err := bad.validate(); err == nil {
		t.Fatal("missing unknown option accepted")
	}
}

func TestProtocolRejects(t *testing.T) {
	p := fixture[Pack](t, "review-triage.pack.json")
	in := fixture[Input](t, "case-1.input.json")
	orig := fixture[[]Capture](t, "review-triage.captures.json")[0]
	tests := []struct {
		name   string
		change func(*Capture)
	}{
		{"omission", func(c *Capture) { delete(c.Response.Answers, "triage") }},
		{"foreign", func(c *Capture) { c.Response.Answers["foreign"] = c.Response.Answers["triage"] }},
		{"overlap", func(c *Capture) { c.Response.Invalid["triage"] = "invalid" }},
		{"probability", func(c *Capture) {
			c.Response.Answers["triage"] = answerWith(c.Response.Answers["triage"], "inspect", map[string]float64{"inspect": 1.1, "defer": -.1, "unknown": 0}, .7)
		}},
		{"sum", func(c *Capture) {
			c.Response.Answers["triage"] = answerWith(c.Response.Answers["triage"], "inspect", map[string]float64{"inspect": .8, "defer": .1, "unknown": .2}, .7)
		}},
		{"choice", func(c *Capture) {
			a := c.Response.Answers["triage"]
			a.Choice = "defer"
			c.Response.Answers["triage"] = a
		}},
		{"confidence", func(c *Capture) {
			a := c.Response.Answers["triage"]
			a.Confidence = .9
			c.Response.Answers["triage"] = a
		}},
		{"mixed", func(c *Capture) {
			c.Response.Declined = map[string]string{"triage": "refused"}
			c.Response.Invalid["triage"] = "invalid"
		}},
		{"provider", func(c *Capture) { c.Response.Provider = "different" }},
		{"model", func(c *Capture) { c.Response.Model = "different" }},
		{"negative-usage", func(c *Capture) { c.Response.Usage.InputTokens = -1 }},
		{"cause-response", func(c *Capture) { c.Cause = "timeout" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(orig)
			var c Capture
			json.Unmarshal(b, &c)
			tc.change(&c)
			if _, err := check(p, in, c); err == nil {
				t.Fatal("invalid protocol accepted")
			}
		})
	}
}

func TestDistinctAbstentions(t *testing.T) {
	p := fixture[Pack](t, "review-triage.pack.json")
	in := fixture[Input](t, "case-1.input.json")
	c := fixture[[]Capture](t, "review-triage.captures.json")[0]
	delete(c.Response.Answers, "triage")
	c.Response.Declined = map[string]string{"triage": "private provider message"}
	r, err := check(p, in, c)
	if err != nil || r.Status != "unusable" || r.Outcomes["triage"] != "declined" {
		t.Fatalf("refusal: %+v %v", r, err)
	}
	delete(c.Response.Declined, "triage")
	c.Response.Invalid["triage"] = "private invalid message"
	r, err = check(p, in, c)
	if err != nil || r.Outcomes["triage"] != "invalid" {
		t.Fatalf("invalid: %+v %v", r, err)
	}
	c.Response = nil
	c.Cause = "timeout"
	r, err = check(p, in, c)
	if err != nil || r.Outcomes["triage"] != "failed" || r.Reason != "timeout" {
		t.Fatalf("cause: %+v %v", r, err)
	}
}

func TestObservedCompleteWithFailedCaptures(t *testing.T) {
	p := fixture[Pack](t, "review-triage.pack.json")
	m := fixture[Manifest](t, "review-triage.manifest.json")
	m.DatasetKind = "observed"
	c := fixture[[]Capture](t, "review-triage.captures.json")
	for i := range c {
		c[i].Response = nil
		c[i].Cause = "timeout"
		c[i].AttemptCount = 1
	}
	r, err := study(p, m, c)
	if err != nil || r.Status != "complete" || r.ExpectedCases != 3 || r.Failed != 3 || r.FailedAttempts != 3 || r.Classes["triage/inspect"].Reading.N != 0 {
		t.Fatalf("failed observed report: %+v %v", r, err)
	}
}

func answerWith(a decision.Answer, choice string, probabilities map[string]float64, confidence float64) decision.Answer {
	a.Choice = choice
	a.Probabilities = probabilities
	a.Confidence = confidence
	return a
}

func TestStrictJSONAndOutput(t *testing.T) {
	dir := t.TempDir()
	pack := "fixtures/review-triage.pack.json"
	input := "fixtures/case-1.input.json"
	for _, raw := range []string{
		`{"version":"decision-lab.input/1","caseId":"x","caseId":"y"}`,
		`{"version":"decision-lab.input/1","caseID":"x"}`,
		`{"version":"decision-lab.input/1","caseId":null}`,
		`{} {}`,
		`{"version":"decision-lab.input/1","caseId":"x","subjectId":"s","revision":"r","capturedState":"x","extra":1}`,
	} {
		path := filepath.Join(dir, "bad.json")
		os.WriteFile(path, []byte(raw), 0600)
		var in Input
		if _, err := readJSON(path, &in); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	capture := fixture[Capture](t, "case-1.capture.json")
	capture.Response.Answers["triage"] = answerWith(capture.Response.Answers["triage"], "inspect", map[string]float64{"inspect": .8, "defer": .1, "unknown": .1}, 0)
	malformed := canonical(capture)
	missingPath := filepath.Join(dir, "missing-confidence.json")
	os.WriteFile(missingPath, bytes.Replace(malformed, []byte(`"Confidence":0`), []byte(`"AbsentConfidence":0`), 1), 0600)
	var parsed Capture
	if _, err := readJSON(missingPath, &parsed); err == nil {
		t.Fatal("noncanonical answer field accepted")
	}
	path := filepath.Join(dir, "bad-utf8.json")
	os.WriteFile(path, []byte{0xff}, 0600)
	var in Input
	if _, err := readJSON(path, &in); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	out := filepath.Join(dir, "private.json")
	var b bytes.Buffer
	if code := run([]string{"prepare", "--pack", pack, "--input", input, "--out", out, "--format", "json"}, &b); code != 0 {
		t.Fatalf("prepare: %s", b.String())
	}
	info, err := os.Stat(out)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private mode: %v %v", info, err)
	}
	if strings.Contains(b.String(), "tenant ownership") {
		t.Fatal("metadata output leaked state")
	}
	b.Reset()
	if code := run([]string{"prepare", "--pack", pack, "--input", input, "--out", out}, &b); code != 2 {
		t.Fatal("exclusive output accepted overwrite")
	}
	b.Reset()
	if code := run([]string{"prepare", "--pack", pack, "--input", input, "--out", dir}, &b); code != 2 {
		t.Fatal("directory output accepted")
	}
	b.Reset()
	if code := run([]string{"prepare", "--pack", pack, "--input", input, "--out", "-"}, &b); code != 0 || !strings.Contains(b.String(), "tenant ownership") {
		t.Fatal("explicit pipe did not emit state")
	}
	deep := filepath.Join(dir, "deep.json")
	os.WriteFile(deep, []byte(strings.Repeat("[", 25)+"0"+strings.Repeat("]", 25)), 0600)
	if _, err := readJSON(deep, &in); err == nil {
		t.Fatal("deep JSON accepted")
	}
	large := filepath.Join(dir, "large.json")
	os.WriteFile(large, bytes.Repeat([]byte("x"), maxJSON+1), 0600)
	if _, err := readJSON(large, &in); err == nil {
		t.Fatal("oversize JSON accepted")
	}
	if code := run([]string{"prepare", "--pack", pack, "--input", input}, brokenWriter{}); code != 2 {
		t.Fatal("output writer failure accepted")
	}
	capture.Response.Invalid["triage"] = "SECRET_PROVIDER_BODY"
	unsafePath := filepath.Join(dir, "capture.json")
	os.WriteFile(unsafePath, canonical(capture), 0600)
	b.Reset()
	if code := run([]string{"check", "--pack", pack, "--input", input, "--capture", unsafePath, "--format", "json"}, &b); code != 2 || strings.Contains(b.String(), "SECRET_PROVIDER_BODY") {
		t.Fatal("provider body leaked on protocol error")
	}
	delete(capture.Response.Answers, "triage")
	os.WriteFile(unsafePath, canonical(capture), 0600)
	b.Reset()
	if code := run([]string{"check", "--pack", pack, "--input", input, "--capture", unsafePath, "--format", "json"}, &b); code != 2 || strings.Contains(b.String(), "SECRET_PROVIDER_BODY") || strings.Contains(b.String(), "tenant ownership") {
		t.Fatal("provider body or source leaked in valid invalid-outcome report")
	}
	unsafeInput := filepath.Join(dir, "unknown-field.json")
	os.WriteFile(unsafeInput, []byte(`{"SECRET_SOURCE_CANARY":1}`), 0600)
	b.Reset()
	if code := run([]string{"prepare", "--pack", pack, "--input", unsafeInput, "--format", "json"}, &b); code != 2 || strings.Contains(b.String(), "SECRET_SOURCE_CANARY") {
		t.Fatal("unknown field name leaked")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
