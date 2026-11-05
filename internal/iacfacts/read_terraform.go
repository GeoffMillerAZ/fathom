package iacfacts

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"path"
	"sort"
	"strconv"
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
// relation and is never expanded, and no directory but dir is listed. Past the product's
// limits the whole directory is refused with ErrLimit.
//
// A symbolic link in dir is followed, as OpenTofu follows it, so the file it names is read wherever it lies: under
// os.DirFS that can be outside dir and outside the repository. A caller that must not read there passes an fs.FS that
// refuses to follow links. The target is looked at before it is opened. Anything but a regular file (a directory, a
// FIFO or device, a link that points nowhere) is not opened and counts as a syntax error.
//
// It follows OpenTofu's file rules. A .tofu file shadows the .tf file of the same name, which
// is not read at all. An override file (override.tf, *_override.tf, and the .tofu spellings)
// adds no resource, and the resource is read once. In the resource it overrides, every attribute the base has
// under a name the override uses reads unknown, a name being an attribute or a nested block (an empty override
// block names it too), and every attribute the override writes under that name is added, unknown, when the base
// has none at that path. OpenTofu replaces the base's blocks of a type whole, so an unknown base attribute may in
// fact be absent. A variable default or local it overrides reads unknown. Attributes are not merged, since the
// value that wins is not worth the guess.
//
// It skips what OpenTofu skips (dot files, editor backups). The JSON syntax (.tf.json and
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
		if link[n] {
			// Look at the target before opening it: opening a FIFO blocks until a writer comes.
			if st, err := fs.Stat(fsys, p); err != nil || !st.Mode().IsRegular() {
				set.SyntaxErrors++
				continue
			}
		}
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
		overVars: map[string]bool{}, overLocals: map[string]bool{}, srcs: map[string][]byte{}}
	for _, f := range files {
		env.srcs[f.file] = f.src
	}
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
	bidx     *badLines
}

// suspect reports whether what the parser returned for rng cannot be trusted. The parser reports the first syntax
// error and then recovers silently, so a later broken attribute reads as a clean literal with the error count left
// at one. An attribute a reported error touches is suspect. So is any attribute at or after the first error, unless
// its own lines parse clean on their own, which is the one proof the recovered parse cannot give.
func (pf *tfParsed) suspect(rng hcl.Range) bool {
	ix := pf.badIndex()
	if rng.Filename == pf.file {
		// The errors whose first line is at or before the attribute's last line, and the furthest line any of them
		// reaches: one binary search, not a scan of every error per attribute.
		if k := sort.SearchInts(ix.starts, rng.End.Line+1); k > 0 && ix.reach[k-1] >= rng.Start.Line {
			return true
		}
	}
	if pf.nerr > 0 && len(pf.bad) == 0 {
		return true // errors with no position: nothing is known to be clean
	}
	if ix.first == 0 || rng.End.Line < ix.first {
		return false
	}
	return !pf.linesParseClean(rng.Start.Line, rng.End.Line)
}

// badLines indexes the lines the parser's errors touch: the first line of each error in ascending order, the
// furthest last line among the errors up to each, and the earliest first line.
type badLines struct {
	starts, reach []int
	first         int
}

