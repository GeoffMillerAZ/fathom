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
