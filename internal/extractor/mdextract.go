package extractor

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	goldast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// MarkdownExtractor implements Extractor and DocResulter for Markdown (.md,
// .mdx) files (LB-Q-markdown, the native markdown dialect). It uses
// goldmark to parse headings; because goldmark is code-fence-aware, a "#"
// inside a fenced code block is NOT treated as a heading (unlike the former
// regex approach). One KindDocChunk is emitted per heading. Tier is
// TierDocClaimed (documentation, not code).
type MarkdownExtractor struct{}

var (
	_ Extractor   = (*MarkdownExtractor)(nil)
	_ DocResulter = (*MarkdownExtractor)(nil)
)

const (
	// MarkdownAnchorStrategy resolves a link's fragment to the heading of the
	// target file whose slug it is (fusion's doc-xref pass, docxref.go).
	MarkdownAnchorStrategy = "md-anchor"
	// mdLinkEdge is a link to another Markdown file (a file-level reference,
	// strategy repo-path); mdAnchorEdge the same link's fragment, resolved to
	// a heading.
	mdLinkEdge   = "link"
	mdAnchorEdge = "link_anchor"
)

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
	r, err := m.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// ExtractDoc implements DocResulter: the headings Extract returns, the first
// heading as the file's root entity, and a cross-file reference for every
// relative link to another Markdown file (the section holding the link is the
// origin) and for its fragment.
func (m *MarkdownExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	pkg := pkgFromPath(relPath)
	var nodes []core.SymbolNode

	md := goldmark.New()
	src = blankFrontMatter(src)
	doc := md.Parser().Parse(text.NewReader(src))

	var links []mdLink
	goldast.Walk(doc, func(n goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *goldast.Link:
			links = append(links, mdLink{dest: string(v.Destination), line: mdNodeLine(src, v)})
			return goldast.WalkContinue, nil
		case *goldast.HTMLBlock:
			for i := 0; i < v.Lines().Len(); i++ {
				seg := v.Lines().At(i)
				links = mdHTMLLinks(links, src, seg.Start, seg.Value(src))
			}
			return goldast.WalkContinue, nil
		case *goldast.RawHTML:
			for i := 0; i < v.Segments.Len(); i++ {
				seg := v.Segments.At(i)
				links = mdHTMLLinks(links, src, seg.Start, seg.Value(src))
			}
			return goldast.WalkContinue, nil
		}
		h, ok := n.(*goldast.Heading)
		if !ok {
			return goldast.WalkContinue, nil
		}

		// The heading's text as written (inline code, emphasis and link
		// markup kept, a setext heading's lines joined), the way ctags names
		// it: a Text-only walk dropped a code span and everything in it.
		var textParts []string
		for i := 0; i < h.Lines().Len(); i++ {
			seg := h.Lines().At(i)
			if t := strings.TrimSpace(string(seg.Value(src))); t != "" {
				textParts = append(textParts, t)
			}
		}
		headingText := strings.Join(textParts, " ")
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
	mdSectionCalls(nodes, links)
	res := DocResult{Nodes: nodes}
	if len(nodes) > 0 {
		res.Roots = []string{nodes[0].Name}
	}
	res.CrossFile = mdCrossFile(relPath, nodes, links)
	return res, nil
}

// mdLink is one inline or reference link, with the line its text starts on.
type mdLink struct {
	dest string
	line int
}

var mdFrontKey = regexp.MustCompile(`^[A-Za-z0-9_.-]+\s*[:=]`)

// blankFrontMatter returns src with a leading YAML (---) or TOML (+++) front
// matter block replaced by spaces, newlines kept so line numbers hold: the
// block is metadata, and its closing --- would otherwise read as a setext
// underline making a heading of its last line. A block counts when it closes
// within 200 lines and holds a key line.
func blankFrontMatter(src []byte) []byte {
	var fence string
	switch {
	case bytes.HasPrefix(src, []byte("---")):
		fence = "---"
	case bytes.HasPrefix(src, []byte("+++")):
		fence = "+++"
	default:
		return src
	}
	lines := bytes.SplitAfter(src, []byte{'\n'})
	if strings.TrimSpace(string(lines[0])) != fence {
		return src
	}
	hasKey := false
	for i := 1; i < len(lines) && i <= 200; i++ {
		t := strings.TrimSpace(string(lines[i]))
		if t == fence || (fence == "---" && t == "...") {
			if !hasKey {
				return src
			}
			out := append([]byte(nil), src...)
			n := 0
			for j := 0; j <= i; j++ {
				for k := 0; k < len(lines[j]); k++ {
					if out[n+k] != '\n' && out[n+k] != '\r' {
						out[n+k] = ' '
					}
				}
				n += len(lines[j])
			}
			return out
		}
		if mdFrontKey.MatchString(t) {
			hasKey = true
		}
	}
	return src
}

// mdSectionCalls sets each heading's Calls to the other headings of the file
// that links within the heading's section name by fragment, sorted.
func mdSectionCalls(heads []core.SymbolNode, links []mdLink) {
	if len(heads) == 0 {
		return
	}
	slugs := MarkdownHeadingSlugs(heads)
	calls := make([]map[string]bool, len(heads))
	for _, l := range links {
		frag, ok := mdAnchorOnly(l.dest)
		if !ok {
			continue
		}
		to, ok := slugs[frag]
		if !ok {
			continue
		}
		owner := 0
		for i, h := range heads {
			if h.Line > l.line {
				break
			}
			owner = i
		}
		if heads[owner].Name == to {
			continue
		}
		if calls[owner] == nil {
			calls[owner] = map[string]bool{}
		}
		calls[owner][to] = true
	}
	for i, c := range calls {
		for name := range c {
			heads[i].Calls = append(heads[i].Calls, name)
		}
		sort.Strings(heads[i].Calls)
	}
}

// mdAnchorOnly is the lower-cased, unescaped fragment of a destination that
// is only a fragment ("#install-steps").
func mdAnchorOnly(dest string) (string, bool) {
	dest = strings.TrimSpace(dest)
	if !strings.HasPrefix(dest, "#") || len(dest) == 1 {
		return "", false
	}
	frag := dest[1:]
	if u, err := url.PathUnescape(frag); err == nil {
		frag = u
	}
	return strings.ToLower(frag), true
}

// mdNodeLine is the 1-based line an inline node starts on: the first text
// segment under it, else the first line of the block holding it.
func mdNodeLine(src []byte, n goldast.Node) int {
	var start = -1
	_ = goldast.Walk(n, func(c goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if t, ok := c.(*goldast.Text); ok && entering {
			start = t.Segment.Start
			return goldast.WalkStop, nil
		}
		return goldast.WalkContinue, nil
	})
	if start < 0 {
		for p := n.Parent(); p != nil && start < 0; p = p.Parent() {
			if p.Type() == goldast.TypeBlock && p.Lines() != nil && p.Lines().Len() > 0 {
				start = p.Lines().At(0).Start
			}
		}
	}
	if start < 0 {
		return 0
	}
	return 1 + bytes.Count(src[:start], []byte{'\n'})
}

var mdHTMLHref = regexp.MustCompile(`(?i)<a\s[^>]*?\bhref\s*=\s*(?:"([^"]*)"|'([^']*)')`)

// mdHTMLLinks appends a link for each <a href> in one raw HTML segment that
// starts at byte offset start of src: READMEs put their badges and logos in
// anchors the Markdown link syntax never sees.
func mdHTMLLinks(links []mdLink, src []byte, start int, seg []byte) []mdLink {
	for _, m := range mdHTMLHref.FindAllSubmatchIndex(seg, -1) {
		var dest string
		if m[2] >= 0 {
			dest = string(seg[m[2]:m[3]])
		} else {
			dest = string(seg[m[4]:m[5]])
		}
		links = append(links, mdLink{dest: html.UnescapeString(dest), line: 1 + bytes.Count(src[:start+m[0]], []byte{'\n'})})
	}
	return links
}

var mdScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// markdownLinkTarget is the repository-relative Markdown file a link
// destination names (relative to relPath's directory, or to the repository
// root when it starts with "/") and its fragment, unescaped and lower-cased
// (a heading slug). ok is false for a destination with a scheme, one leaving
// the repository, an empty one, an anchor within the file itself and any that
// is not a .md or .mdx file.
func markdownLinkTarget(relPath, dest string) (target, frag string, ok bool) {
	dest = strings.TrimSpace(dest)
	if dest == "" || strings.HasPrefix(dest, "//") || mdScheme.MatchString(dest) {
		return "", "", false
	}
	p := dest
	if i := strings.IndexByte(p, '#'); i >= 0 {
		frag, p = p[i+1:], p[:i]
	}
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	if u, err := url.PathUnescape(p); err == nil {
		p = u
	}
	if u, err := url.PathUnescape(frag); err == nil {
		frag = u
	}
	frag = strings.ToLower(frag)
	switch {
	case p == "":
		return "", "", false // an anchor within the file: no cross-file reference
	case strings.HasPrefix(p, "/"):
		p = path.Clean(strings.TrimPrefix(p, "/"))
	default:
		p = path.Join(path.Dir(relPath), p)
	}
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", "", false
	}
	if ext := strings.ToLower(path.Ext(p)); ext != ".md" && ext != ".mdx" {
		return "", "", false
	}
	return p, frag, true
}

