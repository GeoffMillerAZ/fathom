package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// ClaimLifecycle (GL-13, docs/plans/governance-learning.md; design doc
// docs/design/governance-learning.md §6 "Rules that learn") is the
// promotion/retirement axis — ORTHOGONAL to ClaimStatus. Status judges a
// claim's freshness/veracity (is the citation still there, did a human
// reject it); Lifecycle judges whether a rule has earned enough
// corroborating evidence to be treated as established, independent of
// that. A claim can be ClaimFresh and LifecycleCandidate at once (an
// unproven-but-current rule), or ClaimStale and LifecycleActive (an
// established rule whose citation drifted).
//
// Owner decision (attempt 2, worker w-gl-w2-rest): this is an ADDITIVE
// axis, never overloaded onto ClaimStatus — see internal/semantic's
// applyLearning (GL-13) for the promotion/retirement rule that transitions
// it, and cmd/fathom's learningAnnouncements (GL-14) for how a transition
// is announced.
type ClaimLifecycle string

const (
	// LifecycleCandidate — a compiled claim or finding-derived rule that has
	// not yet earned enough corroboration to be treated as established.
	// Nothing in this codebase mints a claim at this state yet (v1: every
	// compiled claim is minted active, no candidate gate) — the state
	// exists so a future producer (a finding-derived rule) has somewhere to
	// start, and applyLearning already knows how to promote out of it.
	LifecycleCandidate ClaimLifecycle = "candidate"
	// LifecycleActive — the default: an established rule, served normally
	// (subject to ClaimStatus.Served() as always). Every claim minted
	// before this field existed reads Lifecycle=="" from the store, which
	// EffectiveLifecycle treats as LifecycleActive — the "default active"
	// owner decision, and the same empty-means-already-serving convention
	// ValidFrom/AssertedBy/Meta each established for their own additive
	// migration.
	LifecycleActive ClaimLifecycle = "active"
	// LifecycleRetired — a validity-window-closed rule (SetClaimValidTo is
	// always called alongside this transition — GL-13's own "retirement
	// closes the validity window, never deletes"). Terminal: once retired,
	// a claim never transitions again (applyLearning never reads a retired
	// claim's signals again, and internal/store's PutClaims durability
	// guard, mirroring the existing ClaimRefuted status guard, refuses to
	// let a recompile's default write silently resurrect it).
	LifecycleRetired ClaimLifecycle = "retired"
)

// Valid reports whether l is a known ClaimLifecycle, INCLUDING "" (a
// legacy/unset claim is a valid state, not a malformed one — see
// EffectiveLifecycle).
func (l ClaimLifecycle) Valid() bool {
	switch l {
	case "", LifecycleCandidate, LifecycleActive, LifecycleRetired:
		return true
	}
	return false
}

