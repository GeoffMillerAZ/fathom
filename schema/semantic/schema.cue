// Package semantic defines the CUE schema for semantic-layer definitions
// (ADR-016 §4.4). A layer definition names its corpus, prompt version, output
// kind, and budgets — the manifest SHAPE exists from day one so that when a
// second layer earns the generic runtime, definitions generalize instead of
// being reinvented. V1 ships exactly one definition: the obligations layer.
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

	// prompt_version is the compile prompt identity (bump on any change).
	prompt_version: string

	// tier every emitted claim carries. V1 layers are citation-grounded
	// restatements, hence doc-claimed; an LLM-inferred kind would need the
	// ai-derived tier and its revival evidence first (ADR-016 §4.2).
	tier: "doc-claimed" | "sme-stated"

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
// aspirational rules on the first fleet compile (see internal/semantic/corpus.go).
obligations: #Layer & {
	name:           "obligations"
	version:        "v1"
	kind:           "obligation"
	corpus: include: ["CLAUDE.md", "ADR.md", "docs/plans/README.md"]
	prompt_version: "obligations/v2"
	tier:           "doc-claimed"
	budgets: {
		max_tokens_per_run: 200000
		max_usd_per_run:    1.0
	}
}
