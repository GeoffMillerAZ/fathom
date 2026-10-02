package langbench

import (
	"bytes"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// The build's own compiler as a tier-A oracle of C and C++ call sites
// (LB-F116, calibration/oracles/clang-calls). scip-clang indexes a template's
// pattern, and three facts follow that no syntactic extractor and no index
// reference settles: a call whose callee depends on a template parameter has
// no callee before instantiation; a call the compiler resolves may hold no
// reference in the index (a function template's specialization, a call inside
// code the index did not index); and a reference the index writes at a
// function's declaration (`int close(int fd);`) looks like a call and is none.
// The compiler's AST, dumped per compilation database entry, classes each
// call-shaped site written in the subject; ScoreSCIPWith reads the classes
// through SCIPOptions.ClangSites.

// Compiler call-site classes (clang-calls' "class").
const (
	// ClangDependent: CXXDependentScopeMemberExpr, UnresolvedMemberExpr,
	// UnresolvedLookupExpr, DependentScopeDeclRefExpr or
	// CXXUnresolvedConstructExpr — no callee exists before instantiation.
	ClangDependent = "dependent"
	// ClangResolved: a function or a method the compiler resolves.
	ClangResolved = "resolved"
	// ClangConstruct: a constructor call written `T(` or `T{`.
	ClangConstruct = "construct"
	// ClangObject: a call through a variable, parameter or field (a lambda,
	// a function pointer, a callable object).
	ClangObject = "object"
	// ClangCast: a functional cast with no constructor (`size_t(n)`): no call.
	ClangCast = "cast"
	// ClangDeclaration: a function's name where it is declared (`int
	// close(int fd);`): no call, though written like one.
	ClangDeclaration = "declaration"
)

// ClangSite is one call-shaped site the compiler classes: the callee's name
// token at a 1-based line and byte column.
type ClangSite struct {
	Line, Col   int
	Name, Class string
}

// clangSiteIndex finds a document's compiler sites by position.
type clangSiteIndex map[string]map[[2]int][]ClangSite

func newClangSiteIndex(sites map[string][]ClangSite) clangSiteIndex {
	out := clangSiteIndex{}
	for doc, ss := range sites {
		m := map[[2]int][]ClangSite{}
		for _, s := range ss {
			k := [2]int{s.Line, s.Col}
			m[k] = append(m[k], s)
		}
		out[doc] = m
	}
	return out
}

// covers reports whether the compiler dumped doc (it wrote any site there).
func (x clangSiteIndex) covers(doc string) bool { return len(x[doc]) > 0 }

// class is the compiler's class of the site naming name at (line, col), 1-based.
func (x clangSiteIndex) class(doc string, line, col int, name string) (string, bool) {
	for _, s := range x[doc][[2]int{line, col}] {
		if s.Name == name {
			return s.Class, true
		}
	}
	return "", false
}

// clangCalls reports whether a compiler class is a call of the truth.
func clangCalls(class string) bool {
	return class == ClangResolved || class == ClangConstruct || class == ClangObject
}

// writtenCallSites are the places in lines [lo, hi] (1-based) where callee is
// written as a call: the identifier itself, then optional template arguments,
// then `(` — the shape of fathom's call captures. Positions are 1-based.
func writtenCallSites(lines [][]byte, lo, hi int, callee string) [][2]int {
	if callee == "" || !clangIdentRE.MatchString(callee) {
		return nil
	}
	var out [][2]int
	for ln := lo; ln <= hi && ln <= len(lines); ln++ {
		if ln < 1 {
			continue
		}
		line := lines[ln-1]
		for from := 0; from < len(line); {
			i := bytes.Index(line[from:], []byte(callee))
			if i < 0 {
				break
			}
			i += from
			end := i + len(callee)
			from = end
			if (i > 0 && isIdentByte(line[i-1])) || (end < len(line) && isIdentByte(line[end])) {
				continue
			}
			r := [4]int32{int32(ln - 1), int32(i), int32(ln - 1), int32(end)}
			if dartCallAt(lines, r, callee) {
				out = append(out, [2]int{ln, i + 1})
			}
		}
	}
	return out
}

// clangBlindDefs are fathom's definitions that make calls, among nodes, whose
// lines hold no occurrence of the index but a macro's in a document the
// compiler covered:
// the index did not index that code (seen in fmt's chrono.h and std.h), so it
// holds no definition to attribute a call to, and the definition stands in for
// itself, its calls scored against the compiler's sites alone.
func clangBlindDefs(nodes []core.SymbolNode, occLines map[string]map[int32]bool, sites clangSiteIndex) []*scipDef {
	var out []*scipDef
	for _, n := range nodes {
		if len(n.Calls) == 0 || !sites.covers(n.FilePath) || occLines[n.FilePath] == nil {
			continue
		}
		lo, hi := int32(n.Line-1), int32(n.EndLine-1)
		if hi < lo {
			hi = lo
		}
		blind := true
		for l := lo; l <= hi && blind; l++ {
			blind = !occLines[n.FilePath][l]
		}
		if !blind {
			continue
		}
		name := BareName(n.Name)
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

// clangUnjudged reports why no oracle can judge fathom's calls of callee in
// the spans: every call written there (a function's own name in its
// declarator, which the compiler declares, is none) is on a line the build
// discards, or is one the compiler classes dependent. why is "" when some call
// is judgeable or none is written.
func clangUnjudged(lines [][]byte, spans [][2]int, doc, callee string, sites clangSiteIndex, active [][2]int) (why string) {
	n, dependent := 0, false
	for _, sp := range spans {
		for _, at := range writtenCallSites(lines, sp[0], sp[1], callee) {
			c, ok := sites.class(doc, at[0], at[1], callee)
			switch {
			case ok && c == ClangDeclaration:
				continue
			case active != nil && !lineIn(active, at[0]):
			case ok && c == ClangDependent:
				dependent = true
			default:
				return ""
			}
			n++
		}
	}
	switch {
	case n == 0:
		return ""
	case dependent:
		return CntClangCallsDependent
	}
	return CntCallsInactive
}

// lineIn reports whether line lies in one of the sorted [first, last] ranges.
func lineIn(ranges [][2]int, line int) bool {
	i := sort.Search(len(ranges), func(i int) bool { return ranges[i][1] >= line })
	return i < len(ranges) && ranges[i][0] <= line
}

// isTypeOrCtor reports whether a SCIP symbol names a type (`T#`) or a method
// (a constructor's reference is its class's method).
func isTypeOrCtor(sym string) bool {
	return strings.HasSuffix(sym, "#") || strings.HasSuffix(sym, ").")
}

// innermostDef is the narrowest definition whose enclosing range holds r.
func innermostDef(defs []*scipDef, r [4]int32) *scipDef {
	var inner *scipDef
	for _, def := range defs {
		if def.hasEnc && contains(def.enc, r) && (inner == nil || contains(inner.enc, def.enc)) {
			inner = def
		}
	}
	return inner
}

// addClangTruth adds to truth each call the compiler resolves (a function or
// method, a constructor, a callable object) inside a definition the index
// maps, in the documents the index holds. A pair the index's references
// already give is not counted; a constructor call the index references as a
// class or constructor is counted as such (CntClangConstructs), every other
// added pair as the compiler's alone (CntClangCallsAdded).
func addClangTruth(truth map[CallPair]bool, all map[string][]ClangSite, docs map[string]bool, defsByDoc map[string][]*scipDef, occStarts map[string]map[[2]int32][]string, counts map[string]int) {
	paths := make([]string, 0, len(all))
	for doc := range all {
		paths = append(paths, doc)
	}
	sort.Strings(paths)
	for _, doc := range paths {
		if !docs[doc] {
			continue
		}
		for _, s := range all[doc] {
			if !clangCalls(s.Class) {
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
			referenced := false
			for _, sym := range occStarts[doc][[2]int32{r[0], r[1]}] {
				referenced = referenced || isTypeOrCtor(sym)
			}
			if s.Class == ClangConstruct && referenced {
				counts[CntClangConstructs]++
			} else {
				counts[CntClangCallsAdded]++
			}
		}
	}
}

// dropClangUnjudged removes from found each pair the truth does not hold
// that no oracle can judge (clangUnjudged): its every written call has no
// callee before instantiation (CntClangCallsDependent) or is on a line the
// build discards (CntCallsInactive).
func dropClangUnjudged(found, truth map[CallPair]bool, nodes []core.SymbolNode, srcOf func(string) [][]byte, sites clangSiteIndex, active map[string][][2]int, counts map[string]int) {
	spans := map[[2]string][][2]int{}
	for _, n := range nodes {
		hi := n.EndLine
		if hi < n.Line {
			hi = n.Line
		}
		k := [2]string{n.FilePath, BareName(n.Name)}
		spans[k] = append(spans[k], [2]int{n.Line, hi})
	}
	for p := range found {
		if truth[p] || !sites.covers(p.Path) {
			continue
		}
		var act [][2]int
		if a, ok := active[p.Path]; ok {
			act = a
			if act == nil {
				act = [][2]int{} // listed with no range: every line discarded
			}
		}
		if why := clangUnjudged(srcOf(p.Path), spans[[2]string{p.Path, p.Caller}], p.Path, p.Callee, sites, act); why != "" {
			delete(found, p)
			counts[why]++
		}
	}
}
