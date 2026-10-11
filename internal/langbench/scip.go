package langbench

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
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
	// CallInIndex marks the truth pairs with an occurrence whose symbol the
	// index itself defines (a local, or a definition in one of its
	// documents): the others call a standard-library or dependency symbol
	// no extractor of the subject has a node for (Go's L2, language-breadth
	// §1.6, grades the calls a service can serve).
	CallInIndex map[CallPair]bool
	// CallFound are fathom's pairs as scored: FathomCallPairs over Docs, less
	// those the compiler shows have no callee (SCIPOptions.ClangSites).
	CallFound map[CallPair]bool
	// ImportRefs are the index's cross-file references, one per (document,
	// line, defining document), sorted — the oracle side of
	// import_resolution (LB-F40, ScoreImports), which keeps those on a
	// fathom import statement's lines. Unlike FileEdges any non-local
	// definition is a target, a module or file symbol included (the one a
	// `mod x;`, a `use` path or an import URI references); Explicit marks
	// a reference the producer gave the Import role.
	ImportRefs []OracleImport
	// ImportEdges are the typed edges the index's import stores between
	// mapped definitions (SCIPOptions.KeepImportEdges; core.EdgeDirect only):
	// the pairs Go's native typed pass is graded against (LB-F300).
	ImportEdges []core.Edge
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
	case fscip.GoScopedTerm(s):
		return "", false // scip-go's function-scoped term (`Options:c.`) is a parameter or a local
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
			name = javaName(sym, name, s[:i])   // scip-java: `<init>` is its class
			name = csharpName(sym, name, s[:i]) // scip-dotnet: `.ctor` is its class (scip_csharp.go)
			return name, callable
		}
	}
	i := strings.LastIndexAny(s, descriptorSeps(sym))
	return csharpName(sym, leafName(s[i+1:]), s[:i+1]), callable
}

// isSCIPDart reports whether sym is one scip-dart wrote.
func isSCIPDart(sym string) bool { return strings.HasPrefix(sym, "scip-dart ") }

// isRustAnalyzer reports whether sym is one rust-analyzer wrote.
func isRustAnalyzer(sym string) bool { return strings.HasPrefix(sym, "rust-analyzer ") }