// mdCrossFile is the references of one file's links, sorted by line, edge and
// raw destination. A link before the first heading originates at the file's
// root ("").
func mdCrossFile(relPath string, heads []core.SymbolNode, links []mdLink) []DocCrossFileRef {
	var out []DocCrossFileRef
	for _, l := range links {
		target, frag, ok := markdownLinkTarget(relPath, l.dest)
		if !ok {
			continue
		}
		from := ""
		for _, h := range heads {
			if h.Line > l.line {
				break
			}
			from = h.Name
		}
		out = append(out, DocCrossFileRef{From: from, Edge: mdLinkEdge, Raw: l.dest, Target: target, Strategy: "repo-path", Line: l.line})
		if frag != "" {
			out = append(out, DocCrossFileRef{From: from, Edge: mdAnchorEdge, Raw: frag, Target: target, Strategy: MarkdownAnchorStrategy, Line: l.line})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Edge != b.Edge {
			return a.Edge < b.Edge
		}
		return a.Raw < b.Raw
	})
	return out
}

var (
	mdLinkText     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdInlineMarkup = regexp.MustCompile(`<[^>]*>`)
)

// mdExplicitID is a heading's own id: {#id} (Docusaurus, pandoc) or the MDX
// comment form {/* #id */}.
var mdExplicitID = regexp.MustCompile(`\s*\{(?:#([^}\s]+)|/\*\s*#([^\s*]+)\s*\*/)\}\s*$`)

