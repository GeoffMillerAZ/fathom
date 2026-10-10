package routes

// reactrouter.go — the React Router reader (LB-F378, wave c3, item react).
//
// React Router (v6 and v7, react-router and react-router-dom) declares its
// routes as route objects or as <Route> elements, both read here:
//
//   - createBrowserRouter, createHashRouter, createMemoryRouter,
//     createStaticHandler and useRoutes over an array literal (or a const of
//     the file bound to one); a const typed RouteObject[] even when no router
//     call in the file takes it (a routes module);
//   - createRoutesFromElements(<Route …>) and <Routes> holding <Route>
//     elements.
//
// A route object (or element) with a path or `index` that renders something
// (element, Component or lazy) is a GET route: a client route is a page view,
// as a Next.js page is (nextjs.go). Its path is its parent's joined with its
// own (a path starting with "/" stands alone), an index route's is its
// parent's, and a pathless layout route adds no segment and is no route of its
// own. Its handler is the component it renders: the tag of `element` (through
// a wrapper with one component child: <Suspense><Page/></Suspense> is Page),
// the Component it names, or, for lazy(), the Component its returned object
// names, else React Router's convention for `lazy: () => import('./x')` (the
// module's own `Component` export). A path the file cannot resolve (an
// imported constant) leaves the route and its children unread. A splat route
// ("dashboard/*") whose component is imported from a module of the repo (or
// is a React.lazy of one) mounts that module under its path, so the module's
// own <Routes> (descendant routes) compose below it.
//
// The JS token scanner (express.go) reads JSX text as code (an apostrophe opens
// a string, a "</" may open a regular expression), so this reader lexes with
// lexJSX: the same tokens for code, and a JSX element's tags, attributes and
// expression containers as tokens of their own, its text skipped.

import (
	"bytes"
	"path"
	"sort"
	"strings"
)

func init() {
	Register(Reader{Framework: "react-router", Langs: []string{"js", "ts"}, Read: readReactRouter})
}

// rrModules are the packages React Router's API is imported from.
var rrModules = map[string]bool{"react-router": true, "react-router-dom": true, "react-router/dom": true, "react-router-native": true}

// rrFactories take an array of route objects as their first argument.
var rrFactories = map[string]bool{"createBrowserRouter": true, "createHashRouter": true, "createMemoryRouter": true, "createStaticHandler": true, "useRoutes": true}

// rrExts are the files the reader reads; JSX is lexed in all but .ts.
var rrExts = map[string]bool{".js": true, ".jsx": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".mts": true, ".cts": true}

type rrReader struct {
	f      *jsFile
	elems  map[int]*jsxElem  // the element opening at each "<" token
	api    map[string]string // a local name bound to React Router -> the name it exports
	arrays map[string]int    // a const bound to an array literal -> its "[" token
	lazies map[string]string // a const bound to lazy(() => import('spec')) -> spec
	routes []Route
	mounts []Mount
}

