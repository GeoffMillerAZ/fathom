package console

// preflight.go — WI-16, WI-17 (docs/design/work-intelligence.md §3 U6, the
// Jev site): the difficulty profile of a plan row and the deterministic
// tier recommendation a published rule table draws from it. Before work
// starts, a row's profile joins facts fathom already stores — the blast
// radius of its anchor, the rules governing it and their levels, the
// certificates its change would re-prove, its zone's interest rate (U2)
// and the effort of its k = 5 nearest proven rows (WI-15) — into one
// model; the recommendation is a table lookup over that profile.
//
// Pure: the caller resolves every fact (a store read, a ledger tail) and
// hands it in with its truth state, so the CLI verb and the console page
// that show a preflight call the same projection (P10). The package never
// starts, assigns or routes work — the recommendation is advisory and the
// harness decides (WI-X3); TestRecommendTierAdvisoryOnly holds this file to
// that (no exec or network import, no function that dispatches).

import (
	"fmt"
	"sort"
	"strings"
)

// PreflightK is how many nearest proven rows a profile reads (design U6).
const PreflightK = 5

// The thresholds of the published rule table (TierRules). The design doc's
// table is generated from the same constants' text: TestTierRuleTable
// Published fails when they drift.
const (
	preflightBlastHigh         = 50  // transitive callers; the change risk receipt's "high"
	preflightBlastMedium       = 10  // transitive callers
	preflightCertsMedium       = 1   // certificates to re-prove
	preflightCertsHigh         = 5   // certificates to re-prove
	preflightInterestHigh      = 2.0 // the change risk receipt's zone-interest threshold
	preflightInterestMedium    = 1.2 // WI-M2's separation threshold
	preflightZoneMinN          = 3   // proven changes before a zone's rate is a reading
	preflightRetriesMedium     = 1   // claimed-unproven readings so far
	preflightRetriesHigh       = 2
	preflightWeightMedium      = 5 // the plan row's weight
	preflightSimilarMinN       = 3 // similar rows before their effort is a reading
	preflightSimilarSlowFactor = 2.0
	preflightSimilarFirstAtLow = 0.5
	preflightBlastWalkLimit    = 5000
)

// Tier is a recommended model tier. The tiers are relative to what the
// harness offers, never a vendor's model names (ADR-011): light is the
// smallest, fastest model and lowest effort setting; heavy the largest and
// highest.
type Tier string

// The recommendation ladder, lowest first.
const (
	TierLight    Tier = "light"
	TierStandard Tier = "standard"
	TierHeavy    Tier = "heavy"
)

// preflightTiers is the ladder, lowest first.
var preflightTiers = []Tier{TierLight, TierStandard, TierHeavy}

// rank is t's position on the ladder, or -1 for none.
func (t Tier) rank() int {
	for i, x := range preflightTiers {
		if x == t {
			return i
		}
	}
	return -1
}

// Governed-rule levels, the change risk receipt's vocabulary
// (internal/review.RuleRef): error is the strictest.
const (
	RuleLevelInfo  = "info"
	RuleLevelWarn  = "warn"
	RuleLevelError = "error"
)

func ruleLevelRank(l string) int {
	switch l {
	case RuleLevelError:
		return 0
	case RuleLevelWarn:
		return 1
	}
	return 2
}

// PreflightRow is the plan row a preflight is for.
type PreflightRow struct {
	Service string `json:"service"`
	Plan    string `json:"plan"`
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	// Symbol and Package are the row's anchor.
	Symbol  string `json:"symbol,omitempty"`
	Package string `json:"package,omitempty"`
	// Weight is the row's plan weight (1 when unset).
	Weight int `json:"weight"`
	// Retries counts the row's claimed-unproven readings so far.
	Retries int `json:"claimed_unproven_retries"`
}

// BlastRadius is the reach of a change to the row's anchor.
type BlastRadius struct {
	Callers     int `json:"transitive_callers"`
	Entrypoints int `json:"entrypoints"`
}

// GovernedRule is one rule governing the row's files and its level.
type GovernedRule struct {
	Rule     string `json:"rule"`
	Level    string `json:"level"`
	File     string `json:"file,omitempty"`
	Citation string `json:"citation,omitempty"`
}

// CertToReprove is one certificate a change to the anchor would re-prove.
type CertToReprove struct {
	Subject string `json:"subject"`
	State   string `json:"state"`
}

