package extractor

// timonidialect.go — the Timoni native dialect (LB-Q-timoni, docs/design/
// language-breadth.md §13, engine "native": cuelang.org/go's parser, the one
// the CUE extractor already reads with). A Timoni module, bundle or runtime is
// plain CUE with a contract: this extractor COMPOSES NewCUEExtractor() (every
// CUE node and reference edge the CUE extraction gives, unchanged) and adds
// the Timoni entities and references the contract carries. It edits nothing of
// the CUE extraction: cueextract.go owns that.
//
// Entities (dialects/timoni/dialect.json). Names are slash-separated so they
// never collide with a CUE field's dotted name:
//
//   - timoni/module (TimoniModule): a file with a top-level `timoni:` field
//     (timoni.cue, the module's entry);
//   - timoni/values (TimoniValues): a top-level `values:` field (values.cue,
//     debug_values.cue, images.cue, and the entry's `values: templates.#Config`);
//   - timoni/object/<#Def> (TimoniObject): a top-level definition whose value
//     (through `&` and parentheses) has a struct with a literal `kind:`: the
//     template of one Kubernetes object;
//   - timoni/bundle (TimoniBundle) and timoni/bundle/instance/<name>
//     (TimoniBundleInstance): a top-level `bundle:` field and each member of
//     its `instances`;
//   - timoni/runtime (TimoniRuntime) and timoni/runtime/cluster/<name>
//     (TimoniCluster): a top-level `runtime:` field and each member of its
//     `clusters`.
//
// References (L2, Calls): timoni/values calls #Config.<k> for each top-level
// key k of a struct-literal `values:` (a key is a field of the module's
// #Config, which Timoni validates the values against); a template definition
// whose body has a `#config: #Config` field calls #Config; #Instance.config ->
// #Config and `values: templates.#Config` are the CUE extraction's own edges.
//
// Cross-file references (L3, DocResult.CrossFile):
//
//   - a bundle instance's `module.url` of `file://<path>` is the module's
//     timoni.cue: a repo-path of the path read relative to the bundle file's
//     directory (Timoni's own rule). An oci:// url names a registry, a
//     non-literal url an expression: neither resolves, and
//     TimoniUnresolvedRefs counts them by reason;
//   - the #Instance definition (a module's rendering root) to each object
//     template it names, through a label-selector join scoped by the module
//     root (the parent of the nearest `templates` directory, else the file's
//     own directory) on the package and definition name. Timoni renders the
//     objects #Instance names, so this is "module -> its rendered Kubernetes
//     kinds" at the template's file;
//   - whatever the composed CUE extraction itself reports, when it reports
//     cross-file references (a DocResulter).

