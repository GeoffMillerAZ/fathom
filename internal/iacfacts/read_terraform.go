package iacfacts

import (
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// ReadTerraformFile reads the resources, data sources and module calls of one Terraform
// or OpenTofu file (.tf). Each attribute gets a path, a class and a span. Variables and
// locals are not resolved: var.x reads as a reference. ReadTerraformModule resolves them
// across a directory. A module call is a resource-like relation and is never expanded.
//
// The read is bounded and deterministic. A file past the product's limits is refused
// whole with ErrLimit. A syntax error is counted in ResourceSet.SyntaxErrors and the
// rest of the file is still read.
func ReadTerraformFile(filePath string, src []byte, opts ...Option) (*ResourceSet, error) {
	body, nerr, err := parseTerraform(filePath, src)
	if err != nil {
		return nil, err
	}
	r := newTFReader(newConfig(opts), nil)
	set := &ResourceSet{Tool: ToolTerraform, SyntaxErrors: nerr}
	if err := r.readBody(set, body); err != nil {
		return nil, err
	}
	sortSet(set)
	return set, nil
}

// ReadTerraformModule reads every .tf and .tofu file directly inside dir of fsys, and
// resolves variable defaults and locals across them: an attribute that reads var.x, or a
// local chain ending in a variable default, is defaulted. A module call is read as a
// relation and is never expanded, and nothing outside dir is read. Past the product's
// limits the whole directory is refused with ErrLimit.
func ReadTerraformModule(fsys fs.FS, dir string, opts ...Option) (*ResourceSet, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("iacfacts: read module directory: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if ext := path.Ext(e.Name()); ext == ".tf" || ext == ".tofu" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) > MaxModuleFiles {
		return nil, limitErr(dir, fmt.Sprintf("%d files, over %d", len(names), MaxModuleFiles))
	}
	type parsed struct {
		body *hclsyntax.Body
	}
	var files []parsed
	set := &ResourceSet{Tool: ToolTerraform}
	total := 0
	for _, n := range names {
		p := path.Join(dir, n)
		f, err := fsys.Open(p)
		if err != nil {
			return nil, fmt.Errorf("iacfacts: open module file: %w", err)
		}
		src, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("iacfacts: read module file: %w", err)
		}
		if total += len(src); total > MaxModuleBytes {
			return nil, limitErr(dir, fmt.Sprintf("module is over %d bytes", MaxModuleBytes))
		}
		body, nerr, err := parseTerraform(p, src)
		if err != nil {
			return nil, err
		}
		set.SyntaxErrors += nerr
		files = append(files, parsed{body})
	}
	env := &tfEnv{vars: map[string]*tfVar{}, locals: map[string]hclsyntax.Expression{}}
	for _, f := range files {
		env.collect(f.body)
	}
	r := newTFReader(newConfig(opts), env)
	for _, f := range files {
		if err := r.readBody(set, f.body); err != nil {
			return nil, err
		}
	}
	sortSet(set)
	return set, nil
}

// parseTerraform lexes, bounds and parses one file. It returns the body, the count of
// syntax errors and ErrLimit past a limit.
func parseTerraform(file string, src []byte) (*hclsyntax.Body, int, error) {
	if err := checkSize(file, len(src)); err != nil {
		return nil, 0, err
	}
	f, diags, err := parseHCLBounded(file, src)
	if err != nil {
		return nil, 0, err
	}
	nerr := 0
	for _, d := range diags {
		if d.Severity == hcl.DiagError {
			nerr++
		}
	}
	if f == nil {
		return &hclsyntax.Body{}, nerr, nil
	}
	body, _ := f.Body.(*hclsyntax.Body)
	if body == nil {
		body = &hclsyntax.Body{}
	}
	return body, nerr, nil
}

// tfVar is a declared variable and its default, when it has one.
type tfVar struct {
	def hclsyntax.Expression
}

// tfEnv is what ReadTerraformModule resolves against: variable defaults and locals.
type tfEnv struct {
	vars   map[string]*tfVar
	locals map[string]hclsyntax.Expression
}

