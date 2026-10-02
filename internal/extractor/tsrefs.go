// tsrefs.go — F-D reference edges (LB-F74, LB-F75; docs/design/
// language-breadth.md §6). TSExtractor.ExtractFileSpecs reads, in one parse,
// everything the file resolver (internal/fusion.ResolveFileEdges) takes from
// one file: its import specs (ExtractImports' own), the names it references
// outside a call's own text (a type annotation, `new X`, a superclass, a bare
// or qualified identifier) and the modules it re-exports. References and
// exports come from the dialect's references query (manifest
// queries.references), compiled and run apart from the tags query, so
// Extract's matches, and every symbol and call it yields, are unchanged.
//
// The capture vocabulary, for any references.scm:
//
//   - @ref.name: a referenced name, the node's text read as a name path
//     (whitespace and generic arguments dropped; segments joined by ".",
//     "::" or "\"; a leading "::" or "\" kept as written); a text that is no
//     name path (a call's result, a subscript) is not a reference;
//   - @ref.head with @ref.member: a qualified reference composed as
//     head.member, for a grammar that keeps the two apart (Java's
//     `Util.call()`, Dart's `p.Thing()`); the member is no reference of its
//     own;
//   - @ref.not: a node no @ref.name capture may read as a reference (a
//     member's name after a ".", an argument's label, the last segment of a
//     scoped path), so a broad pattern (every identifier) stays precise;
//   - @export.stmt, @export.module: a re-export of module; with one or more
//     @export.name (each optionally @export.alias) the statement re-exports
//     those names, with @export.hide every name but those, with
//     @export.namespace the module as one name (not followed), and with
//     none of them every name (TypeScript's `export * from`, Dart's
//     `export 'src/x.dart';`);
//   - @export.default: a name the file itself exports as its default
//     (`export default X`, `module.exports = X`), an ExportSpec with no
//     Module, As "default".
//
// A reference is kept only where the resolver could bind it: its head names
// an import's local binding, or — in a file whose imports bring unnamed names
// into scope (a wildcard import, an include/require/Swift import, a package or
// namespace the language makes visible unqualified) — it starts with an upper
// case letter (a type, class, or constant). A name written at a definition's
// own @name, or inside an import statement, is never a reference. The rest
// (locals, parameters, members) never reaches the store.
package extractor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// RefSpec is one name a file references outside a call's own text, and the
// lines (1-based, ascending, distinct) it is written on.
type RefSpec struct {
	Name  string `json:"n"`
	Lines []int  `json:"l"`
}

// ExportSpec is one re-export a file makes (LB-F75): the names of Module
// visible through this file as if it defined them.
type ExportSpec struct {
	// Module is the re-exported module spec, as an import of it is written;
	// "" for a name of the file's own (a default export).
	Module string `json:"m"`
	// Name is one re-exported name as Module defines it; "" re-exports every
	// name but Hide.
	Name string `json:"n,omitempty"`
	// As is the name Name is exported under, when it is renamed.
	As   string   `json:"a,omitempty"`
	Hide []string `json:"h,omitempty"`
	Line int      `json:"l"`
}

// FileSpecs is everything the file resolver reads from one file, stored as
// the file's file_imports row (internal/fusion owns the row).
type FileSpecs struct {
	Imports []ImportSpec `json:"imports"`
	Refs    []RefSpec    `json:"refs,omitempty"`
	Exports []ExportSpec `json:"exports,omitempty"`
}

// maxRefNames and maxRefLines bound one file's references (a generated or
// pathological file is gated before it gets here; these keep the stored row
// bounded regardless): the first names in sorted order, the first lines of
// each.
const (
	maxRefNames = 4096
	maxRefLines = 256
)

