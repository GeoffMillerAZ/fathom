package memory

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// finding.go — the highest-value memory source, for almost no new code (§5
// item 2).
//
// A RESOLVED finding is already everything a memory needs: a claim (the
// finding's rule), an anchor (the symbol the fix landed on), evidence (a
// machine-observed resolution, not a human's recollection), and provenance
// (the review run and the commit it resolved by). Nothing here instruments
// anything new — it reads rows phase R already writes.

// FindingKind is the claim kind a finding carries. Mirrored as a literal
// rather than imported so this package stays a leaf over internal/core, the
// same convention cmd/fathom/findings.go uses for the layer name.
const FindingKind = "finding"

// MemoryFromFinding turns a resolved finding into a memory Draft.
//
// It refuses, rather than inventing, in every case where the finding is not
// actually a resolved fact about a named symbol:
//
//   - a claim that is not finding-kind (an obligation is a rule, not a
//     memory of something that happened);
//   - a finding that is not resolved — status must be superseded AND the
//     resolution must be a code change. A scope-disappearance resolution
//     means the file was deleted, not that the issue was understood and
//     fixed, and a refuted finding is a claim someone rejected;
//   - an empty symbol, because "a memory with a symbol anchor" without a
//     symbol is just a file anchor with a stronger label.
//
// The evidence is machine-check: the resolution was OBSERVED by a review run
// re-running and no longer emitting the finding, which is a machine check by
// any reasonable reading. WriteMemory still derives the tier from that kind
// rather than taking this function's word for it.
func MemoryFromFinding(claim core.SemanticClaim, events []core.ClaimEvent, symbol string, author Actor) (Draft, error) {
	symbol = strings.TrimSpace(symbol)
	if claim.Kind != FindingKind {
		return Draft{}, refuse(RefusalBadEvidence,
			"claim %s is kind %q, not %q — only a finding becomes a memory this way", claim.ClaimID, claim.Kind, FindingKind)
	}
	if symbol == "" {
		return Draft{}, refuse(RefusalNoAnchor,
			"finding %s has no resolved symbol to anchor to; %s", claim.ClaimID, UnanchoredRemedy)
	}
	if strings.TrimSpace(claim.Rule) == "" {
		return Draft{}, refuse(RefusalNoClaim, "finding %s has no rule text to remember", claim.ClaimID)
	}
	if claim.Status != core.ClaimSuperseded {
		return Draft{}, refuse(RefusalNoEvidence,
			"finding %s is %q, not %q — only a RESOLVED finding is a memory; an open one is a finding",
			claim.ClaimID, claim.Status, core.ClaimSuperseded)
	}

	kind, commit, ok := latestCodeChangeResolution(events, claim.ClaimID)
	if !ok {
		return Draft{}, refuse(RefusalNoEvidence,
			"finding %s carries no %q supersede event, so nothing observed it being fixed",
			claim.ClaimID, core.ResolutionCodeChange)
	}

	evidence := []Evidence{
		{Kind: EvidenceMachineCheck, Ref: "finding:" + claim.ClaimID},
	}
	if commit != "" {
		evidence = append(evidence, Evidence{Kind: EvidenceMachineCheck, Ref: "resolution:" + kind + "@" + commit})
	}

	anchors := []Anchor{{Kind: AnchorSymbol, Ref: symbol}}
	for _, sc := range claim.Scope {
		if sc = strings.TrimSpace(sc); sc != "" {
			anchors = append(anchors, Anchor{Kind: AnchorFile, Ref: sc})
		}
	}

	if author.Kind == "" {
		author = Actor{Kind: ActorMachine, Name: "review"}
	}

	return Draft{
		Service:  claim.ServiceID,
		Claim:    claim.Rule,
		Anchors:  anchors,
		Evidence: evidence,
		Author:   author,
		Commit:   commit,
	}, nil
}

// latestCodeChangeResolution finds the newest superseded event for claimID
// whose ResolutionKind is a code change, mirroring
// internal/analysis/findings' own latestSupersedeResolution rule (newest At
// wins) without importing it — this package must stay a leaf over core.
func latestCodeChangeResolution(events []core.ClaimEvent, claimID string) (kind, commit string, ok bool) {
	matching := make([]core.ClaimEvent, 0, len(events))
	for _, ev := range events {
		if ev.ClaimID != claimID || ev.Verdict != "superseded" {
			continue
		}
		if ev.ResolutionKind != core.ResolutionCodeChange {
			continue
		}
		matching = append(matching, ev)
	}
	if len(matching) == 0 {
		return "", "", false
	}
	sort.Slice(matching, func(i, j int) bool {
		if !matching[i].At.Equal(matching[j].At) {
			return matching[i].At.After(matching[j].At)
		}
		return matching[i].ResolutionCommit > matching[j].ResolutionCommit
	})
	return matching[0].ResolutionKind, matching[0].ResolutionCommit, true
}