// EffectiveLifecycle returns l, or LifecycleActive when l is "" — the
// additive-migration default every reader must apply instead of treating
// an empty column as "unknown" or "candidate" (design doc §6 owner
// decision: "default active").
func (l ClaimLifecycle) EffectiveLifecycle() ClaimLifecycle {
	if l == "" {
		return LifecycleActive
	}
	return l
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

	// Lifecycle (GL-13, design doc §6) is the promotion/retirement axis —
	// see ClaimLifecycle's own doc comment for why it is additive and
	// orthogonal to Status rather than a new Status value. "" for every
	// claim written before this field existed (additive migration, same
	// empty-string store column convention as ValidFrom/AssertedBy/Meta
	// below) — read it through EffectiveLifecycle(), never compared to
	// LifecycleActive directly, so a legacy claim reads as already active.
	Lifecycle ClaimLifecycle

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

	// ValidFrom/ValidTo (SU-19, ADR-041 decision 7, docs/vision/31-substrate.md
	// §3 D6) are the claim's bi-temporal VALIDITY window — "was this rule
	// true then", answered independently of Status, which names only the
	// CURRENT verdict. ValidFrom is stamped once, at mint (compiled_at for a
	// compiled claim, the import/observation time for an imported or
	// human-authored one — see each writer's own ValidFrom assignment) and
	// never moves again: internal/store's PutClaims upsert preserves the
	// FIRST value it ever saw for a claim_id, exactly like a legacy column's
	// additive-migration default, so a recompile of unchanged content never
	// drifts a claim's own history forward. ValidTo is the zero time while
	// the claim is a live assertion and is set to the event time by every
	// path that ends a claim's validity (a native review/compile
	// reconciliation superseding it, an external closure/revocation, a human
	// `fathom claim refute`) — see SetClaimValidTo. Both are the zero
	// time.Time{} for a claim written before this field existed (additive
	// migration, empty-string store column) — ValidAt treats a zero
	// ValidFrom/ValidTo as an unconstrained bound in that direction, never a
	// false "never valid"/"already closed".
	ValidFrom time.Time
	ValidTo   time.Time

	// Meta (vision 33 / ADR-043 stage 1, TC-6) is an additive, opaque JSON
	// provenance blob for a claim whose facts don't fit SemanticClaim's own
	// typed fields — the certifications layer's own {test, suite, class,
	// strength, state, run_digest, attempts, coverage_pct?, mutation_score?}
	// block (design doc §3) is the first user. nil/empty for every claim
	// written before this field existed (additive migration, empty-string
	// store column, same convention as every earlier additive claim column
	// — see ValidFrom/AssertedBy's own doc comments) and for every claim
	// that carries no such provenance. A reader unmarshal-per-layer: this
	// package places no schema on the bytes beyond "valid JSON or empty".
	Meta json.RawMessage

	// AssertedBy (SU-22, ADR-041 decision 7) is the PROV-style link to the
	// activity that minted this claim: a compile run id, a review
	// idempotency key or run id, an import cursor "producer@version", or a
	// hook session id for a human `fathom claim add` — built with
	// core.AssertedBy(kind, id) so every writer's value has the same
	// "kind:id" shape. "" for a claim written before this field existed, or
	// whose writer had no identity to record.
	AssertedBy string
}

// ValidAt reports whether c's bi-temporal validity window contains at (SU-19,
// ADR-041 decision 7): the SAME question "was this rule true then" that
// ObligationsAsOf answers. A zero ValidFrom never rejects on the lower
// bound (the claim's own mint time is unknown or predates this field); a
// zero ValidTo means still open (never rejects on the upper bound) — the
// same "empty means unconstrained" convention every other additive column
// in this store uses for a pre-existing row.
func (c SemanticClaim) ValidAt(at time.Time) bool {
	if !c.ValidFrom.IsZero() && at.Before(c.ValidFrom) {
		return false
	}
	if !c.ValidTo.IsZero() && !at.Before(c.ValidTo) {
		return false
	}
	return true
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
	// RelSupersedes (SU-20, ADR-041 decision 7) is a link FROM the successor
	// claim TO the predecessor it replaces: ClaimID is the successor,
	// TargetRef is the predecessor's claim id, AnchorText is the human- or
	// machine-readable reason. Written by semantic.WriteSupersedes and
	// walked by semantic.SupersessionChain. TargetHash is always empty — a
	// claim id is not a citation to re-hash.
	RelSupersedes = "supersedes"
	// RelAnchors (SU-21, ADR-041 decision 7) binds a claim's citation to the
	// single enclosing symbol definition it resolved against at mint:
	// TargetRef is the symbol's core.SymbolKey, TargetHash is the symbol's
	// own current content fingerprint (core.HashLines over its declared
	// range). Written by semantic.AnchorSymbol; read by semantic.Sweep (to
	// re-anchor a drifted citation onto the symbol's new range instead of
	// flipping stale) and by rename_impact (to name claims anchored to a
	// renamed symbol).
	RelAnchors = "anchors"
	// RelAudience (GL-4, GL-5; docs/design/governance-learning.md §3)
	// narrows WHO a claim is served to: TargetRef is AudienceManual (a
	// rules file its author marked on-request only — Kiro's
	// `inclusion: manual`, a Cursor rule with neither globs nor
	// alwaysApply) or AudienceReview (a BUGBOT.md review rule, served to
	// review verbs, not the edit hook). A claim with no such link is
	// served everywhere, exactly as before this rel existed. Written at
	// compile from the rules file's own declared shape; TargetHash is
	// always empty.
	RelAudience = "audience"
)

