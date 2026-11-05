package iacfacts

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type bindRequired struct {
	source     string
	constraint string
}

type bindCall struct {
	name string
	// dir is the callee's repository-relative directory; empty for a source that is not a local path.
	dir string
	// providers maps the callee's provider key ("aws" or "aws.east") to the caller's expression ("amazon.east").
	providers map[string]string
}

type bindModule struct {
	required map[string]bindRequired
	calls    []bindCall
}

type bindEdge struct {
	parent string
	call   bindCall
}

// bindingReader parses what binding needs from HCL, with a cache so one answer reads each file once.
type bindingReader struct {
	fsys  fs.FS
	mods  map[string]*bindModule
	locks map[string]map[string]string
	notes []string
}

func (r *bindingReader) note(format string, args ...any) {
	n := fmt.Sprintf(format, args...)
	for _, e := range r.notes {
		if e == n {
			return
		}
	}
	r.notes = append(r.notes, n)
}

// readBounded reads a file no larger than maxBindingFileBytes.
func (r *bindingReader) readBounded(name string) ([]byte, bool) {
	st, err := fs.Stat(r.fsys, name)
	if err != nil || !st.Mode().IsRegular() {
		return nil, false
	}
	if st.Size() > maxBindingFileBytes {
		r.note("%s is %d bytes, over the %d byte read bound, and was skipped", name, st.Size(), maxBindingFileBytes)
		return nil, false
	}
	b, err := fs.ReadFile(r.fsys, name)
	if err != nil {
		return nil, false
	}
	return b, true
}

func (r *bindingReader) parse(src []byte, name string) *hclsyntax.Body {
	f, diags := hclsyntax.ParseConfig(src, name, hcl.InitialPos)
	if diags.HasErrors() {
		r.note("%s has HCL errors; blocks it parsed before the error were read", name)
	}
	if f == nil {
		return nil
	}
	body, _ := f.Body.(*hclsyntax.Body)
	return body
}

// module reads the required_providers and module calls of every .tf file in dir.
func (r *bindingReader) module(dir string) *bindModule {
	if m, ok := r.mods[dir]; ok {
		return m
	}
	m := &bindModule{required: map[string]bindRequired{}}
	r.mods[dir] = m
	ents, err := fs.ReadDir(r.fsys, dir)
	if err != nil {
		return m
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tf") {
			continue
		}
		if n++; n > maxBindingTFFiles {
			r.note("%s holds more than %d .tf files; the rest were not read", dir, maxBindingTFFiles)
			break
		}
		name := path.Join(dir, e.Name())
		src, ok := r.readBounded(name)
		if !ok {
			continue
		}
		body := r.parse(src, name)
		if body == nil {
			continue
		}
		for _, blk := range body.Blocks {
			switch blk.Type {
			case "terraform":
				for _, inner := range blk.Body.Blocks {
					if inner.Type == "required_providers" {
						readRequiredProviders(inner.Body, m.required)
					}
				}
			case "module":
				if len(blk.Labels) == 1 {
					m.calls = append(m.calls, readModuleCall(dir, blk))
				}
			}
		}
	}
	return m
}

func literalString(e hclsyntax.Expression) (string, bool) {
	v, diags := e.Value(nil)
	if diags.HasErrors() || v.IsNull() || !v.IsKnown() || v.Type().FriendlyName() != "string" {
		return "", false
	}
	return v.AsString(), true
}

func readRequiredProviders(body *hclsyntax.Body, into map[string]bindRequired) {
	for name, attr := range body.Attributes {
		if s, ok := literalString(attr.Expr); ok { // the legacy `aws = "~> 5.0"` form
			into[name] = bindRequired{constraint: s}
			continue
		}
		obj, ok := attr.Expr.(*hclsyntax.ObjectConsExpr)
		if !ok {
			continue
		}
		var req bindRequired
		for _, it := range obj.Items {
			key := hcl.ExprAsKeyword(it.KeyExpr)
			s, _ := literalString(it.ValueExpr)
			switch key {
			case "source":
				req.source = s
			case "version":
				req.constraint = s
			}
		}
		into[name] = req
	}
}

