package concernflags_test

import (
	"testing"

	cf "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/concern-flags"
)

func TestRegister_RaiseAndList(t *testing.T) {
	var r cf.Register

	flag := cf.ConcernFlag{
		ID:        "flag-001",
		Type:      "accuracy",
		Severity:  cf.SeverityHigh,
		Target:    "doc/section/revenue",
		Rationale: "revenue figure does not match source",
	}

	if err := r.Raise(flag); err != nil {
		t.Fatalf("unexpected error raising flag: %v", err)
	}

	flags := r.List()
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(flags))
	}
	if flags[0].ID != "flag-001" {
		t.Errorf("expected ID flag-001, got %q", flags[0].ID)
	}
}

func TestRegister_DuplicateIDRejected(t *testing.T) {
	var r cf.Register

	flag := cf.ConcernFlag{
		ID:        "dup-001",
		Type:      "accuracy",
		Severity:  cf.SeverityLow,
		Target:    "doc/section/1",
		Rationale: "test",
	}

	if err := r.Raise(flag); err != nil {
		t.Fatalf("first raise: %v", err)
	}
	if err := r.Raise(flag); err == nil {
		t.Error("expected error on duplicate ID, got nil")
	}
}

func TestRegister_ByTarget(t *testing.T) {
	var r cf.Register

	flags := []cf.ConcernFlag{
		{ID: "t1", Type: "accuracy", Severity: cf.SeverityLow, Target: "doc/A", Rationale: "r1"},
		{ID: "t2", Type: "dlp", Severity: cf.SeverityHigh, Target: "doc/B", Rationale: "r2"},
		{ID: "t3", Type: "accuracy", Severity: cf.SeverityMedium, Target: "doc/A", Rationale: "r3"},
	}
	for _, f := range flags {
		if err := r.Raise(f); err != nil {
			t.Fatalf("raise %q: %v", f.ID, err)
		}
	}

	got := r.ByTarget("doc/A")
	if len(got) != 2 {
		t.Fatalf("expected 2 flags for doc/A, got %d", len(got))
	}

	gotB := r.ByTarget("doc/B")
	if len(gotB) != 1 {
		t.Fatalf("expected 1 flag for doc/B, got %d", len(gotB))
	}

	gotNone := r.ByTarget("doc/C")
	if len(gotNone) != 0 {
		t.Fatalf("expected 0 flags for doc/C, got %d", len(gotNone))
	}
}

func TestRegister_InvalidSeverityRejected(t *testing.T) {
	var r cf.Register
	flag := cf.ConcernFlag{
		ID:        "bad-sev",
		Type:      "accuracy",
		Severity:  "extreme",
		Target:    "doc/A",
		Rationale: "test",
	}
	if err := r.Raise(flag); err == nil {
		t.Error("expected error for invalid severity, got nil")
	}
}

func TestRegister_EmptyIDRejected(t *testing.T) {
	var r cf.Register
	flag := cf.ConcernFlag{
		ID:        "",
		Type:      "accuracy",
		Severity:  cf.SeverityLow,
		Target:    "doc/A",
		Rationale: "test",
	}
	if err := r.Raise(flag); err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

func TestRegister_EmptyTargetRejected(t *testing.T) {
	var r cf.Register
	flag := cf.ConcernFlag{
		ID:        "no-target",
		Type:      "accuracy",
		Severity:  cf.SeverityLow,
		Target:    "",
		Rationale: "test",
	}
	if err := r.Raise(flag); err == nil {
		t.Error("expected error for empty Target, got nil")
	}
}

func TestRegister_AllSeverities(t *testing.T) {
	sevs := []cf.Severity{cf.SeverityLow, cf.SeverityMedium, cf.SeverityHigh, cf.SeverityCritical}
	for i, s := range sevs {
		var r cf.Register
		flag := cf.ConcernFlag{
			ID:        "sev-test",
			Type:      "test",
			Severity:  s,
			Target:    "target",
			Rationale: "r",
		}
		_ = i
		if err := r.Raise(flag); err != nil {
			t.Errorf("severity %q should be valid, got error: %v", s, err)
		}
	}
}

func TestRegister_ListReturnsCopy(t *testing.T) {
	var r cf.Register
	r.Raise(cf.ConcernFlag{ //nolint
		ID: "copy-1", Type: "t", Severity: cf.SeverityLow, Target: "x", Rationale: "r",
	})

	list1 := r.List()
	list1[0].ID = "mutated"

	list2 := r.List()
	if list2[0].ID == "mutated" {
		t.Error("List() should return a copy; internal state was mutated")
	}
}
