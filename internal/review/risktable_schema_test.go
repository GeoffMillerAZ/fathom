// risktable_schema_test.go asserts the schema/risk CUE negatives
// documented, commented out, in schema/risk/testdata/invalid.cue (repo
// convention, schema/CLAUDE.md; mirrors internal/testlayer/schema_test.go).
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

func riskSchemaSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "risk", "schema.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(data)
}

func riskValidSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "risk", "testdata", "valid.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read valid.cue: %v", err)
	}
	return strings.Replace(string(data), "package risk\n", "", 1)
}

func TestRiskSchema_ValidInstancesConcrete(t *testing.T) {
	cctx := cuecontext.New()
	v := cctx.CompileString(riskSchemaSource(t) + "\n" + riskValidSource(t))
	if v.Err() != nil {
		t.Fatalf("compile: %v", v.Err())
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("valid.cue instances should validate concretely: %v", err)
	}
}

func TestRiskSchema_Negatives(t *testing.T) {
	schema := riskSchemaSource(t)
	cases := []struct {
		name          string
		bad           string
		wantErrSubstr string
	}{
		{"zero blast radius threshold", `
bad1: #RiskTable & {
	blast_radius_high_callers: 0
}`, "blast_radius_high_callers"},
		{"negative zone interest threshold", `
bad2: #RiskTable & {
	zone_interest_medium_rate: -1.0
}`, "zone_interest_medium_rate"},
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
