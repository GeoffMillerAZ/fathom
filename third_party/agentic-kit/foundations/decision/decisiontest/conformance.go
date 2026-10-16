// Package decisiontest provides an exported conformance suite for
// implementations of decision.Decider. Every adapter (None, llm.LLM,
// typesafe.TypeSafe, MockDecider, and any Decider written outside the kit)
// runs the same checks by supplying a Harness that builds it over a scripted
// offline provider.
//
// The checks key off the Decider's own declaration (decision.CapabilitiesOf):
// which kinds it is asked, how many options and score levels a question
// carries, whether it must abstain, and how many provider calls a batch may
// take all come from what it declares. The suite encodes no vendor limit of
// its own.
//
// What it checks:
//
//   - Capabilities: the Decider declares, reading the declaration makes no
//     provider call and is stable, and the declaration is consistent (no
//     kinds exactly when it reports no probabilities, no negative or
//     inverted limits, no round trip claimed by a Decider that declares it
//     makes none).
//   - Validate handling: a Request decision.Validate refuses gets an error,
//     no answers and no provider call.
//   - Abstain: a Decider that declares no kinds (ConfidenceNone) answers a
//     valid Request with decision.ErrNoProvider and calls nothing; one that
//     declares kinds never does.
//   - Per-question classification: in one batch, a bad answer (a choice
//     outside the options, an omitted answer) lands in Response.Invalid by
//     question ID while every good answer lands in Response.Answers with the
//     provider's values, and the batch never fails for it.
//   - Confidence: every Answer.Confidence is this package's formula over
//     what the provider answered; a confidence the provider reports about
//     itself (SelfReportedConfidence) is ignored.
//   - Exclusive outcomes: no question is in more than one of
//     Response.Answers, Response.Invalid and Response.Declined.
//   - Refusal: when the harness declares its wire carries one (Refusals), a
//     refused question lands in Response.Declined with a reason, never in
//     Answers (a refusal is never read as false, zero or an option) and
//     never in Invalid, and the rest of the batch is answered.
//   - Images: a Decider that does not declare Capabilities.Images answers a
//     Request carrying an image with no provider call and no answer; one
//     that declares it answers the batch as it would without the image.
//   - Provider failure: a failing provider is an error with no answers, and
//     never decision.ErrNoProvider.
//   - No key in an error: when the harness names the fake credential the
//     Decider was built with, no error the suite sees contains it.
//
// Composition: imports only the standard library and the parent
// foundations/decision package. No network: a harness for an HTTP adapter
// scripts an httptest server itself, and one for an llmclient-backed adapter
// scripts an llmclient.MockClient. The suite makes no model call.
//
// Concurrency contract: RunConformance is safe to call from parallel tests;
// each check builds its own Decider through the harness and shares no state
// with the others.
//
// Usage:
//
//	decisiontest.RunConformance(t, decisiontest.Harness{
//		New: func(t *testing.T, s decisiontest.Script) decisiontest.Instance {
//			client := llmclient.NewMockClient(llmclient.MockEntryOK(render(s)))
//			return decisiontest.Instance{
//				Decider:       llm.NewLLM(client, "model"),
//				ProviderCalls: client.CallCount,
//			}
//		},
//	})
package decisiontest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// SelfReportedConfidence is the confidence every scripted answer claims
// about itself. A harness puts it on its provider's wire wherever the wire
// has room for one (a "confidence" field); the suite requires
// Answer.Confidence to ignore it. No answer the suite scripts has a formula
// confidence equal to it.
const SelfReportedConfidence = 0.123

// tolerance bounds the difference the suite accepts between a scripted
// value and the one an adapter reports (renormalizing probabilities that
// already sum to 1 may move the last bits).
const tolerance = 1e-9

