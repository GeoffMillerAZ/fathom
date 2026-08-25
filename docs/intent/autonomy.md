# Autonomy

What an agent may do unasked, must ask first, and never does, in this repo. A three-line
summary lives in `CLAUDE.md` / `AGENTS.md`; this file is the full source.
Rationale: `agentic-kit/docs/project-intent/operating.md` §1.

## Branch model

- Integration branch: `feat/mcp-scoped-surfaces`, the de facto trunk since 2026-06-17
  (335 commits, tags v0.3.0 and v0.4.0 live on it). Agents commit and push here freely.
- `main`: promoted by fast-forward push of a green wave (`origin/main` reflog shows only
  "update by push"); the local `main` ref is stale. Whether to rename or realign is `questions.md` Q1.
- Sibling projects commit to this tree (the dogfood consumers). Stage only your own files
  by explicit path; never `git add -A`; `git diff --stat` before committing.

## May, without asking

- Edit, test, run `task validate`, commit, push the integration branch.
- After a wave with the full gate and `fathom verify` green: roll `CHANGELOG.md`, tag
  `vX.Y.0`, fast-forward `main` to HEAD, push main and the tag (owner's standing
  authorization of 2026-06-16, practiced since: v0.1.0–v0.4.0).
- Refresh the index through the daemon's `ingest` MCP tool; `task deploy` the daemon
  after a source-changing commit and read its serving assertion.
- Update `docs/intent/` per its rules; file `fathom observe` entries and `fathom thread note`s.
- Run the product against fixtures (`testdata/fauxtown/`), temp stores, and this repo.
- Run `fathom fleet report` and `fathom config audit` (read-only).

## Ask first

- Delete files, branches, or worktrees (`.claude/worktrees/*`, `worktree-*` branches),
  or rewrite history on any branch.
- `fathom purge` (irreversible), `fathom fleet reconcile --apply` across other repos,
  `fathom scope suppress` outside this repo.
- Spend LLM budget: `fathom compile --dry-run=false`, `fathom smoke`, any compile of
  another service's layers (a dry run under-reported one compile 25× and $0.64).
- Add a dependency; bump the `agentic-kit` pin (then `task sync-kit`); change anything on
  the compatibility surface (`scope.md`).
- Touch global agent configuration (`~/.claude`, `~/.codex`, `~/.agents`, `~/.fathom`)
  outside what `fathom init` already owns by marker.
- Restart or re-bootstrap the launchd daemon by hand; `task deploy` is the sanctioned path.

## Never

- Force-push `main`. Commit secrets (the daemon's launchd plist holds an API key; never
  copy it into the tree). Read credential stores. `sudo`.
- Deploy the daemon with a raw `cp` (`.fathom/concerns.cue` deploy_safety; O25).
- Run a shell `fathom ingest` against the shared fleet store while the daemon serves it
  (`CLAUDE.md` Index-first; O16's failure path).
- Skip a gate to get green: `LEFTHOOK=0` only after the gate ran green and the failure is
  the transport wedge, and the thread note says so; `--no-verify` never.
- Check a `docs/plans/*.md` box the index cannot prove; claim done without `Verified:`.
- Hand-write observatory markup; edit generated CSS (`cmd/fathom/CLAUDE.md`).

## When unsure

Ask. State the action, the blast radius, and the reversal path in one line each. A
subagent's "ask first" is a return to its orchestrator, never a prompt to the human.

Revisit when: the repo gains a second maintainer, a CI runner, or an external user.
