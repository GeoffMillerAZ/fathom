package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// externalImportPromptVersion is internal/findings/external's PromptVersion
// discriminator (mirrored here as a literal — internal/semantic must stay
// free of any internal/findings dependency, the same leaf-package stance
// internal/analysis/review/claims.go documents for its own
// ClaimsPromptVersion mirror). A claim carrying it was imported by the
// findings-loop governed importer, never compiled; obligationsCore joins it
// against store.ExternalFinding for the fields native claims do not have.
const externalImportPromptVersion = "external-import/v1"

// MaxObligations caps a served payload (ADR-016 §4.5: 3–6 rules, ≤150
// tokens; rules are word-budgeted at compile time so the cap is the whole
// serving budget).
const MaxObligations = 6

// nearestTierReserveSlots (O57 item 4) is how many of a capped payload's
// slots go to the nearest-scope tier unconditionally, before role or
// freshness competes for the rest. See the cap block in obligationsCore
// and reserveNearestTier.
const nearestTierReserveSlots = 2

// RawFallbackReserve (wave O1, ADR-037) is how many raw-tier fallback rows
// RawFallbackRules ever returns for one obligations call — a small, FIXED
// budget on top of MaxObligations' compiled-rule cap, never competing with
// it. A raw row only ever ADDS a governing section the compiled layer
// never reached; it never displaces a compiled row, so it does not need
// (and must never consume) a slot out of the shared six-rule cap.
const RawFallbackReserve = 2

// Obligation is one served rule.
type Obligation struct {
	Rule string `json:"rule"`
	// Src is the primary citation, "path#Lstart-Lend" — always attached
	// (bounded blast radius: the agent can check the source in one Read).
	Src  string `json:"src"`
	Tier string `json:"tier"`
	// Kind labels a claim served from a non-obligation layer ("finding", …).
	// Omitted for the obligations layer (FT-2) — existing consumers
	// (cmd/fathom/edithook.go, the obligations MCP tool response) see the
	// exact same payload shape as before this field existed.
	Kind string `json:"kind,omitempty"`
	// Status is present only when not fresh ("stale" / "contradicted") —
	// drifted rules are served flagged, never silently dropped. A claim
	// never reaches this payload with status "refuted" or "superseded" at
	// all: ObligationsFor only walks claims where core.ClaimStatus.Served()
	// is true, and refuted/superseded are the two statuses Served()
	// excludes. This is the vocabulary the served/never-served line runs on
	// (P1-9, docs/plans/pulled-tools-excellence.md; the founding design at
	// docs/design/semantic-layers.md has the full claims-lifecycle
	// picture): "contradicted" is semantic.Sweep's own machine-detected,
	// anchor-dead judgment (a citation no longer resolves) — the strongest
	// drift signal short of a human verdict, and it stays SERVED, flagged
	// with this Status field, because the underlying rule may still be true
	// even though its evidence anchor broke; a recompile is what heals or
	// confirms it. "refuted" is a categorically different, deliberate
	// judgment — a human via `fathom claim refute`, or the obligations
	// tool's feedback action — and a refuted claim is NEVER served again,
	// so it can never appear here with any Status value. The two verdicts
	// share the append-only ClaimEvent audit mechanism (core.ClaimEvent.
	// Verdict) but must never share a verdict STRING; conflating them would
	// make the audit log claim a never-served verdict for a claim callers
	// can still see served right here.
	Status string `json:"status,omitempty"`
	// CodeDrifted flags a finding-kind claim whose SCOPED CODE (not the
	// cited finding document) has changed since compile — remediation drift
	// (docs/vision/03-findings-and-experts.md §2, FT-6). A serving
	// annotation, never a status flip: drifted code can mean the finding was
	// FIXED, not that the claim is wrong. Always absent for the obligations
	// layer — no scope-fp links are ever recorded for it.
	CodeDrifted bool `json:"code_drifted,omitempty"`
	// Layer names the manifest layer this rule was compiled from (e.g.
	// "obligations", "adr-rules", "spec-bindings", "findings") — T5-2/O14
	// (docs/dogfood-observations.md, docs/plans/trust-and-tokenomics.md):
	// once more than one layer blends into the same served set, an agent
	// (or a human debugging an eviction) needs to know WHICH layer each
	// surviving rule came from, not just its Kind (obligation/finding —
	// a different taxonomy: what KIND of claim, not which manifest layer
	// compiled it). Always populated — layer names are short, so this never
	// meaningfully grows the payload the way an unbounded field would.
	Layer string `json:"layer"`
	// Module names the bound platform module this rule was federated in
	// from (docs/design/platform-modules.md §3.3) — omitted for every
	// repo-local rule (the pre-phase-O default; existing consumers see
	// exactly the same payload shape they always have). Set by
	// internal/modserve.Bind, never by ObligationsFor itself, which knows
	// nothing about modules or bindings.
	Module string `json:"module,omitempty"`
	// Why (wave O1, ADR-037) is set ONLY on a Tier:"raw" row: the coverage
	// defect that made the compiled layer skip this section instead of
	// compiling it into a real claim (e.g. "compiled layer incomplete:
	// budget_stopped; raw nearest guide served"). A compiled row never
	// carries this field, so a payload that never needs a raw fallback row
	// is byte-identical to what it was before this field existed.
	Why string `json:"why,omitempty"`

	// AssertedBy (SU-22, ADR-041 decision 7) carries the underlying claim's
	// core.SemanticClaim.AssertedBy straight through — the PROV-style
	// "kind:id" identity of the activity that minted it. "" (omitted) for
	// every claim written before this field existed, or whose writer had no
	// identity to record — a payload with no such claim in it is
	// byte-identical to what it was before this field existed.
	AssertedBy string `json:"asserted_by,omitempty"`

	// --- Findings-loop source-linked provenance (vision 30 §4 item 4, plan
	// FL — "Serving fields"). Populated ONLY for a claim whose PromptVersion
	// is "external-import/v1" (internal/findings/external's own
	// discriminator, kept out of review-verb/v1's native reconciliation) by
	// joining the served claim against its store.ExternalFinding cursor row.
	// Every field below is omitempty: a payload with no external claims in
	// it is byte-identical to what it was before these fields existed. ---

	// Producer is the catalogued producer id that emitted this finding
	// (e.g. "reviewkit.system-review").
	Producer string `json:"producer,omitempty"`
	// RuleID is the producer's own rule identifier (distinct from Src's
	// citation and from ClaimID — the source system's name for the check).
	RuleID string `json:"rule_id,omitempty"`
	// SourceID is the producer's own finding-source identifier (the
	// per-producer stream this finding's SourceVersion counts within).
	SourceID string `json:"source_id,omitempty"`
	// Class is the finding's failure class (core.FindingClass) — carried
	// here even though it is already on the underlying claim, since an
	// external finding's Class is exactly as useful to a reader of THIS
	// payload as Kind or Tier are.
	Class string `json:"class,omitempty"`
	// Remedy is the producer's stated fix for this finding — distinct from
	// Rule (the claim text / finding statement itself).
	Remedy string `json:"remedy,omitempty"`
	// ObservedAt is when the producer observed this finding, RFC3339.
	ObservedAt string `json:"observed_at,omitempty"`
	// SourceRevision is the producer's own revision identifier at
	// observation time (e.g. a commit SHA) — distinct from Watermark
	// (fathom's own corpus watermark).
	SourceRevision string `json:"source_revision,omitempty"`
	// AnchorCurrent is recomputed at EVERY serve (vision 30 §3.3 item 6):
	// sha256 of the anchor path's CURRENT file bytes, compared against the
	// producer's own anchor digest recorded at import time. true means the
	// file is unchanged since the producer observed it; false means it has
	// moved — served FLAGGED, never dropped and never silently re-anchored.
	// nil when the anchor file cannot currently be read (never coerced to
	// false, which would read as a confident "code moved" when the truth is
	// "could not check"). Always nil when Unanchored.
	AnchorCurrent *bool `json:"anchor_current,omitempty"`
	// Unanchored is true for a process-subject external finding carried
	// with no anchor at all (vision 30 §3.1) — process context, never
	// attached to a file; AnchorCurrent is always nil alongside it.
	Unanchored bool `json:"unanchored,omitempty"`
	// AnchorText (J2, app-kit O6/O16/O24/O26/O29/O30 — the journal's P0
	// "withhold stale obligations from authoritative injection") is set
	// ONLY when Status is non-empty (stale/contradicted) AND the claim's
	// cites link stored the literal text of the cited lines at compile
	// time: sweep.go already proved this rule's citation no longer hashes
	// against the CURRENT file at its stored line range, so a consumer
	// must never re-read that range from the current file and present it
	// as this rule's snippet — the lines there may now be unrelated
	// content (an upstream insertion shifted everything below it, say).
	// AnchorText is the honest alternative: the exact text the rule was
	// compiled against, labeled by every renderer as "cited text at
	// compile; current file differs" rather than a live (and possibly
	// wrong) read. Empty when the claim predates AnchorText (a legacy
	// hash-only cites link) or is fresh — a fresh rule, and a payload with
	// no stale rule at all, are byte-identical to what they were before
	// this field existed.
	AnchorText string `json:"anchor_text,omitempty"`
}

