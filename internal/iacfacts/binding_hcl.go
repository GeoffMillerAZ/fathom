package iacfacts

import (
	"errors"
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
	// dynamic marks a source that is not a literal string (`var.src`, `"./${x}"`): OpenTofu resolves it, the reader
	// cannot, so the call may lead anywhere.
	dynamic bool
	// providers maps the callee's provider key ("aws" or "aws.east") to the caller's expression ("amazon.east").
	providers map[string]string
}

type bindModule struct {
	required map[string]bindRequired
	calls    []bindCall
	// providerVersions holds the `version` of each `provider "<name>"` block by local name. OpenTofu deprecates the
	// attribute but honours it and ANDs it with the required_providers constraint, across every block of the name.
	providerVersions map[string][]string
}

// addProviderVersion records the version constraint a provider block sets.
func (m *bindModule) addProviderVersion(name, constraint string) {
	if m.providerVersions == nil {
		m.providerVersions = map[string][]string{}
	}
	m.providerVersions[name] = append(m.providerVersions[name], constraint)
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
	// seen holds every note kept, kept their number and dropped the number that did not fit, reported the part of
	// that already said.
	seen                    map[string]struct{}
	kept, dropped, reported int
	// unread lists the reasons an input binding needed was not read. A definite answer from the inputs that were
	// read could be contradicted by the ones that were not, so any entry refuses the answer.
	unread []string
}

// maxBindingNotes bounds the disclosures one answer carries. Every note is distinct text, and a module of dynamic
// sources reached by many roots wrote one per source per root, a number of strings that grew with the product.
const maxBindingNotes = 256

// maxBindingUnread bounds the reasons kept for a refused answer, of which the answer names the first.
const maxBindingUnread = 8

func (r *bindingReader) note(format string, args ...any) {
	n := fmt.Sprintf(format, args...)
	if _, dup := r.seen[n]; dup {
		return
	}
	if r.kept >= maxBindingNotes {
		r.dropped++
		return
	}
	if r.seen == nil {
		r.seen = map[string]struct{}{}
	}
	r.seen[n] = struct{}{}
	r.kept++
	r.notes = append(r.notes, n)
}

// takeNotes returns the notes recorded since the last call, and says once more how many did not fit.
func (r *bindingReader) takeNotes() []string {
	out := r.notes
	r.notes = nil
	if r.dropped > r.reported {
		out = append(out, fmt.Sprintf("%d more disclosures past the first %d were dropped", r.dropped-r.reported, maxBindingNotes))
		r.reported = r.dropped
	}
	return out
}

// skip records an input that was not read: a disclosure, and a reason the answer is refused.
func (r *bindingReader) skip(format string, args ...any) {
	n := fmt.Sprintf(format, args...)
	r.note("%s", n)
	if len(r.unread) >= maxBindingUnread {
		return
	}
	for _, e := range r.unread {
		if e == n {
			return
		}
	}
	r.unread = append(r.unread, n)
}

// readBounded reads a file no larger than maxBindingFileBytes. A file it cannot read is skipped with a reason: a
// symbolic link is followed, as OpenTofu follows it, and one that points nowhere or at a directory is unread.
func (r *bindingReader) readBounded(name string) ([]byte, bool) {
	st, err := fs.Stat(r.fsys, name)
	if err != nil || !st.Mode().IsRegular() {
		r.skip("%s is not a readable regular file (a link that points nowhere, or at a directory) and was skipped", name)
		return nil, false
	}
	if st.Size() > maxBindingFileBytes {
		r.skip("%s is %d bytes, over the %d byte read bound, and was skipped", name, st.Size(), maxBindingFileBytes)
		return nil, false
	}
	b, err := fs.ReadFile(r.fsys, name)
	if err != nil {
		r.skip("%s could not be read and was skipped", name)
		return nil, false
	}
	return b, true
}