// MarkdownHeadingSlugs maps each heading's anchor to its name: the GitHub
// slug (lower-case, punctuation dropped, spaces to hyphens, a repeat suffixed
// -1, -2), or the explicit {#id} Docusaurus and MDX allow. nodes are one
// file's headings in document order; the first heading of a name wins a slug.
func MarkdownHeadingSlugs(nodes []core.SymbolNode) map[string]string {
	out := map[string]string{}
	seen := map[string]int{}
	for _, n := range nodes {
		if n.Kind != core.KindDocChunk {
			continue
		}
		if m := mdExplicitID.FindStringSubmatch(n.Name); m != nil {
			id := strings.ToLower(m[1] + m[2])
			if _, dup := out[id]; !dup {
				out[id] = n.Name
			}
			continue
		}
		var b strings.Builder
		for _, r := range strings.ToLower(mdInlineMarkup.ReplaceAllString(mdLinkText.ReplaceAllString(n.Name, "$1"), "")) {
			switch {
			case r == ' ' || r == '-':
				b.WriteByte('-')
			case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
				b.WriteRune(r)
			}
		}
		slug := b.String()
		k := seen[slug]
		seen[slug]++
		if k > 0 {
			slug = fmt.Sprintf("%s-%d", slug, k)
		}
		if _, dup := out[slug]; !dup {
			out[slug] = n.Name
		}
	}
	return out
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