func (e *tfEnv) collect(body *hclsyntax.Body) {
	for _, b := range body.Blocks {
		switch {
		case b.Type == "variable" && len(b.Labels) == 1:
			if _, dup := e.vars[b.Labels[0]]; dup {
				continue
			}
			v := &tfVar{}
			if a, ok := b.Body.Attributes["default"]; ok {
				v.def = a.Expr
			}
			e.vars[b.Labels[0]] = v
		case b.Type == "locals":
			for _, a := range sortedAttrs(b.Body) {
				if _, dup := e.locals[a.Name]; !dup {
					e.locals[a.Name] = a.Expr
				}
			}
		}
	}
}

type tfReader struct {
	emitter
	env       *tfEnv
	resolving map[string]bool
	nodes     int
}

func newTFReader(cfg config, env *tfEnv) *tfReader {
	return &tfReader{emitter: emitter{cfg: cfg}, env: env, resolving: map[string]bool{}}
}

func sortedAttrs(b *hclsyntax.Body) []*hclsyntax.Attribute {
	out := make([]*hclsyntax.Attribute, 0, len(b.Attributes))
	for _, a := range b.Attributes {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SrcRange.Start.Byte < out[j].SrcRange.Start.Byte })
	return out
}

func spanOf(rng hcl.Range) Span {
	return Span{File: rng.Filename, StartLine: rng.Start.Line, StartCol: rng.Start.Column,
		EndLine: rng.End.Line, EndCol: rng.End.Column}
}

// readBody reads the top-level resource, data and module blocks of one file.
func (r *tfReader) readBody(set *ResourceSet, body *hclsyntax.Body) error {
	for _, b := range body.Blocks {
		var res Resource
		switch {
		case b.Type == "resource" && len(b.Labels) == 2:
			res = Resource{Tool: ToolTerraform, Mode: ModeManaged, Type: b.Labels[0], Name: b.Labels[1]}
		case b.Type == "data" && len(b.Labels) == 2:
			res = Resource{Tool: ToolTerraform, Mode: ModeData, Type: b.Labels[0], Name: b.Labels[1]}
		case b.Type == "module" && len(b.Labels) == 1:
			res = Resource{Tool: ToolTerraform, Mode: ModeModule, Type: "module", Name: b.Labels[0]}
		default:
			continue
		}
		res.Span = spanOf(b.Range())
		res.Multiplicity = Multiplicity{Kind: MultiSingle}
		// Meta-arguments that are not attributes of the resource.
		for _, a := range sortedAttrs(b.Body) {
			switch a.Name {
			case "provider":
				if t, ok := a.Expr.(*hclsyntax.ScopeTraversalExpr); ok {
					res.Provider = renderTraversal(t.Traversal)
				}
			case "count", "for_each":
				kind := MultiCount
				if a.Name == "for_each" {
					kind = MultiForEach
				}
				c := r.classify(a.Expr, nil, 0)
				res.Multiplicity = Multiplicity{Kind: kind, Class: c.class, Refs: c.refs, Span: spanOf(a.SrcRange)}
			}
		}
		walk := &tfWalk{file: b.Range().Filename, top: true}
		r.walkBody(b.Body, "", walk, nil, 0, &res.Attrs)
		if r.err != nil {
			return r.err
		}
		set.Resources = append(set.Resources, res)
	}
	return r.err
}

// tfWalk is the state of one resource walk.
type tfWalk struct {
	file string
	top  bool // the walk is at the resource's own body
}

