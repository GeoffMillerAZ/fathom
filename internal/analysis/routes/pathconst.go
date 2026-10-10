package routes

// pathconst.go — imported path constants for the route seam (SG-46, wave h3,
// O735/O774): a React Router path written `paths.app.users.path`, with
// `paths` imported from a module of the same repo, is a deferred reference
// the seam resolves on read, the way a mount is.
//
// Three halves, each pure and per-file until Compose:
//
//   - the React Router reader (reactrouter.go) reads a path that is an
//     imported binding's member chain (no call, template or spread) as a
//     deferred reference element and joins it to its parent's path as a
//     deferred path (deferJoin);
//   - this file's reader emits, for a module whose exported constants look
//     like route paths, those constants (string literals, or object-literal
//     member chains ending in one) as PathConst pseudo-rows, which
//     serviceref stores as service_routes rows of method ConstMethod;
//   - Compose resolves each deferred path against the constants of the
//     module its import names (resolvePathConsts). A path that does not
//     resolve leaves its route unread, as before.
//
// A module emits its constants only when at least one is a string starting
// with "/" (the bound on storage: a config module of strings is not
// emitted), and then every string leaf that has no whitespace, quote or
// angle bracket and at most 200 bytes (relative members such as
// 'discussions' ride with the absolute ones in the same object).

import (
	"bytes"
	"path"
	"sort"
	"strings"
)

func init() {
	Register(Reader{Framework: "path-consts", Langs: []string{"js", "ts"}, Read: readPathConsts})
}

// PathConst is one exported string constant of a module that looks like a
// route-path module: Name is the export's name or its member chain
// ("paths.app.users.path"), Value the string.
type PathConst struct {
	Name  string
	Value string
	File  string
	Line  int
}

// The deferred-path encoding: a path that starts with deferMark is a list of
// elements joined by deferSep, each "L<literal path>" or
// "R<module spec><refSep><member chain>". The control bytes never occur in a
// source path.
const (
	deferMark = "\x02"
	deferSep  = "\x1f"
	refSep    = "\x1e"

	maxConstLeaves = 256
	maxConstDepth  = 6
	maxConstLen    = 200
)

func isDeferred(p string) bool { return strings.HasPrefix(p, deferMark) }

// refElem is the deferred element for member chain of module spec.
func refElem(spec, chain string) string { return deferMark + "R" + spec + refSep + chain }

// deferElems splits a path (literal or deferred) into elements; "" has none.
func deferElems(p string) []string {
	switch {
	case p == "":
		return nil
	case isDeferred(p):
		return strings.Split(strings.TrimPrefix(p, deferMark), deferSep)
	}
	return []string{"L" + p}
}

// deferJoin is rrJoin for a path where either side is deferred: the
// elements, in order. A pathless route keeps its parent's.
func deferJoin(parent, p string, hasPath bool) string {
	if !hasPath {
		return parent
	}
	return deferMark + strings.Join(append(deferElems(parent), deferElems(p)...), deferSep)
}

// readPathConsts emits a route-path module's exported constants.
func readPathConsts(p string, src []byte) Result {
	ext := strings.ToLower(path.Ext(p))
	if !rrExts[ext] || strings.HasSuffix(p, ".d.ts") || strings.Contains("/"+p, "/node_modules/") {
		return Result{}
	}
	if !bytes.Contains(src, []byte("export")) || !(bytes.Contains(src, []byte("'/")) || bytes.Contains(src, []byte("\"/")) || bytes.Contains(src, []byte("`/"))) {
		return Result{}
	}
	toks, _ := lexJSX(src, ext != ".ts" && ext != ".mts" && ext != ".cts")
	f := &jsFile{path: p, toks: toks}
	f.pairBrackets()
	var leaves []PathConst
	for i := 0; i+3 < len(f.toks) && len(leaves) < maxConstLeaves; i++ {
		if !f.toks[i].is(tkIdent, "export") || !f.toks[i+1].is(tkIdent, "const") || f.toks[i+2].kind != tkIdent {
			continue
		}
		name := f.toks[i+2].val
		j := i + 3
		if f.toks[j].punct(":") { // a type annotation
			for j < len(f.toks) && !f.toks[j].punct("=") && !f.toks[j].punct(";") {
				j++
			}
		}
		if j+1 >= len(f.toks) || !f.toks[j].punct("=") {
			continue
		}
		end := f.exprEnd(j + 1)
		leaves = f.constLeaves(leaves, name, j+1, end, 0)
		i = end - 1
	}
	found := false
	for _, l := range leaves {
		if strings.HasPrefix(l.Value, "/") {
			found = true
			break
		}
	}
	if !found {
		return Result{}
	}
	var out []PathConst
	for _, l := range leaves {
		if constLooksLikePath(l.Value) {
			l.File = p
			out = append(out, l)
		}
	}
	return Result{Consts: out}
}

// constLooksLikePath bounds a leaf: short, no whitespace, quote or angle.
func constLooksLikePath(v string) bool {
	if len(v) > maxConstLen {
		return false
	}
	return !strings.ContainsAny(v, " \t\r\n`'\"<>\x00\x02\x1e\x1f")
}

