package routes

// fastapi.go — the FastAPI route reader (LB-F334, wave c2 item phppy).
//
// What it reads, in a module that imports fastapi:
//
//   - the application and its routers: X = FastAPI(...), X =
//     APIRouter(prefix="/p", ...);
//   - a path operation: @X.get/post/put/patch/delete/options/head/trace
//     (path) on a function (handler the function's name, Class.method in a
//     class body), @X.api_route(path, methods=[...]), @X.websocket(path)
//     (method WS), X.add_api_route(path, endpoint, methods=[...]);
//   - X.include_router(Y, prefix="/q"): a router Y of the same module is
//     served under X's prefix, then "/q", then Y's own prefix (so a router
//     included in its module is read only where it is included); a router
//     from another module is a Mount of that module under "/q" (its own
//     APIRouter prefix is applied in its module); X.mount("/q", Y) of a
//     sub-application the same way.
//
// A prefix or a path that is not a literal (an f-string over a setting,
// settings.API_V1_STR) is not guessed: an include or mount with one is
// skipped, a path operation with one too.

import (
	"bytes"
	"strings"
)

func init() {
	Register(Reader{Framework: "fastapi", Langs: []string{"py"}, Read: readFastAPI})
}

var fastapiVerbs = map[string]string{
	"get": "GET", "post": "POST", "put": "PUT", "patch": "PATCH", "delete": "DELETE",
	"options": "OPTIONS", "head": "HEAD", "trace": "TRACE", "websocket": "WS",
}

type fastapiObj struct {
	app    bool   // a FastAPI application (never itself included)
	prefix string // an APIRouter's own prefix
	routes []Route
	incs   []fastapiInclude
}

type fastapiInclude struct {
	child  string // a local object's name, or "" for a Mount
	target string // the Mount Target of a router from another module
	prefix string
	line   int
}

type fastapiReader struct {
	path    string
	imports map[string]pyImport
	objs    map[string]*fastapiObj
	order   []string
	out     Result
}

func readFastAPI(path string, src []byte) Result {
	if isTestPath(path) || !bytes.Contains(src, []byte("fastapi")) {
		return Result{}
	}
	stmts := parsePython(src)
	r := &fastapiReader{path: path, imports: pyImports(stmts), objs: map[string]*fastapiObj{}}
	r.stmts(stmts, "")
	r.emit()
	return r.out
}

func (r *fastapiReader) obj(name string) *fastapiObj {
	if o, ok := r.objs[name]; ok {
		return o
	}
	return nil
}

func (r *fastapiReader) stmts(stmts []*pyNode, class string) {
	for _, s := range stmts {
		switch s.kind {
		case qAssign:
			r.assign(s)
		case qDef:
			r.def(s, class)
			r.stmts(s.body, class)
		case qClass:
			r.stmts(s.body, s.name)
		case qIf, qBlock:
			r.stmts(s.body, class)
			r.stmts(s.orelse, class)
		case qExprStmt:
			r.call(s.x)
		}
	}
}

func (r *fastapiReader) assign(s *pyNode) {
	v := s.x
	if v == nil || v.kind != qCall {
		return
	}
	fn := pyLastName(pyDotted(v.x))
	var o *fastapiObj
	switch fn {
	case "FastAPI":
		o = &fastapiObj{app: true}
	case "APIRouter":
		o = &fastapiObj{}
		if p := pyArg(v, -1, "prefix"); p != nil {
			s, ok := pyStr(p)
			if !ok {
				o.prefix = "{}"
			} else {
				o.prefix = s
			}
		}
	default:
		return
	}
	for _, t := range s.targets {
		if t.kind == qName {
			if _, seen := r.objs[t.name]; !seen {
				r.order = append(r.order, t.name)
			}
			r.objs[t.name] = o
		}
	}
}

// def reads a function's path-operation decorators.
func (r *fastapiReader) def(d *pyNode, class string) {
	for _, deco := range d.decos {
		if deco.kind != qCall || deco.x == nil || deco.x.kind != qAttr {
			continue
		}
		recv := pyDotted(deco.x.x)
		o := r.obj(recv)
		if o == nil && !r.importedRouter(recv) {
			continue
		}
		verb := deco.x.name
		var methods []string
		switch {
		case fastapiVerbs[verb] != "":
			methods = []string{fastapiVerbs[verb]}
		case verb == "api_route":
			methods = upperAll(pyStrings(pyArg(deco, -1, "methods")))
			if len(methods) == 0 {
				methods = []string{"GET"}
			}
		default:
			continue
		}
		path, ok := pyStr(pyArg(deco, 0, "path"))
		if !ok {
			continue
		}
		h := d.name
		if class != "" {
			h = class + "." + d.name
		}
		r.add(recv, o, methods, path, h, deco.line)
	}
}