// ModuleNote is an explicit, never-silent note attached to an Obligations
// payload about a bound platform module — most commonly "this repo binds a
// module the daemon does not have mounted" (docs/design/platform-modules.md
// §3.3's doctrine: "Unmounted-but-bound module = an explicit note in the
// payload, never silence"). Populated by internal/modserve.Bind.
type ModuleNote struct {
	Module string `json:"module"`
	Note   string `json:"note"`
}

// ObligationsPayload is the decision-moment payload. Zero rules means a
// clean file — serialize to nothing (zero-output-on-clean is the contract
// that makes every-edit injection affordable).
type Obligations struct {
	Service string       `json:"service"`
	File    string       `json:"file"`
	Rules   []Obligation `json:"rules,omitempty"`
	// Watermark is the corpus watermark the newest contributing claim was
	// compiled at — the honest "as of".
	Watermark string `json:"compiled_at_watermark,omitempty"`
	// Displaced is the count of served-ELIGIBLE claims (scope-matched,
	// Served() status) dropped by the MaxObligations cap this call — T5-2/O14:
	// a recompile that blends a new layer's exact-path claims can silently
	// evict a rule an agent relied on yesterday (the cap is a serving-time
	// slice of "matched", never a claim-lifecycle event — nothing about the
	// evicted claim itself changes). Zero (omitted) when nothing was cut.
	//
	// O14 split (docs/design/platform-modules.md §3.3 deliverable 2): once
	// module rules can blend into the same served set, Displaced is the
	// TOTAL (DisplacedRepo + DisplacedModule) — a compatible superset of
	// its pre-phase-O meaning (when no module ever contributes,
	// DisplacedModule is always 0 and Displaced == DisplacedRepo exactly as
	// before this field existed). Existing consumers reading only
	// "displaced" keep seeing a meaningful number; a phase-O-aware consumer
	// reads the two named fields for the split.
	Displaced int `json:"displaced,omitempty"`
	// DisplacedRepo is the count ObligationsFor itself cut from this
	// service's own claims (identical to the pre-phase-O Displaced value).
	// Always 0 (omitted) before internal/modserve.Bind runs.
	DisplacedRepo int `json:"displaced_repo,omitempty"`
	// DisplacedModule is the count of bound-module rules that were
	// eligible (scope- and tier-matched under the binding) but did not fit
	// the shared budget — set only by internal/modserve.Bind, never by
	// ObligationsFor itself.
	DisplacedModule int `json:"displaced_module,omitempty"`
	// DisplacedByTier is VB-10's receipt (ADR-035 decision 2): the same
	// Displaced count, broken out by the governing-scope tier
	// (rankByGoverningScope's "exact_file"/"package"/"parent_dir"/"root")
	// each cut, file-scoped-eligible claim carried. A caller can tell a
	// dropped root-level catch-all from a dropped package rule — they read
	// very differently. Populated only for a file-scoped call (a root-scope
	// answer has no "distance to a path" to tier); nil when nothing this
	// call cut had a computable tier.
	DisplacedByTier map[string]int `json:"displaced_by_tier,omitempty"`
	// ModuleNotes carries explicit, never-silent notes about this service's
	// bound platform modules (e.g. bound but not mounted on this daemon) —
	// set only by internal/modserve.Bind.
	ModuleNotes []ModuleNote `json:"module_notes,omitempty"`
	// AsOf (SU-19, ADR-041 decision 7) is set ONLY by ObligationsAsOf, to
	// the RFC3339 instant this payload answers "what was valid then" for —
	// never by ObligationsFor/ObligationsForLayer, whose live-serving
	// payload is byte-identical to what it was before this field existed.
	AsOf string `json:"as_of,omitempty"`
}

