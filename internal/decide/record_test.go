package decide_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// TestRecord_UnsureIsNotApplied pins DP-3/I42 (docs/plans/decision-port.md,
// docs/vision/32-decision-port.md): a decision in the unsure band
// (Confidence < decision.UnsureBelow) is recorded applied:false no matter
// what the caller's applied predicate returns; a confident one is recorded
// per the predicate; a nil Recorder is a no-op; every row carries
// schema_version 2 and rows come out sorted by question id.
func TestRecord_UnsureIsNotApplied(t *testing.T) {
	fixedNow := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	resp := decision.Response{
		Provider: "llm",
		Model:    "test-model",
		Answers: map[string]decision.Answer{
			"b-unsure": {Kind: decision.KindPredicate, Noul: 0.4, Confidence: 0.4},
			"a-sure":   {Kind: decision.KindPredicate, Noul: 0.95, Confidence: 0.95},
		},
		Latency: 12 * time.Millisecond,
		Usage:   decision.Usage{InputTokens: 10, OutputTokens: 5},
	}

	var buf bytes.Buffer
	rec := decide.NewRecorder(&buf, func() time.Time { return fixedNow }, "v-test")

	calledFor := map[string]bool{}
	applied := func(a decision.Answer) bool {
		// Answer does not carry its own question ID, so identify the call
		// by its Noul value (unique per fixture answer above).
		if a.Noul == 0.95 {
			calledFor["a-sure"] = true
		}
		if a.Noul == 0.4 {
			calledFor["b-unsure"] = true
		}
		return true
	}

	rec.Record("test-site", resp, applied)

	if calledFor["b-unsure"] {
		t.Error("applied predicate must not be consulted for an unsure-band answer (I42)")
	}
	if !calledFor["a-sure"] {
		t.Error("applied predicate should be consulted for a confident answer")
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("wrote %d line(s), want 2:\n%s", len(lines), buf.String())
	}

	var rowA, rowB map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rowA); err != nil {
		t.Fatalf("row 0 not JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &rowB); err != nil {
		t.Fatalf("row 1 not JSON: %v", err)
	}

	// Sorted by question id: "a-sure" < "b-unsure".
	if rowA["question_id"] != "a-sure" {
		t.Errorf("row 0 question_id = %v, want %q (rows must be sorted)", rowA["question_id"], "a-sure")
	}
	if rowB["question_id"] != "b-unsure" {
		t.Errorf("row 1 question_id = %v, want %q", rowB["question_id"], "b-unsure")
	}

	if applied, ok := rowA["applied"].(bool); !ok || !applied {
		t.Errorf("a-sure applied = %v, want true", rowA["applied"])
	}
	if applied, ok := rowB["applied"].(bool); !ok || applied {
		t.Errorf("b-unsure applied = %v, want false (unsure band, I42)", rowB["applied"])
	}

	for _, row := range []map[string]any{rowA, rowB} {
		if v, ok := row["schema_version"].(float64); !ok || v != 2 {
			t.Errorf("schema_version = %v, want 2", row["schema_version"])
		}
		if row["fathom_version"] != "v-test" {
			t.Errorf("fathom_version = %v, want %q", row["fathom_version"], "v-test")
		}
		if row["site"] != "test-site" {
			t.Errorf("site = %v, want %q", row["site"], "test-site")
		}
		if row["ts"] != fixedNow.Format(time.RFC3339) {
			t.Errorf("ts = %v, want %q", row["ts"], fixedNow.Format(time.RFC3339))
		}
	}

	// A nil Recorder is a no-op — must not panic.
	var nilRec *decide.Recorder
	nilRec.Record("test-site", resp, applied)
}

// TestRecord_NilRecorderIsNoop pins the "nil Recorder is a no-op" contract
// as its own named test (DP-3's acceptance list), independent of the shared
// fixture above.
func TestRecord_NilRecorderIsNoop(t *testing.T) {
	var rec *decide.Recorder
	resp := decision.Response{
		Provider: "none",
		Answers: map[string]decision.Answer{
			"only": {Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9},
		},
	}
	rec.Record("site", resp, func(decision.Answer) bool { return true })
}

