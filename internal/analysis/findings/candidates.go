package findings

// candidates.go — FL-23, the §3.6 expert-system half of the loop (docs/
// vision/30-findings-loop.md §3.6, docs/design/findings-as-evidence.md):
// `fathom findings candidates` derives RULE CANDIDATES from DerivePatterns
// over native AND external findings alike — a pattern with >= min-sites
// distinct sites emits one candidate: the class, the normalized rule
// shape, member citations, the producers that saw it, how many members
// resolved via an independent code change, a deterministically drafted
// rule sentence, and the ADR-038 check family that could enforce it if a
// human lifts it. Candidates are a REPORT — fathom never writes a rule
// anywhere (§3.6's own last line; ADR-040 decision 5's "only through the
// human-adopted candidate").
//
// External claims are discriminated from native ones by PromptVersion
// "external-import/v1" and a Subject prefixed "ext:" (ADR-040 decisions 2
// and 3) — DerivePatterns itself is blind to the distinction (it groups
// purely by (Class, NormalizeRuleShape(Rule))), which is exactly the
// point: a recurring failure a native review AND an external reviewer
// both caught is one pattern, not two.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// externalSubjectPrefix marks a claim compiled from the governed import
// (ADR-040 decision 2: "derives claim identity deterministically in a
// producer-namespaced subject"). The exact namespacing past this prefix is
// the projection-decoder wave's own contract (FL-4..FL-9); producerOf's
// parse below degrades honestly rather than guessing wrong once that
// format lands.
const externalSubjectPrefix = "ext:"

// nativeReviewProducer is the shipped native producer id (docs/plans/
// findings-loop.md §A's producer catalog) every non-external finding-kind
// claim is attributed to — the deterministic review-verb bridge and the
// LLM findings/** compiler both persist onto the same "findings" layer
// under this one fathom-origin identity.
const nativeReviewProducer = "fathom.review"

// producerOf best-effort derives a claim's producer id from its Subject:
// a native claim (no "ext:" prefix) is nativeReviewProducer; an external
// claim's subject is producer-namespaced ("ext:<producer>:<rest>" or
// "ext:<producer>/<rest>") — the first "/" or ":" after the prefix ends
// the producer id. A subject this parse cannot split degrades to
// "external", never a fabricated name.
func producerOf(subject string) string {
	if !strings.HasPrefix(subject, externalSubjectPrefix) {
		return nativeReviewProducer
	}
	rest := strings.TrimPrefix(subject, externalSubjectPrefix)
	if i := strings.IndexAny(rest, "/:"); i > 0 {
		return rest[:i]
	}
	if rest != "" {
		return rest
	}
	return "external"
}

// checkFamilyFor maps a finding class to the ADR-038 deterministic check
// family (internal/guardspec/enforce.go's own CheckID vocabulary) that
// could enforce it once a human lifts the candidate into
// .fathom/guard.cue — the returned string is exactly what a guard's
// `checks: […]` entry or a layer_rule `kind` would name, so a candidate's
// CheckFamily is directly promotable, never just descriptive. A class
// whose defects a deterministic check cannot mechanically recognize
// (correctness, contract-violation, resource-leak, error-handling,
// concurrency, performance) is "advisory".
func checkFamilyFor(class core.FindingClass) string {
	switch class {
	case core.FindingClassSecurity:
		return "secret_literal" // findingclass.go: "…secret exposure…"
	case core.FindingClassConformance:
		return "import_direction" // findingclass.go: "…a layering boundary, an import-direction constraint…"
	case core.FindingClassTestGap:
		return "usecase_maintained" // findingclass.go: "…includes spec-coverage findings…" — the Maintained-line convention
	case core.FindingClassConfigDrift:
		return "workspace_output" // a generated file landing outside its configured path IS a config-drift instance
	default:
		return "advisory"
	}
}

// draftSentence renders shape as a deterministic rule-sentence template —
// never a model, never freeform prose (§3.6: "a drafted rule sentence").
// shape is already NormalizeRuleShape's coarse, lowercased form.
func draftSentence(shape string) string {
	if shape == "" {
		return ""
	}
	return "Never " + shape + "."
}

// MemberCitation is one pattern member's site — as much location as a bare
// core.SemanticClaim carries at this layer (no ClaimLink "cites" citation:
// RuleCandidates, like DerivePatterns, is a pure function of claims+events
// alone, never a second store read).
type MemberCitation struct {
	ClaimID  string   `json:"claim_id"`
	Subject  string   `json:"subject"`
	Scope    []string `json:"scope"`
	Producer string   `json:"producer"`
}

