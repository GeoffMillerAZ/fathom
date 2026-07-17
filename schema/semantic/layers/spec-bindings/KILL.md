# spec-bindings — kill gate

**Status: shipped 2026-07-17, entering the 3-week generalize-or-kill
window.** spec-bindings is the SDD pack (docs/plans/layer-catalog.md LC-6):
binding requirement lines from gated plans (`docs/plans/**`, excluding
`README.md`), scoped to the package each task governs, served through the
same channel the edit-hook already renders kind-aware (FT-2). It cleared its
shipping eval on the first attempt (`eval/RESULTS.md`: precision 1.00 /
recall 1.00 over 2 consecutive live runs against the `claude-cli`
subscription lane, including two deliberate skip-traps — a `disposition:
deferred` task with no `package`, and an unannotated task line).

**Metric (post-shipping gate, docs/design/semantic-layers.md §4.6):** from
`mcp-calls.jsonl`/hook telemetry — the fraction of edits to a package with an
open gated-plan task preceded by a spec-binding-carrying obligations call,
AND an outcome signal (an agent's edit staying consistent with a served
binding vs. contradicting it, caught in review).

**Window:** 3 weeks from first fleet compile. **Decision date:** 2026-08-07.

**Kill criterion:** fails on call-rate but not quality → fold into
`outline`/`symbol_context` as a capped rider, same as obligations' own
fallback. Fails on quality → kill; the substrate cost was one pack
directory, no Go removed (compile.go/serve.go/sweep.go carry zero
spec-bindings-specific code to revert).

**Note on determinism:** the annotation grammar this layer restates
(`internal/verify/markdown.go`'s `ParseMarkdownPlan`) is already
deterministically parseable — spec-bindings intentionally still compiles it
through the LLM engine (matching LC-5/LC-6's "same engine as obligations,
different corpus and prompt" mandate) rather than a `producers.cue`
deterministic producer (LC-10, deferred). Revival evidence for promoting
this specific corpus to a deterministic `verified`-tier producer: first
consumer that needs spec-binding claims to survive the compile-cost budget
at fleet scale, where LLM restatement of already-structured data is the
weak link.
