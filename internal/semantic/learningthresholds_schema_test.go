// learningthresholds_schema_test.go asserts the schema/learning CUE
// negatives documented, commented out, in
// schema/learning/testdata/invalid.cue (repo convention, schema/AGENTS.md;
// mirrors internal/review/risktable_schema_test.go's precedent for
// schema/risk).
package semantic

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func learningSchemaSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "learning", "schema.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(data)
}

func learningValidSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "learning", "testdata", "valid.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read valid.cue: %v", err)
	}
	return strings.Replace(string(data), "package learning\n", "", 1)
}

func TestLearningSchema_ValidInstancesConcrete(t *testing.T) {
	cctx := cuecontext.New()
	v := cctx.CompileString(learningSchemaSource(t) + "\n" + learningValidSource(t))
	if v.Err() != nil {
		t.Fatalf("compile: %v", v.Err())
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("valid.cue instances should validate concretely: %v", err)
	}
}

func TestLearningSchema_Negatives(t *testing.T) {
	schema := learningSchemaSource(t)
	cases := []struct {
		name          string
		bad           string
		wantErrSubstr string
	}{
		{"zero promotion corroboration threshold", `
bad1: #Learning & {
	promote_corroborations: 0
}`, "promote_corroborations"},
		{"negative retirement refute threshold", `
bad2: #Learning & {
	retire_refutes: -1
}`, "retire_refutes"},
		{"non-int window", `
bad3: #Learning & {
	promote_window_days: 30.5
}`, "promote_window_days"},
	}
	cctx := cuecontext.New()
	if v := cctx.CompileString(schema); v.Err() != nil || v.Validate() != nil {
		t.Fatalf("schema does not validate on its own: %v / %v", v.Err(), v.Validate())
	}
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
			if !strings.Contains(errText, tc.wantErrSubstr) {
				t.Errorf("error %q does not mention %q — this case may be failing for the wrong reason", errText, tc.wantErrSubstr)
			}
		})
	}
}
