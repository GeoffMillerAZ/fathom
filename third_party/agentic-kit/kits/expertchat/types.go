package expertchat

import (
	"strings"
	"unicode"

	scoring "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/scoring"
)

// Status values a served claim may carry, mirroring docs/spec/
// expert-module-v0.md §2's per-claim status: "fresh / stale / contradicted /
// refuted, never silently dropped." expertchat itself never silently drops a
// status either — see EvidenceItem.Citable — but it does refuse to BUILD an
// answer on a contradicted or refuted claim (citing a refuted claim would
// make the answer dishonest, not just unfresh).
const (
	// StatusFresh: the claim's watermark has not drifted since compile.
	StatusFresh = "fresh"
	// StatusStale: the claim is still served, but its module's corpus has
	// drifted since the claim was compiled. Eligible for citation only when
	// Policy.RequireFresh is false (the honest default).
	StatusStale = "stale"
	// StatusContradicted: a newer claim conflicts with this one. Never
	// eligible for citation — see EvidenceItem.Citable.
	StatusContradicted = "contradicted"
	// StatusRefuted: the claim has been actively refuted. Never eligible for
	// citation — see EvidenceItem.Citable.
	StatusRefuted = "refuted"
)

// DefaultLimit is the evidence-fetch limit used when Deps.Limit is zero.
const DefaultLimit = 8

// DefaultMaxRetries is the number of retries used when Deps.MaxRetries is
// zero: one retry (two attempts total), matching the plan's "reject/
// retry-once then refuse on fabricated citations" contract (requirement 4).
const DefaultMaxRetries = 1

// EvidenceItem is one claim returned by a Retriever: the module-host's
// answer to "why should this be trusted, when did it stop being sure, what
// was it compiled from" (docs/spec/expert-module-v0.md §2), still in
// retrieval shape — before tier/freshness policy has been applied and before
// the model has cited it. A Citation is the resolved, cited form of an
// EvidenceItem that survived Run's policy filters and was actually cited by
// the model.
type EvidenceItem struct {
	// Module is the stable module identifier (docs/spec/expert-module-v0.md
	// §3.1's manifest `name`) the claim was served from.
	Module string

	// ModuleVersion is the module manifest's own `version` (§3.1) — NOT a
	// schema-shape version. Part of the claim's provenance.
	ModuleVersion string

	// Watermark is the module's corpus watermark at compile time
	// (docs/spec/expert-module-v0.md §3.3): a 40-hex git SHA, or
	// "manifest:"+64-hex sha256 for a non-git corpus.
	Watermark string

	// ClaimID uniquely identifies this claim within its module. This is the
	// exact identifier the model is instructed to cite — see prompt.go.
	ClaimID string

	// Tier is the claim's evidence tier, in the shared scoring.Tier
	// vocabulary (foundations/review/scoring) — reused, not reinvented, so a
	// module-host claim's tier string means the same thing on both sides of
	// the MCP boundary.
	Tier scoring.Tier

	// Status is the claim's per-claim staleness verdict: one of the
	// StatusXxx constants. Never empty for a well-formed EvidenceItem — see
	// Citable.
	Status string

	// SourceRef identifies where in the corpus the claim's citation resolves
	// (docs/spec/store-format-v0.md §5's anchor rules) — e.g. a file path
	// plus line range.
	SourceRef string

	// Anchor is the quoted anchor text the claim is grounded in — the exact
	// excerpt a human (or a downstream verifier) can check the claim against.
	Anchor string

	// Text is the claim's natural-language content, shown to the model as
	// context for composing an answer. Not itself part of Citation — Citation
	// carries only the provenance fields, per requirement 2 of the founding
	// plan ("module identity, claim/source ref, tier, staleness verdict,
	// quoted anchor text").
	Text string
}

// Citable reports whether e is eligible to ever be cited, independent of any
// caller Policy: only StatusFresh and StatusStale claims are citable.
// StatusContradicted and StatusRefuted (and any unrecognized status — the
// same "unknown is never silently promoted" discipline foundations/
// retrieval/truth-bridge already documents) are never citable: an answer
// built on a refuted or contradicted claim would be actively dishonest, not
// merely stale. Run filters these out before Policy is even applied.
func (e EvidenceItem) Citable() bool {
	return e.Status == StatusFresh || e.Status == StatusStale
}

// Policy is the caller-configurable tier/staleness policy Run applies to
// retrieved evidence before composing an answer. The zero value is the
// honest default: no tier floor, and stale (but not contradicted/refuted)
// evidence is still served — always flagged via Citation.Status, never
// silently promoted to look fresh.
type Policy struct {
	// TierThreshold is the minimum scoring.Tier an EvidenceItem must rank at
	// or above (scoring.TierRank) to be eligible for citation. The zero value
	// ("") applies no floor: every recognized tier is eligible, and an
	// unrecognized tier ranks 0 (scoring.TierRank's documented behavior) and
	// is therefore excluded by any non-zero threshold.
	TierThreshold scoring.Tier

	// RequireFresh, when true, excludes StatusStale evidence from citation
	// entirely (Run refuses with RefusalStaleOnlyEvidence when that leaves
	// nothing eligible). The zero value (false) is the honest default: stale
	// evidence is still served, flagged, never silently dropped or promoted.
	RequireFresh bool
}

// hasForbiddenTokenChars reports whether s contains any rune that would
// make it unsafe to embed in a citeToken ("<Module>#<ClaimID>") or in the
// comma-separated CITES list: '#' (the token's own field separator — its
// presence inside a field creates a token that could be misparsed as a
// DIFFERENT Module/ClaimID split), ',' (the CITES list's separator), or any
// whitespace/control rune (a newline in particular could forge an extra
// prompt line once interpolated).
func hasForbiddenTokenChars(s string) bool {
	for _, r := range s {
		if r == '#' || r == ',' || unicode.IsSpace(r) || unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// validCiteIdentity reports whether e's identity fields (Module, ClaimID,
// Tier) are non-empty and safe to embed in a citeToken and the CITES wire
// format. An item failing this check can never be offered to the model for
// citation, regardless of Citable(): Run drops such items when building its
// citable set (chat.go) rather than offering a token that can only ever
// resolve ambiguously, get permanently rejected by the comma-separated
// format, or (an empty Tier) always fail NewCitedAnswer's own validation
// after the model has already spent a turn citing it.
func validCiteIdentity(e EvidenceItem) bool {
	if strings.TrimSpace(e.Module) == "" || strings.TrimSpace(e.ClaimID) == "" || strings.TrimSpace(string(e.Tier)) == "" {
		return false
	}
	return !hasForbiddenTokenChars(e.Module) && !hasForbiddenTokenChars(e.ClaimID)
}
