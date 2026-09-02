# Running OpenViking beside fathom

**Status: untested.** Nothing in this guide has been run. It is derived from
OpenViking's published documentation and from reading fathom's own hook
stack, and it is written so an adopter who wants both can set them up
without the two fighting. The first person to actually run this pair should
correct it (`fathom observe`), and the "Verified" line at the bottom should
stop saying what it says.

Decision this guide implements: **ADR-028** — external agent memory is a peer
process, never a dependency.

## The one-line answer

Yes, they coexist, and for some people both is genuinely better than either.
They store different things and fail in opposite directions: fathom is
high-precision and low-recall about code, OpenViking is high-recall and
low-precision about conversations. The only thing that must not happen is
OpenViking's memory being believed about code.

## What each one is for

| | fathom | OpenViking |
|---|---|---|
| Subject | source: symbols, call edges, obligations, plan proof | conversation: decisions, preferences, what was tried |
| How a fact is retrieved | exact anchor — symbol, file, glob, commit | vector similarity over LLM-distilled summaries |
| How a fact goes stale | tracked; anchor changes flip a claim to `suspect` | not tracked; a distilled memory does not know its source moved |
| How you capture something | deliberate: `fathom thread note`, an observation, a plan row | automatic: it reads the session and distils |
| Failure mode | forgets what nobody wrote down | remembers something that is no longer true, confidently |

Those failure modes are complements, which is the entire case for running
both. Fathom's continuity threads are only as good as the discipline to
write them; OpenViking captures the things nobody remembers to file.

## The rule that makes it safe

> **Conversational memory may recall and may suggest. It may not assert a
> fact about code.** Anything about a symbol, a caller, a file's obligations,
> or whether a plan row is proven comes from fathom, with its anchor and its
> freshness state. If an injected memory and the index disagree, the index
> wins and the memory is stale.

Put that in `CLAUDE.md` / `AGENTS.md` if you run both. It is a one-paragraph
addition and it is the whole safety story — without it you have an unanchored
memory injecting confident sentences about code into every turn, which is the
exact failure `suspect` exists to catch.

## Hooks: they overlap, and that is fine

Claude Code hook entries are arrays, so several tools can register on one
event and all of them run. Fathom and OpenViking overlap on four:

| Event | fathom | OpenViking |
|---|---|---|
| `SessionStart` | `fathom session-hook` — identity, freshness, thread brief | session memory injection |
| `UserPromptSubmit` | — | per-turn memory injection (≤2k tokens) |
| `PreToolUse` | `fathom edit-hook --pre` | — |
| `PostToolUse` | `fathom edit-hook` — obligations | — |
| `Stop` | `fathom stop-hook` | archives the turn |
| `PreCompact` | `fathom pre-compact-hook` | archives before compaction |
| `SessionEnd` | `fathom session-end-hook` | archives the session |

Nothing here is exclusive. Two practical cautions:

1. **Watch the session-start budget.** Fathom's brief is capped and
   source-gated on purpose. OpenViking's injection is a separate budget on
   the same context. If sessions start feeling front-loaded, cut OpenViking's
   injection budget before cutting fathom's brief — the brief is the smaller
   of the two and carries the freshness line.
2. **Hook latency is additive.** Both run on every `Stop`. If turns feel
   slow, time them individually before blaming either.

## Setting it up

1. Get fathom healthy first, alone: `fathom doctor` clean, `fathom verify`
   exit 0, the MCP tools answering. Adding a second system to a half-wired
   first one makes both undiagnosable.
2. Install OpenViking per its own instructions. It is a Python server plus a
   model provider (or a local Ollama); it does not need fathom's daemon and
   fathom does not need it.
3. Add the authority rule above to `CLAUDE.md` and `AGENTS.md`, in both
   files identically (`intent-doctor` check 15 enforces that they match).
4. Re-run `fathom doctor`. It reports fathom's wiring only, so a clean
   doctor after the install means OpenViking did not disturb fathom's hooks
   — which is the thing worth checking.
5. Give it two weeks, then decide with the call log rather than the vibe:
   `fathom instrument --adoption` shows whether fathom's surfaces are still
   being used at the same rate. A memory tool that is answering code
   questions in fathom's place will show up there as a drop, and that is the
   signal to enforce the authority rule harder.

## What NOT to do

- **Do not vendor, link, or embed it.** ADR-028 clause 1. It is AGPL-3.0;
  running it as a separate process is unencumbered, but linking is not.
- **Do not let it index the repo as a "resource".** It would produce a
  second, unanchored, staleness-blind answer to questions the index already
  answers exactly. Point it at conversations, not at source.
- **Do not migrate threads into it.** A thread note is a durable, dated,
  human-authored claim. A distilled memory is a similarity score. The
  migration is lossy in the direction that matters.

## Is it beneficial?

For a solo repo with disciplined thread notes: marginal. Fathom's continuity
layer already covers the ground, and the second system is mostly cost.

For someone working across many repos and machines, or who does not reliably
file notes: probably yes, and the cross-device recall is the real draw. The
honest framing is that OpenViking is a good answer to "what did we decide
three weeks ago" and a bad answer to "what calls this function" — so run it
for the first question, and never let it be asked the second.

---

Verified against: nothing yet — see the status line at the top. OpenViking
facts as published on 2026-09-01 (`volcengine/OpenViking` v0.4.17.1,
AGPL-3.0).
