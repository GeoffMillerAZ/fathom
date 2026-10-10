package routes

// fastify.go — the Fastify reader (LB-F327).

import "strings"

func init() {
	Register(Reader{Framework: "fastify", Langs: []string{"js", "ts"}, Read: readFastify})
}

// fastifyMethods are the shorthand route methods; "all" is ANY.
var fastifyMethods = map[string]string{
	"get": "GET", "head": "HEAD", "post": "POST", "put": "PUT", "delete": "DELETE",
	"options": "OPTIONS", "patch": "PATCH", "all": "ANY",
}

// fastifyPrefilter: a file that never names fastify registers no route.
var fastifyPrefilter = [][]byte{[]byte("fastify"), []byte("Fastify")}

// fastifyReceiverNames are the parameter names a plugin's instance goes
// by; any parameter typed Fastify… is one too.
var fastifyReceiverNames = map[string]bool{
	"fastify": true, "app": true, "server": true, "instance": true, "f": true, "api": true, "router": true,
}

// readFastify reads Fastify routes and mounts (LB-F327).
//
//   - fastify.METHOD(path, [options,] handler) and
//     fastify.route({method, url, handler}) register routes (a method
//     array yields one route per method; "all" is ANY).
//   - A receiver is a name bound to Fastify() / require('fastify')(), a
//     plugin function's first parameter (passed to register, typed
//     Fastify…, or named fastify, app, server, instance, …), or any such
//     conventional name, in a file that imports fastify or names it. The
//     shorthand methods chain: fastify.get(…).post(…).
//   - fastify.register(plugin, {prefix}) of an inline plugin reads the
//     plugin's routes under the prefix; of a repo module (an import,
//     require('./x'), import('./x')) it is a Mount of that module.
//   - @fastify/autoload: register(autoload, {dir: join(__dirname,
//     'routes'), options: {prefix}}) mounts the directory; a route
//     plugin under a routes/ directory reads its folders as its prefix
//     (autoload's dirNameRoutePrefix), plus an exported autoPrefix.
//   - The handler is the handler function's name, "" when inline.
func readFastify(p string, src []byte) Result {
	if !anyContains(src, fastifyPrefilter) {
		return Result{}
	}
	f := newJSFile(p, src)
	r := fastifyReader{f: f, roots: map[string]bool{}}
	if !r.gate() {
		return Result{}
	}
	r.bind()
	r.dirPrefix = r.autoloadPrefix()
	r.read()
	sortRoutes(r.out.Routes)
	sortMounts(r.out.Mounts)
	return r.out
}

type fastifyReader struct {
	f         *jsFile
	factory   map[string]bool // names bound to the fastify module
	autoload  map[string]bool // names bound to @fastify/autoload
	roots     map[string]bool // names bound to an instance: Fastify()
	dirPrefix string
	out       Result
}

func (r *fastifyReader) gate() bool {
	r.factory, r.autoload = map[string]bool{}, map[string]bool{}
	fastifyish := false
	for local, imp := range r.f.imports {
		switch {
		case imp.spec == "express":
			return false
		case imp.spec == "fastify":
			fastifyish = true
			if imp.name == "default" || imp.name == "*" || imp.name == "fastify" || imp.name == "Fastify" {
				r.factory[local] = true
			}
		case imp.spec == "@fastify/autoload" || imp.spec == "fastify-autoload":
			fastifyish = true
			r.autoload[local] = true
		case strings.Contains(imp.spec, "fastify"):
			fastifyish = true
		}
	}
	if !fastifyish {
		for _, t := range r.f.toks {
			if t.kind == tkIdent && (t.val == "fastify" || strings.HasPrefix(t.val, "Fastify")) {
				return true
			}
		}
	}
	return fastifyish
}