// ExtractFileSpecs parses relPath once and returns its imports (what
// ExtractImports returns), its references and its re-exports (the dialect's
// references query; none when the manifest names no such query).
func (e *TSExtractor) ExtractFileSpecs(relPath string, src []byte) (FileSpecs, error) {
	variant := e.variantFor(relPath)

	e.mu.Lock()
	defer e.mu.Unlock()

	l, q, err := e.leaseFor(context.Background(), variant)
	if err != nil {
		return FileSpecs{}, err
	}
	defer l.Release() // a no-op after noteFault's Discard
	e.parses.Add(1)
	tree, err := l.Parser().Parse(src)
	if err != nil {
		e.noteFault(l)
		return FileSpecs{}, fmt.Errorf("tsextract[%s] %s: parse: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}
	defer tree.Close()

	matches, err := q.Run(tree)
	if err != nil {
		e.noteFault(l)
		return FileSpecs{}, fmt.Errorf("tsextract[%s] %s: query: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}
	return e.fileSpecsLocked(l, tree, variant.rules, relPath, src, matches)
}

// fileSpecsLocked is ExtractFileSpecs after its parse and tags query: the
// imports from the tags matches, then the references query over tree. It is
// shared with ExtractWithFileSpecs, which has the tree and matches already.
// Caller holds e.mu and l.
func (e *TSExtractor) fileSpecsLocked(l *tsengine.Lease, tree *tsengine.Tree, rules *tsDialectRules, relPath string, src []byte, matches []tsengine.Match) (FileSpecs, error) {
	out := FileSpecs{Imports: importSpecsFromCaptures(matches, src)}
	if rules.refsQuery == "" {
		return out, nil
	}
	p := l.Parser()
	rq, ok := p.Cached("refs").(*tsengine.Query)
	var err error
	if !ok {
		rq, err = p.CompileQuery(rules.refsQuery)
		if err != nil {
			return FileSpecs{}, fmt.Errorf("tsextract[%s]: compiling references of %s: %w", e.spec.name, rules.id, err)
		}
		p.SetCached("refs", rq)
	}
	rmatches, err := rq.Run(tree)
	if err != nil {
		e.noteFault(l)
		return FileSpecs{}, fmt.Errorf("tsextract[%s] %s: references query: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}
	keep := refKeeper(out.Imports, rules)
	out.Refs = refSpecsFromCaptures(rmatches, src, definitionNames(matches), importRanges(matches), keep)
	out.Exports = exportSpecsFromCaptures(rmatches, src)
	return out, nil
}

// definitionNames is the byte range of every definition's own @name among
// the tags query's matches: a name written there declares, never references.
func definitionNames(matches []tsengine.Match) map[[2]uint32]bool {
	out := map[[2]uint32]bool{}
	for _, m := range matches {
		var name *tsengine.Node
		def := false
		for i := range m.Captures {
			c := &m.Captures[i]
			switch {
			case c.Name == "name":
				n := c.Node
				name = &n
			case strings.HasPrefix(c.Name, "definition."):
				def = true
			}
		}
		if def && name != nil {
			out[stmtKey(*name)] = true
		}
	}
	return out
}

// importRanges is the byte range of every import statement among the tags
// query's matches (a capture named import.*stmt): a name written in an
// import binds, it never references.
func importRanges(matches []tsengine.Match) [][2]uint32 {
	var rs [][2]uint32
	for _, m := range matches {
		for _, c := range m.Captures {
			if strings.HasPrefix(c.Name, "import.") && strings.HasSuffix(c.Name, "stmt") {
				rs = append(rs, stmtKey(c.Node))
			}
		}
	}
	return rs
}

// mergeRanges sorts byte ranges by start and merges overlapping ones.
func mergeRanges(rs [][2]uint32) [][2]uint32 {
	sort.Slice(rs, func(i, j int) bool { return rs[i][0] < rs[j][0] })
	var out [][2]uint32
	for _, r := range rs {
		if n := len(out); n > 0 && r[0] < out[n-1][1] {
			out[n-1][1] = max(out[n-1][1], r[1])
			continue
		}
		out = append(out, r)
	}
	return out
}

// inRanges reports whether n starts inside one of ranges (merged, sorted).
func inRanges(ranges [][2]uint32, n tsengine.Node) bool {
	i := sort.Search(len(ranges), func(i int) bool { return ranges[i][0] > n.StartByte })
	return i > 0 && n.StartByte < ranges[i-1][1]
}

// refKeeper is the filter described in the package doc: a reference whose
// head an import binds, or, where unnamed names are in scope, one starting
// with an upper case letter.
func refKeeper(imports []ImportSpec, rules *tsDialectRules) func(head string) bool {
	bound := map[string]bool{}
	upper := rules.packageStrategy == "capture"
	switch rules.importStrategy {
	case "include-path", "file-symbol", "swift-module":
		upper = true
	}
	for _, s := range imports {
		if s.Kind == ImportWildcard {
			upper = true
		}
		if s.LocalName != "" && s.LocalName != "*" {
			bound[s.LocalName] = true
			bound[refHead(s.LocalName)] = true
		}
	}
	return func(head string) bool {
		if bound[head] || rules.keepAllRefs {
			return true
		}
		if !upper {
			return false
		}
		r, _ := utf8.DecodeRuneInString(head)
		return unicode.IsUpper(r)
	}
}

// refSpecsFromCaptures reads the @ref.* captures (package doc), grouped by
// name, sorted by name, each name's lines ascending and distinct. A node a
// match captures as @ref.member or @ref.not is never a reference of its own
// (a member's name, an argument's label), and neither is one inside an
// import or a re-export statement (stmts: the tags query's import
// statements; the @export.stmt captures are added here).
func refSpecsFromCaptures(matches []tsengine.Match, src []byte, defNames map[[2]uint32]bool, stmts [][2]uint32, keep func(string) bool) []RefSpec {
	not := map[[2]uint32]bool{}
	for _, m := range matches {
		for _, c := range m.Captures {
			switch c.Name {
			case "ref.member", "ref.not":
				not[stmtKey(c.Node)] = true
			case "export.stmt":
				stmts = append(stmts, stmtKey(c.Node))
			}
		}
	}
	stmts = mergeRanges(stmts)
	skip := func(n tsengine.Node) bool {
		k := stmtKey(n)
		return defNames[k] || not[k] || inRanges(stmts, n)
	}
	lines := map[string]map[int]bool{}
	add := func(name string, line int) {
		if name == "" || !keep(refHead(name)) {
			return
		}
		ls := lines[name]
		if ls == nil {
			ls = map[int]bool{}
			lines[name] = ls
		}
		ls[line] = true
	}
	for _, m := range matches {
		var name, head, member *tsengine.Node
		for i := range m.Captures {
			c := &m.Captures[i]
			n := c.Node
			switch c.Name {
			case "ref.name":
				name = &n
			case "ref.head":
				head = &n
			case "ref.member":
				member = &n
			}
		}
		switch {
		case head != nil && member != nil:
			if defNames[stmtKey(*head)] || inRanges(stmts, *head) {
				continue
			}
			h, ok1 := refPath(head.Text(src))
			mb, ok2 := refPath(member.Text(src))
			if ok1 && ok2 && !strings.HasPrefix(mb, "::") && !strings.HasPrefix(mb, `\`) {
				add(h+"."+mb, head.Line())
			}
		case name != nil:
			if skip(*name) {
				continue
			}
			if p, ok := refPath(name.Text(src)); ok {
				add(p, name.Line())
			}
		}
	}
	if len(lines) == 0 {
		return nil
	}
	names := make([]string, 0, len(lines))
	for n := range lines {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) > maxRefNames {
		names = names[:maxRefNames]
	}
	out := make([]RefSpec, 0, len(names))
	for _, n := range names {
		ls := make([]int, 0, len(lines[n]))
		for l := range lines[n] {
			ls = append(ls, l)
		}
		sort.Ints(ls)
		if len(ls) > maxRefLines {
			ls = ls[:maxRefLines]
		}
		out = append(out, RefSpec{Name: n, Lines: ls})
	}
	return out
}

// refPath reads text as a name path: whitespace and every <...> group
// (generic arguments) dropped, then one or more identifiers joined by ".",
// "::" or "\", with an optional leading "::", "\" or "." (an absolute path).
// ok is false for anything else.
func refPath(text string) (string, bool) {
	var b strings.Builder
	depth := 0
	for _, r := range text {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth == 0 {
				return "", false
			}
			depth--
		case depth > 0, unicode.IsSpace(r):
		default:
			b.WriteRune(r)
		}
	}
	if depth != 0 {
		return "", false
	}
	s := b.String()
	rest := s
	switch {
	case strings.HasPrefix(rest, "::"):
		rest = rest[2:]
	case strings.HasPrefix(rest, `\`), strings.HasPrefix(rest, "."):
		rest = rest[1:]
	}
	if rest == "" {
		return "", false
	}
	for rest != "" {
		i := 0
		for i < len(rest) {
			r, w := utf8.DecodeRuneInString(rest[i:])
			if !(r == '_' || r == '$' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r))) {
				break
			}
			i += w
		}
		if i == 0 {
			return "", false
		}
		rest = rest[i:]
		switch {
		case rest == "":
		case strings.HasPrefix(rest, "::") && len(rest) > 2:
			rest = rest[2:]
		case (rest[0] == '.' || rest[0] == '\\') && len(rest) > 1:
			rest = rest[1:]
		default:
			return "", false
		}
	}
	return s, true
}

// refHead is a name path's first identifier.
func refHead(name string) string {
	name = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(name, "::"), `\`), ".")
	if i := strings.IndexAny(name, `.:\`); i >= 0 {
		return name[:i]
	}
	return name
}

// exportSpecsFromCaptures reads the @export.* captures (package doc), one
// statement at a time in source order.
func exportSpecsFromCaptures(matches []tsengine.Match, src []byte) []ExportSpec {
	type stmt struct {
		module    string
		line      int
		names     [][2]string // (name, alias)
		hide      []string
		namespace bool
	}
	var order [][2]uint32
	byStmt := map[[2]uint32]*stmt{}
	for _, m := range matches {
		caps := capMap(m)
		st, ok := caps["export.stmt"]
		mod, okm := caps["export.module"]
		if !ok || !okm {
			continue
		}
		k := stmtKey(st)
		s := byStmt[k]
		if s == nil {
			s = &stmt{module: unquoteJS(mod.Text(src)), line: st.Line()}
			byStmt[k] = s
			order = append(order, k)
		}
		if n, ok := caps["export.name"]; ok {
			pair := [2]string{n.Text(src), ""}
			if a, ok := caps["export.alias"]; ok {
				pair[1] = a.Text(src)
			}
			if !containsPair(s.names, pair) {
				s.names = append(s.names, pair)
			}
		}
		if h, ok := caps["export.hide"]; ok {
			s.hide = append(s.hide, h.Text(src))
		}
		if has(caps, "export.namespace") {
			s.namespace = true
		}
	}
	var out []ExportSpec
	for _, m := range matches {
		for _, c := range m.Captures {
			if c.Name == "export.default" {
				out = append(out, ExportSpec{Name: c.Node.Text(src), As: "default", Line: c.Node.Line()})
			}
		}
	}
	for _, k := range order {
		s := byStmt[k]
		switch {
		case s.module == "" || s.namespace:
		case len(s.names) > 0:
			for _, p := range s.names {
				es := ExportSpec{Module: s.module, Name: p[0], Line: s.line}
				if p[1] != "" && p[1] != p[0] {
					es.As = p[1]
				}
				out = append(out, es)
			}
		default:
			hide := append([]string(nil), s.hide...)
			sort.Strings(hide)
			out = append(out, ExportSpec{Module: s.module, Hide: compactStrings(hide), Line: s.line})
		}
	}
	return out
}

func containsPair(xs [][2]string, p [2]string) bool {
	for _, x := range xs {
		if x == p {
			return true
		}
	}
	return false
}

func compactStrings(xs []string) []string {
	if len(xs) == 0 {
		return nil
	}
	out := xs[:1]
	for _, x := range xs[1:] {
		if x != out[len(out)-1] {
			out = append(out, x)
		}
	}
	return out
}

// PackageScoped reports whether lang's code sees its own package's names
// unqualified (LB-F74): a namespace-dir dialect whose package comes from a
// clause (package.strategy "capture": Java, Kotlin, C#, PHP). lang is a
// tags dialect id or an extractor group (whose members must all agree).
func PackageScoped(lang string) bool {
	ds, err := LoadDialects()
	if err != nil {
		return false
	}
	members := tsExtractorGroups[lang]
	if members == nil {
		members = []string{lang}
	}
	any := false
	for _, d := range ds {
		for _, m := range members {
			if d.ID != m {
				continue
			}
			if d.Imports == nil || d.Imports.Strategy != "namespace-dir" || d.Package.Strategy != "capture" {
				return false
			}
			any = true
		}
	}
	return any
}
