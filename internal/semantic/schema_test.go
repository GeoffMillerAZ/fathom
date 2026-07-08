package semantic

// schema_test.go — asserts the schema/semantic CUE negatives (repo
// convention: testdata/invalid.cue documents them commented-out so live
// `cue vet` stays green; this test proves each actually fails).

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func schemaSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "semantic", "schema.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(data)
}

func TestLayerSchema_Negatives(t *testing.T) {
	schema := schemaSource(t)
	cases := []struct {
		name string
		bad  string
	}{
		{"empty corpus include", `
bad: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: []
	prompt_version: "x/v1"
	tier: "doc-claimed"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
}`},
		{"serving cap above payload contract", `
bad: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: ["docs/**"]
	prompt_version: "x/v1"
	tier: "doc-claimed"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
	serving: max_rules: 12
}`},
		{"tier outside the v1 claim model", `
bad: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: ["docs/**"]
	prompt_version: "x/v1"
	tier: "verified"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
}`},
	}
	cctx := cuecontext.New()

	// Sanity: the schema itself (with its obligations instance) validates.
	if v := cctx.CompileString(schema); v.Err() != nil || v.Validate() != nil {
		t.Fatalf("schema does not validate on its own: %v / %v", v.Err(), v.Validate())
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
