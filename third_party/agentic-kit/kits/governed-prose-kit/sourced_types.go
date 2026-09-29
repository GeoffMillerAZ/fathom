package governedprosekit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// ErrEvidence identifies missing, denied, expired, revoked, stale or corrupt evidence.
var ErrEvidence = errors.New("governedprosekit: evidence unavailable or invalid")

// ErrUnsupported identifies a claim or classification that cannot be established.
var ErrUnsupported = errors.New("governedprosekit: support not established")

// ErrArtifact identifies a malformed, changed or invalidated artifact.
var ErrArtifact = errors.New("governedprosekit: invalid artifact")

// SourceRef identifies immutable bytes at an exact source revision.
type SourceRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

// SourceSnapshot is an authorized source revision. Content is canonical UTF-8
// text; adapters extracting PDF/cells must retain their original provenance and
// expose the extracted text as a separate snapshot, never reinterpret byte offsets.
type SourceSnapshot struct {
	Ref         SourceRef `json:"ref"`
	Content     string    `json:"content"`
	MediaType   string    `json:"media_type"`
	AcquiredBy  string    `json:"acquired_by"`
	AcquiredAt  time.Time `json:"acquired_at"`
	Access      string    `json:"access"` // public or restricted
	RetainUntil time.Time `json:"retain_until"`
	Revoked     bool      `json:"revoked"`
}

// SourceProvider resolves authorized snapshots and current revision pointers.
// Implementations MUST authenticate principal, enforce access/retention, and return
// errors rather than an empty successful result. Resolve must never substitute a
// different version. Current lets strict paths detect drift independently of Resolve.
// Calls are observations, not a distributed transaction with subsequent publication.
type SourceProvider interface {
	Resolve(context.Context, string, SourceRef) (SourceSnapshot, error)
	Current(context.Context, string, string) (SourceRef, error)
}

// Derivation specifies exact rational arithmetic over prior facts. Version is
// arithmetic/v1; Operation is sum, difference or ratio. Values use base-10 rational
// notation; the result must be exact (no silent rounding). Units must match; ratio
// yields the unit "ratio". The caller owns the semantic meaning of the calculation.
type Derivation struct {
	Version   string   `json:"version"`
	Operation string   `json:"operation"`
	Inputs    []string `json:"inputs"`
}

// Fact separates an extracted assertion from generated wording. Base facts use
// canonical Span provenance and an exact byte-range Locator (inclusive End, as in
// evidence-span). Derived facts retain a provenance Span and name earlier inputs;
// their source/locator is inherited through those inputs, not fabricated.
type Fact struct {
	Span       evidencespan.Span `json:"span"`
	Source     SourceRef         `json:"source"`
	Unit       string            `json:"unit"`
	Historical bool              `json:"historical"`
	Derivation *Derivation       `json:"derivation,omitempty"`
}

// ProseClaim is the exact assertion being checked, with selected supporting and
// contradicting fact IDs. Contradictions are disclosed to the independent reviewer.
type ProseClaim struct {
	ID             string   `json:"id"`
	Assertion      string   `json:"assertion"`
	FactIDs        []string `json:"fact_ids"`
	Contradictions []string `json:"contradictions"`
}

