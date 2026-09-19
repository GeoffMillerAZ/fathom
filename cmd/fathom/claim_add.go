package main

// claim_add.go — `fathom claim add`, SU-17 (docs/plans/substrate.md §D,
// docs/vision/31-substrate.md §3 D8 item 4): a human's "remember this"
// becomes one scoped, content-addressed claim (producer human.review, tier
// capped at sme-stated) without waiting for recurrence. Flag parsing and
// receipt rendering only — pkg/client.AddClaim does the construction and
// the write, reusing internal/findings/external's own claim-construction
// helpers so a human-authored claim is shaped exactly like an imported one.
//
// v0 is store-direct only (--store required), mirroring `fathom claim
// refute|corroborate` / `fathom findings import`'s own discipline: a
// deliberate, single write best made directly against an operator-
// inspectable store, not routed through the read-serving daemon.

import (
	"context"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdClaimAdd implements:
//
//	fathom claim add --store PATH --service NAME --scope GLOB [--scope GLOB ...] --rule TEXT [--remedy TEXT] [--cite path#Lstart-Lend] [--json]
func cmdClaimAdd(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("claim add", stderr)
	storePath := fs.String("store", "", "path to the store, .sqlite/.db (required)")
	service := fs.String("service", "", "service ID (required)")
	var scopes stringSliceFlag
	fs.Var(&scopes, "scope", "code-path glob this claim governs (repeatable; at least one required)")
	rule := fs.String("rule", "", "the claim text served to agents (required)")
	remedy := fs.String("remedy", "", "optional remediation guidance")
	cite := fs.String("cite", "", "optional citation anchor, path#Lstart-Lend")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable summary")
	// SU-17: tier is capped at sme-stated for producer human.review, always
	// — there is no flag that can raise it. --tier is parsed ONLY so a
	// caller who tries it gets a named refusal instead of the flag package's
	// own "flag provided but not defined" error, and instead of silently
	// accepting and ignoring a value that looks like it did something.
	tier := fs.String("tier", "", "not settable — refused if given; tier is always capped at sme-stated for human.review")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *tier != "" {
		fmt.Fprintf(stderr, "error: claim add: --tier is not settable (got %q) — tier is always capped at sme-stated for producer human.review\n", *tier)
		return 2
	}
	if *storePath == "" || *service == "" || len(scopes) == 0 || *rule == "" {
		fmt.Fprintln(stderr, "error: claim add requires --store, --service, at least one --scope, and --rule")
		fs.Usage()
		return 2
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 2
	}
	defer st.Close()

	c := client.New(connector.NewLocalConnector(), st)
	// SU-22 (ADR-041 decision 7): currentSessionID() resolves the harness's
	// hook session id (rate.go) the same way `fathom rate` already does —
	// "" when none is resolvable (a script invoking the CLI directly), and
	// AddClaim records the claim with an empty AssertedBy in that case,
	// never a fabricated one.
	res, err := c.AddClaim(context.Background(), client.AddClaimRequest{
		Service: *service, Scope: []string(scopes), Rule: *rule, Remedy: *remedy, CiteRef: *cite,
		SessionID: currentSessionID(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	if *jsonOut {
		jsonPrint(stdout, res)
		return 0
	}
	fmt.Fprintf(stdout, "claim:    %s\n", res.ClaimID)
	fmt.Fprintf(stdout, "service:  %s\n", res.Claim.ServiceID)
	fmt.Fprintf(stdout, "producer: %s\n", humanReviewProducerLabel)
	fmt.Fprintf(stdout, "tier:     %s\n", res.Claim.Tier)
	fmt.Fprintf(stdout, "scope:    %v\n", res.Claim.Scope)
	fmt.Fprintf(stdout, "rule:     %s\n", res.Claim.Rule)
	return 0
}

// humanReviewProducerLabel mirrors client.humanReviewProducer (unexported
// across the package boundary) purely for this render line.
const humanReviewProducerLabel = "human.review"
