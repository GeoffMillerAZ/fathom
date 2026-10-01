package decision_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// decorStub is a stateless, concurrency-safe Decider for decorator tests:
// fn answers every call, and calls counts them. Unlike MockDecider it does
// not script responses in call order, so concurrent callers each get the
// answer their own Request deserves.
type decorStub struct {
	name  string
	caps  decision.Capabilities
	fn    func(req decision.Request) (decision.Response, error)
	calls atomic.Int64
}

func (s *decorStub) Name() string { return s.name }

func (s *decorStub) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	s.calls.Add(1)
	return s.fn(req)
}

func (s *decorStub) Capabilities() decision.Capabilities { return s.caps }

// decorPredicate returns a valid predicate Answer whose Confidence follows
// the package rule: Noul when Noul >= 0.5, else 1-Noul.
func decorPredicate(id string, noul float64) decision.Answer {
	c := noul
	if noul < 0.5 {
		c = 1 - noul
	}
	return decision.Answer{ID: id, Kind: decision.KindPredicate, Noul: noul, Confidence: c}
}

// decorChoice returns a valid choice Answer whose Confidence is this
// package's Confidence over probs.
func decorChoice(id, choice string, probs map[string]float64) decision.Answer {
	return decision.Answer{ID: id, Kind: decision.KindChoice, Choice: choice, Probabilities: probs, Confidence: decision.Confidence(probs)}
}

// decorQuestion is a predicate Question whose Instructions name its ID, so
// two decorQuestions with different IDs are different templates.
func decorQuestion(id string) decision.Question {
	return decision.Question{ID: id, Kind: decision.KindPredicate, Instructions: "is " + id + " true?"}
}

// decorChoiceQuestion is a three-option choice Question.
func decorChoiceQuestion(id string) decision.Question {
	return decision.Question{ID: id, Kind: decision.KindChoice, Instructions: "which " + id + "?", Options: []decision.Option{{Name: "a"}, {Name: "b"}, {Name: "c"}}}
}

// decorIDs returns the Question IDs of req, in order.
func decorIDs(req decision.Request) []string {
	ids := make([]string, len(req.Questions))
	for i, q := range req.Questions {
		ids[i] = q.ID
	}
	return ids
}

// decorEcho answers every predicate question of req with noul, under
// provider and model.
func decorEcho(provider, model string, noul float64) func(decision.Request) (decision.Response, error) {
	return func(req decision.Request) (decision.Response, error) {
		answers := make(map[string]decision.Answer, len(req.Questions))
		for _, q := range req.Questions {
			answers[q.ID] = decorPredicate(q.ID, noul)
		}
		return decision.Response{Provider: provider, Model: model, Answers: answers, Invalid: map[string]string{}}, nil
	}
}