// parse parses an HCL file. It returns the body, which is nil for a file past the nesting bound, and whether the
// parser reported errors.
func (r *bindingReader) parse(src []byte, name string) (body *hclsyntax.Body, hadErrors bool) {
	f, diags, err := parseHCLBounded(name, src)
	if err != nil {
		r.skip("%s nests, or repeats a template piece or a function namespace, past the read bound and was skipped", name)
		return nil, true
	}
	if diags.HasErrors() {
		r.note("%s has HCL errors; blocks it parsed before the error were read", name)
		hadErrors = true
	}
	if f == nil {
		return nil, hadErrors
	}
	body, _ = f.Body.(*hclsyntax.Body)
	return body, hadErrors
}

// module reads the required_providers and module calls of every .tf, .tofu, .tf.json and .tofu.json file in dir.
func (r *bindingReader) module(dir string) *bindModule {
	if m, ok := r.mods[dir]; ok {
		return m
	}
	m := &bindModule{required: map[string]bindRequired{}}
	r.mods[dir] = m
	ents, err := fs.ReadDir(r.fsys, dir)
	if err != nil {
		// A directory that does not exist holds nothing. One that exists and cannot be listed may hold anything.
		if !errors.Is(err, fs.ErrNotExist) {
			r.skip("%s could not be listed and was skipped", dir)
		}
		return m
	}
	names := tfModuleFileNames(ents)
	if len(names) > maxBindingTFFiles {
		r.skip("%s holds more than %d configuration files; the rest were not read", dir, maxBindingTFFiles)
		names = names[:maxBindingTFFiles]
	}
	// Override files go last: they replace a required_providers entry by name. A module call in one is not
	// merged, so one that sets a source or providers refuses the answer.
	sort.SliceStable(names, func(i, j int) bool { return !isOverrideFile(names[i]) && isOverrideFile(names[j]) })
	for _, n := range names {
		name := path.Join(dir, n)
		src, ok := r.readBounded(name)
		if !ok {
			continue
		}
		override := isOverrideFile(n)
		if isTFJSON(n) {
			r.jsonModule(dir, name, src, override, m)
			continue
		}
		body, hadErrors := r.parse(src, name)
		if hadErrors {
			// The parser recovers past an error and can hand back a different block than the file declares, so what
			// it returned is not an answer: the refusal a lockfile gets.
			r.skip("%s does not parse, so a required provider or module call in it may be hidden or misread", name)
		}
		if body == nil {
			continue
		}
		for _, blk := range body.Blocks {
			switch blk.Type {
			case "terraform":
				for _, inner := range blk.Body.Blocks {
					if inner.Type == "required_providers" {
						r.readRequiredProviders(name, inner.Body, m.required)
					}
				}
			case "provider":
				if len(blk.Labels) != 1 {
					continue
				}
				attr, ok := blk.Body.Attributes["version"]
				if !ok {
					continue
				}
				if override {
					r.skip("%s sets the version of provider %q in an override file, which this reader does not merge, so the constraint is not known", name, blk.Labels[0])
					continue
				}
				if s, ok := literalString(attr.Expr); ok && strings.TrimSpace(s) != "" {
					m.addProviderVersion(blk.Labels[0], s)
				} else {
					r.skip("%s: the version of provider %q is not a non-empty literal string, which OpenTofu evaluates or rejects and this reader does not", name, blk.Labels[0])
				}
			case "module":
				if len(blk.Labels) != 1 {
					continue
				}
				if !override {
					m.calls = append(m.calls, readModuleCall(dir, blk))
				} else if _, ok := blk.Body.Attributes["source"]; ok {
					r.skipOverrideModule(name, blk.Labels[0])
				} else if _, ok := blk.Body.Attributes["providers"]; ok {
					r.skipOverrideModule(name, blk.Labels[0])
				}
			}
		}
	}
	return m
}