// readReactRouter reads React Router routes (LB-F378).
func readReactRouter(p string, src []byte) Result {
	ext := strings.ToLower(path.Ext(p))
	if !rrExts[ext] || strings.HasSuffix(p, ".d.ts") || strings.Contains("/"+p, "/node_modules/") {
		return Result{}
	}
	if !bytes.Contains(src, []byte("react-router")) {
		return Result{}
	}
	toks, roots := lexJSX(src, ext != ".ts" && ext != ".mts" && ext != ".cts")
	f := &jsFile{path: p, toks: toks}
	f.pairBrackets()
	f.consts = f.collectConsts()
	f.imports = f.collectImports()
	r := &rrReader{f: f, elems: map[int]*jsxElem{}, api: map[string]string{}, arrays: map[string]int{}, lazies: map[string]string{}}
	var index func(e *jsxElem)
	index = func(e *jsxElem) {
		r.elems[e.open] = e
		for _, c := range e.children {
			index(c)
		}
		for _, a := range e.attrs {
			for _, n := range a.elems {
				index(n)
			}
		}
	}
	for _, e := range roots {
		index(e)
	}
	for local, imp := range f.imports {
		if rrModules[imp.spec] {
			r.api[local] = imp.name
		}
	}
	routeObjectTyped := r.collectArrays()
	if len(r.api) == 0 && len(routeObjectTyped) == 0 {
		return Result{}
	}
	read := map[int]bool{} // array literals already read (a const passed twice)
	for i := 0; i+1 < len(toks); i++ {
		t := toks[i]
		if t.kind != tkIdent || !toks[i+1].punct("(") {
			continue
		}
		name := r.api[t.val]
		if i > 1 && toks[i-1].punct(".") { // RR.createBrowserRouter(…) through a namespace import
			name = ""
			if toks[i-2].kind == tkIdent && r.api[toks[i-2].val] == "*" && (i < 3 || !toks[i-3].punct(".")) {
				name = t.val
			}
		}
		switch name {
		case "createRoutesFromElements", "createRoutesFromChildren":
			if args := f.args(i + 1); len(args) > 0 {
				if e := r.elemAt(args[0][0], args[0][1]); e != nil {
					r.elementRoutes([]*jsxElem{e}, "")
				}
			}
		default:
			if !rrFactories[name] {
				continue
			}
			args := f.args(i + 1)
			if len(args) == 0 {
				continue
			}
			if open, ok := r.arrayAt(args[0][0], args[0][1]); ok && !read[open] {
				read[open] = true
				r.objectRoutes(open, "")
			}
		}
	}
	for _, open := range routeObjectTyped {
		if !read[open] {
			read[open] = true
			r.objectRoutes(open, "")
		}
	}
	var walk func(es []*jsxElem)
	walk = func(es []*jsxElem) {
		for _, e := range es {
			if r.apiName(e.name) == "Routes" {
				r.elementRoutes(e.children, "")
				continue
			}
			walk(e.children)
			for _, k := range sortedAttrNames(e.attrs) {
				walk(e.attrs[k].elems)
			}
		}
	}
	walk(roots)
	return Result{Routes: r.routes, Mounts: r.mounts}
}

// apiName is the React Router export a tag or callee names: through a named
// import, or a namespace import's member (<RR.Route>); "" when none.
func (r *rrReader) apiName(name string) string {
	if n, ok := r.api[name]; ok && n != "*" {
		return n
	}
	if head, tail, ok := strings.Cut(name, "."); ok && r.api[head] == "*" && !strings.Contains(tail, ".") {
		return tail
	}
	return ""
}

// sortedAttrNames are an element's attribute names, sorted.
func sortedAttrNames(m map[string]jsxAttr) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// collectArrays records each `const|let|var NAME [: T] = [` of the file and
// returns the "[" of those typed RouteObject[].
func (r *rrReader) collectArrays() []int {
	f := r.f
	var typed []int
	for i := 0; i+3 < len(f.toks); i++ {
		t := f.toks[i]
		if t.kind != tkIdent || (t.val != "const" && t.val != "let" && t.val != "var") || f.toks[i+1].kind != tkIdent {
			continue
		}
		name := f.toks[i+1].val
		j := i + 2
		isRouteObjects := false
		if f.toks[j].punct(":") {
			for j < len(f.toks) && !f.toks[j].punct("=") && !f.toks[j].punct(";") {
				if f.toks[j].is(tkIdent, "RouteObject") {
					isRouteObjects = true
				}
				j++
			}
		}
		if j+1 < len(f.toks) && f.toks[j].punct("=") {
			if spec, ok := r.lazyImport(j + 1); ok {
				r.lazies[name] = spec
			}
		}
		if j+1 >= len(f.toks) || !f.toks[j].punct("=") || !f.toks[j+1].punct("[") {
			continue
		}
		r.arrays[name] = j + 1
		if isRouteObjects {
			typed = append(typed, j+1)
		}
	}
	return typed
}

// arrayAt resolves [lo, hi) to an array literal's "[": the literal itself or a
// const of the file bound to one.
func (r *rrReader) arrayAt(lo, hi int) (int, bool) {
	f := r.f
	if lo < hi && f.toks[lo].punct("[") && f.closeOf(lo) == hi-1 {
		return lo, true
	}
	if hi-lo == 1 && f.toks[lo].kind == tkIdent {
		open, ok := r.arrays[f.toks[lo].val]
		return open, ok
	}
	return 0, false
}

// elemAt is the JSX element [lo, hi) holds, through parentheses.
func (r *rrReader) elemAt(lo, hi int) *jsxElem {
	f := r.f
	for lo < hi && f.toks[lo].punct("(") {
		lo++
	}
	return r.elems[lo]
}

