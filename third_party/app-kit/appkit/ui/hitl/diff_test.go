package hitl

import (
	"regexp"
	"strings"
	"testing"
)

var hexColorRE = regexp.MustCompile(`(?i)(fill|stroke|color|background)[=:]["']?\s*#[0-9a-f]{3,8}\b`)

func TestDiffView_BeforeAfter(t *testing.T) {
	got := DiffView([]Change{
		{Label: "Status", Before: "open", After: "resolved"},
	})
	for _, want := range []string{
		`role="group"`, "Status", "open", "resolved",
		"border-danger bg-danger-soft",     // current (before)
		"border-positive bg-positive-soft", // proposed (after)
		"→",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("diff missing %q in:\n%s", want, got)
		}
	}
}

func TestDiffView_AdditionAndRemoval(t *testing.T) {
	// Empty Before = addition (placeholder on the left); empty After = removal.
	add := DiffView([]Change{{Label: "Tag", After: "urgent"}})
	if !strings.Contains(add, "—") || !strings.Contains(add, "urgent") {
		t.Errorf("addition should show a placeholder before + the new value: %s", add)
	}
	rm := DiffView([]Change{{Label: "Tag", Before: "draft"}})
	if !strings.Contains(rm, "draft") || strings.Count(rm, "—") != 1 {
		t.Errorf("removal should show old value + a placeholder after: %s", rm)
	}
}

func TestDiffView_EscapingAndTokensOnly(t *testing.T) {
	got := DiffView([]Change{{Label: `<b>`, Before: `<script>x</script>`, After: `"y`}})
	if strings.Contains(got, "<script>x</script>") || strings.Contains(got, "<b>") {
		t.Errorf("diff did not escape: %s", got)
	}
	if hexColorRE.MatchString(got) {
		t.Errorf("hardcoded hex color — must use token utilities: %s", got)
	}
}

func TestDiffView_Deterministic(t *testing.T) {
	c := []Change{{Label: "A", Before: "1", After: "2"}, {Label: "B", After: "new"}}
	if DiffView(c) != DiffView(c) {
		t.Error("DiffView is non-deterministic")
	}
}
