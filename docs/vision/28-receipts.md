# 28 — Receipts: every answer, hand-off and reading carries the evidence it rests on

Status: ACCEPTED 2026-09-13 (ADR-034). Plan: `docs/plans/receipts.md`.
Builds on 27 (the strengths bench), 24 (work frontier: `ctx:` pointers
resolved fresh), 21 (threads hold pointers, never knowledge), 19 (the
exploration ledger), ADR-028 (memory recalls, the index asserts).

## 0. The verdict in one paragraph

The first strengths-matrix round (27) was honest about its numbers and
wrong about what they measured. A second reading of the raw transcripts
found that in all 72 wired sessions the fathom MCP server was absent (the
driver handed the harness the un-rendered arm and `--strict-mcp-config`
hid the worktree's own entry), that the graphify arm's server failed in 12
of 12 sessions on a wrong command, that gopls and Serena connected and
were never called once, that every session — control included — received
fathom's own live session primer from the operator's global hooks, and
that the thread brief crossed from one task into another because its scope
was a directory basename. So M3, M4, M5 and the paid half of M6 measured
prompt text against prompt text, and M1 measured hooks without tools. The
lesson is not "be more careful". It is fathom's own thesis turned on the
bench: a reading, a hand-off and an answer are claims, and a claim without
its evidence is not made. This vision makes that mechanical in four places:
the instrument (truth flags on every cell, evidence anchors the verify gate
checks), the lookup surface (a receipt on every hit, a budget on every
tool), the hand-off (discoveries as pointers with provenance, resolved
fresh for workers and the next session), and tests (coverage as a code
fact by layer, judgment left to the review kit).

## 1. What the second reading found (the facts this vision answers)

| # | Fact | Where it bites |
|---|---|---|
| F1 | `mcp_servers: []` in 72/72 wired scenario sessions; the rendered MCP config never reached `claude -p`; and `RenderArm` truncated the hooks `fathom init` had written to one nonexistent command, so the wired arm had no fathom hooks either (found by the review of the fix) | M1–M5 readings measure no tool and no hook |
| F2 | graphify server `failed` 12/12 (`graphify --mcp` is not a command; the documented server is `python -m graphify.serve <graph.json>` under uv); gopls and Serena `connected`, 0 calls in 24 sessions | M6/M7 paid rows measure grep five times |
| F3 | `permissionMode: auto` in every session; `allowed_tools: [Read, Grep, Bash]` arms wrote files 35 times | arm restrictions are decorative |
| F4 | every session inherited the operator's config: ~90 slash commands, plugins, and fathom's own SessionStart primer on the control arm; one home cell answered a Stop hook's plan snapshot instead of the task | the control is not a control |
| F5 | `threadScope{repo: filepath.Base(repoRoot)}`: two tasks pinned to one commit shared a worktree basename and each other's thread notes; session two of the wired arm re-verified the other task's note (11–25 pre-edit calls v 3–7) | M4 "refuted" was contamination |
| F6 | compile cost never reached the ledger; the overlap reading weighed only `Read`, while workers discovered through Grep and `cat` | M5's 13–34 % v 61–100 % rests on 0–3 Read calls per worker |
| F7 | home bench, tools present: `find_callers("run")` 28 KB, `staleness_report` 46.5 KB, `outline` 16.7 KB twice — the five uncapped tools are ~35 % of the exclusive arm's 2.66× | H2-3 |
| F8 | home bench: 27 % of Grep calls are followed by a Read of the same file; a fathom answer is trusted without re-check 85–90 % of the time when called; `staleness_report` called once in 156 opportunities on the two staleness tasks | the search receipt (SB-48); routing text |
| F9 | `references` on an interface method could not say which types implement it; the agent read a 19 KB file to count them | one missing shape |
| F10 | SB-cli__cli-10009's fail_to_pass names unit tests of a package that does not exist at base; 10072's oracle asserts exact CLI text a behaviourally equal fix fails | oracle honesty |
| F11 | SubagentStart injects 600 bytes of constant text; nothing carries a planner's discoveries to a worker except a plan row's `ctx:` pointers pasted by hand | the hand-off gap |
| F12 | guards corpora: every rule sat in a file both arms read before the edit (CLAUDE.md, or the comment above the edit site); zero violations both arms is the corpus, not the claim | M1 needs rules far from the edit |

## 2. Receipts on the instrument

1. **Instrument truth on every cell.** A session's init event is parsed:
   MCP server statuses, tool count, and a histogram of every tool call.
   A wired or surface arm whose server is not `connected`, or whose
   surface was never called, is LOST or flagged `surface_unused` — the
   row carries `tool_calls` and `surface_calls` so a stranger can see the
   tool was used, not just installed.
2. **A clean room for both arms.** Sessions run under Claude Code's
   `--restricted` mode with the arm's own rendered settings file as the only
   settings source (no operator hooks, plugins or slash commands; the login
   stays intact — a scratch config dir loses it), `--permission-mode default`
   so an allow-list denies, `--tools` naming the arm's tools, a per-cell
   `FATHOM_THREADS`, and the arm's own hooks only. The home bench does the
   same. The control arm sees nothing of fathom's.
3. **Fair competitors.** An arm's surface gets the same shape of routing
   stanza fathom's does (which verb answers which question), because F2
   showed a tool that is present and un-named is a tool never used. The
   competitor's stanza is data on the arm, in the corpus, beside its
   prediction.
4. **Structured readings, not prose.** `rule_served_before_edit`,
   `violation`, `tokens_to_first_edit` for every session,
   `facts_reused`, an overlap that weighs Read, Grep and shell reads,
   `trusted_without_recheck` on the home bench, and the compile cost on
   the row. A verdict in the README is rendered from these fields.
5. **Evidence anchors the gate verifies.** A published reading is a plan
   row with an `evidence:` anchor (`ledger:docs/bench/scenarios-ledger.jsonl#<run_id>/<task>/<arm>`);
   `fathom verify` resolves the anchor and refuses a row whose cell is
   LOST, `surface_unused`, or absent. Numbers in the README come from the
   render, never from a keyboard.
6. **Cost discipline.** Screen at n=2, escalate to n≥3 only where the
   delta sits within the criterion's margin; turn budgets set from
   utilization (two guards tasks hit 50/50 four times; the lookups never
   passed 15 of 30); treatment and control cells of one task run
   concurrently; an oracle that cannot be won from the task text is
   dropped with the reason on the corpus (10009).
7. **The verdict in plain language.** Every artifact and the README's
   verdict section open with where fathom stands, better or worse than
   which tool, which tools it pairs with, and whether it is worth keeping
   — before any ratio.

## 3. Receipts on the answer

1. **A budget on every tool.** `find_callers`, `outline`, `symbol_context`,
   `repo_map` and `staleness_report` join `references`, `query_symbols`
   and `read_source` under the 16 KB discipline: elide, count, and name
   the refinement (`scope: package`, `file:`) in the elision line.
2. **A generic name asks for scope.** A caller or reference query whose
   fan-in exceeds the budget returns the count, the top packages, and the
   refinement — never 28 KB of `run`.
3. **Search with a receipt** (SB-48, the registered reply to H2-3): grep
   underneath, each hit annotated from the index with its enclosing symbol
   and kind, so the Read that followed 27 % of Greps is not needed.
4. **Implementations on interface hits.** `references` on an interface
   method returns the implementing types first (F9).
5. **Routing names the tool for the shape.** "Is this still true / what
   was known" names `staleness_report`; "what governs this repository"
   is `obligations` with no path (root conventions); a doc-versus-doc
   contradiction is grep's, and the text says so.

## 4. Receipts on the hand-off — discoveries

A **discovery** is a pointer a tool answer established — `symbol
internal/mcp.registerReferences → references_tool.go:41`, `callers of
PutObservations (7)`, `rules for internal/store/*`, `read internal/core/model.go`
— with its provenance: the tool, the time, the index watermark. It is
never rendered text (24 §2: authored text is stale state); it is resolved
fresh at read time and the index marks what no longer holds (ADR-028).

- **Write path, mechanical.** The MCP call log gains the answer's top
  pointers; an agent may also file one (`fathom thread point`). No model
  distills anything (21 §4).
- **Workers.** SubagentStart renders the parent session's discoveries in
  this repo since its start — pointers resolved fresh, stale ones marked —
  within 1 200 bytes; the `discoveries` tool (and `fathom discoveries`)
  pulls more on demand. The scenario driver gives its workers the same
  brief, so M5 measures the feature and not the prompt.
- **The next session.** The session-end note carries the session's top
  pointers; the SessionStart thread brief renders them resolved fresh in
  its existing budget.
- **Routes** (long-lived discovery paths). `fathom routes` derives, per
  package or concern, the pointers established in two or more sessions
  over the last thirty days, resolved fresh and dropped when the index
  no longer has them; the primer names routes in one line only when one
  exists; `thread promote` can write a route onto a plan row's `ctx:`.
- **Scope by identity.** The thread scope is the repo's identity (origin
  URL, else path), not a directory basename (F5).