// Script is what the scripted provider does when the Decider under test
// calls it.
type Script struct {
	// Request is the Request the suite will pass to Decide. A harness whose
	// provider is a script of whole Responses (MockDecider) builds them from
	// it; a harness that renders a wire reply needs only Answers.
	Request decision.Request
	// Answers are the provider's answers, at most one per question ID, in
	// question order. A question with no entry is one the provider leaves
	// out of its reply.
	Answers []ScriptedAnswer
	// Fail, when non-empty, makes the provider fail the call with this text
	// (an HTTP error body, a client error) instead of answering.
	Fail string
}

// ScriptedAnswer is one answer the scripted provider gives, in neutral
// terms a harness renders onto its provider's wire.
type ScriptedAnswer struct {
	// ID is the question answered.
	ID string
	// Kind is the question's kind, and so which fields below apply.
	Kind decision.Kind
	// Choice is the option chosen (KindChoice). The suite may script a name
	// that is not among the question's options; the adapter must then
	// classify the answer as invalid.
	Choice string
	// Probabilities maps every option name to its probability (KindChoice);
	// they sum to 1.
	Probabilities map[string]float64
	// Score is the level index answered (KindScore). The suite's score
	// questions carry Levels with Min and Max unset, so the index is the
	// Answer.Score the adapter must report.
	Score float64
	// LevelProbabilities is the provider's distribution over the levels,
	// index by index (KindScore), for a wire that carries one; a harness
	// whose wire has no place for it drops it.
	LevelProbabilities []float64
	// Probability is the probability the predicate's statement is true
	// (KindPredicate), reported as Answer.Noul.
	Probability float64
	// Refused, when true, makes the provider refuse this question on its
	// wire instead of answering it; the fields above are then ignored. The
	// suite scripts a refusal only for a harness that declares Refusals.
	Refused bool
	// SelfConfidence is the confidence the provider claims about this
	// answer: always SelfReportedConfidence. A harness sends it wherever its
	// wire has room.
	SelfConfidence float64
}

// Instance is one Decider under test, built by Harness.New over a scripted
// provider.
type Instance struct {
	// Decider is the adapter under test.
	Decider decision.Decider
	// ProviderCalls reports how many calls have reached the scripted
	// provider so far. Required; a Decider with no provider reports 0.
	ProviderCalls func() int
}

// Harness adapts one Decider implementation to the suite.
type Harness struct {
	// New returns a fresh Decider over a scripted offline provider that
	// behaves as s describes. The suite builds a new Instance for every
	// Decide call it makes. Required.
	New func(t *testing.T, s Script) Instance

	// APIKey is the fake credential the Decider was built with, which no
	// error may contain. It must not be shaped like any real vendor key.
	// Empty means the Decider holds no credential of its own (None, a
	// scripted double, or LLM, whose key belongs to the wrapped client and
	// is checked by that client's own conformance suite); the key checks are
	// then skipped and logged.
	APIKey string

	// RedactsEchoedKey declares that the Decider keeps APIKey out of its
	// errors even when the provider's own failure text echoes the key back.
	// When false the echo check is skipped and logged, so the gap stays
	// visible; a Decider that never echoes provider text passes it.
	RedactsEchoedKey bool

	// ScoreConfidenceFromLevels declares that the Decider's wire carries a
	// distribution over a score's levels, so a KindScore answer's Confidence
	// is decision.Confidence over that distribution. When false a score has
	// no alternative set and its Confidence is decision.Confidence(nil),
	// which is 1.
	ScoreConfidenceFromLevels bool

	// Refusals declares that the provider's wire can refuse a question (a
	// ScriptedAnswer with Refused set), so the suite checks that the
	// Decider reports the refusal in Response.Declined. When false the
	// refusal check is skipped and logged.
	Refusals bool
}