// Audience values carried by a RelAudience link's TargetRef.
const (
	AudienceManual = "manual"
	AudienceReview = "review"
)

// AssertedBy builds the canonical "kind:id" form every claim/event writer
// records into AssertedBy (SU-22, ADR-041 decision 7) — a compile run id
// ("compile:<run_id>"), a review idempotency key or run id
// ("review:<key>"), an import cursor ("import:<producer>@<version>"), or a
// hook session id for a human `fathom claim add` ("claim-add:<session>").
// One builder so every writer's value has the identical shape a later
// reader can split on the first ":" without re-deriving the convention.
func AssertedBy(kind, id string) string {
	return kind + ":" + id
}

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

// Why a section stayed uncompiled (CompileRun.UncompiledReasons, EW-35).
// "corpus_grew" and "stale" are serve-time reasons the live coverage
// derives itself for sections no run recorded a reason for.
const (
	UncompiledFailed      = "failed"      // the model's answer never arrived usable (or its per-call deadline hit)
	UncompiledPrefiltered = "prefiltered" // the decision port's pre-filter skipped it
	UncompiledRefused     = "refused"     // the run's pre-flight budget refused the whole run
	UncompiledBudget      = "budget"      // the mid-run budget stop never reached it
	UncompiledCancelled   = "cancelled"   // the run aborted before it: the caller's context ended, or a drift-anchor store failure (FT-6)
	UncompiledCorpusGrew  = "corpus_grew" // added to the corpus after the last run
	UncompiledStale       = "stale"       // compiled once, its content or prompt changed since
)

