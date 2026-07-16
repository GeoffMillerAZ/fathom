# init + adoption — one command, one canonical doc, zero stale guidance

The coding-agent story (docs/vision/01 §4, 04 §1) demands ≤10-minute
one-binary adoption and documentation that matches what shipped. Today:
`fathom init` does not exist (adoption is a 410-line manual recipe), the
June integration docs predate obligations/verify/hooks entirely, and
usage.md still documents the deleted set_profile.

## The command

- [x] fathom init wires a repo end-to-end, idempotently: .fathomignore if absent, daemon ingest (fail-soft exit 2 with startup instructions when unreachable), project .claude/settings.json hooks (SessionStart session-hook, PostToolUse edit-hook, Stop stop-hook — JSON-merged, never duplicated, binary path from os.Executable), CLAUDE.md + AGENTS.md fathom stanzas between marker comments, printed manual steps (MCP registration one-liner, lefthook snippet) {id: IA-1, symbol: cmdInit, package: cmd/fathom}
- [x] Settings merge is additive and idempotent: existing hooks preserved, re-running init changes nothing the second time {id: IA-2, symbol: mergeHookSettings, package: cmd/fathom}
- [x] The stanza is one source: emitted into both CLAUDE.md and AGENTS.md (Codex reads the latter), replaceable in place via begin/end markers {id: IA-3, symbol: fathomStanza, package: cmd/fathom}

## The canonical doc

- [x] docs/use-cases/coding-agents.md — THE adoption guide: the loop (index → serve → inject → gate → measure), exact hook wiring, CLAUDE.md/AGENTS.md guidance, agent-surface registration, lefthook/CI gates, instrument feedback loop, and the honest cost story {id: IA-4, symbol: cmdInit, skip_test: true, skip: "docs deliverable anchored to the command it documents"}
- [x] Stale guidance retired: usage.md set_profile sections removed, claude-code-integration.md and getting-started.md point at the canonical doc for wiring, use-cases README indexes it {id: IA-5, symbol: cmdInit, skip_test: true, skip: "docs deliverable anchored to the command that supersedes the manual recipes"}

## Deferred

- [-] Homebrew tap / release binaries {id: IA-6, disposition: deferred, reason: "H2 publish scope; repos are private until D4", trigger: "H1 exit gates green + D4 ratified"}
- [-] Codex lifecycle-hook parity beyond AGENTS.md {id: IA-7, disposition: deferred, reason: "L1 gates + MCP are already agent-agnostic; Codex hook story is shifting", trigger: "a second harness becomes a real consumer with a stable hook surface"}