// ObligationLayerFilter selects which claims obligations serves (VB-9,
// ADR-035 decision 2). The default answers "what governs this file" with
// human-authored rules only — the split O23 asked for: "what governs this
// file" (CLAUDE.md/AGENTS.md/ADR invariants — RoleGoverns) and "what plan
// rows claim this file" (spec-bindings — RoleClaims) are different
// questions with different readers, and serving them through one ranked
// list is what let prose-matched plan rows crowd out the rule that
// mattered.
type ObligationLayerFilter string

const (
	// ObligationLayerRules serves ONLY RoleGoverns claims — the default.
	ObligationLayerRules ObligationLayerFilter = "rules"
	// ObligationLayerClaims serves ONLY RoleClaims claims bound STRICTLY by
	// a plan row's own {symbol, package} contract (ContractScope ok=true
	// and non-empty) — never the compile-time prose fallback. A row with
	// no contract binds to nothing under this filter, exactly as it always
	// has under the default; a row whose citation is not a plan row at all
	// (a findings/** corpus) is likewise excluded — "layer: claims" means
	// plan-row bindings specifically, not every RoleClaims claim.
	ObligationLayerClaims ObligationLayerFilter = "claims"
	// ObligationLayerAll is the pre-VB-9 union: every served claim,
	// RoleClaims claims scoped by contract when the citation is a plan row
	// and by the compile-time (prose) fallback otherwise. ObligationsFor
	// and RootObligationsFor always pass this — VB-9's split is additive,
	// reached only through ObligationsForLayer / RootObligationsForLayer.
	ObligationLayerAll ObligationLayerFilter = "all"
)

// ObligationsFor returns the claims governing file across EVERY compiled
// layer (FT-2: obligations plus, when compiled, findings and any future
// kind), capped at max (MaxObligations when max<=0). Deterministic: same
// store, same answer.
//
// Three things decide what the agent actually sees, in this order
// (ADR-032 §4, the fixes the stopped bench run found — D1, D2, D5):
//
//  1. SCOPE (BP-19, D2). A claims-role layer's rule is re-scoped at serve
//     time from the {package, symbol} contract its cited plan row names
//     (ContractScope), and a row that names no contract binds to nothing
//     and is not served. Prose-matched scopes are what let six plan rows
//     about C++, CUE and tree-sitter bind to one Go file (O23). The
//     compile-time, LLM-proposed scope survives only as the fallback for a
//     claims layer over a NON-plan corpus.
//
//  2. BUDGET (BP-18, D1). Governs-role rules (obligations, adr-rules — the
//     binding rules a human wrote) fill the cap FIRST; claims-role rules
//     (spec-bindings, findings) take only the slots left. Displaced still
//     counts both roles, so an eviction is never silent.
//
//  3. ORDER (BP-17, D5). What survives is ordered fresh before non-fresh,
//     then governs before claims, then by scope specificity (narrowest
//     glob first), then rule text, then claim id. Both T02 payloads in the
//     stopped run led with stale rules; the agent reads top-down, so the
//     freshest binding rule goes first.
func ObligationsFor(ctx context.Context, st store.Store, serviceID, file string, max int) (Obligations, error) {
	return obligationsCore(ctx, st, serviceID, file, max, fileScopeMatch(file), ObligationLayerAll, nil)
}

// ObligationsForLayer is ObligationsFor's VB-9 sibling: layer selects which
// claims are eligible before scope, budget and order ever run (see
// ObligationLayerFilter). ObligationsFor itself always passes
// ObligationLayerAll, so every pre-VB-9 caller sees byte-identical
// behavior; a caller that wants the split asks for it explicitly.
func ObligationsForLayer(ctx context.Context, st store.Store, serviceID, file string, max int, layer ObligationLayerFilter) (Obligations, error) {
	return obligationsCore(ctx, st, serviceID, file, max, fileScopeMatch(file), layer, nil)
}

