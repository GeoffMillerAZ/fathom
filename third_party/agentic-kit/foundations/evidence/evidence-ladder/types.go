// Package evidenceladder implements the three-tier evidence-acquisition ladder:
// fast cache lookup (Tier-1) → exhaustive search (Tier-2) → human escalation
// (Tier-3). It composes evidence-signature, evidence-grade, and the extraction-
// gap-log without re-implementing any primitive.
//
// # Deterministic progression (non-negotiable invariants)
//
//   - Tier-1 provides signal only, never verdicts. An empty Tier-1 result is a
//     cache miss, never a conclusion. Only Tier-2 exhaustive search can produce
//     a verdict.
//   - PreconditionSatisfied false after exhaustive Tier-2 ⇒ escalate to Tier-3.
//   - Tier-3 timeout is fail-closed to UNRESOLVED, never to ABSENT.
//   - Every Tier-2 escalation emits a GapRecord to the extraction-gap-log.
//
// # ABSENT_PROVEN discipline (Plan D10)
//
// An ABSENT_PROVEN verdict is scoped: "absent within the declared scope at
// snapshot <tree_hash>". It requires:
//   - A CoverageManifest with all files accounted for (Files + Skipped).
//   - Zero unexplained skips (Binary/Oversize/LFSStub/NonUTF8 all empty).
//   - Any unexplained skip downgrades the verdict to ABSENT_UNVERIFIED (fail-closed).
//
// # Tier-3 and kits/hitl (layering note)
//
// This package defines the Tier-3 INTERFACE (Escalate contract) and the
// EscalationRecord/HumanVerdict types. The actual propose→preview→approve→apply
// wiring is a CONSUMER responsibility that composes kits/hitl:
//
//	// In your kit or app (NOT in foundations):
//	eng := hitl.New(applyEscalation, cloneState)
//	rec := evidenceladder.EscalationRecord{...}
//	pv  := eng.Preview(store.Snapshot(), rec)
//	// … human approves …
//	summary, err := eng.Apply(store.Live(), rec)
//
// Foundations must not import kits; only the record types and the Escalate
// interface live here.
//
// # Concurrency
//
// Tier1, Tier2, and Tier3 interfaces may be implemented by concurrent types;
// the Ladder itself calls them sequentially per-concern and carries no shared
// mutable state between concerns. The Ladder is safe for concurrent use across
// independent concerns when all injected tiers are.
package evidenceladder

import (
	"context"
	"fmt"
	"strings"
	"time"

	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	evidencesig "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-signature"
)

// Signal is a Tier-1 cache entry: past findings keyed to a concern ID.
// Signals are hints only — they reduce Tier-2 cost but never replace it.
// A Signal does NOT satisfy PreconditionSatisfied; only Tier-2 exhaustive
// search with a complete CoverageManifest can do that.
type Signal struct {
	// ConcernID is the concern this signal was observed for.
	ConcernID string `json:"concern_id"`
	// Refs are file:// or source:// references from previous findings.
	// May be stale; always re-verify via Tier-2 before asserting a verdict.
	Refs []string `json:"refs,omitempty"`
	// ContentHash is the hash of the content at the time of the hit.
	// Used to invalidate absence-shaped cache entries on content change.
	ContentHash string `json:"content_hash,omitempty"`
}

// Findings is the output of a Tier-2 Search call.
type Findings struct {
	// Items are the individual search results.
	Items []evidencesig.Finding `json:"items"`
}

// EscalationRecord is the dossier sent to a human reviewer via Tier-3.
// It is proposed through kits/hitl's propose→preview→approve engine by the
// consumer; the kit-level wiring is the consumer's responsibility.
//
// CUE mirror: #EscalationRecord in schema.cue.
type EscalationRecord struct {
	// ConcernID identifies the concern that triggered escalation.
	ConcernID string `json:"concern_id"`
	// SignatureID references the ConcernSignature that triggered escalation.
	SignatureID string `json:"signature_id"`
	// ManifestTreeHash ties the escalation to a specific tree snapshot.
	ManifestTreeHash string `json:"manifest_tree_hash"`
	// Candidates are the (potentially weak/non-decisive) findings from Tier-2.
	Candidates []string `json:"candidates,omitempty"`
	// Question is the human-readable query for the SME.
	Question string `json:"question"`
}

