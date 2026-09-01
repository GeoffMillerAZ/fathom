package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

func TestRetargetUtility(t *testing.T) {
	cases := []struct {
		in     string
		action retargetAction
		result string
	}{
		{"bg-indigo-600", actionMap, "bg-accent"},
		{"text-gray-900", actionMap, "text-ink"},
		{"text-gray-500", actionMap, "text-ink-muted"},
		{"text-gray-300", actionMap, "text-ink-faint"},
		{"bg-white", actionMap, "bg-surface"},
		{"bg-gray-50", actionMap, "bg-surface"},
		{"bg-gray-100", actionMap, "bg-surface-sunken"},
		{"border-gray-200", actionMap, "border-line"},
		{"border-gray-500", actionMap, "border-line-strong"},
		{"ring-indigo-500", actionMap, "ring-focus"},
		{"bg-red-600", actionMap, "bg-danger"},
		{"text-green-700", actionMap, "text-positive"},
		{"bg-amber-400", actionMap, "bg-caution"},
		{"hover:bg-indigo-700", actionMap, "hover:bg-accent"}, // variant preserved
		{"dark:bg-gray-800", actionDrop, ""},                  // dark color variant dropped
		{"flex", actionKeep, ""},                              // not a color utility
		{"p-4", actionKeep, ""},                               // not a color utility
		{"bg-[#ff0000]", actionKeep, ""},                      // arbitrary value, no guess
		{"bg-gray-800", actionKeep, ""},                       // ambiguous dark panel, no guess
		{"text-transparent", actionKeep, ""},                  // special, no role
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := retargetUtility(c.in)
			if got.Action != c.action {
				t.Fatalf("retargetUtility(%q) action = %q, want %q (note: %s)", c.in, got.Action, c.action, got.Note)
			}
			if c.action == actionMap && got.Result != c.result {
				t.Errorf("retargetUtility(%q) result = %q, want %q", c.in, got.Result, c.result)
			}
		})
	}
}

func TestRetargetClassList(t *testing.T) {
	in := "dark:bg-gray-800 bg-white text-gray-900 ring-indigo-500 border-gray-200 flex p-4"
	_, rewritten := RetargetClassList(in)
	want := "bg-surface text-ink ring-focus border-line flex p-4"
	if rewritten != want {
		t.Errorf("rewritten = %q, want %q", rewritten, want)
	}
}

// TestMappingTargetsAreRealRoles guards against drift: every role the mapping
// can emit must exist in the kit's generated token role set. If a role is
// renamed in CUE, this fails instead of the Assistant recommending a dead class.
func TestMappingTargetsAreRealRoles(t *testing.T) {
	valid := map[string]bool{}
	for _, r := range tokens.ColorNames {
		valid[r] = true
	}
	// Collect every role the mapping tables / neutralRole can produce.
	produced := map[string]bool{}
	for _, r := range semanticFamily {
		produced[r] = true
	}
	produced["focus"] = true // accent ring special-case
	for _, prop := range []string{"bg", "text", "border"} {
		for _, shade := range []int{0, 50, 100, 300, 400, 600, 900} {
			if role, ok, _ := neutralRole(prop, shade); ok {
				produced[role] = true
			}
		}
	}
	for role := range produced {
		if !valid[role] {
			t.Errorf("mapping emits role %q which is not a kit token role (%v)", role, tokens.ColorNames)
		}
	}
}

func TestThemingGuideMentionsKeyRules(t *testing.T) {
	g := themingGuide()
	for _, want := range []string{"bg-accent", "dark:", "ring-focus", "No Node", "surface"} {
		if !strings.Contains(g, want) {
			t.Errorf("theming guide missing %q", want)
		}
	}
}

// TestThemingGuideDoesNotBanInlineColor — get_kit_theming is reached at the
// exact moment an agent is choosing colors, and until 2026-08-07 it said "No
// Node, no inline color styles. Use utilities only (D2)". The kit's own
// appkit/ui/viz breaks that on every element it emits: zero class=, every color
// a var(--ak-color-*) presentation attribute. An agent obeying the guidance
// could not ship an L3a chart, and an agent that shipped one was told it had
// violated D2. The kit has two theming tiers (ADR-0017 §1) and this is the
// surface that has to name both.
func TestThemingGuideDoesNotBanInlineColor(t *testing.T) {
	g := themingGuide()
	lower := strings.ToLower(g)

	// Matched as phrases, not words: the guide legitimately says "non-color
	// `dark:` utilities only if truly needed" and "these role utilities only
	// resolve if…", neither of which is the retired rule.
	for _, banned := range []string{"use utilities only", "inline color styles"} {
		if strings.Contains(lower, banned) {
			t.Errorf("theming guide still says %q — it contradicts appkit/ui/viz, which is the kit's own L3a tier", banned)
		}
	}
	for _, want := range []string{"L3a", "L3b", "var(--ak-color-", "literal"} {
		if !strings.Contains(g, want) {
			t.Errorf("theming guide does not mention %q — the tier split has to be legible here, not just in the ADR", want)
		}
	}
	// The half of the old rule that was always right.
	if !strings.Contains(g, "No Node") {
		t.Error("theming guide dropped the no-Node half of rule 5; only the utilities-only half was wrong")
	}
}

// TestThemingGuideMatchesVizReality is the other end of the same claim: the
// guide asserts ui/viz emits no class= and only var(--ak-color-*). If viz ever
// drifts, appkit/ui/viz's own TestVizEmitsNoClassAttribute fails first — this
// records that the guide depends on it, so the dependency is findable from
// here.
func TestThemingGuideMatchesVizReality(t *testing.T) {
	if !strings.Contains(themingGuide(), "appkit/ui/viz") {
		t.Error("theming guide must name the package whose behavior it is describing")
	}
}