// RunConformance runs the decision.Decider contract checks against the
// Deciders h builds, failing t on any violation. See the package doc.
func RunConformance(t *testing.T, h Harness) {
	t.Helper()
	if h.New == nil {
		t.Fatal("decisiontest: Harness.New is required")
	}
	if h.APIKey == "" {
		t.Log("decisiontest: Harness.APIKey is empty: the Decider holds no credential of its own; key checks skipped")
	}

	probe := h.instance(t, Script{})
	caps := decision.CapabilitiesOf(probe.Decider)

	t.Run("Capabilities", func(t *testing.T) { checkCapabilities(t, probe, caps) })
	if !caps.Declared {
		return
	}
	t.Run("ValidateRefusesBeforeProvider", func(t *testing.T) { checkValidate(t, h) })
	if len(caps.Kinds) == 0 {
		t.Run("Abstain", func(t *testing.T) { checkAbstain(t, h) })
		return
	}
	t.Run("BatchClassifiesPerQuestion", func(t *testing.T) { checkBatch(t, h, caps) })
	t.Run("RefusalIsDeclined", func(t *testing.T) { checkRefusal(t, h, caps) })
	t.Run("ImagesOnlyWhenDeclared", func(t *testing.T) { checkImages(t, h, caps) })
	t.Run("ProviderFailureIsAnError", func(t *testing.T) { checkProviderFailure(t, h, caps) })
	t.Run("NoKeyInEchoedFailure", func(t *testing.T) { checkEchoedKey(t, h, caps) })
}

// instance builds one Instance through h and checks it is usable.
func (h Harness) instance(t *testing.T, s Script) Instance {
	t.Helper()
	in := h.New(t, s)
	if in.Decider == nil {
		t.Fatal("decisiontest: Harness.New returned an Instance with a nil Decider")
	}
	if in.ProviderCalls == nil {
		t.Fatal("decisiontest: Harness.New returned an Instance with a nil ProviderCalls")
	}
	return in
}

// decide builds an Instance for s, calls Decide with s.Request and checks
// that no error it returns contains the harness's key.
func (h Harness) decide(t *testing.T, s Script) (Instance, decision.Response, error) {
	t.Helper()
	in := h.instance(t, s)
	resp, err := in.Decider.Decide(context.Background(), s.Request)
	h.checkNoKey(t, err)
	return in, resp, err
}

// checkNoKey fails t when err, printed either way, contains h.APIKey.
func (h Harness) checkNoKey(t *testing.T, err error) {
	t.Helper()
	if err == nil || h.APIKey == "" {
		return
	}
	if strings.Contains(err.Error(), h.APIKey) || strings.Contains(fmt.Sprintf("%+v", err), h.APIKey) {
		t.Errorf("key leak: an error contains the API key: %q", err.Error())
	}
}

// checkCapabilities checks the declaration itself.
func checkCapabilities(t *testing.T, probe Instance, caps decision.Capabilities) {
	if !caps.Declared {
		t.Error("capabilities: CapabilitiesOf reports Declared false; a Decider under conformance declares what it answers (implement decision.CapabilityDeclarer)")
		return
	}
	if again := decision.CapabilitiesOf(probe.Decider); !reflect.DeepEqual(again, caps) {
		t.Errorf("capabilities: two reads differ: %+v then %+v", caps, again)
	}
	if n := probe.ProviderCalls(); n != 0 {
		t.Errorf("capabilities: reading the declaration made %d provider call(s), want 0", n)
	}
	for _, k := range caps.Kinds {
		if k != decision.KindChoice && k != decision.KindScore && k != decision.KindPredicate {
			t.Errorf("capabilities: declares kind %q, which decision.Validate refuses", k)
		}
	}
	if none := caps.ConfidenceSemantics == decision.ConfidenceNone; none != (len(caps.Kinds) == 0) {
		t.Errorf("capabilities: declares kinds %v with confidence semantics %q; a Decider reports no probabilities exactly when it answers no kind", caps.Kinds, caps.ConfidenceSemantics)
	}
	if caps.MaxOptions < 0 || caps.MinScoreLevels < 0 || caps.MaxScoreLevels < 0 || caps.MaxQuestionsPerCall < 0 {
		t.Errorf("capabilities: a negative limit: %+v", caps)
	}
	if caps.MinScoreLevels > 0 && caps.MaxScoreLevels > 0 && caps.MinScoreLevels > caps.MaxScoreLevels {
		t.Errorf("capabilities: MinScoreLevels %d above MaxScoreLevels %d", caps.MinScoreLevels, caps.MaxScoreLevels)
	}
	if hasKind(caps, decision.KindChoice) && caps.MaxOptions == 1 {
		t.Error("capabilities: declares KindChoice with MaxOptions 1; a choice needs two options")
	}
	if caps.SingleRoundTrip && noProvider(caps) {
		t.Errorf("capabilities: declares a single provider round trip and latency %q, cost %q (no provider call)", caps.Latency, caps.Cost)
	}
}

