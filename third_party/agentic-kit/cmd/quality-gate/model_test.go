package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/openai"
)

type modelStub struct {
	calls    int
	response decision.Response
	err      error
	request  decision.Request
}

func (s *modelStub) Name() string { return "stub" }
func (s *modelStub) Decide(_ context.Context, r decision.Request) (decision.Response, error) {
	s.calls++
	s.request = r
	return s.response, s.err
}

func modelFixture(mode string) (policy, snapshot) {
	site := modelSite{ID: "source", Version: "1", Question: "Does this change violate policy?", Boundaries: []string{"commit"}, Select: selector{Prefixes: []string{"src/"}}, Mode: mode, MinConfidence: .5}
	p := policy{ID: "repo", Model: modelPolicy{Provider: "openai", ModelEnv: "QUALITY_TEST_MODEL", ExpectedModelEnv: "QUALITY_TEST_EXPECTED", MaxInputBytes: 10000, TimeoutSeconds: 1, Sites: []modelSite{site}}}
	s := snapshot{BaseTree: "base", Tree: "next", Changes: []change{{Path: "src/a.go", BeforeOID: "before", AfterOID: "after", BeforeMode: "100644", AfterMode: "100644", Before: []byte("before"), After: []byte("after")}}}
	return p, s
}
func setModelEnv(t *testing.T) {
	t.Helper()
	t.Setenv("QUALITY_TEST_MODEL", "pinned")
	t.Setenv("QUALITY_TEST_EXPECTED", "pinned")
}
func stubAnswer(choice string) decision.Response {
	return decision.Response{Provider: "openai", Model: "pinned", Answers: map[string]decision.Answer{"source": {ID: "source", Kind: decision.KindChoice, Choice: choice, Probabilities: map[string]float64{"clear": .05, "concern": .9, "insufficient": .05}}}}
}

func TestModelTypedBatchAndUnknowns(t *testing.T) {
	setModelEnv(t)
	p, s := modelFixture("shadow")
	d := &modelStub{response: stubAnswer("concern")}
	got := evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true}, d)
	if d.calls != 1 || len(d.request.Questions) != 1 || got.Sites[0].Outcome != "concern" || got.Sites[0].Held {
		t.Fatalf("batch=%d result=%+v", d.calls, got)
	}
	if !strings.Contains(d.request.State, "before") || !strings.Contains(d.request.State, "after") || !strings.Contains(d.request.State, "policy") {
		t.Fatal("source state missing provenance")
	}
	for _, tc := range []struct {
		name     string
		response decision.Response
		err      error
		want     string
	}{
		{"refusal", decision.Response{Provider: "openai", Model: "pinned", Declined: map[string]string{"source": "private refusal"}}, nil, "refusal"},
		{"empty-refusal", decision.Response{Provider: "openai", Model: "pinned", Declined: map[string]string{"source": ""}, Answers: stubAnswer("concern").Answers}, nil, "refusal"},
		{"missing", decision.Response{Provider: "openai", Model: "pinned"}, nil, "invalid-answer"},
		{"empty-invalid", decision.Response{Provider: "openai", Model: "pinned", Invalid: map[string]string{"source": ""}, Answers: stubAnswer("concern").Answers}, nil, "invalid-answer"},
		{"drift", decision.Response{Provider: "openai", Model: "other"}, nil, "model-drift"},
		{"failure", decision.Response{Usage: decision.Usage{InputTokens: 7}}, errors.New("secret response body"), "provider-unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &modelStub{response: tc.response, err: tc.err}
			got := evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true}, d)
			if got.Sites[0].Reason != tc.want || strings.Contains(got.Sites[0].Reason, "secret") || d.calls != 1 {
				t.Fatalf("%+v calls=%d", got, d.calls)
			}
			if tc.name == "failure" && got.InputTokens != 7 {
				t.Fatalf("usage lost: %+v", got)
			}
		})
	}
}