// PreflightInput is every fact a profile reads, resolved by the caller. A
// fact the caller could not read is an absent (or errored) Value carrying
// its reason; a zero Value means the caller did not supply it, and the
// profile names that.
type PreflightInput struct {
	Row   PreflightRow
	Blast Value[BlastRadius]
	Rules Value[[]GovernedRule]
	Certs Value[[]CertToReprove]
	// Zones are the zones of the anchor's files.
	Zones []string
	// Interest is U2 over the repository's proven changes.
	Interest InterestModel
	// Index is WI-15's nearest-neighbour index; nil when none was built.
	Index *RowFeatureIndex
	// ExcludeWeight prevents an estimate from selecting its own evidence.
	// The authored weight remains available to the difficulty rules.
	ExcludeWeight bool
}

// LevelCounts counts governing rules per level.
type LevelCounts struct {
	Error int `json:"error"`
	Warn  int `json:"warn"`
	Info  int `json:"info"`
}

// ZoneRate is one of the row's zones with its U2 interest rate.
type ZoneRate struct {
	Zone string         `json:"zone"`
	N    int            `json:"n"`
	Rate Value[float64] `json:"rate"`
}

// SimilarEffort is the measured effort of the row's nearest proven rows.
type SimilarEffort struct {
	K                 int          `json:"k"`
	Rows              []SimilarRow `json:"rows"`
	MedianTokens      float64      `json:"median_tokens"`
	MedianTurns       float64      `json:"median_turns"`
	MedianWallSeconds float64      `json:"median_wall_seconds"`
	FirstAttemptRate  float64      `json:"first_attempt_rate"`
	// FleetMedianTokens is U2's fleet median, the yardstick the similar
	// rows' median is read against; 0 when U2 has none.
	FleetMedianTokens float64 `json:"fleet_median_tokens"`
	// Basis names what "similar" means here, so a reader is never told a
	// neighbour matches on a fact the index does not hold.
	Basis string `json:"basis"`
}

// Feature is one named number of the profile, the vector the decision
// port's Score is asked about.
type Feature struct {
	Name  string         `json:"name"`
	Value Value[float64] `json:"value"`
}

// PreflightFeatureNames names the profile's features, in order.
var PreflightFeatureNames = []string{
	"blast radius: transitive callers", "blast radius: entrypoints reached",
	"governing rules at level error", "governing rules at level warn", "governing rules, all levels",
	"certificates to re-prove", "highest zone interest rate", "claimed-unproven readings so far", "plan row weight",
	"similar rows: median tokens", "similar rows: share proven at the first attempt",
}

// Profile is the difficulty profile of a plan row (WI-16).
type Profile struct {
	Row          PreflightRow           `json:"row"`
	Blast        Value[BlastRadius]     `json:"blast_radius"`
	Rules        Value[[]GovernedRule]  `json:"governing_rules"`
	RuleLevels   LevelCounts            `json:"rule_levels"`
	Certs        Value[[]CertToReprove] `json:"certificates_to_reprove"`
	ZoneInterest []ZoneRate             `json:"zone_interest"`
	MaxZoneRate  Value[float64]         `json:"highest_zone_interest_rate"`
	Similar      Value[SimilarEffort]   `json:"similar_rows"`
	Features     []Feature              `json:"features"`
}

// PreflightBasis states what nearest means (SimilarEffort.Basis).
const PreflightBasis = "proven changes nearest on the facts a row has before work — its zone count and overlap, claimed-unproven readings and plan weight; blast radius, rules and certificates are not stored per past change"

