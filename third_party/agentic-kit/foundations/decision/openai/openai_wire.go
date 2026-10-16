package openai

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// The wire's question and answer types. The kinds map one to one; the API
// adds "refusal" as an answer type.
const (
	openaiTypePredicate = "predicate"
	openaiTypeChoice    = "choice"
	openaiTypeScore     = "score"
	openaiTypeRefusal   = "refusal"
)

// openaiRefusalReason is the Response.Declined reason for an answer of type
// "refusal": the guide documents the type and the echoed name, no text.
const openaiRefusalReason = "the provider refused to answer"

// openaiWireRequest is the POST body for {BaseURL}/decisions. Input is a
// string (State) or a one-element []openaiWireMessage when the Request
// carries images. Questions keep the Request's order.
type openaiWireRequest struct {
	Model     string               `json:"model"`
	Input     any                  `json:"input"`
	Questions []openaiWireQuestion `json:"questions"`
}

// openaiWireMessage is one user message of the input form that carries
// images.
type openaiWireMessage struct {
	Role    string           `json:"role"`
	Content []openaiWirePart `json:"content"`
}

// openaiWirePart is one content part: input_text carries Text, input_image
// carries ImageURL (a base64 data URL).
type openaiWirePart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// openaiWireQuestion is the wire rendering of one Question. Choices is set
// for a choice, Levels for a score; Instructions is always sent.
type openaiWireQuestion struct {
	Type         string             `json:"type"`
	Name         string             `json:"name"`
	Instructions string             `json:"instructions"`
	Choices      []openaiWireChoice `json:"choices,omitempty"`
	Levels       []openaiWireLevel  `json:"levels,omitempty"`
}

// openaiWireChoice is one choice: the Option's Name and, when set, its
// Criteria.
type openaiWireChoice struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// openaiWireLevel is one score level, lowest first.
type openaiWireLevel struct {
	Label string `json:"label"`
}

// openaiScorePlan records how to read a score answer: how many levels were
// sent and the offset to add to the wire's 0-based score (Min for a
// synthesized range, 0 for explicit Levels, where Validate forces Min to 0).
type openaiScorePlan struct {
	levels int
	offset float64
}

// openaiRenderRequest renders req for the wire. A question outside the
// declared limits or a score that cannot be rendered is recorded in the
// returned invalid map by ID and left off the wire.
func openaiRenderRequest(model string, req decision.Request) (openaiWireRequest, map[string]openaiScorePlan, map[string]string) {
	wire := openaiWireRequest{Model: model, Input: req.State, Questions: make([]openaiWireQuestion, 0, len(req.Questions))}
	plans := make(map[string]openaiScorePlan)
	invalid := make(map[string]string)
	for _, q := range req.Questions {
		wq, plan, reason, ok := openaiRenderQuestion(q)
		if !ok {
			invalid[q.ID] = reason
			continue
		}
		wire.Questions = append(wire.Questions, wq)
		if decision.CanonicalKind(q.Kind) == decision.KindScore {
			plans[q.ID] = plan
		}
	}
	if len(req.Images) > 0 {
		parts := make([]openaiWirePart, 0, len(req.Images)+1)
		parts = append(parts, openaiWirePart{Type: "input_text", Text: req.State})
		for _, im := range req.Images {
			parts = append(parts, openaiWirePart{Type: "input_image", ImageURL: "data:" + im.MediaType + ";base64," + base64.StdEncoding.EncodeToString(im.Data)})
		}
		wire.Input = []openaiWireMessage{{Role: "user", Content: parts}}
	}
	return wire, plans, invalid
}

// openaiRenderQuestion renders q. ok is false, with the reason, for a
// question outside the declared limits, a score with neither Levels nor an
// integral [Min, Max] range of at least two values, or an unknown kind
// (unreachable after Validate). It switches on q's canonical Kind, so a raw
// "noul" Kind is sent as a predicate.
func openaiRenderQuestion(q decision.Question) (openaiWireQuestion, openaiScorePlan, string, bool) {
	if why, outside := decision.OutsideDeclaredLimits("openai", openaiCapabilities(), q); outside {
		return openaiWireQuestion{}, openaiScorePlan{}, why, false
	}
	switch decision.CanonicalKind(q.Kind) {
	case decision.KindPredicate:
		return openaiWireQuestion{
			Type:         openaiTypePredicate,
			Name:         q.ID,
			Instructions: openaiInstructions(q.Instructions, "True when: ", q.TrueCriteria, "False when: ", q.FalseCriteria),
		}, openaiScorePlan{}, "", true
	case decision.KindChoice:
		choices := make([]openaiWireChoice, len(q.Options))
		for i, o := range q.Options {
			choices[i] = openaiWireChoice{Value: o.Name, Description: o.Criteria}
		}
		return openaiWireQuestion{
			Type:         openaiTypeChoice,
			Name:         q.ID,
			Instructions: openaiInstructions(q.Instructions),
			Choices:      choices,
		}, openaiScorePlan{}, "", true
	case decision.KindScore:
		labels, offset, ok := openaiScoreLevels(q)
		if !ok {
			return openaiWireQuestion{}, openaiScorePlan{}, "score needs Levels or an integral [Min, Max] range for the openai provider", false
		}
		levels := make([]openaiWireLevel, len(labels))
		for i, l := range labels {
			levels[i] = openaiWireLevel{Label: l}
		}
		return openaiWireQuestion{
			Type:         openaiTypeScore,
			Name:         q.ID,
			Instructions: openaiInstructions(q.Instructions, "Rubric: ", q.Rubric),
			Levels:       levels,
		}, openaiScorePlan{levels: len(labels), offset: offset}, "", true
	default:
		return openaiWireQuestion{}, openaiScorePlan{}, fmt.Sprintf("question has unknown kind %q", q.Kind), false
	}
}

