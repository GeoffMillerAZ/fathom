package decide

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// WithReadingsDir names the directory the calibration gate reads its
// readings from, in place of ReadingsDir().
func WithReadingsDir(dir string) Option {
	return func(o *openOptions) { o.readingsDir, o.readingsSet = dir, true }
}

// CalibratedFor is an Option whose readings directory holds, for every one
// of sites, a reading that lets it rule on provider under model (100 of
// 100 right in the proceed band, the deterministic answer 50 of 100):
// what a test of the provider path behind the calibration gate (SR-58)
// opens its port with.
func CalibratedFor(t testing.TB, provider, model string, sites ...string) Option {
	t.Helper()
	dir := t.TempDir()
	for _, site := range sites {
		samples := make([]decision.LabeledSample, 100)
		for i := range samples {
			samples[i] = decision.LabeledSample{Confidence: 0.95, Correct: true, Provenance: decision.ProvenanceComputed}
		}
		key := decision.ReadingKey{Site: site, TemplateVersion: TemplateVersion(site), Provider: provider, ResolvedModel: model}
		r, err := decision.NewReading(key, samples)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(CalibrationReading{Schema: CalibrationReadingSchema, Taken: "2026-10-20T00:00:00Z", Reading: r, Baseline: &ABReading{N: 100, Correct: 50}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ReadingFileBase("2026-10-20", key)+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return WithReadingsDir(dir)
}