import (
	"path"
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const (
	kindTimoniModule         core.SourceKind = "TimoniModule"
	kindTimoniValues         core.SourceKind = "TimoniValues"
	kindTimoniObject         core.SourceKind = "TimoniObject"
	kindTimoniBundle         core.SourceKind = "TimoniBundle"
	kindTimoniBundleInstance core.SourceKind = "TimoniBundleInstance"
	kindTimoniRuntime        core.SourceKind = "TimoniRuntime"
	kindTimoniCluster        core.SourceKind = "TimoniCluster"

	// Edges of the Timoni dialect's cross-file references.
	timoniEdgeModule  = "module"
	timoniEdgeRenders = "renders"

	// TimoniScopePrefix is the label-selector join scope of a module's objects.
	timoniScopePrefix = "timoni:"
)

// TimoniExtractor implements Extractor and DocResulter for Timoni modules,
// bundles and runtimes, over the CUE extraction.
type TimoniExtractor struct {
	base *CUEExtractor
}

var (
	_ Extractor   = (*TimoniExtractor)(nil)
	_ DocResulter = (*TimoniExtractor)(nil)
)

// NewTimoniExtractor returns the timoni dialect's extractor.
func NewTimoniExtractor() *TimoniExtractor { return &TimoniExtractor{base: NewCUEExtractor()} }

// TimoniDialectExtractor is the extractor of the native dialect this file
// defines, by dialect id; ok is false for any other id.
func TimoniDialectExtractor(id string) (Extractor, bool) {
	if id == "timoni" {
		return NewTimoniExtractor(), true
	}
	return nil, false
}

// Extract implements Extractor.
func (x *TimoniExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := x.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// ExtractDoc implements DocResulter. A file the CUE parser cannot recover
// anything from yields nothing; one with syntax errors yields what its parsed
// declarations carry.
func (x *TimoniExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	r, _, err := x.extract(service, relPath, src)
	return r, err
}

// TimoniUnresolved is one module reference no static reading resolved, and why.
type TimoniUnresolved struct {
	Line int
	// Edge is the reference's edge ("module"); Raw the url as written.
	Edge, Raw string
	// Reason is "external:oci" (a registry module: no repository file),
	// "external:other" (a url scheme that is neither file nor oci),
	// "expression" (not a string literal), "absolute-path" or "outside-repo".
	Reason string
}

// TimoniUnresolvedRefs are the module references of one file that ExtractDoc
// leaves unresolved, each with its reason, in source order.
func TimoniUnresolvedRefs(relPath string, src []byte) []TimoniUnresolved {
	_, u, _ := NewTimoniExtractor().extract("svc", relPath, src)
	return u
}

// TimoniModuleRoot is the module root a path's join scope names: the parent of
// its nearest (deepest) `templates` directory, else its own directory.
func TimoniModuleRoot(relPath string) string {
	dir := path.Dir(strings.ReplaceAll(relPath, "\\", "/"))
	if dir == "." || dir == "" {
		return "."
	}
	segs := strings.Split(dir, "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if segs[i] == "templates" {
			if i == 0 {
				return "."
			}
			return strings.Join(segs[:i], "/")
		}
	}
	return dir
}

func (x *TimoniExtractor) extract(service, relPath string, src []byte) (DocResult, []TimoniUnresolved, error) {
	var res DocResult
	var baseErr error
	if d, ok := any(x.base).(DocResulter); ok {
		// The CUE extraction reports cross-file references of its own: kept.
		res, baseErr = d.ExtractDoc(service, relPath, src)
	} else {
		res.Nodes, baseErr = x.base.Extract(service, relPath, src)
	}
	f, _ := parser.ParseFile(relPath, src)
	if f == nil {
		return res, nil, baseErr
	}
	pkg := f.PackageName()
	if pkg == "" {
		pkg = pkgFromPath(relPath)
	}
	w := &timoniWalk{
		service: service, pkg: pkg, relPath: relPath,
		root:    TimoniModuleRoot(relPath),
		imports: timoniImportPaths(f),
		taken:   map[string]bool{},
	}
	for _, n := range res.Nodes {
		w.taken[n.Name] = true
	}
	var fields []*ast.Field
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.Field); ok {
			fields = append(fields, fd)
		}
	}
	for _, fd := range fields {
		name, _, err := ast.LabelName(fd.Label)
		if err != nil || name == "" {
			continue
		}
		switch name {
		case "timoni":
			w.module(fd)
		case "values":
			w.values(fd)
		case "bundle":
			w.bundle(fd)
		case "runtime":
			w.runtime(fd)
		default:
			if strings.HasPrefix(name, "#") {
				w.definition(name, fd)
			}
		}
	}
	// A template definition that takes the module's config: a reference of the
	// definition to #Config the field-value walk of the CUE extraction does not
	// reach (the field sits in a struct operand of a conjunction).
	w.attachConfigCalls(res.Nodes)

	res.Nodes = append(res.Nodes, w.nodes...)
	sort.SliceStable(res.Nodes, func(i, j int) bool {
		if res.Nodes[i].Line != res.Nodes[j].Line {
			return res.Nodes[i].Line < res.Nodes[j].Line
		}
		return res.Nodes[i].Name < res.Nodes[j].Name
	})
	res.CrossFile = append(res.CrossFile, w.cross...)
	res.Labels = append(res.Labels, w.labels...)
	if w.root0 != "" && len(res.Roots) == 0 {
		res.Roots = []string{w.root0}
	}
	return res, w.unresolved, baseErr
}