// ClaimRange links a claim to UTF-8 byte offsets [Start, End) in a prose unit.
// Boundaries must fall between complete runes; the selected text equals Assertion.
type ClaimRange struct {
	ClaimID string `json:"claim_id"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
}

// ProseUnit is factual by default. Every non-whitespace byte must be covered by
// a claim. NonFactualReason is only a request for independent classification review;
// it is never a model-controlled exemption from the publication gate.
type ProseUnit struct {
	ID               string       `json:"id"`
	Text             string       `json:"text"`
	Claims           []ClaimRange `json:"claims"`
	NonFactualReason string       `json:"non_factual_reason,omitempty"`
}

// ProseDraft is the bounded structured response expected from a generator.
type ProseDraft struct {
	Claims []ProseClaim `json:"claims"`
	Units  []ProseUnit  `json:"units"`
}

// SupportDecision reports support, not source truth. Status is supported,
// unsupported or inconclusive. Reviewer identifies an authenticated human or a
// configured evaluator, not an identity asserted by the generated draft.
type SupportDecision struct {
	Status   string `json:"status"`
	Reviewer string `json:"reviewer"`
	Method   string `json:"method"`
	Reason   string `json:"reason"`
}

// SupportReview contains the exact draft context and resolved evidence. Facts
// includes the full authorized set so a reviewer can detect omitted contradictions.
// For classification Claim is nil and Unit.NonFactualReason must be reviewed.
type SupportReview struct {
	Claim *ProseClaim
	Unit  ProseUnit
	Facts []Fact
}

// SupportReviewer is the consumer-owned semantic/classification policy boundary.
// It must not trust model-authored status/reviewer fields. A model judge is heuristic,
// not a guarantee of entailment. Errors and non-supported outcomes block export.
type SupportReviewer interface {
	Review(context.Context, SupportReview) (SupportDecision, error)
}

// SourcedRequest provides trusted input facts and the authenticated principal.
// Historical evidence is explicit per fact; other facts must remain current.
type SourcedRequest struct {
	Principal string `json:"principal"`
	Topic     string `json:"topic"`
	Facts     []Fact `json:"facts"`
}

// ProseLimits bounds work and storage per operation, including complete JSON
// payloads. Client-side transport bounds remain the injected client's responsibility.
type ProseLimits struct {
	MaxFacts  int `json:"max_facts"`
	MaxUnits  int `json:"max_units"`
	MaxClaims int `json:"max_claims"`
	MaxBytes  int `json:"max_bytes"`
	MaxTokens int `json:"max_tokens"`
}

// DefaultProseLimits supplies a small document profile, not an unbounded default.
func DefaultProseLimits() ProseLimits {
	return ProseLimits{MaxFacts: 256, MaxUnits: 256, MaxClaims: 512, MaxBytes: 1 << 20, MaxTokens: 8192}
}

// SourcedConfig versions policy, generation template and model configuration.
// All fields are required. Model is a caller-supplied identity, not a hardcoded ID.
type SourcedConfig struct {
	PolicyVersion string      `json:"policy_version"`
	PromptVersion string      `json:"prompt_version"`
	Model         string      `json:"model"`
	Limits        ProseLimits `json:"limits"`
	// AllowRestrictedGeneration explicitly allows restricted source facts to
	// reach this configured model. False refuses before the generation call.
	AllowRestrictedGeneration bool `json:"allow_restricted_generation"`
}

// ProseReceipt binds checks to an exact artifact, evidence and generation request.
// Support is separate from Integrity; neither field establishes source truth or
// authorization to publish. Receipts are not signatures or bearer credentials.
type ProseReceipt struct {
	Version        string                     `json:"version"`
	ArtifactDigest string                     `json:"artifact_digest"`
	EvidenceDigest string                     `json:"evidence_digest"`
	PromptDigest   string                     `json:"prompt_digest"`
	ConfigDigest   string                     `json:"config_digest"`
	Model          string                     `json:"model"`
	Generation     string                     `json:"generation"`
	CheckedAt      time.Time                  `json:"checked_at"`
	Integrity      string                     `json:"integrity"`
	Support        map[string]SupportDecision `json:"support"`
}

// SourcedArtifact retains inputs, structured wording and an integrity receipt.
// Mutating anything requires rechecking; ApprovalRef is cleared on invalidation.
// The approval reference is informational: Export authorizes through its callback.
type SourcedArtifact struct {
	Request      SourcedRequest `json:"request"`
	Draft        ProseDraft     `json:"draft"`
	Receipt      ProseReceipt   `json:"receipt"`
	InvalidFacts []string       `json:"invalid_facts"`
	InvalidUnits []string       `json:"invalid_units"`
	ApprovalRef  string         `json:"approval_ref,omitempty"`
}

// PublicationAuthorizer checks current authorization for a checked receipt and
// destination audience. Implementations bind actor, artifact/evidence digests and
// expiry (e.g. kits/hitl). Nil is never implicit authorization.
type PublicationAuthorizer func(context.Context, string, ProseReceipt, string) error

// SourceMapEntry ties rendered Markdown bytes to a unit, claims and their exact
// facts. It includes only evidence used by that unit, never the whole bibliography.
type SourceMapEntry struct {
	UnitID string       `json:"unit_id"`
	Start  int          `json:"start"`
	End    int          `json:"end"`
	Claims []ProseClaim `json:"claims"`
	Facts  []Fact       `json:"facts"`
	Ranges []ClaimRange `json:"ranges"`
}

// ProseExport is readable Markdown and its inspectable, content-bound sidecar.
// Redacted units contain no original text, facts, source IDs, or review rationale.
// Any restricted input taints the complete public export, including generated IDs.
type ProseExport struct {
	Markdown       string           `json:"markdown"`
	MarkdownDigest string           `json:"markdown_digest"`
	SidecarDigest  string           `json:"sidecar_digest"`
	ArtifactDigest string           `json:"artifact_digest"`
	EvidenceDigest string           `json:"evidence_digest"`
	Audience       string           `json:"audience"`
	SourceMap      []SourceMapEntry `json:"source_map"`
	RedactedUnits  int              `json:"redacted_units"`
}

// SourcedPipeline is the strict generate/check/export path. It retains no run
// state; concurrent calls are safe only if injected dependencies and Clock are safe.
// Unlike legacy Pipeline, it does not treat a bibliography as claim support.
type SourcedPipeline struct {
	client   llmclient.LLMClient
	sources  SourceProvider
	reviewer SupportReviewer
	config   SourcedConfig
	now      func() time.Time
}

// ContentDigest returns the canonical lowercase SHA-256 digest of exact bytes.
func ContentDigest(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
