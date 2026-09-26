package intent

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/adr"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// parseRouter reads the router: its decisions-home pointer and its
// sensitive-surface table. The router is the manifest of homes (the
// practice's §11a), so where the decision records live is read from it
// rather than guessed.
func (c *Corpus) parseRouter(body string) {
	lines := splitLines(body)
	inSurfaces := false
	surfaceRows := 0
	surfaceHeadingLine := 0
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "## ") {
			inSurfaces = strings.Contains(strings.ToLower(t), "sensitive surface")
			if inSurfaces {
				surfaceHeadingLine = i + 1
			}
			continue
		}
		if inSurfaces && strings.HasPrefix(t, "|") && !isTableRule(t) {
			cells := tableCells(t)
			if len(cells) < 3 || strings.EqualFold(strings.TrimSpace(cells[0]), "surface") {
				continue
			}
			globs := backtickTokens(cells[0])
			if len(globs) == 0 {
				continue
			}
			surfaceRows++
			c.Surfaces = append(c.Surfaces, Surface{
				Globs:     globs,
				Why:       strings.TrimSpace(cells[1]),
				DoneNeeds: strings.TrimSpace(cells[2]),
				Tests:     uniqueStrings(testNameRe.FindAllString(cells[2], -1)),
				Line:      i + 1,
			})
		}
		if c.VisionsHome == "" && !inSurfaces && strings.HasPrefix(t, "|") && !isTableRule(t) {
			// The docs-map row whose QUESTION cell names the vision series
			// is the vision home (DP-3). The question cell, not the whole
			// row: "product-vision.md" in another row's answer is not it.
			if cells := tableCells(t); len(cells) >= 2 && strings.Contains(strings.ToLower(cells[0]), "vision") {
				c.VisionsHome, c.VisionsIndex = visionPointer(c.Root, c.RouterPath, strings.Join(cells[1:], " "))
			}
		}
		if strings.Contains(strings.ToLower(t), "decision") && strings.Contains(t, "|") && c.DecisionsHome == "" {
			// A docs-map row routinely names TWO pointers: the index and the
			// records ("`decisions/README.md` (index); records are the
			// `## ADR-NNN` sections of `../../ADR.md`"). The home is
			// whichever one actually HOLDS records — an index is a table of
			// contents, and reading it as the home yields zero decisions,
			// which is exactly the plausible-empty this package refuses.
			c.DecisionsHome = recordBearingPointer(c.Root, c.RouterPath, t)
		}
	}
	// A present table that yields nothing is a finding, not health.
	if surfaceHeadingLine > 0 && surfaceRows == 0 {
		c.Unparsed = append(c.Unparsed, Unparsed{
			File: c.RouterPath, Line: surfaceHeadingLine,
			Reason: "sensitive-surface table present but no row parsed (table shape changed)",
		})
	}
	c.parseConditionsIn(c.RouterPath, body)
}

// parseLivingDoc reads one living document: its conditions always, and its
// invariant rows when it is invariants.md.
func (c *Corpus) parseLivingDoc(rel, body string) {
	c.parseConditionsIn(rel, body)
	if filepath.Base(rel) == "invariants.md" {
		c.parseInvariants(rel, body)
	}
	if filepath.Base(rel) == "scope.md" {
		c.expectShape(rel, body, goalRe, "goal (`**G<n>.`)")
		c.parseGoals(rel, body)
	}
	if filepath.Base(rel) == "futures.md" {
		c.expectShape(rel, body, futureRe, "future (`## F<n>.`)")
	}
}

// expectShape records an Unparsed entry when a document that should carry a
// known shape carries none of it — the healthy-empty guard.
func (c *Corpus) expectShape(rel, body string, re *regexp.Regexp, what string) {
	for _, ln := range splitLines(body) {
		if re.MatchString(strings.TrimSpace(ln)) {
			return
		}
	}
	c.Unparsed = append(c.Unparsed, Unparsed{
		File: rel, Line: 1,
		Reason: "no " + what + " found — the document's shape changed or it is empty",
	})
}

