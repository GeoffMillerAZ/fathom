package readout

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func schemaDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "schema", "readouts")
}

// TestReadoutsSchemaMatchesSource holds the embedded schema byte-identical
// to schema/readouts/schema.cue, the one authoritative source.
func TestReadoutsSchemaMatchesSource(t *testing.T) {
	want, err := os.ReadFile(filepath.Join(schemaDir(t), "schema.cue"))
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(readoutsSchema) {
		t.Fatal("internal/readout/readouts_schema.cue has drifted from schema/readouts/schema.cue: copy the source verbatim")
	}
}

// TestReadoutsSchemaNegatives asserts each commented case in
// schema/readouts/testdata/invalid.cue fails, and valid.cue passes.
func TestReadoutsSchemaNegatives(t *testing.T) {
	dir := schemaDir(t)
	schema, err := os.ReadFile(filepath.Join(dir, "schema.cue"))
	if err != nil {
		t.Fatal(err)
	}
	valid, err := os.ReadFile(filepath.Join(dir, "testdata", "valid.cue"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := cuecontext.New()
	body := strings.TrimPrefix(string(valid), "package readouts\n")
	if v := ctx.CompileString(string(schema) + "\n" + body); v.Validate() != nil {
		t.Fatalf("valid.cue fails the schema: %v", v.Validate())
	}
	invalid, err := os.ReadFile(filepath.Join(dir, "testdata", "invalid.cue"))
	if err != nil {
		t.Fatal(err)
	}
	cases := regexp.MustCompile(`(?m)^// (bad\d+: .*)$`).FindAllStringSubmatch(string(invalid), -1)
	if len(cases) < 8 {
		t.Fatalf("invalid.cue carries %d cases, want at least 8", len(cases))
	}
	for _, c := range cases {
		v := ctx.CompileString(string(schema) + "\n" + c[1])
		if v.Err() == nil && v.Validate() == nil {
			t.Errorf("negative case passed the schema: %s", c[1])
		}
	}
}
