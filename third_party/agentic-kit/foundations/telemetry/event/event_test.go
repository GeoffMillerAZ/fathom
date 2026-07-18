package event_test

import (
	"strings"
	"testing"
	"time"

	ev "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
)

// fixedTime is a stable reference instant used throughout the test suite.
var fixedTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// makeEvent builds a minimal valid event for use in tests.
func makeEvent(kind ev.Kind, source string) ev.Event {
	return ev.Event{
		ID:        "test-id-001",
		Kind:      kind,
		Source:    source,
		Timestamp: fixedTime,
	}
}

// ---- Normalize: basic field canonicalisation --------------------------------

func TestNormalize_LowerCasesSourceAndKind(t *testing.T) {
	e := ev.Event{
		ID:        "id-1",
		Kind:      "TOOL_CALL",
		Source:    "LLMClient/Ladder",
		Timestamp: fixedTime,
	}
	e.Normalize()

	if e.Kind != ev.KindToolCall {
		t.Errorf("Kind not lower-cased: got %q, want %q", e.Kind, ev.KindToolCall)
	}
	if e.Source != "llmclient/ladder" {
		t.Errorf("Source not lower-cased: got %q", e.Source)
	}
}

func TestNormalize_FillsTimestampWhenZero(t *testing.T) {
	before := time.Now().UTC()
	e := ev.Event{Kind: ev.KindInfo, Source: "test", ID: "x"}
	e.Normalize()
	after := time.Now().UTC()

	if e.Timestamp.IsZero() {
		t.Fatal("Timestamp should not be zero after Normalize")
	}
	if e.Timestamp.Before(before) || e.Timestamp.After(after) {
		t.Errorf("Timestamp %v is outside [before=%v, after=%v]", e.Timestamp, before, after)
	}
}

func TestNormalize_DoesNotOverwriteExistingTimestamp(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	e.Normalize()
	if !e.Timestamp.Equal(fixedTime) {
		t.Errorf("Normalize must not overwrite a non-zero Timestamp; got %v", e.Timestamp)
	}
}

func TestNormalize_FillsIDWhenEmpty(t *testing.T) {
	e := ev.Event{
		Kind:      ev.KindInfo,
		Source:    "test/source",
		Timestamp: fixedTime,
	}
	e.Normalize()
	if strings.TrimSpace(e.ID) == "" {
		t.Error("Normalize must fill ID when empty")
	}
	// The default ID must embed the source.
	if !strings.HasPrefix(e.ID, "test/source/") {
		t.Errorf("default ID should start with source; got %q", e.ID)
	}
}

func TestNormalize_DoesNotOverwriteExistingID(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	e.ID = "my-custom-id"
	e.Normalize()
	if e.ID != "my-custom-id" {
		t.Errorf("Normalize must not overwrite a non-empty ID; got %q", e.ID)
	}
}

// ---- Normalize: attribute key canonicalisation ------------------------------

func TestNormalize_AttributeKeysAreLowerSnakeCase(t *testing.T) {
	e := ev.Event{
		ID:        "id-2",
		Kind:      ev.KindToolCall,
		Source:    "test",
		Timestamp: fixedTime,
		Attributes: map[string]string{
			"Tool Name":  "search",
			"Latency_MS": "50",
		},
	}
	e.Normalize()

	if _, ok := e.Attributes["tool_name"]; !ok {
		t.Errorf("expected key 'tool_name' after normalize; got %v", e.Attributes)
	}
	if _, ok := e.Attributes["latency_ms"]; !ok {
		t.Errorf("expected key 'latency_ms' after normalize; got %v", e.Attributes)
	}
}

func TestNormalize_NilAttributesIsValid(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	e.Attributes = nil
	e.Normalize()
	// Must not panic and result remains nil or empty — both are valid.
}

func TestNormalize_EmptyAttributesIsValid(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	e.Attributes = map[string]string{}
	e.Normalize()
	// Must not panic.
}