// RuleCandidate is one derived candidate a human could lift into a rule
// (CLAUDE.md or .fathom/guard.cue). fathom never writes it.
type RuleCandidate struct {
	Class     core.FindingClass `json:"class"`
	Shape     string            `json:"shape"`
	Sites     int               `json:"sites"`
	Members   []MemberCitation  `json:"members"`
	Producers []string          `json:"producers"`

	CodeChangeResolutions int    `json:"code_change_resolutions"`
	Draft                 string `json:"draft"`
	CheckFamily           string `json:"check_family"`

	// Score, PatternID and Disputes are SU-15 (docs/vision/31-substrate.md
	// §3 D8 item 1, ADR-041 decision 8): Score is what CadenceCap ranks and
	// caps on (sites × mean ownership weight × dispute decay — see
	// CandidatePolicy); PatternID is the deterministic id embedded in
	// Draft's "promoted_from" tag; Disputes is how many members that decay
	// was actually applied for.
	Score     float64 `json:"score"`
	PatternID string  `json:"pattern_id"`
	Disputes  int     `json:"disputes"`
}

// CandidatePolicy configures RuleCandidatesWithPolicy's scoring, decay, and
// cadence (docs/vision/31-substrate.md §3 D8 item 1; ADR-041 decision 8;
// plan SU-15) — the promotion half of the findings loop the survey named as
// missing: "promotion with decay, ownership weight and a cadence cap".
type CandidatePolicy struct {
	// MinSites is DerivePatterns' own admission floor, clamped to >= 2 —
	// mirrors RuleCandidates' pre-existing minSites parameter.
	MinSites int
	// DecayPerDispute is the fraction a candidate's score is multiplied down
	// by, ONCE PER disputed member (a member with a "refuted" claim_event) —
	// applied as (1-DecayPerDispute)^disputes, so a pattern that keeps
	// getting refuted keeps losing cadence priority. 0 (the zero value)
	// means no decay — RuleCandidates' own default.
	DecayPerDispute float64
	// OwnershipWeight, when non-nil, returns a member file's ownership
	// coverage weight (the codebase's own convention: 1.0 covered by
	// CODEOWNERS, 0.0 not) — a candidate's weight is the MEAN of this over
	// every member's scoped file(s). A nil func (the zero value; "no
	// CODEOWNERS") weights every candidate 1.0, byte-identical to
	// RuleCandidates' pre-SU-15 behavior.
	OwnershipWeight func(path string) float64
	// CadenceCap is the maximum number of candidates RuleCandidatesWithPolicy
	// emits per call — the rest are counted as `deferred`, never silently
	// dropped. <= 0 (the zero value) means unbounded — RuleCandidates' own
	// default.
	CadenceCap int
}

// disputedClaimIDs returns the set of claim IDs carrying at least one
// "refuted" claim_event — the literal "from claim events" §3.6/ADR-041
// decision 8 item 1 names for dispute decay.
func disputedClaimIDs(events []core.ClaimEvent) map[string]bool {
	out := map[string]bool{}
	for _, ev := range events {
		if ev.Verdict == "refuted" {
			out[ev.ClaimID] = true
		}
	}
	return out
}

// decayFactor applies CandidatePolicy.DecayPerDispute once per dispute:
// (1-decayPerDispute)^disputes, clamped to [0,1] inputs so a caller-supplied
// decay outside that range never inverts or explodes the score.
func decayFactor(disputes int, decayPerDispute float64) float64 {
	if disputes <= 0 || decayPerDispute <= 0 {
		return 1.0
	}
	d := decayPerDispute
	if d > 1 {
		d = 1
	}
	f := 1.0
	for i := 0; i < disputes; i++ {
		f *= 1 - d
	}
	return f
}

// meanOwnershipWeight is "mean CODEOWNERS coverage of member files": the
// average of weight(path) over every scoped path of every pattern member. A
// nil weight func (no CODEOWNERS wired in) returns 1.0 — never a penalty for
// a repo, or a caller, that has none. A weight func present but with zero
// scoped paths to weight (every member carries an empty Scope) also returns
// 1.0 — there is nothing to report as under-covered.
func meanOwnershipWeight(members []PatternMember, weight func(string) float64) float64 {
	if weight == nil {
		return 1.0
	}
	var sum float64
	var n int
	for _, m := range members {
		for _, path := range m.Scope {
			sum += weight(path)
			n++
		}
	}
	if n == 0 {
		return 1.0
	}
	return sum / float64(n)
}