// objectRoutes reads the route objects of the array opening at open.
func (r *rrReader) objectRoutes(open int, parent string) {
	f := r.f
	for _, a := range f.args(open) {
		lo, hi := a[0], a[1]
		if hi-lo == 2 && f.toks[lo].punct("...") && f.toks[lo+1].kind == tkIdent { // ...otherRoutes
			if o, ok := r.arrays[f.toks[lo+1].val]; ok && o != open {
				r.objectRoutes(o, parent)
			}
			continue
		}
		if lo < hi && f.toks[lo].punct("{") && f.closeOf(lo) == hi-1 {
			r.objectRoute(lo, parent)
		}
	}
}

// objectRoute reads one route object opening at open.
func (r *rrReader) objectRoute(open int, parent string) {
	f := r.f
	var (
		p, handler          string
		hasPath, isIndex    bool
		renders, unresolved bool
		children            = -1
	)
	for _, a := range f.args(open) {
		lo, hi := a[0], a[1]
		k := lo
		if f.tok(k).is(tkIdent, "async") {
			k++
		}
		key := f.tok(k)
		if key.kind != tkIdent && key.kind != tkString {
			continue
		}
		if f.tok(k+1).punct("(") && key.val == "lazy" { // lazy() { … } (a method)
			renders = true
			if handler == "" {
				handler = r.lazyHandler(k+1, hi)
			}
			continue
		}
		if f.tok(k+1).punct("(") && key.val == "Component" { // Component() { … }: a function named Component
			renders, handler = true, "Component"
			continue
		}
		if !f.tok(k + 1).punct(":") {
			continue
		}
		vlo := k + 2
		switch key.val {
		case "path":
			s, ok := f.stringAt(vlo, hi)
			if !ok {
				unresolved = true
				continue
			}
			p, hasPath = s, true
		case "index":
			isIndex = hi-vlo == 1 && f.toks[vlo].is(tkIdent, "true")
		case "element":
			renders = true
			if e := r.elemAt(vlo, hi); e != nil {
				handler = rrComponentOf(e)
			}
		case "Component":
			renders = true
			if hi-vlo == 1 && f.toks[vlo].kind == tkIdent {
				handler = f.toks[vlo].val
			}
		case "lazy":
			renders = true
			if handler == "" {
				handler = r.lazyHandler(vlo, hi)
			}
		case "children":
			if o, ok := r.arrayAt(vlo, hi); ok {
				children = o
			}
		}
	}
	if unresolved {
		return
	}
	full := rrJoin(parent, p, hasPath)
	if (hasPath || isIndex) && renders {
		r.routes = append(r.routes, Route{Method: "GET", Path: full, Handler: handler, File: f.path, Line: f.toks[open].line})
		r.descendants(full, handler, f.toks[open].line)
	}
	if children >= 0 {
		r.objectRoutes(children, full)
	}
}

// lazyHandler names the component a lazy() at [lo, hi) loads: the Component
// (or element) its returned object names, else, for a bare import() of a
// module, React Router's convention (the module's `Component` export); "" when
// it cannot tell (the import's result is transformed by a call).
func (r *rrReader) lazyHandler(lo, hi int) string {
	f := r.f
	imported := false
	for i := lo; i < hi; i++ {
		t := f.toks[i]
		switch {
		case t.is(tkIdent, "Component") && f.tok(i+1).punct(":") && f.tok(i+2).kind == tkIdent && !f.tok(i+3).punct("."):
			return f.toks[i+2].val
		case t.is(tkIdent, "element") && f.tok(i+1).punct(":"):
			if e := r.elemAt(i+2, hi); e != nil {
				return rrComponentOf(e)
			}
		case t.is(tkIdent, "import") && f.tok(i+1).punct("("):
			imported = true
			if c := f.closeOf(i + 1); c > 0 && f.tok(c+1).punct(".") && f.tok(c+2).is(tkIdent, "then") {
				return ""
			}
		}
	}
	if imported {
		return "Component"
	}
	return ""
}