// bind finds names bound to Fastify(…) / require('fastify')(…).
func (r *fastifyReader) bind() {
	f := r.f
	for i := range f.toks {
		created := false
		if spec, c, ok := f.requireSpec(i); ok && (spec == "fastify" || strings.HasSuffix(spec, "/fastify")) && f.tok(c+1).punct("(") {
			created = true
		} else if t := f.toks[i]; t.kind == tkIdent && r.factory[t.val] && f.tok(i+1).punct("(") && !f.tok(i-1).punct(".") {
			created = true
		}
		if !created {
			continue
		}
		if f.tok(i-1).punct("=") && f.tok(i-2).kind == tkIdent {
			r.roots[f.toks[i-2].val] = true
		}
		if f.tok(i-1).is(tkIdent, "await") && f.tok(i-2).punct("=") && f.tok(i-3).kind == tkIdent {
			r.roots[f.toks[i-3].val] = true
		}
	}
}

// receiverFunc reports whether the identifier at i is an instance and, for
// a plugin parameter, which function declares it.
func (r *fastifyReader) receiverFunc(i int) (jsFunc, bool, bool) {
	f := r.f
	t := f.tok(i)
	if t.kind != tkIdent || f.tok(i-1).punct(".") {
		return jsFunc{}, false, false
	}
	fn, ok := f.enclosing(i)
	for ok {
		for k, p := range fn.params {
			if p == t.val {
				if k == 0 && (fastifyReceiverNames[p] || r.typedFastify(fn, p) || r.inlinePlugin(fn)) {
					return fn, true, true
				}
				return jsFunc{}, false, false // shadowed by another parameter
			}
		}
		fn, ok = f.enclosing(fn.open - 1)
	}
	// A root instance: bound to Fastify(), or a conventional instance name
	// (fastify, app, server, …) in this Fastify file.
	if r.roots[t.val] || fastifyReceiverNames[t.val] {
		return jsFunc{}, false, true
	}
	return jsFunc{}, false, false
}

// inlinePlugin reports whether fn is the plugin argument of a register call.
func (r *fastifyReader) inlinePlugin(fn jsFunc) bool {
	f := r.f
	return fn.callArg > 0 && fn.argIndex == 0 && f.tok(fn.callArg-1).is(tkIdent, "register") && f.tok(fn.callArg-2).punct(".")
}

// typedFastify reports whether parameter p of fn is annotated with a
// Fastify… type.
func (r *fastifyReader) typedFastify(fn jsFunc, p string) bool {
	f := r.f
	for k := fn.open - 1; k >= 0 && k > fn.open-60; k-- {
		if f.toks[k].is(tkIdent, p) && f.tok(k+1).punct(":") && strings.HasPrefix(f.tok(k+2).val, "Fastify") {
			return true
		}
	}
	return false
}

// prefixes returns the paths the receiver at i registers under: an inline
// plugin's parameter inherits its register call's prefixes.
func (r *fastifyReader) prefixes(i int, depth int) []string {
	fn, isParam, _ := r.receiverFunc(i)
	if !isParam || fn.callArg < 0 || fn.argIndex != 0 || depth > 8 {
		if isParam {
			return []string{r.dirPrefix}
		}
		return []string{""}
	}
	f := r.f
	call := fn.callArg // the "(" of Q.register(fn, opts)
	if !f.tok(call-1).is(tkIdent, "register") || !f.tok(call-2).punct(".") {
		return []string{r.dirPrefix}
	}
	q := call - 3
	if _, _, ok := r.receiverFunc(q); !ok {
		return []string{""}
	}
	own := r.registerPrefix(call)
	var out []string
	for _, pp := range r.prefixes(q, depth+1) {
		out = appendUnique(out, joinRoute(pp, own))
	}
	return out
}

// registerPrefix reads the prefix option of the register call opening at
// call.
func (r *fastifyReader) registerPrefix(call int) string {
	f := r.f
	args := f.args(call)
	if len(args) < 2 || !f.toks[args[1][0]].punct("{") {
		return ""
	}
	if v, ok := f.props(args[1][0])["prefix"]; ok {
		if s, ok := f.stringAt(v[0], v[1]); ok {
			return s
		}
	}
	return ""
}

