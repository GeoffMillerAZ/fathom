// Package aiprovenance defines the authorship record attached to every
// AI- or human-produced finding, decision, or prose node.
//
// CUE is authoritative for data shape (see schema.cue); Go is authoritative
// for behaviour. The two must stay in sync — drift is a bug.
//
// Design goal: embed Author as a required field on evidence spans, decisions,
// and prose nodes so that every claim is traceable to its exact producer. For
// AI authors this means a full reproduction key (model + prompt + input) that
// enables later invalidation or re-running. For human authors it is a minimal
// identity reference.
//
// Concurrency: values are immutable and safe for concurrent use.
package aiprovenance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// AuthorKind discriminates between a human and an AI producer.
// CUE mirror: the Kind discriminator in #Author.
type AuthorKind string

const (
	// KindHuman identifies a human-produced finding.
	KindHuman AuthorKind = "human"
	// KindAI identifies an AI-produced finding.
	KindAI AuthorKind = "ai"
)

// HumanAuthor carries the identity of a human producer.
// CUE mirror: #Author.Human.
type HumanAuthor struct {
	// ID is a non-empty identifier for the human (e.g. email or user UUID).
	ID string
}

// AIAuthor carries the full reproduction record of an AI inference.
// CUE mirror: #Author.AI.
type AIAuthor struct {
	// ModelID is the canonical model identifier (e.g. "claude-opus-4-8").
	ModelID string

	// ModelVersion is the specific version string of the model.
	ModelVersion string

	// PromptHash is the hash of the fully-rendered prompt sent to the model.
	PromptHash string

	// PromptTemplateVersion is the semver-style version of the prompt template.
	// Optional — empty string means not tracked.
	PromptTemplateVersion string

	// ParamsHash is the hash of sampling parameters (temperature, top_p, etc.).
	// Optional — empty string means not tracked.
	ParamsHash string

	// InputDigest is the hash of the evidence or content fed as input.
	InputDigest string

	// At is the timestamp of the inference.
	At time.Time
}

// Author records who or what produced an evidence span, decision, or prose
// node. Exactly one of Human or AI is populated and must match Kind.
//
// CUE mirror: #Author in schema.cue.
type Author struct {
	// Kind discriminates between KindHuman and KindAI.
	Kind AuthorKind

	// Human is populated when Kind == KindHuman.
	Human *HumanAuthor

	// AI is populated when Kind == KindAI.
	AI *AIAuthor
}

// Validate returns an error if the Author violates any invariant:
//   - Kind must be KindHuman or KindAI.
//   - Exactly one of Human/AI must be non-nil and must match Kind.
//   - For KindHuman: Human.ID must be non-empty.
//   - For KindAI: ModelID, ModelVersion, PromptHash, InputDigest must be
//     non-empty; At must be non-zero.
func (a Author) Validate() error {
	switch a.Kind {
	case KindHuman:
		if a.AI != nil {
			return fmt.Errorf("aiprovenance: Kind is %q but AI field is set", a.Kind)
		}
		if a.Human == nil {
			return fmt.Errorf("aiprovenance: Kind is %q but Human field is nil", a.Kind)
		}
		if a.Human.ID == "" {
			return fmt.Errorf("aiprovenance: Human.ID must not be empty")
		}
	case KindAI:
		if a.Human != nil {
			return fmt.Errorf("aiprovenance: Kind is %q but Human field is set", a.Kind)
		}
		if a.AI == nil {
			return fmt.Errorf("aiprovenance: Kind is %q but AI field is nil", a.Kind)
		}
		ai := a.AI
		if ai.ModelID == "" {
			return fmt.Errorf("aiprovenance: AI.ModelID must not be empty")
		}
		if ai.ModelVersion == "" {
			return fmt.Errorf("aiprovenance: AI.ModelVersion must not be empty")
		}
		if ai.PromptHash == "" {
			return fmt.Errorf("aiprovenance: AI.PromptHash must not be empty")
		}
		if ai.InputDigest == "" {
			return fmt.Errorf("aiprovenance: AI.InputDigest must not be empty")
		}
		if ai.At.IsZero() {
			return fmt.Errorf("aiprovenance: AI.At must not be zero")
		}
	default:
		return fmt.Errorf("aiprovenance: unknown Kind %q, must be %q or %q", a.Kind, KindHuman, KindAI)
	}
	return nil
}

// NewHuman constructs an Author with Kind == KindHuman.
// id must be non-empty; Validate will catch a violation if it is not.
func NewHuman(id string) Author {
	return Author{
		Kind:  KindHuman,
		Human: &HumanAuthor{ID: id},
	}
}