// descriptorSeps is the set of characters that end a descriptor before a
// symbol's leaf. "]" ends one too for rust-analyzer, which writes an impl
// block's members as `impl#[Type]name().` (`[Type]` a descriptor of its own),
// and for scip-dart, whose reading was taken under that rule; for every other
// producer the set is the one their readings were measured with (integration
// review, 2026-09-29).
func descriptorSeps(sym string) string {
	if isRustAnalyzer(sym) || isSCIPDart(sym) {
		return "/#.:] "
	}
	return "/#.: "
}

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
	// KeepImportEdges keeps the import's direct edges in SCIPResult.ImportEdges
	// (Go: scip_go.go, LB-F300).
	KeepImportEdges bool
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
	// CallSyntax: a callable reference is a call site only where its name is
	// written followed by `(` (needs Source), the Dart standard (LB-F65):
	// scip-python references a function passed as a value and a decorator
	// applied by name; neither is a call site, and both are counted
	// (CntScipCallsUnwritten) and disclosed.
	CallSyntax bool
	// CalledNames: a reference to a class (`Foo#`), a term (`x.`: a field, a
	// variable, an interface method, which scip-go writes as a term) or a
	// local (named by what is written) whose name is written followed by `(`
	// is a call to that name (needs Source; LB-F65). Python constructs by
	// calling the class and scip-python names the class there; a call
	// through a Go interface or a func-typed field references a term. Each is
	// counted (CntScipCalledNames) and disclosed.
	CalledNames bool
	// ClangMacroCalls: a reference to a macro symbol is a call to the name
	// written there when it is followed by `(` (needs Source, LB-F80): scip-clang
	// names a macro by its location, and C's extractor records `ASSERT_OK(x)` as
	// a call (C++'s excludes an all-capitals invocation).
	ClangMacroCalls bool
	// ClangMacroArgs: a callable reference at a macro invocation's name whose
	// callee is written as a call later on that line is a call written inside the
	// macro's argument (`ASSERT_OK(uv_ip4_addr(..))`), which scip-clang references
	// at the macro's own range (needs Source, LB-F80).
	ClangMacroArgs bool
	// ClangSites (LB-F116) are the build's own compiler's call sites per
	// document (calibration/oracles/clang-calls, tier A; needs Source): a
	// fathom pair whose every written site has no callee before
	// instantiation is left out of both counts; a call the compiler resolves
	// is in the truth whether or not the index references it (a constructor
	// call written `T(` scored against the index's class reference there); a
	// callable reference the index writes where the compiler declares the
	// function (`int close(int fd);`) is no call; a definition a macro writes
	// is named by what is written at its range; and a fathom definition in
	// code the index holds no occurrence in is scored against the compiler's
	// sites alone. Each is counted (CntClang*, CntScipDefsMacroNamed) and
	// disclosed.
	ClangSites map[string][]ClangSite
	// ActiveLines are the lines the build's own preprocessor keeps per
	// document, as [first, last] ranges (the clang-preprocessor oracle's
	// active_lines). With ClangSites, a fathom pair written only on lines
	// the build discards (a `#ifdef _WIN32` branch inside a function) is in
	// neither the index nor the compiler, and is left out of both counts
	// (CntCallsInactive), as a definition on such a line is.
	ActiveLines map[string][][2]int
	// TypeScriptNames: scip-typescript's vocabulary (LB-F174, O440). A
	// constructor or accessor definition takes the name the language gives
	// it (`<constructor>` is "constructor", `<get>x` is x), as SCIPDefs
	// names them for L1; a constructor reference is a call to the class
	// written at its range (`new Foo(` calls Foo); with CalledNames a
	// non-callable reference (a term, a parameter, a local, an object
	// literal's property) is named by what is written there; a call may be
	// written `name!(`, `name?.(` or as a tagged template; and columns are
	// UTF-16 code units, read against the source's bytes (needs Source).
	TypeScriptNames bool
	// OracleBlindSites: a fathom pair the truth lacks whose every written call
	// site (`name(` inside the caller's lines) has no occurrence of the index
	// at the callee's range is left out of both counts and counted
	// (CntCallsOracleBlind; needs Source): the producer writes no occurrence
	// where it resolves no symbol (scip-typescript at `super(...)` and at a
	// member of an untyped receiver), so the call is one it cannot judge,
	// LB-F116's compiler-unjudged rule with the index as its own witness
	// (design §1.4 amendment 2026-10-02). A pair with no written site found
	// stays.
	OracleBlindSites bool
	// JSXElementCalls: a JSX element is a call (owner ruling M2, O860, K19;
	// scip_ts.go): in a document TypeScript parses with JSX (.tsx, .jsx, .js,
	// .mjs, .cjs), a reference at an opening or self-closing element's tag that
	// names a component (`<Foo/>`; the last segment of `<Foo.Bar>`) is a call to
	// the name written there, counted (CntScipJSXElementCalls), and the element is
	// a written call site for OracleBlindSites; an intrinsic tag (`<div>`), a
	// closing tag and a fragment are none (needs Source and TypeScriptNames).
	JSXElementCalls bool
	// RustMacroDefs: a definition rust-analyzer finds in a macro's expansion, written by
	// no source line, is counted apart and left out of def_mapping, and the references
	// inside it out of typed_recall (needs Source; scip_rust.go, LB-F249).
	RustMacroDefs bool
	// RustNamespaces: a rust-analyzer module (a namespace descriptor: a file module, a
	// crate root, an inline `mod x { }`) is file granularity, never a fathom symbol (K18):
	// its definitions leave def_mapping and the references to one or made at its top level
	// leave typed_recall, each counted (scip_rust.go, LB-F389).
	RustNamespaces bool
	// CSharpCalls: scip-dotnet's references read at their written site (needs
	// Source; scip_csharp.go, LB-F286): a constructor reference not written
	// `new X(` (an attribute applied) is no call site, and a type reference
	// written `new X(` is a call to X; each counted and disclosed.
	CSharpCalls bool
	// CSharp is the compiler's semantic model beside a scip-dotnet index
	// (calibration/oracles/roslyn calls mode, tier A; scip_csharp.go,
	// LB-F287): a call it binds is in the truth with or without an index
	// reference, and a fathom pair whose every written site it binds to
	// nothing or no compiled configuration keeps is left out, counted.
	CSharp *CSharpCompiler
	// ClangDefNames: def_mapping names a scip-clang definition as SCIPDefs names it
	// for L1 (needs Source; scip_clang.go clangMappedName, LB-F292): a macro, whose
	// symbol is its location, by the name written at its range; a file's own
	// symbol (`<file>/src/a.c`/) is no definition and is counted apart.
	ClangDefNames bool
	// ClangScope, with ClangDefNames, is the documents the reading measures
	// (lang-oracle's -only and discovery's exclusions, LB-F292): def_mapping
	// counts the definitions in them and typed_recall the references in them,
	// to definitions in them; the rest are counted apart (scip_defs_out_of_scope,
	// scip_edges_out_of_scope), as the file-edge truth is scoped (§1.4). nil
	// scopes nothing.
	ClangScope map[string]bool
	// DefScoped: def_mapping reads only the definitions in DefScope's documents
	// (TypeScript and JavaScript; owner ruling L5, K19, LB-F467). A definition
	// in any other document of the index (a TypeScript declaration file beside
	// JavaScript, a file -only leaves out, one ingest's gates skip) is left out
	// of def_mapping's denominator and counted (scip_defs_out_of_scope): the
	// out-of-scope principle ClangScope (LB-F292) and the oracle side's file
	// scoping (§1.4) already apply. typed_recall, the call truth and every other
	// family read every document as before.
	DefScoped bool
	// DefScope is the documents the reading measures (lang-oracle's inScope:
	// the dialect's files, a variant's included, inside -only, that ingest's
	// gates keep), handed over when DefScoped; nil scopes nothing.
	DefScope map[string]bool
	// FileEdgesOracleBlind: lang-oracle leaves out of the file-edge comparison
	// a pair the truth lacks whose every written site has no occurrence of the
	// index (needs Source; scip_clang.go FileEdgesOracleBlind, LB-F296),
	// counted under LB-F203's cap (merge.go).
	FileEdgesOracleBlind bool
}

