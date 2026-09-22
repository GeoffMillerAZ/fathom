package decision

import (
	"encoding/json"
	"fmt"
)

// llmSystemPrompt is the fixed contract sent as the LLM request's System
// field on every Decide call. It is pinned byte-for-byte by
// testdata/prompt.golden's companion schema.golden — see llm_golden_test.go.
const llmSystemPrompt = `You are a fast-decision engine. Answer ONLY with JSON matching the provided schema — no prose, no markdown fences, no extra keys.

The user message is a JSON object: {"state": <the situation>, "questions": [<question>, ...]}. Answer every question about "state". Respond with a JSON object {"answers": {<question id>: <answer>, ...}} containing exactly one entry per question, keyed by that question's "id":

  - kind "choice": the answer is {"choice": <one of "options"[].name, exactly>, "probabilities": {<each option name>: <your probability it is correct>}}. Probabilities must be given for EVERY option and should sum to 1.0.
  - kind "score": the answer is {"score": <a number within ["min", "max"]>}.
  - kind "noul": the answer is {"noul": <your probability, in [0, 1], that the statement in "instructions" is true>}.

Never include a question id that was not asked. Never omit a question id that was asked. Never include a confidence field — confidence is computed by the caller, not reported by you.`

// wireOption is the prompt-JSON rendering of an Option.
type wireOption struct {
	Name     string `json:"name"`
	Criteria string `json:"criteria,omitempty"`
}

// wireQuestion is the prompt-JSON rendering of one Question. Only the
// fields meaningful for Kind are populated (see questionToWire) — omitempty
// keeps the rendered JSON free of zero-valued fields that do not apply to
// this question's Kind.
type wireQuestion struct {
	ID            string       `json:"id"`
	Kind          Kind         `json:"kind"`
	Instructions  string       `json:"instructions,omitempty"`
	Options       []wireOption `json:"options,omitempty"`
	Min           *float64     `json:"min,omitempty"`
	Max           *float64     `json:"max,omitempty"`
	Rubric        string       `json:"rubric,omitempty"`
	TrueCriteria  string       `json:"true_criteria,omitempty"`
	FalseCriteria string       `json:"false_criteria,omitempty"`
}

// wirePrompt is the top-level prompt-JSON payload.
type wirePrompt struct {
	State     string         `json:"state"`
	Questions []wireQuestion `json:"questions"`
}

// questionToWire renders q into its wireQuestion, populating only the
// fields meaningful for q.Kind.
func questionToWire(q Question) wireQuestion {
	w := wireQuestion{ID: q.ID, Kind: q.Kind, Instructions: q.Instructions}
	switch q.Kind {
	case KindChoice:
		w.Options = make([]wireOption, len(q.Options))
		for i, o := range q.Options {
			w.Options[i] = wireOption{Name: o.Name, Criteria: o.Criteria}
		}
	case KindScore:
		minV, maxV := q.Min, q.Max
		w.Min, w.Max = &minV, &maxV
		w.Rubric = q.Rubric
	case KindNoul:
		w.TrueCriteria = q.TrueCriteria
		w.FalseCriteria = q.FalseCriteria
	}
	return w
}

// renderPrompt renders req as deterministic JSON — Question order preserved
// from req.Questions, no map used for the question list — for use as the
// llmclient.Request.Prompt. Per-question fields are rendered via slices and
// pointers only (never a map), so the ordering guarantee holds regardless
// of how encoding/json is asked to marshal it.
func renderPrompt(req Request) ([]byte, error) {
	w := wirePrompt{State: req.State, Questions: make([]wireQuestion, len(req.Questions))}
	for i, q := range req.Questions {
		w.Questions[i] = questionToWire(q)
	}
	b, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return nil, err
	}
	return b, nil
}

// buildOutputSchema builds the JSON Schema for {"answers": {<id>: <answer
// schema per kind>}}, constraining the LLMClient's structured-output
// surface (llmclient.Request.OutputSchema). required lists question IDs in
// req.Questions order (deterministic from the caller's input order); the
// "properties" objects are built from Go maps keyed by question ID or
// option name, but encoding/json always emits map[string]any keys in
// sorted order — so the marshaled schema is byte-deterministic regardless
// of map iteration order. See llm_golden_test.go, which pins this output.
func buildOutputSchema(questions []Question) json.RawMessage {
	answerProps := make(map[string]any, len(questions))
	required := make([]string, 0, len(questions))
	for _, q := range questions {
		answerProps[q.ID] = answerSchema(q)
		required = append(required, q.ID)
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"answers": map[string]any{
				"type":                 "object",
				"properties":           answerProps,
				"required":             required,
				"additionalProperties": false,
			},
		},
		"required":             []string{"answers"},
		"additionalProperties": false,
	}

	// Marshaling a value built entirely from strings/bools/numbers/maps/
	// slices cannot fail.
	b, _ := json.MarshalIndent(schema, "", "  ")
	return b
}

