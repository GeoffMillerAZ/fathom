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
	pf, err := parseTerraform(filePath, src)
	if err != nil {
		return nil, err
	}
	r := newTFReader(newConfig(opts), nil)
	set := &ResourceSet{Tool: ToolTerraform, SyntaxErrors: pf.nerr}
	r.pf, r.file = pf, filePath
	if err := r.readBody(set, pf.body); err != nil {
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
//
// It follows OpenTofu's file rules. A .tofu file shadows the .tf file of the same name, which
// is not read at all. An override file (override.tf, *_override.tf, and the .tofu spellings)
// adds no resource: every attribute or nested block it names reads unknown in the resource it
// overrides, a variable default or local it overrides reads unknown, and the resource is read
// once. Attributes are not merged, since the value that wins is not worth the guess.
//
// It skips what OpenTofu skips (dot files, editor backups) and follows a symbolic link to a file as OpenTofu does;
// a link that points nowhere, or at a directory, counts as a syntax error. The JSON syntax (.tf.json and
// .tofu.json) is not read: a JSON file counts one syntax error, since it declares blocks this read does not see,
// except that a JSON override file marks the attributes, variable defaults and locals it names as unknown.
func ReadTerraformModule(fsys fs.FS, dir string, opts ...Option) (*ResourceSet, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("iacfacts: read module directory: %w", err)
	}
	names := tfModuleFileNames(entries)
	if len(names) > MaxModuleFiles {
		return nil, limitErr(dir, fmt.Sprintf("%d files, over %d", len(names), MaxModuleFiles))
	}
	var files []*tfParsed
	set := &ResourceSet{Tool: ToolTerraform}
	total := 0
	link := map[string]bool{}
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			link[e.Name()] = true
		}
	}
	for _, n := range names {
		p := path.Join(dir, n)
		f, err := fsys.Open(p)
		if err != nil {
			if link[n] {
				set.SyntaxErrors++ // a link that points nowhere: OpenTofu refuses the module, the file is not read
				continue
			}
			return nil, fmt.Errorf("iacfacts: open module file: %w", err)
		}
		if link[n] {
			if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
				_ = f.Close()
				set.SyntaxErrors++
				continue
			}
		}
		src, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("iacfacts: read module file: %w", err)
		}
		if total += len(src); total > MaxModuleBytes {
			return nil, limitErr(dir, fmt.Sprintf("module is over %d bytes", MaxModuleBytes))
		}
		if isTFJSON(n) {
			if err := checkSize(p, len(src)); err != nil {
				return nil, err
			}
			pf := &tfParsed{file: p, body: &hclsyntax.Body{}, override: isOverrideFile(n)}
			if pf.override {
				if pf.jsonOver = readJSONOverride(p, src); pf.jsonOver == nil {
					set.SyntaxErrors++
				}
			} else {
				set.SyntaxErrors++
			}
			files = append(files, pf)
			continue
		}
		pf, err := parseTerraform(p, src)
		if err != nil {
			return nil, err
		}
		set.SyntaxErrors += pf.nerr
		pf.override = isOverrideFile(n)
		files = append(files, pf)
	}
	env := &tfEnv{vars: map[string]*tfVar{}, locals: map[string]hclsyntax.Expression{},
		overVars: map[string]bool{}, overLocals: map[string]bool{}}
	// Overrides are collected after every base file, whatever their names sort to, since a sensitive-only override
	// changes the variable the base declares.
	for _, f := range files {
		if !f.override {
			env.collect(f)
		}
	}
	for _, f := range files {
		if f.override {
			env.collectOverrides(f)
		}
	}
	r := newTFReader(newConfig(opts), env)
	for _, f := range files {
		if f.override {
			continue
		}
		r.pf, r.file = f, f.file
		if err := r.readBody(set, f.body); err != nil {
			return nil, err
		}
	}
	ix := newOverrideIndex(set)
	for _, f := range files {
		if !f.override {
			continue
		}
		r.pf, r.file = f, f.file
		if f.jsonOver != nil {
			for _, o := range f.jsonOver.res {
				ix.apply(set, o)
			}
			continue
		}
		if err := r.applyOverrides(set, ix, f.body); err != nil {
			return nil, err
		}
	}
	sortSet(set)
	return set, nil
}

