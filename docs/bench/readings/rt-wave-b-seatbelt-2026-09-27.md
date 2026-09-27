# Responsible tenant, wave B: the seatbelt, live (2026-09-27)

Binary: the wave-B integrated head `f837a0cc` (rtB-integ: governor, lifecycle, ingest
state, admission and visibility lanes merged). Incident: INC-23. Design:
`docs/design/responsible-tenant.md` §2, §2.1, §2.3, §3.4, §8.4. Plan:
`docs/plans/responsible-tenant.md` §B (RT-29..RT-65, RT-150..RT-177). Wave-A reading:
`rt-wave-a-incident-replay-2026-09-26.md`.

## 1. Verdict first

**The seatbelt holds. Under the incident's traffic and at the default budget, the daemon's
peak footprint was 1.00 GiB. It stayed 3 GiB under the 4 GiB refuse mark, and the ladder
never left `normal`.** On the wave-A head, the same `symbol_context` ×20 burst reached
5.2–5.9 GiB, and on the baseline 5.6 GiB. Under `enforce` the peak was 0.45 GiB, because
the `tree` class ran 2 at a time. Nothing reached the 6 GiB harness kill line in any run.

**At a deliberately tiny budget, every rung fired live and did what the design says:**

- **normal → shed → refuse:** caches were dropped and memory was returned to the OS.
- **Refusals:** tree and fleet calls got 503 + `Retry-After` + `-32050`, with a named
  reason. In-flight tree calls were cancelled as `abandoned-by-governor`. Index and point
  calls kept answering.
- **Step-down and re-entry.**
- **refuse → exit:** an in-flight ingest was halted, its service quarantined, and the
  daemon exited 70 over budget. It wrote diagnostics, an `exit{reason: over_budget}` row
  and a `quarantine` row to the ledger.
- **The crash-loop breaker, across relaunches with `--lifecycle-id`:**
  1. Three counted exits in 30 minutes put the next start in safe mode, with the reason
     named.
  2. In safe mode, tree and fleet calls got a 503 and index and point calls were served.
  3. An over-budget exit while in safe mode set the breaker to STOP.
  4. Every later start exits 0 in about 120 ms.

**Keep wave B. It turns "the daemon grew to 30 GB" into a bounded, named and recorded
event.** The live run also found gaps that the unit tests do not reach; §5 lists them.

- **The biggest gap: the typed pass (`go/packages` inside ingest) is not governed at
  refuse.** An ingest admitted before the ladder climbed kept type-checking for 10
  minutes while the daemon sat at `refuse`. Only the exit mark stopped it, and it
  overshot that mark by 2.4–4.6×. With `go list` children counted, it cost up to 1.3 GiB
  against a 128 MiB budget. The seatbelt held, but the refuse rung did not bound this
  producer.

## 2. Setup

- **Scratch daemon:** one unmanaged `serve-http` at a time on `127.0.0.1:7691` (never
  7575). It ran with a scratch `HOME`, `--auto-refresh=false`, and a sqlite `.backup`
  copy of `workspace/db/fleet.sqlite` (256 MiB, 18 services).
- **Harness:** `footprint -p` every 0.3 s and `ps rss` every 0.25 s, with a SIGKILL
  above 6 GiB. That kill never fired. `/metrics` was polled every 0.25 s for
  `fathom_tenancy_level` and footprint.
- **Workload:** rtA's `workload.json`. That is 20 edit-hook `obligations` calls on the
  three largest services, per round, and 20 `symbol_context` calls across app-kit,
  platform-engineering and fathom.
- **Ladder runs:** `[tenancy] admission = "enforce"`, `[tenancy.breaker] warmup = "5s"`
  (shortened so warm-up does not mask the ladder), and `[tenancy.unmanaged] budget =
  "128MiB"`. The marks were shed 96 MiB, refuse 128 MiB, exit 256 MiB. The breaker runs
  used `FATHOM_TENANCY_BUDGET=64MiB` (exit mark 128 MiB).
- **Default-budget runs:** `FATHOM_TENANCY_BUDGET=4GiB`, which is the managed auto budget
  on this 32 GB machine (clamp(32/8, 1, 4)). The marks were shed 3, refuse 4 and exit
  8 GiB. Admission was left at its default, `observe`, and was also run at `enforce`.
  Warm-up was the default 120 s. An unmanaged daemon's own default is 512 MiB, so the
  4 GiB budget had to be named.

