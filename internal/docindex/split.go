// Package docindex builds the knowledge-retrieval corpus
// (docs/design/knowledge-retrieval.md, docs/plans/knowledge-retrieval.md
// wave 2): splitting Markdown documents into sections (SplitDocUnits, KR-2),
// enumerating the corpus (KR-3), hash-gated ingest into internal/store's
// docs_fts/doc_units (KR-8), and BM25-plus-concern ranking (KR-4).
package docindex

import (
	"regexp"
	"strings"
)

// DocKind hints which of §3's per-source splitting rules SplitDocUnits
// applies. It is a property of the SOURCE (which corpus row it came from,
// KR-3), not something the splitter infers from content alone — an ADR-
// shaped bold-label paragraph in a random Markdown file is not special-cased
// unless the caller says this file is the decisions doc.
type DocKind string

const (
	// DocKindADR is ADR.md: `## ADR-NNN` sections, each split further at
	// bold paragraph labels (Status, Decision, Why, Consequences, Revisit
	// when, ...).
	DocKindADR DocKind = "adr"
	// DocKindPlan is a docs/plans/*.md file: leaf heading sections as in
	// DocKindOther, PLUS every checklist row carrying an `{id: ...}`
	// annotation as its own unit, pulled out of the surrounding section's
	// body so it is never indexed twice.
	DocKindPlan DocKind = "plan"
	// DocKindOther is every other Markdown source §3 lists (intent, design,
	// vision, general docs): leaf heading sections only — a heading becomes
	// a unit exactly when it owns body text directly beneath it, before its
	// first child heading (or the next heading at any level, if it has no
	// children); a purely structural heading with no text of its own is
	// never emitted as an (empty) unit, only kept on the path stack for its
	// descendants.
	DocKindOther DocKind = "other"
)

// Unit kinds recorded on doc_units.kind (design §3/§4's "kind per unit").
const (
	KindADRSection = "adr-section"
	KindPlanRow    = "plan-row"
	KindSection    = "section"
)

// Unit is one section SplitDocUnits produces: a heading path, its 1-based
// inclusive line range in the source, a kind tag, and its body text
// (heading line and, for a plan section, its own row lines excluded) — the
// caller (KR-8's ingest) hashes Body and IDs the unit before handing it to
// store.PutDocUnits.
type Unit struct {
	HeadingPath string
	LineStart   int
	LineEnd     int
	Kind        string
	Body        string
	// RowID is set only for a KindPlanRow unit: the row's `{id: ...}` value,
	// which the ingest layer uses to build a stable UnitID (§3: "plan rows
	// as their own units").
	RowID string
}

