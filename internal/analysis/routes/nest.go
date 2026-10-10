package routes

// nest.go — the NestJS reader (LB-F326).

func init() {
	Register(Reader{Framework: "nest", Langs: []string{"js", "ts"}, Read: readNest})
}

// nestMethods are Nest's route decorators (@nestjs/common) and the method
// each registers; @All is ANY, @Sse a GET.
var nestMethods = map[string]string{
	"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE", "Patch": "PATCH",
	"Options": "OPTIONS", "Head": "HEAD", "All": "ANY", "Search": "SEARCH", "Sse": "GET",
	"Propfind": "PROPFIND", "Proppatch": "PROPPATCH", "Mkcol": "MKCOL", "Copy": "COPY",
	"Move": "MOVE", "Lock": "LOCK", "Unlock": "UNLOCK",
}

var nestPrefilter = [][]byte{[]byte("@Controller"), []byte("@Module"), []byte("setGlobalPrefix")}

// readNest reads NestJS routes and the mounts that place them (LB-F326).
//
//   - A class decorated @Controller(prefix | [prefixes] | {path, version})
//     contributes one route per route decorator (@Get, @Post, @Put,
//     @Delete, @Patch, @All, @Head, @Options, @Search, @Sse and the WebDAV
//     set) on its methods: the controller path joined with the method
//     path, handler Class.method.
//   - A version (@Version on the method, else the controller's version)
//     is the URI segment v{n}; VERSION_NEUTRAL adds none. URI versioning
//     is assumed: a header or media-type scheme is configured in main.ts,
//     which this file cannot see.
//   - A @Module mounts what it lists: each controller and each imported
//     module that is a repo file is a Mount under "", or under the path a
//     RouterModule.register entry of the same file gives it (children
//     joined to their parent's path). The module tree is how a prefix
//     reaches every controller.
//   - app.setGlobalPrefix('api') beside NestFactory.create(AppModule)
//     mounts the root module's file under /api (the app's source
//     directory when the root module is not a repo import). Its exclude
//     list cannot be written as a Mount and is not read.
func readNest(p string, src []byte) Result {
	if !anyContains(src, nestPrefilter) {
		return Result{}
	}
	f := newJSFile(p, src)
	n := nestReader{f: f}
	n.controllers()
	n.modules()
	n.globalPrefix()
	sortRoutes(n.out.Routes)
	sortMounts(n.out.Mounts)
	return n.out
}

type nestReader struct {
	f   *jsFile
	out Result
}

func (n *nestReader) controllers() {
	f := n.f
	for _, c := range f.classes {
		ctrl := -1
		for _, d := range c.decorators {
			if name, _ := f.decorator(d); name == "Controller" {
				ctrl = d
			}
		}
		if ctrl < 0 || c.name == "" {
			continue
		}
		_, open := f.decorator(ctrl)
		paths, versions, ok := n.controllerArgs(open)
		if !ok {
			continue
		}
		n.members(c, paths, versions)
	}
}

// controllerArgs reads @Controller's argument: none, a path, an array of
// paths, or an options object {path, version}.
func (n *nestReader) controllerArgs(open int) (paths, versions []string, ok bool) {
	f := n.f
	if open < 0 {
		return []string{""}, nil, true
	}
	args := f.args(open)
	if len(args) == 0 {
		return []string{""}, nil, true
	}
	a := args[0]
	if f.toks[a[0]].punct("{") && f.closeOf(a[0]) == a[1]-1 {
		props := f.props(a[0])
		paths = []string{""}
		if v, has := props["path"]; has {
			if paths, ok = f.stringsAt(v[0], v[1]); !ok {
				return nil, nil, false
			}
		}
		if v, has := props["version"]; has {
			versions = n.versionValues(v[0], v[1])
		}
		return paths, versions, true
	}
	paths, ok = f.stringsAt(a[0], a[1])
	return paths, nil, ok
}

// versionValues reads a version: a string, an array, or VERSION_NEUTRAL
// (written as "" — no segment).
func (n *nestReader) versionValues(lo, hi int) []string {
	f := n.f
	if hi-lo == 1 && f.toks[lo].is(tkIdent, "VERSION_NEUTRAL") {
		return []string{""}
	}
	if f.toks[lo].punct("[") && f.closeOf(lo) == hi-1 {
		var out []string
		for _, a := range f.args(lo) {
			out = append(out, n.versionValues(a[0], a[1])...)
		}
		return out
	}
	if s, ok := f.stringAt(lo, hi); ok {
		return []string{s}
	}
	return nil
}

