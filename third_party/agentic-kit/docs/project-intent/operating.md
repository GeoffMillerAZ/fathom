# Operating records — autonomy, incidents, tool tripwires, parallel agents

Sibling to [README.md](README.md) (intent) and [changes.md](changes.md) (change
records). Intent says why. Change records say what changed. Operating records say **who
may do what, what happens when something breaks, and what the wiring was verified
against.** Each is small, each has one home, and each is read at the moment it applies
rather than at session start.

## 1. Autonomy policy (`docs/intent/autonomy.md`)

**The gap.** Agents act at machine speed on actions with human-speed consequences:
push, tag, release, delete, rewrite history, edit global configuration, run a tool
against real user state. Harness permission systems gate *commands*; they do not know
that in this repo `poc` is free and `main` is protected, or that running the product
against `$HOME` is the one thing that must never happen unasked. That knowledge is
project intent, so it lives in `docs/intent/`.

**The shape.** Three tiers and a branch model, under 60 lines:

- **Branch model.** Which branch agents integrate on freely, which is protected, who
  promotes. A repo nobody depends on yet can run `poc` as its integration branch and
  promote to `main` by hand.
- **May, without asking.** Edits, tests, gates, commits and pushes to the integration
  branch, `docs/intent/` maintenance, index refresh, the product against fixtures.
- **Ask first.** Protected-branch merges, tags, releases, history rewrites, deletions
  under named directories, anything against real user state, new dependencies,
  compatibility-surface changes, global agent configuration.
- **Never.** Force-push protected, commit secrets, read credential stores, `sudo`,
  disable a gate to get green, claim done without `Verified:`.
- **When unsure.** Ask, stating action, blast radius, reversal path.

**Loading.** A three-line summary lives in the always-loaded conventions because the
moment of action is not a design session; the full file is routed to by the router's
"any push, tag, delete, release, or global-config change" row. `intent-design` step 7
checks it before work that includes such an action.

**Why not the harness alone.** Claude Code permissions and Codex sandbox modes are the
enforcement; this file is the intent they enforce. When they disagree, this file is
wrong or the harness config is, and either way the repo now says which.

## 2. Incident flywheel (`docs/intent/incidents.md`, `intent-incident`)

**The gap.** Projects keep incident knowledge in the heads of whoever was there, or in
a one-off post-mortem nobody routes to. The same defect class recurs with a new face.

**The rule.** *Every incident yields an invariant, a test, or a decision.* An incident
record is ten lines: what, evidence, what would have caught it, class, outcome. It is
open until its outcome exists as a real artifact (a row in `invariants.md` with an
honest Enforced-by column, a named test, a decision record, a platform fact, an autonomy
change, or an evidence requirement on a sensitive surface). Then it is closed and
compressed to one line. The file stays small because the knowledge moved somewhere that
gets routed to.

**Classes** (they decide the outcome type): `gate-bypassed` → an evidence requirement
or a hook; `wrong-done-claim` → a proof gate; `intent-violated` → an invariant row or an
autonomy rule; `platform-fact` → `domain.md`; `contract-broken` → compatibility surface
plus a `Breaking` line; `other`.

**Predecessors.** If the project exists because a predecessor failed, that catalog
(`PAIN.md`-style) stays where it is and `incidents.md` points at it. Only the
consequence for this project is recorded.

## 3. Tool-version tripwires (router footer, `intent-doctor` check 18)

**The gap.** The agent tooling itself drifts: CLI flags are removed, undocumented config
keys stop being honored, skill locations move. Integration that silently degrades is
worse than integration that breaks.

**The rule.** Anything that depends on an external tool's behavior states what it was
verified against: `Verified against: Claude Code 2.1.245, Codex CLI 0.146.0
(2026-08-25)` as the router's last line, and the same form in any adapter, hook, or doc
that encodes tool behavior. `intent-doctor` compares installed versions and warns on
drift; the fix is to re-verify, not to bump the line. Where a real tripwire is possible
(a probe that fails loudly when the dependence breaks), build it and name it in the
same line; the codexcli adapter's `Ping` in this kit is the pattern.

