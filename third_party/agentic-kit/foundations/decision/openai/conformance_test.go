package openai_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/decisiontest"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/openai"
)

// conformanceKey is the fake credential the OpenAI harness builds with. It
// is deliberately not shaped like any real vendor key.
const conformanceKey = "decisiontest-fake-credential-not-a-key"

// openaiWireAnswer renders one scripted answer as the Decisions API would:
// a typed answer naming its question, the provider's own confidence (which
// the adapter must ignore) and, for a choice or a score, the distribution
// as an array; a refusal carries only its type and name.
func openaiWireAnswer(a decisiontest.ScriptedAnswer) map[string]any {
	if a.Refused {
		return map[string]any{"type": "refusal", "name": a.ID}
	}
	out := map[string]any{"name": a.ID, "confidence": a.SelfConfidence}
	switch a.Kind {
	case decision.KindChoice:
		out["type"] = "choice"
		out["choice"] = a.Choice
		var dist []map[string]any
		for _, v := range slices.Sorted(maps.Keys(a.Probabilities)) {
			dist = append(dist, map[string]any{"value": v, "probability": a.Probabilities[v]})
		}
		out["probabilities"] = dist
	case decision.KindScore:
		out["type"] = "score"
		out["score"] = a.Score
		var dist []map[string]any
		for i, p := range a.LevelProbabilities {
			dist = append(dist, map[string]any{"value": i, "label": "level " + strconv.Itoa(i), "probability": p})
		}
		out["probabilities"] = dist
	default:
		out["type"] = "predicate"
		out["probability"] = a.Probability
		delete(out, "confidence") // the API reports no confidence for a predicate
	}
	return out
}

func TestOpenAIPassesConformance(t *testing.T) {
	decisiontest.RunConformance(t, decisiontest.Harness{
		New: func(t *testing.T, s decisiontest.Script) decisiontest.Instance {
			var calls atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				if s.Fail != "" {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(s.Fail))
					return
				}
				answers := []map[string]any{}
				for _, a := range s.Answers {
					answers = append(answers, openaiWireAnswer(a))
				}
				body, err := json.Marshal(map[string]any{"model": "decisiontest-model-1", "answers": answers})
				if err != nil {
					w.WriteHeader(http.StatusTeapot)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
			}))
			t.Cleanup(srv.Close)
			o, err := openai.NewOpenAI(openai.OpenAIConfig{APIKey: conformanceKey, BaseURL: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			return decisiontest.Instance{Decider: o, ProviderCalls: func() int { return int(calls.Load()) }}
		},
		APIKey:                    conformanceKey,
		RedactsEchoedKey:          true,
		ScoreConfidenceFromLevels: true,
		Refusals:                  true,
	})
}
