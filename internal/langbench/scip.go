package langbench

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	fscip "github.com/geoffmilleraz/fathom/internal/scip"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// SCIPResult is one subject's tier-A measurement from a SCIP index.
type SCIPResult struct {
	Tool, Version string
	Counts        map[string]int
	// FileEdges are the index's cross-file edges at file granularity:
	// (referencing document, defining document), both in this index.
	FileEdges map[[2]string]bool
	// EnclosingFallback counts definitions the producer gave no
	// enclosing_range (scip-clang 0.4.0 sets none) that took their mapped
	// fathom definition's line span instead (owner decision D5), so call
	// truth stays measurable; the reading discloses the count.
	EnclosingFallback int
	// CallTruth and Docs are the call pairs the index proves and the
	// documents it covers: the worker's diagnosis of a precision or recall
	// miss (pairs on either side of the comparison), never part of a reading.
	CallTruth map[CallPair]bool
	Docs      map[string]bool
}

// scipBareName returns a SCIP symbol's leaf descriptor name and whether it
// is callable (a method descriptor, "name()."). Locals return "".
func scipBareName(sym string) (string, bool) {
	if sym == "" || strings.HasPrefix(sym, "local ") {
		return "", false
	}
	callable := strings.HasSuffix(sym, ").")
	if isSCIPDart(sym) && strings.HasSuffix(sym, "/") {
		return "", false // scip-dart's file symbol (`lib/`a.dart`/`) is no definition
	}
	s := sym
	// Drop the trailing descriptor suffix: "()." method, "." term, "#"
	// type, "/" namespace, ":" meta, "!" macro, "[x]" type parameter,
	// "(x)" parameter.
	switch {
	case strings.HasSuffix(s, ")."):
		s = s[:strings.LastIndex(s, "(")]
	case strings.HasSuffix(s, "]"), strings.HasSuffix(s, ")"):
		return "", false // type parameters and parameters are not definitions fathom maps
	default:
		s = s[:len(s)-1]
	}
	if strings.HasSuffix(s, "`") {
		if i := strings.LastIndex(s[:len(s)-1], "`"); i >= 0 {
			name := leafName(s[i+1 : len(s)-1])
			if isSCIPDart(sym) {
				name = dartName(name, s[:i])
			}
			return name, callable
		}
	}
	// "]" ends a descriptor too: rust-analyzer writes an impl block's members
	// as `impl#[Type]name().`, where `[Type]` is a descriptor of its own.
	i := strings.LastIndexAny(s, "/#.:] ")
	return leafName(s[i+1:]), callable
}

// isSCIPDart reports whether sym is one scip-dart wrote.
func isSCIPDart(sym string) bool { return strings.HasPrefix(sym, "scip-dart ") }

// dartName is scip-dart's descriptor leaf in the name fathom's dialect gives
// the same definition: an unnamed constructor (`Foo#`<constructor>`().`) is
// its class, Foo, which is also how Foo(...) is written as a call, and an
// accessor (`Foo#`<get>x`.`, `<set>x`) is x. owner is the descriptor text
// before the leaf ("…/Foo#").
func dartName(leaf, owner string) string {
	switch {
	case leaf == "<constructor>":
		o := strings.TrimSuffix(owner, "#")
		return o[strings.LastIndexAny(o, "/#.:] `")+1:]
	case strings.HasPrefix(leaf, "<get>"), strings.HasPrefix(leaf, "<set>"):
		return leaf[len("<get>"):]
	}
	return leaf
}

// dartCallAt reports whether the source at range r (0-based line, start and end
// columns) is the callee's own name written as a call: followed, on its line, by
// `(` or by type arguments and then `(`.
func dartCallAt(lines [][]byte, r [4]int32, callee string) bool {
	if r[0] != r[2] || int(r[0]) >= len(lines) || int(r[3]) > len(lines[r[0]]) || r[1] > r[3] {
		return false
	}
	line := lines[r[0]]
	if string(line[r[1]:r[3]]) != callee {
		return false
	}
	rest := bytes.TrimLeft(line[r[3]:], " \t")
	if len(rest) > 0 && rest[0] == '<' {
		depth := 0
		for i, c := range rest {
			switch c {
			case '<':
				depth++
			case '>':
				depth--
				if depth == 0 {
					rest = bytes.TrimLeft(rest[i+1:], " \t")
					return len(rest) > 0 && rest[0] == '('
				}
			}
		}
		return false
	}
	return len(rest) > 0 && rest[0] == '('
}

