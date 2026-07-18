package aiprovenance_test

import (
	"strings"
	"testing"
	"time"

	ap "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
)

// ── helpers ──────────────────────────────────────────────────────────────────

var fixedAt = time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

func validHuman() ap.Author {
	return ap.NewHuman("user-001")
}

func validAI() ap.Author {
	return ap.NewAI(
		"claude-opus-4-8",
		"20260608",
		"prompthash-abc123",
		"paramshash-xyz",
		"inputdigest-def456",
		fixedAt,
	)
}

// ── Validate: happy paths ─────────────────────────────────────────────────────

func TestValidateHuman_Valid(t *testing.T) {
	if err := validHuman().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateAI_Valid(t *testing.T) {
	if err := validAI().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// ── Validate: human error cases ──────────────────────────────────────────────

func TestValidateHuman_EmptyID(t *testing.T) {
	a := ap.NewHuman("")
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for empty Human.ID, got nil")
	}
}

func TestValidateHuman_AIFieldSet(t *testing.T) {
	a := validHuman()
	a.AI = validAI().AI
	if err := a.Validate(); err == nil {
		t.Fatal("expected error when Kind=human but AI field is set")
	}
}

func TestValidateHuman_HumanNil(t *testing.T) {
	a := ap.Author{Kind: ap.KindHuman, Human: nil}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error when Kind=human but Human is nil")
	}
}

// ── Validate: AI error cases ──────────────────────────────────────────────────

func TestValidateAI_HumanFieldSet(t *testing.T) {
	a := validAI()
	a.Human = validHuman().Human
	if err := a.Validate(); err == nil {
		t.Fatal("expected error when Kind=ai but Human field is set")
	}
}

func TestValidateAI_AIFieldNil(t *testing.T) {
	a := ap.Author{Kind: ap.KindAI, AI: nil}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error when Kind=ai but AI field is nil")
	}
}

