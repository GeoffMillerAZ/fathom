package decide

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	llmclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// countingDecider answers every question at confidence 0.95 under model
// and counts its asks: the fake provider (no adapter, no key, no spend).
type countingDecider struct {
	name, model string
	mu          sync.Mutex
	calls       int
}

func (c *countingDecider) Name() string { return c.name }

func (c *countingDecider) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	resp := decision.Response{Provider: c.name, Model: c.model, Answers: map[string]decision.Answer{}}
	for _, q := range req.Questions {
		resp.Answers[q.ID] = decision.Answer{ID: q.ID, Kind: q.Kind, Noul: 0.95, Confidence: 0.95}
	}
	return resp, nil
}

func (c *countingDecider) asked() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// writeTestReading writes a reading of n proceed-band samples, correct of
// them right, under dir, with an A/B of baseline right out of n when
// baseline >= 0.
func writeTestReading(t *testing.T, dir, site, provider, model string, n, correct, baseline int) {
	t.Helper()
	samples := make([]decision.LabeledSample, n)
	for i := range samples {
		samples[i] = decision.LabeledSample{Confidence: 0.95, Correct: i < correct, Provenance: decision.ProvenanceComputed}
	}
	key := decision.ReadingKey{Site: site, TemplateVersion: TemplateVersion(site), Provider: provider, ResolvedModel: model}
	r, err := decision.NewReading(key, samples)
	if err != nil {
		t.Fatal(err)
	}
	cr := CalibrationReading{Schema: CalibrationReadingSchema, Taken: "2026-10-20T00:00:00Z", Reading: r}
	if baseline >= 0 {
		cr.Baseline = &ABReading{N: n, Correct: baseline}
	}
	data, err := json.Marshal(cr)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ReadingFileBase("2026-10-20", key)+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

var predicateRequest = decision.Request{State: "s", Questions: []decision.Question{{ID: "q1", Kind: decision.KindPredicate, Instructions: "is it?"}}}

// TestUncalibratedSiteRefusesToRule pins SR-58 (docs/plans/surface-
// reckoning.md, F-69): a decision-port site outside certify is a
// proposal. It rules only when the latest calibration reading for (site,
// template, provider, resolved model) clears the site's bar and an A/B
// shows the provider beating the deterministic answer; otherwise it is
// never asked (or, on a model move, its answer is dropped), the site gets
// the deterministic path's signal and one "decide: not calibrated" line.
// certify_suggest keeps its own rule, and provider none is unchanged.
func TestUncalibratedSiteRefusesToRule(t *testing.T) {
	ctx := context.Background()
	refused := func(t *testing.T, err error, want string) {
		t.Helper()
		var nce *NotCalibratedError
		if !errors.As(err, &nce) {
			t.Fatalf("err = %v, want a *NotCalibratedError", err)
		}
		if !errors.Is(err, decision.ErrNoProvider) {
			t.Errorf("err = %v, want it to read as no provider, so the site takes its deterministic path", err)
		}
		if MissClass(err) != NotCalibratedClass {
			t.Errorf("MissClass = %q, want %q", MissClass(err), NotCalibratedClass)
		}
		if !strings.Contains(nce.Reason, want) {
			t.Errorf("reason = %q, want it to contain %q", nce.Reason, want)
		}
	}

	t.Run("no reading refuses and serves the fallback", func(t *testing.T) {
		dir := t.TempDir()
		var buf bytes.Buffer
		fake := &countingDecider{name: "typesafe", model: "m1"}
		d := propose(fake, &degrader{w: &buf}, dir)
		for range 2 {
			resp, err := d.Decide(WithSite(ctx, SiteRoute), predicateRequest)
			refused(t, err, "no calibration reading")
			if len(resp.Answers) != 0 {
				t.Errorf("answers = %v, want none served", resp.Answers)
			}
		}
		if fake.asked() != 0 {
			t.Errorf("provider asked %d times, want 0: an uncalibrated site spends nothing", fake.asked())
		}
		if got := strings.Count(buf.String(), "decide: not calibrated"); got != 1 {
			t.Errorf("disclosed lines = %d (%q), want exactly 1", got, buf.String())
		}
		if !strings.Contains(buf.String(), "site route") || !strings.Contains(buf.String(), "deterministic path") {
			t.Errorf("line = %q, want it to name the site and the deterministic path", buf.String())
		}
	})

	t.Run("a failing reading refuses", func(t *testing.T) {
		dir := t.TempDir()
		writeTestReading(t, dir, SiteRoute, "typesafe", "m1", 100, 50, 10)
		fake := &countingDecider{name: "typesafe", model: "m1"}
		_, err := propose(fake, &degrader{w: &bytes.Buffer{}}, dir).Decide(WithSite(ctx, SiteRoute), predicateRequest)
		refused(t, err, string(decision.CalibrationFail))
		if fake.asked() != 0 {
			t.Errorf("provider asked %d times, want 0", fake.asked())
		}
	})

	t.Run("a passing reading without an A/B refuses", func(t *testing.T) {
		dir := t.TempDir()
		writeTestReading(t, dir, SiteRoute, "typesafe", "m1", 100, 100, -1)
		_, err := propose(&countingDecider{name: "typesafe", model: "m1"}, &degrader{w: &bytes.Buffer{}}, dir).Decide(WithSite(ctx, SiteRoute), predicateRequest)
		refused(t, err, "no A/B")
	})

	t.Run("an A/B the provider loses refuses", func(t *testing.T) {
		dir := t.TempDir()
		writeTestReading(t, dir, SiteRoute, "typesafe", "m1", 100, 95, 97)
		_, err := propose(&countingDecider{name: "typesafe", model: "m1"}, &degrader{w: &bytes.Buffer{}}, dir).Decide(WithSite(ctx, SiteRoute), predicateRequest)
		refused(t, err, "does not beat")
	})

	t.Run("a passing reading and a won A/B rule", func(t *testing.T) {
		dir := t.TempDir()
		writeTestReading(t, dir, SiteRoute, "typesafe", "m1", 100, 100, 60)
		fake := &countingDecider{name: "typesafe", model: "m1"}
		resp, err := propose(fake, &degrader{w: &bytes.Buffer{}}, dir).Decide(WithSite(ctx, SiteRoute), predicateRequest)
		if err != nil {
			t.Fatalf("err = %v, want the provider's answer", err)
		}
		if _, ok := resp.Answers["q1"]; !ok || fake.asked() != 1 {
			t.Errorf("answers = %v asked %d, want the provider's answer from one ask", resp.Answers, fake.asked())
		}
	})

	t.Run("a model move makes the site due", func(t *testing.T) {
		dir := t.TempDir()
		writeTestReading(t, dir, SiteRoute, "typesafe", "m1", 100, 100, 60)
		_, err := propose(&countingDecider{name: "typesafe", model: "m2"}, &degrader{w: &bytes.Buffer{}}, dir).Decide(WithSite(ctx, SiteRoute), predicateRequest)
		refused(t, err, "recalibration due")
	})

	t.Run("a pruning site needs the gate bar", func(t *testing.T) {
		dir := t.TempDir()
		// 95 of 100 clears the hint bar but its Wilson lower bound (0.888)
		// is under I62's 0.90.
		writeTestReading(t, dir, SiteCompilePrefilter, "typesafe", "m1", 100, 95, 60)
		writeTestReading(t, dir, SiteAssess, "typesafe", "m1", 100, 95, 60)
		d := propose(&countingDecider{name: "typesafe", model: "m1"}, &degrader{w: &bytes.Buffer{}}, dir)
		_, err := d.Decide(WithSite(ctx, SiteCompilePrefilter), predicateRequest)
		refused(t, err, string(decision.CalibrationFail))
		if _, err := d.Decide(WithSite(ctx, SiteAssess), predicateRequest); err != nil {
			t.Errorf("assess at the hint bar: err = %v, want it to rule", err)
		}
	})

	t.Run("an ask that names no site is not a site's ask", func(t *testing.T) {
		fake := &countingDecider{name: "typesafe", model: "m1"}
		if _, err := propose(fake, &degrader{w: &bytes.Buffer{}}, t.TempDir()).Decide(ctx, predicateRequest); err != nil || fake.asked() != 1 {
			t.Errorf("unscoped ask: err %v asked %d, want it passed through", err, fake.asked())
		}
	})

	t.Run("certify is unaffected", func(t *testing.T) {
		var buf bytes.Buffer
		fake := &countingDecider{name: "typesafe", model: "m1"}
		resp, err := propose(fake, &degrader{w: &buf}, t.TempDir()).Decide(WithSite(ctx, SiteCertifySuggest), predicateRequest)
		if err != nil || len(resp.Answers) != 1 || buf.Len() != 0 {
			t.Errorf("certify_suggest: err %v answers %v line %q, want its answer and no line", err, resp.Answers, buf.String())
		}
		if ok, why := requireCalibratedSite(SiteCertifySuggest, decision.ReadingKey{Site: SiteCertifySuggest}, nil); !ok {
			t.Errorf("requireCalibratedSite(certify_suggest) = false (%s), want exempt", why)
		}
	})

	t.Run("a refusal is not a miss and its row names the deterministic tier", func(t *testing.T) {
		var log bytes.Buffer
		rec := NewRecorder(&log, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
		err := &DegradedError{Provider: "typesafe", Err: &NotCalibratedError{Site: SiteRoute, Reason: "no calibration reading"}}
		rec.RecordError(SiteRoute, "typesafe", "", 1, 0, err)
		var row map[string]any
		if jerr := json.Unmarshal(log.Bytes(), &row); jerr != nil {
			t.Fatalf("row %q: %v", log.String(), jerr)
		}
		if row["decided_by"] != DecidedComputed {
			t.Errorf("decided_by = %v, want %q", row["decided_by"], DecidedComputed)
		}
		if counted, _ := askOutcome(err.Error(), ""); counted {
			t.Error("a not-calibrated refusal counted toward the site's miss window; the provider never saw it")
		}
	})

	t.Run("provider none is byte-identical", func(t *testing.T) {
		var buf bytes.Buffer
		ports, err := Open(config.Decide{Provider: "none"}, nil, "", nil, WithDisclose(&buf), WithReadingsDir(t.TempDir()))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := ports.Decider.(decision.None); !ok {
			t.Fatalf("Decider = %T, want decision.None", ports.Decider)
		}
		_, derr := ports.Decider.Decide(WithSite(ctx, SiteRoute), predicateRequest)
		var nce *NotCalibratedError
		if !errors.Is(derr, decision.ErrNoProvider) || errors.As(derr, &nce) || buf.Len() != 0 {
			t.Errorf("none: err %v line %q, want ErrNoProvider alone and no line", derr, buf.String())
		}
	})

	t.Run("Open puts every non-certify ask behind the gate", func(t *testing.T) {
		client := llmclient.NewMockClient()
		lane := func(string) (llmclient.LLMClient, error) { return client, nil }
		var buf bytes.Buffer
		ports, err := Open(config.Decide{Provider: "llm"}, lane, "m1", nil, WithDisclose(&buf), WithReadingsDir(t.TempDir()))
		if err != nil {
			t.Fatal(err)
		}
		_, derr := ports.Decider.Decide(WithSite(ctx, SiteObligationsRerank), predicateRequest)
		refused(t, derr, "no calibration reading")
		if client.CallCount() != 0 {
			t.Errorf("lane called %d times, want 0", client.CallCount())
		}
		_, gerr := ports.Gated(func(d decision.Decider) decision.Decider { return d }).Decide(WithSite(ctx, SiteRoute), predicateRequest)
		refused(t, gerr, "no calibration reading")
		_, _ = ports.Grader().Decide(WithSite(ctx, SiteRoute), predicateRequest)
		if client.CallCount() == 0 {
			t.Error("Grader() asked nothing: calibrate must grade through the port without the gate")
		}
	})
}