// literalString reads a plain quoted string from syntax. It never evaluates the expression: a for-expression or a
// long `1+1+...` in a repository file would cost memory or stack on the reader's side, and only literal syntax
// names a source, a version or a constraint. Anything else, an interpolation included, is not a literal.
func literalString(e hclsyntax.Expression) (string, bool) {
	t, ok := e.(*hclsyntax.TemplateExpr)
	if !ok || !t.IsStringLiteral() {
		return "", false
	}
	lit, ok := t.Parts[0].(*hclsyntax.LiteralValueExpr)
	if !ok || lit.Val.IsNull() || !lit.Val.IsKnown() || lit.Val.Type().FriendlyName() != "string" {
		return "", false
	}
	return lit.Val.AsString(), true
}

// skipOverrideModule refuses the answer for an override file that sets a module's source or providers. OpenTofu
// merges the override over the call, so the module graph is not the one the base file declares, and this reader
// does not merge it.
func (r *bindingReader) skipOverrideModule(file, module string) {
	r.skip("%s overrides the source or providers of module %q, which this reader does not merge, so the module graph is not known", file, module)
}

// readRequiredProviders reads one required_providers block. An entry, or a source or version in one, that is not a
// literal string is something OpenTofu evaluates or rejects and this reader does not, so it refuses the answer and
// is never read as an entry that states nothing.
func (r *bindingReader) readRequiredProviders(file string, body *hclsyntax.Body, into map[string]bindRequired) {
	names := make([]string, 0, len(body.Attributes))
	for name := range body.Attributes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		attr := body.Attributes[name]
		if s, ok := literalString(attr.Expr); ok { // the legacy `aws = "~> 5.0"` form
			if strings.TrimSpace(s) == "" {
				r.skip("%s: required_providers entry %q has an empty version constraint, which OpenTofu rejects", file, name)
				continue
			}
			into[name] = bindRequired{constraint: s}
			continue
		}
		obj, ok := attr.Expr.(*hclsyntax.ObjectConsExpr)
		if !ok {
			r.skip("%s: required_providers entry %q is not a literal string or object and was not read", file, name)
			continue
		}
		var req bindRequired
		for _, it := range obj.Items {
			key, ok := requiredKey(it.KeyExpr)
			if !ok {
				r.skip("%s: required_providers entry %q has a key that is neither a bare word nor a quoted string, which OpenTofu rejects", file, name)
				continue
			}
			switch key {
			case "source", "version":
			case "configuration_aliases":
				continue
			default:
				r.skip("%s: required_providers entry %q has the key %q, which OpenTofu rejects", file, name, key)
				continue
			}
			s, ok := literalString(it.ValueExpr)
			if !ok || (key == "version" && strings.TrimSpace(s) == "") || (key == "source" && strings.TrimSpace(s) == "") {
				r.skip("%s: the %s of required_providers entry %q is not a non-empty literal string and was not read", file, key, name)
				continue
			}
			if key == "source" {
				req.source = s
			} else {
				req.constraint = s
			}
		}
		into[name] = req
	}
}

// requiredKey reads an object key in a required_providers entry as OpenTofu does: a bare word is itself and a
// quoted string is its literal. A parenthesised key is an expression, and anything else is no key at all.
func requiredKey(e hclsyntax.Expression) (string, bool) {
	k, ok := e.(*hclsyntax.ObjectConsKeyExpr)
	if !ok || k.ForceNonLiteral {
		return "", false
	}
	if kw := hcl.ExprAsKeyword(k.Wrapped); kw != "" {
		return kw, true
	}
	return literalString(k.Wrapped)
}

