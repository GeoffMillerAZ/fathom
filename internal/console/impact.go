package console

// impact.go — OC-41 (vision 35 §3 Evidence, design §3 "Impact"): the Impact
// page's projection, shared with `fathom metrics impact --json`.
//
// The survey found the page ordering its sections exploration, Q2, Q1, Q3,
// cost, Q4; burying the A/B result in key/value prose with no range; and
// colouring zeros green and red for tools that have no outcome instrument
// at all ("obligations … 100% not recorded, yet show 0 hits in green").
// ImpactView fixes all three in the model itself:
//
//   - the four questions come in one fixed order, Q1..Q4;
//   - each counterfactual arm is a range — the interquartile span of its
//     per-turn token units with the median marked — beside the number of
//     sessions it rests on (law 12: a savings figure exists only as an arm
//     range, never as a point);
//   - a tool whose calls carry no outcome field has absent hits, misses and
//     did-you-mean values whose reason says "not instrumented": there is no
//     number for a renderer to colour.

import (
	"fmt"
	"sort"
)

// ArmInputs are the counterfactual arms as the session ledger admits them:
// one per-turn token unit per admissible session (armUnitFor's rule).
type ArmInputs struct {
	On, Off []int
	// Excluded counts OFF-arm (or contaminated) rows that yielded no unit —
	// counted, never silently dropped.
	Excluded   int
	Criterion  string // the pre-registered criterion's state, in words
	PilotState string
	Detail     string
}

// ToolOutcomeInput is one tool's outcome profile from the call log.
type ToolOutcomeInput struct {
	Tool        string
	Class       CallerClass
	Calls       int
	Hits        int
	Misses      int
	DidYouMean  int
	NotRecorded int
}

// ImpactInputs is everything ImpactView reads. A non-empty LedgerReason
// means the session ledger could not be read (LedgerError: it exists and
// failed, an error rather than an absence); CallLogReason is the same for
// the call log.
type ImpactInputs struct {
	LedgerReason  string
	LedgerError   bool
	LedgerSource  string
	Sessions      int // distinct sessions the ledger holds
	Arms          ArmInputs
	CallLogReason string
	CallLogError  bool
	CallLogSource string

	// Q1 (observational): the exploration ledger's per-question costs.
	ExpSessions               int
	ExpRawCalls, ExpRawTokens int
	ExpIdxCalls, ExpIdxTokens int
	ExpRatioX10               *int
	ExpNote                   string

	// Q2: the gate (verify ledger), the stop hook (session ledger) and the
	// rules served before edits (call log).
	GateRefusals      *int // nil: the verify ledger has no run (not instrumented)
	GateRuns          int
	GateNote          string
	StopCatches       *int // nil: verify never ran in a ledger session
	StopCatchesRan    int
	ObligationsServed int

	// Q3: per-tool outcomes.
	Tools []ToolOutcomeInput

	// Q4: warm-up.
	FirstEditMedian   *int
	FirstEditSessions int
	FirstEditNote     string
}

// ImpactFigure is one number a question rests on.
type ImpactFigure struct {
	ID    string     `json:"id"`
	Label string     `json:"label"`
	Unit  string     `json:"unit,omitempty"`
	Value Value[int] `json:"value"`
}

// ImpactQuestion is one of the four questions, in order.
type ImpactQuestion struct {
	ID       string         `json:"id"`
	Question string         `json:"question"`
	Answer   string         `json:"answer"`
	Figures  []ImpactFigure `json:"figures"`
}

// ArmRange is one counterfactual arm as a range: the interquartile span of
// its per-turn units with the median, over Units admitted sessions.
type ArmRange struct {
	Arm    string     `json:"arm"`
	Label  string     `json:"label"`
	Units  Value[int] `json:"units"`
	Low    Value[int] `json:"p25"`
	Median Value[int] `json:"median"`
	High   Value[int] `json:"p75"`
}