// parseInvariants reads invariants.md's rows: `| I<n> | statement | rung |
// source |`. Backticks and bold are stripped before classification because
// the real table writes rungs every way a human writes emphasis.
func (c *Corpus) parseInvariants(rel, body string) {
	rows := 0
	sawTable := false
	for i, ln := range splitLines(body) {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "| # |") || strings.HasPrefix(t, "| I") {
			sawTable = true
		}
		m := invRowRe.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		cells := tableCells(t)
		if len(cells) < 3 {
			continue
		}
		rows++
		inv := Invariant{
			ID:        m[1],
			Statement: strings.TrimSpace(cells[1]),
			RungRaw:   strings.TrimSpace(cells[2]),
			File:      rel,
			Line:      i + 1,
		}
		inv.Rung, inv.Tests, inv.GateSteps = classifyRung(inv.RungRaw)
		c.Invariants = append(c.Invariants, inv)
	}
	if sawTable && rows == 0 {
		c.Unparsed = append(c.Unparsed, Unparsed{
			File: rel, Line: 1,
			Reason: "invariants table present but no `| I<n> |` row parsed (table shape changed)",
		})
	}
}

// classifyRung reads an Enforced-by cell. Precedence is deliberate:
// VIOLATED anywhere wins (a row that says part of it is false is not
// enforced), then the strongest claim present.
func classifyRung(raw string) (Rung, []string, []string) {
	plain := strings.TrimSpace(stripEmphasis(raw))
	low := strings.ToLower(plain)
	tests := uniqueStrings(testNameRe.FindAllString(plain, -1))

	var gates []string
	for _, m := range gateTaskRe.FindAllStringSubmatch(plain, -1) {
		gates = append(gates, "task "+m[1])
	}
	for _, m := range gateHookRe.FindAllStringSubmatch(plain, -1) {
		gates = append(gates, "lefthook "+m[1]+" "+m[2])
	}

	// The LEADING token decides the rung. A substring search would read
	// "`stated` — nothing checks; a denylist test would lift it" as a test
	// claim, which is exactly backwards: that row's whole point is that
	// nothing enforces it.
	if strings.Contains(strings.ToUpper(raw), "VIOLATED") {
		return RungViolated, tests, gates
	}
	switch leadingToken(low) {
	case "gate":
		if len(gates) == 0 && len(tests) == 0 {
			return RungUnevaluable, tests, gates
		}
		return RungGate, tests, gates
	case "proven":
		return RungProven, tests, gates
	case "test":
		return RungTest, tests, gates
	case "lint":
		return RungLint, tests, gates
	case "guard":
		return RungGuard, tests, gates
	case "code":
		return RungCode, tests, gates
	case "reviewed":
		return RungReviewed, tests, gates
	case "stated":
		return RungStated, tests, gates
	}
	// A cell that opens with something else but names tests is a test claim
	// written loosely (e.g. "`code` (constructor default) + test Foo").
	if len(tests) > 0 {
		return RungTest, tests, gates
	}
	return RungUnevaluable, tests, gates
}

// leadingToken returns the first alphabetic word of a rung cell.
func leadingToken(low string) string {
	for _, f := range strings.FieldsFunc(low, func(r rune) bool {
		return !(r >= 'a' && r <= 'z')
	}) {
		return f
	}
	return ""
}

