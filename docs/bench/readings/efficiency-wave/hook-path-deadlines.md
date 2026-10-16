# EW-75 / EW-76: hook-path calls against their callers' deadlines (G11)

**Verdict, in plain words.** EW-76 fails its bar. Dropping the test-run hook's `localize` and sizing the point calls to the hook's own deadline had a large effect in the loaded re-reading: the edit, prompt and stop hooks injected an answer 87 times, against 12 at base. The two graph-backed calls left on hook paths are still abandoned almost every time: the pre-edit blast radius 72% and the stop hook's plan verification 100%. While they hold the daemon, the post-edit obligations call is still cut in 6.5% of calls. The rest of this paragraph is EW-75's reading. Hook-path calls split into two classes. Point calls (`obligations`, `find_symbol`, `query_symbols`, `list_services`) answer in 10 to 250 ms and were abandoned only at a fixed client cap set well under the hook's own deadline, or while they waited behind a call-graph build. Graph-backed calls (`localize`, `blast_radius`, `verify_plan`, `test_impact`) need the service's call graph. On this repository's index (53,381 symbols), the default daemon does not keep that graph resident, so each of these calls rebuilds it in 7 to 20 s. That never fits a hook. An abandoned graph-backed call also trips the hooks' shared breaker, and its build keeps running. In the controlled base run, the breaker was open for most hook invocations, and point calls queued behind the orphaned builds. Of the 1,486 abandonments in the fleet's call log, 980 came from a hook. 385 of them were the test-run hook's `localize`, which a 2 s cap cut short in 88% of its calls.

EW-75 reading: hook-path abandonment is two classes, a fixed client cap under the hook's own deadline (point calls: edit-hook obligations 9.2% of 5,228 abandoned at 2 s in a 4 s hook, session-hook query_symbols 39.3% at 2 s in an 8 s hook) and graph-backed calls no hook budget holds on this index (test-run localize 385 of 437 abandoned at 2 s, pre-edit blast_radius 15 of 28 at 500 ms, stop-hook verify_plan 3 of 7 at 5 s; a direct cold localize took p50 15.0 s, p95 16.3 s in the controlled run); 980 of 1,486 abandonments carry a hook origin, the other 479 come from agents' own calls (symbol_context 325 of 1,827, query_symbols 79 of 489) and are no hook's.

## Source and method

- **Fleet call log.** This is a read-only copy of the daemon's call log (`workspace/logs/mcp-calls.jsonl`, 115,811 rows, sha256 prefix `c062aa96f5566c29`). From 2026-09-27T14:12Z to 2026-10-15T23:07Z, 18,687 of its rows carry an `outcome`. Only statistics were read: no prompt, path or argument content. An abandoned row's error names its cause, the client going away or the call's deadline passing (`internal/mcp/admission.go` settleCall). Its `latency_ms` is when the handler noticed the dead context, so it is a lower bound for the work. Calls are split by the `X-Fathom-Origin` header (`origin`).
- **Correction of 2026-10-11, applied.** Only `localize`'s abandonments are hook-path (its test-run-hook origin). In G11's window, `symbol_context` and most `query_symbols` abandonments carry no origin. They are agents' own MCP calls, so this reading counts them apart from the hook paths.
- **Load gauge (fleet).** The gauge is EW-6's (`hook-latency.md`): the p90 wall time of the edit-hook rows in `~/.fathom/hook-latency.jsonl{,.1}` (read-only copies, 42,498 rows) within 120 s either side of a call. Idle is the lowest third, at most 10 ms. Loaded is the highest third, over 13 ms. Only calls inside that log's window (2026-10-10T12:43Z to 2026-10-15T23:08Z) are gauged.
- **Controlled run.** `hook-path-harness.py` starts a scratch `serve-http` on a free loopback port. The daemon has a private `FATHOM_HOME`, its own call log, `--auto-refresh=false`, and default `[tenancy]`. It serves one scratch store, ingested once from a checkout of 99aa205a, and the same store serves both readings. The harness drives every hook that calls the daemon, 25 times each, with fixed payloads and one fresh process per invocation:
  - test-run: `session-hook --event failed-test-run` on a failing `go test`;
  - edit-post: PostToolUse Edit;
  - edit-pre: `--pre` on a guarded file;
  - prompt: a prompt naming a file and a symbol;
  - session: SessionStart;
  - stop: a working-tree plan claim.

  The harness then probes each hook-path tool directly, with the hook's own arguments and a 60 s client (origin `probe-hook`), so the full cost of a call the hook abandons is still read. Cold means 8 daemon restarts, each followed by the test-run hook and then a direct `localize`. The unloaded condition is the machine as it was: other wave work ran on it, at a load average of 4.4 to 10.1. The loaded condition runs the same loop beside a scratch ingest of the checkout into a second scratch store, restarted whenever it finished. Both halves of each run held the wave's ingest lock. The shared daemon and the real `~/.fathom` were never touched.