func TestModelGuardsBeforeDispatch(t *testing.T) {
	setModelEnv(t)
	d := &modelStub{response: stubAnswer("clear")}
	cases := []struct {
		name   string
		mutate func(*policy, *snapshot)
		opts   modelOptions
		want   string
	}{
		{"remote", func(*policy, *snapshot) {}, modelOptions{}, "remote-not-authorized"},
		{"limit", func(p *policy, _ *snapshot) { p.Model.MaxInputBytes = 2 }, modelOptions{AllowRemote: true}, "input-limit"},
		{"missing", func(_ *policy, s *snapshot) { s.Changes[0].After = nil }, modelOptions{AllowRemote: true}, "missing-source"},
		{"binary", func(_ *policy, s *snapshot) { s.Changes[0].After = []byte{0} }, modelOptions{AllowRemote: true}, "binary-source"},
		{"symlink", func(_ *policy, s *snapshot) { s.Changes[0].AfterMode = "120000" }, modelOptions{AllowRemote: true}, "non-regular-source"},
		{"lfs", func(_ *policy, s *snapshot) {
			s.Changes[0].After = []byte("version https://git-lfs.github.com/spec/v1\noid sha256:x\n")
		}, modelOptions{AllowRemote: true}, "lfs-pointer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, s := modelFixture("hold")
			tc.mutate(&p, &s)
			d.calls = 0
			got := evaluateModelsWith(context.Background(), p, "commit", "policy", s, tc.opts, d)
			if got.Sites[0].Reason != tc.want || !got.Sites[0].Held || d.calls != 0 {
				t.Fatalf("%+v calls=%d", got, d.calls)
			}
		})
	}
}

func TestModelCalibrationBindingAndCoverage(t *testing.T) {
	site := modelSite{ID: "source", Version: "1", Question: "Question", MinConfidence: .5}
	c := &calibration{Site: "repo/commit/source", Provider: "openai", ResolvedModel: "pinned", ValidUntil: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), MinPerClass: 1, MinN: 1, MinAccuracy: .1}
	c.QuestionSHA256 = siteDigest(site)
	site.Calibration = c
	for _, choice := range []string{"clear", "concern", "insufficient"} {
		c.Cases = append(c.Cases, labeledCase{ID: choice, Expected: choice, Actual: choice, Confidence: .9, Provenance: "human"})
	}
	now := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := calibrationStatus("repo", "commit", site, "openai", "pinned", "clear", .9, now); got != "pass" {
		t.Fatal(got)
	}
	if got := calibrationStatus("repo", "commit", site, "openai", "other", "clear", .9, now); got != "binding-mismatch" {
		t.Fatal(got)
	}
	if got := calibrationStatus("repo", "commit", site, "openai", "pinned", "clear", .9, c.ValidUntil); got != "expired" {
		t.Fatal(got)
	}
	c.Cases = c.Cases[:2]
	if got := calibrationStatus("repo", "commit", site, "openai", "pinned", "clear", .9, now); got != "insufficient-labels" {
		t.Fatal(got)
	}
}

func TestSiteDigestBindsEffectiveQuestionAndCohort(t *testing.T) {
	site := modelSite{ID: "source", Version: "1", Question: "Question", Boundaries: []string{"commit"}, Select: selector{Prefixes: []string{"src/"}}, Mode: "hold", MinConfidence: .5}
	other := modelSite{ID: "other", Version: "2", Question: "Other question", Select: selector{Prefixes: []string{"docs/"}}}
	base := siteDigest(site, site, other)
	if siteDigest(site, other, site) != base {
		t.Fatal("cohort order changed digest")
	}
	if siteDigest(site, site, other) == siteDigestForStateVersion(site, modelStateVersion+1, site, other) {
		t.Fatal("state version omitted from digest")
	}
	if siteDigest(site, site, other) == siteDigest(other, site, other) {
		t.Fatal("selected site omitted from digest")
	}
	other.Question = "New question"
	if siteDigest(site, site, other) == base {
		t.Fatal("cohort question omitted from digest")
	}
	other.Question = "Other question"
	other.Select.Prefixes[0] = "test/"
	if siteDigest(site, site, other) == base {
		t.Fatal("cohort selector omitted from digest")
	}
	other.Select.Prefixes[0] = "docs/"
	site.Mode = "shadow"
	site.Calibration = &calibration{QuestionSHA256: "old"}
	if siteDigest(site, site, other) != base {
		t.Fatal("mode or calibration changed digest")
	}
	question := modelQuestion(site)
	if question.Instructions == site.Question || len(question.Options) != 3 {
		t.Fatal("fixed instruction or options missing from request")
	}
}