func readModuleCall(dir string, blk *hclsyntax.Block) bindCall {
	c := bindCall{name: blk.Labels[0], providers: map[string]string{}}
	if attr, ok := blk.Body.Attributes["source"]; ok {
		if s, ok := literalString(attr.Expr); ok && (strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../")) {
			if d := path.Clean(path.Join(dir, s)); !strings.HasPrefix(d, "../") && d != ".." {
				c.dir = d
			}
		}
	}
	if attr, ok := blk.Body.Attributes["providers"]; ok {
		if obj, ok := attr.Expr.(*hclsyntax.ObjectConsExpr); ok {
			for _, it := range obj.Items {
				k, v := traversalName(it.KeyExpr), traversalName(it.ValueExpr)
				if k != "" && v != "" {
					c.providers[k] = v
				}
			}
		}
	}
	return c
}

// traversalName renders `aws.east` or `aws` from a reference expression or object key.
func traversalName(e hclsyntax.Expression) string {
	t, diags := hcl.AbsTraversalForExpr(e)
	if diags.HasErrors() || len(t) == 0 {
		return ""
	}
	var parts []string
	for _, step := range t {
		switch s := step.(type) {
		case hcl.TraverseRoot:
			parts = append(parts, s.Name)
		case hcl.TraverseAttr:
			parts = append(parts, s.Name)
		default:
			return ""
		}
	}
	return strings.Join(parts, ".")
}

// lockfile reads a .terraform.lock.hcl into provider address to version.
func (r *bindingReader) lockfile(name string) map[string]string {
	if l, ok := r.locks[name]; ok {
		return l
	}
	if r.locks == nil {
		r.locks = map[string]map[string]string{}
	}
	out := map[string]string{}
	r.locks[name] = out
	src, ok := r.readBounded(name)
	if !ok {
		return out
	}
	body := r.parse(src, name)
	if body == nil {
		return out
	}
	for _, blk := range body.Blocks {
		if blk.Type != "provider" || len(blk.Labels) != 1 {
			continue
		}
		if attr, ok := blk.Body.Attributes["version"]; ok {
			if s, ok := literalString(attr.Expr); ok {
				out[blk.Labels[0]] = s
			}
		}
	}
	return out
}

// reaches reports whether target is root itself or a module root's calls lead to, following local sources to a
// bounded depth and count. It returns the last call edge on the way in.
func (r *bindingReader) reaches(root, target string) (string, *bindEdge) {
	if root == target {
		return "self", nil
	}
	type item struct {
		dir   string
		depth int
		edge  *bindEdge
	}
	seen := map[string]bool{root: true}
	queue := []item{{dir: root}}
	visited := 0
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		if visited++; visited > maxBindingModules {
			r.note("the module walk from %s stopped at %d modules", root, maxBindingModules)
			return "", nil
		}
		if it.depth >= maxBindingModuleDepth {
			r.note("the module walk from %s stopped at depth %d", root, maxBindingModuleDepth)
			continue
		}
		for _, c := range r.module(it.dir).calls {
			if c.dir == "" || seen[c.dir] {
				continue
			}
			seen[c.dir] = true
			e := &bindEdge{parent: it.dir, call: c}
			if c.dir == target {
				return "module call", e
			}
			queue = append(queue, item{dir: c.dir, depth: it.depth + 1, edge: e})
		}
	}
	return "", nil
}

// checkMapping compares the provider a call's `providers` map hands the callee with the callee's own source for
// that name. Terraform rejects a mismatch, so a block behind one has no honest binding.
func (r *bindingReader) checkMapping(e *bindEdge, local, alias, childSource string) string {
	key := local
	if alias != "" {
		key = local + "." + alias
	}
	expr, ok := e.call.providers[key]
	if !ok && alias != "" {
		expr, ok = e.call.providers[local]
	}
	if !ok {
		return ""
	}
	parentLocal, _, _ := strings.Cut(expr, ".")
	req := r.module(e.parent).required[parentLocal]
	parentSource := normalizeProviderSource(req.source, parentLocal)
	if sameProvider(parentSource, childSource) {
		return ""
	}
	return fmt.Sprintf("module %q in %s maps %s to %s, which is %s, but the module's own provider is %s; Terraform rejects the pair", e.call.name, e.parent, key, expr, parentSource, childSource)
}

// findLockfiles walks the repository for .terraform.lock.hcl, skipping version-control and dependency trees.
func findLockfiles(fsys fs.FS) ([]string, []string) {
	var out, notes []string
	visited := 0
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if visited++; visited > maxBindingWalk {
			notes = append(notes, fmt.Sprintf("the lockfile search stopped after %d entries", maxBindingWalk))
			return fs.SkipAll
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".terraform", "node_modules", ".hg":
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() == LockfileName {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out, notes
}
