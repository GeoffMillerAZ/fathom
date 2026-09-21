package fitnessfunctionregistry_test

import (
	"reflect"
	"testing"
	"time"

	evalgate "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-gate"
	ffr "github.com/geoffmilleraz/agentic-kit/foundations/quality/fitness-function-registry"
)

// fixedAt is a stable timestamp used throughout the tests so output is
// reproducible.
var fixedAt = time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)

// --- Tier --------------------------------------------------------------------

func TestTier_ValidAndRank(t *testing.T) {
	tests := []struct {
		tier       ffr.Tier
		wantValid  bool
		wantRank   int
		wantRankOK bool
	}{
		{ffr.TierPrototype, true, 0, true},
		{ffr.TierIntegration, true, 1, true},
		{ffr.TierApex, true, 2, true},
		{ffr.Tier("bogus"), false, 0, false},
		{ffr.Tier(""), false, 0, false},
	}
	for _, tc := range tests {
		t.Run(string(tc.tier), func(t *testing.T) {
			if got := tc.tier.Valid(); got != tc.wantValid {
				t.Errorf("Valid() = %v, want %v", got, tc.wantValid)
			}
			rank, ok := tc.tier.Rank()
			if ok != tc.wantRankOK {
				t.Errorf("Rank() ok = %v, want %v", ok, tc.wantRankOK)
			}
			if ok && rank != tc.wantRank {
				t.Errorf("Rank() = %d, want %d", rank, tc.wantRank)
			}
		})
	}
}

func TestTier_RankOrdering(t *testing.T) {
	// Prototype < Integration < Apex must hold so callers can compare strength.
	proto, _ := ffr.TierPrototype.Rank()
	integ, _ := ffr.TierIntegration.Rank()
	apex, _ := ffr.TierApex.Rank()
	if !(proto < integ && integ < apex) {
		t.Errorf("tier ordering broken: prototype=%d integration=%d apex=%d", proto, integ, apex)
	}
}

// --- Register ----------------------------------------------------------------

func passingFF(name string) ffr.FitnessFunction {
	return ffr.FitnessFunction{
		Name:      name,
		Invariant: "always true",
		Checks:    []evalgate.Check{{Name: "c", Passed: true, Weight: 1}},
		Tier:      ffr.TierApex,
		RunFreq:   "per-commit",
	}
}