// ObligationsAsOf answers "what governed file AT asOf" (SU-19, ADR-041
// decision 7, docs/vision/31-substrate.md §3 D6) instead of "what governs it
// now": every claim whose bi-temporal validity window (core.SemanticClaim.
// ValidAt) contains asOf is eligible, REGARDLESS of its CURRENT status — a
// claim later superseded or refuted still answers this question if asOf
// falls before that event. This is deliberately a SEPARATE, slower read path
// from ObligationsFor/ObligationsForLayer (a full per-service claims load,
// filtered in Go) rather than a parameter on obligationsCore's live fetch:
// it is an audit/debug question, never the edit-hook hot path, and the live
// functions' own fetch (store.Store.ClaimsForServe, prefiltered by CURRENT
// status) must stay untouched so their served payload is byte-identical to
// what it was before this function existed. The returned Obligations.AsOf
// carries asOf back as the honest "as of" label.
func ObligationsAsOf(ctx context.Context, st store.Store, serviceID, file string, max int, asOf time.Time) (Obligations, error) {
	return obligationsCore(ctx, st, serviceID, file, max, fileScopeMatch(file), ObligationLayerAll, &asOf)
}

// servedClaimStatuses is core.ClaimStatus.Served()'s three admitted
// statuses (fresh, stale, contradicted) as a slice store.Store.
// ClaimsForServe's status filter can consume directly (SU-2). Kept in sync
// with Served() by TestServedClaimStatusesMatchesServed.
var servedClaimStatuses = []core.ClaimStatus{core.ClaimFresh, core.ClaimStale, core.ClaimContradicted}

// scopeServePrefixes returns outFile itself plus every non-root ancestor
// directory, deepest first — the substrings ClaimsForServe's scope-prefix
// filter checks for literal containment in a stored scope glob (SU-2, see
// the Store interface's ClaimsForServe doc comment). The repo root itself
// ("") is deliberately excluded: an empty LIKE/Contains pattern would
// match every row and defeat the filter entirely — root/global reach is
// instead covered by ClaimsForServe's own wildcard/bare-name safety net.
func scopeServePrefixes(outFile string) []string {
	prefixes := []string{outFile}
	for dir := dirOf(outFile); dir != ""; dir = dirOf(dir) {
		prefixes = append(prefixes, dir)
	}
	return prefixes
}

// fileScopeMatch is the match function ObligationsFor/ObligationsForLayer
// share: a claim's specificity is the byte length of its longest scope glob
// that actually matches file — the KEYWORD tie-break rankByGoverningScope
// (VB-8) now sits behind, inside a proximity tier, rather than in front of
// it.
func fileScopeMatch(file string) func(core.SemanticClaim, []string) int {
	return func(_ core.SemanticClaim, scope []string) int {
		best := -1
		for _, g := range scope {
			if filter.New([]string{g}).Match(file) {
				if len(g) > best {
					best = len(g)
				}
			}
		}
		return best
	}
}

// globReachesRoot reports whether glob g is unanchored or wildcard-rooted —
// a bare "*", a bare "**", or anything starting "**/" — i.e. it reaches the
// repo root by construction. "internal/**/*.go" does not qualify: BP-32's
// scopeFloor prefixed it to a nested CLAUDE.md's own subtree, and it can
// never match a file outside that subtree.
func globReachesRoot(g string) bool {
	g = strings.TrimSpace(g)
	return g == "*" || g == "**" || strings.HasPrefix(g, "**/")
}

// isRootLevelSource reports whether subject — a claim's governing-source
// citation, e.g. "CLAUDE.md#deterministic-excellence-gate" — names a
// root-level governing doc: the repo-root CLAUDE.md/AGENTS.md (never a
// NESTED one — "internal/mcp/CLAUDE.md" governs only its own subtree per
// scopeFloorDir/instructionFileBases and fails this exact-string check), or
// anything under docs/intent/ (the project-intent router and its living
// docs, repo-wide by convention).
func isRootLevelSource(subject string) bool {
	path := subject
	if i := strings.IndexByte(subject, '#'); i >= 0 {
		path = subject[:i]
	}
	if instructionFileBases[path] {
		return true
	}
	return strings.HasPrefix(path, "docs/intent/")
}

// RootObligationsFor answers "what governs this repository" (RC-22,
// docs/vision/28-receipts.md §3.5): the served claims whose SOURCE is a
// root-level governing doc (isRootLevelSource) or whose scope glob is
// unanchored/wildcard-rooted (globReachesRoot) — i.e. rules that reach the
// repo root by construction, not merely a glob that happens to match one
// literal probe path. That literal-probe approach is the bug this replaces:
// a repo-wide "**/*.go" rule from the root CLAUDE.md never matches the
// literal string "CLAUDE.md", so root-scope code rules were silently
// dropped from every root-scope call. Budgeted and ordered identically to a
// file-scoped call (BP-17/BP-18); out.File is always "" (no single file
// answers this question).
func RootObligationsFor(ctx context.Context, st store.Store, serviceID string, max int) (Obligations, error) {
	return obligationsCore(ctx, st, serviceID, "", max, rootScopeMatch, ObligationLayerAll, nil)
}

// RootObligationsForLayer is RootObligationsFor's VB-9 sibling — see
// ObligationsForLayer's doc comment; the same layer split applies to the
// root-scope question.
func RootObligationsForLayer(ctx context.Context, st store.Store, serviceID string, max int, layer ObligationLayerFilter) (Obligations, error) {
	return obligationsCore(ctx, st, serviceID, "", max, rootScopeMatch, layer, nil)
}