func (r *fastifyReader) read() {
	f := r.f
	for i := range f.toks {
		if !f.tok(i+1).punct(".") || f.tok(i+2).kind != tkIdent || !f.tok(i+3).punct("(") {
			continue
		}
		if _, _, ok := r.receiverFunc(i); !ok {
			continue
		}
		// fastify.get(…).post(…): each shorthand returns the instance.
		for open := i + 3; open > 0; {
			r.call(i, open)
			c := f.closeOf(open)
			if c < 0 || !f.tok(c+1).punct(".") || f.tok(c+2).kind != tkIdent || !f.tok(c+3).punct("(") {
				break
			}
			open = c + 3
		}
	}
}

// call reads one instance method call whose "(" is at open, on the
// receiver at i.
func (r *fastifyReader) call(i, open int) {
	f := r.f
	m := f.toks[open-1].val
	switch {
	case m == "register":
		r.register(i, open)
	case m == "route":
		r.route(i, open)
	case fastifyMethods[m] != "":
		args := f.args(open)
		if len(args) < 2 {
			return
		}
		paths, ok := f.stringsAt(args[0][0], args[0][1])
		if !ok || !fastifyPaths(paths) {
			return
		}
		h := ""
		last := args[len(args)-1]
		if f.toks[last[0]].punct("{") && f.closeOf(last[0]) == last[1]-1 {
			if v, ok := f.props(last[0])["handler"]; ok {
				h = jsHandlerName(f, v[0], v[1])
			}
		} else {
			h = jsHandlerName(f, last[0], last[1])
		}
		r.emit([]string{fastifyMethods[m]}, paths, h, i, open)
	}
}

func fastifyPaths(ps []string) bool {
	for _, p := range ps {
		if p != "" && p != "*" && !strings.HasPrefix(p, "/") {
			return false
		}
	}
	return len(ps) > 0
}

func (r *fastifyReader) emit(methods, paths []string, h string, i, open int) {
	f := r.f
	for _, pre := range r.prefixes(i, 0) {
		for _, m := range methods {
			for _, p := range paths {
				r.out.Routes = append(r.out.Routes, Route{Method: m, Path: joinRoute(pre, p), Handler: h, File: f.path, Line: f.toks[open-1].line})
			}
		}
	}
}

// route reads fastify.route({method, url | path, handler}).
func (r *fastifyReader) route(i, open int) {
	f := r.f
	args := f.args(open)
	if len(args) != 1 || !f.toks[args[0][0]].punct("{") {
		return
	}
	props := f.props(args[0][0])
	mv, ok := props["method"]
	if !ok {
		return
	}
	methods, ok := f.stringsAt(mv[0], mv[1])
	if !ok {
		return
	}
	for k, m := range methods {
		methods[k] = strings.ToUpper(m)
	}
	uv, ok := props["url"]
	if !ok {
		uv, ok = props["path"]
	}
	if !ok {
		return
	}
	paths, ok := f.stringsAt(uv[0], uv[1])
	if !ok || !fastifyPaths(paths) {
		return
	}
	h := ""
	if v, ok := props["handler"]; ok {
		h = jsHandlerName(f, v[0], v[1])
	}
	r.emit(methods, paths, h, i, open)
}

// register reads fastify.register(target, {prefix}): a Mount when target is
// a repo module or the autoloaded routes directory.
func (r *fastifyReader) register(i, open int) {
	f := r.f
	args := f.args(open)
	if len(args) == 0 {
		return
	}
	lo, hi := args[0][0], args[0][1]
	target := ""
	prefix := r.registerPrefix(open)
	if hi-lo == 1 && r.autoload[f.toks[lo].val] {
		target, prefix = r.autoloadDir(args)
	} else if spec, ok := r.module(lo, hi); ok {
		target = mountTarget(spec)
	}
	if target == "" {
		return
	}
	for _, pre := range r.prefixes(i, 0) {
		r.out.Mounts = append(r.out.Mounts, Mount{Prefix: mountPrefix(pre, prefix), Target: target, File: f.path, Line: f.toks[open-1].line})
	}
}

