package modspec

// A well-formed module manifest (docs/spec/expert-module-v0.md §3's
// worked example): an engineering-standards module compiled from a git
// corpus, eval-gated, recompilable under a named prompt identity.
standardsModule: #Module & {
	name:        "engineering-standards-module"
	version:     "v1"
	description: "Engineering standards and governing docs, compiled as claims."
	corpus: include: ["CLAUDE.md", "ADR.md", "docs/design/*.md"]
	watermark:    "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	tier_profile: "engineering"
	eval_evidence: {
		evaled: true
		path:   "schema/module/eval/RESULTS.md"
	}
	recompile: {
		allowed:         true
		prompt_identity: "standards-module/v1"
	}
	license: "Apache-2.0"
	provenance: {
		compiled_by: "claude-haiku-4-5-20251001"
		compiled_at: "2026-07-28T00:00:00Z"
	}
}

// A closed, non-recompilable module over a non-git corpus (the manifest:
// watermark path) with no eval evidence yet — evaled defaults false, no
// path required.
findingsSnapshot: #Module & {
	name:        "findings-snapshot"
	version:     "v1"
	description: "A frozen findings export; not intended to be recompiled by the recipient."
	corpus: include: ["findings/**"]
	watermark: "manifest:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	eval_evidence: evaled: false
	recompile: allowed: false
	provenance: {
		compiled_by: "deterministic"
		compiled_at: "2026-07-28T12:30:00Z"
	}
}
