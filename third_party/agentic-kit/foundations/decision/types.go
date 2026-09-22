package decision

import (
	"context"
	"errors"
	"time"
)

// Kind selects which shape of fast-decision question a Question asks.
//
// CUE mirror: #Kind in schema.cue.
type Kind string

// The three question kinds, mirroring TypeSafe AI's System One primitives
// (Choice/Score/Noul) — see the package doc's mapping table.
const (
	// KindChoice asks the caller to pick among named Options and reports a
	// probability distribution over them.
	KindChoice Kind = "choice"
	// KindScore asks for a value on a continuous [Min, Max] range.
	KindScore Kind = "score"
	// KindNoul asks a yes/no question and reports the probability the
	// statement is true.
	KindNoul Kind = "noul"
)

// Option is one named alternative for a KindChoice Question. Criteria is
// optional free text describing when this option applies; it may be empty.
//
// CUE mirror: #Option in schema.cue.
type Option struct {
	// Name identifies the option. Must be unique within a Question's
	// Options and non-empty (enforced by Validate).
	Name string
	// Criteria is optional free text describing when this option applies.
	// May be empty.
	Criteria string
}

// Question is one typed fast-decision question. Kind selects which of the
// remaining fields are meaningful — see the Kind constants and Validate.
//
// CUE mirror: #Question in schema.cue.
type Question struct {
	// ID identifies this question within a Request. Must be non-empty and
	// unique within the Request (enforced by Validate); it keys
	// Response.Answers and Response.Invalid.
	ID string
	// Kind selects the question shape: KindChoice, KindScore, or KindNoul.
	Kind Kind
	// Instructions is the question text shown to the decider — for
	// KindNoul, the statement whose truth probability is being asked.
	Instructions string

	// Options is the set of named alternatives. Meaningful only for
	// KindChoice: 2..255 options with unique names (enforced by Validate).
	Options []Option

	// Min and Max bound a KindScore answer. Meaningful only for KindScore:
	// Max must be greater than Min (enforced by Validate).
	Min, Max float64
	// Rubric is optional guidance for scoring a KindScore question. May be
	// empty.
	Rubric string

	// TrueCriteria and FalseCriteria are optional free text describing what
	// counts as true/false for a KindNoul question. May be empty.
	TrueCriteria, FalseCriteria string
}

// Request bundles a caller's State — the blob of state being asked about,
// rendered as text — with the Questions to ask about it.
//
// CUE mirror: #Request in schema.cue.
type Request struct {
	// State is the blob of state the Questions are asked about. Must be
	// non-empty (enforced by Validate).
	State string
	// Questions are the typed questions to ask about State, in the order a
	// Decider should answer and render them.
	Questions []Question
}

// Answer is one Decider's answer to one Question, identified by ID and
// Kind. Exactly the fields meaningful for Kind are populated:
//
//   - KindChoice: Choice (one of the Question's Option names) and
//     Probabilities (every option present, summing to 1.0).
//   - KindScore: Score, within [Question.Min, Question.Max].
//   - KindNoul: Noul, within [0, 1].
//
// Confidence is always populated: for KindChoice, Confidence(Probabilities);
// for KindScore, Confidence called over a degenerate single-outcome input
// (a Score has no discrete alternative set to form a distribution over, so
// this collapses to 1.0 by Confidence's own k<=1 rule — see the package
// doc); for KindNoul, Noul when Noul>=0.5, else 1-Noul.
type Answer struct {
	// ID is the answered Question's ID.
	ID string
	// Kind is the answered Question's Kind.
	Kind Kind

	// Choice is the selected option name (KindChoice only).
	Choice string
	// Probabilities maps every KindChoice option name to its probability;
	// the values sum to 1.0 (KindChoice only).
	Probabilities map[string]float64

	// Score is the answer value within [Question.Min, Question.Max]
	// (KindScore only).
	Score float64

	// Noul is the probability the KindNoul question's statement is true,
	// within [0, 1] (KindNoul only).
	Noul float64

	// Confidence is a single confidence number in [0, 1] — see the type
	// doc for how it is derived per Kind.
	Confidence float64
}

// Usage reports token accounting for one Decide call. Zero values mean "not
// reported".
type Usage struct {
	// InputTokens is the token count of the request sent to the provider.
	InputTokens int
	// OutputTokens is the token count of the provider's response.
	OutputTokens int
}

// Response is the outcome of one Decide call over a Request's Questions.
type Response struct {
	// Provider is the Decider's Name() that produced this Response.
	Provider string
	// Model is the underlying model identifier, when the provider has one.
	// Empty for providers without a model concept (e.g. None).
	Model string

	// Answers holds one Answer per Question the provider answered validly,
	// keyed by Question.ID. A Question the provider could not answer
	// validly is ABSENT here and named in Invalid instead.
	Answers map[string]Answer
	// Invalid names, by Question.ID, every question the provider could not
	// answer validly (malformed, a choice not in the options, or out of
	// range), with a human-readable reason.
	Invalid map[string]string

	// Usage reports token accounting for the call, when the provider
	// reports it.
	Usage Usage
	// Latency is how long the call took.
	Latency time.Duration
}

// Decider answers typed fast-decision Questions about a blob of state.
// Implementations: None (the zero-configuration stub) and LLM (backed by an
// llmclient.LLMClient). A future "typesafe" adapter is parked — see the
// package doc.
type Decider interface {
	// Name identifies the Decider implementation (e.g. "none", "llm").
	Name() string
	// Decide answers req.Questions about req.State. ctx governs
	// cancellation/timeout of any underlying call.
	Decide(ctx context.Context, req Request) (Response, error)
}

// ErrNoProvider is returned by None.Decide: no fast-decision provider is
// configured. Callers can distinguish "not configured" from a real provider
// failure with errors.Is(err, ErrNoProvider) instead of inspecting error
// strings.
var ErrNoProvider = errors.New("decision: no provider configured")
