package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// calibrateSites writes under dir a reading in which each of sites may rule
// on provider under model (SR-58's gate): 100 of 100 right in the proceed
// band, the deterministic answer 50 of 100.
func calibrateSites(t *testing.T, dir, provider, model string, sites ...string) {
	t.Helper()
	samples := make([]decision.LabeledSample, 100)
	for i := range samples {
		samples[i] = decision.LabeledSample{Confidence: 0.95, Correct: true, Provenance: decision.ProvenanceComputed}
	}
	for _, site := range sites {
		r, err := decision.NewReading(decision.ReadingKey{Site: site, TemplateVersion: decide.TemplateVersion(site), Provider: provider, ResolvedModel: model}, samples)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decide.WriteReadingJSON(dir, "2026-10-20", decide.CalibrationReading{Taken: "2026-10-20T00:00:00Z", Reading: r, Baseline: &decide.ABReading{N: 100, Correct: 50}}); err != nil {
			t.Fatal(err)
		}
	}
}

func near(got *float64, want float64) bool { return got != nil && math.Abs(*got-want) < 1e-4 }

// TestWriteCalibrationReadingCarriesWilsonBrierAndProvenance pins JF-11
// (docs/plans/judgment-foundations.md, A8): a calibration reading per
// (site, template version, provider, resolved model) is written as a
// docs/bench/readings-shaped pair, a JSON reading the gate reads and a
// Markdown reading with a plain verdict and a confidence-bucket table, with
// n, accuracy by band, Wilson bounds, Brier score, AUROC and the label
// provenance mix; deterministic under an injected clock; an empty set says
// so and never states an accuracy of 0.
func TestWriteCalibrationReadingCarriesWilsonBrierAndProvenance(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) }
	key := decision.ReadingKey{Site: decide.SiteRoute, TemplateVersion: decide.TemplateVersion(decide.SiteRoute), Provider: "typesafe", ResolvedModel: "vendor/model-1.2"}
	r, err := decision.NewReading(key, []decision.LabeledSample{
		{Confidence: 0.95, Correct: true, Provenance: decision.ProvenanceHuman},
		{Confidence: 0.92, Correct: true, Provenance: decision.ProvenanceComputed},
		{Confidence: 0.6, Correct: false, Provenance: decision.ProvenanceFrontier},
		{Confidence: 0.3, Correct: false, Provenance: decision.ProvenanceComputed},
	})
	if err != nil {
		t.Fatal(err)
	}
	cr := decide.CalibrationReading{Reading: r, Baseline: &decide.ABReading{N: 4, Correct: 1}}

	dir := t.TempDir()
	mdPath, jsonPath, err := writeCalibrationReading(dir, cr, now)
	if err != nil {
		t.Fatalf("writeCalibrationReading: %v", err)
	}
	if want := "decision-port-calibration-2026-10-20.route.typesafe.vendor_model-1_2"; filepath.Base(jsonPath) != want+".json" || filepath.Base(mdPath) != want+".md" {
		t.Errorf("paths = %s, %s, want %s.{json,md}", jsonPath, mdPath, want)
	}

	var got decide.CalibrationReading
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("the JSON reading does not decode: %v", err)
	}
	if got.Schema != decide.CalibrationReadingSchema || got.Taken != "2026-10-20T12:00:00Z" || got.Reading.Key != key {
		t.Errorf("schema %q taken %q key %+v, want the schema, the injected clock and the key", got.Schema, got.Taken, got.Reading.Key)
	}
	// 2 of 4: Wilson 95% [0.1500, 0.8500]; Brier (0.05² + 0.08² + 0.6² +
	// 0.3²)/4 = 0.114725; every right answer outranks every wrong one.
	if !near(got.Reading.AccuracyLo, 0.1500) || !near(got.Reading.AccuracyHi, 0.8500) {
		t.Errorf("Wilson = %v..%v, want 0.1500..0.8500", got.Reading.AccuracyLo, got.Reading.AccuracyHi)
	}
	if !near(got.Reading.Brier, 0.114725) || !near(got.Reading.AUROC, 1) {
		t.Errorf("Brier %v AUROC %v, want 0.114725 and 1", got.Reading.Brier, got.Reading.AUROC)
	}
	if got.Reading.Labels != (decision.ProvenanceMix{Human: 1, Frontier: 1, Computed: 2}) {
		t.Errorf("labels = %+v, want human 1, frontier 1, computed 2", got.Reading.Labels)
	}
	if got.Baseline == nil || got.Baseline.N != 4 || got.Baseline.Correct != 1 || !near(got.Baseline.Accuracy, 0.25) {
		t.Errorf("baseline = %+v, want 1 of 4 at 0.25", got.Baseline)
	}

	md, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## Verdict, plainly", "2 of 4 right", "0.1500 to 0.8500", "Brier score: 0.1147", "AUROC: 1.0000",
		"human: 1", "frontier: 1", "computed: 2", "| proceed | 2/2 | 1.0000 |", "may not rule",
		"deterministic answer: 1 of 4 right",
	} {
		if !strings.Contains(string(md), want) {
			t.Errorf("the Markdown reading lacks %q:\n%s", want, md)
		}
	}
	if table, ok := console.ParseCalibrationReading(mdPath, md); !ok || len(table.Buckets) != 3 || table.Verdict == "" {
		t.Errorf("fathom decide log cannot read it: %+v (ok %v)", table, ok)
	}

	again := t.TempDir()
	mdAgain, jsonAgain, err := writeCalibrationReading(again, cr, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{mdPath, mdAgain}, {jsonPath, jsonAgain}} {
		a, _ := os.ReadFile(pair[0])
		b, _ := os.ReadFile(pair[1])
		if !bytes.Equal(a, b) {
			t.Errorf("%s is not byte-identical across two writes", filepath.Base(pair[0]))
		}
	}

	empty, err := decision.NewReading(key, nil)
	if err != nil {
		t.Fatal(err)
	}
	emptyMD, emptyJSON, err := writeCalibrationReading(t.TempDir(), decide.CalibrationReading{Reading: empty}, now)
	if err != nil {
		t.Fatal(err)
	}
	md, _ = os.ReadFile(emptyMD)
	if !strings.Contains(string(md), "no labeled samples") || strings.Contains(string(md), "0.0000") {
		t.Errorf("an empty set must say so and state no 0:\n%s", md)
	}
	data, _ = os.ReadFile(emptyJSON)
	var raw struct {
		Reading map[string]any `json:"reading"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if v, ok := raw.Reading["accuracy"]; !ok || v != nil {
		t.Errorf("empty reading accuracy = %v (present %v), want an explicit null", v, ok)
	}
}

// calibFake answers a predicate true when its state ends in "true", at
// confidence 0.97, under model, and notes the site each ask came from.
type calibFake struct {
	model string
	mu    sync.Mutex
	sites []string
}

func (f *calibFake) Name() string { return "typesafe" }

func (f *calibFake) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	f.mu.Lock()
	f.sites = append(f.sites, decide.ScopeFrom(ctx).Site)
	f.mu.Unlock()
	noul := 0.03
	if strings.HasSuffix(req.State, "true") {
		noul = 0.97
	}
	resp := decision.Response{Provider: "typesafe", Model: f.model, Answers: map[string]decision.Answer{}}
	for _, q := range req.Questions {
		resp.Answers[q.ID] = decision.Answer{ID: q.ID, Kind: q.Kind, Noul: noul, Confidence: 0.97}
	}
	return resp, nil
}

// TestCmdDecideCalibrateRegradesEverySite pins JF-12 (owner rule 2):
// `fathom decide calibrate --provider X` grades every registered site
// against its calibration set and writes one JF-11 reading per site, with
// the deterministic answer's A/B on the same set; provider none grades the
// deterministic answer itself; a site with no set is named; a provider that
// cannot open (no key) is refused with the disclosed line, never skipped.
func TestCmdDecideCalibrateRegradesEverySite(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Setenv(decide.KillEnv, "")
	sets, out := t.TempDir(), t.TempDir()
	var withSets []string
	for _, site := range decide.AllSites {
		if site == decide.SiteAssess {
			continue // no set: named, never silently passed over
		}
		withSets = append(withSets, site)
		var b strings.Builder
		for i, item := range []struct{ state, expected, baseline, provenance string }{
			{"a true", "true", "true", "human"}, {"b true", "true", "false", "computed"}, {"c false", "false", "false", "frontier"},
		} {
			line, _ := json.Marshal(map[string]any{"id": "c" + string(rune('1'+i)), "state": item.state, "kind": "predicate",
				"instructions": "Is it so?", "expected": item.expected, "baseline": item.baseline, "provenance": item.provenance})
			b.Write(append(line, '\n'))
		}
		if err := os.WriteFile(filepath.Join(sets, site+".jsonl"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	prevPorts, prevNow := decideCalibratePorts, decideCalibrateNow
	t.Cleanup(func() { decideCalibratePorts, decideCalibrateNow = prevPorts, prevNow })
	decideCalibrateNow = func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) }
	fake := &calibFake{model: "fake-model-1"}
	decideCalibratePorts = func(loaded config.Loaded, _ io.Writer) *decide.Ports {
		return &decide.Ports{Decider: fake, Cfg: loaded.Config.Decide}
	}

	var stdout, stderr bytes.Buffer
	if code := cmdDecideCalibrate([]string{"--provider", "typesafe", "--sets", sets, "--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr.String())
	}
	for _, site := range withSets {
		cr, err := decide.LatestReading(out, site, "typesafe")
		if err != nil || cr == nil {
			t.Fatalf("site %s: no reading (%v)", site, err)
		}
		if cr.Reading.N != 3 || cr.Reading.Correct != 3 || cr.Reading.Key.ResolvedModel != "fake-model-1" || cr.Reading.Key.TemplateVersion != decide.TemplateVersion(site) {
			t.Errorf("site %s: reading %+v, want 3 of 3 under fake-model-1", site, cr.Reading)
		}
		if cr.Baseline == nil || cr.Baseline.N != 3 || cr.Baseline.Correct != 2 {
			t.Errorf("site %s: A/B %+v, want the deterministic answer 2 of 3", site, cr.Baseline)
		}
		if cr.Reading.Labels != (decision.ProvenanceMix{Human: 1, Frontier: 1, Computed: 1}) {
			t.Errorf("site %s: labels %+v, want one of each", site, cr.Reading.Labels)
		}
		if !slices.Contains(fake.sites, site) {
			t.Errorf("site %s was never asked under its own scope (asked: %v)", site, fake.sites)
		}
	}
	if !strings.Contains(stdout.String(), "assess: no calibration set") {
		t.Errorf("the site with no set is not named:\n%s", stdout.String())
	}
	if r, _ := decide.LatestReading(out, decide.SiteAssess, "typesafe"); r != nil {
		t.Errorf("assess has no set but got a reading: %+v", r)
	}

	t.Run("provider none grades the deterministic answer", func(t *testing.T) {
		decideCalibratePorts = prevPorts
		noneOut := t.TempDir()
		var so, se bytes.Buffer
		if code := cmdDecideCalibrate([]string{"--provider", "none", "--sets", sets, "--out", noneOut}, &so, &se); code != 0 {
			t.Fatalf("exit %d, stderr:\n%s", code, se.String())
		}
		cr, err := decide.LatestReading(noneOut, decide.SiteRoute, "none")
		if err != nil || cr == nil {
			t.Fatalf("no baseline reading (%v)", err)
		}
		if cr.Reading.N != 3 || cr.Reading.Correct != 2 || cr.Baseline != nil {
			t.Errorf("baseline reading %+v (A/B %+v), want 2 of 3 and no A/B against itself", cr.Reading, cr.Baseline)
		}
	})

	t.Run("a provider with no key is refused with the disclosed line", func(t *testing.T) {
		decideCalibratePorts = prevPorts
		for _, v := range []string{"OPENAI_API_KEY", "OPENROUTER_API_KEY", "TYPESAFE_API_KEY"} {
			t.Setenv(v, "")
		}
		keyOut := t.TempDir()
		var so, se bytes.Buffer
		if code := cmdDecideCalibrate([]string{"--provider", "typesafe", "--sets", sets, "--out", keyOut}, &so, &se); code == 0 {
			t.Fatalf("exit 0 with no key; stdout:\n%s", so.String())
		}
		if !strings.Contains(se.String(), "warn: decision port disabled") || !strings.Contains(se.String(), "refused") {
			t.Errorf("stderr lacks the disclosed line and the refusal:\n%s", se.String())
		}
		if m, _ := filepath.Glob(filepath.Join(keyOut, "*")); len(m) != 0 {
			t.Errorf("a refused run wrote %v", m)
		}
	})

	if strings.Contains(stdout.String()+stderr.String(), "JF-") || strings.Contains(stdout.String()+stderr.String(), "SR-") {
		t.Errorf("output names a row id:\n%s%s", stdout.String(), stderr.String())
	}
}