// ---- Normalize: returns *Event for chaining ---------------------------------

func TestNormalize_ReturnsPointerForChaining(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	got := e.Normalize()
	if got == nil {
		t.Fatal("Normalize must return a non-nil *Event")
	}
}

// ---- Validate: valid event passes -------------------------------------------

func TestValidate_ValidEventPasses(t *testing.T) {
	e := makeEvent(ev.KindLLMCall, "llmclient/ladder")
	e.Normalize()
	if err := e.Validate(); err != nil {
		t.Errorf("valid event should pass Validate; got: %v", err)
	}
}

func TestValidate_AllKindsPass(t *testing.T) {
	for _, k := range []ev.Kind{ev.KindToolCall, ev.KindLLMCall, ev.KindError, ev.KindInfo} {
		e := makeEvent(k, "test")
		e.Normalize()
		if err := e.Validate(); err != nil {
			t.Errorf("Kind %q should pass Validate; got: %v", k, err)
		}
	}
}

// ---- Validate: invalid events fail -----------------------------------------

func TestValidate_EmptyIDFails(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	e.ID = "  "
	if err := e.Validate(); err == nil {
		t.Error("empty ID must fail Validate")
	}
}

func TestValidate_UnknownKindFails(t *testing.T) {
	e := makeEvent("metric", "test")
	if err := e.Validate(); err == nil {
		t.Error("unknown Kind must fail Validate")
	}
}

func TestValidate_EmptySourceFails(t *testing.T) {
	e := makeEvent(ev.KindInfo, "")
	if err := e.Validate(); err == nil {
		t.Error("empty Source must fail Validate")
	}
}

func TestValidate_ZeroTimestampFails(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	e.Timestamp = time.Time{}
	if err := e.Validate(); err == nil {
		t.Error("zero Timestamp must fail Validate")
	}
}

func TestValidate_MultipleErrorsReportedTogether(t *testing.T) {
	e := ev.Event{} // all zero/empty
	err := e.Validate()
	if err == nil {
		t.Fatal("empty Event must fail Validate")
	}
	// Error message should mention more than one failure.
	msg := err.Error()
	if !strings.Contains(msg, "ID") {
		t.Errorf("error should mention ID; got: %s", msg)
	}
	if !strings.Contains(msg, "Kind") {
		t.Errorf("error should mention Kind; got: %s", msg)
	}
	if !strings.Contains(msg, "Source") {
		t.Errorf("error should mention Source; got: %s", msg)
	}
}

// ---- Normalize + Validate: round-trip ----------------------------------------

func TestNormalizeValidate_RoundTrip(t *testing.T) {
	// An event with only the required semantic fields set; Normalize fills rest.
	e := ev.Event{
		Kind:   ev.KindToolCall,
		Source: "Security/Injection-Detector",
		Attributes: map[string]string{
			"Pattern Name": "sql_injection",
		},
	}
	e.Normalize()
	if err := e.Validate(); err != nil {
		t.Errorf("normalised event should be valid; got: %v", err)
	}
	if e.Source != "security/injection-detector" {
		t.Errorf("Source not normalised; got %q", e.Source)
	}
	if _, ok := e.Attributes["pattern_name"]; !ok {
		t.Errorf("Attribute key not normalised; got %v", e.Attributes)
	}
}

// ---- Kind constants are stable ----------------------------------------------

func TestKindConstants(t *testing.T) {
	if ev.KindToolCall != "tool_call" {
		t.Errorf("KindToolCall mismatch: %q", ev.KindToolCall)
	}
	if ev.KindLLMCall != "llm_call" {
		t.Errorf("KindLLMCall mismatch: %q", ev.KindLLMCall)
	}
	if ev.KindError != "error" {
		t.Errorf("KindError mismatch: %q", ev.KindError)
	}
	if ev.KindInfo != "info" {
		t.Errorf("KindInfo mismatch: %q", ev.KindInfo)
	}
}