// openaiInstructions joins base and each non-empty labelled extra (pairs of
// label, text) into the wire's instructions, one per line.
func openaiInstructions(base string, labelled ...string) string {
	lines := make([]string, 0, 1+len(labelled)/2)
	if base != "" {
		lines = append(lines, base)
	}
	for i := 0; i+1 < len(labelled); i += 2 {
		if labelled[i+1] != "" {
			lines = append(lines, labelled[i]+labelled[i+1])
		}
	}
	return strings.Join(lines, "\n")
}

// openaiScoreLevels returns q's level labels, lowest first, and the offset
// to add to the wire's 0-based score: Levels verbatim with offset 0, or the
// integers Min..Max with offset Min when Levels is empty and the range is
// integral with at least two values. ok is false otherwise.
func openaiScoreLevels(q decision.Question) (labels []string, offset float64, ok bool) {
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
	labels = make([]string, n)
	for i := range labels {
		labels[i] = strconv.Itoa(int(q.Min) + i)
	}
	return labels, q.Min, true
}

// openaiWireResponse is the reply: answers in any order, each naming its
// question. Usage is not documented by the guide; it is read when present.
type openaiWireResponse struct {
	Model   string            `json:"model"`
	Answers []json.RawMessage `json:"answers"`
	Usage   *openaiWireUsage  `json:"usage"`
}

// openaiWireUsage is the reply's token accounting, when present.
type openaiWireUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// openaiWireHead is the part of every answer that says which question it
// answers and how.
type openaiWireHead struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// openaiMapAnswers maps env's answers onto req's questions: valid answers
// are returned, refusals go to declined, and every other outcome is added
// to invalid (which already holds the questions never sent). An answer
// naming no asked question is ignored.
func openaiMapAnswers(req decision.Request, env openaiWireResponse, plans map[string]openaiScorePlan, invalid map[string]string) (map[string]decision.Answer, map[string]string) {
	byName := make(map[string]json.RawMessage, len(env.Answers))
	heads := make(map[string]openaiWireHead, len(env.Answers))
	repeated := make(map[string]bool)
	for _, raw := range env.Answers {
		var h openaiWireHead
		if err := json.Unmarshal(raw, &h); err != nil || h.Name == "" {
			continue
		}
		if _, seen := byName[h.Name]; seen {
			repeated[h.Name] = true
		}
		byName[h.Name], heads[h.Name] = raw, h
	}

	answers := make(map[string]decision.Answer, len(req.Questions))
	declined := make(map[string]string)
	for _, q := range req.Questions {
		if _, skipped := invalid[q.ID]; skipped {
			continue
		}
		raw, ok := byName[q.ID]
		switch {
		case !ok:
			invalid[q.ID] = "no answer"
			continue
		case repeated[q.ID]:
			invalid[q.ID] = "the reply carries more than one answer for this question"
			continue
		case heads[q.ID].Type == openaiTypeRefusal:
			declined[q.ID] = openaiRefusalReason
			continue
		}
		a, reason, ok := openaiParseAnswer(q, heads[q.ID].Type, raw, plans[q.ID])
		if !ok {
			invalid[q.ID] = reason
			continue
		}
		answers[q.ID] = a
	}
	return answers, declined
}

// openaiParseAnswer decodes and checks raw, an answer of wire type typ,
// against q.
func openaiParseAnswer(q decision.Question, typ string, raw json.RawMessage, plan openaiScorePlan) (decision.Answer, string, bool) {
	kind := decision.CanonicalKind(q.Kind)
	want := map[decision.Kind]string{decision.KindPredicate: openaiTypePredicate, decision.KindChoice: openaiTypeChoice, decision.KindScore: openaiTypeScore}[kind]
	if typ != want {
		return decision.Answer{}, fmt.Sprintf("answer type %q does not match question kind %q", typ, kind), false
	}
	switch kind {
	case decision.KindPredicate:
		return openaiParsePredicate(q, raw)
	case decision.KindChoice:
		return openaiParseChoice(q, raw)
	default: // decision.KindScore; the type check above excludes any other kind
		return openaiParseScore(q, raw, plan)
	}
}

