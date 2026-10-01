package fusion

// refresolve.go — F-D reference edges (LB-F74, LB-F76; docs/design/
// language-breadth.md §6): a name a file references outside a call's own text
// (extractor.RefSpec: a type annotation or hint, `new X`, extends, implements
// or with, a bare `X` or a qualified `Y.Z`) becomes a file-resolved edge from
// the definition enclosing the reference to the definition the name binds —
// through the same import bindings a call resolves through (a named, whole or
// wildcard import, a re-export it reaches), and through the language's own
// scope where the manifest says the language has one: its own package's names
// for a namespace-dir dialect whose package is a clause (Java, Kotlin, C#,
// PHP: PHP-Parser's NameResolver rule for an unqualified or relative class
// name), every enclosing namespace's too under imports.package_scope
// "nested" (C#), and a constant looked up through its lexical scopes under
// imports.constants "lexical" (Ruby, Sorbet's and Zeitwerk's constant
// resolution).
//
// Edge kind (the decision LB-F74 records): a reference edge is
// core.EdgeFileResolved, the call edges' own kind, tier pre-scan. Every
// consumer already reads it as a reach, not as a typed call:
// core.EdgeConfidenceOf reads it heuristic, the call graph walks it as an
// adjacency edge (callgraph.AddTypedEdges: callers, callees, blast_radius and
// verify's test_reaches), and the Go extractor has always kept a type
// reference in its Calls for the same reach (LB-F66); doc-xref's tool-format
// references are the same kind (docxref.go). A distinct kind would make each
// of those consumers skip a reference that a change to the type does reach.
//
// A reference binds nothing inside its own file (a same-file binding draws no
// edge: the file is already the reader's), and a reference outside every
// definition (an import line, a module-level statement) has no definition to
// draw from and is dropped.

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// refEdges resolves path's references, each from the innermost definition
// whose lines enclose it.
func (idx *resolveIndex) refEdges(r *importResolver, sc fileScope, path string, syms []core.SymbolNode, refs []extractor.RefSpec) []core.Edge {
	if len(refs) == 0 || len(syms) == 0 {
		return nil
	}
	at := enclosingAt(syms)
	var edges []core.Edge
	seen := map[[2]string]bool{}
	for _, ref := range refs {
		for _, line := range ref.Lines {
			s := at(line)
			if s == nil {
				continue
			}
			toKey, ok := idx.matchRef(r, sc, ref.Name, s, path)
			if !ok || toKey == s.Key() || idx.keyFile[toKey] == path {
				continue
			}
			k := [2]string{s.Key(), toKey}
			if seen[k] {
				continue
			}
			seen[k] = true
			edges = append(edges, core.Edge{FromFile: path, FromSymbol: s.Key(), ToSymbol: toKey, Kind: core.EdgeFileResolved})
		}
	}
	return edges
}

