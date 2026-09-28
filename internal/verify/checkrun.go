package verify

// checkrun.go — GL-21 (docs/plans/governance-learning.md, design doc §8
// "Check payload"): a verb writes a GitHub check-run JSON body (name,
// conclusion, output.summary, output.annotations[]) to stdout or a file so
// the caller's CI (or any forge app) can POST it to the Checks API.
// **fathom never posts it itself** — the same "reviewer output, not a
// reviewer" discipline sarif.go documents for SARIF, and the design doc's
// §12 refusal ("posting to a forge or running a review service").
//
// Deterministic: annotations keep the caller's own order (GateCheckRunAnnotations
// hands them in report order, sorted by task id, the same order GateSARIF
// already established); renderCheckRun only truncates, never reorders.

import (
	"encoding/json"
	"fmt"
)

// checkRunMaxAnnotations is the GitHub Checks API's own per-request
// annotation cap (Update a check run: "annotations" accepts at most 50
// entries per call). A caller with more findings than this makes several
// calls; renderCheckRun's job is one call's worth, so it truncates rather
// than emitting a payload the API would reject outright.
const checkRunMaxAnnotations = 50

// CheckRunAnnotation is one GitHub Checks API annotation: a location
// (Path, StartLine..EndLine) plus a level and message. Title is optional
// (the API's own "title" field); an empty Title is simply omitted.
type CheckRunAnnotation struct {
	Path            string
	StartLine       int
	EndLine         int
	AnnotationLevel string // "notice" | "warning" | "failure" (Checks API vocabulary)
	Message         string
	Title           string
}

// checkRunAnnotationJSON is CheckRunAnnotation's wire shape — the Checks
// API's own field names (snake_case), never fathom's Go-idiomatic ones.
type checkRunAnnotationJSON struct {
	Path            string `json:"path"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	AnnotationLevel string `json:"annotation_level"`
	Message         string `json:"message"`
	Title           string `json:"title,omitempty"`
}

// renderCheckRun builds the check-run JSON body for name/conclusion/summary
// over annotations, capped at checkRunMaxAnnotations. conclusion is the
// caller's own choice from the Checks API vocabulary ("success", "failure",
// "neutral", …) — this function renders whatever it is handed rather than
// computing one, so a caller scoring its own domain (a plan gate, a review
// run) stays the single source of that judgment.
func renderCheckRun(name, conclusion, summary string, annotations []CheckRunAnnotation) []byte {
	capped := annotations
	if len(capped) > checkRunMaxAnnotations {
		capped = capped[:checkRunMaxAnnotations]
	}
	out := make([]checkRunAnnotationJSON, 0, len(capped))
	for _, a := range capped {
		out = append(out, checkRunAnnotationJSON{
			Path: a.Path, StartLine: a.StartLine, EndLine: a.EndLine,
			AnnotationLevel: a.AnnotationLevel, Message: a.Message, Title: a.Title,
		})
	}
	doc := map[string]any{
		"name":       name,
		"conclusion": conclusion,
		"output": map[string]any{
			"title":       name,
			"summary":     summary,
			"annotations": out,
		},
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		// Only strings/ints and the typed annotation slice above are
		// marshalled: an error here is a programming error, surfaced
		// inline rather than swallowed (sarif.go's renderSARIF precedent).
		return []byte(fmt.Sprintf(`{"name":%q,"conclusion":"failure","output":{"summary":%q}}`, name, err.Error()))
	}
	return append(b, '\n')
}

// RenderCheckRun is renderCheckRun's exported face — a verb hands it its
// own name/conclusion/summary/annotations.
func RenderCheckRun(name, conclusion, summary string, annotations []CheckRunAnnotation) []byte {
	return renderCheckRun(name, conclusion, summary, annotations)
}

// GateCheckRunAnnotations mirrors GateSARIF exactly (same inputs, same
// per-task/per-criterion walk, same report order) but renders the Checks
// API's own annotation shape instead of a SARIF result: one annotation per
// failing criterion of each claimed, non-skipped task, plus, under strict,
// one "warning"-level annotation per claimed task whose status is Unknown.
func GateCheckRunAnnotations(planFiles []string, reports []Report, strict bool) []CheckRunAnnotation {
	var out []CheckRunAnnotation
	for i, rep := range reports {
		file := ""
		if i < len(planFiles) {
			file = planFiles[i]
		}
		for _, tv := range rep.Tasks {
			if !tv.Claimed || tv.Skipped {
				continue
			}
			switch {
			case tv.Status == Fail:
				for _, c := range tv.Criteria {
					if c.Status != Fail {
						continue
					}
					out = append(out, CheckRunAnnotation{
						Path: file, StartLine: tv.Line, EndLine: tv.Line,
						AnnotationLevel: "failure",
						Title:           c.Name,
						Message:         fmt.Sprintf("%s %s: %s", tv.TaskID, tv.Title, c.Detail),
					})
				}
			case strict && tv.Status == Unknown:
				out = append(out, CheckRunAnnotation{
					Path: file, StartLine: tv.Line, EndLine: tv.Line,
					AnnotationLevel: "warning",
					Title:           tv.Label,
					Message:         fmt.Sprintf("%s %s: claimed [x] but unknown under --strict (%s)", tv.TaskID, tv.Title, gateFailDetail(tv, true)),
				})
			}
		}
	}
	return out
}