## 3. The ladder, live (128 MiB budget, enforce)

### 3.1 Transitions, run 1 (`--lifecycle-id rtb`)

| t | Level | Footprint (own + children) | What the daemon did |
|---|---|---|---|
| 0.5 s | normal | — | Ready. The startup line named the budget and marks, the admission slots (point 2, index 2, tree 1, fleet 1, ingest 1, egress 2) and the GC hint (256 MiB, the exit mark). |
| 7.7 s | normal | — | 20 edit-hook `obligations` calls: 1 answered, 19 got **503 `-32050` "tree-class slots full (1/1): obligations"**, `X-Fathom-Busy-Scope: class`, `Retry-After: 5`. |
| 10.4 s | **shed** | 230 MiB | `normal → shed`. `scavenge (shed entry)` released 95.5 MiB: phys_footprint went from 230.1 to 134.6 MiB. |
| 12.5 s | **refuse** | 237 MiB | `shed → refuse`. **Refuse cancelled 1 in-flight tree call** (`symbol_context`, youngest first). A heap capture and `inflight.json` were written under `diag/…-refuse`. `scavenge (refuse entry)` released 139.5 MiB: phys_footprint went from 234.9 to 95.5 MiB. |
| 12–22 s | refuse | ≤ 130 MiB | The 20 `symbol_context` calls queued before refuse: 1 `abandoned-by-governor`, 19 **`over budget: tree producer halted at refuse`** (the RT-153 producer governor stopped each within one unit). |
| 28–86 s | refuse | 70–130 MiB | Every new `symbol_context` got **HTTP 503, `-32050` "fathom busy: fathom is over its memory budget (refuse): tree-class work is refused — fathom resources", `X-Fathom-Busy-Scope: daemon`, `Retry-After: 30`**. The index trickle (`find_symbol` every 0.3 s) went on answering 200. |
| 86.5 s | **shed** | 69 MiB | Step-down after 60 s below 0.6 × the refuse mark. |
| 90.4 s | **refuse** | 258 MiB | The next heavy burst was admitted at shed. One `symbol_context` pushed the footprint to 258 MiB, and the ladder re-entered refuse with the same refusals. |
| 127 s | — | — | SIGTERM. Ledger row `exit{reason: signal, code 143}`: not counted. |

In total, the index trickle made 128 `find_symbol` calls and got 128 × 200. Refuse
returned 3,445 `symbol_context` calls with a 503. Another 40 were stopped in flight with a
named `-32603`: 20 queued at 7.7 s and 20 admitted at shed at 86.7 s. The 15 still
connected at SIGTERM were reset. None were answered empty and none failed silently. The peak was **0.25 GiB phys_footprint / 0.28 GiB RSS**. That is below
the 256 MiB exit mark for one sample and never over it for two, so the exit rung was not
needed.

### 3.2 Exit, halt and quarantine, run 2 (the same ledger)

An explicit `ingest` tool call onboarded a clone of this repository as `rtb-probe`. It
was admitted at `normal`, while warm-up was over:

- 10 s: `normal → shed` (190 MiB), then `shed → refuse` at 12 s (194 MiB). The ingest
  **was not stopped by refuse** (§5 F1). It spent 10 minutes in the typed pass while the
  daemon sat at refuse, with a scavenge every 30 s. `fathom resources` later showed the
  24 h ring maximum as **1.3 GiB own + children at 10:47, at refuse**. My own-pid samplers
  saw at most 0.60 GiB, so the rest was `go list` children.
- 607.7 s: `refuse → exit` (own 525.6 MiB ≥ exit mark 256 MiB). The daemon **halted the
  in-flight ingest-class work** (`ingest(rtb-probe)` ×2: the agent's call and its
  pipeline), wrote a **`quarantine` row** (`rtb-probe exceeded budget during ingest`),
  logged `the next sample decides`, found itself still over at 630.8 MiB, and **exited
  70**. It wrote this ledger row:

```json
{"row":"exit","code":70,"reason":"over_budget","detail":"own footprint 630.8 MiB ≥ exit mark 256 MiB after halting the in-flight ingest","footprint":661488936,"children":0,"budget":134217728,"exit_mark":268435456,"top_in_flight":["ingest(ingest, rtb-probe, 1m15s)","ingest(ingest, rtb-probe, 1m15s)"],"diag":"…/rtb/diag/20260927T105600Z-exit"}
```