// CompileRun is the per-run cost/audit ledger row (real usage, never
// heuristics — ADR-016 §4.1).
type CompileRun struct {
	RunID           string // sha256 of (service, layer, corpus watermark, started-at)
	ServiceID       string
	Layer           string
	CorpusWatermark string
	Model           string
	PromptVersion   string
	DryRun          bool
	TokensIn        int
	TokensOut       int

	// Dry-run estimate band (O8, plan IO-8): output tokens are unknowable
	// before a run — a dry run carries this honest RANGE instead of the
	// confident zero that once preceded a $0.64, 21-minute compile. Zero on
	// real runs; not persisted (compile_runs has no columns for these).
	EstTokensOutLow  int     `json:"est_tokens_out_low,omitempty"`
	EstTokensOutHigh int     `json:"est_tokens_out_high,omitempty"`
	EstCostLowUSD    float64 `json:"est_cost_low_usd,omitempty"`
	EstCostHighUSD   float64 `json:"est_cost_high_usd,omitempty"`
	// EstOutRatioLow/High and EstOutBasis (EW-74, G8) say where the output
	// band came from: the out/in token ratio bracket the estimate applied,
	// and either the service's own completed-run history (a sentence naming
	// the run count and layer) or the documented default band. Dry-run
	// only; not persisted.
	EstOutRatioLow  float64 `json:"est_out_ratio_low,omitempty"`
	EstOutRatioHigh float64 `json:"est_out_ratio_high,omitempty"`
	EstOutBasis     string  `json:"est_out_basis,omitempty"`
	// EstSectionMS is the service's past wall time per compiled section
	// (EW-73, G7), from its completed compile_runs; 0 when it has none.
	// A spending compile through the daemon sizes its client timeout from
	// it. Dry-run only; not persisted.
	EstSectionMS int64 `json:"est_section_ms,omitempty"`
	// PriceKnown is DryRun-only (meaningless on a completed run, which
	// always knows its own real CostUSD): true when the caller resolved a
	// real per-model USD rate before estimating, so EstCostLowUSD/
	// EstCostHighUSD are a genuine computed range; false means no price is
	// known for the model that would run (an unrecognized --profile pin, an
	// unpriced FATHOM_COMPILE_MODEL) — EstCostLowUSD/EstCostHighUSD then
	// stay at their zero value, and a caller must never print that zero as
	// a confident "$0.00" estimate (the exact failure this field exists to
	// prevent one level up from O8: a dry run reporting cost_usd
	// "est. 0.00-0.00" for a model it simply never priced, even with a
	// profile explicitly given). Set by the caller's own price resolution
	// (cmd/fathom's rateForModel), never computed inside internal/semantic;
	// not persisted (compile_runs has no column for it, like every other
	// Est* field above).
	PriceKnown       bool `json:"price_known"`
	CostUSD          float64
	DurationMS       int64
	SectionsTotal    int // sections in the corpus
	SectionsCompiled int // sections that missed the digest gate (spent tokens)
	SectionsFailed   int // sections skipped because the model's answer never arrived usable (after one retry)
	// SectionsCarried and ClaimsCarried (AG-26, docs/plans/agents-md.md)
	// count sections that kept the claims of a section compiled at another
	// path (same content digest and prompt identity, a one-to-one move),
	// re-cited to the new path with no model call, and the claims they
	// kept. Neither is in SectionsCompiled or ClaimsEmitted. Not persisted
	// (compile_runs has no columns for them, like the Est* fields).
	SectionsCarried int `json:"sections_carried,omitempty"`
	ClaimsCarried   int `json:"claims_carried,omitempty"`
	// SectionErrors names the first failed sections ("subject: error"),
	// capped, so a partial compile says what it left out.
	SectionErrors  []string `json:"section_errors,omitempty"`
	ClaimsEmitted  int
	ClaimsRejected int // failed compile-time anchor/scope verification
	RanAt          time.Time

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

	// ResumedFrom is O54 deliverable 3 (`fathom compile --layer obligations
	// --continue`): the subject ("<doc>#<slug>") of the first
	// not-yet-compiled section in compile order this run picked up from,
	// set only when the caller asked to resume (semantic.CompileInput.
	// Resume). Empty on every normal run, and empty on a resume request
	// whose corpus was already fully compiled — proof the run actually
	// continued rather than restarted, not a claim.
	ResumedFrom string
	// UncompiledScopes is O54 deliverable 2's layer_coverage.
	// uncompiled_scopes source: the sorted, deduplicated document paths
	// this run's compile order (internal/semantic's
	// orderSectionsByGoverningScope) never reached — the tail after a
	// BudgetStopped break, nil when the run walked its whole ordered
	// corpus (nothing left uncompiled by THIS run). A caller reads this
	// off the LATEST compile_runs row for (service, layer), not off
	// whether any given section has a claim — a section skipped by the
	// digest gate already has one from an earlier run and is never in
	// this tail.
	UncompiledScopes []string `json:"uncompiled_scopes,omitempty"`

	// SectionsPrefiltered (ADR-042 decision 2, vision 32 §2 item 1) counts
	// ordered sections the decision port's PrefilterSections skipped this
	// run — asked one batched noul ("does this section state a binding
	// rule") and scored below Compiler.PrefilterThreshold, or unanswered.
	// Zero when Compiler.Decider is nil (the default: no wave wires one
	// without operator config) or every section passed. A prefiltered
	// section is counted here, never in SectionsCompiled or SectionsFailed,
	// and its subject is also folded into UncompiledScopes' document set —
	// ADR-037's raw fallback must still be able to serve it.
	SectionsPrefiltered int `json:"sections_prefiltered,omitempty"`
	// UncompiledReasons (EW-35) names WHY each section this run left
	// uncompiled is uncompiled: section subject ("<doc>#<slug>") to one of
	// the Uncompiled* reason constants. Populated on a real run only;
	// persisted as one added compile_runs column (never a table), nil on a
	// legacy row.
	UncompiledReasons map[string]string `json:"uncompiled_reasons,omitempty"`
	// PrefilteredSections names, by stable section id ("<path>#<slug>"),
	// every section SectionsPrefiltered counts — the same identity
	// Section.Subject and a compiled claim's own Subject use, so a caller
	// can cross-reference a specific skipped section without re-deriving
	// it from UncompiledScopes' coarser document-path granularity.
	PrefilteredSections []string `json:"prefiltered_sections,omitempty"`
	// Decisions is every decision the decision port produced during this
	// run (currently: PrefilterSections' "compile_prefilter" site only) —
	// an in-memory disclosure for the caller to log, not persisted to
	// compile_runs (ephemeral ranking is instrument truth, not durable
	// evidence — ADR-042's "Considered and rejected"). Nil when
	// Compiler.Decider is nil or every question in every chunk answered
	// invalid.
	Decisions []DecisionRecord `json:"decisions,omitempty"`
	// PrefilterError (DP-22, O109) is set only when the compile pre-filter's
	// own decision-port call hard-failed (anything but
	// decision.ErrNoProvider) — this run aborted with an error and no
	// claims were compiled, but the failed call itself is still named here
	// so the caller can log it, instead of it vanishing along with the
	// aborted run. nil on every successful run, and nil when Compiler.
	// Decider is nil (the pre-filter never called out at all).
	PrefilterError *DecisionError `json:"-"`
}

