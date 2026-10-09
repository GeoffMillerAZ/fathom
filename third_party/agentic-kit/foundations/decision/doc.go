// Package decision is a typed fast-decision port: callers ask typed
// questions about a blob of state and get probabilities back, never text.
//
// # Why "fast decision"
//
// A fast-decision call answers a narrow, structured question (a choice
// among named options, a score on a range, a yes/no probability) rather
// than carrying on a conversation. The Decider interface is deliberately
// thin — Name and one Decide call taking a batch of Questions against one
// State — so a caller can swap providers without touching call sites.
//
// # Kinds
//
// Three generic question kinds are the port's vocabulary; none is any
// provider's term:
//
//	decision.Kind    Asks                          Answer
//	KindChoice       pick one of 2..255 named      Choice plus a probability
//	                 Options                       per option
//	KindScore        a value on [Min, Max], or a   Score
//	                 position among ordered Levels
//	KindPredicate    is a stated criterion true?   Noul, the probability it
//	                                               is true
//
// Each adapter maps the kinds onto its own wire and keeps any vendor
// vocabulary there. ParseKind decodes a Kind from text and accepts "noul"
// (the predicate's former name and one vendor's term) as an alias for
// KindPredicate; Validate and every adapter also accept a Kind holding that
// alias and answer it as KindPredicate.
//
// # Adapters
//
// None, the zero-configuration stub that always returns ErrNoProvider,
// ships in this package. The provider adapters live in their own packages
// and import this one, never the reverse, so the port carries no vendor
// code and no transport:
//
//   - foundations/decision/llm: llm.LLM, one llmclient.LLMClient completion
//     per Decide call, asked to answer the typed contract via a rendered
//     prompt and JSON schema; it renders Question.Levels into both when set.
//   - foundations/decision/typesafe: typesafe.TypeSafe, a native
//     typed-judgment provider's API called directly rather than emulated,
//     on its own endpoint or through OpenRouter's Decisions router
//     (TypeSafeConfig.Transport); its doc carries the kind mapping and wire
//     details.
//   - foundations/decision/openai: openai.OpenAI, OpenAI's Decisions API,
//     which also takes images (Request.Images) and can refuse a question.
//
// Every provider adapter is optional and replaceable at the contract, but
// not interchangeable without a reading: the same question gets different
// numbers from different providers, so calibration is per provider and
// resolved model (decision 0017, decision 0018). Remote adapters take their
// key from a file reference or an environment variable name (or a value
// set in code), never from configuration inline, and redact any key a
// provider echoes into an error.
//
// A caller switches from one adapter to another by construction swap alone:
// each produces the identical Response/Answer/Confidence shape. A
// typesafe.TypeSafe or openai.OpenAI with no key source cannot be
// constructed at all (their constructors return an error); a caller with
// no key configured constructs llm.LLM or None instead, and no adapter
// silently falls back on your behalf.
//
// # Refusals and images
//
// A provider that refuses a question reports it in Response.Declined, by
// question ID: the question is absent from Answers and Invalid, so a
// refusal is never read as false, zero or the lowest option. Cascade
// escalates a declined question like an abstention; Fallback treats a
// Response that declines any question as a disclosed miss (a failure
// wrapping ErrDeclined) and tries the next Decider; Cached never stores
// one. A Request may carry Images; a Decider that does not declare
// Capabilities.Images answers none of its questions (UnsupportedImages)
// rather than answer without the image.
//
// An adapter written outside the kit implements Decider, may implement
// CapabilityDeclarer, and uses CanonicalKind and OutsideDeclaredLimits to
// fold the "noul" alias and refuse a question outside its declared limits
// the way the shipped adapters do; decisiontest.RunConformance checks it.
//
// # Confidence formula
//
// Confidence implements the vendor-documented formula for turning a
// probability distribution into a single confidence number: with k options
// and the winning (maximum) probability max, confidence is 1 when k<=1
// (a degenerate single-outcome "distribution" is certain by construction),
// otherwise (k*max-1)/(k-1), clamped to [0,1]. For k=3 this is
// (3*max-1)/2. A Score answer is Confidence over the provider's
// distribution across the levels when the adapter's wire carries one
// (typesafe, openai); when it carries none (llm) there is no alternative
// set, so Confidence is called with a degenerate (k<=1) input and is 1 —
// see Answer's doc comment. A predicate answer
// instead uses the simpler two-outcome rule documented on
// Answer.Confidence, because it is not a probability distribution over
// named alternatives.
//
// This holds for every adapter: a provider whose wire response carries its
// own "confidence" field is NEVER trusted for it — Confidence is always
// recomputed from the wire's probabilities by this package's own formula.
// The invariant is about where the number comes from, not a disagreement
// in the math.
//
// BandOf classifies a Confidence value into a Band a caller can switch on
// without re-deriving thresholds: below UnsureBelow is BandUnsure, at or
// above ProceedAtOrAbove is BandProceed, otherwise BandConfirm.
//
// # Batching
//
// Every adapter answers a whole Request.Questions batch in ONE round trip:
// llm.LLM as one llmclient completion, typesafe.TypeSafe as one POST
// carrying every question keyed by ID, openai.OpenAI as one POST carrying
// every question named by ID. Batch your questions per state instead of calling
// Decide once per question.
//
// # Capabilities
//
// A Decider may declare what it answers by implementing the optional
// CapabilityDeclarer interface. Read it with CapabilitiesOf, never with a
// type assertion. Decider itself is unchanged, so a Decider written outside
// this package keeps compiling and reads as undeclared (Declared false,
// every class unknown). A declaration names the kinds answered, the most
// options and the score-level bounds (0 means no limit of the Decider's
// own), whether a whole Request is one round trip, and the most questions
// per call. It also names three classes, each with an explicit unknown:
// latency, list-price cost, and confidence semantics, meaning where the
// probabilities come from (none, calibrated-probability, logprob,
// native-probability or self-reported). No semantics value is a calibration
// reading: calibration is per site and the caller measures it. Two flags
// complete it: Images (the Decider sends Request.Images) and ModelPinning
// (the provider documents pinning an exact model version).
// CapabilitiesOf returns a canonical copy the caller may mutate, with a
// kind declared by the alias "noul" folded into KindPredicate. None,
// MockDecider (settable, undeclared by default), llm.LLM,
// typesafe.TypeSafe and openai.OpenAI declare, and each Capabilities method makes no call and
// is safe for concurrent use.
//
// # Decorators
//
// Four decorators wrap Deciders and are Deciders themselves, so they
// compose (a Cascade whose first stage is a Cached over a Fallback, say).
// Each is built from a plain options struct, so a composer can assemble it
// from configuration data, and each declares Capabilities derived from
// the Deciders it wraps. None makes a model call of its own.
//
//   - Cascade (NewCascade) consults stages cheapest first and escalates,
//     per question, only what a stage abstained on (ErrNoProvider, Invalid,
//     or absent) or answered below a confidence threshold. The default
//     threshold, DefaultEscalateBelow, is UnsureBelow: an uncalibrated
//     starting value.
//   - Fallback (NewFallback) tries its Deciders in order and moves on only
//     when one returns an error.
//   - Cached (NewCached) serves repeated judgments from an injected
//     CacheStore keyed by (question template digest, content hash) and
//     stamped with the Provider and Model that produced them; an entry
//     under another stamp is a miss, so a better model re-grades cleanly.
//     MemoryCacheStore is in-process only: no durable store, file or table.
//   - Recorder (NewRecorder) records Request, Response and error triples
//     into a JSON-serializable Transcript, and Replay (NewReplay) answers
//     from one without calling any provider, for deterministic tests and
//     offline runs.
//
// ComposeFromConfig assembles these decorators from a Policy: configuration
// data (#Policy in schema.cue) that names its leaf Deciders, cache stores,
// transcripts and Observe hooks by key from a caller-supplied Registry, so
// reordering a cascade or a fallback, moving a threshold or caching a node
// is an edit of data, not a rebuild. It constructs no provider and no
// store, and refuses an unknown or missing key with a *ComposeError naming
// the node's path.
//
// Provenance and disclosure: a decorator never reports an answer under a
// Provider or Model that did not produce it. A Response whose answers all
// come from one inner Response carries that Response's Provider and
// Model; a merge of several carries the decorator's Name and an empty
// Model. Per-answer provenance (AnswerSource) and every escalation,
// failure, cache hit and store error are in the decorator's report, which
// DecideReport returns and the options' Observe hook receives. A Fallback
// requires Observe, so it never switches providers silently. Cache hits
// are disclosed and kept apart from proof: CachedReport.Fresh drops them,
// and no decorator writes Records or a ledger.
//
// # Concurrency
//
// None is stateless and safe for concurrent use. The adapter packages state
// their own contracts: llm.LLM holds no mutable state after construction
// and is safe for concurrent use exactly when its wrapped
// llmclient.LLMClient is; typesafe.TypeSafe and openai.OpenAI guard the key
// they read at first use with a mutex and are safe for concurrent use.
//
// Cascade and Fallback hold no mutable state after construction and are
// safe for concurrent use when the Deciders they wrap are. Cached guards
// its current stamp with a mutex and is safe for concurrent use when its
// inner Decider and CacheStore are; MemoryCacheStore is. Recorder and
// Replay guard their transcript state with a mutex and are safe for
// concurrent use (a Recorder when its inner Decider is). An Observe hook
// may be called concurrently from concurrent Decide calls.
package decision
