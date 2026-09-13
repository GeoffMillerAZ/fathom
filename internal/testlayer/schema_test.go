// schema_test.go — asserts the schema/testlayer CUE negatives documented,
// commented out, in schema/testlayer/testdata/invalid.cue (repo
// convention, schema/CLAUDE.md; mirrors internal/guardspec/schema_test.go).
package testlayer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func testlayerSchemaSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "testlayer", "schema.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(data)
}

func testlayerValidSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "testlayer", "testdata", "valid.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read valid.cue: %v", err)
	}
	return strings.Replace(string(data), "package testlayer\n", "", 1)
}

func TestTestlayerSchema_ValidInstancesConcrete(t *testing.T) {
	cctx := cuecontext.New()
	v := cctx.CompileString(testlayerSchemaSource(t) + "\n" + testlayerValidSource(t))
	if v.Err() != nil {
		t.Fatalf("compile: %v", v.Err())
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("valid.cue instances should validate concretely: %v", err)
	}
}

func TestTestlayerSchema_Negatives(t *testing.T) {
	schema := testlayerSchemaSource(t)
	cases := []struct {
		name          string
		bad           string
		wantErrSubstr string
	}{
		{"empty name", `
bad1: #Layers & {
	layers: [{name: ""}]
}`, "name"},
		{"empty globs list", `
bad2: #Layers & {
	layers: [{name: "x", globs: []}]
}`, "globs"},
		{"empty build_tags list", `
bad3: #Layers & {
	layers: [{name: "x", build_tags: []}]
}`, "build_tags"},
		{"empty dirs list", `
bad4: #Layers & {
	layers: [{name: "x", dirs: []}]
}`, "dirs"},
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