func TestCascadeEscalatesOnAbstainOrLowConfidence(t *testing.T) {
	ctx := context.Background()
	req := decision.Request{State: "the build log", Questions: []decision.Question{
		decorQuestion("q1"), decorQuestion("q2"), decorChoiceQuestion("q3"), decorQuestion("q4"),
	}}
	lowProbs := map[string]float64{"a": 0.4, "b": 0.35, "c": 0.25} // Confidence 0.1
	highProbs := map[string]float64{"a": 0.02, "b": 0.96, "c": 0.02}

	t.Run("abstain, invalid, absent and low confidence escalate; provenance is recoverable", func(t *testing.T) {
		cheap := decision.NewMockDecider("cheap", decision.MockEntry{Response: decision.Response{
			Provider: "cheap", Model: "c-1",
			Answers: map[string]decision.Answer{
				"q1": decorPredicate("q1", 0.95),
				"q3": decorChoice("q3", "a", lowProbs),
			},
			Invalid: map[string]string{"q2": "choice not in options"},
			Usage:   decision.Usage{InputTokens: 10, OutputTokens: 1},
			Latency: 2 * time.Millisecond,
		}})
		mid := decision.NewMockDecider("mid", decision.MockEntry{Response: decision.Response{Provider: "mid"}, Err: decision.ErrNoProvider})
		frontier := decision.NewMockDecider("frontier", decision.MockEntry{Response: decision.Response{
			Provider: "frontier", Model: "f-9",
			Answers: map[string]decision.Answer{
				"q2": decorPredicate("q2", 0.1),
				"q3": decorChoice("q3", "b", highProbs),
			},
			Invalid: map[string]string{"q4": "out of range"},
			Usage:   decision.Usage{InputTokens: 30, OutputTokens: 3},
			Latency: 7 * time.Millisecond,
		}})

		var observed []decision.CascadeReport
		c, err := decision.NewCascade(decision.CascadeOptions{
			Stages:  []decision.Decider{cheap, mid, frontier},
			Observe: func(r decision.CascadeReport) { observed = append(observed, r) },
		})
		if err != nil {
			t.Fatalf("NewCascade() error = %v", err)
		}
		if got := c.Name(); got != "cascade" {
			t.Errorf("Name() = %q, want cascade", got)
		}

		resp, report, err := c.DecideReport(ctx, req)
		if err != nil {
			t.Fatalf("DecideReport() error = %v", err)
		}

		// Stage order is caller order; each later stage sees only the
		// unresolved questions, in request order, with the same State.
		if got := decorIDs(cheap.Calls()[0]); !reflect.DeepEqual(got, []string{"q1", "q2", "q3", "q4"}) {
			t.Errorf("cheap asked %v", got)
		}
		for _, st := range []*decision.MockDecider{mid, frontier} {
			calls := st.Calls()
			if len(calls) != 1 {
				t.Fatalf("%s called %d times, want 1", st.Name(), len(calls))
			}
			if got := decorIDs(calls[0]); !reflect.DeepEqual(got, []string{"q2", "q3", "q4"}) {
				t.Errorf("%s asked %v, want [q2 q3 q4]", st.Name(), got)
			}
			if calls[0].State != req.State {
				t.Errorf("%s State = %q, want %q", st.Name(), calls[0].State, req.State)
			}
		}

		wantAnswers := map[string]decision.Answer{
			"q1": decorPredicate("q1", 0.95),
			"q2": decorPredicate("q2", 0.1),
			"q3": decorChoice("q3", "b", highProbs),
		}
		if !reflect.DeepEqual(resp.Answers, wantAnswers) {
			t.Errorf("Answers = %+v, want %+v", resp.Answers, wantAnswers)
		}
		if !reflect.DeepEqual(resp.Invalid, map[string]string{"q4": "out of range"}) {
			t.Errorf("Invalid = %v", resp.Invalid)
		}
		// Answers from two stages: never labeled with either stage's
		// Provider or Model.
		if resp.Provider != "cascade" || resp.Model != "" {
			t.Errorf("mixed Response labeled %q/%q, want cascade/\"\"", resp.Provider, resp.Model)
		}
		if resp.Usage != (decision.Usage{InputTokens: 40, OutputTokens: 4}) {
			t.Errorf("Usage = %+v, want the sum over stages", resp.Usage)
		}
		if resp.Latency != 9*time.Millisecond {
			t.Errorf("Latency = %v, want the sum over stages", resp.Latency)
		}

		wantSources := map[string]decision.AnswerSource{
			"q1": {Stage: 0, Provider: "cheap", Model: "c-1"},
			"q2": {Stage: 2, Provider: "frontier", Model: "f-9"},
			"q3": {Stage: 2, Provider: "frontier", Model: "f-9"},
		}
		if !reflect.DeepEqual(report.Sources, wantSources) {
			t.Errorf("Sources = %+v, want %+v", report.Sources, wantSources)
		}
		if len(report.Steps) != 3 {
			t.Fatalf("len(Steps) = %d, want 3", len(report.Steps))
		}
		if !report.Steps[1].Abstained || report.Steps[0].Abstained {
			t.Errorf("Abstained = %v/%v/%v, want only mid", report.Steps[0].Abstained, report.Steps[1].Abstained, report.Steps[2].Abstained)
		}
		if got := report.Steps[0].Escalated; len(got) != 3 || got["q2"] == "" || got["q3"] == "" || got["q4"] == "" {
			t.Errorf("cheap Escalated = %v, want reasons for q2, q3, q4", got)
		}
		if len(report.LowConfidence) != 0 {
			t.Errorf("LowConfidence = %v, want none", report.LowConfidence)
		}
		if len(observed) != 1 || !reflect.DeepEqual(observed[0], report) {
			t.Errorf("Observe got %d reports, want the one DecideReport returned", len(observed))
		}

		// Split back into per-stage Responses, each under its own
		// Provider and Model, so Records attribute every answer correctly.
		parts := report.Responses(resp)
		if len(parts) != 2 {
			t.Fatalf("len(Responses) = %d, want 2", len(parts))
		}
		if parts[0].Provider != "cheap" || parts[0].Model != "c-1" || len(parts[0].Answers) != 1 {
			t.Errorf("part 0 = %+v", parts[0])
		}
		if parts[1].Provider != "frontier" || parts[1].Model != "f-9" || len(parts[1].Answers) != 2 {
			t.Errorf("part 1 = %+v", parts[1])
		}
		for _, rec := range parts[1].Records("site") {
			if rec.Provider != "frontier" {
				t.Errorf("record %s under %q, want frontier", rec.QuestionID, rec.Provider)
			}
		}
	})

	t.Run("resolved by the first stage: later stages never called, labeled as that stage", func(t *testing.T) {
		cheap := &decorStub{name: "cheap", fn: decorEcho("cheap", "c-1", 0.97)}
		pricey := &decorStub{name: "pricey", fn: decorEcho("pricey", "p-1", 0.99)}
		c, err := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{cheap, pricey}})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := c.Decide(ctx, decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1"), decorQuestion("q2")}})
		if err != nil {
			t.Fatal(err)
		}
		if pricey.calls.Load() != 0 {
			t.Errorf("pricey called %d times, want 0", pricey.calls.Load())
		}
		if resp.Provider != "cheap" || resp.Model != "c-1" || len(resp.Answers) != 2 {
			t.Errorf("Response = %+v, want both answers under cheap/c-1", resp)
		}
	})

	t.Run("a later stage's answers are never reported under an earlier stage", func(t *testing.T) {
		cheap := &decorStub{name: "cheap", fn: func(req decision.Request) (decision.Response, error) {
			inv := map[string]string{}
			for _, q := range req.Questions {
				inv[q.ID] = "cannot answer"
			}
			return decision.Response{Provider: "cheap", Model: "c-1", Invalid: inv}, nil
		}}
		pricey := &decorStub{name: "pricey", fn: decorEcho("pricey", "p-1", 0.99)}
		c, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{cheap, pricey}})
		resp, err := c.Decide(ctx, decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1")}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Provider != "pricey" || resp.Model != "p-1" {
			t.Errorf("labeled %q/%q, want pricey/p-1", resp.Provider, resp.Model)
		}
		if len(resp.Invalid) != 0 {
			t.Errorf("Invalid = %v, want empty once a later stage answered", resp.Invalid)
		}
	})

	t.Run("low confidence at every stage keeps the latest valid answer and discloses it", func(t *testing.T) {
		cheap := decision.NewMockDecider("cheap", decision.MockEntry{Response: decision.Response{
			Provider: "cheap", Model: "c-1",
			Answers: map[string]decision.Answer{"q3": decorChoice("q3", "a", lowProbs)},
		}})
		c, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{cheap, decision.None{}}})
		resp, report, err := c.DecideReport(ctx, decision.Request{State: "s", Questions: []decision.Question{decorChoiceQuestion("q3")}})
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Answers["q3"].Choice; got != "a" {
			t.Errorf("q3 = %q, want the low-confidence answer a", got)
		}
		if resp.Provider != "cheap" {
			t.Errorf("Provider = %q, want cheap", resp.Provider)
		}
		if !reflect.DeepEqual(report.LowConfidence, []string{"q3"}) {
			t.Errorf("LowConfidence = %v, want [q3]", report.LowConfidence)
		}
	})

	t.Run("the threshold is configurable and a negative one disables it", func(t *testing.T) {
		for _, tc := range []struct {
			below     float64
			escalates bool
		}{{0, false}, {0.8, true}, {-1, false}} {
			cheap := &decorStub{name: "cheap", fn: decorEcho("cheap", "c-1", 0.7)}
			pricey := &decorStub{name: "pricey", fn: decorEcho("pricey", "p-1", 0.99)}
			c, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{cheap, pricey}, EscalateBelow: tc.below})
			if _, err := c.Decide(ctx, decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1")}}); err != nil {
				t.Fatal(err)
			}
			if got := pricey.calls.Load() == 1; got != tc.escalates {
				t.Errorf("EscalateBelow %v: escalated = %v, want %v", tc.below, got, tc.escalates)
			}
		}
		if decision.DefaultEscalateBelow != decision.UnsureBelow {
			t.Errorf("DefaultEscalateBelow = %v, want UnsureBelow", decision.DefaultEscalateBelow)
		}
	})

	t.Run("every stage abstaining is ErrNoProvider", func(t *testing.T) {
		c, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{decision.None{}, decision.None{}}})
		resp, err := c.Decide(ctx, decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1")}})
		if !errors.Is(err, decision.ErrNoProvider) {
			t.Fatalf("err = %v, want ErrNoProvider", err)
		}
		if resp.Provider != "cascade" {
			t.Errorf("Provider = %q, want cascade", resp.Provider)
		}
	})

	t.Run("a stage error that is not an abstain fails the call", func(t *testing.T) {
		boom := errors.New("boom")
		failing := &decorStub{name: "failing", fn: func(decision.Request) (decision.Response, error) { return decision.Response{}, boom }}
		later := &decorStub{name: "later", fn: decorEcho("later", "l", 0.99)}
		c, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{failing, later}})
		_, err := c.Decide(ctx, decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1")}})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want boom", err)
		}
		if later.calls.Load() != 0 {
			t.Errorf("later called after a hard error")
		}
	})

	t.Run("an invalid Request reaches no stage", func(t *testing.T) {
		st := &decorStub{name: "st", fn: decorEcho("st", "m", 0.9)}
		c, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{st}})
		if _, err := c.Decide(ctx, decision.Request{}); err == nil {
			t.Fatal("want a Validate error")
		}
		if st.calls.Load() != 0 {
			t.Errorf("stage called for an invalid Request")
		}
	})

	t.Run("OrderByCost puts the cheaper declared stage first", func(t *testing.T) {
		high := &decorStub{name: "high", caps: decision.Capabilities{Declared: true, Kinds: []decision.Kind{decision.KindPredicate}, Cost: decision.CostHigh}, fn: decorEcho("high", "h", 0.99)}
		unknown := &decorStub{name: "unknown", fn: decorEcho("unknown", "u", 0.99)}
		low := &decorStub{name: "low", caps: decision.Capabilities{Declared: true, Kinds: []decision.Kind{decision.KindPredicate}, Cost: decision.CostLow}, fn: decorEcho("low", "l", 0.99)}
		c, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{unknown, high, low}, OrderByCost: true})
		if got := c.Stages(); !reflect.DeepEqual(got, []string{"low", "high", "unknown"}) {
			t.Errorf("Stages() = %v, want [low high unknown]", got)
		}
		resp, err := c.Decide(ctx, decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1")}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Provider != "low" || high.calls.Load() != 0 {
			t.Errorf("Provider = %q, high calls %d; want low first", resp.Provider, high.calls.Load())
		}
	})
}

