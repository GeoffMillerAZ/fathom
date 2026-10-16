package event_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	ev "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/event"
)

// fixedRandHex returns a deterministic randHex source: nBytes is honoured
// (2*nBytes hex chars) and successive calls cycle a counter so IDs differ.
func fixedRandHex() func(nBytes int) string {
	calls := 0
	return func(nBytes int) string {
		calls++
		return strings.Repeat(fmt.Sprintf("%02x", calls), nBytes)
	}
}

// isLowerHex reports whether s consists solely of [0-9a-f].
func isLowerHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// ---- ID generation -----------------------------------------------------------

func TestNewRunID_DefaultCryptoRand(t *testing.T) {
	id := ev.NewRunID(nil)
	if len(id) != 32 {
		t.Errorf("RunID must be 32 hex chars (16 bytes); got %d: %q", len(id), id)
	}
	if !isLowerHex(id) {
		t.Errorf("RunID must be lowercase hex; got %q", id)
	}
	if other := ev.NewRunID(nil); other == id {
		t.Errorf("two crypto RunIDs must differ; both %q", id)
	}
}

func TestNewSpanID_DefaultCryptoRand(t *testing.T) {
	id := ev.NewSpanID(nil)
	if len(id) != 16 {
		t.Errorf("SpanID must be 16 hex chars (8 bytes); got %d: %q", len(id), id)
	}
	if !isLowerHex(id) {
		t.Errorf("SpanID must be lowercase hex; got %q", id)
	}
	if other := ev.NewSpanID(nil); other == id {
		t.Errorf("two crypto SpanIDs must differ; both %q", id)
	}
}

func TestNewRunID_InjectedRandomnessIsDeterministic(t *testing.T) {
	if got := ev.NewRunID(fixedRandHex()); got != strings.Repeat("01", 16) {
		t.Errorf("injected randHex not honoured; got %q", got)
	}
	if got := ev.NewSpanID(fixedRandHex()); got != strings.Repeat("01", 8) {
		t.Errorf("injected randHex not honoured; got %q", got)
	}
}

// ---- NewTrace / Child ----------------------------------------------------------

func TestNewTrace_RootHasNoParent(t *testing.T) {
	tc := ev.NewTrace(fixedRandHex())
	if tc.RunID == "" || tc.SpanID == "" {
		t.Fatalf("NewTrace must fill RunID and SpanID; got %+v", tc)
	}
	if tc.ParentSpanID != "" {
		t.Errorf("root trace must have empty ParentSpanID; got %q", tc.ParentSpanID)
	}
}

func TestChild_DerivesCorrectly(t *testing.T) {
	rh := fixedRandHex()
	parent := ev.NewTrace(rh)
	child := parent.Child(rh)

	if child.RunID != parent.RunID {
		t.Errorf("child must share RunID: parent %q, child %q", parent.RunID, child.RunID)
	}
	if child.ParentSpanID != parent.SpanID {
		t.Errorf("child.ParentSpanID must equal parent.SpanID: got %q, want %q", child.ParentSpanID, parent.SpanID)
	}
	if child.SpanID == parent.SpanID {
		t.Errorf("child must get a fresh SpanID; both %q", child.SpanID)
	}
	// Parent must be unchanged (value semantics).
	if parent.ParentSpanID != "" {
		t.Errorf("Child must not mutate the receiver; parent.ParentSpanID = %q", parent.ParentSpanID)
	}
}

func TestChild_GrandchildChain(t *testing.T) {
	rh := fixedRandHex()
	root := ev.NewTrace(rh)
	child := root.Child(rh)
	grandchild := child.Child(rh)

	if grandchild.RunID != root.RunID {
		t.Errorf("grandchild must share root RunID; got %q, want %q", grandchild.RunID, root.RunID)
	}
	if grandchild.ParentSpanID != child.SpanID {
		t.Errorf("grandchild parent must be child span; got %q, want %q", grandchild.ParentSpanID, child.SpanID)
	}
}

// ---- IsZero -------------------------------------------------------------------

