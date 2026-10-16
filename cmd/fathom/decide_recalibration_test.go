package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// TestOpenDecisionRecorderSeedsFromTheLogTail pins JF-10's product wiring:
// the recorder every entry point opens (openDecisionRecorder) is seeded from
// the existing decisions.jsonl, so the first row after a restart under a
// moved model is marked recalibration_due, and an unmoved one is not. The
// first row written after the restart is the one that tells a seeded
// recorder from an unseeded one: without the seed, a site's first logged
// model is never a move.
func TestOpenDecisionRecorderSeedsFromTheLogTail(t *testing.T) {
	prior := `{"schema_version":2,"site":"route","provider":"typesafe","model":"jev-1.13.0","question_id":"verb","kind":"choice"}` + "\n"
	answer := func(model string) decision.Response {
		return decision.Response{Provider: "typesafe", Model: model, Answers: map[string]decision.Answer{
			"verb": {ID: "verb", Kind: decision.KindChoice, Choice: "find_symbol", Confidence: 0.95},
		}}
	}
	for _, tc := range []struct {
		name  string
		model string
		want  bool
	}{
		{"a moved model's first row after the restart is due", "jev-1.14.0", true},
		{"an unmoved model's first row after the restart is not", "jev-1.13.0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			callLog := filepath.Join(dir, "calls.jsonl")
			logPath := filepath.Join(dir, "decisions.jsonl")
			if err := os.WriteFile(logPath, []byte(prior), 0o644); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			rec := openDecisionRecorder(callLog, &stderr)
			if rec == nil {
				t.Fatalf("openDecisionRecorder = nil: %s", stderr.String())
			}
			rec.Record("route", answer(tc.model), func(decision.Answer) bool { return false })

			data, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != 2 {
				t.Fatalf("decisions.jsonl has %d rows, want 2:\n%s", len(lines), data)
			}
			var row struct {
				Model            string `json:"model"`
				RecalibrationDue bool   `json:"recalibration_due"`
			}
			if err := json.Unmarshal([]byte(lines[1]), &row); err != nil {
				t.Fatal(err)
			}
			if row.Model != tc.model || row.RecalibrationDue != tc.want {
				t.Errorf("first row after the restart: model %s recalibration_due = %v, want model %s and %v (the log last said jev-1.13.0)",
					row.Model, row.RecalibrationDue, tc.model, tc.want)
			}
		})
	}
}