// members walks a controller's body: decorators, then the member they
// decorate.
func (n *nestReader) members(c jsClass, ctrlPaths, ctrlVersions []string) {
	f := n.f
	var pending []int
	for k := c.open + 1; k < c.close; k++ {
		t := f.toks[k]
		switch {
		case t.punct("@"):
			pending = append(pending, k)
			k = f.decoratorEnd(k)
		case t.punct(";") || t.punct("="):
			pending = nil
			if t.punct("=") {
				k = f.exprEnd(k+1) - 1
			}
		case t.punct("{") || t.punct("[") || t.punct("("):
			if e := f.closeOf(k); e > 0 {
				k = e
			}
		case t.kind == tkIdent && (f.tok(k+1).punct("(") || f.tok(k+1).punct("<")) && !nestModifier[t.val]:
			n.member(c, t.val, pending, ctrlPaths, ctrlVersions)
			pending = nil
			o := k + 1
			if f.tok(o).punct("<") {
				for o < c.close && !f.toks[o].punct("(") {
					o++
				}
			}
			if e := f.closeOf(o); e > 0 {
				k = e
			}
		}
	}
}

var nestModifier = map[string]bool{
	"async": true, "public": true, "private": true, "protected": true, "static": true,
	"override": true, "readonly": true, "abstract": true, "declare": true,
}

func (n *nestReader) member(c jsClass, method string, decs []int, ctrlPaths, ctrlVersions []string) {
	f := n.f
	versions := ctrlVersions
	type route struct {
		method string
		paths  []string
		line   int
	}
	var rs []route
	for _, d := range decs {
		name, open := f.decorator(d)
		if name == "Version" && open >= 0 {
			if args := f.args(open); len(args) > 0 {
				versions = n.versionValues(args[0][0], args[0][1])
			}
			continue
		}
		m := nestMethods[name]
		if m == "" {
			continue
		}
		paths := []string{""}
		if open >= 0 {
			if args := f.args(open); len(args) > 0 {
				var ok bool
				if paths, ok = f.stringsAt(args[0][0], args[0][1]); !ok {
					continue
				}
			}
		}
		rs = append(rs, route{m, paths, f.toks[d].line})
	}
	if len(versions) == 0 {
		versions = []string{""}
	}
	for _, r := range rs {
		for _, v := range versions {
			seg := ""
			if v != "" {
				seg = "v" + v
			}
			for _, cp := range ctrlPaths {
				for _, mp := range r.paths {
					n.out.Routes = append(n.out.Routes, Route{
						Method: r.method, Path: joinRoute(seg, cp, mp),
						Handler: c.name + "." + method, File: f.path, Line: r.line,
					})
				}
			}
		}
	}
}

// modules reads each @Module({imports, controllers}) and the
// RouterModule.register paths of the same file.
func (n *nestReader) modules() {
	f := n.f
	paths := n.routerModulePaths()
	for _, c := range f.classes {
		for _, d := range c.decorators {
			name, open := f.decorator(d)
			if name != "Module" || open < 0 {
				continue
			}
			args := f.args(open)
			if len(args) == 0 || !f.toks[args[0][0]].punct("{") {
				continue
			}
			props := f.props(args[0][0])
			for _, key := range []string{"imports", "controllers"} {
				v, ok := props[key]
				if !ok || !f.toks[v[0]].punct("[") {
					continue
				}
				for _, a := range f.args(v[0]) {
					local := n.moduleRef(a[0], a[1])
					if local == "" {
						continue
					}
					imp, ok := f.imports[local]
					if !ok || !relSpec(imp.spec) {
						continue
					}
					prefixes := []string{""}
					if ps, ok := paths[local]; ok {
						prefixes = ps
					}
					for _, pre := range prefixes {
						n.out.Mounts = append(n.out.Mounts, Mount{Prefix: mountPrefix(pre), Target: mountTarget(imp.spec), File: f.path, Line: f.toks[a[0]].line})
					}
				}
			}
		}
	}
}

// moduleRef names the module or controller an imports/controllers entry
// refers to: X, X.forRoot(…), X.register(…), forwardRef(() => X).
func (n *nestReader) moduleRef(lo, hi int) string {
	f := n.f
	t := f.tok(lo)
	if t.kind != tkIdent {
		return ""
	}
	if t.val == "forwardRef" && f.tok(lo+1).punct("(") {
		for k := lo + 2; k < hi; k++ {
			if f.toks[k].punct("=>") && f.tok(k+1).kind == tkIdent {
				return f.toks[k+1].val
			}
		}
		return ""
	}
	if hi-lo == 1 || f.tok(lo+1).punct(".") {
		return t.val
	}
	return ""
}

