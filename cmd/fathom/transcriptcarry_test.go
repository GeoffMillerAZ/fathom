package main

// transcriptcarry_test.go — every field the transcript parser computes must
// reach the ledger row.
//
// transcriptSummaryFrom used to copy three field pairs out of
// summarizeTranscriptBytes by hand. Every field added afterwards was
// computed and then silently dropped. Measured on the live ledger
// 2026-08-12, three OFF-arm sessions into a 20-session run:
//
//   FirstTS          -> arm_coverage read "unknown", so NOT ONE OFF-arm
//                       session was admissible; the whole run was
//                       collecting nothing
//   TokensIn/Out     -> every row counted as "pre-split", so neither arm
//                       could ever have a spend-shaped median
//   FirstEditTurn    -> "no sessions carry first_edit_turn yet"
//   exploration      -> "no session carries the exploration ledger yet"
//
// All four were reported as "rows predate the instrument". The instruments
// were wired. Their output was being thrown away three lines later.
//
// This test asserts on the STRUCT, not on a hand-listed set of fields, so
// it cannot drift out of date the same way the copy did.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// richTranscript writes a transcript exercising every instrument.
func richTranscript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "transcript.jsonl")
	base := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	line := func(i int, body map[string]any) string {
		body["timestamp"] = base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339)
		b, _ := json.Marshal(body)
		return string(b)
	}
	rows := []string{
		line(0, map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "start"}}),
		line(1, map[string]any{"type": "assistant", "message": map[string]any{
			"role": "assistant", "usage": map[string]any{
				"input_tokens": 100, "output_tokens": 50,
				"cache_read_input_tokens": 900, "cache_creation_input_tokens": 30,
			},
			"content": []any{map[string]any{"type": "tool_use", "name": "Grep", "input": map[string]any{"pattern": "x"}}},
		}}),
		line(2, map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "content": "some result bytes"},
		}}}),
		line(3, map[string]any{"type": "assistant", "message": map[string]any{
			"role": "assistant", "usage": map[string]any{"input_tokens": 20, "output_tokens": 10},
			"content": []any{map[string]any{"type": "tool_use", "name": "Edit", "input": map[string]any{"file_path": "a.go"}}},
		}}),
	}
	body := ""
	for _, r := range rows {
		body += r + "\n"
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestTranscriptSummaryCarriesEveryField is the structural guard: whatever
// the parser produces for a given input, the reader must return the same
// value for every field. A future field is covered automatically.
func TestTranscriptSummaryCarriesEveryField(t *testing.T) {
	path := richTranscript(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want := summarizeTranscriptBytes(raw)
	got := transcriptSummaryFrom(path)

	// Truncated is the ONE field the reader legitimately owns: the parser
	// cannot know the caller capped the bytes.
	want.Truncated = got.Truncated

	wv, gv := reflect.ValueOf(want), reflect.ValueOf(got)
	tp := wv.Type()
	for i := range tp.NumField() {
		name := tp.Field(i).Name
		if !reflect.DeepEqual(wv.Field(i).Interface(), gv.Field(i).Interface()) {
			t.Errorf("field %s is dropped between the parser and the reader:\n  parser: %v\n  reader: %v\n"+
				"Every field must be carried by construction — hand-copying is what silently disabled "+
				"four instruments for weeks.", name, deref(wv.Field(i)), deref(gv.Field(i)))
		}
	}
}

// TestTranscriptSummaryPopulatesTheInstruments pins the specific four that
// were dead, so a regression names them rather than showing a struct diff.
func TestTranscriptSummaryPopulatesTheInstruments(t *testing.T) {
	got := transcriptSummaryFrom(richTranscript(t))

	if got.FirstTS == nil {
		t.Error("FirstTS is nil — arm_coverage then reads \"unknown\" and NO OFF-arm session is ever admissible")
	}
	if got.TokensIn == nil || got.TokensOut == nil {
		t.Errorf("the in/out split is absent (in=%v out=%v) — every row counts as pre-split and no arm gets a spend-shaped median",
			got.TokensIn, got.TokensOut)
	}
	if got.TokensCacheRead == nil {
		t.Error("cache-read tokens absent — the 596M-median inflation cannot be disclosed without them")
	}
	if got.FirstEditTurn == nil {
		t.Error("FirstEditTurn is nil — the warm-up instrument (IQ-2) reports \"rows predate the instrument\" forever")
	}
	if got.RawDiscoveryCalls == 0 && got.FathomCalls == 0 {
		t.Error("the exploration ledger is empty — per-question cost (EX-1) never fills in")
	}
	if got.TokensTotal == nil || got.DurationMS == nil || got.TurnCount == nil {
		t.Error("the three originally-copied fields must still arrive")
	}
}

func deref(v reflect.Value) any {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "<nil>"
		}
		return v.Elem().Interface()
	}
	return v.Interface()
}