// invalidRequest is one Request decision.Validate refuses.
type invalidRequest struct {
	name string
	req  decision.Request
}

// invalidRequests returns Requests every version of decision.Validate
// refuses for a structural reason; none depends on a vendor limit.
func invalidRequests() []invalidRequest {
	const state = "decisiontest: a state"
	pred := decision.Question{ID: "q", Kind: decision.KindPredicate, Instructions: "is the state a state?"}
	one := func(q decision.Question) decision.Request {
		return decision.Request{State: state, Questions: []decision.Question{q}}
	}
	return []invalidRequest{
		{"empty State", decision.Request{Questions: []decision.Question{pred}}},
		{"empty question ID", one(decision.Question{Kind: decision.KindPredicate, Instructions: "is it?"})},
		{"duplicate question ID", decision.Request{State: state, Questions: []decision.Question{pred, pred}}},
		{"unknown kind", one(decision.Question{ID: "q", Kind: decision.Kind("decisiontest-unknown-kind")})},
		{"choice with one option", one(decision.Question{ID: "q", Kind: decision.KindChoice, Options: []decision.Option{{Name: "only"}}})},
		{"choice with a duplicate option", one(decision.Question{ID: "q", Kind: decision.KindChoice, Options: []decision.Option{{Name: "a"}, {Name: "a"}}})},
		{"choice with an empty option name", one(decision.Question{ID: "q", Kind: decision.KindChoice, Options: []decision.Option{{Name: "a"}, {Name: ""}}})},
		{"score with Max not above Min", one(decision.Question{ID: "q", Kind: decision.KindScore, Min: 1, Max: 1})},
	}
}

// checkValidate checks that every invalid Request is refused before the
// provider.
func checkValidate(t *testing.T, h Harness) {
	for _, c := range invalidRequests() {
		t.Run(c.name, func(t *testing.T) {
			if decision.Validate(c.req) == nil {
				t.Fatalf("decisiontest: fixture %q passes this decision.Validate; the suite is out of date", c.name)
			}
			in, resp, err := h.decide(t, Script{Request: c.req})
			if err == nil {
				t.Errorf("validate: Decide(%s) error = nil, want an error", c.name)
			}
			if len(resp.Answers) != 0 {
				t.Errorf("validate: Decide(%s) returned %d answer(s) for an invalid Request", c.name, len(resp.Answers))
			}
			if n := in.ProviderCalls(); n != 0 {
				t.Errorf("validate: Decide(%s) made %d provider call(s), want 0: an invalid Request never reaches the provider", c.name, n)
			}
		})
	}
}

