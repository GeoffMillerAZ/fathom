package llm_test

import (
	"encoding/json"
	"errors"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/decisiontest"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/llm"
	llmclient "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// llmWireAnswer renders one scripted answer into the JSON object the LLM
// contract carries: the contract's own predicate label "noul" and the
// provider's self-reported confidence, which the adapter must ignore.
func llmWireAnswer(a decisiontest.ScriptedAnswer) map[string]any {
	out := map[string]any{"confidence": a.SelfConfidence}
	switch a.Kind {
	case decision.KindChoice:
		out["choice"] = a.Choice
		out["probabilities"] = a.Probabilities
	case decision.KindScore:
		out["score"] = a.Score
	default:
		out["noul"] = a.Probability
	}
	return out
}

// llmConformanceReply renders s as the reply text the LLM contract asks
// for: {"answers": {<id>: <answer>}}.
func llmConformanceReply(t *testing.T, s decisiontest.Script) string {
	t.Helper()
	answers := map[string]any{}
	for _, a := range s.Answers {
		answers[a.ID] = llmWireAnswer(a)
	}
	b, err := json.Marshal(map[string]any{"answers": answers})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLLMPassesConformance(t *testing.T) {
	decisiontest.RunConformance(t, decisiontest.Harness{
		New: func(t *testing.T, s decisiontest.Script) decisiontest.Instance {
			entry := llmclient.MockEntryOK(llmConformanceReply(t, s))
			if s.Fail != "" {
				entry = llmclient.MockEntryErr(errors.New(s.Fail))
			}
			client := llmclient.NewMockClient(entry)
			return decisiontest.Instance{
				Decider:       llm.NewLLM(client, "decisiontest-model"),
				ProviderCalls: client.CallCount,
			}
		},
	})
}
