package extractor

import (
	"bufio"
	"bytes"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// MarkdownExtractor implements Extractor for Markdown (.md) files. It
// chunks the document by ATX headings (# / ## / ###) and emits one
// KindDocChunk per heading. The chunk name is the heading text.
// Tier is TierDocClaimed (documentation, not code).
type MarkdownExtractor struct{}

// NewMarkdownExtractor returns a new MarkdownExtractor.
func NewMarkdownExtractor() *MarkdownExtractor { return &MarkdownExtractor{} }

// Extract parses Markdown source bytes and returns one SymbolNode per heading.
func (m *MarkdownExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNo := 0
	pkg := pkgFromPath(relPath)

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()

		if !strings.HasPrefix(line, "#") {
			continue
		}

		// ATX heading: strip leading '#' characters and space.
		heading := strings.TrimLeft(line, "#")
		heading = strings.TrimSpace(heading)
		if heading == "" {
			continue
		}

		nodes = append(nodes, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     heading,
			Kind:     core.KindDocChunk,
			FilePath: relPath,
			Line:     lineNo,
			Tier:     core.TierDocClaimed,
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}
