// Package semantic implements the semantic-layer compiler, sweep, and
// serving logic (ADR-016): governing docs are split into sections, compiled
// by an injected LLM into governed claims with verified anchors, kept honest
// by a post-ingest citation sweep, and served in budgeted payloads at
// decision moments. Reasoning at compile time, determinism at query time.
package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
)

// Section is one governing-doc section — the unit of compilation and of
// incremental digest gating. Line numbers are 1-based and inclusive, in
// document coordinates (citations produced from a section are document
// citations, so the sweep can verify them against the raw file).
type Section struct {
	Path      string // repo-relative doc path
	Subject   string // stable open ref: "<path>#<heading-slug>" (ADR-016 claim subject)
	StartLine int
	EndLine   int
	Text      string
	Digest    string // sha256 of Text — the incremental compile gate
}

var atxHeading = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)

// SplitMarkdownSections splits a markdown document into sections at ATX
// headings. Content before the first heading becomes the "preamble" section.
// Deterministic: same bytes, same sections. Empty/whitespace-only sections
// are dropped (nothing to compile from them).
func SplitMarkdownSections(path string, content []byte) []Section {
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")

	type mark struct {
		line    int // 1-based heading line
		heading string
	}
	marks := []mark{{line: 1, heading: "preamble"}}
	inFence := false
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := atxHeading.FindStringSubmatch(l); m != nil {
			marks = append(marks, mark{line: i + 1, heading: m[1]})
		}
	}

	var out []Section
	seen := map[string]int{} // slug -> occurrences, for stable dedupe
	for i, mk := range marks {
		start := mk.line
		end := len(lines)
		if i+1 < len(marks) {
			end = marks[i+1].line - 1
		}
		if end < start {
			continue
		}
		text := strings.Join(lines[start-1:end], "\n")
		if strings.TrimSpace(text) == "" {
			continue
		}
		slug := slugify(mk.heading)
		if n := seen[slug]; n > 0 {
			slug = slug + "-" + itoa(n+1)
		}
		seen[slugify(mk.heading)]++
		sum := sha256.Sum256([]byte(text))
		out = append(out, Section{
			Path:      path,
			Subject:   path + "#" + slug,
			StartLine: start,
			EndLine:   end,
			Text:      text,
			Digest:    hex.EncodeToString(sum[:]),
		})
	}
	return out
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlug.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func itoa(n int) string { return strconv.Itoa(n) }
