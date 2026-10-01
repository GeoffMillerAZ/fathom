package decision_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/typesafe"
)

func TestFallbackUsesNextDeciderOnError(t *testing.T) {
	ctx := context.Background()
	req := decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1")}}

	t.Run("an error moves to the next Decider, which answers under its own name, and the failures are disclosed", func(t *testing.T) {
		unauthorized := &typesafe.TypeSafeError{Status: 401, Message: "missing key"}
		native := decision.NewMockDecider("native", decision.MockEntry{Err: unauthorized})
		llm := decision.NewMockDecider("llm", decision.MockEntry{Response: decision.Response{
			Provider: "llm", Model: "m-2", Answers: map[string]decision.Answer{"q1": decorPredicate("q1", 0.8)},
		}})

		var observed []decision.FallbackReport
		f, err := decision.NewFallback(decision.FallbackOptions{
			Deciders: []decision.Decider{native, decision.None{}, llm},
			Observe:  func(r decision.FallbackReport) { observed = append(observed, r) },
		})
		if err != nil {
			t.Fatalf("NewFallback() error = %v", err)
		}
		if got := f.Name(); got != "fallback" {
			t.Errorf("Name() = %q, want fallback", got)
		}

		resp, report, err := f.DecideReport(ctx, req)
		if err != nil {
			t.Fatalf("DecideReport() error = %v", err)
		}
		// Never silent: the Response names the Decider that answered.
		if resp.Provider != "llm" || resp.Model != "m-2" {
			t.Errorf("Response labeled %q/%q, want llm/m-2", resp.Provider, resp.Model)
		}
		if report.Answered != 2 || report.Provider != "llm" || report.Model != "m-2" {
			t.Errorf("report answered %d by %q/%q, want 2 by llm/m-2", report.Answered, report.Provider, report.Model)
		}
		if len(report.Failures) != 2 {
			t.Fatalf("len(Failures) = %d, want 2", len(report.Failures))
		}
		var tse *typesafe.TypeSafeError
		if f0 := report.Failures[0]; f0.Index != 0 || f0.Name != "native" || !errors.As(f0.Err, &tse) || tse.Retryable() {
			t.Errorf("failure 0 = %+v, want native's non-retryable TypeSafeError", f0)
		}
		if f1 := report.Failures[1]; f1.Index != 1 || f1.Name != "none" || !errors.Is(f1.Err, decision.ErrNoProvider) {
			t.Errorf("failure 1 = %+v, want none's ErrNoProvider", f1)
		}
		if len(observed) != 1 || !reflect.DeepEqual(observed[0], report) {
			t.Errorf("Observe got %v, want the one report", observed)
		}
	})

	t.Run("the next Decider runs only on an error", func(t *testing.T) {
		first := &decorStub{name: "first", fn: func(req decision.Request) (decision.Response, error) {
			// A valid Response whose every question is Invalid is not an
			// error: it is returned as is (escalation is Cascade's job).
			return decision.Response{Provider: "first", Model: "f", Invalid: map[string]string{"q1": "out of range"}}, nil
		}}
		second := &decorStub{name: "second", fn: decorEcho("second", "s", 0.9)}
		f, _ := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{first, second}, Observe: func(decision.FallbackReport) {}})
		resp, report, err := f.DecideReport(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if second.calls.Load() != 0 {
			t.Errorf("second called %d times after a nil error", second.calls.Load())
		}
		if resp.Provider != "first" || report.Answered != 0 || len(report.Failures) != 0 {
			t.Errorf("resp %q, report %+v; want first with no failures", resp.Provider, report)
		}
	})

	t.Run("every Decider failing returns every failure", func(t *testing.T) {
		limited := &typesafe.TypeSafeError{Status: 429, Message: "slow down"}
		native := decision.NewMockDecider("native", decision.MockEntry{Err: limited})
		f, _ := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{native, decision.None{}}, Observe: func(decision.FallbackReport) {}})
		_, report, err := f.DecideReport(ctx, req)
		var tse *typesafe.TypeSafeError
		if !errors.As(err, &tse) || !tse.Retryable() {
			t.Errorf("err = %v, want it to carry the retryable TypeSafeError", err)
		}
		if !errors.Is(err, decision.ErrNoProvider) {
			t.Errorf("err = %v, want it to carry ErrNoProvider", err)
		}
		if report.Answered != -1 || len(report.Failures) != 2 {
			t.Errorf("report = %+v, want Answered -1 and two failures", report)
		}
	})

	t.Run("a done context stops the fallback", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		first := &decorStub{name: "first", fn: func(decision.Request) (decision.Response, error) {
			cancel()
			return decision.Response{}, context.Canceled
		}}
		second := &decorStub{name: "second", fn: decorEcho("second", "s", 0.9)}
		f, _ := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{first, second}, Observe: func(decision.FallbackReport) {}})
		if _, err := f.Decide(cctx, req); !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
		if second.calls.Load() != 0 {
			t.Errorf("second called after the context was done")
		}
	})

	t.Run("an invalid Request reaches no Decider", func(t *testing.T) {
		first := &decorStub{name: "first", fn: decorEcho("first", "f", 0.9)}
		f, _ := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{first}, Observe: func(decision.FallbackReport) {}})
		if _, err := f.Decide(ctx, decision.Request{}); err == nil {
			t.Fatal("want a Validate error")
		}
		if first.calls.Load() != 0 {
			t.Errorf("Decider called for an invalid Request")
		}
	})
}

