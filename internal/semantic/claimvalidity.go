// claimvalidity.go — GL-15 (docs/plans/governance-learning.md, design doc
// docs/design/governance-learning.md §7 "Validity windows everywhere a fact
// ends (N8.2)"): closeClaimValidity is the one call this package's own
// claim-ending paths funnel through, so "flip a status, close the window,
// never delete" (SU-19, ADR-041 decision 7) is enforced by construction
// instead of repeated — and, as compile reconciliation's own supersede loop
// was before this row, sometimes forgotten — at each call site.
//
// The three call sites that already existed before this row (cmd/fathom's
// claim.go refute path, internal/findings/external/importer.go's revocation
// and closure branches) already paired SetClaimStatus with SetClaimValidTo
// by hand, and stay that way: neither package can reach an unexported
// internal/semantic helper (cmd/fathom would need to import this package,
// and importer.go's own package would gain a needless dependency for three
// lines). This file exists for compile.go's supersede path — the one
// claim-ending transition that lives inside this package and had drifted
// from the pattern. See TestNoClaimEndingPathDeletes.
package semantic

import (
	"context"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// closeClaimValidity flips claimID's status to a terminal one and closes its
// bi-temporal validity window (ValidTo) at the same instant (SU-19) — the
// two writes always travel together, so no caller can set one without the
// other and no in-package claim-ending path can silently leave a terminal
// claim's window open. It never deletes a row: Store.SetClaimStatus and
// Store.SetClaimValidTo are both UPDATEs against the existing row, the same
// "flip a status, never remove a claim" discipline core.ClaimStatus's own
// doc comment states.
func closeClaimValidity(ctx context.Context, st store.Store, serviceID, claimID string, status core.ClaimStatus, at time.Time) error {
	if err := st.SetClaimStatus(ctx, serviceID, claimID, status); err != nil {
		return fmt.Errorf("semantic: closeClaimValidity set status %s: %w", claimID, err)
	}
	if err := st.SetClaimValidTo(ctx, serviceID, claimID, at); err != nil {
		return fmt.Errorf("semantic: closeClaimValidity set valid_to %s: %w", claimID, err)
	}
	return nil
}