// elementRoutes reads <Route> elements under parent.
func (r *rrReader) elementRoutes(elems []*jsxElem, parent string) {
	for _, e := range elems {
		if r.apiName(e.name) != "Route" {
			continue
		}
		var p, handler string
		hasPath, isIndex, renders := false, false, false
		if a, ok := e.attrs["path"]; ok {
			s, ok := r.attrString(a)
			if !ok {
				continue
			}
			p, hasPath = s, true
		}
		if a, ok := e.attrs["index"]; ok {
			isIndex = a.bare || (a.hi-a.lo == 1 && r.f.toks[a.lo].is(tkIdent, "true"))
		}
		if a, ok := e.attrs["element"]; ok {
			renders = true
			if len(a.elems) > 0 {
				handler = rrComponentOf(a.elems[0])
			}
		}
		if a, ok := e.attrs["Component"]; ok {
			renders = true
			if handler == "" && a.hi-a.lo == 1 && r.f.toks[a.lo].kind == tkIdent {
				handler = r.f.toks[a.lo].val
			}
		}
		if a, ok := e.attrs["lazy"]; ok {
			renders = true
			if handler == "" {
				handler = r.lazyHandler(a.lo, a.hi)
			}
		}
		full := rrJoin(parent, p, hasPath)
		if (hasPath || isIndex) && renders {
			r.routes = append(r.routes, Route{Method: "GET", Path: full, Handler: handler, File: r.f.path, Line: e.line})
			r.descendants(full, handler, e.line)
		}
		r.elementRoutes(e.children, full)
	}
}

// descendants mounts, under a splat route's path ("dashboard/*" ->
// /dashboard), the module of the repo its component is imported from: React
// Router serves that component's own <Routes> (descendant routes) below the
// route, which Compose applies across files (LB-F321's mounts).
func (r *rrReader) descendants(full, handler string, line int) {
	prefix, ok := strings.CutSuffix(full, "/*")
	if !ok || prefix == "" || handler == "" {
		return
	}
	spec := r.lazies[handler]
	if imp, ok := r.f.imports[handler]; ok {
		spec = imp.spec
	}
	if !relSpec(spec) {
		return
	}
	r.mounts = append(r.mounts, Mount{Prefix: prefix, Target: mountTarget(spec), File: r.f.path, Line: line})
}

// lazyImport reads `lazy(() => import('spec'))` or `React.lazy(…)` starting
// at i: the module a React.lazy component loads.
func (r *rrReader) lazyImport(i int) (string, bool) {
	f := r.f
	if f.tok(i).is(tkIdent, "React") && f.tok(i+1).punct(".") {
		i += 2
	}
	if !f.tok(i).is(tkIdent, "lazy") || !f.tok(i+1).punct("(") {
		return "", false
	}
	end := f.closeOf(i + 1)
	for k := i + 2; k > 0 && k < end; k++ {
		if f.toks[k].is(tkIdent, "import") && f.tok(k+1).punct("(") && f.tok(k+2).kind == tkString && f.tok(k+3).punct(")") {
			return f.toks[k+2].val, true
		}
	}
	return "", false
}

// attrString is a JSX attribute's string value: a literal, or an expression
// the file resolves to one.
func (r *rrReader) attrString(a jsxAttr) (string, bool) {
	if a.isStr {
		return a.str, true
	}
	if a.bare {
		return "", false
	}
	return r.f.stringAt(a.lo, a.hi)
}

// rrComponentOf names the component an element renders: its tag's last
// segment, through a wrapper whose only child is one component element.
func rrComponentOf(e *jsxElem) string {
	for len(e.children) == 1 && !e.text && rrComponentTag(e.children[0].name) {
		e = e.children[0]
	}
	if !rrComponentTag(e.name) {
		return ""
	}
	if i := strings.LastIndexByte(e.name, '.'); i >= 0 {
		return e.name[i+1:]
	}
	return e.name
}

// rrComponentTag: a capitalized or member tag names a component; a
// lower-case one is an HTML element, a fragment none.
func rrComponentTag(name string) bool {
	return name != "" && (strings.Contains(name, ".") || (name[0] >= 'A' && name[0] <= 'Z'))
}

// rrJoin is a route's full path: a path starting with "/" stands alone,
// another joins its parent's, and an index or pathless route keeps its
// parent's.
func rrJoin(parent, p string, hasPath bool) string {
	switch {
	case !hasPath:
		if parent == "" {
			return "/"
		}
		return parent
	case strings.HasPrefix(p, "/"):
		return joinRoute(p)
	}
	return joinRoute(parent, p)
}