// NewAI constructs an Author with Kind == KindAI.
// promptTemplateVersion and paramsHash may be empty (optional fields).
// All other string arguments and at must be non-empty/non-zero; Validate will
// catch violations.
func NewAI(modelID, modelVersion, promptHash, paramsHash, inputDigest string, at time.Time) Author {
	return Author{
		Kind: KindAI,
		AI: &AIAuthor{
			ModelID:      modelID,
			ModelVersion: modelVersion,
			PromptHash:   promptHash,
			ParamsHash:   paramsHash,
			InputDigest:  inputDigest,
			At:           at,
		},
	}
}

// ReproKey returns a stable, deterministic key that identifies the exact
// conditions of this authorship event.
//
// For KindAI it is the lowercase hex SHA-256 of the length-prefixed
// concatenation of fields:
//
//	len(ModelID):ModelID|len(ModelVersion):ModelVersion|len(PromptHash):PromptHash|len(ParamsHash):ParamsHash|len(InputDigest):InputDigest|
//
// Length-prefixing makes the encoding injective: field values containing "|"
// do not cause distinct tuples to collide. The timestamp (At) is intentionally
// excluded so that two inferences made at different times with identical model,
// prompt, and input produce the same key — enabling deduplication and re-run
// detection. The key is purely over reproducible inputs.
//
// For KindHuman it is "human:" + Human.ID.
//
// ReproKey does not validate; callers should call Validate first. If called on
// an invalid Author the key may be empty-ish but will not panic.
func (a Author) ReproKey() string {
	if a.Kind == KindHuman {
		id := ""
		if a.Human != nil {
			id = a.Human.ID
		}
		return "human:" + id
	}
	// KindAI (or unknown kind treated as AI path for safety)
	if a.AI == nil {
		return ""
	}
	ai := a.AI
	var b strings.Builder
	for _, f := range []string{ai.ModelID, ai.ModelVersion, ai.PromptHash, ai.ParamsHash, ai.InputDigest} {
		fmt.Fprintf(&b, "%d:%s|", len(f), f)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// IsAI reports whether this Author was produced by an AI model.
func (a Author) IsAI() bool {
	return a.Kind == KindAI
}

// Clone returns a deep copy of the Author. The returned value does not share
// any pointer with the original — mutating Clone().AI or Clone().Human cannot
// affect the source Author.
//
// Use Clone whenever storing an Author in a slice, map, or any structure that
// may be modified independently of the source. A plain Go value copy (b := a)
// shares the underlying *AIAuthor/*HumanAuthor, which can silently corrupt
// independent evidence spans.
func (a Author) Clone() Author {
	out := Author{Kind: a.Kind}
	if a.AI != nil {
		cp := *a.AI // copy the AIAuthor value
		out.AI = &cp
	}
	if a.Human != nil {
		cp := *a.Human // copy the HumanAuthor value
		out.Human = &cp
	}
	return out
}

// AIOption is a functional option for NewAIWithOpts.
type AIOption func(*AIAuthor)

// WithPromptTemplateVersion sets the PromptTemplateVersion field on an
// AIAuthor. This records the semver-style version of the prompt template that
// shaped what the model saw, enabling later identification of which generation
// logic was active at inference time.
func WithPromptTemplateVersion(v string) AIOption {
	return func(a *AIAuthor) { a.PromptTemplateVersion = v }
}

// WithParamsHash sets the ParamsHash field on an AIAuthor.
func WithParamsHash(h string) AIOption {
	return func(a *AIAuthor) { a.ParamsHash = h }
}

// NewAIWithOpts constructs an Author with Kind == KindAI using functional
// options for optional fields (PromptTemplateVersion, ParamsHash).
//
// Required fields: modelID, modelVersion, promptHash, inputDigest, at.
// Optional fields are set via AIOption values (WithPromptTemplateVersion,
// WithParamsHash). Validate will catch missing required fields.
//
// This constructor is the preferred way to set PromptTemplateVersion — it
// makes the choice deliberate and visible at every call site, rather than
// relying on post-construction field assignment that is easy to overlook.
func NewAIWithOpts(modelID, modelVersion, promptHash, inputDigest string, at time.Time, opts ...AIOption) Author {
	ai := &AIAuthor{
		ModelID:      modelID,
		ModelVersion: modelVersion,
		PromptHash:   promptHash,
		InputDigest:  inputDigest,
		At:           at,
	}
	for _, opt := range opts {
		opt(ai)
	}
	return Author{
		Kind: KindAI,
		AI:   ai,
	}
}
