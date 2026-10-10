package routes

// flask.go — the Flask route reader (LB-F335, wave c2 item phppy), with the
// flask-restx / flask-restful resource idiom Flask applications commonly
// serve their APIs through.
//
// What it reads, in a module that imports flask (or flask_restx,
// flask_restful, flask_restplus):
//
//   - the objects routes hang on: X = Flask(...), X = Blueprint(name,
//     import_name, url_prefix="/p"), X = Api(app_or_blueprint, prefix=...),
//     X = Namespace(name, path="/p");
//   - @X.route(rule, methods=[...]) on a function (GET when no methods;
//     the handler its name), and on a Resource or MethodView class (one
//     route per HTTP method the class defines, handler Class.method);
//     @X.get/post/put/patch/delete(rule); X.add_url_rule(rule, endpoint,
//     view_func, methods=[...]) with a function or a local
//     Class.as_view(...); api.add_resource(Resource, rule, ...);
//   - composition: X.register_blueprint(Y, url_prefix=...) and
//     api.add_namespace(ns, path) serve a local Y or ns under X (a given
//     url_prefix or path replaces the child's own, as Flask and flask-restx
//     do); a Y or ns from another module is a Mount of that module; an
//     Api bound to a local blueprint serves under its url_prefix;
//   - a module that defines a Blueprint or a Namespace and imports
//     submodules of its own package (from . import views) mounts them under
//     that object's prefix: their @bp.route routes are the object's.
//
// A rule that is not a literal skips the route.

import (
	"bytes"
	stdpath "path"
	"strings"
)

func init() {
	Register(Reader{Framework: "flask", Langs: []string{"py"}, Read: readFlask})
}

var flaskHTTPMethods = []string{"get", "post", "put", "patch", "delete", "head", "options"}

type flaskKind uint8

const (
	fkApp flaskKind = iota
	fkBlueprint
	fkAPI
	fkNamespace
)

type flaskObj struct {
	kind   flaskKind
	prefix string // the object's own url_prefix / path
	nsPath string // a Namespace's path when added without one ("/" + name)
	routes []Route
	incs   []flaskInclude
}

type flaskInclude struct {
	child   string
	target  string
	prefix  string
	replace bool // the include's prefix replaces the child's own
	line    int
}

type flaskReader struct {
	path    string
	imports map[string]pyImport
	classes map[string]*pyNode
	objs    map[string]*flaskObj
	order   []string
	out     Result
}

func readFlask(path string, src []byte) Result {
	if isTestPath(path) || !(bytes.Contains(src, []byte("flask")) || bytes.Contains(src, []byte("Flask"))) {
		return Result{}
	}
	stmts := parsePython(src)
	r := &flaskReader{path: path, imports: pyImports(stmts), classes: pyClasses(stmts), objs: map[string]*flaskObj{}}
	r.stmts(stmts)
	r.submoduleMounts(stmts)
	r.emit()
	return r.out
}

func (r *flaskReader) stmts(stmts []*pyNode) {
	for _, s := range stmts {
		switch s.kind {
		case qAssign:
			r.assign(s)
		case qDef:
			r.decorated(s, "")
			r.stmts(s.body)
		case qClass:
			r.decorated(s, s.name)
		case qIf, qBlock:
			r.stmts(s.body)
			r.stmts(s.orelse)
		case qExprStmt:
			r.call(s.x)
		}
	}
}

func (r *flaskReader) define(name string, o *flaskObj) {
	if _, seen := r.objs[name]; !seen {
		r.order = append(r.order, name)
	}
	r.objs[name] = o
}