// DifficultyProfile joins a row's facts into its profile. Deterministic:
// every list is sorted and no clock is read, so the same facts in any
// order give the same profile.
func DifficultyProfile(in PreflightInput) Profile {
	p := Profile{Row: in.Row}
	if p.Row.Weight < 1 {
		p.Row.Weight = 1
	}

	p.Blast = in.Blast
	if p.Blast.State == "" {
		p.Blast = Absent[BlastRadius]("the caller resolved no blast radius for the row's anchor")
	}

	p.Rules = in.Rules
	if p.Rules.State == "" {
		p.Rules = Absent[[]GovernedRule]("the caller resolved no governing rules for the row's files")
	}
	if p.Rules.Known() {
		rs := append([]GovernedRule(nil), p.Rules.V...)
		sort.SliceStable(rs, func(i, j int) bool {
			if a, b := ruleLevelRank(rs[i].Level), ruleLevelRank(rs[j].Level); a != b {
				return a < b
			}
			if rs[i].File != rs[j].File {
				return rs[i].File < rs[j].File
			}
			return rs[i].Rule < rs[j].Rule
		})
		p.Rules.V = rs
		for _, r := range rs {
			switch r.Level {
			case RuleLevelError:
				p.RuleLevels.Error++
			case RuleLevelWarn:
				p.RuleLevels.Warn++
			default:
				p.RuleLevels.Info++
			}
		}
	}

	p.Certs = in.Certs
	if p.Certs.State == "" {
		p.Certs = Absent[[]CertToReprove]("the caller resolved no certificates for the row's anchor")
	}
	if p.Certs.Known() {
		cs := append([]CertToReprove(nil), p.Certs.V...)
		sort.SliceStable(cs, func(i, j int) bool {
			if cs[i].Subject != cs[j].Subject {
				return cs[i].Subject < cs[j].Subject
			}
			return cs[i].State < cs[j].State
		})
		p.Certs.V = cs
	}

	// Zones: distinct, sorted, each with its interest rate (a zone with
	// too few proven changes has none — one expensive change is not a rate).
	zoneSet := map[string]bool{}
	for _, z := range in.Zones {
		zoneSet[z] = true
	}
	zones := sortedKeys(zoneSet)
	rates := map[string]ZoneInterest{}
	for _, z := range in.Interest.Zones {
		rates[z.Zone] = z
	}
	p.ZoneInterest = make([]ZoneRate, 0, len(zones))
	maxRate, haveRate := 0.0, false
	for _, z := range zones {
		zr := ZoneRate{Zone: z}
		zi, ok := rates[z]
		switch {
		case !ok:
			zr.Rate = Absent[float64]("no proven change with measured effort in this zone")
		case zi.N < preflightZoneMinN:
			zr.N = zi.N
			zr.Rate = Absent[float64](fmt.Sprintf("%s in this zone; a rate needs %d", nOf(zi.N, "proven change", "proven changes"), preflightZoneMinN))
		default:
			zr.N, zr.Rate = zi.N, zi.Rate
			if zi.Rate.Known() {
				if !haveRate || zi.Rate.V > maxRate {
					maxRate, haveRate = zi.Rate.V, true
				}
			}
		}
		p.ZoneInterest = append(p.ZoneInterest, zr)
	}
	switch {
	case len(zones) == 0:
		p.MaxZoneRate = Absent[float64]("no zone resolved for the row's anchor")
	case !haveRate:
		p.MaxZoneRate = Absent[float64]("no zone of the row has an interest rate yet")
	default:
		p.MaxZoneRate = Measured(round2(maxRate), "U2's interest rate: zone median over fleet median")
	}

	// The k nearest proven rows.
	switch {
	case in.Index == nil || in.Index.Len() == 0:
		p.Similar = Absent[SimilarEffort]("no proven change with measured effort to compare with")
	default:
		near := in.Index.Similar(RowQuery{Zones: zones, Retries: p.Row.Retries, Weight: p.Row.Weight, ExcludeWeight: in.ExcludeWeight}, PreflightK)
		se := SimilarEffort{K: PreflightK, Rows: near, Basis: PreflightBasis}
		if in.ExcludeWeight {
			se.Basis = strings.Replace(PreflightBasis, "claimed-unproven readings and plan weight", "claimed-unproven readings, excluding plan weight for estimation", 1)
		}
		var tok, turns, wall []float64
		first := 0
		for _, r := range near {
			tok, turns, wall = append(tok, r.Tokens), append(turns, r.Turns), append(wall, r.WallSeconds)
			if r.FirstAttempt {
				first++
			}
		}
		se.MedianTokens, se.MedianTurns, se.MedianWallSeconds = round2(quantile(tok, 0.5)), round2(quantile(turns, 0.5)), round2(quantile(wall, 0.5))
		se.FirstAttemptRate = round2(float64(first) / float64(len(near)))
		if in.Interest.FleetMedian.Known() {
			se.FleetMedianTokens = in.Interest.FleetMedian.V
		}
		src := fmt.Sprintf("the %d nearest of the newest %d proven changes (WI-15)", len(near), in.Index.Len())
		if len(near) < PreflightK {
			p.Similar = Partial(se, fmt.Sprintf("only %d proven changes are indexed; %d were asked for", len(near), PreflightK)).WithSource(src)
		} else {
			p.Similar = Present(se, src)
		}
	}

	p.Features = profileFeatures(p)
	return p
}

