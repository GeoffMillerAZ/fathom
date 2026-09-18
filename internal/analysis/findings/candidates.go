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
}

// RuleCandidates derives §3.6's rule candidates from DerivePatterns over
// claims (native and external findings alike). Only patterns with
// >= minSites distinct sites emit a candidate; minSites below 2 clamps to
// DerivePatterns' own admission floor (a single-site "pattern" is one
// finding, not recurrence). Sorted by Sites desc, then Class, then Shape —
// deterministic, never cached.
func RuleCandidates(claims []core.SemanticClaim, events []core.ClaimEvent, minSites int) []RuleCandidate {
	if minSites < 2 {
		minSites = 2
	}
	var out []RuleCandidate
	for _, p := range DerivePatterns(claims, events) {
		sites := p.SiteCount()
		if sites < minSites {
			continue
		}
		members := make([]MemberCitation, len(p.Members))
		seen := map[string]bool{}
		var producers []string
		for i, m := range p.Members {
			prod := producerOf(m.Subject)
			members[i] = MemberCitation{ClaimID: m.ClaimID, Subject: m.Subject, Scope: m.Scope, Producer: prod}
			if !seen[prod] {
				seen[prod] = true
				producers = append(producers, prod)
			}
		}
		sort.Strings(producers)
		out = append(out, RuleCandidate{
			Class:                 p.Class,
			Shape:                 p.RuleShape,
			Sites:                 sites,
			Members:               members,
			Producers:             producers,
			CodeChangeResolutions: p.ResolvedCodeChangeCount(),
			Draft:                 draftSentence(p.RuleShape),
			CheckFamily:           checkFamilyFor(p.Class),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sites != out[j].Sites {
			return out[i].Sites > out[j].Sites
		}
		if out[i].Class != out[j].Class {
			return out[i].Class < out[j].Class
		}
		return out[i].Shape < out[j].Shape
	})
	return out
}
