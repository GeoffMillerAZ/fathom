package semantic

// supersede.go — SU-20 (ADR-041 decision 7, docs/vision/31-substrate.md §3
// D6): an explicit "supersedes" link from a successor claim to the
// predecessor it replaces, instead of the predecessor's terminal status
// alone. Written only where a genuine successor claim exists — a native
// review (or compile) reconciliation that re-emits the SAME subject under a
// DIFFERENT content-addressed claim id (the rule text changed, so the id
// changed too). External closure/revocation and `fathom claim refute` are
// deliberately NOT callers: both retire a claim in place (the closing event
// computes the SAME content-addressed claim id as the finding it closes, or
// simply marks the existing claim refuted) — there is no distinct successor
// row for either to link to.

import (
	"context"
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// WriteSupersedes records that successorID replaces predecessorID: a single
// "supersedes" ClaimLink on the SUCCESSOR (ClaimID = successorID, TargetRef
// = predecessorID, AnchorText = why). Appended via store.Store.
// AppendClaimLink — never PutClaims, whose link-writing side would replace
// the successor's WHOLE link set (dropping the cites/scopes/scope-fp links
// its own mint already wrote). serviceID scopes the stored link row exactly
// like every other claim_links write (store.Store.PutClaims' own svcOf
// lookup). A no-op when either id is empty — a caller unsure whether it
// found a genuine successor should simply not call this, but an accidental
// empty string is treated as "nothing to record" rather than a corrupt link.
func WriteSupersedes(ctx context.Context, st store.Store, serviceID, successorID, predecessorID, why string) error {
	if successorID == "" || predecessorID == "" {
		return nil
	}
	link := core.ClaimLink{ClaimID: successorID, Rel: core.RelSupersedes, TargetRef: predecessorID, AnchorText: why}
	if err := st.AppendClaimLink(ctx, serviceID, link); err != nil {
		return fmt.Errorf("semantic: WriteSupersedes %s -> %s: %w", successorID, predecessorID, err)
	}
	return nil
}

// SupersessionChain walks the "supersedes" links starting at claimID and
// returns every predecessor it transitively replaces, nearest first (the
// claim claimID directly superseded, then THAT claim's own predecessor, and
// so on) — nil when claimID supersedes nothing. Cycle-safe: a claim id
// already seen in the walk stops it rather than looping forever, which a
// malformed or adversarial link graph could otherwise cause.
func SupersessionChain(ctx context.Context, st store.Store, serviceID, claimID string) ([]string, error) {
	if claimID == "" {
		return nil, nil
	}
	var chain []string
	seen := map[string]bool{claimID: true}
	cur := claimID
	for {
		links, err := st.ClaimLinks(ctx, serviceID, []string{cur})
		if err != nil {
			return chain, fmt.Errorf("semantic: SupersessionChain %s: %w", claimID, err)
		}
		next := ""
		for _, l := range links {
			if l.Rel == core.RelSupersedes {
				next = l.TargetRef
				break
			}
		}
		if next == "" || seen[next] {
			return chain, nil
		}
		chain = append(chain, next)
		seen[next] = true
		cur = next
	}
}