var (
	headingRE  = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*$`)
	adrHeadRE  = regexp.MustCompile(`^ADR-\d+`)
	boldLabRE  = regexp.MustCompile(`^\*\*([^*]{1,80}?)\.\*\*\s*(.*)$`)
	planRowRE  = regexp.MustCompile(`^\s*-\s*\[[ xX-]\]\s`)
	rowIDRE    = regexp.MustCompile(`\{[^}]*\bid:\s*([A-Za-z0-9_.-]+)`)
	trailingWS = regexp.MustCompile(`[ \t]+\n`)
)

type headingLine struct {
	level int
	text  string
	line  int // 1-based
}

// SplitDocUnits splits content (one Markdown document, path used only for
// RowID-free error messages/debugging — it never affects the split) into
// Units per kind's rule in §3. Line numbers are 1-based and inclusive of
// both endpoints; a Unit with no non-blank Body is never emitted.
func SplitDocUnits(kind DocKind, content string) []Unit {
	content = trailingWS.ReplaceAllString(content, "\n")
	rawLines := strings.Split(content, "\n")
	// strings.Split on a trailing "\n" yields a final empty element; keep it
	// out of the 1-based line numbering so LineEnd never points past EOF.
	if len(rawLines) > 0 && rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}

	var heads []headingLine
	for i, l := range rawLines {
		if m := headingRE.FindStringSubmatch(l); m != nil {
			heads = append(heads, headingLine{level: len(m[1]), text: m[2], line: i + 1})
		}
	}

	switch kind {
	case DocKindADR:
		return splitADR(rawLines, heads)
	case DocKindPlan:
		return splitPlan(rawLines, heads)
	default:
		return splitLeafSectionsExcluding(rawLines, heads, nil)
	}
}

// headingPathStack walks heads in order, returning, for each heading index,
// the joined " > " path of itself and every still-open ancestor (a heading
// closes when a later heading at its level or shallower appears).
func headingPathStack(heads []headingLine) []string {
	paths := make([]string, len(heads))
	var stack []headingLine
	for i, h := range heads {
		for len(stack) > 0 && stack[len(stack)-1].level >= h.level {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, h)
		parts := make([]string, len(stack))
		for j, s := range stack {
			parts[j] = s.text
		}
		paths[i] = strings.Join(parts, " > ")
	}
	return paths
}

// splitLeafSectionsExcluding is DocKindOther's rule: one Unit per heading
// that owns body text directly beneath it (before its first child heading,
// or the next heading at any level if it has none). exclude, when non-nil,
// marks 0-based line indices to drop from every body — DocKindPlan uses
// this to pull row lines out before this same leaf logic runs over what
// remains.
func splitLeafSectionsExcluding(rawLines []string, heads []headingLine, exclude map[int]bool) []Unit {
	paths := headingPathStack(heads)
	var units []Unit
	// A virtual heading at line 0 covers any preamble before the first real
	// heading, so top-of-file body text is not silently dropped.
	type span struct {
		path      string
		start     int // 1-based, first body line
		headLine  int // 1-based heading line, 0 for the preamble
		endBefore int // 1-based, exclusive: first line NOT in this span
	}
	var spans []span
	// Build spans: preamble (lines 1..heads[0].line-1), then each heading's
	// body up to the next heading's line (exclusive).
	cursor := 1
	if len(heads) == 0 || heads[0].line > 1 {
		end := len(rawLines) + 1
		if len(heads) > 0 {
			end = heads[0].line
		}
		spans = append(spans, span{path: "", start: cursor, headLine: 0, endBefore: end})
	}
	for i, h := range heads {
		end := len(rawLines) + 1
		if i+1 < len(heads) {
			end = heads[i+1].line
		}
		spans = append(spans, span{path: paths[i], start: h.line + 1, headLine: h.line, endBefore: end})
	}

	for _, sp := range spans {
		var bodyLines []string
		for ln := sp.start; ln < sp.endBefore; ln++ {
			if exclude[ln-1] {
				continue
			}
			bodyLines = append(bodyLines, rawLines[ln-1])
		}
		body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
		if body == "" {
			continue
		}
		lineStart := sp.start
		if sp.headLine > 0 {
			lineStart = sp.headLine
		}
		lineEnd := sp.endBefore - 1
		if lineEnd < lineStart {
			lineEnd = lineStart
		}
		units = append(units, Unit{
			HeadingPath: sp.path,
			LineStart:   lineStart,
			LineEnd:     lineEnd,
			Kind:        KindSection,
			Body:        body,
		})
	}
	return units
}

// splitADR applies DocKindADR's rule: within each `## ADR-NNN ...` section's
// span, split further at bold-label paragraphs (`**Label.** text`); text
// before the first label (rare) is its own unit under the bare ADR heading
// path.
func splitADR(rawLines []string, heads []headingLine) []Unit {
	paths := headingPathStack(heads)
	var units []Unit
	for i, h := range heads {
		if h.level != 2 || !adrHeadRE.MatchString(h.text) {
			continue
		}
		end := len(rawLines) + 1
		if i+1 < len(heads) {
			end = heads[i+1].line
		}
		units = append(units, splitADRSection(rawLines, paths[i], h.line+1, end)...)
	}
	return units
}

func splitADRSection(rawLines []string, sectionPath string, start, endBefore int) []Unit {
	type label struct {
		text string
		line int
	}
	var labels []label
	for ln := start; ln < endBefore; ln++ {
		if m := boldLabRE.FindStringSubmatch(rawLines[ln-1]); m != nil {
			labels = append(labels, label{text: strings.TrimSpace(m[1]), line: ln})
		}
	}
	var units []Unit
	// Preamble before the first label (if any) — rare, but never dropped.
	firstLabelLine := endBefore
	if len(labels) > 0 {
		firstLabelLine = labels[0].line
	}
	if body := strings.TrimSpace(strings.Join(rawLines[start-1:min(firstLabelLine, endBefore)-1], "\n")); body != "" {
		units = append(units, Unit{
			HeadingPath: sectionPath,
			LineStart:   start,
			LineEnd:     max(start, firstLabelLine-1),
			Kind:        KindADRSection,
			Body:        body,
		})
	}
	for i, lb := range labels {
		next := endBefore
		if i+1 < len(labels) {
			next = labels[i+1].line
		}
		body := strings.TrimSpace(strings.Join(rawLines[lb.line-1:next-1], "\n"))
		if body == "" {
			continue
		}
		units = append(units, Unit{
			HeadingPath: sectionPath + " > " + lb.text,
			LineStart:   lb.line,
			LineEnd:     next - 1,
			Kind:        KindADRSection,
			Body:        body,
		})
	}
	return units
}

// splitPlan applies DocKindPlan's rule: every checklist row carrying an
// `{id: ...}` annotation becomes its own KindPlanRow unit (heading path =
// enclosing section path + " > " + its id), pulled out of the body before
// the same leaf-section logic DocKindOther uses runs over what remains —
// so a plan row is indexed exactly once, never inside its section's prose
// too.
func splitPlan(rawLines []string, heads []headingLine) []Unit {
	paths := headingPathStack(heads)
	exclude := map[int]bool{}
	var rowUnits []Unit
	for idx, l := range rawLines {
		if !planRowRE.MatchString(l) {
			continue
		}
		m := rowIDRE.FindStringSubmatch(l)
		if m == nil {
			continue // a checklist line with no {id: ...} is left as ordinary prose
		}
		id := m[1]
		exclude[idx] = true
		lineNo := idx + 1
		path := enclosingPath(heads, paths, lineNo)
		if path != "" {
			path += " > " + id
		} else {
			path = id
		}
		rowUnits = append(rowUnits, Unit{
			HeadingPath: path,
			LineStart:   lineNo,
			LineEnd:     lineNo,
			Kind:        KindPlanRow,
			Body:        strings.TrimSpace(l),
			RowID:       id,
		})
	}
	sectionUnits := splitLeafSectionsExcluding(rawLines, heads, exclude)
	// Row units interleave with section units in line order for a
	// deterministic, human-legible output (I2/determinism: no map
	// iteration reaches this order — both slices are already
	// line-ordered, this is a plain merge).
	return mergeByLine(sectionUnits, rowUnits)
}

// enclosingPath returns the heading path open at 1-based lineNo: the last
// heading whose line is < lineNo.
func enclosingPath(heads []headingLine, paths []string, lineNo int) string {
	path := ""
	for i, h := range heads {
		if h.line < lineNo {
			path = paths[i]
		} else {
			break
		}
	}
	return path
}

func mergeByLine(a, b []Unit) []Unit {
	out := make([]Unit, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].LineStart <= b[j].LineStart {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	out = append(out, a[i:]...)
	out = append(out, b[j:]...)
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
