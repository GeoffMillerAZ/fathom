//go:build decideexp

package decide_test

// exp_ask_test.go — what E2 (exp_e2_test.go) and E3 (exp_e3_test.go) share
// with E1's harness (exp_live_test.go): the kill check, one decider per run
// (the kit's validating mock in dry mode, else openExpDecider), and E1's
// stop rule over ask.Budget, factored for a request at a time. E1's own loop
// is left exactly as it ran; this is the same rule written once more:
// before a request is sent the budget admits its conservative estimate or
// stops the run for good, three failed requests in a row stop it too, a
// sent request is charged its reported usage when it has any and else its
// estimate (a failed one included), and each call has 90 seconds.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/ask"
	"github.com/geoffmilleraz/fathom/internal/testisolate"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// expKillReason is the KS-1 kill E1 honours, the value this suite
// inherited included (testisolate unsets FATHOM_DECIDE for every test), or
// "" when no kill holds.
func expKillReason() string {
	if strings.EqualFold(os.Getenv("FATHOM_DECIDE"), "off") || strings.EqualFold(testisolate.InheritedDecide(), "off") {
		return "FATHOM_DECIDE=off"
	}
	return ""
}

// expAsker sends one run's batched requests under E1's stop rule.
type expAsker struct {
	decider  decision.Decider
	mock     *decision.MockDecider // dry mode: each request's answer is scripted just before it is sent
	disclose bytes.Buffer
	budget   ask.Budget
	failures int
	stop     string
	killed   string // decide.Open answered decision.None: nothing is asked
}

// newExpAsker opens the run's decider: the validating mock when dry (no
// key looked at, nothing dialed), else decide.Open through openExpDecider.
// A decision.None from Open leaves killed set and the decider nil.
func newExpAsker(t *testing.T, dry bool) *expAsker {
	t.Helper()
	a := &expAsker{budget: ask.Budget{CapUSD: expCap(t), Price: ask.ListedPrice}}
	if dry {
		a.mock = decision.NewMockDecider("mock")
		a.mock.SetValidate(true) // refuse what every shipped adapter refuses
		a.decider = a.mock
		return a
	}
	d, killed, err := openExpDecider(os.Getenv("FATHOM_DECIDE_EXP_OPENROUTER_KEY_FILE"), &a.disclose)
	if err != nil {
		t.Fatalf("decide.Open: %v", err)
	}
	if killed {
		a.killed = "decide.Open answered decision.None (a kill holds)"
		return a
	}
	a.decider = d
	return a
}

// name is the decider's name, "none" when Open answered decision.None.
func (a *expAsker) name() string {
	if a.decider == nil {
		return "none"
	}
	return a.decider.Name()
}

// expSent is one request's outcome. Sent is false when the stop rule held
// it back; then nothing else is set.
type expSent struct {
	Sent    bool
	Resp    decision.Response
	Err     error
	USD     float64
	Latency time.Duration
}

// expRequestBytes is what the conservative estimate counts for a request:
// the state, each question's instructions and, for a choice, each option's
// name and criteria (E1's predicates carried none).
func expRequestBytes(state string, qs []decision.Question) int {
	n := len(state)
	for _, q := range qs {
		n += len(q.Instructions)
		for _, o := range q.Options {
			n += len(o.Name) + len(o.Criteria)
		}
	}
	return n
}

// send asks qs about state in one request, or holds it back under the stop
// rule. dry scripts the mock's answer and is called only when the request
// is sent in dry mode.
func (a *expAsker) send(ctx context.Context, state string, qs []decision.Question, dry func() decision.Response) expSent {
	estIn, estOut := ask.EstimateTokens(expRequestBytes(state, qs)), ask.OutputTokensPerQuestion*len(qs)
	if a.failures >= expMaxConsecutiveFailures {
		if a.stop == "" {
			a.stop = fmt.Sprintf("%d consecutive failed requests", a.failures)
		}
		return expSent{}
	}
	if !a.budget.Admit(estIn, estOut) {
		if a.stop == "" {
			a.stop = fmt.Sprintf("cap: the next request's estimate would pass $%g", a.budget.CapUSD)
		}
		return expSent{}
	}
	if a.mock != nil {
		a.mock.AddEntry(decision.MockEntry{Response: dry()})
	}
	callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	start := time.Now()
	resp, err := a.decider.Decide(callCtx, decision.Request{State: state, Questions: qs})
	wall := time.Since(start)
	cancel()
	out := expSent{Sent: true, Resp: resp, Err: err}
	out.USD = a.budget.Charge(resp.Usage.InputTokens, resp.Usage.OutputTokens, estIn, estOut)
	if err != nil {
		a.failures++
		return out
	}
	a.failures = 0
	out.Latency = resp.Latency
	if out.Latency <= 0 {
		out.Latency = wall
	}
	return out
}

// spend is the run's spend line: the tally, or 0 in dry mode, where the
// tally reads only what the run would have cost.
func (a *expAsker) spend() float64 {
	if a.mock != nil {
		return 0
	}
	return a.budget.SpentUSD
}

// expMockPredicate answers a predicate at p.
func expMockPredicate(id string, p float64) decision.Answer {
	return decision.Answer{ID: id, Kind: decision.KindPredicate, Noul: p, Confidence: max(p, 1-p)}
}

// expMockChoice answers a choice with pick at 0.95 and the rest of the
// mass split evenly over the other options.
func expMockChoice(id string, options []decision.Option, pick string) decision.Answer {
	probs := make(map[string]float64, len(options))
	rest := 0.05 / float64(len(options)-1)
	for _, o := range options {
		probs[o.Name] = rest
	}
	probs[pick] = 0.95
	return decision.Answer{ID: id, Kind: decision.KindChoice, Choice: pick, Probabilities: probs, Confidence: decision.Confidence(probs)}
}
