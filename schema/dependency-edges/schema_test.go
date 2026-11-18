package dependencyedges

import (
	"os"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func TestDependencyEdgeSchema(t *testing.T) {
	ctx := cuecontext.New()
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	schema := read("schema.cue")
	valid := read("testdata/valid.cue")
	if err := ctx.CompileString(schema + "\n" + strings.TrimPrefix(valid, "package dependencyedges\n")).Validate(cue.Concrete(true)); err != nil {
		t.Fatal(err)
	}
	definitions := ctx.CompileString(schema)
	invalid := read("testdata/invalid.cue")
	for _, line := range strings.Split(invalid, "\n") {
		for _, example := range []struct{ prefix, definition string }{
			{"// case: ", "#Batch"},
			{"// export: ", "#DeclaredExport"},
		} {
			bad, ok := strings.CutPrefix(line, example.prefix)
			if !ok {
				continue
			}
			value := ctx.CompileString(bad)
			if err := value.Err(); err != nil {
				t.Fatalf("invalid fixture syntax: %v", err)
			}
			def := definitions.LookupPath(cue.ParsePath(example.definition))
			if def.Unify(value).Validate(cue.Concrete(true)) == nil {
				t.Errorf("schema accepted %s", bad)
			}
		}
	}
}
