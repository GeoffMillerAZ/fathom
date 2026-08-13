package scope

import (
	"strings"
	"testing"
	"time"
)

func mustSet(t *testing.T, sups ...Suppression) *Set {
	t.Helper()
	set, err := NewSet(sups)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	return set
}

// TestSuppressionIsDisclosed is SP-3: ApplySuppressions returns a non-nil
// Disclosure whenever it dropped anything — count, human reason naming the
// pattern (and the expiry when time-boxed), and the suppression ids — and
// nil when it dropped nothing, so "hidden and not explained" is impossible
// to express.
func TestSuppressionIsDisclosed(t *testing.T) {
	exp := fixedTime.Add(48 * time.Hour)
	set := mustSet(
		t,
		Suppression{ID: "s1", Service: "svc", Pattern: "vendor/**", Reason: "third-party corpus", CreatedAt: fixedTime, ExpiresAt: &exp, CreatedBy: OriginCLI},
	)

	items := []string{"vendor/a/one.go", "internal/keep.go", "vendor/b/two.go:14"}
	kept, d := ApplySuppressions(set, "svc", fixedTime, items, func(s string) string { return s })
	if len(kept) != 1 || kept[0] != "internal/keep.go" {
		t.Fatalf("kept = %v, want only the unmatched ref", kept)
	}
	if d == nil {
		t.Fatal("two rows were dropped with a nil disclosure — a silent filter is indistinguishable from a recall bug")
	}
	if d.Count != 2 {
		t.Errorf("disclosure count = %d, want 2", d.Count)
	}
	if !strings.Contains(d.Reason, "third-party corpus") || !strings.Contains(d.Reason, "vendor/**") {
		t.Errorf("reason %q does not name the reason and the pattern", d.Reason)
	}
	if !strings.Contains(d.Reason, exp.UTC().Format("2006-01-02")) {
		t.Errorf("reason %q does not disclose the expiry of a time-boxed suppression", d.Reason)
	}
	if len(d.IDs) != 1 || d.IDs[0] != "s1" {
		t.Errorf("ids = %v, want [s1] so unsuppress is one copy-paste away", d.IDs)
	}

	// Nothing dropped → nil disclosure, and the input is returned intact.
	kept2, d2 := ApplySuppressions(set, "svc", fixedTime, []string{"internal/a.go"}, func(s string) string { return s })
	if d2 != nil {
		t.Fatalf("nothing was hidden but a disclosure was produced: %+v", d2)
	}
	if len(kept2) != 1 {
		t.Fatalf("kept2 = %v", kept2)
	}

	// A service-scoped suppression never touches another service.
	kept3, d3 := ApplySuppressions(set, "other", fixedTime, items, func(s string) string { return s })
	if len(kept3) != 3 || d3 != nil {
		t.Fatalf("a suppression scoped to svc filtered service other: kept=%v d=%+v", kept3, d3)
	}
}

// TestApplySuppressionsExpiryPerCall: the same Set filters at one clock and
// not after expiry — the read-time judgement inside the filter itself.
func TestApplySuppressionsExpiryPerCall(t *testing.T) {
	exp := fixedTime.Add(time.Hour)
	set := mustSet(
		t,
		Suppression{ID: "s1", Pattern: "vendor/**", Reason: "r", CreatedAt: fixedTime, ExpiresAt: &exp, CreatedBy: OriginCLI},
	)
	items := []string{"vendor/a.go"}
	if kept, _ := ApplySuppressions(set, "svc", fixedTime, items, func(s string) string { return s }); len(kept) != 0 {
		t.Fatal("active suppression did not filter")
	}
	if kept, d := ApplySuppressions(set, "svc", exp.Add(time.Minute), items, func(s string) string { return s }); len(kept) != 1 || d != nil {
		t.Fatal("expired suppression still filtered — expiry must be judged per call")
	}
}

func TestNormalizeRef(t *testing.T) {
	cases := map[string]string{
		"vendor/a.go":     "vendor/a.go",
		"vendor/a.go:12":  "vendor/a.go",
		"./vendor/a.go":   "vendor/a.go",
		"./vendor/a.go:9": "vendor/a.go",
		"pkg/v2/file.go":  "pkg/v2/file.go",
		"weird:name":      "weird:name",
		"weird:name:12":   "weird:name",
		"docs/adr-013.md": "docs/adr-013.md",
		"a/b/c.py:1":      "a/b/c.py",
	}
	for in, want := range cases {
		if got := NormalizeRef(in); got != want {
			t.Errorf("NormalizeRef(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDisclosureMergeIsDeterministic: merging accumulates counts and keeps
// reasons/ids unique and sorted — same inputs, same rendered sentence.
func TestDisclosureMergeIsDeterministic(t *testing.T) {
	a := &Disclosure{Count: 1, Reasons: []string{"b (y/**)"}, IDs: []string{"i2"}}
	b := &Disclosure{Count: 2, Reasons: []string{"a (x/**)", "b (y/**)"}, IDs: []string{"i1", "i2"}}
	a.Merge(b)
	if a.Count != 3 {
		t.Errorf("count = %d, want 3", a.Count)
	}
	if a.Reason != "a (x/**); b (y/**)" {
		t.Errorf("reason = %q", a.Reason)
	}
	if strings.Join(a.IDs, ",") != "i1,i2" {
		t.Errorf("ids = %v", a.IDs)
	}
}