// timoniWalk accumulates one file's Timoni entities and references.
type timoniWalk struct {
	service, pkg, relPath, root string
	imports                     map[string]string // identifier -> import path
	taken                       map[string]bool
	nodes                       []core.SymbolNode
	cross                       []DocCrossFileRef
	labels                      []DocLabelSet
	unresolved                  []TimoniUnresolved
	root0                       string // the entity a file-level reference resolves to
	configDefs                  map[string]bool
}

// timoniImportPaths maps each import's identifier (its alias, else the last
// path segment) to its path.
func timoniImportPaths(f *ast.File) map[string]string {
	m := map[string]string{}
	for imp := range f.ImportSpecs() {
		if imp.Path == nil || imp.Path.Value == "" {
			continue
		}
		p := imp.Path.Value
		if t := strings.TrimLeft(p, "#"); len(t) > 0 && (t[0] == '"' || t[0] == '\'') {
			if u, err := literal.Unquote(p); err == nil {
				p = u
			}
		}
		if p == "" {
			continue
		}
		ident := path.Base(p)
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" {
			ident = imp.Name.Name
		}
		m[ident] = p
	}
	return m
}

func (w *timoniWalk) add(name string, kind core.SourceKind, fd ast.Node, sig string, calls []string) bool {
	if w.taken[name] {
		return false
	}
	w.taken[name] = true
	w.nodes = append(w.nodes, core.SymbolNode{
		Service: w.service, Package: w.pkg, Name: name, Kind: kind,
		FilePath: w.relPath, Line: fd.Pos().Line(), EndLine: fd.End().Line(),
		Signature: sig, Tier: core.TierVerified, Calls: calls,
	})
	return true
}

func (w *timoniWalk) module(fd *ast.Field) {
	if w.add("timoni/module", kindTimoniModule, fd, "", nil) && w.root0 == "" {
		w.root0 = "timoni/module"
	}
}

func (w *timoniWalk) values(fd *ast.Field) {
	var calls []string
	if lit, ok := fd.Value.(*ast.StructLit); ok {
		seen := map[string]bool{}
		for _, e := range lit.Elts {
			kf, ok := e.(*ast.Field)
			if !ok {
				continue
			}
			k, _, err := ast.LabelName(kf.Label)
			if err != nil || k == "" || strings.HasPrefix(k, "#") || strings.HasPrefix(k, "_") || seen[k] {
				continue
			}
			seen[k] = true
			calls = append(calls, "#Config."+k)
		}
		sort.Strings(calls)
	}
	w.add("timoni/values", kindTimoniValues, fd, "", calls)
}

// definition reads a top-level #Def: an object template when its body names a
// literal Kubernetes kind, and a config consumer when it has a #config field.
func (w *timoniWalk) definition(name string, fd *ast.Field) {
	kind, apiVersion := timoniKindLiteral(fd.Value)
	if kind != "" {
		sig := kind
		if apiVersion != "" {
			sig = apiVersion + " " + kind
		}
		if w.add("timoni/object/"+name, kindTimoniObject, fd, sig, nil) {
			w.labels = append(w.labels, DocLabelSet{
				Entity: "timoni/object/" + name,
				Scope:  timoniScopePrefix + w.root,
				Labels: map[string]string{"pkg": w.pkg, "def": name},
				Line:   fd.Pos().Line(),
			})
		}
	}
	if name == "#Instance" {
		w.instanceRefs(fd)
	}
	if timoniTakesConfig(fd.Value) {
		if w.configDefs == nil {
			w.configDefs = map[string]bool{}
		}
		w.configDefs[name] = true
	}
}

// timoniOperands are the struct literals of a conjunction (a & b & {..}).
func timoniOperands(e ast.Expr, out *[]*ast.StructLit) {
	switch v := e.(type) {
	case *ast.StructLit:
		*out = append(*out, v)
	case *ast.BinaryExpr:
		if v.Op == token.AND {
			timoniOperands(v.X, out)
			timoniOperands(v.Y, out)
		}
	case *ast.ParenExpr:
		timoniOperands(v.X, out)
	}
}

