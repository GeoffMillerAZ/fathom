package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// cmdClaim implements the "claim" subcommand: fathom claim refute|corroborate
// — wiring the previously-dead claim_events triage machinery (docs/plans/
// findings-tracer.md FT-3, docs/vision/03-findings-and-experts.md §3).
// AppendClaimEvent already existed with zero non-test callers before this;
// the shipped freshness axis (fresh/stale/contradicted/superseded) is
// machine-owned, and this is the human judgment axis beside it:
//
//	fathom claim refute      --store DB --service S --claim ID --note "..."
//	fathom claim corroborate --store DB --service S --claim ID --note "..."
//
// refute appends a ClaimEvent{Verdict:"refuted"} AND flips the claim's status
// to ClaimRefuted — suppression is refutation with a note. corroborate
// appends a ClaimEvent{Verdict:"corroborated"} and NEVER changes status or
// tier: corroboration is provenance, not evidence-laundering
// (internal/core/claims.go's ClaimRefuted doc comment states the doctrine
// this command must not violate).
//
// v0 is store-direct only (--store required, mirroring `verify --store` /
// `remap --store`): a triage verdict is a rare, deliberate, single write best
// made directly against the store an operator can inspect, not routed
// through the read-serving daemon.
func cmdClaim(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: claim requires a verb: refute | corroborate")
		printClaimUsage(stderr)
		return 1
	}
	switch args[0] {
	case "refute":
		return cmdClaimVerdict(args[1:], stdout, stderr, "refuted", core.ClaimRefuted)
	case "corroborate":
		return cmdClaimVerdict(args[1:], stdout, stderr, "corroborated", "")
	default:
		fmt.Fprintf(stderr, "error: unknown claim verb %q (want refute | corroborate)\n", args[0])
		printClaimUsage(stderr)
		return 1
	}
}

func printClaimUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: fathom claim <refute|corroborate> --store DB --service S --claim ID --note \"...\" [--author NAME]")
}

// cmdClaimVerdict implements one triage verb. newStatus is the ClaimStatus to
// flip the claim to when non-empty; corroborate passes "" — it never changes
// claim status (only appends the event).
func cmdClaimVerdict(args []string, stdout, stderr io.Writer, verdict string, newStatus core.ClaimStatus) int {
	fs := newFlagSet("claim "+verdict, stderr)
	storePath := fs.String("store", "", "path to the DuckDB store (required)")
	service := fs.String("service", "", "service ID (required)")
	claimID := fs.String("claim", "", "claim ID (required)")
	note := fs.String("note", "", "verdict rationale (required)")
	author := fs.String("author", "", "verdict author (optional)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" || *claimID == "" || *note == "" {
		fmt.Fprintf(stderr, "error: claim %s requires --store, --service, --claim, and --note\n", verdict)
		fs.Usage()
		return 1
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()

	ctx := context.Background()

	// The claim must actually exist for this service — a typo'd ID would
	// otherwise silently append a dangling event nothing ever serves.
	claims, err := st.ClaimsForService(ctx, *service, "")
	if err != nil {
		fmt.Fprintf(stderr, "error: load claims: %v\n", err)
		return 1
	}
	found := false
	for _, c := range claims {
		if c.ClaimID == *claimID {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(stderr, "error: claim %q not found for service %q\n", *claimID, *service)
		return 1
	}

	ev := core.ClaimEvent{
		ClaimID: *claimID,
		Verdict: verdict,
		Author:  *author,
		Note:    *note,
		At:      time.Now().UTC(),
	}
	if err := st.AppendClaimEvent(ctx, ev); err != nil {
		fmt.Fprintf(stderr, "error: append claim event: %v\n", err)
		return 1
	}

	if newStatus != "" {
		if err := st.SetClaimStatus(ctx, *service, *claimID, newStatus); err != nil {
			fmt.Fprintf(stderr, "error: set claim status: %v\n", err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "claim:   %s\n", *claimID)
	fmt.Fprintf(stdout, "service: %s\n", *service)
	fmt.Fprintf(stdout, "verdict: %s\n", verdict)
	if newStatus != "" {
		fmt.Fprintf(stdout, "status:  %s\n", newStatus)
	}
	return 0
}