// tfModuleFileNames lists the files OpenTofu reads in one module directory, sorted: every regular .tf and .tofu
// file, where a .tofu file shadows the .tf file of the same name. Binding and the reader share it, so the two never
// disagree about which files make a module.
func tfModuleFileNames(entries []fs.DirEntry) []string {
	have := map[string]bool{}
	for _, e := range entries {
		// A symbolic link is a file, as it is to OpenTofu; the reader checks that it points at one.
		if t := e.Type(); (t.IsRegular() || t&fs.ModeSymlink != 0) && !tfIgnoredFile(e.Name()) {
			have[e.Name()] = true
		}
	}
	var names []string
	for n := range have {
		switch {
		case strings.HasSuffix(n, ".tf"):
			if have[strings.TrimSuffix(n, ".tf")+".tofu"] {
				continue // shadowed, as OpenTofu does
			}
			names = append(names, n)
		case strings.HasSuffix(n, ".tf.json"):
			if have[strings.TrimSuffix(n, ".tf.json")+".tofu.json"] {
				continue
			}
			names = append(names, n)
		case strings.HasSuffix(n, ".tofu"), strings.HasSuffix(n, ".tofu.json"):
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// tfIgnoredFile reports a name OpenTofu skips: a dot file, an editor backup ending in ~, or an editor lock
// file written between #.
func tfIgnoredFile(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") || (strings.HasPrefix(name, "#") && strings.HasSuffix(name, "#"))
}

// tfParsed is one parsed file: its body, the count of syntax errors and the source ranges the errors point at.
type tfParsed struct {
	file string
	body *hclsyntax.Body
	nerr int
	bad  []hcl.Range
	// override marks an override file, which only changes what the other files declare.
	override bool
	// jsonOver is what a .tf.json override file names; its body is empty.
	jsonOver *tfJSONOver
	src      []byte
	lineAt   []int // byte offset of each line start, built on first use
}

// suspect reports whether what the parser returned for rng cannot be trusted. The parser reports the first syntax
// error and then recovers silently, so a later broken attribute reads as a clean literal with the error count left
// at one. An attribute a reported error touches is suspect. So is any attribute at or after the first error, unless
// its own lines parse clean on their own, which is the one proof the recovered parse cannot give.
func (pf *tfParsed) suspect(rng hcl.Range) bool {
	first := 0
	for _, b := range pf.bad {
		last := b.End.Line
		if b.End.Column == 1 && last > b.Start.Line {
			last-- // a newline token ends at the start of the next line, which it does not occupy
		}
		if b.Filename == rng.Filename && b.Start.Line <= rng.End.Line && rng.Start.Line <= last {
			return true
		}
		if first == 0 || b.Start.Line < first {
			first = b.Start.Line
		}
	}
	if pf.nerr > 0 && len(pf.bad) == 0 {
		return true // errors with no position: nothing is known to be clean
	}
	if first == 0 || rng.End.Line < first {
		return false
	}
	return !pf.linesParseClean(rng.Start.Line, rng.End.Line)
}

// linesParseClean parses lines from..to of the file on their own, in a fresh parser that is not in recovery.
func (pf *tfParsed) linesParseClean(from, to int) bool {
	if pf.lineAt == nil {
		pf.lineAt = []int{0}
		for i, c := range pf.src {
			if c == '\n' {
				pf.lineAt = append(pf.lineAt, i+1)
			}
		}
	}
	if from < 1 || to < from || to > len(pf.lineAt) {
		return false
	}
	end := len(pf.src)
	if to < len(pf.lineAt) {
		end = pf.lineAt[to]
	}
	_, diags, err := parseHCLBounded(pf.file, pf.src[pf.lineAt[from-1]:end])
	return err == nil && !diags.HasErrors()
}

// isOverrideFile reports whether a file name is one OpenTofu treats as an override file.
func isOverrideFile(name string) bool {
	stem := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, ".json"), ".tf"), ".tofu")
	return stem == "override" || strings.HasSuffix(stem, "_override")
}

// parseTerraform lexes, bounds and parses one file. It returns ErrLimit past a limit.
func parseTerraform(file string, src []byte) (*tfParsed, error) {
	if err := checkSize(file, len(src)); err != nil {
		return nil, err
	}
	f, diags, err := parseHCLBounded(file, src)
	if err != nil {
		return nil, err
	}
	pf := &tfParsed{file: file, body: &hclsyntax.Body{}, src: src}
	for _, d := range diags {
		if d.Severity != hcl.DiagError {
			continue
		}
		pf.nerr++
		if d.Subject != nil {
			pf.bad = append(pf.bad, *d.Subject)
		}
	}
	if f != nil {
		if body, _ := f.Body.(*hclsyntax.Body); body != nil {
			pf.body = body
		}
	}
	return pf, nil
}