// ToolOutcome is one tool's outcomes; a tool with no outcome instrument has
// absent Hits, Misses and DidYouMean ("not instrumented"), never zeros.
type ToolOutcome struct {
	Tool         string      `json:"tool"`
	Class        CallerClass `json:"caller_class"`
	Calls        int         `json:"calls"`
	Instrumented Value[int]  `json:"instrumented_calls"`
	Hits         Value[int]  `json:"hits"`
	Misses       Value[int]  `json:"misses"`
	DidYouMean   Value[int]  `json:"did_you_mean"`
}

// ImpactModel is the Impact page's projection.
type ImpactModel struct {
	Questions []ImpactQuestion `json:"questions"`
	ArmUnit   string           `json:"arm_unit"`
	Arms      []ArmRange       `json:"arms"`
	ArmNote   string           `json:"arm_note"`
	Excluded  Value[int]       `json:"excluded_off_rows"`
	Tools     []ToolOutcome    `json:"tools"`
	// Uninstrumented counts the tools none of whose calls carry an outcome.
	Uninstrumented int `json:"uninstrumented_tools"`
}

// Question IDs, in the page's fixed order.
const (
	QuestionSaved       = "Q1"
	QuestionPrevented   = "Q2"
	QuestionFloundering = "Q3"
	QuestionCompounding = "Q4"
)

// ArmUnit names what an arm's values measure.
const ArmUnit = "real tokens (in + out) per assistant turn"

// notInstrumented is the reason a tool with no outcome field carries.
func notInstrumented(calls int) string {
	return fmt.Sprintf("not instrumented — none of its %s records an outcome", countNoun(calls, "call"))
}

// quartiles returns the nearest-rank p25, median and p75 of vals, with the
// median rule every impact figure shares (the upper middle, s[n/2]).
func quartiles(vals []int) (lo, med, hi int, ok bool) {
	if len(vals) == 0 {
		return 0, 0, 0, false
	}
	s := append([]int(nil), vals...)
	sort.Ints(s)
	n := len(s)
	return s[n/4], s[n/2], s[(3*n)/4], true
}