func (r *flaskReader) assign(s *pyNode) {
	v := s.x
	if v == nil || v.kind != qCall || len(s.targets) == 0 || s.targets[0].kind != qName {
		return
	}
	name := s.targets[0].name
	fn := pyLastName(pyDotted(v.x))
	if strings.HasSuffix(fn, "Flask") { // Flask, or an application's Flask subclass
		r.define(name, &flaskObj{kind: fkApp})
		return
	}
	switch fn {
	case "Blueprint":
		o := &flaskObj{kind: fkBlueprint}
		if p := pyArg(v, -1, "url_prefix"); p != nil {
			s, ok := pyStr(p)
			if !ok {
				return
			}
			o.prefix = s
		}
		r.define(name, o)
	case "Namespace":
		o := &flaskObj{kind: fkNamespace}
		if n, ok := pyStr(pyArg(v, 0, "name")); ok {
			o.nsPath = "/" + n
		}
		if p := pyArg(v, -1, "path"); p != nil {
			s, ok := pyStr(p)
			if !ok {
				return
			}
			o.prefix = s
			o.nsPath = s
		}
		r.define(name, o)
	case "Api":
		o := &flaskObj{kind: fkAPI}
		prefix := ""
		if p := pyArg(v, -1, "prefix"); p != nil {
			s, ok := pyStr(p)
			if !ok {
				return
			}
			prefix = s
		}
		r.define(name, o)
		parent := pyArg(v, 0, "app")
		if parent != nil && parent.kind == qName {
			if po := r.objs[parent.name]; po != nil {
				po.incs = append(po.incs, flaskInclude{child: name, prefix: prefix, line: s.line})
				return
			}
		}
		o.prefix = prefix
	}
}

// classMethods lists the HTTP methods a class defines (get, post, ...), or
// its literal `methods = [...]` attribute.
func (r *flaskReader) classMethods(cls *pyNode) []string {
	if cls == nil {
		return nil
	}
	for _, s := range cls.body {
		if s.kind == qAssign && len(s.targets) == 1 && s.targets[0].kind == qName && s.targets[0].name == "methods" {
			if ms := pyStrings(s.x); len(ms) > 0 {
				return ms
			}
		}
	}
	var out []string
	for _, m := range pyMethods(cls) {
		for _, h := range flaskHTTPMethods {
			if m.name == h {
				out = append(out, h)
			}
		}
	}
	return out
}

// decorated reads route decorators on a function (class "") or a class.
func (r *flaskReader) decorated(d *pyNode, class string) {
	for _, deco := range d.decos {
		if deco.kind != qCall || deco.x == nil || deco.x.kind != qAttr {
			continue
		}
		recv := pyDotted(deco.x.x)
		o := r.objs[recv]
		if o == nil {
			if _, imported := r.imports[recv]; !imported {
				continue
			}
		}
		verb := deco.x.name
		var rules []string
		var methods []string
		switch verb {
		case "route":
			for i := 0; ; i++ {
				a := pyArg(deco, i, "")
				if a == nil {
					break
				}
				s, ok := pyStr(a)
				if !ok {
					rules = nil
					break
				}
				rules = append(rules, s)
			}
			if len(rules) == 0 {
				if s, ok := pyStr(pyArg(deco, -1, "rule")); ok {
					rules = []string{s}
				}
			}
			methods = upperAll(pyStrings(pyArg(deco, -1, "methods")))
		case "get", "post", "put", "patch", "delete":
			if class != "" {
				continue
			}
			s, ok := pyStr(pyArg(deco, 0, "rule"))
			if !ok {
				continue
			}
			rules = []string{s}
			methods = []string{strings.ToUpper(verb)}
		default:
			continue
		}
		for _, rule := range rules {
			if class != "" {
				for _, m := range r.classMethods(d) {
					r.add(o, strings.ToUpper(m), rule, class+"."+strings.ToLower(m), deco.line)
				}
				continue
			}
			ms := methods
			if len(ms) == 0 {
				ms = []string{"GET"}
			}
			for _, m := range dropHeadBesideGet(ms) {
				r.add(o, m, rule, d.name, deco.line)
			}
		}
	}
}

