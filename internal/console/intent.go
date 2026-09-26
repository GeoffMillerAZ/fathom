package console

// intent.go — OC-38 (vision 35 §3 "Intent"; design §3 "Intent"): the
// project's intent corpus as the page shows it — invariants with their
// enforcement rung and audit state, decisions with their revisit
// conditions (fired or not), futures, questions and goals, the unproven
// claims, and the intent ledger's debt over time.
//
// It projects ONE run of `fathom intent audit --read-only`: the page never
// runs an audit that writes (the survey watched a plain audit append clock
// and debt rows to the user's ledger just by being looked at). Prose is
// never evaluated: a condition is human-judged unless its author attached
// an oracle, and "fired" is only ever what the ledger recorded.

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/readcoverage"
)

// IntentInvariant is one invariant row.
type IntentInvariant struct {
	ID        string   `json:"id"`
	Statement string   `json:"statement"`
	Rung      string   `json:"rung"`
	Enforced  bool     `json:"enforced"`
	Tests     []string `json:"tests,omitempty"`
	GateSteps []string `json:"gate_steps,omitempty"`
	// State is the audit's word for the row: violated, unproven,
	// unenforced (an honest stated/reviewed rung, never debt), not computed
	// (the audit could not check it), or no finding.
	State  string   `json:"state"`
	Detail []string `json:"detail,omitempty"`
	File   string   `json:"file"`
	Line   int      `json:"line"`
}