// TestRecorder_Append pins Append's contract: same row shape as Record, a
// nil Recorder / nil writer / empty records is a no-op, and — since
// decision.Record carries no Applied flag — applied is derived from band
// alone (I42: unsure is never applied; every other band is).
func TestRecorder_Append(t *testing.T) {
	fixedNow := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	records := []decision.Record{
		{
			Site: "compile_prefilter", Provider: "llm", Model: "test-model",
			QuestionID: "sec-2", Kind: decision.KindPredicate,
			Noul: 0.4, Confidence: 0.4, // unsure band
		},
		{
			Site: "compile_prefilter", Provider: "llm", Model: "test-model",
			QuestionID: "sec-1", Kind: decision.KindPredicate,
			Noul: 0.95, Confidence: 0.95, // proceed band
		},
	}

	var buf bytes.Buffer
	rec := decide.NewRecorder(&buf, func() time.Time { return fixedNow }, "v-test")
	rec.Append(records)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("wrote %d line(s), want 2:\n%s", len(lines), buf.String())
	}

	byQID := map[string]map[string]any{}
	for _, line := range lines {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("row not JSON: %v", err)
		}
		byQID[row["question_id"].(string)] = row
	}

	if applied, ok := byQID["sec-2"]["applied"].(bool); !ok || applied {
		t.Errorf("sec-2 (unsure band) applied = %v, want false", byQID["sec-2"]["applied"])
	}
	if applied, ok := byQID["sec-1"]["applied"].(bool); !ok || !applied {
		t.Errorf("sec-1 (proceed band) applied = %v, want true", byQID["sec-1"]["applied"])
	}
	if byQID["sec-1"]["site"] != "compile_prefilter" {
		t.Errorf("site = %v, want %q", byQID["sec-1"]["site"], "compile_prefilter")
	}

	// nil Recorder, nil writer, and empty records are all safe no-ops.
	var nilRec *decide.Recorder
	nilRec.Append(records)

	zeroWriterRec := decide.NewRecorder(nil, func() time.Time { return fixedNow }, "v-test")
	zeroWriterRec.Append(records)

	var buf2 bytes.Buffer
	emptyRec := decide.NewRecorder(&buf2, func() time.Time { return fixedNow }, "v-test")
	emptyRec.Append(nil)
	if buf2.Len() != 0 {
		t.Errorf("Append(nil) wrote %d bytes, want 0", buf2.Len())
	}
}

// TestRecordError pins DP-22/O109's fix: a hard-failed Decide call (no
// decision.Response at all) still leaves ONE JSONL row — schema_version,
// site, provider, model, questions, latency_ms, error, applied:false — so
// silence in decisions.jsonl means "never asked", not "asked and failed".
// A nil Recorder, a nil writer, and a nil err are all safe no-ops.
func TestRecordError(t *testing.T) {
	fixedNow := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	boom := errors.New("decision: llm response is not JSON: unexpected end of JSON input")

	var buf bytes.Buffer
	rec := decide.NewRecorder(&buf, func() time.Time { return fixedNow }, "v-test")

	rec.RecordError("obligations", "llm", "gpt-6-luna", 20, 18370*time.Millisecond, boom)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("wrote %d line(s), want 1:\n%s", len(lines), buf.String())
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatalf("row not JSON: %v", err)
	}

	if v, ok := row["schema_version"].(float64); !ok || v != 2 {
		t.Errorf("schema_version = %v, want 2", row["schema_version"])
	}
	if row["fathom_version"] != "v-test" {
		t.Errorf("fathom_version = %v, want %q", row["fathom_version"], "v-test")
	}
	if row["ts"] != fixedNow.Format(time.RFC3339) {
		t.Errorf("ts = %v, want %q", row["ts"], fixedNow.Format(time.RFC3339))
	}
	if row["site"] != "obligations" {
		t.Errorf("site = %v, want %q", row["site"], "obligations")
	}
	if row["provider"] != "llm" {
		t.Errorf("provider = %v, want %q", row["provider"], "llm")
	}
	if row["model"] != "gpt-6-luna" {
		t.Errorf("model = %v, want %q", row["model"], "gpt-6-luna")
	}
	if v, ok := row["questions"].(float64); !ok || v != 20 {
		t.Errorf("questions = %v, want 20", row["questions"])
	}
	if v, ok := row["latency_ms"].(float64); !ok || v != 18370 {
		t.Errorf("latency_ms = %v, want 18370", row["latency_ms"])
	}
	if row["error"] != boom.Error() {
		t.Errorf("error = %v, want %q", row["error"], boom.Error())
	}
	if applied, ok := row["applied"].(bool); !ok || applied {
		t.Errorf("applied = %v, want false", row["applied"])
	}
	if _, ok := row["question_id"]; ok && row["question_id"] != "" {
		t.Errorf("question_id = %v, want empty (no single question answered)", row["question_id"])
	}

	// Safe no-ops.
	var nilRec *decide.Recorder
	nilRec.RecordError("site", "p", "m", 1, time.Millisecond, boom)

	zeroWriterRec := decide.NewRecorder(nil, func() time.Time { return fixedNow }, "v-test")
	zeroWriterRec.RecordError("site", "p", "m", 1, time.Millisecond, boom)

	var buf2 bytes.Buffer
	nilErrRec := decide.NewRecorder(&buf2, func() time.Time { return fixedNow }, "v-test")
	nilErrRec.RecordError("site", "p", "m", 1, time.Millisecond, nil)
	if buf2.Len() != 0 {
		t.Errorf("RecordError(nil err) wrote %d bytes, want 0 (nothing failed)", buf2.Len())
	}
}