func TestValidatedAnswerConfidenceRounding(t *testing.T) {
	for _, tc := range []struct {
		name  string
		probs map[string]float64
		valid bool
		max   float64
	}{
		{"over", map[string]float64{"clear": .9, "concern": .0501, "insufficient": .05}, true, .9},
		{"under", map[string]float64{"clear": .9, "concern": .0499, "insufficient": .05}, true, .9},
		{"far", map[string]float64{"clear": .9, "concern": .03, "insufficient": .05}, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			confidence, valid := validatedAnswerConfidence(decision.Answer{Choice: "clear", Probabilities: tc.probs})
			if valid != tc.valid || valid && (confidence > tc.max || confidence <= 0) {
				t.Fatalf("valid=%t confidence=%.9f", valid, confidence)
			}
			if tc.name == "over" && confidence >= .9 {
				t.Fatalf("overfull probabilities cleared exact threshold: %.9f", confidence)
			}
		})
	}
}

func TestModelSharedBatchBudget(t *testing.T) {
	setModelEnv(t)
	p, s := modelFixture("hold")
	budget := newModelBudget(p.Model)
	d := &modelStub{response: stubAnswer("concern")}
	opts := modelOptions{AllowRemote: true, Budget: budget}
	first := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	second := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	if d.calls != 1 || !first.RemoteAttempted || first.BatchOrdinal != 1 || first.BatchLimit != 1 || second.RemoteAttempted || second.BatchOrdinal != 0 || second.Reason != "batch-budget-exhausted" || second.Sites[0].Reason != second.Reason || !second.Sites[0].Held {
		t.Fatalf("calls=%d first=%+v second=%+v", d.calls, first, second)
	}
	budget = newModelBudget(p.Model)
	d.err = errors.New("provider failed")
	opts.Budget = budget
	failed := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	blocked := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	if d.calls != 2 || !failed.RemoteAttempted || blocked.Reason != "batch-budget-exhausted" {
		t.Fatalf("provider error did not consume attempt: calls=%d failed=%+v blocked=%+v", d.calls, failed, blocked)
	}
}

func TestModelCacheShadowOnly(t *testing.T) {
	setModelEnv(t)
	p, s := modelFixture("shadow")
	p.Model.CacheTTLSeconds = 10
	p.Model.MaxBatches = 4
	d := &modelStub{response: stubAnswer("concern")}
	dir := t.TempDir()
	now := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC)
	opts := modelOptions{AllowRemote: true, CacheDir: dir, Now: func() time.Time { return now }, Budget: newModelBudget(p.Model)}
	first := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	if d.calls != 1 || first.Cache != "none" {
		t.Fatalf("%+v", first)
	}
	hit := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	if d.calls != 1 || opts.Budget.used != 1 || hit.Cache != "hit" || hit.RemoteAttempted || hit.BatchOrdinal != 0 || hit.BatchLimit != 4 {
		t.Fatalf("%+v calls=%d", hit, d.calls)
	}
	if b, err := os.ReadFile(filepath.Join(privateCacheDir(dir), digestJSON(struct{ Request, Provider, Model, Expected, Boundary, Policy string }{"sha256:" + first.RequestSHA256, "openai", "pinned", "pinned", "commit", "policy"})+".json")); err != nil || strings.Contains(string(b), "before") {
		t.Fatalf("cache source leaked or missing: %v", err)
	}
	opts.Now = func() time.Time { return now.Add(11 * time.Second) }
	evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	if d.calls != 2 {
		t.Fatalf("expired cache reused: %d", d.calls)
	}
	p.Model.Sites[0].Mode = "hold"
	evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	if d.calls != 3 {
		t.Fatalf("hold cache reused: %d", d.calls)
	}
}

