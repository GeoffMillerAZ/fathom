package chrome

import (
	"strings"
	"testing"
)

func TestTabs(t *testing.T) {
	got := Tabs([]Tab{
		{Label: "Sequence", Href: "?view=sequence", Active: true},
		{Label: "Condensed", Href: "?view=condensed"},
	}, "Timeline view")
	for _, want := range []string{
		`role="tablist"`, "Sequence", "Condensed", "?view=condensed",
		`aria-current="page"`, "border-accent", `aria-label="Timeline view"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tabs missing %q: %s", want, got)
		}
	}
	if Tabs(nil, "x") != "" {
		t.Errorf("empty tabs should be empty string")
	}
}

func TestTabs_Escaping(t *testing.T) {
	got := Tabs([]Tab{{Label: `<script>`, Href: `"x`}}, "")
	if strings.Contains(got, "<script>") {
		t.Errorf("tabs did not escape label")
	}
}

func TestSkeleton(t *testing.T) {
	got := Skeleton(3)
	if strings.Count(got, "animate-pulse") != 3 {
		t.Errorf("expected 3 shimmer bars, got: %s", got)
	}
	if !strings.Contains(got, `aria-busy="true"`) || !strings.Contains(got, "w-2/3") {
		t.Errorf("skeleton structure wrong: %s", got)
	}
	if strings.Count(Skeleton(0), "animate-pulse") != 1 {
		t.Errorf("Skeleton(0) should render one bar")
	}
}

func TestSkeletonCard(t *testing.T) {
	got := SkeletonCard(2)
	if !strings.Contains(got, "bg-surface-raised") || !strings.Contains(got, "animate-pulse") || !strings.Contains(got, `aria-busy="true"`) {
		t.Errorf("skeleton card structure wrong: %s", got)
	}
}

func TestSkeleton_Determinism(t *testing.T) {
	if Skeleton(4) != Skeleton(4) || SkeletonCard(3) != SkeletonCard(3) {
		t.Errorf("skeleton non-deterministic")
	}
}