// importedRouter reports a decorator receiver imported from another module
// (a router defined there): its routes are read at local paths.
func (r *fastapiReader) importedRouter(name string) bool {
	_, ok := r.imports[name]
	return ok
}

func (r *fastapiReader) add(recv string, o *fastapiObj, methods []string, path, handler string, line int) {
	if strings.Contains(path, "{}") {
		return
	}
	for _, m := range dropHeadBesideGet(methods) {
		rt := Route{Method: m, Path: path, Handler: handler, File: r.path, Line: line}
		if o == nil {
			r.out.Routes = append(r.out.Routes, rt)
			continue
		}
		o.routes = append(o.routes, rt)
	}
}

func dropHeadBesideGet(ms []string) []string {
	return symfonyDropHead(append([]string(nil), ms...))
}

func upperAll(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, strings.ToUpper(s))
	}
	return out
}

// call reads X.include_router(...), X.mount(...) and X.add_api_route(...).
func (r *fastapiReader) call(e *pyNode) {
	if e == nil || e.kind != qCall || e.x == nil || e.x.kind != qAttr {
		return
	}
	recv := pyDotted(e.x.x)
	o := r.obj(recv)
	if o == nil {
		return
	}
	switch e.x.name {
	case "include_router", "mount":
		childArg := pyArg(e, 0, "router")
		prefixArg := pyArg(e, -1, "prefix")
		if e.x.name == "mount" {
			prefixArg = pyArg(e, 0, "path")
			childArg = pyArg(e, 1, "app")
		}
		prefix := ""
		if prefixArg != nil {
			s, ok := pyStr(prefixArg)
			if !ok {
				return
			}
			prefix = s
		}
		inc := fastapiInclude{prefix: prefix, line: e.line}
		switch {
		case childArg == nil:
			return
		case childArg.kind == qName && r.obj(childArg.name) != nil:
			inc.child = childArg.name
		case childArg.kind == qName:
			imp, ok := r.imports[childArg.name]
			if !ok {
				return
			}
			inc.target = pyModuleTarget(imp, false)
		case childArg.kind == qAttr && childArg.x.kind == qName:
			imp, ok := r.imports[childArg.x.name]
			if !ok {
				return
			}
			inc.target = pyModuleTarget(imp, true)
		default:
			return
		}
		if inc.child == "" && inc.target == "" {
			return
		}
		o.incs = append(o.incs, inc)
	case "add_api_route", "add_api_websocket_route":
		path, ok := pyStr(pyArg(e, 0, "path"))
		if !ok {
			return
		}
		ep := pyLastName(pyDotted(pyArg(e, 1, "endpoint")))
		methods := upperAll(pyStrings(pyArg(e, -1, "methods")))
		if e.x.name == "add_api_websocket_route" {
			methods = []string{"WS"}
		}
		if len(methods) == 0 {
			methods = []string{"GET"}
		}
		r.add(recv, o, methods, path, ep, e.line)
	}
}

// emit serves each local object's routes: an object included by another
// local object only through that include, every other one (an app, or a
// router its module never includes) at its own prefix.
func (r *fastapiReader) emit() {
	included := map[string]bool{}
	for _, name := range r.order {
		for _, inc := range r.objs[name].incs {
			if inc.child != "" {
				included[inc.child] = true
			}
		}
	}
	for _, name := range r.order {
		if included[name] {
			continue
		}
		r.serve(name, "", map[string]bool{}, 0)
	}
}

func (r *fastapiReader) serve(name, prefix string, onPath map[string]bool, depth int) {
	o := r.objs[name]
	if o == nil || onPath[name] || depth > 8 {
		return
	}
	onPath[name] = true
	defer delete(onPath, name)
	base := prefix + o.prefix
	if strings.Contains(base, "{}") {
		return
	}
	for _, rt := range o.routes {
		rt.Path = base + rt.Path
		if rt.Path == "" {
			rt.Path = "/"
		}
		r.out.Routes = append(r.out.Routes, rt)
	}
	for _, inc := range o.incs {
		if inc.child != "" {
			r.serve(inc.child, base+inc.prefix, onPath, depth+1)
			continue
		}
		p := base + inc.prefix
		if p == "" {
			p = "/"
		}
		r.out.Mounts = append(r.out.Mounts, Mount{Prefix: p, Target: inc.target, File: r.path, Line: inc.line})
	}
}
