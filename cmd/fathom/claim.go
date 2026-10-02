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
		fmt.Fprintln(stderr, "error: claim requires a verb: add | refute | corroborate | show | list | effectiveness | learn")
		printClaimUsage(stderr)
		return 1
	}
	switch args[0] {
	case "show":
		return cmdClaimShow(args[1:], stdout, stderr)
	case "list":
		return cmdClaimList(args[1:], stdout, stderr)
	case "effectiveness":
		return cmdClaimEffectiveness(args[1:], stdout, stderr)
	case "learn":
		return cmdClaimLearn(args[1:], stdout, stderr)
	case "add":
		return cmdClaimAdd(args[1:], stdout, stderr)
	case "refute":
		return cmdClaimVerdict(args[1:], stdout, stderr, "refuted", core.ClaimRefuted)
	case "corroborate":
		return cmdClaimVerdict(args[1:], stdout, stderr, "corroborated", "")
	default:
		fmt.Fprintf(stderr, "error: unknown claim verb %q (want add | refute | corroborate | show | list | effectiveness | learn)\n", args[0])
		printClaimUsage(stderr)
		return 1
	}
}

// claimHumanCLIIdentity is the identity recorded for a corroborate/refute
// verdict when no harness session is exported — a human running the CLI
// directly, outside any wired hook. Never "": applyLearning's promotion
// count needs a distinct, non-empty identity to attribute an event to.
const claimHumanCLIIdentity = "human:cli"

// claimVerdictIdentity resolves a corroborate/refute verdict's identity
// (GL-13/GL-14 owner decision, docs/design/governance-learning.md §6): the
// harness session id (currentSessionID, cmd/fathom/rate.go — the same
// registry-driven resolution `fathom rate` uses) when the hook env carries
// one, else claimHumanCLIIdentity.
func claimVerdictIdentity() string {
	if sid := currentSessionID(); sid != "" {
		return sid
	}
	return claimHumanCLIIdentity
}

func printClaimUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  fathom claim add          --store DB --service S --scope GLOB [--scope GLOB ...] --rule \"...\" [--remedy \"...\"] [--cite path#Lstart-Lend] [--json]")
	fmt.Fprintln(w, "  fathom claim refute       --store DB --service S --claim ID --note \"...\" [--author NAME] [--dry-run] [--json]")
	fmt.Fprintln(w, "  fathom claim corroborate  --store DB --service S --claim ID --note \"...\" [--author NAME]")
	fmt.Fprintln(w, "  fathom claim show         <id> [--service S] [--json] [--store DB | --url U]")
	fmt.Fprintln(w, "  fathom claim list         [--layer L] [--service S] [--status current|fresh|stale|contradicted|superseded|refuted|all] [--page N] [--json] [--store DB | --url U]")
	fmt.Fprintln(w, "  fathom claim effectiveness [--service S] [--class ignored|contested|never_triggered|unclassified|effective|all] [--page N] [--json] [--store DB | --url U]")
	fmt.Fprintln(w, "  fathom claim learn        [--service S] [--store DB] [--apply] [--json]")
}

// cmdClaimVerdict implements one triage verb. newStatus is the ClaimStatus to
// flip the claim to when non-empty; corroborate passes "" — it never changes
// claim status (only appends the event).
func cmdClaimVerdict(args []string, stdout, stderr io.Writer, verdict string, newStatus core.ClaimStatus) int {
	fs := newFlagSet("claim "+verdict, stderr)
	storePath := fs.String("store", "", "path to the store, .sqlite/.db (required)")
	service := fs.String("service", "", "service ID (required)")
	claimID := fs.String("claim", "", "claim ID (required)")
	note := fs.String("note", "", "verdict rationale (required, unless --dry-run)")
	author := fs.String("author", "", "verdict author (optional)")
	// SU-18 (docs/plans/substrate.md §D): --dry-run previews refute's
	// disposition — which currently served rows it would withhold — without
	// recording anything. Parsed for every verdict so a caller gets a named
	// refusal rather than "flag provided but not defined"; only "refute"
	// implements it (there is nothing to preview for corroborate, which
	// never withholds anything).
	dryRun := fs.Bool("dry-run", false, "refute only: preview the disposition without writing anything")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable summary")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" || *claimID == "" {
		fmt.Fprintf(stderr, "error: claim %s requires --store, --service, and --claim\n", verdict)
		fs.Usage()
		return 1
	}
	if *dryRun && verdict != "refuted" {
		fmt.Fprintf(stderr, "error: --dry-run is only supported for claim refute (nothing to preview for %s)\n", verdict)
		return 1
	}
	if !*dryRun && *note == "" {
		fmt.Fprintf(stderr, "error: claim %s requires --note\n", verdict)
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
	found, err := claimExists(ctx, st, *service, *claimID)
	if err != nil {
		fmt.Fprintf(stderr, "error: load claims: %v\n", err)
		return 1
	}
	if !found {
		fmt.Fprintf(stderr, "error: claim %q not found for service %q\n", *claimID, *service)
		return 1
	}

	if *dryRun {
		preview, perr := previewRefute(ctx, st, *service, *claimID)
		if perr != nil {
			fmt.Fprintf(stderr, "error: %v\n", perr)
			return 1
		}
		if *jsonOut {
			jsonPrint(stdout, preview)
			return 0
		}
		// A dry-run over one claim: every withheld row is exactly one file
		// (this rule stops serving there), so WouldWithhold == the file
		// count — preview.Files is only the (possibly truncated) NAMED
		// subset, capped separately from the count.
		fmt.Fprintf(stdout, "would withhold %d served row(s) across %d file(s): %s\n",
			preview.WouldWithhold, preview.WouldWithhold, joinTruncated(preview.Files, preview.Truncated))
		return 0
	}

	// GL-13/GL-14 owner decision (docs/design/governance-learning.md §6):
	// a corroborate/refute verdict's identity is the harness session id
	// when the hook env carries one, else the literal "human:cli" — never
	// "" (applyLearning's distinct-session promotion count could never
	// attribute an unidentified event to anyone).
	if err := recordClaimVerdict(ctx, st, claimVerdictInput{
		Service: *service, ClaimID: *claimID, Verdict: verdict, NewStatus: newStatus,
		Author: *author, Note: *note, Identity: claimVerdictIdentity(), At: time.Now().UTC(),
	}); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "claim:   %s\n", *claimID)
	fmt.Fprintf(stdout, "service: %s\n", *service)
	fmt.Fprintf(stdout, "verdict: %s\n", verdict)
	if newStatus != "" {
		fmt.Fprintf(stdout, "status:  %s\n", newStatus)
	}
	return 0
}