// matchRef binds one reference made in s: lexically first for a constant
// (imports.constants "lexical"), then through the file's imports as a call
// binds (no same-file sibling), then a type alias written bare, then the
// caller's own package.
func (idx *resolveIndex) matchRef(r *importResolver, sc fileScope, name string, s *core.SymbolNode, path string) (string, bool) {
	if r.params.TypePaths == "package-dir" {
		return idx.matchTypePath(name, append([]string{path}, sc.wildFiles...))
	}
	if r.params.Constants == "lexical" {
		if key, ok := idx.matchLexicalConstant(name, s.Name, nil, sc.wild); ok {
			return key, true
		}
	}
	absolute := strings.HasPrefix(name, `\`) || strings.HasPrefix(name, "::")
	dotted := refDotted(name)
	if !absolute {
		if key, ok := matchFileResolvedCall(dotted, sc.named, sc.whole, sc.x, nil, sc.wild); ok {
			return key, true
		}
		if keys, ok := sc.typeAlias[dotted]; ok {
			return keys[0], true
		}
		if key, ok := matchTypeAlias(dotted, sc.typeAlias, idx.qualified, idx.keyFile, idx.symName); ok {
			return key, true
		}
		if key, ok := matchWildMember(dotted, sc.wild, idx.qualified, idx.keyFile); ok {
			return key, true
		}
		if key, ok := matchNamedMember(dotted, sc.named, idx.qualified, idx.keyFile, idx.symName); ok {
			return key, true
		}
	}
	if r.pkgScope {
		return idx.matchPackage(r, name, s.Package)
	}
	return "", false
}

// refDotted is a reference's name path with "." between its segments ("::"
// and "\" read as "."), a leading absolute marker dropped.
func refDotted(name string) string {
	name = strings.TrimPrefix(strings.TrimPrefix(name, "::"), `\`)
	name = strings.ReplaceAll(name, "::", ".")
	return strings.ReplaceAll(name, `\`, ".")
}

// refSegments splits a name path at ".", "::" and "\".
func refSegments(name string) []string {
	return strings.Split(refDotted(name), ".")
}

// matchPackage binds a name (a call's text or a reference) through the
// caller's own package (LB-F74): the type pkg<sep>name declares — a relative
// qualified name resolved under the package, PHP's rule — else the type its
// head names there, then its member; under package_scope "nested" each
// enclosing namespace in turn (C#); and, for a qualified name, the fully
// qualified type it names. A name written absolute (`\App\Foo`) is only
// fully qualified. Only a type binds this way: a package's function is left
// to imports (PHP falls back from a namespace's function to the global one
// of that name at run time, so `json_decode()` in a namespace a test stubs
// it in means either).
func (idx *resolveIndex) matchPackage(r *importResolver, name, pkg string) (string, bool) {
	if idx.types == nil {
		return "", false
	}
	sep := r.submoduleSep()
	segs := refSegments(name)
	if len(segs) == 0 || segs[0] == "" {
		return "", false
	}
	full := strings.Join(segs, sep)
	absolute := strings.HasPrefix(name, `\`) || strings.HasPrefix(name, "::")
	if !absolute && pkg != "" {
		for _, p := range packageScopes(pkg, sep, r.params.PackageScope == "nested") {
			if key, ok := idx.soleKey(idx.types[p+sep+full]); ok {
				return key, true
			}
			if len(segs) > 1 {
				if keys := idx.types[p+sep+segs[0]]; len(keys) > 0 {
					return idx.memberOf(keys, segs[1])
				}
			}
		}
	}
	if len(segs) > 1 || absolute {
		return idx.soleKey(idx.types[full])
	}
	return "", false
}

// soleKey is a type's one key; a type more than one file declares (a C#
// partial class) is ambiguous at file granularity — which file a reference
// to it means has no answer — and binds nothing, as a name two wildcard
// imports define binds nothing (wildcardNames).
func (idx *resolveIndex) soleKey(keys []string) (string, bool) {
	if len(keys) == 0 {
		return "", false
	}
	file := ""
	for _, k := range keys {
		if k != keys[0] || !idx.isType[k] {
			return "", false
		}
		for _, f := range idx.keyFiles[k] {
			if file != "" && f != file {
				return "", false
			}
			file = f
		}
	}
	return keys[0], true
}

// memberOf is the member named member of the type keys declare: the one
// file of them that defines it (a partial class's member lives in one of
// its files); with none, the type itself when one file declares it; with
// more than one (an overload in two files), nothing, never a guess.
func (idx *resolveIndex) memberOf(keys []string, member string) (string, bool) {
	var found, foundIn string
	for _, k := range keys {
		for _, f := range idx.keyFiles[k] {
			mk, ok := idx.qualified[f][idx.symName[k]+"."+member]
			if !ok || (mk == found && f == foundIn) {
				continue
			}
			if found != "" {
				return "", false
			}
			found, foundIn = mk, f
		}
	}
	if found != "" {
		return found, true
	}
	return idx.soleKey(keys)
}

// packageScopes is pkg, then (nested) each enclosing namespace, innermost
// first.
func packageScopes(pkg, sep string, nested bool) []string {
	out := []string{pkg}
	if !nested {
		return out
	}
	for {
		i := strings.LastIndex(pkg, sep)
		if i <= 0 {
			return out
		}
		pkg = pkg[:i]
		out = append(out, pkg)
	}
}

// matchLexicalConstant binds a constant reference made in caller through its
// lexical scopes (LB-F76, Ruby; Sorbet's and Zeitwerk's constant lookup,
// behaviour only): `Base` in a method of Sinatra::Application is
// Sinatra::Application::Base, else Sinatra::Base, else Base — each looked up
// among the caller's own file's top-level names (localTop, nil for a
// reference), then the names a require brought into scope. A constant no
// require of the file brings binds nothing: Ruby resolves it from whichever
// file loaded it, which the file's own requires do not say (a global lookup,
// an autoloader's view, read file_edge_precision .06 on jekyll against the
// require dump). A qualified reference (`Sinatra::Base`) is looked up the same way, and one
// written absolute (`::Top`) only at the top level. The caller's scopes are
// its name less its method (after the last "."), split at "::": a class
// written `class A::B` is scoped like one nested in `module A`.
func (idx *resolveIndex) matchLexicalConstant(ref, caller string, localTop, wild map[string]string) (string, bool) {
	for _, c := range lexicalCandidates(ref, caller) {
		if key, ok := localTop[c]; ok {
			return key, true
		}
		if key, ok := wild[c]; ok {
			return key, true
		}
	}
	return "", false
}

// lexicalCandidates is the constant paths ref may name from caller,
// innermost scope first.
func lexicalCandidates(ref, caller string) []string {
	ref = strings.ReplaceAll(ref, ".", "::")
	if abs, ok := strings.CutPrefix(ref, "::"); ok {
		return []string{abs}
	}
	scope := caller
	if i := strings.LastIndex(scope, "."); i > strings.LastIndex(scope, "::") {
		scope = scope[:i]
	}
	var out []string
	for scope != "" {
		out = append(out, scope+"::"+ref)
		i := strings.LastIndex(scope, "::")
		if i < 0 {
			break
		}
		scope = scope[:i]
	}
	return append(out, ref)
}

// isConstantName reports a name whose last segment starts with an upper case
// letter (a class, a module or a constant, in a language that capitalizes
// them).
func isConstantName(name string) bool {
	if i := strings.LastIndexAny(name, ".:"); i >= 0 {
		name = name[i+1:]
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// enclosingAt indexes syms (one file's) by line: the innermost definition
// whose lines [Line, EndLine] hold a line (a definition with no EndLine holds
// its own line only); nil when none does.
func enclosingAt(syms []core.SymbolNode) func(line int) *core.SymbolNode {
	order := make([]int, len(syms))
	maxLine := 0
	for i, s := range syms {
		order[i] = i
		maxLine = max(maxLine, s.Line, s.EndLine)
	}
	span := func(s core.SymbolNode) int { return max(s.EndLine, s.Line) - s.Line }
	sort.SliceStable(order, func(a, b int) bool {
		sa, sb := syms[order[a]], syms[order[b]]
		if span(sa) != span(sb) {
			return span(sa) > span(sb)
		}
		if sa.Line != sb.Line {
			return sa.Line < sb.Line
		}
		return sa.Name < sb.Name
	})
	at := make([]int32, maxLine+1)
	for i := range at {
		at[i] = -1
	}
	for _, i := range order {
		s := syms[i]
		if s.Line <= 0 {
			continue
		}
		for l := s.Line; l <= max(s.EndLine, s.Line); l++ {
			at[l] = int32(i)
		}
	}
	return func(line int) *core.SymbolNode {
		if line <= 0 || line >= len(at) || at[line] < 0 {
			return nil
		}
		return &syms[at[line]]
	}
}

// typeContainers are the canonical kinds whose definitions hold members
// (design §2.3): a definition with an unqualified name inside one of them
// is a member, not a name of the file's scope.
var typeContainers = map[string]bool{"class": true, "struct": true, "interface": true, "enum": true, "trait": true, "union": true}

// memberKeys is every symbol whose Name carries no qualifier (no ".", no
// "::") yet whose lines lie inside a type's definition in its own file: a
// field, a property or an enum member some dialects name bare (a C#
// property, a Java field). Such a name is no top-level name of its file and
// no type of its package: a wildcard import and a package scope never bring
// it into scope (a property named CancellationToken is not the type a
// reference to CancellationToken means).
func memberKeys(symbols []core.SymbolNode) map[string]bool {
	byFile := map[string][]int{}
	for i, s := range symbols {
		if c, ok := extractor.CanonicalKind(string(s.Kind)); ok && typeContainers[c] {
			byFile[s.FilePath] = append(byFile[s.FilePath], i)
		}
	}
	if len(byFile) == 0 {
		return nil
	}
	out := map[string]bool{}
	for _, s := range symbols {
		if strings.Contains(s.Name, ".") || strings.Contains(s.Name, "::") {
			continue
		}
		for _, ci := range byFile[s.FilePath] {
			c := symbols[ci]
			if c.Key() == s.Key() || (c.Line == s.Line && c.Name == s.Name) {
				continue
			}
			if c.Line <= s.Line && max(s.EndLine, s.Line) <= max(c.EndLine, c.Line) && (c.Line < s.Line || c.EndLine > s.EndLine) {
				out[s.Key()] = true
				break
			}
		}
	}
	return out
}

// matchTypePath binds a package-qualified type path (imports.type_paths
// "package-dir", protobuf; LB-F77) among files (the referring file first,
// then the files its imports bring into scope): `other_package.Embed.Enumerated`
// is the type Embed.Enumerated of a file whose directory ends in the segment
// other_package; an unqualified `Embed.Enumerated` is looked up in the
// referring file's own package (its directory) first, as protobuf resolves a
// name from the current package outward, then in any of them. Each split of
// the path into a package prefix and a type name is tried, the longest type
// name first; a split two files answer is ambiguous and binds nothing.
func (idx *resolveIndex) matchTypePath(name string, files []string) (string, bool) {
	segs := refSegments(name)
	own := relDir(files[0])
	for i := 0; i < len(segs); i++ {
		typeName := strings.Join(segs[i:], ".")
		pkgDir := strings.Join(segs[:i], "/")
		in := func(f string) bool {
			d := relDir(f)
			if pkgDir == "" {
				return true
			}
			return d == pkgDir || strings.HasSuffix(d, "/"+pkgDir)
		}
		if pkgDir == "" {
			if key, ok, ambiguous := idx.uniqueIn(files, typeName, func(f string) bool { return relDir(f) == own }); ok || ambiguous {
				return key, ok
			}
		}
		if key, ok, ambiguous := idx.uniqueIn(files, typeName, in); ok || ambiguous {
			return key, ok
		}
	}
	return "", false
}

// uniqueIn is the one key typeName names among files passing keep; ambiguous
// when two of them define it.
func (idx *resolveIndex) uniqueIn(files []string, typeName string, keep func(string) bool) (key string, ok, ambiguous bool) {
	foundIn := ""
	for _, f := range files {
		k, has := idx.qualified[f][typeName]
		if !has || !keep(f) || f == foundIn {
			continue
		}
		if foundIn != "" {
			return "", false, true
		}
		key, foundIn = k, f
	}
	return key, foundIn != "", false
}
