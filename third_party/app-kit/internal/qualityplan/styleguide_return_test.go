package qualityplan

import (
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
)

const (
	fathomGuideArtifact = "52666a587cc44dfc146064cd58d5003d34d79d319592d8d16d6394c1ff454198"
	fathomGuideEvidence = "docs/assessments/2026-09-22-fathom-guide-only.md"
)

func TestStyleguideReturnRecordsFathomWithoutPromotion(t *testing.T) {
	capabilities, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := capabilities.Get("recipe.styleguide.local")
	if !ok {
		t.Fatal("style-guide capability is missing")
	}
	if entry.Maturity != "experimental" || !slices.Contains(entry.Consumers, "Fathom") {
		t.Fatalf("catalog lost experimental Fathom return boundary: %+v", entry)
	}
	joined := strings.Join(append([]string{entry.Usage}, entry.Gaps...), " ")
	for _, want := range []string{fathomGuideEvidence, "large-value chart geometry", "actual zoom", "full G3/G4"} {
		if !strings.Contains(joined, want) {
			t.Errorf("catalog Fathom return omits %q", want)
		}
	}
	if strings.Contains(joined, "No real-consumer migration") {
		t.Fatal("catalog still denies the recorded Fathom integration")
	}

	plan, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	detail, err := plan.Get("core.themes-style-guide", "")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Item.State != "in-progress" || detail.Item.Gates["performance"].Status != "failed" ||
		detail.Item.Gates["integration"].Status != "partial" {
		t.Fatalf("Fathom return promoted unfinished quality: %+v", detail.Item)
	}
	for _, consumer := range detail.Item.Consumers {
		if consumer.App != "fathom" {
			continue
		}
		if consumer.IntegrationStatus != "verified-scoped" || consumer.Revision.Kind != "artifact" ||
			consumer.Revision.SHA256 != fathomGuideArtifact {
			t.Fatalf("wrong Fathom consumer boundary: %+v", consumer)
		}
		if len(consumer.Evidence) != 1 || consumer.Evidence[0].Path != fathomGuideEvidence {
			t.Fatalf("wrong Fathom evidence: %+v", consumer.Evidence)
		}
		return
	}
	t.Fatal("quality inventory omits Fathom consumer return")
}