// checkAbstain checks a Decider that declares no kinds.
func checkAbstain(t *testing.T, h Harness) {
	req := decision.Request{
		State:     "decisiontest: a state",
		Questions: []decision.Question{{ID: "q", Kind: decision.KindPredicate, Instructions: "is the state a state?"}},
	}
	in, resp, err := h.decide(t, Script{Request: req})
	if !errors.Is(err, decision.ErrNoProvider) {
		t.Errorf("abstain: a Decider that declares no kinds answered a valid Request with error %v, want decision.ErrNoProvider", err)
	}
	if len(resp.Answers) != 0 || len(resp.Invalid) != 0 {
		t.Errorf("abstain: Response carries %d answer(s) and %d invalid, want none", len(resp.Answers), len(resp.Invalid))
	}
	if resp.Provider != in.Decider.Name() {
		t.Errorf("abstain: Response.Provider = %q, want the Decider's Name %q", resp.Provider, in.Decider.Name())
	}
	if n := in.ProviderCalls(); n != 0 {
		t.Errorf("abstain: made %d provider call(s), want 0", n)
	}
}

// batch is the suite's mixed Request and the provider's script for it.
type batch struct {
	script Script
	good   []decision.Question // answered validly; must land in Answers
	bad    []decision.Question // answered badly or omitted; must land in Invalid
	want   map[string]ScriptedAnswer
}

// buildBatch builds one question per declared kind plus a bad choice (when
// KindChoice is declared) and an omitted answer, within the declared
// limits.
func buildBatch(t *testing.T, caps decision.Capabilities) batch {
	const state = "decisiontest: a widget stopped after a grinding noise"
	b := batch{want: map[string]ScriptedAnswer{}}
	var first decision.Question
	for _, k := range caps.Kinds {
		q, a := goodQuestion(k, caps)
		b.good = append(b.good, q)
		b.want[q.ID] = a
		if first.ID == "" {
			first = q
		}
	}
	omitted := first
	omitted.ID = "omitted-q"
	b.bad = append(b.bad, omitted)
	if hasKind(caps, decision.KindChoice) {
		q, a := goodQuestion(decision.KindChoice, caps)
		q.ID, a.ID, a.Choice = "bad-choice-q", "bad-choice-q", "decisiontest-not-an-option"
		b.bad = append(b.bad, q)
		b.want[q.ID] = a
	}

	// The first good question, then the omitted one, then the rest, so a
	// per-call limit of two still pairs a good answer with a bad one.
	questions := append([]decision.Question{b.good[0], b.bad[0]}, b.good[1:]...)
	questions = append(questions, b.bad[1:]...)
	if limit := caps.MaxQuestionsPerCall; limit > 0 && limit < len(questions) {
		t.Logf("batch: the Decider declares at most %d question(s) per call; the batch keeps the first %d of %d", limit, limit, len(questions))
		questions = questions[:limit]
		b.good, b.bad = keep(b.good, questions), keep(b.bad, questions)
	}
	b.script.Request = decision.Request{State: state, Questions: questions}
	for _, q := range questions {
		if a, ok := b.want[q.ID]; ok {
			b.script.Answers = append(b.script.Answers, a)
		}
	}
	return b
}

// goodQuestion returns one question of kind k within caps, and the answer
// the provider gives it.
func goodQuestion(k decision.Kind, caps decision.Capabilities) (decision.Question, ScriptedAnswer) {
	switch k {
	case decision.KindChoice:
		names := []string{"alpha", "beta", "gamma"}
		probs := []float64{0.7, 0.2, 0.1}
		if caps.MaxOptions == 2 {
			names, probs = names[:2], []float64{0.7, 0.3}
		}
		q := decision.Question{ID: "choice-q", Kind: k, Instructions: "what caused it?"}
		a := ScriptedAnswer{ID: q.ID, Kind: k, Choice: names[0], Probabilities: map[string]float64{}, SelfConfidence: SelfReportedConfidence}
		for i, n := range names {
			q.Options = append(q.Options, decision.Option{Name: n})
			a.Probabilities[n] = probs[i]
		}
		return q, a
	case decision.KindScore:
		n := 3
		if caps.MinScoreLevels > n {
			n = caps.MinScoreLevels
		}
		if caps.MaxScoreLevels > 0 && caps.MaxScoreLevels < n {
			n = caps.MaxScoreLevels
		}
		q := decision.Question{ID: "score-q", Kind: k, Instructions: "how severe is it?"}
		a := ScriptedAnswer{ID: q.ID, Kind: k, Score: float64(n - 1), SelfConfidence: SelfReportedConfidence}
		for i := 0; i < n; i++ {
			q.Levels = append(q.Levels, "level "+strconv.Itoa(i))
			p := 0.7
			if i < n-1 {
				p = 0.3 / float64(n-1)
			}
			a.LevelProbabilities = append(a.LevelProbabilities, p)
		}
		return q, a
	default: // decision.KindPredicate
		q := decision.Question{ID: "predicate-q", Kind: k, Instructions: "is it safe to restart?"}
		return q, ScriptedAnswer{ID: q.ID, Kind: k, Probability: 0.2, SelfConfidence: SelfReportedConfidence}
	}
}