func (r *tfReader) walkBody(body *hclsyntax.Body, prefix string, w *tfWalk, iters map[string]bool, depth int, out *[]Attr) {
	if r.err != nil {
		return
	}
	if depth > MaxDepth {
		r.fail(w.file, fmt.Sprintf("blocks nest past %d deep", MaxDepth))
		return
	}
	top := w.top
	for _, a := range sortedAttrs(body) {
		if top && (a.Name == "provider" || a.Name == "count" || a.Name == "for_each") {
			continue
		}
		p := joinPath(prefix, a.Name)
		var c readRes
		if prefix == "lifecycle" && a.Name == "ignore_changes" {
			c = ignoreChanges(a.Expr, r.cfg.values)
		}
		if c.class == "" {
			c = r.classify(a.Expr, iters, 0)
		}
		r.emit(w.file, out, p, c, spanOf(a.SrcRange))
	}
	counts := map[string]int{}
	for _, b := range body.Blocks {
		if b.Type == "dynamic" && len(b.Labels) == 1 {
			counts[b.Labels[0]]++
		} else {
			counts[b.Type]++
		}
	}
	for _, b := range body.Blocks {
		w.top = false
		if top && (b.Type == "provisioner" || b.Type == "connection") {
			continue
		}
		if b.Type == "dynamic" && len(b.Labels) == 1 {
			name := b.Labels[0]
			p := joinPath(prefix, name) + "[*]"
			iterName := name
			if ia, ok := b.Body.Attributes["iterator"]; ok {
				if t, ok := ia.Expr.(*hclsyntax.ScopeTraversalExpr); ok && len(t.Traversal) == 1 {
					iterName = t.Traversal.RootName()
				}
			}
			inner := with(iters, iterName)
			if fe, ok := b.Body.Attributes["for_each"]; ok {
				r.emit(w.file, out, p, r.classify(fe.Expr, iters, 0), spanOf(fe.SrcRange))
			}
			for _, cb := range b.Body.Blocks {
				if cb.Type == "content" {
					r.walkBody(cb.Body, p, w, inner, depth+1, out)
				}
			}
			continue
		}
		p := joinPath(prefix, b.Type)
		if counts[b.Type] > 1 {
			p += "[*]"
		}
		r.walkBody(b.Body, p, w, iters, depth+1, out)
	}
	w.top = top
}

func with(m map[string]bool, k string) map[string]bool {
	out := make(map[string]bool, len(m)+1)
	for a := range m {
		out[a] = true
	}
	out[k] = true
	return out
}

// ignoreChanges reads lifecycle.ignore_changes, whose bare names are keywords for
// attribute paths, not references: [tags, ami] and all.
func ignoreChanges(e hclsyntax.Expression, values bool) readRes {
	var names []any
	switch x := e.(type) {
	case *hclsyntax.ScopeTraversalExpr:
		if len(x.Traversal) == 1 && x.Traversal.RootName() == "all" {
			return readRes{class: ClassLiteral, val: "all", hasV: values}
		}
		return readRes{}
	case *hclsyntax.TupleConsExpr:
		for _, el := range x.Exprs {
			t, ok := el.(*hclsyntax.ScopeTraversalExpr)
			if !ok {
				return readRes{}
			}
			names = append(names, renderTraversal(t.Traversal))
		}
		if names == nil {
			names = []any{}
		}
		return readRes{class: ClassLiteral, val: names, hasV: values}
	}
	return readRes{}
}

