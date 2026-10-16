package decision_test

import (
	"errors"
	"fmt"
	"math"
	"testing"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/decisiontest"
)

// The LLM and TypeSafe adapters run the same suite in their own packages:
// TestLLMPassesConformance in foundations/decision/llm and
// TestTypeSafePassesConformance in foundations/decision/typesafe.

func TestNonePassesConformance(t *testing.T) {
	decisiontest.RunConformance(t, decisiontest.Harness{
		New: func(*testing.T, decisiontest.Script) decisiontest.Instance {
			return decisiontest.Instance{Decider: decision.None{}, ProviderCalls: func() int { return 0 }}
		},
	})
}

// mockConformanceResponse builds the Response a well-behaved provider would
// give for s, as a test author scripts a MockDecider: an omitted answer or a
// choice outside the options lands in Invalid, and every Confidence is the
// package's formula.
func mockConformanceResponse(s decisiontest.Script) decision.Response {
	resp := decision.Response{
		Provider: "mock",
		Model:    "decisiontest-model",
		Answers:  map[string]decision.Answer{},
		Invalid:  map[string]string{},
	}
	scripted := map[string]decisiontest.ScriptedAnswer{}
	for _, a := range s.Answers {
		scripted[a.ID] = a
	}
	for _, q := range s.Request.Questions {
		a, ok := scripted[q.ID]
		if !ok {
			resp.Invalid[q.ID] = "no answer"
			continue
		}
		if a.Refused {
			if resp.Declined == nil {
				resp.Declined = map[string]string{}
			}
			resp.Declined[q.ID] = "the provider refused to answer"
			continue
		}
		switch q.Kind {
		case decision.KindChoice:
			known := false
			for _, o := range q.Options {
				known = known || o.Name == a.Choice
			}
			if !known {
				resp.Invalid[q.ID] = "choice is not one of the options"
				continue
			}
			resp.Answers[q.ID] = decision.Answer{ID: q.ID, Kind: q.Kind, Choice: a.Choice, Probabilities: a.Probabilities, Confidence: decision.Confidence(a.Probabilities)}
		case decision.KindScore:
			resp.Answers[q.ID] = decision.Answer{ID: q.ID, Kind: q.Kind, Score: a.Score, Confidence: decision.Confidence(nil)}
		default:
			resp.Answers[q.ID] = decision.Answer{ID: q.ID, Kind: q.Kind, Noul: a.Probability, Confidence: math.Max(a.Probability, 1-a.Probability)}
		}
	}
	return resp
}

// newConformanceMock returns a validating MockDecider scripted for s, with
// images declared as given.
func newConformanceMock(s decisiontest.Script, images bool) *decision.MockDecider {
	m := decision.NewMockDecider("mock")
	m.SetValidate(true)
	m.SetCapabilities(decision.Capabilities{
		Declared:        true,
		Kinds:           []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate},
		SingleRoundTrip: true,
		Images:          images,
	})
	if s.Fail != "" {
		m.AddEntry(decision.MockEntry{Err: errors.New(s.Fail)})
	} else {
		m.AddEntry(decision.MockEntry{Response: mockConformanceResponse(s)})
	}
	return m
}

func TestMockDeciderPassesConformance(t *testing.T) {
	for _, images := range []bool{false, true} {
		t.Run(fmt.Sprintf("images=%v", images), func(t *testing.T) {
			decisiontest.RunConformance(t, decisiontest.Harness{
				New: func(_ *testing.T, s decisiontest.Script) decisiontest.Instance {
					m := newConformanceMock(s, images)
					return decisiontest.Instance{Decider: m, ProviderCalls: func() int { return len(m.Calls()) }}
				},
				Refusals: true,
			})
		})
	}
}

// TestFallbackPassesConformanceWithRefusals runs the suite over a Fallback
// whose Deciders refuse the same question: the refusal is a disclosed miss
// (the second Decider is tried), and with no Decider answering it the
// first declined Response is returned with the question still in Declined,
// never turned into an answer.
func TestFallbackPassesConformanceWithRefusals(t *testing.T) {
	decisiontest.RunConformance(t, decisiontest.Harness{
		New: func(_ *testing.T, s decisiontest.Script) decisiontest.Instance {
			first, second := newConformanceMock(s, false), newConformanceMock(s, false)
			f, err := decision.NewFallback(decision.FallbackOptions{
				Name:     "mock",
				Deciders: []decision.Decider{first, second},
				Observe:  func(decision.FallbackReport) {},
			})
			if err != nil {
				t.Fatal(err)
			}
			return decisiontest.Instance{Decider: f, ProviderCalls: func() int { return len(first.Calls()) }}
		},
		Refusals: true,
	})
}