// DecisionError is internal/core's storage-friendly, dependency-free mirror
// of internal/semantic.DecisionError (DP-22, O109) — same discipline as
// DecisionRecord's own doc comment: a LOCAL type, never carrying the kit's
// own error value, so internal/core stays buildable in isolation. Err is
// the underlying error's message, never a secret (the kit's own adapters
// already keep API keys and raw provider bodies out of their returned
// errors).
type DecisionError struct {
	Site      string
	Provider  string
	Questions int
	LatencyMS int64
	Err       string
}

// DecisionRecord is internal/core's storage-friendly mirror of agentic-kit
// foundations/decision.Record — same fields, deliberately a LOCAL type
// rather than an import of that package: internal/core is built, in
// isolation and without fathom's own go.mod replace directives, by
// `fathom rules gen-test`'s generated temp modules (cmd/fathom/rules.go),
// so a third-party dependency here breaks their offline build. The
// semantic package (internal/semantic/compile.go) converts a real
// decision.Record into this shape when it populates CompileRun.Decisions.
type DecisionRecord struct {
	Site         string
	Provider     string
	Model        string
	QuestionID   string
	Kind         string
	Choice       string
	Confidence   float64
	Score        float64
	Noul         float64
	LatencyMS    int64
	InputTokens  int
	OutputTokens int
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
	// "re-anchored" | "superseded" | "annotated". "refuted" and "contradicted" name DIFFERENT judgments
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
	//
	// A FOURTH and FIFTH case (vision 30 §3.3 item 3, ADR-040 decision 2,
	// plan FL-3/FL-6) are the findings-loop's own: ResolutionExternalClosure
	// ("external-closure") when an external producer's own observation/pass
	// or closure event superseded the claim, and ResolutionRevoked
	// ("revoked") when a revocation event did. Both are set by
	// internal/findings/external, never by internal/analysis/review's
	// ClassifyResolution — an external producer's own verdict, projected,
	// never a fathom-derived one.
	ResolutionKind   string
	ResolutionCommit string

	// AssertedBy (SU-22, ADR-041 decision 7) is the same "kind:id" identity
	// (core.AssertedBy) as SemanticClaim.AssertedBy, but for the ACTIVITY
	// that recorded THIS event specifically — a sweep's own re-anchor/
	// contradicted events carry "sweep:<claim_id>" (see semantic.Sweep),
	// an import's closure/revocation events carry the SAME
	// "import:<producer>@<version>" identity as the claim they close, and a
	// human `fathom claim refute`/`corroborate` carries
	// "claim-add:<session>" — GL-13/GL-14's owner decision
	// (docs/design/governance-learning.md §6, cmd/fathom's
	// claimVerdictIdentity) resolves <session> to the harness session id
	// when the hook env carries one, else the literal "human:cli", so this
	// is ALWAYS populated for a claim refute/corroborate written after that
	// decision landed — never "" for lack of an identity. "" remains for
	// an event whose writer predates this field.
	AssertedBy string
}