// Validate returns an error if the EscalationRecord is malformed.
func (r EscalationRecord) Validate() error {
	if r.ConcernID == "" {
		return fmt.Errorf("evidenceladder: EscalationRecord.ConcernID must not be empty")
	}
	if r.SignatureID == "" {
		return fmt.Errorf("evidenceladder: EscalationRecord.SignatureID must not be empty")
	}
	if r.ManifestTreeHash == "" {
		return fmt.Errorf("evidenceladder: EscalationRecord.ManifestTreeHash must not be empty")
	}
	if r.Question == "" {
		return fmt.Errorf("evidenceladder: EscalationRecord.Question must not be empty")
	}
	return nil
}

// HumanVerdict is the structured response from a subject-matter expert via Tier-3.
//
// Feedback is mandatory. An empty Feedback means the signature is not improved,
// so the same escalation will recur on every subsequent ladder run — a silent
// infinite loop. Consumers must reject HumanVerdict with empty Feedback.
//
// CUE mirror: #HumanVerdict in schema.cue.
type HumanVerdict struct {
	// Who identifies the SME who provided the verdict.
	Who string `json:"who"`
	// When is the RFC3339 timestamp of the verdict.
	When string `json:"when"`
	// GradeTier is the epistemic tier the SME assigns to their finding.
	GradeTier evidencegrade.Tier `json:"grade_tier"`
	// Rationale is the SME's explanation of their verdict.
	Rationale string `json:"rationale"`
	// Feedback describes how the signature or retrieval should be amended.
	// Mandatory: an empty Feedback means the escalation will recur forever.
	Feedback string `json:"feedback"`
}

// Validate returns an error if the HumanVerdict is malformed.
func (v HumanVerdict) Validate() error {
	if v.Who == "" {
		return fmt.Errorf("evidenceladder: HumanVerdict.Who must not be empty")
	}
	if v.When == "" {
		return fmt.Errorf("evidenceladder: HumanVerdict.When must not be empty")
	}
	if err := v.GradeTier.Validate(); err != nil {
		return fmt.Errorf("evidenceladder: HumanVerdict.GradeTier: %w", err)
	}
	if v.Rationale == "" {
		return fmt.Errorf("evidenceladder: HumanVerdict.Rationale must not be empty")
	}
	if v.Feedback == "" {
		return fmt.Errorf("evidenceladder: HumanVerdict.Feedback must not be empty (empty feedback means the escalation will recur forever — this is a bug in the SME workflow)")
	}
	return nil
}

// Verdict classifies the ladder outcome for a single concern.
//
// CUE mirror: #Verdict in schema.cue.
type Verdict string

const (
	// VerdictFound: decisive findings exist.
	VerdictFound Verdict = "found"

	// VerdictAbsentProven: zero findings, manifest is complete (earned ABSENT_PROVEN).
	// Scoped: "absent within declared scope at snapshot <hash>".
	// Requires CoverageManifest.Complete() == true.
	VerdictAbsentProven Verdict = "absent_proven"

	// VerdictAbsentUnverified: zero findings but manifest has unexplained skips.
	// Fail-closed: the absence cannot be asserted without complete coverage.
	VerdictAbsentUnverified Verdict = "absent_unverified"

	// VerdictUnresolved: ladder exhausted or Tier-3 timed out. No conclusion.
	// Fail-closed: Tier-3 timeout always produces UNRESOLVED, never absent.
	VerdictUnresolved Verdict = "unresolved"
)

