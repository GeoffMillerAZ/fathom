# Autonomy

What an agent may do unasked, must ask first, and never does, in this repo. A three-line
summary lives in `AGENTS.md`; this file is the full source.
Rationale: `agentic-kit/docs/project-intent/operating.md` §1.

## Branch model

- Integration branch: `main`, from v0.5.0 (b8f47c3, 2026-08-26). Agents commit and push
  here freely after the gate; releases are tags on `main`.
- `feat/mcp-scoped-surfaces` carried the trunk from 2026-06-17 to v0.5.0 (338 commits,
  tags v0.3.0–v0.5.0). It is retired: it mirrors `main` until the next release, then is
  deleted once no worktree references it (ask first, as any deletion).
- Sibling projects commit to this tree (the dogfood consumers). Stage only your own files
  by explicit path; never `git add -A`; `git diff --stat` before committing.

## May, without asking

- Edit, test, run `task validate`, commit, push the integration branch.
- After a wave with the full gate and `fathom verify` green: roll `CHANGELOG.md`, tag
  `vX.Y.0` on `main`, push `main` and the tag (owner's standing authorization of
  2026-06-16, practiced since: v0.1.0–v0.5.0).
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
- Spend LLM budget: `fathom compile --dry-run=false`, `fathom smoke`, a live `fathom
  assess` (ADR-046; its `--dry-run` spends nothing and needs no ask), any compile of
  another service's layers (a dry run under-reported one compile 25× and $0.64).
- Run a judgment-program bench that spends provider or agent budget (ADR-056): J0 and the I3 smoke test
  (about $0.25 of provider spend), the level 10 cell ($90 to $360 of agent spend), any later `ask` or
  traversal bench, and the served-not-kept bench PR-M2 (ADR-057; about $76 of agent spend, 40 cells).
  Enabling a new decision-port site in a repository's `[decide] sites` is the operator's call.
- Add a dependency; bump the `agentic-kit` pin (then `task sync-kit`); change anything on
  the compatibility surface (`scope.md`).
- Touch global agent configuration (`~/.claude`, `~/.codex`, `~/.agents`, `~/.fathom`)
  outside what `fathom init` already owns by marker.
- Restart or re-bootstrap the launchd daemon by hand; `task deploy` is the sanctioned path.

## Held (owner answers)

- **Bench and model spend: declined for now (owner, 2026-09-29).** No bench and no
  reading that spends provider or agent budget runs until the owner lifts this hold, and
  none is asked for meanwhile; an approval given before this date (PR-M2's $76) is
  suspended. The rows waiting on a spend decision: XM-M2 (`docs/plans/exploration-maps.md`),
  PR-M2 (`served-not-kept.md`, ADR-057), TR-M1 (`test-run-hook.md`), CA-M2
  (`carry-across.md`), WS-M2 (`work-signals.md`) and OP-1 (`ambient-fathom.md`), with J0
  and the level 10 cell above; any other row that spends (the `operator spend` rows among
  them) is held by the same rule. $0 replays, fixtures and scratch stores are not spend and
  go on. Lift: the owner says so; record the date and delete this entry. **Lifted in part 2026-10-16 (owner):** up to
  $10 in all on OpenRouter for Jev, through the decision port only (ADR-073): stage 0 of
  `docs/plans/agentic-decisions.md` at most $2, stage 1 at most $8 and only when EXP-M4 opens it; the key read through
  `api_key_file` only and never printed; every reading records its spend. OpenAI Decisions spend, agent-budget
  benches (stage 2 among them) and every row named above stay held.

## Never

- Force-push `main`. Commit secrets (the daemon's launchd plist holds an API key; never
  copy it into the tree). Read credential stores. `sudo`.
- Deploy the daemon with a raw `cp` (`.fathom/concerns.cue` deploy_safety; O25).
- Run a shell `fathom ingest` against the shared fleet store while the daemon serves it
  (`AGENTS.md` question-shape routing; O16's failure path).
- Skip a gate to get green: `LEFTHOOK=0` only after the gate ran green and the failure is
  the transport wedge, and the thread note says so; `--no-verify` never.
- Check a `docs/plans/*.md` box the index cannot prove; claim done without `Verified:`.
- Hand-write observatory markup; edit generated CSS (`cmd/fathom/AGENTS.md`).

## When unsure

Ask. State the action, the blast radius, and the reversal path in one line each. A
subagent's "ask first" is a return to its orchestrator, never a prompt to the human.

Revisit when: the repo gains a second maintainer, a CI runner, or an external user.