func TestIsZero(t *testing.T) {
	tests := []struct {
		name string
		tc   ev.TraceContext
		want bool
	}{
		{name: "zero value", tc: ev.TraceContext{}, want: true},
		{name: "run only", tc: ev.TraceContext{RunID: "r"}, want: false},
		{name: "span only", tc: ev.TraceContext{SpanID: "s"}, want: false},
		{name: "parent only", tc: ev.TraceContext{ParentSpanID: "p"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tc.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---- Context carry --------------------------------------------------------------

func TestWithTrace_TraceFrom_RoundTrip(t *testing.T) {
	tc := ev.TraceContext{RunID: "run-1", SpanID: "span-1", ParentSpanID: "span-0"}
	ctx := ev.WithTrace(context.Background(), tc)

	got, ok := ev.TraceFrom(ctx)
	if !ok {
		t.Fatal("TraceFrom must find the attached TraceContext")
	}
	if got != tc {
		t.Errorf("round trip mismatch: got %+v, want %+v", got, tc)
	}
}

func TestTraceFrom_AbsentReturnsNotOK(t *testing.T) {
	got, ok := ev.TraceFrom(context.Background())
	if ok {
		t.Error("TraceFrom on a bare context must return ok=false")
	}
	if !got.IsZero() {
		t.Errorf("absent trace must return zero value; got %+v", got)
	}
}

func TestWithTrace_LaterAttachReplaces(t *testing.T) {
	ctx := ev.WithTrace(context.Background(), ev.TraceContext{RunID: "old"})
	ctx = ev.WithTrace(ctx, ev.TraceContext{RunID: "new"})

	got, ok := ev.TraceFrom(ctx)
	if !ok || got.RunID != "new" {
		t.Errorf("later WithTrace must replace; got %+v ok=%v", got, ok)
	}
}

// ---- Apply --------------------------------------------------------------------

func TestApply_StampsAllReservedKeys(t *testing.T) {
	e := makeEvent(ev.KindToolCall, "test")
	tc := ev.TraceContext{RunID: "run-1", SpanID: "span-2", ParentSpanID: "span-1"}
	ev.Apply(tc, &e)

	want := map[string]string{
		ev.AttrRunID:        "run-1",
		ev.AttrSpanID:       "span-2",
		ev.AttrParentSpanID: "span-1",
	}
	for k, v := range want {
		if got := e.Attributes[k]; got != v {
			t.Errorf("attribute %q = %q, want %q", k, got, v)
		}
	}
}

func TestApply_AllocatesNilAttributes(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	e.Attributes = nil
	ev.Apply(ev.TraceContext{RunID: "run-1", SpanID: "span-1"}, &e)
	if e.Attributes == nil {
		t.Fatal("Apply must allocate a nil Attributes map")
	}
	if e.Attributes[ev.AttrRunID] != "run-1" {
		t.Errorf("run id not stamped; got %v", e.Attributes)
	}
}

func TestApply_SkipsEmptyFields(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	ev.Apply(ev.TraceContext{RunID: "run-1", SpanID: "span-1"}, &e) // root: no parent

	if _, ok := e.Attributes[ev.AttrParentSpanID]; ok {
		t.Errorf("empty ParentSpanID must not be stamped; got %v", e.Attributes)
	}
}

func TestApply_ZeroTraceIsNoop(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	e.Attributes = nil
	ev.Apply(ev.TraceContext{}, &e)
	if e.Attributes != nil {
		t.Errorf("zero TraceContext must not touch the event; got %v", e.Attributes)
	}
}

func TestApply_NilEventIsNoop(t *testing.T) {
	// Must not panic.
	ev.Apply(ev.TraceContext{RunID: "run-1"}, nil)
}

func TestApply_PreservesExistingAttributesAndOverwritesReserved(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	e.Attributes = map[string]string{
		"tool_name":  "search",
		ev.AttrRunID: "stale-run",
	}
	ev.Apply(ev.TraceContext{RunID: "run-9", SpanID: "span-9"}, &e)

	if e.Attributes["tool_name"] != "search" {
		t.Errorf("unrelated attribute lost; got %v", e.Attributes)
	}
	if e.Attributes[ev.AttrRunID] != "run-9" {
		t.Errorf("reserved key must be overwritten by Apply; got %q", e.Attributes[ev.AttrRunID])
	}
}

func TestApply_KeysSurviveNormalize(t *testing.T) {
	e := makeEvent(ev.KindInfo, "test")
	ev.Apply(ev.TraceContext{RunID: "run-1", SpanID: "span-1", ParentSpanID: "span-0"}, &e)
	e.Normalize()

	for _, k := range []string{ev.AttrRunID, ev.AttrSpanID, ev.AttrParentSpanID} {
		if _, ok := e.Attributes[k]; !ok {
			t.Errorf("reserved key %q lost after Normalize; got %v", k, e.Attributes)
		}
	}
	if err := e.Validate(); err != nil {
		t.Errorf("trace-stamped event must remain valid; got %v", err)
	}
}

// ---- Reserved key constants are stable ------------------------------------------

func TestReservedAttributeKeyConstants(t *testing.T) {
	if ev.AttrRunID != "trace.run_id" {
		t.Errorf("AttrRunID mismatch: %q", ev.AttrRunID)
	}
	if ev.AttrSpanID != "trace.span_id" {
		t.Errorf("AttrSpanID mismatch: %q", ev.AttrSpanID)
	}
	if ev.AttrParentSpanID != "trace.parent_span_id" {
		t.Errorf("AttrParentSpanID mismatch: %q", ev.AttrParentSpanID)
	}
}
