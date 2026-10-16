package prompttemplate

import (
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// ApplyTo copies the three prompt-derived provenance fields from the
// Rendered value onto an aiprovenance.AIAuthor:
//
//	Rendered.PromptHash      → AIAuthor.PromptHash
//	Rendered.TemplateVersion → AIAuthor.PromptTemplateVersion
//	Rendered.ParamsHash      → AIAuthor.ParamsHash
//
// All other AIAuthor fields (ModelID, ModelVersion, InputDigest, At) remain
// untouched — they describe the inference, not the prompt, and are the
// caller's responsibility. A nil receiver argument is a no-op.
//
// This is the only point of contact with the ai-provenance foundation. The
// coupling is one-way and optional: ai-provenance does not know this package
// exists, and consumers who do not record provenance never call ApplyTo.
//
// Typical use:
//
//	r, err := prompttemplate.Render(tpl, params)
//	// ... send r.Text to the model ...
//	author := aiprovenance.NewAI(modelID, modelVersion, "", "", inputDigest, at)
//	r.ApplyTo(author.AI)
func (r Rendered) ApplyTo(ai *aiprovenance.AIAuthor) {
	if ai == nil {
		return
	}
	ai.PromptHash = r.PromptHash
	ai.PromptTemplateVersion = r.TemplateVersion
	ai.ParamsHash = r.ParamsHash
}
