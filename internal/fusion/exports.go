package fusion

// exports.go — LB-F75, export following: the generic channel through which
// a name one file makes visible from another reaches the file that defines
// it. A file's re-exports are extractor.ExportSpec values (a dialect's
// references query: TypeScript's `export * from`, `export { A as B } from`,
// Dart's barrel `export 'src/x.dart' show/hide …`; a Rust `pub use` plugs in
// the same way) and, under dotted-module (Python), its own imports: a module's
// imported names are its attributes, so `from pkg import Foo` reaches Foo
// through pkg/__init__.py's `from .core import Foo`.
//
// A lookup follows re-exports depth-first, at most maxExportDepth files deep,
// never through a file twice; a name two different re-exports of one file
// make visible from different definitions is ambiguous and binds nothing
// (TypeScript and Dart both refuse such a name), never a guess.

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// maxExportDepth bounds how many re-exporting files a lookup follows.
const maxExportDepth = 8

// exportEdge is one re-export of one file, its module resolved.
type exportEdge struct {
	targets []string // the module's files, sorted
	name    string   // the name in targets; "" for every name
	as      string   // the name it is visible under ("" = name)
	hide    map[string]bool
}

// exportIndex is every file's re-exports, resolved lazily.
type exportIndex struct {
	r        *importResolver
	topLevel map[string]map[string]string
	specs    map[string][]extractor.ExportSpec
	edges    map[string][]exportEdge
	names    map[string]map[string]exportHit // memoized visibleNames
}

// newExportIndex reads every file's re-exports: its ExportSpecs, and under
// dotted-module its named and wildcard imports.
func newExportIndex(r *importResolver, specs map[string]extractor.FileSpecs, topLevel map[string]map[string]string) *exportIndex {
	x := &exportIndex{r: r, topLevel: topLevel, specs: map[string][]extractor.ExportSpec{}, edges: map[string][]exportEdge{}, names: map[string]map[string]exportHit{}}
	for file, fs := range specs {
		out := append([]extractor.ExportSpec(nil), fs.Exports...)
		if r.strategy == "dotted-module" {
			for _, s := range fs.Imports {
				switch s.Kind {
				case extractor.ImportNamed:
					es := extractor.ExportSpec{Module: s.Module, Name: s.ExportedName, Line: s.Line}
					if s.LocalName != s.ExportedName {
						es.As = s.LocalName
					}
					out = append(out, es)
				case extractor.ImportWildcard:
					out = append(out, extractor.ExportSpec{Module: s.Module, Line: s.Line})
				}
			}
		}
		if len(out) > 0 {
			x.specs[file] = out
		}
	}
	return x
}

// of is file's re-exports, each module resolved from file.
func (x *exportIndex) of(file string) []exportEdge {
	if x == nil {
		return nil
	}
	if es, ok := x.edges[file]; ok {
		return es
	}
	var out []exportEdge
	for _, s := range x.specs[file] {
		var targets []string
		if s.Module == "" {
			targets = []string{file} // the file's own name, exported under As
		} else if targets = x.r.resolve(file, s.Module); len(targets) == 0 {
			continue
		}
		e := exportEdge{targets: targets, name: s.Name, as: s.As}
		if len(s.Hide) > 0 {
			e.hide = map[string]bool{}
			for _, h := range s.Hide {
				e.hide[h] = true
			}
		}
		out = append(out, e)
	}
	x.edges[file] = out
	return out
}

// exportHit is the definition a name binds: its key and the file whose
// top level defines it.
type exportHit struct{ key, file string }

// lookup is the key of the definition name binds when imported from file:
// file's own top-level name, else the one re-export that makes it visible
// ("default": the file's default export, LB-F75).
func (x *exportIndex) lookup(file, name string) (string, bool) {
	h, ok := x.lookupIn(file, name, maxExportDepth, map[string]bool{})
	return h.key, ok
}

func (x *exportIndex) lookupIn(file, name string, depth int, seen map[string]bool) (exportHit, bool) {
	if x == nil {
		return exportHit{}, false
	}
	if key, ok := x.topLevel[file][name]; ok {
		return exportHit{key, file}, true
	}
	if depth == 0 || seen[file] {
		return exportHit{}, false
	}
	seen[file] = true
	defer delete(seen, file)
	var found exportHit
	for _, e := range x.of(file) {
		inner := ""
		switch {
		case e.name == "":
			if e.hide[name] {
				continue
			}
			inner = name
		case e.as == name || (e.as == "" && e.name == name):
			inner = e.name
		default:
			continue
		}
		for _, t := range e.targets {
			if t == file {
				if key, ok := x.topLevel[file][inner]; ok && e.name != "" && inner != name {
					h := exportHit{key, file} // a default export of the file's own name
					if found.key != "" && found != h {
						return exportHit{}, false
					}
					found = h
				}
				continue
			}
			h, ok := x.lookupIn(t, inner, depth-1, seen)
			if !ok {
				continue
			}
			if found.key != "" && found != h {
				return exportHit{}, false // two re-exports bind the name differently
			}
			found = h
		}
	}
	return found, found.key != ""
}

// visibleNames is every name a wildcard import of file brings into scope:
// its top-level names and every name its re-exports make visible, each to
// its definition (an ambiguous name left out).
func (x *exportIndex) visibleNames(file string) map[string]exportHit {
	if x == nil {
		return nil
	}
	if m, ok := x.names[file]; ok {
		return m
	}
	own := map[string]exportHit{}
	for name, key := range x.topLevel[file] {
		own[name] = exportHit{key, file}
	}
	x.names[file] = own // a cycle back to file sees its own names
	if len(x.specs[file]) == 0 {
		return own
	}
	out := map[string]exportHit{}
	for n, h := range own {
		out[n] = h
	}
	ambiguous := map[string]bool{}
	add := func(name string, h exportHit) {
		if _, mine := own[name]; mine {
			return // a file's own definition shadows what it re-exports
		}
		if prev, ok := out[name]; ok && prev != h {
			ambiguous[name] = true
		}
		out[name] = h
	}
	for _, e := range x.of(file) {
		for _, t := range e.targets {
			if t == file {
				continue
			}
			if e.name != "" {
				if h, ok := x.lookupIn(t, e.name, maxExportDepth, map[string]bool{}); ok {
					as := e.as
					if as == "" {
						as = e.name
					}
					add(as, h)
				}
				continue
			}
			inner := x.visibleNames(t)
			names := make([]string, 0, len(inner))
			for n := range inner {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				if !e.hide[n] {
					add(n, inner[n])
				}
			}
		}
	}
	for n := range ambiguous {
		delete(out, n)
	}
	x.names[file] = out
	return out
}

// topLevelOnly is file's own top-level names (shared: never mutated).
func (x *exportIndex) topLevelOnly(file string) map[string]string {
	if x == nil {
		return nil
	}
	return x.topLevel[file]
}
