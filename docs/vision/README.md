# Fathom excellence review — recommendations (2026-07-14)

**Status: FOR REVIEW.** Nothing in this directory is binding. These are
recommendations produced by the 2026-07 excellence review (a 13-agent
survey + strategist panel over the repo, the call log, the design docs, and
the July-2026 competitive landscape, followed by synthesis and an
adversarial critique pass). Items you accept graduate into ADRs,
`docs/design/`, and `docs/plans/` as durable specs; items you reject die
here. This directory is deliberately **excluded from the obligations
corpus** — it is exactly the kind of aspirational prose the first fleet
compile taught us never to compile into rules.

## The thesis in one paragraph

Fathom's genuine, verified differentiation is not code navigation (being
commoditized by free LSP/tree-sitter MCP servers), not analysis findings
(owned by Semgrep/CodeQL), and not AI review (saturated). It is the thing
no surveyed tool does: **governing knowledge about code the way code
governs itself** — claims that are content-addressed, pinned to citation
hashes, automatically swept stale/contradicted when their sources drift,
carried in portable single-file stores with provenance, and **delivered by
configuration, not agent choice** (hooks that inject, gates that block).
Everything you want to build — expert modules, findings backends, connected
experts, review products — is a *consumer of that substrate*, and the
substrate is largely shipped. The risk is not the architecture; it is
sequence. Finish proving the wedge (obligations injection + verify gate,
measured) before laying stones no one is standing on.

## Direct answers to your questions

| Your question | Verdict | Doc |
|---|---|---|
| Knowledge expert built on top? | Yes — as expert-module packs (data) over the existing claims stratum, not a new system | [03](03-findings-and-experts.md) |
| Backend for reviews / investigations / compliance — separate tools or part of fathom? | Part of fathom, as **module packs** (CUE manifest + prompt + corpus + eval + kill gate). No new repos. | [02](02-shape.md) |
| CUE cataloging companion system? | Don't build a companion. Make the CUE manifest you already have **load-bearing** inside fathom; a pack format follows; a registry only if H3 conditions fire. | [02 §4](02-shape.md) |
| Hosted expert backend linking modules, findings layered on? | Findings-as-claims **now, locally** (the slot is already reserved in `internal/core/claims.go`); federation = DuckDB ATTACH over portable stores, **contract now, host never-until-pulled**. | [03](03-findings-and-experts.md) |
| How to handle findings vs code versions? | Mostly solved by shipped ADR-016/017 machinery (watermarks + citation hashes + the $0 sweep); one new mechanic needed — the remediation-drift flag on scoped code. | [03 §2](03-findings-and-experts.md) |
| Projects extending semantic layers — plugin system? | Yes, but data-defined (CUE manifests), not code plugins. Rule of three: build layer 2 by hand first, then generalize. | [02 §3](02-shape.md) |
| General tool for many domains without bloat — plugins? what? | One substrate, N data-defined packs; an admission rule + a KILL.md per pack; no plugin API needed. | [06 §4](06-use-cases.md), [02 §5](02-shape.md) |
| Pluggable chat interfaces per use case? | MCP **is** the pluggable chat interface. Command center = failure-mode module + prompt over MCP (entry-conditioned at 15–20 findings); an optional read-only store viewer later. Learning platform: trap. | [03 §4](03-findings-and-experts.md), [06](06-use-cases.md) |
| Am I reaching too far? | Architecturally no — your shipped design anticipates all of it. In **sequence**, yes. Adopt the rung ladder. | [07](07-questions-and-risks.md) |
| What am I not thinking about? | Use-case whitespace + the questions you're not asking | [06](06-use-cases.md), [07](07-questions-and-risks.md) |
| How do you frame this better? | *Fathom is the truth-maintenance system for agent instructions* — and the enforcement layer for spec-driven development. | [01 §4](01-thesis.md) |
| Making coding agents incredible? | Freeze new capability; finish + instrument the loop (8 ranked gaps, ordered). SDD enforcement is the killer feature. | [04](04-coding-agents.md) |
| What does world-class mean, measurably? | The metric table — injection coverage, serve-time precision, share of discovery, adoption friction, context-bench — is the operational definition for the wedge. | [04 §1](04-coding-agents.md) |
| Is up-front AI preprocessing world-class? | Half right — and the half matters. Distilling *code* loses; distilling *binding documents into governed claims* is differentiated **in design, unproven in measured value** until the context-bench and injection gates run. | [01](01-thesis.md), [07 R8](07-questions-and-risks.md) |
| Clean up the tools/MCP surface? | Yes, now — the evidence arrived. Two surfaces, a 15-tool agent surface, kill aliases, execute-and-flag gate. | [05](05-surfaces.md) |

## The documents

1. **[01-thesis.md](01-thesis.md)** — what fathom is at its best; the honest preprocessing verdict; differentiation.
2. **[02-shape.md](02-shape.md)** — kernel / module packs / satellites; extensibility mechanism; the CUE decision; bloat discipline.
3. **[03-findings-and-experts.md](03-findings-and-experts.md)** — the findings model; connected experts; hosted; chat.
4. **[04-coding-agents.md](04-coding-agents.md)** — the path to incredible for Claude Code (and Codex); ranked gaps; SDD enforcement.
5. **[05-surfaces.md](05-surfaces.md)** — MCP surface cleanup: keep/kill/merge, conventions, gate semantics.
6. **[06-use-cases.md](06-use-cases.md)** — the portfolio with FOCUS/OPTION/TRAP verdicts; layer-2 recommendation; landscape.
7. **[07-questions-and-risks.md](07-questions-and-risks.md)** — red team: risks rated, the questions you're not asking, three decisions.
8. **[08-roadmap.md](08-roadmap.md)** — the consolidated sequence and your decision checklist. **Start your review here or at 01.**

## How this review was grounded

Every **repo-facing** number is measured, not argued: the call log
(`workspace/logs/mcp-calls.jsonl`, 1,186 calls over 8 days at the review
snapshot), the adoption baseline (0 fathom calls vs 193 greps, recorded in
`CHANGELOG.md`), the live tool registry (87 tools via `capabilities`), the
dogfood observations, and the shipped code as of `3bb6c29` plus the
uncommitted obligations-v2 hardening. **Landscape figures** (competitor
stats, market claims) come from a July-2026 web survey and are sourced
inline in [06 §3](06-use-cases.md) — treat them as survey claims, not
measurements. Where a claim is speculation, the docs label it. One known
caveat the critique pass surfaced: the call log has no origin field, so
gate/hook plumbing and genuine agent tool-picking are not fully separable
([04 Gap 1](04-coding-agents.md), [05](05-surfaces.md)).
