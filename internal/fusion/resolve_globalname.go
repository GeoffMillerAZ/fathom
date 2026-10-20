package fusion

// resolve_globalname.go — the global-name import strategy (LB-F580, design
// §6's closed set). A GraphQL schema and a Prisma schema folder are the
// union of their files with no import statement: a type a field names is
// whichever file of the schema defines it. The dialect's imports query
// captures each type reference as a named spec (the name is the module); the
// spec binds the ONE file of its scope that defines the name with one of the
// manifest's defining kinds (imports.defines: a GraphQL type, never an
// extension of one). Scope "service" is every file of the dialect, "folder"
// the importer's own directory (prismaSchemaFolder's merge). A name two or
// more files of the scope define binds nothing and is counted ambiguous; a
// name none defines (a built-in scalar, an enum value) and a name the
// importer itself defines bind nothing and are no miss: the importer never
// edges to itself. The index is the resolution pass's symbols, built once
// per pass in memory (no table, no cache).

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// globalNameIndex is a universe's top-level definitions of defining kinds:
// the files defining each name (sorted, distinct), and the distinct
// (importer, name) lookups that bound or were ambiguous.
type globalNameIndex struct {
	files     map[string][]string
	bound     map[string]bool
	ambiguous map[string]bool
}

// indexGlobalNames builds env's global-name index from the pass's symbols:
// each top-level (undotted) definition of a universe file whose source kind
// is in defines (every kind when defines is empty).
func (e *resolveEnv) indexGlobalNames(symbols []core.SymbolNode, defines map[string]bool) {
	x := &globalNameIndex{files: map[string][]string{}, bound: map[string]bool{}, ambiguous: map[string]bool{}}
	for _, s := range symbols {
		if !e.set[s.FilePath] || s.Name == "" || strings.Contains(s.Name, ".") {
			continue
		}
		if len(defines) > 0 && !defines[string(s.Kind)] {
			continue
		}
		x.files[s.Name] = append(x.files[s.Name], s.FilePath)
	}
	for n, fs := range x.files {
		sort.Strings(fs)
		x.files[n] = dedupSorted(fs)
	}
	e.globals = x
}

// globalNameCounts is the distinct (importer, name) lookups that bound one
// file and that found two or more.
func (e *resolveEnv) globalNameCounts() (bound, ambiguous int) {
	if e.globals == nil {
		return 0, 0
	}
	return len(e.globals.bound), len(e.globals.ambiguous)
}

// resolveGlobalName resolves one type name referenced from importer to the
// one file of its scope defining it (see the file comment).
func resolveGlobalName(p extractor.ImportParams, env *resolveEnv, importer, name string) []string {
	x := env.globals
	if x == nil {
		return nil
	}
	defs := x.files[name]
	if len(defs) == 0 {
		return nil
	}
	if p.Scope == "folder" {
		dir := relDir(importer)
		var in []string
		for _, f := range defs {
			if relDir(f) == dir {
				in = append(in, f)
			}
		}
		defs = in
	}
	key := importer + "\x00" + name
	for _, f := range defs {
		if f == importer {
			return nil // the importer's own definition: no edge to itself
		}
	}
	switch len(defs) {
	case 0:
		return nil
	case 1:
		x.bound[key] = true
		return []string{defs[0]}
	}
	x.ambiguous[key] = true
	return nil
}

// globalNameDefines is the set of a resolver's defining source kinds.
func globalNameDefines(kinds []string) map[string]bool {
	if len(kinds) == 0 {
		return nil
	}
	m := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		m[k] = true
	}
	return m
}
