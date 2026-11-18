package dependencyboundaries

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps/boundaries"
)

func TestDependencyBoundarySchema(t *testing.T) {
	ctx := cuecontext.New()
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	schema := read("schema.cue")
	def := ctx.CompileString(schema).LookupPath(cue.ParsePath("#Rules"))
	if err := def.Err(); err != nil {
		t.Fatal(err)
	}
	valid := strings.TrimPrefix(read("testdata/valid.cue"), "package dependencyboundaries\n")
	sample := ctx.CompileString(schema + "\n" + valid).LookupPath(cue.ParsePath("sample"))
	sampleJSON, err := sample.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{sampleJSON, []byte(read("../../.fathom/rules/dependency-boundaries.json"))} {
		if err := def.Unify(ctx.CompileBytes(data)).Validate(cue.Concrete(true)); err != nil {
			t.Fatal(err)
		}
		if _, err := boundaries.ParseRules(bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
	}
	for _, line := range strings.Split(read("testdata/invalid.cue"), "\n") {
		bad, ok := strings.CutPrefix(line, "// case: ")
		if !ok {
			continue
		}
		value := ctx.CompileString(bad)
		if err := value.Err(); err != nil {
			t.Fatalf("invalid fixture syntax: %v", err)
		}
		if def.Unify(value).Validate(cue.Concrete(true)) == nil {
			t.Errorf("schema accepted %s", bad)
		}
		if _, err := boundaries.ParseRules(strings.NewReader(bad)); err == nil {
			t.Errorf("runtime accepted %s", bad)
		}
	}
}
