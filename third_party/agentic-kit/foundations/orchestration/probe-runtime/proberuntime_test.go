package proberuntime

import (
	"context"
	"testing"
)

func passProbe(_ string) ProbeFunc {
	return func(_ context.Context) ProbeResult {
		return ProbeResult{Status: StatusPass, Detail: "ok"}
	}
}

func failProbe() ProbeFunc {
	return func(_ context.Context) ProbeResult {
		return ProbeResult{Status: StatusFail, Detail: "check failed"}
	}
}

func panicProbe() ProbeFunc {
	return func(_ context.Context) ProbeResult {
		panic("something exploded")
	}
}

func TestRegister_And_RunAll(t *testing.T) {
	rt := NewRuntime()
	if err := rt.Register("alpha", passProbe("alpha")); err != nil {
		t.Fatalf("Register alpha: %v", err)
	}
	if err := rt.Register("beta", passProbe("beta")); err != nil {
		t.Fatalf("Register beta: %v", err)
	}

	results := rt.RunAll(context.Background())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != StatusPass {
			t.Errorf("probe %q: expected pass, got %v", r.Name, r.Status)
		}
	}
}

func TestRunAll_DeterministicOrder(t *testing.T) {
	rt := NewRuntime()
	// Register in reverse alphabetical order.
	_ = rt.Register("zebra", passProbe("zebra"))
	_ = rt.Register("alpha", passProbe("alpha"))
	_ = rt.Register("mango", passProbe("mango"))

	results := rt.RunAll(context.Background())
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	expected := []string{"alpha", "mango", "zebra"}
	for i, want := range expected {
		if results[i].Name != want {
			t.Errorf("index %d: expected %q, got %q", i, want, results[i].Name)
		}
	}
}

func TestRunAll_NameOverwrite(t *testing.T) {
	// The probe's self-reported name is ignored; the registered name is used.
	rt := NewRuntime()
	_ = rt.Register("registered-name", ProbeFunc(func(_ context.Context) ProbeResult {
		return ProbeResult{Name: "wrong-name", Status: StatusPass}
	}))
	results := rt.RunAll(context.Background())
	if results[0].Name != "registered-name" {
		t.Fatalf("expected registered name, got %q", results[0].Name)
	}
}

func TestRunAll_FailProbe(t *testing.T) {
	rt := NewRuntime()
	_ = rt.Register("checker", failProbe())
	results := rt.RunAll(context.Background())
	if results[0].Status != StatusFail {
		t.Fatalf("expected StatusFail, got %v", results[0].Status)
	}
}

func TestRunAll_PanicRecovery(t *testing.T) {
	rt := NewRuntime()
	_ = rt.Register("boom", panicProbe())
	results := rt.RunAll(context.Background())
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusError {
		t.Fatalf("expected StatusError on panic, got %v", results[0].Status)
	}
	if results[0].Detail == "" {
		t.Fatal("expected non-empty Detail on panic recovery")
	}
}

func TestRunAll_Empty(t *testing.T) {
	rt := NewRuntime()
	results := rt.RunAll(context.Background())
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty runtime, got %d", len(results))
	}
}

func TestRegister_EmptyName(t *testing.T) {
	rt := NewRuntime()
	if err := rt.Register("", passProbe("")); err != ErrEmptyName {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
}

func TestRegister_Duplicate(t *testing.T) {
	rt := NewRuntime()
	_ = rt.Register("probe", passProbe("probe"))
	if err := rt.Register("probe", passProbe("probe")); err != ErrDuplicateName {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}
}

func TestRunAll_ContextPropagation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	rt := NewRuntime()
	sawCancelled := false
	_ = rt.Register("ctx-check", ProbeFunc(func(c context.Context) ProbeResult {
		if c.Err() != nil {
			sawCancelled = true
			return ProbeResult{Status: StatusFail, Detail: "context cancelled"}
		}
		return ProbeResult{Status: StatusPass}
	}))
	rt.RunAll(ctx)
	if !sawCancelled {
		t.Fatal("expected probe to see cancelled context")
	}
}