// tfVar is a declared variable and its default, when it has one.
type tfVar struct {
	def hclsyntax.Expression
	// secret marks a variable declared sensitive or ephemeral, or with an expression the reader cannot tell
	// from true. Its default is never kept as a value, whatever the caller asks.
	secret bool
}

// tfEnv is what ReadTerraformModule resolves against: variable defaults and locals.
type tfEnv struct {
	vars   map[string]*tfVar
	locals map[string]hclsyntax.Expression
	// overVars and overLocals name the variable defaults and locals an override file replaces. Which value
	// wins is not read, so a reference to one reads unknown.
	overVars, overLocals map[string]bool
}

// isFalseLiteral reports whether an expression is the literal false.
func isFalseLiteral(e hclsyntax.Expression) bool {
	l, ok := e.(*hclsyntax.LiteralValueExpr)
	return ok && l.Val.IsKnown() && l.Val.Type() == cty.Bool && l.Val.False()
}

// collectOverrides records the variable defaults and locals an override file replaces.
func (e *tfEnv) collectOverrides(pf *tfParsed) {
	if pf.jsonOver != nil {
		for _, n := range pf.jsonOver.vars {
			e.overVars[n] = true
		}
		for n, secret := range pf.jsonOver.secret {
			if v, ok := e.vars[n]; ok {
				v.secret = secret
			}
		}
		for _, n := range pf.jsonOver.locals {
			e.overLocals[n] = true
		}
		return
	}
	for _, b := range pf.body.Blocks {
		switch {
		case b.Type == "variable" && len(b.Labels) == 1:
			if _, ok := b.Body.Attributes["default"]; ok {
				e.overVars[b.Labels[0]] = true
			}
			// The override's sensitive or ephemeral replaces the base's: only the literal false clears it, and an
			// expression the reader cannot tell from true counts as true, as it does in the base.
			if v, ok := e.vars[b.Labels[0]]; ok {
				for _, flag := range []string{"sensitive", "ephemeral"} {
					if a, ok := b.Body.Attributes[flag]; ok {
						v.secret = !isFalseLiteral(a.Expr) || pf.suspect(a.SrcRange)
					}
				}
			}
		case b.Type == "locals":
			for name := range b.Body.Attributes {
				e.overLocals[name] = true
			}
		}
	}
}

func (e *tfEnv) collect(pf *tfParsed) {
	for _, b := range pf.body.Blocks {
		switch {
		case b.Type == "variable" && len(b.Labels) == 1:
			if _, dup := e.vars[b.Labels[0]]; dup {
				continue
			}
			v := &tfVar{}
			if a, ok := b.Body.Attributes["default"]; ok {
				v.def = a.Expr
				if pf.suspect(a.SrcRange) {
					e.overVars[b.Labels[0]] = true // a default the parser may have guessed reads unknown
				}
			}
			for _, flag := range []string{"sensitive", "ephemeral"} {
				if a, ok := b.Body.Attributes[flag]; ok && (!isFalseLiteral(a.Expr) || pf.suspect(a.SrcRange)) {
					v.secret = true
				}
			}
			e.vars[b.Labels[0]] = v
		case b.Type == "locals":
			for _, a := range sortedAttrs(b.Body) {
				if _, dup := e.locals[a.Name]; !dup {
					e.locals[a.Name] = a.Expr
					if pf.suspect(a.SrcRange) {
						e.overLocals[a.Name] = true
					}
				}
			}
		}
	}
}

type tfReader struct {
	emitter
	// pf is the file being read: an attribute its syntax errors touch, or that follows its first one without
	// parsing clean on its own, reads unknown.
	pf        *tfParsed
	file      string // the file being read, for errors
	env       *tfEnv
	resolving map[string]bool
	nodes     int
	// inners memoizes what a variable default or a local classifies to, so a thousand references to one local read
	// its expression once. An answer computed while a cycle was being cut is not kept: it depends on where the
	// walk entered.
	inners map[string]readRes
	cuts   int
}

func newTFReader(cfg config, env *tfEnv) *tfReader {
	return &tfReader{emitter: emitter{cfg: cfg}, env: env, resolving: map[string]bool{}, inners: map[string]readRes{}}
}

