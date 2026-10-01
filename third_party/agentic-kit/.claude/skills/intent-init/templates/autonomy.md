# Autonomy

<!-- What an agent may do unasked, must ask first, and never does, in this repo. This is
     policy about action, so a three-line summary also lives in the always-loaded
     conventions (AGENTS.md); this file is the full source. Under 60 lines.
     Rationale: agentic-kit/docs/project-intent/operating.md §1. -->

## Branch model

<!-- Which branch agents integrate on, who promotes to the protected branch, and how. -->

- Integration branch: `<poc | develop | feature/*>`. Agents commit and push here freely.
- Protected branch: `main`. Promoted from the integration branch by a human.

## May, without asking

- Edit files, run tests and gates, commit to the integration branch, push it.
- Update `docs/intent/` per its rules; prune within budget; refresh the code index.
- Run the project's own binary only against temporary roots and fixtures.

## Ask first

- Merge to the protected branch; tag; release; publish anything.
- Delete or rewrite history on any branch; delete data or files under `<protected dirs>`.
- Run anything against real user state (`$HOME`, real package roots, real services).
- Add a dependency; change an item on the compatibility surface (breaking).
- Modify global agent configuration (`~/.claude`, `~/.codex`, `~/.agents`).

## Never

- Force-push the protected branch. Commit secrets. Read credential stores.
- Run `sudo` or escalate privileges.
- Disable or skip a gate, hook, or test to get green.
- Claim done without a `Verified:` line, or check a plan box the index cannot prove.

## When unsure

Ask. State the action, the blast radius, and the reversal path in one line each.

Revisit when: the repo gains a second maintainer, a CI-driven release, or a user.
