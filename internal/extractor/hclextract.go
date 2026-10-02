package extractor

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// HCLExtractor implements Extractor for Terraform HCL (.tf) files using
// hclsyntax.ParseConfig for a real AST parse. It handles:
//
//   - resource "T" "N" → KindTerraformResource, Name = "T.N"
//   - data "T" "N"     → KindTerraformData,     Name = "data.T.N"
//   - module "N"       → KindTerraformModule,   Name = "module.N"
//   - variable "N"     → KindTerraformVariable, Name = "var.N"
//   - output "N"       → KindTerraformOutput,   Name = "output.N"
//   - provider "N"     → KindTerraformProvider, Name = "provider.N"
//   - locals { k = … } → KindTerraformLocal,   Name = "local.k" (one per attr)
//
// ExtractDoc (LB-F84) also returns the file's cross-file references for the
// doc-xref pass (fusion.ResolveDocXrefs): see ExtractDoc.
type HCLExtractor struct{}

var (
	_ Extractor   = (*HCLExtractor)(nil)
	_ DocResulter = (*HCLExtractor)(nil)
)

// NewHCLExtractor returns a new HCLExtractor.
func NewHCLExtractor() *HCLExtractor { return &HCLExtractor{} }

// Extract parses Terraform HCL bytes and returns SymbolNodes.
// It is best-effort: if parsing produces partial results (f non-nil with
// diagnostics), those results are still processed. Only a nil file pointer
// causes a nil return.
func (h *HCLExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	nodes, _, err := h.extract(service, relPath, src)
	return nodes, err
}

// hclRef is one reference a definition makes, as written: its key
// (referenceFromTraversal's), the output a module reference reads (module.m.o,
// module.m[0].o, module.m[*].o: "o"; "" otherwise) and its line.
type hclRef struct {
	key, output string
	line        int
}

// hclDef is what ExtractDoc needs of one definition beyond its node: its
// references and, for a module call, its source and its input arguments.
type hclDef struct {
	refs   []hclRef
	source string
	inputs []hclInput
}

type hclInput struct {
	name string
	line int
}

// hclModuleMeta are a module block's meta-arguments: every other attribute
// is an input variable of the called module.
var hclModuleMeta = map[string]bool{
	"source": true, "version": true, "count": true, "for_each": true, "depends_on": true, "providers": true,
}