// profileFeatures is the profile as named numbers, in PreflightFeatureNames
// order; an unknown fact stays absent.
func profileFeatures(p Profile) []Feature {
	num := func(v float64, src string) Value[float64] { return Measured(v, src) }
	absent := func(reason string) Value[float64] { return Absent[float64](reason) }
	vals := make([]Value[float64], len(PreflightFeatureNames))
	if p.Blast.Known() {
		vals[0], vals[1] = num(float64(p.Blast.V.Callers), "the call graph"), num(float64(p.Blast.V.Entrypoints), "the call graph")
	} else {
		vals[0], vals[1] = absent(p.Blast.Reason), absent(p.Blast.Reason)
	}
	if p.Rules.Known() {
		vals[2], vals[3], vals[4] = num(float64(p.RuleLevels.Error), "the governing rules"), num(float64(p.RuleLevels.Warn), "the governing rules"), num(float64(len(p.Rules.V)), "the governing rules")
	} else {
		vals[2], vals[3], vals[4] = absent(p.Rules.Reason), absent(p.Rules.Reason), absent(p.Rules.Reason)
	}
	if p.Certs.Known() {
		vals[5] = num(float64(len(p.Certs.V)), "the certifications layer")
	} else {
		vals[5] = absent(p.Certs.Reason)
	}
	vals[6] = p.MaxZoneRate
	if !p.MaxZoneRate.Known() {
		vals[6] = absent(p.MaxZoneRate.Reason)
	}
	vals[7], vals[8] = num(float64(p.Row.Retries), "the plan row's proof history"), num(float64(p.Row.Weight), "the plan row")
	if p.Similar.Known() {
		vals[9], vals[10] = num(p.Similar.V.MedianTokens, "the similar rows"), num(p.Similar.V.FirstAttemptRate, "the similar rows")
	} else {
		vals[9], vals[10] = absent(p.Similar.Reason), absent(p.Similar.Reason)
	}
	out := make([]Feature, len(vals))
	for i, v := range vals {
		out[i] = Feature{Name: PreflightFeatureNames[i], Value: v}
	}
	return out
}

// BlastRadiusOf walks the callers of start transitively — breadth first,
// each symbol once — and counts them and the entrypoints among them. The
// walk stops at limit symbols (preflightBlastWalkLimit when limit <= 0)
// and says so: a capped count is a lower bound (state partial). callersOf
// and isEntry come from the caller's call graph; a nil isEntry counts no
// entrypoints.
func BlastRadiusOf(start string, callersOf func(string) []string, isEntry func(string) bool, limit int) Value[BlastRadius] {
	if limit <= 0 {
		limit = preflightBlastWalkLimit
	}
	seen := map[string]bool{start: true}
	queue := []string{start}
	var br BlastRadius
	capped := false
walk:
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		next := append([]string(nil), callersOf(cur)...)
		sort.Strings(next)
		for _, c := range next {
			if seen[c] {
				continue
			}
			if br.Callers >= limit {
				capped = true
				break walk
			}
			seen[c] = true
			br.Callers++
			if isEntry != nil && isEntry(c) {
				br.Entrypoints++
			}
			queue = append(queue, c)
		}
	}
	const src = "the index's call graph, transitive callers of the row's anchor"
	if capped {
		return Partial(br, fmt.Sprintf("the walk stopped at %d callers; more exist", limit)).WithSource(src)
	}
	if br == (BlastRadius{}) {
		return Value[BlastRadius]{State: StateZero, Source: src}
	}
	return Present(br, src)
}

// ─── the tier recommendation (WI-17) ────────────────────────────────────────

// TierRuleTableVersion names the published table's revision, so an outcome
// recorded against a recommendation can be read against the table that made
// it. Bump it with any change to TierRules.
const TierRuleTableVersion = 1

