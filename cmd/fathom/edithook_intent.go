package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// edithook_intent.go — the intent slots of the pre-edit guard (IV-3b,
// ADR-074 decision 6).
//
// A generated guard entry (intentexport.go) carries a router row as a
// pointer note: its globs, the invariant ids with their rungs, the
// Done-needs tests and the row's stamp. At an edit that note is not printed
// raw: the ids and the tests ride at most intentSlotsMax of I35's six
// slots, the governing rules get the rest, and the stamp (the audit's
// business) never reaches the agent. A generated entry speaks even when a
// hand-written guard matches the same file first: the hand guard keeps its
// note and options, and the intent lines are added beside it.

const (
	// intentSlotsMax is how many of I35's semantic.MaxObligations slots the
	// intent lines may take at one edit.
	intentSlotsMax = 2
	// intentSlotMaxWords is I35's per-rule word cap, applied to each intent
	// line as to any rule.
	intentSlotMaxWords = 28
	// intentInvariantsPath is where the ids resolve (the kit's corpus layout).
	intentInvariantsPath = "docs/intent/invariants.md"
)

// intentStampTailRe matches a generated note's trailing provenance stamp.
var intentStampTailRe = regexp.MustCompile(`\s*@ [0-9a-f]{12}\s*$`)

// isIntentGuard reports whether g is a generated (router-derived) entry:
// its note opens with `intent:` and ends with the row's stamp. Every other
// entry is hand-written and fathom never interprets it.
func isIntentGuard(g guardspec.Guard) bool {
	return strings.HasPrefix(g.Note, "intent:") && intentStampTailRe.MatchString(g.Note)
}

// matchingGuards splits the file guards matching repoRelFile into the first
// hand-written one (the guard that decides note, cap and options, exactly
// as matchingFileGuard did) and every generated entry.
func matchingGuards(gs guardspec.GuardSet, repoRelFile string) (hand guardspec.Guard, handOK bool, intents []guardspec.Guard) {
	for _, g := range gs.Guards {
		if len(g.Scopes) == 0 || !filter.New(g.Scopes).Match(repoRelFile) {
			continue
		}
		if isIntentGuard(g) {
			intents = append(intents, g)
			continue
		}
		if !handOK {
			hand, handOK = g, true
		}
	}
	return hand, handOK, intents
}

// intentSlotLines renders the generated entries' ids and tests as at most
// intentSlotsMax lines, each within the word cap. Ids and tests are merged
// across entries (a file two rows cover gets both rows' ids once).
func intentSlotLines(intents []guardspec.Guard) []string {
	var ids, tests, other []string
	seenID, seenTest := map[string]bool{}, map[string]bool{}
	for _, g := range intents {
		i, t, o := parseIntentNote(g.Note)
		for _, id := range i {
			if !seenID[id] {
				seenID[id] = true
				ids = append(ids, id)
			}
		}
		for _, tn := range t {
			if !seenTest[tn] {
				seenTest[tn] = true
				tests = append(tests, tn)
			}
		}
		if o != "" {
			other = append(other, o)
		}
	}
	sortInvariantIDs(ids)
	sortStringsLocal(tests)
	var lines []string
	if len(ids) > 0 {
		lines = append(lines, capWords("intent "+intentInvariantsPath+":", ids, " · ", "invariant(s)"))
	}
	switch {
	case len(tests) > 0:
		lines = append(lines, capWords("done needs:", tests, ", ", "test(s)"))
	case len(other) > 0:
		lines = append(lines, capWords("done needs:", strings.Fields(strings.Join(other, "; ")), " ", "word(s)"))
	}
	if len(lines) > intentSlotsMax {
		lines = lines[:intentSlotsMax]
	}
	return lines
}

// parseIntentNote reads a generated note back: `intent: <globs> — <ids>;
// done needs: <tests> @ <stamp>`. It also reads the EP-21 pasted spelling
// (`intent: governed by <ids>; done needs: <tests> (fathom intent audit) @
// <stamp>`), so an un-regenerated file degrades to the same lines.
// Ids keep their rung: "I19 [test]".
func parseIntentNote(note string) (ids, tests []string, other string) {
	s := strings.TrimSpace(strings.TrimPrefix(note, "intent:"))
	s = intentStampTailRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(strings.TrimSuffix(s, "(fathom intent audit)"))
	if i := strings.Index(s, " — "); i >= 0 {
		s = s[i+len(" — "):]
	}
	idsPart, donePart := s, ""
	if i := strings.Index(s, "done needs:"); i >= 0 {
		idsPart = strings.TrimSuffix(strings.TrimSpace(s[:i]), ";")
		donePart = strings.TrimSpace(s[i+len("done needs:"):])
	}
	idsPart = strings.TrimPrefix(strings.TrimSpace(idsPart), "governed by ")
	for _, id := range strings.Split(idsPart, " · ") {
		if id = strings.TrimSpace(id); intentIDRungRe.MatchString(id) {
			ids = append(ids, id)
		}
	}
	if donePart == "" {
		return ids, nil, ""
	}
	for _, tn := range strings.Split(donePart, ",") {
		tn = strings.TrimSpace(tn)
		if !testNameRe.MatchString(tn) {
			return ids, nil, donePart // prose, not a test list: carried whole
		}
		tests = append(tests, tn)
	}
	return ids, tests, ""
}

var (
	intentIDRungRe = regexp.MustCompile(`^I\d+( \[[a-z-]+\])?$`)
	testNameRe     = regexp.MustCompile(`^Test\w*$`)
)

// capWords joins items after lead, stopping before the line would pass the
// word cap (counted as the agent sees it, separators included) and naming
// what it left out — a cut that counts itself.
func capWords(lead string, items []string, sep, noun string) string {
	line := lead
	for i := range items {
		cand := lead + " " + strings.Join(items[:i+1], sep)
		tail := ""
		if i < len(items)-1 {
			tail = " +N more " + noun
		}
		if len(strings.Fields(cand+tail)) > intentSlotMaxWords {
			return line + " +" + strconv.Itoa(len(items)-i) + " more " + noun
		}
		line = cand
	}
	return line
}

// intentRuleCap is the governing-rule cap for an edit that also carries
// intent lines: the guard's own cap, never more than the slots I35 leaves.
func intentRuleCap(maxRules, intentLines int) int {
	if maxRules <= 0 || maxRules > semantic.MaxObligations {
		maxRules = semantic.MaxObligations
	}
	if left := semantic.MaxObligations - intentLines; maxRules > left {
		maxRules = left
	}
	return maxRules
}