// extract is Extract plus each node's hclDef (defs[i] is nodes[i]'s; a
// node with nothing to add has a nil one).
func (h *HCLExtractor) extract(service, relPath string, src []byte) ([]core.SymbolNode, []*hclDef, error) {
	f, diags := hclsyntax.ParseConfig(src, relPath, hcl.Pos{Line: 1, Column: 1})
	if f == nil {
		// Fatal parse failure — no AST at all.
		if diags.HasErrors() {
			return nil, nil, fmt.Errorf("hcl parse %s: %s", relPath, diags.Error())
		}
		return nil, nil, nil
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok || body == nil {
		return nil, nil, nil
	}

	pkg := pkgFromPath(relPath)

	// .tfvars files hold only top-level key = value assignments (no
	// blocks); each key is one KindTerraformVarValue symbol (queue item
	// terraform's notes: ".tfvars keys").
	if strings.HasSuffix(relPath, ".tfvars") {
		return extractTFVars(service, pkg, relPath, body), nil, nil
	}

	var nodes []core.SymbolNode
	var defs []*hclDef
	// add appends a node and its def, setting the node's Calls from the
	// def's references.
	add := func(n core.SymbolNode, d *hclDef) {
		if d != nil {
			n.Calls = refKeys(d.refs)
		}
		nodes = append(nodes, n)
		defs = append(defs, d)
	}

	for _, b := range body.Blocks {
		switch b.Type {
		case "resource":
			if len(b.Labels) < 2 {
				continue
			}
			rType, rName := b.Labels[0], b.Labels[1]
			add(core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      rType + "." + rName,
				Kind:      core.KindTerraformResource,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("resource %q %q", rType, rName),
				EndLine:   b.Range().End.Line,
				Tier:      core.TierVerified,
			}, &hclDef{refs: collectRefs(b.Body, "provider")})

		case "data":
			if len(b.Labels) < 2 {
				continue
			}
			dType, dName := b.Labels[0], b.Labels[1]
			add(core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "data." + dType + "." + dName,
				Kind:      core.KindTerraformData,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("data %q %q", dType, dName),
				EndLine:   b.Range().End.Line,
				Tier:      core.TierVerified,
			}, &hclDef{refs: collectRefs(b.Body, "provider")})

		case "module":
			if len(b.Labels) < 1 {
				continue
			}
			mName := b.Labels[0]
			sig := fmt.Sprintf("module %q", mName)
			// The module source is required to be a static string literal
			// (the Terraform language spec), so a nil EvalContext always
			// resolves it when present. A local path ("./…", "../…") is
			// recorded as-is (§13 note: "module sources, local path L3");
			// a registry ref ("ns/name/provider") is recorded the same way
			// ("registry refs recorded"). ExtractDoc resolves a local one to
			// the called module's directory (its inputs and outputs, L3); a
			// registry or remote one names nothing in the repository.
			d := &hclDef{refs: collectRefs(b.Body, "providers")}
			if b.Body != nil {
				if attr, ok := b.Body.Attributes["source"]; ok {
					if v, diags := attr.Expr.Value(nil); !diags.HasErrors() && v.Type() == cty.String {
						sig = fmt.Sprintf("module %q { source = %q }", mName, v.AsString())
						d.source = v.AsString()
					}
				}
				for name, attr := range b.Body.Attributes {
					if !hclModuleMeta[name] {
						d.inputs = append(d.inputs, hclInput{name: name, line: attr.SrcRange.Start.Line})
					}
				}
				sort.Slice(d.inputs, func(i, j int) bool { return d.inputs[i].name < d.inputs[j].name })
			}
			add(core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "module." + mName,
				Kind:      core.KindTerraformModule,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: sig,
				EndLine:   b.Range().End.Line,
				Tier:      core.TierVerified,
			}, d)

		case "variable":
			if len(b.Labels) < 1 {
				continue
			}
			vName := b.Labels[0]
			add(core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "var." + vName,
				Kind:      core.KindTerraformVariable,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("variable %q", vName),
				EndLine:   b.Range().End.Line,
				Tier:      core.TierVerified,
			}, nil)

		case "output":
			if len(b.Labels) < 1 {
				continue
			}
			oName := b.Labels[0]
			add(core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "output." + oName,
				Kind:      core.KindTerraformOutput,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("output %q", oName),
				EndLine:   b.Range().End.Line,
				Tier:      core.TierVerified,
			}, &hclDef{refs: collectRefs(b.Body, "")})

		case "provider":
			if len(b.Labels) < 1 {
				continue
			}
			pName := b.Labels[0]
			add(core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "provider." + pName,
				Kind:      core.KindTerraformProvider,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("provider %q", pName),
				EndLine:   b.Range().End.Line,
				Tier:      core.TierVerified,
			}, nil)

		case "locals":
			// locals blocks have no labels; each attribute is one local value.
			if b.Body == nil {
				continue
			}
			for k, attr := range b.Body.Attributes {
				add(core.SymbolNode{
					Service:   service,
					Package:   pkg,
					Name:      "local." + k,
					Kind:      core.KindTerraformLocal,
					FilePath:  relPath,
					Line:      attr.SrcRange.Start.Line,
					Signature: fmt.Sprintf("locals { %s }", k),
					EndLine:   attr.SrcRange.End.Line,
					Tier:      core.TierVerified,
				}, &hclDef{refs: collectRefsFromExpr(attr.Expr)})
			}
		}
	}

	order := make([]int, len(nodes))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := nodes[order[i]], nodes[order[j]]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sortedNodes := make([]core.SymbolNode, len(nodes))
	sortedDefs := make([]*hclDef, len(nodes))
	for i, k := range order {
		sortedNodes[i], sortedDefs[i] = nodes[k], defs[k]
	}
	return sortedNodes, sortedDefs, nil
}