// SCIPOptionsFor is the option set for a dialect's SCIP producer.
func SCIPOptionsFor(dialect string, src func(path string) []byte) SCIPOptions {
	if dialect == "rust" {
		return SCIPOptions{Source: src, MacroCalls: true, WrittenCallsOnly: src != nil, RustMacroDefs: src != nil, RustNamespaces: true}
	}
	if dialect == "dart" {
		return SCIPOptions{Source: src, PrivateMembers: src != nil}
	}
	if dialect == "typescript" || dialect == "javascript" {
		// scip-typescript: call sites as written (LB-F174), the calls it
		// writes no occurrence for left out as unjudged (LB-F175), and
		// def_mapping over the documents the reading measures (ruling L5).
		return SCIPOptions{Source: src, CallSyntax: src != nil, CalledNames: src != nil, TypeScriptNames: true, OracleBlindSites: src != nil, DefScoped: true, JSXElementCalls: src != nil}
	}
	if dialect == "python" {
		return SCIPOptions{Source: src, CallSyntax: src != nil, CalledNames: src != nil}
	}
	if dialect == "ruby" {
		return rubySCIPOptions(src) // scip_ruby.go: the calls Sorbet writes no occurrence for (LB-F257)
	}
	if dialect == "c" || dialect == "cpp" {
		// Call sites written in the source: scip-clang references a function
		// passed as a value, and a call a macro expands to at the macro's own
		// name; neither is written as a call. C records a function-like macro
		// invocation as a call, C++ does not.
		return SCIPOptions{Source: src, CallSyntax: src != nil, ClangMacroArgs: src != nil, ClangMacroCalls: src != nil && dialect == "c", ClangDefNames: src != nil, FileEdgesOracleBlind: src != nil}
	}
	if dialect == "go" {
		// Go keeps every callable reference: its extractor records a
		// function value as a call (VS-6).
		return SCIPOptions{Source: src, CalledNames: src != nil, KeepImportEdges: true}
	}
	if dialect == "csharp" {
		return csharpSCIPOptions(src) // scip_csharp.go: written call sites (LB-F286), the oracle-blind rule (LB-F175, O464)
	}
	if dialect == "php" {
		// The calls the index writes no occurrence for are left out as
		// unjudged (LB-F175, O464): scip-php at a member of a receiver it
		// cannot type. Disclosure only while the reading is over the cap
		// (LB-F203).
		return SCIPOptions{Source: src, OracleBlindSites: src != nil}
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
	var gerr error
	if idx, raw, gerr = goWithoutOutsideDocs(idx, raw, res.Counts); gerr != nil { // scip_go.go (LB-F299)
		return SCIPResult{}, gerr
	}
	var sites clangSiteIndex
	srcCache := map[string][][]byte{}
	srcOf := func(doc string) [][]byte {
		if ls, ok := srcCache[doc]; ok {
			return ls
		}
		var ls [][]byte
		if opts.Source != nil {
			ls = bytes.Split(opts.Source(doc), []byte("\n"))
		}
		srcCache[doc] = ls
		return ls
	}
	occLines := map[string]map[int32]bool{}         // lines holding an occurrence but a macro reference (clangBlindDefs)
	occStarts := map[string]map[[2]int32][]string{} // symbols referenced at each (line, column) start
	if opts.ClangSites != nil && opts.Source != nil {
		sites = newClangSiteIndex(opts.ClangSites)
		for _, d := range idx.GetDocuments() {
			doc := d.GetRelativePath()
			if occLines[doc] == nil {
				occLines[doc], occStarts[doc] = map[int32]bool{}, map[[2]int32][]string{}
			}
			for _, o := range d.GetOccurrences() {
				r, ok := rangeOf(o.GetRange())
				if !ok {
					continue
				}
				isDef := o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) != 0
				if !isDef {
					occStarts[doc][[2]int32{r[0], r[1]}] = append(occStarts[doc][[2]int32{r[0], r[1]}], o.GetSymbol())
				}
				if isDef || !strings.HasSuffix(o.GetSymbol(), "!") {
					for l := r[0]; l <= r[2]; l++ {
						occLines[doc][l] = true
					}
				}
			}
		}
	}

	type fk struct{ path, name string }
	fathomAt := map[fk][][2]int{} // (line, end line) per same-named definition
	for _, n := range nodes {
		k := fk{n.FilePath, BareName(n.Name)}
		fathomAt[k] = append(fathomAt[k], [2]int{n.Line, n.EndLine})
	}
	docKinds := map[string]map[string]scippb.SymbolInformation_Kind{} // scip-dart: each document's symbol kinds
	definedIn := map[string][]string{}                                // symbol -> every defining document, in index order (K25 ruling 3)
	defDoc := map[string]string{}                                     // every non-local symbol -> its defining document (ImportRefs); "" when more than one
	defsByDoc := map[string][]*scipDef{}
	macroNamed := map[fk]bool{} // fathom definitions a macro-written definition is named after (LB-F116)
	docs := map[string]bool{}
	goDefs := newGoDefClasses(res.Tool)
	for _, d := range idx.GetDocuments() {
		docs[d.GetRelativePath()] = true
		goDefs.document(d)
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
		var rustKinds map[string]scippb.SymbolInformation_Kind
		if opts.RustMacroDefs && opts.Source != nil {
			rustKinds = rustDocKinds(d)
		}
		javaLocals := javaLocalMethods(res.Tool, d) // scip-java: an anonymous or local class's methods (O458)
		for _, o := range d.GetOccurrences() {
			if o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) == 0 {
				continue
			}
			if sym := o.GetSymbol(); sym != "" && !strings.HasPrefix(sym, "local ") {
				if prev, seen := defDoc[sym]; !seen {
					defDoc[sym] = d.GetRelativePath()
				} else if prev != d.GetRelativePath() {
					// One symbol defined in two documents (scip-typescript names
					// every sample app's src/app.module.ts alike): which file a
					// reference means is the index's guess, never import truth.
					defDoc[sym] = ""
				}
			}
			if kinds[o.GetSymbol()] == scippb.SymbolInformation_Namespace {
				continue
			}
			name, _ := scipBareName(o.GetSymbol())
			if fscip.ModuleDefinition(o.GetSymbol()) {
				name = core.ModuleSymbolName // K18: the file's module symbol, never a `__init__` method
			}
			if opts.TypeScriptNames {
				name = tsDefName(name)
			}
			r, ok := rangeOf(o.GetRange())
			if name == "" && ok && lines != nil && strings.HasPrefix(o.GetSymbol(), "local ") && dartPrivateKind(kinds[o.GetSymbol()]) &&
				r[0] == r[2] && int(r[0]) < len(lines) && int(r[3]) <= len(lines[r[0]]) {
				name = string(lines[r[0]][r[1]:r[3]])
			}
			apart := "" // LB-F292: a file's own symbol or a nameless one is no definition def_mapping counts
			if opts.ClangDefNames && ok {
				name, apart = clangMappedName(name, o.GetSymbol(), srcOf(d.GetRelativePath()), r)
				if opts.ClangScope != nil && !opts.ClangScope[d.GetRelativePath()] {
					apart = CntScipDefsOutOfScope // LB-F292
				}
			}
			javaLocal := false
			if name == "" && ok && javaLocals != nil {
				name = javaLocalMethodName(javaLocals, o, r)
				javaLocal = name != ""
			}
			if name == "" || !ok {
				continue
			}
			if in := definedIn[o.GetSymbol()]; !javaLocal && !slices.Contains(in, d.GetRelativePath()) {
				definedIn[o.GetSymbol()] = append(in, d.GetRelativePath())
			}
			def := &scipDef{doc: d.GetRelativePath(), line: int(r[0]) + 1, name: name}
			if enc, ok := rangeOf(o.GetEnclosingRange()); ok {
				def.enc, def.hasEnc = enc, true
			}
			generated := rustKinds != nil && rustGeneratedDef(rustKinds[o.GetSymbol()], srcOf(def.doc), r) // LB-F249
			namespace := opts.RustNamespaces && rustNamespace(o.GetSymbol())                               // K18 (LB-F389)
			objectLiteral := opts.TypeScriptNames && tsObjectLiteralProperty(o.GetSymbol())                // D2 (LB-F413)
			outOfScope := scipDefOutOfScope(opts, def.doc)                                                 // ruling L5 (LB-F467)
			if opts.ClangDefNames {
				// LB-F293: one the dialect names anyway (tree-sitter's recovery reads
				// curl's `CURLOPT(CURLOPT_URL, ...)` as the enumerator CURLOPT_URL) is
				// counted as any other definition
				generated = generated || clangGeneratedDef(name, srcOf(def.doc), r) && !clangNamedNear(fathomAt[fk{def.doc, name}], def.line)
			}
			if generated {
				res.Counts[CntScipDefsMacroGenerated]++
			} else if namespace {
				res.Counts[CntScipDefsNamespace]++
			} else if objectLiteral {
				res.Counts[CntScipDefsObjectLiteral]++
			} else if outOfScope {
				res.Counts[CntScipDefsOutOfScope]++
			} else if apart != "" {
				res.Counts[apart]++
			} else if !javaLocal {
				res.Counts[CntScipDefs]++
			}
			mapTo := func(as string, disclose bool) (int, bool) {
				for _, span := range fathomAt[fk{def.doc, as}] {
					if d := span[0] - def.line; d >= -2 && d <= 2 {
						def.fathomDef = as
						if !def.hasEnc && span[1] >= span[0] && span[1] > 0 {
							// D5: no enclosing_range from the producer — the
							// mapped definition's own lines stand in (0-based,
							// whole lines).
							lo := min(int32(span[0]-1), r[0])
							def.enc, def.hasEnc = [4]int32{lo, 0, int32(span[1] - 1), 1 << 30}, true
							if disclose {
								res.EnclosingFallback++
							}
						}
						return span[0], true
					}
				}
				return 0, false
			}
			if javaLocal {
				// O458: a caller of the call truth only when the extractor
				// defines the same-named method inside it; otherwise its calls
				// stay the enclosing named method's, as before.
				if line, ok := mapTo(name, false); ok && line >= int(def.enc[0])+1 && line <= int(def.enc[2])+1 {
					res.Counts[CntScipJavaLocalMethods]++
					defsByDoc[def.doc] = append(defsByDoc[def.doc], def)
				}
				continue
			}
			_, mappedOK := mapTo(name, true)
			goDefs.count(o.GetSymbol(), mappedOK) // scip_go.go: Go's def_mapping misses by descriptor class (LB-F299)
			if mappedOK {
				if !generated && !namespace && !objectLiteral && !outOfScope {
					res.Counts[CntScipDefsMapped]++
				}
			} else if sites != nil && strings.HasSuffix(o.GetSymbol(), ").") {
				// LB-F116: a callable a macro writes (gtest's TEST(suite, name)
				// is suite_name_Test::TestBody, reported over `TEST`) is named
				// by what is written at its range, as fathom's extractor names
				// it, so the calls in its body are measured. Not a mapping of
				// def_mapping's: the index's own name is not fathom's.
				if ls := srcOf(def.doc); r[0] == r[2] && r[1] >= 0 && r[1] <= r[3] && int(r[0]) < len(ls) && int(r[3]) <= len(ls[r[0]]) {
					if written := string(ls[r[0]][r[1]:r[3]]); clangMacroGenerated(name, written) && dartCallAt(ls, r, written) {
						if line, ok := mapTo(written, false); ok { // counted as macro-named, not as D5
							macroNamed[fk{def.doc, written + ":" + strconv.Itoa(line)}] = true
						}
					}
				}
			}
			defsByDoc[def.doc] = append(defsByDoc[def.doc], def)
		}
	}

	if len(macroNamed) > 0 {
		res.Counts[CntScipDefsMacroNamed] = len(macroNamed)
	}
	if opts.CSharpCalls {
		// LB-F287: a fathom definition the index defines nothing for stands in.
		for _, def := range csharpStandIns(nodes, defsByDoc, docs) {
			defsByDoc[def.doc] = append(defsByDoc[def.doc], def)
			res.Counts[CntCSharpStandInDefs]++
		}
	}
	if sites != nil {
		if blind := clangBlindDefs(nodes, occLines, sites); len(blind) > 0 {
			for _, def := range blind {
				defsByDoc[def.doc] = append(defsByDoc[def.doc], def)
			}
			res.Counts[CntClangBlindDefs] += len(blind)
		}
	}

	truth := map[CallPair]bool{}
	multiDefined := map[[2]string]bool{} // C#: edges only a symbol two documents define draws (LB-F287)
	blind := oracleSites{}
	inIndex := map[CallPair]bool{}
	importRefs := map[OracleImport]bool{} // (doc, line, to) -> any occurrence there has the Import role
	multiDef := newGoMultiDefRefs(res.Tool)
	everyDefiner := definerEdges{}
	for _, d := range idx.GetDocuments() {
		doc := d.GetRelativePath()
		var srcLines [][]byte
		if (opts.WrittenCallsOnly || opts.PrivateMembers || opts.CallSyntax || opts.CalledNames || opts.ClangMacroCalls || opts.ClangMacroArgs || opts.TypeScriptNames || opts.OracleBlindSites || opts.CSharpCalls) && opts.Source != nil {
			srcLines = bytes.Split(opts.Source(doc), []byte("\n"))
		}
		checkWritten := opts.WrittenCallsOnly || opts.PrivateMembers || opts.CallSyntax
		callAt := dartCallAt
		if opts.TypeScriptNames {
			callAt = tsCallAt
		}
		for _, o := range d.GetOccurrences() {
			if opts.OracleBlindSites && srcLines != nil {
				// LB-F175: where the index writes any occurrence at all.
				if r, ok := rangeOf(o.GetRange()); ok {
					r = utf16Range(srcLines, r)
					blind.mark(doc, r)
				}
			}
			if o.GetSymbolRoles()&int32(scippb.SymbolRole_Definition) != 0 {
				continue
			}
			r, ok := rangeOf(o.GetRange())
			if !ok {
				continue
			}
			if opts.TypeScriptNames && srcLines != nil {
				r = utf16Range(srcLines, r) // scip-typescript counts UTF-16 code units
			}
			if tos, ok := definedIn[o.GetSymbol()]; ok && !documentScopedLocal(res.Tool, o.GetSymbol()) {
				switch to := tos[0]; {
				case to == doc && (multiDef != nil || opts.CSharpCalls):
					// the producer rules below read the first definer alone, as before
				case multiDef.holds(defDoc, o.GetSymbol()):
					multiDef[[2]string{doc, to}] = true // scip_go.go: a package names no one file (LB-F298)
				case opts.CSharpCalls && defDoc[o.GetSymbol()] == "":
					multiDefined[[2]string{doc, to}] = true // LB-F287: no one file (scip_csharp.go)
				case clangNamespace(res.Tool, o.GetSymbol()):
					everyDefiner.leaveOut(doc, tos) // owner ruling L2 (O806): a namespace names no file (scip_clang.go)
				default:
					// K25 ruling 3: every defining document, but rust-analyzer's per-target
					// copies of one symbol (scip_rust.go)
					everyDefiner.add(doc, tos, rustDuplicateDefiners(o.GetSymbol(), doc, textAt(srcLines, r), tos), res.FileEdges)
				}
			}
			if to, ok := defDoc[o.GetSymbol()]; ok && to != doc {
				k := OracleImport{Path: doc, Line: int(r[0]) + 1, ToPath: to, Ambiguous: to == ""}
				importRefs[k] = importRefs[k] || o.GetSymbolRoles()&int32(scippb.SymbolRole_Import) != 0
			}
			callee, callable := scipBareName(o.GetSymbol())
			if opts.TypeScriptNames && callable {
				callee = tsCalleeName(callee, srcLines, r)
			}
			if opts.CSharpCalls && srcLines != nil {
				callee, callable = csharpCallee(o.GetSymbol(), callee, callable, srcLines, r, res.Counts) // LB-F286
			}
			if opts.MacroCalls && strings.HasSuffix(o.GetSymbol(), "!") {
				callable = true
			}
			if opts.ClangMacroCalls && strings.HasSuffix(o.GetSymbol(), "!") {
				// scip-clang's macro symbol is its location, never its name: a
				// function-like macro invoked as `NAME(` is a call to what is
				// written; an object-like macro is no call (and no occurrence
				// is counted for it).
				name, ok := clangMacroCall(srcLines, r)
				if !ok {
					continue
				}
				callee, callable = name, true
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
			if opts.CalledNames && srcLines != nil && !callable {
				// A class, a term or a local written as a call (LB-F65).
				name := callee
				if (opts.TypeScriptNames || strings.HasPrefix(o.GetSymbol(), "local ")) && r[0] == r[2] && int(r[0]) < len(srcLines) && r[1] >= 0 && r[1] <= r[3] && int(r[3]) <= len(srcLines[r[0]]) {
					name = string(srcLines[r[0]][r[1]:r[3]])
				}
				if name != "" && callAt(srcLines, r, name) {
					callee, callable = name, true
					res.Counts[CntScipCalledNames]++
				}
			}
			jsx := false // owner ruling M2: a JSX element's tag is the call site (scip_ts.go)
			if opts.JSXElementCalls && srcLines != nil && tsJSXVariant(doc) {
				if name, ok := tsJSXElementAt(srcLines, r); ok {
					callee, callable, jsx = name, true, true
					res.Counts[CntScipJSXElementCalls]++
				}
			}
			if !callable || callee == "" {
				continue
			}
			direct := false // written as a call at the reference's own range
			if checkWritten && srcLines != nil {
				written := writtenAt(srcLines, r, callee)
				if opts.PrivateMembers || opts.CallSyntax {
					// Dart: a call is the name followed by its arguments; the
					// producer also references a tear-off (`onError: c.addError`), an
					// operator and the `super`/`this` of an initializer.
					written = callAt(srcLines, r, callee)
				}
				direct = written
				if !written && opts.ClangMacroArgs {
					written = clangMacroArgCall(srcLines, r, callee)
				}
				written = written || jsx
				if !written {
					res.Counts[CntScipCallsUnwritten]++
					continue
				}
			}
			inner := innermostDef(defsByDoc[doc], r)
			if inner == nil || inner.fathomDef == "" {
				continue
			}
			if direct && sites.covers(doc) {
				// LB-F116: written like a call, but the compiler declares the
				// function there (`int close(int fd);`).
				if c, ok := sites.class(doc, int(r[0])+1, int(r[1])+1, callee); ok && c == ClangDeclaration {
					res.Counts[CntClangNotCalls]++
					continue
				}
			}
			pair := CallPair{doc, inner.fathomDef, callee}
			truth[pair] = true
			if _, defined := defDoc[o.GetSymbol()]; defined || strings.HasPrefix(o.GetSymbol(), "local ") {
				inIndex[pair] = true
			}
		}
	}
	res.CallInIndex = inIndex
	everyDefiner.finish(res.FileEdges, res.Counts)
	goDefs.finish(res.Counts)
	multiDef.finish(res.FileEdges, res.Counts)
	for e := range multiDefined {
		if !res.FileEdges[e] {
			res.Counts[CntScipCSharpFileEdgesMultiDefined]++
		}
	}
	if opts.CSharp != nil {
		addCSharpFileEdges(res.FileEdges, opts.CSharp, docs, res.Counts) // LB-F287
	}
	found := FathomCallPairs(nodes, docs)
	if opts.CSharp != nil && opts.Source != nil {
		// LB-F287: the compiler judges first, so a call it binds is never
		// counted oracle-blind and one it cannot judge is counted as such.
		addCSharpTruth(truth, opts.CSharp, docs, defsByDoc, res.Counts)
		dropCSharpUnjudged(found, truth, nodes, srcOf, opts.CSharp, res.Counts)
	}
	if opts.OracleBlindSites && opts.Source != nil {
		blind.drop(found, truth, nodes, srcOf, opts.JSXElementCalls, res.Counts)
	}
	if sites != nil {
		addClangTruth(truth, opts.ClangSites, docs, defsByDoc, occStarts, res.Counts)
		dropClangUnjudged(found, truth, nodes, srcOf, sites, opts.ActiveLines, res.Counts)
	}
	ScorePairs(truth, found, res.Counts, CntCallTruth, CntCallTruthFound, CntCallFathom, CntCallFathomInTrue)
	res.CallTruth, res.CallFound, res.Docs = truth, found, docs
	for k, explicit := range importRefs {
		k.Explicit = explicit
		res.ImportRefs = append(res.ImportRefs, k)
	}
	sort.Slice(res.ImportRefs, func(i, j int) bool {
		a, b := res.ImportRefs[i], res.ImportRefs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.ToPath != b.ToPath {
			return a.ToPath < b.ToPath
		}
		return !a.Ambiguous && b.Ambiguous
	})

	imp, err := fscip.ImportIndex(service, raw, nodes)
	if err != nil {
		return SCIPResult{}, err
	}
	goSkipCounts(imp.Report.SkippedByClass, res.Counts) // scip_go.go (LB-F299)
	if opts.KeepImportEdges {
		res.ImportEdges = importedDirectEdges(imp) // scip_go.go: Go's typed_recall against the native pass (LB-F300)
	}
	res.Counts[CntScipEdgesTyped] = imp.Report.Direct
	res.Counts[CntScipEdges] = imp.Report.Direct + imp.Report.Skipped["unmapped-endpoint"]
	cutIdx := idx
	if opts.RustMacroDefs && opts.Source != nil {
		// LB-F249: the references written inside a macro-generated definition are no
		// edge fathom could attribute; leave them out of the denominator, counted.
		if cut, any := rustWithoutGenerated(idx, opts.Source); any {
			if err := rustRecount(service, cut, nodes, res.Counts, CntScipEdgesMacroGenerated); err != nil {
				return SCIPResult{}, err
			}
			cutIdx = cut
		}
	}
	if opts.RustNamespaces {
		// K18 (LB-F389): a reference to a module, then one made at a module's top level.
		if cut, any := rustWithoutNamespaces(cutIdx, false); any {
			if err := rustRecount(service, cut, nodes, res.Counts, CntScipEdgesNamespaceTarget); err != nil {
				return SCIPResult{}, err
			}
			cutIdx = cut
		}
		if cut, any := rustWithoutNamespaces(cutIdx, true); any {
			if err := rustRecount(service, cut, nodes, res.Counts, CntScipEdgesNamespaceFrom); err != nil {
				return SCIPResult{}, err
			}
		}
	}
	if opts.ClangDefNames && opts.Source != nil {
		// LB-F293, LB-F292: typed_recall over the documents the reading
		// measures, and none of the references written inside a definition a
		// macro writes (scip_clang.go clangTyped).
		if err := clangTyped(idx, service, nodes, opts, res.Counts); err != nil {
			return SCIPResult{}, err
		}
	}
	if n := imp.Report.Skipped["parameter-target"]; n > 0 {
		res.Counts[CntScipEdgesParamTarget] = n // LB-F239: disclosed, out of scip_edges
	}
	return res, nil
}