// classify classifies one expression. iters are the dynamic-block iterator names in scope.
func (r *tfReader) classify(e hclsyntax.Expression, iters map[string]bool, depth int) readRes {
	if r.err != nil {
		return readRes{class: ClassUnknown}
	}
	if r.nodes++; r.nodes > MaxNodes {
		r.fail("", fmt.Sprintf("more than %d expression nodes", MaxNodes))
		return readRes{class: ClassUnknown}
	}
	if depth > MaxDepth {
		r.fail("", fmt.Sprintf("expressions nest past %d deep", MaxDepth))
		return readRes{class: ClassUnknown}
	}
	switch x := e.(type) {
	case *hclsyntax.LiteralValueExpr:
		return readRes{class: ClassLiteral, val: ctyToGo(x.Val), hasV: r.cfg.values}
	case *hclsyntax.TemplateExpr:
		return r.foldTemplate(x.Parts, iters, depth)
	case *hclsyntax.TemplateWrapExpr:
		return r.classify(x.Wrapped, iters, depth+1)
	case *hclsyntax.ParenthesesExpr:
		return r.classify(x.Expression, iters, depth+1)
	case *hclsyntax.TupleConsExpr:
		parts := make([]readRes, len(x.Exprs))
		for i, el := range x.Exprs {
			parts[i] = r.classify(el, iters, depth+1)
		}
		out := foldRes(parts)
		if out.hasV {
			list := make([]any, len(parts))
			for i, p := range parts {
				list[i] = p.val
			}
			out.val = list
		}
		return out
	case *hclsyntax.ObjectConsExpr:
		parts := make([]readRes, 0, len(x.Items))
		keys := make([]readRes, len(x.Items))
		vals := make([]readRes, len(x.Items))
		for i, it := range x.Items {
			keys[i] = r.classifyKey(it.KeyExpr, iters, depth+1)
			vals[i] = r.classify(it.ValueExpr, iters, depth+1)
			// A literal key is structure, not content: only a computed key
			// changes what the object's class is.
			if keys[i].class != ClassLiteral {
				parts = append(parts, keys[i])
			}
			parts = append(parts, vals[i])
		}
		out := foldRes(parts)
		if out.hasV {
			m := make(map[string]any, len(x.Items))
			for i := range keys {
				ks, ok := keys[i].val.(string)
				if !ok {
					out.hasV = false
					out.val = nil
					return out
				}
				m[ks] = vals[i].val
			}
			out.val = m
		}
		return out
	case *hclsyntax.ObjectConsKeyExpr:
		return r.classifyKey(x, iters, depth+1)
	case *hclsyntax.ScopeTraversalExpr:
		return r.classifyTraversal(x, iters, depth)
	case *hclsyntax.SplatExpr:
		src := r.classify(x.Source, iters, depth+1)
		if rel, ok := x.Each.(*hclsyntax.RelativeTraversalExpr); ok && src.class == ClassReference {
			if _, anon := rel.Source.(*hclsyntax.AnonSymbolExpr); anon {
				return readRes{class: ClassReference, refs: src.refs}
			}
		}
		return readRes{class: ClassExpression, refs: r.variableRefs(e, iters)}
	case *hclsyntax.RelativeTraversalExpr, *hclsyntax.IndexExpr, *hclsyntax.FunctionCallExpr,
		*hclsyntax.ConditionalExpr, *hclsyntax.BinaryOpExpr, *hclsyntax.UnaryOpExpr,
		*hclsyntax.ForExpr, *hclsyntax.TemplateJoinExpr, *hclsyntax.AnonSymbolExpr:
		return readRes{class: ClassExpression, refs: r.variableRefs(e, iters)}
	}
	return readRes{class: ClassUnknown}
}

// classifyKey classifies an object key: a bare name is the string it spells.
func (r *tfReader) classifyKey(e hclsyntax.Expression, iters map[string]bool, depth int) readRes {
	if k, ok := e.(*hclsyntax.ObjectConsKeyExpr); ok {
		if t, ok := k.Wrapped.(*hclsyntax.ScopeTraversalExpr); ok && len(t.Traversal) == 1 && !k.ForceNonLiteral {
			return readRes{class: ClassLiteral, val: t.Traversal.RootName(), hasV: r.cfg.values}
		}
		return r.classify(k.Wrapped, iters, depth)
	}
	return r.classify(e, iters, depth)
}

func (r *tfReader) foldTemplate(parts []hclsyntax.Expression, iters map[string]bool, depth int) readRes {
	rs := make([]readRes, len(parts))
	for i, p := range parts {
		rs[i] = r.classify(p, iters, depth+1)
	}
	out := foldRes(rs)
	if out.hasV {
		out.val, out.hasV = concatVals(rs)
	}
	return out
}

// concatVals joins the string values of a template's parts.
func concatVals(parts []readRes) (string, bool) {
	var sb strings.Builder
	for _, p := range parts {
		s, ok := p.val.(string)
		if !ok {
			if s, ok = scalarString(p.val); !ok {
				return "", false
			}
		}
		sb.WriteString(s)
	}
	return sb.String(), true
}

func scalarString(v any) (string, bool) {
	switch x := v.(type) {
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	case int64:
		return big.NewInt(x).String(), true
	case float64:
		return new(big.Float).SetFloat64(x).Text('g', -1), true
	}
	return "", false
}