- The heap at exit (`diag/…-exit/heap.pb.gz`, 559 MB in use) is 90% under
  `errgroup.Go` → `go/packages.(*loader).loadPackage` → `go/types.(*Checker)`. That is
  the in-process typed pass: `recordTypeAndValue` 148 MB, `go/parser` 132 MB and wazero
  41 MB.
- Index calls made during the ingest answered in 1.7–7.9 s until one call blocked for
  **531 s** (§5 F2). Once the level reached exit, index calls got a 503 too (`exit:
  index-class work is refused`), as §2.1 specifies.
- On the next start, `/statusz`, `resources` and doctor all carried `quarantined:
  rtb-probe exceeded budget during ingest — retry with the daemon's ingest tool …`.

### 3.3 The breaker across relaunches (`--lifecycle-id rtb-breaker`, 64 MiB budget)

| Start | What happened | Ledger rows |
|---|---|---|
| 1 | A single `symbol_context` jumped from 64 MiB to 168 MiB between two samples: normal → shed → **exit 70** | `exit{over_budget, "own footprint 168.4 MiB ≥ exit mark 128 MiB", top_in_flight: symbol_context(tree, fathom, 4s)}` |
| 2, 3 | The same probe reached refuse only (timing): `list_services` got 503 fleet; `symbol_context` was `abandoned-by-governor`; `ingest` got **503 `-32050` "ingest-class work is refused"**; a post-commit-hook ingest was refused the same way. SIGTERM. | `exit{signal}`: not counted |
| 4, 5 | SIGKILL (standing in for jetsam or force-quit) | no exit row |
| 6 | **Safe mode.** Startup logged `predecessor exited unclean — pid 58448 … has no exit row — SIGKILL, jetsam, force-quit or a runtime fatal (crash.log)` and `fathom safe mode since 04:15: 3 counted exits in 30m0s (over_budget ×1, unclean ×2) — tree and fleet refused; auto-refresh, feeders and the decision port off; point and index served`. `capabilities` and `find_symbol` answered 200. `symbol_context` and `list_services` got **503 `-32050`, `Retry-After: 60`, `data.reason` = the safe-mode line**. The explicit `ingest` (not refused in safe mode) reached 595 MiB in 6 s and was halted, quarantined and **exited 70**. | synthesized `exit{unclean}`, `breaker{to: safe}`, `start{mode: safe}`, `quarantine`, `exit{over_budget}` |
| 7 | **Breaker STOP**: `pid 58498 exited over budget in safe mode — exiting 0` (0.12 s) | `breaker{to: stop}` |
| 8 | Stop again, exit 0: the ledger holds the STOP without `disabled.json`. An unmanaged daemon writes no switch file, by design (lifecycle.go only writes it for `--managed`). | `breaker{to: stop}` |

This is RT-M3's shape, a seatbelt without brakes, on an unmanaged daemon:

- **Footprint:** the maximum was **0.61 GiB against a 128 MiB exit mark**. That is within
  the "exit mark + 1 GiB" bound.
- **Diagnostics:** a capture was written at every refuse entry and at every exit.
- **Breaker:** safe mode came at start 6 and STOP at start 7. The row's counts differ
  (by start 3 / by start 5) because two of my starts ended with a SIGTERM, which is
  correctly not counted.
- **Still open:** the managed-daemon version of this (launchd relaunch, `disabled.json`)
  stays the operator's RT-M3 row.

## 4. The incident replay at the default budget (4 GiB)

The traffic was `list_services` ×32 ×4 rounds (session hook), then `obligations` ×20 ×5
rounds (edit hook), then `symbol_context` ×20 (agent), all on one daemon.