// parseConditionsIn extracts every condition line from one document.
//
// The grammar, from the corpus as it is actually written: a label followed
// by ":" or ".**", not inside backticks; the text runs to the next label,
// a blank line, the next list item, or the end of a table cell — conditions
// wrap, and a parser that stops at the newline loses half of them. The
// annotation is the trailing `{…}` block, parsed by the PLAN tokenizer so
// the two grammars can never drift into two quoting rules.
func (c *Corpus) parseConditionsIn(rel, body string) {
	lines := splitLines(body)
	record := ""
	counts := map[string]int{}
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if id := recordID(t); id != "" {
			record = id
		}
		for _, ex := range excludedLabelMentions(ln) {
			c.Excluded = append(c.Excluded, Unparsed{File: rel, Line: i + 1, Reason: ex})
		}
		for _, loc := range labelLocations(ln) {
			label, textStart := loc.label, loc.end
			text := strings.TrimSpace(ln[textStart:])
			// Continue across wrapped lines: stop at a blank line, a new
			// list item, a heading, or another label.
			for j := i + 1; j < len(lines) && !strings.HasSuffix(t, "|"); j++ {
				nxt := strings.TrimSpace(lines[j])
				if nxt == "" || strings.HasPrefix(nxt, "- ") || strings.HasPrefix(nxt, "#") ||
					strings.HasPrefix(nxt, "|") || len(labelLocations(lines[j])) > 0 {
					break
				}
				text += " " + nxt
			}
			text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "|"))

			ann, rest, suspect := verify.ParseAnnotation(text)
			id := record
			if id == "" {
				id = strings.TrimSuffix(filepath.Base(rel), ".md")
			}
			counts[id]++
			c.Conditions = append(c.Conditions, Condition{
				ID:       id + "#" + itoa(counts[id]),
				Record:   record,
				Label:    label,
				Text:     strings.TrimSpace(rest),
				File:     rel,
				Line:     i + 1,
				Ann:      ann,
				Terminal: terminalRe.MatchString(rest),
				Suspect:  suspect,
			})
			if suspect != "" {
				c.Unparsed = append(c.Unparsed, Unparsed{
					File: rel, Line: i + 1,
					Reason: "brace block on a condition looked like an annotation but yielded no keys: " + suspect,
				})
			}
		}
	}
}

// excludedLabelMentions explains every appearance of a condition label on a
// line that is NOT a condition, so the parser's set and the router's grep
// can be reconciled line by line.
func excludedLabelMentions(ln string) []string {
	t := strings.TrimSpace(ln)
	var out []string
	// bareLabelRe matches a label with NO punctuation requirement — the
	// shape the router's grep uses. Every hit that labelLocations did not
	// take as a condition is explained here.
	for _, m := range bareLabelRe.FindAllStringSubmatchIndex(ln, -1) {
		label := ln[m[2]:m[3]]
		if isConditionAt(ln, m[0]) {
			continue
		}
		switch {
		case inBackticks(ln, m[0]):
			out = append(out, "label `"+label+"` written as a code span — prose about the practice, not a condition")
		case strings.HasPrefix(t, "|") && headerCellNames(t, label):
			out = append(out, "table header cell naming the `"+label+"` column — not a condition")
		default:
			out = append(out, "label \""+label+"\" mentioned without a following `:` or `.` — prose, not a condition")
		}
	}
	return out
}

// isConditionAt reports whether the label at index i was taken as a real
// condition (labelRe requires the punctuation that opens one, and the label
// must not be inside a code span).
func isConditionAt(ln string, i int) bool {
	for _, m := range labelRe.FindAllStringSubmatchIndex(ln, -1) {
		if m[0] == i && !inBackticks(ln, i) {
			return true
		}
	}
	return false
}

// headerCellNames reports whether a table row's cells contain exactly the
// label — the decisions index's "Revisit when" column.
func headerCellNames(t, label string) bool {
	for _, cell := range tableCells(t) {
		if strings.EqualFold(strings.TrimSpace(cell), label) {
			return true
		}
	}
	return false
}

type labelLoc struct {
	label string
	end   int
}

