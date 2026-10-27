# Lane adm: named refusals in a cold run at 512 MiB, a public proxy, 2026-10-27

**This is a public proxy, measured on this repository.** It is not the consumer-corpus cold-arm
receipt the game-development plan asks for, and it proves no plan row.

**Verdict, in plain words.** In four cold runs, two of origin/main (01ea58c6b) and two of this
lane (6d117677e), every refusal and every abandonment was named somewhere. None was silent:
the one abandoned call named its cause in its response and its call-log row, and every plan
`plans status` could not measure was listed as not measured. One gap remains. Three of the four
daemons left at the exit rung during `plans status`, both arms alike. The cause (`exiting 70 over
budget: own footprint 1.0 GiB ≥ exit mark 1 GiB`) is named only in the daemon's own log. The
client named each plan it could no longer measure by its transport error (`EOF`, `connection
refused`), not by that exit. The lane does not change memory and was not expected to: both arms
peaked at about 1.0 GiB, and whether a daemon reached the exit rung, and when, varied run to run
under this machine's load, so two cold starts per arm do not separate the arms. The memory
itself is O1014's graph residency, outside this lane.

## What ran

- Corpus: one fresh CLI ingest of the lane tree (6d117677e) per binary, into a scratch store:
  7,826 files discovered, 70,964 symbols, outcome `ok`. CLI ingest has no governor: peak
  footprint 2,560,199,272 B (origin/main binary, 188.6 s) and 2,561,034,880 B (lane binary,
  96.0 s; wall times here vary with the machine's load).
- Daemon: an unmanaged `serve-http` per run on its own port, with a scratch `FATHOM_HOME`, store,
  call log and verify ledger, `FATHOM_TENANCY_BUDGET=512MiB` (shed 384 MiB, refuse 512 MiB, exit
  1 GiB), admission in its default `observe` mode, auto-refresh off, `GOPROXY=off`. Each run
  started cold on a fresh copy of its arm's post-ingest store.
- Requests, in order: `fathom verify --url <daemon> --service fathom --plan
  docs/plans/language-breadth.md` (794 tasks, 578 claimed), then `fathom plans status --url
  <daemon> --service fathom --json` (96 plans).
- Footprint: `/metrics` `fathom_footprint_bytes` plus `fathom_children_footprint_bytes`, sampled
  every 1 s (the governor itself samples every 2 s): a sampled peak, not a true maximum.
- The machine ran other lanes' gates and tests during the window (12 CPUs; load was not
  sampled), so wall times are this machine's at that hour, not a benchmark.

## Results

| run | verify: exit, wall, calls | plans status: exit, wall, measured, calls | peak (sampled) | ladder |
|---|---|---|---|---|
| origin/main 1 | 0, 22.8 s, 1 ok (578 of 578 verified) | 2, 887 s, 94 of 96 (2 `EOF`), 161 ok | 1030 MiB | shed at 14 s, refuse at 20 s, exit 70 at 15 min 8 s |
| lane 1 | 0, 19.5 s, 1 ok (578 of 578 verified) | 2, 265 s, 35 of 96 (58 `connection refused`, 3 `EOF`), 51 ok | 1033 MiB | shed at 10 s, refuse at 18 s, exit 70 at 4 min 40 s |
| origin/main 2 | 0, 18.9 s, 1 ok (578 of 578 verified) | 0, 975 s, 96 of 96, 173 ok | 970 MiB | shed at 10 s, refuse at 18 s, no exit (stopped by the harness) |
| lane 2 | 2, 31.2 s, 1 abandoned | 2, 473 s, 56 of 96 (36 `connection refused`, 4 `EOF`), 89 ok | 1029 MiB | shed at 17 s, refuse at 29 s, exit 70 at 8 min 21 s |

- The abandoned call (lane 2's verify) answered `abandoned: the call's deadline passed (the
  index-class server deadline or X-Fathom-Deadline); verify_plan did not finish and no partial
  answer is served`, and its call-log row's outcome is `abandoned`. The CLI exited 2, the
  operational code `fathom verify` fails open on.
- Admission refused nothing: in `observe` mode every call is decided and counted, and only
  `enforce` refuses. Each run logged and counted one would-refuse (`list_services`, fleet class,
  agent origin, level refuse; `fathom_would_refuse_total{class="fleet",reason="level"} 1`) and
  served it.
- Every call that ran to an answer answered `ok`. No `fathom_inflight` count remained in the one
  run whose end-of-run `/metrics` could be read (origin/main 2); the three that reached the exit
  rung left with the process.

## Against O1014

O1014's fx5 (v0.15.3 at 512 MiB, language-breadth at 768 rows) took 54 `verify_plan` calls and
665 s, and ended on `the call graph for service fathom could not be loaded: context canceled;
fathom is at tenancy level refuse`. Here the same request is one call in 19 to 31 s at 794
rows, and it answered in three of four runs. That is fx6's one-call-per-plan change, which
both arms carry, not this lane. fx6's 512 MiB sub-run read `plans status` at exit 2 with 1 of 76
plans unmeasured, a `refuse` cancel named in the answer. Here `plans status` lost the daemon
itself in three of four runs.

## What this lane changes here, and what it does not

The lane's admission changes (a setup panic that frees its slot, the ingest tool's refusals and
the lifecycle body in the §3.4 wire format) and its ingest changes (named CLI cancellation, the
busy refusal) are not exercised by these two requests: no handler panicked, nothing called the
`ingest` tool, and nothing ingested beside another ingest. The runs show they cost nothing on
this path. The open gap is outside the lane: a client of a daemon that left at the exit rung
cannot name why. Naming it would need the exit reason somewhere a client reads after the daemon
has gone, for example the daemon record or the lifecycle ledger, which `fathom resources`
already reads.
