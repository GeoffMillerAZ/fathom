package decision_test

import (
	"context"
	"reflect"
	"testing"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/llm"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/typesafe"
	llmclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// silentDecider is a bespoke Decider that declares nothing: it implements
// Decider only, never CapabilityDeclarer.
type silentDecider struct{}

func (silentDecider) Name() string { return "silent" }

func (silentDecider) Decide(context.Context, decision.Request) (decision.Response, error) {
	return decision.Response{Provider: "silent"}, nil
}

// undeclared is the one value CapabilitiesOf returns for a Decider that
// declares nothing: Declared false and every class explicitly unknown.
func undeclared() decision.Capabilities {
	return decision.Capabilities{
		Latency:             decision.LatencyUnknown,
		Cost:                decision.CostUnknown,
		ConfidenceSemantics: decision.ConfidenceUnknown,
	}
}

func allKinds() []decision.Kind {
	return []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate}
}

func newTestTypeSafe(t *testing.T) *typesafe.TypeSafe {
	t.Helper()
	// A dummy key that matches no secret pattern; construction makes no
	// network call and nothing here calls Decide.
	ts, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{APIKey: "dummy-key"})
	if err != nil {
		t.Fatalf("NewTypeSafe: %v", err)
	}
	return ts
}

func TestCapabilitiesOfDeclaresKindsLimitsBatchingCostAndConfidence(t *testing.T) {
	declaredMock := decision.NewMockDecider("mock")
	declaredMock.SetCapabilities(decision.Capabilities{
		Declared: true,
		// Out of order and duplicated: CapabilitiesOf returns the fixed
		// order with each kind once.
		Kinds:               []decision.Kind{decision.KindPredicate, decision.KindChoice, decision.KindPredicate},
		MaxOptions:          4,
		MinScoreLevels:      3,
		MaxScoreLevels:      5,
		SingleRoundTrip:     true,
		MaxQuestionsPerCall: 8,
		Latency:             decision.LatencySubSecond,
		// Cost and ConfidenceSemantics left empty: CapabilitiesOf reports
		// them as the explicit unknown value, never "".
	})

	// A declarer that sets fields but says Declared=false is undeclared:
	// CapabilitiesOf never returns half a declaration.
	halfMock := decision.NewMockDecider("half")
	halfMock.SetCapabilities(decision.Capabilities{Kinds: allKinds(), MaxOptions: 9})

	// A declarer that names the yes/no kind by its decode alias "noul"
	// (DP-12) declares KindPredicate: the alias folds into it once, never
	// reported as a second, unknown kind.
	aliasMock := decision.NewMockDecider("alias")
	aliasMock.SetCapabilities(decision.Capabilities{
		Declared: true,
		Kinds:    []decision.Kind{decision.Kind("noul"), decision.KindChoice, decision.KindPredicate},
	})

	var nilMock *decision.MockDecider

	cases := []struct {
		name string
		d    decision.Decider
		want decision.Capabilities
	}{
		{
			name: "none answers nothing, costs nothing, makes no round trip",
			d:    decision.None{},
			want: decision.Capabilities{
				Declared:            true,
				Latency:             decision.LatencyNone,
				Cost:                decision.CostNone,
				ConfidenceSemantics: decision.ConfidenceNone,
			},
		},
		{
			name: "llm emulates every kind in one completion with self-reported numbers",
			d:    llm.NewLLM(llmclient.NewMockClient(), "test-model"),
			want: decision.Capabilities{
				Declared:            true,
				Kinds:               allKinds(),
				MaxOptions:          255,
				SingleRoundTrip:     true,
				Latency:             decision.LatencyUnknown,
				Cost:                decision.CostUnknown,
				ConfidenceSemantics: decision.ConfidenceSelfReported,
			},
		},
		{
			name: "typesafe answers every kind natively in one POST, 2..10 score levels",
			d:    newTestTypeSafe(t),
			want: decision.Capabilities{
				Declared:            true,
				Kinds:               allKinds(),
				MaxOptions:          255,
				MinScoreLevels:      2,
				MaxScoreLevels:      10,
				SingleRoundTrip:     true,
				Latency:             decision.LatencySubSecond,
				Cost:                decision.CostLow,
				ConfidenceSemantics: decision.ConfidenceNativeProbability,
			},
		},
		{
			name: "mock declared",
			d:    declaredMock,
			want: decision.Capabilities{
				Declared:            true,
				Kinds:               []decision.Kind{decision.KindChoice, decision.KindPredicate},
				MaxOptions:          4,
				MinScoreLevels:      3,
				MaxScoreLevels:      5,
				SingleRoundTrip:     true,
				MaxQuestionsPerCall: 8,
				Latency:             decision.LatencySubSecond,
				Cost:                decision.CostUnknown,
				ConfidenceSemantics: decision.ConfidenceUnknown,
			},
		},
		{
			name: "mock declaring the noul alias declares KindPredicate once",
			d:    aliasMock,
			want: decision.Capabilities{
				Declared:            true,
				Kinds:               []decision.Kind{decision.KindChoice, decision.KindPredicate},
				Latency:             decision.LatencyUnknown,
				Cost:                decision.CostUnknown,
				ConfidenceSemantics: decision.ConfidenceUnknown,
			},
		},
		{
			name: "mock undeclared by default",
			d:    decision.NewMockDecider("mock"),
			want: undeclared(),
		},
		{
			name: "mock with fields but Declared false",
			d:    halfMock,
			want: undeclared(),
		},
		{
			name: "bespoke decider that declares nothing",
			d:    silentDecider{},
			want: undeclared(),
		},
		{
			name: "nil decider",
			d:    nil,
			want: undeclared(),
		},
		{
			name: "typed nil declarer",
			d:    nilMock,
			want: undeclared(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decision.CapabilitiesOf(tc.d)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("CapabilitiesOf() =\n  %+v\nwant\n  %+v", got, tc.want)
			}
			// Deterministic: a second call returns the same value.
			if again := decision.CapabilitiesOf(tc.d); !reflect.DeepEqual(again, got) {
				t.Errorf("second CapabilitiesOf() = %+v, want %+v", again, got)
			}
		})
	}

	// The shipped adapters declare; the interface is optional, so Decider
	// itself is unchanged and silentDecider still satisfies it.
	var _ decision.CapabilityDeclarer = decision.None{}
	var _ decision.CapabilityDeclarer = (*llm.LLM)(nil)
	var _ decision.CapabilityDeclarer = (*typesafe.TypeSafe)(nil)
	var _ decision.CapabilityDeclarer = (*decision.MockDecider)(nil)
	var _ decision.Decider = silentDecider{}
}

