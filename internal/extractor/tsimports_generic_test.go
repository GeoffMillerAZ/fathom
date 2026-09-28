package extractor

import (
	"fmt"
	"strings"
	"testing"
)

func importStrings(specs []ImportSpec) string {
	var out []string
	for _, s := range specs {
		kind := map[ImportKind]string{ImportNamed: "named", ImportWhole: "whole", ImportWildcard: "wildcard"}[s.Kind]
		out = append(out, fmt.Sprintf("%d:%s:%s<-%s:%s", s.Line, kind, s.LocalName, s.Module, s.ExportedName))
	}
	return strings.Join(out, " ")
}

// TestDialectImportsGeneric (LB-F7, design §3.7): any dialect's imports.scm
// can use the generic captures — @import.stmt, @import.module, @import.name,
// @import.alias, @import.wildcard — without Go of its own; the Python
// from/rel and the JavaScript forms keep working alongside them.
func TestDialectImportsGeneric(t *testing.T) {
	t.Run("kotlin whole, alias, wildcard", func(t *testing.T) {
		q := `
(import_header (identifier) @import.module
  (import_alias (type_identifier) @import.alias)?
  (wildcard_import)? @import.wildcard) @import.stmt
`
		e := testTSExtractor("kotlin", "tree_sitter_kotlin", q, map[string]tsDefKind{}, nil)
		defer e.Close()
		specs, err := e.ExtractImports("A.kt", []byte("import a.b.C\nimport a.b.D as E\nimport a.c.*\n"))
		if err != nil {
			t.Fatal(err)
		}
		want := "1:whole:a.b.C<-a.b.C: 2:whole:E<-a.b.D: 3:wildcard:*<-a.c:"
		if got := importStrings(specs); got != want {
			t.Errorf("got  %s\nwant %s", got, want)
		}
	})
	t.Run("python named via generic captures", func(t *testing.T) {
		q := `
(import_from_statement
  module_name: (dotted_name) @import.module
  name: [(dotted_name) @import.name
         (aliased_import name: (dotted_name) @import.name alias: (identifier) @import.alias)]) @import.stmt
(import_from_statement
  module_name: (dotted_name) @import.module
  (wildcard_import) @import.wildcard) @import.stmt
`
		e := testTSExtractor("python", "tree_sitter_python", q, map[string]tsDefKind{}, nil)
		defer e.Close()
		specs, err := e.ExtractImports("m.py", []byte("from pkg.mod import a, b as c\nfrom other import *\n"))
		if err != nil {
			t.Fatal(err)
		}
		want := "1:named:a<-pkg.mod:a 1:named:c<-pkg.mod:b 2:wildcard:*<-other:"
		if got := importStrings(specs); got != want {
			t.Errorf("got  %s\nwant %s", got, want)
		}
	})
	t.Run("quoted module specifiers are unquoted", func(t *testing.T) {
		q := `(import_statement (import_clause (identifier) @import.alias) source: (string) @import.module) @import.stmt`
		e := testTSExtractor("javascript", "tree_sitter_javascript", q, map[string]tsDefKind{}, nil)
		defer e.Close()
		specs, err := e.ExtractImports("m.js", []byte("import d from './dep';\n"))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := importStrings(specs), "1:whole:d<-./dep:"; got != want {
			t.Errorf("got %s want %s", got, want)
		}
	})
}
