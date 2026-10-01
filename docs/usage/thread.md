### `thread` — continuity across sessions (vision 21)

```sh
fathom thread brief   [--budget-bytes 2400] [--repo .] [--source startup|clear|resume|compact|fork] [--all-repos] [--json]
                                               # what a fresh session gets injected (startup/clear only; resume/compact get nothing)
                                               # --json: {kind:"brief", source, budget_bytes, bytes, text} — text is "" on resume/compact
fathom thread note    "text" [--thread T] [--kind K] [--completes-when …] [--repo .]   # file a handoff BEFORE /clear
fathom thread point   "what happened" [--session S] [--thread T] [--repo .]          # side-task pointer
fathom thread note    --map ID --anchor <symbol|path#Lx-Ly> --why "…" [--agent ID] [--completes-when …]
                                               # an exploration-map entry (ADR-056); see `map` below
fathom thread note    "text" --map-ref ID[,ID…] # a note that references maps; the brief re-surfaces them (XM-13)
fathom thread list    [--all-repos] [--all] [--thread T] [--repo .] [--json]   # this repo's open items; --all-repos: every repo; --all: also closed + dormant, every repo
                                               # --json: {kind:"list", items:[{id,status,kind,thread,text,repo,session_id,filed_at,closed_at,closed_by,commit,pointers}], hidden_other_repos}
fathom thread close   <id> [--why …]
fathom thread promote <thread> [--out docs/plans/<thread>.md] [--repo .]   # scaffold a thread's history into a plan
fathom thread handoff [--repo .]               # derived in-flight state for a handoff
```

Store: `~/.fathom/threads.jsonl` — user-central, append-only,
latest-row-wins; never shared (law 13). Deposits from the lifecycle hooks
(`session-end-hook`, `pre-compact-hook` — wired by `fathom init`) are
mechanical facts only. `--kind` is one of `decision-pending`, `spec-unbuilt`,
`in-flight`, `side-task`, `handoff-note`, `session-end`, `pre-compact`
(default `handoff-note`). Items with `completes_when`
(`file_exists:<path>` / `plan_row_checked:<plan>#<ROW-ID>`) close themselves
when reality catches up. The brief is byte-budgeted and enforced; overflow is
named (`and N more`), never silently dropped.

**Repo-scoped (TH-6).** `note`/`point` file on the current repo's
daemon-resolved service id by default (`general` only when nothing resolves;
`--thread` wins). `list` and `brief` show this repo's items and print one
line `N open item(s) in other repos — fathom thread list --all`; untagged
legacy items are never hidden. The continuity-thread bet itself is
**experimental** (4/20 units collected).