func TestModelInputCapIncludesQuestionBytes(t *testing.T) {
	setModelEnv(t)
	p, s := modelFixture("hold")
	d := &modelStub{response: stubAnswer("concern")}
	got := evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true}, d)
	if got.Sites[0].Outcome != "concern" || d.calls != 1 {
		t.Fatalf("baseline: %+v", got)
	}
	wire, err := json.Marshal(d.request)
	if err != nil {
		t.Fatal(err)
	}
	p.Model.MaxInputBytes = len(wire) - 1
	d.calls = 0
	got = evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true}, d)
	if got.Sites[0].Reason != "input-limit" || d.calls != 0 {
		t.Fatalf("cap below request: %+v calls=%d", got, d.calls)
	}
	p.Model.MaxInputBytes = len(wire)
	evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true}, d)
	if d.calls != 1 {
		t.Fatalf("exact cap rejected: calls=%d", d.calls)
	}
}

func TestModelPinAndProviderIdentity(t *testing.T) {
	setModelEnv(t)
	p, s := modelFixture("hold")
	d := &modelStub{response: stubAnswer("concern")}
	t.Setenv("QUALITY_TEST_EXPECTED", "")
	got := evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true}, d)
	if d.calls != 0 || got.Sites[0].Reason != "provider-unavailable" {
		t.Fatalf("missing pin dispatched: %+v calls=%d", got, d.calls)
	}
	t.Setenv("QUALITY_TEST_EXPECTED", "pinned")
	d.response.Model = "moving-alias"
	got = evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true}, d)
	if got.Sites[0].Reason != "model-drift" || !got.Sites[0].Held {
		t.Fatalf("wrong resolved model: %+v", got.Sites[0])
	}
	d.response.Model = "pinned"
	d.response.Provider = "other"
	got = evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true}, d)
	if got.Sites[0].Reason != "provider-unavailable" || !got.Sites[0].Held {
		t.Fatalf("wrong provider: %+v", got.Sites[0])
	}
}

func TestModelClearRequiresCalibratedCurrentBinding(t *testing.T) {
	setModelEnv(t)
	p, s := modelFixture("hold")
	d := &modelStub{response: decision.Response{Provider: "openai", Model: "pinned", Answers: map[string]decision.Answer{"source": {ID: "source", Kind: decision.KindChoice, Choice: "clear", Probabilities: map[string]float64{"clear": .95, "concern": .025, "insufficient": .025}}}}}
	now := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC)
	opts := modelOptions{AllowRemote: true, Now: func() time.Time { return now }}
	checkHeld := func(want string) {
		t.Helper()
		got := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
		if got.Sites[0].Outcome != "unknown" || !got.Sites[0].Held || got.Sites[0].Calibration != want {
			t.Fatalf("calibration %s: %+v", want, got.Sites[0])
		}
	}
	checkHeld("missing")
	site := &p.Model.Sites[0]
	c := &calibration{Site: "repo/commit/source", Provider: "openai", ResolvedModel: "pinned", ValidUntil: now.Add(time.Hour), MinPerClass: 1, MinN: 1, MinAccuracy: .9}
	c.QuestionSHA256 = siteDigest(*site)
	c.Cases = []labeledCase{
		{ID: "clear-false", Expected: "concern", Actual: "clear", Confidence: .95, Provenance: "human"},
		{ID: "clear-true", Expected: "clear", Actual: "clear", Confidence: .95, Provenance: "human"},
		{ID: "other", Expected: "insufficient", Actual: "insufficient", Confidence: .95, Provenance: "computed"},
	}
	site.Calibration = c
	checkHeld("fail")
	c.Cases[0].Actual = "concern"
	got := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	if got.Sites[0].Outcome != "clear" || got.Sites[0].Held || got.Sites[0].Calibration != "pass" {
		t.Fatalf("calibrated clear: %+v", got.Sites[0])
	}
	c.QuestionSHA256 = "stale"
	checkHeld("binding-mismatch")
}

