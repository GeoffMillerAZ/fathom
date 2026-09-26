package semantic

// explain.go — OC-32/OC-33 (docs/design/observatory-console.md §3 "Serving
// explainer", ADR-045): the obligations algorithm made visible. Serving
// answers "what governs this file"; ExplainObligations answers "why this
// rule and not that one": every scope-matched candidate with its scope tier
// and its place in the ranking, the cap and the nearest-tier reservation,
// the served list, and each displaced claim with the reason it lost.
//
// The explanation is not a second implementation of serving. It runs the
// one engine (obligationsCoreTrace) with a trace attached, so the served
// list it reports IS what ObligationsForLayer returns for the same inputs —
// TestExplainObligations_DisplacedReasons pins that equality — and the key
// that decided each lost comparison comes from compareScored, the same
// comparison serving sorts by. A nil trace records nothing, so every
// serving caller's output is unchanged by this file.
//
// It never calls the decision port: an explanation is read by a person on
// a page or at a terminal, and rendering must not spend or leave the
// machine. RankedBy therefore always reports "none"; when the daemon's
// serving re-ranks within tiers ([decide] rank_within_tier), a caller says
// so beside the explanation instead of silently showing a different order.

import (
	"context"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Scope tiers, nearest first — governingScope.name()'s vocabulary, the four
// columns the serving explainer draws.
const (
	ScopeTierExactFile = "exact_file"
	ScopeTierPackage   = "package"
	ScopeTierParentDir = "parent_dir"
	ScopeTierRoot      = "root"
)

// ScopeTiers is the four tiers in proximity order.
var ScopeTiers = []string{ScopeTierExactFile, ScopeTierPackage, ScopeTierParentDir, ScopeTierRoot}

// Ranking keys, in the order compareScored consults them.
const (
	RankKeyFreshness   = "freshness"
	RankKeyRole        = "role"
	RankKeyScopeTier   = "scope_tier"
	RankKeyRerank      = "rerank"
	RankKeyRelevance   = "relevance"
	RankKeySpecificity = "specificity"
	RankKeyText        = "text"
	RankKeyClaimID     = "claim_id"
)

// Displacement reasons.
const (
	// DisplacedDuplicate: the same normalized statement is already served
	// (or already ranked) — OC-33; never counted as displaced.
	DisplacedDuplicate = "duplicate"
	// DisplacedTier: a nearer scope tier filled the cap.
	DisplacedTier = "tier"
	// DisplacedRank: the same scope tier, ranked lower (relevance to the
	// file, scope specificity, then rule text).
	DisplacedRank = "rank"
	// DisplacedFreshness: fresh rules rank ahead of stale or contradicted.
	DisplacedFreshness = "freshness"
	// DisplacedRole: governs rules fill the budget before claims rules.
	DisplacedRole = "role"
	// DisplacedReserve: the nearest tier's reserved slots (O57) took it.
	DisplacedReserve = "reserve"
)

// explainTrace is what obligationsCoreTrace records for ExplainObligations.
type explainTrace struct {
	ranked    []scored          // every matched candidate, in ranking order, duplicates included
	src       map[string]string // claim id → primary citation
	dupOf     map[string]string // duplicate claim id → the kept claim id
	reserved  map[string]bool   // claim ids given a reserved nearest-tier slot
	reserveN  int
	servedIDs []string // claim ids in served order
}

// ExplainCandidate is one scope-matched claim and what serving did with it.
type ExplainCandidate struct {
	ClaimID string   `json:"claim_id"`
	Rule    string   `json:"rule"`
	Src     string   `json:"src"`
	Layer   string   `json:"layer"`
	Kind    string   `json:"kind,omitempty"`
	Tier    string   `json:"tier"`   // evidence tier
	Status  string   `json:"status"` // fresh | stale | contradicted
	Role    string   `json:"role"`   // governs | claims
	Scope   []string `json:"scope"`
	// ScopeTier is how near the claim's closest scope glob reaches the file
	// (exact_file, package, parent_dir, root); "root" for every candidate
	// of a root-scope question.
	ScopeTier string `json:"scope_tier"`
	// Rank is the 1-based position in serving's ranking, before the cap.
	Rank int `json:"rank"`
	// Served is true when the rule reached the served list; Position is its
	// 1-based place in that compiled list. The obligations tool prepends
	// raw-fallback rows (ADR-037) before it, so the agent reads the rule
	// after those: a page showing both numbers the compiled rules by
	// Position and the raw rows apart, never one rule at two places.
	Served   bool `json:"served"`
	Position int  `json:"position,omitempty"`
	// Reserved marks a rule that took one of the nearest tier's reserved
	// slots.
	Reserved  bool          `json:"reserved,omitempty"`
	Displaced *Displacement `json:"displaced,omitempty"`
}

// Displacement is why a candidate was not served.
type Displacement struct {
	Reason string `json:"reason"`
	Why    string `json:"why"`
	// By is the claim that held the place: the served duplicate, or the
	// lowest-ranked served rule that outranked this one.
	By string `json:"by,omitempty"`
}

// ExplainTier summarizes one scope tier: the serving explainer's column.
type ExplainTier struct {
	Tier       string `json:"tier"`
	Candidates int    `json:"candidates"`
	Served     int    `json:"served"`
	Displaced  int    `json:"displaced"`
	Duplicates int    `json:"duplicates"`
}

// ObligationsExplanation is ExplainObligations' answer.
type ObligationsExplanation struct {
	Service string `json:"service"`
	// File is the edited path; "" asks the root-scope question.
	File  string `json:"file"`
	Layer string `json:"layer"`
	// Cap is the served budget; ReservedSlots is how many of its first
	// slots the nearest scope tier holds (0 when nothing was cut, or for a
	// root-scope question).
	Cap           int    `json:"cap"`
	ReservedSlots int    `json:"reserved_slots"`
	RankedBy      string `json:"ranked_by"`
	// Ranking names the keys serving ranks by, in the order it consults
	// them.
	Ranking    []string           `json:"ranking"`
	Eligible   int                `json:"eligible"`
	Deduped    int                `json:"deduped"`
	Displaced  int                `json:"displaced"`
	Tiers      []ExplainTier      `json:"tiers"`
	Candidates []ExplainCandidate `json:"candidates"`
	// Served is exactly what serving returns for the same inputs
	// (ObligationsForLayer / RootObligationsForLayer).
	Served Obligations `json:"served"`
}

// ExplainObligations explains serving for (serviceID, file): every candidate
// with its scope tier, the ranking and the cap, the served list and each
// displaced claim's reason. file "" asks the root-scope question
// (RootObligationsForLayer). layer "" is the obligations tool's own default,
// ObligationLayerRules — what the edit hook serves an agent.
func ExplainObligations(ctx context.Context, st store.Store, serviceID, file string, max int, layer ObligationLayerFilter) (ObligationsExplanation, error) {
	if layer == "" {
		layer = ObligationLayerRules
	}
	if max <= 0 || max > MaxObligations {
		max = MaxObligations
	}
	match := fileScopeMatch(file)
	if file == "" {
		match = rootScopeMatch
	}
	trace := &explainTrace{dupOf: map[string]string{}}
	served, err := obligationsCoreTrace(ctx, st, serviceID, file, max, match, layer, nil, ObligationsOptions{}, trace)
	out := ObligationsExplanation{
		Service: serviceID, File: file, Layer: string(layer), Cap: max,
		RankedBy: served.RankedBy, Served: served,
		Ranking:    []string{RankKeyFreshness, RankKeyRole, RankKeyScopeTier, RankKeyRelevance, RankKeySpecificity, RankKeyText},
		Candidates: []ExplainCandidate{},
	}
	if file == "" {
		out.Ranking = []string{RankKeyFreshness, RankKeyRole, RankKeySpecificity, RankKeyText}
	}
	if err != nil {
		return out, err
	}
	out.ReservedSlots = trace.reserveN
	out.Displaced = served.Displaced
	out.Deduped = len(trace.dupOf)
	out.Eligible = len(trace.ranked)

	position := make(map[string]int, len(trace.servedIDs))
	for i, id := range trace.servedIDs {
		position[id] = i + 1
	}
	rankOf := make(map[string]int, len(trace.ranked))
	for i, m := range trace.ranked {
		rankOf[m.cl.ClaimID] = i + 1
	}
	tierOf := func(m scored) string {
		if file == "" {
			return ScopeTierRoot
		}
		return m.governing.name()
	}
	byTier := map[string]*ExplainTier{}
	for _, t := range ScopeTiers {
		byTier[t] = &ExplainTier{Tier: t}
	}
	for i, m := range trace.ranked {
		role := RoleGoverns
		if m.claims {
			role = RoleClaims
		}
		status := string(m.cl.Status)
		c := ExplainCandidate{
			ClaimID: m.cl.ClaimID, Rule: m.cl.Rule, Src: trace.src[m.cl.ClaimID],
			Layer: m.cl.Layer, Tier: string(m.cl.Tier), Status: status, Role: role,
			Scope: append([]string{}, m.cl.Scope...), ScopeTier: tierOf(m), Rank: i + 1,
			Reserved: trace.reserved[m.cl.ClaimID],
		}
		if m.cl.Kind != KindObligation {
			c.Kind = m.cl.Kind
		}
		if p := position[m.cl.ClaimID]; p > 0 {
			c.Served, c.Position = true, p
		} else {
			c.Displaced = explainDisplacement(trace, i, file, rankOf, position)
		}
		tier := byTier[c.ScopeTier]
		if tier == nil {
			tier = &ExplainTier{Tier: c.ScopeTier}
			byTier[c.ScopeTier] = tier
		}
		tier.Candidates++
		switch {
		case c.Served:
			tier.Served++
		case c.Displaced != nil && c.Displaced.Reason == DisplacedDuplicate:
			tier.Duplicates++
		default:
			tier.Displaced++
		}
		out.Candidates = append(out.Candidates, c)
	}
	for _, t := range ScopeTiers {
		if file == "" && t != ScopeTierRoot {
			continue
		}
		out.Tiers = append(out.Tiers, *byTier[t])
	}
	return out, nil
}

// explainDisplacement names why trace.ranked[i] was not served.
func explainDisplacement(trace *explainTrace, i int, file string, rankOf, position map[string]int) *Displacement {
	d := trace.ranked[i]
	if kept, ok := trace.dupOf[d.cl.ClaimID]; ok {
		why := fmt.Sprintf("the same statement as the rule ranked #%d", rankOf[kept])
		if p := position[kept]; p > 0 {
			why += fmt.Sprintf(", served at #%d", p)
		} else {
			why += ", itself displaced"
		}
		return &Displacement{Reason: DisplacedDuplicate, By: kept, Why: why + " — one statement is served once"}
	}
	// A served rule ranked BELOW this one took its place: the budget's
	// partitions, not the ranking, decided.
	var reservedBelow, governsBelow bool
	for _, m := range trace.ranked[i+1:] {
		if position[m.cl.ClaimID] == 0 {
			continue
		}
		if trace.reserved[m.cl.ClaimID] && m.governing.tier != d.governing.tier {
			reservedBelow = true
		}
		if !m.claims {
			governsBelow = true
		}
	}
	switch {
	case reservedBelow:
		return &Displacement{Reason: DisplacedReserve, Why: fmt.Sprintf("the nearest scope tier holds the first %d slots (O57); the rest of the cap went to higher-ranked rules", trace.reserveN)}
	case d.claims && governsBelow:
		return &Displacement{Reason: DisplacedRole, Why: "governs rules (human-authored guides and ADRs) fill the cap before claims rules (BP-18)"}
	}
	// Otherwise the lowest-ranked served rule above it outranked it on one
	// key: name that key.
	for j := i - 1; j >= 0; j-- {
		k := trace.ranked[j]
		p := position[k.cl.ClaimID]
		if p == 0 {
			continue
		}
		_, key := compareScored(k, d, file, nil)
		by := k.cl.ClaimID
		switch key {
		case RankKeyFreshness:
			return &Displacement{Reason: DisplacedFreshness, By: by, Why: fmt.Sprintf("fresh rules rank ahead of %s ones, and the cap filled first", d.cl.Status)}
		case RankKeyRole:
			return &Displacement{Reason: DisplacedRole, By: by, Why: "governs rules (human-authored guides and ADRs) rank ahead of claims rules, and the cap filled first"}
		case RankKeyScopeTier:
			return &Displacement{Reason: DisplacedTier, By: by, Why: fmt.Sprintf("nearer-scope rules (scoped to %s) filled the cap before this rule, scoped to %s", tierWords(k.governing.name()), tierWords(d.governing.name()))}
		default:
			return &Displacement{Reason: DisplacedRank, By: by, Why: fmt.Sprintf("same scope tier as the rule served at #%d; ranked lower on %s", p, rankKeyWords(key))}
		}
	}
	return &Displacement{Reason: DisplacedRank, Why: "ranked below the cap"}
}

// compareScored is serving's ORDER contract (BP-17 freshness, BP-18 role,
// VB-8 proximity with O3's cross-tier promotion, DP-7's re-rank, O2's
// lexical relevance, then glob specificity, rule text and claim id): it
// reports whether a ranks before b and the key that decided it.
// obligationsCore's less() is this function's first result.
func compareScored(a, b scored, outFile string, decideRelevance map[string]float64) (less bool, key string) {
	if af, bf := a.cl.Status == core.ClaimFresh, b.cl.Status == core.ClaimFresh; af != bf {
		return af, RankKeyFreshness // BP-17/D5: fresh before stale or contradicted
	}
	if a.claims != b.claims {
		return !a.claims, RankKeyRole // BP-18/D1: governs before claims
	}
	// VB-8 (ADR-035 decision 2): proximity to the edited path is the
	// PRIMARY tie-break inside a role/freshness tier — exact file beats
	// package beats a named ancestor directory beats the repo root. Only
	// meaningful for a file-scoped call (outFile != ""); a root-scope
	// answer has no path to be proximate to, so every candidate carries the
	// same noGoverningScope and this comparison is always a no-op there.
	// wave O3, item 1 (wave O4, item 1 narrowed the bar): crossTierCloser
	// lets a farther-tier claim with STRONG lexical relevance leapfrog a
	// nearer-tier claim that is not itself strong — see its own doc
	// comment; every other case reproduces plain closerThan exactly.
	if outFile != "" && a.governing != b.governing {
		return crossTierCloser(a.governing, b.governing, a.strong, b.strong), RankKeyScopeTier
	}
	// DP-7 (ADR-042 decision 2): STRICTLY INSIDE a governing-scope tier, a
	// decision-port re-rank's noul answer decides NEXT, before the lexical
	// relevance tie-break below — but only when BOTH sides cleared I42's
	// unsure-band floor (usableNoul): an absent answer, or one below
	// decision.UnsureBelow, falls straight through to the EXACT same
	// comparison this rule would have made before decideRelevance ever
	// existed — "never moves a rule". With no rerank ever run,
	// decideRelevance is nil and this block is always a no-op.
	if outFile != "" && decideRelevance != nil {
		if ra, oka := usableNoul(a.cl.ClaimID, decideRelevance); oka {
			if rb, okb := usableNoul(b.cl.ClaimID, decideRelevance); okb && ra != rb {
				return ra > rb, RankKeyRerank
			}
		}
	}
	// wave O2 (vs-pilot-6 second reading §5): STRICTLY INSIDE a
	// governing-scope tier, the lexical relevance score decides NEXT —
	// several same-tier, same-freshness, same-role claims (a nested guide's
	// own several sections, all floored to the same directory) no longer
	// fall straight to the byte-length/rule-text tie-break below, which is
	// closer to alphabetical/document order than to "which rule actually
	// names this file".
	if outFile != "" && a.relevance != b.relevance {
		return a.relevance > b.relevance, RankKeyRelevance
	}
	// The keyword score (byte length of the matched glob) breaks ties
	// INSIDE a tier — two claims scoped to the same directory still need a
	// deterministic order.
	if a.specificity != b.specificity {
		return a.specificity > b.specificity, RankKeySpecificity
	}
	if a.cl.Rule != b.cl.Rule {
		return a.cl.Rule < b.cl.Rule, RankKeyText
	}
	return a.cl.ClaimID < b.cl.ClaimID, RankKeyClaimID
}

// dedupeStatements keeps the first claim of every normalized statement in
// matched (already in ranking order) and records each dropped duplicate in
// trace (when non-nil). A statement that normalizes to nothing is never
// treated as a duplicate of another.
func dedupeStatements(matched []scored, trace *explainTrace) []scored {
	seen := make(map[string]string, len(matched))
	out := make([]scored, 0, len(matched))
	for _, m := range matched {
		key := NormalizedStatement(m.cl.Rule)
		if kept, dup := seen[key]; dup && key != "" {
			if trace != nil {
				trace.dupOf[m.cl.ClaimID] = kept
			}
			continue
		}
		seen[key] = m.cl.ClaimID
		out = append(out, m)
	}
	return out
}

// NormalizedStatement is the identity serving dedupes rules by (OC-33):
// case-folded, with Markdown code and emphasis marks (backticks, asterisks)
// removed, whitespace collapsed and trailing punctuation dropped — so
// "Inject clocks (`now func() time.Time`); never call `time.Now()`." and
// "Inject clocks (now func() time.Time); never call time.Now()" are one
// statement. Words, identifiers and their order are never touched.
func NormalizedStatement(rule string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(rule) {
		switch {
		case r == '`' || r == '*':
			continue
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			space = b.Len() > 0
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), ".;:,! ")
}

// tierWords renders a scope tier for a person.
func tierWords(tier string) string {
	switch tier {
	case ScopeTierExactFile:
		return "this file"
	case ScopeTierPackage:
		return "its package directory"
	case ScopeTierParentDir:
		return "an ancestor directory"
	case ScopeTierRoot:
		return "the repository root"
	}
	return tier
}

// rankKeyWords renders an in-tier ranking key for a person.
func rankKeyWords(key string) string {
	switch key {
	case RankKeyRerank:
		return "the decision-port re-rank"
	case RankKeyRelevance:
		return "lexical relevance to this file"
	case RankKeySpecificity:
		return "scope specificity (a broader glob)"
	case RankKeyText:
		return "rule text order (an exact tie on every other key)"
	case RankKeyClaimID:
		return "claim id order (an exact tie on every other key)"
	}
	return key
}