func readModuleCall(dir string, blk *hclsyntax.Block) bindCall {
	c := bindCall{name: blk.Labels[0], providers: map[string]string{}}
	if attr, ok := blk.Body.Attributes["source"]; ok {
		if s, ok := literalString(attr.Expr); !ok {
			c.dynamic = true
		} else {
			c.dir = localModuleDir(dir, s)
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

// localModuleDir resolves a literal module source against the calling directory. It is empty for a source that is
// not a local path, and for one that leaves the repository.
func localModuleDir(dir, source string) string {
	if !strings.HasPrefix(source, "./") && !strings.HasPrefix(source, "../") {
		return ""
	}
	d := path.Clean(path.Join(dir, source))
	if strings.HasPrefix(d, "../") || d == ".." {
		return ""
	}
	return d
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

// lockfile reads a .terraform.lock.hcl into provider address to version. A lockfile that does not parse, or is not
// read, is unread: a pin it holds may be hidden by the fault, so the answer is refused and never made from the pins
// that parsed.
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
	body, hadErrors := r.parse(src, name)
	if hadErrors {
		r.skip("%s does not parse (a merge conflict marker is the usual cause), so a provider it pins may be hidden", name)
	}
	if body == nil {
		return out
	}
	for _, blk := range body.Blocks {
		if blk.Type != "provider" {
			continue
		}
		if len(blk.Labels) != 1 {
			r.skip("%s holds a provider block with %d labels, which is not a lock entry, so a pin it holds may be hidden", name, len(blk.Labels))
			continue
		}
		attr, ok := blk.Body.Attributes["version"]
		if !ok {
			r.skip("%s: the lock entry for %q has no version, so what it pins is not known", name, blk.Labels[0])
			continue
		}
		s, ok := literalString(attr.Expr)
		if !ok {
			r.skip("%s: the version of the lock entry for %q is not a literal string, so what it pins is not known", name, blk.Labels[0])
			continue
		}
		out[strings.ToLower(blk.Labels[0])] = s
	}
	return out
}

// reaches reports whether target is root itself or a module root's calls lead to, following local sources to a
// bounded depth and count. It returns every call edge that enters target, so each one's provider mapping is checked,
// and whether the walk stopped before it saw every module: a bound, or a module call whose source is not a literal
// and so may lead to target.
func (r *bindingReader) reaches(root, target string) (via string, edges []*bindEdge, truncated bool) {
	if root == target {
		return "self", nil, false
	}
	type item struct {
		dir   string
		depth int
	}
	seen := map[string]bool{root: true}
	queue := []item{{dir: root}}
	visited := 0
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		if visited++; visited > maxBindingModules {
			r.note("the module walk from %s stopped at %d modules", root, maxBindingModules)
			truncated = true
			break
		}
		if it.depth >= maxBindingModuleDepth {
			r.note("the module walk from %s stopped at depth %d", root, maxBindingModuleDepth)
			truncated = true
			continue
		}
		for _, c := range r.module(it.dir).calls {
			if c.dir == "" {
				if c.dynamic {
					r.note("module %q in %s has a source that is not a literal string, which OpenTofu resolves and this reader does not; the walk from %s may miss a module", c.name, it.dir, root)
					truncated = true
				}
				continue
			}
			if c.dir == target {
				edges = append(edges, &bindEdge{parent: it.dir, call: c})
				continue
			}
			if seen[c.dir] {
				continue
			}
			seen[c.dir] = true
			queue = append(queue, item{dir: c.dir, depth: it.depth + 1})
		}
	}
	if len(edges) > 0 {
		return "module call", edges, truncated
	}
	return "", nil, truncated
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

// findLockfiles walks the repository for .terraform.lock.hcl, skipping version-control and dependency trees. A
// directory it could not read may hold a lockfile, so each one goes to unread and refuses the answer.
func findLockfiles(fsys fs.FS) (out, notes, unread []string, truncated bool) {
	visited := 0
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			msg := fmt.Sprintf("%s could not be read during the lockfile search and was skipped", p)
			for _, e := range unread {
				if e == msg {
					return nil
				}
			}
			unread = append(unread, msg)
			return nil
		}
		if visited++; visited > maxBindingWalk {
			notes = append(notes, fmt.Sprintf("the lockfile search stopped after %d entries", maxBindingWalk))
			truncated = true
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
	return out, notes, unread, truncated
}