| Run | list_services | obligations | symbol_context ×20 | Level | Peak footprint / RSS |
|---|---|---|---|---|---|
| observe, burst inside warm-up | 128/128 ok, p50 < 0.5 s | 100/100 ok | 2 ok; **18 × 503 warm-up** ("tree-class holds 1 slot for 2m0s after the start") | normal | **0.23 / 0.26 GiB** |
| observe, burst after warm-up | 128/128 ok, p50 368 ms | 100/100 ok, p50 125 ms | 14 ok (p50 38 s, max 59 s); **6 abandoned at the 60 s tree deadline** (a named `-32603` "abandoned: the call's deadline passed", never empty) | normal | **1.00 / 1.04 GiB** |
| enforce, burst after warm-up | 128/128 ok, p50 1.0 s | 100/100 ok, p50 376 ms | 13 ok (11 of them queued, p50 18 s, max 36 s); **7 × 503 class-scoped "tree-class slots full (2/2)"**, `Retry-After: 5` | normal | **0.45 / 0.48 GiB** |

Against earlier binaries, for the same `symbol_context` ×20:

| Binary | Peak footprint | Peak RSS |
|---|---|---|
| wave-A head | 5.21 / 5.88 GiB | 5.78 / 6.06 GiB (the second run was killed) |
| wave-A symbol_context sequence | 2.96 GiB | 3.13 GiB |
| wave B, observe | **1.00 GiB** | 1.04 GiB |
| wave B, enforce | **0.45 GiB** | 0.48 GiB |

The refuse mark (4 GiB) was never approached, and no shed was needed. In the "after
warm-up" rows, the machine slept during the harness's 118 s pause, so those daemons were
idle for longer than planned before the burst. The figures are for the burst itself.

## 5. What the live run found that the tests do not

| # | Finding | Evidence | Weight |
|---|---|---|---|
| F1 | **The typed pass inside ingest ignores refuse.** RT-153's producer governor is checked in the connector's Fetch, `Sources` and the obligations corpus (`internal/connector/local.go:186`, `pkg/client/client.go:530`, `pkg/client/obligations_corpus.go:282`), but not inside the in-process `go/packages` typed pass. An ingest admitted before the climb ran 10 min at refuse. Own footprint overshot the exit mark by 2.4× (256 → 631 MiB); in safe mode, 4.6× (128 → 595 MiB in 6 s). With children counted, the peak was 1.3 GiB against a 128 MiB budget. | §3.2, §3.3 start 6; exit heap 90% `go/types` | High: the seatbelt held only at the exit rung. RT-173 says the exit mark halts "an in-flight ingest or typed pass", but nothing halts it at refuse. |
| F2 | **An index read blocked for 531 s behind the ingest.** A `find_symbol` sent at t=75.7 s returned at t=607 s (`matched_via: miss`, 0 results). The call log recorded `latency_ms 5045`, `queued_ms 0`, so about 526 s was spent before the handler's clock started, past the 30 s index deadline and never counted. | ladder2-ingest results vs. `calls.jsonl` | High: a hook-shaped read can hang for minutes during an ingest while the log says 5 s. |
| F3 | **The warm-up gate never treats a hook as a hook.** `warmupGate.acquire` compares the raw `X-Fathom-Origin` to the literal `"hook"` (`cmd/fathom/lifecycle_gate.go`, `origin != "hook"`), but hooks send `edit-hook`, `session-hook` or `prime`. During warm-up, hook `list_services` and `obligations` calls therefore queue up to 5 s instead of being refused at once. They are serialized (enforce run: p50 1.0 s, max 2.6 s; `queued_ms` 0 because the wait happens before admission). Fix: use `tenancy.ParseOrigin(origin, false) == tenancy.OriginHook`. | §4 enforce row; code | Medium |
| F4 | **Lifecycle refusals do not follow §3.4.** Safe mode, warm-up, drain and store distress go through `writeBusy`. It sends `message: "fathom busy"` with no reason (the reason is only in `data.reason`) and no `X-Fathom-Busy-Scope` header. The admission path sends `"fathom busy: <reason>"` and a scope. A hook treats a missing scope as daemon-wide, so the breaker behaviour is right for safe mode but wrong for a warm-up refusal, which is one class. | §3.3 start 6 `rpc_data`; §4 warm-up row | Medium |
| F5 | **A queued waiter that wins a slot after the ladder reached refuse is admitted without a level re-check** (`Gate.Admit` checks `decideLevel` only on entry). The producer governor stopped each one at once, so memory stayed bounded. But the caller got a 200 carrying `-32603 "over budget: tree producer halted"` instead of a 503 with `Retry-After`. | §3.1, 12–22 s | Low |
| F6 | **Doctor's `daemon_lifecycle` reads the wrong ledger for a `--lifecycle-id` daemon.** It opened `addr-7691-<pid>/lifecycle.jsonl` and reported "starts and unclean exits not recorded … RT-37 is not in the build that wrote it", which is false: the build has RT-37, and the ledger was under `rtb/`. `fathom resources` (with `--id rtb`) labels the same unmanaged daemon "(managed)" and "pid 0". | `doctor.txt`, `resources.txt` | Low |
| F7 | `top_in_flight` said `ingest(…, 1m15s)` for an ingest that had been running for 10 min. | exit row, §3.2 | Low |
| F8 | With cwd = `$HOME`, the user config `~/.fathom/config.toml` is also read as the repository's and is named "ignored: budgets are read at user scope only", even though its budget was applied. | smoke start | Cosmetic |