// openaiProbability reports why p is not a probability, or "" when it is.
func openaiProbability(p *float64) string {
	switch {
	case p == nil:
		return "missing"
	case *p < 0 || *p > 1:
		return fmt.Sprintf("%v is outside [0, 1]", *p)
	}
	return ""
}

func openaiParsePredicate(q decision.Question, raw json.RawMessage) (decision.Answer, string, bool) {
	var w struct {
		Probability *float64 `json:"probability"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return decision.Answer{}, fmt.Sprintf("malformed predicate answer: %v", err), false
	}
	if why := openaiProbability(w.Probability); why != "" {
		return decision.Answer{}, "predicate probability " + why, false
	}
	p := *w.Probability
	return decision.Answer{ID: q.ID, Kind: decision.KindPredicate, Noul: p, Confidence: math.Max(p, 1-p)}, "", true
}

func openaiParseChoice(q decision.Question, raw json.RawMessage) (decision.Answer, string, bool) {
	var w struct {
		Choice        string `json:"choice"`
		Probabilities []struct {
			Value       string   `json:"value"`
			Probability *float64 `json:"probability"`
		} `json:"probabilities"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return decision.Answer{}, fmt.Sprintf("malformed choice answer: %v", err), false
	}
	probs := make(map[string]float64, len(q.Options))
	for _, o := range q.Options {
		probs[o.Name] = 0
	}
	if _, ok := probs[w.Choice]; !ok {
		return decision.Answer{}, fmt.Sprintf("choice %q is not one of the question's options", w.Choice), false
	}
	seen := make(map[string]bool, len(w.Probabilities))
	sum := 0.0
	for _, e := range w.Probabilities {
		if _, ok := probs[e.Value]; !ok {
			return decision.Answer{}, fmt.Sprintf("probability for %q, which is not one of the question's options", e.Value), false
		}
		if seen[e.Value] {
			return decision.Answer{}, fmt.Sprintf("more than one probability for option %q", e.Value), false
		}
		seen[e.Value] = true
		if why := openaiProbability(e.Probability); why != "" {
			return decision.Answer{}, fmt.Sprintf("probability for option %q %s", e.Value, why), false
		}
		probs[e.Value] = *e.Probability
		sum += *e.Probability
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

func openaiParseScore(q decision.Question, raw json.RawMessage, plan openaiScorePlan) (decision.Answer, string, bool) {
	var w struct {
		Score         *float64 `json:"score"`
		Probabilities []struct {
			Value       *float64 `json:"value"`
			Probability *float64 `json:"probability"`
		} `json:"probabilities"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return decision.Answer{}, fmt.Sprintf("malformed score answer: %v", err), false
	}
	n := plan.levels
	if w.Score == nil {
		return decision.Answer{}, `score answer is missing "score"`, false
	}
	if *w.Score < 0 || *w.Score > float64(n-1) {
		return decision.Answer{}, fmt.Sprintf("score %v is outside [0, %d]", *w.Score, n-1), false
	}
	if len(w.Probabilities) == 0 {
		return decision.Answer{}, "score answer has no level distribution to derive a confidence from", false
	}
	probs := make(map[string]float64, n)
	for i := 0; i < n; i++ {
		probs[strconv.Itoa(i)] = 0
	}
	seen := make(map[int]bool, len(w.Probabilities))
	sum := 0.0
	for _, e := range w.Probabilities {
		if e.Value == nil || *e.Value != math.Trunc(*e.Value) || *e.Value < 0 || *e.Value > float64(n-1) {
			return decision.Answer{}, fmt.Sprintf("a level probability names no level in [0, %d]", n-1), false
		}
		level := int(*e.Value)
		if seen[level] {
			return decision.Answer{}, fmt.Sprintf("more than one probability for level %d", level), false
		}
		seen[level] = true
		if why := openaiProbability(e.Probability); why != "" {
			return decision.Answer{}, fmt.Sprintf("probability for level %d %s", level, why), false
		}
		probs[strconv.Itoa(level)] = *e.Probability
		sum += *e.Probability
	}
	if math.Abs(sum-1) > 0.01 {
		return decision.Answer{}, fmt.Sprintf("level probabilities sum to %v, want within 0.01 of 1.0", sum), false
	}
	return decision.Answer{
		ID:            q.ID,
		Kind:          decision.KindScore,
		Score:         plan.offset + *w.Score,
		Probabilities: probs,
		Confidence:    decision.Confidence(probs),
	}, "", true
}
