package console

// rowevidence.go — a plan row's evidence (OC-23, design §3 "Row
// evidence"): the contract as authored, the verdict the gate computed, the
// path the gate's reach claim stands on (a test to the symbol, over the
// call graph) or the line a lexical oracle matched, and the row's history.
// Everything here is read from an instrument; what no instrument records is
// said absent (the verify ledger keeps per-plan counts, not per-row
// verdicts, so per-row refutations are not measured).

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ReachTest is one test that reaches the row's symbol over the call graph.
type ReachTest struct {
	Test         string `json:"test"`
	ViaDispatch  bool   `json:"via_dispatch,omitempty"`
	Implementers int    `json:"implementers,omitempty"`
}

// ReachFacts is what the call graph says about the row's symbol: every
// test reaching it (callgraph.TestImpactDetail) and the shortest call path
// from the chosen test to it (callgraph.CallPath).
type ReachFacts struct {
	Symbol string      `json:"symbol"`
	Tests  []ReachTest `json:"tests"`
	// From is the test the path starts at: the row's reachable_from when
	// it names one, else the first reaching test.
	From string   `json:"from,omitempty"`
	Path []string `json:"path,omitempty"`
	// Err names why the graph could not be read.
	Err string `json:"error,omitempty"`
}

// OracleMatch is a lexical oracle's evaluation against the working tree:
// the file, how many times the pattern matched, and the first matching
// line with its neighbors (Lines, numbered from FirstLine).
type OracleMatch struct {
	Path      string   `json:"path"`
	Pattern   string   `json:"pattern"`
	Count     int      `json:"count"`
	Line      int      `json:"line,omitempty"`
	FirstLine int      `json:"first_line,omitempty"`
	Lines     []string `json:"lines,omitempty"`
	Err       string   `json:"error,omitempty"`
	// Missing is set when the file does not exist in the working tree —
	// the gate's path-absent, a measured nothing, not a read failure.
	Missing bool `json:"missing,omitempty"`
}

// RowRevision is the row's checkbox at one commit of its plan.
type RowRevision struct {
	Commit  string    `json:"commit"`
	At      time.Time `json:"at"`
	Present bool      `json:"present"`
	Claimed bool      `json:"claimed"`
	Skipped bool      `json:"skipped"`
}

// RowEvidenceInput is everything the row page reads.
type RowEvidenceInput struct {
	Service string
	Plan    PlanFacts
	RowID   string
	Reach   *ReachFacts
	Oracle  *OracleMatch
	// History is the row's checkbox across its plan's commits, oldest
	// first; HistoryNote names why there is none.
	History     []RowRevision
	HistoryNote string
	// MaxTests bounds the reaching tests listed; the rest are counted.
	MaxTests int
}

// VerdictView is the gate's verdict on the row.
type VerdictView struct {
	State    string          `json:"state"`
	Status   string          `json:"status,omitempty"`
	Label    string          `json:"label,omitempty"`
	Criteria []CriterionFact `json:"criteria"`
	Reason   string          `json:"reason,omitempty"`
}

// ReachView is the reach half of the evidence.
type ReachView struct {
	State   State       `json:"state"`
	Reason  string      `json:"reason,omitempty"`
	Symbol  string      `json:"symbol,omitempty"`
	From    string      `json:"from,omitempty"`
	Path    []string    `json:"path,omitempty"`
	Hops    int         `json:"hops"`
	Tests   []ReachTest `json:"tests"`
	Total   int         `json:"total"`
	Omitted int         `json:"omitted"`
}

