// Package topicgate decides whether a request is in-scope before paying for
// a full model call. It applies a layered, cheapest-first strategy:
//
//  1. Pre-filter: length and injection checks (no model call).
//  2. Embedding gate: cosine similarity against a precomputed exemplar centroid.
//  3. LLM judge (optional): called only when the score is ambiguous.
//
// When no embedding backend exists, Config.JudgeOnly runs the gate as
// prefilters → judge: layer 2 is skipped and EVERY non-prefiltered input
// costs one judge LLM call — the embedding layer is what makes most
// decisions free, so prefer it whenever an Embedder is available.
//
// The Gate is concurrency-safe and deterministic given fixed deps. It compiles
// to wasm (no file I/O, no time.Now in logic).
package topicgate

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	embeddingclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/embedding/embedding-client"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	injectiondetector "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/injection-detector"
)

// Verdict is the outcome of an Evaluate call.
type Verdict int

const (
	// Allow means the request is on-topic and may proceed.
	Allow Verdict = iota
	// SoftDeny means the request is off-topic or ambiguous; no penalty is recorded.
	SoftDeny
	// HardDeny means the request contained a security violation; worth a strike.
	HardDeny
)

// String implements fmt.Stringer.
func (v Verdict) String() string {
	switch v {
	case Allow:
		return "Allow"
	case SoftDeny:
		return "SoftDeny"
	case HardDeny:
		return "HardDeny"
	default:
		return "Unknown"
	}
}

// Decision is the result of Evaluate.
type Decision struct {
	// Verdict is the outcome.
	Verdict Verdict
	// Reason is a human-readable explanation.
	Reason string
	// Score is the cosine similarity to the centroid (0 when not computed).
	Score float64
}

// Config holds parameters for a Gate.
type Config struct {
	// Exemplars are on-topic example strings used to build the centroid.
	// At least one is required, unless JudgeOnly is set (the centroid is
	// never computed in judge-only mode, so exemplars are unused).
	Exemplars []string

	// MaxLen is the maximum rune length of an accepted input.
	// Inputs exceeding this get SoftDeny without embedding.
	MaxLen int

	// LowSim is the lower cosine similarity threshold.
	// score < LowSim → SoftDeny (clearly off-topic).
	LowSim float64

	// HighSim is the upper cosine similarity threshold.
	// score >= HighSim → Allow (clearly on-topic).
	// LowSim <= score < HighSim → ambiguous, escalate to Judge.
	HighSim float64

	// JudgeOnly skips the embedding similarity layer entirely: every input
	// that survives the pre-filters (emptiness, length, injection) goes
	// straight to the Judge.
	//
	// COST TRADE-OFF: judge-only means one LLM judge call for EVERY
	// non-prefiltered question. The embedding layer exists precisely to make
	// most decisions free (a cosine score against a precomputed centroid
	// costs one cheap embedding call, and clearly-on/off-topic inputs never
	// reach an LLM). Enable JudgeOnly only when no embedding backend is
	// available.
	//
	// When JudgeOnly is true, Deps.Judge is required; Deps.Embedder and
	// Exemplars become optional and are unused. Decision.Score is always 0.
	JudgeOnly bool
}

// Deps holds external dependencies injected into a Gate.
type Deps struct {
	// Embedder is required — used to embed exemplars and user queries.
	Embedder embeddingclient.Embedder

	// Detector is required — used to pre-screen for injection attacks.
	Detector *injectiondetector.Detector

	// Judge is optional. When set, it resolves ambiguous (mid-band) queries.
	// If nil or if it returns an error, the gate fails closed (SoftDeny).
	Judge llmclient.LLMClient
}

// Gate evaluates whether a request is in-scope.
// The zero value is not usable; use New.
type Gate struct {
	cfg      Config
	deps     Deps
	centroid embeddingclient.Vector
}

// New creates a Gate, precomputing the exemplar centroid.
// It returns an error if exemplars are empty, deps are missing, or embedding
// fails. In JudgeOnly mode the embedding layer is skipped: Deps.Judge becomes
// required, Deps.Embedder and Exemplars optional (no centroid is computed).
func New(cfg Config, deps Deps) (*Gate, error) {
	if deps.Detector == nil {
		return nil, &configError{"Detector is required"}
	}
	if cfg.MaxLen <= 0 {
		cfg.MaxLen = 1000
	}

	if cfg.JudgeOnly {
		if deps.Judge == nil {
			return nil, &configError{"JudgeOnly requires a Judge"}
		}
		return &Gate{cfg: cfg, deps: deps}, nil
	}

	if len(cfg.Exemplars) == 0 {
		return nil, &configError{"at least one exemplar is required"}
	}
	if deps.Embedder == nil {
		return nil, &configError{"Embedder is required"}
	}

	vecs, err := deps.Embedder.EmbedBatch(context.Background(), cfg.Exemplars)
	if err != nil {
		return nil, &configError{"failed to embed exemplars: " + err.Error()}
	}
	centroid := computeCentroid(vecs)

	return &Gate{
		cfg:      cfg,
		deps:     deps,
		centroid: centroid,
	}, nil
}

