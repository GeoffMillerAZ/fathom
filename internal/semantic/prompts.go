package semantic

// prompts.go — content-addressed prompt identity (M1-4/M1-5,
// docs/plans/m1-manifest.md). The obligations compile system prompt lives as
// an agentic-kit prompttemplate.Template; obligationsPromptVersion derives
// the compile-time identity from the template's TextHash instead of a
// hand-bumped const. The retired PromptVersion const ("obligations/v2") was
// exactly the failure mode this closes: the v1→v2 prompt edit forgot to bump
// the digest gate's key in lockstep, leaving 27/32 sections on stale v1
// claims. A Text edit below is now AUTOMATICALLY a new compile identity.

import (
	prompttemplate "github.com/geoffmilleraz/agentic-kit/foundations/prompt/prompt-template"
)

// obligationsSystemPromptTemplate is the obligations compile system prompt.
// It has no {{ }} placeholders — Template is used here purely for its
// content-addressed TextHash, not for Render.
var obligationsSystemPromptTemplate = prompttemplate.Template{
	Name:    "obligations-system-prompt",
	Version: "1",
	Text: `You compile engineering obligations from governing documents into strict JSON.
An obligation is a rule a developer editing matching files MUST follow while
writing code, restated faithfully from the document — never invented. Output
ONLY a JSON object {"claims": [...]} whose claims array (possibly empty) holds
objects of the shape {"rule", "scope", "cite_start", "cite_end"}.
- rule: imperative, <= 28 words, self-contained.
- scope: gitignore-style glob(s) for the code paths the rule governs. Prefer the
  NARROWEST glob the document justifies (a named package or directory); use
  ["**"] only for rules the document explicitly applies to all code.
- cite_start/cite_end: the 1-based DOCUMENT line range (shown in the input) that
  states the rule. Cite the narrowest range that fully supports the rule.
Emit {"claims": []} for sections that contain no binding developer rules. These
are NOT obligations: project metrics/gates ("achieve X%"), roadmap/vision
statements, descriptions of how the system works, decisions' context/rationale,
historical records. Only emit rules the cited lines actually state. No commentary.`,
}

// obligationsPromptVersion derives the obligations compile prompt's identity
// as "obligations/<first 8 hex chars of TextHash>". The digest gate
// (allDigestsMatch) keys on this, not a hand-maintained version string — a
// prompt-text edit is automatically a new compile identity for every
// section, killing the v1-to-v2 bug class (a hand-bumped const that silently
// didn't change in lockstep with the text it was supposed to identify).
func obligationsPromptVersion() string {
	return "obligations/" + obligationsSystemPromptTemplate.TextHash()[:8]
}

// findingsSystemPromptTemplate is the findings-layer compile system prompt
// (D3 tracer bullet, docs/plans/findings-tracer.md FT-1). It targets the same
// strict-JSON claim envelope as obligations ({"claims":[...]}, claimDraft
// shape) — the compiler is layer-aware, not envelope-aware (one schema, one
// decode path, for every layer) — but a finding's "rule" is the document's
// actionable remediation statement, not a developer obligation restated from
// a standard, and its "scope" is the affected code paths the FINDING names,
// not a rule's governing area.
var findingsSystemPromptTemplate = prompttemplate.Template{
	Name:    "findings-system-prompt",
	Version: "1",
	Text: `You compile review/audit findings from a findings corpus into strict JSON.
A finding document describes one reviewed issue: severity, affected area, and a
recommended remediation. Output ONLY a JSON object {"claims": [...]} whose claims
array (possibly empty) holds objects of the shape {"rule", "scope", "cite_start",
"cite_end"}.
- rule: the finding's actionable remediation statement, imperative, <= 28 words,
  self-contained — restated faithfully from the document, never invented.
- scope: gitignore-style glob(s) for the affected code paths the document states.
  Use the NARROWEST glob the document justifies (a named package, module, or
  file). A finding that genuinely applies repo-wide scopes to its affected file
  class (for example "**/*.go"), never a bare "**".
- cite_start/cite_end: the 1-based DOCUMENT line range (shown in the input) that
  states the finding's remediation. Cite the narrowest range that fully supports it.
Emit {"claims": []} for sections with no actionable remediation — severity/area
metadata alone, background/context prose, or a finding explicitly marked resolved
or withdrawn are NOT findings to restate. Only emit rules the cited lines actually
state. No commentary.`,
}

// findingsPromptVersion mirrors obligationsPromptVersion's content-addressed
// derivation for the findings layer's prompt identity: "findings/<first 8 hex
// chars of TextHash>".
func findingsPromptVersion() string {
	return "findings/" + findingsSystemPromptTemplate.TextHash()[:8]
}

// promptForLayer resolves the compile-time system prompt text and its
// content-addressed version identity for a manifest layer name (FT-1:
// generalizing the compiler past the single hardcoded obligations layer).
// Falls back to the obligations prompt for any layer name this package does
// not yet wire a prompt for — every caller (pkg/client.CompileLayer,
// cmd/fathom compile, the compile_layer MCP tool) routes the name through
// semantic.LoadLayer first, so an unwired-but-manifest-valid layer name
// silently compiling under the obligations prompt is a wiring gap for a test
// to catch, not a condition worth a runtime panic over.
func promptForLayer(layerName string) (systemPrompt, promptVersion string) {
	switch layerName {
	case LayerFindings:
		return findingsSystemPromptTemplate.Text, findingsPromptVersion()
	default:
		return obligationsSystemPromptTemplate.Text, obligationsPromptVersion()
	}
}