// checkBatch checks per-question classification, the answered values and
// Confidence over one mixed batch.
func checkBatch(t *testing.T, h Harness, caps decision.Capabilities) {
	verifyBatch(t, h, caps, buildBatch(t, caps))
}

// verifyBatch runs b and checks per-question classification, the answered
// values and Confidence.
func verifyBatch(t *testing.T, h Harness, caps decision.Capabilities, b batch) {
	t.Helper()
	in, resp, err := h.decide(t, b.script)
	if err != nil {
		t.Fatalf("batch: Decide failed the whole batch: %v; one bad answer never fails the batch", err)
	}
	checkExclusive(t, b.script.Request, resp)
	if resp.Provider != in.Decider.Name() {
		t.Errorf("batch: Response.Provider = %q, want the Decider's Name %q", resp.Provider, in.Decider.Name())
	}
	calls := in.ProviderCalls()
	if caps.SingleRoundTrip && calls != 1 {
		t.Errorf("batch: declares a single round trip but made %d provider calls for one batch", calls)
	}
	if noProvider(caps) && calls != 0 {
		t.Errorf("batch: declares latency %q, cost %q (no provider call) but made %d", caps.Latency, caps.Cost, calls)
	}

	asked := map[string]bool{}
	for _, q := range b.script.Request.Questions {
		asked[q.ID] = true
	}
	for id := range resp.Answers {
		if !asked[id] {
			t.Errorf("batch: Answers holds %q, which was not asked", id)
		}
	}
	for id := range resp.Invalid {
		if !asked[id] {
			t.Errorf("batch: Invalid holds %q, which was not asked", id)
		}
	}
	for _, q := range b.bad {
		if _, ok := resp.Answers[q.ID]; ok {
			t.Errorf("batch: bad answer %q landed in Answers, want it only in Invalid", q.ID)
		}
		if reason, ok := resp.Invalid[q.ID]; !ok || reason == "" {
			t.Errorf("batch: bad answer %q is not in Invalid with a reason (got %q, %v)", q.ID, reason, ok)
		}
	}
	for _, q := range b.good {
		if reason, ok := resp.Invalid[q.ID]; ok {
			t.Errorf("batch: good answer %q landed in Invalid: %s", q.ID, reason)
		}
		a, ok := resp.Answers[q.ID]
		if !ok {
			t.Errorf("batch: good answer %q is missing from Answers", q.ID)
			continue
		}
		checkAnswer(t, h, q, a, b.want[q.ID])
	}
}

// checkExclusive fails t when a question of req is in more than one of
// resp's Answers, Invalid and Declined.
func checkExclusive(t *testing.T, req decision.Request, resp decision.Response) {
	t.Helper()
	for _, q := range req.Questions {
		_, answered := resp.Answers[q.ID]
		_, invalid := resp.Invalid[q.ID]
		_, declined := resp.Declined[q.ID]
		n := 0
		for _, in := range []bool{answered, invalid, declined} {
			if in {
				n++
			}
		}
		if n > 1 {
			t.Errorf("exclusive: question %q is in %d of Answers (%v), Invalid (%v) and Declined (%v); a question has one outcome", q.ID, n, answered, invalid, declined)
		}
	}
}