// module resolves a register target to a repo module: an imported name,
// require('./x'), require('./x').default, import('./x').
func (r *fastifyReader) module(lo, hi int) (string, bool) {
	f := r.f
	if spec, c, ok := f.requireSpec(lo); ok && (c == hi-1 || c+3 == hi) {
		return spec, relSpec(spec)
	}
	if f.toks[lo].is(tkIdent, "import") && f.tok(lo+1).punct("(") && f.tok(lo+2).kind == tkString && f.closeOf(lo+1) == hi-1 {
		return f.toks[lo+2].val, relSpec(f.toks[lo+2].val)
	}
	if f.toks[lo].kind == tkIdent && (hi-lo == 1 || hi-lo == 3 && f.tok(lo+1).punct(".")) {
		if imp, ok := f.imports[f.toks[lo].val]; ok && relSpec(imp.spec) {
			return imp.spec, true
		}
	}
	return "", false
}

// autoloadDir reads an autoload registration's dir and options.prefix:
// dir is a string or path.join(__dirname | import.meta.dirname, 'a', 'b').
func (r *fastifyReader) autoloadDir(args [][2]int) (string, string) {
	f := r.f
	if len(args) < 2 || !f.toks[args[1][0]].punct("{") {
		return "", ""
	}
	props := f.props(args[1][0])
	prefix := ""
	if o, ok := props["options"]; ok && f.toks[o[0]].punct("{") {
		if v, ok := f.props(o[0])["prefix"]; ok {
			if s, ok := f.stringAt(v[0], v[1]); ok {
				prefix = s
			}
		}
	}
	d, ok := props["dir"]
	if !ok {
		return "", ""
	}
	if s, ok := f.stringAt(d[0], d[1]); ok {
		return "./" + strings.Trim(strings.TrimPrefix(s, "./"), "/") + "/", prefix
	}
	// path.join(base, 'a', 'b') / join(base, 'a')
	k := d[0]
	for k < d[1] && !f.toks[k].punct("(") {
		k++
	}
	if k >= d[1] || f.tok(k-1).val != "join" && f.tok(k-1).val != "resolve" {
		return "", ""
	}
	parts := f.args(k)
	if len(parts) < 2 {
		return "", ""
	}
	base := f.toks[parts[0][0]]
	if !(base.is(tkIdent, "__dirname") || base.is(tkIdent, "import") && f.tok(parts[0][0]+2).is(tkIdent, "meta")) {
		return "", ""
	}
	var segs []string
	for _, a := range parts[1:] {
		s, ok := f.stringAt(a[0], a[1])
		if !ok {
			return "", ""
		}
		segs = append(segs, strings.Trim(s, "/"))
	}
	return "./" + strings.Join(segs, "/") + "/", prefix
}

// autoloadPrefix is the prefix autoload gives a route plugin under a
// routes/ directory: its folders below routes/, plus an exported
// autoPrefix. "" for any other file.
func (r *fastifyReader) autoloadPrefix() string {
	f := r.f
	segs := strings.Split(f.path, "/")
	at := -1
	for k := 0; k < len(segs)-1; k++ {
		if segs[k] == "routes" {
			at = k
		}
	}
	if at < 0 || !r.exportsPlugin() {
		return ""
	}
	pre := joinRoute(segs[at+1 : len(segs)-1]...)
	for i, t := range f.toks {
		// export const autoPrefix = '/x' / module.exports.autoPrefix = '/x'
		if t.is(tkIdent, "autoPrefix") && f.tok(i+1).punct("=") && f.tok(i+2).kind == tkString && (f.tok(i-1).punct(".") || f.tok(i-1).is(tkIdent, "const")) {
			pre = joinRoute(pre, f.toks[i+2].val)
			break
		}
	}
	if pre == "/" {
		return ""
	}
	return pre
}

// exportsPlugin reports a default export or module.exports assignment.
func (r *fastifyReader) exportsPlugin() bool {
	f := r.f
	for i, t := range f.toks {
		if t.is(tkIdent, "export") && f.tok(i+1).is(tkIdent, "default") {
			return true
		}
		if t.is(tkIdent, "module") && f.tok(i+1).punct(".") && f.tok(i+2).is(tkIdent, "exports") && f.tok(i+3).punct("=") {
			return true
		}
	}
	return false
}
