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

// statusParagraph returns the last `**Status.**` paragraph of a section
// body (the one ParseSections' status came from), lower-cased: the label
// line and its continuation lines up to a blank line or the next bold label.
func statusParagraph(body string) string {
	lines := strings.Split(body, "\n")
	var last string
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(t, "**Status.**") && !strings.HasPrefix(t, "**Status:**") {
			continue
		}
		para := []string{t}
		for j := i + 1; j < len(lines); j++ {
			n := strings.TrimSpace(lines[j])
			if n == "" || strings.HasPrefix(n, "**") || strings.HasPrefix(n, "#") {
				break
			}
			para = append(para, n)
		}
		last = strings.ToLower(strings.Join(para, " "))
	}
	return last
}

// claimInForce reports the standing of a claim whose primary citation is
// cite ("ADR.md#L852-L853") against the decision log's parsed sections
// (file order, as ParseSections returns them): inForce, partlySuperseded or
// fullySuperseded, and the id of the record that decided it. A citation
// outside every section, or not into a decision log, is in force.
func claimInForce(cite string, sections []adr.Section) (standing, record string) {
	p, start, _, ok := parseCite(cite)
	if !ok || !isDecisionLog(p) || len(sections) == 0 {
		return inForce, ""
	}
	// The section holding start: the last one whose heading is at or above it.
	i := sort.Search(len(sections), func(i int) bool { return sections[i].Line > start }) - 1
	if i < 0 {
		return inForce, ""
	}
	sec := sections[i]
	if sec.Status != adr.StatusSuperseded {
		return inForce, ""
	}
	if partialSupersessionRe.MatchString(statusParagraph(sec.Body)) {
		return partlySuperseded, sec.ID
	}
	return fullySuperseded, sec.ID
}

// inForceResolver answers claimInForce for one serve call, parsing each
// cited decision log at most once and only when a candidate cites one.
func inForceResolver(root string) func(cite string) (standing, record string) {
	parsed := map[string][]adr.Section{}
	return func(cite string) (string, string) {
		p, _, _, ok := parseCite(cite)
		if !ok || !isDecisionLog(p) {
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
