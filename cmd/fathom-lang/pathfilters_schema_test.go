package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func TestPathFiltersCoverageMapSchema(t *testing.T) {
	dir := filepath.Join("..", "..", "schema", "dependency-pathfilters")
	read := func(name string) string {
		t.Helper()
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(src)
	}
	ctx := cuecontext.New()
	schema := read("schema.cue")
	def := ctx.CompileString(schema).LookupPath(cue.ParsePath("#CoverageMap"))
	if err := def.Err(); err != nil {
		t.Fatal(err)
	}
	valid := strings.TrimPrefix(read("testdata/valid.cue"), "package dependencypathfilters\n")
	sample := ctx.CompileString(schema + "\n" + valid).LookupPath(cue.ParsePath("sample"))
	wire, err := sample.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{wire, []byte(pathFilterTestMap)} {
		if err := def.Unify(ctx.CompileBytes(data)).Validate(cue.Concrete(true)); err != nil {
			t.Fatal(err)
		}
		if _, err := parsePathFilterMap(data); err != nil {
			t.Fatal(err)
		}
	}
	cases := 0
	for _, line := range strings.Split(read("testdata/invalid.cue"), "\n") {
		raw, ok := strings.CutPrefix(line, "// case: ")
		if !ok {
			continue
		}
		cases++
		value := ctx.CompileString(raw)
		if err := value.Err(); err != nil {
			t.Fatalf("invalid fixture syntax: %v", err)
		}
		if def.Unify(value).Validate(cue.Concrete(true)) == nil {
			t.Errorf("schema accepted %s", raw)
		}
		if _, err := parsePathFilterMap([]byte(raw)); err == nil {
			t.Errorf("runtime accepted %s", raw)
		}
	}
	if cases == 0 {
		t.Fatal("no rejected schema fixture cases")
	}
}