func rootScopeMatch(cl core.SemanticClaim, scope []string) int {
	best := -1
	for _, g := range scope {
		if globReachesRoot(g) && len(g) > best {
			best = len(g)
		}
	}
	if best < 0 && isRootLevelSource(cl.Subject) {
		// The SOURCE alone is enough even when the model proposed a
		// scope glob that is not itself wildcard-rooted (a narrow
		// "CLAUDE.md#section" restatement scoped to one file, say) —
		// a root-level governing doc's claim is root-scope by where it
		// came from, not only by how widely its glob reaches.
		best = 0
	}
	return best
}

// scored is one matched claim carrying everything the budget/order phase
// of obligationsCore needs about it: cl.claims (RoleClaims fills the
// budget only after governs, BP-18), governing (its proximity to the
// edited path, VB-8) and relevance (wave O2, item 1: the lexical tie-break
// used STRICTLY INSIDE a governing-scope tier — see less()) alongside the
// plain glob-length specificity score. Package-level (not local to
// obligationsCore) so reserveNearestTier can share it.
type scored struct {
	cl          core.SemanticClaim
	specificity int
	claims      bool
	governing   governingScope
	relevance   float64
	strong      bool // wave O3, item 1: relevanceStrength's verdict — see crossTierCloser
}

// claimRelevanceText builds the text relevanceScore scores one compiled
// claim against: the claim's own Subject heading words (the slug after
// "#", hyphens turned to spaces, folded into the synthetic FIRST line so
// relevanceScore's own heading/body split weighs a heading-named match the
// same way it does for a raw section's real ATX heading) followed by the
// claim's Rule text — the only claim-authored prose obligationsCore ever
// holds; no full cited-source re-read here, unlike RawFallbackRules, which
// scores a whole raw section's actual text.
func claimRelevanceText(cl core.SemanticClaim) string {
	heading := cl.Subject
	if i := strings.IndexByte(heading, '#'); i >= 0 {
		heading = heading[i+1:]
	}
	heading = strings.ReplaceAll(heading, "-", " ")
	return heading + "\n" + cl.Rule
}

// reserveNearestTier returns up to n items from matched belonging to the
// single closest governingScope TIER actually present among them — tier
// 0 (exact_file) if any matched claim reaches it, else tier 1 (package),
// else tier 2 (parent_dir), else tier 3 (root) — in matched's existing
// order (already sorted by less() before the caller reaches here), so
// which of that tier's own rules fill the reserved slots still respects
// freshness, role and specificity; only the cross-tier competition
// against FARTHER rules is bypassed for these n slots. Empty when
// outFile is "" (a root-scope call has no path to be proximate to, so
// every candidate carries noGoverningScope and reservation is a no-op)
// or when nothing in matched carries a real governing scope.
func reserveNearestTier(matched []scored, n int, outFile string) []scored {
	if outFile == "" || n <= 0 {
		return nil
	}
	nearestTier := -1
	for _, m := range matched {
		if m.governing.tier < 0 {
			continue
		}
		if nearestTier < 0 || m.governing.tier < nearestTier {
			nearestTier = m.governing.tier
		}
	}
	if nearestTier < 0 {
		return nil
	}
	var reserved []scored
	for _, m := range matched {
		if len(reserved) >= n {
			break
		}
		if m.governing.tier == nearestTier {
			reserved = append(reserved, m)
		}
	}
	return reserved
}