func sortedAttrs(b *hclsyntax.Body) []*hclsyntax.Attribute {
	out := make([]*hclsyntax.Attribute, 0, len(b.Attributes))
	for _, a := range b.Attributes {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SrcRange.Start.Byte < out[j].SrcRange.Start.Byte })
	return out
}

// broken reports whether the parser's answer for rng is suspect (tfParsed.suspect). The parser recovers from a
// broken expression with a placeholder or a partial value, so what it returns there says nothing about what was
// written. The test is by line, so tokens left over after a complete expression ("x = a b") mark the attribute too.
func (r *tfReader) broken(rng hcl.Range) bool {
	return r.pf != nil && r.pf.suspect(rng)
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
		case b.Type == "ephemeral" && len(b.Labels) == 2:
			res = Resource{Tool: ToolTerraform, Mode: ModeEphemeral, Type: b.Labels[0], Name: b.Labels[1]}
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
				c := r.classifyAttr(a, nil)
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
		if prefix == "lifecycle" && a.Name == "ignore_changes" && !r.broken(a.SrcRange) {
			c = ignoreChanges(a.Expr, r.cfg.values)
		}
		if c.class == "" {
			c = r.classifyAttr(a, iters)
		}
		r.emit(w.file, out, p, c, spanOf(a.SrcRange))
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
				r.emit(w.file, out, p, r.classifyAttr(fe, iters), spanOf(fe.SrcRange))
			}
			for _, cb := range b.Body.Blocks {
				if cb.Type == "content" {
					r.walkBody(cb.Body, p, w, inner, depth+1, out)
				}
			}
			continue
		}
		// Every nested block segment takes [*], as tofu lists blocks, so one schema attribute has one
		// spelling whether a resource writes one block or several. The resource's own lifecycle block is
		// meta-configuration and not a schema block.
		p := joinPath(prefix, b.Type)
		if !(top && b.Type == "lifecycle") {
			p += "[*]"
		}
		r.walkBody(b.Body, p, w, iters, depth+1, out)
	}
	w.top = top
}

// overrideKey names the resource an override block changes.
type overrideKey struct {
	mode       string
	typ, rname string
}

// overrideIndex finds, without scanning, the base resource an override block changes and the attributes of that
// resource under one top segment. A scan per override attribute is quadratic in the attribute count: 40,000 base
// and 40,000 override attributes took 34 s.
type overrideIndex struct {
	res  map[overrideKey]int
	segs map[int]map[string][]int
}

func newOverrideIndex(set *ResourceSet) *overrideIndex {
	ix := &overrideIndex{res: map[overrideKey]int{}, segs: map[int]map[string][]int{}}
	for i := range set.Resources {
		k := overrideKey{set.Resources[i].Mode, set.Resources[i].Type, set.Resources[i].Name}
		if _, dup := ix.res[k]; !dup {
			ix.res[k] = i
		}
	}
	return ix
}

// segments is base resource i's attribute indexes by top segment, built on first use.
func (ix *overrideIndex) segments(set *ResourceSet, i int) map[string][]int {
	m, ok := ix.segs[i]
	if !ok {
		m = map[string][]int{}
		for j := range set.Resources[i].Attrs {
			seg := topSegment(set.Resources[i].Attrs[j].Path)
			m[seg] = append(m[seg], j)
		}
		ix.segs[i] = m
	}
	return m
}

// applyOverrides reads an override file's blocks and marks, in the resource each one overrides, every
// attribute or nested block it names as unknown. A block with no base is ignored, as OpenTofu refuses it.
func (r *tfReader) applyOverrides(set *ResourceSet, ix *overrideIndex, body *hclsyntax.Body) error {
	var over ResourceSet
	if err := r.readBody(&over, body); err != nil {
		return err
	}
	for _, o := range over.Resources {
		ix.apply(set, o)
	}
	return nil
}

// apply changes the base resource an override resource names, if there is one.
func (ix *overrideIndex) apply(set *ResourceSet, o Resource) {
	i, ok := ix.res[overrideKey{o.Mode, o.Type, o.Name}]
	if !ok {
		return
	}
	base := &set.Resources[i]
	if o.Multiplicity.Kind != MultiSingle {
		base.Multiplicity = Multiplicity{Kind: o.Multiplicity.Kind, Class: ClassUnknown, Refs: o.Multiplicity.Refs, Span: o.Multiplicity.Span}
	}
	if o.Provider != "" {
		base.Provider = o.Provider
	}
	for _, a := range o.Attrs {
		ix.overrideAttr(set, i, a)
	}
}