func TestNewCascadeRejectsEmptyOrNilStage(t *testing.T) {
	if _, err := decision.NewCascade(decision.CascadeOptions{}); err == nil {
		t.Error("no stages: want an error")
	}
	if _, err := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{decision.None{}, nil}}); err == nil {
		t.Error("nil stage: want an error")
	}
	if _, err := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{decision.None{}}, EscalateBelow: 1.5}); err == nil {
		t.Error("EscalateBelow above 1: want an error")
	}
}

func TestCascadeDeclaresCapabilitiesFromItsStages(t *testing.T) {
	cheap := &decorStub{name: "cheap", fn: decorEcho("cheap", "c", 0.9), caps: decision.Capabilities{
		Declared: true, Kinds: []decision.Kind{decision.KindChoice, decision.KindPredicate}, MaxOptions: 16,
		SingleRoundTrip: true, MaxQuestionsPerCall: 20, Latency: decision.LatencySubSecond, Cost: decision.CostLow,
		ConfidenceSemantics: decision.ConfidenceNativeProbability,
	}}
	pricey := &decorStub{name: "pricey", fn: decorEcho("pricey", "p", 0.9), caps: decision.Capabilities{
		Declared: true, Kinds: []decision.Kind{decision.KindChoice, decision.KindScore}, MaxOptions: 64,
		MinScoreLevels: 2, MaxScoreLevels: 10, SingleRoundTrip: true, MaxQuestionsPerCall: 8,
		Latency: decision.LatencySeconds, Cost: decision.CostMedium, ConfidenceSemantics: decision.ConfidenceNativeProbability,
	}}
	c, err := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{cheap, decision.None{}, pricey}})
	if err != nil {
		t.Fatal(err)
	}
	want := decision.Capabilities{
		Declared:            true,
		Kinds:               []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate},
		MaxOptions:          64,
		MinScoreLevels:      2,
		MaxScoreLevels:      10,
		SingleRoundTrip:     true, // None answers no kind, so it does not count
		MaxQuestionsPerCall: 8,
		Latency:             decision.LatencySeconds,
		Cost:                decision.CostMedium,
		ConfidenceSemantics: decision.ConfidenceNativeProbability,
	}
	if got := decision.CapabilitiesOf(c); !reflect.DeepEqual(got, want) {
		t.Errorf("CapabilitiesOf(cascade) =\n %+v\nwant\n %+v", got, want)
	}

	// One undeclared stage makes the whole cascade undeclared.
	c2, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{cheap, &decorStub{name: "anon", fn: decorEcho("anon", "", 0.9)}}})
	if got := decision.CapabilitiesOf(c2); got.Declared || got.Cost != decision.CostUnknown {
		t.Errorf("with an undeclared stage: %+v, want undeclared", got)
	}
}