// routerModulePaths reads RouterModule.register([{path, module, children}])
// into module name -> full paths.
func (n *nestReader) routerModulePaths() map[string][]string {
	f := n.f
	out := map[string][]string{}
	for i := range f.toks {
		if !f.toks[i].is(tkIdent, "RouterModule") || !f.tok(i+1).punct(".") || !f.tok(i+2).is(tkIdent, "register") || !f.tok(i+3).punct("(") {
			continue
		}
		args := f.args(i + 3)
		if len(args) == 0 || !f.toks[args[0][0]].punct("[") {
			continue
		}
		n.routerEntries(args[0][0], "", out, 0)
	}
	return out
}

func (n *nestReader) routerEntries(open int, parent string, out map[string][]string, depth int) {
	f := n.f
	if depth > 8 {
		return
	}
	for _, a := range f.args(open) {
		if !f.toks[a[0]].punct("{") {
			continue
		}
		props := f.props(a[0])
		p := ""
		if v, ok := props["path"]; ok {
			s, ok := f.stringAt(v[0], v[1])
			if !ok {
				continue
			}
			p = s
		}
		full := joinRoute(parent, p)
		if v, ok := props["module"]; ok && v[1]-v[0] == 1 && f.toks[v[0]].kind == tkIdent {
			out[f.toks[v[0]].val] = appendUnique(out[f.toks[v[0]].val], full)
		}
		if v, ok := props["children"]; ok && f.toks[v[0]].punct("[") {
			n.routerEntries(v[0], full, out, depth+1)
		}
	}
}

// globalPrefix reads app.setGlobalPrefix('api') in the file that calls
// NestFactory.create(Root).
func (n *nestReader) globalPrefix() {
	f := n.f
	root := ""
	for i := range f.toks {
		if f.toks[i].is(tkIdent, "NestFactory") && f.tok(i+1).punct(".") && f.tok(i+2).kind == tkIdent {
			o := i + 3
			if f.tok(o).punct("<") {
				for o < len(f.toks) && o < i+40 && !f.toks[o].punct("(") {
					o++
				}
			}
			if args := f.args(o); len(args) > 0 && args[0][1]-args[0][0] == 1 {
				root = f.toks[args[0][0]].val
			}
		}
	}
	if root == "" {
		return
	}
	target := "./"
	if imp, ok := f.imports[root]; ok && relSpec(imp.spec) {
		target = mountTarget(imp.spec)
	}
	for i := range f.toks {
		if !f.toks[i].is(tkIdent, "setGlobalPrefix") || !f.tok(i-1).punct(".") || !f.tok(i+1).punct("(") {
			continue
		}
		args := f.args(i + 1)
		if len(args) == 0 {
			continue
		}
		prefix, ok := f.stringAt(args[0][0], args[0][1])
		if !ok {
			continue
		}
		n.out.Mounts = append(n.out.Mounts, Mount{Prefix: mountPrefix(prefix), Target: target, File: f.path, Line: f.toks[i].line})
	}
}

// decoratorEnd is the last token of the decorator at "@" index at.
func (f *jsFile) decoratorEnd(at int) int {
	j := at + 1
	for f.tok(j+1).punct(".") && f.tok(j+2).kind == tkIdent {
		j += 2
	}
	if f.tok(j + 1).punct("(") {
		if c := f.closeOf(j + 1); c > 0 {
			return c
		}
	}
	return j
}

// props reads an object literal opening at open: key -> value token range.
// A shorthand `{ path }` maps path to its own token.
func (f *jsFile) props(open int) map[string][2]int {
	out := map[string][2]int{}
	for _, a := range f.args(open) {
		lo, hi := a[0], a[1]
		if lo >= hi {
			continue
		}
		k := f.toks[lo]
		if k.kind != tkIdent && k.kind != tkString {
			continue
		}
		switch {
		case lo+1 < hi && f.toks[lo+1].punct(":"):
			out[k.val] = [2]int{lo + 2, hi}
		case hi-lo == 1 && k.kind == tkIdent:
			out[k.val] = [2]int{lo, hi}
		}
	}
	return out
}
