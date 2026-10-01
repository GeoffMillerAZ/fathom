package typesafe_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/decisiontest"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/typesafe"
)

// conformanceKey is the fake credential the TypeSafe harness builds with. It
// is deliberately not shaped like any real vendor key.
const conformanceKey = "decisiontest-fake-credential-not-a-key"

// typesafeWireAnswer renders one scripted answer into the JSON object the
// TypeSafe wire carries: a "type" naming the vendor primitive, the
// provider's self-reported confidence (which the adapter must ignore) and,
// for a score, the level distribution.
func typesafeWireAnswer(a decisiontest.ScriptedAnswer) map[string]any {
	out := map[string]any{"confidence": a.SelfConfidence}
	switch a.Kind {
	case decision.KindChoice:
		out["type"] = "choice"
		out["choice"] = a.Choice
		out["probabilities"] = a.Probabilities
	case decision.KindScore:
		out["type"] = "score"
		out["score"] = a.Score
		dist := map[string]float64{}
		for i, p := range a.LevelProbabilities {
			dist[strconv.Itoa(i)] = p
		}
		out["probabilities"] = dist
	default:
		out["type"] = "noul"
		out["noul"] = a.Probability
	}
	return out
}

func TestTypeSafePassesConformance(t *testing.T) {
	t.Log("RedactsEchoedKey is false: TypeSafeError.Message is the provider's response body verbatim, so a body that echoes the key would carry it (kit observation O3)")
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
				answers := map[string]any{}
				for _, a := range s.Answers {
					answers[a.ID] = typesafeWireAnswer(a)
				}
				body, err := json.Marshal(map[string]any{
					"model":   "decisiontest-model-1",
					"answers": answers,
					"usage":   map[string]int{"input_tokens": 1, "output_tokens": 1},
				})
				if err != nil {
					w.WriteHeader(http.StatusTeapot)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
			}))
			t.Cleanup(srv.Close)
			ts, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{APIKey: conformanceKey, BaseURL: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			return decisiontest.Instance{Decider: ts, ProviderCalls: func() int { return int(calls.Load()) }}
		},
		APIKey:                    conformanceKey,
		RedactsEchoedKey:          false,
		ScoreConfidenceFromLevels: true,
	})
}
