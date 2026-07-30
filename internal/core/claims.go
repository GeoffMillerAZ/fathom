package core

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// claims.go — the semantic-layer domain model (ADR-016, ADR-017).
//
// A SemanticClaim is the unit of compiled knowledge: a governed restatement
// of a governing source, anchored by citations (ClaimLink rel="cites") and
// joined to code by scope globs (rel="scopes"). Claims live in additive
// tables BESIDE observations — never inside them: re-ingest purges
// observations destructively per source, and the kind-blind contradiction
// scan would false-positive against them.

// ClaimStatus is the freshness/veracity state of a claim. Claims are never
// deleted by compilation — they are status-flipped, so audit history and
// "that rule went stale" (as opposed to silent absence) are both preserved.
type ClaimStatus string

const (
	// ClaimFresh — citations verified at last sweep; served normally.
	ClaimFresh ClaimStatus = "fresh"
	// ClaimStale — a cited line range changed since compile; served flagged,
	// never silently dropped and never silently trusted.
	ClaimStale ClaimStatus = "stale"
	// ClaimContradicted — a citation no longer resolves at all (file or
	// range gone); served flagged, strongest drift signal short of refutation.
	ClaimContradicted ClaimStatus = "contradicted"
	// ClaimSuperseded — a newer compile of the same subject produced a
	// replacement claim; not served.
	ClaimSuperseded ClaimStatus = "superseded"
	// ClaimRefuted — a human (or machine) verdict rejected the claim via
	// claim_events; not served. Human corroboration never promotes tier —
	// refutation is the only verdict with serving consequences.
	ClaimRefuted ClaimStatus = "refuted"
)

// Valid reports whether s is a known ClaimStatus.
func (s ClaimStatus) Valid() bool {
	switch s {
	case ClaimFresh, ClaimStale, ClaimContradicted, ClaimSuperseded, ClaimRefuted:
		return true
	}
	return false
}

// Served reports whether a claim in this status may appear in a payload
// (flagged or not). Superseded and refuted claims are never served.
func (s ClaimStatus) Served() bool {
	return s == ClaimFresh || s == ClaimStale || s == ClaimContradicted
}

// SemanticClaim is one compiled, governed claim.
type SemanticClaim struct {
	// ClaimID is content-addressed: sha256 over (layer, kind, subject,
	// repro key) — stable across recompiles of unchanged inputs, so every
	// deferred mechanism (lifecycle ledger, feedback, links) can attach to
	// it non-destructively. Compute with ClaimID().
	ClaimID   string
	ServiceID string

	Layer        string   // e.g. "obligations"
	LayerVersion string   // layer definition version, e.g. "v1"
	Kind         string   // v1: "obligation"; later: "finding", "failure-mode", …
	Subject      string   // open ref: the governing source section, e.g. "CLAUDE.md#deterministic-excellence-gate"
	Rule         string   // the claim text served to agents (budgeted at compile)
	Scope        []string // sorted glob(s) over code paths this claim governs

	// Tier is the evidence tier (v1 always TierDocClaimed: the rule text is
	// a citation-grounded restatement; only the scope mapping is inferred).
	Tier EvidenceTier
	// TierProfile names the tier vocabulary this claim's Tier belongs to
	// (ADR-017: data now, machinery at its revival evidence). V1: "engineering".
	TierProfile string

	Status ClaimStatus

	// Provenance (flattened; the full aiprovenance retrofit attaches by ClaimID).
	Model         string // model ID that compiled the claim ("" for deterministic passes)
	PromptVersion string // versioned prompt identity, e.g. "obligations/v1"
	ReproKey      string // provenance repro key: InputDigest is the default
	InputDigest   string // sha256 of the source section the claim was compiled from

	// CompiledAtWatermark is the service corpus watermark at compile time
	// (git HEAD or manifest hash — the honest "as of" every payload carries).
	CompiledAtWatermark string
	CompiledAt          time.Time

	// Class (docs/design/findings-as-evidence.md §3.2, §6 item 2) is the
	// closed-vocabulary failure class (FindingClass) a finding-kind claim
	// carries — assigned by the reviewer at emit time, validated at
	// compile/persist. "" for every non-finding claim, and for a
	// finding-kind claim compiled before this field existed (additive
	// migration — internal/store's conditional column add).
	Class FindingClass
}

