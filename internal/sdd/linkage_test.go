// Package sdd implements spec-driven design linkage analysis: spec ID recognition,
// reference extraction, and coverage matrix operations.
package sdd_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/sdd"
)

// ─── ExtractRefs ──────────────────────────────────────────────────────────────

func TestExtractRefs_Empty(t *testing.T) {
	got := sdd.ExtractRefs("")
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestExtractRefs_NoRefs(t *testing.T) {
	got := sdd.ExtractRefs("This comment mentions nothing trackable.")
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestExtractRefs_SingleRef(t *testing.T) {
	got := sdd.ExtractRefs("// Implements [US-042] login flow.")
	want := []string{"[US-042]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractRefs_MultipleRefs(t *testing.T) {
	got := sdd.ExtractRefs("// See [US-042] and [EP-013] for context.")
	want := []string{"[EP-013]", "[US-042]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractRefs_Deduplicated(t *testing.T) {
	got := sdd.ExtractRefs("[US-042] repeated [US-042] again [EP-013]")
	want := []string{"[EP-013]", "[US-042]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractRefs_SortedUnique(t *testing.T) {
	got := sdd.ExtractRefs("[NFR-1] before [FR-7] then [EP-013]")
	want := []string{"[EP-013]", "[FR-7]", "[NFR-1]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractRefs_DefaultPrefixes(t *testing.T) {
	// Default prefixes: US, EP, FR, NFR — XY should not match default.
	got := sdd.ExtractRefs("[US-1] [EP-2] [FR-3] [NFR-4] [XY-5]")
	want := []string{"[EP-2]", "[FR-3]", "[NFR-4]", "[US-1]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractRefs_MultiLineComment(t *testing.T) {
	text := `// HandleLogin implements the login flow.
// Ref: [US-042], [EP-013].
// See also [US-099] for session handling.`
	got := sdd.ExtractRefs(text)
	want := []string{"[EP-013]", "[US-042]", "[US-099]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractRefs_Deterministic(t *testing.T) {
	text := "[FR-7] [US-042] [EP-013] [US-042] [FR-7]"
	first := sdd.ExtractRefs(text)
	for i := 0; i < 20; i++ {
		got := sdd.ExtractRefs(text)
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("non-deterministic: run %d got %v, want %v", i, got, first)
		}
	}
}

// ─── Configurable prefixes ────────────────────────────────────────────────────

func TestExtractRefsWithPrefixes_CustomPrefixes(t *testing.T) {
	// Only "TC" and "BUG" prefixes.
	got := sdd.ExtractRefsWithPrefixes("[TC-001] [BUG-42] [US-001]", []string{"TC", "BUG"})
	want := []string{"[BUG-42]", "[TC-001]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractRefsWithPrefixes_SingleChar_NotMatched(t *testing.T) {
	// Single-character prefixes must not be treated as valid (design spec: "2+ uppercase letters").
	got := sdd.ExtractRefsWithPrefixes("[A-1] [US-1]", []string{"US"})
	want := []string{"[US-1]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractRefsWithPrefixes_EmptyPrefixes_ReturnsNone(t *testing.T) {
	// Empty prefix list → nothing matches.
	got := sdd.ExtractRefsWithPrefixes("[US-042]", []string{})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

// ─── Link type ────────────────────────────────────────────────────────────────

func TestLink_Fields(t *testing.T) {
	l := sdd.Link{Spec: "[US-042]", Symbol: "HandleLogin", File: "internal/auth/login.go"}
	if l.Spec != "[US-042]" || l.Symbol != "HandleLogin" || l.File != "internal/auth/login.go" {
		t.Fatalf("unexpected Link fields: %+v", l)
	}
}

// ─── BuildMatrix / Coverage ───────────────────────────────────────────────────

func TestBuildMatrix_AllCovered(t *testing.T) {
	specs := []string{"[US-001]", "[US-002]"}
	codeRefs := map[string][]string{
		"HandleLogin":  {"[US-001]"},
		"HandleSignup": {"[US-002]"},
	}
	m := sdd.BuildMatrix(specs, codeRefs)

	cov1 := m.Coverage("[US-001]")
	if !contains(cov1, "HandleLogin") {
		t.Fatalf("Coverage([US-001]) = %v, want HandleLogin", cov1)
	}

	cov2 := m.Coverage("[US-002]")
	if !contains(cov2, "HandleSignup") {
		t.Fatalf("Coverage([US-002]) = %v, want HandleSignup", cov2)
	}
}

func TestBuildMatrix_Uncovered(t *testing.T) {
	specs := []string{"[US-001]", "[US-002]", "[US-003]"}
	codeRefs := map[string][]string{
		"HandleLogin": {"[US-001]"},
	}
	m := sdd.BuildMatrix(specs, codeRefs)

	uncov := m.Uncovered(specs)
	if !contains(uncov, "[US-002]") {
		t.Fatalf("Uncovered should contain [US-002], got %v", uncov)
	}
	if !contains(uncov, "[US-003]") {
		t.Fatalf("Uncovered should contain [US-003], got %v", uncov)
	}
	if contains(uncov, "[US-001]") {
		t.Fatalf("Uncovered should NOT contain [US-001] (it is covered), got %v", uncov)
	}
}

func TestBuildMatrix_Orphans(t *testing.T) {
	specs := []string{"[US-001]"}
	codeRefs := map[string][]string{
		"HandleLogin":  {"[US-001]"},
		"HandleLogout": {"[US-999]"}, // [US-999] not in declared specs
	}
	m := sdd.BuildMatrix(specs, codeRefs)

	orphans := m.Orphans(codeRefs)
	if !contains(orphans, "[US-999]") {
		t.Fatalf("Orphans should contain [US-999], got %v", orphans)
	}
	if contains(orphans, "[US-001]") {
		t.Fatalf("Orphans should NOT contain [US-001], got %v", orphans)
	}
}

func TestBuildMatrix_NoOrphans(t *testing.T) {
	specs := []string{"[US-001]", "[US-002]"}
	codeRefs := map[string][]string{
		"HandleLogin":  {"[US-001]"},
		"HandleSignup": {"[US-002]"},
	}
	m := sdd.BuildMatrix(specs, codeRefs)
	orphans := m.Orphans(codeRefs)
	if len(orphans) != 0 {
		t.Fatalf("expected no orphans, got %v", orphans)
	}
}

func TestBuildMatrix_SymbolMultipleSpecs(t *testing.T) {
	specs := []string{"[US-001]", "[EP-001]"}
	codeRefs := map[string][]string{
		"HandleAuth": {"[US-001]", "[EP-001]"},
	}
	m := sdd.BuildMatrix(specs, codeRefs)

	cov1 := m.Coverage("[US-001]")
	if !contains(cov1, "HandleAuth") {
		t.Fatalf("Coverage([US-001]) = %v, want HandleAuth", cov1)
	}
	cov2 := m.Coverage("[EP-001]")
	if !contains(cov2, "HandleAuth") {
		t.Fatalf("Coverage([EP-001]) = %v, want HandleAuth", cov2)
	}
}

func TestBuildMatrix_MultipleSymbolsPerSpec(t *testing.T) {
	specs := []string{"[US-001]"}
	codeRefs := map[string][]string{
		"HandleLogin":   {"[US-001]"},
		"ValidateToken": {"[US-001]"},
	}
	m := sdd.BuildMatrix(specs, codeRefs)

	cov := m.Coverage("[US-001]")
	if !contains(cov, "HandleLogin") || !contains(cov, "ValidateToken") {
		t.Fatalf("Coverage([US-001]) = %v, want both HandleLogin and ValidateToken", cov)
	}
}

func TestBuildMatrix_EmptySpecs(t *testing.T) {
	m := sdd.BuildMatrix(nil, nil)
	if len(m.Uncovered(nil)) != 0 {
		t.Fatal("expected no uncovered specs")
	}
	if len(m.Orphans(nil)) != 0 {
		t.Fatal("expected no orphans")
	}
}

func TestBuildMatrix_CoverageUnknownSpec_ReturnsEmpty(t *testing.T) {
	m := sdd.BuildMatrix([]string{"[US-001]"}, map[string][]string{})
	cov := m.Coverage("[UNKNOWN-999]")
	if len(cov) != 0 {
		t.Fatalf("expected empty for unknown spec, got %v", cov)
	}
}

// ─── Coverage determinism ─────────────────────────────────────────────────────

func TestBuildMatrix_Deterministic(t *testing.T) {
	specs := []string{"[US-001]", "[US-002]", "[EP-001]"}
	codeRefs := map[string][]string{
		"Alpha": {"[US-001]", "[EP-001]"},
		"Beta":  {"[US-001]", "[US-002]"},
		"Gamma": {"[EP-001]"},
	}

	first := sdd.BuildMatrix(specs, codeRefs)
	firstCov := first.Coverage("[US-001]")

	for i := 0; i < 20; i++ {
		m := sdd.BuildMatrix(specs, codeRefs)
		got := m.Coverage("[US-001]")
		if !reflect.DeepEqual(got, firstCov) {
			t.Fatalf("non-deterministic Coverage on run %d: got %v, want %v", i, got, firstCov)
		}
		uncov := m.Uncovered(specs)
		firstUncov := first.Uncovered(specs)
		if !reflect.DeepEqual(uncov, firstUncov) {
			t.Fatalf("non-deterministic Uncovered on run %d: got %v, want %v", i, uncov, firstUncov)
		}
		orphans := m.Orphans(codeRefs)
		firstOrphans := first.Orphans(codeRefs)
		if !reflect.DeepEqual(orphans, firstOrphans) {
			t.Fatalf("non-deterministic Orphans on run %d: got %v, want %v", i, orphans, firstOrphans)
		}
	}
}

// ─── WhatImplements ───────────────────────────────────────────────────────────

func TestWhatImplements_Found(t *testing.T) {
	specs := []string{"[US-001]"}
	codeRefs := map[string][]string{
		"HandleLogin":  {"[US-001]"},
		"ValidateUser": {"[US-001]"},
		"HandleSignup": {"[US-002]"},
	}
	m := sdd.BuildMatrix(specs, codeRefs)

	got := sdd.WhatImplements(m, "[US-001]")
	if !contains(got, "HandleLogin") {
		t.Fatalf("WhatImplements([US-001]) = %v, want HandleLogin", got)
	}
	if !contains(got, "ValidateUser") {
		t.Fatalf("WhatImplements([US-001]) = %v, want ValidateUser", got)
	}
}

func TestWhatImplements_NotFound(t *testing.T) {
	specs := []string{"[US-001]"}
	codeRefs := map[string][]string{}
	m := sdd.BuildMatrix(specs, codeRefs)

	got := sdd.WhatImplements(m, "[US-001]")
	if len(got) != 0 {
		t.Fatalf("WhatImplements for uncovered spec should be empty, got %v", got)
	}
}

// ─── SpecStatus ───────────────────────────────────────────────────────────────

func TestSpecStatus_Covered(t *testing.T) {
	specs := []string{"[US-001]"}
	codeRefs := map[string][]string{
		"HandleLogin": {"[US-001]"},
	}
	m := sdd.BuildMatrix(specs, codeRefs)

	got := sdd.SpecStatus(m, "[US-001]")
	if got != "covered" {
		t.Fatalf("SpecStatus = %q, want %q", got, "covered")
	}
}

func TestSpecStatus_Uncovered(t *testing.T) {
	specs := []string{"[US-001]"}
	codeRefs := map[string][]string{}
	m := sdd.BuildMatrix(specs, codeRefs)

	got := sdd.SpecStatus(m, "[US-001]")
	if got != "uncovered" {
		t.Fatalf("SpecStatus = %q, want %q", got, "uncovered")
	}
}

func TestSpecStatus_UnknownSpec_Uncovered(t *testing.T) {
	m := sdd.BuildMatrix(nil, nil)
	got := sdd.SpecStatus(m, "[UNKNOWN-1]")
	if got != "uncovered" {
		t.Fatalf("SpecStatus for unknown spec = %q, want %q", got, "uncovered")
	}
}

// ─── Sorted output guarantees ─────────────────────────────────────────────────

func TestCoverage_ReturnsSorted(t *testing.T) {
	specs := []string{"[US-001]"}
	codeRefs := map[string][]string{
		"Zebra": {"[US-001]"},
		"Alpha": {"[US-001]"},
		"Mango": {"[US-001]"},
	}
	m := sdd.BuildMatrix(specs, codeRefs)
	got := m.Coverage("[US-001]")
	want := []string{"Alpha", "Mango", "Zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Coverage not sorted: got %v, want %v", got, want)
	}
}

func TestUncovered_ReturnsSorted(t *testing.T) {
	specs := []string{"[US-003]", "[US-001]", "[US-002]"}
	codeRefs := map[string][]string{}
	m := sdd.BuildMatrix(specs, codeRefs)
	got := m.Uncovered(specs)
	want := []string{"[US-001]", "[US-002]", "[US-003]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Uncovered not sorted: got %v, want %v", got, want)
	}
}

func TestOrphans_ReturnsSorted(t *testing.T) {
	specs := []string{"[US-001]"}
	codeRefs := map[string][]string{
		"FnC": {"[US-999]"},
		"FnA": {"[US-777]"},
		"FnB": {"[US-888]"},
	}
	m := sdd.BuildMatrix(specs, codeRefs)
	got := m.Orphans(codeRefs)
	want := []string{"[US-777]", "[US-888]", "[US-999]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Orphans not sorted: got %v, want %v", got, want)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