func TestNewFallbackRequiresDecidersAndAnObserver(t *testing.T) {
	observe := func(decision.FallbackReport) {}
	if _, err := decision.NewFallback(decision.FallbackOptions{Observe: observe}); err == nil {
		t.Error("no Deciders: want an error")
	}
	if _, err := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{decision.None{}, nil}, Observe: observe}); err == nil {
		t.Error("nil Decider: want an error")
	}
	// Without an observer a switch of provider would be silent.
	if _, err := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{decision.None{}}}); err == nil {
		t.Error("no Observe: want an error")
	}
}

func TestFallbackDeclaresCapabilitiesFromItsDeciders(t *testing.T) {
	native := &decorStub{name: "native", fn: decorEcho("native", "n", 0.9), caps: decision.Capabilities{
		Declared: true, Kinds: []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate},
		MaxOptions: 255, MinScoreLevels: 2, MaxScoreLevels: 10, SingleRoundTrip: true,
		Latency: decision.LatencySubSecond, Cost: decision.CostLow, ConfidenceSemantics: decision.ConfidenceNativeProbability,
	}}
	f, _ := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{native, decision.None{}}, Observe: func(decision.FallbackReport) {}})
	want := decision.Capabilities{
		Declared: true, Kinds: []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate},
		MaxOptions: 255, MinScoreLevels: 2, MaxScoreLevels: 10, SingleRoundTrip: true,
		Latency: decision.LatencySubSecond, Cost: decision.CostLow, ConfidenceSemantics: decision.ConfidenceNativeProbability,
	}
	if got := decision.CapabilitiesOf(f); !reflect.DeepEqual(got, want) {
		t.Errorf("CapabilitiesOf(fallback) =\n %+v\nwant\n %+v", got, want)
	}
	f2, _ := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{native, decision.NewMockDecider("anon")}, Observe: func(decision.FallbackReport) {}})
	if got := decision.CapabilitiesOf(f2); got.Declared {
		t.Errorf("with an undeclared Decider: %+v, want undeclared", got)
	}
}

func TestFallbackConcurrentUse(t *testing.T) {
	var n atomic.Int64
	flaky := &decorStub{name: "flaky", fn: func(req decision.Request) (decision.Response, error) {
		if n.Add(1)%2 == 0 {
			return decision.Response{}, errors.New("transient")
		}
		return decorEcho("flaky", "f", 0.9)(req)
	}}
	steady := &decorStub{name: "steady", fn: decorEcho("steady", "s", 0.9)}
	var failures atomic.Int64
	f, err := decision.NewFallback(decision.FallbackOptions{
		Deciders: []decision.Decider{flaky, steady},
		Observe:  func(r decision.FallbackReport) { failures.Add(int64(len(r.Failures))) },
	})
	if err != nil {
		t.Fatal(err)
	}
	const calls = 32
	var wg sync.WaitGroup
	errs := make(chan error, calls)
	var bySteady atomic.Int64
	for i := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := f.Decide(context.Background(), decision.Request{State: fmt.Sprintf("s%d", i), Questions: []decision.Question{decorQuestion("q1")}})
			if err != nil {
				errs <- err
				return
			}
			if resp.Provider == "steady" {
				bySteady.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if failures.Load() != bySteady.Load() {
		t.Errorf("disclosed %d failures, but steady answered %d calls", failures.Load(), bySteady.Load())
	}
}
