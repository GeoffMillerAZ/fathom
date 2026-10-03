package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
)

func TestBareName(t *testing.T) {
	for in, want := range map[string]string{
		"timoni/values":                    "values",
		"timoni/object/#Deployment":        "Deployment",
		"timoni/bundle/instance/shared.db": "db",
		"timoni/runtime/cluster/staging":   "staging",
		"values":                           "values",
	} {
		if got := bareName(in); got != want {
			t.Errorf("bareName(%q) = %q, want %q", in, got, want)
		}
	}
}

// The oracle's walk on the extractor's fixture: the entities and references the
// dialect's contract names, derived here without the dialect's code.
func TestWalkOnTheDialectFixture(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "internal", "extractor", "dialects", "timoni", "testdata", "app", "templates")
	src, err := os.ReadFile(filepath.Join(dir, "deployment.cue"))
	if err != nil {
		t.Fatal(err)
	}
	f, _ := parser.ParseFile("deployment.cue", src)
	var objects []string
	var refs [][2]string
	for _, d := range f.Decls {
		fd, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		name := labelOf(fd)
		if k := literalOf(fd.Value, "kind"); k != "" {
			objects = append(objects, name+"="+k)
		}
		if takesConfig(fd.Value) {
			refs = append(refs, [2]string{name, "#Config"})
		}
	}
	if want := []string{"#Deployment=Deployment"}; !reflect.DeepEqual(objects, want) {
		t.Errorf("objects = %v, want %v", objects, want)
	}
	if want := [][2]string{{"#Deployment", "#Config"}}; !reflect.DeepEqual(refs, want) {
		t.Errorf("config consumers = %v, want %v", refs, want)
	}

	var got []string
	fields(f, "deployment.cue", importIdents(f), func(name string, fd *ast.Field, rs []string) {
		for _, r := range rs {
			got = append(got, name+"->"+r)
		}
	})
	sort.Strings(got)
	// the import selector `appsv1.#Deployment` is a reference of #Deployment, and the
	// field `#config: #Config` sits in a struct operand of the conjunction, which the CUE
	// extraction's field walk does not enter
	if want := []string{"#Deployment->#Deployment"}; !reflect.DeepEqual(got, want) {
		t.Errorf("field references = %v, want %v", got, want)
	}
}

func TestInstanceRefsScan(t *testing.T) {
	src := []byte("package x\n\n#Instance: {\n\tconfig: #Config\n\tobjects: {a: #A & {#config: config}}\n}\n")
	f, _ := parser.ParseFile("x.cue", src)
	var got []string
	fields(f, "x.cue", importIdents(f), func(name string, fd *ast.Field, rs []string) {
		for _, r := range rs {
			got = append(got, name+"->"+r)
		}
	})
	sort.Strings(got)
	want := []string{"#Instance.config->#Config", "#Instance.objects.a->#A"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("refs = %v, want %v", got, want)
	}
}
