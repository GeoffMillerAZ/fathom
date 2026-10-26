package fusion

// cproto.go — LB-Q-c-rest (O281): a call bound through a header prototype.
//
// Under the include-path strategy a call binds the definition in the caller's
// file or in a file it includes, directly or through other headers. A C call
// to a function another .c file defines reaches it through a prototype in a
// header — and a prototype is no symbol, so the call bound nothing and the
// caller's file edge to that .c file was never drawn (libuv: 1,666 of 2,131
// index file edges end in a .c file).
//
// A call whose name binds no definition, while a header the caller includes
// declares that name, binds the one source file (.c, .cc, .cpp, .cxx, .m, .mm)
// that defines it with external linkage: what the linker would do. A static
// definition is no candidate. When a compile database names the files the build
// compiles, only those are candidates (libuv's unix/ and win/ trees define one
// function twice; one is built). Two candidates bind nothing, and every outcome
// is counted in FileResolveReport.

import (
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// protoCounts are the prototype binding's outcomes, each a distinct
// (caller file, name).
type protoCounts struct {
	bound, ambiguous, noDefiner, buildFiltered, notBuilt map[string]bool
}

func newProtoCounts() *protoCounts {
	return &protoCounts{map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}}
}

// includesOf is the files file's own #include lines resolve to, less those
// inside a conditional branch the build is known not to take (cguards.go).
func (idx *resolveIndex) includesOf(r *importResolver, file string) []string {
	if out, ok := idx.incl[file]; ok {
		return out
	}
	var out []string
	var excluded map[int]bool // the lines a guard known false keeps from the build (cguards.go, O468)
	if env := r.env.guardEnv(file); env != nil {
		if src := r.env.manifest(file); src != nil {
			excluded = cExcludedLines(src, env)
		}
	}
	for _, sp := range idx.specs[file].Imports {
		if sp.Kind != extractor.ImportWhole && sp.Kind != extractor.ImportWildcard {
			continue
		}
		if excluded[sp.Line] {
			continue
		}
		for _, t := range r.resolve(file, sp.Module) {
			if t != file {
				out = append(out, t)
			}
		}
	}
	out = dedupeKeep(out)
	if idx.incl == nil {
		idx.incl = map[string][]string{}
	}
	idx.incl[file] = out
	return out
}

// maxIncludeDepth bounds how many #include levels a closure follows.
const maxIncludeDepth = 16

// includeClosure is roots and every file they include, transitively, each
// once.
func (idx *resolveIndex) includeClosure(r *importResolver, roots []string) []string {
	seen := map[string]bool{}
	var out []string
	frontier := roots
	for depth := 0; len(frontier) > 0 && depth <= maxIncludeDepth; depth++ {
		var next []string
		for _, f := range frontier {
			if seen[f] {
				continue
			}
			seen[f] = true
			out = append(out, f)
			next = append(next, idx.includesOf(r, f)...)
		}
		frontier = next
	}
	return out
}

// declaredIn is the function names a header declares, scanned once.
func (idx *resolveIndex) declaredIn(r *importResolver, file string) map[string]bool {
	if m, ok := idx.protoDecl[file]; ok {
		return m
	}
	var m map[string]bool
	if b := r.env.manifest(file); b != nil {
		m = scanDeclaredFunctions(b)
	}
	if idx.protoDecl == nil {
		idx.protoDecl = map[string]map[string]bool{}
	}
	idx.protoDecl[file] = m
	return m
}

// isImplFile reports whether p is a C/C++ translation unit, not a header.
func isImplFile(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".c", ".cc", ".cpp", ".cxx", ".m", ".mm":
		return true
	}
	return false
}

// externalDefiners is, per function name, the source files that define it with
// external linkage.
func (idx *resolveIndex) externalDefiners() map[string][]string {
	if idx.definers != nil {
		return idx.definers
	}
	idx.definers = map[string][]string{}
	for file, syms := range idx.byFile {
		if !isImplFile(file) {
			continue
		}
		for _, s := range syms {
			if c, ok := extractor.CanonicalKind(string(s.Kind)); !ok || c != "function" || strings.Contains(s.Name, ".") {
				continue
			}
			if isStaticDefinition(s) {
				continue
			}
			if fs := idx.definers[s.Name]; len(fs) == 0 || fs[len(fs)-1] != file {
				idx.definers[s.Name] = append(fs, file)
			}
		}
	}
	for _, fs := range idx.definers {
		sort.Strings(fs)
	}
	return idx.definers
}

// isStaticDefinition reports whether a definition's own signature says static
// (internal linkage).
func isStaticDefinition(s core.SymbolNode) bool {
	head := s.Signature
	if i := strings.Index(head, "("); i >= 0 {
		head = head[:i]
	}
	for _, w := range strings.Fields(head) {
		if w == "static" {
			return true
		}
	}
	return false
}

