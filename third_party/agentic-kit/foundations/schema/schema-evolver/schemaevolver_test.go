package schemaevolver_test

import (
	"testing"

	se "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/schema/schema-evolver"
)

// ---- helpers ----------------------------------------------------------------

// v is a convenience constructor.
func v(major, minor int) se.Version { return se.Version{Major: major, Minor: minor} }

// mig builds a Migration with a non-empty description.
func mig(from, to se.Version, desc string) se.Migration {
	return se.Migration{From: from, To: to, Description: desc}
}

// mustRegister registers a migration and fails the test if it returns an error.
func mustRegister(t *testing.T, e *se.Evolver, m se.Migration) {
	t.Helper()
	if err := e.Register(m); err != nil {
		t.Fatalf("Register(%v→%v): %v", m.From, m.To, err)
	}
}

// ---- Register ---------------------------------------------------------------

func TestRegister_EmptyDescriptionIsRejected(t *testing.T) {
	var e se.Evolver
	err := e.Register(se.Migration{From: v(1, 0), To: v(2, 0), Description: ""})
	if err == nil {
		t.Fatal("expected error for empty description, got nil")
	}
}

func TestRegister_DuplicateIsIgnored(t *testing.T) {
	var e se.Evolver
	mustRegister(t, &e, mig(v(1, 0), v(2, 0), "first"))
	// Registering the same (from, to) pair a second time must not error.
	if err := e.Register(mig(v(1, 0), v(2, 0), "second")); err != nil {
		t.Fatalf("unexpected error on duplicate register: %v", err)
	}
	// There should still be only one step for this pair.
	path, err := e.Path(v(1, 0), v(2, 0))
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if len(path) != 1 {
		t.Errorf("expected 1 step, got %d", len(path))
	}
	if path[0].Description != "first" {
		t.Errorf("expected first registration to be kept, got %q", path[0].Description)
	}
}

// ---- Path: direct and multi-hop ---------------------------------------------

func TestPath_SameVersion_EmptyPath(t *testing.T) {
	var e se.Evolver
	path, err := e.Path(v(1, 0), v(1, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(path) != 0 {
		t.Errorf("expected empty path for same version, got %d steps", len(path))
	}
}

func TestPath_DirectStep(t *testing.T) {
	var e se.Evolver
	mustRegister(t, &e, mig(v(1, 0), v(2, 0), "major bump"))

	path, err := e.Path(v(1, 0), v(2, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(path) != 1 {
		t.Fatalf("expected 1 step, got %d", len(path))
	}
	if path[0].From != v(1, 0) || path[0].To != v(2, 0) {
		t.Errorf("unexpected step: %v→%v", path[0].From, path[0].To)
	}
}

func TestPath_MultiHop(t *testing.T) {
	var e se.Evolver
	mustRegister(t, &e, mig(v(1, 0), v(1, 1), "minor add optional field"))
	mustRegister(t, &e, mig(v(1, 1), v(2, 0), "breaking: add required field"))
	mustRegister(t, &e, mig(v(2, 0), v(2, 1), "minor add tags"))

	path, err := e.Path(v(1, 0), v(2, 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(path) != 3 {
		t.Fatalf("expected 3 steps, got %d: %v", len(path), path)
	}
	// Verify the sequence is contiguous.
	if path[0].From != v(1, 0) {
		t.Errorf("step 0 from: got %v, want 1.0", path[0].From)
	}
	if path[1].From != v(1, 1) {
		t.Errorf("step 1 from: got %v, want 1.1", path[1].From)
	}
	if path[2].From != v(2, 0) {
		t.Errorf("step 2 from: got %v, want 2.0", path[2].From)
	}
	if path[2].To != v(2, 1) {
		t.Errorf("step 2 to: got %v, want 2.1", path[2].To)
	}
}

func TestPath_NoPath_ReturnsError(t *testing.T) {
	var e se.Evolver
	mustRegister(t, &e, mig(v(1, 0), v(2, 0), "1.0 → 2.0"))
	// 3.0 has no incoming edges — no path from 1.0.
	_, err := e.Path(v(1, 0), v(3, 0))
	if err == nil {
		t.Fatal("expected error for missing path, got nil")
	}
}

func TestPath_BFSFindsShortestPath(t *testing.T) {
	var e se.Evolver
	// Long route: 1.0 → 1.1 → 2.0 → 3.0
	mustRegister(t, &e, mig(v(1, 0), v(1, 1), "step"))
	mustRegister(t, &e, mig(v(1, 1), v(2, 0), "step"))
	mustRegister(t, &e, mig(v(2, 0), v(3, 0), "step"))
	// Short route: 1.0 → 3.0 (direct skip)
	mustRegister(t, &e, mig(v(1, 0), v(3, 0), "direct skip"))

	path, err := e.Path(v(1, 0), v(3, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(path) != 1 {
		t.Errorf("BFS should find shortest path (1 step), got %d steps", len(path))
	}
}

func TestPath_Deterministic(t *testing.T) {
	var e se.Evolver
	mustRegister(t, &e, mig(v(1, 0), v(2, 0), "1.0 → 2.0"))
	mustRegister(t, &e, mig(v(1, 0), v(1, 1), "1.0 → 1.1"))
	mustRegister(t, &e, mig(v(1, 1), v(2, 0), "1.1 → 2.0"))

	path1, err := e.Path(v(1, 0), v(2, 0))
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	path2, err := e.Path(v(1, 0), v(2, 0))
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(path1) != len(path2) {
		t.Fatalf("non-deterministic: len %d vs %d", len(path1), len(path2))
	}
	for i := range path1 {
		if path1[i] != path2[i] {
			t.Errorf("step %d differs between calls", i)
		}
	}
}

// ---- IsBackwardCompatible ---------------------------------------------------

func TestIsBackwardCompatible_OnlyMinorIncreases(t *testing.T) {
	var e se.Evolver
	mustRegister(t, &e, mig(v(1, 0), v(1, 1), "add optional field"))
	mustRegister(t, &e, mig(v(1, 1), v(1, 2), "add another optional field"))

	if !e.IsBackwardCompatible(v(1, 0), v(1, 2)) {
		t.Error("expected backward-compatible for minor-only increases")
	}
}

func TestIsBackwardCompatible_MajorBump_IsBreaking(t *testing.T) {
	var e se.Evolver
	mustRegister(t, &e, mig(v(1, 0), v(2, 0), "breaking change"))

	if e.IsBackwardCompatible(v(1, 0), v(2, 0)) {
		t.Error("expected breaking for major version bump")
	}
}

func TestIsBackwardCompatible_SameVersion(t *testing.T) {
	var e se.Evolver
	if !e.IsBackwardCompatible(v(3, 7), v(3, 7)) {
		t.Error("same version must be backward-compatible")
	}
}

func TestIsBackwardCompatible_NoPath_ReturnsFalse(t *testing.T) {
	var e se.Evolver
	if e.IsBackwardCompatible(v(1, 0), v(9, 9)) {
		t.Error("expected false when no path exists")
	}
}

func TestIsBackwardCompatible_MixedPath_IsBreaking(t *testing.T) {
	var e se.Evolver
	mustRegister(t, &e, mig(v(1, 0), v(1, 1), "safe"))
	mustRegister(t, &e, mig(v(1, 1), v(2, 0), "breaking"))

	if e.IsBackwardCompatible(v(1, 0), v(2, 0)) {
		t.Error("expected breaking: path contains a major bump")
	}
}
