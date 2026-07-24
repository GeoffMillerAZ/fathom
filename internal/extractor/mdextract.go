package extractor

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	goldast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// MarkdownExtractor implements Extractor for Markdown (.md) files. It uses
// goldmark to parse headings; because goldmark is code-fence-aware, a "#"
// inside a fenced code block is NOT treated as a heading (unlike the former
// regex approach). One KindDocChunk is emitted per heading. Tier is
// TierDocClaimed (documentation, not code).
type MarkdownExtractor struct{}

// NewMarkdownExtractor returns a new MarkdownExtractor.
func NewMarkdownExtractor() *MarkdownExtractor { return &MarkdownExtractor{} }

// Extract parses Markdown source bytes and returns one SymbolNode per
// heading. Each node's EndLine is the line immediately before the next
// heading (or EOF for the last heading in the file) — a real section
// extent, not a heading-relative guess (T5-5, docs/plans/trust-and-
// tokenomics.md: the T08 class was a downstream tool computing "line 20" by
// arithmetic off an L18 heading when the truth was L29; carrying the real
// end line here lets a caller — e.g. read_source over [Line, EndLine] — pin
// the section's actual line numbers instead).
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

	// nodes is in true document order here — goldast.Walk visits headings as
	// flat, increasing-offset siblings, never nested — so each node's
	// immediate successor in this slice is its actual next heading. Compute
	// EndLine from that adjacency BEFORE the stable sort below, which only
	// exists to order same-line ties and must not change EndLine's meaning.
	totalLines := countLines(src)
	for i := range nodes {
		if i+1 < len(nodes) {
			nodes[i].EndLine = nodes[i+1].Line - 1
		} else {
			nodes[i].EndLine = totalLines
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

// countLines returns the 1-based number of lines in src: one more than the
// count of "\n" separators, unless src ends exactly on a "\n" (no partial
// line follows). Empty src is 0 lines.
func countLines(src []byte) int {
	if len(src) == 0 {
		return 0
	}
	n := bytes.Count(src, []byte{'\n'})
	if src[len(src)-1] != '\n' {
		n++
	}
	return n
}