// matchPrototype binds call, made from path, through a prototype a header in
// the caller's include closure declares (see the file comment).
func (idx *resolveIndex) matchPrototype(r *importResolver, sc fileScope, path, call string) (string, bool) {
	if r.strategy != "include-path" || !isIdentToken(call) {
		return "", false
	}
	// Declared in the caller's own file (a header calling through its own
	// prototype, a forward declaration) or in a header it includes.
	declared := idx.declaredIn(r, path)[call]
	for _, h := range sc.wildFiles {
		if declared {
			break
		}
		declared = idx.declaredIn(r, h)[call]
	}
	if !declared {
		return "", false
	}
	key := path + "\x00" + call
	defs := idx.externalDefiners()[call]
	n := len(defs)
	if built := r.env.compiledFiles(); built != nil && n > 0 {
		var kept []string
		for _, f := range defs {
			if built[f] {
				kept = append(kept, f)
			}
		}
		if len(kept) == 0 {
			idx.proto.notBuilt[key] = true
			return "", false
		}
		if len(kept) < n {
			idx.proto.buildFiltered[key] = true
		}
		defs = kept
	}
	switch len(defs) {
	case 0:
		idx.proto.noDefiner[key] = true
		return "", false
	case 1:
		if k, ok := idx.topLevel[defs[0]][call]; ok {
			idx.proto.bound[key] = true
			return k, true
		}
		return "", false
	}
	idx.proto.ambiguous[key] = true
	return "", false
}

// Foreign headers. The header router sends a .h to C++ on a C++ keyword
// anywhere in its text (comments included: libuv's uv.h and curl's curl.h and
// curl_setup.h read as C++ for the word "class" in a comment), so the headers
// a C file's includes pass through are not all in its language's universe. They
// are still the headers its compiler reads: their includes and prototypes are
// read from their text, and they bind no definition (no symbols are given for
// them). foreignHeaders finds them; addForeign admits them to the set an
// include resolves against.

// addForeign admits headers outside the universe to the set includes resolve
// against.
func (e *resolveEnv) addForeign(paths []string) {
	for _, p := range paths {
		if !e.set[p] {
			e.set[p] = true
		}
	}
}

// foreignHeader is a header the router sent to the other language, read but
// not yet parsed.
type foreignHeader struct {
	path string
	src  []byte
}

// foreignPruneOff is a test seam: with it set every foreign header is parsed,
// the shape before reachability (LB-F179).
var foreignPruneOff bool

// includeText finds an #include's operand text on a line (continuations
// folded in): every quoted or angled token on it.
var (
	includeLineRE  = regexp.MustCompile(`#[ \t]*(?:include|import)(?:\\\r?\n|[^\n])*`)
	includeTokenRE = regexp.MustCompile(`"([^"\n]*)"|<([^>\n]*)>`)
)

// includeBase is the last element of an include operand: every file an
// #include resolves to ends in its operand, so its base name is the operand's.
func includeBase(spec string) string {
	spec = strings.Trim(spec, "\"<> \t")
	if spec == "" {
		return ""
	}
	return path.Base(spec)
}

// reachableForeign is the foreign headers an #include can reach from the
// language's own files, in hs's order: those whose base name an own file's
// import names, then, transitively, those a reached header's text includes.
// An include resolves to a file whose path ends in its operand, so a header
// no operand names by base can be no include's target: parsing it changes no
// edge, and admitting it to the include set changes no resolution. The scan of
// a header's text over-approximates what its parse would give (a commented
// include counts), so nothing a parse would reach is left out.
func reachableForeign(own map[string]extractor.FileSpecs, hs []foreignHeader) []foreignHeader {
	src := make(map[string][]byte, len(hs))
	paths := make([]string, len(hs))
	for i, h := range hs {
		src[h.path], paths[i] = h.src, h.path
	}
	return reachableForeignRead(own, paths, func(p string) ([]byte, bool) { b, ok := src[p]; return b, ok })
}

// reachableForeignRead is reachableForeign over header paths, read only once
// an include names their base name (round 3 F6): a repository's unrelated
// headers are never read, where the eager pass read every one to prune them.
// A header read fails (ok false) is skipped, as an unreadable one was.
func reachableForeignRead(own map[string]extractor.FileSpecs, paths []string, read func(string) ([]byte, bool)) []foreignHeader {
	got := make([][]byte, len(paths))
	ok := make([]bool, len(paths))
	taken := make([]bool, len(paths))
	if foreignPruneOff {
		var out []foreignHeader
		for _, p := range paths {
			if b, ok := read(p); ok {
				out = append(out, foreignHeader{p, b})
			}
		}
		return out
	}
	byBase := map[string][]int{}
	for i, p := range paths {
		byBase[path.Base(p)] = append(byBase[path.Base(p)], i)
	}
	var queue []string
	for _, sp := range own {
		for _, im := range sp.Imports {
			if b := includeBase(im.Module); b != "" {
				queue = append(queue, b)
			}
		}
	}
	for len(queue) > 0 {
		b := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, i := range byBase[b] {
			if taken[i] {
				continue
			}
			taken[i] = true
			if got[i], ok[i] = read(paths[i]); !ok[i] {
				continue
			}
			for _, line := range includeLineRE.FindAll(got[i], -1) {
				for _, m := range includeTokenRE.FindAllSubmatch(line, -1) {
					op := m[1]
					if op == nil {
						op = m[2]
					}
					if nb := includeBase(string(op)); nb != "" {
						queue = append(queue, nb)
					}
				}
			}
		}
	}
	var out []foreignHeader
	for i, p := range paths {
		if taken[i] && ok[i] {
			out = append(out, foreignHeader{p, got[i]})
		}
	}
	return out
}

// maxForeignHeaders bounds how many headers a disk walk admits.
const maxForeignHeaders = 20000

// foreignHeaders is the .h files under root that are not in set, sorted: a
// walk that skips dot directories and node_modules.
func foreignHeaders(root string, set map[string]bool) []string {
	if root == "" {
		return nil
	}
	var out []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(path.Ext(name), ".h") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !set[rel] && len(out) < maxForeignHeaders {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}