// labelLocations finds every condition label in a line, ignoring labels
// inside backticks (the router's own grep line names all four).
func labelLocations(ln string) []labelLoc {
	var out []labelLoc
	for _, m := range labelRe.FindAllStringSubmatchIndex(ln, -1) {
		if inBackticks(ln, m[0]) {
			continue
		}
		out = append(out, labelLoc{label: canonicalLabel(ln[m[2]:m[3]]), end: m[1]})
	}
	return out
}

func canonicalLabel(s string) string {
	for _, l := range conditionLabels {
		if strings.EqualFold(l, s) {
			return l
		}
	}
	return s
}

// inBackticks reports whether position i sits inside a code span.
func inBackticks(s string, i int) bool {
	return strings.Count(s[:i], "`")%2 == 1
}

// recordID returns the id a heading or bold opener establishes, so a
// condition's id names the record it belongs to rather than a line number
// that renumbers on every edit.
func recordID(t string) string {
	if m := futureRe.FindStringSubmatch(t); m != nil {
		return m[1]
	}
	if m := goalRe.FindStringSubmatch(t); m != nil {
		return m[1]
	}
	if m := questionRe.FindStringSubmatch(t); m != nil {
		return m[1]
	}
	if m := incidentRe.FindStringSubmatch(t); m != nil {
		return m[1]
	}
	if m := invRowRe.FindStringSubmatch(t); m != nil {
		return m[1]
	}
	if m := regexp.MustCompile(`^##\s+(ADR-\d+)`).FindStringSubmatch(t); m != nil {
		return m[1]
	}
	if m := nonGoalRe.FindStringSubmatch(t); m != nil {
		return "NG-" + slugWords(m[1], 3)
	}
	return ""
}

// decisionsHome resolves where the decision records live: what the router
// says, else this repo's two conventional homes. A home that resolves to
// nothing is reported by the caller, never read as zero decisions.
func (c *Corpus) decisionsHome() string {
	if c.DecisionsHome != "" {
		return c.DecisionsHome
	}
	for _, cand := range []string{"ADR.md", filepath.Join("docs", "adr"), filepath.Join("docs", "intent", "decisions")} {
		if _, err := os.Stat(filepath.Join(c.Root, cand)); err == nil {
			return filepath.ToSlash(cand)
		}
	}
	return ""
}

// parseDecisions reads the decision records' Revisit-when trailers — the
// largest population of conditions in a mature repo.
func (c *Corpus) parseDecisions() {
	home := c.DecisionsHome
	if home == "" {
		return
	}
	abs := filepath.Join(c.Root, home)
	info, err := os.Stat(abs)
	if err != nil {
		c.Unparsed = append(c.Unparsed, Unparsed{
			File: home, Line: 1,
			Reason: "the router names a decisions home that does not exist",
		})
		return
	}
	if info.IsDir() {
		entries, _ := filepath.Glob(filepath.Join(abs, "*.md"))
		for _, e := range entries {
			b, ok := c.read(e)
			if !ok {
				c.Unparsed = append(c.Unparsed, Unparsed{
					File: filepath.ToSlash(rel(c.Root, e)), Line: 1,
					Reason: "decision record could not be read, so its conditions are not in this corpus",
				})
				continue
			}
			c.parseConditionsIn(filepath.ToSlash(rel(c.Root, e)), string(b))
		}
		return
	}
	b, ok := c.read(abs)
	if !ok {
		c.Unparsed = append(c.Unparsed, Unparsed{
			File: home, Line: 1,
			Reason: "the decisions home could not be read, so NO decision's conditions are in this corpus",
		})
		return
	}
	// Sections carry their own ids; parse them so a trailer's condition is
	// attributed to its record rather than the file.
	secs := adr.ParseSections(b)
	if len(secs) == 0 {
		c.Unparsed = append(c.Unparsed, Unparsed{
			File: home, Line: 1,
			Reason: "decisions file present but no `## ADR-NNN` section parsed",
		})
	}
	c.parseConditionsIn(home, string(b))
}