// CntScipFileEdgesEveryDefiner counts the file-edge truth pairs only a second or later
// defining document of a symbol draws (K25 ruling 3, O539, LB-F387): a symbol the index
// defines in more than one document resolves to every one of them, never to the first
// alone. Before the rule the harness took the first document as the definer, so a
// reference to a symbol two documents define was an edge to whichever the index listed
// first: rust-analyzer names two test targets' `utils` modules (clap's tests/builder and
// tests/derive) by one symbol, and a reference in tests/builder/*.rs was an edge to
// tests/derive/utils.rs. Language-generic; scip-go's package (LB-F298) and C#'s
// multiply-defined symbol (LB-F287) keep their own rules, which draw no edge for one.
const CntScipFileEdgesEveryDefiner = "scip_file_edges_every_definer"

// definerEdges collects the file edges of K25 ruling 3: first holds the pairs the first
// defining document draws (the rule before it), later those a second or later one does,
// narrowed the pairs every definer would draw that a producer's rule left out
// (rustDuplicateDefiners), and namespace those a scip-clang namespace would draw
// (clangNamespace, owner ruling L2).
type definerEdges struct{ first, later, narrowed, namespace map[[2]string]bool }

func (d *definerEdges) init() {
	if d.first == nil {
		d.first, d.later, d.narrowed, d.namespace = map[[2]string]bool{}, map[[2]string]bool{}, map[[2]string]bool{}, map[[2]string]bool{}
	}
}