// dartPrivateKind reports whether a scip-dart local symbol of this kind is a
// private member (one written with a leading `_`), not a variable or a
// parameter of a function.
func dartPrivateKind(k scippb.SymbolInformation_Kind) bool {
	switch k {
	case scippb.SymbolInformation_Method, scippb.SymbolInformation_Function, scippb.SymbolInformation_Constructor,
		scippb.SymbolInformation_Field, scippb.SymbolInformation_Property, scippb.SymbolInformation_Variable,
		scippb.SymbolInformation_Class, scippb.SymbolInformation_Mixin, scippb.SymbolInformation_Enum,
		scippb.SymbolInformation_TypeAlias, scippb.SymbolInformation_Extension:
		return true
	}
	return false
}

// leafName is a SCIP descriptor's own name in the form BareName gives fathom's:
// scip-clang spells a constructor of a class template with its arguments
// ("basic_fp<F>") and a conversion or symbolic operator with the operator's
// own spelling ("operator basic_string_view", "operator()"); a template
// argument list is not part of the name, and BareName owns the operator rule.
// Operators are left to BareName untouched ("operator<=>" ends in ">").
func leafName(name string) string {
	if strings.Contains(name, "operator") {
		return BareName(name)
	}
	if i := strings.IndexByte(name, '<'); i > 0 && strings.HasSuffix(name, ">") {
		name = name[:i]
	}
	return name
}

type scipDef struct {
	doc       string
	line      int // 1-based
	enc       [4]int32
	hasEnc    bool
	name      string
	fathomDef string // the mapped fathom node's bare name, "" when unmapped
}

func rangeOf(r []int32) ([4]int32, bool) {
	switch len(r) {
	case 3:
		return [4]int32{r[0], r[1], r[0], r[2]}, true
	case 4:
		return [4]int32{r[0], r[1], r[2], r[3]}, true
	}
	return [4]int32{}, false
}

func contains(outer, inner [4]int32) bool {
	before := func(l1, c1, l2, c2 int32) bool { return l1 < l2 || (l1 == l2 && c1 <= c2) }
	return before(outer[0], outer[1], inner[0], inner[1]) && before(inner[2], inner[3], outer[2], outer[3])
}

// ScoreSCIP measures nodes (fathom's extraction of the subject) against a
// SCIP index (tier A, §1.3): def_mapping (definitions matched to a fathom
// definition in the same file by name, |Δline| ≤ 2), call pairs (§1.4:
// callable references inside a mapped definition), typed_recall (the share
// of the index's reference edges fathom's SCIP import keeps as typed
// edges, internal/scip.ImportIndex), and the cross-file edge set.
func ScoreSCIP(raw []byte, service string, nodes []core.SymbolNode) (SCIPResult, error) {
	return ScoreSCIPWith(raw, service, nodes, SCIPOptions{})
}

// SCIPOptions are a language's rules for what its SCIP producer's references
// prove as call sites (LB-Q-rust). The zero value is every other language's:
// a callable descriptor ("name().") is a call.
type SCIPOptions struct {
	// Source reads a subject file (nil: no source reads); WrittenCallsOnly
	// needs it.
	Source func(path string) []byte
	// MacroCalls: a reference to a macro symbol (descriptor suffix "!") is a
	// call. Rust writes `println!(..)` as a call to `println`, as the
	// incumbent extractor's own contract records; rust-analyzer references
	// every invocation by the macro's name.
	MacroCalls bool
	// WrittenCallsOnly: a callable reference is a call site only when the
	// source at its range is the callee's own name. rust-analyzer resolves an
	// operator to its trait method — `?` to branch, `a == b` to eq, `!x` to
	// not, `*x` to deref, `a[i]` to index — at the operator's own range: no
	// call is written there, so no extractor of call sites can find one. The
	// occurrences left out are counted (CntScipCallsUnwritten) and disclosed.
	WrittenCallsOnly bool
	// PrivateMembers: a `local N` definition whose SymbolInformation.kind is a
	// callable, field, accessor or variable is a definition too, named by the
	// identifier written at its occurrence (needs Source). scip-dart writes every
	// private member (a leading `_`) as a local symbol; without this, a call inside
	// a private method is attributed to the class around it and a private
	// definition is left out of def_mapping. It also holds a call to the
	// Dart standard: a callable reference is a call site only when its name is
	// written followed by `(` (or type arguments and `(`); a tear-off, an operator
	// and the `super(...)` of an initializer are not written calls, and are counted
	// (CntScipCallsUnwritten) and disclosed.
	PrivateMembers bool
}

