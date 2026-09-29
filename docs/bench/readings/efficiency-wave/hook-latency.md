# EW-6: what the session hook and the stop hook cost, from the hook-latency log

EW-6 reading: work, not load, for the session hook (p50 976 ms at an idle machine, 811 ms loaded, no dependence on load); a work floor plus load for the stop hook (p50 1,586 ms idle, 3,244 ms loaded); the log holds no CPU time, so the CPU half of the row is unread until EW-29 records it.

## Source and method

- Source: `~/.fathom/hook-latency.jsonl` and `.1`, read-only copies, statistics only (no prompt or path content read). 48,188 rows, 2026-09-28T03:05Z to 2026-09-29T04:42Z (25.6 h): 47,371 edit-hook, 516 session-hook, 136 stop-hook, and 165 others.
- A row holds wall time `ms`, the deadline `budget_ms` and the invocation id. It holds **no CPU time**, so "CPU and wall time" cannot be read from it; the row's CPU half is open (EW-29).
- The log has no machine-load column. Its own load gauge: the edit hook's work is a stat and a store read (p50 3 ms over the whole log), so when it slows the machine is slow. Each session-hook and stop-hook invocation is scored by the p90 wall time of the edit hook in the 120 s either side; idle is the lowest third of that gauge (edit-hook p90 at most 8 ms), loaded the highest third (over 15 ms for the session hook, over 18 ms for the stop hook). The gauge measures the machine's response, not its cause, and is one proxy: the reading is only as good as it.
- Reproduce: `python3 docs/bench/readings/efficiency-wave/hook-latency.py ~/.fathom/hook-latency.jsonl.1 ~/.fathom/hook-latency.jsonl` (fixed seed, 4,000 bootstrap draws).

## Result

| channel | all: n, p50 / p90 / p99 (ms) | idle third: n, p50 / p90 | loaded third: n, p50 / p90 | loaded minus idle p50 |
|---|---|---|---|---|
| session-hook | 516, 940 / 2,666 / 4,394 | 205, 976 / 2,303 | 166, 811 / 3,210 | -165 ms, 95% CI [-421, 340], Mann-Whitney p 0.65 |
| stop-hook | 136, 2,263 / 5,211 / 8,897 | 46, 1,586 / 3,170 | 44, 3,244 / 6,401 | +1,658 ms, 95% CI [1,195, 2,488], p under 0.001 |

- The row's figures (p50 895 ms and 2,261 ms under load on 2026-09-29) sit inside these windows: 940 and 2,263 over the whole log, 714 and 3,364 on the 2026-09-29 rows alone (110 and 20 invocations).
- **Session hook: work.** Machine load moves its p50 by no measurable amount (the interval spans zero; rank correlation of wall time with the gauge 0.03). Its cost is a mix of paths, independent of load: 15% under 50 ms, 13% at 50 to 300 ms, 24% at 300 to 1,000 ms, 45% at 1 to 4 s, 3% over 4 s; 9 of 516 ran past the deadline. The 37 rows written with an 8,000 ms deadline (a later configuration) have p50 2,108 ms against 829 ms for the 479 at 4,000 ms.
- **Stop hook: a work floor plus load.** At an idle machine its p50 is still 1,586 ms and none of 136 ran under 300 ms; load adds about 1.7 s at the median (rank correlation 0.47). 19% ran over 4 s and 2 ran past the 8,000 ms deadline.
- Concurrency does not explain either: the number of other session or stop hooks in flight has rank correlation -0.02 (session) and -0.11 (stop) with wall time.

## What follows

A fix row follows, because the session hook and the stop hook are work at an idle machine. The log cannot say whether that work is CPU (parsing, git, the store) or waiting (the daemon, a child process), so the first row records CPU beside wall time: EW-29. A row that cuts the work waits on that reading rather than guessing at it.

---

EW-29 re-read: the part of both hooks that a scratch harness reproduces is CPU work, not waiting. The session hook costs about 0.56 s of CPU per invocation (p50, 36 runs) against 0.55 s of wall time at the machine's own load and 0.98 s with 16 busy processes added; the stop hook costs about 0.45 s of CPU against 0.41 s and 0.65 s of wall time. Added load stretches the wall time (x1.8 and x1.6) far more than the CPU (x1.4 and x1.2): the stretch is waiting for a core, and the floor beneath it is work. The harness reproduces about 60% of the session hook's p50 in the EW-6 window and about a quarter of the stop hook's idle-third p50; the rest of the stop hook's 1.6 s is unread until the fleet's own rows carry CPU (the last section).

## What the log now holds

- Every hook invocation's row in `hook-latency.jsonl` gains `cpu_ms`: the invocation's CPU time, user plus system, of the hook process and the children it reaped (the git processes it forked), read with `getrusage` at the guard's begin and again at its end (`beginHookGuard`, `recordHookLatency`). It can exceed `ms`: the Go runtime is multi-threaded and the hook forks children that run beside it.
- The field is absent, never 0, where the CPU cannot be read at both ends (a platform without `getrusage`, a failed call) and in every row an older binary wrote. A read that did not advance is a measured 0 and is kept.
- The cost is two `getrusage` pairs per invocation (0.4 to 0.5 us a pair, measured at a load average of 30) and no file read; the guard still makes one append.
- `fathom doctor`'s `hook_budget` and `fathom resources` name `p50 wall … cpu …` beside n for a channel that has rows with `cpu_ms`, comparing wall time on those same rows; a channel without any reads as before.
- `hook-latency.py` reads `cpu_ms` where present: CPU and wall p50 / p90 and the CPU share of wall time, overall and in the idle and loaded thirds of its gauge; on today's real log it prints "none of N rows carries cpu_ms", since the deployed binary predates this row.