// answerSchema builds the per-question answer schema for buildOutputSchema,
// keyed by q.Kind.
func answerSchema(q Question) map[string]any {
	switch q.Kind {
	case KindScore:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"score": map[string]any{"type": "number", "minimum": q.Min, "maximum": q.Max},
			},
			"required":             []string{"score"},
			"additionalProperties": false,
		}
	case KindNoul:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"noul": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			},
			"required":             []string{"noul"},
			"additionalProperties": false,
		}
	default: // KindChoice
		names := make([]string, len(q.Options))
		probProps := make(map[string]any, len(q.Options))
		for i, o := range q.Options {
			names[i] = o.Name
			probProps[o.Name] = map[string]any{"type": "number"}
		}
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"choice": map[string]any{"type": "string", "enum": names},
				"probabilities": map[string]any{
					"type":                 "object",
					"properties":           probProps,
					"required":             names,
					"additionalProperties": false,
				},
			},
			"required":             []string{"choice", "probabilities"},
			"additionalProperties": false,
		}
	}
}

// wireEnvelope is the top-level shape of an LLM reply: {"answers": {<id>:
// <raw>}}.
type wireEnvelope struct {
	Answers map[string]json.RawMessage `json:"answers"`
}

// parseResponse decodes text (the LLMClient Response.Text) against req and
// classifies each question's answer into either the returned answers map
// or the returned invalid map (question ID -> reason). It returns a non-nil
// error ONLY when text is not JSON at all — every other failure mode
// (missing "answers", a missing per-question entry, a malformed per-kind
// shape, a choice outside the options, an out-of-range value) is recorded
// per-question in invalid instead of failing the whole batch.
func parseResponse(req Request, text string) (answers map[string]Answer, invalid map[string]string, err error) {
	var env wireEnvelope
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		return nil, nil, fmt.Errorf("llm response is not JSON: %w", err)
	}

	answers = make(map[string]Answer)
	invalid = make(map[string]string)
	for _, q := range req.Questions {
		raw, ok := env.Answers[q.ID]
		if !ok {
			invalid[q.ID] = "missing answer for question"
			continue
		}
		a, reason, ok := parseAnswer(q, raw)
		if !ok {
			invalid[q.ID] = reason
			continue
		}
		answers[q.ID] = a
	}
	return answers, invalid, nil
}

// parseAnswer decodes and validates raw against q, returning the Answer and
// ok=true on success, or a human-readable reason and ok=false otherwise.
func parseAnswer(q Question, raw json.RawMessage) (Answer, string, bool) {
	switch q.Kind {
	case KindChoice:
		return parseChoiceAnswer(q, raw)
	case KindScore:
		return parseScoreAnswer(q, raw)
	case KindNoul:
		return parseNoulAnswer(q, raw)
	default:
		return Answer{}, fmt.Sprintf("question has unknown kind %q", q.Kind), false
	}
}

func parseChoiceAnswer(q Question, raw json.RawMessage) (Answer, string, bool) {
	var w struct {
		Choice        string             `json:"choice"`
		Probabilities map[string]float64 `json:"probabilities"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return Answer{}, fmt.Sprintf("malformed choice answer: %v", err), false
	}

	found := false
	for _, o := range q.Options {
		if o.Name == w.Choice {
			found = true
			break
		}
	}
	if !found {
		return Answer{}, fmt.Sprintf("choice %q is not one of the question's options", w.Choice), false
	}

	probs := make(map[string]float64, len(q.Options))
	sum := 0.0
	for _, o := range q.Options {
		v := w.Probabilities[o.Name]
		if v < 0 {
			v = 0
		}
		probs[o.Name] = v
		sum += v
	}
	if sum > 0 {
		for name, v := range probs {
			probs[name] = v / sum
		}
	} else {
		uniform := 1.0 / float64(len(q.Options))
		for name := range probs {
			probs[name] = uniform
		}
	}

	return Answer{
		ID:            q.ID,
		Kind:          KindChoice,
		Choice:        w.Choice,
		Probabilities: probs,
		Confidence:    Confidence(probs),
	}, "", true
}

func parseScoreAnswer(q Question, raw json.RawMessage) (Answer, string, bool) {
	var w struct {
		Score *float64 `json:"score"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return Answer{}, fmt.Sprintf("malformed score answer: %v", err), false
	}
	if w.Score == nil {
		return Answer{}, "score answer is missing \"score\"", false
	}

	s := *w.Score
	if s < q.Min {
		s = q.Min
	}
	if s > q.Max {
		s = q.Max
	}

	// A Score has no discrete alternative set to form a distribution over;
	// Confidence's own k<=1 rule collapses this to 1.0 — see the package
	// doc.
	return Answer{
		ID:         q.ID,
		Kind:       KindScore,
		Score:      s,
		Confidence: Confidence(nil),
	}, "", true
}

func parseNoulAnswer(q Question, raw json.RawMessage) (Answer, string, bool) {
	var w struct {
		Noul *float64 `json:"noul"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return Answer{}, fmt.Sprintf("malformed noul answer: %v", err), false
	}
	if w.Noul == nil {
		return Answer{}, "noul answer is missing \"noul\"", false
	}

	n := clamp01(*w.Noul)
	conf := n
	if n < 0.5 {
		conf = 1 - n
	}

	return Answer{
		ID:         q.ID,
		Kind:       KindNoul,
		Noul:       n,
		Confidence: conf,
	}, "", true
}
