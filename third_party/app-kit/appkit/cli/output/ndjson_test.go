package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/cli/output"
)

// TestNDJSONRowShape asserts one JSON object per line, with key order
// id, status, detail, remedy (marshaled via a struct so field order is
// load-bearing, not merely accidental map-iteration order).
func TestNDJSONRowShape(t *testing.T) {
	rows := []output.StatusRow{
		{ID: "git", Status: "ok", Detail: "found at /usr/bin/git", Remedy: ""},
		{ID: "nix-daemon", Status: "fail", Detail: "unreachable", Remedy: "restart it"},
	}
	var buf bytes.Buffer
	if err := output.WriteNDJSON(&buf, rows); err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != len(rows) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(rows), buf.String())
	}

	wantKeyOrder := []string{"id", "status", "detail", "remedy"}
	for i, line := range lines {
		gotOrder := keyOrder(t, line)
		if len(gotOrder) != len(wantKeyOrder) {
			t.Fatalf("line %d: got %d keys %v, want %v", i, len(gotOrder), gotOrder, wantKeyOrder)
		}
		for j, k := range wantKeyOrder {
			if gotOrder[j] != k {
				t.Errorf("line %d: key %d = %q, want %q (order %v)", i, j, gotOrder[j], k, gotOrder)
			}
		}

		var decoded map[string]string
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("line %d: not valid JSON: %v", i, err)
		}
		if decoded["id"] != rows[i].ID || decoded["status"] != rows[i].Status ||
			decoded["detail"] != rows[i].Detail || decoded["remedy"] != rows[i].Remedy {
			t.Errorf("line %d: decoded %v does not match row %+v", i, decoded, rows[i])
		}
	}
}

// keyOrder extracts a JSON object's top-level key order by walking the raw
// text (encoding/json's map decode loses order; this test needs to see it).
func keyOrder(t *testing.T, line string) []string {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(line))
	tok, err := dec.Token() // '{'
	if err != nil || tok != json.Delim('{') {
		t.Fatalf("expected object start, got %v (%v)", tok, err)
	}
	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			t.Fatalf("reading key: %v", err)
		}
		keys = append(keys, keyTok.(string))
		if _, err := dec.Token(); err != nil { // value
			t.Fatalf("reading value: %v", err)
		}
	}
	return keys
}