## 4. Parallel agents

**The gap.** Several agents in one repo at once, each with a worktree, produce stray
worktrees, overlapping edits, and merges nobody reviewed. Evidence in this kit: two
abandoned `.claude/worktrees/agent-*` checkouts with no owner.

**The protocol**, five rules:

1. **One unit of work per agent**, named by a plan row or a task id. The brief names the
   paths the agent may touch and, by id, the records that govern the work (invariants,
   decisions, the plan row); two agents never own the same path at once. Briefs are how
   intent reaches a subagent that never opens the router.
2. **One worktree per agent**, on a branch named for the unit, from the integration
   branch. The agent commits there with the L1 format (`changes.md` §3) and never
   touches `main`.
3. **The orchestrator integrates**, in an order it states before spawning (contract
   producers before consumers). Integration is a fast-forward or a reviewed merge, never
   an agent's own `git push` to the integration branch.
4. **Shared files are contract files** (`CHANGELOG.md`, the decisions index, the
   router). Agents append rows; the orchestrator resolves order. Nobody rewrites a
   sibling's row.
5. **Clean up on completion.** `git worktree remove` and delete the branch once
   integrated; `git worktree prune` at the end of every orchestrated session. A
   worktree older than the session that made it is an incident (`gate-bypassed`).

Autonomy tiers apply per agent: a subagent's "ask first" is a return to its orchestrator,
never a prompt to the human.

## 5. Push gates (`lefthook` pre-push, `task push`)

Three rules, each paid for by a real incident in a consuming repo (fathom's
INC-20, INC-21, INC-22, 2026-09-19). The scripts that enforce the first and
third ship in `fathom-pack/scripts/` (`push-check.sh`, `push-http.sh`, each
with a test); the second is a property of the gate you write.

1. **A push refuses without the hook.** A wiped hooks directory (it happens:
   an unrelated process emptied `.git/hooks` and `.git/HEAD` at midnight) turns
   a gated push into an ungated one with no error. `task push` runs
   `push-check.sh` first: it resolves the hook through `git rev-parse --git-path`
   (honoring `core.hooksPath` and linked worktrees) and refuses, exit 3, unless
   the pre-push hook is lefthook's. A two-line push log is the tell of a gate
   that never fired.
2. **The gate verifies the tree it pushes.** A pre-push step that asks a live
   daemon, a shared index, or the operator's checkout is measuring a different
   tree — one that can lag the push by a whole round. A `fathom verify` step
   ingests HEAD into a scratch store and verifies against that; an ingest
   failure is a named fail-open line, never a silent pass. The general form:
   every gate input is derived from HEAD inside the hook, or the step says why
   it could not be.
3. **No transport session is held open across the gate.** git opens the
   transport before it runs the pre-push hook; over ssh that session sits idle
   for the whole gate and is gone when git sends the pack — a fully green gate
   followed by exit 141 (SIGPIPE) and an unchanged remote, twice, with
   keepalives on. `push-http.sh` derives `https://host/path` from the push
   remote (never a hardcoded URL — the kit stays org-agnostic) and pushes over
   smart HTTP, where the ref advertisement and the pack are separate requests;
   credentials come from the GitHub CLI when it is on `PATH`, otherwise from
   git's own configured helper. Never `--no-verify` to get around a hook.

Adopt by calling the vendored scripts from the repo's `push` task and running
their `_test.sh` files in the repo's own script gate; a repo that copies them
instead keeps the provenance header so a later kit bump can re-sync by it.

## 6. Considered and rejected

- **A permissions DSL.** The harness has one. This file states intent in prose the
  harness owner can translate.
- **A post-mortem template with sections for timeline, five whys, action items.** Ten
  lines and one required outcome beat a document nobody finishes.
- **Auto-bumping the Verified-against line on tool upgrade.** That is how a tripwire
  becomes a decoration.
- **A lock file for parallel agents.** Path ownership in the brief is the lock; a file
  would be one more shared file to fight over.