func TestValidateAI_RequiredFields(t *testing.T) {
	cases := []struct {
		name  string
		mutFn func(*ap.Author)
	}{
		{"empty-ModelID", func(a *ap.Author) { a.AI.ModelID = "" }},
		{"empty-ModelVersion", func(a *ap.Author) { a.AI.ModelVersion = "" }},
		{"empty-PromptHash", func(a *ap.Author) { a.AI.PromptHash = "" }},
		{"empty-InputDigest", func(a *ap.Author) { a.AI.InputDigest = "" }},
		{"zero-At", func(a *ap.Author) { a.AI.At = time.Time{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := validAI()
			tc.mutFn(&a)
			if err := a.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

func TestValidateAI_OptionalFieldsAllowEmpty(t *testing.T) {
	a := validAI()
	a.AI.PromptTemplateVersion = ""
	a.AI.ParamsHash = ""
	if err := a.Validate(); err != nil {
		t.Fatalf("empty optional fields should be allowed, got: %v", err)
	}
}

// ── Validate: unknown Kind ────────────────────────────────────────────────────

func TestValidate_UnknownKind(t *testing.T) {
	a := ap.Author{Kind: "robot"}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for unknown Kind, got nil")
	}
}

// ── NewHuman / NewAI constructors ─────────────────────────────────────────────

func TestNewHuman_RoundTrip(t *testing.T) {
	a := ap.NewHuman("alice@example.com")
	if err := a.Validate(); err != nil {
		t.Fatalf("NewHuman produced invalid Author: %v", err)
	}
	if a.Kind != ap.KindHuman {
		t.Errorf("expected Kind %q, got %q", ap.KindHuman, a.Kind)
	}
	if a.Human == nil || a.Human.ID != "alice@example.com" {
		t.Errorf("unexpected Human value: %+v", a.Human)
	}
	if a.AI != nil {
		t.Errorf("AI field should be nil for human author")
	}
}

func TestNewAI_RoundTrip(t *testing.T) {
	a := ap.NewAI("m", "v", "ph", "parh", "id", fixedAt)
	if err := a.Validate(); err != nil {
		t.Fatalf("NewAI produced invalid Author: %v", err)
	}
	if a.Kind != ap.KindAI {
		t.Errorf("expected Kind %q, got %q", ap.KindAI, a.Kind)
	}
	if a.Human != nil {
		t.Errorf("Human field should be nil for AI author")
	}
	if a.AI == nil {
		t.Fatal("AI field must not be nil")
	}
	if a.AI.ModelID != "m" {
		t.Errorf("ModelID mismatch: %q", a.AI.ModelID)
	}
}

// ── ReproKey ──────────────────────────────────────────────────────────────────

func TestReproKey_HumanPrefix(t *testing.T) {
	key := ap.NewHuman("bob@example.com").ReproKey()
	if !strings.HasPrefix(key, "human:") {
		t.Errorf("human ReproKey should start with 'human:', got %q", key)
	}
	if key != "human:bob@example.com" {
		t.Errorf("unexpected human key: %q", key)
	}
}

func TestReproKey_AIIsDeterministic(t *testing.T) {
	a1 := ap.NewAI("model", "v1", "ph1", "parh1", "id1", fixedAt)
	a2 := ap.NewAI("model", "v1", "ph1", "parh1", "id1", fixedAt.Add(24*time.Hour)) // different At
	if a1.ReproKey() != a2.ReproKey() {
		t.Errorf("ReproKey must be identical regardless of At timestamp:\n  a1=%q\n  a2=%q", a1.ReproKey(), a2.ReproKey())
	}
}

func TestReproKey_AIChangesOnInputChange(t *testing.T) {
	base := ap.NewAI("model", "v1", "ph1", "parh1", "id1", fixedAt)
	diff := ap.NewAI("model", "v1", "ph1", "parh1", "id2", fixedAt) // different InputDigest
	if base.ReproKey() == diff.ReproKey() {
		t.Error("ReproKey must differ when InputDigest changes")
	}
}

func TestReproKey_AIChangesOnPromptChange(t *testing.T) {
	base := ap.NewAI("model", "v1", "ph1", "parh1", "id1", fixedAt)
	diff := ap.NewAI("model", "v1", "ph2", "parh1", "id1", fixedAt) // different PromptHash
	if base.ReproKey() == diff.ReproKey() {
		t.Error("ReproKey must differ when PromptHash changes")
	}
}

func TestReproKey_AIIsHex64Chars(t *testing.T) {
	key := validAI().ReproKey()
	if len(key) != 64 {
		t.Errorf("AI ReproKey should be 64 hex chars (SHA-256), got %d: %q", len(key), key)
	}
	for _, ch := range key {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("AI ReproKey contains non-hex char %q: %q", ch, key)
			break
		}
	}
}

// ── IsAI ──────────────────────────────────────────────────────────────────────

func TestIsAI(t *testing.T) {
	if validHuman().IsAI() {
		t.Error("human author: IsAI should return false")
	}
	if !validAI().IsAI() {
		t.Error("AI author: IsAI should return true")
	}
}

// ── FIX B1 regression: injective ReproKey (pipe-collision) ───────────────────

// TestReproKey_PipeCollision verifies that field tuples containing "|" do not
// collide. Before FIX B1, ModelID="a|b",ModelVersion="c" and
// ModelID="a",ModelVersion="b|c" produced identical keys.
func TestReproKey_PipeCollision(t *testing.T) {
	// These two Authors differ only in how "|" falls within the fields.
	// With naive "|"-join they collapse to the same raw string "a|b|c|ph|parh|id"
	// and "a|b|c|ph|parh|id" — both become "a|b|c|ph|parh|id".
	a1 := ap.NewAI("a|b", "c", "ph", "parh", "id", fixedAt)
	a2 := ap.NewAI("a", "b|c", "ph", "parh", "id", fixedAt)

	if a1.ReproKey() == a2.ReproKey() {
		t.Errorf("ReproKey collision: distinct field tuples with embedded '|' produced same key %q", a1.ReproKey())
	}
}

// TestReproKey_IdenticalInputsSameKey verifies that identical field values
// still produce the same key after FIX B1 (determinism preserved).
func TestReproKey_IdenticalInputsSameKey(t *testing.T) {
	a1 := ap.NewAI("model-x", "v2", "phA", "parhA", "idA", fixedAt)
	a2 := ap.NewAI("model-x", "v2", "phA", "parhA", "idA", fixedAt.Add(time.Hour))
	if a1.ReproKey() != a2.ReproKey() {
		t.Errorf("identical inputs must produce same ReproKey:\n  a1=%q\n  a2=%q", a1.ReproKey(), a2.ReproKey())
	}
}

// ── Clone / alias safety (Finding 2) ─────────────────────────────────────────

// TestClone_AINoAlias confirms that two Author values produced by Clone() do
// not share their AIAuthor pointee — mutating one must not affect the other.
func TestClone_AINoAlias(t *testing.T) {
	orig := validAI()
	clone := orig.Clone()

	// Mutate the clone's AI sub-struct.
	clone.AI.ModelID = "tampered"

	if orig.AI.ModelID == "tampered" {
		t.Fatal("Clone: mutating clone.AI.ModelID corrupted orig.AI.ModelID — pointer aliasing detected")
	}
}

// TestClone_HumanNoAlias confirms Clone independence for human authors.
func TestClone_HumanNoAlias(t *testing.T) {
	orig := ap.NewHuman("original-user")
	clone := orig.Clone()

	clone.Human.ID = "tampered"

	if orig.Human.ID == "tampered" {
		t.Fatal("Clone: mutating clone.Human.ID corrupted orig.Human.ID — pointer aliasing detected")
	}
}

// TestClone_AICopiedByValue confirms that a plain value copy (b := a) DOES
// alias (baseline for the language), and Clone() is the required remedy.
func TestClone_AICopiedByValue(t *testing.T) {
	orig := validAI()
	// Plain value copy shares the pointer.
	valueCopy := orig
	valueCopy.AI.ModelID = "shared"
	if orig.AI.ModelID != "shared" {
		t.Skip("language semantics changed — plain copy no longer aliases; re-evaluate Clone necessity")
	}
	// This is the known alias hazard — Clone() must fix it.
	fixed := validAI()
	safeClone := fixed.Clone()
	safeClone.AI.ModelID = "safe"
	if fixed.AI.ModelID == "safe" {
		t.Fatal("Clone did not prevent aliasing")
	}
}

// ── NewAIWithOpts / PromptTemplateVersion (Finding 3) ────────────────────────

// TestNewAIWithOpts_PromptTemplateVersion verifies that the functional-options
// constructor surfaces PromptTemplateVersion without requiring post-construction
// field assignment.
func TestNewAIWithOpts_PromptTemplateVersion(t *testing.T) {
	a := ap.NewAIWithOpts(
		"claude-opus-4-8",
		"20260608",
		"prompthash-abc",
		"inputdigest-xyz",
		fixedAt,
		ap.WithParamsHash("paramshash-123"),
		ap.WithPromptTemplateVersion("1.2.0"),
	)
	if err := a.Validate(); err != nil {
		t.Fatalf("NewAIWithOpts produced invalid Author: %v", err)
	}
	if a.AI.PromptTemplateVersion != "1.2.0" {
		t.Errorf("PromptTemplateVersion: got %q, want %q", a.AI.PromptTemplateVersion, "1.2.0")
	}
	if a.AI.ParamsHash != "paramshash-123" {
		t.Errorf("ParamsHash: got %q, want %q", a.AI.ParamsHash, "paramshash-123")
	}
}

// TestNewAIWithOpts_NoOpts confirms NewAIWithOpts works with zero options
// (optional fields default to empty, which is allowed by Validate).
func TestNewAIWithOpts_NoOpts(t *testing.T) {
	a := ap.NewAIWithOpts("m", "v", "ph", "id", fixedAt)
	if err := a.Validate(); err != nil {
		t.Fatalf("NewAIWithOpts with no opts should be valid: %v", err)
	}
	if a.AI.PromptTemplateVersion != "" {
		t.Errorf("expected empty PromptTemplateVersion, got %q", a.AI.PromptTemplateVersion)
	}
}