func timoniString(e ast.Expr) (string, bool) {
	b, ok := e.(*ast.BasicLit)
	if !ok || b.Kind != token.STRING {
		return "", false
	}
	if t := strings.TrimLeft(b.Value, "#"); len(t) == 0 || (t[0] != '"' && t[0] != '\'') {
		return "", false
	}
	s, err := literal.Unquote(b.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// timoniKindLiteral is the literal `kind` (and `apiVersion`) a definition's
// struct operands declare at their top level.
func timoniKindLiteral(e ast.Expr) (kind, apiVersion string) {
	var ops []*ast.StructLit
	timoniOperands(e, &ops)
	for _, s := range ops {
		for _, el := range s.Elts {
			fd, ok := el.(*ast.Field)
			if !ok {
				continue
			}
			n, _, err := ast.LabelName(fd.Label)
			if err != nil {
				continue
			}
			switch n {
			case "kind":
				if v, ok := timoniString(fd.Value); ok && kind == "" {
					kind = v
				}
			case "apiVersion":
				if v, ok := timoniString(fd.Value); ok && apiVersion == "" {
					apiVersion = v
				}
			}
		}
	}
	return kind, apiVersion
}

// timoniTakesConfig reports whether a definition's struct operands declare
// `#config: #Config`.
func timoniTakesConfig(e ast.Expr) bool {
	var ops []*ast.StructLit
	timoniOperands(e, &ops)
	for _, s := range ops {
		for _, el := range s.Elts {
			fd, ok := el.(*ast.Field)
			if !ok {
				continue
			}
			n, _, err := ast.LabelName(fd.Label)
			if err != nil || n != "#config" {
				continue
			}
			if id, ok := fd.Value.(*ast.Ident); ok && id.Name == "#Config" {
				return true
			}
		}
	}
	return false
}

// attachConfigCalls adds the call to #Config the config consumers carry.
func (w *timoniWalk) attachConfigCalls(nodes []core.SymbolNode) {
	if len(w.configDefs) == 0 {
		return
	}
	for i := range nodes {
		n := &nodes[i]
		if !w.configDefs[n.Name] {
			continue
		}
		key := core.SymbolKey(w.service, w.pkg, "#Config")
		for _, c := range n.Calls {
			if c == key || c == "#Config" {
				key = ""
			}
		}
		if key == "" {
			continue
		}
		if w.taken["#Config"] {
			n.Calls = append(n.Calls, key)
		} else {
			n.Calls = append(n.Calls, "#Config")
		}
		sort.Strings(n.Calls)
	}
}

// instanceRefs: the definitions #Instance names anywhere in its body, each a
// label-selector reference to the object template of that package and name.
func (w *timoniWalk) instanceRefs(fd *ast.Field) {
	type ref struct{ pkg, def string }
	seen := map[ref]bool{}
	var order []ref
	line := map[ref]int{}
	var walk func(n ast.Node) bool
	walk = func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Field:
			ast.Walk(v.Value, walk, nil)
			return false
		case *ast.SelectorExpr:
			if id, ok := v.X.(*ast.Ident); ok {
				if p, ok := w.imports[id.Name]; ok {
					if def, _, err := ast.LabelName(v.Sel); err == nil && strings.HasPrefix(def, "#") {
						r := ref{path.Base(p), def}
						if !seen[r] {
							seen[r] = true
							order = append(order, r)
							line[r] = v.Pos().Line()
						}
					}
				}
			}
			return false
		case *ast.Ident:
			if strings.HasPrefix(v.Name, "#") {
				r := ref{w.pkg, v.Name}
				if !seen[r] {
					seen[r] = true
					order = append(order, r)
					line[r] = v.Pos().Line()
				}
			}
		}
		return true
	}
	ast.Walk(fd.Value, walk, nil)
	for _, r := range order {
		if r.def == "#Instance" || r.def == "#Config" {
			continue
		}
		raw := r.def
		if r.pkg != w.pkg {
			raw = r.pkg + "." + r.def
		}
		w.cross = append(w.cross, DocCrossFileRef{
			From: "#Instance", Edge: timoniEdgeRenders, Raw: raw, Strategy: "label-selector",
			Line: line[r], Scope: timoniScopePrefix + w.root,
			Labels: map[string]string{"pkg": r.pkg, "def": r.def},
		})
	}
}