// TierRuleDoc is one published rule table row.
type TierRuleDoc struct {
	ID    string `json:"id"`
	When  string `json:"when"`
	Floor Tier   `json:"floor"`
}

// tierRule is a table row with its test. eval reports whether the rule
// could be evaluated (its facts are known) and whether it fired, with the
// evidence it fired on.
type tierRule struct {
	TierRuleDoc
	eval func(p Profile) (evaluated, fired bool, evidence string)
}

// tierRules is the published table, in id order. A rule sets a floor: the
// recommendation is the highest floor among the rules that fired. Rules on
// one fact are banded, so at most one of them fires per fact.
var tierRules = []tierRule{
	{
		TierRuleDoc{"T1", fmt.Sprintf("more than %d transitive callers of the anchor", preflightBlastHigh), TierHeavy},
		func(p Profile) (bool, bool, string) {
			if !p.Blast.Known() {
				return false, false, ""
			}
			n := p.Blast.V.Callers
			return true, n > preflightBlastHigh, nOf(n, "transitive caller", "transitive callers")
		},
	},
	{
		TierRuleDoc{"T2", fmt.Sprintf("%d to %d transitive callers of the anchor", preflightBlastMedium+1, preflightBlastHigh), TierStandard},
		func(p Profile) (bool, bool, string) {
			if !p.Blast.Known() {
				return false, false, ""
			}
			n := p.Blast.V.Callers
			return true, n > preflightBlastMedium && n <= preflightBlastHigh, nOf(n, "transitive caller", "transitive callers")
		},
	},
	{
		TierRuleDoc{"T3", "a governing rule at level error", TierHeavy},
		func(p Profile) (bool, bool, string) {
			if !p.Rules.Known() {
				return false, false, ""
			}
			return true, p.RuleLevels.Error > 0, nOf(p.RuleLevels.Error, "rule at level error", "rules at level error")
		},
	},
	{
		TierRuleDoc{"T4", "a governing rule at level warn and none at level error", TierStandard},
		func(p Profile) (bool, bool, string) {
			if !p.Rules.Known() {
				return false, false, ""
			}
			return true, p.RuleLevels.Warn > 0 && p.RuleLevels.Error == 0, nOf(p.RuleLevels.Warn, "rule at level warn", "rules at level warn")
		},
	},
	{
		TierRuleDoc{"T5", fmt.Sprintf("%d or more certificates to re-prove", preflightCertsHigh), TierHeavy},
		func(p Profile) (bool, bool, string) {
			if !p.Certs.Known() {
				return false, false, ""
			}
			n := len(p.Certs.V)
			return true, n >= preflightCertsHigh, nOf(n, "certificate to re-prove", "certificates to re-prove")
		},
	},
	{
		TierRuleDoc{"T6", fmt.Sprintf("%d to %d certificates to re-prove", preflightCertsMedium, preflightCertsHigh-1), TierStandard},
		func(p Profile) (bool, bool, string) {
			if !p.Certs.Known() {
				return false, false, ""
			}
			n := len(p.Certs.V)
			return true, n >= preflightCertsMedium && n < preflightCertsHigh, nOf(n, "certificate to re-prove", "certificates to re-prove")
		},
	},
	{
		TierRuleDoc{"T7", fmt.Sprintf("a zone's interest rate of %.1f or more (a zone needs at least %d proven changes to have one)", preflightInterestHigh, preflightZoneMinN), TierHeavy},
		func(p Profile) (bool, bool, string) {
			if !p.MaxZoneRate.Known() {
				return false, false, ""
			}
			return true, p.MaxZoneRate.V >= preflightInterestHigh, fmt.Sprintf("highest zone interest rate %.2f", p.MaxZoneRate.V)
		},
	},
	{
		TierRuleDoc{"T8", fmt.Sprintf("a zone's interest rate from %.1f up to %.1f", preflightInterestMedium, preflightInterestHigh), TierStandard},
		func(p Profile) (bool, bool, string) {
			if !p.MaxZoneRate.Known() {
				return false, false, ""
			}
			r := p.MaxZoneRate.V
			return true, r >= preflightInterestMedium && r < preflightInterestHigh, fmt.Sprintf("highest zone interest rate %.2f", r)
		},
	},
	{
		TierRuleDoc{"T9", fmt.Sprintf("%d or more claimed-unproven readings of the row so far", preflightRetriesHigh), TierHeavy},
		func(p Profile) (bool, bool, string) {
			return true, p.Row.Retries >= preflightRetriesHigh, nOf(p.Row.Retries, "claimed-unproven reading", "claimed-unproven readings")
		},
	},
	{
		TierRuleDoc{"T10", fmt.Sprintf("%d claimed-unproven reading of the row so far", preflightRetriesMedium), TierStandard},
		func(p Profile) (bool, bool, string) {
			return true, p.Row.Retries >= preflightRetriesMedium && p.Row.Retries < preflightRetriesHigh, nOf(p.Row.Retries, "claimed-unproven reading", "claimed-unproven readings")
		},
	},
	{
		TierRuleDoc{"T11", fmt.Sprintf("a plan row weight of %d or more", preflightWeightMedium), TierStandard},
		func(p Profile) (bool, bool, string) {
			return true, p.Row.Weight >= preflightWeightMedium, fmt.Sprintf("plan row weight %d", p.Row.Weight)
		},
	},
	{
		TierRuleDoc{"T12", fmt.Sprintf("the %d nearest proven rows' median tokens at least %.1fx the fleet median (needs %d such rows)", PreflightK, preflightSimilarSlowFactor, preflightSimilarMinN), TierStandard},
		func(p Profile) (bool, bool, string) {
			if !p.Similar.Known() || len(p.Similar.V.Rows) < preflightSimilarMinN || p.Similar.V.FleetMedianTokens <= 0 {
				return false, false, ""
			}
			ratio := p.Similar.V.MedianTokens / p.Similar.V.FleetMedianTokens
			return true, ratio >= preflightSimilarSlowFactor, fmt.Sprintf("similar rows' median %.0f tokens is %.2fx the fleet median %.0f", p.Similar.V.MedianTokens, ratio, p.Similar.V.FleetMedianTokens)
		},
	},
	{
		TierRuleDoc{"T13", fmt.Sprintf("fewer than %.0f%% of the %d nearest proven rows proven at the first attempt (needs %d such rows)", preflightSimilarFirstAtLow*100, PreflightK, preflightSimilarMinN), TierStandard},
		func(p Profile) (bool, bool, string) {
			if !p.Similar.Known() || len(p.Similar.V.Rows) < preflightSimilarMinN {
				return false, false, ""
			}
			return true, p.Similar.V.FirstAttemptRate < preflightSimilarFirstAtLow, fmt.Sprintf("%.0f%% of the similar rows were proven at the first attempt", p.Similar.V.FirstAttemptRate*100)
		},
	},
}

