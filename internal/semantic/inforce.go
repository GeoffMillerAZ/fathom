package semantic

// inforce.go — RS-2 (ADR-077 d3, docs/plans/rule-selection.md): a compiled
// claim whose citation lies in a FULLY superseded decision record is not in
// force. Serving withholds it from the cap and counts it on the payload
// (Obligations.Superseded), so the withhold is disclosed, never silent (I7,
// I35's tail line). A PARTLY superseded record ("Superseded in mechanism by
// ADR-013; revived in part by ADR-021") still binds in part: its claims stay,
// tagged (Obligation.InForce). The record's own status line is the oracle,
// read through internal/adr.ParseSections exactly as internal/intent's audit
// reads it: a section without a status line parses as accepted. Nothing is
// written: valid_to stays empty and no column is added (ADR-077 d3, d6).

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/adr"
)

// The standings claimInForce reports.
const (
	inForce          = ""
	partlySuperseded = "partly_superseded"
	fullySuperseded  = "superseded"
)

// partialSupersessionRe matches the status-line phrases that keep a
// superseded record partly in force ("superseded in mechanism", "revived in
// part"), as words: "in particular" is not one.
var partialSupersessionRe = regexp.MustCompile(`\b(in part|partly|partially|in mechanism|revived)\b`)

// isDecisionLog reports whether a cited path is a single-file decision log
// (ParseSections' grammar: one `## ADR-NNN` section per record).
func isDecisionLog(p string) bool {
	return strings.EqualFold(path.Base(p), "ADR.md")
}

// decisionSections reads and parses one decision log under root, bounded
// like every serve-time citation re-read (readCitedLines). Nil when the
// file is missing, unreadable or over the bound: an unread log withholds
// nothing.
func decisionSections(root, rel string) []adr.Section {
	if root == "" {
		return nil
	}
	lines := readCitedLines(filepath.Join(root, filepath.FromSlash(rel)))
	if lines == nil {
		return nil
	}
	return adr.ParseSections([]byte(strings.Join(lines, "\n")))
}

// statusParagraph reads only the labelled status value, never surrounding prose.
var statusLabel = regexp.MustCompile(`(?i)^\*\*Status[.:]?\*\*[:.]?\s*(.*)$`)

func statusParagraph(body string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		m := statusLabel.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		value := m[1]
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if next == "" || strings.HasPrefix(next, "**") || strings.HasPrefix(next, "#") {
				break
			}
			value += " " + next
		}
		return strings.ToLower(strings.TrimSpace(value))
	}
	return ""
}

func recordStanding(body string) string {
	status := statusParagraph(body)
	words := strings.Fields(status)
	if len(words) == 0 {
		return inForce
	}
	first := strings.Trim(words[0], ".,:;()")
	switch first {
	case "proposed", "rejected", "deferred":
		return first
	case "superseded":
		if partialSupersessionRe.MatchString(status) {
			return partlySuperseded
		}
		return fullySuperseded
	case "partly", "partially":
		if len(words) > 1 && strings.Trim(words[1], ".,:;()") == "superseded" {
			return partlySuperseded
		}
	}
	return inForce
}

func inactiveStanding(st string) bool { return st != "" && st != partlySuperseded }

// claimInForce reports the standing of a claim whose primary citation is
// cite ("ADR.md#L852-L853") against the decision log's parsed sections
// (file order, as ParseSections returns them): inForce, partlySuperseded or
// fullySuperseded, and the id of the record that decided it. A citation
// outside every section, or not into a decision log, is in force.
func claimInForce(cite string, sections []adr.Section) (standing, record string) {
	p, start, _, ok := parseCite(cite)
	if strings.HasPrefix(p, "docs/plans/archive/") {
		return "archived", ""
	}
	if !ok || !isDecisionLog(p) || len(sections) == 0 {
		return inForce, ""
	}
	// The section holding start: the last one whose heading is at or above it.
	i := sort.Search(len(sections), func(i int) bool { return sections[i].Line > start }) - 1
	if i < 0 {
		return inForce, ""
	}
	sec := sections[i]
	standing = recordStanding(sec.Body)
	if standing == inForce {
		return inForce, ""
	}
	return standing, sec.ID
}

// rawSectionStanding applies the same status parser to any ADR heading.
func rawSectionStanding(sec Section, decisions []adr.Section) string {
	if strings.HasPrefix(sec.Path, "docs/plans/archive/") {
		return "archived"
	}
	if isDecisionLog(sec.Path) {
		st, _ := claimInForce(fmt.Sprintf("%s#L%d-L%d", sec.Path, sec.StartLine, sec.EndLine), decisions)
		return st
	}
	if regexpADRHeading.MatchString(strings.TrimSpace(strings.SplitN(sec.Text, "\n", 2)[0])) {
		return recordStanding(sec.Text)
	}
	return inForce
}
func rawSectionInForce(sec Section, decisions []adr.Section) bool {
	return !inactiveStanding(rawSectionStanding(sec, decisions))
}

var regexpADRHeading = regexp.MustCompile(`^#+\s+ADR-\d+\b`)
var checkedPlanRow = regexp.MustCompile(`^[-*+]\s+\[[xX]\]\s`)

// inForceResolver retains only request-local parsed files. Unread files fail open.
func inForceResolver(root string) func(cite string) (standing, record string) {
	parsed := map[string][]adr.Section{}
	plans := map[string][]string{}
	return func(cite string) (string, string) {
		p, start, _, ok := parseCite(cite)
		if strings.HasPrefix(p, "docs/plans/archive/") {
			return "archived", ""
		}
		if !ok {
			return inForce, ""
		}
		if strings.HasPrefix(p, "docs/plans/") {
			lines, done := plans[p]
			if !done {
				lines = readCitedLines(filepath.Join(root, filepath.FromSlash(p)))
				plans[p] = lines
			}
			// A continuation citation inherits the nearest preceding plan row.
			for i := min(start, len(lines)) - 1; i >= 0; i-- {
				line := strings.TrimSpace(lines[i])
				if checkedPlanRow.MatchString(line) {
					return "checked", ""
				}
				if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "- [") || strings.HasPrefix(line, "* [") || strings.HasPrefix(line, "+ [") {
					break
				}
			}
		}
		if !isDecisionLog(p) {
			return inForce, ""
		}
		secs, done := parsed[p]
		if !done {
			secs = decisionSections(root, p)
			parsed[p] = secs
		}
		return claimInForce(cite, secs)
	}
}