func TestModelCacheTamperAndHoldCannotAuthorize(t *testing.T) {
	setModelEnv(t)
	p, s := modelFixture("shadow")
	p.Model.CacheTTLSeconds = 60
	d := &modelStub{response: stubAnswer("concern")}
	dir := t.TempDir()
	now := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC)
	opts := modelOptions{AllowRemote: true, CacheDir: dir, Now: func() time.Time { return now }}
	first := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	key := digestJSON(struct{ Request, Provider, Model, Expected, Boundary, Policy string }{"sha256:" + first.RequestSHA256, "openai", "pinned", "pinned", "commit", "policy"})
	path := filepath.Join(privateCacheDir(dir), key+".json")
	if err := os.WriteFile(path, []byte(`{"key":"forged"}`), 0600); err != nil {
		t.Fatal(err)
	}
	evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	if d.calls != 2 {
		t.Fatalf("tampered cache reused: %d", d.calls)
	}
	p.Model.Sites[0].Mode = "hold"
	d.response = decision.Response{Provider: "openai", Model: "pinned", Answers: map[string]decision.Answer{"source": {ID: "source", Kind: decision.KindChoice, Choice: "clear", Probabilities: map[string]float64{"clear": .9, "concern": .05, "insufficient": .05}}}}
	got := evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	if d.calls != 3 || got.Cache == "hit" || !got.Sites[0].Held || got.Sites[0].Outcome != "unknown" {
		t.Fatalf("hold authorized by cache: %+v calls=%d", got, d.calls)
	}
	evaluateModelsWith(context.Background(), p, "commit", "policy", s, opts, d)
	if d.calls != 4 {
		t.Fatalf("hold reused cache on next call: %d", d.calls)
	}
}

func TestHTTPModelAttestationPreventsAdapterFallback(t *testing.T) {
	setModelEnv(t)
	p, s := modelFixture("hold")
	now := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC)
	calibratedConfidence := .9
	c := &calibration{Site: "repo/commit/source", Provider: "openai", ResolvedModel: "pinned", ValidUntil: now.Add(time.Hour), MinPerClass: 1, MinN: 1, MinAccuracy: .1,
		Cases: []labeledCase{{ID: "clear", Expected: "clear", Actual: "clear", Confidence: calibratedConfidence, Provenance: "human"}, {ID: "concern", Expected: "concern", Actual: "concern", Confidence: calibratedConfidence, Provenance: "human"}, {ID: "insufficient", Expected: "insufficient", Actual: "insufficient", Confidence: calibratedConfidence, Provenance: "computed"}}}
	c.QuestionSHA256 = siteDigest(p.Model.Sites[0])
	p.Model.Sites[0].Calibration = c
	validAnswer := `"answers":[{"type":"choice","name":"source","choice":"clear","probabilities":[{"value":"clear","probability":0.9},{"value":"concern","probability":0.05},{"value":"insufficient","probability":0.05}]}],"usage":{"input_tokens":4,"output_tokens":2}`
	for _, tc := range []struct {
		name, body, reason string
		tokens             int
	}{
		{"explicit", `{"model":"pinned",` + validAnswer + `}`, "", 4},
		{"missing", `{` + validAnswer + `}`, "model-drift", 4},
		{"duplicate", `{"model":"pinned","model":"pinned",` + validAnswer + `}`, "model-drift", 4},
		{"wrong", `{"model":"other",` + validAnswer + `}`, "model-drift", 4},
		{"malformed", `{"model":"pinned",`, "invalid-answer", 0},
		{"oversize", fmt.Sprintf(`{"model":"pinned","padding":"%s"}`, strings.Repeat("x", maxModelResponseBytes)), "input-limit", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/decisions" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			dec, err := openai.NewOpenAI(openai.OpenAIConfig{APIKey: "test-only", BaseURL: srv.URL, Model: "pinned", HTTPClient: newAttestingClient(time.Second, "pinned")})
			if err != nil {
				t.Fatal(err)
			}
			got := evaluateModelsWith(context.Background(), p, "commit", "policy", s, modelOptions{AllowRemote: true, Now: func() time.Time { return now }}, dec)
			if calls != 1 || got.InputTokens != tc.tokens {
				t.Fatalf("calls=%d usage=%d result=%+v", calls, got.InputTokens, got)
			}
			if tc.reason == "" {
				if got.Sites[0].Outcome != "clear" || got.Sites[0].Held {
					t.Fatalf("explicit model rejected: %+v", got)
				}
			} else if got.Sites[0].Reason != tc.reason || !got.Sites[0].Held {
				t.Fatalf("missing/wrong wire model passed: %+v", got)
			}
		})
	}
}

