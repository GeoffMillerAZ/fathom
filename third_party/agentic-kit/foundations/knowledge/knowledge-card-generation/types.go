// Package knowledgecardgeneration generates typed, governed knowledge cards
// from parsed document segments and a knowledge-card archetype.
//
// A KnowledgeCard is the atomic unit of governed knowledge in the platform.
// It carries a title derived from the archetype name and the first non-empty
// segment heading, a map of extracted fields, and the source document ID.
//
// This package has no third-party dependencies; it imports only the stdlib
// and the contracts foundation for the KnowledgeCardArchetype type.
//
// Concurrency contract: Generate is a pure function with no shared mutable
// state; it is safe to call concurrently from multiple goroutines. KnowledgeCard
// is a plain value type safe to read concurrently.
package knowledgecardgeneration

import (
	"errors"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/agentic-kit/foundations/contracts"
	dp "github.com/geoffmilleraz/agentic-kit/foundations/knowledge/document-parsing"
)

// KnowledgeCard is a typed, provenance-tagged unit of knowledge derived from
// one or more document segments.
//
// CUE mirror: #KnowledgeCard in schema.cue.
type KnowledgeCard struct {
	// Title is a human-readable summary formed from the archetype name and
	// the first non-empty heading found in the segments.
	Title string

	// Fields holds the extracted key/value pairs for this card.
	// Keys are the names of the FieldSpec entries from the archetype.
	// Values are extracted from the segment text that corresponds to each field.
	Fields map[string]string

	// SourceID identifies the source document from which this card was derived.
	// It is passed through from the Generate call unchanged.
	SourceID string
}

// Validate checks that the KnowledgeCard satisfies its structural invariants.
// It returns a non-nil error if:
//   - Title is empty
//   - Fields is nil
//   - SourceID is empty
func (k KnowledgeCard) Validate() error {
	var errs []string

	if strings.TrimSpace(k.Title) == "" {
		errs = append(errs, "Title must be non-empty")
	}
	if k.Fields == nil {
		errs = append(errs, "Fields must be non-nil")
	}
	if strings.TrimSpace(k.SourceID) == "" {
		errs = append(errs, "SourceID must be non-empty")
	}

	if len(errs) > 0 {
		return fmt.Errorf("KnowledgeCard: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Generate produces a KnowledgeCard deterministically from segments, an
// archetype, and a source document identifier.
//
// Template-fill algorithm:
//  1. The card Title is "<archetype.Name>: <first non-empty segment heading>".
//     If no non-empty heading exists, the first non-empty segment Text is used
//     (truncated to 60 characters). If segments is empty, Title is archetype.Name.
//  2. Fields is populated by iterating over archetype.Fields in order.
//     For each FieldSpec, the value is the Text of the first segment whose
//     Heading case-folds to the field Name. If no such segment exists, the
//     value is the concatenation of all segment texts (trimmed, joined with " ").
//  3. SourceID is set to sourceID unchanged.
//
// Generate is deterministic: same inputs → same output.
// If archetype.Fields is empty, Fields will be an empty (non-nil) map.
func Generate(segments []dp.Segment, archetype contracts.KnowledgeCardArchetype, sourceID string) KnowledgeCard {
	title := buildTitle(segments, archetype.Name)
	fields := buildFields(segments, archetype.Fields)

	return KnowledgeCard{
		Title:    title,
		Fields:   fields,
		SourceID: sourceID,
	}
}

// buildTitle constructs the card title.
func buildTitle(segments []dp.Segment, archetypeName string) string {
	for _, s := range segments {
		if strings.TrimSpace(s.Heading) != "" {
			return archetypeName + ": " + strings.TrimSpace(s.Heading)
		}
	}
	// No heading found: fall back to first non-empty text snippet.
	for _, s := range segments {
		if t := strings.TrimSpace(s.Text); t != "" {
			if len(t) > 60 {
				t = t[:60]
			}
			return archetypeName + ": " + t
		}
	}
	return archetypeName
}

// buildFields maps each FieldSpec name to the best-matching segment text.
func buildFields(segments []dp.Segment, specs []contracts.FieldSpec) map[string]string {
	fields := make(map[string]string, len(specs))

	// Pre-compute a joined fallback from all segment texts.
	var parts []string
	for _, s := range segments {
		if t := strings.TrimSpace(s.Text); t != "" {
			parts = append(parts, t)
		}
	}
	fallback := strings.Join(parts, " ")

	for _, spec := range specs {
		value := ""
		for _, s := range segments {
			if strings.EqualFold(strings.TrimSpace(s.Heading), spec.Name) {
				value = strings.TrimSpace(s.Text)
				break
			}
		}
		if value == "" {
			value = fallback
		}
		fields[spec.Name] = value
	}

	return fields
}

// ValidateAgainstArchetype checks that every required FieldSpec in archetype
// is present and non-empty in the card's Fields map.
// It returns a non-nil error listing all violations.
func ValidateAgainstArchetype(card KnowledgeCard, archetype contracts.KnowledgeCardArchetype) error {
	if err := card.Validate(); err != nil {
		return err
	}

	var errs []string
	for _, spec := range archetype.Fields {
		if !spec.Required {
			continue
		}
		v, ok := card.Fields[spec.Name]
		if !ok || strings.TrimSpace(v) == "" {
			errs = append(errs, fmt.Sprintf("required field %q is missing or empty", spec.Name))
		}
	}

	if len(errs) > 0 {
		return errors.New("KnowledgeCard archetype validation: " + strings.Join(errs, "; "))
	}
	return nil
}
