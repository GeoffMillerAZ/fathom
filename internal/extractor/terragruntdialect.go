package extractor

// terragruntdialect.go — the Terragrunt native dialect (LB-Q-terragrunt,
// docs/design/language-breadth.md §13, engine "native": hclsyntax, the parser
// the terraform dialect already uses; the tags engine's grammar differs from
// the catalogue's and cannot emit a cross-file reference). It reads
// terragrunt.hcl, root.hcl and the other .hcl files a Terragrunt content sniff
// claims (an included _envcommon file, an account.hcl).
//
// Entities (dialects/terragrunt/dialect.json):
//
//   - the file (TerragruntConfig, its base name): every file's root, what an
//     include or a dependency of it resolves to;
//   - include "l" (include.l; an unlabelled one is "include"), dependency "n"
//     (dependency.n), dependencies, terraform, remote_state, generate "g"
//     (generate.g), feature "f" (feature.f);
//   - each locals attribute (local.k) and each inputs key (input.k: the keys
//     of an object literal, directly or as an argument of merge()).
//
// Intra-file references (L2, Calls): dependency.n.outputs.x is a call of the
// entity it is read in to dependency.n, local.x to local.x, include.n.x to
// include.n.
//
// Cross-file references (L3), every path read as the static value of its
// expression (tgEval):
//
//   - include.path: repo-path candidates. find_in_parent_folders("root.hcl")
//     and find_in_parent_folders() (terragrunt.hcl) name the nearest ancestor
//     file of that name, starting at the parent of the file's directory, so
//     one candidate per ancestor directory is emitted and the resolver keeps
//     those that are files of the repository, as the nginx dialect does for an
//     include. A literal, a local of the file that is itself static, and
//     dirname()/get_terragrunt_dir() of those, joined by a template, are read
//     the same way;
//   - dependency.config_path and dependencies.paths: the directory's
//     terragrunt.hcl (the value itself when it names a .hcl file);
//   - terraform.source with a local path ("./", "../" or "."): every
//     definition of the module directory, through the terraform dialect's
//     label-selector join (scope terraform:<dir>, label terraform.module),
//     and each inputs key to the module's variable of that name. A "//subdir"
//     below a local path names the module directory inside it.
//
// Only a file named terragrunt.hcl, a unit, resolves a relative path or a
// find_in_parent_folders: in a file an include brings in (a root.hcl, a
// _common/app.hcl) Terragrunt reads both from the including unit's directory,
// which one file cannot see, so every path there is unresolved ("included-file")
// and only an external terraform.source is classified.
//
// What no static reading resolves is not guessed: a function other than the
// ones above, a reference to a dependency or an include, a git::, registry or
// remote source is TerragruntUnresolvedRefs' to count by reason.

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

const (
	kindTGConfig       core.SourceKind = "TerragruntConfig"
	kindTGInclude      core.SourceKind = "TerragruntInclude"
	kindTGDependency   core.SourceKind = "TerragruntDependency"
	kindTGDependencies core.SourceKind = "TerragruntDependencies"
	kindTGTerraform    core.SourceKind = "TerragruntTerraform"
	kindTGRemoteState  core.SourceKind = "TerragruntRemoteState"
	kindTGGenerate     core.SourceKind = "TerragruntGenerate"
	kindTGFeature      core.SourceKind = "TerragruntFeature"
	kindTGLocal        core.SourceKind = "TerragruntLocal"
	kindTGInput        core.SourceKind = "TerragruntInput"

	// Edges of the Terragrunt dialect's cross-file references (the include
	// edge is the proxy dialects' "includes").
	tgEdgeDependency = "dependency"
	tgEdgeSource     = "module-source"
)

// TerragruntExtractor implements Extractor and DocResulter for Terragrunt
// configuration files.
type TerragruntExtractor struct{}

var (
	_ Extractor   = (*TerragruntExtractor)(nil)
	_ DocResulter = (*TerragruntExtractor)(nil)
)

