package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
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
	// A level is what a reading measured, and a regrade moves it. The level
	// table has its own drift test (TestLevelsTableMatchesReadings), so the
	// golden pins the field and not the state of a reading.
	checkGolden(t, "extract.json", levelValue.ReplaceAll(out.Bytes(), []byte(`"level": "<level>"`)))

	// The supervised child is the default, and a batch that faults nowhere prints the same bytes in process.
	var inProcess bytes.Buffer
	inArgs := append([]string{"extract", "-in-process"}, args[1:]...)
	if code := run(context.Background(), inArgs, strings.NewReader(""), &inProcess, &errw); code != 0 {
		t.Fatalf("extract -in-process exit %d: %s", code, errw.String())
	}
	if !bytes.Equal(out.Bytes(), inProcess.Bytes()) {
		t.Errorf("supervised and -in-process output differ\n--- supervised\n%s\n--- in process\n%s", out.Bytes(), inProcess.Bytes())
	}

	out.Reset()
	if code := run(context.Background(), []string{"dialects"}, strings.NewReader(""), &out, &errw); code != 0 {
		t.Fatalf("dialects exit %d: %s", code, errw.String())
	}
	checkGolden(t, "dialects.json", shapeOfDialects(t, out.Bytes()))
}

// LL-35: the JSON of resources and contract is compatibility surface too. The reads run in the supervised child by
// default, and a batch that faults nowhere prints the same bytes in process. A value the source marks secret is in no
// golden, with or without -values.
func TestResourcesJSONGolden(t *testing.T) {
	src := filepath.Join("testdata", "golden", "src")
	for _, c := range []struct {
		name     string
		args     []string
		wantExit int
	}{
		// a.go has no resource reader: its entry has an error and the run exits 1, like an unreadable file in extract.
		{"resources.json", []string{"resources", "-root", src, "main.tf", "cfn.yaml", "Pulumi.yaml", "a.go"}, 1},
		{"resources-values.json", []string{"resources", "-values", "-root", src, "-module", "mod", "cfn.yaml", "Pulumi.yaml"}, 0},
	} {
		var out, errw bytes.Buffer
		if code := run(context.Background(), c.args, strings.NewReader(""), &out, &errw); code != c.wantExit {
			t.Fatalf("%v: exit %d, want %d: %s", c.args, code, c.wantExit, errw.String())
		}
		checkGolden(t, c.name, out.Bytes())
		for _, secret := range []string{"golden-noecho-sentinel", "golden-sensitive-sentinel"} {
			if strings.Contains(out.String(), secret) {
				t.Errorf("%s holds %q, which the source marks secret", c.name, secret)
			}
		}
		var inProcess bytes.Buffer
		inArgs := append([]string{"resources", "-in-process"}, c.args[1:]...)
		if code := run(context.Background(), inArgs, strings.NewReader(""), &inProcess, &errw); code != c.wantExit {
			t.Fatalf("%v: exit %d, want %d: %s", inArgs, code, c.wantExit, errw.String())
		}
		if !bytes.Equal(out.Bytes(), inProcess.Bytes()) {
			t.Errorf("%s: supervised and -in-process output differ\n--- supervised\n%s\n--- in process\n%s", c.name, out.Bytes(), inProcess.Bytes())
		}
	}
}

func TestContractJSONGolden(t *testing.T) {
	cat := filepath.Join("..", "..", "pkg", "lang", "iac", "testdata", "registry.opentofu.org_example_widget.v1.iaccat.zip")
	for _, c := range []struct {
		name string
		args []string
	}{
		{"contract-catalog.json", []string{"contract", "--catalog", cat}},
		{"contract-type.json", []string{"contract", "--catalog", cat, "widget_thing"}},
		{"contract-attr.json", []string{"contract", "--catalog", cat, "-version", "1.0.0", "widget_thing", "rule"}},
	} {
		var out, errw bytes.Buffer
		if code := run(context.Background(), c.args, strings.NewReader(""), &out, &errw); code != 0 {
			t.Fatalf("%v: exit %d: %s", c.args, code, errw.String())
		}
		checkGolden(t, c.name, out.Bytes())
	}
}

// levelValue matches a measured level in the extract output: L1 to L4 or none.
// "unmeasured" is not a reading and stays literal.
var levelValue = regexp.MustCompile(`"level": "(L[1-4]|none)"`)

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
		SchemaVersion int     `json:"schema_version"`
		Dialects      []entry `json:"dialects"`
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
		SchemaVersion int     `json:"schema_version"`
		Dialects      []entry `json:"dialects"`
	}{doc.SchemaVersion, out}); err != nil {
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
