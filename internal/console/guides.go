package console

// guides.go — OC-48's projections (design §3 Learn, "Guides"): the
// use-case guides fathom ships (docs/use-cases/*.md, indexed by that
// directory's README table), the glossary (docs/intent/domain.md's
// Vocabulary) and the laws (docs/intent/principles.md, P1–P15 = laws 1–15),
// parsed from the documents themselves — never a second, hand-kept copy of
// either list. Rendering (markdown, the table of contents) is the page's
// job; these functions only read.

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// GuideKind is what a guide page renders.
type GuideKind string

const (
	GuideUseCase  GuideKind = "use-case"
	GuideGlossary GuideKind = "glossary"
	GuideLaws     GuideKind = "laws"
)

// The two guides built from the intent corpus rather than a use-case file.
const (
	GlossarySlug = "glossary"
	LawsSlug     = "laws"
)

// GuideEntry is one guide in the index.
type GuideEntry struct {
	Slug        string    `json:"slug"`
	Kind        GuideKind `json:"kind"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	// Source is the repository path the guide is rendered from.
	Source string `json:"source"`
}

var guideSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ValidGuideSlug reports whether s can name a guide.
func ValidGuideSlug(s string) bool { return guideSlugRE.MatchString(s) }

// guideRowRE matches a README index row: | [name.md](name.md) | text |
var guideRowRE = regexp.MustCompile(`^\|\s*\[([a-z0-9-]+)\.md\]\(([a-z0-9-]+)\.md\)\s*\|\s*(.+?)\s*\|\s*$`)

// ParseGuideIndex reads the use-case README's guide table: one entry per
// row, in the README's order, with the row's description (inline markdown
// emphasis removed).
func ParseGuideIndex(readme []byte) []GuideEntry {
	var out []GuideEntry
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(readme))
	for sc.Scan() {
		m := guideRowRE.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil || m[1] != m[2] || !ValidGuideSlug(m[1]) || seen[m[1]] || m[1] == GlossarySlug || m[1] == LawsSlug {
			continue
		}
		seen[m[1]] = true
		out = append(out, GuideEntry{
			Slug: m[1], Kind: GuideUseCase, Title: m[1],
			Description: PlainInline(m[3]),
			Source:      "docs/use-cases/" + m[1] + ".md",
		})
	}
	return out
}

// GuideTitle is a markdown document's first level-one heading, as plain
// text; "" when it has none.
func GuideTitle(md []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(md))
	inFence := false
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if !inFence && strings.HasPrefix(line, "# ") {
			return PlainInline(strings.TrimSpace(line[2:]))
		}
	}
	return ""
}

var (
	inlineCodeRE = regexp.MustCompile("`([^`]*)`")
	inlineLinkRE = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	inlineEmRE   = regexp.MustCompile(`(\*\*|__|\*|_)([^*_]+)(\*\*|__|\*|_)`)
)

// PlainInline strips inline markdown (links keep their text, code keeps
// its content, emphasis markers go).
func PlainInline(s string) string {
	s = inlineLinkRE.ReplaceAllString(s, "$1")
	s = inlineCodeRE.ReplaceAllString(s, "$1")
	for i := 0; i < 3; i++ {
		s = inlineEmRE.ReplaceAllString(s, "$2")
	}
	return strings.TrimSpace(s)
}

// GlossaryTerm is one vocabulary entry.
type GlossaryTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
	Source     string `json:"source,omitempty"`
}

var termRE = regexp.MustCompile(`^- \*\*(.+?)\*\*\s*[—-]\s*(.*)$`)

// ParseGlossary reads the "## Vocabulary" section of the intent corpus's
// domain document: one term per bullet ("- **term** — definition. Source:
// …"), continuation lines folded in.
func ParseGlossary(domain []byte) []GlossaryTerm {
	var out []GlossaryTerm
	in := false
	var cur *GlossaryTerm
	flush := func() {
		if cur != nil {
			def, src := splitSource(cur.Definition)
			cur.Definition, cur.Source = def, src
			out = append(out, *cur)
			cur = nil
		}
	}
	sc := bufio.NewScanner(bytes.NewReader(domain))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			flush()
			in = strings.TrimSpace(line[3:]) == "Vocabulary"
			continue
		}
		if !in {
			continue
		}
		if m := termRE.FindStringSubmatch(line); m != nil {
			flush()
			cur = &GlossaryTerm{Term: m[1], Definition: m[2]}
			continue
		}
		if cur != nil && strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" {
			cur.Definition += " " + strings.TrimSpace(line)
			continue
		}
		if strings.TrimSpace(line) == "" {
			flush()
		}
	}
	flush()
	return out
}