// NewTerragruntExtractor returns the terragrunt dialect's extractor.
func NewTerragruntExtractor() *TerragruntExtractor { return &TerragruntExtractor{} }

// Extract implements Extractor.
func (x *TerragruntExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := x.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// ExtractDoc implements DocResulter. A file hclsyntax cannot recover anything
// from yields no entities and an error, as the terraform dialect does; one
// with syntax errors yields what its other blocks declare.
func (x *TerragruntExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	r, _, err := x.extract(service, relPath, src)
	return r, err
}

// TerragruntUnresolved is one reference no static reading resolved, and why.
type TerragruntUnresolved struct {
	Line int
	// Edge is the reference's edge ("includes", "dependency", "module-source");
	// Raw the expression or source as written.
	Edge, Raw string
	// Reason is "function:<name>", "reference:<root>", "expression",
	// "non-string", "no-ancestor", "outside-repo", "absolute-path", or, for a
	// terraform.source that names nothing in the repository,
	// "external:git", "external:registry", "external:remote" or
	// "external:other".
	Reason string
}

// TerragruntUnresolvedRefs are the references of one file that ExtractDoc
// leaves unresolved, each with its reason, in source order.
func TerragruntUnresolvedRefs(relPath string, src []byte) []TerragruntUnresolved {
	_, u, _ := NewTerragruntExtractor().extract("svc", relPath, src)
	return u
}

// tgPath is one candidate value of a path expression: rooted when it is
// already repository-relative (a function's result), relative to the file's
// directory when it is written as a literal.
type tgPath struct {
	s      string
	rooted bool
}

// tgEval reads the static value of a path expression of the file at dir.
type tgEval struct {
	dir    string
	unit   bool
	locals map[string]hclsyntax.Expression
	depth  int
}

const tgMaxCandidates = 64

// eval is the candidate values of x, or the reason it has none.
func (e *tgEval) eval(x hclsyntax.Expression) ([]tgPath, string) {
	switch n := x.(type) {
	case *hclsyntax.LiteralValueExpr:
		if s, ok := hclLiteralString(n); ok {
			return []tgPath{{s: s}}, ""
		}
		return nil, "non-string"
	case *hclsyntax.ParenthesesExpr:
		return e.eval(n.Expression)
	case *hclsyntax.TemplateWrapExpr:
		return e.eval(n.Wrapped)
	case *hclsyntax.TemplateExpr:
		acc := []tgPath{{}}
		for i, part := range n.Parts {
			sub, why := e.eval(part)
			if why != "" {
				return nil, why
			}
			var next []tgPath
			for _, a := range acc {
				for _, b := range sub {
					r := a.rooted
					if i == 0 {
						r = b.rooted
					}
					next = append(next, tgPath{s: a.s + b.s, rooted: r})
				}
			}
			if len(next) > tgMaxCandidates {
				return nil, "expression"
			}
			acc = next
		}
		return acc, ""
	case *hclsyntax.ScopeTraversalExpr:
		root := n.Traversal.RootName()
		if root == "local" && len(n.Traversal) == 2 && e.depth < 8 {
			if a, ok := n.Traversal[1].(hcl.TraverseAttr); ok {
				if le, ok := e.locals[a.Name]; ok {
					e.depth++
					defer func() { e.depth-- }()
					return e.eval(le)
				}
			}
		}
		return nil, "reference:" + root
	case *hclsyntax.FunctionCallExpr:
		return e.call(n)
	}
	return nil, "expression"
}

func (e *tgEval) call(n *hclsyntax.FunctionCallExpr) ([]tgPath, string) {
	switch n.Name {
	case "get_terragrunt_dir", "get_original_terragrunt_dir":
		return []tgPath{{s: e.dir, rooted: true}}, ""
	case "dirname":
		if len(n.Args) != 1 {
			return nil, "function:dirname"
		}
		sub, why := e.eval(n.Args[0])
		if why != "" {
			return nil, why
		}
		out := make([]tgPath, 0, len(sub))
		for _, p := range sub {
			out = append(out, tgPath{s: path.Dir(p.s), rooted: p.rooted})
		}
		return out, ""
	case "find_in_parent_folders":
		name := "terragrunt.hcl"
		if len(n.Args) > 0 {
			sub, why := e.eval(n.Args[0])
			if why != "" || len(sub) != 1 || sub[0].rooted {
				return nil, "function:find_in_parent_folders"
			}
			name = sub[0].s
		}
		var out []tgPath
		for a := path.Dir(e.dir); e.dir != "."; a = path.Dir(a) {
			out = append(out, tgPath{s: path.Join(a, name), rooted: true})
			if a == "." {
				break
			}
		}
		if len(out) == 0 {
			return nil, "no-ancestor"
		}
		return out, ""
	}
	return nil, "function:" + n.Name
}

// candidates are the repository paths of a path expression's values, sorted
// and de-duplicated; reason is why there are none ("" when there are some).
func (e *tgEval) candidates(x hclsyntax.Expression) ([]string, string) {
	if !e.unit {
		return nil, "included-file"
	}
	vals, why := e.eval(x)
	if why != "" {
		return nil, why
	}
	seen := map[string]bool{}
	var out []string
	reason := ""
	for _, v := range vals {
		p := path.Clean(v.s)
		if !v.rooted {
			if strings.HasPrefix(v.s, "/") {
				reason = "absolute-path"
				continue
			}
			p = path.Join(e.dir, v.s)
		}
		if strings.HasPrefix(p, "/") {
			reason = "absolute-path"
			continue
		}
		if p == ".." || strings.HasPrefix(p, "../") {
			reason = "outside-repo"
			continue
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, reason
	}
	return out, ""
}

var (
	tgSchemeRE   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)
	tgRegistryRE = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9_-]*/[a-z0-9][a-z0-9_-]*($|[?/]))`)
)

// tgLocalSource is the module directory a terraform.source value names when
// it is a local path ("." or beginning "./" or "../"), relative to dir; for
// any other value, the reason it names nothing of the repository.
func tgLocalSource(dir, source string) (string, string) {
	switch {
	case strings.Contains(source, "::"):
		return "", "external:git"
	case strings.HasPrefix(source, "git@") || strings.HasPrefix(source, "github.com/") || strings.HasPrefix(source, "bitbucket.org/"):
		return "", "external:git"
	case strings.HasPrefix(source, "tfr:"):
		return "", "external:registry"
	case tgSchemeRE.MatchString(source):
		return "", "external:remote"
	case tgRegistryRE.MatchString(source) && !strings.HasPrefix(source, "./") && !strings.HasPrefix(source, "../"):
		return "", "external:registry"
	case source != "." && !strings.HasPrefix(source, "./") && !strings.HasPrefix(source, "../"):
		return "", "external:other"
	}
	if i := strings.IndexByte(source, '?'); i >= 0 {
		source = source[:i]
	}
	// "root//subdir": the module is subdir below root.
	source = strings.Replace(source, "//", "/", 1)
	c := path.Join(dir, source)
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", "outside-repo"
	}
	return c, ""
}

// tgDependencyFile is the file a dependency path names: the directory's
// terragrunt.hcl, or the file itself when the path ends in .hcl.
func tgDependencyFile(p string) string {
	if strings.HasSuffix(p, ".hcl") {
		return p
	}
	return path.Join(p, "terragrunt.hcl")
}

func (x *TerragruntExtractor) extract(service, relPath string, src []byte) (DocResult, []TerragruntUnresolved, error) {
	body, err := hclParseBody(relPath, src)
	if err != nil {
		return DocResult{}, nil, err
	}
	if body == nil {
		return DocResult{}, nil, nil
	}
	w := newHCLWalk(service, relPath, src)
	root := path.Base(relPath)
	w.add(root, kindTGConfig, "", 1, w.lines, nil)
	dir := path.Dir(relPath)
	ev := &tgEval{dir: dir, unit: path.Base(relPath) == "terragrunt.hcl", locals: map[string]hclsyntax.Expression{}}
	for _, b := range body.Blocks {
		if b.Type == "locals" && b.Body != nil {
			for k, a := range b.Body.Attributes {
				ev.locals[k] = a.Expr
			}
		}
	}
	var unresolved []TerragruntUnresolved
	// pathRefs records the reference an expression's static value makes, per
	// candidate, or its reason.
	pathRefs := func(from, edge string, x hclsyntax.Expression, line int, file bool) {
		cands, why := ev.candidates(x)
		raw := hclExprText(src, x)
		if why != "" {
			unresolved = append(unresolved, TerragruntUnresolved{Line: line, Edge: edge, Raw: raw, Reason: why})
			return
		}
		for _, c := range cands {
			if file {
				c = tgDependencyFile(c)
			}
			w.ref(DocCrossFileRef{From: from, Edge: edge, Raw: raw, Target: c, Strategy: "repo-path", Line: line})
		}
	}
	// the module directory of this file's terraform.source, when local
	var childDir string
	for _, b := range body.Blocks {
		if b.Type != "terraform" || b.Body == nil {
			continue
		}
		a, ok := b.Body.Attributes["source"]
		if !ok {
			continue
		}
		vals, why := ev.eval(a.Expr)
		raw := hclExprText(src, a.Expr)
		if why != "" {
			unresolved = append(unresolved, TerragruntUnresolved{Line: a.SrcRange.Start.Line, Edge: tgEdgeSource, Raw: raw, Reason: why})
			continue
		}
		for _, v := range vals {
			c, reason := tgLocalSource(dir, v.s)
			if reason == "" && !ev.unit {
				reason = "included-file"
			}
			if reason != "" {
				unresolved = append(unresolved, TerragruntUnresolved{Line: a.SrcRange.Start.Line, Edge: tgEdgeSource, Raw: raw, Reason: reason})
				continue
			}
			if childDir == "" {
				childDir = c
			}
			w.ref(DocCrossFileRef{
				From: "terraform", Edge: tgEdgeSource, Raw: v.s, Target: hclModuleLabel + "=" + c, Strategy: "label-selector",
				Line: a.SrcRange.Start.Line, Scope: "terraform:" + c, Labels: map[string]string{hclModuleLabel: c},
			})
		}
	}

	refKey := func(t hcl.Traversal) string { return tgRefKey(t) }
	for _, b := range body.Blocks {
		if b.Body == nil {
			continue
		}
		line, end := b.TypeRange.Start.Line, b.Range().End.Line
		refs := hclBodyRefs(b.Body, nil, nil, refKey, nil)
		switch b.Type {
		case "include":
			l := hclBlockLabel(b, 0)
			name := "include"
			if l != "" {
				name += "." + l
			}
			name = w.add(name, kindTGInclude, tgSig(b), line, end, refs)
			if a, ok := b.Body.Attributes["path"]; ok {
				pathRefs(name, edgeIncludes, a.Expr, a.SrcRange.Start.Line, false)
			}
		case "dependency":
			name := w.add("dependency."+hclBlockLabel(b, 0), kindTGDependency, tgSig(b), line, end, refs)
			if a, ok := b.Body.Attributes["config_path"]; ok {
				pathRefs(name, tgEdgeDependency, a.Expr, a.SrcRange.Start.Line, true)
			}
		case "dependencies":
			name := w.add("dependencies", kindTGDependencies, "dependencies", line, end, refs)
			if a, ok := b.Body.Attributes["paths"]; ok {
				for _, it := range hclTupleItems(a.Expr) {
					pathRefs(name, tgEdgeDependency, it, it.Range().Start.Line, true)
				}
			}
		case "terraform":
			w.add("terraform", kindTGTerraform, "terraform", line, end, refs)
		case "remote_state":
			w.add("remote_state", kindTGRemoteState, tgSig(b), line, end, refs)
		case "generate":
			w.add("generate."+hclBlockLabel(b, 0), kindTGGenerate, tgSig(b), line, end, refs)
		case "feature":
			w.add("feature."+hclBlockLabel(b, 0), kindTGFeature, tgSig(b), line, end, refs)
		case "locals":
			for _, a := range hclSortedAttrs(b.Body) {
				w.add("local."+a.Name, kindTGLocal, "locals { "+a.Name+" }", a.SrcRange.Start.Line, a.SrcRange.End.Line, hclExprRefs(a.Expr, refKey, nil))
			}
		}
	}
	if a, ok := body.Attributes["inputs"]; ok {
		x.inputs(w, a, childDir, refKey)
	}
	return w.result(root), unresolved, nil
}

// tgSig is a block's header as written: `include "root"`.
func tgSig(b *hclsyntax.Block) string {
	s := b.Type
	for _, l := range b.Labels {
		s += ` "` + l + `"`
	}
	return s
}

// inputs adds one entity per key of an inputs object literal (or of the
// object literals merge() is given), each calling what its value reads, and,
// for a unit whose terraform.source is a local module, each key's reference to
// the module's variable of that name. An inputs value of any other shape is
// one entity, "inputs".
func (x *TerragruntExtractor) inputs(w *hclWalk, a *hclsyntax.Attribute, childDir string, refKey func(hcl.Traversal) string) {
	rest := []hclsyntax.Expression{}
	var objs []*hclsyntax.ObjectConsExpr
	var gather func(e hclsyntax.Expression)
	gather = func(e hclsyntax.Expression) {
		switch n := e.(type) {
		case *hclsyntax.ObjectConsExpr:
			objs = append(objs, n)
		case *hclsyntax.ParenthesesExpr:
			gather(n.Expression)
		case *hclsyntax.FunctionCallExpr:
			if n.Name == "merge" {
				for _, arg := range n.Args {
					gather(arg)
				}
				return
			}
			rest = append(rest, e)
		default:
			rest = append(rest, e)
		}
	}
	gather(a.Expr)
	for _, o := range objs {
		for _, it := range o.Items {
			k, ok := hclKeyName(it.KeyExpr)
			if !ok {
				rest = append(rest, it.ValueExpr)
				continue
			}
			name := w.add("input."+k, kindTGInput, "inputs { "+k+" }", it.KeyExpr.Range().Start.Line, it.ValueExpr.Range().End.Line, hclExprRefs(it.ValueExpr, refKey, nil))
			if childDir != "" {
				w.ref(DocCrossFileRef{
					From: name, Edge: hclEdgeModuleInput, Raw: k, Target: hclAddressLabel + "=var." + k, Strategy: "label-selector",
					Line: it.KeyExpr.Range().Start.Line, Scope: "terraform:" + childDir, Labels: map[string]string{hclAddressLabel: "var." + k},
				})
			}
		}
	}
	if len(rest) > 0 {
		var refs []hclRef
		for _, e := range rest {
			refs = append(refs, hclExprRefs(e, refKey, nil)...)
		}
		w.add("inputs", kindTGInput, "inputs", a.SrcRange.Start.Line, a.SrcRange.End.Line, refs)
	}
}

// tgRefKey is the entity a traversal reads: local.x, dependency.n and
// include.n (a deeper chain is a read of that entity's attribute); anything
// else (feature flags aside) names no entity of the file.
func tgRefKey(t hcl.Traversal) string {
	if len(t) < 2 {
		return ""
	}
	root, ok := t[0].(hcl.TraverseRoot)
	if !ok {
		return ""
	}
	switch root.Name {
	case "local", "dependency", "include", "feature":
		if a, ok := t[1].(hcl.TraverseAttr); ok {
			return root.Name + "." + a.Name
		}
		if ix, ok := t[1].(hcl.TraverseIndex); ok && ix.Key.Type().FriendlyName() == "string" && ix.Key.AsString() != "" {
			return root.Name + "." + ix.Key.AsString()
		}
	}
	return ""
}