// foldRes folds the classes of a structure's parts: a structure whose every part is a
// constant is a constant, one whose every part is a constant or a default has a static
// answer through its defaults, one made only of references is a relation, and anything
// else is computed. One unclassifiable part makes the structure unknown.
func foldRes(parts []readRes) readRes {
	out := readRes{class: ClassLiteral, hasV: true}
	anyDef, anyUnknown, allRef := false, false, len(parts) > 0
	allLitDef := true
	for _, p := range parts {
		out.refs = append(out.refs, p.refs...)
		out.vsize += partSize(p)
		out.big = out.big || p.big
		if !p.hasV {
			out.hasV = false
		}
		switch p.class {
		case ClassLiteral:
			allRef = false
		case ClassDefaulted:
			anyDef = true
			allRef = false
		case ClassReference:
			allLitDef = false
		case ClassUnknown:
			anyUnknown = true
			allLitDef = false
			allRef = false
		default:
			allLitDef = false
			allRef = false
		}
	}
	switch {
	case anyUnknown:
		out.class = ClassUnknown
	case allLitDef && anyDef:
		out.class = ClassDefaulted
	case allLitDef:
		out.class = ClassLiteral
	case allRef:
		out.class = ClassReference
	default:
		out.class = ClassExpression
	}
	if out.class != ClassLiteral && out.class != ClassDefaulted {
		out.hasV = false
	}
	if out.hasV && (out.big || out.vsize > MaxValueBytes) {
		out.hasV, out.big = false, true
	}
	return out
}

// classifyTraversal classifies a reference by what its root names.
func (r *tfReader) classifyTraversal(t *hclsyntax.ScopeTraversalExpr, iters map[string]bool, depth int) readRes {
	root := t.Traversal.RootName()
	ref := renderTraversal(t.Traversal)
	switch root {
	case "each", "count", "self", "path", "terraform":
		return readRes{class: ClassExpression}
	case "var", "local":
		if len(t.Traversal) < 2 {
			return readRes{class: ClassReference, refs: []string{ref}}
		}
		attr, ok := t.Traversal[1].(hcl.TraverseAttr)
		if !ok || r.env == nil {
			return readRes{class: ClassReference, refs: []string{ref}}
		}
		key := root + "." + attr.Name
		var expr hclsyntax.Expression
		if root == "var" {
			if v := r.env.vars[attr.Name]; v != nil {
				expr = v.def
			}
		} else {
			expr = r.env.locals[attr.Name]
		}
		if expr == nil {
			return readRes{class: ClassReference, refs: []string{ref}}
		}
		if r.resolving[key] {
			return readRes{class: ClassUnknown, refs: []string{ref}}
		}
		r.resolving[key] = true
		inner := r.classify(expr, nil, depth+1)
		delete(r.resolving, key)
		out := readRes{class: inner.class, refs: []string{ref}}
		if root == "var" && inner.class == ClassLiteral {
			out.class = ClassDefaulted
		}
		if inner.class == ClassReference {
			// A variable default or a local that is itself a reference is still one.
			out.class = ClassReference
		}
		if len(t.Traversal) == 2 && inner.hasV && (out.class == ClassLiteral || out.class == ClassDefaulted) {
			out.val, out.hasV, out.vsize = inner.val, true, inner.vsize
		}
		out.big = inner.big
		return out
	}
	if iters[root] {
		return readRes{class: ClassExpression}
	}
	return readRes{class: ClassReference, refs: []string{ref}}
}

// variableRefs lists the static references an expression reads, minus the names that
// are scopes and not things.
func (r *tfReader) variableRefs(e hclsyntax.Expression, iters map[string]bool) []string {
	var out []string
	for _, t := range e.Variables() {
		switch t.RootName() {
		case "each", "count", "self", "path", "terraform":
			continue
		}
		if iters[t.RootName()] {
			continue
		}
		out = append(out, renderTraversal(t))
	}
	return sortedRefs(out)
}

// renderTraversal spells a traversal up to its first index or splat step. It names a
// thing and carries no value.
func renderTraversal(t hcl.Traversal) string {
	var sb strings.Builder
	for _, s := range t {
		switch x := s.(type) {
		case hcl.TraverseRoot:
			sb.WriteString(x.Name)
		case hcl.TraverseAttr:
			sb.WriteByte('.')
			sb.WriteString(x.Name)
		default:
			return sb.String()
		}
	}
	return sb.String()
}

// ctyToGo converts a constant to a plain Go value.
func ctyToGo(v cty.Value) any {
	if v.IsNull() || !v.IsKnown() {
		return nil
	}
	switch v.Type() {
	case cty.String:
		return v.AsString()
	case cty.Bool:
		return v.True()
	case cty.Number:
		bf := v.AsBigFloat()
		if bf.IsInt() {
			if i, acc := bf.Int64(); acc == big.Exact {
				return i
			}
		}
		f, _ := bf.Float64()
		return f
	}
	return nil
}
