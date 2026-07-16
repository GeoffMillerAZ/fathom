// Package semantic defines the CUE schema for semantic-layer definitions
// (ADR-016 §4.4). A layer definition names its corpus, prompt version, output
// kind, and budgets — the manifest SHAPE exists from day one so that when a
// second layer earns the generic runtime, definitions generalize instead of
// being reinvented. V1 ships exactly one definition: the obligations layer.
//
// This file is authoritative AND live (M1, docs/plans/m1-manifest.md):
// internal/semantic embeds a byte-identical copy (internal/semantic/
// manifest_schema.cue, drift-tested against this file) and LoadLayer decodes
// it at runtime — corpus globs, tier, budgets, and serving caps here are the
// only source of truth; nothing hand-mirrors them in Go anymore.
package semantic

// #Layer is one semantic-layer definition.
#Layer: {
	// name identifies the layer ("obligations", "failure-mode", …).
	name: string & =~"^[a-z][a-z0-9-]*$"

	// version of the layer definition; part of every claim's provenance.
	version: string & =~"^v[0-9]+$"

	// kind of claim this layer emits (v1: "obligation").
	kind: string & =~"^[a-z][a-z0-9-]*$"

	// corpus selects the governing documents, as repo-relative
	// gitignore-style globs evaluated against the service's indexed files.
	corpus: {
		include: [...string] & [_, ...]
		exclude?: [...string]
	}

	// prompt_version is ADVISORY manifest metadata (M1 amendment, ADR-016).
	// For content-addressed layers (obligations), the runtime's real
	// compile-time prompt identity is derived from the Go
	// prompttemplate.Template text via TextHash — see
	// internal/semantic/prompts.go's obligationsPromptVersion — NOT this
	// field. It is kept so every #Layer instance satisfies the shape and a
	// human skimming the manifest sees a plausible label; LoadLayer decodes
	// it but callers must not treat it as authoritative for the digest gate.
	prompt_version: string

	// tier every emitted claim carries. V1 layers are citation-grounded
	// restatements, hence doc-claimed; an LLM-inferred kind would need the
	// ai-derived tier and its revival evidence first (ADR-016 §4.2).
	// verified admits deterministic, Model-empty compile paths (ADR-018
	// oracle classes — index/cmd/lexical; ADR-016 M1 amendment). No layer
	// emits verified yet.
	tier: "doc-claimed" | "sme-stated" | "verified"

	// tier_profile names the tier vocabulary (ADR-017: data now).
	tier_profile: string | *"engineering"

	// budgets bound a compile run; the runner must refuse to exceed them.
	budgets: {
		max_tokens_per_run:  int & >0
		max_usd_per_run:     number & >=0
		max_rule_words:      int & >0 | *28
		max_claims_per_file: int & >0 | *40
	}

	// serving caps the decision-moment payload (zero-output-on-clean is
	// behavioral, not schematic).
	serving: {
		max_rules:  int & >0 & <=6 | *6
		max_tokens: int & >0 | *150
	}
}

// obligations is the v1 tracer-bullet layer (ADR-016 §4.8 #1). The corpus is
// deliberately only the BINDING documents — design essays produced noisy
// aspirational rules on the first fleet compile. This is the manifest's
// authoritative source for the corpus include list (M1: internal/semantic
// loads it via LoadLayer; the old hand-synced ObligationsInclude Go var is
// gone).
obligations: #Layer & {
	name:    "obligations"
	version: "v1"
	kind:    "obligation"
	corpus: include: ["CLAUDE.md", "ADR.md", "docs/plans/README.md"]
	// Advisory only — see #Layer.prompt_version above. The real identity is
	// content-addressed at runtime (internal/semantic/prompts.go).
	prompt_version: "obligations/content-addressed"
	tier:           "doc-claimed"
	budgets: {
		max_tokens_per_run: 200000
		max_usd_per_run:    1.0
	}
}

// findings is the D3 review-findings tracer bullet
// (docs/plans/findings-tracer.md FT-1, docs/vision/03-findings-and-experts.md
// §3.2): one markdown file per review/audit finding in a corpus directory,
// compiled through the same engine as obligations but under kind "finding"
// and the honest sme-stated tier — a human review conclusion is not a
// code-derived fact, and the shipped tier ranking already ranks it below
// verified/doc-claimed evidence.
findings: #Layer & {
	name:    "findings"
	version: "v1"
	kind:    "finding"
	corpus: include: ["findings/**"]
	// Advisory only — see #Layer.prompt_version above. The real identity is
	// content-addressed at runtime (internal/semantic/prompts.go).
	prompt_version: "findings/content-addressed"
	tier:           "sme-stated"
	budgets: {
		max_tokens_per_run: 200000
		max_usd_per_run:    1.0
	}
}