func TestRegister_Errors(t *testing.T) {
	tests := []struct {
		name    string
		ff      ffr.FitnessFunction
		wantErr bool
	}{
		{
			name:    "empty name",
			ff:      ffr.FitnessFunction{Name: ""},
			wantErr: true,
		},
		{
			name:    "unrecognised tier",
			ff:      ffr.FitnessFunction{Name: "x", Tier: ffr.Tier("supreme")},
			wantErr: true,
		},
		{
			name:    "negative threshold",
			ff:      ffr.FitnessFunction{Name: "x", Threshold: -0.1},
			wantErr: true,
		},
		{
			name:    "threshold above one",
			ff:      ffr.FitnessFunction{Name: "x", Threshold: 1.5},
			wantErr: true,
		},
		{
			name:    "valid, empty tier allowed",
			ff:      ffr.FitnessFunction{Name: "x", Tier: ""},
			wantErr: false,
		},
		{
			name:    "valid, no checks allowed",
			ff:      ffr.FitnessFunction{Name: "x", Tier: ffr.TierPrototype},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ffr.NewRegistry()
			err := r.Register(tc.ff)
			if (err != nil) != tc.wantErr {
				t.Errorf("Register() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestRegister_DuplicateRejected(t *testing.T) {
	r := ffr.NewRegistry()
	if err := r.Register(passingFF("dup")); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	err := r.Register(passingFF("dup"))
	if err == nil {
		t.Fatalf("expected error registering duplicate name, got nil")
	}
	if r.Len() != 1 {
		t.Errorf("expected Len()=1 after rejected duplicate, got %d", r.Len())
	}
}

func TestRegister_ZeroRegistryRejected(t *testing.T) {
	// The zero value is not ready for use; Register must fail rather than panic.
	var r ffr.Registry
	if err := r.Register(passingFF("x")); err == nil {
		t.Errorf("expected error registering into zero-value Registry, got nil")
	}
}

// --- Get / List --------------------------------------------------------------

func TestGet(t *testing.T) {
	r := ffr.NewRegistry()
	_ = r.Register(passingFF("alpha"))

	got, ok := r.Get("alpha")
	if !ok {
		t.Fatalf("expected Get(alpha) to find the function")
	}
	if got.Name != "alpha" {
		t.Errorf("got Name=%q, want alpha", got.Name)
	}

	if _, ok := r.Get("missing"); ok {
		t.Errorf("expected Get(missing) ok=false")
	}

	// Get on a zero-value registry must not panic.
	var zr ffr.Registry
	if _, ok := zr.Get("anything"); ok {
		t.Errorf("expected zero-value Get ok=false")
	}
}

func TestList_SortedAndDeterministic(t *testing.T) {
	r := ffr.NewRegistry()
	// Register out of order; List must return Name-sorted regardless.
	for _, name := range []string{"gamma", "alpha", "beta"} {
		if err := r.Register(passingFF(name)); err != nil {
			t.Fatalf("Register(%q) failed: %v", name, err)
		}
	}
	want := []string{"alpha", "beta", "gamma"}

	// Run List several times: map iteration is randomised, so a non-sorted
	// implementation would eventually disagree.
	for i := 0; i < 50; i++ {
		got := r.List()
		gotNames := make([]string, len(got))
		for j, ff := range got {
			gotNames[j] = ff.Name
		}
		if !reflect.DeepEqual(gotNames, want) {
			t.Fatalf("iteration %d: List order = %v, want %v", i, gotNames, want)
		}
	}
}

func TestList_EmptyNonNil(t *testing.T) {
	r := ffr.NewRegistry()
	got := r.List()
	if got == nil {
		t.Errorf("expected non-nil empty slice from List on empty registry")
	}
	if len(got) != 0 {
		t.Errorf("expected len 0, got %d", len(got))
	}
}

func TestList_ReturnsCopySafeToMutate(t *testing.T) {
	r := ffr.NewRegistry()
	_ = r.Register(passingFF("alpha"))
	got := r.List()
	got[0].Name = "MUTATED" // caller mutates the returned slice
	// Registry's own copy must be unaffected.
	ff, ok := r.Get("alpha")
	if !ok || ff.Name != "alpha" {
		t.Errorf("registry copy was mutated by caller: got %q ok=%v", ff.Name, ok)
	}
}

// --- Execute -----------------------------------------------------------------

func TestExecute_TableScoring(t *testing.T) {
	tests := []struct {
		name       string
		ff         ffr.FitnessFunction
		wantScore  float64
		wantPassed bool
		wantFails  []string
	}{
		{
			name: "all pass, default threshold",
			ff: ffr.FitnessFunction{
				Name: "f",
				Checks: []evalgate.Check{
					{Name: "a", Passed: true, Weight: 1},
					{Name: "b", Passed: true, Weight: 1},
				},
			},
			wantScore:  1.0,
			wantPassed: true,
			wantFails:  nil,
		},
		{
			name: "weighted partial, threshold met",
			ff: ffr.FitnessFunction{
				Name:      "f",
				Threshold: 0.6,
				Checks: []evalgate.Check{
					{Name: "heavy", Passed: true, Weight: 2},
					{Name: "light", Passed: false, Weight: 1},
				},
			},
			wantScore:  2.0 / 3.0,
			wantPassed: true,
			wantFails:  []string{"light"},
		},
		{
			name: "weighted partial, threshold not met",
			ff: ffr.FitnessFunction{
				Name:      "f",
				Threshold: 0.9,
				Checks: []evalgate.Check{
					{Name: "heavy", Passed: true, Weight: 2},
					{Name: "light", Passed: false, Weight: 1},
				},
			},
			wantScore:  2.0 / 3.0,
			wantPassed: false,
			wantFails:  []string{"light"},
		},
		{
			name: "zero threshold means all must pass",
			ff: ffr.FitnessFunction{
				Name: "f",
				Checks: []evalgate.Check{
					{Name: "a", Passed: true, Weight: 1},
					{Name: "b", Passed: false, Weight: 1},
				},
			},
			wantScore:  0.5,
			wantPassed: false,
			wantFails:  []string{"b"},
		},
		{
			name:       "no checks fails closed",
			ff:         ffr.FitnessFunction{Name: "empty"},
			wantScore:  0.0,
			wantPassed: false,
			wantFails:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := ffr.Execute(tc.ff, "run-1", fixedAt)

			if res.FunctionName != tc.ff.Name {
				t.Errorf("FunctionName = %q, want %q", res.FunctionName, tc.ff.Name)
			}
			if !floatEq(res.Score, tc.wantScore) {
				t.Errorf("Score = %v, want %v", res.Score, tc.wantScore)
			}
			if res.Passed != tc.wantPassed {
				t.Errorf("Passed = %v, want %v", res.Passed, tc.wantPassed)
			}
			if !reflect.DeepEqual(res.Failures, tc.wantFails) {
				t.Errorf("Failures = %v, want %v", res.Failures, tc.wantFails)
			}
			if res.RunID != "run-1" {
				t.Errorf("RunID = %q, want run-1", res.RunID)
			}
			if !res.At.Equal(fixedAt) {
				t.Errorf("At = %v, want %v", res.At, fixedAt)
			}
			if res.Duration != 0 {
				t.Errorf("Execute Duration = %v, want 0 (pure)", res.Duration)
			}
		})
	}
}

func TestExecute_MatchesEvalGateDirectly(t *testing.T) {
	// Execute must delegate scoring to eval-gate, not reimplement it. Compare
	// against a direct eval-gate evaluation of the same checks + threshold.
	checks := []evalgate.Check{
		{Name: "x", Passed: true, Weight: 3},
		{Name: "y", Passed: false, Weight: 1},
	}
	ff := ffr.FitnessFunction{Name: "f", Threshold: 0.7, Checks: checks}

	res := ffr.Execute(ff, "rid", fixedAt)
	want := evalgate.Gate{Threshold: 0.7}.Evaluate(checks)

	if !floatEq(res.Score, want.Score) || res.Passed != want.Passed {
		t.Errorf("Execute disagrees with eval-gate: got score=%v passed=%v, eval-gate score=%v passed=%v",
			res.Score, res.Passed, want.Score, want.Passed)
	}
	if !reflect.DeepEqual(res.Failures, want.Failures) {
		t.Errorf("Failures = %v, want %v (eval-gate)", res.Failures, want.Failures)
	}
}

func TestExecute_InjectedIdentityPassedThroughUnchanged(t *testing.T) {
	// RunID and At must be passed through verbatim — even unusual / "unsafe"
	// values (control chars, unicode, quotes) — without escaping or mutation.
	weird := "run\t\"id\"\n<script>// 日本語 \x00"
	at := time.Date(1999, 12, 31, 23, 59, 59, 123456789, time.FixedZone("odd", -7*3600))

	res := ffr.Execute(passingFF("f"), weird, at)
	if res.RunID != weird {
		t.Errorf("RunID was altered: got %q, want %q", res.RunID, weird)
	}
	if !res.At.Equal(at) {
		t.Errorf("At was altered: got %v, want %v", res.At, at)
	}
}

func TestExecute_SpecialCharsInNamesPreserved(t *testing.T) {
	// Check names and function names with special characters must flow into the
	// result unescaped and unmodified (no injection, no normalisation).
	ff := ffr.FitnessFunction{
		Name: "ƒ​/with\tweird\"chars\"",
		Checks: []evalgate.Check{
			{Name: "bad\ncheck<&>", Passed: false, Weight: 1},
			{Name: "日本語チェック", Passed: false, Weight: 1},
		},
	}
	res := ffr.Execute(ff, "r", fixedAt)
	if res.FunctionName != ff.Name {
		t.Errorf("FunctionName altered: %q", res.FunctionName)
	}
	wantFails := []string{"bad\ncheck<&>", "日本語チェック"}
	if !reflect.DeepEqual(res.Failures, wantFails) {
		t.Errorf("Failures = %#v, want %#v", res.Failures, wantFails)
	}
}

func TestExecute_Deterministic(t *testing.T) {
	// Same inputs → byte-identical result, repeated. Crucially, Failures order
	// must be stable (it mirrors input Check order, never map order).
	ff := ffr.FitnessFunction{
		Name:      "det",
		Threshold: 0.5,
		Checks: []evalgate.Check{
			{Name: "f1", Passed: false, Weight: 1},
			{Name: "p1", Passed: true, Weight: 1},
			{Name: "f2", Passed: false, Weight: 1},
		},
	}
	first := ffr.Execute(ff, "run-x", fixedAt)
	for i := 0; i < 100; i++ {
		got := ffr.Execute(ff, "run-x", fixedAt)
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("iteration %d: non-deterministic result\n got=%+v\nwant=%+v", i, got, first)
		}
	}
	// Sanity: failures preserve input order f1, f2.
	if !reflect.DeepEqual(first.Failures, []string{"f1", "f2"}) {
		t.Errorf("Failures = %v, want [f1 f2] (input order preserved)", first.Failures)
	}
}

func TestExecuteTimed_RecordsNonNegativeDuration(t *testing.T) {
	res := ffr.ExecuteTimed(passingFF("t"), "r", fixedAt)
	if res.Duration < 0 {
		t.Errorf("ExecuteTimed Duration negative: %v", res.Duration)
	}
	// Every non-Duration field must match the pure Execute exactly.
	pure := ffr.Execute(passingFF("t"), "r", fixedAt)
	res.Duration = 0
	if !reflect.DeepEqual(res, pure) {
		t.Errorf("ExecuteTimed differs from Execute beyond Duration:\n timed=%+v\n pure =%+v", res, pure)
	}
}

// --- ExecuteAll --------------------------------------------------------------

func TestExecuteAll_SortedDeterministicSharedIdentity(t *testing.T) {
	r := ffr.NewRegistry()
	specs := map[string]bool{ // name -> should pass
		"charlie": true,
		"alpha":   false,
		"bravo":   true,
	}
	for name, pass := range specs {
		if err := r.Register(ffr.FitnessFunction{
			Name:   name,
			Checks: []evalgate.Check{{Name: "c", Passed: pass, Weight: 1}},
		}); err != nil {
			t.Fatalf("Register(%q): %v", name, err)
		}
	}

	results := r.ExecuteAll("batch-1", fixedAt)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Sorted by FunctionName.
	wantOrder := []string{"alpha", "bravo", "charlie"}
	for i, res := range results {
		if res.FunctionName != wantOrder[i] {
			t.Errorf("result[%d] = %q, want %q", i, res.FunctionName, wantOrder[i])
		}
		// Shared run identity across the batch.
		if res.RunID != "batch-1" || !res.At.Equal(fixedAt) {
			t.Errorf("result[%d] identity not shared: RunID=%q At=%v", i, res.RunID, res.At)
		}
	}

	// Per-function pass/fail respected.
	if results[0].Passed { // alpha should fail
		t.Errorf("alpha expected to fail")
	}
	if !results[1].Passed || !results[2].Passed { // bravo, charlie pass
		t.Errorf("bravo/charlie expected to pass")
	}

	// Determinism across repeated calls.
	for i := 0; i < 25; i++ {
		again := r.ExecuteAll("batch-1", fixedAt)
		if !reflect.DeepEqual(again, results) {
			t.Fatalf("ExecuteAll non-deterministic on iteration %d", i)
		}
	}
}

func TestExecuteAll_Empty(t *testing.T) {
	r := ffr.NewRegistry()
	got := r.ExecuteAll("r", fixedAt)
	if len(got) != 0 {
		t.Errorf("expected 0 results from empty registry, got %d", len(got))
	}
}

// --- helpers -----------------------------------------------------------------

func floatEq(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	return d < eps && d > -eps
}
