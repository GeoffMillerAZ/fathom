package review_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestUnanchoredPaths(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", FilePath: "pkg/a.go"},
	}
	got := review.UnanchoredPaths([]string{"pkg/a.go", "docs/x.md", ".github/workflows/y.yml"}, nodes)
	want := []string{".github/workflows/y.yml", "docs/x.md"}
	if len(got) != len(want) {
		t.Fatalf("UnanchoredPaths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("UnanchoredPaths = %v, want %v", got, want)
		}
	}
}

func TestBuildCoverage(t *testing.T) {
	v := review.Verdict{
		Dimensions: []review.Dimension{
			{Name: "impact", Checked: true},
			{Name: "arch_conformance", Checked: false, Reason: "no rules configured"},
		},
	}
	cov := review.BuildCoverage(v, false, false, false)
	if cov.Complete {
		t.Fatal("an unchecked dimension must make Complete false")
	}
	if len(cov.Unknown) != 1 || cov.Unknown[0] != "arch_conformance: no rules configured" {
		t.Errorf("Unknown = %v", cov.Unknown)
	}

	allChecked := review.Verdict{Dimensions: []review.Dimension{{Name: "impact", Checked: true}}}
	clean := review.BuildCoverage(allChecked, false, false, false)
	if !clean.Complete || len(clean.Unknown) != 0 {
		t.Errorf("clean run = %+v, want complete with no unknowns", clean)
	}

	staleOnly := review.BuildCoverage(allChecked, false, true, false)
	if staleOnly.Complete || len(staleOnly.Unknown) != 1 || staleOnly.Unknown[0] != "stale_index" {
		t.Errorf("stale_index run = %+v", staleOnly)
	}
}

func TestReviewResponsePayload(t *testing.T) {
	v := review.Verdict{
		Service:      "svc",
		ChangedFiles: []string{"pkg/a.go"},
		Dimensions: []review.Dimension{
			{Name: review.DimImpact, Checked: true},
		},
		Summary: "1 changed file",
	}
	resp := review.BuildResponse(
		v, "report",
		review.ProducerInfo{Name: "fathom", Version: "v1", Build: "abc"},
		review.ReviewInputs{ManifestDigest: "md", SourceWatermark: "sw", IndexWatermark: "iw", RulesDigest: "rd", PlansDigest: "pd", ExtractorVersions: map[string]string{"fathom": "v1"}},
		false,
		review.Coverage{Complete: true},
		false, false, false,
		nil,
	)
	if resp.SchemaVersion != review.ReviewSchemaVersion {
		t.Errorf("SchemaVersion = %d", resp.SchemaVersion)
	}
	payload := resp.Payload()
	if payload["service"] != "svc" {
		t.Errorf("service = %v", payload["service"])
	}
	if payload["mode"] != "report" {
		t.Errorf("mode = %v", payload["mode"])
	}
	if payload["schema_version"] != 1 {
		t.Errorf("schema_version = %v", payload["schema_version"])
	}
	producer, ok := payload["producer"].(map[string]any)
	if !ok || producer["name"] != "fathom" {
		t.Errorf("producer = %v", payload["producer"])
	}
	coverage, ok := payload["coverage"].(map[string]any)
	if !ok || coverage["complete"] != true {
		t.Errorf("coverage = %v", payload["coverage"])
	}
	dims, ok := payload["dimensions"].([]any)
	if !ok || len(dims) != 1 {
		t.Fatalf("dimensions = %v", payload["dimensions"])
	}
}