// nOf renders "1 rule" / "3 rules" with an irregular plural.
func nOf(n int, one, many string) string { return fmt.Sprintf("%d %s", n, plural(n, one, many)) }

// TierRules is the published rule table, in id order: what each rule
// tests and the floor it sets.
func TierRules() []TierRuleDoc {
	out := make([]TierRuleDoc, len(tierRules))
	for i, r := range tierRules {
		out[i] = r.TierRuleDoc
	}
	return out
}

// FiredRule is one table row that fired, with the evidence it fired on.
type FiredRule struct {
	ID       string `json:"id"`
	When     string `json:"when"`
	Floor    Tier   `json:"floor"`
	Evidence string `json:"evidence"`
}

// Recommendation is the tier the published rule table draws from a
// profile. It is advice: fathom recommends and the harness decides what to
// run (the dispatch non-goal, WI-X3).
type Recommendation struct {
	// Tier is the highest floor among the fired rules; "" when no rule
	// could be evaluated or none fired on incomplete facts (an unknown fact
	// never lowers a tier, and never certifies a light one).
	Tier Tier `json:"tier"`
	// Advisory is always true.
	Advisory bool `json:"advisory"`
	// Complete is true when every rule of the table could be evaluated; a
	// tier on incomplete facts is a floor the unevaluated rules could raise.
	Complete bool `json:"complete"`
	// TableVersion is the published table's revision.
	TableVersion int         `json:"table_version"`
	Fired        []FiredRule `json:"fired"`
	// Unevaluated names each rule whose facts were not known, with why.
	Unevaluated []string `json:"not_evaluated"`
	Note        string   `json:"note"`
}

// PreflightAdvisory is the sentence every recommendation carries.
const PreflightAdvisory = "advisory: fathom recommends and the harness decides what to run; a recommendation starts, assigns and routes nothing"

