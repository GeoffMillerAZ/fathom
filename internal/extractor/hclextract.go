package extractor

import (
	"bufio"
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// HCLExtractor implements Extractor for Terraform HCL (.tf) files using
// line-based heuristics. It detects:
//   - resource "type" "name" → KindTerraformResource, Name = "type.name"
//   - module "name" → KindTerraformResource, Name = "module.name"
//   - data "type" "name" → KindTerraformResource, Name = "data.type.name"
type HCLExtractor struct{}

// NewHCLExtractor returns a new HCLExtractor.
func NewHCLExtractor() *HCLExtractor { return &HCLExtractor{} }

var (
	hclResourceRe = regexp.MustCompile(`^\s*resource\s+"([^"]+)"\s+"([^"]+)"`)
	hclModuleRe   = regexp.MustCompile(`^\s*module\s+"([^"]+)"`)
	hclDataRe     = regexp.MustCompile(`^\s*data\s+"([^"]+)"\s+"([^"]+)"`)
)

// Extract parses Terraform HCL bytes and returns SymbolNodes.
func (h *HCLExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNo := 0
	pkg := pkgFromPath(relPath)

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		stripped := strings.TrimSpace(line)

		if stripped == "" || strings.HasPrefix(stripped, "#") || strings.HasPrefix(stripped, "//") {
			continue
		}

		if m := hclResourceRe.FindStringSubmatch(line); m != nil {
			name := m[1] + "." + m[2]
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindTerraformResource,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierVerified,
			})
			continue
		}

		if m := hclDataRe.FindStringSubmatch(line); m != nil {
			name := "data." + m[1] + "." + m[2]
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindTerraformResource,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierVerified,
			})
			continue
		}

		if m := hclModuleRe.FindStringSubmatch(line); m != nil {
			name := "module." + m[1]
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindTerraformResource,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierVerified,
			})
			continue
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
