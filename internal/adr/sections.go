package adr

import (
	"regexp"
	"sort"
	"strings"
)

// sections.go — EP-1: the single-file decision log.
//
// Parse handles one record per file (docs/adr/NNNN-slug.md). This repo, and
// any repo whose Project Intent router names a single ADR.md, keeps every
// record as a `## ADR-NNN` SECTION of one append-only file, with a
// `**Status.**` line only where the status is not the file's default and a
// `**Revisit when.**` trailer carrying the condition that would reopen it.
//
// ADR-026's intent corpus needs those trailers — they are the largest
// population of `Revisit when` conditions in the repo — so the parser lives
// here beside the file-per-record one rather than as a second ADR parser in
// internal/intent (a second system for one grammar is how two answers to
// the same question start drifting).
type Section struct {
	ID     string // "ADR-014"
	Title  string // heading text with the id prefix and separators stripped
	Status Status // the record's own **Status.** line, else DefaultStatus
	// Revisit is every `**Revisit when.**` paragraph's text, in file order.
	// A record may carry several (an amendment often adds one).
	Revisit []string
	// Line is the 1-based line of the section heading, for file:line evidence.
	Line int
	// Body is the section's raw text, for callers that scan further.
	Body string
}

var (
	// sectionHeadingRe matches both heading shapes this repo uses:
	//   ## ADR-001 — Three-layer architecture
	//   ## ADR-020: Store backend — pure-Go SQLite (2026-07-22)
	sectionHeadingRe = regexp.MustCompile(`(?i)^##\s+(ADR-\d+)\s*[—:–-]?\s*(.*)$`)
	// boldLabelRe matches a `**Label.**` or `**Label:**` paragraph opener,
	// the shape ADR.md uses for Status, Decision, Why, Revisit when.
	boldLabelRe = regexp.MustCompile(`^\*\*([^*]+?)[.:]?\*\*\s*(.*)$`)
)

// DefaultStatus is what a section means when it carries no `**Status.**`
// line: this repo's ADR.md says "Status: `accepted` unless noted", and that
// convention is what makes a bare section readable. A caller whose file
// says otherwise passes its own default to ParseSectionsWithDefault.
const DefaultStatus = StatusAccepted

// ParseSections splits a single-file decision log into its `## ADR-NNN`
// sections. Sections are returned in file order (never sorted — the file's
// own order is the historical record).
func ParseSections(content []byte) []Section {
	return ParseSectionsWithDefault(content, DefaultStatus)
}

// ParseSectionsWithDefault is ParseSections with the status a section means
// when it declares none.
func ParseSectionsWithDefault(content []byte, def Status) []Section {
	lines := splitLines(content)
	var (
		out []Section
		cur *Section
		buf []string
	)
	flush := func() {
		if cur == nil {
			return
		}
		cur.Body = strings.Join(buf, "\n")
		cur.Status, cur.Revisit = statusAndRevisit(buf, def)
		out = append(out, *cur)
		cur, buf = nil, nil
	}
	for i, ln := range lines {
		if m := sectionHeadingRe.FindStringSubmatch(strings.TrimSpace(ln)); m != nil {
			flush()
			cur = &Section{
				ID:    strings.ToUpper(m[1]),
				Title: strings.TrimSpace(m[2]),
				Line:  i + 1,
			}
			continue
		}
		if cur != nil {
			buf = append(buf, ln)
		}
	}
	flush()
	return out
}

// statusAndRevisit reads a section body's `**Status.**` line and every
// `**Revisit when.**` paragraph. A paragraph runs to the next bold label or
// blank-line-then-bold-label, so a multi-line or bulleted condition list
// survives intact — several records write the condition as a bullet list.
func statusAndRevisit(body []string, def Status) (Status, []string) {
	status := def
	var revisit []string
	for i := 0; i < len(body); i++ {
		m := boldLabelRe.FindStringSubmatch(strings.TrimSpace(body[i]))
		if m == nil {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(m[1]))
		rest := strings.TrimSpace(m[2])
		switch {
		case label == "status":
			if s := statusFromText(rest); s != StatusUnknown {
				status = s
			}
		case strings.HasPrefix(label, "revisit when"):
			para := []string{rest}
			for j := i + 1; j < len(body); j++ {
				t := strings.TrimSpace(body[j])
				if boldLabelRe.MatchString(t) || sectionHeadingRe.MatchString(t) {
					break
				}
				para = append(para, t)
			}
			text := strings.TrimSpace(strings.Join(para, "\n"))
			if text != "" {
				revisit = append(revisit, text)
			}
		}
	}
	return status, revisit
}

// statusFromText reads the first recognised status word in s. Amendment
// prose ("Superseded by ADR-020 …") is matched on its leading word, so a
// record that explains its supersession still reports it.
func statusFromText(s string) Status {
	low := strings.ToLower(s)
	for _, st := range []Status{StatusSuperseded, StatusDeprecated, StatusRejected, StatusProposed, StatusAccepted} {
		if strings.Contains(low, string(st)) {
			return st
		}
	}
	return StatusUnknown
}

// SectionIDs returns the ids ParseSections found, sorted — for callers that
// need a set rather than the file's order.
func SectionIDs(secs []Section) []string {
	ids := make([]string, 0, len(secs))
	for _, s := range secs {
		ids = append(ids, s.ID)
	}
	sort.Strings(ids)
	return ids
}