// SCIPOptionsFor is the option set for a dialect's SCIP producer.
func SCIPOptionsFor(dialect string, src func(path string) []byte) SCIPOptions {
	if dialect == "rust" {
		return SCIPOptions{Source: src, MacroCalls: true, WrittenCallsOnly: src != nil}
	}
	if dialect == "dart" {
		return SCIPOptions{Source: src, PrivateMembers: src != nil}
	}
	return SCIPOptions{}
}

// ScoreSCIPWith is ScoreSCIP under a language's call-site rules.
func ScoreSCIPWith(raw []byte, service string, nodes []core.SymbolNode, opts SCIPOptions) (SCIPResult, error) {
	idx := &scippb.Index{}
	if err := proto.Unmarshal(raw, idx); err != nil {
		return SCIPResult{}, fmt.Errorf("scip: decode index: %w", err)
	}
	res := SCIPResult{Counts: map[string]int{}, FileEdges: map[[2]string]bool{}}
	if ti := idx.GetMetadata().GetToolInfo(); ti != nil {
		res.Tool, res.Version = ti.GetName(), ti.GetVersion()
	}

	type fk struct{ path, name string }
	fathomAt := map[fk][][2]int{} // (line, end line) per same-named definition
	for _, n := range nodes {
		k := fk{n.FilePath, BareName(n.Name)}
		fathomAt[k] = append(fathomAt[k], [2]int{n.Line, n.EndLine})
	}
	docKinds := map[string]map[string]scippb.SymbolInformation_Kind{} // scip-dart: each document's symbol kinds
	definedIn := map[string]string{}                                  // symbol -> defining document
	defsByDoc := map[string][]*scipDef{}
	docs := map[string]bool{}
	for _, d := range idx.GetDocuments() {
		docs[d.GetRelativePath()] = true
		// scip-dart defines an `import ... as p` prefix as a Namespace: no
		// extractor has a symbol for it (nor for a file, scipBareName).
		var kinds map[string]scippb.SymbolInformation_Kind
		var lines [][]byte
		if res.Tool == "scip-dart" {
			kinds = map[string]scippb.SymbolInformation_Kind{}
			for _, si := range d.GetSymbols() {
				kinds[si.GetSymbol()] = si.GetKind()
			}
			docKinds[d.GetRelativePath()] = kinds
			if opts.PrivateMembers && opts.Source != nil {
				lines = bytes.Split(opts.Source(d.GetRelativePath()), []byte("\n"))
			}
		}
		for _, o := range d.GetOccurrences() {
			if o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) == 0 {
				continue
			}
			if kinds[o.GetSymbol()] == scippb.SymbolInformation_Namespace {
				continue
			}
			name, _ := scipBareName(o.GetSymbol())
			r, ok := rangeOf(o.GetRange())
			if name == "" && ok && lines != nil && strings.HasPrefix(o.GetSymbol(), "local ") && dartPrivateKind(kinds[o.GetSymbol()]) &&
				r[0] == r[2] && int(r[0]) < len(lines) && int(r[3]) <= len(lines[r[0]]) {
				name = string(lines[r[0]][r[1]:r[3]])
			}
			if name == "" || !ok {
				continue
			}
			if _, seen := definedIn[o.GetSymbol()]; !seen {
				definedIn[o.GetSymbol()] = d.GetRelativePath()
			}
			def := &scipDef{doc: d.GetRelativePath(), line: int(r[0]) + 1, name: name}
			if enc, ok := rangeOf(o.GetEnclosingRange()); ok {
				def.enc, def.hasEnc = enc, true
			}
			res.Counts[CntScipDefs]++
			for _, span := range fathomAt[fk{def.doc, name}] {
				if d := span[0] - def.line; d >= -2 && d <= 2 {
					def.fathomDef = name
					res.Counts[CntScipDefsMapped]++
					if !def.hasEnc && span[1] >= span[0] && span[1] > 0 {
						// D5: no enclosing_range from the producer — the
						// mapped definition's own lines stand in (0-based,
						// whole lines).
						lo := min(int32(span[0]-1), r[0])
						def.enc, def.hasEnc = [4]int32{lo, 0, int32(span[1] - 1), 1 << 30}, true
						res.EnclosingFallback++
					}
					break
				}
			}
			defsByDoc[def.doc] = append(defsByDoc[def.doc], def)
		}
	}

	truth := map[CallPair]bool{}
	for _, d := range idx.GetDocuments() {
		doc := d.GetRelativePath()
		var srcLines [][]byte
		if (opts.WrittenCallsOnly || opts.PrivateMembers) && opts.Source != nil {
			srcLines = bytes.Split(opts.Source(doc), []byte("\n"))
		}
		for _, o := range d.GetOccurrences() {
			if o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) != 0 {
				continue
			}
			r, ok := rangeOf(o.GetRange())
			if !ok {
				continue
			}
			if to, ok := definedIn[o.GetSymbol()]; ok && to != doc {
				res.FileEdges[[2]string{doc, to}] = true
			}
			callee, callable := scipBareName(o.GetSymbol())
			if opts.MacroCalls && strings.HasSuffix(o.GetSymbol(), "!") {
				callable = true
			}
			if opts.PrivateMembers && srcLines != nil && !callable && strings.HasPrefix(o.GetSymbol(), "local ") {
				// A private method, function or constructor is a local symbol: it
				// is callable by its kind and named by what is written.
				switch docKinds[doc][o.GetSymbol()] {
				case scippb.SymbolInformation_Method, scippb.SymbolInformation_Function, scippb.SymbolInformation_Constructor:
					if r[0] == r[2] && int(r[0]) < len(srcLines) && int(r[3]) <= len(srcLines[r[0]]) {
						callee, callable = string(srcLines[r[0]][r[1]:r[3]]), true
					}
				}
			}
			if !callable || callee == "" {
				continue
			}
			if srcLines != nil {
				written := writtenAt(srcLines, r, callee)
				if opts.PrivateMembers {
					// Dart: a call is the name followed by its arguments; the
					// producer also references a tear-off (`onError: c.addError`), an
					// operator and the `super`/`this` of an initializer.
					written = dartCallAt(srcLines, r, callee)
				}
				if !written {
					res.Counts[CntScipCallsUnwritten]++
					continue
				}
			}
			var inner *scipDef
			for _, def := range defsByDoc[doc] {
				if def.hasEnc && contains(def.enc, r) && (inner == nil || contains(inner.enc, def.enc)) {
					inner = def
				}
			}
			if inner == nil || inner.fathomDef == "" {
				continue
			}
			truth[CallPair{doc, inner.fathomDef, callee}] = true
		}
	}
	found := FathomCallPairs(nodes, docs)
	ScorePairs(truth, found, res.Counts, CntCallTruth, CntCallTruthFound, CntCallFathom, CntCallFathomInTrue)
	res.CallTruth, res.Docs = truth, docs

	imp, err := fscip.ImportIndex(service, raw, nodes)
	if err != nil {
		return SCIPResult{}, err
	}
	res.Counts[CntScipEdgesTyped] = imp.Report.Direct
	res.Counts[CntScipEdges] = imp.Report.Direct + imp.Report.Skipped["unmapped-endpoint"]
	return res, nil
}

