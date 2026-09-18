package dashboard

import (
	"strings"
	"testing"
)

// TestSkipLinkAndContentTargetAreTheKitsJob — accessibility that every consumer
// re-implements is accessibility that some consumer omits, which is what was
// happening: the portal carried its own RenderSkipLink and the other consumers
// carried nothing.
func TestSkipLinkAndContentTargetAreTheKitsJob(t *testing.T) {
	// Default: opt-in, so a shell embedded in a page that already has a skip
	// link does not emit a second one. Two skip links is worse than none — the
	// second sends a keyboard user somewhere they did not ask to go.
	quiet := RenderShell(Shell{Title: "T", Content: "c"})
	if strings.Contains(quiet, "Skip to content") {
		t.Error("the shell emitted a skip link nobody asked for")
	}
	if !strings.Contains(quiet, `<main id="content"`) {
		t.Errorf("main carries no default id, so an app-supplied skip link has nowhere to jump:\n%s", quiet)
	}

	got := RenderShell(Shell{Title: "T", Content: "c", SkipLink: true})
	if !strings.Contains(got, `href="#content"`) {
		t.Errorf("the skip link does not target the content id:\n%s", got)
	}
	if !strings.Contains(got, `<main id="content" tabindex="-1"`) {
		t.Error("the skip destination must accept focus without joining the normal tab order")
	}
	// FIRST in document order. "The first focusable element" is a statement
	// about source order, and a link nested after the sidebar is one the reader
	// tabs to after the thing it exists to skip.
	if !strings.HasPrefix(got, `<a href="#content"`) {
		t.Errorf("the skip link is not the first element; a keyboard user reaches it after the nav it skips:\n%.200s", got)
	}
	// Hidden until focused: the only treatment that serves a screen-reader user
	// and a sighted keyboard user without adding a control nobody else needs.
	if !strings.Contains(got, "sr-only focus:not-sr-only") {
		t.Error("the skip link is not visually hidden until focused")
	}

	// A custom id moves BOTH ends together, or the link points at nothing.
	custom := RenderShell(Shell{Title: "T", Content: "c", SkipLink: true, ContentID: "atrium-main"})
	if !strings.Contains(custom, `href="#atrium-main"`) || !strings.Contains(custom, `<main id="atrium-main"`) {
		t.Errorf("ContentID did not move both the link and its target:\n%.300s", custom)
	}
	if !strings.Contains(custom, `<main id="atrium-main" tabindex="-1"`) {
		t.Error("custom content target lost programmatic focus support")
	}
}

func TestMobileNavigationSummaryHasTargetAndFocusTreatment(t *testing.T) {
	got := RenderShell(Shell{Title: "T", Nav: "nav", Content: "c"})
	for _, class := range []string{"min-h-[44px]", "focus-visible:outline-2", "focus-visible:outline-offset-2", "focus-visible:outline-focus"} {
		if !strings.Contains(got, class) {
			t.Errorf("mobile navigation summary is missing %q: %s", class, got)
		}
	}
}
