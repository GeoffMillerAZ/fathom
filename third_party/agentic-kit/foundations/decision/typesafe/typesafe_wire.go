package typesafe

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// typesafeWireRequest is the top-level POST body for TypeSafe's System One
// API (POST {BaseURL}/v1/systemone). Questions is a genuine Go map keyed by
// Question.ID — encoding/json always emits map[string]T keys in sorted
// order, so the marshaled body is byte-deterministic regardless of map
// iteration order, matching the same convention llm_wire.go documents for
// its own (slice-based) rendering.
type typesafeWireRequest struct {
	Model     string                          `json:"model"`
	State     string                          `json:"state"`
	Questions map[string]typesafeWireQuestion `json:"questions"`
}

// typesafeWireQuestion is the wire rendering of one Question for TypeSafe's
// System One API. Criteria's shape depends on Type: an object with "true"/
// "false" keys for "noul" (present only when either criterion is
// non-empty), an object mapping option name to description-or-null for
// "choice" (always present), or an ordered array of level descriptions for
// "score" (always present — see typesafeRenderQuestion/typesafeScoreLevels).
type typesafeWireQuestion struct {
	Type         string          `json:"type"`
	Instructions *string         `json:"instructions,omitempty"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
}

// typesafeInstructions is q's instructions for the wire: omitted when
// empty, as the golden request pins (the OpenRouter transport sends an
// empty string instead; see Decide).
func typesafeInstructions(q decision.Question) *string {
	if q.Instructions == "" {
		return nil
	}
	v := q.Instructions
	return &v
}

// typesafeTypeNoul is TypeSafe's native primitive for KindPredicate. The
// vendor term stays here, in the adapter's wire code; the core names the
// kind KindPredicate. Pinned by testdata/typesafe_request.golden.
const typesafeTypeNoul = "noul"

// typesafeNoulCriteria is the wire shape of a "noul" question's criteria.
type typesafeNoulCriteria struct {
	True  string `json:"true"`
	False string `json:"false"`
}

// typesafeScorePlan records how to interpret a "score" question's wire
// answer: the ordered Levels actually sent (explicit or synthesized) and
// the offset to add to the wire's 0-indexed score to produce Answer.Score.
// offset is always q.Min: Validate forces Min to 0 whenever Question.Levels
// is set, so the explicit-Levels case (offset 0, answer IS the wire score)
// and the synthesized-from-Min/Max case (offset q.Min) share one formula.
type typesafeScorePlan struct {
	levels []string
	offset float64
}

// typesafeRenderQuestion renders q into its TypeSafe wire shape. ok is
// false for a question outside the limits TypeSafe declares in its
// Capabilities (kinds, options, and score levels, counted from explicit
// Levels and an integral [Min, Max] range alike; Validate does not enforce
// the score-level limits, they are the vendor's), for a KindScore question
// that cannot be rendered (see typesafeScoreLevels), or for an unrecognised
// Kind (unreachable once req has passed Validate, which rejects unknown
// Kinds) — reason then names why, for the caller to record in
// Response.Invalid without sending q at all.
// It switches on q's canonical Kind, so a raw "noul" Kind is sent as the
// vendor's "noul" primitive like KindPredicate.
func typesafeRenderQuestion(q decision.Question) (wire typesafeWireQuestion, plan typesafeScorePlan, reason string, ok bool) {
	if why, outside := decision.OutsideDeclaredLimits("typesafe", typesafeCapabilities(), q); outside {
		return typesafeWireQuestion{}, typesafeScorePlan{}, why, false
	}
	switch decision.CanonicalKind(q.Kind) {
	case decision.KindChoice:
		return typesafeWireQuestion{
			Type:         "choice",
			Instructions: typesafeInstructions(q),
			Criteria:     typesafeChoiceCriteria(q),
		}, typesafeScorePlan{}, "", true
	case decision.KindPredicate:
		return typesafeWireQuestion{
			Type:         typesafeTypeNoul,
			Instructions: typesafeInstructions(q),
			Criteria:     typesafeNoulCriteriaJSON(q),
		}, typesafeScorePlan{}, "", true
	case decision.KindScore:
		levels, offset, levelsOK := typesafeScoreLevels(q)
		if !levelsOK {
			return typesafeWireQuestion{}, typesafeScorePlan{}, "score needs Levels or an integral [Min, Max] range for the typesafe provider", false
		}
		b, _ := json.Marshal(levels)
		return typesafeWireQuestion{
			Type:         "score",
			Instructions: typesafeInstructions(q),
			Criteria:     b,
		}, typesafeScorePlan{levels: levels, offset: offset}, "", true
	default:
		return typesafeWireQuestion{}, typesafeScorePlan{}, fmt.Sprintf("question has unknown kind %q", q.Kind), false
	}
}

// typesafeNoulCriteriaJSON renders q's TrueCriteria/FalseCriteria as the
// wire "criteria" object, or nil (omitted from the wire question entirely,
// via Criteria's omitempty) when both are empty.
func typesafeNoulCriteriaJSON(q decision.Question) json.RawMessage {
	if q.TrueCriteria == "" && q.FalseCriteria == "" {
		return nil
	}
	b, _ := json.Marshal(typesafeNoulCriteria{True: q.TrueCriteria, False: q.FalseCriteria})
	return b
}

// typesafeChoiceCriteria renders q's Options as the wire "criteria" object
// mapping each option name to its Criteria, or JSON null when an option's
// Criteria is empty. Always present for a choice question (unlike noul's
// optional criteria) — TypeSafe's "choice" primitive requires it.
func typesafeChoiceCriteria(q decision.Question) json.RawMessage {
	m := make(map[string]*string, len(q.Options))
	for _, o := range q.Options {
		if o.Criteria == "" {
			m[o.Name] = nil
			continue
		}
		v := o.Criteria
		m[o.Name] = &v
	}
	b, _ := json.Marshal(m)
	return b
}

// typesafeScoreLevels resolves q's level descriptions for the TypeSafe wire
// "score" criteria, and the offset to add to the wire's 0-indexed answer
// score to produce Answer.Score:
//
//   - q.Levels set (Min==0 by Validate): used verbatim, offset 0.
//   - q.Levels empty: synthesized when q.Min and q.Max are both integral,
//     as "<Rubric> — level <Min+i>" per entry (or just the level number
//     when Rubric is empty), offset q.Min.
//   - anything else: ok is false — the question cannot be sent to
//     TypeSafe's "score" primitive at all.
//
// It does not bound the level count: TypeSafe's 2..10 lives in
// typesafeCapabilities, and typesafeRenderQuestion refuses a score outside
// it before calling this (see outsideDeclaredLimits).
func typesafeScoreLevels(q decision.Question) (levels []string, offset float64, ok bool) {
	if len(q.Levels) > 0 {
		return q.Levels, q.Min, true
	}
	if q.Min != math.Trunc(q.Min) || q.Max != math.Trunc(q.Max) {
		return nil, 0, false
	}
	n := int(q.Max-q.Min) + 1
	if n < 2 {
		return nil, 0, false
	}
	levels = make([]string, n)
	for i := 0; i < n; i++ {
		level := int(q.Min) + i
		if q.Rubric == "" {
			levels[i] = strconv.Itoa(level)
		} else {
			levels[i] = fmt.Sprintf("%s — level %d", q.Rubric, level)
		}
	}
	return levels, q.Min, true
}

// typesafeWireResponse is the top-level shape of a TypeSafe System One
// reply: {"model": <version that ran>, "answers": {<id>: <raw>}, "usage":
// {...}}.
type typesafeWireResponse struct {
	Model   string                     `json:"model"`
	Answers map[string]json.RawMessage `json:"answers"`
	Usage   typesafeWireUsage          `json:"usage"`
}

// typesafeWireUsage is the wire shape of a TypeSafe response's token
// accounting.
type typesafeWireUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// typesafeParseAnswer decodes and validates raw (one entry of the wire
// response's "answers" map) against q, returning the Answer and ok=true on
// success, or a human-readable reason and ok=false otherwise. plan is
// meaningful only for q.Kind == KindScore (see typesafeRenderQuestion).
// It switches on q's canonical Kind, so a raw "noul" Kind is answered as a
// KindPredicate.
func typesafeParseAnswer(q decision.Question, raw json.RawMessage, plan typesafeScorePlan) (decision.Answer, string, bool) {
	switch decision.CanonicalKind(q.Kind) {
	case decision.KindChoice:
		return typesafeParseChoiceAnswer(q, raw)
	case decision.KindScore:
		return typesafeParseScoreAnswer(q, raw, plan)
	case decision.KindPredicate:
		return typesafeParseNoulAnswer(q, raw)
	default:
		return decision.Answer{}, fmt.Sprintf("question has unknown kind %q", q.Kind), false
	}
}

func typesafeParseChoiceAnswer(q decision.Question, raw json.RawMessage) (decision.Answer, string, bool) {
	var w struct {
		Type          string             `json:"type"`
		Choice        string             `json:"choice"`
		Probabilities map[string]float64 `json:"probabilities"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return decision.Answer{}, fmt.Sprintf("malformed choice answer: %v", err), false
	}
	if w.Type != "" && w.Type != "choice" {
		return decision.Answer{}, fmt.Sprintf("answer type %q does not match question kind %q", w.Type, q.Kind), false
	}

	found := false
	for _, o := range q.Options {
		if o.Name == w.Choice {
			found = true
			break
		}
	}
	if !found {
		return decision.Answer{}, fmt.Sprintf("choice %q is not one of the question's options", w.Choice), false
	}

	probs := make(map[string]float64, len(q.Options))
	sum := 0.0
	for _, o := range q.Options {
		v := w.Probabilities[o.Name]
		probs[o.Name] = v
		sum += v
	}
	if math.Abs(sum-1) > 0.01 {
		return decision.Answer{}, fmt.Sprintf("probabilities sum to %v, want within 0.01 of 1.0", sum), false
	}

	return decision.Answer{
		ID:            q.ID,
		Kind:          decision.KindChoice,
		Choice:        w.Choice,
		Probabilities: probs,
		Confidence:    decision.Confidence(probs),
	}, "", true
}