// add draws doc's reference to a symbol defined in tos (index order) to every defining
// document in kept (nil: all of tos) but doc itself.
func (d *definerEdges) add(doc string, tos, kept []string, edges map[[2]string]bool) {
	d.init()
	for i, to := range tos {
		if to == doc {
			continue
		}
		e := [2]string{doc, to}
		if kept != nil && !slices.Contains(kept, to) {
			d.narrowed[e] = true
			continue
		}
		edges[e] = true
		if i == 0 {
			d.first[e] = true
		} else {
			d.later[e] = true
		}
	}
}

// leaveOut draws no edge for doc's reference to a symbol defined in tos, and records the
// pairs every definer would have drawn (a scip-clang namespace, clangNamespace).
func (d *definerEdges) leaveOut(doc string, tos []string) {
	d.init()
	for _, to := range tos {
		if to != doc {
			d.namespace[[2]string{doc, to}] = true
		}
	}
}

// finish counts the pairs only a later definer drew, the pairs a producer's rule left out
// that nothing else drew, and the pairs a namespace would draw that nothing else drew.
func (d *definerEdges) finish(edges map[[2]string]bool, counts map[string]int) {
	n := 0
	for e := range d.later {
		if !d.first[e] {
			n++
		}
	}
	if n > 0 {
		counts[CntScipFileEdgesEveryDefiner] += n
	}
	n = 0
	for e := range d.narrowed {
		if !edges[e] {
			n++
		}
	}
	if n > 0 {
		counts[CntScipFileEdgesDuplicateNarrowed] += n
	}
	n = 0
	for e := range d.namespace {
		if !edges[e] {
			n++
		}
	}
	if n > 0 {
		counts[CntScipFileEdgesNamespace] += n
	}
}

