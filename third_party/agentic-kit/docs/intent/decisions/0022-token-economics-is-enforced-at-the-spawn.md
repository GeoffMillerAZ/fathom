# 0022: Token economics is enforced at the spawn, structurally, and measured locally

**Status:** accepted
**Type:** architecture
**Date:** 2026-10-19
**Request:** "We should rethink the way that we do tokenomic enforcement ... Maybe we should always require you to specify the model and give guidance on which model families should be used for what ... make them basically the same between claude and codex" (Geoff, 2026-10-19)

## Context

Measured on 5,052 Claude Code transcripts (2026-09-02 to 10-19) and 792 Codex rollouts
([evidence](../../evidence/tokenomics/2026-10-19/README.md)): subagents carried 91% of
Claude Code tokens and Opus-class subagents 47% of output; 118 forks and 130 unset-model
spawns inherited the session model; one spawn in 463 used the cheapest tier. The
previous hook (deny unset or Opus worker spawns, nudge "/caveman" on every worker spawn)
saw only 23.5% of denied spawns retried with a model; the caveman nudge fired 110 times
with 6% compliance, and about 240 notes landed on allowed spawns. Codex already sets
model and effort on 90% of spawns; its parents carry 70% of its input. Prices moved:
the strong tier now costs about twice the standard tier and the fast tier a twentieth.

## Decision

- `cmd/agent-hooks`, one stdlib binary, serves both hosts from one consumer-owned policy
  (`schema.cue`). Every spawn names its model (and, on Codex, its reasoning effort); the
  model resolves to a tier; worker briefs cap at the standard tier and critic briefs at
  deep, with an explicit tier marker to go higher. Claude Code forks, which cannot take
  a model, need a fork marker. All failed requirements are reported in one deny that
  carries the routing table. After a configurable number of denies in a row in one session
  the next spawn goes through with a note (a deny-loop breaker), so a host change that
  hides a field can never wedge a session.
- Allowed spawns are silent. Advice is policy data delivered at most once per session.
  Compressed reports come from agent definitions (the cavecrew trio, same instructions on
  both hosts), not from per-spawn nudges.
- Large main-thread Bash output in Claude Code keeps its head and tail inline and the
  full text on disk (the evidence link survives, VISION belief 3). Subagent output is
  never touched.
- Every decision, spawn result, subagent stop and tool-output size goes to a local
  JSONL ledger with no prompt or command text; `agent-hooks report` is the keep-or-kill
  instrument and `agent-hooks doctor` the existence check (I19).
- Hooks fail open on their own faults and deny only by policy. The kit names tiers and
  model families, never model ids (I2): consumers map ids in their policy.

## Rationale

Spawn time is the one moment the delegation's cost is chosen, and both hosts expose it.
Structure (required fields, agent definitions, containment) changed behavior in the
evidence; reminders did not.

## Alternatives rejected

- **Rewrite the model silently with `updatedInput`.** Cheaper than a deny-retry, but the
  agent never learns the routing and the choice is invisible in the brief. Wins if:
  measured deny-retry overhead exceeds 5% of subagent tokens over two weeks.
- **Keep per-spawn caveman nudges.** 6% compliance, measured. Wins if: never.
- **Run the main session as a restricted `--agent`.** Replaces Claude Code's whole system
  prompt to remove tools. Wins if: a host lets a main-thread agent keep the default
  system prompt.
- **Adopt a command-rewriting proxy (rtk) or a sandbox plugin (context-mode).** Vendor-
  only savings, independent benchmarks found none for rtk, and their hooks overlap.
  Wins if: an A/B in this ledger shows a net saving above 10%.
- **A memory plugin (claude-mem and similar).** Captures every tool call and injects at
  start, overlapping native memory and fathom continuity. Wins if: a cross-host recall
  need appears that thread notes cannot serve.

## Consequences

The kit now maintains a host-facing binary whose contracts drift with host releases: the
live smoke script (`scripts/agent-hooks-smoke.sh`) and the doctor are part of every host
upgrade. Codex hooks need one trust approval per handler per machine.

## Revisit when

- `agent-hooks report` over 14 days shows fewer than 60% of denied spawns recovering, or
  more than 20% recovering as forks (the gate is being routed around).
- A host changes the spawn tool name, payload fields or hook envelope (the smoke script
  fails), or Codex hook payloads gain a subagent marker on tool events.
- Allowed spawns on the fast tier stay below 15% of spawns for 30 days (the guidance is
  not landing), or contained main-thread output exceeds 20% of main-thread output.