// Resolution outcome kinds (docs/design/findings-as-evidence.md §3.4): the
// ClaimEvent.ResolutionKind values a "superseded" event may carry.
// "refutation" is deliberately NOT a ResolutionKind constant — it is
// recognized by Verdict=="refuted" directly (see ClaimEvent.ResolutionKind's
// doc comment).
const (
	ResolutionCodeChange         = "code-change"
	ResolutionScopeDisappearance = "scope-disappearance"
	// ResolutionExternalClosure (vision 30 §3.3 item 3, FL-3): an external
	// producer's own observation/pass or closure event superseded the claim.
	ResolutionExternalClosure = "external-closure"
	// ResolutionRevoked (vision 30 §3.3 item 3, FL-3): an external
	// producer's revocation event superseded the claim — a revoked source is
	// not a refuted claim; refute stays a human verdict.
	ResolutionRevoked = "revoked"
)

// ValidVerdict reports whether v is a recognized ClaimEvent.Verdict value —
// "refuted" | "corroborated" | "contradicted" | "re-anchored" |
// "superseded" | "annotated" (FL-3: "annotated" is the findings-loop's
// status-preserving verdict for an external annotation event — see
// ClaimEvent.Verdict's doc comment). No caller currently gates writes on
// this (ClaimEvent.Verdict has no existing validation point to hook into);
// it exists so a caller that DOES need to check one has a single, tested
// source of truth instead of hand-rolling the switch.
func ValidVerdict(v string) bool {
	switch v {
	case "refuted", "corroborated", "contradicted", "re-anchored", "superseded", "annotated":
		return true
	}
	return false
}

// ExternalFinding is the per-claim cursor row the findings-loop importer
// (internal/findings/external, plan FL-7) maintains alongside the claim it
// governs: one row per (ServiceID, ClaimID), keyed by the producer's own
// identity fields, recording exactly enough to enforce ordering/idempotency
// on the NEXT import (older SourceVersion → stale; same version, different
// PayloadDigest → conflicting-replay; same version and digest → duplicate)
// and to serve source-linked provenance (vision 30 §4 item 4) — producer,
// rule, source id/version/revision, anchor path/lines/symbol/digest,
// observed/imported timestamps — without re-deriving any of it from the
// claim's own Subject/ReproKey encoding.
type ExternalFinding struct {
	ServiceID string
	ClaimID   string

	Producer        string
	ProducerVersion string

	SourceID       string
	SourceVersion  uint64
	PayloadDigest  string
	SourceRevision string

	RuleID      string
	RuleVersion string
	Remedy      string

	AnchorPath   string
	AnchorStart  int
	AnchorEnd    int
	AnchorSymbol string
	AnchorDigest string

	ObservedAt time.Time
	ImportedAt time.Time

	// Unanchored is true for a process-subject finding carried with no
	// anchor at all (vision 30 §3.1, §4 item 4) — served as process context,
	// never attached to a file.
	Unanchored bool
}

func itoa(n int) string { return strconv.Itoa(n) }

func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}
