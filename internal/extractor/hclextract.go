package extractor

import (
	"fmt"
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
type HCLExtractor struct{}

// NewHCLExtractor returns a new HCLExtractor.
func NewHCLExtractor() *HCLExtractor { return &HCLExtractor{} }

// Extract parses Terraform HCL bytes and returns SymbolNodes.
// It is best-effort: if parsing produces partial results (f non-nil with
// diagnostics), those results are still processed. Only a nil file pointer
// causes a nil return.
func (h *HCLExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	f, diags := hclsyntax.ParseConfig(src, relPath, hcl.Pos{Line: 1, Column: 1})
	if f == nil {
		// Fatal parse failure — no AST at all.
		if diags.HasErrors() {
			return nil, fmt.Errorf("hcl parse %s: %s", relPath, diags.Error())
		}
		return nil, nil
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok || body == nil {
		return nil, nil
	}

	pkg := pkgFromPath(relPath)

	// .tfvars files hold only top-level key = value assignments (no
	// blocks); each key is one KindTerraformVarValue symbol (queue item
	// terraform's notes: ".tfvars keys").
	if strings.HasSuffix(relPath, ".tfvars") {
		return extractTFVars(service, pkg, relPath, body), nil
	}

	var nodes []core.SymbolNode

	for _, b := range body.Blocks {
		switch b.Type {
		case "resource":
			if len(b.Labels) < 2 {
				continue
			}
			rType, rName := b.Labels[0], b.Labels[1]
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      rType + "." + rName,
				Kind:      core.KindTerraformResource,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("resource %q %q", rType, rName),
				EndLine:   b.Range().End.Line,
				Calls:     collectRefs(b.Body),
				Tier:      core.TierVerified,
			})

		case "data":
			if len(b.Labels) < 2 {
				continue
			}
			dType, dName := b.Labels[0], b.Labels[1]
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "data." + dType + "." + dName,
				Kind:      core.KindTerraformData,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("data %q %q", dType, dName),
				EndLine:   b.Range().End.Line,
				Calls:     collectRefs(b.Body),
				Tier:      core.TierVerified,
			})

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
			// ("registry refs recorded") — neither is resolved to a file,
			// since L3 file-edge scoring is not wired for native-engine
			// tool formats yet (no ResolveFileEdges strategy applies).
			if b.Body != nil {
				if attr, ok := b.Body.Attributes["source"]; ok {
					if v, diags := attr.Expr.Value(nil); !diags.HasErrors() && v.Type() == cty.String {
						sig = fmt.Sprintf("module %q { source = %q }", mName, v.AsString())
					}
				}
			}
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "module." + mName,
				Kind:      core.KindTerraformModule,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: sig,
				EndLine:   b.Range().End.Line,
				Calls:     collectRefs(b.Body),
				Tier:      core.TierVerified,
			})

		case "variable":
			if len(b.Labels) < 1 {
				continue
			}
			vName := b.Labels[0]
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "var." + vName,
				Kind:      core.KindTerraformVariable,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("variable %q", vName),
				EndLine:   b.Range().End.Line,
				Tier:      core.TierVerified,
			})

		case "output":
			if len(b.Labels) < 1 {
				continue
			}
			oName := b.Labels[0]
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "output." + oName,
				Kind:      core.KindTerraformOutput,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("output %q", oName),
				EndLine:   b.Range().End.Line,
				Calls:     collectRefs(b.Body),
				Tier:      core.TierVerified,
			})

		case "provider":
			if len(b.Labels) < 1 {
				continue
			}
			pName := b.Labels[0]
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "provider." + pName,
				Kind:      core.KindTerraformProvider,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("provider %q", pName),
				EndLine:   b.Range().End.Line,
				Tier:      core.TierVerified,
			})

		case "locals":
			// locals blocks have no labels; each attribute is one local value.
			if b.Body == nil {
				continue
			}
			for k, attr := range b.Body.Attributes {
				nodes = append(nodes, core.SymbolNode{
					Service:   service,
					Package:   pkg,
					Name:      "local." + k,
					Kind:      core.KindTerraformLocal,
					FilePath:  relPath,
					Line:      attr.SrcRange.Start.Line,
					Signature: fmt.Sprintf("locals { %s }", k),
					EndLine:   attr.SrcRange.End.Line,
					Calls:     collectRefsFromExpr(attr.Expr),
					Tier:      core.TierVerified,
				})
			}
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
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
// `ingress { … }` block inside a security-group resource) and returns the
// sorted, de-duplicated set of other-block references it makes: a resource
// ("TYPE.NAME"), a data source ("data.TYPE.NAME"), a module ("module.NAME")
// or a var/local ("var.NAME" / "local.NAME") — the L2 "resource reference
// edge" callees the terraform queue item's notes ask for (design §1.1: tool
// formats' L2 is intra-file references). An explicit `depends_on = [...]`
// list is an ordinary attribute expression, so its traversals are picked up
// the same way as an implicit reference.
func collectRefs(body *hclsyntax.Body) []string {
	if body == nil {
		return nil
	}
	refs := map[string]bool{}
	_ = hclsyntax.Walk(body, refCollector{refs: refs})
	return sortedRefs(refs)
}

// collectRefsFromExpr is collectRefs for a single expression (a locals
// block shares one Body across every local value, so each one's refs come
// from its own attribute's expression, not the whole block).
func collectRefsFromExpr(expr hclsyntax.Expression) []string {
	if expr == nil {
		return nil
	}
	refs := map[string]bool{}
	_ = hclsyntax.Walk(expr, refCollector{refs: refs})
	return sortedRefs(refs)
}

func sortedRefs(refs map[string]bool) []string {
	if len(refs) == 0 {
		return nil
	}
	out := make([]string, 0, len(refs))
	for r := range refs {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// refCollector is an hclsyntax.Walker that records every scope-traversal
// expression's reference key.
type refCollector struct{ refs map[string]bool }

func (c refCollector) Enter(n hclsyntax.Node) hcl.Diagnostics {
	if t, ok := n.(*hclsyntax.ScopeTraversalExpr); ok {
		if ref := referenceFromTraversal(t.Traversal); ref != "" {
			c.refs[ref] = true
		}
	}
	return nil
}

func (c refCollector) Exit(hclsyntax.Node) hcl.Diagnostics { return nil }

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
