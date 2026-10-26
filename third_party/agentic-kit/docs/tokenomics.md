# Token economics for orchestrating agents

How to keep long Claude Code and Codex sessions that delegate to many subagents cheap and
sharp, and the hook pack (`cmd/agent-hooks`) that enforces the parts a rule can decide.
Decision: [0022](intent/decisions/0022-token-economics-is-enforced-at-the-spawn.md).

The experimental [governed spawn profile](harness-spawn-economics.md) adds a
strict private evaluator and protected HC8/HMT receiver. Required checks stay
unsampled; repeated denials never release on that path. The legacy hook behavior
described below remains unchanged. Native installation is a separate qualification.
Evidence: [2026-10-19](evidence/tokenomics/2026-10-19/README.md) ·
[tools assessment](evidence/tokenomics/2026-10-19/tools-assessment.md).

## 1. Where the tokens go

In one owner's measured Claude Code use, subagents carried 91% of all tokens and
Opus-class subagents nearly half of all output. In Codex the parent thread carried 70%.
So the levers, in order: which model each subagent runs on, how hard the parent thinks,
and how much tool output the long-lived main context absorbs. Compression of prose is a
distant fourth (about 8% measured).

## 2. Rules for an orchestrator (both hosts)

1. **Name the model on every spawn.** An unset model inherits the session's, usually the
   most expensive. On Codex also name `reasoning_effort`: an omitted effort inherits the
   parent's.
2. **Pick the cheapest tier that fits.**

   | Tier | Claude family | Use for | Effort |
   |---|---|---|---|
   | fast | Haiku | locate code, search, read and extract, classify, summarize logs or diffs, mechanical edits from an exact spec | low |
   | standard | Sonnet | implement from a spec, write or fix tests, multi-file edits, gather research, review a small diff | medium |
   | deep | Opus | adversarial review, verification, root-cause debugging, design tradeoffs, synthesis | high |
   | frontier | Fable | rare: the hardest long-horizon reasoning, the highest-stakes final judgment | high |

   Codex models map onto the same tiers in the consumer's policy (the kit names no
   model ids, invariant I2). Prices on the day: the strong tier costs about twice the
   standard tier and the fast tier about a twentieth, so most savings come from moving
   mechanical work down to fast, not from avoiding deep.
3. **Workers cap at standard, critics at deep.** A worker brief that truly needs more says
   so with `[deep]`; judgment work says `[critic]`; `[frontier]` admits the top tier. The
   marker is a visible claim in the brief, which is the point. The built-in heuristic only
   recognizes unmistakable critic words (adversarial, audit, red-team, synthesis); "verify"
   and "review" alone stay worker words, because worker briefs use them constantly.
4. **Fresh agents over forks.** A Claude Code fork inherits the session's model and
   cannot take one. Use a fresh agent with a self-contained brief; put `[fork]` in the
   prompt only when the inherited context is truly essential.
5. **Compressed reports come from agent definitions.** Workers that return site lists or
   change receipts should be the cavecrew agents (`cavecrew-investigator`,
   `cavecrew-builder`, `cavecrew-reviewer`), whose output contract is built in, on both
   hosts. Critics write full prose: their reasoning is the deliverable.
6. **Keep the main context for decisions.** Hand investigation, troubleshooting and long
   reads to a subagent and keep its conclusion, not its file dumps. A one-line fix whose
   site you already know is cheaper inline than a brief plus a fresh agent re-reading the
   file; the rule is a budget, not a ban.