// ---------------------------------------------------------------------
// lexJSX: express.go's scanner with JSX as tokens.
// ---------------------------------------------------------------------

// jsxElem is one JSX element: its tag ("" for a fragment), the line and token
// index of its "<", its attributes, child elements and whether it holds text.
type jsxElem struct {
	name     string
	line     int
	open     int
	attrs    map[string]jsxAttr
	children []*jsxElem
	text     bool
}

// jsxAttr is one attribute: a string value, a bare name, or an expression
// container whose tokens are [lo, hi) (the braces excluded) and whose
// outermost JSX elements are elems.
type jsxAttr struct {
	str    string
	isStr  bool
	bare   bool
	lo, hi int
	elems  []*jsxElem
}

type jsxLexer struct {
	jsLexer
	jsx   bool
	roots []*jsxElem
	// sink, when not nil, collects the outermost elements an expression
	// container holds (an attribute's value).
	sink *[]*jsxElem
}

// lexJSX splits src into express.go's tokens, lexing JSX elements (when jsx
// is true) as tokens of their own: "<", the tag as one identifier, each
// attribute's name, "=", a string or an expression container's tokens, ">" or
// "/" ">", and a closing tag's "<" "/" tag ">"; text between tags is skipped.
// It returns the tokens and the outermost elements in source order.
func lexJSX(src []byte, jsx bool) ([]jsTok, []*jsxElem) {
	l := &jsxLexer{jsLexer: jsLexer{src: src, line: 1}, jsx: jsx}
	if bytes.HasPrefix(src, []byte("#!")) {
		for l.i < len(src) && src[l.i] != '\n' {
			l.i++
		}
	}
	l.code(false)
	return l.out, l.roots
}

// code lexes JavaScript until the input ends or, when inBraces, until the
// "}" closing the container it was called in (emitted).
func (l *jsxLexer) code(inBraces bool) {
	src := l.src
	depth := 0
	for l.i < len(src) {
		c := src[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == '/' && l.i+1 < len(src) && src[l.i+1] == '/':
			for l.i < len(src) && src[l.i] != '\n' {
				l.i++
			}
		case c == '/' && l.i+1 < len(src) && src[l.i+1] == '*':
			end := bytes.Index(src[l.i+2:], []byte("*/"))
			stop := len(src)
			if end >= 0 {
				stop = l.i + 2 + end + 2
			}
			l.line += bytes.Count(src[l.i:stop], []byte("\n"))
			l.i = stop
		case c == '\'' || c == '"':
			l.str(c)
		case c == '`':
			start := l.line
			val, subs := l.template()
			l.out = append(l.out, jsTok{kind: tkTemplate, val: val, line: start, subs: subs})
		case c >= '0' && c <= '9' || c == '.' && l.i+1 < len(src) && src[l.i+1] >= '0' && src[l.i+1] <= '9':
			j := l.i + 1
			for j < len(src) && (isIdentByte(src[j], false) || src[j] == '.') {
				j++
			}
			l.out = append(l.out, jsTok{kind: tkNumber, val: string(src[l.i:j]), line: l.line})
			l.i = j
		case isIdentByte(c, true):
			j := l.i + 1
			for j < len(src) && isIdentByte(src[j], false) {
				j++
			}
			l.out = append(l.out, jsTok{kind: tkIdent, val: string(src[l.i:j]), line: l.line})
			l.i = j
		case c == '<' && l.jsx && l.regexAllowed() && l.i+1 < len(src) && (src[l.i+1] == '>' || isIdentByte(src[l.i+1], true)):
			e := l.element()
			if l.sink != nil {
				*l.sink = append(*l.sink, e)
			} else {
				l.roots = append(l.roots, e)
			}
		case c == '/' && l.regexAllowed():
			if !l.regex() {
				l.punct()
			}
		case c == '{':
			depth++
			l.punct()
		case c == '}':
			if inBraces && depth == 0 {
				l.punct()
				return
			}
			depth--
			l.punct()
		default:
			l.punct()
		}
	}
}

