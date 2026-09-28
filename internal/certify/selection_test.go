package certify

import (
	"reflect"
	"testing"
)

func TestSelectCertifyingFirstCoversTouched(t *testing.T) {
	// Two touched targets ("widget:Render" via its own resolved symbol
	// being changed, "widget:Parse" via its certifying test's reach
	// touching a changed symbol) and one untouched target
	// ("widget:Unrelated"). One test certifies both touched targets at
	// once — the greedy cover should pick it alone, over the two
	// single-target tests that could otherwise cover the same ground.
	changed := []string{"pkg.Render", "pkg.helper"}
	targetSymbols := map[string][]string{
		"widget:Render":    {"pkg.Render"},
		"widget:Parse":     {"pkg.Parse"},
		"widget:Unrelated": {"pkg.Unrelated"},
	}
	candidates := []CertifyingTest{
		{Test: "pkg.TestBoth", Targets: []string{"widget:Render", "widget:Parse"}, Reach: []string{"pkg.Render", "pkg.Parse"}},
		{Test: "pkg.TestRenderOnly", Targets: []string{"widget:Render"}, Reach: []string{"pkg.Render"}},
		{Test: "pkg.TestParseOnly", Targets: []string{"widget:Parse"}, Reach: []string{"pkg.Parse", "pkg.helper"}},
		{Test: "pkg.TestUnrelated", Targets: []string{"widget:Unrelated"}, Reach: []string{"pkg.Unrelated"}},
	}

	got := SelectCertifyingFirst(changed, candidates, targetSymbols, false)

	wantTouched := []string{"widget:Parse", "widget:Render"}
	if !reflect.DeepEqual(got.TouchedTargets, wantTouched) {
		t.Fatalf("TouchedTargets = %+v, want %+v", got.TouchedTargets, wantTouched)
	}
	if len(got.Uncovered) != 0 {
		t.Fatalf("Uncovered = %+v, want none", got.Uncovered)
	}
	// TestBoth covers both touched targets in one test — the greedy set
	// cover (most yet-uncovered targets first) must prefer it over the
	// two narrower tests.
	if len(got.Tests) != 1 || got.Tests[0] != "pkg.TestBoth" {
		t.Fatalf("Tests = %+v, want [pkg.TestBoth]", got.Tests)
	}
	wantReproves := []string{"widget:Parse", "widget:Render"}
	if !reflect.DeepEqual(got.Reproves["pkg.TestBoth"], wantReproves) {
		t.Errorf("Reproves[pkg.TestBoth] = %+v, want %+v", got.Reproves["pkg.TestBoth"], wantReproves)
	}
	wantSkipped := []string{"pkg.TestParseOnly", "pkg.TestRenderOnly", "pkg.TestUnrelated"}
	if !reflect.DeepEqual(got.Skipped, wantSkipped) {
		t.Errorf("Skipped = %+v, want %+v", got.Skipped, wantSkipped)
	}
}

func TestSelectCertifyingFirst_DeterministicTiebreak(t *testing.T) {
	// Two single-target tests, each covering a DIFFERENT one of the two
	// touched targets, tied at 1-target-covered-per-pick each round — the
	// result (which one is picked first) must not depend on input order,
	// only on name.
	changed := []string{"pkg.A", "pkg.B"}
	targetSymbols := map[string][]string{
		"t:A": {"pkg.A"},
		"t:B": {"pkg.B"},
	}
	forward := []CertifyingTest{
		{Test: "pkg.TestA", Targets: []string{"t:A"}, Reach: []string{"pkg.A"}},
		{Test: "pkg.TestB", Targets: []string{"t:B"}, Reach: []string{"pkg.B"}},
	}
	reversed := []CertifyingTest{forward[1], forward[0]}

	gotForward := SelectCertifyingFirst(changed, forward, targetSymbols, false)
	gotReversed := SelectCertifyingFirst(changed, reversed, targetSymbols, false)
	if !reflect.DeepEqual(gotForward.Tests, gotReversed.Tests) {
		t.Fatalf("selection depends on candidate order: forward=%+v reversed=%+v", gotForward.Tests, gotReversed.Tests)
	}
	want := []string{"pkg.TestA", "pkg.TestB"}
	if !reflect.DeepEqual(gotForward.Tests, want) {
		t.Errorf("Tests = %+v, want %+v", gotForward.Tests, want)
	}
}

func TestSelectCertifyingFirst_UncoveredTargetNamed(t *testing.T) {
	// A target touched by symbol overlap with no certifying test at all
	// (a governed symbol changed, but nothing certifies it) must be named
	// in Uncovered, never silently dropped.
	changed := []string{"pkg.Orphaned"}
	targetSymbols := map[string][]string{"t:Orphaned": {"pkg.Orphaned"}}
	got := SelectCertifyingFirst(changed, nil, targetSymbols, false)
	if len(got.Tests) != 0 {
		t.Fatalf("Tests = %+v, want none", got.Tests)
	}
	if !reflect.DeepEqual(got.Uncovered, []string{"t:Orphaned"}) {
		t.Fatalf("Uncovered = %+v, want [t:Orphaned]", got.Uncovered)
	}
}

func TestSelectCertifyingFirst_SafeAddsReachingTests(t *testing.T) {
	changed := []string{"pkg.Render"}
	targetSymbols := map[string][]string{"t:Render": {"pkg.Render"}}
	candidates := []CertifyingTest{
		{Test: "pkg.TestRender", Targets: []string{"t:Render"}, Reach: []string{"pkg.Render"}},
		// Reaches the changed symbol but certifies nothing — excluded
		// without --safe, added with it.
		{Test: "pkg.TestIncidental", Reach: []string{"pkg.Render"}},
		// Reaches nothing changed — never added either way.
		{Test: "pkg.TestUnrelated", Reach: []string{"pkg.Other"}},
	}

	unsafe := SelectCertifyingFirst(changed, candidates, targetSymbols, false)
	want := []string{"pkg.TestRender"}
	if !reflect.DeepEqual(unsafe.Tests, want) {
		t.Fatalf("unsafe Tests = %+v, want %+v", unsafe.Tests, want)
	}

	safe := SelectCertifyingFirst(changed, candidates, targetSymbols, true)
	wantSafe := []string{"pkg.TestIncidental", "pkg.TestRender"}
	if !reflect.DeepEqual(safe.Tests, wantSafe) {
		t.Fatalf("safe Tests = %+v, want %+v", safe.Tests, wantSafe)
	}
}

func TestSelectCertifyingFirst_NoChangeSelectsNothing(t *testing.T) {
	got := SelectCertifyingFirst(nil, []CertifyingTest{
		{Test: "pkg.TestA", Targets: []string{"t:A"}, Reach: []string{"pkg.A"}},
	}, map[string][]string{"t:A": {"pkg.A"}}, true)
	if len(got.Tests) != 0 || len(got.TouchedTargets) != 0 {
		t.Fatalf("got = %+v, want an empty selection for no changed symbols", got)
	}
}
