package determinismgate

import (
	"testing"
	"time"

	fitnessfunctionregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
)

var testEpoch = time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

// TestFitnessRegistration verifies that FitnessFunctions returns exactly four
// well-formed functions, all of which can be registered without error, and
// that an all-passing config yields Passed=true for every function.
func TestFitnessRegistration(t *testing.T) {
	cfg := EvalLoopConfig{
		IndependenceValidated: true,
		ProvenanceGatesOn:     true,
		JournalAttached:       true,
		DeterminismProbed:     true,
	}
	fns := FitnessFunctions(cfg)

	if len(fns) != 4 {
		t.Fatalf("FitnessFunctions: got %d functions, want 4", len(fns))
	}

	reg := fitnessfunctionregistry.NewRegistry()
	for _, fn := range fns {
		if err := reg.Register(fn); err != nil {
			t.Errorf("Register(%q): %v", fn.Name, err)
		}
	}
	if reg.Len() != 4 {
		t.Fatalf("registry len: got %d, want 4", reg.Len())
	}

	// All checks pass → all functions should pass.
	results := reg.ExecuteAll("test-run", testEpoch)
	for _, r := range results {
		if !r.Passed {
			t.Errorf("function %q: expected Passed=true when all config is true, got Score=%g Failures=%v",
				r.FunctionName, r.Score, r.Failures)
		}
	}
}

func TestFitnessRegistration_allFail(t *testing.T) {
	cfg := EvalLoopConfig{} // all false
	fns := FitnessFunctions(cfg)

	reg := fitnessfunctionregistry.NewRegistry()
	for _, fn := range fns {
		if err := reg.Register(fn); err != nil {
			t.Errorf("Register(%q): %v", fn.Name, err)
		}
	}

	results := reg.ExecuteAll("test-run", testEpoch)
	for _, r := range results {
		if r.Passed {
			t.Errorf("function %q: expected Passed=false when all config is false, got Passed=true",
				r.FunctionName)
		}
	}
}

func TestFitnessRegistration_names(t *testing.T) {
	want := map[string]bool{
		"eval.independence-validated": false,
		"eval.provenance-gates-on":    false,
		"eval.journal-attached":       false,
		"eval.determinism-probed":     false,
	}
	for _, fn := range FitnessFunctions(EvalLoopConfig{}) {
		if _, ok := want[fn.Name]; !ok {
			t.Errorf("unexpected function name: %q", fn.Name)
		}
		want[fn.Name] = true
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("missing expected function: %q", name)
		}
	}
}
