package extractor

import (
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
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
				Tier:      core.TierVerified,
			})

		case "module":
			if len(b.Labels) < 1 {
				continue
			}
			mName := b.Labels[0]
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      "module." + mName,
				Kind:      core.KindTerraformModule,
				FilePath:  relPath,
				Line:      b.TypeRange.Start.Line,
				Signature: fmt.Sprintf("module %q", mName),
				EndLine:   b.Range().End.Line,
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