// SCIPRuleNotes are a subject's disclosures of the counted SCIP rules this file and its
// producer files apply beside the metrics: K25 ruling 3 (EveryDefinerNote), for
// rust-analyzer K18's namespace rule (rustNamespaceNote), and for scip-typescript D2's
// object-literal rule (tsObjectLiteralNote).
func SCIPRuleNotes(name, tool string, counts map[string]int) []string {
	var out []string
	if n := EveryDefinerNote(name, counts); n != "" {
		out = append(out, n)
	}
	if tool == "rust-analyzer" {
		if n := rustNamespaceNote(name, counts); n != "" {
			out = append(out, n)
		}
	}
	if tool == "scip-typescript" {
		if n := tsObjectLiteralNote(name, counts); n != "" {
			out = append(out, n)
		}
		if n := tsJSXElementNote(name, counts); n != "" {
			out = append(out, n) // owner ruling M2 (LB-F481)
		}
	}
	return out
}

// EveryDefinerNote is the reading's disclosure of K25 ruling 3 for a subject, "" when the
// rule drew no pair the first definer did not and left none out.
func EveryDefinerNote(name string, counts map[string]int) string {
	n, cut, ns := counts[CntScipFileEdgesEveryDefiner], counts[CntScipFileEdgesDuplicateNarrowed], counts[CntScipFileEdgesNamespace]
	if n == 0 && cut == 0 && ns == 0 {
		return ""
	}
	note := fmt.Sprintf("%s: %d file-edge truth pairs are drawn to a second or later document defining one SCIP symbol (K25 ruling 3, LB-F387: a symbol defined in more than one document resolves to every one, never the first alone; %s)", name, n, CntScipFileEdgesEveryDefiner)
	if cut > 0 {
		note += fmt.Sprintf("; %d pairs every definer would add are left out for rust-analyzer's per-target copies of one symbol (%s: a crate named by its package's name is its library root, a reference inside a defining document is to its own copy, a module is the copy in the nearest directory above the reference, K18; O751 steps 1, 2 and 4, owner ruling L1 on O795)", cut, CntScipFileEdgesDuplicateNarrowed)
	}
	if ns > 0 {
		note += fmt.Sprintf("; %d pairs a reference to a scip-clang namespace would draw to the documents reopening it are left out (%s, owner ruling L2, O806: a namespace symbol draws no file edge, since reopening a namespace adds declarations to it and makes no file depend on another, K18's file granularity and scip-go's package, LB-F298; a document's `<file>` symbol, an include's target, keeps its edge)", ns, CntScipFileEdgesNamespace)
	}
	return note
}