// writtenAt reports whether the source at range r (0-based line, start and
// end columns) is the callee's own name, allowing a raw identifier's r# prefix.
func writtenAt(lines [][]byte, r [4]int32, callee string) bool {
	if r[0] != r[2] || int(r[0]) >= len(lines) {
		return false
	}
	line := lines[r[0]]
	if r[1] < 0 || r[1] > r[3] || int(r[3]) > len(line) {
		return false
	}
	text := strings.TrimPrefix(string(line[r[1]:r[3]]), "r#")
	return text == strings.TrimPrefix(callee, "r#")
}

// ScoreFileEdges compares fathom's file-resolved edges (FromFile ->
// ToSymbol's file) against SCIP's cross-file edges at file granularity.
func ScoreFileEdges(truth map[[2]string]bool, edges []core.Edge, nodes []core.SymbolNode, counts map[string]int) {
	fileOf := map[string]string{}
	for _, n := range nodes {
		fileOf[n.Key()] = n.FilePath
	}
	mine := map[[2]string]bool{}
	for _, e := range edges {
		to, ok := fileOf[e.ToSymbol]
		if !ok || to == e.FromFile {
			continue
		}
		mine[[2]string{e.FromFile, to}] = true
	}
	keys := make([][2]string, 0, len(mine))
	for k := range mine {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i][0]+keys[i][1] < keys[j][0]+keys[j][1] })
	counts[CntFileEdgesTruth] += len(truth)
	counts[CntFileEdgesFathom] += len(mine)
	for _, k := range keys {
		if truth[k] {
			counts[CntFileEdgesAgree]++
		}
	}
}

