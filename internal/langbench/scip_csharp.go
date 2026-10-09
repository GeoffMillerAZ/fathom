package langbench

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// C#'s SCIP rules (scip-dotnet 0.2.14, wave lf1): its vocabulary read in the
// names the dialect gives (LB-F286), and the compiler's own semantic model as
// the call-site oracle beside the index (LB-F287, calibration/oracles/roslyn
// calls mode, tier A), scored the way LB-F116 scores clang-calls beside
// scip-clang.

// isSCIPDotnet reports whether sym is one scip-dotnet wrote.
func isSCIPDotnet(sym string) bool { return strings.HasPrefix(sym, "scip-dotnet ") }

// csharpSCIPOptions are C#'s SCIP options: its call sites as written (a
// constructor reference at an attribute is none, a type written `new X(` is
// a call to X) and the calls the index writes no occurrence for left out as
// unjudged under LB-F203's cap (LB-F175, O464).
func csharpSCIPOptions(src func(path string) []byte) SCIPOptions {
	return SCIPOptions{Source: src, OracleBlindSites: src != nil, CSharpCalls: src != nil}
}

// csharpName is a scip-dotnet descriptor leaf in the name the dialect gives
// the same definition (any other producer's leaf is returned as it is): a
// constructor `.ctor` or static constructor `.cctor` is its class, which is
// also the name `new X(..)` calls; an explicit interface implementation
// (`Polly.IAsyncPolicy.WithPolicyKey`, the interface's type arguments
// included) is its member's own name; an indexer (`this[]`) is `this`, the
// Roslyn dump's name for it; a type's arity suffix (List`1) is not part of
// its name. owner is the descriptor text before the leaf ("…/Widget#").
func csharpName(sym, leaf, owner string) string {
	if !isSCIPDotnet(sym) {
		return leaf
	}
	switch {
	case leaf == ".ctor" || leaf == ".cctor":
		o := strings.TrimSuffix(owner, "#")
		return csharpArity(o[strings.LastIndexAny(o, "/#.:] `")+1:])
	case leaf == "this[]":
		return "this"
	}
	depth := 0
	for i := len(leaf) - 1; i >= 0; i-- {
		switch leaf[i] {
		case '>':
			depth++
		case '<':
			depth--
		case '.':
			if depth == 0 && i > 0 {
				return leaf[i+1:]
			}
		}
	}
	return csharpArity(leaf)
}

// csharpArity drops a metadata arity suffix (`List`1` is List).
func csharpArity(name string) string {
	if i := strings.LastIndexByte(name, '`'); i > 0 && i < len(name)-1 && strings.Trim(name[i+1:], "0123456789") == "" {
		return name[:i]
	}
	return name
}

// Counts of C#'s call-site rules, each disclosed in the reading's notes.
const (
	// CntScipCSharpCtorNotCalls: scip-dotnet constructor (`.ctor`) references
	// not written as `new X(`: an attribute applied (`[Obsolete("x")]`), whose
	// constructor runs only when reflection reads it, so nothing is called
	// where it is written. Left out of the call truth.
	CntScipCSharpCtorNotCalls = "scip_csharp_ctor_not_calls"
	// CntScipCSharpConstructions: type references written `new X(..)`,
	// `new X<T>(..)` or `new X { .. }`, counted as calls to X (scip-dotnet
	// references the type there, never its constructor).
	CntScipCSharpConstructions = "scip_csharp_constructions"
	// CntCSharpCallsAdded: calls the compiler binds at a written call, in the
	// call truth with no callable reference of the index behind them (a
	// generic method called with explicit type arguments, O271; a
	// delegate-typed local, parameter or field invoked; a local function).
	CntCSharpCallsAdded = "csharp_compiler_calls_added"
	// CntCSharpConstructs: constructions the compiler binds that no index
	// reference already gave.
	CntCSharpConstructs = "csharp_compiler_constructs"
	// CntCSharpCallsUnresolved: fathom pairs whose every written site the
	// compiler binds to nothing in any target framework (late-bound dynamic,
	// an error): left out of both counts.
	CntCSharpCallsUnresolved = "csharp_compiler_calls_unresolved"
	// CntCSharpStandInDefs: fathom definitions that make calls with no
	// definition of the index at their name's line (scip-dotnet writes none
	// for a conversion operator, nor in code its one target framework does
	// not compile), each standing in for itself as a caller (csharpStandIns).
	CntCSharpStandInDefs = "csharp_scip_stand_in_defs"
	// CntScipCSharpFileEdgesMultiDefined: index file edges whose only
	// references name a symbol more than one document defines (a partial
	// type; a generic type and its non-generic namesake, which scip-dotnet
	// gives one symbol), left out of the file-edge truth: which file such a
	// reference means has no answer, as the import truth already holds for a
	// symbol two documents define (LB-F287).
	CntScipCSharpFileEdgesMultiDefined = "scip_csharp_file_edges_multi_defined"
	// CntCSharpFileEdgesAdded: the compiler's cross-file edges, from a
	// document the index holds, that the index's references do not give.
	CntCSharpFileEdgesAdded = "csharp_compiler_file_edges_added"
)

