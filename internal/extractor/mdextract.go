package extractor

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	goldast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// MarkdownExtractor implements Extractor for Markdown (.md) files. It uses
// goldmark to parse headings; because goldmark is code-fence-aware, a "#"
// inside a fenced code block is NOT treated as a heading (unlike the former
// regex approach). One KindDocChunk is emitted per heading. Tier is
// TierDocClaimed (documentation, not code).
type MarkdownExtractor struct{}

// NewMarkdownExtractor returns a new MarkdownExtractor.
func NewMarkdownExtractor() *MarkdownExtractor { return &MarkdownExtractor{} }

// Extract parses Markdown source bytes and returns one SymbolNode per heading.
func (m *MarkdownExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)
	var nodes []core.SymbolNode

	md := goldmark.New()
	reader := text.NewReader(src)
	doc := md.Parser().Parse(reader)

	goldast.Walk(doc, func(n goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}
		h, ok := n.(*goldast.Heading)
		if !ok {
			return goldast.WalkContinue, nil
		}

		// Build heading text by walking child Text nodes.
		var textParts []string
		for child := h.FirstChild(); child != nil; child = child.NextSibling() {
			if t, ok := child.(*goldast.Text); ok {
				seg := t.Segment
				textParts = append(textParts, string(seg.Value(src)))
			}
		}
		headingText := strings.TrimSpace(strings.Join(textParts, ""))
		if headingText == "" {
			return goldast.WalkContinue, nil
		}

		// Compute 1-based line number from the byte offset of the first line
		// segment. h.Lines() holds the raw source lines for the heading.
		lineNo := 0
		if h.Lines().Len() > 0 {
			seg := h.Lines().At(0)
			lineNo = 1 + bytes.Count(src[:seg.Start], []byte{'\n'})
		}

		nodes = append(nodes, core.SymbolNode{
			Service:   service,
			Package:   pkg,
			Name:      headingText,
			Kind:      core.KindDocChunk,
			FilePath:  relPath,
			Line:      lineNo,
			Signature: fmt.Sprintf("h%d", h.Level),
			Tier:      core.TierDocClaimed,
		})
		return goldast.WalkContinue, nil
	}) //nolint:errcheck — Walk only returns errors from the walker func, which we never do

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}