// constLeaves appends the string leaves of the expression [lo, hi) named
// name: a string literal, or an object literal's members, recursively. A
// call, spread, template substitution or anything else adds none.
func (f *jsFile) constLeaves(out []PathConst, name string, lo, hi, depth int) []PathConst {
	// `as const`, `satisfies T`: a trailing assertion.
	for k := lo; k < hi; k++ {
		if f.toks[k].is(tkIdent, "as") || f.toks[k].is(tkIdent, "satisfies") {
			hi = k
			break
		}
	}
	if hi-lo == 1 {
		t := f.toks[lo]
		if t.kind == tkString || (t.kind == tkTemplate && len(t.subs) == 0) {
			if len(out) < maxConstLeaves {
				out = append(out, PathConst{Name: name, Value: t.val, Line: t.line})
			}
		}
		return out
	}
	if depth >= maxConstDepth || hi-lo < 2 || !f.toks[lo].punct("{") || f.closeOf(lo) != hi-1 {
		return out
	}
	for _, a := range f.args(lo) {
		k := a[0]
		if f.tok(k).kind != tkIdent && f.tok(k).kind != tkString || !f.tok(k+1).punct(":") {
			continue
		}
		out = f.constLeaves(out, name+"."+f.toks[k].val, k+2, a[1], depth+1)
	}
	return out
}

// pathRef reads [lo, hi) as a member chain of an imported binding
// (`paths.app.users.path`; a trailing `as const` allowed): the deferred
// reference element for it, else false. Only an import of something other
// than React Router, bound by name or as a namespace, is a reference.
func (r *rrReader) pathRef(lo, hi int) (string, bool) {
	f := r.f
	if hi-lo >= 3 && f.toks[hi-2].is(tkIdent, "as") {
		hi -= 2
	}
	if lo >= hi || (hi-lo)%2 == 0 {
		return "", false
	}
	var chain []string
	for k := lo; k < hi; k++ {
		if (k-lo)%2 == 0 {
			if f.toks[k].kind != tkIdent {
				return "", false
			}
			chain = append(chain, f.toks[k].val)
		} else if !f.toks[k].punct(".") {
			return "", false
		}
	}
	imp, ok := f.imports[chain[0]]
	if !ok || rrModules[imp.spec] || imp.spec == "" {
		return "", false
	}
	switch imp.name {
	case "default":
		return "", false
	case "*":
		if len(chain) < 2 {
			return "", false
		}
		return refElem(imp.spec, strings.Join(chain[1:], ".")), true
	}
	return refElem(imp.spec, imp.name+strings.Join(append([]string{""}, chain[1:]...), ".")), true
}

// ---------------------------------------------------------------------
// Compose: resolving deferred paths.
// ---------------------------------------------------------------------

// pathConsts indexes one service's PathConst rows: file -> name -> value.
type pathConsts map[string]map[string]string

func newPathConsts(files []Result) pathConsts {
	pc := pathConsts{}
	for _, f := range files {
		for _, c := range f.Consts {
			m := pc[c.File]
			if m == nil {
				m = map[string]string{}
				pc[c.File] = m
			}
			m[c.Name] = c.Value
		}
	}
	return pc
}

// resolveImportedPath turns a deferred path of file from into a literal path; false when
// any reference does not resolve.
func (pc pathConsts) resolveImportedPath(from, p string) (string, bool) {
	if !isDeferred(p) {
		return p, true
	}
	acc := ""
	for _, e := range deferElems(p) {
		v := ""
		switch {
		case strings.HasPrefix(e, "L"):
			v = e[1:]
		case strings.HasPrefix(e, "R"):
			spec, chain, ok := strings.Cut(e[1:], refSep)
			if !ok {
				return "", false
			}
			file, ok := pc.module(from, spec)
			if !ok {
				return "", false
			}
			if v, ok = pc[file][chain]; !ok {
				return "", false
			}
		default:
			return "", false
		}
		if strings.HasPrefix(v, "/") {
			acc = joinRoute(v)
		} else {
			acc = joinRoute(acc, v)
		}
	}
	return acc, true
}

// jsModuleExts are the extensions a module specifier may omit.
var jsModuleExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"}

// moduleBase is a file path without its JS extension, and without a
// trailing "/index".
func moduleBase(file string) (string, bool) {
	ext := path.Ext(file)
	for _, e := range jsModuleExts {
		if ext == e {
			b := strings.TrimSuffix(file, ext)
			return strings.TrimSuffix(b, "/index"), true
		}
	}
	return "", false
}

// module resolves an import specifier of file from to the file (with
// constants) it names, no source read:
//
//   - "./x" or "../x": relative to from's directory;
//   - otherwise an alias or baseUrl import ("@/config/paths",
//     "~/config/paths", "src/config/paths"): the specifier minus its alias
//     lead ("@/", "~/", "#/") is a path suffix; among the constant modules
//     whose extensionless path ends with it, the one sharing the longest
//     directory prefix with from, and none when two tie.
func (pc pathConsts) module(from, spec string) (string, bool) {
	spec = mountTarget(spec)
	var want string
	if relSpec(spec) {
		want = path.Join(path.Dir(from), spec)
		for _, f := range sortedKeys(pc) {
			if b, ok := moduleBase(f); ok && (b == want || b == strings.TrimSuffix(want, "/index")) {
				return f, true
			}
		}
		return "", false
	}
	for _, lead := range []string{"@/", "~/", "#/"} {
		if rest, ok := strings.CutPrefix(spec, lead); ok {
			spec = rest
			break
		}
	}
	if spec == "" || strings.HasPrefix(spec, "/") {
		return "", false
	}
	best, bestN, tie := "", -1, false
	for _, f := range sortedKeys(pc) {
		b, ok := moduleBase(f)
		if !ok || (b != spec && !strings.HasSuffix(b, "/"+spec)) {
			continue
		}
		n := commonDirs(from, f)
		switch {
		case n > bestN:
			best, bestN, tie = f, n, false
		case n == bestN:
			tie = true
		}
	}
	return best, best != "" && !tie
}

// commonDirs counts the leading directories a and b share.
func commonDirs(a, b string) int {
	as, bs := strings.Split(path.Dir(a), "/"), strings.Split(path.Dir(b), "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

func sortedKeys(m pathConsts) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
