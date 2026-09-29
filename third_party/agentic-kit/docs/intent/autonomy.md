# Autonomy

What an agent may do unasked, must ask first, and never does in this repo. A three-line
summary lives in `CLAUDE.md` / `AGENTS.md`; this is the full source. Rationale:
`docs/project-intent/operating.md` §1. Policy set 2026-08-25. The known consumers
are listed in [scope.md](scope.md) (`Taskfile.yml` `downstream-check`: fathom,
throughline, growing-gardens-bi, racli); `main` is a published surface, not a scratch
branch. Report which consumers the configured downstream checks actually exercised;
a skipped or unconfigured consumer is not a compatibility pass.

## Branch model

- Integration: a short-lived branch off `main` per unit of work (`<topic>` or
  `worktree-<agent>`), pushed freely. Merged fast-forward into `main` only with the
  gate green (`task validate`) and, for anything under `foundations/`, `kits/`,
  `runtimes/`, `surfaces/`, or `toolchains/`, `task downstream-check` green too.
- Protected: `main`. History is never rewritten there. Tags (`vX.Y.Z`) are cut by Geoff.
- Parallel agents: one worktree per agent under `.claude/worktrees/`, removed on
  completion; a worktree older than its session is an incident (`operating.md` §4).

## May, without asking

- Edit, run `task validate` / `task test` / `task constitution-lint`, commit to the
  topic branch, push it.
- Merge the topic branch fast-forward into `main` and push when the gates above are
  green and the change is additive or a documented fix (no `compat.md` tier demotion,
  no exported-API removal).
- Maintain `docs/intent/`, `CHANGELOG.md` `[Unreleased]`, `TRACKING.md`, and
  `docs/catalog.md` (via `task docs-index`) in the same change as the code.
- Run the eval and fuzz seeds (`task eval`, `task fuzz`): both offline. Not
  `eval-record` (overwrites committed fixtures; ask first) and not `smoke-live` (needs
  a live key).

## Ask first

- Cut or move a tag; edit a released `CHANGELOG.md` block; change `compat.md`
  stability tiers downward; remove or rename an exported symbol or a package.
- Add a dependency to `go.mod` (the module is stdlib-only by constitution; any
  `require` line is a decision, not a convenience).
- Change `cmd/constitution-lint` checks, `org-denylist.txt`, `secret-patterns.txt`,
  or `lefthook.yml`: these are the gate itself.
- Touch `_reference/` directories, `workspace/`, or `tmp/`.
- Modify `~/.claude`, `~/.codex`, `~/.agents`, or a consumer repo's tree.

## Never

- Force-push `main`. Commit secrets, cassettes with live keys, or org-specific tokens.
- `LEFTHOOK=0`, `--no-verify`, or editing a check to make it pass.
- Claim a `TRACKING.md` `[x]` or a plan `[x]` the gate has not proven; commit without
  a `Verified:` line.
- Call a live LLM or the network from a test; every test is hermetic (cassettes, fakes).
- Import from a `_reference/` directory (port from it only).

## When unsure

Ask, stating the action, its blast radius (which consumers), and the reversal path.
For a subagent, "ask" means return to the orchestrator.

Revisit when: a second maintainer lands commits; a consumer pins a kit version by
tag rather than tracking `main`; or CI starts cutting releases.
