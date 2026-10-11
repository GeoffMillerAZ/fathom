# Hook-path abandonment: most test-run hook answers never arrive (hygiene note G11, 2026-10-11)

Reading: in the call log from 2026-10-09 00:00Z to 2026-10-11 18:32Z, `localize` from the test-run hook was abandoned
200 of 223 times, `obligations` from the edit hook 58 of 259, and `query_symbols` from the session hook 7 of 11, each
with "the client went away". Untagged MCP clients also abandoned `symbol_context` 113 of 453 and `query_symbols` 49
of 175.

Plain reading: under the load of 2026-10-09 to 2026-10-11 (two or three worker waves at once), the test-run hook's
`localize` call almost never returned inside the hook's own deadline of about 2 s. Its median latency is 2,003 ms,
which is that deadline: the hook gave up, closed the connection, and the daemon recorded the call as abandoned. A
hook answer that times out is silently absent, so the agent saw no test-run context on 9 calls in 10 and nothing told
it so. The edit hook's `obligations` call was abandoned about one time in five, at a median of 2,037 ms for the
abandoned calls against 379 ms for all of them. This is a defect to fix, not a property of fathom on a quiet machine:
the same calls on an unloaded host are unmeasured. Fixes on the table: a deadline sized to the call class, a cheaper
answer shape, a named silence instead of an empty one, or dropping the call from the hook.

`G11` is the operator's hygiene-queue label for this finding (2026-10-11 self-use notes). It is not scope goal G11.

## Provenance

- Source: the daemon's MCP call log (`workspace/logs/mcp-calls.jsonl` under the owner's checkout, schema version 3),
  read on 2026-10-11 by `docs/bench/readings/self-use/call-log-abandonment.py` with the window above. $0, no network,
  no model. Aggregate counts only: no path, session id or caller pid is committed (ADR-011).
- Graded *measured*. The self-use notes of 2026-10-11 18:20Z recorded 6,751 calls and localize 200 of 223,
  symbol_context 113 of 453, query_symbols 56 of 186 and obligations 58 of 281. This re-read with an explicit window
  end of 18:32:30Z counts 6,752 calls and the same abandonment counts; the totals differ by one or a few calls
  because the original window end was not recorded to the second.
- Re-run: `python3 -I docs/bench/readings/self-use/call-log-abandonment.py <call log> 2026-10-09T00:00:00Z
  2026-10-11T18:32:30Z`.

## Counts

| Tool | Origin (the call log's `origin` field) | Calls | Abandoned, client went away | Abandoned, other cause | Share abandoned |
|---|---|---|---|---|---|
| `localize` | `test-run-hook` | 223 | 200 | 3 | 91% |
| `obligations` | `edit-hook` | 259 | 58 | 1 | 23% |
| `obligations` | `prompt-hook` | 16 | 0 | 0 | 0% |
| `obligations` | none | 7 | 0 | 0 | 0% |
| `query_symbols` | `session-hook` | 11 | 7 | 0 | 64% |
| `query_symbols` | none | 175 | 49 | 1 | 29% |
| `symbol_context` | none | 453 | 113 | 0 | 25% |
| `verify_plan` | `verify-cli` and none | 5,237 | 0 | 2 | 0% |

| Tool | Median latency, all calls | Median latency, abandoned calls |
|---|---|---|
| `localize` | 2,003 ms | 2,003 ms |
| `obligations` | 379 ms | 2,037 ms |
| `query_symbols` | 1,381 ms | 2,025 ms |
| `symbol_context` | 307 ms | 1,146 ms |

## Correction to the first note

The first note called all four counts "hook-path". The `origin` field says otherwise for two of them: every
`symbol_context` call and 175 of 186 `query_symbols` calls carry no hook origin, so they came from agent or worker
MCP clients whose connection closed first (a worker killed by its orchestrator, a client-side timeout, or a session
that ended). Those abandonments are real lost work but not silent hook answers. The hook-path findings are
`localize` (test-run hook), `obligations` (edit hook) and the session hook's `query_symbols`.

## What this does not claim

- Nothing about an unloaded host: every call in the window ran while two or three worker waves shared the machine.
- No cause inside the daemon: "the client went away" means the caller closed first; whether the daemon was slow,
  queued or starved is not in this count (`queued_ms` is in the log and not summarized here).
- No effect on agent outcomes: the spend hold of 2026-09-29 means no outcome reading exists for this window.

Records that cite this reading: `docs/assessments/2026-10-11-reviews-and-incidents.md` section 6.3 (real-time
incident claims wait on IM-M5 and this fix) and `docs/assessments/2026-10-11-making-the-vision-happen.md`.