// sharingDecider declares by returning the same backing slice on every
// call, so only CapabilitiesOf's own copy keeps callers from mutating it.
type sharingDecider struct {
	silentDecider
	kinds []decision.Kind
}

func (s sharingDecider) Name() string { return "sharing" }

func (s sharingDecider) Capabilities() decision.Capabilities {
	return decision.Capabilities{Declared: true, Kinds: s.kinds}
}

func TestCapabilitiesOfReturnsCopies(t *testing.T) {
	mockKinds := []decision.Kind{decision.KindChoice, decision.KindScore}
	mock := decision.NewMockDecider("mock")
	mock.SetCapabilities(decision.Capabilities{Declared: true, Kinds: mockKinds})
	// Mutating the slice passed to SetCapabilities does not change the
	// declaration.
	mockKinds[0] = "mutated"

	deciders := []decision.Decider{
		llm.NewLLM(llmclient.NewMockClient(), "test-model"),
		newTestTypeSafe(t),
		mock,
		sharingDecider{kinds: []decision.Kind{decision.KindChoice, decision.KindPredicate}},
	}
	for _, d := range deciders {
		t.Run(d.Name(), func(t *testing.T) {
			first := decision.CapabilitiesOf(d)
			if len(first.Kinds) == 0 {
				t.Fatalf("CapabilitiesOf(%s).Kinds is empty", d.Name())
			}
			if first.Kinds[0] != decision.KindChoice {
				t.Fatalf("CapabilitiesOf(%s).Kinds[0] = %q, want %q", d.Name(), first.Kinds[0], decision.KindChoice)
			}
			first.Kinds[0] = "mutated"
			first.Kinds = append(first.Kinds, "appended")

			second := decision.CapabilitiesOf(d)
			for _, k := range second.Kinds {
				if k == "mutated" || k == "appended" {
					t.Fatalf("CapabilitiesOf(%s) after mutating a returned value = %v; the adapter's declaration changed", d.Name(), second.Kinds)
				}
			}
			if second.Kinds[0] != decision.KindChoice {
				t.Errorf("CapabilitiesOf(%s).Kinds[0] = %q, want %q", d.Name(), second.Kinds[0], decision.KindChoice)
			}
		})
	}
}

// The class values are a vocabulary a consumer may log or persist beside a
// decision record, so their strings are pinned.
func TestCapabilityClassesHaveStableValues(t *testing.T) {
	latency := map[decision.LatencyClass]string{
		decision.LatencyUnknown:   "unknown",
		decision.LatencyNone:      "none",
		decision.LatencySubSecond: "sub-second",
		decision.LatencySeconds:   "seconds",
	}
	cost := map[decision.CostClass]string{
		decision.CostUnknown: "unknown",
		decision.CostNone:    "none",
		decision.CostLow:     "low",
		decision.CostMedium:  "medium",
		decision.CostHigh:    "high",
	}
	confidence := map[decision.ConfidenceSemantics]string{
		decision.ConfidenceUnknown:               "unknown",
		decision.ConfidenceNone:                  "none",
		decision.ConfidenceCalibratedProbability: "calibrated-probability",
		decision.ConfidenceLogprob:               "logprob",
		decision.ConfidenceNativeProbability:     "native-probability",
		decision.ConfidenceSelfReported:          "self-reported",
	}
	for got, want := range latency {
		if string(got) != want {
			t.Errorf("LatencyClass %q, want %q", got, want)
		}
	}
	for got, want := range cost {
		if string(got) != want {
			t.Errorf("CostClass %q, want %q", got, want)
		}
	}
	for got, want := range confidence {
		if string(got) != want {
			t.Errorf("ConfidenceSemantics %q, want %q", got, want)
		}
	}
	if len(latency) != 4 || len(cost) != 5 || len(confidence) != 6 {
		t.Errorf("class values collide: %d latency, %d cost, %d confidence distinct keys", len(latency), len(cost), len(confidence))
	}
}
