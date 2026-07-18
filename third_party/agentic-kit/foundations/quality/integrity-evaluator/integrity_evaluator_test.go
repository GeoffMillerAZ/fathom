package integrityevaluator_test

import (
	"testing"

	ie "github.com/geoffmilleraz/agentic-kit/foundations/quality/integrity-evaluator"
)

func TestEvaluate_Clean(t *testing.T) {
	output := "The system uses TLS [1] as documented in doc:tls-guide."
	knownRefs := map[string]string{
		"[1]":           "RFC 8446",
		"doc:tls-guide": "Internal TLS guide v2",
	}
	r := ie.Evaluate(output, knownRefs)
	if !r.Passed {
		t.Errorf("expected Passed=true, got violations: %v", r.Violations)
	}
	if len(r.Violations) != 0 {
		t.Errorf("expected no violations, got %d", len(r.Violations))
	}
}

func TestEvaluate_FabricatedRef(t *testing.T) {
	output := "See [99] for details."
	knownRefs := map[string]string{
		"[1]": "some real doc",
	}
	r := ie.Evaluate(output, knownRefs)
	if r.Passed {
		t.Errorf("expected Passed=false (fabricated ref)")
	}
	if len(r.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(r.Violations), r.Violations)
	}
	if r.Violations[0].Kind != ie.ViolationFabricatedRef {
		t.Errorf("expected ViolationFabricatedRef, got %q", r.Violations[0].Kind)
	}
	if r.Violations[0].Ref != "[99]" {
		t.Errorf("expected Ref=[99], got %q", r.Violations[0].Ref)
	}
}

func TestEvaluate_UnresolvableRef(t *testing.T) {
	output := "See [2] for details."
	knownRefs := map[string]string{
		"[2]": "", // empty resolved value
	}
	r := ie.Evaluate(output, knownRefs)
	if r.Passed {
		t.Errorf("expected Passed=false (unresolvable ref)")
	}
	found := false
	for _, v := range r.Violations {
		if v.Kind == ie.ViolationUnresolvableRef && v.Ref == "[2]" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ViolationUnresolvableRef for [2], violations: %v", r.Violations)
	}
}

func TestEvaluate_NoRefsInOutput(t *testing.T) {
	output := "This is a plain statement with no citations."
	knownRefs := map[string]string{
		"[1]": "RFC 8446",
	}
	r := ie.Evaluate(output, knownRefs)
	if !r.Passed {
		t.Errorf("expected Passed=true (no refs in output), got: %v", r.Violations)
	}
}

func TestEvaluate_EmptyOutputEmptyRefs(t *testing.T) {
	r := ie.Evaluate("", nil)
	if !r.Passed {
		t.Errorf("expected Passed=true for empty output and refs")
	}
}

func TestEvaluate_Deterministic(t *testing.T) {
	output := "See [1] and [2] for details."
	knownRefs := map[string]string{
		"[1]": "doc1",
		"[2]": "",
	}
	r1 := ie.Evaluate(output, knownRefs)
	r2 := ie.Evaluate(output, knownRefs)
	if r1.Passed != r2.Passed || len(r1.Violations) != len(r2.Violations) {
		t.Errorf("non-deterministic results: %+v vs %+v", r1, r2)
	}
	for i := range r1.Violations {
		if r1.Violations[i].Kind != r2.Violations[i].Kind ||
			r1.Violations[i].Ref != r2.Violations[i].Ref {
			t.Errorf("non-deterministic violation[%d]: %+v vs %+v",
				i, r1.Violations[i], r2.Violations[i])
		}
	}
}

func TestEvaluate_MultipleViolationsSorted(t *testing.T) {
	output := "See [b] and [a] for details."
	knownRefs := map[string]string{} // neither is known
	r := ie.Evaluate(output, knownRefs)
	if len(r.Violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(r.Violations))
	}
	if r.Violations[0].Ref >= r.Violations[1].Ref {
		t.Errorf("violations not sorted: %q >= %q",
			r.Violations[0].Ref, r.Violations[1].Ref)
	}
}

func TestEvaluate_DocPrefixRef(t *testing.T) {
	output := "Refer to doc:api-spec for the interface definition."
	knownRefs := map[string]string{} // not present
	r := ie.Evaluate(output, knownRefs)
	if r.Passed {
		t.Errorf("expected Passed=false (doc:api-spec not in knownRefs)")
	}
	found := false
	for _, v := range r.Violations {
		if v.Ref == "doc:api-spec" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected violation for doc:api-spec, got %v", r.Violations)
	}
}
