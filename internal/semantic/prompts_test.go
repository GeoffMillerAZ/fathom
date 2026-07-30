package semantic

import (
	"strings"
	"testing"
)

// obligationsPromptText loads the obligations layer's prompt text through the
// SAME path production code uses (LoadLayer + promptForLayer) — LC-1 retired
// the package-level obligationsSystemPromptTemplate var the v3-prompt tests
// used to read directly; prompt text is data now, not Go.
func obligationsPromptText(t *testing.T) string {
	t.Helper()
	def, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	text, _, err := promptForLayer(def)
	if err != nil {
		t.Fatalf("promptForLayer(obligations): %v", err)
	}
	return text
}

// ─── O8/OP-1: prompt v3 — file-class scoping ────────────────────────────────

// TestObligationsPrompt_FileClassScoping asserts the v3 prompt instructs the
// model to scope every rule to the FILE CLASS its governing document
// actually addresses (code rules → source globs, docs rules → markdown
// globs) and to never emit a bare "**" or other catch-all scope. This is the
// compile-time-precision half of O8: without it, the model has no notion of
// file-class and compiles repo-wide scopes for rules that only make sense for
// one file kind (docs/dogfood-observations.md O8, docs/vision/04 Gap 3).
func TestObligationsPrompt_FileClassScoping(t *testing.T) {
	text := obligationsPromptText(t)

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

// ─── LC-1: prompt-as-data, zero identity rotation ──────────────────────────

// TestObligationsPromptVersion_Pinned is the identity pin LC-1's HARD
// CONSTRAINT requires (docs/plans/layer-catalog.md): moving the prompt text
// from a Go var to a manifest-referenced file must NOT change the obligations
// layer's compile-time identity. This value was computed from the pre-LC-1
// code (obligationsSystemPromptTemplate.Text's TextHash) BEFORE the prompt
// text moved to schema/semantic/prompts/obligations.md — a mismatch here
// means the extraction was not byte-identical and every already-compiled
// obligations claim in the fleet would silently need a full recompile.
func TestObligationsPromptVersion_Pinned(t *testing.T) {
	const want = "obligations/c02b1aa1"
	if got := obligationsPromptVersion(); got != want {
		t.Fatalf("obligationsPromptVersion() = %q, want %q (LC-1 must not rotate the obligations compile identity)", got, want)
	}
}

// TestFindingsPromptVersion_Pinned mirrors the obligations pin for the
// findings layer's prompt, extracted the same way. Bumped by the
// findings-as-evidence class-vocabulary prompt edit (docs/design/findings-
// as-evidence.md §3.2, §6 item 2) — the prompt now instructs the reviewer
// to emit a "class" field, which is exactly the kind of change this pin
// exists to make loud rather than silent.
func TestFindingsPromptVersion_Pinned(t *testing.T) {
	const want = "findings/6400b14f"
	if got := findingsPromptVersion(); got != want {
		t.Fatalf("findingsPromptVersion() = %q, want %q (LC-1 must not rotate the findings compile identity)", got, want)
	}
}

// TestPromptForLayer_UnknownPromptFileIsLoud is the LC-1 correctness fix: an
// unknown layer name or missing prompt_file must be a loud, typed error —
// never a silent fallback to the obligations prompt (the bug FT-1's switch
// statement carried: an unwired-but-manifest-valid layer name silently
// compiled under the wrong prompt).
//
// Byte-identity of the embedded prompt files against their
// schema/semantic/layers/<name>/prompt.md source is TestPackFilesMatchSource
// (manifest_test.go, LC-4) — the pack-shape migration folded prompt.md into
// the same per-pack drift gate as layer.cue.
func TestPromptForLayer_UnknownPromptFileIsLoud(t *testing.T) {
	if _, _, err := promptForLayer(LayerDef{Name: "mystery"}); err == nil {
		t.Fatal("promptForLayer with no PromptFile: want an error, got nil")
	}
	if _, _, err := promptForLayer(LayerDef{Name: "mystery", PromptFile: "schema/semantic/layers/mystery/does-not-exist.md"}); err == nil {
		t.Fatal("promptForLayer with an unresolvable PromptFile: want an error, got nil")
	}
	if _, _, err := promptForLayer(LayerDef{}); err == nil {
		t.Fatal("promptForLayer with no Name: want an error, got nil")
	}
}