// The compiler's call-site classes (calibration/oracles/roslyn calls mode).
const (
	CSharpResolved   = "resolved"   // an invocation bound to a method
	CSharpObject     = "object"     // a delegate-typed variable, parameter, field or property invoked
	CSharpConstruct  = "construct"  // `new X(..)` bound to a constructor
	CSharpUnresolved = "unresolved" // bound to nothing in any compiled target framework
)

// CSharpCompiler is the compiler's semantic model of the subject (tier A,
// LB-F287): its call sites per document (1-based line and byte column of the
// callee's name token) and the lines some compiled configuration keeps per
// document. A document it does not list is not judged by it.
type CSharpCompiler struct {
	Sites  map[string][]ClangSite
	Active map[string][][2]int
	// QueryClauses counts LINQ query clauses the compiler binds to a method
	// (`where` to Where) where no invocation is written: never a call site.
	QueryClauses int
	// FileEdges are the compiler's (referencing file, declaring file) pairs
	// over every target framework (LB-F287); MultiDeclared counts the names it
	// bound to a symbol declared in more than one file, which draw none.
	FileEdges     [][2]string
	MultiDeclared int
}

// csharpCallee reads one scip-dotnet reference at its written site (LB-F286):
// a constructor reference is a call only when written `new X(`, and a type
// reference written `new X(` is a call to X. Every other reference is
// returned as it is.
func csharpCallee(sym, callee string, callable bool, lines [][]byte, r [4]int32, counts map[string]int) (string, bool) {
	if !isSCIPDotnet(sym) {
		return callee, callable
	}
	if callable && (strings.Contains(sym, "`.ctor`(") || strings.Contains(sym, "`.cctor`(")) {
		if w, ok := csharpNewAt(lines, r); ok {
			counts[CntScipCSharpConstructions]++
			return w, true
		}
		counts[CntScipCSharpCtorNotCalls]++
		return "", false
	}
	if !callable && strings.HasSuffix(sym, "#") {
		if w, ok := csharpNewAt(lines, r); ok && w == callee {
			counts[CntScipCSharpConstructions]++
			return w, true
		}
	}
	return callee, callable
}

