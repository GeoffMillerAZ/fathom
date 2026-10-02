package fusion

// crefs.go — LB-F196 (O283, O369): a C++ reference binds through the include
// graph and its namespaces.
//
// The C++ incumbent names a definition by its own name (`Point`, never
// `lib.Point`) and keeps a namespace as a symbol of its own, so a qualified
// name `lib::Point` cannot be looked up by its text. A reference's last segment
// names the candidate definitions in the files its #include graph reaches
// (fileScope.wildFiles, the closure includeClosure builds in compile-database
// -I order); each candidate's enclosing namespaces, read from the namespace
// definitions whose lines hold it, must end with the qualifier the reference
// writes. An unqualified name binds the one definition the reached headers
// hold, or, among several, the one whose namespaces enclose the referring
// definition. A name two reached files still both answer binds nothing, a name
// the file defines itself binds nothing (refresolve.go), and a definition a
// function body holds (a local alias, a local class) is no name of a header.
// Nothing here is stored: the namespace chains are computed per resolve.

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// isCppSymbol reports whether s is a C++ definition (the incumbent's and the
// dialect's kinds both start with "Cpp").
func isCppSymbol(s *core.SymbolNode) bool { return strings.HasPrefix(string(s.Kind), "Cpp") }

// enclosingChain is the names of the namespace definitions of file whose lines
// hold line, outermost first; self is skipped (a namespace is not its own
// qualifier).
func (idx *resolveIndex) enclosingChain(file string, line int, self string) []string {
	type ns struct {
		name       string
		start, end int
	}
	var held []ns
	for _, s := range idx.byFile[file] {
		if c, ok := extractor.CanonicalKind(string(s.Kind)); !ok || c != "namespace" || s.Key() == self {
			continue
		}
		if s.Line < line && line <= max(s.EndLine, s.Line) {
			held = append(held, ns{s.Name, s.Line, max(s.EndLine, s.Line)})
		}
	}
	sort.SliceStable(held, func(a, b int) bool { return held[a].start < held[b].start })
	out := make([]string, len(held))
	for i, h := range held {
		out[i] = h.name
	}
	return out
}

// insideCallable reports whether the definition at line of file lies inside a
// function's lines (a local alias, a local type): no header's name.
func (idx *resolveIndex) insideCallable(file string, line int, self string) bool {
	for _, s := range idx.byFile[file] {
		if c, ok := extractor.CanonicalKind(string(s.Kind)); !ok || c != "function" || s.Key() == self {
			continue
		}
		if s.Line < line && line <= s.EndLine {
			return true
		}
	}
	return false
}

// endsWith reports whether chain's tail is qual.
func endsWith(chain, qual []string) bool {
	if len(qual) > len(chain) {
		return false
	}
	off := len(chain) - len(qual)
	for i, q := range qual {
		if chain[off+i] != q {
			return false
		}
	}
	return true
}

// isPrefixOf reports whether chain is a leading part of of.
func isPrefixOf(chain, of []string) bool {
	if len(chain) > len(of) {
		return false
	}
	for i, c := range chain {
		if of[i] != c {
			return false
		}
	}
	return true
}

// matchCppRef binds a C++ reference made in s (a definition of path) to the
// definition its last segment names in a file path's includes reach.
func (idx *resolveIndex) matchCppRef(sc fileScope, name string, s *core.SymbolNode, path string) (string, bool) {
	segs := refSegments(name)
	for len(segs) > 0 && segs[0] == "" {
		segs = segs[1:]
	}
	if len(segs) == 0 {
		return "", false
	}
	last, qual := segs[len(segs)-1], segs[:len(segs)-1]
	// A class member or nested type is named by its whole path: Shape::area.
	if len(qual) > 0 {
		whole := strings.Join(segs, ".")
		var hit string
		for _, f := range sc.wildFiles {
			if key, ok := idx.qualified[f][whole]; ok {
				if hit != "" && hit != key {
					hit = ""
					break
				}
				hit = key
			}
		}
		if hit != "" {
			return hit, true
		}
	}
	type cand struct {
		key   string
		chain []string
	}
	var cands []cand
	seen := map[string]bool{}
	for _, f := range sc.wildFiles {
		key, ok := idx.qualified[f][last]
		if !ok || seen[key] {
			continue
		}
		var node *core.SymbolNode
		for i := range idx.byFile[f] {
			if idx.byFile[f][i].Key() == key {
				node = &idx.byFile[f][i]
				break
			}
		}
		if node == nil || idx.insideCallable(f, node.Line, key) {
			continue
		}
		seen[key] = true
		cands = append(cands, cand{key, idx.enclosingChain(f, node.Line, key)})
	}
	if len(qual) > 0 {
		var keep []cand
		for _, c := range cands {
			if endsWith(c.chain, qual) {
				keep = append(keep, c)
			}
		}
		cands = keep
	} else if len(cands) > 1 {
		own := idx.enclosingChain(path, s.Line, s.Key())
		var keep []cand
		for _, c := range cands {
			if isPrefixOf(c.chain, own) {
				keep = append(keep, c)
			}
		}
		cands = keep
	}
	if len(cands) != 1 {
		return "", false
	}
	return cands[0].key, true
}

// namespaceTarget reports whether key is a namespace definition under the
// include-path strategy: a call `std::move(x)` or `fmt::format(..)` names a
// function through its qualifier, and binding the qualifier's namespace block
// (a file that merely opens `namespace std`) draws an edge the call does not
// make.
func (idx *resolveIndex) namespaceTarget(r *importResolver, key string) bool {
	if r.strategy != "include-path" {
		return false
	}
	if idx.nsKeys == nil {
		idx.nsKeys = map[string]bool{}
		for _, syms := range idx.byFile {
			for _, s := range syms {
				if c, ok := extractor.CanonicalKind(string(s.Kind)); ok && c == "namespace" {
					idx.nsKeys[s.Key()] = true
				}
			}
		}
	}
	return idx.nsKeys[key]
}