// findNestedRouters names routers below the root. v1 audits the root only;
// naming what it did not audit is the difference between a scoped tool and
// a silently partial one.
func (c *Corpus) findNestedRouters() {
	_ = filepath.WalkDir(c.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "workspace", "third_party", "ref", "tmp":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "README.md" {
			return nil
		}
		r := filepath.ToSlash(rel(c.Root, p))
		if r == c.RouterPath || !strings.HasSuffix(r, "docs/intent/README.md") {
			return nil
		}
		c.NestedRouters = append(c.NestedRouters, r)
		return nil
	})
	sortStrings(c.NestedRouters)
}

// read routes a corpus read through the coverage recorder, so a file that
// exists and cannot be opened is accounted for rather than silently dropped
// (ADR-027). A nil recorder (a Corpus built by a test literal) falls back to
// a plain read: coverage is a property of Load, not of the type.
func (c *Corpus) read(abs string) ([]byte, bool) {
	if c.reads == nil {
		b, err := os.ReadFile(abs)
		return b, err == nil
	}
	return c.reads.ReadFile(abs)
}

// goalTitleRe reads a goal opener's title: `**G2. The A/B becomes
// admissible.**` → "The A/B becomes admissible.".
var goalTitleRe = regexp.MustCompile(`^\*\*G\d+\.\s*(.*?)\*\*`)

// parseGoals records scope.md's goals by identity (DP-4). Whether each is a
// bet is LoadBets' verdict over the goal's conditions, which
// parseConditionsIn already attributed to it by record id — one condition
// grammar, one tokenizer, no second parser for the same lines.
func (c *Corpus) parseGoals(rel, body string) {
	lines := splitLines(body)
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		m := goalRe.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		// A long title wraps ("**G12. … falsifiable\n(ADR-035).**"): join
		// the paragraph's next lines until the bold closes.
		joined := t
		for j := i + 1; j < len(lines) && j <= i+3 && strings.Count(joined, "**") < 2; j++ {
			nxt := strings.TrimSpace(lines[j])
			if nxt == "" {
				break
			}
			joined += " " + nxt
		}
		title := ""
		if tm := goalTitleRe.FindStringSubmatch(joined); tm != nil {
			title = strings.TrimSpace(tm[1])
		}
		c.Goals = append(c.Goals, Goal{ID: m[1], Title: title, File: rel, Line: i + 1})
	}
}

// visionPointer resolves a docs-map row's answer cell to the vision
// series' home directory and index file. The first backticked pointer whose
// path names a vision directory and resolves inside the repo wins; a
// pointer to a file makes its directory the home and the file the index.
func visionPointer(root, routerRel, cell string) (home, index string) {
	base := filepath.Dir(filepath.Join(root, routerRel))
	for _, m := range backtickRe.FindAllStringSubmatch(cell, -1) {
		tok := strings.TrimSpace(m[1])
		if tok == "" || strings.ContainsAny(tok, " *<") || !strings.Contains(strings.ToLower(tok), "vision") {
			continue
		}
		cand := filepath.Clean(filepath.Join(base, tok))
		relPath, err := filepath.Rel(root, cand)
		if err != nil || strings.HasPrefix(relPath, "..") {
			continue
		}
		info, err := os.Stat(cand)
		if err != nil {
			continue
		}
		if info.IsDir() {
			idx := filepath.Join(cand, "README.md")
			if statExists(idx) {
				r, _ := filepath.Rel(root, idx)
				return filepath.ToSlash(relPath), filepath.ToSlash(r)
			}
			return filepath.ToSlash(relPath), ""
		}
		dir := filepath.Dir(relPath)
		if !strings.Contains(strings.ToLower(dir), "vision") {
			// A single vision FILE (`VISION.md`) is not a series; the
			// pipeline tracks a directory of promises.
			continue
		}
		return filepath.ToSlash(dir), filepath.ToSlash(relPath)
	}
	return "", ""
}
