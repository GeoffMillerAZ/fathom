package main

// edithook_skipnote.go — O56 defect A fix item 3: the PostToolUse edit hook
// notes, at the moment of the edit, when a plan row just gained skip_test —
// BEFORE a worker gets the chance to check the box straight past a fail (or
// to point verify_plan at a bare skip and be told it "passes"). This is
// deliberately a NOTE, not a gate: it fires whether or not the row is even
// claimed yet, needs no daemon call (pure local git diff, same discipline as
// claimMomentPlanRel/newlyClaimedTasks), and never blocks the edit — the
// same silence contract every other hook payload in this file follows.

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// withSkipGainedNote composes skipGainedNote's note (if any) onto the edit
// hook's payload — the SAME "lead with the reason this edit needs
// attention, turn a silent hook into an answering one" composition
// withClaimMoment uses.
func withSkipGainedNote(projDir, filePath, text string, ok bool) (string, bool) {
	note := skipGainedNote(projDir, filePath)
	if note == "" {
		return text, ok
	}
	if ok && text != "" {
		return note + "\n\n" + text, true
	}
	return note, true
}

// skipGainedNote returns one line per row that gained `skip_test: true` in
// this edit relative to git HEAD — "" when filePath is not a depth-1 plan
// under any service candidate, the daemon-free local diff finds nothing
// gained, or the file cannot be read. Matches claimMomentVerifyWith's own
// root-candidate walk so the two paths can never disagree about which root
// a plan resolves under.
func skipGainedNote(projDir, filePath string) string {
	for _, cand := range serviceCandidates(projDir) {
		rel, ok := claimMomentPlanRel(cand.root, filePath)
		if !ok {
			continue
		}
		newMD, err := os.ReadFile(filePath)
		if err != nil {
			return ""
		}
		oldMD := headFileVersion(cand.root, rel) // nil: new file / no git
		ids := skipTestGained(oldMD, newMD, "")
		if len(ids) == 0 {
			return ""
		}
		sort.Strings(ids)
		lines := make([]string, len(ids))
		for i, id := range ids {
			lines[i] = fmt.Sprintf(
				"row %s gained skip_test in this edit — verify will count it unproven until the skip predates the claim", id,
			)
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

// skipTestGained returns the IDs of tasks that carry `skip_test: true` in
// newMD but did NOT in oldMD (oldMD nil ⇒ every skip_test row in newMD is
// "gained" — a brand-new file has no prior state to compare against).
// Matching is by task ID, mirroring newlyClaimedTasks — independent of
// claimed/skipped/in-progress state, since the note fires whether or not
// the row is claimed yet.
func skipTestGained(oldMD, newMD []byte, service string) []string {
	oldSkip := map[string]bool{}
	if oldMD != nil {
		for _, t := range verify.ParseMarkdownPlan(service, oldMD).Tasks {
			if t.Expect.SkipTest {
				oldSkip[t.ID] = true
			}
		}
	}
	var out []string
	for _, t := range verify.ParseMarkdownPlan(service, newMD).Tasks {
		if t.Expect.SkipTest && !oldSkip[t.ID] {
			out = append(out, t.ID)
		}
	}
	return out
}