func TestCascadeConcurrentUse(t *testing.T) {
	cheap := &decorStub{name: "cheap", fn: func(req decision.Request) (decision.Response, error) {
		// Confident on even-numbered questions only.
		answers := map[string]decision.Answer{}
		for i, q := range req.Questions {
			noul := 0.95
			if i%2 == 1 {
				noul = 0.6
			}
			answers[q.ID] = decorPredicate(q.ID, noul)
		}
		return decision.Response{Provider: "cheap", Model: "c", Answers: answers}, nil
	}}
	pricey := &decorStub{name: "pricey", fn: decorEcho("pricey", "p", 0.99)}
	var observed atomic.Int64
	c, err := decision.NewCascade(decision.CascadeOptions{
		Stages: []decision.Decider{cheap, pricey}, EscalateBelow: 0.9,
		Observe: func(decision.CascadeReport) { observed.Add(1) },
	})
	if err != nil {
		t.Fatal(err)
	}
	const n = 32
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := decision.Request{State: fmt.Sprintf("state-%d", i), Questions: []decision.Question{decorQuestion("q0"), decorQuestion("q1")}}
			resp, report, err := c.DecideReport(context.Background(), req)
			if err != nil {
				errs <- err
				return
			}
			if report.Sources["q0"].Provider != "cheap" || report.Sources["q1"].Provider != "pricey" || resp.Provider != "cascade" {
				errs <- fmt.Errorf("goroutine %d: sources %+v, provider %q", i, report.Sources, resp.Provider)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if observed.Load() != n {
		t.Errorf("Observe called %d times, want %d", observed.Load(), n)
	}
}