// LadderResult is the output of one complete ladder run for one concern.
//
// CUE mirror: #LadderResult in schema.cue.
type LadderResult struct {
	// ConcernID identifies the concern.
	ConcernID string `json:"concern_id"`
	// Verdict is the ladder's conclusion.
	Verdict Verdict `json:"verdict"`
	// Refs contains file:// or source:// references when Verdict==VerdictFound.
	Refs []string `json:"refs,omitempty"`
	// ScopeHash is the CoverageManifest.TreeHash tying the result to a snapshot.
	ScopeHash string `json:"scope_hash,omitempty"`
	// Reason explains non-found verdicts.
	Reason string `json:"reason,omitempty"`
}

// Tier1 is the fast cache tier. It returns past signals keyed to a concern ID.
//
// Contract:
//   - Returns (nil, nil) on cache miss — this is a cache miss, NOT a conclusion.
//   - Never returns a verdict. Signals are hints only.
//   - The fleet's sextant/fathom index is a downstream adapter of this interface;
//     this package never imports it (layering: foundations do not import fleet tools).
type Tier1 interface {
	Lookup(ctx context.Context, concernID string) ([]Signal, error)
}

// Tier2 is the exhaustive search tier. It performs a deterministic, reproducible
// search over the declared scope and returns findings plus an explicit
// CoverageManifest accounting for every file in scope.
//
// Contract:
//   - The CoverageManifest must account for every file: searched + all skip categories.
//   - A complete manifest (Complete()==true) enables ABSENT_PROVEN when zero findings.
//   - An incomplete manifest forces ABSENT_UNVERIFIED (fail-closed).
type Tier2 interface {
	Search(ctx context.Context, sig evidencesig.ConcernSignature) (Findings, evidencesig.CoverageManifest, error)
}

// Tier3 is the human escalation tier. It proposes an EscalationRecord to a
// human reviewer and blocks until a HumanVerdict is received or ctx expires.
//
// Contract:
//   - Timeout is fail-closed to UNRESOLVED — never to absent. If the context
//     expires before a verdict arrives, the caller must treat the result as
//     UNRESOLVED.
//   - HumanVerdict.Feedback is mandatory; consumers must validate it.
//   - The propose→preview→approve→apply wiring (kits/hitl) is the caller's
//     responsibility; this interface only defines the contract.
type Tier3 interface {
	Escalate(ctx context.Context, rec EscalationRecord) (HumanVerdict, error)
}

// GapRecord records a Tier-2 → Tier-3 escalation event for the extraction-gap-log.
// See package extractiongaplog for persistence and projection.
type GapRecord struct {
	// ConcernID identifies the concern that triggered the gap.
	ConcernID string `json:"concern_id"`
	// Tier1KeyMissed is the cache key that was absent from Tier-1.
	Tier1KeyMissed string `json:"tier1_key_missed,omitempty"`
	// ScopeSearched describes the scope Tier-2 searched.
	ScopeSearched string `json:"scope_searched"`
	// Outcome is the verdict after Tier-3 (or UNRESOLVED on timeout).
	Outcome Verdict `json:"outcome"`
	// FoundRefs are the references returned, if verdict was found.
	FoundRefs []string `json:"found_refs,omitempty"`
	// OccurredAt is the injected timestamp of the gap event.
	OccurredAt time.Time `json:"occurred_at"`
}

// Ladder orchestrates the three-tier progression for a single concern.
// Call Run to execute the full ladder.
//
// Concurrency: Ladder is safe for concurrent use across independent concerns.
// Each Run call uses only local state; no shared mutable state.
type Ladder struct {
	tier1 Tier1
	tier2 Tier2
	tier3 Tier3
	now   func() time.Time
	// onGap is called when a GapRecord is emitted (Tier-2 → Tier-3 escalation).
	// May be nil (no logging).
	onGap func(GapRecord)
}

// LadderOption configures a Ladder.
type LadderOption func(*Ladder)

