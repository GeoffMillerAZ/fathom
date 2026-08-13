# 21 — Continuity threads: the third dimension

> Owner ask, 2026-08-12: *"a memory that's tied to continuity rather than a
> session thread… take just the continuity-style stuff — what plans we had
> going, decisions made or left hanging, specs not yet made or made but not
> implemented — cut out the context pollution, but know side tasks happened
> and point to the session id where they happened… maybe memories get
> stronger with use and age out… help me ask the bigger questions. Should
> we just skip this and update CLAUDE.md better? And it must be compatible
> with Claude Code, Codex, whatever."*
>
> Also binding, same day: *"we need to make sure we are considering how
> many tokens it would take to enable this and how many it would save."*

---

## 1. The framing that makes this a vision and not a feature

Fathom has two dimensions today:

- **Space** — where things are and what governs them: the index,
  obligations, guards. Mature.
- **Truth** — what is known and how verified: evidence tiers, anchored
  memory, the verify gate. Landing now (phase W).

The ask names the missing third: **Time** — what is *in flight*. What were
we doing, what did we decide and not yet write down, what is specced but
unbuilt, where did that side-task happen. Today that state lives in one
place only: the harness session's context window. Which means it dies at
`/clear`, decays at every compaction, and — the economically absurd part —
is kept alive by *paying rent on the entire history every turn*.

The unit of work was never the session. A **thread** — one continuous
strand of intent, spanning sessions, sometimes spanning repos — is the
real unit, and no tool owns it. The session is a harness artifact; the
thread is the work.

**The vision play:** fathom becomes the continuity layer that makes
sessions *disposable*. The index already makes code knowledge
session-independent; obligations make governance session-independent;
threads make **intent** session-independent. That is the full claim behind
"ultimate agentic coding assistant": nothing of value lives only in a
context window.

## 2. The tokenomics, first — because it gates everything (owner rule)

**Cost side (bounded, computable now):**

| Component | Context tokens |
|---|---|
| Deposits (Stop, SessionEnd, PreCompact hooks write to disk) | **0** — hooks inject nothing on the way out |
| Thread brief at SessionStart | **≤ ~600**, budget-bounded, and ONLY on `startup`/`clear` — never on `resume` (context survives) or `compact` (the summary survives) |
| MCP schema tax | **0 to start** — CLI-first (`fathom thread …`); no new agent-surface tools until the ratio work (phase X) can price them |
| LLM calls in the loop | **0** — deposits are mechanical (§4); there is no summarization lane |

**Savings side (never asserted — pre-registered, measured by instruments
that already exist):**

The candidate saving is the single largest spend line this program has
ever measured. One resumed mega-session showed **896M cache-read tokens
against 4.06M real tokens** — at cache-read pricing (~10% of input), that
is ~90M input-equivalent, **~22× the real spend**, paid as rent on
accumulated context. The rent exists *because* clearing a session
currently destroys continuity. If threads make `/clear` cheap, the rent
stops being mandatory.

**Pre-registered bet (filed before any thread data exists):** in
ledger-measured terms, a session opened with a thread brief after `/clear`
reaches its first edit within the warm-up band of resumed sessions
(`first_edit_turn`, IQ-2), at a lower all-in per-turn cost
(in + out + cache-read×0.1, per assistant turn). If after 20 fresh-with-
brief units the warm-up band is not met, **the brief is not working and the
injection is removed** — a continuity feature that does not pay its ~600
tokens is context pollution with a good conscience.

Worst case is therefore bounded: ~600 tokens on some session starts, for a
shot at the 22× line. That asymmetry is why this is worth building at all.

## 3. The refusal boundary — what a thread is NOT

This only stays excellent if the boundary is brutal:

- **A thread is not knowledge.** Claims about the codebase go to anchored
  memory (evidence-tiered, drift-expiring) or to plans (gate-checked).
  A thread holds **intent, state, and pointers** — things that are true
  *by construction* ("we were doing X", "it happened in session S") rather
  than assertions needing verification. Different truth model, different
  store, and the reason phase W's refusals don't apply here.
- **No transcript mining.** anchored-memory.md §5's refusal stands in
  full: no LLM summarizes a session into "lessons learned". Every deposit
  is either **derived** from durable artifacts or **explicitly filed** at
  the moment it happened. Mechanical, falsifiable, zero generation cost.
- **Not a general note store** (vision 20 X3 stands), **not shared**
  (law 13 — user-central, local, per-repo-taggable, sharing is a human
  verb), **not a to-do app** — plans remain the work ledger; threads point
  at plans, never replace them.