// extractTFVars turns a .tfvars file's top-level attributes into one
// KindTerraformVarValue symbol per key (design queue item terraform's
// notes: ".tfvars keys"). tfvars files carry no blocks, only assignments.
func extractTFVars(service, pkg, relPath string, body *hclsyntax.Body) []core.SymbolNode {
	var nodes []core.SymbolNode
	for k, attr := range body.Attributes {
		nodes = append(nodes, core.SymbolNode{
			Service:   service,
			Package:   pkg,
			Name:      k,
			Kind:      core.KindTerraformVarValue,
			FilePath:  relPath,
			Line:      attr.SrcRange.Start.Line,
			Signature: fmt.Sprintf("%s = …", k),
			EndLine:   attr.SrcRange.End.Line,
			Tier:      core.TierVerified,
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes
}

// hclMetaRoots are traversal roots that reference the evaluation context
// (count.index, each.value, self.*, path.*, terraform.*) rather than
// another definition — they carry no cross-symbol reference edge.
var hclMetaRoots = map[string]bool{
	"each": true, "count": true, "self": true, "path": true, "terraform": true,
}

// collectRefs walks a block's body (including nested blocks, e.g. an
// `ingress { … }` block inside a security-group resource) and returns every
// other-block reference it makes: a resource ("TYPE.NAME"), a data source
// ("data.TYPE.NAME"), a module ("module.NAME") or a var/local ("var.NAME" /
// "local.NAME") — the L2 "resource reference edge" callees the terraform
// queue item's notes ask for (design §1.1: tool formats' L2 is intra-file
// references). An explicit `depends_on = [...]` list is an ordinary
// attribute expression, so its traversals are picked up the same way as an
// implicit reference. skip names a top-level meta-argument that names a
// provider configuration, not a block (`provider` on a resource or data
// source, `providers` on a module call): OpenTofu lists neither as a
// reference (LB-Q-terraform).
func collectRefs(body *hclsyntax.Body, skip string) []hclRef {
	if body == nil {
		return nil
	}
	c := &refCollector{}
	for name, attr := range body.Attributes {
		if name != skip {
			_ = hclsyntax.Walk(attr, c)
		}
	}
	for _, b := range body.Blocks {
		_ = hclsyntax.Walk(b, c)
	}
	return c.refs
}

// collectRefsFromExpr is collectRefs for a single expression (a locals
// block shares one Body across every local value, so each one's refs come
// from its own attribute's expression, not the whole block).
func collectRefsFromExpr(expr hclsyntax.Expression) []hclRef {
	if expr == nil {
		return nil
	}
	c := &refCollector{}
	_ = hclsyntax.Walk(expr, c)
	return c.refs
}

// refKeys is the sorted, de-duplicated set of reference keys: a node's Calls.
func refKeys(refs []hclRef) []string {
	if len(refs) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if !seen[r.key] {
			seen[r.key] = true
			out = append(out, r.key)
		}
	}
	sort.Strings(out)
	return out
}

// refCollector is an hclsyntax.Walker that records every scope-traversal
// expression's reference. A name bound inside the walked expression is no
// reference to a block: a for expression's key and value variables, and a
// dynamic block's iterator (its label, or its `iterator` argument), while
// the walk is inside them.
type refCollector struct {
	refs  []hclRef
	bound []string
}

func (c *refCollector) Enter(n hclsyntax.Node) hcl.Diagnostics {
	switch n := n.(type) {
	case *hclsyntax.ForExpr:
		c.bound = append(c.bound, n.KeyVar, n.ValVar)
	case *hclsyntax.Block:
		if n.Type == "dynamic" {
			c.bound = append(c.bound, dynamicIterator(n))
		}
	case *hclsyntax.SplatExpr:
		// module.m[*].o: the splat's source names the module, its each
		// step the output.
		src, ok1 := n.Source.(*hclsyntax.ScopeTraversalExpr)
		each, ok2 := n.Each.(*hclsyntax.RelativeTraversalExpr)
		if ok1 && ok2 && len(src.Traversal) == 2 && src.Traversal.RootName() == "module" && !c.isBound("module") && len(each.Traversal) > 0 {
			if a, ok := each.Traversal[0].(hcl.TraverseAttr); ok {
				if ref := referenceFromTraversal(src.Traversal); ref != "" {
					c.refs = append(c.refs, hclRef{key: ref, output: a.Name, line: src.SrcRange.Start.Line})
				}
			}
		}
	case *hclsyntax.ScopeTraversalExpr:
		if c.isBound(n.Traversal.RootName()) {
			return nil
		}
		if ref := referenceFromTraversal(n.Traversal); ref != "" {
			c.refs = append(c.refs, hclRef{key: ref, output: moduleOutput(n.Traversal), line: n.SrcRange.Start.Line})
		}
	}
	return nil
}

func (c *refCollector) Exit(n hclsyntax.Node) hcl.Diagnostics {
	switch n := n.(type) {
	case *hclsyntax.ForExpr:
		c.bound = c.bound[:len(c.bound)-2]
	case *hclsyntax.Block:
		if n.Type == "dynamic" {
			c.bound = c.bound[:len(c.bound)-1]
		}
	}
	return nil
}

func (c *refCollector) isBound(name string) bool {
	for _, b := range c.bound {
		if b != "" && b == name {
			return true
		}
	}
	return false
}

// dynamicIterator is the name a dynamic block's content reads its element
// by: the `iterator` argument when set, the block's label otherwise.
func dynamicIterator(b *hclsyntax.Block) string {
	if b.Body != nil {
		if attr, ok := b.Body.Attributes["iterator"]; ok {
			if t, diags := hcl.AbsTraversalForExpr(attr.Expr); !diags.HasErrors() {
				return t.RootName()
			}
		}
	}
	if len(b.Labels) > 0 {
		return b.Labels[0]
	}
	return ""
}

// moduleOutput is the output a module traversal reads (module.m.o,
// module.m[0].o, module.m["k"].o), "" for any other traversal.
func moduleOutput(t hcl.Traversal) string {
	if len(t) < 3 || t.RootName() != "module" {
		return ""
	}
	for _, step := range t[2:] {
		switch s := step.(type) {
		case hcl.TraverseIndex:
			continue
		case hcl.TraverseAttr:
			return s.Name
		default:
			return ""
		}
	}
	return ""
}

// referenceFromTraversal turns a traversal into a reference key. Chains
// past the first one or two segments are dropped: the reference is to the
// BLOCK the traversal starts at (its defining symbol), not to a specific
// attribute of it — matching how L2's callee bare name (§1.4) already
// reduces a qualified reference to its own last segment.
func referenceFromTraversal(t hcl.Traversal) string {
	if len(t) == 0 {
		return ""
	}
	root, ok := t[0].(hcl.TraverseRoot)
	if !ok {
		return ""
	}
	if hclMetaRoots[root.Name] {
		return ""
	}
	switch root.Name {
	case "var", "local", "module":
		if len(t) > 1 {
			if a, ok := t[1].(hcl.TraverseAttr); ok {
				return root.Name + "." + a.Name
			}
		}
		return ""
	case "data":
		if len(t) > 2 {
			a1, ok1 := t[1].(hcl.TraverseAttr)
			a2, ok2 := t[2].(hcl.TraverseAttr)
			if ok1 && ok2 {
				return "data." + a1.Name + "." + a2.Name
			}
		}
		return ""
	default:
		// A bare identifier root followed by an attribute step is a
		// resource reference: "<resource_type>.<resource_name>".
		if len(t) > 1 {
			if a, ok := t[1].(hcl.TraverseAttr); ok {
				return root.Name + "." + a.Name
			}
		}
		return ""
	}
}

// hclAddressLabel is the label a terraform definition carries for the
// doc-xref join: its address as a reference writes it (aws_vpc.this,
// data.aws_ami.x, module.m, var.x, local.x, output.o).
const hclAddressLabel = "terraform.address"

// Edges of the terraform dialect's cross-file references.
const (
	hclEdgeReferences   = "references"
	hclEdgeModuleInput  = "module-input"
	hclEdgeModuleOutput = "module-output"
)

// ExtractDoc implements DocResulter (LB-F84): Extract's nodes, plus what the
// doc-xref pass (fusion.ResolveDocXrefs' label-selector join) resolves across
// files. A module is a directory, so every definition carries one label set,
// {terraform.address: its address}, scoped "terraform:<its directory>".
// References are label-selector references in such a scope:
//
//   - a reference to a definition this file does not declare, in the file's
//     own module directory (a same-file reference is a Call and nothing
//     more: the join would only find the file itself);
//   - a module block with a local source ("./…", "../…"): each input argument
//     `a = …` to var.a in the called directory, and each module.m.o this file
//     reads to output.o there.
//
// A registry or remote module source names nothing in the repository, and
// module.m.o read in a file that does not declare module "m" resolves to the
// call alone (which directory m names is another file's to say).
func (h *HCLExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	nodes, defs, err := h.extract(service, relPath, src)
	if err != nil {
		return DocResult{}, err
	}
	res := DocResult{Nodes: nodes}
	if strings.HasSuffix(relPath, ".tfvars") {
		return res, nil
	}
	dir := path.Dir(relPath)
	scope := "terraform:" + dir
	declared := map[string]bool{}
	childDir := map[string]string{} // module call → its local source's directory
	for i, n := range nodes {
		declared[n.Name] = true
		if d := defs[i]; d != nil && n.Kind == core.KindTerraformModule {
			if c, ok := localModuleDir(dir, d.source); ok {
				childDir[n.Name] = c
			}
		}
	}
	for _, n := range nodes {
		if n.Kind == core.KindTerraformProvider {
			continue
		}
		res.Labels = append(res.Labels, DocLabelSet{Entity: n.Name, Scope: scope, Labels: map[string]string{hclAddressLabel: n.Name}, Line: n.Line})
	}
	for i, n := range nodes {
		d := defs[i]
		if d == nil {
			continue
		}
		first := map[[2]string]int{} // (edge, raw) → first line, for one reference per target
		refs := map[[2]string]DocCrossFileRef{}
		ref := func(edge, raw, scope, address string, line int) {
			k := [2]string{edge, scope + "\x00" + address}
			if l, seen := first[k]; seen && l <= line {
				return
			}
			first[k] = line
			refs[k] = DocCrossFileRef{
				From: n.Name, Edge: edge, Raw: raw, Target: hclAddressLabel + "=" + address,
				Strategy: "label-selector", Line: line, Scope: scope, Labels: map[string]string{hclAddressLabel: address},
			}
		}
		for _, r := range d.refs {
			if !declared[r.key] {
				ref(hclEdgeReferences, r.key, scope, r.key, r.line)
			}
			if c, ok := childDir[r.key]; ok && r.output != "" {
				ref(hclEdgeModuleOutput, r.key+"."+r.output, "terraform:"+c, "output."+r.output, r.line)
			}
		}
		if c, ok := childDir[n.Name]; ok {
			for _, in := range d.inputs {
				ref(hclEdgeModuleInput, in.name, "terraform:"+c, "var."+in.name, in.line)
			}
		}
		for _, c := range refs {
			res.CrossFile = append(res.CrossFile, c)
		}
	}
	sort.Slice(res.CrossFile, func(i, j int) bool {
		a, b := res.CrossFile[i], res.CrossFile[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Edge != b.Edge {
			return a.Edge < b.Edge
		}
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		return a.Target < b.Target
	})
	return res, nil
}

// localModuleDir is the repository directory a module source names when it
// is a local path (Terraform's rule: it begins with ./ or ../), relative to
// the calling module's directory; ok is false for a registry or remote
// source, and for a path that leaves the repository or names the caller.
func localModuleDir(dir, source string) (string, bool) {
	if !strings.HasPrefix(source, "./") && !strings.HasPrefix(source, "../") {
		return "", false
	}
	c := path.Join(dir, source)
	if c == ".." || strings.HasPrefix(c, "../") || c == dir {
		return "", false
	}
	return c, true
}