// emit appends a punctuator token and advances past it.
func (l *jsxLexer) emit(val string) {
	l.out = append(l.out, jsTok{kind: tkPunct, val: val, line: l.line})
	l.i += len(val)
}

// space skips whitespace, counting lines.
func (l *jsxLexer) space() {
	for l.i < len(l.src) {
		switch l.src[l.i] {
		case '\n':
			l.line++
		case ' ', '\t', '\r', '\f', '\v':
		default:
			return
		}
		l.i++
	}
}

// tag reads a tag or attribute name (dotted, namespaced or hyphenated) as one
// identifier token; "" when none starts here.
func (l *jsxLexer) tag() string {
	j := l.i
	for j < len(l.src) && (isIdentByte(l.src[j], j == l.i) || j > l.i && (l.src[j] == '.' || l.src[j] == '-' || l.src[j] == ':' || l.src[j] >= '0' && l.src[j] <= '9')) {
		j++
	}
	name := string(l.src[l.i:j])
	if name != "" {
		l.out = append(l.out, jsTok{kind: tkIdent, val: name, line: l.line})
		l.i = j
	}
	return name
}

// container lexes an expression container from its "{" to its "}", both
// emitted, and returns the token range between them and the outermost
// elements it holds.
func (l *jsxLexer) container() (int, int, []*jsxElem) {
	l.emit("{")
	lo := len(l.out)
	var held []*jsxElem
	saved := l.sink
	l.sink = &held
	l.code(true)
	l.sink = saved
	hi := len(l.out) - 1
	if hi < lo {
		hi = lo
	}
	return lo, hi, held
}

// element lexes one JSX element from its "<"; nested elements are its
// children.
func (l *jsxLexer) element() *jsxElem {
	e := &jsxElem{line: l.line, open: len(l.out), attrs: map[string]jsxAttr{}}
	l.emit("<")
	l.space()
	if l.i < len(l.src) && l.src[l.i] != '>' {
		e.name = l.tag()
	}
	for l.i < len(l.src) { // attributes
		l.space()
		if l.i >= len(l.src) {
			return e
		}
		switch c := l.src[l.i]; {
		case c == '/' && l.i+1 < len(l.src) && l.src[l.i+1] == '>':
			l.emit("/")
			l.emit(">")
			return e
		case c == '>':
			l.emit(">")
			l.children(e)
			return e
		case c == '{': // {...spread}
			l.container()
		default:
			name := l.tag()
			if name == "" {
				l.i++ // a byte no attribute starts with
				continue
			}
			l.space()
			if l.i >= len(l.src) || l.src[l.i] != '=' {
				e.attrs[name] = jsxAttr{bare: true}
				continue
			}
			l.emit("=")
			l.space()
			switch {
			case l.i < len(l.src) && (l.src[l.i] == '"' || l.src[l.i] == '\''):
				q := l.src[l.i]
				j := l.i + 1
				for j < len(l.src) && l.src[j] != q {
					j++
				}
				val := string(l.src[l.i+1 : min(j, len(l.src))])
				l.out = append(l.out, jsTok{kind: tkString, val: val, line: l.line})
				l.line += strings.Count(val, "\n")
				l.i = min(j+1, len(l.src))
				e.attrs[name] = jsxAttr{str: val, isStr: true}
			case l.i < len(l.src) && l.src[l.i] == '{':
				lo, hi, held := l.container()
				e.attrs[name] = jsxAttr{lo: lo, hi: hi, elems: held}
			case l.i < len(l.src) && l.src[l.i] == '<':
				child := l.element()
				e.attrs[name] = jsxAttr{elems: []*jsxElem{child}}
			}
		}
	}
	return e
}

// children lexes an element's children up to and including its closing tag.
func (l *jsxLexer) children(e *jsxElem) {
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == '<' && l.i+1 < len(l.src) && l.src[l.i+1] == '/':
			l.emit("<")
			l.emit("/")
			l.space()
			l.tag()
			l.space()
			if l.i < len(l.src) && l.src[l.i] == '>' {
				l.emit(">")
			}
			return
		case c == '<':
			e.children = append(e.children, l.element())
		case c == '{':
			// An expression container is a child of its own (React sees
			// whatever it evaluates to), so the element is no plain wrapper.
			l.container()
			e.text = true
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r':
			l.i++
		default:
			e.text = true
			l.i++
		}
	}
}
