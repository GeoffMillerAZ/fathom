package verify

// checkrun_test.go — GL-21 (docs/plans/governance-learning.md, design doc
// §8 "Check payload"): a GitHub check-run JSON body (name, conclusion,
// output.summary, output.annotations[]), capped at 50 annotations per the
// Checks API's own per-request limit. fathom never posts this — a verb
// writes it to stdout or a file and the caller's CI does the posting.

import (
	"encoding/json"
	"testing"
)

func TestCheckRunPayloadCapped(t *testing.T) {
	anns := make([]CheckRunAnnotation, 0, 75)
	for i := 0; i < 75; i++ {
		anns = append(anns, CheckRunAnnotation{
			Path: "docs/plans/a.md", StartLine: i + 1, EndLine: i + 1,
			AnnotationLevel: "failure", Message: "refused",
		})
	}
	a := renderCheckRun("fathom verify", "failure", "75 refusals", anns)
	b := renderCheckRun("fathom verify", "failure", "75 refusals", anns)
	if string(a) != string(b) {
		t.Fatal("renderCheckRun is not deterministic")
	}

	var doc struct {
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
		Output     struct {
			Summary     string `json:"summary"`
			Annotations []struct {
				Path            string `json:"path"`
				StartLine       int    `json:"start_line"`
				EndLine         int    `json:"end_line"`
				AnnotationLevel string `json:"annotation_level"`
				Message         string `json:"message"`
			} `json:"annotations"`
		} `json:"output"`
	}
	if err := json.Unmarshal(a, &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, a)
	}
	if doc.Name != "fathom verify" || doc.Conclusion != "failure" || doc.Output.Summary != "75 refusals" {
		t.Fatalf("shape wrong: %+v", doc)
	}
	if len(doc.Output.Annotations) != checkRunMaxAnnotations {
		t.Fatalf("annotations = %d, want capped at %d", len(doc.Output.Annotations), checkRunMaxAnnotations)
	}
	if doc.Output.Annotations[0].Path != "docs/plans/a.md" || doc.Output.Annotations[0].StartLine != 1 {
		t.Fatalf("first annotation wrong: %+v", doc.Output.Annotations[0])
	}
}

func TestCheckRunPayloadUncapped(t *testing.T) {
	anns := []CheckRunAnnotation{
		{Path: "docs/plans/a.md", StartLine: 3, EndLine: 3, AnnotationLevel: "warning", Message: "claimed but unknown"},
	}
	out := renderCheckRun("fathom verify", "neutral", "1 warning", anns)
	var doc struct {
		Output struct {
			Annotations []map[string]any `json:"annotations"`
		} `json:"output"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if len(doc.Output.Annotations) != 1 {
		t.Fatalf("annotations = %d, want 1 (no cap under the limit)", len(doc.Output.Annotations))
	}
}

func TestGateCheckRunAnnotations(t *testing.T) {
	reports := []Report{{Tasks: []TaskVerdict{
		{
			TaskID: "GL-1", Title: "rules source", Claimed: true, Status: Fail, Line: 5,
			Criteria: []Criterion{{Name: "symbol_present", Status: Fail, Detail: "not found"}},
		},
		{TaskID: "GL-2", Title: "cursor", Claimed: true, Status: Pass, Line: 6},
	}}}
	anns := GateCheckRunAnnotations([]string{"docs/plans/gl.md"}, reports, false)
	if len(anns) != 1 {
		t.Fatalf("annotations = %d, want 1 (only the failing criterion)", len(anns))
	}
	if anns[0].Path != "docs/plans/gl.md" || anns[0].StartLine != 5 || anns[0].AnnotationLevel != "failure" {
		t.Fatalf("annotation wrong: %+v", anns[0])
	}
}