// ImpactView projects the four questions, the arm ranges and the per-tool
// outcomes. Pure: every input is already read.
func ImpactView(in ImpactInputs) ImpactModel {
	m := ImpactModel{ArmUnit: ArmUnit, ArmNote: in.Arms.Detail}
	ledgerValue := func(v Value[int]) Value[int] {
		switch {
		case in.LedgerReason != "" && in.LedgerError:
			return Errored[int](in.LedgerReason).WithSource(in.LedgerSource)
		case in.LedgerReason != "":
			return Absent[int](in.LedgerReason).WithSource(in.LedgerSource)
		}
		return v.WithSource(in.LedgerSource)
	}
	callValue := func(v Value[int]) Value[int] {
		switch {
		case in.CallLogReason != "" && in.CallLogError:
			return Errored[int](in.CallLogReason).WithSource(in.CallLogSource)
		case in.CallLogReason != "":
			return Absent[int](in.CallLogReason).WithSource(in.CallLogSource)
		}
		return v.WithSource(in.CallLogSource)
	}

	// The arms.
	for _, arm := range []struct {
		id, label string
		units     []int
	}{{"on", "ON — channels live", in.Arms.On}, {"off", "OFF — the counterfactual", in.Arms.Off}} {
		r := ArmRange{Arm: arm.id, Label: arm.label}
		lo, med, hi, ok := quartiles(arm.units)
		denom := countNoun(len(arm.units), "admitted session")
		switch {
		case !ok:
			reason := "no clean " + arm.id + "-arm session yet — a savings figure exists only as an arm range (law 12)"
			r.Units = ledgerValue(Measured(0, ""))
			r.Low, r.Median, r.High = ledgerValue(Absent[int](reason)), ledgerValue(Absent[int](reason)), ledgerValue(Absent[int](reason))
		default:
			r.Units = ledgerValue(Measured(len(arm.units), ""))
			r.Low = ledgerValue(Measured(lo, "")).WithDenominator("p25 of " + denom)
			r.Median = ledgerValue(Measured(med, "")).WithDenominator("median of " + denom)
			r.High = ledgerValue(Measured(hi, "")).WithDenominator("p75 of " + denom)
		}
		m.Arms = append(m.Arms, r)
	}
	m.Excluded = ledgerValue(Measured(in.Arms.Excluded, "")).WithDenominator("OFF-arm rows that began before the arm or carry unknown coverage — counted, never used")

	// Q1 — what the agent did not have to do.
	q1 := ImpactQuestion{ID: QuestionSaved, Question: "What did the agent not have to do?"}
	q1.Answer = in.Arms.Criterion
	if q1.Answer == "" {
		q1.Answer = "the arms are compared only once both carry admitted sessions"
	}
	q1.Figures = append(q1.Figures,
		ImpactFigure{ID: "arm.on.median", Label: "ON arm median", Unit: "tokens/turn", Value: m.Arms[0].Median},
		ImpactFigure{ID: "arm.off.median", Label: "OFF arm median", Unit: "tokens/turn", Value: m.Arms[1].Median},
		ImpactFigure{ID: "arm.off.sessions", Label: "OFF-arm sessions admitted", Value: m.Arms[1].Units},
	)
	expReason := in.ExpNote
	if expReason == "" {
		expReason = "no session carries the exploration ledger"
	}
	if in.ExpSessions > 0 && in.ExpRawCalls > 0 && in.ExpIdxCalls > 0 {
		q1.Figures = append(q1.Figures,
			ImpactFigure{ID: "exp.raw_per_question", Label: "raw exploration, per question (observational)", Unit: "tokens", Value: ledgerValue(Measured(in.ExpRawTokens/in.ExpRawCalls, "")).WithDenominator(countNoun(in.ExpRawCalls, "Grep/Read/Glob call"))},
			ImpactFigure{ID: "exp.indexed_per_question", Label: "indexed discovery, per question (observational)", Unit: "tokens", Value: ledgerValue(Measured(in.ExpIdxTokens/in.ExpIdxCalls, "")).WithDenominator(countNoun(in.ExpIdxCalls, "fathom answer"))},
		)
	} else {
		q1.Figures = append(q1.Figures,
			ImpactFigure{ID: "exp.raw_per_question", Label: "raw exploration, per question (observational)", Unit: "tokens", Value: ledgerValue(Absent[int](expReason))},
			ImpactFigure{ID: "exp.indexed_per_question", Label: "indexed discovery, per question (observational)", Unit: "tokens", Value: ledgerValue(Absent[int](expReason))},
		)
	}
	m.Questions = append(m.Questions, q1)

	// Q2 — what fathom prevented.
	q2 := ImpactQuestion{ID: QuestionPrevented, Question: "What did fathom prevent?"}
	gate := Absent[int]("not instrumented — the verify ledger records no `fathom verify` run").WithSource("the verify ledger")
	if in.GateRefusals != nil {
		gate = Measured(*in.GateRefusals, "the verify ledger").WithDenominator(fmt.Sprintf("refusals across %s", countNoun(in.GateRuns, "recorded verify run")))
	}
	stop := ledgerValue(Absent[int]("verify never ran in any ledger session — unknown, not zero"))
	if in.StopCatches != nil {
		stop = ledgerValue(Measured(*in.StopCatches, "")).WithDenominator(fmt.Sprintf("verify ran in %d of %s", in.StopCatchesRan, countNoun(in.Sessions, "ledger session")))
	}
	served := callValue(Measured(in.ObligationsServed, "")).WithDenominator("obligations calls — rules injected before an edit; whether the edit conformed is IQ-13")
	q2.Figures = []ImpactFigure{
		{ID: "gate.refusals", Label: "verify gate refusals", Value: gate},
		{ID: "stop.catches", Label: "stop-hook verify catches", Value: stop},
		{ID: "obligations.served", Label: "rules served before edits", Value: served},
	}
	switch {
	case in.GateRefusals != nil:
		q2.Answer = fmt.Sprintf("the verify gate refused %d of %s; every catch is on the timeline below", *in.GateRefusals, countNoun(in.GateRuns, "recorded run"))
	default:
		q2.Answer = "the gate has no recorded run yet — nothing is claimed as prevented"
	}
	m.Questions = append(m.Questions, q2)

	// Q3 — where the agent still flounders.
	q3 := ImpactQuestion{ID: QuestionFloundering, Question: "Where does the agent still flounder?"}
	misses, dym, instrumented := 0, 0, 0
	for _, t := range in.Tools {
		o := ToolOutcome{Tool: t.Tool, Class: t.Class, Calls: t.Calls}
		inst := t.Calls - t.NotRecorded
		if inst < 0 {
			inst = 0
		}
		o.Instrumented = callValue(Measured(inst, "")).WithDenominator("of " + countNoun(t.Calls, "call"))
		if inst == 0 {
			reason := notInstrumented(t.Calls)
			o.Hits, o.Misses, o.DidYouMean = callValue(Absent[int](reason)), callValue(Absent[int](reason)), callValue(Absent[int](reason))
			m.Uninstrumented++
		} else {
			denom := "of " + countNoun(inst, "instrumented call")
			o.Hits = callValue(Measured(t.Hits, "")).WithDenominator(denom)
			o.Misses = callValue(Measured(t.Misses, "")).WithDenominator(denom)
			o.DidYouMean = callValue(Measured(t.DidYouMean, "")).WithDenominator(denom)
			misses += t.Misses
			dym += t.DidYouMean
			instrumented += inst
		}
		m.Tools = append(m.Tools, o)
	}
	missValue := callValue(Measured(misses, "")).WithDenominator("across " + countNoun(instrumented, "instrumented call"))
	if instrumented == 0 && in.CallLogReason == "" {
		missValue = Absent[int]("no call in the log records an outcome").WithSource(in.CallLogSource)
	}
	q3.Figures = []ImpactFigure{
		{ID: "tools.misses", Label: "misses", Value: missValue},
		{ID: "tools.did_you_mean", Label: "did-you-mean answers", Value: func() Value[int] {
			if instrumented == 0 && in.CallLogReason == "" {
				return missValue
			}
			return callValue(Measured(dym, "")).WithDenominator("across " + countNoun(instrumented, "instrumented call"))
		}()},
		{ID: "tools.uninstrumented", Label: "tools with no outcome instrument", Value: callValue(Measured(m.Uninstrumented, "")).WithDenominator("of " + countNoun(len(in.Tools), "called tool"))},
	}
	if in.CallLogReason != "" {
		q3.Answer = in.CallLogReason
	} else {
		q3.Answer = fmt.Sprintf("%s and %s across %s; %s record no outcome and say so rather than show a zero", countNoun(misses, "miss"), countNoun(dym, "did-you-mean answer"), countNoun(instrumented, "instrumented call"), countNoun(m.Uninstrumented, "tool"))
	}
	m.Questions = append(m.Questions, q3)

	// Q4 — is the knowledge compounding.
	q4 := ImpactQuestion{ID: QuestionCompounding, Question: "Is the knowledge compounding?"}
	warm := ledgerValue(Absent[int](nonEmpty(in.FirstEditNote, "no session records its first edit turn")))
	if in.FirstEditMedian != nil {
		warm = ledgerValue(Measured(*in.FirstEditMedian, "")).WithDenominator("median across " + countNoun(in.FirstEditSessions, "session") + " with a recorded first edit")
	}
	q4.Figures = []ImpactFigure{
		{ID: "warmup.median", Label: "median warm-up", Unit: "turns to the first edit", Value: warm},
		{ID: "obligations.served", Label: "rules served before edits", Value: served},
	}
	q4.Answer = "a shorter warm-up and a growing rule supply are the signs; the growth curve arrives with weekly rollups"
	m.Questions = append(m.Questions, q4)
	return m
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