// patternID is the deterministic identity RuleCandidate.Draft's
// "promoted_from" tag carries — sha256 over (class, normalized shape),
// truncated to 12 hex characters (the same truncation convention SU-17's
// human-authored source id uses): stable across re-derivation, since a
// Pattern's grouping key IS (Class, RuleShape).
func patternID(p Pattern) string {
	h := sha256.Sum256([]byte(string(p.Class) + "\x00" + p.RuleShape))
	return hex.EncodeToString(h[:])[:12]
}

// draftWithProvenance is draftSentence plus SU-15's provenance tag — so a
// human who lifts a candidate into CLAUDE.md/guard.cue keeps, in the text
// itself, which pattern and which producers it came from.
func draftWithProvenance(shape, patternID string, memberCount int, producers []string) string {
	base := draftSentence(shape)
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s [promoted_from: %s, members: %d, producers: [%s]]", base, patternID, memberCount, strings.Join(producers, ", "))
}

// RuleCandidatesWithPolicy is RuleCandidates' SU-15 policy-driven form: same
// derivation (DerivePatterns over native and external findings alike), but
// each candidate's Score is sites × mean ownership weight × dispute decay
// (see CandidatePolicy), sorted Score desc then Class then Shape, and only
// the first policy.CadenceCap survive — the rest are counted in deferred,
// never silently dropped. deferred is always 0 when CadenceCap <= 0.
func RuleCandidatesWithPolicy(claims []core.SemanticClaim, events []core.ClaimEvent, policy CandidatePolicy) (emitted []RuleCandidate, deferred int) {
	minSites := policy.MinSites
	if minSites < 2 {
		minSites = 2
	}
	disputed := disputedClaimIDs(events)

	var out []RuleCandidate
	for _, p := range DerivePatterns(claims, events) {
		sites := p.SiteCount()
		if sites < minSites {
			continue
		}
		members := make([]MemberCitation, len(p.Members))
		seen := map[string]bool{}
		var producers []string
		disputes := 0
		for i, m := range p.Members {
			prod := producerOf(m.Subject)
			members[i] = MemberCitation{ClaimID: m.ClaimID, Subject: m.Subject, Scope: m.Scope, Producer: prod}
			if !seen[prod] {
				seen[prod] = true
				producers = append(producers, prod)
			}
			if disputed[m.ClaimID] {
				disputes++
			}
		}
		sort.Strings(producers)
		weight := meanOwnershipWeight(p.Members, policy.OwnershipWeight)
		score := float64(sites) * weight * decayFactor(disputes, policy.DecayPerDispute)
		pid := patternID(p)
		out = append(out, RuleCandidate{
			Class:                 p.Class,
			Shape:                 p.RuleShape,
			Sites:                 sites,
			Members:               members,
			Producers:             producers,
			CodeChangeResolutions: p.ResolvedCodeChangeCount(),
			Draft:                 draftWithProvenance(p.RuleShape, pid, len(p.Members), producers),
			CheckFamily:           checkFamilyFor(p.Class),
			Score:                 score,
			PatternID:             pid,
			Disputes:              disputes,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Class != out[j].Class {
			return out[i].Class < out[j].Class
		}
		return out[i].Shape < out[j].Shape
	})

	if policy.CadenceCap > 0 && len(out) > policy.CadenceCap {
		deferred = len(out) - policy.CadenceCap
		out = out[:policy.CadenceCap]
	}
	return out, deferred
}

// RuleCandidates derives §3.6's rule candidates from DerivePatterns over
// claims (native and external findings alike). Only patterns with
// >= minSites distinct sites emit a candidate; minSites below 2 clamps to
// DerivePatterns' own admission floor (a single-site "pattern" is one
// finding, not recurrence). Sorted by Sites desc, then Class, then Shape —
// deterministic, never cached. This is RuleCandidatesWithPolicy at the zero
// CandidatePolicy (no decay, no ownership weighting, no cadence cap), so
// Score == Sites exactly and every pre-SU-15 caller sees byte-identical
// ordering and candidate count.
func RuleCandidates(claims []core.SemanticClaim, events []core.ClaimEvent, minSites int) []RuleCandidate {
	out, _ := RuleCandidatesWithPolicy(claims, events, CandidatePolicy{MinSites: minSites})
	return out
}
