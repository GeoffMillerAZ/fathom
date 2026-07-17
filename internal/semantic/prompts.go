package semantic

// prompts.go — content-addressed prompt identity (M1-4/M1-5,
// docs/plans/m1-manifest.md; generalized to prompt-as-data by LC-1,
// docs/plans/layer-catalog.md). Every #Layer instance now carries its own
// prompt_file (schema/semantic/schema.cue); promptForLayer loads that file's
// bytes and derives PromptVersion = "<layer>/" + sha256(bytes)[:8] — a prompt
// edit is AUTOMATICALLY a new compile identity, closing the v1→v2 bug class
// this whole mechanism exists to prevent (a hand-bumped const that silently
// didn't change in lockstep with the text it was supposed to identify).
//
// LC-1's identity pin: this refactor moved the obligations/findings prompt
// TEXT byte-for-byte into what is now (LC-4's pack-shape migration)
// schema/semantic/layers/{obligations,findings}/prompt.md (extracted
// verbatim from the retired obligationsSystemPromptTemplate/
// findingsSystemPromptTemplate Go vars — see TestPromptFilesMatchSource and
// TestObligationsPromptVersion_Pinned) — zero identity rotation, no fleet
// recompile triggered by either change.
//
// FT-1's per-layer switch statement in promptForLayer (a name→prompt hard
// wire, one case per layer) is gone with it: an unwired-but-manifest-valid
// layer used to silently compile under the obligations prompt — a
// correctness bug, not just missing polish (docs/plans/layer-catalog.md).
// promptForLayer is now data-driven and layer-agnostic; an unknown layer
// name or a missing/unresolvable prompt_file is a loud, typed error.

import (
	"fmt"
	"strings"

	prompttemplate "github.com/geoffmilleraz/agentic-kit/foundations/prompt/prompt-template"
)

// promptFileRoot is the repo-relative directory schema.cue's #Layer.prompt_file
// values are rooted at for the layers this package embeds (LC-4:
// schema/semantic/layers/, mirrored as internal/semantic/layers/ —
// manifest.go's embeddedLayers). prompt_file is always repo-relative
// (ADR-017 convention, matching ClaimLink.TargetRef and
// #Layer.corpus.include); this constant is the ONLY place that repo-relative
// convention is translated into the embedded FS's own (necessarily
// package-local) directory layout.
const promptFileRoot = "schema/semantic/layers/"

// promptForLayer resolves the compile-time system prompt text and its
// content-addressed version identity for a manifest layer definition. Every
// #Layer instance carries its own prompt_file (LC-1) — there is no more
// per-layer Go wiring, and therefore no more silent fallback: a layer with no
// prompt_file configured, or one this package cannot resolve to embedded
// bytes, is a loud error, not a quiet compile under the wrong prompt.
func promptForLayer(def LayerDef) (systemPrompt, promptVersion string, err error) {
	if def.Name == "" {
		return "", "", fmt.Errorf("semantic: promptForLayer: layer definition has no name")
	}
	if def.PromptFile == "" {
		return "", "", fmt.Errorf("semantic: layer %q: manifest has no prompt_file configured", def.Name)
	}
	rel := strings.TrimPrefix(def.PromptFile, promptFileRoot)
	if rel == def.PromptFile {
		return "", "", fmt.Errorf("semantic: layer %q: prompt_file %q is not under the embedded pack root %q (project-local prompt_file resolution is not yet supported for this layer)", def.Name, def.PromptFile, promptFileRoot)
	}
	data, rerr := embeddedLayers.ReadFile("layers/" + rel)
	if rerr != nil {
		return "", "", fmt.Errorf("semantic: layer %q: prompt_file %q not found among embedded packs: %w", def.Name, def.PromptFile, rerr)
	}
	text := string(data)
	// TextHash is sha256 of the exact Text bytes (agentic-kit prompttemplate
	// contract) — reused here rather than re-implemented so the hash formula
	// stays the single one the rest of the fleet already audits.
	tpl := prompttemplate.Template{Name: def.Name + "-system-prompt", Version: "1", Text: text}
	return text, def.Name + "/" + tpl.TextHash()[:8], nil
}

// obligationsPromptVersion returns the built-in obligations layer's
// content-addressed compile identity. Kept as a named helper (tests and
// compile.go's no-Layer-configured default path both want the obligations
// identity without re-deriving it inline); production code otherwise always
// resolves a layer's prompt via LoadLayer + promptForLayer like every other
// layer — this is not a second code path, just a named shortcut through the
// same one.
func obligationsPromptVersion() string {
	v, err := layerPromptVersion(LayerObligations)
	if err != nil {
		// The built-in obligations layer's manifest entry and embedded
		// prompt file are shipped in the same binary — this can only fail
		// if the binary itself is broken (drift-tested at build time), so a
		// panic here is the same class of guarantee as a failed
		// go:embed compile.
		panic(fmt.Errorf("semantic: built-in obligations layer failed to resolve: %w", err))
	}
	return v
}

// findingsPromptVersion mirrors obligationsPromptVersion for the findings
// layer.
func findingsPromptVersion() string {
	v, err := layerPromptVersion(LayerFindings)
	if err != nil {
		panic(fmt.Errorf("semantic: built-in findings layer failed to resolve: %w", err))
	}
	return v
}

func layerPromptVersion(name string) (string, error) {
	def, err := LoadLayer(name)
	if err != nil {
		return "", err
	}
	_, v, err := promptForLayer(def)
	return v, err
}
