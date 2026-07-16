package semantic

import (
	"strings"
	"testing"
)

// ─── O8/OP-1: prompt v3 — file-class scoping ────────────────────────────────

// TestObligationsPrompt_FileClassScoping asserts the v3 prompt instructs the
// model to scope every rule to the FILE CLASS its governing document
// actually addresses (code rules → source globs, docs rules → markdown
// globs) and to never emit a bare "**" or other catch-all scope. This is the
// compile-time-precision half of O8: without it, the model has no notion of
// file-class and compiles repo-wide scopes for rules that only make sense for
// one file kind (docs/dogfood-observations.md O8, docs/vision/04 Gap 3).
func TestObligationsPrompt_FileClassScoping(t *testing.T) {
	text := obligationsSystemPromptTemplate.Text

	mustContain := []string{
		"FILE CLASS",
		"SOURCE globs",
		"MARKDOWN globs",
		`"**/*.go"`,
		`"**/*.md"`,
		`NEVER emit a bare`,
	}
	for _, want := range mustContain {
		if !strings.Contains(text, want) {
			t.Errorf("obligations prompt v3 missing file-class scoping instruction: %q\n---\n%s", want, text)
		}
	}

	// v2 instructions must survive the v3 rewrite: binding-rules-only, the
	// 28-word cap, narrowest-scope preference, and the {"claims": [...]}
	// JSON envelope.
	mustKeep := []string{
		`{"claims": [...]}`,
		"<= 28 words",
		"NARROWEST glob",
		"Emit {\"claims\": []} for sections that contain no binding developer rules",
	}
	for _, want := range mustKeep {
		if !strings.Contains(text, want) {
			t.Errorf("obligations prompt v3 dropped a v2 instruction: %q", want)
		}
	}
}

// TestObligationsPromptVersion_RotatedFromCommittedV2Hash pins that the v3
// prompt-text edit changed the content-addressed compile identity from the
// value it carried before this change (the v2 prompt's TextHash-derived
// PromptVersion, "obligations/92c5294c"). PromptVersion rotates automatically
// via TextHash (see prompts.go) — this test is the regression guard that the
// v1→v2 hand-bumped-const bug (docs/dogfood-observations.md, M1-4/M1-5)
// cannot recur: a Text edit with no matching identity bump would leave every
// already-compiled section on stale (pre-v3) claims forever.
func TestObligationsPromptVersion_RotatedFromCommittedV2Hash(t *testing.T) {
	const priorCommittedVersion = "obligations/92c5294c"
	if got := obligationsPromptVersion(); got == priorCommittedVersion {
		t.Fatalf("obligationsPromptVersion() = %q, want it to differ from the pre-v3 committed identity %q (TextHash must rotate on every prompt-text edit)", got, priorCommittedVersion)
	}
}