// OracleView is the lexical-oracle half of the evidence.
type OracleView struct {
	State     State    `json:"state"`
	Reason    string   `json:"reason,omitempty"`
	Path      string   `json:"path,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	Count     int      `json:"count"`
	Line      int      `json:"line,omitempty"`
	FirstLine int      `json:"first_line,omitempty"`
	Lines     []string `json:"lines,omitempty"`
	// Bound is the row's count bound in words (CV-29; "at least 1 match"
	// when it authors none) and BoundMet whether the count meets it — the
	// verify engine's own rule, so a count and a verdict never read as a
	// contradiction.
	Bound    string `json:"bound,omitempty"`
	BoundMet *bool  `json:"bound_met,omitempty"`
}

// RowChange is one change of the row's checkbox in the plan's history.
type RowChange struct {
	Commit string `json:"commit"`
	At     string `json:"at"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// HistoryView is the row's history: when it was first claimed and how its
// checkbox moved, from the plan's git history.
type HistoryView struct {
	State        State       `json:"state"`
	Reason       string      `json:"reason,omitempty"`
	Added        string      `json:"added,omitempty"`
	FirstClaimed string      `json:"first_claimed,omitempty"`
	Changes      []RowChange `json:"changes"`
	Reopened     int         `json:"reopened"`
	// Refutations is what the gate refuted per row over time: the verify
	// ledger keeps per-plan counts only, so this is absent, never zero.
	Refutations Value[int] `json:"refutations"`
}

// RowEvidenceView is one row's evidence page and drawer.
type RowEvidenceView struct {
	Service   string      `json:"service"`
	Plan      string      `json:"plan"`
	Rel       string      `json:"rel"`
	PlanTitle string      `json:"plan_title"`
	Row       PlanRowFact `json:"row"`
	Found     bool        `json:"found"`
	State     string      `json:"state"`
	Contract  Contract    `json:"contract"`
	Verdict   VerdictView `json:"verdict"`
	Reach     ReachView   `json:"reach"`
	Oracle    OracleView  `json:"oracle"`
	History   HistoryView `json:"history"`
}

// RowEvidence projects one row's evidence (OC-23). Reach applies to a row
// whose contract is symbol + test reach (or certified_by, which names its
// tests); the oracle applies to a lexical row; each half says "does not
// apply" rather than rendering an empty success.
func RowEvidence(in RowEvidenceInput) RowEvidenceView {
	v := RowEvidenceView{Service: in.Service, Plan: in.Plan.Path, Rel: in.Plan.Rel, PlanTitle: in.Plan.Title}
	for _, r := range in.Plan.Rows {
		if r.ID == in.RowID {
			v.Row, v.Found = r, true
			break
		}
	}
	if !v.Found {
		v.Row = PlanRowFact{ID: in.RowID}
		v.Verdict = VerdictView{State: "missing", Reason: "no row " + in.RowID + " in " + in.Plan.Rel, Criteria: []CriterionFact{}}
		v.Reach = ReachView{State: StateAbsent, Reason: "no such row", Tests: []ReachTest{}}
		v.Oracle = OracleView{State: StateAbsent, Reason: "no such row"}
		v.History = HistoryView{State: StateAbsent, Reason: "no such row", Changes: []RowChange{}, Refutations: Absent[int]("no such row")}
		return v
	}
	r := v.Row
	v.State = RowState(r, in.Plan.DaemonStale)
	v.Contract = ContractOf(r)
	v.Verdict = VerdictView{State: v.State, Status: r.Status, Label: r.Label, Criteria: r.Criteria}
	if v.Verdict.Criteria == nil {
		v.Verdict.Criteria = []CriterionFact{}
	}
	switch {
	case in.Plan.Err != "":
		v.Verdict.Reason = "the gate could not verify this plan: " + in.Plan.Err
	case !r.Verified && !r.Skipped:
		v.Verdict.Reason = "not yet verified at the current index — the verdict appears once the gate has run"
	case r.Skipped:
		v.Verdict.Reason = "skipped rows are exempt from the gate; the disposition is the contract"
	}
	v.Reach = reachView(v.Contract, r, in.Reach, in.MaxTests)
	v.Oracle = oracleView(v.Contract, r, in.Oracle)
	v.History = historyView(in.History, in.HistoryNote)
	return v
}

func reachView(c Contract, r PlanRowFact, f *ReachFacts, maxTests int) ReachView {
	v := ReachView{Tests: []ReachTest{}, Symbol: r.Symbol}
	if c.Kind != ContractReach && c.Kind != ContractCertified {
		v.State, v.Reason = StateAbsent, "does not apply — a "+c.Kind+" row makes no reach claim"
		return v
	}
	switch {
	case f == nil:
		v.State, v.Reason = StateAbsent, "the call graph was not read for this row"
		return v
	case f.Err != "":
		v.State, v.Reason = StateError, "the call graph could not be read: "+f.Err
		return v
	}
	if maxTests <= 0 {
		maxTests = 20
	}
	v.Total = len(f.Tests)
	v.Tests = append(v.Tests, f.Tests...)
	if len(v.Tests) > maxTests {
		v.Omitted = len(v.Tests) - maxTests
		v.Tests = v.Tests[:maxTests]
	}
	v.From, v.Path = f.From, f.Path
	if len(f.Path) > 1 {
		v.Hops = len(f.Path) - 1
	}
	switch {
	case v.Total == 0:
		v.State = StateZero
		v.Reason = "no test reaches " + r.Symbol + " over the call graph"
	case len(f.Path) == 0:
		v.State = StatePartial
		v.Reason = fmt.Sprintf("%s reach %s, but no call path from %s was found", countNoun(v.Total, "test"), r.Symbol, orText(f.From, "the named test"))
	default:
		v.State = StatePresent
	}
	return v
}

func oracleView(c Contract, r PlanRowFact, m *OracleMatch) OracleView {
	if c.Kind != ContractLexical {
		return OracleView{State: StateAbsent, Reason: "does not apply — a " + c.Kind + " row has no lexical oracle"}
	}
	if m == nil {
		return OracleView{State: StateAbsent, Reason: "the working tree was not read for this row"}
	}
	v := OracleView{Path: m.Path, Pattern: m.Pattern, Count: m.Count, Line: m.Line, FirstLine: m.FirstLine, Lines: m.Lines}
	v.Bound = orText(BoundText(r.OracleEq, r.OracleMin, r.OracleMax), "at least 1 match (no bound authored)")
	if m.Err != "" && !m.Missing {
		v.State, v.Reason, v.Bound = StateError, m.Path+" could not be read: "+m.Err, ""
		return v
	}
	if m.Missing {
		// The gate fails the row as path-absent whatever its bound.
		v.State, v.Count = StateZero, 0
		v.Reason = m.Path + " does not exist in the working tree — the gate reads the row as path-absent"
		met := false
		v.BoundMet = &met
		return v
	}
	met := BoundMet(m.Count, r.OracleEq, r.OracleMin, r.OracleMax)
	v.BoundMet = &met
	verdict := "the row requires " + v.Bound + ", so the bound is met"
	if !met {
		verdict = "the row requires " + v.Bound + ", so the bound is not met"
	}
	switch {
	case m.Count == 0:
		v.State, v.Reason = StateZero, "the pattern matches nothing in "+m.Path+" — "+verdict
	default:
		v.State = StatePresent
		v.Reason = fmt.Sprintf("the pattern matches %s in %s — %s", countNoun(m.Count, "time"), m.Path, verdict)
	}
	return v
}

func checkbox(r RowRevision) string {
	switch {
	case !r.Present:
		return "absent"
	case r.Skipped:
		return "[-]"
	case r.Claimed:
		return "[x]"
	}
	return "[ ]"
}

func historyView(revs []RowRevision, note string) HistoryView {
	h := HistoryView{Changes: []RowChange{}, Refutations: Absent[int]("the verify ledger records per-plan counts, not per-row verdicts — per-row refutations are not measured")}
	if len(revs) == 0 {
		h.State, h.Reason = StateAbsent, orText(note, "the plan file has no git history")
		return h
	}
	h.State = StatePresent
	prev := "absent"
	for _, r := range revs {
		cur := checkbox(r)
		at := r.At.UTC().Format("2006-01-02 15:04")
		if cur != prev {
			h.Changes = append(h.Changes, RowChange{Commit: shortCommit(r.Commit), At: at, From: prev, To: cur})
			if prev == "absent" && h.Added == "" {
				h.Added = shortCommit(r.Commit) + " (" + at + ")"
			}
			if cur == "[x]" && h.FirstClaimed == "" {
				h.FirstClaimed = shortCommit(r.Commit) + " (" + at + ")"
			}
			if prev == "[x]" && cur == "[ ]" {
				h.Reopened++
			}
		}
		prev = cur
	}
	if h.Added == "" {
		h.Reason = "the row is not in any committed revision of the plan yet"
	}
	return h
}

// MatchOracle evaluates a lexical oracle's pattern against a file's text:
// the match count and the first matching line with up to context lines on
// each side. The pattern is the verify engine's: a Go regexp in multiline
// mode, exactly as `fathom verify` compiles it.
func MatchOracle(path, pattern, text string, context int) OracleMatch {
	m := OracleMatch{Path: path, Pattern: pattern}
	re, err := regexp.Compile("(?m)" + pattern)
	if err != nil {
		m.Err = "the pattern does not compile: " + err.Error()
		return m
	}
	locs := re.FindAllStringIndex(text, -1)
	m.Count = len(locs)
	if m.Count == 0 {
		return m
	}
	m.Line = 1 + strings.Count(text[:locs[0][0]], "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	lo, hi := m.Line-context, m.Line+context
	if lo < 1 {
		lo = 1
	}
	if hi > len(lines) {
		hi = len(lines)
	}
	m.FirstLine = lo
	m.Lines = append(m.Lines, lines[lo-1:hi]...)
	return m
}