var (
	csharpNewBefore = regexp.MustCompile(`\bnew\s+(?:global::)?(?:@?[A-Za-z_][A-Za-z0-9_]*\s*\.\s*)*$`)
	csharpIdentRE   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// csharpNewAt reports the identifier written at range r when it is the type
// of an object creation: `new` (and a qualifier) before it, and after it,
// past any type arguments, `(` or `{` on its line or the next written one.
func csharpNewAt(lines [][]byte, r [4]int32) (string, bool) {
	if r[0] != r[2] || r[0] < 0 || int(r[0]) >= len(lines) || r[1] < 0 || r[1] > r[3] || int(r[3]) > len(lines[r[0]]) {
		return "", false
	}
	line := lines[r[0]]
	w := string(bytes.TrimPrefix(line[r[1]:r[3]], []byte("@")))
	if !csharpIdentRE.MatchString(w) || !csharpNewBefore.Match(line[:r[1]]) {
		return "", false
	}
	rest := bytes.TrimLeft(line[r[3]:], " \t\r")
	if len(rest) > 0 && rest[0] == '<' {
		depth, closed := 0, false
		for i, c := range rest {
			switch c {
			case '<':
				depth++
			case '>':
				depth--
			}
			if depth == 0 {
				rest, closed = bytes.TrimLeft(rest[i+1:], " \t\r"), true
				break
			}
		}
		if !closed {
			return "", false
		}
	}
	for next := int(r[0]) + 1; len(rest) == 0 && next < len(lines) && next <= int(r[0])+2; next++ {
		rest = bytes.TrimLeft(lines[next], " \t\r")
	}
	if len(rest) > 0 && (rest[0] == '(' || rest[0] == '{') {
		return w, true
	}
	return "", false
}

// addCSharpTruth adds to truth each call the compiler binds (a method, a
// delegate invoked, a construction) inside a definition the index maps, in
// the documents the index holds. A pair the index's references already give
// is not counted.
func addCSharpTruth(truth map[CallPair]bool, cc *CSharpCompiler, docs map[string]bool, defsByDoc map[string][]*scipDef, counts map[string]int) {
	paths := make([]string, 0, len(cc.Sites))
	for doc := range cc.Sites {
		paths = append(paths, doc)
	}
	sort.Strings(paths)
	for _, doc := range paths {
		if !docs[doc] {
			continue
		}
		for _, s := range cc.Sites[doc] {
			if s.Class != CSharpResolved && s.Class != CSharpObject && s.Class != CSharpConstruct {
				continue
			}
			r := [4]int32{int32(s.Line - 1), int32(s.Col - 1), int32(s.Line - 1), int32(s.Col - 1 + len(s.Name))}
			inner := innermostDef(defsByDoc[doc], r)
			if inner == nil || inner.fathomDef == "" {
				continue
			}
			p := CallPair{doc, inner.fathomDef, s.Name}
			if truth[p] {
				continue
			}
			truth[p] = true
			if s.Class == CSharpConstruct {
				counts[CntCSharpConstructs]++
			} else {
				counts[CntCSharpCallsAdded]++
			}
		}
	}
}

// dropCSharpUnjudged removes from found each pair the truth does not hold
// whose every written call site (tsWrittenCallSites inside the caller's
// lines) no compiled configuration keeps (CntCallsInactive) or the compiler
// binds to nothing (CntCSharpCallsUnresolved). A pair with one site the
// compiler judges, or with none written, stays.
func dropCSharpUnjudged(found, truth map[CallPair]bool, nodes []core.SymbolNode, srcOf func(string) [][]byte, cc *CSharpCompiler, counts map[string]int) {
	classAt := map[string]map[[2]int][]ClangSite{}
	for doc, ss := range cc.Sites {
		m := map[[2]int][]ClangSite{}
		for _, s := range ss {
			m[[2]int{s.Line, s.Col}] = append(m[[2]int{s.Line, s.Col}], s)
		}
		classAt[doc] = m
	}
	spans := map[[2]string][][2]int{}
	for _, n := range nodes {
		k := [2]string{n.FilePath, BareName(n.Name)}
		spans[k] = append(spans[k], [2]int{n.Line, max(n.EndLine, n.Line)})
	}
	for p := range found {
		active, compiled := cc.Active[p.Path]
		if truth[p] || !compiled {
			continue
		}
		lines := srcOf(p.Path)
		n, unresolved, judged := 0, false, false
		for _, sp := range spans[[2]string{p.Path, p.Caller}] {
			for _, at := range tsWrittenCallSites(lines, sp[0], sp[1], p.Callee) {
				n++
				if !lineIn(active, at[0]) {
					continue
				}
				cls := ""
				for _, s := range classAt[p.Path][[2]int{at[0], at[1] + 1}] {
					if s.Name == p.Callee {
						cls = s.Class
					}
				}
				if cls == CSharpUnresolved {
					unresolved = true
					continue
				}
				judged = true
			}
		}
		if n == 0 || judged {
			continue
		}
		delete(found, p)
		if unresolved {
			counts[CntCSharpCallsUnresolved]++
		} else {
			counts[CntCallsInactive]++
		}
	}
}

// csharpStandIns are fathom's definitions that make calls, in documents the
// index holds, with no definition of the index at their name's line and none
// mapped to them within two lines: scip-dotnet writes no definition for a
// conversion operator (only its parameter) nor for code outside the one
// target framework it indexed, so the calls in such a body would be
// attributed to the type around it. Each stands in for itself, its own lines
// its span, as clangBlindDefs' definitions do (LB-F116).
func csharpStandIns(nodes []core.SymbolNode, defsByDoc map[string][]*scipDef, docs map[string]bool) []*scipDef {
	var out []*scipDef
	for _, n := range nodes {
		if len(n.Calls) == 0 || !docs[n.FilePath] {
			continue
		}
		name := BareName(n.Name)
		defined := false
		for _, d := range defsByDoc[n.FilePath] {
			if d.line == n.Line || (d.fathomDef == name && d.line >= n.Line-2 && d.line <= n.Line+2) {
				defined = true
				break
			}
		}
		if defined {
			continue
		}
		lo, hi := int32(n.Line-1), int32(max(n.EndLine, n.Line)-1)
		out = append(out, &scipDef{doc: n.FilePath, line: n.Line, name: name, fathomDef: name, enc: [4]int32{lo, 0, hi, 1 << 30}, hasEnc: true})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].doc != out[j].doc {
			return out[i].doc < out[j].doc
		}
		return out[i].line < out[j].line
	})
	return out
}

// addCSharpFileEdges joins the compiler's cross-file edges to the file-edge
// truth (LB-F287): each from a document the index holds (fathom's edges from
// any other file are out of the file-edge scope, §1.4), counted when the
// index's references did not already give it.
func addCSharpFileEdges(edges map[[2]string]bool, cc *CSharpCompiler, docs map[string]bool, counts map[string]int) {
	for _, e := range cc.FileEdges {
		if !docs[e[0]] || e[0] == e[1] || edges[e] {
			continue
		}
		edges[e] = true
		counts[CntCSharpFileEdgesAdded]++
	}
}