// docKeywords are the declaration keywords a SCIP producer's hover text
// (SymbolInformation.documentation, e.g. scip-typescript's "```ts\nclass
// Foo\n```" or "(method) Foo.bar(): void") names a definition's kind by.
var docKeywords = map[string]bool{
	"class": true, "interface": true, "enum": true, "function": true, "method": true,
	"constructor": true, "property": true, "field": true, "var": true, "let": true,
	"const": true, "type": true, "module": true, "namespace": true, "struct": true,
	"record": true, "trait": true, "getter": true, "setter": true, "alias": true,
	"macro": true, "event": true, "delegate": true, "operator": true, "object": true,
	"get": true, "set": true,
}

var docModifiers = map[string]bool{
	"public": true, "private": true, "protected": true, "internal": true, "static": true,
	"abstract": true, "final": true, "export": true, "declare": true, "default": true,
	"readonly": true, "async": true, "sealed": true, "override": true, "virtual": true,
	"partial": true, "extern": true, "unsafe": true, "open": true, "data": true,
}

// docKeyword is the declaration keyword of a documentation string's first
// code line ("(method) …" or "[modifiers] <keyword> …"), "" when it names
// none this harness knows.
func docKeyword(doc string) string {
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		if strings.HasPrefix(line, "(") {
			if i := strings.IndexByte(line, ')'); i > 0 {
				if k := strings.TrimSpace(line[1:i]); docKeywords[k] {
					return k
				}
			}
			return ""
		}
		for _, w := range strings.Fields(line) {
			if docModifiers[w] {
				continue
			}
			if docKeywords[w] {
				return w
			}
			return ""
		}
		return ""
	}
	return ""
}

// scipDescriptorKind is a definition's kind from its symbol's last
// descriptor (SCIP's grammar), and whether its parent is a type.
func scipDescriptorKind(sym string) (kind string, scoped bool) {
	s := sym
	switch {
	case strings.HasSuffix(s, ")."):
		kind, s = "function", s[:strings.LastIndex(s, "(")]
	case strings.HasSuffix(s, "#"):
		kind, s = "type", s[:len(s)-1]
	case strings.HasSuffix(s, "."):
		kind, s = "term", s[:len(s)-1]
	case strings.HasSuffix(s, "/"):
		kind, s = "namespace", s[:len(s)-1]
	case strings.HasSuffix(s, "!"):
		kind, s = "macro", s[:len(s)-1]
	case strings.HasSuffix(s, ":"):
		kind, s = "meta", s[:len(s)-1]
	default:
		return "", false
	}
	if strings.HasSuffix(s, "`") {
		if i := strings.LastIndex(s[:len(s)-1], "`"); i >= 0 {
			s = s[:i]
		}
	} else if i := strings.LastIndexAny(s, "/#.:] "); i >= 0 {
		s = s[:i+1]
	}
	// An impl block has no name of its own: rust-analyzer's members of one sit
	// under `impl#[Type]` or `impl#[Type][Trait]`, a type member like any other.
	return kind, strings.HasSuffix(s, "#") || (strings.HasSuffix(s, "]") && strings.Contains(s, "impl#["))
}