// checkRefusal checks that a question the provider refuses lands in
// Response.Declined, when the harness declares its wire can refuse.
func checkRefusal(t *testing.T, h Harness, caps decision.Capabilities) {
	if !h.Refusals {
		t.Skip("decisiontest: the harness declares Refusals false: the provider's wire carries no refusal, so none is checked")
	}
	b := buildBatch(t, caps)
	refused := b.good[0].ID
	for i := range b.script.Answers {
		if b.script.Answers[i].ID == refused {
			b.script.Answers[i].Refused = true
		}
	}
	_, resp, err := h.decide(t, b.script)
	if err != nil {
		t.Fatalf("refusal: Decide failed the whole batch: %v; a refusal never fails the batch", err)
	}
	checkExclusive(t, b.script.Request, resp)
	if reason, ok := resp.Declined[refused]; !ok || reason == "" {
		t.Errorf("refusal: refused question %q is not in Declined with a reason (got %q, %v)", refused, reason, ok)
	}
	if a, ok := resp.Answers[refused]; ok {
		t.Errorf("refusal: refused question %q was read as an answer: %+v", refused, a)
	}
	if why, ok := resp.Invalid[refused]; ok {
		t.Errorf("refusal: refused question %q landed in Invalid (%s), want Declined", refused, why)
	}
	for _, q := range b.good[1:] {
		if _, ok := resp.Answers[q.ID]; !ok {
			t.Errorf("refusal: good answer %q beside the refusal is missing from Answers", q.ID)
		}
	}
}

// checkImages checks that a Request carrying an image is answered only by
// a Decider that declares Capabilities.Images, and never without the image.
func checkImages(t *testing.T, h Harness, caps decision.Capabilities) {
	b := buildBatch(t, caps)
	b.script.Request.Images = []decision.Image{{MediaType: "image/png", Data: []byte("decisiontest: image bytes")}}
	if caps.Images {
		verifyBatch(t, h, caps, b)
		return
	}
	in, resp, err := h.decide(t, b.script)
	if n := in.ProviderCalls(); n != 0 {
		t.Errorf("images: a Decider that declares no Images made %d provider call(s) for a Request carrying one, want 0", n)
	}
	if len(resp.Answers) != 0 {
		t.Errorf("images: a Decider that declares no Images answered %d question(s) of a Request carrying one, without the image", len(resp.Answers))
	}
	if err != nil {
		return
	}
	for _, q := range b.script.Request.Questions {
		_, invalid := resp.Invalid[q.ID]
		_, declined := resp.Declined[q.ID]
		if !invalid && !declined {
			t.Errorf("images: question %q of a Request carrying an image is in neither Invalid nor Declined, want a reason", q.ID)
		}
	}
}