// Law is one of the laws: the principle that carries it and its text.
type Law struct {
	Number    int    `json:"number"`
	Principle string `json:"principle"` // "P1"
	Title     string `json:"title"`
	Text      string `json:"text"`
	Source    string `json:"source,omitempty"`
}

var (
	principleRE = regexp.MustCompile(`^\*\*(P\d+)\. (.+?)\*\*\s*(.*)$`)
	lawNumberRE = regexp.MustCompile(`\blaw (\d+)\b`)
)

// ParseLaws reads the principles document: every principle whose Source
// names "law N" is law N, in law order.
func ParseLaws(principles []byte) []Law {
	var out []Law
	var cur *Law
	flush := func() {
		if cur == nil {
			return
		}
		text, src := splitSource(cur.Text)
		cur.Text, cur.Source = text, src
		if m := lawNumberRE.FindStringSubmatch(src); m != nil {
			cur.Number, _ = strconv.Atoi(m[1])
			out = append(out, *cur)
		}
		cur = nil
	}
	sc := bufio.NewScanner(bytes.NewReader(principles))
	for sc.Scan() {
		line := sc.Text()
		if m := principleRE.FindStringSubmatch(line); m != nil {
			flush()
			cur = &Law{Principle: m[1], Title: strings.TrimSuffix(PlainInline(m[2]), "."), Text: m[3]}
			continue
		}
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if cur != nil {
			cur.Text += " " + strings.TrimSpace(line)
		}
	}
	flush()
	for i := 1; i < len(out); i++ { // insertion sort by law number: stable, tiny
		for j := i; j > 0 && out[j].Number < out[j-1].Number; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// splitSource separates a trailing "Source: …" citation from its text.
func splitSource(s string) (text, source string) {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "Source: "); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSuffix(strings.TrimSpace(s[i+len("Source: "):]), ".")
	}
	return s, ""
}

// GlossaryMarkdown renders the glossary as a guide document: one section
// per term, each with a given anchor (term-<n>), so the table of contents
// and a link to a term never depend on the term's prose.
func GlossaryMarkdown(terms []GlossaryTerm) []byte {
	var b strings.Builder
	b.WriteString("# Glossary\n\nWhat fathom's own words mean. Each term is read from the project's intent corpus (`docs/intent/domain.md`, Vocabulary), with the source it cites.\n")
	for i, t := range terms {
		fmt.Fprintf(&b, "\n## %s {#term-%d}\n\n%s\n", escapeHeading(PlainInline(t.Term)), i+1, t.Definition)
		if t.Source != "" {
			fmt.Fprintf(&b, "\n*Source: %s.*\n", t.Source)
		}
	}
	return []byte(b.String())
}

// LawsMarkdown renders the laws as a guide document: one section per law
// with the anchor law-<N>, so "law 13" anywhere links to #law-13.
func LawsMarkdown(laws []Law) []byte {
	var b strings.Builder
	b.WriteString("# The laws\n\nFathom's doctrine: fifteen laws adopted across the vision series and carried as principles P1–P15 in `docs/intent/principles.md`. Pages cite them by number (\"law 3: absent is never zero\").\n")
	for _, l := range laws {
		fmt.Fprintf(&b, "\n## Law %d — %s {#law-%d}\n\n%s\n", l.Number, escapeHeading(l.Title), l.Number, l.Text)
		src := l.Principle
		if l.Source != "" {
			src += "; " + l.Source
		}
		fmt.Fprintf(&b, "\n*Principle %s.*\n", src)
	}
	return []byte(b.String())
}

// escapeHeading keeps a heading's text from being read as attribute syntax.
func escapeHeading(s string) string { return strings.ReplaceAll(s, "{", `\{`) }