// claimExists reports whether claimID is one of service's claims.
func claimExists(ctx context.Context, st store.Store, service, claimID string) (bool, error) {
	claims, err := st.ClaimsForService(ctx, service, "")
	if err != nil {
		return false, err
	}
	for _, c := range claims {
		if c.ClaimID == claimID {
			return true, nil
		}
	}
	return false, nil
}

// claimVerdictInput is one corroborate/refute verdict to record.
// NewStatus is core.ClaimRefuted for a refute and "" for a corroborate
// (corroboration never changes status or tier). Identity is the session
// the verdict is attributed to (claimVerdictIdentity on the CLI, the
// hook's own session id for a one-key answer).
type claimVerdictInput struct {
	Service, ClaimID, Verdict string
	NewStatus                 core.ClaimStatus
	Author, Note, Identity    string
	At                        time.Time
}

// recordClaimVerdict is the verdict core `fathom claim refute|corroborate`
// and the edit hook's one-key answer (SR-48w, askruleverdict.go) share:
// refuse an unknown claim (no dangling event), append the ClaimEvent,
// record the verdict in the GL-10 rule-outcome log, and for a refute flip
// the status and close the validity window at the same instant (SU-19).
func recordClaimVerdict(ctx context.Context, st store.Store, in claimVerdictInput) error {
	found, err := claimExists(ctx, st, in.Service, in.ClaimID)
	if err != nil {
		return fmt.Errorf("load claims: %w", err)
	}
	if !found {
		return fmt.Errorf("claim %q not found for service %q", in.ClaimID, in.Service)
	}
	ev := core.ClaimEvent{
		ClaimID:    in.ClaimID,
		Verdict:    in.Verdict,
		Author:     in.Author,
		Note:       in.Note,
		At:         in.At.UTC(),
		AssertedBy: core.AssertedBy("claim-add", in.Identity),
	}
	if err := st.AppendClaimEvent(ctx, ev); err != nil {
		return fmt.Errorf("append claim event: %w", err)
	}
	// GL-10 (docs/plans/governance-learning.md, design doc §5): a human
	// verdict is one of the six event kinds the rule-effectiveness signal
	// source records, feeding the SAME log the edit and PreToolUse hooks
	// do — never a second, disconnected verdict channel. Verdict is
	// exactly "refuted" or "corroborated", matching ruleOutcomeRefuted/
	// ruleOutcomeCorroborated by value.
	recordRuleOutcome(in.ClaimID, in.Verdict, in.Identity, "")

	if in.NewStatus != "" {
		if err := st.SetClaimStatus(ctx, in.Service, in.ClaimID, in.NewStatus); err != nil {
			return fmt.Errorf("set claim status: %w", err)
		}
		// SU-19 (ADR-041 decision 7): refute is a terminal verdict — close
		// the claim's bi-temporal validity window at the same instant its
		// status flips to refuted. corroborate never reaches here.
		if err := st.SetClaimValidTo(ctx, in.Service, in.ClaimID, ev.At); err != nil {
			return fmt.Errorf("close claim validity window: %w", err)
		}
	}
	return nil
}

// claimRunner runs `fathom claim` (registered below; EW-45).
type claimRunner struct{}

func (claimRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdClaim(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "claim", Group: "plans", Order: 50, Run: claimRunner{},
		Summary: "Triage a compiled claim: refute or corroborate, with a note | show <id> [--json]: statement, citation excerpt, scope, tier, status and verdict history | list [--layer L] [--service S] [--status ST] [--json]: the claims a count counts | effectiveness [--service S] [--class C] [--json]: per governing rule, the hook's served/top_changed/still_served/blocked/refuted counts with n over 30 and 90 days, the class each falls in (ignored, contested, never triggered, effective) and the change the evidence proposes — proposals only, nothing is applied (GL-18, U4) | learn [--service S] [--apply]: promote and retire claims under .fathom/learning.cue from the rule-outcome log, a dry run by default (GL-13)"})
}