// timoniChild is a member of a struct reached through a chain of single-field
// structs (`a: b: c: {..}` parses as nested structs).
func timoniMember(fd *ast.Field, name string) (*ast.Field, bool) {
	lit, ok := fd.Value.(*ast.StructLit)
	if !ok {
		return nil, false
	}
	for _, e := range lit.Elts {
		cf, ok := e.(*ast.Field)
		if !ok {
			continue
		}
		if n, _, err := ast.LabelName(cf.Label); err == nil && n == name {
			return cf, true
		}
	}
	return nil, false
}

func timoniMembers(fd *ast.Field) []*ast.Field {
	lit, ok := fd.Value.(*ast.StructLit)
	if !ok {
		return nil
	}
	var out []*ast.Field
	for _, e := range lit.Elts {
		if cf, ok := e.(*ast.Field); ok {
			out = append(out, cf)
		}
	}
	return out
}

func (w *timoniWalk) bundle(fd *ast.Field) {
	if w.add("timoni/bundle", kindTimoniBundle, fd, "", nil) && w.root0 == "" {
		w.root0 = "timoni/bundle"
	}
	insts, ok := timoniMember(fd, "instances")
	if !ok {
		return
	}
	for _, in := range timoniMembers(insts) {
		name, _, err := ast.LabelName(in.Label)
		if err != nil || name == "" {
			continue
		}
		ent := "timoni/bundle/instance/" + name
		if !w.add(ent, kindTimoniBundleInstance, in, "", nil) {
			continue
		}
		mod, ok := timoniMember(in, "module")
		if !ok {
			continue
		}
		urlf, ok := timoniMember(mod, "url")
		if !ok {
			continue
		}
		w.moduleRef(ent, urlf)
	}
}

// moduleRef records one instance's module url as a cross-file reference or as
// an unresolved one.
func (w *timoniWalk) moduleRef(from string, urlf *ast.Field) {
	line := urlf.Pos().Line()
	raw, ok := timoniString(urlf.Value)
	if !ok {
		w.unresolved = append(w.unresolved, TimoniUnresolved{Line: line, Edge: timoniEdgeModule, Reason: "expression"})
		return
	}
	switch {
	case strings.HasPrefix(raw, "oci://"):
		w.unresolved = append(w.unresolved, TimoniUnresolved{Line: line, Edge: timoniEdgeModule, Raw: raw, Reason: "external:oci"})
		return
	case !strings.HasPrefix(raw, "file://"):
		w.unresolved = append(w.unresolved, TimoniUnresolved{Line: line, Edge: timoniEdgeModule, Raw: raw, Reason: "external:other"})
		return
	}
	p := strings.TrimPrefix(raw, "file://")
	if strings.HasPrefix(p, "/") {
		w.unresolved = append(w.unresolved, TimoniUnresolved{Line: line, Edge: timoniEdgeModule, Raw: raw, Reason: "absolute-path"})
		return
	}
	target := path.Join(path.Dir(strings.ReplaceAll(w.relPath, "\\", "/")), p)
	if target == ".." || strings.HasPrefix(target, "../") {
		w.unresolved = append(w.unresolved, TimoniUnresolved{Line: line, Edge: timoniEdgeModule, Raw: raw, Reason: "outside-repo"})
		return
	}
	w.cross = append(w.cross, DocCrossFileRef{
		From: from, Edge: timoniEdgeModule, Raw: raw, Target: path.Join(target, "timoni.cue"),
		Strategy: "repo-path", Line: line,
	})
}

func (w *timoniWalk) runtime(fd *ast.Field) {
	w.add("timoni/runtime", kindTimoniRuntime, fd, "", nil)
	cl, ok := timoniMember(fd, "clusters")
	if !ok {
		return
	}
	for _, c := range timoniMembers(cl) {
		name, _, err := ast.LabelName(c.Label)
		if err != nil || name == "" {
			continue
		}
		w.add("timoni/runtime/cluster/"+name, kindTimoniCluster, c, "", nil)
	}
}
