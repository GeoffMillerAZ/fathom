package main

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// sessionhook_intent.go — repo-wide invariants at session start (IV-3c,
// ADR-074 decision 6).
//
// An invariant that no sensitive-surface row of the router names has no
// generated guard entry, so no edit ever carries it: it governs the whole
// repository, not a path. Its home is the session-start primer, listed once
// by id (the statements stay in invariants.md), inside the whole-primer cap
// (I12). The edit path never lists them (intentSlotLines reads only the
// generated entries, which name only surface-row ids).

// repoWideLineMaxBytes bounds the line so a corpus with hundreds of
// unrouted invariants cannot crowd the primer; the cut names its count.
const repoWideLineMaxBytes = 480

const repoWideLead = "📐 intent — repo-wide invariants (no sensitive surface names them, so no edit carries them; " +
	intentInvariantsPath + "): "

// repoWideInvariantLine is the primer line for projDir, or "" when the repo
// has no router or every invariant is named by a surface row.
func repoWideInvariantLine(projDir string) string {
	c, err := intent.Load(projDir)
	if err != nil || c.Tier != intent.TierRouter || len(c.Invariants) == 0 {
		return ""
	}
	byID := map[string]intent.Invariant{}
	for _, inv := range c.Invariants {
		byID[inv.ID] = inv
	}
	named := map[string]bool{}
	for _, s := range c.Surfaces {
		for _, id := range surfaceInvariantIDs(s, byID) {
			named[id] = true
		}
	}
	var ids []string
	seen := map[string]bool{}
	for _, inv := range c.Invariants {
		if !named[inv.ID] && !seen[inv.ID] {
			seen[inv.ID] = true
			ids = append(ids, inv.ID)
		}
	}
	if len(ids) == 0 {
		return ""
	}
	sortInvariantIDs(ids)
	return renderRepoWideLine(ids)
}

// renderRepoWideLine lists ids after the lead, cut whole ids at a time to
// fit repoWideLineMaxBytes, the cut counted.
func renderRepoWideLine(ids []string) string {
	full := repoWideLead + strings.Join(ids, ", ")
	if len(full) <= repoWideLineMaxBytes {
		return full
	}
	for n := len(ids) - 1; n > 0; n-- {
		line := repoWideLead + strings.Join(ids[:n], ", ") + fmt.Sprintf(" … and %d more", len(ids)-n)
		if len(line) <= repoWideLineMaxBytes {
			return line
		}
	}
	return repoWideLead + fmt.Sprintf("%d invariants", len(ids))
}
