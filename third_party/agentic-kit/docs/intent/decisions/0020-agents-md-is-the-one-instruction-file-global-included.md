# 0020: AGENTS.md is the one instruction file, root and nested; the global file is one shared ~/.codex/AGENTS.md

**Status:** accepted
**Type:** process
**Date:** 2026-10-17

## Context

The kit already tells adopters to keep the project stanza in `AGENTS.md` only (changelog,
`integration.md` section 2). Three things were still specialised: the global stanza was
kept as two identical copies (`~/.claude/CLAUDE.md` and `~/.codex/AGENTS.md`), the shim
`CLAUDE.md` was described as a case to serve rather than something to delete, and
`adopting-hosts.md` led with appending an import to an adopter's `CLAUDE.md`. The owner
decided on 2026-10-17: why specialise if we can generalise with no penalty. Claude Code
2.1.281 and later reads `AGENTS.md` natively only while no `CLAUDE.md`, `.claude/CLAUDE.md`
or `CLAUDE.local.md` exists in the working directory or above (`~/.claude/CLAUDE.md` does
not count); Codex reads only `AGENTS.md`; Claude Code has no user-level `AGENTS.md`.

## Decision

1. `AGENTS.md`, root and nested, is the only project instruction file. The kit never writes
   or prescribes a `CLAUDE.md` or `CLAUDE.local.md`.
2. An adopter's existing `CLAUDE.md` content moves into `AGENTS.md`; `CLAUDE.md` is then
   exactly `@AGENTS.md` or deleted. The installer's marked import block remains the
   fallback for a file the adopter keeps.
3. Global instructions are one shared `~/.codex/AGENTS.md`; `~/.claude/CLAUDE.md` is the
   single line `@~/.codex/AGENTS.md`.
4. Skills live in `.claude/skills` with `.agents/skills` linked to it (unchanged).
5. Host checks accept an `AGENTS.md`-only adopter as passing (already true; pinned by
   `TestHostClaudeInstallWritesAgentsMdAndNeverCreatesClaudeMd`).

## Alternatives rejected

- **Keep two global copies.** They drift; the import line costs nothing. Wins if: Claude
  Code stops resolving `~` imports.
- **Make doctor fail a shim `CLAUDE.md`.** A shim is harmless once it is the one import line.

## Consequences

Doctor still checks only the root `CLAUDE.md`; a `.claude/CLAUDE.md` or `CLAUDE.local.md`
is the adopter's to import or remove. The fathom-* skills are fathom-owned and generated;
their CLAUDE.md wording is fathom's to refresh.

## Preserves / forecloses

Preserves the one-copy stanza rule. Forecloses a Claude-specific instruction file in
adopter repos.

**Decision models.** None applies: a static documentation and file-layout rule with no
runtime question to ask a model.

**Revisit when:** Claude Code reads a user-level `AGENTS.md`, or stops reading `AGENTS.md`
natively, or a supported session type reads only CLAUDE files.