// obligationsCore is the shared match/budget/order engine behind
// ObligationsFor and RootObligationsFor: outFile is what the returned
// Obligations.File carries (a real path, or "" for the root-scope
// answer — there is no probe file at all, honest absence); match decides,
// per claim, whether its (already ContractScope-resolved) scope belongs in
// the answer, returning a specificity (higher wins ties) or <0 for no
// match. See ObligationsFor's doc comment for the SCOPE/BUDGET/ORDER
// contract this enforces regardless of which match function is passed.
//
// asOf is nil for every live caller (ObligationsFor/ObligationsForLayer/
// RootObligationsFor/RootObligationsForLayer) — the fetch and status
// handling below are UNCHANGED from before ObligationsAsOf existed in that
// case, so their served payload stays byte-identical. Non-nil only via
// ObligationsAsOf: see its own doc comment for why that path fetches and
// filters differently.
func obligationsCore(ctx context.Context, st store.Store, serviceID, outFile string, max int, match func(cl core.SemanticClaim, scope []string) int, layerFilter ObligationLayerFilter, asOf *time.Time) (Obligations, error) {
	if max <= 0 || max > MaxObligations {
		max = MaxObligations
	}
	out := Obligations{Service: serviceID, File: outFile}
	if asOf != nil {
		out.AsOf = asOf.UTC().Format(time.RFC3339)
	}

	// serviceRoot is the ONE place a service's project directory is knowable
	// from (ctx, st, serviceID); both the layer-role split below (BP-18) and
	// the citation re-read behind ContractScope (BP-19) resolve through it,
	// so serving sees the same embedded ∪ project-local union a compile saw.
	root := serviceRoot(ctx, st, serviceID)

	// SU-2 (docs/plans/substrate.md §A, ADR-041 decision 3): claims are
	// prefiltered in the STORE (SQL WHERE for SQLiteStore, a Go loop for
	// MemoryStore) by served status, and — for a file-scoped call — by the
	// edited file's own directory prefixes, before any Go scoring runs.
	// This replaces the old "load every claim for the service, then
	// cl.Status.Served() in Go" full scan.
	//
	// A claims-role layer's REAL matching scope is re-derived at serve time
	// below from its citation (ContractScope, BP-19) — the STORED scope
	// column this stage's prefix filter reads is only a compile-time
	// fallback, so it must never be used to exclude a claims-role claim
	// here. claimsRoleLayers is resolved once, from config alone (no store
	// I/O), and also backs layerRole below so the role lookup the rest of
	// this function needs is a single map read instead of one
	// LoadLayerIn/loadUnion rebuild per distinct layer name.
	roleByLayer := LayerRolesIn(root)
	claimsRoleLayers := make([]string, 0, len(roleByLayer))
	for name := range roleByLayer {
		claimsRoleLayers = append(claimsRoleLayers, name)
	}
	sort.Strings(claimsRoleLayers)
	layerRole := func(layer string) string {
		if roleByLayer[layer] == RoleClaims {
			return RoleClaims
		}
		return RoleGoverns
	}

	var scopePrefixes []string
	if outFile != "" {
		scopePrefixes = scopeServePrefixes(outFile)
	}

	byID := map[string]core.SemanticClaim{}
	if asOf != nil {
		// SU-19: "as of" answers a DIFFERENT question than live serving —
		// not "what is CURRENTLY valid" (the status-prefiltered fetch
		// below) but "what WAS valid at asOf, whatever its status is now".
		// A full per-service load, filtered in Go by core.SemanticClaim.
		// ValidAt: slower than the live path by design (see ObligationsAsOf's
		// doc comment), and deliberately independent of the claims-role/
		// governs-role split's own scope-prefix optimization — match() below
		// still re-checks scope for every candidate regardless of how it was
		// fetched, so skipping that optimization here changes performance
		// only, never which claims are eligible.
		all, err := st.ClaimsForService(ctx, serviceID, "")
		if err != nil {
			return out, fmt.Errorf("semantic: obligations load (as_of): %w", err)
		}
		for _, cl := range all {
			if cl.ValidAt(*asOf) {
				byID[cl.ClaimID] = cl
			}
		}
	} else {
		if len(claimsRoleLayers) > 0 && len(scopePrefixes) > 0 {
			// A file-scoped call scope-filters the main fetch below; claims-role
			// layers must be fetched WITHOUT that filter (see above), so they
			// get their own unfiltered-by-scope pass here. Skipped for a
			// root-scope call (scopePrefixes empty) or when no layer resolves
			// to RoleClaims at all — the main fetch already covers both.
			crClaims, err := st.ClaimsForServe(ctx, serviceID, claimsRoleLayers, servedClaimStatuses, nil)
			if err != nil {
				return out, fmt.Errorf("semantic: obligations load (claims-role): %w", err)
			}
			for _, cl := range crClaims {
				byID[cl.ClaimID] = cl
			}
		}
		govClaims, err := st.ClaimsForServe(ctx, serviceID, nil, servedClaimStatuses, scopePrefixes)
		if err != nil {
			return out, fmt.Errorf("semantic: obligations load: %w", err)
		}
		for _, cl := range govClaims {
			byID[cl.ClaimID] = cl
		}
	}
	if len(byID) == 0 {
		return out, nil
	}
	served := make([]core.SemanticClaim, 0, len(byID))
	for _, cl := range byID {
		served = append(served, cl)
	}
	// Reproduce ClaimsForService's exact ordering: the two prefiltered
	// fetches above are each independently ordered, but the union (map
	// iteration) is not — re-sort so downstream logic (and any tie-break
	// that reads input order) sees the SAME order the old full-scan path
	// always fed it.
	sort.Slice(served, func(i, j int) bool {
		if served[i].Layer != served[j].Layer {
			return served[i].Layer < served[j].Layer
		}
		if served[i].Subject != served[j].Subject {
			return served[i].Subject < served[j].Subject
		}
		return served[i].ClaimID < served[j].ClaimID
	})
	ids := make([]string, len(served))
	for i, cl := range served {
		ids[i] = cl.ClaimID
	}

	links, err := st.ClaimLinks(ctx, serviceID, ids)
	if err != nil {
		return out, fmt.Errorf("semantic: obligations links: %w", err)
	}
	primaryCite := map[string]string{}
	for _, l := range links {
		if l.Rel == core.RelCites {
			if _, ok := primaryCite[l.ClaimID]; !ok {
				primaryCite[l.ClaimID] = l.TargetRef
			}
		}
	}

	anchorText := map[string]string{}
	for _, l := range links {
		if l.Rel == core.RelCites && l.AnchorText != "" {
			if _, ok := anchorText[l.ClaimID]; !ok {
				anchorText[l.ClaimID] = l.AnchorText
			}
		}
	}
	citedText := citedTextResolver(root, anchorText)
	lookup := symbolFileLookup(ctx, st, serviceID)
	// wave O2, item 1/2: path-derived tokens for the lexical relevance
	// tie-break less() applies STRICTLY INSIDE a governing-scope tier (see
	// less() below). Computed once here — pure function of outFile alone,
	// no store I/O — never for a root-scope call, which has no path to be
	// relevant to.
	var relevanceTokens []weightedToken
	if outFile != "" {
		relevanceTokens = pathRelevanceTokens(outFile)
	}

	var matched []scored
	for _, cl := range served {
		claims := layerRole(cl.Layer) == RoleClaims
		scope := cl.Scope
		contractBound := false
		if claims {
			// BP-19/D2: the plan row's own contract, not its prose. ok=false
			// means the citation is not a plan row — a findings-style corpus
			// — and the compile-time scope stands.
			if cs, ok := ContractScope(citedText(primaryCite[cl.ClaimID], cl.ClaimID), lookup); ok {
				scope = cs
				contractBound = len(cs) > 0
			}
		}
		// VB-9 (ADR-035 decision 2): the layer split happens BEFORE match —
		// an excluded claim never even competes for a scope tier or a
		// budget slot.
		switch layerFilter {
		case ObligationLayerRules:
			if claims {
				continue
			}
		case ObligationLayerClaims:
			if !claims || !contractBound {
				continue
			}
		}
		best := match(cl, scope)
		if best >= 0 {
			governing := noGoverningScope
			var relevance float64
			var strong bool
			if outFile != "" {
				governing = rankByGoverningScope(scope, outFile)
				relevance, strong = relevanceStrength(claimRelevanceText(cl), relevanceTokens)
			}
			matched = append(matched, scored{cl: cl, specificity: best, claims: claims, governing: governing, relevance: relevance, strong: strong})
		}
	}
	if len(matched) == 0 {
		return out, nil
	}
	less := func(a, b scored) bool {
		if af, bf := a.cl.Status == core.ClaimFresh, b.cl.Status == core.ClaimFresh; af != bf {
			return af // BP-17/D5: fresh before stale or contradicted
		}
		if a.claims != b.claims {
			return !a.claims // BP-18/D1: governs before claims
		}
		// VB-8 (ADR-035 decision 2): proximity to the edited path is the
		// PRIMARY tie-break inside a role/freshness tier — exact file beats
		// package beats a named ancestor directory beats the repo root.
		// Only meaningful for a file-scoped call (outFile != ""); a
		// root-scope answer has no path to be proximate to, so every
		// candidate carries the same noGoverningScope and this comparison
		// is always a no-op there. wave O3, item 1 (wave O4, item 1 narrowed
		// the bar): crossTierCloser lets a farther-tier claim with STRONG
		// lexical relevance leapfrog a nearer-tier claim that is not itself
		// strong — an ancestor-directory-only hit no longer blocks this,
		// only a hit on the file's own package/stem/symbol does — see its
		// own doc comment; every other case reproduces plain closerThan
		// exactly.
		if outFile != "" && a.governing != b.governing {
			return crossTierCloser(a.governing, b.governing, a.strong, b.strong)
		}
		// wave O2 (vs-pilot-6 second reading §5): STRICTLY INSIDE a
		// governing-scope tier (same a.governing/b.governing — VB-8 above
		// already decided the tier itself, this never overrides it), the
		// lexical relevance score decides NEXT — several same-tier, same-
		// freshness, same-role claims (a nested guide's own several
		// sections, all floored to the same directory) no longer fall
		// straight to the byte-length/rule-text tie-break below, which is
		// closer to alphabetical/document order than to "which rule
		// actually names this file".
		if outFile != "" && a.relevance != b.relevance {
			return a.relevance > b.relevance
		}
		// The keyword score (byte length of the matched glob) breaks ties
		// INSIDE a tier — two claims scoped to the same directory still
		// need a deterministic order.
		if a.specificity != b.specificity {
			return a.specificity > b.specificity
		}
		if a.cl.Rule != b.cl.Rule {
			return a.cl.Rule < b.cl.Rule
		}
		return a.cl.ClaimID < b.cl.ClaimID
	}
	sort.Slice(matched, func(i, j int) bool { return less(matched[i], matched[j]) })
	if len(matched) > max {
		// T5-2/O14: record the cap's damage BEFORE slicing it away — this is
		// the only point that knows how many served-eligible claims existed
		// versus how many fit the budget. BP-18: the cut is role-partitioned,
		// so a claims rule can never evict a governs rule, but Displaced
		// still counts BOTH — the eviction stays one honest integer.
		out.Displaced = len(matched) - max
		// O57 item 4: the nearest-scope tier gets first claim on the first
		// nearestTierReserveSlots slots, unconditionally — before role or
		// freshness gets a say. VB-8's proximity tie-break (governingscope.go)
		// already sorts a FRESH, governs-role nearest rule to the front of
		// its bucket, which is why the ordering-only test below already
		// passed before this reservation existed; what it does NOT protect
		// is a nearest rule that is stale, or claims-role — six fresh root
		// rules can fill every slot before either is even considered, which
		// is exactly what guards-pilot-5's second reading caught (a served
		// payload naming no rule closer than "root" while a package guide
		// governed the edited file). Reservation only decides MEMBERSHIP;
		// which of the nearest tier's own rules fill the reserved slots
		// still goes through matched's existing freshness/role/specificity
		// order, and the final presentation order below is unaffected by
		// how a rule reached kept.
		reserveN := nearestTierReserveSlots
		if reserveN > max {
			reserveN = max
		}
		reserved := reserveNearestTier(matched, reserveN, outFile)
		reservedIDs := make(map[string]bool, len(reserved))
		for _, m := range reserved {
			reservedIDs[m.cl.ClaimID] = true
		}
		kept := make([]scored, 0, max)
		kept = append(kept, reserved...)
		for _, m := range matched {
			if len(kept) >= max {
				break
			}
			if !m.claims && !reservedIDs[m.cl.ClaimID] {
				kept = append(kept, m)
			}
		}
		if len(kept) > max {
			kept = kept[:max]
		}
		for _, m := range matched {
			if len(kept) >= max {
				break
			}
			if m.claims && !reservedIDs[m.cl.ClaimID] {
				kept = append(kept, m)
			}
		}
		// VB-10 (ADR-035 decision 2): before the survivors are re-sorted,
		// tally the DROPPED claims by governing-scope tier — a caller can
		// then see WHAT KIND of rule the cap displaced, not just how many.
		// File-scoped only: a root-scope call carries no computable tier.
		if outFile != "" {
			keptIDs := make(map[string]bool, len(kept))
			for _, k := range kept {
				keptIDs[k.cl.ClaimID] = true
			}
			byTier := map[string]int{}
			for _, m := range matched {
				if !keptIDs[m.cl.ClaimID] {
					byTier[m.governing.name()]++
				}
			}
			if len(byTier) > 0 {
				out.DisplacedByTier = byTier
			}
		}
		// Re-order the survivors: the role partition above is a BUDGET
		// decision, not a presentation one, and it can leave a stale governs
		// rule ahead of a fresh claims rule. The served payload is ordered
		// by less() — fresh first (BP-17) — whatever the budget did.
		sort.Slice(kept, func(i, j int) bool { return less(kept[i], kept[j]) })
		matched = kept
	}

	// Remediation-drift annotation (FT-6): computed once, only when a claim
	// from a drift-tracked layer (LC-2: serving.drift_tracked, not a
	// kind=="finding" Go hardcode; LC-3: resolved through the SAME embedded
	// ∪ project-local union a compile saw, via serviceRoot) actually reached
	// the served set — DriftedFindings itself short-circuits to a single
	// empty ClaimsForService read when the service has no drift-tracked
	// claims at all, but skipping the call entirely for pure-obligations
	// services avoids even that. layerIsDriftTracked recompiles the whole
	// pack union per call, so it is memoized per distinct layer name for
	// this ObligationsFor invocation (the same map pattern DriftedFindings
	// itself uses for fingerprints) — a served set of 6 same-layer rules
	// costs one union load, not six, on the edit-hook hot path.
	var drifted map[string]bool
	trackedByLayer := map[string]bool{}
	for _, m := range matched {
		tracked, ok := trackedByLayer[m.cl.Layer]
		if !ok {
			tracked = layerIsDriftTracked(root, m.cl.Layer)
			trackedByLayer[m.cl.Layer] = tracked
		}
		if tracked {
			drifted, err = DriftedFindings(ctx, st, serviceID)
			if err != nil {
				return out, fmt.Errorf("semantic: obligations drift: %w", err)
			}
			break
		}
	}

	for _, m := range matched {
		ob := Obligation{
			Rule:       m.cl.Rule,
			Src:        primaryCite[m.cl.ClaimID],
			Tier:       string(m.cl.Tier),
			Layer:      m.cl.Layer,
			AssertedBy: m.cl.AssertedBy,
		}
		if m.cl.Kind != KindObligation {
			ob.Kind = m.cl.Kind
		}
		if m.cl.Status != core.ClaimFresh {
			ob.Status = string(m.cl.Status)
			ob.AnchorText = anchorText[m.cl.ClaimID]
		}
		if drifted[m.cl.ClaimID] {
			ob.CodeDrifted = true
		}
		if m.cl.PromptVersion == externalImportPromptVersion {
			fillExternalFinding(ctx, st, serviceID, root, m.cl, &ob)
		}
		out.Rules = append(out.Rules, ob)
		if m.cl.CompiledAtWatermark != "" && out.Watermark == "" {
			out.Watermark = m.cl.CompiledAtWatermark
		}
	}
	return out, nil
}