func TestClearCalibrationUsesOnlyClearPredictions(t *testing.T) {
	site := modelSite{ID: "source", Version: "1", Question: "Question", MinConfidence: .5}
	now := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC)
	c := &calibration{Site: "repo/commit/source", Provider: "openai", ResolvedModel: "pinned", ValidUntil: now.Add(time.Hour), MinPerClass: 1, MinN: 5, MinAccuracy: .9}
	c.QuestionSHA256 = siteDigest(site)
	for i := 0; i < 50; i++ {
		c.Cases = append(c.Cases, labeledCase{ID: fmt.Sprintf("concern-%d", i), Expected: "concern", Actual: "concern", Confidence: .95, Provenance: "human"})
	}
	for i := 0; i < 5; i++ {
		c.Cases = append(c.Cases, labeledCase{ID: fmt.Sprintf("false-clear-%d", i), Expected: "concern", Actual: "clear", Confidence: .95, Provenance: "human"})
	}
	c.Cases = append(c.Cases, labeledCase{ID: "true-clear", Expected: "clear", Actual: "clear", Confidence: .95, Provenance: "human"}, labeledCase{ID: "insufficient", Expected: "insufficient", Actual: "insufficient", Confidence: .95, Provenance: "computed"})
	site.Calibration = c
	if got := calibrationStatus("repo", "commit", site, "openai", "pinned", "clear", .95, now); got != "fail" {
		t.Fatalf("false clear masked by concern accuracy: %s", got)
	}
}

func TestAttestingClientRefusesRedirect(t *testing.T) {
	redirected := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected++
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	resp, err := newAttestingClient(time.Second, "pinned").Post(source.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect || redirected != 0 {
		t.Fatalf("redirect followed: status=%d target_calls=%d", resp.StatusCode, redirected)
	}
}

func TestAttestingClientBoundsErrorResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		tokens           int
	}{
		{"error-usage", `{"usage":{"input_tokens":7,"output_tokens":2},"error":"secret body"}`, "provider-unavailable", 7},
		{"error-malformed", `{"usage":`, "provider-unavailable", 0},
		{"error-oversize", strings.Repeat("secret", maxModelResponseBytes/6+1), "input-limit", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			resp, err := newAttestingClient(time.Second, "pinned").Post(srv.URL, "application/json", strings.NewReader(`{}`))
			if resp != nil {
				resp.Body.Close()
				t.Fatalf("error body reached adapter: status=%d", resp.StatusCode)
			}
			var attested *modelAttestationError
			if !errors.As(err, &attested) || attested.reason != tc.want || attested.usage.InputTokens != tc.tokens || strings.Contains(err.Error(), "secret") {
				t.Fatalf("err=%v attestation=%+v", err, attested)
			}
		})
	}
}

func TestAttestingClientTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := newAttestingClient(time.Second, "pinned").Do(req)
	if resp != nil {
		resp.Body.Close()
		t.Fatal("timed-out response reached adapter")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
}
