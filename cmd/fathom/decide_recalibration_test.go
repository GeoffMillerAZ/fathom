package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// TestOpenDecisionRecorderSeedsFromTheLogTail pins JF-10's product wiring:
// the recorder every entry point opens (openDecisionRecorder) is seeded from
// the existing decisions.jsonl, so the first row after a restart under a
// moved model is marked recalibration_due, and an unmoved one is not.
func TestOpenDecisionRecorderSeedsFromTheLogTail(t *testing.T) {
	dir := t.TempDir()
	callLog := filepath.Join(dir, "calls.jsonl")
	logPath := filepath.Join(dir, "decisions.jsonl")
	prior := `{"schema_version":2,"site":"route","provider":"typesafe","model":"jev-1.13.0","question_id":"verb","kind":"choice"}` + "\n"
	if err := os.WriteFile(logPath, []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	rec := openDecisionRecorder(callLog, &stderr)
	if rec == nil {
		t.Fatalf("openDecisionRecorder = nil: %s", stderr.String())
	}
	answer := func(model string) decision.Response {
		return decision.Response{Provider: "typesafe", Model: model, Answers: map[string]decision.Answer{
			"verb": {ID: "verb", Kind: decision.KindChoice, Choice: "find_symbol", Confidence: 0.95},
		}}
	}
	rec.Record("route", answer("jev-1.13.0"), func(decision.Answer) bool { return false })
	rec.Record("route", answer("jev-1.14.0"), func(decision.Answer) bool { return false })

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("decisions.jsonl has %d rows, want 3:\n%s", len(lines), data)
	}
	for i, want := range []bool{false, true} {
		var row struct {
			Model            string `json:"model"`
			RecalibrationDue bool   `json:"recalibration_due"`
		}
		if err := json.Unmarshal([]byte(lines[i+1]), &row); err != nil {
			t.Fatal(err)
		}
		if row.RecalibrationDue != want {
			t.Errorf("row at model %s: recalibration_due = %v, want %v", row.Model, row.RecalibrationDue, want)
		}
	}
}