// recommendTier is the deterministic tier recommendation (WI-17): each
// rule of the published table (TierRules) that could be evaluated on the
// profile either fires with its evidence or does not; the tier is the
// highest floor fired.
func recommendTier(p Profile) Recommendation {
	rec := Recommendation{Advisory: true, TableVersion: TierRuleTableVersion, Fired: []FiredRule{}, Unevaluated: []string{}}
	evaluated := 0
	for _, r := range tierRules {
		ok, fired, evidence := r.eval(p)
		if !ok {
			rec.Unevaluated = append(rec.Unevaluated, r.ID+": "+r.When+" — "+unevaluatedWhy(r.ID, p))
			continue
		}
		evaluated++
		if !fired {
			continue
		}
		rec.Fired = append(rec.Fired, FiredRule{ID: r.ID, When: r.When, Floor: r.Floor, Evidence: evidence})
		if r.Floor.rank() > rec.Tier.rank() {
			rec.Tier = r.Floor
		}
	}
	rec.Complete = len(rec.Unevaluated) == 0
	switch {
	case evaluated == 0:
		rec.Note = "no rule of the table could be evaluated; " + PreflightAdvisory
	case rec.Tier == "" && rec.Complete:
		rec.Tier = TierLight
		rec.Note = "no rule of the table fired; " + PreflightAdvisory
	case rec.Tier == "":
		rec.Note = fmt.Sprintf("no rule fired, but %d could not be evaluated, so no tier is recommended; %s", len(rec.Unevaluated), PreflightAdvisory)
	case !rec.Complete:
		rec.Note = fmt.Sprintf("at least %s: %d rules could not be evaluated and could raise it; %s", rec.Tier, len(rec.Unevaluated), PreflightAdvisory)
	default:
		rec.Note = PreflightAdvisory
	}
	return rec
}

// unevaluatedWhy names the fact a rule needed and did not find.
func unevaluatedWhy(id string, p Profile) string {
	switch {
	case id == "T1", id == "T2":
		return p.Blast.Reason
	case id == "T3", id == "T4":
		return p.Rules.Reason
	case id == "T5", id == "T6":
		return p.Certs.Reason
	case id == "T7", id == "T8":
		return p.MaxZoneRate.Reason
	}
	if p.Similar.Known() {
		return fmt.Sprintf("only %s of the %d needed", nOf(len(p.Similar.V.Rows), "similar row", "similar rows"), preflightSimilarMinN)
	}
	return p.Similar.Reason
}

// ─── the row's own proof history ────────────────────────────────────────────

// rowHistoryOf finds a row's history by service, plan and id. A plan is
// matched by path suffix either way, so a plan named relative to the
// repository and one named absolutely find the same row.
func rowHistoryOf(h History, service, plan, id string) (RowHistory, bool) {
	for _, rh := range h.Rows {
		if rh.ID != id || !strings.EqualFold(rh.Service, service) {
			continue
		}
		if rh.Plan == plan || strings.HasSuffix(plan, "/"+rh.Plan) || strings.HasSuffix(rh.Plan, "/"+plan) {
			return rh, true
		}
	}
	return RowHistory{}, false
}

// RowProofFacts is what a row's proof history says before work starts:
// how many claimed-unproven readings it has had and its plan weight.
// Found is false when the history has never read the row (then Retries is
// 0 and Weight 1, and the caller says the history had no reading).
func RowProofFacts(h History, service, plan, id string) (retries, weight int, found bool) {
	rh, ok := rowHistoryOf(h, service, plan, id)
	if !ok {
		return 0, 1, false
	}
	for _, r := range rh.Readings {
		if r.State == HistUnproven {
			retries++
		}
	}
	return retries, max(rh.Latest().Weight, 1), true
}

// RowFirstAttempt reports whether the row was proven with no
// claimed-unproven reading before its first proven one; known is false
// when the history has no proven reading of it (or never read it).
func RowFirstAttempt(h History, service, plan, id string) (first, known bool) {
	rh, ok := rowHistoryOf(h, service, plan, id)
	if !ok {
		return false, false
	}
	for _, r := range rh.Readings {
		switch r.State {
		case HistUnproven:
			return false, true
		case HistProven:
			return true, true
		}
	}
	return false, false
}
