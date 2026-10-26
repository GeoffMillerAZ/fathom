# Memory and assistive tools addendum, 2026-10-22

Follow-up to [the 2026-10-19 tools assessment](../2026-10-19/tools-assessment.md), same
scoring and verdicts. It covers only what that pass did not: tools named since, and the
native memory features that were live but unassessed. GitHub statistics were read on
2026-10-22. Live checks ran on the Mac mini (Claude Code 2.1.30x, Codex CLI 0.163.0).

## Answer

No memory tool would change outcomes here. Memory already runs in three layers:

- AGENTS.md holds the rules.
- Native memory: Claude Code auto memory (151 fact files across projects) and Codex
  memories (on; MEMORY.md 69 KB, memory_summary.md 9 KB loaded per session).
- Fathom thread notes are the curated continuity.

The well-received memory products are either model-summarized transcript memory, which
duplicates native memory and is a fathom non-goal, or task trackers that overlap fathom
plans. The real risk is stale memory, not missing memory: nothing reviews what the
native layers write. Response: a weekly `memory-prune` pass for active projects
(workstation repo, 2026-10-22).

Ranked, for this setup:

1. **Native memory plus pruning.** Zero install, both hosts, already loaded every session.
   Value depends on accuracy, so pruning is the investment.
2. **cass, Trial only on felt need.** The one tool that fills a real gap: raw "what did we
   decide in session X" recall across both hosts. Thin proof (1.2k stars, one maintainer).
3. **Nothing else.** Each alternative below copies a layer already run.

## Memory and persistent context

### Codex native memories

Codex turns idle chats into `~/.codex/memories/` (raw memories, rollout summaries, then a
consolidated MEMORY.md and memory_summary.md, in a git repository). (Adopted, already on)

- Scores: Impact 2/5, Risk 2/5, Proven 2/5, Reception 3/5
- Fits: Codex parity with Claude Code auto memory; no install.
- Fathom: Model-summarized transcript memory, a fathom non-goal, but isolated in `~/.codex`.
- Risk: Unreviewed model output loaded each session (about 2.3k tokens). Vendor docs say
  required guidance belongs in AGENTS.md, memories are a recall aid.
- Evidence: Vendor docs only. Off by default; this machine has it enabled.
- Absorb?: Keep outside. Prune it (Layout B of `memory-prune`).

### Claude Code auto memory

Per-project fact files under `~/.claude/projects/<project>/memory/` with a MEMORY.md
index, first 200 lines or 25 KB loaded per session. (Adopted, already on)

- Fits: The native memory the guidance already relies on.
- Fathom: No new overlap.
- Absorb?: Keep outside. Prune it (Layout A of `memory-prune`).

### beads (bd)

Git-synced issue tracker for agents with a dependency graph and ready queue, on Dolt.
(Pass; steveyegge/beads)

- Scores: Impact 2/5, Risk 3/5, Proven 2/5, Reception 4/5
- Fathom: Overlaps thread notes and plan rows; a second state store per repository.
- Risk: 1.5k open issues, an experimental HTTP surface, self-reported data-loss bugs in
  early versions; no independent reliability evaluation.
- Evidence: 28.1k stars, v1.3.1 on 2026-09-30.
- What would change it: a need for multi-agent work queues, which fathom does not have.

### GitHub spec-kit and claude-task-master

Spec-driven scaffolding and an MCP task list. (Pass)

- Fathom: `docs/plans` plus `fathom verify` already give spec-driven work with
  machine-proven completion; spec-kit checklists are unverified prose.
- Evidence: spec-kit 142k stars, v1.2.3 on 2026-10-22; critiques call it spec-first only
  and document-heavy; outcomes not independently validated. task-master 28k stars, no
  release since 2026-03-31.

## Assistive tools

### Claude Code LSP plugins (gopls-lsp, typescript-lsp)

First-party language-server wiring for the native LSP tool: definition, references,
hover, call hierarchy. (Adopted)

- Scores: Impact 3/5, Risk 1/5, Proven 3/5, Reception 4/5
- Fathom: Complements it. LSP is live and type-aware with no index lag; fathom keeps blast
  radius, obligations and verification.
- Risk: Open issues where a plugin installs without its server config. A file outside the
  session's workspace root gets false "undefined" diagnostics from gopls.
- Evidence: gopls-lsp answered hover on agentic-kit; typescript-lsp returned
  documentSymbol on an app-kit `.ts` file in a fresh session.
- Install: typescript-language-server 6.0.1 and typescript 6.0.3 through the workstation
  repo's pinned npm installer, not Nix, to avoid store churn; the Nix move is a tracked
  todo. Codex support unverified.

### CodeRabbit CLI

Local LLM diff review whose findings return to the agent. (Pass)

- Fathom: Overlaps `review_diff` with model review instead of rule-based review.
- Why not: Beta, vendor-only evidence, credit-priced, and it sends code off the machine.
- What would change it: an independent evaluation and a local or self-hosted mode.

### mise and repomix

Tool-version manager and repository packer. (Pass)

- Why not: Nix and Home Manager own toolchain pins; fathom's index and `read_source`
  replace whole-repository packing.

## Fathom fit

| Capability | Verdict | Why |
|---|---|---|
| Memory storage or summarized recall | Keep outside | "No transcript mining"; native memory and thread notes cover it |
| Memory verification | Absorb candidate | A fact that cites a path, symbol, flag or version is a claim; checking claims against the index is fathom's job. `memory-prune` does this today with fathom tools; a deterministic `fathom` check of memory files would make it cheap and exact |
| Work queues (beads) | Keep outside | No present need |
| Live LSP queries | Keep outside | Already a non-goal; the native LSP tool covers it |