// ClaimID computes the content-addressed identity for a claim.
func ClaimID(layer, kind, subject, reproKey string) string {
	h := sha256.Sum256([]byte(layer + "\x00" + kind + "\x00" + subject + "\x00" + reproKey))
	return hex.EncodeToString(h[:])
}

// ClaimLink is a typed edge from a claim to a target. The rel vocabulary is
// open (ADR-016); v1 materializes "cites", "scopes", and (for finding-kind
// claims only, FT-6) "scope-fp" — reserved beyond that: supports,
// contradicts, supersedes.
type ClaimLink struct {
	ClaimID string
	Rel     string // "cites" | "scopes" | "scope-fp" (reserved: supports, contradicts, supersedes)
	// TargetRef: for "cites", "path#Lstart-Lend" into the governing corpus;
	// for "scopes", the code glob; for "scope-fp", a repo-relative path.
	// Always repo-relative (ADR-017).
	TargetRef string
	// TargetHash: for "cites", sha256 of the exact cited lines (the sweep
	// re-hashes these to detect drift). For "scope-fp", the store's content
	// fingerprint (fusion.FingerprintHash) of the scoped file AT COMPILE
	// TIME — the remediation-drift anchor (docs/vision/03-findings-and-
	// experts.md §2, FT-6): a live re-read of the CURRENT fingerprint that
	// disagrees means the scoped code changed since compile. Empty for
	// "scopes".
	TargetHash string
	// AnchorText is, for "cites" links only, the EXACT text of the cited
	// line range at compile time (LinesText's output over the same range
	// HashLines hashed into TargetHash — same normalization, so the two are
	// always consistent). This is O13's fix (docs/dogfood-observations.md,
	// docs/plans/trust-and-tokenomics.md T5-1): a hash alone can only ever
	// detect drift, never heal it — once the cited lines no longer hash-
	// match, there is no way back to "where did this text go?" without the
	// text itself. With AnchorText stored, semantic.Sweep can search the
	// cited file for the rule's exact snippet on drift: a single unambiguous
	// match means an upstream edit merely SHIFTED the lines (e.g. a prose
	// insertion above the rule) and the sweep re-anchors the citation in
	// place instead of flipping the claim stale. Empty for links written
	// before this field existed (legacy hash-only claims) and for every
	// non-"cites" rel — those citations can only ever stale, never heal, and
	// the sweep must treat an empty AnchorText as "cannot attempt re-anchor"
	// rather than searching for an empty string.
	AnchorText string
}

const (
	RelCites   = "cites"
	RelScopes  = "scopes"
	RelScopeFP = "scope-fp"
)

// CiteRef builds a citation TargetRef. Lines are 1-based and inclusive.
func CiteRef(path string, startLine, endLine int) string {
	return path + "#L" + itoa(startLine) + "-L" + itoa(endLine)
}

// ParseCiteRef splits a citation TargetRef into path and 1-based inclusive
// line range. ok is false when ref is not a well-formed citation.
func ParseCiteRef(ref string) (path string, start, end int, ok bool) {
	i := strings.LastIndex(ref, "#L")
	if i < 0 {
		return "", 0, 0, false
	}
	path = ref[:i]
	rangePart := ref[i+2:]
	j := strings.Index(rangePart, "-L")
	if j < 0 {
		return "", 0, 0, false
	}
	start, ok1 := atoi(rangePart[:j])
	end, ok2 := atoi(rangePart[j+2:])
	if !ok1 || !ok2 || path == "" || start < 1 || end < start {
		return "", 0, 0, false
	}
	return path, start, end, true
}

