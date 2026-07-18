package knowledgecardgeneration_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/contracts"
	dp "github.com/geoffmilleraz/agentic-kit/foundations/knowledge/document-parsing"
	kcg "github.com/geoffmilleraz/agentic-kit/foundations/knowledge/knowledge-card-generation"
)

// singleFieldArchetype returns a KnowledgeCardArchetype with one required field.
func singleFieldArchetype(name, fieldName string) contracts.KnowledgeCardArchetype {
	return contracts.KnowledgeCardArchetype{
		Name: name,
		Fields: []contracts.FieldSpec{
			{Name: fieldName, Kind: contracts.FieldKindString, Required: true},
		},
	}
}

// multiFieldArchetype returns a KnowledgeCardArchetype with two fields.
func multiFieldArchetype() contracts.KnowledgeCardArchetype {
	return contracts.KnowledgeCardArchetype{
		Name: "service-capability",
		Fields: []contracts.FieldSpec{
			{Name: "description", Kind: contracts.FieldKindString, Required: true},
			{Name: "slo", Kind: contracts.FieldKindString, Required: false},
		},
	}
}

// TestGenerateTitleUsesFirstHeading verifies that the card title is built from
// the archetype name and the first non-empty heading in the segments.
func TestGenerateTitleUsesFirstHeading(t *testing.T) {
	segs := []dp.Segment{
		{Heading: "", Text: "Preamble", Order: 0},
		{Heading: "Payments Service", Text: "Handles payments.", Order: 1},
	}
	arch := singleFieldArchetype("service-capability", "description")
	card := kcg.Generate(segs, arch, "docs/payments.md")

	want := "service-capability: Payments Service"
	if card.Title != want {
		t.Errorf("Title: want %q, got %q", want, card.Title)
	}
}

// TestGenerateTitleFallsBackToText verifies that when no heading exists, the
// first non-empty text (truncated to 60 chars) is used.
func TestGenerateTitleFallsBackToText(t *testing.T) {
	longText := strings.Repeat("x", 80)
	segs := []dp.Segment{
		{Heading: "", Text: longText, Order: 0},
	}
	arch := singleFieldArchetype("deployment-fact", "config")
	card := kcg.Generate(segs, arch, "cfg.yaml")

	// Title should be "deployment-fact: " + first 60 chars of text
	wantSuffix := strings.Repeat("x", 60)
	if !strings.HasSuffix(card.Title, wantSuffix) {
		t.Errorf("Title should end with 60 'x' chars; got %q", card.Title)
	}
}

// TestGenerateTitleArchetypeNameOnly verifies that an empty segments slice
// sets the title to just the archetype name.
func TestGenerateTitleArchetypeNameOnly(t *testing.T) {
	arch := singleFieldArchetype("operational-runbook", "procedure")
	card := kcg.Generate(nil, arch, "runbook.md")
	if card.Title != "operational-runbook" {
		t.Errorf("Title: want %q, got %q", "operational-runbook", card.Title)
	}
}

// TestGenerateFieldsMatchedByHeading verifies that a field is populated from
// the segment whose heading matches the field name (case-insensitive).
func TestGenerateFieldsMatchedByHeading(t *testing.T) {
	segs := []dp.Segment{
		{Heading: "Description", Text: "A service that processes payments.", Order: 0},
		{Heading: "SLO", Text: "99.9% uptime.", Order: 1},
	}
	arch := multiFieldArchetype()
	card := kcg.Generate(segs, arch, "readme.md")

	if card.Fields["description"] != "A service that processes payments." {
		t.Errorf("Fields[description]: got %q", card.Fields["description"])
	}
	if card.Fields["slo"] != "99.9% uptime." {
		t.Errorf("Fields[slo]: got %q", card.Fields["slo"])
	}
}

// TestGenerateFieldsFallbackToAllText verifies that when no segment heading
// matches a field name, the value is the concatenation of all segment texts.
func TestGenerateFieldsFallbackToAllText(t *testing.T) {
	segs := []dp.Segment{
		{Heading: "Intro", Text: "Part one.", Order: 0},
		{Heading: "Details", Text: "Part two.", Order: 1},
	}
	arch := singleFieldArchetype("service-capability", "summary")
	card := kcg.Generate(segs, arch, "doc.md")

	// No heading matches "summary", so fallback is "Part one. Part two."
	want := "Part one. Part two."
	if card.Fields["summary"] != want {
		t.Errorf("Fields[summary]: want %q, got %q", want, card.Fields["summary"])
	}
}