// IntentCondition is one expiry condition (Revisit when, Promote when, …).
type IntentCondition struct {
	ID     string `json:"id"`
	Record string `json:"record,omitempty"`
	Label  string `json:"label"`
	Text   string `json:"text"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	// State: terminal (can never fire), human-judged (no oracle — prose is
	// never evaluated), fired (the ledger recorded it firing), or not fired.
	State string `json:"state"`
	// FiredAt is when the ledger first recorded it firing.
	FiredAt string `json:"fired_at,omitempty"`
}

// IntentDebtPoint is one recorded audit's unproven total.
type IntentDebtPoint struct {
	At    string `json:"at"`
	Total int    `json:"total"`
}

// IntentViewModel is the intent tab's projection.
type IntentViewModel struct {
	Root          string `json:"root"`
	Tier          string `json:"tier"`
	Router        string `json:"router,omitempty"`
	DecisionsHome string `json:"decisions_home,omitempty"`
	// Corpus is the corpus's tier with its truth state: present when the
	// corpus was read whole, partial when some of it could not be read,
	// absent when the repository has no router and no decision records (a
	// choice, not a defect), and an ERROR when it has no readable router or
	// record because one that exists could not be read — an unreadable file
	// is never reported as no corpus (I38).
	Corpus Value[string] `json:"corpus"`
	// ReadOnly is always true: the view is built only from a read-only run.
	ReadOnly   bool              `json:"read_only"`
	Invariants []IntentInvariant `json:"invariants"`
	// Conditions grouped: decisions (Revisit when / If false in the
	// decisions home), futures, questions, and the rest (goals, scope,
	// domain assumptions).
	Decisions []IntentCondition `json:"decisions"`
	Futures   []IntentCondition `json:"futures"`
	Questions []IntentCondition `json:"questions"`
	Other     []IntentCondition `json:"other"`
	Fired     Value[int]        `json:"fired"`
	// Unproven is the audit's list; UnprovenCount a lower bound when the
	// corpus could not be read completely.
	Unproven      []intent.Claim    `json:"unproven"`
	UnprovenCount Value[int]        `json:"unproven_count"`
	Uncomputable  []string          `json:"uncomputable"`
	Debt          []IntentDebtPoint `json:"debt"`
	DebtState     Value[int]        `json:"debt_points"`
	ClockStarted  string            `json:"clock_started_at,omitempty"`
}

// IntentView projects one read-only audit run. ledger is the intent ledger
// as that run read it (never appended to).
func IntentView(c intent.Corpus, a intent.Audit, ledger []intent.LedgerRow, now time.Time) IntentViewModel {
	m := IntentViewModel{
		Root: c.Root, Tier: string(c.Tier), Router: c.RouterPath, DecisionsHome: c.DecisionsHome, ReadOnly: true,
		Invariants: []IntentInvariant{}, Decisions: []IntentCondition{}, Futures: []IntentCondition{},
		Questions: []IntentCondition{}, Other: []IntentCondition{}, Unproven: []intent.Claim{},
		Uncomputable: []string{}, Debt: []IntentDebtPoint{},
	}
	const source = "fathom intent audit --read-only"
	m.Corpus = corpusState(c, source)

	claimsByRef := map[string][]intent.Claim{}
	for _, cl := range a.Unproven {
		claimsByRef[cl.Ref] = append(claimsByRef[cl.Ref], cl)
	}
	u4Blind := false
	for _, u := range a.Uncomputable {
		if strings.HasPrefix(u, "U4") {
			u4Blind = true
		}
	}
	for _, inv := range c.Invariants {
		row := IntentInvariant{
			ID: inv.ID, Statement: inv.Statement, Rung: string(inv.Rung), Enforced: inv.Rung.Enforced(),
			Tests: inv.Tests, GateSteps: inv.GateSteps, File: inv.File, Line: inv.Line,
		}
		claims := claimsByRef[inv.ID]
		for _, cl := range claims {
			row.Detail = append(row.Detail, cl.Kind+": "+cl.Detail)
		}
		switch {
		case inv.Rung == intent.RungViolated:
			row.State = "violated"
		case len(claims) > 0:
			row.State = "unproven"
		case inv.Rung == intent.RungStated || inv.Rung == intent.RungReviewed:
			row.State = "unenforced"
		case u4Blind && len(inv.Tests) > 0:
			row.State = "not computed"
		case inv.Rung == intent.RungUnevaluable:
			row.State = "not computed"
		default:
			row.State = "no finding"
		}
		m.Invariants = append(m.Invariants, row)
	}

	decisionsHome := filepath.ToSlash(c.DecisionsHome)
	fired := 0
	for _, cond := range c.Conditions {
		row := IntentCondition{ID: cond.ID, Record: cond.Record, Label: cond.Label, Text: cond.Text, File: cond.File, Line: cond.Line}
		switch {
		case cond.Terminal:
			row.State = "terminal"
		case !cond.Annotated():
			row.State = "human-judged"
		default:
			if first, ok := intent.FirstSeen(ledger, cond.ID, "fired"); ok {
				row.State, row.FiredAt = "fired", first.At
				fired++
			} else {
				row.State = "not fired"
			}
		}
		file := filepath.ToSlash(cond.File)
		switch {
		case decisionsHome != "" && (file == decisionsHome || strings.HasPrefix(file, strings.TrimSuffix(decisionsHome, "/")+"/")):
			m.Decisions = append(m.Decisions, row)
		case strings.HasSuffix(file, "futures.md"):
			m.Futures = append(m.Futures, row)
		case strings.HasSuffix(file, "questions.md"):
			m.Questions = append(m.Questions, row)
		default:
			m.Other = append(m.Other, row)
		}
	}
	annotated := 0
	for _, cond := range c.Conditions {
		if cond.Annotated() && !cond.Terminal {
			annotated++
		}
	}
	if annotated == 0 {
		m.Fired = Absent[int]("no condition carries an oracle annotation — every condition is human-judged, so none can be shown to have fired (prose is never evaluated)").WithSource(source)
	} else {
		m.Fired = Measured(fired, source).WithDenominator("of " + thousands(annotated) + " annotated conditions, the ones the intent ledger recorded firing")
	}

	m.Unproven = append(m.Unproven, a.Unproven...)
	m.Uncomputable = append(m.Uncomputable, a.Uncomputable...)
	if a.Coverage.Complete() {
		m.UnprovenCount = Measured(len(a.Unproven), source).WithDenominator("claims the corpus makes that no instrument shows")
	} else {
		m.UnprovenCount = Partial(len(a.Unproven), readcoverage.Note(a.Coverage)).WithSource(source)
	}
	if len(a.Uncomputable) > 0 && m.UnprovenCount.State != StatePartial {
		m.UnprovenCount = Partial(len(a.Unproven), strings.Join(a.Uncomputable, "; ")+" — a lower bound").WithSource(source)
	}

	for _, r := range ledger {
		if r.Kind != intent.KindDebt || r.RepoRoot != c.Root {
			continue
		}
		total := 0
		for _, n := range r.ByKind {
			total += n
		}
		m.Debt = append(m.Debt, IntentDebtPoint{At: r.At, Total: total})
	}
	sort.SliceStable(m.Debt, func(i, j int) bool { return m.Debt[i].At < m.Debt[j].At })
	if len(m.Debt) == 0 {
		m.DebtState = Absent[int]("the intent ledger holds no recorded audit for " + c.Root + " — a read-only run records none, so debt over time starts with the first `fathom intent audit` run from the command line").WithSource(intent.LedgerPath())
	} else {
		m.DebtState = Measured(len(m.Debt), intent.LedgerPath()).WithDenominator("recorded audits of this root in the intent ledger").WithAsOf(now)
	}
	if row, ok := intent.Clock(ledger, c.Root); ok {
		m.ClockStarted = row.At
	}
	return m
}

// corpusState is the corpus's tier as a truth state, decided by the
// corpus's own reads (a source outside the corpus that could not be read
// makes the audit's counts a lower bound; it does not make the corpus
// missing).
func corpusState(c intent.Corpus, source string) Value[string] {
	note := readcoverage.Note(c.Coverage)
	switch {
	case c.Tier == intent.TierNone && note != "":
		return Errored[string]("the intent corpus could not be read: " + note + " — a router or decision record that exists and cannot be read is never reported as no corpus (I38)").WithSource(source)
	case c.Tier == intent.TierNone:
		return Absent[string]("no intent corpus: " + c.Root + " has no docs/intent/ router and no decision records — nothing to audit (a choice, not a defect)").WithSource(source)
	case note != "":
		return Partial(string(c.Tier), "the corpus was read in part: "+note+" — every count below is a lower bound").WithSource(source)
	}
	return Measured(string(c.Tier), source).WithDenominator("the corpus's tier: a docs/intent/ router, or decision records alone")
}