// topSegment is the first segment of an attribute path: the attribute, or the nested block it sits in.
func topSegment(p string) string {
	if i := strings.IndexAny(p, ".["); i >= 0 {
		return p[:i]
	}
	return p
}

// overrideAttr reads unknown every attribute of base resource i under the same top segment as a, which an
// override replaces whole when it is a nested block, and adds a when the base never set it.
func (ix *overrideIndex) overrideAttr(set *ResourceSet, i int, a Attr) {
	r := &set.Resources[i]
	seg := topSegment(a.Path)
	segs := ix.segments(set, i)
	if held := segs[seg]; len(held) > 0 {
		for _, j := range held {
			r.Attrs[j] = Attr{Path: r.Attrs[j].Path, Class: ClassUnknown, Span: r.Attrs[j].Span}
		}
		return
	}
	r.Attrs = append(r.Attrs, Attr{Path: a.Path, Class: ClassUnknown, Span: a.Span})
	segs[seg] = append(segs[seg], len(r.Attrs)-1)
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

// classifyAttr classifies an attribute's expression, or reads it unknown when a syntax error points into it.
func (r *tfReader) classifyAttr(a *hclsyntax.Attribute, iters map[string]bool) readRes {
	if r.broken(a.SrcRange) {
		return readRes{class: ClassUnknown}
	}
	return r.classify(a.Expr, iters, 0)
}

// classify classifies one expression. iters are the dynamic-block iterator names in scope.
func (r *tfReader) classify(e hclsyntax.Expression, iters map[string]bool, depth int) readRes {
	if r.err != nil {
		return readRes{class: ClassUnknown}
	}
	if r.nodes++; r.nodes > MaxNodes {
		r.fail(r.file, fmt.Sprintf("more than %d expression nodes", MaxNodes))
		return readRes{class: ClassUnknown}
	}
	if depth > MaxDepth {
		r.fail(r.file, fmt.Sprintf("expressions nest past %d deep", MaxDepth))
		return readRes{class: ClassUnknown}
	}
	switch x := e.(type) {
	case *hclsyntax.LiteralValueExpr:
		if !x.Val.IsKnown() {
			// The parser's placeholder for an expression it could not read.
			return readRes{class: ClassUnknown}
		}
		return readRes{class: ClassLiteral, val: ctyToGo(x.Val), hasV: r.cfg.values}
	case *hclsyntax.UnaryOpExpr:
		// A negative number is a constant as written, as it is for OpenTofu: from_port = -1. Any other unary
		// operator is computed, and so is a negation of anything but a number literal.
		if lit, ok := x.Val.(*hclsyntax.LiteralValueExpr); ok && x.Op == hclsyntax.OpNegate && lit.Val.IsKnown() && lit.Val.Type() == cty.Number {
			return readRes{class: ClassLiteral, val: ctyToGo(lit.Val.Negate()), hasV: r.cfg.values}
		}
		return readRes{class: ClassExpression, refs: r.variableRefs(e, iters)}
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
		*hclsyntax.ConditionalExpr, *hclsyntax.BinaryOpExpr,
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
	lists := make([][]string, len(parts))
	for i, p := range parts {
		lists[i] = p.refs
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
	out.refs = unionRefs(lists...)
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
		if (root == "var" && r.env.overVars[attr.Name]) || (root == "local" && r.env.overLocals[attr.Name]) {
			return readRes{class: ClassUnknown, refs: []string{ref}}
		}
		var expr hclsyntax.Expression
		secret := false
		if root == "var" {
			if v := r.env.vars[attr.Name]; v != nil {
				expr, secret = v.def, v.secret
			}
		} else {
			expr = r.env.locals[attr.Name]
		}
		if expr == nil {
			return readRes{class: ClassReference, refs: []string{ref}}
		}
		inner, done := r.inners[key]
		if !done {
			if r.resolving[key] {
				r.cuts++
				return readRes{class: ClassUnknown, refs: []string{ref}}
			}
			r.resolving[key] = true
			cuts := r.cuts
			inner = r.classify(expr, nil, depth+1)
			delete(r.resolving, key)
			if r.cuts == cuts {
				r.inners[key] = inner
				r.hold(r.file, inner)
			}
		}
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
		if secret {
			// The class stays: a check still learns that a default is there. The value does not.
			out.val, out.hasV = nil, false
		}
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