// fillExternalFinding joins cl (a claim imported by internal/findings/
// external, PromptVersion "external-import/v1") against its
// store.ExternalFinding cursor row and fills ob's source-linked provenance
// fields (vision 30 §4 item 4). A missing cursor row (should not happen —
// the importer always writes one alongside the claim — but never trusted
// blindly) leaves ob's new fields at their zero value rather than erroring
// the whole obligations call over one malformed row.
func fillExternalFinding(ctx context.Context, st store.Store, serviceID, root string, cl core.SemanticClaim, ob *Obligation) {
	ef, found, err := st.ExternalFinding(ctx, serviceID, cl.ClaimID)
	if err != nil || !found {
		return
	}
	ob.Producer = ef.Producer
	ob.RuleID = ef.RuleID
	ob.SourceID = ef.SourceID
	ob.Class = string(cl.Class)
	ob.Remedy = ef.Remedy
	if !ef.ObservedAt.IsZero() {
		ob.ObservedAt = ef.ObservedAt.UTC().Format(time.RFC3339)
	}
	ob.SourceRevision = ef.SourceRevision
	ob.Unanchored = ef.Unanchored

	if ef.Unanchored || ef.AnchorPath == "" {
		return
	}
	// anchor_current is recomputed at EVERY serve, never cached (vision 30
	// §3.3 item 6): a file that has moved since the producer observed it is
	// served flagged the moment it moves, not on the next import.
	data, ok := readCurrentFile(root, ef.AnchorPath)
	if !ok {
		// Unreadable (or no service root known): "could not check", never
		// coerced to a confident false.
		return
	}
	sum := sha256.Sum256(data)
	current := hex.EncodeToString(sum[:])
	match := current == ef.AnchorDigest
	ob.AnchorCurrent = &match
}

// readCurrentFile reads path (repo-relative) under a service's on-disk root
// — the same serviceRoot every other serve-time drift check in this file
// resolves through. ok is false when root is unknown or the read fails for
// any reason (absent, unreadable, permission) — AnchorCurrent's caller
// treats every such case identically: "could not check", never "changed".
func readCurrentFile(root, path string) ([]byte, bool) {
	if root == "" || path == "" {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return nil, false
	}
	return data, true
}
