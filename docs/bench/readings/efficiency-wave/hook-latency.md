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