func typesafeParseScoreAnswer(q decision.Question, raw json.RawMessage, plan typesafeScorePlan) (decision.Answer, string, bool) {
	var w struct {
		Type          string             `json:"type"`
		Score         *float64           `json:"score"`
		Probabilities map[string]float64 `json:"probabilities"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return decision.Answer{}, fmt.Sprintf("malformed score answer: %v", err), false
	}
	if w.Type != "" && w.Type != "score" {
		return decision.Answer{}, fmt.Sprintf("answer type %q does not match question kind %q", w.Type, q.Kind), false
	}
	if w.Score == nil {
		return decision.Answer{}, "score answer is missing \"score\"", false
	}

	n := len(plan.levels)
	if *w.Score < 0 || *w.Score > float64(n-1) {
		return decision.Answer{}, fmt.Sprintf("score %v is outside [0, %d]", *w.Score, n-1), false
	}

	probs := make(map[string]float64, n)
	for i := 0; i < n; i++ {
		probs[strconv.Itoa(i)] = w.Probabilities[strconv.Itoa(i)]
	}

	return decision.Answer{
		ID:            q.ID,
		Kind:          decision.KindScore,
		Score:         plan.offset + *w.Score,
		Probabilities: probs,
		Confidence:    decision.Confidence(probs),
	}, "", true
}

func typesafeParseNoulAnswer(q decision.Question, raw json.RawMessage) (decision.Answer, string, bool) {
	var w struct {
		Type string   `json:"type"`
		Noul *float64 `json:"noul"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return decision.Answer{}, fmt.Sprintf("malformed noul answer: %v", err), false
	}
	if w.Type != "" && w.Type != typesafeTypeNoul {
		// The reason names the vendor primitive, as the wire does.
		return decision.Answer{}, fmt.Sprintf("answer type %q does not match question kind %q", w.Type, typesafeTypeNoul), false
	}
	if w.Noul == nil {
		return decision.Answer{}, "noul answer is missing \"noul\"", false
	}

	n := *w.Noul
	if n < 0 || n > 1 {
		return decision.Answer{}, fmt.Sprintf("noul %v is outside [0, 1]", n), false
	}
	conf := n
	if n < 0.5 {
		conf = 1 - n
	}

	return decision.Answer{
		ID:         q.ID,
		Kind:       decision.KindPredicate,
		Noul:       n,
		Confidence: conf,
	}, "", true
}
