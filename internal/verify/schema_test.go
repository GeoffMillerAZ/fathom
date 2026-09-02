package verify

// schema_test.go — asserts the schema/plan CUE negatives (repo convention:
// testdata/invalid.cue documents them commented-out so live `cue vet` stays
// green; this test proves each actually fails) and that the valid fixture,
// which includes grammar-v2 fields, still unifies.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func planSchemaSource(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "plan", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func TestPlanSchema_Negatives(t *testing.T) {
	schema := planSchemaSource(t, "schema.cue")
	cases := []struct {
		name string
		bad  string
	}{
		{"state outside the v2 vocabulary", `
bad: #Task & {id: "T-1", state: "half-done"}`},
		{"disposition outside refused|deferred", `
bad: #Task & {id: "T-2", state: "skipped", disposition: "sideways"}`},
		{"min_tier outside the tier vocabulary", `
bad: #Task & {id: "T-3", expect: {symbol: "X", min_tier: "vibes"}}`},
		{"plan with a non-string service", `
bad: #Plan & {service: 42, tasks: []}`},
		{"oracle outside the ADR-030 vocabulary", `
bad: #Task & {id: "T-5", expect: {oracle: "vibes", path: "docs/x.md", pattern: "y"}}`},
	}
	cctx := cuecontext.New()

	// Sanity: the schema validates on its own, and the valid fixture (which
	// exercises the grammar-v2 fields) unifies with it.
	if v := cctx.CompileString(schema); v.Err() != nil || v.Validate() != nil {
		t.Fatalf("schema does not validate on its own: %v / %v", v.Err(), v.Validate())
	}
	valid := planSchemaSource(t, filepath.Join("testdata", "valid.cue"))
	if v := cctx.CompileString(schema + "\n" + stripPackageLine(valid)); v.Err() != nil || v.Validate() != nil {
		t.Fatalf("valid.cue does not unify with the schema: %v / %v", v.Err(), v.Validate())
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := cctx.CompileString(schema + "\n" + tc.bad)
			if v.Err() != nil {
				return // compile-time rejection is a pass
			}
			if err := v.Validate(); err == nil {
				t.Errorf("expected validation failure, got none")
			}
		})
	}
}

// stripPackageLine removes the "package plan" clause so a fixture can be
// concatenated onto the schema source for single-string compilation.
func stripPackageLine(src string) string {
	lines := []byte(src)
	const decl = "package plan\n"
	if i := indexOf(lines, decl); i >= 0 {
		return string(lines[:i]) + string(lines[i+len(decl):])
	}
	return src
}

func indexOf(b []byte, sub string) int {
	for i := 0; i+len(sub) <= len(b); i++ {
		if string(b[i:i+len(sub)]) == sub {
			return i
		}
	}
	return -1
}