func (r *flaskReader) add(o *flaskObj, method, rule, handler string, line int) {
	rt := Route{Method: method, Path: flaskRule(rule), Handler: handler, File: r.path, Line: line}
	if o == nil {
		r.out.Routes = append(r.out.Routes, rt)
		return
	}
	o.routes = append(o.routes, rt)
}

// flaskRule spells a Werkzeug rule's <converter:name> segments as {name}.
func flaskRule(rule string) string {
	return djangoConverterRe.ReplaceAllString(flaskConverterArgs(rule), "{$1}")
}

// flaskConverterArgs drops a converter's arguments (<int(min=1):id>,
// <any(a, b):kind>) so the converter pattern reads its name.
func flaskConverterArgs(rule string) string {
	if !strings.Contains(rule, "(") {
		return rule
	}
	var b strings.Builder
	in, paren := false, 0
	for i := 0; i < len(rule); i++ {
		c := rule[i]
		switch {
		case c == '<':
			in = true
		case c == '>':
			in = false
		case in && c == '(':
			paren++
			continue
		case in && c == ')':
			paren--
			continue
		}
		if paren > 0 {
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func (r *flaskReader) call(e *pyNode) {
	if e == nil || e.kind != qCall || e.x == nil || e.x.kind != qAttr {
		return
	}
	recv := pyDotted(e.x.x)
	o := r.objs[recv]
	if o == nil {
		return
	}
	switch e.x.name {
	case "register_blueprint", "add_namespace":
		childArg := pyArg(e, 0, "blueprint")
		prefixKey := "url_prefix"
		if e.x.name == "add_namespace" {
			childArg = pyArg(e, 0, "ns")
			prefixKey = "path"
		}
		inc := flaskInclude{line: e.line}
		if p := pyArg(e, 1, prefixKey); p != nil && (e.x.name == "add_namespace" || pyArg(e, -1, "url_prefix") != nil) {
			s, ok := pyStr(p)
			if !ok {
				return
			}
			inc.prefix = s
			inc.replace = true
		}
		switch {
		case childArg == nil:
			return
		case childArg.kind == qName && r.objs[childArg.name] != nil:
			inc.child = childArg.name
			if !inc.replace && r.objs[childArg.name].kind == fkNamespace {
				inc.prefix = r.objs[childArg.name].nsPath
				inc.replace = true
			}
		case childArg.kind == qName:
			imp, ok := r.imports[childArg.name]
			if !ok {
				return
			}
			inc.target = pyModuleTarget(imp, false)
		case childArg.kind == qAttr && childArg.x != nil && childArg.x.kind == qName:
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
	case "add_url_rule":
		rule, ok := pyStr(pyArg(e, 0, "rule"))
		if !ok {
			return
		}
		view := pyArg(e, 2, "view_func")
		if view == nil {
			return
		}
		methods := upperAll(pyStrings(pyArg(e, -1, "methods")))
		if view.kind == qCall && view.x != nil && view.x.kind == qAttr && view.x.name == "as_view" {
			cls := pyLastName(pyDotted(view.x.x))
			local := r.classes[cls]
			if local == nil {
				return
			}
			cms := r.classMethods(local)
			for _, m := range cms {
				if len(methods) > 0 && !containsFold(methods, m) {
					continue
				}
				r.add(o, strings.ToUpper(m), rule, cls+"."+strings.ToLower(m), e.line)
			}
			return
		}
		h := pyLastName(pyDotted(view))
		if h == "" {
			return
		}
		if len(methods) == 0 {
			methods = []string{"GET"}
		}
		for _, m := range dropHeadBesideGet(methods) {
			r.add(o, m, rule, h, e.line)
		}
	case "add_resource":
		res := pyLastName(pyDotted(pyArg(e, 0, "resource")))
		local := r.classes[res]
		if local == nil {
			return
		}
		for i := 1; ; i++ {
			a := pyArg(e, i, "")
			if a == nil {
				break
			}
			rule, ok := pyStr(a)
			if !ok {
				continue
			}
			for _, m := range r.classMethods(local) {
				r.add(o, strings.ToUpper(m), rule, res+"."+strings.ToLower(m), e.line)
			}
		}
	}
}

func containsFold(list []string, s string) bool {
	for _, l := range list {
		if strings.EqualFold(l, s) {
			return true
		}
	}
	return false
}

// submoduleMounts mounts the submodules of this module's own package that
// it imports, under its Blueprint's or Namespace's prefix, when it defines
// exactly one such object.
func (r *flaskReader) submoduleMounts(stmts []*pyNode) {
	var owner *flaskObj
	for _, name := range r.order {
		o := r.objs[name]
		if o.kind != fkBlueprint && o.kind != fkNamespace {
			continue
		}
		if owner != nil {
			return
		}
		owner = o
	}
	if owner == nil {
		return
	}
	dir := pathDir(r.path)
	pkgDir := dir
	if stdpath.Base(r.path) != "__init__.py" {
		return
	}
	seen := map[string]bool{}
	var walk func([]*pyNode)
	walk = func(ns []*pyNode) {
		for _, s := range ns {
			switch s.kind {
			case qFrom:
				for _, nm := range s.names {
					t := ""
					switch {
					case s.level == 1 && s.name == "":
						t = "./" + nm[0]
					case s.level == 0 && s.name != "" && (pkgDir == strings.ReplaceAll(s.name, ".", "/") || strings.HasSuffix(pkgDir, "/"+strings.ReplaceAll(s.name, ".", "/"))):
						t = s.name + "." + nm[0]
					}
					if t == "" || seen[t] || r.objs[nm[0]] != nil {
						continue
					}
					seen[t] = true
					p := owner.prefix
					if p == "" {
						p = "/"
					}
					r.out.Mounts = append(r.out.Mounts, Mount{Prefix: p, Target: t, File: r.path, Line: s.line})
				}
			case qIf, qBlock:
				walk(s.body)
				walk(s.orelse)
			}
		}
	}
	walk(stmts)
}

// emit serves each local object's routes: an object a local
// register_blueprint, add_namespace or Api(...) includes only through that
// include, every other one at its own prefix.
func (r *flaskReader) emit() {
	included := map[string]bool{}
	for _, name := range r.order {
		for _, inc := range r.objs[name].incs {
			if inc.child != "" {
				included[inc.child] = true
			}
		}
	}
	for _, name := range r.order {
		if !included[name] {
			r.serve(name, "", false, map[string]bool{}, 0)
		}
	}
}

func (r *flaskReader) serve(name, prefix string, replaced bool, onPath map[string]bool, depth int) {
	o := r.objs[name]
	if o == nil || onPath[name] || depth > 8 {
		return
	}
	onPath[name] = true
	defer delete(onPath, name)
	base := prefix
	if !replaced {
		base = joinFlask(prefix, o.prefix)
	}
	for _, rt := range o.routes {
		rt.Path = joinFlask(base, rt.Path)
		r.out.Routes = append(r.out.Routes, rt)
	}
	for _, inc := range o.incs {
		if inc.child != "" {
			r.serve(inc.child, joinFlask(base, inc.prefix), inc.replace, onPath, depth+1)
			continue
		}
		p := joinFlask(base, inc.prefix)
		if p == "" {
			p = "/"
		}
		r.out.Mounts = append(r.out.Mounts, Mount{Prefix: p, Target: inc.target, File: r.path, Line: inc.line})
	}
}

// joinFlask joins a prefix and a rule the way Flask's blueprint setup state
// does: the prefix's trailing slash and the rule's leading one meet as one.
func joinFlask(prefix, rule string) string {
	switch {
	case prefix == "":
		return rule
	case rule == "":
		return prefix
	}
	return strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(rule, "/")
}