// LinesText returns the exact text of the 1-based inclusive line range of
// content, with line endings normalized (CRLF→LF) and lines joined by "\n" —
// the same extraction HashLines hashes. Exported so callers can persist the
// literal snippet (ClaimLink.AnchorText, T5-1/O13) alongside its hash: the
// hash alone can detect drift but never heal it, since there is no way back
// from a hash to "what did this text used to say" without the text itself.
func LinesText(content []byte, start, end int) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	if start < 1 || end < start || end > len(lines) {
		return "", false
	}
	return strings.Join(lines[start-1:end], "\n"), true
}

// HashLines returns the sha256 of the 1-based inclusive line range of content,
// with lines joined by "\n" (line endings normalized). This is the citation
// hash contract shared by the compiler and the sweep.
func HashLines(content []byte, start, end int) (string, bool) {
	text, ok := LinesText(content, start, end)
	if !ok {
		return "", false
	}
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:]), true
}

// CompileRun is the per-run cost/audit ledger row (real usage, never
// heuristics — ADR-016 §4.1).
type CompileRun struct {
	RunID            string // sha256 of (service, layer, corpus watermark, started-at)
	ServiceID        string
	Layer            string
	CorpusWatermark  string
	Model            string
	PromptVersion    string
	DryRun           bool
	TokensIn         int
	TokensOut        int
	CostUSD          float64
	DurationMS       int64
	SectionsTotal    int // sections in the corpus
	SectionsCompiled int // sections that missed the digest gate (spent tokens)
	SectionsFailed   int // sections skipped for a malformed/truncated LLM response
	ClaimsEmitted    int
	ClaimsRejected   int // failed compile-time anchor/scope verification
	RanAt            time.Time

	// Refused is true when the run's pre-flight cost estimate exceeded the
	// layer's max_usd_per_run budget: the run never started (no LLM calls,
	// no claims written) but the estimate is still ledgered here — refusals
	// are recorded, not silent (ADR-016 §4.1, M1-7 enforceBudgets).
	Refused bool
	// RefusalReason explains Refused or BudgetStopped (empty otherwise).
	RefusalReason string
	// BudgetStopped is true when the run's running token total crossed
	// max_tokens_per_run mid-corpus: the run stopped cleanly after storing
	// whatever it had already compiled. Sections not yet reached are
	// untouched and resume via the digest gate on the next invocation.
	BudgetStopped bool

	// Escape is a free-text audit note recording that this run proceeded
	// only because an explicit CLI override flag bypassed a compile guard
	// that would otherwise have refused it — the idempotency guard's
	// --force (A2-1) or the eval-gate's --unsafe-skip-eval-gate (D3-1), both
	// docs/plans/activation.md / docs/plans/expert-depth.md. Empty for every
	// normal run; non-empty is itself the durable "logged into compile_runs"
	// audit trail the kill criteria for both flags count against.
	Escape string

	// RejectionReasons is O19's fix (docs/dogfood-observations.md O19,
	// docs/design/observability-surfaces.md §8 item 5): before this field
	// existed, a rejected draft's WHY was discarded the moment
	// ClaimsRejected was incremented — only the count survived. Populated
	// only on a real (non-dry) run — verifyDraft never runs during a dry
	// run, so the field stays nil there, never a fabricated empty
	// breakdown. Sorted deterministically by Cause (internal/semantic's
	// buildRejectionReasons).
	RejectionReasons []RejectionBreakdown
}

// RejectionBreakdown is one compile run's rejection-cause bucket (O19's
// fix): a count plus up to a handful of example messages so an author can
// act on the WHY, not just see a number. Cause is one of "malformed",
// "anchor-missing", "scope-unverifiable", "over-broad"
// (internal/semantic's classifyRejectionCause).
type RejectionBreakdown struct {
	Cause   string   `json:"cause"`
	Count   int      `json:"count"`
	Samples []string `json:"samples,omitempty"`
}

