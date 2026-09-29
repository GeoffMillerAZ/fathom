// schema_test.go — asserts the schema/binding CUE negatives documented,
// commented out, in schema/binding/testdata/invalid.cue (repo convention,
// schema/AGENTS.md: "negatives are asserted by Go tests, not
// live-vetted").
package bindspec

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func schemaSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "binding", "schema.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(data)
}

func validSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "binding", "testdata", "valid.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read valid.cue: %v", err)
	}
	return strings.Replace(string(data), "package binding\n", "", 1)
}

func TestBindingSchema_ValidInstancesConcrete(t *testing.T) {
	cctx := cuecontext.New()
	v := cctx.CompileString(schemaSource(t) + "\n" + validSource(t))
	if v.Err() != nil {
		t.Fatalf("compile: %v", v.Err())
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("valid.cue instances should validate concretely: %v", err)
	}
}

func TestBindingSchema_Negatives(t *testing.T) {
	schema := schemaSource(t)
	cases := []struct {
		name          string
		bad           string
		wantErrSubstr string
	}{
		{"bad module name shape", `
bad2: #Bindings & {
	bindings: [{module: "Not_Valid"}]
}`, "module"},
		{"max_rules must be positive", `
bad3: #Bindings & {
	bindings: [{module: "platform-standards", max_rules: 0}]
}`, "max_rules"},
		{"min_tier must be a recognised tier", `
bad4: #Bindings & {
	bindings: [{module: "platform-standards", min_tier: "gold"}]
}`, "min_tier"},
		{"empty scopes list", `
bad5: #Bindings & {
	bindings: [{module: "platform-standards", scopes: []}]
}`, "scopes"},
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
