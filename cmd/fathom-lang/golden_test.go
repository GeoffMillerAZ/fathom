package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite the JSON goldens under testdata/golden")

// LL-27 (design section 8, row 8): fathom-lang's JSON is compatibility
// surface for tools not written in Go, so a change to it must be a reviewed
// edit of a golden file. After an intended change run:
//
//	go test ./cmd/fathom-lang -run TestFathomLangJSONGolden -args -update-golden
//
// and read the diff of testdata/golden as the review. Fields are added, never
// renamed (docs/intent/scope.md).
func TestFathomLangJSONGolden(t *testing.T) {
	var out, errw bytes.Buffer
	args := []string{"extract", "-root", filepath.Join("testdata", "golden", "src"), "main.tf", "a.go", "blob.bin", "misc.yaml"}
	if code := run(context.Background(), args, strings.NewReader(""), &out, &errw); code != 0 {
		t.Fatalf("extract exit %d: %s", code, errw.String())
	}
	checkGolden(t, "extract.json", out.Bytes())

	out.Reset()
	if code := run(context.Background(), []string{"dialects"}, strings.NewReader(""), &out, &errw); code != 0 {
		t.Fatalf("dialects exit %d: %s", code, errw.String())
	}
	checkGolden(t, "dialects.json", shapeOfDialects(t, out.Bytes()))
}

// shapeOfDialects keeps three stable entries of the dialects output and
// replaces the values that move as languages improve (matches, level, grade)
// with their JSON type, so the golden pins the field names, their types and
// the identity fields, and not the state of a reading.
func shapeOfDialects(t *testing.T, raw []byte) []byte {
	t.Helper()
	type entry struct {
		ID      string `json:"id"`
		Kind    string `json:"kind"`
		Engine  string `json:"engine"`
		Matches any    `json:"matches"`
		Level   any    `json:"level"`
		Grade   any    `json:"grade"`
	}
	var doc struct {
		Dialects []entry `json:"dialects"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("dialects output has a field this golden does not know (add it, deliberately): %v", err)
	}
	keep := map[string]bool{"go": true, "terraform": true, "dockerfile": true}
	out := []entry{}
	for _, d := range doc.Dialects {
		if !keep[d.ID] {
			continue
		}
		d.Matches, d.Level, d.Grade = jsonType(d.Matches), jsonType(d.Level), jsonType(d.Grade)
		out = append(out, d)
	}
	if len(out) != len(keep) {
		t.Fatalf("expected %d pinned dialects, found %d", len(keep), len(out))
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(struct {
		Dialects []entry `json:"dialects"`
	}{out}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// jsonType names the JSON type of a decoded value.
func jsonType(v any) string {
	switch v.(type) {
	case []any:
		return "<array>"
	case string:
		return "<string>"
	case nil:
		return "<null>"
	case float64:
		return "<number>"
	case bool:
		return "<bool>"
	}
	return "<object>"
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -args -update-golden to write it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("fathom-lang's JSON changed against %s. It is compatibility surface: review the change, then update the golden with -args -update-golden.\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}
