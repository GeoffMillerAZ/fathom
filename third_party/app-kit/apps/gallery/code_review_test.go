package gallery

import (
	"net/http"
	"strings"
	"testing"
)

// TestCodeReviewPageShowsExcerptAndDiff pins the /agentic/code specimen to its own
// builder output (not the served shell) so a deleted specimen cannot pass on a
// string that survives in the navigation.
func TestCodeReviewPageShowsExcerptAndDiff(t *testing.T) {
	content, err := buildCodeReview()
	if err != nil {
		t.Fatal(err)
	}
	body := string(content)
	for _, want := range []string{
		"appkit/ui/code", "hitl.RenderUnifiedDiff", "hitl.DiffView",
		// Excerpt: every state, anchors, permalink, gap, custom label, reveal.
		`id="review-L47"`, `data-state="changed"`, `data-state="not-executed"`, `data-state="highlighted"`,
		`href="#review-L49"`, `Line 52 not shown`, `aria-label="Line states"`,
		`Reviewer note: `, `>U+202E</span>`, `>U+2066</span>`, `>U+2028</span>`, `>U+E0061</span>`,
		`href="/catalog/content.code-excerpt"`,
		"No lines to show in this excerpt.", `data-refusal>code: source link must be`,
		// Diff: computed hunks with collapsed context, precomputed gaps, no changes.
		`data-unified-diff`, `data-diff-collapsed`, `@@ -3,9 +3,10 @@`, `@@ -16,6 +17,6 @@`,
		`data-diff-omitted>Lines 1–5 not included</p>`, `data-diff-omitted>Not included: <bdi>main</bdi> 9–18, <bdi>branch</bdi> 10–19</p>`,
		`Added, proposed line 9: `, `Removed, current line 6: `,
		"No changes: the texts are identical.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/agentic/code specimen missing %q", want)
		}
	}
	if strings.Contains(body, "<table") {
		t.Error("code review specimens must stay table-free")
	}
	res, page := get(t, New(), "/agentic/code?theme=dark")
	if res.StatusCode != http.StatusOK || !strings.Contains(page, `data-theme="dark"`) || !strings.Contains(page, `href="/agentic?theme=dark" aria-current="page"`) {
		t.Fatalf("/agentic/code did not serve the dark shell with Agentic current: %d", res.StatusCode)
	}
	if res, _ := get(t, New(), "/agentic/code/extra"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown subpage path = %d, want 404", res.StatusCode)
	}
	agentic, err := buildAgentic()
	if err != nil || !strings.Contains(string(agentic), `href="/agentic/code"`) {
		t.Fatalf("/agentic must link its code review specimens: %v", err)
	}
}