## 4. The mechanical model

```
thread  := { id, title, repos[], opened, last_touch, state: open|dormant|closed }
item    := { thread, kind, text, pointers{session_id, repo, plan_row?, commit?, path?},
             filed_at, filed_by, completes_when? }
kinds   :=  decision-pending   — chosen but not yet durable (no ADR/CHANGELOG yet)
            spec-unbuilt       — designed, not implemented (often derivable)
            in-flight          — stashes, WIP branches, half-done edits (derivable from git)
            side-task          — "X happened; transcript session_id=S has the detail"
            handoff-note       — explicitly filed for the next session
```

Three properties carry the design:

**Derived beats deposited.** Most of what the owner listed is *already
derivable* from durable state: open plan rows (`fathom plans next`),
claimed-unproven boxes, specs-without-code (design docs whose plan rows
are unchecked), stashes and WIP branches (git). The thread store holds
only what is NOT derivable: pending decisions, side-task pointers,
handoff notes. The brief composes both at read time — so the store stays
small and cannot drift from reality it merely mirrors.

**Items close themselves.** `completes_when` names a checkable condition —
a plan row proving, a file existing, an ADR appearing, a stash landing.
The brief never shows a completed item; the same self-expiring philosophy
as anchored memory, keyed on *artifact existence* instead of anchor drift.
"Strengthens with use / ages out" (the owner's instinct) is **reframed**:
recency-of-touch ranks what fits the ≤600-token brief — it is a *ranking*
signal, never a truth signal; an untouched open item ages to `dormant`
(visible via CLI, absent from the brief) and never silently vanishes.

**Pointers, not prose.** A side-task item is one line plus a session id.
The transcripts already exist on disk keyed by session id; fathom never
copies their content — it keeps the address. "We hit this purge
blast-radius question before" costs a pointer, and re-reading the detail
costs nothing until the day it is actually needed.

## 5. Harness adapters (the compatibility answer)

The store and verbs are harness-neutral; adapters are thin:

| Moment | Claude Code | Codex |
|---|---|---|
| deposit, incremental | Stop hook (already ours) records mechanical pointers | PostToolUse where available; else CLI |
| deposit, terminal | **SessionEnd** (fires on `/clear`, reason attached) finalizes the session's items | — (no lifecycle hooks) |
| deposit, pre-loss | **PreCompact** — the exact moment in-flight state dies today; capture BEFORE the summary is made | — |
| inject | **SessionStart** `source`-aware: brief on `startup`/`clear`; **nothing** on `resume`/`compact` | AGENTS.md stanza instructs `fathom thread brief` at start |
| on demand | `fathom thread …` CLI (both harnesses); MCP tools only after phase X can price their schema | same |

The `source` field is what keeps the cost model honest: the brief injects
exactly when context was actually lost, and never duplicates what a resume
or compaction summary already carries.

## 6. "Should I just update CLAUDE.md better?" — the split

CLAUDE.md is for **stable law** — it churns rarely, is committed, and is
shared with every session including other people's. Threads are
**in-flight state** — churning, personal, per-moment, wrong to commit.
This morning's compositional restructure made CLAUDE.md excellent at its
half; nothing makes it right for the other half: a CLAUDE.md that carried
"we stashed W1 mid-flight and V is held for rework" would be stale within
hours and polluting every session start until someone noticed. Different
lifetimes, different homes. (The one overlap: a decision-pending item that
*graduates* — its durable form lands in ADR/CHANGELOG/CLAUDE.md and the
item closes itself. Threads are the airlock, not the archive.)

## 7. Gate on the asks

| Ask | Verdict |
|---|---|
| Hook on `/clear` + inject at next start | **ACCEPTED** — SessionEnd/SessionStart(`source`) exist and are exactly shaped for it |
| "Continuity thread rather than session thread" | **ACCEPTED as the core object** — the thread is the unit of work; sessions are events on it |
| Side-tasks findable by session id | **ACCEPTED** — pointers, not copies |
| Memories strengthen with use / age out | **REFRAMED** — recency ranks the brief; age moves items to dormant. Never a truth signal: a "strengthened" wrong item is just confidently wrong (law 14) |
| LLM distills the session at end | **REJECTED** — transcript-mining refusal stands; deposits are mechanical or explicit |
| Skip it, update CLAUDE.md | **REJECTED as either/or** — CLAUDE.md got its upgrade this morning; it structurally cannot hold in-flight state (§6) |
| Cross-harness | **ACCEPTED with the guards precedent** — hooks where they exist, AGENTS.md+CLI where they don't; the store never knows which harness wrote to it |

## 8. Phases

| Phase | Ships | Terminal | Kill |
|---|---|---|---|
| **TH-core** | thread store (user-central JSONL), `fathom thread` verbs (brief/note/point/list/close), derived-state composition, `completes_when` self-closing | a handoff note filed before `/clear` appears in the next session's brief, and a completed plan row closes its item with no human action | any deposit path that calls an LLM |
| **TH-hooks** | SessionEnd + PreCompact deposits, SessionStart `source`-aware injection (≤600 tokens, count enforced by test), Stop-hook mechanical pointers | `/clear` → new session knows the open threads; `resume` injects **nothing** | injection on `resume`/`compact`, or an unbounded brief |
| **TH-measure** | fresh-with-brief units labeled in the ledger; the §2 pre-registered bet evaluated in `fathom flywheel`/brief | the warm-up + all-in-cost comparison publishes with denominators | the band unmet after 20 units → **the injection is removed** (the registered kill) |
| **TH-codex** | AGENTS.md stanza + CLI parity | a Codex session reads the same brief by instruction | — |

## 9. The spine — threads make SDD end-to-end (owner ask, same day)

Fathom already owns every STATION of a unit of work's lifecycle. What it
lacks is the conveyor between them:

```
intent ──► plan ──► build ──► proof ──► record ──► knowledge
thread     docs/     obligations verify   CHANGELOG  anchored
(new)      plans/*   + guards    gate     + ADR      memory
```

Every station exists and is individually checkable. **The thread is the
spine that gives one piece of work identity across all of them** — and the
hooks are the conveyor that advances it without ceremony:

- **plan ⇢ thread (derived, zero deposits):** the brief's "what is next"
  IS `fathom plans next`, filtered to the thread's repos and rows. SDD
  stays the source of truth; the thread never duplicates it.
- **proof ⇢ thread (the Stop hook already runs verify):** a row proving
  out closes its thread item (`completes_when`); a row claimed-unproven
  becomes the next session's loudest brief line. The gate's verdict and
  the continuity story are the same fact, seen twice.
- **session ⇢ record (SessionEnd/PreCompact):** the mechanical SDD delta —
  which boxes flipped this session, which claims remain unproven — is
  deposited as pointers at exactly the moments that context dies.
- **intent ⇢ plan (`fathom thread promote`):** a thread that has
  accumulated real items graduates into a gate-ready plan scaffold (the
  fathom-plan grammar), items becoming candidate rows. The airlock feeds
  the ledger; nothing skips the gate.
- **thread ⇢ any agent (`fathom thread handoff`):** the brief is not
  Claude-shaped — it is the natural handoff artifact to a subagent, a
  Codex session, or another machine. The multi-agent story arrives free,
  because continuity was never stored in a context window to begin with.

Two hook opportunities deliberately HELD, with reasons:

- **UserPromptSubmit** (exists in Claude Code) could match a prompt against
  open threads and inject the relevant one mid-session. Held until phase X
  can price it: it fires on every prompt, and an unbudgeted per-prompt
  injection is the schema-tax mistake in a new costume.
- **Per-edit plan-drift flags** (PostToolUse knows the file; verify knows
  which contracts anchor it) could warn "this edit touches plan row X"
  live. Held: the Stop hook's session-end verify already carries this at
  zero marginal context; per-edit only earns its cost if sessions
  demonstrably push unproven claims mid-stream.

What makes this world-class rather than a feature pile is a single
invariant, now stated once for the whole program:

> **Nothing of value lives only in a context window.** Code knowledge
> lives in the index; governance in obligations; truth in the gate and
> tiers; lessons in anchored memory; intent in threads. The session is
> disposable — and the instruments (§2) exist to prove that disposability
> is CHEAPER, not just cleaner.

## 10. Falsification

- If, after 30 days, threads hold mostly items that were *derivable
  anyway*, the store is ceremony — collapse to a pure derived brief and
  delete the deposit verbs.
- If briefs are consistently ignored (fresh sessions re-ask what the brief
  answered — measurable from repeated early discovery), the format is
  wrong before the idea is; fix ranking before adding content.
- If the §2 bet fails, the kill executes as registered. The thread CLI may
  survive for humans; the *injection* dies.
