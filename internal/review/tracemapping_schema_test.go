// tracemapping_schema_test.go asserts the schema/trace CUE positives and
// the negatives documented, commented out, in
// schema/trace/testdata/invalid.cue (repo convention, schema/AGENTS.md).
package review

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func traceSchemaFile(t *testing.T, rel ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	parts := append([]string{filepath.Dir(file), "..", "..", "schema", "trace"}, rel...)
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatalf("read %v: %v", rel, err)
	}
	return strings.Replace(string(data), "package trace\n", "", 1)
}

func TestTraceMappingSchema_ValidInstancesConcrete(t *testing.T) {
	v := cuecontext.New().CompileString(traceSchemaFile(t, "schema.cue") + "\n" + traceSchemaFile(t, "testdata", "valid.cue"))
	if v.Err() != nil {
		t.Fatalf("compile: %v", v.Err())
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("valid.cue should validate concretely: %v", err)
	}
	// The valid example also decodes through the Go loader.
	full := v.LookupPath(cue.ParsePath("full"))
	src, err := full.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseStandardMapping("full.json", src); err != nil {
		t.Errorf("ParseStandardMapping(valid full): %v", err)
	}
}

func TestTraceMappingSchema_Negatives(t *testing.T) {
	schema := traceSchemaFile(t, "schema.cue")
	cases := []struct{ name, bad, want string }{
		{"no clauses", `bad1: #StandardMapping & {
	standard: "empty"
	clauses: []
}`, "clauses"},
		{"standard not a slug", `bad2: #StandardMapping & {
	standard: "Has Spaces"
	clauses: [{id: "C-1"}]
}`, "standard"},
		{"empty clause id", `bad3: #StandardMapping & {
	standard: "x"
	clauses: [{id: ""}]
}`, "id"},
	}
	cctx := cuecontext.New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := cctx.CompileString(schema + "\n" + tc.bad)
			var errText string
			if v.Err() != nil {
				errText = v.Err().Error()
			} else if err := v.Validate(cue.Concrete(true)); err != nil {
				errText = err.Error()
			} else {
				t.Fatal("expected a validation failure, got none")
			}
			if !strings.Contains(errText, tc.want) {
				t.Errorf("error %q does not mention %q", errText, tc.want)
			}
		})
	}
}
