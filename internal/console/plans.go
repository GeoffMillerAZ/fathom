package console

// plans.go — the proof ledger's projections (vision 35 §4.4, design §3
// "Proof ledger"; plan rows OC-21, OC-22, SD-2, SD-3). A plan row's proof
// state is what the verify engine computed, never the checkbox alone:
// every projection here reads rows joined with their verdicts (PlanRowFact)
// and a row with no verdict yet is "not yet verified", never proven.
//
// One rollup feeds every surface that counts plan rows: `fathom plans
// status --json` (cmd/fathom/plans.go), the portfolio's proof bars and the
// plan page all call RollupPlan, so a verb and a page cannot disagree
// (P10, OC-8).

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The proof ledger's shared metrics (OC-9): the fleet's claimed-but-
// unproven rows (the SD-2 headline) and its proven rows, each the
// portfolio's own value (PlanPortfolio) wherever a page shows it.
const (
	MetricPlansUnproven MetricID = "plans.claimed_unproven"
	MetricPlansProven   MetricID = "plans.proven"
)

// CriterionFact is one checked clause of a row's contract, with its
// evidence, exactly as the verify engine reported it.
type CriterionFact struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// PlanRowFact is one authored plan row joined with its verdict — the join
// every plans verb computes from (cmd/fathom's planRow, made typed and
// UI-free here).
type PlanRowFact struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Line       int    `json:"line,omitempty"`
	Claimed    bool   `json:"claimed,omitempty"`
	Skipped    bool   `json:"skipped,omitempty"`
	InProgress bool   `json:"in_progress,omitempty"`
	// Op marks an "(op)" measurement row (docs/plans/README.md): it
	// closes with evidence of a measurement rather than code.
	Op          bool   `json:"op,omitempty"`
	Disposition string `json:"disposition,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Trigger     string `json:"trigger,omitempty"`

	// The contract, as authored.
	Symbol        string   `json:"symbol,omitempty"`
	Package       string   `json:"package,omitempty"`
	ReachableFrom string   `json:"reachable_from,omitempty"`
	SkipTest      bool     `json:"skip_test,omitempty"`
	SkipReason    string   `json:"skip,omitempty"`
	CertifiedBy   []string `json:"certified_by,omitempty"`
	Oracle        string   `json:"oracle,omitempty"`
	OraclePath    string   `json:"oracle_path,omitempty"`
	OraclePattern string   `json:"oracle_pattern,omitempty"`
	Gone          string   `json:"gone,omitempty"`
	// OracleEq, OracleMin and OracleMax are a lexical or gone: row's count
	// bound (CV-29): the gate decides pass or fail on the match count
	// against them, so "matches nothing" is a pass for an `eq: 0` row.
	OracleEq  *int `json:"oracle_eq,omitempty"`
	OracleMin *int `json:"oracle_min,omitempty"`
	OracleMax *int `json:"oracle_max,omitempty"`

	// The verdict. Verified is false while no verdict has been computed
	// for the current watermark; Status is then empty.
	Verified bool            `json:"verified"`
	Status   string          `json:"status,omitempty"` // pass | fail | unknown
	Label    string          `json:"label,omitempty"`
	Criteria []CriterionFact `json:"criteria,omitempty"`
}

// IsOpTitle reports whether a row title marks an "(op)" measurement row.
func IsOpTitle(title string) bool {
	return strings.HasPrefix(strings.TrimSpace(title), "(op)")
}

// Contract is what the gate checks for a row, named by kind for a card and
// spelled out field by field for the evidence drawer.
type Contract struct {
	Kind   string          `json:"kind"`
	Fields []ContractField `json:"fields"`
}

// ContractField is one authored contract field.
type ContractField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// The contract kinds a card names.
const (
	ContractReach       = "symbol + test reach"
	ContractSymbol      = "symbol present (test waived)"
	ContractLexical     = "lexical oracle"
	ContractLedger      = "ledger oracle"
	ContractGone        = "gone"
	ContractCertified   = "certified_by"
	ContractDisposition = "disposition"
	ContractNone        = "no contract"
)

// ContractOf names a row's contract and lists its authored fields.
func ContractOf(r PlanRowFact) Contract {
	var c Contract
	add := func(name, value string) {
		if value != "" {
			c.Fields = append(c.Fields, ContractField{Name: name, Value: value})
		}
	}
	switch {
	case r.Skipped:
		c.Kind = ContractDisposition
		add("disposition", orText(r.Disposition, "none — skipped without a disposition"))
		add("reason", r.Reason)
		add("trigger", r.Trigger)
	case r.Gone != "":
		c.Kind = ContractGone
		add("gone", r.Gone)
		add("package", r.Package)
		add("count", BoundText(r.OracleEq, r.OracleMin, r.OracleMax))
	case r.Oracle == "lexical":
		c.Kind = ContractLexical
		add("path", r.OraclePath)
		add("pattern", r.OraclePattern)
		add("count", BoundText(r.OracleEq, r.OracleMin, r.OracleMax))
	case r.Oracle != "":
		c.Kind = ContractLedger
		add("oracle", r.Oracle)
		add("path", r.OraclePath)
	case len(r.CertifiedBy) > 0:
		c.Kind = ContractCertified
		add("symbol", r.Symbol)
		add("certified_by", strings.Join(r.CertifiedBy, ", "))
	case r.Symbol != "" && r.SkipTest:
		c.Kind = ContractSymbol
		add("symbol", r.Symbol)
		add("package", r.Package)
		add("skip", r.SkipReason)
	case r.Symbol != "":
		c.Kind = ContractReach
		add("symbol", r.Symbol)
		add("package", r.Package)
		add("reachable_from", r.ReachableFrom)
	default:
		c.Kind = ContractNone
		add("skip", r.SkipReason)
	}
	if !r.Skipped && r.SkipReason != "" && c.Kind != ContractSymbol && c.Kind != ContractNone {
		add("skip", r.SkipReason)
	}
	return c
}

// BoundText states a row's authored count bound (CV-29) in words: "exactly
// 0 (the pattern must not match)", "at least 2", "at most 3", "2 to 5"; ""
// when the row authors none (the oracle's own default then holds).
func BoundText(eq, min, max *int) string {
	switch {
	case eq != nil && *eq == 0:
		return "exactly 0 matches (the pattern must not match)"
	case eq != nil:
		return fmt.Sprintf("exactly %d matches", *eq)
	case min != nil && max != nil:
		return fmt.Sprintf("%d to %d matches", *min, *max)
	case min != nil:
		return fmt.Sprintf("at least %d matches", *min)
	case max != nil:
		return fmt.Sprintf("at most %d matches", *max)
	}
	return ""
}

// BoundMet reports whether a lexical oracle's match count meets the row's
// bound — the verify engine's rule (lexicalBoundsVerdict): eq exact, min and
// max a range, and with none authored at least one match.
func BoundMet(count int, eq, min, max *int) bool {
	switch {
	case eq != nil:
		return count == *eq
	case min != nil || max != nil:
		return (min == nil || count >= *min) && (max == nil || count <= *max)
	}
	return count >= 1
}

// PlanRollup is one plan's computed rollup — `fathom plans status --json`'s
// wire shape, field for field (cmd/fathom's planRollup is this type). Each
// row lands in exactly one of the six buckets; OpProven and OpOpen are the
// "(op)" measurement rows inside Proven and Open (subsets, not buckets).
type PlanRollup struct {
	Plan            string `json:"plan"`
	Total           int    `json:"total"`
	Proven          int    `json:"proven"`
	ClaimedUnproven int    `json:"claimed_unproven"`
	// MeasuredStale (PC-5): claimed rows the daemon could not prove while
	// its binary was stale — counted here, never as unproven.
	MeasuredStale       int `json:"measured_stale,omitempty"`
	Open                int `json:"open"`
	DeferredWithTrigger int `json:"deferred_with_trigger"`
	SkippedOther        int `json:"skipped_other"`
	// OpProven and OpOpen are the measurement rows among Proven and Open.
	OpProven int `json:"op_proven,omitempty"`
	OpOpen   int `json:"op_open,omitempty"`
}

// RollupPlan computes one plan's rollup. Bucket precedence (each row lands
// in exactly one bucket):
//  1. skipped with disposition deferred → DeferredWithTrigger
//  2. skipped otherwise → SkippedOther
//  3. claimed and verified pass → Proven
//  4. claimed, not pass, daemon measured stale → MeasuredStale
//  5. claimed, not pass (fail, unknown, or no verdict) → ClaimedUnproven
//  6. neither claimed nor skipped ([ ] open or [~] in progress) → Open
func RollupPlan(plan string, rows []PlanRowFact, daemonStale bool) PlanRollup {
	r := PlanRollup{Plan: plan, Total: len(rows)}
	for _, row := range rows {
		switch {
		case row.Skipped && row.Disposition == "deferred":
			r.DeferredWithTrigger++
		case row.Skipped:
			r.SkippedOther++
		case row.Claimed:
			switch {
			case row.Status == "pass":
				r.Proven++
				if row.Op {
					r.OpProven++
				}
			case daemonStale:
				r.MeasuredStale++
			default:
				r.ClaimedUnproven++
			}
		default:
			r.Open++
			if row.Op {
				r.OpOpen++
			}
		}
	}
	return r
}

// Row states: where a row sits on the board and in the proof bar.
const (
	RowProven         = "proven"
	RowOpMeasured     = "op-measured"
	RowClaimedFailing = "claimed-failing"
	RowClaimedUnknown = "claimed-unknown"
	RowMeasuredStale  = "measured-stale"
	RowPending        = "not-yet-verified"
	RowOpen           = "open"
	RowInProgress     = "in-progress"
	RowOpWaiting      = "op-waiting"
	RowDeferred       = "deferred"
	RowSkipped        = "skipped"
)

// RowState classifies one row, with the same precedence as RollupPlan; a
// claimed row with no verdict yet is "not yet verified", never proven.
func RowState(r PlanRowFact, daemonStale bool) string {
	switch {
	case r.Skipped && r.Disposition == "deferred":
		return RowDeferred
	case r.Skipped:
		return RowSkipped
	case r.Claimed:
		switch {
		case !r.Verified:
			return RowPending
		case r.Status == "pass" && r.Op:
			return RowOpMeasured
		case r.Status == "pass":
			return RowProven
		case daemonStale:
			return RowMeasuredStale
		case r.Status == "fail":
			return RowClaimedFailing
		default:
			return RowClaimedUnknown
		}
	case r.Op:
		return RowOpWaiting
	case r.InProgress:
		return RowInProgress
	default:
		return RowOpen
	}
}

// BarSegment is one state's share of a proof bar. Role is a kit token role;
// the label and count always travel with it, so color is never the only
// channel.
type BarSegment struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Role  string `json:"role"`
	Count int    `json:"count"`
}

// ProofBar is a plan's (or a fleet's) rows by proof state, in the fixed
// segment order: proven first, then what blocks proof, then what is not in
// play. Segments with no rows are omitted.
type ProofBar struct {
	Total    int          `json:"total"`
	Segments []BarSegment `json:"segments"`
}

// proofBarOrder is the fixed segment order and its roles.
var proofBarOrder = []BarSegment{
	{Key: RowProven, Label: "proven, not counting (op) measurements", Role: "positive"},
	{Key: RowOpMeasured, Label: "measurement rows (op) measured", Role: "info"},
	{Key: RowClaimedFailing, Label: "claimed but failing", Role: "danger"},
	{Key: RowClaimedUnknown, Label: "claimed, the gate cannot decide", Role: "caution"},
	{Key: RowMeasuredStale, Label: "claimed, daemon measured stale", Role: "caution-soft"},
	{Key: RowPending, Label: "claimed, not yet verified", Role: "ink-faint"},
	{Key: RowInProgress, Label: "in progress", Role: "accent"},
	{Key: RowOpen, Label: "open", Role: "line-strong"},
	{Key: RowOpWaiting, Label: "measurement rows (op) waiting", Role: "info-soft"},
	{Key: RowDeferred, Label: "deferred with a trigger", Role: "ink-muted"},
	{Key: RowSkipped, Label: "refused or skipped", Role: "line"},
}

// ProofBarOf counts rows by state into a bar.
func ProofBarOf(rows []PlanRowFact, daemonStale bool) ProofBar {
	counts := map[string]int{}
	for _, r := range rows {
		counts[RowState(r, daemonStale)]++
	}
	return ProofBarFromCounts(counts)
}

// ProofBarFromCounts builds a bar from per-state counts, in the fixed
// segment order with the fixed roles.
func ProofBarFromCounts(counts map[string]int) ProofBar {
	var bar ProofBar
	for _, seg := range proofBarOrder {
		if n := counts[seg.Key]; n > 0 {
			seg.Count = n
			bar.Segments = append(bar.Segments, seg)
			bar.Total += n
		}
	}
	return bar
}

// Count returns the bar's count for one segment key.
func (b ProofBar) Count(key string) int {
	for _, s := range b.Segments {
		if s.Key == key {
			return s.Count
		}
	}
	return 0
}

// PlanFacts is one plan as a projection sees it: its rows (authored, and
// verdicts when the engine has run at the current watermark) and whether
// the verdicts are there yet.
type PlanFacts struct {
	Service string `json:"service"`
	// Path is the plan file as discovered (absolute); Rel is the
	// repo-relative path the gate names (docs/plans/x.md).
	Path  string        `json:"path"`
	Rel   string        `json:"rel"`
	Title string        `json:"title"`
	Rows  []PlanRowFact `json:"rows"`
	// Verified is true when every row carries the verdict computed at the
	// current watermarks. Pending is true while that computation is queued
	// or running; Err names why it failed.
	Verified    bool   `json:"verified"`
	Pending     bool   `json:"pending,omitempty"`
	Err         string `json:"error,omitempty"`
	DaemonStale bool   `json:"daemon_stale,omitempty"`
	// StoreCommit is the index watermark the verdicts were computed at.
	StoreCommit string    `json:"store_commit,omitempty"`
	VerifiedAt  time.Time `json:"verified_at,omitzero"`
}

// Slug is the plan's route segment: its file name without ".md".
func (p PlanFacts) Slug() string { return PlanSlug(p.Path) }

// PlanSlug is a plan file's route segment.
func PlanSlug(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".md")
}

// ServicePlans is one registered service's plans.
type ServicePlans struct {
	Service string      `json:"service"`
	Root    string      `json:"root"`
	Plans   []PlanFacts `json:"plans"`
	// Err is non-empty when the service's docs/plans could not be read.
	Err string `json:"error,omitempty"`
}

// PortfolioInput is everything the plans portfolio reads.
type PortfolioInput struct {
	Services []ServicePlans
	// Registered is the registry's size, the denominator for "services
	// with plans".
	Registered int
	// Ledger is the verify ledger read (for the SD-3 trend) and its path.
	Ledger     []LedgerRun
	LedgerPath string
	LedgerErr  string
	// PlanHref builds a plan page's link.
	PlanHref func(service, slug string) string
	// MinTrendDays gates the trend (SD-3): fewer distinct days of ledger
	// rows render the trend absent with the count.
	MinTrendDays int
}

// PortfolioPlan is one plan's row in the portfolio.
type PortfolioPlan struct {
	Service string      `json:"service"`
	Plan    string      `json:"plan"`
	Rel     string      `json:"rel"`
	Title   string      `json:"title"`
	Href    string      `json:"href"`
	Rollup  *PlanRollup `json:"rollup,omitempty"` // nil until verified
	Bar     ProofBar    `json:"bar"`
	State   State       `json:"state"`
	Reason  string      `json:"reason,omitempty"`
}

// PortfolioService is one service's line in the portfolio.
type PortfolioService struct {
	Service string `json:"service"`
	Plans   int    `json:"plans"`
	Rows    int    `json:"rows"`
	// Unproven is the service's claimed-but-unproven count (the SD-2
	// headline, per repository).
	Unproven Value[int] `json:"claimed_unproven"`
	Proven   Value[int] `json:"proven"`
	Err      string     `json:"error,omitempty"`
}

// Portfolio is the plans page: every plan across the fleet with its proof
// bar, the per-service lines, and the claimed-vs-proven headline.
type Portfolio struct {
	Plans    []PortfolioPlan    `json:"plans"`
	Services []PortfolioService `json:"services"`
	// WithPlans counts the services that have any plan; Registered is the
	// registry's size.
	WithPlans  int `json:"services_with_plans"`
	Registered int `json:"services_registered"`
	// Bar sums every verified plan's rows (pending plans are named, not
	// guessed).
	Bar ProofBar `json:"bar"`
	// Unproven is the headline (SD-2): claimed rows the gate does not
	// prove, across every verified plan. Proven is its companion.
	Unproven Value[int] `json:"claimed_unproven"`
	Proven   Value[int] `json:"proven"`
	// Trend is the fleet's claimed-vs-proven trend from the verify ledger
	// (SD-3): absent until the ledger has MinTrendDays days of rows.
	Trend Trend `json:"trend"`
}

// PlanPortfolio projects every service's plans into the portfolio (OC-21).
// A plan's bar counts its rows by the state the gate computed; a plan still
// being verified shows its claimed rows as "not yet verified", and the
// headline then says it is a lower bound. A service whose plans could not
// be read makes the headline an error naming it, never a smaller total
// (I38): a gap count without them would read as fewer defects.
func PlanPortfolio(in PortfolioInput) Portfolio {
	out := Portfolio{Registered: in.Registered}
	href := in.PlanHref
	if href == nil {
		href = func(service, slug string) string { return "/plans/" + service + "/" + slug }
	}
	fleet := map[string]int{}
	var unproven, proven, opProven, stale, verifiedPlans, totalPlans int
	var pending, failed, unreadable []string
	for _, sp := range in.Services {
		ps := PortfolioService{Service: sp.Service, Plans: len(sp.Plans), Err: sp.Err}
		if sp.Err != "" {
			unreadable = append(unreadable, sp.Service+" ("+sp.Err+")")
			ps.Unproven = Errored[int]("docs/plans unreadable: " + sp.Err)
			ps.Proven = Errored[int]("docs/plans unreadable: " + sp.Err)
			out.Services = append(out.Services, ps)
			continue
		}
		if len(sp.Plans) == 0 {
			continue
		}
		out.WithPlans++
		svcUnproven, svcProven, svcOpProven, svcStale, svcVerified := 0, 0, 0, 0, 0
		var svcPending, svcFailed []string
		for _, p := range sp.Plans {
			totalPlans++
			ps.Rows += len(p.Rows)
			pp := PortfolioPlan{
				Service: p.Service, Plan: p.Path, Rel: p.Rel, Title: p.Title,
				Href: href(p.Service, p.Slug()), Bar: ProofBarOf(p.Rows, p.DaemonStale),
			}
			switch {
			case p.Err != "":
				pp.State, pp.Reason = StateError, "the gate could not verify this plan: "+p.Err
				failed = append(failed, p.Service+"/"+p.Rel)
				svcFailed = append(svcFailed, p.Rel)
			case !p.Verified:
				pp.State = StatePartial
				pp.Reason = "verifying — its claimed rows read not yet verified until the gate's verdicts arrive"
				pending = append(pending, p.Service+"/"+p.Rel)
				svcPending = append(svcPending, p.Rel)
			default:
				r := RollupPlan(p.Path, p.Rows, p.DaemonStale)
				pp.Rollup = &r
				pp.State = StatePresent
				svcUnproven += r.ClaimedUnproven
				svcProven += r.Proven
				svcOpProven += r.OpProven
				svcStale += r.MeasuredStale
				svcVerified++
				verifiedPlans++
				for _, seg := range pp.Bar.Segments {
					fleet[seg.Key] += seg.Count
				}
			}
			out.Plans = append(out.Plans, pp)
		}
		unproven += svcUnproven
		proven += svcProven
		opProven += svcOpProven
		stale += svcStale
		ps.Unproven = unprovenValue(gapValue(svcUnproven, svcVerified, len(sp.Plans), svcPending, svcFailed, nil, "claimed rows the gate does not prove"), svcStale)
		ps.Proven = provenValue(gapValue(svcProven, svcVerified, len(sp.Plans), svcPending, svcFailed, nil, "rows the gate proves"), svcOpProven)
		out.Services = append(out.Services, ps)
	}
	out.Bar = ProofBarFromCounts(fleet)
	out.Unproven = unprovenValue(gapValue(unproven, verifiedPlans, totalPlans, pending, failed, unreadable, "claimed rows the gate does not prove"), stale)
	out.Proven = provenValue(gapValue(proven, verifiedPlans, totalPlans, pending, failed, unreadable, "rows the gate proves"), opProven)
	out.Trend = PlansTrend(in.Ledger, in.LedgerPath, in.LedgerErr, in.MinTrendDays, in.Services)
	return out
}

// unprovenValue makes a claimed-but-unproven count a named lower bound when
// some claimed rows were measured by a stale daemon (PC-5): those rows are
// kept out of the count, and they are not proven either, so the count
// alone would read as "this many and no more" — a stale 0 as nothing
// unproven. A stale reading may not be hidden (measuredstale.go).
func unprovenValue(v Value[int], stale int) Value[int] {
	if stale == 0 || !v.State.HasValue() {
		return v
	}
	reason := fmt.Sprintf("%s the daemon measured stale are not counted here (PC-5) and are not proven either, so this is a lower bound — task deploy or restart the daemon on the current binary, then reload", countNoun(stale, "claimed row"))
	if v.State == StatePartial {
		reason = v.Reason + "; " + reason
	}
	out := Partial(v.V, reason).WithSource(v.Source)
	out.Denominator = v.Denominator
	return out
}

// provenValue names the (op) measurement rows inside a proven count: the
// headline's "proven by the gate" counts them (as `plans status`'s proven
// does), while the proof bar gives them their own segment.
func provenValue(v Value[int], op int) Value[int] {
	if op > 0 && v.State.HasValue() && v.Denominator != "" {
		noun := "a measurement row (op)"
		if op > 1 {
			noun = "measurement rows (op)"
		}
		v.Denominator += fmt.Sprintf(", %s of them %s", thousands(op), noun)
	}
	return v
}

// PlansTrend is the claimed-vs-proven trend over the plans a portfolio
// shows (SD-3): the verify-ledger runs of each service's CURRENT plans,
// matched by service and file name. The ledger keeps every run it ever
// recorded — scratch service IDs re-running another repository's plans,
// plan files since deleted or archived, README.md — and summing them drew
// a fleet of 321 plans beside a portfolio of 142. A service whose plans
// could not be read makes the trend an error naming it (I38): a trend
// without its plans would read as less work.
func PlansTrend(runs []LedgerRun, path, readErr string, minDays int, services []ServicePlans) Trend {
	current := map[string]map[string]bool{}
	var unreadable []string
	plans, repos := 0, 0
	for _, sp := range services {
		if sp.Err != "" {
			unreadable = append(unreadable, sp.Service+" ("+sp.Err+")")
			continue
		}
		if len(sp.Plans) == 0 {
			continue
		}
		repos++
		key := strings.ToLower(sp.Service)
		if current[key] == nil {
			current[key] = map[string]bool{}
		}
		for _, p := range sp.Plans {
			if !current[key][filepath.Base(p.Path)] {
				current[key][filepath.Base(p.Path)] = true
				plans++
			}
		}
	}
	repoWord := "repositories"
	if repos == 1 {
		repoWord = "repository"
	}
	scope := fmt.Sprintf("the %s of %d %s, by service and file name — scratch services, deleted plans and README.md are not counted",
		countNoun(plans, "current plan"), repos, repoWord)
	if len(unreadable) > 0 {
		if minDays <= 0 {
			minDays = DefaultTrendDays
		}
		return Trend{State: StateError, Source: "verify ledger " + path, Need: minDays, Scope: scope,
			Reason: fmt.Sprintf("docs/plans unreadable in %s — the trend counts the current plans, so none is drawn without them", joinFew(unreadable))}
	}
	if plans == 0 {
		if minDays <= 0 {
			minDays = DefaultTrendDays
		}
		return Trend{State: StateAbsent, Source: "verify ledger " + path, Need: minDays, Scope: scope,
			Reason: "no plans under docs/plans in this scope — there is no trend to draw"}
	}
	t := LedgerTrend(runs, path, readErr, minDays, func(r LedgerRun) bool {
		return current[strings.ToLower(r.Service)][filepath.Base(r.Plan)]
	})
	t.Scope = scope
	return t
}

// gapValue states a plans total with its coverage: exact when every plan was
// verified, a named lower bound while some are pending, an error naming the
// plans or services that could not be read or verified.
func gapValue(n, verified, total int, pending, failed, unreadable []string, what string) Value[int] {
	const source = "the verify engine at the current index (fathom plans status)"
	switch {
	case len(unreadable) > 0:
		return Errored[int](fmt.Sprintf("docs/plans unreadable in %s — no total is shown, since one without them would read as fewer defects", joinFew(unreadable))).WithSource(source)
	case len(failed) > 0:
		return Errored[int](fmt.Sprintf("the gate could not verify %s — no total is shown", joinFew(failed))).WithSource(source)
	case total == 0:
		return Absent[int]("no plans under docs/plans in any registered service").WithSource(source)
	case verified == 0:
		return Absent[int](fmt.Sprintf("none of %s verified yet — the gate's verdicts are still being computed", countNoun(total, "plan"))).WithSource(source)
	case len(pending) > 0:
		return Partial(n, fmt.Sprintf("%d of %s verified; the rest are still verifying, so this is a lower bound", verified, countNoun(total, "plan"))).WithSource(source).WithDenominator(what + " across verified plans")
	}
	return Measured(n, source).WithDenominator(what + " across " + countNoun(total, "plan"))
}

// joinFew names up to three items and counts the rest.
func joinFew(items []string) string {
	if len(items) <= 3 {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:3], ", ") + fmt.Sprintf(" and %d more", len(items)-3)
}

// LedgerRun is one verify-ledger row: one plan's counts from one gate run.
type LedgerRun struct {
	At       time.Time `json:"at"`
	Service  string    `json:"service"`
	Plan     string    `json:"plan"` // as recorded (relative or absolute)
	Commit   string    `json:"commit,omitempty"`
	Total    int       `json:"total"`
	Claimed  int       `json:"claimed"`
	Verified int       `json:"verified"`
	Failing  int       `json:"failing"`
	Origin   string    `json:"origin,omitempty"`
}

// PlanKey is the ledger's identity for a plan: its service (case-folded,
// as the registry's names are) and file name. Ledger rows come from clones
// and worktrees whose roots differ, so the root is not part of the key.
func (r LedgerRun) PlanKey() string { return strings.ToLower(r.Service) + "|" + filepath.Base(r.Plan) }

// TrendPoint is one day of the claimed-vs-proven trend: for each plan, its
// last gate run on or before that day, summed.
type TrendPoint struct {
	Day      string `json:"day"`
	Plans    int    `json:"plans"`
	Claimed  int    `json:"claimed"`
	Proven   int    `json:"proven"`
	Failing  int    `json:"failing"`
	RowsSeen int    `json:"runs"` // ledger rows written that day
}

// Trend is a ledger-derived trend, gated (SD-3): until the ledger holds
// rows on MinDays distinct days it is absent with the count, never a line
// drawn through too little data.
type Trend struct {
	State  State  `json:"state"`
	Reason string `json:"reason,omitempty"`
	Source string `json:"source"`
	// Scope names the plans the trend counts (a portfolio's trend: its
	// current plans; a plan's burn-up: that plan).
	Scope  string       `json:"scope,omitempty"`
	Days   int          `json:"days"`
	Need   int          `json:"days_needed"`
	Points []TrendPoint `json:"points,omitempty"`
}

// DefaultTrendDays is SD-3's gate: 14 days of ledger rows.
const DefaultTrendDays = 14

// LedgerTrend folds ledger runs into a daily claimed-vs-proven trend. keep,
// when non-nil, selects the plans that count (one plan's burn-up passes a
// filter; the fleet passes nil). The trend is absent until the selected
// rows fall on at least minDays distinct UTC days (SD-3), and the reason
// says how many days there are.
func LedgerTrend(runs []LedgerRun, path, readErr string, minDays int, keep func(LedgerRun) bool) Trend {
	if minDays <= 0 {
		minDays = DefaultTrendDays
	}
	t := Trend{Source: "verify ledger " + path, Need: minDays}
	if readErr != "" {
		t.State, t.Reason = StateError, "verify ledger unreadable: "+readErr
		return t
	}
	var sel []LedgerRun
	for _, r := range runs {
		if r.At.IsZero() || (keep != nil && !keep(r)) {
			continue
		}
		sel = append(sel, r)
	}
	if len(sel) == 0 {
		t.State = StateAbsent
		t.Reason = "no verify-ledger rows yet — the trend is drawn once the ledger has rows on " + fmt.Sprint(minDays) + " days"
		if path == "" {
			t.Reason = "no verify ledger configured — the trend is not measured"
		}
		return t
	}
	sort.SliceStable(sel, func(i, j int) bool { return sel[i].At.Before(sel[j].At) })
	days := map[string]int{}
	for _, r := range sel {
		days[r.At.UTC().Format("2006-01-02")]++
	}
	t.Days = len(days)
	if t.Days < minDays {
		t.State = StateAbsent
		t.Reason = fmt.Sprintf("the verify ledger has rows on %s; the trend needs %d — not drawn from too little data", countNoun(t.Days, "day"), minDays)
		return t
	}
	first := sel[0].At.UTC().Truncate(24 * time.Hour)
	last := sel[len(sel)-1].At.UTC().Truncate(24 * time.Hour)
	latest := map[string]LedgerRun{}
	i := 0
	for d := first; !d.After(last); d = d.Add(24 * time.Hour) {
		end := d.Add(24 * time.Hour)
		for i < len(sel) && sel[i].At.Before(end) {
			latest[sel[i].PlanKey()] = sel[i]
			i++
		}
		p := TrendPoint{Day: d.Format("2006-01-02"), RowsSeen: days[d.Format("2006-01-02")]}
		for _, r := range latest {
			p.Plans++
			p.Claimed += r.Claimed
			p.Proven += r.Verified
			p.Failing += r.Failing
		}
		t.Points = append(t.Points, p)
	}
	t.State = StatePresent
	return t
}

// PlanRevision is one commit of a plan file, parsed: its scope and claimed
// rows at that commit.
type PlanRevision struct {
	Commit  string    `json:"commit"`
	At      time.Time `json:"at"`
	Total   int       `json:"total"`
	Claimed int       `json:"claimed"`
	Skipped int       `json:"skipped"`
}

// BurnupInput is what a plan's burn-up reads: the plan file's git history
// and the verify ledger's runs of it.
type BurnupInput struct {
	Service string
	Plan    string // the plan's path (its file name keys the ledger)
	// Revisions are the plan's commits, oldest first; HistoryNote names
	// why there are none (not a git repository, never committed).
	Revisions   []PlanRevision
	HistoryNote string
	Ledger      []LedgerRun
	LedgerPath  string
	LedgerErr   string
	MinDays     int
	// MaxPoints bounds the revisions drawn; older ones are counted.
	MaxPoints int
}

// BurnupPoint is one drawn revision.
type BurnupPoint struct {
	Commit  string `json:"commit"`
	At      string `json:"at"`
	Scope   int    `json:"scope"`
	Claimed int    `json:"claimed"`
}

// CrossCheck compares ledger runs with the plan's own history: a run whose
// commit is a revision of the plan should have counted the rows that
// revision holds.
type CrossCheck struct {
	Matched  int      `json:"matched"`
	Agree    int      `json:"agree"`
	Disagree []string `json:"disagree,omitempty"`
	// Unmatched counts runs whose commit is not a revision of the plan (a
	// later commit that did not touch it, or a dirty tree).
	Unmatched int `json:"unmatched"`
}

// Burnup is a plan's scope against claimed rows over its history, with the
// gate's proven trend from the ledger (SD-3 gated) and the cross-check
// between the two.
type Burnup struct {
	State      State         `json:"state"`
	Reason     string        `json:"reason,omitempty"`
	Points     []BurnupPoint `json:"points"`
	Omitted    int           `json:"omitted"`
	MaxScope   int           `json:"max_scope"`
	Commits    int           `json:"commits"`
	Proven     Trend         `json:"proven_trend"`
	CrossCheck CrossCheck    `json:"cross_check"`
}

// PlanBurnup projects one plan's burn-up (OC-22's third part).
func PlanBurnup(in BurnupInput) Burnup {
	b := Burnup{Points: []BurnupPoint{}}
	name := filepath.Base(in.Plan)
	mine := func(r LedgerRun) bool { return r.Service == in.Service && filepath.Base(r.Plan) == name }
	b.Proven = LedgerTrend(in.Ledger, in.LedgerPath, in.LedgerErr, in.MinDays, mine)
	b.Commits = len(in.Revisions)
	if len(in.Revisions) == 0 {
		b.State = StateAbsent
		b.Reason = orText(in.HistoryNote, "the plan file has no git history")
	} else {
		b.State = StatePresent
		revs := in.Revisions
		max := in.MaxPoints
		if max > 0 && len(revs) > max {
			b.Omitted = len(revs) - max
			revs = revs[len(revs)-max:]
		}
		for _, r := range revs {
			b.Points = append(b.Points, BurnupPoint{Commit: shortCommit(r.Commit), At: r.At.UTC().Format("2006-01-02 15:04"), Scope: r.Total, Claimed: r.Claimed})
			if r.Total > b.MaxScope {
				b.MaxScope = r.Total
			}
		}
	}
	byCommit := map[string]PlanRevision{}
	for _, r := range in.Revisions {
		byCommit[shortCommit(r.Commit)] = r
	}
	for _, run := range in.Ledger {
		if !mine(run) || run.Commit == "" {
			continue
		}
		rev, ok := byCommit[shortCommit(run.Commit)]
		if !ok {
			b.CrossCheck.Unmatched++
			continue
		}
		b.CrossCheck.Matched++
		if rev.Total == run.Total {
			b.CrossCheck.Agree++
		} else if len(b.CrossCheck.Disagree) < 5 {
			b.CrossCheck.Disagree = append(b.CrossCheck.Disagree, fmt.Sprintf("the run at %s counted %d rows; the plan at %s holds %d", run.At.UTC().Format("2006-01-02 15:04"), run.Total, shortCommit(run.Commit), rev.Total))
		}
	}
	return b
}

// shortCommit trims a commit id for display.
func shortCommit(c string) string {
	if len(c) > 8 {
		return c[:8]
	}
	return c
}

// BoardColumn is one lane of a plan's board.
type BoardColumn struct {
	ID    string      `json:"id"`
	Label string      `json:"label"`
	Cards []BoardCard `json:"cards"`
}

// BoardCard is one row on the board: its id, title, contract kind and the
// state the gate computed.
type BoardCard struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	State    string `json:"state"`
	Contract string `json:"contract"`
	Detail   string `json:"detail,omitempty"`
	Href     string `json:"href"`
}

// boardColumns maps row states to board lanes, in lane order.
var boardColumns = []struct {
	id, label string
	states    []string
}{
	{"remaining", "Remaining", []string{RowOpen, RowInProgress}},
	{"claimed", "Claimed, not proven", []string{RowClaimedFailing, RowClaimedUnknown, RowMeasuredStale, RowPending}},
	{"proven", "Proven", []string{RowProven}},
	{"op-measured", "Measured (op)", []string{RowOpMeasured}},
	{"op-waiting", "Waiting on a measurement (op)", []string{RowOpWaiting}},
	{"deferred", "Deferred or refused", []string{RowDeferred, RowSkipped}},
}

// PlanLedgerInput is what one plan's page reads.
type PlanLedgerInput struct {
	Plan    PlanFacts
	Burnup  BurnupInput
	RowHref func(id string) string
}

// PlanRowView is one row in the plan page's table alternative to the board.
type PlanRowView struct {
	PlanRowFact
	State    string `json:"state"`
	Contract string `json:"contract"`
	Href     string `json:"href"`
}

// PlanLedgerView is one plan's page: its proof bar, its board of rows by
// state with each row's contract kind, and its burn-up.
type PlanLedgerView struct {
	Service string         `json:"service"`
	Plan    string         `json:"plan"`
	Rel     string         `json:"rel"`
	Title   string         `json:"title"`
	State   State          `json:"state"`
	Reason  string         `json:"reason,omitempty"`
	Rollup  *PlanRollup    `json:"rollup,omitempty"`
	Bar     ProofBar       `json:"bar"`
	Columns []BoardColumn  `json:"columns"`
	Rows    []PlanRowView  `json:"rows"`
	Burnup  Burnup         `json:"burnup"`
	Stamp   PlanStampFacts `json:"stamp"`
}

// PlanStampFacts says which index the verdicts came from.
type PlanStampFacts struct {
	StoreCommit string `json:"store_commit,omitempty"`
	VerifiedAt  string `json:"verified_at,omitempty"`
	DaemonStale bool   `json:"daemon_stale,omitempty"`
}

// PlanLedger projects one plan into its page (OC-22): the proof bar, the
// board (Remaining, Claimed, Proven, the two op lanes, Deferred) with each
// card's contract kind, the table alternative, and the burn-up from the
// plan's history cross-checked with the verify ledger.
func PlanLedger(in PlanLedgerInput) PlanLedgerView {
	p := in.Plan
	href := in.RowHref
	if href == nil {
		href = func(id string) string { return "#" + id }
	}
	v := PlanLedgerView{
		Service: p.Service, Plan: p.Path, Rel: p.Rel, Title: p.Title,
		Bar: ProofBarOf(p.Rows, p.DaemonStale), Rows: []PlanRowView{},
		Stamp: PlanStampFacts{StoreCommit: shortCommit(p.StoreCommit), DaemonStale: p.DaemonStale},
	}
	if !p.VerifiedAt.IsZero() {
		v.Stamp.VerifiedAt = p.VerifiedAt.UTC().Format(time.RFC3339)
	}
	switch {
	case p.Err != "":
		v.State, v.Reason = StateError, "the gate could not verify this plan: "+p.Err
	case !p.Verified:
		v.State, v.Reason = StatePartial, "not yet verified at the current index — claimed rows show as not yet verified until the gate's verdicts arrive"
	default:
		r := RollupPlan(p.Path, p.Rows, p.DaemonStale)
		v.Rollup, v.State = &r, StatePresent
	}
	cards := map[string][]BoardCard{}
	for _, r := range p.Rows {
		state := RowState(r, p.DaemonStale)
		c := ContractOf(r)
		v.Rows = append(v.Rows, PlanRowView{PlanRowFact: r, State: state, Contract: c.Kind, Href: href(r.ID)})
		cards[state] = append(cards[state], BoardCard{ID: r.ID, Title: r.Title, State: state, Contract: c.Kind, Detail: cardDetail(r, state), Href: href(r.ID)})
	}
	for _, col := range boardColumns {
		bc := BoardColumn{ID: col.id, Label: col.label, Cards: []BoardCard{}}
		for _, s := range col.states {
			bc.Cards = append(bc.Cards, cards[s]...)
		}
		v.Columns = append(v.Columns, bc)
	}
	in.Burnup.Service, in.Burnup.Plan = p.Service, p.Path
	v.Burnup = PlanBurnup(in.Burnup)
	return v
}

// runtimeCriterion is the verify engine's runtime clause, reported unknown
// on every row until CI evidence is ingested.
const runtimeCriterion = "runtime_pass"

// cardDetail is the one line a card carries under its title: why a claimed
// row is not proven, or what a deferred row waits for.
func cardDetail(r PlanRowFact, state string) string {
	switch state {
	case RowClaimedFailing:
		for _, c := range r.Criteria {
			if c.Status == "fail" {
				return c.Name + ": " + c.Detail
			}
		}
		return r.Label
	case RowClaimedUnknown:
		// runtime_pass is unknown on every row, proven ones included — it
		// never decides one — so it is never the reason a row is
		// undecidable. A row with no contract is undecidable because it
		// names none: no CI run would change that.
		if r.Label == "no-contract" || ContractOf(r).Kind == ContractNone {
			return "no contract — the row names no symbol, oracle or gone: pattern, so the gate cannot decide it; give it one"
		}
		for _, c := range r.Criteria {
			if c.Status == "unknown" && c.Name != runtimeCriterion {
				return c.Name + ": " + c.Detail
			}
		}
		return orText(r.Label, "the gate could not decide this row")
	case RowPending:
		return "the gate's verdict has not arrived yet"
	case RowDeferred:
		return orText(r.Trigger, r.Reason)
	case RowSkipped:
		return orText(r.Reason, r.Disposition)
	case RowOpWaiting:
		return r.SkipReason
	}
	return ""
}

func orText(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