7. **Long sessions:** leave continuity in fathom thread notes (or the repo's equivalent),
   clear at phase boundaries, and keep native memory indexes short (Claude Code silently
   truncates `MEMORY.md` past 200 lines).

8. **Subagent-driven development.** A fresh implementer per task from a self-contained
   brief, a review per task, a final review of the whole change. The orchestrator keeps
   a ledger of tasks and narrates one line each. Adopted as a pattern from
   obra/superpowers; the plugin itself is passed (large session-start skill, workflows
   that compete with the repo's intent practice). Pairs with fathom `review_diff` and
   `verify_plan` at the review step.

## 3. The hook pack

One stdlib binary handles every event for both hosts: `agent-hooks hook --host claude|codex`.

| Event | Claude Code | Codex | Effect |
|---|---|---|---|
| PreToolUse | `Agent\|Task` | `spawn_agent\|collaborationspawn_agent` | spawn gate: model, tier ceiling, fork marker, effort |
| PostToolUse | `Agent\|Task\|Bash\|Read\|WebFetch` | `spawn_agent\|collaborationspawn_agent\|Bash` | ledger; main-thread Bash containment (Claude Code) |
| SubagentStop | all | all | ledger: report size and transcript path |

- **Gate.** Every failed requirement is reported in one deny that carries the routing
  table, so one retry fixes everything. Allowed spawns print nothing.
- **Advice.** Policy rules (regex over the brief, per class) delivered once per session.
- **Deny-loop breaker.** After `spawn.deny_loop_limit` denies in a row in one session
  (default 6), the next spawn goes through with a note naming what was wrong, so a host
  that stops exposing a field can never wedge a session. A compliant spawn resets it.
- **Pinned agents.** When a spawn names an agent whose definition pins a model, the deny
  names that model, so the retry keeps the cheap pin instead of picking a new one.
- **Containment.** Main-thread Bash output above 10k characters keeps 4k of head and 4k
  of tail inline; the full text is saved under the state directory and its path is in
  the notice. Subagent output is never touched. Above about 30k characters Claude Code
  persists output itself and the hook stands down.
- **Ledger.** `~/.local/state/agent-hooks/ledger.jsonl`: decisions, spawn results,
  subagent stops and tool-output sizes. No prompt, command or output text. Rotates at
  16 MiB, keeps three.
- **Failure contract.** Any internal fault exits 0 with no output and a `fault` record.
  An invalid policy falls back to the built-in one and says so once per session.

## 4. Adopting it

1. **Build** from a checkout: `go build -o ~/.local/bin/agent-hooks ./cmd/agent-hooks`.
   Keep that path stable: Codex trusts each hook by its exact definition, so an unchanged
   command line keeps its approval across binary upgrades.
2. **Policy**: `agent-hooks policy default > ~/.config/agent-hooks/policy.json`, add the
   host's model ids to each tier (Claude family words already match full ids), then
   `agent-hooks policy check` (or `cue vet -d '#Policy' cmd/agent-hooks/schema.cue policy.json`).
3. **Register**: `agent-hooks wire --host claude` and `agent-hooks wire --host codex`
   merge the registration into `~/.claude/settings.json` and `~/.codex/hooks.json`:
   idempotent, append-only (Codex keys trust by array position, so existing approvals
   survive), key order preserved, a one-time `.agent-hooks.bak`, `--dry-run` to preview.
   Commands use `$HOME/.local/bin/agent-hooks`, so one file serves every machine. Where
   another tool owns the file, merge `agent-hooks print-hooks --host H` into its source
   instead. Remove any older spawn gate: doctor warns when two gates decide one call.
4. **Trust (Codex)**: approve the new handlers once with `/hooks`. Untrusted hooks are
   skipped silently.
5. **Agents**: install `cmd/agent-hooks/assets/agents/claude/*.md` into
   `~/.claude/agents/` and `…/codex/*.toml` into `~/.codex/agents/`.
6. **Safety nets** for paths the gate cannot see: Claude Code `CLAUDE_CODE_SUBAGENT_MODEL`
   (used when neither the call nor the agent names a model); Codex
   `[agents] default_subagent_reasoning_effort`.
7. **Verify**: `agent-hooks doctor` (policy, self-test, wiring, trust, binary, ledger,
   agent drift) and `scripts/agent-hooks-smoke.sh` (live sessions on both hosts, a few
   cents).
8. **Measure**: `agent-hooks report --days 14` is the keep-or-kill instrument for every
   rule; decision 0022 names the readings that reopen it.

## 5. Living with other hooks

- Hosts run all matching hooks in parallel. For PreToolUse the strictest decision wins
  (deny over ask over allow), so the gate composes with other policy hooks.
- One output rewriter per tool. This pack is the only PostToolUse rewriter of Bash
  output; do not add a second (command proxies, sandbox plugins) on the same matcher:
  there is no merge order.
- `additionalContext` accumulates on every call that emits it. This pack emits it only
  for once-per-session advice; prefer that discipline for every hook.
- Codex: each handler needs trust; keep command strings stable and fold new behavior into
  the binary rather than new hook entries.

## 6. Companion tools

The owner's adopted set (assessed 2026-10-19, full ratings in
[tools-assessment](evidence/tokenomics/2026-10-19/tools-assessment.md)). None is required
by the hook pack.

| Tool | Role | Why | Risk |
|---|---|---|---|
| ccusage, `@ccusage/codex` | Whole-session dollar view from local logs, by day, session and model | Complements the per-spawn ledger: together they show whether routing changed the bill | Figures are API-equivalent estimates, not subscription charges |
| ccstatusline | Claude Code status line showing context-window percent | Only the status line receives the percent, hooks do not; keeps the main context visible | Owns the one status line slot. Codex has its own: `[tui] status_line` in config.toml, a list of item ids such as `model-with-reasoning`, `context-remaining`, `used-tokens`, `git-branch` (Codex 0.163; Codex does not validate the ids, so check them against `codex --help` output or the binary) |
| ast-grep | Structural search and rewrite by syntax tree | One exact command a fast-tier builder runs instead of many edits. Fathom says what to change and who breaks, ast-grep changes it | A wrong pattern rewrites broadly: dry-run, then review the diff. Zero matches is unproven until `rg` agrees: in Go, a call pattern whose only argument is `$$$` (e.g. `f($$$)`) matches nothing, so anchor one argument (`f($A, $$$B)`). |
| difftastic | Syntax-aware diffs for human review | Moved and reformatted agent code becomes obvious | Use the `git dft` alias (`-c diff.external=difft diff`). Never set `diff.external` globally: agents and tools parse unified diffs |
| gitleaks | Machine-wide secret scan before every commit | A deterministic gate behind the rule that secrets never enter a repo | False positives: add an inline `gitleaks:allow` |
| context7 (both hosts) | Third-party library docs on demand | Replaces web fetches into the main context | Library names leave the machine |

gitleaks runs as a git 2.54 config-defined hook, so one setting covers every repo:

```
hook.gitleaks.event=pre-commit
hook.gitleaks.command=<path>/gitleaks git --pre-commit --staged --redact --no-banner
```

It runs alongside lefthook's hook-directory scripts, not instead of them (verified
locally 2026-10-19: the config hook ran, then the hook-directory script).

### Skills

Keep a few focused, hand-curated skills. Give every skill relied on a one-line pointer
in AGENTS.md, then test implicit triggering on each host. Installation alone proves
neither invocation nor benefit. [Assessment and usage](evidence/tokenomics/2026-10-20/skills-assessment.md).

Front-load trigger words in descriptions and keep them under about 400 characters:
every description competes in the session listing. Claude Code budgets that listing
as a fraction of context; Codex also caps metadata. Move procedure into the body.
Copy third-party skills from a pinned commit after reading every file, including
scripts and assets. Keep licenses and exact patches; never use an installer skill or
automatic updates. Prune by measured use and keep removal reversible.
On Codex, set `policy.allow_implicit_invocation: false` in `agents/openai.yaml` for a
specialist skill that should run only when named. Keep ordinary adopted skills implicit.
Pair the method with fathom evidence: the skill says how, fathom supplies evidence and
receipts. Compose these roles instead of growing fathom's tool surface.

## 7. What it does not cover

Claude Code Workflow-script agents and agent teams were not observed reaching
PreToolUse; hosted tools never hook; Codex tool events do not say which thread made the
call, so containment is Claude Code only. Markers declare intent; they do not prove the
work's class. Hooks are guidance with a measurement loop, not a security boundary.