- **SHAs.** Base: `fathom` v0.11.0 @ 99aa205a, the run 2026-10-16T07:32:30Z to 07:41:30Z. Fix: dc7c4c20 (EW-76, on 071ef016), the run 07:49:45Z to 08:01:54Z, over the same scratch store, with the same base binary running the loaded half's ingest.
- **Reproduce:**
  - `python3 -I docs/bench/readings/efficiency-wave/hook-path-deadlines.py <mcp-calls.jsonl> <hook-latency.jsonl.1> <hook-latency.jsonl>`
  - `python3 -I docs/bench/readings/efficiency-wave/hook-path-harness.py --bin <fathom> --load-bin <fathom at 99aa205a> --subject <checkout> --home <scratch> --store <scratch store> --ingest-store <second scratch store> --n 25 --cold 8 --probes 8`, held under the ingest lock.

## The fleet's call log at 99aa205a

| origin | calls | abandoned | share of its calls | share of all abandonments |
|---|---|---|---|---|
| edit-hook | 5,401 | 499 | 9.2% | 33.6% |
| none (agents' own calls) | 6,366 | 479 | 7.5% | 32.2% |
| test-run-hook | 437 | 385 | 88.1% | 25.9% |
| session-hook | 212 | 73 | 34.4% | 4.9% |
| doctor | 435 | 22 | 5.1% | 1.5% |
| prime | 105 | 20 | 19.0% | 1.3% |
| verify-cli | 5,262 | 5 | 0.1% | 0.3% |
| stop-hook | 20 | 3 | 15.0% | 0.2% |
| prompt-hook | 50 | 0 | 0.0% | 0.0% |

Per hook-path tool and origin. "Caller's deadline" is the cap the calling code sets. The hook's own deadline is the descriptor's TimeoutSec less max(1 s, 20%) (I53).

| origin | tool | calls | abandoned (share) | cause: client gone / deadline | completed p50 / p95 ms | all rows p50 / p95 ms | abandoned at p50 / p95 ms | caller's deadline |
|---|---|---|---|---|---|---|---|---|
| test-run-hook | localize | 437 | 385 (88.1%) | 380 / 5 | 1,570 / 1,933 | 2,003 / 2,092 | 2,005 / 2,113 | 2 s client in a 4 s hook (TimeoutSec 5) |
| test-run-hook | test_impact | 0 | - | - | - | - | - | 2 s client in the same 4 s |
| edit-hook | obligations | 5,228 | 483 (9.2%) | 464 / 19 | 113 / 802 | 123 / 2,013 | 2,018 / 2,865 | post 2 s; pre what is left of a shared 2 s (at least 200 ms); 4 s hook |
| edit-hook | blast_radius | 28 | 15 (53.6%) | 15 / 0 | 98 / 281 | 496 / 597 | 500 / 789 | 500 ms client (pre-edit trailer) |
| edit-hook | verify_plan | 8 | 1 (12.5%) | 1 / 0 | 355 / 1,123 | 568 / 2,139 | 2,677 / 2,677 | 5 s client in a 4 s hook |
| edit-hook | list_services | 137 | 0 (0.0%) | 0 / 0 | 28 / 567 | 28 / 567 | - | 2 s client |
| session-hook | query_symbols | 178 | 70 (39.3%) | 70 / 0 | 631 / 1,486 | 1,180 / 2,157 | 2,040 / 2,311 | 2 s (liveExampleLine) in an 8 s hook |
| session-hook | list_services | 34 | 3 (8.8%) | 3 / 0 | 46 / 364 | 47 / 13,814 | 35,488 / 37,358 | 2 s client |
| prime | query_symbols | 94 | 20 (21.3%) | 20 / 0 | 636 / 1,518 | 722 / 2,067 | 2,024 / 2,117 | 2 s (the same liveExampleLine) |
| stop-hook | verify_plan | 7 | 3 (42.9%) | 2 / 1 | 754 / 3,529 | 3,009 / 5,050 | 4,888 / 5,097 | 5 s client in an 8 s hook |
| prompt-hook | obligations | 41 | 0 (0.0%) | 0 / 0 | 191 / 5,141 | 191 / 5,141 | - | what is left of a shared 8 s |
| prompt-hook | find_symbol | 8 | 0 (0.0%) | 0 / 0 | 152 / 1,132 | 152 / 1,132 | - | the same 8 s |

The same tools without an origin, called by agents and not by a hook: `symbol_context` 325 of 1,827 abandoned (17.8%), `query_symbols` 79 of 489 (16.2%), `localize` 0 of 19, `obligations` 1 of 428. In G11's window (2026-10-09 to 10-11), 221 of the 241 test-run `localize` calls were abandoned. Neither `symbol_context`'s 130 abandonments nor 53 of the 60 `query_symbols` abandonments carry a hook origin.

Load (fleet gauge, 331 gauged hook-path calls):

| origin | tool | idle: calls, abandoned, all-rows p50 ms | loaded: calls, abandoned, all-rows p50 ms |
|---|---|---|---|
| edit-hook | obligations | 103, 11 (11%), 198 | 66, 15 (23%), 560 |
| test-run-hook | localize | 33, 28 (85%), 2,003 | 32, 32 (100%), 2,010 |
| session-hook | query_symbols | 2, 0 (0%), 1,216 | 2, 2 (100%), 2,136 |
| prompt-hook | obligations | 8, 0 (0%), 368 | 2, 0 (0%), 253 |
| prompt-hook | find_symbol | 4, 0 (0%), 62 | 1, 0 (0%), 200 |

Hook wall time stayed inside its budget almost everywhere. Of 28,329 edit-hook invocations, 1 ran past its 4 s budget. Of 14,063 session-hook invocations, 10 ran past theirs. The hooks themselves meet their deadlines; the calls inside them are cut by their own caps.

## The controlled base run (99aa205a)

Per hook-path tool, from the scratch daemon's call log, with each call attributed to the hook invocation it started in:

| tool (hook) | unloaded: n, abandoned | loaded: n, abandoned | at |
|---|---|---|---|
| localize (test-run) | 8, 8 (100%); cold 8, 8 | 8, 8 (100%) | 2,000 ms |
| obligations (edit-post, edit-pre, prompt, session) | 17, 5 (29.4%) | 16, 5 (31.2%) | edit-post at 2,000 ms |
| blast_radius (edit-pre) | 3, 3 (100%) | 4, 3 (75.0%) | 500 ms |
| verify_plan (stop) | 6, 6 (100%) | 5, 5 (100%) | 5,000 ms |
| query_symbols (session) | 3, 0 | 5, 0 | p50 174 to 186 ms |
| find_symbol (prompt, session) | 3, 0 | 4, 0 | p50 9 to 10 ms |

The same tools called directly with a 60 s client (unloaded / loaded):

- `obligations`: p50 14 / 14 ms.
- `query_symbols`: p50 167 / 196 ms.
- `find_symbol`: p50 9 / 10 ms.
- `localize`: p50 10.7 / 12.0 s, p95 16.5 / 17.0 s. Cold, as the first call after a daemon start, p50 15.0 s and p95 16.3 s.
- `blast_radius`: answered 1 of 8 calls unloaded (14.5 s) and none loaded. The 15 failures returned in 2 to 3 ms with "no symbols for service fathom" (O1013).

Most hook invocations made no call at all. Over the two conditions, the breaker skipped 77 edit-hook, 76 session-hook, 43 prompt-hook and 39 stop-hook calls. Only 6 of 50 edit-post and 7 of 50 edit-pre invocations injected anything, and the prompt hook injected 3 of 50.

**Why.** A CPU profile of a warm `localize` (budget 1, a single find_symbol hop, 12.1 s) spent 9.8 of its 10.8 s of samples in `callgraph.Build` under `memo.GraphFor`. The scratch daemon logged `tenancy: normal → shed: footprint+children 504.0 MiB ≥ shed mark 384 MiB` 16 s after it started, and at shed the memo keeps only its trim, so the call graph is rebuilt on every graph-backed call (O1014). Three things follow:

- A hook caller that leaves before half a build's expected cost cancels the flight (`memo.Group.leave`), so a hook's attempt never warms the graph for the next caller.
- Every abandonment is a timeout, which `hookguard` counts as a breaker failure. Two in 10 s open the shared breaker for every hook.
- `localize`'s first candidate for the harness's failing test was the failing test itself: `named TestDaemonTestRunIndexQualifiesService`, score 8 against 5 for every other hop. The line printed that candidate only when no changed symbol reached the failing tests. So the hook's one use of `localize` restated what the run's own output already names.

## EW-76: the fixes, per call class

- **Graph-backed, where the hook gains nothing: dropped.** The test-run hook asks the daemon no `localize` (`daemonTestRunIndex.Localize`). The agent's own `localize` is unchanged.
- **Point calls under a fixed cap: a deadline sized to the hook.** The post-edit `obligations` call gets what is left of the hook's own deadline, less 500 ms for the rendering after it (`hookCallTimeout`, floor 200 ms), instead of a fixed 2 s. The test-run hook's remaining daemon calls (identity, `test_impact`) carry the hook's deadline in their context and no client cap of their own (8 s outside a hook).
- **A hook that cannot answer in time names it.** It records GL-24's `daemon-error` in the answered beat, beside the error beat that names the cause, never silence. This applies to post-edit and pre-edit obligations, and to a test-run row that missed its deadline and emitted nothing. A deadline miss is a daemon call that failed, and the bash-write path already counted it as `daemon-error`, so no new vocabulary value was added.
- **Not fixed by this item.**
  - The pre-edit `blast_radius` trailer is graph-backed, but the hook gains from it whenever the graph is resident: 13 of 28 fleet calls answered in p50 98 ms. Its 500 ms additive budget is the design's (guarded-edits §2.2), and it is left as is.
  - The stop hook's `verify_plan` 5 s cap and the session hook's 2 s `query_symbols` ceiling sit in `stophook.go` and `sessionhook.go`, outside this item's files.
  - A graph-backed call that cannot finish before a hook-origin deadline should be refused or kept from starting a build at the daemon, which is O1014's fix. That is RT territory and waits until RT-M1 reads.

## EW-76: the loaded re-reading (dc7c4c20)

Per hook-path tool (unloaded / loaded):

| tool (hook) | base, loaded: n, abandoned | fix, unloaded: n, abandoned | fix, loaded: n, abandoned | at |
|---|---|---|---|---|
| localize (test-run) | 8, 8 (100%) | 0 calls | 0 calls | not asked |
| obligations (all hooks) | 16, 5 (31.2%) | 64, 6 (9.4%) | 93, 6 (6.5%) | edit-post at its sized 3,456 to 3,500 ms (6 of 13 loaded); edit-pre 0 of 33, prompt 0 of 23, session 0 of 24 |
| blast_radius (edit-pre) | 4, 3 (75.0%) | 15, 15 (100%) | 25, 18 (72.0%) | 18 of 18 edit-pre calls at 501 ms; the other 7 rows failed in 1 to 2 ms (O1013) |
| verify_plan (stop) | 5, 5 (100%) | 15, 15 (100%) | 25, 25 (100%) | 5,000 ms |
| query_symbols (session) | 5, 0 | 15, 0 | 25, 0 | p50 194 ms |
| find_symbol (prompt, session) | 4, 0 | 15, 0 | 25, 0 | p50 10 ms |

What the hooks delivered (loaded, base / fix):

| hook | injected | wall p50 / p95 ms, base | wall p50 / p95 ms, fix |
|---|---|---|---|
| test-run | 25 / 25 of 25 | 70 / 2,083 | 77 / 110 |
| edit-post | 3 / 19 | 26 / 2,023 | 1,972 / 3,525 |
| edit-pre | 4 / 25 | 26 / 535 | 539 / 666 |
| prompt | 0 / 18 | 21 / 56 | 59 / 67 |
| stop | 5 / 25 | 505 / 5,641 | 5,548 / 5,672 |

In the loaded half, an open breaker skipped 116 hook calls at base and none in the fix. Every edit hook that could not answer named `daemon-error`: 22 unloaded and 6 loaded. The longer walls on the fix side are hooks that now reach the daemon instead of being skipped by an open breaker. The stop hook waits its full 5 s on a `verify_plan` that the daemon cannot finish, and that cap sits outside this item's files.

The test-run hook's cold `localize`: the hook asks none, and its cold wall time was p50 47 ms, p95 87 ms over 8 daemon restarts. `localize` itself, called directly with a 60 s client, took cold p50 6.9 s and p95 8.9 s with no hook call competing. Unloaded it took p50 12.4 s, p95 18.3 s; loaded, p50 17.0 s, p95 19.9 s. That is far over the 3 s IM-M5 asks.

EW-76 verdict: FAIL. Abandonment on the loaded re-reading is blast_radius 72.0% (18 of 25), verify_plan 100% (25 of 25) and obligations 6.5% (6 of 93), all over the 5% bar. localize (not asked on the hook path), query_symbols 0 of 25 and find_symbol 0 of 25 are under it. The test-run hook's cold localize p95 is 87 ms of hook wall time because the hook asks none; localize's own cold p95 is 8.9 s, over IM-M5's 3 s.

**What a PASS needs.**

1. A graph-backed call that cannot finish inside a hook-origin deadline is refused at the daemon instead of starting a build, or the graph stays resident under the default budget (O1014, after RT-M1). That covers `blast_radius`, `verify_plan` and the `obligations` calls queued behind their builds.
2. The stop hook's `verify_plan` cap and the session hook's `query_symbols` ceiling get the same sizing as the post-edit call (`stophook.go`, `sessionhook.go`).
3. A re-reading with this harness.