// textAt is the source text at a one-line range r (0-based line and columns), "" when
// there is none to read.
func textAt(lines [][]byte, r [4]int32) string {
	if r[0] != r[2] || r[0] < 0 || int(r[0]) >= len(lines) || r[1] < 0 || r[1] > r[3] || int(r[3]) > len(lines[r[0]]) {
		return ""
	}
	return string(lines[r[0]][r[1]:r[3]])
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
	} else if i := strings.LastIndexAny(s, descriptorSeps(sym)); i >= 0 {
		s = s[:i+1]
	}
	// An impl block has no name of its own: rust-analyzer's members of one sit
	// under `impl#[Type]` or `impl#[Type][Trait]`, a type member like any other.
	return kind, strings.HasSuffix(s, "#") || (isRustAnalyzer(sym) && strings.HasSuffix(s, "]") && strings.Contains(s, "impl#["))
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
	// scip-clang reports a macro-generated definition over the macro's name
	// (clangMacroGenerated); scoped to its symbols.
	isClang := tool == "scip-clang"
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
			if r[0] != r[2] && (isRA || isDart) {
				// A name is written on one line. A multi-line range is a
				// whole-file module occurrence (rust-analyzer's, for the
				// `crate/` root and for every file module `mod x;` names),
				// not a definition anyone wrote: none is emitted here. A file
				// module's declaration is recovered below from its reference.
				// Scoped to rust-analyzer and scip-dart (whose reading was taken
				// under it): every other producer's readings predate the rule.
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
			if isClang && src != nil && r[0] == r[2] {
				name, kind = clangDefName(name, kind, sym, lineText(int(r[0])), int(r[1]), int(r[3]))
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
			if isClang && kind != "anonymous" && src != nil && r[0] == r[2] {
				if t := lineText(int(r[0])); int(r[1]) <= int(r[3]) && int(r[3]) <= len(t) && clangMacroGenerated(name, t[r[1]:r[3]]) {
					def.Generated = true
				}
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
	if isClang {
		defs = clangDedupe(defs)
	}
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