None of these broke the bound. F1 and F2 are the ones a wave-C or wave-D lane should
take before `admission` defaults to `enforce` (RT-96).

## 6. `fathom resources` and doctor on the scratch daemon

`HOME=<scratch> fathom resources --id rtb --url http://127.0.0.1:7691`, exit 0:

```
state      normal · mode normal · budget 128 MiB · shed 96 MiB · refuse 128 MiB · exit mark 256 MiB [live daemon (/statusz)]
footprint  47.8 MiB now (+0 B children) [phys_footprint] · 24h max 1.3 GiB (09-27 03:47, refuse) · ·······················█ [ring: 16 samples, 7 transitions]
lifecycle  up 0s · 3 starts/24h · 0 unclean · 1 over-budget exits/24h · 0 counted exits/30m · switch run · 1 quarantined
           last exit 2026-09-27T10:56:00Z over_budget (code 70): own footprint 630.8 MiB ≥ exit mark 256 MiB after halting the in-flight ingest
quarantine quarantined: rtb-probe exceeded budget during ingest — retry with the daemon's `ingest` tool (service "rtb-probe"); a completed explicit ingest lifts it
host       pressure normal · swap 8.8 GiB/10 GiB · store volume 8.1 GiB free · swap volume 8.1 GiB free
in-flight  point 0 · index 0 · tree 0 · fleet 0 · ingest 0 · egress 0 · abandoned by governor 0 · refused: refused 0 · would refuse 0 (admission enforce; by class in /metrics)
scratch    1 unmanaged daemon(s), 0 leaked, 0 stale record(s) [ps + daemon records]
disk       ~/.fathom 1.1 MiB · written by the daemon 6.9 MiB since its start [rusage]
           written 0.01/2 GiB in the last 1m0s (since the first reading) outside explicit ingest (+0 B during explicit ingest) — within budget so far
diag       last capture 09-27 03:56 exit → …/rtb/diag/20260927T105600Z-exit
verdict    within budget (exit 0)
```

These are doctor's tenancy rows (`fathom doctor --url http://127.0.0.1:7691`, scratch
`HOME`, on a clone). Doctor exited 1 overall because the scratch `HOME` has no hooks:

```
daemon_footprint  PASS  phys_footprint 48.0 MiB (+0 B children) vs budget 128 MiB (shed 96 MiB, refuse 128 MiB) · level normal · mode normal · GC hint 0.25 GiB
disk_writes       PASS  written 6.9 MiB in the 2m0s since the daemon's first reading (the disk-write ring is in memory …) against 2 GiB/day
admission_mode    PASS  admission enforce
daemon_posture    SKIP  no fathom launchd agent installed
daemon_lifecycle  WARN  quarantined: rtb-probe exceeded budget during ingest — … (see F6 for the wrong-ledger clause)
scratch_daemons   PASS  1 unmanaged serve-http daemon(s), 0 leaked, 0 stale record(s)
tenancy_scope     PASS  [tenancy] is read at user scope only (…/home/.fathom/config.toml) and env
```

## 7. Safety and cleanup

- Only one scratch daemon ran at a time, on port 7691. The live daemon on 7575 was not
  touched, and neither were `~/.fathom`, `~/Library/LaunchAgents` or the operator's call
  log.
- No run came near the 6 GiB kill line. The highest footprint in any run was 1.00 GiB
  own, or 1.3 GiB with children.
- The store copies, scratch `HOME`, corpus clone and binary were deleted after the
  reading.