// ClaimEvent is one append-only human/machine verdict on a claim (the refute
// channel the truth lens demanded — ADR-016 §4.1).
type ClaimEvent struct {
	ClaimID string
	// Verdict is "refuted" | "corroborated" | "contradicted" |
	// "re-anchored" | "superseded". "refuted" and "contradicted" name DIFFERENT judgments
	// and must never be conflated (P1-9, docs/plans/
	// pulled-tools-excellence.md): "refuted" is a deliberate human (or
	// human-equivalent) verdict via `fathom claim refute` / the obligations
	// tool's feedback action — it pairs with status ClaimRefuted, which
	// Served() excludes (never served again). "contradicted" is
	// semantic.Sweep's machine-detected, anchor-dead verdict — it pairs with
	// status ClaimContradicted, which Served() includes (served WITH the
	// status label, pending a recompile that heals or confirms it).
	// "corroborated" never changes status. "re-anchored" (T5-1/O13,
	// docs/plans/trust-and-tokenomics.md) is machine-authored and
	// sweep-issued like "contradicted", but uniquely status-PRESERVING: it
	// pairs with NO status change at all — it records that a drifted
	// citation's stored AnchorText resolved unambiguously at a new location
	// in the cited file and the sweep moved the citation there
	// (store.UpdateClaimLink), old→new ref in the Note; the claim keeps
	// serving fresh because its evidence was relocated, not weakened.
	// "superseded" (docs/design/observability-surfaces.md §1's Interventions
	// table, phase Q) is machine-authored by `fathom review`'s persist path
	// (recordReviewSupersedeEvents, cmd/fathom/review.go) — it pairs with
	// status ClaimSuperseded on a review-minted finding that is no longer
	// emitted by a later review run (the finding was fixed), recorded
	// alongside the pre-existing SetClaimStatus call so "a finding was
	// raised, then reconciled as fixed" becomes a durable, queryable event
	// instead of only an instantaneous status flip.
	Verdict string
	Author  string
	Note    string
	At      time.Time

	// ResolutionKind and ResolutionCommit (docs/design/findings-as-
	// evidence.md §3.4, §6 item 4) distinguish WHAT resolved a superseded
	// finding — set ONLY on "superseded" events, by
	// internal/analysis/review.ClassifyResolution at the moment
	// recordReviewSupersedeEvents (cmd/fathom/review.go) writes the event:
	//
	//   - ResolutionCodeChange ("code-change"): the finding's scoped file(s)
	//     still exist in the corpus — the underlying code changed and the
	//     issue no longer reproduces. ResolutionCommit names the review
	//     run's watermark (the commit the fix landed by).
	//   - ResolutionScopeDisappearance ("scope-disappearance"): the scoped
	//     file no longer exists at all (deleted) — the finding's subject is
	//     gone, not fixed in place.
	//
	// A THIRD case, refutation ("a human said it was wrong"), needs no new
	// field here: it is already fully recorded by a DIFFERENT event —
	// Verdict=="refuted" (pkg/client.ClaimFeedback's "refute" action) — so
	// resolution-outcome classification (internal/analysis/findings)
	// treats a claim's refuted event as its own, third, self-describing
	// outcome and never expects ResolutionKind on it. Both fields are ""
	// for every event that isn't a classified supersede — additive, never
	// required.
	ResolutionKind   string
	ResolutionCommit string
}

// Resolution outcome kinds (docs/design/findings-as-evidence.md §3.4): the
// two ClaimEvent.ResolutionKind values a "superseded" event may carry.
// "refutation" is deliberately NOT a ResolutionKind constant — it is
// recognized by Verdict=="refuted" directly (see ClaimEvent.ResolutionKind's
// doc comment).
const (
	ResolutionCodeChange         = "code-change"
	ResolutionScopeDisappearance = "scope-disappearance"
)

func itoa(n int) string { return strconv.Itoa(n) }

func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}
