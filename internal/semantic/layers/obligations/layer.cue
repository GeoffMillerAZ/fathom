// obligations is the v1 tracer-bullet layer (ADR-016 §4.8 #1). The corpus is
// deliberately only the BINDING documents — design essays produced noisy
// aspirational rules on the first fleet compile. This is the manifest's
// authoritative source for the corpus include list (M1: internal/semantic
// loads it via LoadLayer; the old hand-synced ObligationsInclude Go var is
// gone).
//
// LC-4 pack shape (docs/plans/layer-catalog.md, docs/vision/02-shape.md §3):
// this file is plain data — unified against schema/semantic/schema.cue's
// #Layer by internal/semantic's loader, never authored against it directly.
// A pack author (or `fathom layers install`) edits values here, never CUE
// definitions.
name:    "obligations"
version: "v1"
kind:    "obligation"
corpus: include: ["CLAUDE.md", "ADR.md", "docs/plans/README.md", "**/CLAUDE.md"]
// **/CLAUDE.md (BP-32, O23's root cause): a repo's NESTED instruction files
// are binding rules a human wrote about one subtree — internal/CLAUDE.md's
// "adding an extractor: register it in fusion.New and add a golden test" is
// exactly the obligation the edit hook should serve for
// internal/extractor/*.go. With a root-only corpus that rule was never
// compiled, and the hook served six broad root rules with the precise one
// absent. The compile-time directory scope floor (internal/semantic/
// compile.go, scopeFloor) keeps the new claims honest: a claim compiled from
// <dir>/CLAUDE.md may only scope inside <dir>/.
// AGENTS.md is deliberately NOT included: fathom writes the same stanza into
// both files (cmd/fathom stanza injector), so `**/AGENTS.md` compiles a
// byte-duplicate of text CLAUDE.md already contributes — double spend, two
// claim sets, one rule. A repo whose AGENTS.md carries rules its CLAUDE.md
// does not is a project-local pack override, not a default.
corpus: exclude: ["third_party/**", ".claude/worktrees/**", "workspace/**", "node_modules/**"]
// docs/intent/** is served DETERMINISTICALLY by `fathom intent audit`
// (ADR-026), never compiled here: an LLM restating rules that are already
// rules pays compile cost for no precision, and ADR.md is already
// dual-served (its Revisit-when trailers reach the edit hook through this
// layer AND the intent corpus). Pinned by
// TestObligationsCorpusNeverWidensToIntent so the corpus cannot widen by
// accident.
// Advisory only — see #Layer.prompt_version's doc comment in schema.cue. The
// real identity is content-addressed at runtime (internal/semantic/prompts.go).
prompt_version: "obligations/content-addressed"
prompt_file:    "schema/semantic/layers/obligations/prompt.md"
tier:           "doc-claimed"
// governs (the #Layer default, stated here because this is THE governing
// layer): CLAUDE.md/ADR.md rules are binding rules a human wrote about
// the code, and they fill the serving budget before any claims layer.
role: "governs"
// concern, not platform (ADR-024, docs/design/pack-taxonomy.md "The
// obligations call"): the corpus (CLAUDE.md/ADR.md) is as org-specific as
// content gets, but this pack's prompt/corpus-selector/eval fixtures are
// authored and gated by fathom once, shipped identically to every adopting
// repo — authorship, not compiled-corpus content, is what stratum names.
stratum: "concern"
// eval/RESULTS.md: original ADR-016 baseline gate pass (the admission
// record); a later production-model re-check found recall unstable around
// the 0.75 bar (0.71/0.86) and is escalated as a pre-OP-6 input, not a
// gate failure of this record — see eval/RESULTS.md's PRE-OP-6 SIGNAL entry.
evaled: true
budgets: {
	max_tokens_per_run: 200000
	max_usd_per_run:    1.0
}