// checkAnswer checks one valid Answer against what the provider answered.
func checkAnswer(t *testing.T, h Harness, q decision.Question, a decision.Answer, want ScriptedAnswer) {
	t.Helper()
	if a.ID != q.ID || a.Kind != q.Kind {
		t.Errorf("answer %q: ID, Kind = %q, %q, want %q, %q", q.ID, a.ID, a.Kind, q.ID, q.Kind)
	}
	var conf float64
	switch q.Kind {
	case decision.KindChoice:
		if a.Choice != want.Choice {
			t.Errorf("answer %q: Choice = %q, want %q", q.ID, a.Choice, want.Choice)
		}
		if len(a.Probabilities) != len(q.Options) {
			t.Errorf("answer %q: %d probabilities, want one per option (%d)", q.ID, len(a.Probabilities), len(q.Options))
		}
		for _, o := range q.Options {
			if got, ok := a.Probabilities[o.Name]; !ok || math.Abs(got-want.Probabilities[o.Name]) > tolerance {
				t.Errorf("answer %q: Probabilities[%q] = %v, want %v", q.ID, o.Name, got, want.Probabilities[o.Name])
			}
		}
		conf = decision.Confidence(want.Probabilities)
	case decision.KindScore:
		if math.Abs(a.Score-want.Score) > tolerance {
			t.Errorf("answer %q: Score = %v, want %v (a position in the question's %d Levels)", q.ID, a.Score, want.Score, len(q.Levels))
		}
		conf = decision.Confidence(nil)
		if h.ScoreConfidenceFromLevels {
			conf = decision.Confidence(levelMap(want.LevelProbabilities))
		}
	default: // decision.KindPredicate
		if math.Abs(a.Noul-want.Probability) > tolerance {
			t.Errorf("answer %q: Noul = %v, want %v", q.ID, a.Noul, want.Probability)
		}
		conf = math.Max(want.Probability, 1-want.Probability)
	}
	switch {
	case math.Abs(a.Confidence-want.SelfConfidence) <= tolerance:
		t.Errorf("confidence %q: Confidence = %v is the provider's self-reported value; want this package's formula, %v", q.ID, a.Confidence, conf)
	case math.IsNaN(a.Confidence) || math.Abs(a.Confidence-conf) > tolerance:
		t.Errorf("confidence %q: Confidence = %v, want this package's formula, %v", q.ID, a.Confidence, conf)
	}
}

// checkProviderFailure checks that a failing provider is an error.
func checkProviderFailure(t *testing.T, h Harness, caps decision.Capabilities) {
	b := buildBatch(t, caps)
	b.script.Fail = "decisiontest: scripted provider failure"
	in, resp, err := h.decide(t, b.script)
	if err == nil {
		t.Fatal("provider failure: Decide error = nil when the provider failed")
	}
	if errors.Is(err, decision.ErrNoProvider) {
		t.Errorf("provider failure: error %v is decision.ErrNoProvider; a configured provider that fails is not an absent one", err)
	}
	if len(resp.Answers) != 0 {
		t.Errorf("provider failure: Response carries %d answer(s) with an error", len(resp.Answers))
	}
	if !noProvider(caps) && in.ProviderCalls() == 0 {
		t.Error("provider failure: the scripted failure never reached the provider")
	}
}

// checkEchoedKey checks that a provider echoing the key back does not put
// it in an error, when the harness declares the Decider redacts it.
func checkEchoedKey(t *testing.T, h Harness, caps decision.Capabilities) {
	switch {
	case h.APIKey == "":
		t.Skip("decisiontest: no APIKey; nothing to echo")
	case !h.RedactsEchoedKey:
		t.Skip("decisiontest: the harness declares RedactsEchoedKey false: a provider failure that echoes the key is NOT checked")
	}
	b := buildBatch(t, caps)
	b.script.Fail = "decisiontest: credential " + h.APIKey + " was rejected"
	_, _, err := h.decide(t, b.script)
	if err == nil {
		t.Error("echoed key: Decide error = nil when the provider failed")
	}
}

// hasKind reports whether caps declares k.
func hasKind(caps decision.Capabilities, k decision.Kind) bool {
	for _, d := range caps.Kinds {
		if d == k {
			return true
		}
	}
	return false
}

// noProvider reports whether caps declares that Decide calls no provider.
func noProvider(caps decision.Capabilities) bool {
	return caps.Latency == decision.LatencyNone || caps.Cost == decision.CostNone
}

// keep returns the questions of qs whose IDs are in within, in qs order.
func keep(qs, within []decision.Question) []decision.Question {
	var out []decision.Question
	for _, q := range qs {
		for _, w := range within {
			if w.ID == q.ID {
				out = append(out, q)
				break
			}
		}
	}
	return out
}

// levelMap keys a level distribution by level index, the shape
// decision.Confidence takes.
func levelMap(ps []float64) map[string]float64 {
	m := make(map[string]float64, len(ps))
	for i, p := range ps {
		m[strconv.Itoa(i)] = p
	}
	return m
}