// WithGapLogger supplies a callback invoked each time a GapRecord is emitted.
// Use this to connect to extractiongaplog.
func WithGapLogger(fn func(GapRecord)) LadderOption {
	return func(l *Ladder) { l.onGap = fn }
}

// WithClock injects a clock for GapRecord timestamps. Defaults to time.Now.
func WithClock(now func() time.Time) LadderOption {
	return func(l *Ladder) { l.now = now }
}

// New builds a Ladder. tier2 is required; tier1 and tier3 may be nil (nil
// tier1 means always-miss; nil tier3 means Tier-3 always returns UNRESOLVED).
func New(tier1 Tier1, tier2 Tier2, tier3 Tier3, opts ...LadderOption) *Ladder {
	l := &Ladder{
		tier1: tier1,
		tier2: tier2,
		tier3: tier3,
		now:   time.Now,
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

// Run executes the full three-tier ladder for sig and returns a LadderResult.
//
// Progression:
//  1. Tier-1 Lookup (cache). Empty result = miss → proceed to Tier-2.
//     Non-empty = signal only; still run Tier-2 to verify.
//  2. Tier-2 Search (exhaustive). Produces Findings + CoverageManifest.
//     - PreconditionSatisfied → verdict (found/absent_proven/absent_unverified).
//     - Not satisfied → emit GapRecord → proceed to Tier-3.
//  3. Tier-3 Escalate (human). Timeout → UNRESOLVED (fail-closed, never absent).
func (l *Ladder) Run(ctx context.Context, sig evidencesig.ConcernSignature) LadderResult {
	// Validate the signature first.
	if err := sig.Validate(); err != nil {
		return LadderResult{
			ConcernID: sig.ID,
			Verdict:   VerdictUnresolved,
			Reason:    fmt.Sprintf("invalid signature: %v", err),
		}
	}

	// Tier-1: cache lookup. Result is signal only, never a conclusion.
	// (nil tier1 or empty result both mean cache miss — proceed to Tier-2.)
	if l.tier1 != nil {
		_, _ = l.tier1.Lookup(ctx, sig.ID)
		// We deliberately discard Tier-1 results here: they are hints for
		// Tier-2 tuning only (passed in retriever_hints or similar). Tier-1
		// signals do NOT satisfy PreconditionSatisfied.
	}

	// Tier-2: exhaustive search.
	if l.tier2 == nil {
		return LadderResult{
			ConcernID: sig.ID,
			Verdict:   VerdictUnresolved,
			Reason:    "no Tier-2 engine configured",
		}
	}
	findings, manifest, err := l.tier2.Search(ctx, sig)
	if err != nil {
		return LadderResult{
			ConcernID: sig.ID,
			Verdict:   VerdictUnresolved,
			Reason:    fmt.Sprintf("Tier-2 search error: %v", err),
		}
	}

	// Evaluate the verdict from Tier-2 results.
	verdict, refs, reason := evaluateVerdict(sig, findings, manifest)
	if verdict != VerdictUnresolved {
		return LadderResult{
			ConcernID: sig.ID,
			Verdict:   verdict,
			Refs:      refs,
			ScopeHash: manifest.TreeHash,
			Reason:    reason,
		}
	}

	// Tier-2 did not satisfy PreconditionSatisfied → escalate to Tier-3.
	// Emit a GapRecord before escalation.
	gap := GapRecord{
		ConcernID:     sig.ID,
		ScopeSearched: scopeDescription(sig),
		Outcome:       VerdictUnresolved,
		OccurredAt:    l.now(),
	}
	if l.onGap != nil {
		l.onGap(gap)
	}

	if l.tier3 == nil {
		return LadderResult{
			ConcernID: sig.ID,
			Verdict:   VerdictUnresolved,
			Reason:    "Tier-2 did not satisfy precondition; no Tier-3 engine configured",
		}
	}

	// Build escalation record.
	var candidateRefs []string
	for _, f := range findings.Items {
		candidateRefs = append(candidateRefs, fmt.Sprintf("file://%s#L%d-L%d", f.Path, f.LineStart, f.LineEnd))
	}
	escRec := EscalationRecord{
		ConcernID:        sig.ID,
		SignatureID:      sig.ID,
		ManifestTreeHash: manifest.TreeHash,
		Candidates:       candidateRefs,
		Question:         buildQuestion(sig, findings, manifest),
	}

	hv, err := l.tier3.Escalate(ctx, escRec)
	if err != nil {
		// Timeout or cancellation → UNRESOLVED (fail-closed, never absent).
		return LadderResult{
			ConcernID: sig.ID,
			Verdict:   VerdictUnresolved,
			Reason:    fmt.Sprintf("Tier-3 timeout or error (fail-closed): %v", err),
		}
	}

	// A valid HumanVerdict with a finding → FOUND.
	if err := hv.Validate(); err != nil {
		return LadderResult{
			ConcernID: sig.ID,
			Verdict:   VerdictUnresolved,
			Reason:    fmt.Sprintf("Tier-3 returned invalid HumanVerdict: %v", err),
		}
	}
	return LadderResult{
		ConcernID: sig.ID,
		Verdict:   VerdictFound,
		Refs:      candidateRefs,
		ScopeHash: manifest.TreeHash,
		Reason:    fmt.Sprintf("Tier-3 human verdict: %s (grade: %s)", hv.Rationale, hv.GradeTier),
	}
}

// evaluateVerdict converts Tier-2 output into a Verdict.
// Returns VerdictUnresolved when PreconditionSatisfied is false (escalate to Tier-3).
func evaluateVerdict(sig evidencesig.ConcernSignature, findings Findings, manifest evidencesig.CoverageManifest) (Verdict, []string, string) {
	if evidencesig.PreconditionSatisfied(sig, findings.Items, manifest) {
		switch sig.Archetype {
		case evidencesig.ArchetypeAbsence:
			return VerdictAbsentProven, nil,
				fmt.Sprintf("absent within declared scope at snapshot %q", manifest.TreeHash)
		default:
			refs := make([]string, 0, len(findings.Items))
			for _, f := range findings.Items {
				if f.Decisive {
					refs = append(refs, fmt.Sprintf("file://%s#L%d-L%d", f.Path, f.LineStart, f.LineEnd))
				}
			}
			return VerdictFound, refs, ""
		}
	}

	// Not satisfied. Check if this is an absence arch with incomplete manifest.
	if sig.Archetype == evidencesig.ArchetypeAbsence && !manifest.Complete() {
		unexplained := manifest.Skipped.UnexplainedSkips()
		return VerdictAbsentUnverified, nil,
			fmt.Sprintf("manifest has %d unexplained skip(s): %v", len(unexplained), unexplained)
	}

	// General non-satisfaction → escalate.
	return VerdictUnresolved, nil, "Tier-2 did not satisfy precondition; escalating to Tier-3"
}

// scopeDescription returns a human-readable description of the sig's scope.
func scopeDescription(sig evidencesig.ConcernSignature) string {
	if sig.Scope.IsGlobBased() {
		return strings.Join(sig.Scope.Globs, ",")
	}
	return strings.Join(sig.Scope.SourceIDs, ",")
}

// buildQuestion constructs the escalation question for the SME.
func buildQuestion(sig evidencesig.ConcernSignature, findings Findings, manifest evidencesig.CoverageManifest) string {
	return fmt.Sprintf(
		"Concern %q (archetype: %s): Tier-2 searched %q (tree: %s) and found %d candidates. "+
			"Please confirm whether the concern is satisfied and provide feedback to improve the signature.",
		sig.ID, sig.Archetype, scopeDescription(sig), manifest.TreeHash, len(findings.Items),
	)
}
