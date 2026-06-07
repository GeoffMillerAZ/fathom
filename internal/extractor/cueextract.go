package extractor

import (
	"bufio"
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// CUEExtractor implements Extractor for CUE language files. It detects
// top-level #Definition: patterns → KindCUEDefinition.
type CUEExtractor struct{}

// NewCUEExtractor returns a new CUEExtractor.
func NewCUEExtractor() *CUEExtractor { return &CUEExtractor{} }

var cueDefRe = regexp.MustCompile(`^(#[A-Za-z_][A-Za-z0-9_]*):\s`)

// Extract parses CUE source bytes and returns SymbolNodes for #Definition: entries.
func (c *CUEExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNo := 0
	pkg := pkgFromPath(relPath)

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		stripped := strings.TrimSpace(line)

		if stripped == "" || strings.HasPrefix(stripped, "//") {
			continue
		}

		if m := cueDefRe.FindStringSubmatch(line); m != nil {
			name := m[1]
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindCUEDefinition,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierVerified,
			})
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