// TestGenerateFieldsNonNilWhenNoSpecs verifies that Generate returns a non-nil
// Fields map even when the archetype has no fields.
func TestGenerateFieldsNonNilWhenNoSpecs(t *testing.T) {
	arch := contracts.KnowledgeCardArchetype{
		Name:   "empty-archetype",
		Fields: []contracts.FieldSpec{},
	}
	segs := []dp.Segment{{Heading: "Sec", Text: "body", Order: 0}}
	card := kcg.Generate(segs, arch, "doc.md")

	if card.Fields == nil {
		t.Error("Fields must be non-nil even when archetype has no fields")
	}
}

// TestGenerateIsDeterministic verifies that calling Generate twice with the
// same inputs produces identical KnowledgeCard values.
func TestGenerateIsDeterministic(t *testing.T) {
	segs := []dp.Segment{
		{Heading: "Description", Text: "Handles auth.", Order: 0},
		{Heading: "SLO", Text: "99.95% uptime.", Order: 1},
	}
	arch := multiFieldArchetype()

	a := kcg.Generate(segs, arch, "auth.md")
	b := kcg.Generate(segs, arch, "auth.md")

	if a.Title != b.Title || a.SourceID != b.SourceID {
		t.Errorf("non-deterministic output: a=%+v b=%+v", a, b)
	}
	for k := range a.Fields {
		if a.Fields[k] != b.Fields[k] {
			t.Errorf("Fields[%q] differs: %q vs %q", k, a.Fields[k], b.Fields[k])
		}
	}
}

// TestValidatePass verifies that a well-formed card passes Validate.
func TestValidatePass(t *testing.T) {
	card := kcg.KnowledgeCard{
		Title:    "service-capability: Auth",
		Fields:   map[string]string{"description": "Handles auth."},
		SourceID: "auth.md",
	}
	if err := card.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

// TestValidateEmptyTitle verifies that Validate rejects an empty Title.
func TestValidateEmptyTitle(t *testing.T) {
	card := kcg.KnowledgeCard{
		Title:    "",
		Fields:   map[string]string{},
		SourceID: "doc.md",
	}
	if err := card.Validate(); err == nil {
		t.Error("expected error for empty Title, got nil")
	}
}

// TestValidateNilFields verifies that Validate rejects a nil Fields map.
func TestValidateNilFields(t *testing.T) {
	card := kcg.KnowledgeCard{
		Title:    "some title",
		Fields:   nil,
		SourceID: "doc.md",
	}
	if err := card.Validate(); err == nil {
		t.Error("expected error for nil Fields, got nil")
	}
}

// TestValidateEmptySourceID verifies that Validate rejects an empty SourceID.
func TestValidateEmptySourceID(t *testing.T) {
	card := kcg.KnowledgeCard{
		Title:    "some title",
		Fields:   map[string]string{},
		SourceID: "",
	}
	if err := card.Validate(); err == nil {
		t.Error("expected error for empty SourceID, got nil")
	}
}

// TestValidateAgainstArchetypePass verifies that a card with all required
// fields present passes ValidateAgainstArchetype.
func TestValidateAgainstArchetypePass(t *testing.T) {
	arch := singleFieldArchetype("service-capability", "description")
	card := kcg.KnowledgeCard{
		Title:    "service-capability: X",
		Fields:   map[string]string{"description": "Does something."},
		SourceID: "doc.md",
	}
	if err := kcg.ValidateAgainstArchetype(card, arch); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestValidateAgainstArchetypeMissingRequired verifies that missing required
// fields are reported as errors.
func TestValidateAgainstArchetypeMissingRequired(t *testing.T) {
	arch := singleFieldArchetype("service-capability", "description")
	card := kcg.KnowledgeCard{
		Title:    "service-capability: X",
		Fields:   map[string]string{},
		SourceID: "doc.md",
	}
	if err := kcg.ValidateAgainstArchetype(card, arch); err == nil {
		t.Error("expected error for missing required field, got nil")
	}
}

// TestValidateAgainstArchetypeOptionalMayBeAbsent verifies that an optional
// field may be absent without triggering an error.
func TestValidateAgainstArchetypeOptionalMayBeAbsent(t *testing.T) {
	arch := contracts.KnowledgeCardArchetype{
		Name: "service-capability",
		Fields: []contracts.FieldSpec{
			{Name: "description", Kind: contracts.FieldKindString, Required: true},
			{Name: "slo", Kind: contracts.FieldKindString, Required: false},
		},
	}
	card := kcg.KnowledgeCard{
		Title:    "service-capability: Y",
		Fields:   map[string]string{"description": "Does something."},
		SourceID: "doc.md",
	}
	if err := kcg.ValidateAgainstArchetype(card, arch); err != nil {
		t.Errorf("optional field absence should not error: %v", err)
	}
}
