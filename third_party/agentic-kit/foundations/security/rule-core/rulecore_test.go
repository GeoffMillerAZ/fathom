package rulecore

import (
	"strings"
	"testing"
)

// TestCompile_RejectsInvalidPattern verifies that Compile returns an error
// when a pattern is not a valid Go regexp.
func TestCompile_RejectsInvalidPattern(t *testing.T) {
	specs := []PatternSpec{
		{ID: "T-001", Pattern: `valid`},
		{ID: "T-002", Pattern: `[invalid`},
	}
	_, err := Compile(specs)
	if err == nil {
		t.Fatal("expected error for invalid pattern, got nil")
	}
	if !strings.Contains(err.Error(), "T-002") {
		t.Errorf("error should mention rule ID T-002, got: %v", err)
	}
}

// TestScanFirst_FindsMatches verifies that ScanFirst returns one Finding per
// matching rule, in rule order, with correct RuleID and non-empty Excerpt.
func TestScanFirst_FindsMatches(t *testing.T) {
	specs := []PatternSpec{
		{ID: "R-001", Pattern: `foo`},
		{ID: "R-002", Pattern: `bar`},
		{ID: "R-003", Pattern: `baz`},
	}
	cs, err := Compile(specs)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	findings := cs.ScanFirst("the foo is near the bar now")
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if findings[0].RuleID != "R-001" {
		t.Errorf("findings[0].RuleID = %q, want R-001", findings[0].RuleID)
	}
	if findings[1].RuleID != "R-002" {
		t.Errorf("findings[1].RuleID = %q, want R-002", findings[1].RuleID)
	}
	if findings[0].Excerpt != "foo" {
		t.Errorf("findings[0].Excerpt = %q, want %q", findings[0].Excerpt, "foo")
	}
}

// TestScanFirst_NoMatch verifies that ScanFirst returns nil when no patterns match.
func TestScanFirst_NoMatch(t *testing.T) {
	specs := []PatternSpec{{ID: "X-001", Pattern: `xyz`}}
	cs, err := Compile(specs)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	findings := cs.ScanFirst("hello world")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// TestScanFirst_ExcerptCapped verifies that long matches are truncated to
// ExcerptMaxBytes and suffixed with "…".
func TestScanFirst_ExcerptCapped(t *testing.T) {
	// Build a pattern that matches a 200-byte string.
	long := strings.Repeat("a", 200)
	specs := []PatternSpec{{ID: "L-001", Pattern: `a+`}}
	cs, err := Compile(specs)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	findings := cs.ScanFirst(long)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	excerpt := findings[0].Excerpt
	if !strings.HasSuffix(excerpt, "…") {
		t.Errorf("expected excerpt to end with '…', got %q", excerpt)
	}
	// Byte length should be ExcerptMaxBytes + len("…") (3 bytes for UTF-8 ellipsis).
	if len(excerpt) != ExcerptMaxBytes+len("…") {
		t.Errorf("excerpt byte length = %d, want %d", len(excerpt), ExcerptMaxBytes+len("…"))
	}
}

// TestReplaceFixpoint_Converges verifies that a straightforward replacement
// terminates and converged == true.
func TestReplaceFixpoint_Converges(t *testing.T) {
	specs := []PatternSpec{
		{ID: "P-001", Pattern: `secret`},
	}
	cs, err := Compile(specs)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	result, converged := cs.ReplaceFixpoint("my secret value", map[string]string{"P-001": "[REDACTED]"}, 16)
	if !converged {
		t.Error("expected converged=true")
	}
	if strings.Contains(result, "secret") {
		t.Errorf("replacement did not fire: %q", result)
	}
	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("replacement not applied: %q", result)
	}
}

// TestReplaceFixpoint_MutualRetrigger verifies that when a replacement
// continuously re-triggers itself (growing without bound), the fixpoint is
// NOT reached within maxPasses and converged==false.
func TestReplaceFixpoint_MutualRetrigger(t *testing.T) {
	// Rule P-001 replaces "aa" with "aaa".
	// Each pass makes the string longer and re-triggers the rule next pass.
	// Starting from "aa": "aa" → "aaa" → "aaaa" → ... indefinitely.
	specs := []PatternSpec{
		{ID: "P-001", Pattern: `aa`},
	}
	cs, err := Compile(specs)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	replacements := map[string]string{
		"P-001": "aaa",
	}
	_, converged := cs.ReplaceFixpoint("aa", replacements, 4)
	if converged {
		t.Error("expected converged=false for self-re-triggering rule")
	}
}

// TestReplaceFixpoint_LiteralSafe verifies that "$" in replacement strings is
// never expanded as a backreference.
func TestReplaceFixpoint_LiteralSafe(t *testing.T) {
	specs := []PatternSpec{
		{ID: "Q-001", Pattern: `(secret)`},
	}
	cs, err := Compile(specs)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// Replacement contains "$1"; with ReplaceAllLiteralString it must be literal.
	result, _ := cs.ReplaceFixpoint("my secret", map[string]string{"Q-001": "[REDACTED:$1]"}, 16)
	want := "my [REDACTED:$1]"
	if result != want {
		t.Errorf("ReplaceFixpoint result = %q, want %q", result, want)
	}
}

// TestOrdinal_GTE verifies ordering for a three-level scale.
func TestOrdinal_GTE(t *testing.T) {
	ord := NewOrdinal([]string{"low", "medium", "high"})

	tests := []struct {
		a, b string
		want bool
	}{
		{"high", "low", true},
		{"high", "high", true},
		{"low", "high", false},
		{"medium", "low", true},
		{"medium", "medium", true},
		{"low", "medium", false},
	}
	for _, tc := range tests {
		got := ord.GTE(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("GTE(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestOrdinal_MaxReturnsHighest verifies Max returns the highest-ranked element.
func TestOrdinal_MaxReturnsHighest(t *testing.T) {
	ord := NewOrdinal([]string{"none", "low", "medium", "high"})

	levels := []string{"low", "high", "none", "medium"}
	got := ord.Max(levels)
	if got != "high" {
		t.Errorf("Max = %q, want %q", got, "high")
	}
}

// TestOrdinal_MaxEmptyReturnsEmpty verifies that Max returns "" for an empty slice.
func TestOrdinal_MaxEmptyReturnsEmpty(t *testing.T) {
	ord := NewOrdinal([]string{"low", "high"})
	got := ord.Max(nil)
	if got != "" {
		t.Errorf("Max(nil) = %q, want %q", got, "")
	}
	got = ord.Max([]string{})
	if got != "" {
		t.Errorf("Max([]) = %q, want %q", got, "")
	}
}