// Evaluate decides if text is in-scope. It is concurrency-safe.
func (g *Gate) Evaluate(ctx context.Context, text string) Decision {
	// Step a: pre-filter.
	if text == "" {
		return Decision{Verdict: SoftDeny, Reason: "empty input"}
	}
	if utf8.RuneCountInString(text) > g.cfg.MaxLen {
		return Decision{Verdict: SoftDeny, Reason: "input exceeds maximum length"}
	}

	result := g.deps.Detector.Scan(text)
	if result.Flagged {
		return Decision{
			Verdict: HardDeny,
			Reason:  "injection detected: " + string(result.MaxSeverity),
		}
	}

	// Judge-only mode: skip the embedding layer; every surviving input costs
	// one judge LLM call (see Config.JudgeOnly for the cost trade-off).
	if g.cfg.JudgeOnly {
		return g.askJudge(ctx, text, 0)
	}

	// Step b: embedding gate.
	vec, err := g.deps.Embedder.Embed(ctx, text)
	if err != nil {
		// Fail closed on embedding error.
		return Decision{Verdict: SoftDeny, Reason: "embedding error: " + err.Error()}
	}

	score := cosineSimilarity(vec, g.centroid)

	if score >= g.cfg.HighSim {
		return Decision{Verdict: Allow, Reason: "embedding similarity above threshold", Score: score}
	}
	if score < g.cfg.LowSim {
		return Decision{Verdict: SoftDeny, Reason: "embedding similarity below threshold", Score: score}
	}

	// Step c: ambiguous band — escalate to judge.
	return g.askJudge(ctx, text, score)
}

// askJudge sends a yes/no prompt to the LLM judge. Fail-closed on error or absence.
func (g *Gate) askJudge(ctx context.Context, text string, score float64) Decision {
	if g.deps.Judge == nil {
		return Decision{Verdict: SoftDeny, Reason: "ambiguous score, no judge configured (fail-closed)", Score: score}
	}

	prompt := buildJudgePrompt(text)
	resp, err := g.deps.Judge.Complete(ctx, llmclient.Request{
		Prompt:    prompt,
		MaxTokens: 5,
	})
	if err != nil {
		return Decision{Verdict: SoftDeny, Reason: "judge error (fail-closed): " + err.Error(), Score: score}
	}

	answer := strings.ToLower(strings.TrimSpace(resp.Text))
	if strings.HasPrefix(answer, "yes") {
		return Decision{Verdict: Allow, Reason: "judge approved", Score: score}
	}
	return Decision{Verdict: SoftDeny, Reason: "judge denied", Score: score}
}

// buildJudgePrompt constructs a minimal yes/no prompt for the judge.
func buildJudgePrompt(text string) string {
	return "Is the following user request on-topic for this assistant? Reply with only 'yes' or 'no'.\n\nRequest: " + text
}

// computeCentroid returns the mean vector of vecs, then L2-normalises it.
// If vecs is empty or all-zero, it returns a zero vector.
func computeCentroid(vecs []embeddingclient.Vector) embeddingclient.Vector {
	if len(vecs) == 0 {
		return nil
	}
	dim := len(vecs[0])
	centroid := make(embeddingclient.Vector, dim)
	for _, v := range vecs {
		for i, val := range v {
			centroid[i] += val
		}
	}
	n := float64(len(vecs))
	var sumSq float64
	for i := range centroid {
		centroid[i] /= n
		sumSq += centroid[i] * centroid[i]
	}
	if sumSq > 0 {
		norm := math.Sqrt(sumSq)
		for i := range centroid {
			centroid[i] /= norm
		}
	}
	return centroid
}

// cosineSimilarity returns the dot product of two unit-normalised vectors.
// If either vector has zero magnitude, returns 0.
func cosineSimilarity(a, b embeddingclient.Vector) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// configError is a sentinel error type for invalid configuration.
type configError struct{ msg string }

func (e *configError) Error() string { return "topicgate: " + e.msg }