## Method

`docs/bench/readings/efficiency-wave/hook-cpu-harness.py`, in the manner of O140. A private `FATHOM_HOME` holds a copy of the real beat log (66 MB; the stop hook's retro reads all of it) and session ledger, a 2 MB session transcript copy that grows by 20 real lines before each Stop (so each Stop folds something), a scratch ingest of this repository and a scratch `serve-http` on a free loopback port. The shared daemon and the real `~/.fathom` are never touched. Each invocation is a fresh process (`session-hook`, `stop-hook`) fed a real-shaped payload; one warm-up of each is discarded. Two conditions, 12 invocations of each hook per condition, three runs: **ambient** (the machine as it was) and **loaded** (16 busy processes added on a 10-CPU machine). The cross-check is the operating system's own accounting of the process tree (`wait4`'s rusage): it exceeds the row's `cpu_ms` by a median 19 ms (ambient) to 26 ms (loaded), which is the runtime's start-up and exit outside the guard's window, so the row's figure is the invocation and not an artefact.

| hook | condition | n | wall p50 / p90 (ms) | CPU p50 / p90 (ms) | CPU / wall, p50 | machine load average |
|---|---|---|---|---|---|---|
| session-hook | ambient | 36 | 554 / 845 | 561 / 726 | 1.01 | 20 to 32 |
| session-hook | loaded | 36 | 979 / 1,566 | 774 / 808 | 0.78 | 19 to 33 |
| stop-hook | ambient | 36 | 411 / 549 | 447 / 521 | 1.13 | 19 to 30 |
| stop-hook | loaded | 36 | 652 / 747 | 549 / 584 | 0.84 | 24 to 35 |

Per-run p50s (wall / CPU, ms) for the three runs: session-hook ambient 755 / 678, 531 / 542, 552 / 558; loaded 1,050 / 796, 952 / 778, 976 / 762. Stop-hook ambient 494 / 478, 339 / 410, 398 / 447; loaded 664 / 566, 550 / 535, 691 / 550. All 144 invocations exited 0 and wrote exactly one row carrying `cpu_ms`.

## EW-6's two work findings

- **The session hook, near 0.9 s at any load: CPU.** With no added load the process tree burns as much CPU as the wall clock passes (CPU / wall 1.01), about 0.56 s. Added load moves the wall time by 425 ms and the CPU by 213 ms: half of the stretch is waiting for a core, half is the same work running slower on a contended machine (CPU time itself grows under contention). So the floor does not come from waiting on the daemon or a child: the scratch daemon answers from an idle single-service index, and the hook still spends 0.56 s of CPU. Whether that CPU is the hook process itself or its git children is not split by this row; the row that cuts it starts from a profile.
- **The stop hook, 1.6 s at an idle machine: CPU for the 0.45 s reproduced; the other 1.1 s is unread.** The harness stop hook is CPU-bound (CPU / wall 1.13 ambient); by O140's stage timings the retro's whole read of the beat log (O141; 66 MB here) is its largest piece, though this row does not split the hook's CPU by stage. The fleet's idle-third p50 in the EW-6 window (1,586 ms) is 3.5 to 3.9 times the harness figure (CPU 447 ms, wall 411 ms). What the harness lacks against the fleet is a busy shared daemon, a long real transcript and a session with hundreds of beats, so the difference is either more CPU on larger inputs or waiting; the log can settle it once the fleet's rows carry `cpu_ms`.

Reference, not a result of this row: the same script on the real log as it stands (23.2 h to 2026-09-29T22:23Z, before any row carries CPU) reads session-hook p50 666 ms (idle third 650, loaded third 1,181) and stop-hook p50 1,510 ms (idle 941, loaded 3,048). The harness's ambient session hook (554 ms) sits close to that window's idle third; EW-6's first window read the session hook as load-independent and this one does not, so "at any load" is one window's finding.

## Limits

- The machine was never idle: other workers kept its load average between 19 and 35 on 10 CPUs for the whole reading, so "ambient" is a loaded machine, and a quiet one would show lower wall times with CPU/wall near 1. CPU time is the load-insensitive half, but it moved by about 20% between runs (session-hook ambient 542 to 678 ms), so read the medians as a band of plus or minus 100 ms.
- One repository, one index, one transcript size, an idle scratch daemon. The harness bounds the work from below; it cannot show waiting on a busy shared daemon.
- A child still running when the hook returns is not counted (children are counted when reaped).

## What follows

- The fleet re-read on the real `~/.fathom/hook-latency.jsonl` follows the owner's `task deploy` of a binary with this row: the deployed binary writes no `cpu_ms` until then, and no worker deploys or copies a binary. Re-read it then with `python3 docs/bench/readings/efficiency-wave/hook-latency.py ~/.fathom/hook-latency.jsonl.1 ~/.fathom/hook-latency.jsonl`, whose CPU block reports the fleet's CPU share of wall time by idle and loaded third.
- A fix row for the session hook's 0.56 s of CPU starts from a CPU profile of `fathom session-hook`; a fix row for the stop hook's retro is O141's.