- **Kill criteria, pre-registered.** M5 at K=3 with the worker brief:
  `facts_reused ≥ 3` per worker and tokens-to-first-edit below the
  control on two of three tasks, else the worker brief is removed and
  `discoveries` stays pull-only. Routes: pulled in five of the next twenty
  sessions on this repo, or removed.

## 5. Receipts on tests — coverage as a code fact

Fathom asserts facts, the review kit judges. Facts fathom can own from a
supplied coverage profile joined to the index: which statements of which
symbol ran under which tests; the test-to-subject map; which changed
symbols have no covering test at any layer; a layer name per test from a
repo-declared, open-vocabulary config (default: `_test.go` is `unit`,
build tags and directory names as declared). Judgments — is the test
meaningful, is the mock honest, is the floor right — stay in the review
kit (it already models fathom as one adapter and parks coverage as
context) and in test-kit (declared suites, floors, attestations). First
slice: `fathom tests coverage --profile` and a coverage block on
`review_diff`/`fathom review --json`, which is the one surface the review
kit consumes. An obligation of the shape "a change to X needs a test at
layer Y" waits for a consumer.

## 6. Cost and sequence

| Wave | What | Spend |
|---|---|---|
| W0 | instrument truth, clean room, fair competitors, structured readings, cost discipline | free |
| W1 | budgets on every tool, scoped generic names, search with a receipt, implementations, routing | free |
| W2 | discoveries: call-log pointers, worker and session briefs, routes, thread scope | free |
| W3 | evidence anchors in the gate; the README/artifact verdict rendered | free |
| W4 | coverage first slice | free |
| W5 | reruns: dry cells; lookups five arms; guards two corpora; public three tasks; reuse K=3; freshness; home | ≈ $120 pilot tier |

Then the loop again: read the transcripts before the numbers.

## 7. What would falsify this vision

- The rerun with every tool present and a receipt on search still shows
  the lookup surface above 1.0× grep on payload with equal success: the
  token claim is withdrawn and the lookup tools are kept for the review
  and verify use cases only.
- The discoveries kill criteria fire (§4).
- A reading published after this vision is later found on a LOST or
  `surface_unused` cell: the anchor check failed, and bench readings
  leave the docs until it is repaired.