// SCIPDefs reads a SCIP index as a tier-A L1 oracle (owner decision D2):
// every definition occurrence of a named, non-parameter symbol, with its
// 1-based line, its enclosing range's last line (0 when the producer gives
// none), and a kind — SymbolInformation.kind lowercased when set, else the
// documentation's declaration keyword, else the descriptor suffix
// ("function", "type", "term", "namespace", "macro", "meta") — Scoped when
// the symbol's parent is a type (a manifest's "<kind>@scoped" oracle key).
//
// src (nil: no source reads) supplies the subject's files for what the
// symbol string alone cannot say: a local symbol ("local N" — a function,
// class, type or interface declared inside another definition) is named by
// the identifier at its own occurrence range, admitted only when its
// documentation names a declaration keyword other than a variable's or a
// parameter's; and a symbol with more than one definition occurrence
// (TypeScript declaration merging: `interface X` and `const X`) takes each
// occurrence's kind from the declaration keyword written before its name
// on that line, since its one SymbolInformation describes only one of them
// — and so does every definition whose line writes such a keyword before
// its name (scip-typescript's hover for a merged `const X` reads
// "interface X"): the written keyword outranks the hover text.
// Accessor and constructor names become the names languages give them:
// `<get>x`/`<set>x` -> x, `<constructor>` -> "constructor", JVM `<init>` ->
// the enclosing type's name. Sorted by path, line, name.
func SCIPDefs(raw []byte, src func(path string) []byte) (defs []OracleDef, tool, version string, err error) {
	idx := &scippb.Index{}
	if err := proto.Unmarshal(raw, idx); err != nil {
		return nil, "", "", fmt.Errorf("scip: decode index: %w", err)
	}
	if ti := idx.GetMetadata().GetToolInfo(); ti != nil {
		tool, version = ti.GetName(), ti.GetVersion()
	}
	defCount := map[string]int{}
	for _, d := range idx.GetDocuments() {
		for _, o := range d.GetOccurrences() {
			if o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) != 0 {
				defCount[o.GetSymbol()]++
			}
		}
	}
	// rust-analyzer expands macros and names every module by its file, which
	// its definitions show two ways no source-level extractor can match; both
	// are read from the subject's own source (see rustDeclSite, fileModules).
	isRA := tool == "rust-analyzer"
	// scip-dart's SymbolInformation.kind is authoritative (a `get`, `const` or
	// `operator` written before a name is not a different kind), its private
	// members are local symbols, and a constructor or accessor is named as the
	// dialect names it (dartName).
	isDart := tool == "scip-dart"
	docLines := map[string][][]byte{}
	linesOf := func(path string) [][]byte {
		if src == nil {
			return nil
		}
		l, ok := docLines[path]
		if !ok {
			l = bytes.Split(src(path), []byte("\n"))
			docLines[path] = l
		}
		return l
	}
	fileModules := map[string]bool{} // module symbols whose one definition is their whole file
	for _, d := range idx.GetDocuments() {
		info := map[string]*scippb.SymbolInformation{}
		for _, si := range d.GetSymbols() {
			info[si.GetSymbol()] = si
		}
		lines := linesOf(d.GetRelativePath())
		lineText := func(l int) string {
			if l < 0 || l >= len(lines) {
				return ""
			}
			return string(lines[l])
		}
		for _, o := range d.GetOccurrences() {
			if o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) == 0 {
				continue
			}
			sym := o.GetSymbol()
			r, ok := rangeOf(o.GetRange())
			if !ok {
				continue
			}
			if r[0] != r[2] {
				// A name is written on one line. A multi-line range is a
				// whole-file module occurrence (rust-analyzer's, for the
				// `crate/` root and for every file module `mod x;` names),
				// not a definition anyone wrote: none is emitted here. A file
				// module's declaration is recovered below from its reference.
				if isRA && strings.HasSuffix(sym, "/") {
					fileModules[sym] = true
				}
				continue
			}
			docKind := ""
			si := info[sym]
			if si != nil {
				if k := si.GetKind(); k != scippb.SymbolInformation_UnspecifiedKind {
					docKind = strings.ToLower(k.String())
				} else {
					for _, doc := range si.GetDocumentation() {
						if k := docKeyword(doc); k != "" {
							docKind = k
							break
						}
					}
				}
			}
			name, _ := scipBareName(sym)
			kind, scoped := scipDescriptorKind(sym)
			if strings.HasPrefix(sym, "local ") {
				switch docKind {
				case "", "var", "let", "const", "parameter", "property", "field":
					// scip-dart writes every private member (a leading `_`) as a
					// local symbol whose kind still says what it is: a private field,
					// accessor or variable is a definition, a local variable (kind
					// unspecified) or a parameter is not.
					if !(isDart && (docKind == "property" || docKind == "field")) {
						continue
					}
				case "namespace", "typeparameter":
					if isDart {
						continue
					}
				}
				if t := lineText(int(r[0])); r[0] == r[2] && int(r[1]) < int(r[3]) && int(r[3]) <= len(t) {
					name, kind = t[r[1]:r[3]], "function"
				} else {
					continue
				}
			}
			if name == "" {
				continue
			}
			if docKind != "" {
				kind = docKind
			}
			if src != nil && !isDart {
				if t := lineText(int(r[0])); int(r[1]) <= len(t) {
					if k := keywordBefore(t[:r[1]]); k != "" {
						kind = k
					}
				}
			}
			switch {
			case name == "<constructor>":
				name = "constructor"
			case name == "<init>":
				if parent := strings.TrimSuffix(sym[:strings.LastIndex(sym, "`<init>`")], "#"); parent != "" {
					name, _ = scipBareName(parent + "#")
				}
			case strings.HasPrefix(name, "<get>"), strings.HasPrefix(name, "<set>"):
				name = name[len("<get>"):]
			}
			def := OracleDef{Path: d.GetRelativePath(), Name: name, Line: int(r[0]) + 1, Kind: kind, Scoped: scoped}
			if enc, ok := rangeOf(o.GetEnclosingRange()); ok {
				def.End = int(enc[2]) + 1
			}
			if isRA && src != nil && rustDeclKinds[kind] {
				if t := lineText(int(r[0])); int(r[1]) <= len(t) && !rustDeclSite(t[:r[1]]) {
					def.Generated = true
				}
			}
			defs = append(defs, def)
		}
	}
	if len(fileModules) > 0 && src != nil {
		// A file module (`mod x;`) is defined by its whole file; the
		// declaration a reader sees is the reference written after `mod`.
		for _, d := range idx.GetDocuments() {
			lines := linesOf(d.GetRelativePath())
			for _, o := range d.GetOccurrences() {
				if o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) != 0 || !fileModules[o.GetSymbol()] {
					continue
				}
				r, ok := rangeOf(o.GetRange())
				if !ok || r[0] != r[2] || int(r[0]) >= len(lines) || int(r[1]) > len(lines[r[0]]) {
					continue
				}
				if !rustModRE.Match(lines[r[0]][:r[1]]) {
					continue
				}
				name, _ := scipBareName(o.GetSymbol())
				defs = append(defs, OracleDef{Path: d.GetRelativePath(), Name: name, Line: int(r[0]) + 1, End: int(r[0]) + 1, Kind: "module"})
			}
		}
	}
	sort.SliceStable(defs, func(i, j int) bool {
		a, b := defs[i], defs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	return defs, tool, version, nil
}

// rustDeclKinds are the kinds whose definition must be written where the
// producer reports it (fields, variants and locals are not checked).
var rustDeclKinds = map[string]bool{
	"function": true, "method": true, "staticmethod": true, "traitmethod": true, "struct": true,
	"enum": true, "trait": true, "constant": true, "const": true, "staticvariable": true,
	"typealias": true, "type": true, "macro": true, "module": true,
}

var (
	rustDeclRE = regexp.MustCompile(`\b(?:fn|struct|enum|union|trait|type|const|static|static\s+mut|mod|macro_rules!|macro)\s+(?:r#)?$`)
	rustModRE  = regexp.MustCompile(`\bmod\s+(?:r#)?$`)
)

// rustDeclSite reports whether prefix — the text before a reported
// definition's name on its line — ends in a Rust declaration keyword. A
// definition rust-analyzer reports with none was generated by a macro's
// expansion (globset's `syntax!(literal1, ...)` writes no `fn literal1`); it
// exists only after expansion, so no syntactic extractor can see it.
func rustDeclSite(prefix string) bool { return rustDeclRE.MatchString(prefix) }

var wordRE = regexp.MustCompile(`[A-Za-z_]+`)

// keywordBefore is the declaration keyword closest before a name on its
// line ("export const " -> const), skipping modifiers; "" when the nearest
// non-modifier word is no declaration keyword.
func keywordBefore(prefix string) string {
	words := wordRE.FindAllString(prefix, -1)
	for i := len(words) - 1; i >= 0; i-- {
		w := words[i]
		if docModifiers[w] {
			continue
		}
		if docKeywords[w] {
			return w
		}
		return ""
	}
	return ""
}
