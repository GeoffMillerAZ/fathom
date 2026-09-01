package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// kitDocs holds the app-kit guidance an agent needs to use the kit well: the
// ADRs (docs/adr/NNNN-*.md) plus the binding conventions (CLAUDE.md, the
// DECISIONS register, VISION). It is loaded at startup from the kit checkout
// and degrades gracefully — if the root is wrong, the slices are simply empty
// and the tools say so.
type kitDocs struct {
	root      string
	decisions []decisionDoc
	extra     map[string]decisionDoc // slug -> doc (claude, decisions, vision)
	dRange    string                 // "D1–D17", derived from DECISIONS.md — never hardcoded
}

type decisionDoc struct {
	ID    string // "0005" for ADRs; a slug for extras
	Slug  string // "loopback-invariant"
	Title string
	Body  string
}

func loadKitDocs(root string) *kitDocs {
	d := &kitDocs{root: root, extra: map[string]decisionDoc{}}

	adrDir := filepath.Join(root, "docs", "adr")
	entries, _ := os.ReadDir(adrDir)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		if name == "README.md" || name == "template.md" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(adrDir, name))
		if err != nil {
			continue
		}
		base := strings.TrimSuffix(name, ".md")
		id, slug, _ := strings.Cut(base, "-")
		d.decisions = append(d.decisions, decisionDoc{
			ID:    id,
			Slug:  slug,
			Title: firstHeading(string(body)),
			Body:  string(body),
		})
	}
	sort.Slice(d.decisions, func(i, j int) bool { return d.decisions[i].ID < d.decisions[j].ID })

	loadExtra := func(slug, path string) {
		body, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return
		}
		d.extra[slug] = decisionDoc{ID: slug, Slug: slug, Title: firstHeading(string(body)), Body: string(body)}
	}
	loadExtra("claude", "CLAUDE.md")
	loadExtra("decisions", filepath.Join("docs", "DECISIONS.md"))
	loadExtra("vision", "VISION.md")
	// The theming/contrast hazard checklist. It also rides along with
	// get_kit_theming, but registering it here makes get_kit_decision("hazards")
	// resolve it directly — the review checklist an agent reaches for by name.
	loadExtra("hazards", filepath.Join("docs", "theming-hazards.md"))

	d.dRange = decisionRange(d.extra["decisions"].Body)

	return d
}

// decisionRange derives the D-register's span ("D1–D17") from the register
// itself. It was a hardcoded string until 2026-08-07, by which time it said
// "D1–D15" against a file containing D16 — an agent was being told two
// decisions did not exist. A derived range cannot go stale, so the honesty
// here is structural rather than a thing someone has to remember.
//
// It reads the index-table rows (`| D7 | … |`), not the body headings, because
// the table is the register's canonical list. Returns "" when nothing parses,
// and guidance() then omits the range rather than inventing one.
func decisionRange(md string) string {
	lo, hi := 0, 0
	for _, line := range strings.Split(md, "\n") {
		s := strings.TrimSpace(line)
		if !strings.HasPrefix(s, "| D") {
			continue
		}
		field, _, ok := strings.Cut(strings.TrimPrefix(s, "| "), "|")
		if !ok {
			continue
		}
		num := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(field), "D"))
		n, err := strconv.Atoi(num)
		if err != nil || n <= 0 {
			continue
		}
		if lo == 0 || n < lo {
			lo = n
		}
		if n > hi {
			hi = n
		}
	}
	if hi == 0 {
		return ""
	}
	return fmt.Sprintf("D%d–D%d", lo, hi)
}

// guidance returns the primary "how to build on app-kit" text: CLAUDE.md (the
// hard rules and workflow) with a pointer to the ADRs and decision register.
func (d *kitDocs) guidance() string {
	var b strings.Builder
	if claude, ok := d.extra["claude"]; ok {
		b.WriteString(claude.Body)
	} else {
		b.WriteString("app-kit guidance unavailable: run appkit-mcp with -kit-root pointing at the app-kit checkout.\n")
	}
	b.WriteString("\n---\nFor the binding decisions")
	if d.dRange != "" {
		b.WriteString(" (" + d.dRange + ")")
	}
	b.WriteString(" call get_kit_decision(\"decisions\"); for the north-star vision call get_kit_decision(\"vision\"); for any architecture decision call list_kit_decisions then get_kit_decision(<id>).\n")
	return b.String()
}

// findDecision resolves an ADR by id (e.g. "0005", "5") or slug (e.g.
// "loopback-invariant", "loopback"), and also matches the extra docs
// ("claude", "decisions", "vision").
func (d *kitDocs) findDecision(query string) (decisionDoc, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return decisionDoc{}, false
	}
	if doc, ok := d.extra[q]; ok {
		return doc, true
	}
	qpad := q
	for len(qpad) < 4 && isAllDigits(qpad) {
		qpad = "0" + qpad
	}
	// An exact id or slug always wins.
	for _, doc := range d.decisions {
		if doc.ID == q || doc.ID == qpad || doc.Slug == q {
			return doc, true
		}
	}
	// Otherwise accept a substring match only when it is unambiguous; a partial
	// like "cue" matches both 0002 and 0007, so return not-found and let the
	// caller list_kit_decisions rather than silently picking the lowest id.
	var match decisionDoc
	n := 0
	for _, doc := range d.decisions {
		if strings.Contains(doc.Slug, q) {
			match, n = doc, n+1
		}
	}
	if n == 1 {
		return match, true
	}
	return decisionDoc{}, false
}

func firstHeading(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "# "))
		}
	}
	return ""
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