func (pf *tfParsed) badIndex() *badLines {
	if pf.bidx != nil {
		return pf.bidx
	}
	type span struct{ start, last int }
	spans := make([]span, 0, len(pf.bad))
	for _, b := range pf.bad {
		last := b.End.Line
		if b.End.Column == 1 && last > b.Start.Line {
			last-- // a newline token ends at the start of the next line, which it does not occupy
		}
		spans = append(spans, span{b.Start.Line, last})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	ix := &badLines{starts: make([]int, len(spans)), reach: make([]int, len(spans))}
	for i, s := range spans {
		ix.starts[i] = s.start
		ix.reach[i] = s.last
		if i > 0 && ix.reach[i-1] > s.last {
			ix.reach[i] = ix.reach[i-1]
		}
	}
	if len(spans) > 0 {
		ix.first = spans[0].start
	}
	pf.bidx = ix
	return ix
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
	// sensitive and ephemeral are the two flags, held apart. A flag is on unless it is the literal false, and an
	// expression the reader cannot tell from true is on. An override replaces one flag and leaves the other, so
	// only both together say whether the variable is secret.
	sensitive, ephemeral bool
}

// secret reports whether the variable is sensitive or ephemeral. Its default is never kept as a value, whatever the
// caller asks.
func (v *tfVar) secret() bool { return v.sensitive || v.ephemeral }

// setFlag sets one of the two flags: "sensitive" or "ephemeral".
func (v *tfVar) setFlag(flag string, on bool) {
	if flag == "sensitive" {
		v.sensitive = on
	} else {
		v.ephemeral = on
	}
}

// tfEnv is what ReadTerraformModule resolves against: variable defaults and locals.
type tfEnv struct {
	vars   map[string]*tfVar
	locals map[string]hclsyntax.Expression
	// overVars and overLocals name the variable defaults and locals an override file replaces. Which value
	// wins is not read, so a reference to one reads unknown.
	overVars, overLocals map[string]bool
	// srcs holds the source of each file of the module by the name its ranges carry. A local or a variable default
	// is classified while another file is read, and the text of its literal is in its own file.
	srcs map[string][]byte
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
		for n, flags := range pf.jsonOver.flags {
			if v, ok := e.vars[n]; ok {
				for flag, on := range flags {
					v.setFlag(flag, on)
				}
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
			// The override's sensitive replaces the base's sensitive, and its ephemeral the base's ephemeral, and
			// neither touches the other: tofu keeps sensitive when only ephemeral is overridden to false. Only the
			// literal false clears a flag, and an expression the reader cannot tell from true counts as true, as it
			// does in the base.
			if v, ok := e.vars[b.Labels[0]]; ok {
				for _, flag := range []string{"sensitive", "ephemeral"} {
					if a, ok := b.Body.Attributes[flag]; ok {
						v.setFlag(flag, !isFalseLiteral(a.Expr) || pf.suspect(a.SrcRange))
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
				if a, ok := b.Body.Attributes[flag]; ok {
					v.setFlag(flag, !isFalseLiteral(a.Expr) || pf.suspect(a.SrcRange))
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
	// varBytes is the source bytes variableRefs has walked.
	varBytes int
	// override is set while an override file is read, to collect the nested blocks each resource names.
	override bool
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
			r.unreadBlock(set, b)
			continue
		}
		res.Span = spanOf(b.Range())
		res.Multiplicity = Multiplicity{Kind: MultiSingle}
		// Meta-arguments that are not attributes of the resource.
		for _, a := range sortedAttrs(b.Body) {
			switch a.Name {
			case "provider":
				res.Provider = providerRef(a.Expr)
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
		res.named = walk.blocks
		set.Resources = append(set.Resources, res)
	}
	return r.err
}

// providerRef is the provider configuration a `provider =` meta-argument names: tls.by for tls.by and for
// tls.by[each.key], whose instance key picks one instance of the configuration and is not part of its name (OpenTofu
// reports provider_config_key tls.by). It is empty for an expression that is not a traversal.
func providerRef(e hclsyntax.Expression) string {
	for {
		ix, ok := e.(*hclsyntax.IndexExpr)
		if !ok {
			break
		}
		e = ix.Collection
	}
	if t, ok := e.(*hclsyntax.ScopeTraversalExpr); ok {
		return renderTraversal(t.Traversal)
	}
	return ""
}

// unreadBlock accounts for a top-level block readBody did not read as a resource, so that absence is never silent:
// a resource, data, ephemeral or module block with the wrong number of labels, which OpenTofu refuses, counts as a
// syntax error, and a data source inside a check block, which exists but is scoped to the check, is Dropped.
func (r *tfReader) unreadBlock(set *ResourceSet, b *hclsyntax.Block) {
	switch b.Type {
	case "resource", "data", "ephemeral", "module":
		set.SyntaxErrors++
	case "check":
		name := ""
		if len(b.Labels) > 0 {
			name = b.Labels[0]
		}
		for _, nb := range b.Body.Blocks {
			switch {
			case nb.Type == "data" && len(nb.Labels) == 2:
				set.Dropped = append(set.Dropped, "check."+name+".data."+nb.Labels[0]+"."+nb.Labels[1])
			case nb.Type == "data":
				set.SyntaxErrors++
			}
		}
	}
}

// segmentFits reports whether a nested block's path, prefix plus name, is inside MaxPathBytes, and fails the read when
// it is not. Only emit checked this, after the path of every block was built, so a long block name repeated over
// many empty blocks copied it once per block without an attribute to refuse it.
func (r *tfReader) segmentFits(file, prefix, name string) bool {
	if len(prefix)+1+len(name) > MaxPathBytes {
		r.fail(file, fmt.Sprintf("an attribute path is over %d bytes", MaxPathBytes))
		return false
	}
	return true
}

// tfWalk is the state of one resource walk.
type tfWalk struct {
	file string
	top  bool // the walk is at the resource's own body
	// blocks collects, in an override file, the nested blocks the resource names, empty ones too.
	blocks []string
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
			if !r.segmentFits(w.file, prefix, name) {
				return
			}
			if top && r.override {
				w.blocks = append(w.blocks, name)
			}
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
		if !r.segmentFits(w.file, prefix, b.Type) {
			return
		}
		if top && r.override && b.Type != "lifecycle" {
			w.blocks = append(w.blocks, b.Type)
		}
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
	// done marks the top segments of a base resource an override already read unknown: a second override of the
	// same segment has nothing left to mark, and scanning the segment again for each one is quadratic.
	done map[int]map[string]bool
	// paths holds the attribute paths of a base resource, base and added, built on first use, so an override
	// attribute is added once however many overrides name it.
	paths map[int]map[string]bool
}

func newOverrideIndex(set *ResourceSet) *overrideIndex {
	ix := &overrideIndex{res: map[overrideKey]int{}, segs: map[int]map[string][]int{}, done: map[int]map[string]bool{}, paths: map[int]map[string]bool{}}
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
	r.override = true
	err := r.readBody(&over, body)
	r.override = false
	if err != nil {
		return err
	}
	set.SyntaxErrors += over.SyntaxErrors
	set.Dropped = append(set.Dropped, over.Dropped...)
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
	// An override replaces the attributes and nested blocks it names, whole. The reader does not guess the values
	// that win: every attribute of a base resource under a name the override uses reads unknown, and every
	// attribute the override writes under it is added, unknown, when the base has none at that path. A nested
	// block the override writes empty names the segment too, with no attribute to say so.
	var order []string
	group := map[string][]Attr{}
	for _, name := range o.named {
		if _, ok := group[name]; !ok {
			order = append(order, name)
			group[name] = nil
		}
	}
	for _, a := range o.Attrs {
		seg := topSegment(a.Path)
		if _, ok := group[seg]; !ok {
			order = append(order, seg)
		}
		group[seg] = append(group[seg], a)
	}
	for _, seg := range order {
		ix.overrideSeg(set, i, seg, group[seg])
	}
}

// topSegment is the first segment of an attribute path: the attribute, or the nested block it sits in.
func topSegment(p string) string {
	if i := strings.IndexAny(p, ".["); i >= 0 {
		return p[:i]
	}
	return p
}

// overrideSeg reads unknown every attribute of base resource i under top segment seg, once, and adds the override's
// attributes under it that the base does not have.
func (ix *overrideIndex) overrideSeg(set *ResourceSet, i int, seg string, attrs []Attr) {
	r := &set.Resources[i]
	segs := ix.segments(set, i)
	if ix.done[i] == nil {
		ix.done[i] = map[string]bool{}
	}
	if !ix.done[i][seg] {
		ix.done[i][seg] = true
		for _, j := range segs[seg] {
			r.Attrs[j] = Attr{Path: r.Attrs[j].Path, Class: ClassUnknown, Span: r.Attrs[j].Span}
		}
	}
	if len(attrs) == 0 {
		return
	}
	have := ix.paths[i]
	if have == nil {
		have = make(map[string]bool, len(r.Attrs)+len(attrs))
		for _, a := range r.Attrs {
			have[a.Path] = true
		}
		ix.paths[i] = have
	}
	for _, a := range attrs {
		if have[a.Path] {
			continue
		}
		have[a.Path] = true
		r.Attrs = append(r.Attrs, Attr{Path: a.Path, Class: ClassUnknown, Span: a.Span})
		segs[seg] = append(segs[seg], len(r.Attrs)-1)
	}
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
	size := 0
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
			name := renderTraversal(t.Traversal)
			names = append(names, name)
			size += len(name) + 1
		}
		if names == nil {
			names = []any{}
		}
		return readRes{class: ClassLiteral, val: names, hasV: values}.withBytes(size)
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
		return readRes{class: ClassLiteral, val: r.literalValue(x, false), hasV: r.cfg.values}
	case *hclsyntax.UnaryOpExpr:
		// A negative number is a constant as written, as it is for OpenTofu: from_port = -1. Any other unary
		// operator is computed, and so is a negation of anything but a number literal.
		if lit, ok := x.Val.(*hclsyntax.LiteralValueExpr); ok && x.Op == hclsyntax.OpNegate && lit.Val.IsKnown() && lit.Val.Type() == cty.Number {
			return readRes{class: ClassLiteral, val: r.literalValue(lit, true), hasV: r.cfg.values}
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
		for _, k := range keys {
			// A literal key past MaxValueBytes is a value past it, though it is no part.
			out.big = out.big || k.big
		}
		if out.hasV && !out.big {
			m := make(map[string]any, len(x.Items))
			keyBytes := 0
			for i := range keys {
				ks, ok := keys[i].val.(string)
				if !ok {
					out.hasV = false
					out.val = nil
					return out
				}
				m[ks] = vals[i].val
				if keys[i].class == ClassLiteral {
					// A computed key is one of parts and is sized there.
					keyBytes += len(ks) + 1
				}
			}
			out.val = m
			out = out.withBytes(keyBytes)
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
		if src.class == ClassExpression && refsAreVariables(x.Source) {
			// The source's refs are all of its variables, so the splat's are those and its own steps'. Walking the
			// whole expression again at each nested splat is quadratic in its size times its depth.
			if steps := r.variableRefs(x.Each, iters); len(steps) > 0 {
				return readRes{class: ClassExpression, refs: unionRefs(src.refs, steps)}
			}
			return readRes{class: ClassExpression, refs: src.refs}
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
		// The concat below builds a real string, kept or not: a list holding it and a reference is no literal, and
		// drops it. Charge it first, so a template read under a thousand attributes cannot build gigabytes that no
		// emit ever sees.
		out = r.hold(r.file, out)
		if r.err != nil {
			out.hasV = false
			return out
		}
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
		// cty prints a number in positional form, whatever its size: 1e6 is 1000000 and 0.00001 is 0.00001.
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case json.Number:
		// cty holds a number at 512 bits and prints it positionally. A text with an exponent past what a float64
		// holds by a long way would print as a million digits, so such a number has no known string.
		f, _, err := big.ParseFloat(string(x), 10, 512, big.ToNearestEven)
		if err != nil {
			return "", false
		}
		if exp := f.MantExp(nil); exp > 4096 || exp < -4096 {
			return "", false
		}
		return f.Text('f', -1), true
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
	case "each", "count", "self", "path", "terraform", "tofu":
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
				expr, secret = v.def, v.secret()
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

// maxVariableWalkBytes bounds the source bytes the reference search of one file walks, counting a byte once for every
// walk that covers it.
const maxVariableWalkBytes = 32 << 20

// refsAreVariables reports whether the refs classify gave an expression of class Expression are exactly the
// variables of that expression: true of a splat, a parenthesis around one, and of every kind classify sends to
// variableRefs.
func refsAreVariables(e hclsyntax.Expression) bool {
	for {
		p, ok := e.(*hclsyntax.ParenthesesExpr)
		if !ok {
			break
		}
		e = p.Expression
	}
	switch e.(type) {
	case *hclsyntax.SplatExpr, *hclsyntax.RelativeTraversalExpr, *hclsyntax.IndexExpr, *hclsyntax.FunctionCallExpr,
		*hclsyntax.ConditionalExpr, *hclsyntax.BinaryOpExpr, *hclsyntax.ForExpr, *hclsyntax.TemplateJoinExpr,
		*hclsyntax.AnonSymbolExpr:
		return true
	}
	return false
}

// variableRefs lists the static references an expression reads, minus the names that
// are scopes and not things.
func (r *tfReader) variableRefs(e hclsyntax.Expression, iters map[string]bool) []string {
	// Every walk is charged its source span, so a shape that walks the same bytes again and again is refused by
	// size and not by the minutes it takes.
	rng := e.Range()
	if r.varBytes += rng.End.Byte - rng.Start.Byte; r.varBytes > maxVariableWalkBytes {
		r.fail(r.file, fmt.Sprintf("expressions were searched for references over %d bytes in all", maxVariableWalkBytes))
		return nil
	}
	var out []string
	for _, t := range e.Variables() {
		switch t.RootName() {
		case "each", "count", "self", "path", "terraform", "tofu":
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

// literalValue is the Go value of a literal, negated when neg. A number goes through numberValue with the text the
// file wrote, the rule every reader shares, so 1e999 is never +Inf and 1.0 is a float64 as it is in CloudFormation and
// Pulumi. Only when the text cannot be found does it fall back on cty's value, which spells 1.0 and 1 the same.
func (r *tfReader) literalValue(lit *hclsyntax.LiteralValueExpr, neg bool) any {
	v := lit.Val
	if neg {
		v = v.Negate()
	}
	if v.IsNull() || !v.IsKnown() || v.Type() != cty.Number {
		return ctyToGo(v)
	}
	bf := v.AsBigFloat()
	// The text the file wrote, when it can be found, is read by the one rule every reader shares; cty has kept only
	// the value, which spells 1.0 and 1 the same.
	if src := r.sourceOf(lit.SrcRange.Filename); src != nil {
		if rng := lit.SrcRange; rng.Start.Byte >= 0 && rng.End.Byte <= len(src) && rng.Start.Byte < rng.End.Byte {
			if t := string(src[rng.Start.Byte:rng.End.Byte]); jsonNumberType(t) != "" {
				if neg {
					t = "-" + t
				}
				return numberValue(t)
			}
		}
	}
	if bf.IsInt() {
		if i, acc := bf.Int64(); acc == big.Exact {
			return i
		}
	}
	return numberValue(bf.Text('g', -1))
}

// sourceOf is the source of the file a range names: the file being read, or another file of the module whose local or
// default is being classified. It is nil for a file the reader does not hold, and a range must never be read against
// the wrong file's bytes, which spell some other number at that offset.
func (r *tfReader) sourceOf(file string) []byte {
	if r.pf != nil && r.pf.file == file {
		return r.pf.src
	}
	if r.env != nil {
		return r.env.srcs[file]
	}
	return nil
}

// ctyToGo converts a constant to a plain Go value. A number is read by literalValue, which has the source text.
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
		return numberValue(v.AsBigFloat().Text('g', -1))
	}
	return nil
}
