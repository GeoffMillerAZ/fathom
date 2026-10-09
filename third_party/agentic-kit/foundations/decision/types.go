package decision

import (
	"context"
	"errors"
	"time"
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
	// Kind selects the question shape: KindChoice, KindScore, or
	// KindPredicate.
	Kind Kind
	// Instructions is the question text shown to the decider — for
	// KindPredicate, the statement whose truth probability is being asked.
	Instructions string

	// Options is the set of named alternatives. Meaningful only for
	// KindChoice: 2..255 options with unique names (enforced by Validate).
	Options []Option

	// Min and Max bound a KindScore answer. Meaningful only for KindScore:
	// Max must be greater than Min (enforced by Validate), unless Levels is
	// set — see Levels.
	Min, Max float64
	// Rubric is optional guidance for scoring a KindScore question. May be
	// empty.
	Rubric string
	// Levels is an optional ordered list of level descriptions for a
	// KindScore question, low to high (at least 2 entries when set —
	// enforced by Validate, which sets no upper bound). When set, Min/Max
	// must be either unset (both zero) or exactly 0 and len(Levels)-1
	// (enforced by Validate); a Score answer then addresses a position in
	// Levels rather than an arbitrary [Min, Max] float. Meaningful only
	// for KindScore; ignored otherwise. Each adapter maps Levels onto its
	// own wire and declares how many levels it takes (Capabilities'
	// MinScoreLevels and MaxScoreLevels): TypeSafe takes 2..10 and records
	// an out-of-limit question in Response.Invalid; LLM declares no limit.
	Levels []string

	// TrueCriteria and FalseCriteria are optional free text describing what
	// counts as true/false for a KindPredicate question. May be empty.
	TrueCriteria, FalseCriteria string
}

// Image is one image a Request carries alongside its State: the raw bytes
// and their media type. A Decider sends it only when it declares
// Capabilities.Images; one that does not refuses the Request's questions
// (see UnsupportedImages) rather than answer them without the image.
//
// CUE mirror: #Image in schema.cue.
type Image struct {
	// MediaType is the image's media type, for example "image/png". Must
	// begin with "image/" (enforced by Validate).
	MediaType string
	// Data is the image's raw bytes, not base64. Must be non-empty
	// (enforced by Validate).
	Data []byte
}

// Request bundles a caller's State — the blob of state being asked about,
// rendered as text — with the Questions to ask about it, and optionally
// the Images the Questions are also about.
//
// CUE mirror: #Request in schema.cue.
type Request struct {
	// State is the blob of state the Questions are asked about. Must be
	// non-empty (enforced by Validate).
	State string
	// Questions are the typed questions to ask about State, in the order a
	// Decider should answer and render them.
	Questions []Question
	// Images are images the Questions are asked about together with State,
	// in order. Optional: empty means a text-only Request, the only kind
	// before Images existed, which every Decider answers exactly as before.
	// A Decider that does not declare Capabilities.Images answers none of a
	// Request's questions when Images is set (see UnsupportedImages).
	Images []Image `json:",omitempty"`
}

// Answer is one Decider's answer to one Question, identified by ID and
// Kind. Exactly the fields meaningful for Kind are populated:
//
//   - KindChoice: Choice (one of the Question's Option names) and
//     Probabilities (every option present, summing to 1.0).
//   - KindScore: Score, within [Question.Min, Question.Max], and, when the
//     provider reports a distribution over the levels, Probabilities keyed
//     by each level's 0-based position as a decimal string ("0", "1", ...).
//   - KindPredicate: Noul, the probability the statement is true, within
//     [0, 1].
//
// Confidence is always populated: for KindChoice, Confidence(Probabilities);
// for KindScore, Confidence over the level distribution when the adapter's
// wire carries one (typesafe and openai: Confidence(Probabilities)), else
// Confidence over a degenerate single-outcome input, which is 1.0 by
// Confidence's own k<=1 rule (llm, whose wire has no distribution — see the
// package doc); for KindPredicate, Noul when Noul>=0.5, else 1-Noul. No
// adapter passes a provider's own confidence number through.
type Answer struct {
	// ID is the answered Question's ID.
	ID string
	// Kind is the answered Question's Kind.
	Kind Kind

	// Choice is the selected option name (KindChoice only).
	Choice string
	// Probabilities maps every KindChoice option name to its probability;
	// the values sum to 1.0. For KindScore it is the provider's distribution
	// over the levels, keyed by 0-based level position ("0", "1", ...), when
	// the adapter's wire carries one (typesafe and openai do; llm leaves it
	// nil). Nil for KindPredicate.
	Probabilities map[string]float64

	// Score is the answer value within [Question.Min, Question.Max]
	// (KindScore only).
	Score float64

	// Noul is the probability the KindPredicate question's statement is
	// true, within [0, 1] (KindPredicate only).
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
	// Declined names, by Question.ID, every question the provider refused
	// to answer (a refusal on its wire), with a human-readable reason. A
	// declined question is absent from Answers and from Invalid: it is
	// never read as false, zero or the lowest option, and a decorator never
	// turns it into an answer (Fallback treats it as a disclosed miss,
	// Cascade escalates it). Nil when no question was declined, including
	// from every adapter whose wire has no refusal (llm, typesafe).
	Declined map[string]string `json:",omitempty"`

	// Usage reports token accounting for the call, when the provider
	// reports it.
	Usage Usage
	// Latency is how long the call took.
	Latency time.Duration
}

// Decider answers typed fast-decision Questions about a blob of state.
// Implementations: None (the zero-configuration stub, in this package),
// llm.LLM (backed by an llmclient.LLMClient, in foundations/decision/llm)
// and typesafe.TypeSafe (a native typed-judgment provider, in
// foundations/decision/typesafe) — see the package doc's Adapters section.
type Decider interface {
	// Name identifies the Decider implementation (e.g. "none", "llm").
	Name() string
	// Decide answers req.Questions about req.State. ctx governs
	// cancellation/timeout of any underlying call.
	Decide(ctx context.Context, req Request) (Response, error)
}

// ErrDeclined marks a provider's refusal where a decorator reports it as an
// error: a Fallback records a Response that declined questions as a
// failure wrapping ErrDeclined (see Fallback). Adapters never return it from
// Decide; a refusal is a Response.Declined entry, not an error.
var ErrDeclined = errors.New("decision: the provider declined to answer")

// ErrNoProvider is returned by None.Decide: no fast-decision provider is
// configured. Callers can distinguish "not configured" from a real provider
// failure with errors.Is(err, ErrNoProvider) instead of inspecting error
// strings.
var ErrNoProvider = errors.New("decision: no provider configured")
