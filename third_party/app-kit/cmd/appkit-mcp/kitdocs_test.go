package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func testDocs() *kitDocs {
	return &kitDocs{
		decisions: []decisionDoc{
			{ID: "0002", Slug: "cue-tokens", Title: "CUE tokens"},
			{ID: "0005", Slug: "loopback-invariant", Title: "The loopback invariant"},
			{ID: "0007", Slug: "cue-source-of-truth", Title: "CUE is the source of truth"},
			{ID: "0010", Slug: "component-data-pipeline", Title: "Component data pipeline"},
		},
		extra: map[string]decisionDoc{
			"decisions": {ID: "decisions", Slug: "decisions", Title: "Decision register"},
			"vision":    {ID: "vision", Slug: "vision", Title: "Vision"},
		},
	}
}

func TestFindDecision(t *testing.T) {
	d := testDocs()
	cases := []struct {
		query  string
		wantID string
		wantOK bool
	}{
		{"0005", "0005", true},                    // exact padded id
		{"5", "0005", true},                       // bare id zero-pads
		{"loopback-invariant", "0005", true},      // exact slug
		{"loopback", "0005", true},                // unambiguous substring
		{"component-data-pipeline", "0010", true}, // exact slug
		{"decisions", "decisions", true},          // extra doc
		{"vision", "vision", true},                // extra doc
		{"cue", "", false},                        // ambiguous: 0002 + 0007 → not found
		{"nonexistent", "", false},                // no match
		{"", "", false},                           // empty
		{"   ", "", false},                        // whitespace only
		{"LOOPBACK", "0005", true},                // case-insensitive
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			doc, ok := d.findDecision(c.query)
			if ok != c.wantOK {
				t.Fatalf("findDecision(%q) ok = %v, want %v (got id %q)", c.query, ok, c.wantOK, doc.ID)
			}
			if ok && doc.ID != c.wantID {
				t.Errorf("findDecision(%q) id = %q, want %q", c.query, doc.ID, c.wantID)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Governance assertions (ADR-0017 / plan multi-target-kit.md §B).
//
// These read the real docs tree rather than a fixture on purpose: the failure
// they exist to catch is a doc going stale, and a fixture cannot go stale.
// ─────────────────────────────────────────────────────────────────────────────

const kitRootRel = "../.."

// adrFiles returns docs/adr/NNNN-*.md as (id, path) pairs, skipping README and
// the template.
func adrFiles(t *testing.T) map[string]string {
	t.Helper()
	dir := filepath.Join(kitRootRel, "docs", "adr")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	out := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "README.md" || name == "template.md" {
			continue
		}
		id, _, ok := strings.Cut(strings.TrimSuffix(name, ".md"), "-")
		if !ok || len(id) != 4 {
			t.Errorf("ADR filename %q is not NNNN-slug.md", name)
			continue
		}
		out[id] = filepath.Join(dir, name)
	}
	if len(out) == 0 {
		t.Fatalf("no ADRs found under %s", dir)
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// statusLine returns the ADR's `- **Status:** …` value, joined across
// continuation lines so a wrapped status is still one string.
func statusLine(md string) string {
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "- **Status:**") {
			continue
		}
		status := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- **Status:**"))
		for _, cont := range lines[i+1:] {
			trimmed := strings.TrimSpace(cont)
			if trimmed == "" || strings.HasPrefix(trimmed, "- **") || strings.HasPrefix(trimmed, "#") {
				break
			}
			status += " " + trimmed
		}
		return status
	}
	return ""
}

var fourDigits = regexp.MustCompile(`\d{4}`)

// TestADRStatusLinesAreHonest — a status line that says "Superseded" must name
// a successor that exists, and the two supersessions ADR-0017 performed must
// still be visible in the documents they touched. The failure this catches is
// the cheap one: a status line edited to Superseded with no successor named, or
// ADR-0002's retired clause quietly un-struck so it reads as current again.
func TestADRStatusLinesAreHonest(t *testing.T) {
	adrs := adrFiles(t)

	for id, path := range adrs {
		status := statusLine(readFile(t, path))
		if status == "" {
			t.Errorf("ADR-%s has no `- **Status:**` line", id)
			continue
		}
		if !strings.Contains(strings.ToLower(status), "supersed") {
			continue
		}
		named := false
		for _, cand := range fourDigits.FindAllString(status, -1) {
			if cand == id {
				continue // its own number doesn't count as a successor
			}
			if _, ok := adrs[cand]; ok {
				named = true
			}
		}
		if !named {
			t.Errorf("ADR-%s status says superseded but names no existing successor ADR: %q", id, status)
		}
	}

	// ADR-0016's carve-out became a target class in ADR-0017.
	if s := statusLine(readFile(t, adrs["0016"])); !strings.Contains(s, "0017") {
		t.Errorf("ADR-0016 status must name ADR-0017 as its successor, got %q", s)
	}

	// ADR-0002's status must name 0017, and the clause ADR-0017 §3 retired must
	// survive ONLY inside a strikethrough. It is the sentence the kit breaks
	// four times (chat.js, overlay.js, router.js, motion.js); a reader arriving
	// at 0002 via an old citation has to see that it died.
	adr2 := readFile(t, adrs["0002"])
	if s := statusLine(adr2); !strings.Contains(s, "0017") {
		t.Errorf("ADR-0002 status must name ADR-0017, got %q", s)
	}
	const retired = "JavaScript is never written as source"
	flat := strings.Join(strings.Fields(adr2), " ")
	for i := 0; ; {
		j := strings.Index(flat[i:], retired)
		if j < 0 {
			break
		}
		at := i + j
		before := flat[max(0, at-4):at]
		if !strings.Contains(before, "~~") {
			t.Errorf("ADR-0002 still asserts %q outside a strikethrough (preceded by %q) — ADR-0017 §3 retired it", retired, before)
		}
		i = at + len(retired)
	}
	if !strings.Contains(flat, "~~") {
		t.Error("ADR-0002 contains no strikethrough at all — the retired clause was deleted rather than struck; the trail must stay readable")
	}
}

// TestDecisionsIndexCoversEveryADR — every ADR on disk has a row in both
// indexes, and the guidance footer's D-range is derived rather than typed. The
// hardcoded footer read "D1–D15" for two decisions after D16 landed, which is
// an agent being told a binding decision does not exist.
func TestDecisionsIndexCoversEveryADR(t *testing.T) {
	register := readFile(t, filepath.Join(kitRootRel, "docs", "DECISIONS.md"))
	index := readFile(t, filepath.Join(kitRootRel, "docs", "adr", "README.md"))

	for _, path := range adrFiles(t) {
		link := filepath.Base(path)
		if !strings.Contains(register, link) {
			t.Errorf("docs/DECISIONS.md has no row linking %s", link)
		}
		if !strings.Contains(index, link) {
			t.Errorf("docs/adr/README.md has no row linking %s", link)
		}
	}

	// The register's body headings (### D17 — …) are an independent witness to
	// the table's rows, so a footer derived from the table still has to agree
	// with the prose. That is what makes this non-tautological.
	bodyMax := 0
	for _, m := range regexp.MustCompile(`(?m)^### D(\d+)\b`).FindAllStringSubmatch(register, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > bodyMax {
			bodyMax = n
		}
	}
	if bodyMax == 0 {
		t.Fatal("docs/DECISIONS.md has no `### DN` body sections")
	}
	want := fmt.Sprintf("–D%d", bodyMax)
	docs := loadKitDocs(kitRootRel)
	if !strings.Contains(docs.guidance(), want) {
		t.Errorf("get_kit_guidance footer does not end at %s (dRange = %q); the register's highest body section is D%d", want, docs.dRange, bodyMax)
	}
}

// TestFindDecision_Hazards — the theming/contrast hazard checklist must resolve
// by name through get_kit_decision("hazards"). It rides along with
// get_kit_theming too, but an agent that reaches for the review checklist
// directly (the five-minute audit before shipping a theme) should find it
// without knowing which tool bundles it. loadKitDocs registers it from
// docs/theming-hazards.md; this pins that it stays reachable.
func TestFindDecision_Hazards(t *testing.T) {
	doc, ok := loadKitDocs(kitRootRel).findDecision("hazards")
	if !ok {
		t.Fatal(`get_kit_decision("hazards") does not resolve — register loadExtra("hazards", …) in loadKitDocs`)
	}
	if strings.TrimSpace(doc.Body) == "" {
		t.Error(`get_kit_decision("hazards") resolved to an empty body`)
	}
	// It is the hazard checklist, not some other doc that happened to match.
	if !strings.Contains(doc.Body, "MEASURE, NEVER READ") {
		t.Error(`the "hazards" doc is not docs/theming-hazards.md (missing its H3 heading)`)
	}
}

// TestGuidanceIsCurrent — get_kit_guidance returns app-kit/CLAUDE.md verbatim
// (kitdocs.go), so CLAUDE.md is what every agent session reads first. Fixing
// the global ~/.claude/CLAUDE.md without fixing this file silently regresses on
// the next session's first MCP call, which is exactly how the old Path A/Path B
// rule outlived the decision that retired it.
func TestGuidanceIsCurrent(t *testing.T) {
	g := loadKitDocs(kitRootRel).guidance()

	for _, want := range []string{"ADR-0017", "L3a", "appkit vendor", "guides, it does not dictate", "Stopping is a complete adoption"} {
		if !strings.Contains(g, want) {
			t.Errorf("kit guidance does not mention %q", want)
		}
	}
	for _, stale := range []string{"D1–D15", "D1-D15"} {
		if strings.Contains(g, stale) {
			t.Errorf("kit guidance still carries the stale hardcoded D-range %q", stale)
		}
	}
	// The old rule, in the words it was written in. It said React apps could not
	// use the kit; a byte-identical copy of router.js had been in production on
	// a React site for months when it was written down.
	if strings.Contains(g, "the no-Node rule do **not** apply here") {
		t.Error("kit guidance still carries the retired Path A/Path B framing")
	}
}
