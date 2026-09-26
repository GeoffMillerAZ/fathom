# The assess lanes: typed judgments of on-disk work, spend kept out of the units

> Rows DO-3, IR-3, IR-4 and IR-5 in `docs/plans/ambient-fathom.md`, which
> `docs/intent/scope.md` G4 listed as undesigned. Written 2026-09-25, before
> any assess run existed. Governing sources: `docs/vision/20-ambient-fathom.md`
> (the plan's binding promotion), `docs/design/continuity-threads.md` §4.1
> (outcomes are derived, not self-reported), `docs/design/intervention-outcomes.md`
> §1 (the two-lane retro and the owner's isolation caveat), `docs/vision/24-work-frontier.md`
> §7 (the story program ships through the sanitized bundle), and
> `docs/vision/32-decision-port.md` with ADR-042 (the decision port). Decision
> record: ADR-046.

## 0. Before designing: what applies

- **P2, determinism first.** The mechanical lanes (DO-1's derived outcome, IR-2's
  `fathom retro`) stay the standing instruments. The model works only at the
  margin they cannot decide, and its output is gated on calibration (§4).
- **Law 12 (P12).** Assess judges quality. It never licenses a savings claim and
  never supplies a causal link. Only the arm does that.
- **Law 13 (P13).** Everything assess and retro write stays local. The retro corpus
  leaves the machine only through the sanitized bundle, only when a human passes
  `--include-retros`, and never with a verdict in it.
- **Law 14 (P14).** A verdict is a lens. Nothing downstream consumes one as
  evidence while it is uncalibrated, and no verdict is ever a claim.
- **I14 (egress).** Assess makes its model calls only through the decision port.
  That is the existing sanctioned egress, not a third one. I14's wording said the
  port is always opened from the daemon, which `certify suggest` already
  contradicted. The amendment names both CLI verbs and keeps "never from a hook".
- **I42.** `[decide] provider = none` (the default) still makes no call anywhere.
  Assess then refuses and names the remedy.
- **ADR-023's revisit trigger fires.** Its condition was "fathom adds an LLM-judged
  evaluation lane", and this is one. The lane stays bespoke for v1 because
  `qa-harness` scores scenarios against a cassette-replayed corpus with a committed
  baseline, and assess has neither until the adjudicated sample in §4 exists. That
  sample would be the natural `qa-harness` corpus. F12's promote condition (a second
  consumer) is not met. **The owner decides** whether ADR-023 stays as is with this
  note or schedules adoption at M-A1.
- **Refused and still refused.** Transcript mining (DF-4), per-fire LLM journaling
  (intervention-outcomes §1), any LLM summarization in the thread loop (vision 21),
  and a new MCP tool (P4). Assess reads no transcript, and a test proves it.

## 1. The two judgment lanes

Both lanes are CLI verbs, explicitly invoked (P10). Neither runs from a hook, a
daemon or a schedule.

```
fathom assess [--session S | latest stamped] [--project DIR] [--interventions]
              [--dry-run] [--json] [--max-tokens N]
fathom assess spend [--since D] [--by label|day|model] [--json]
```

**The unit** is one session's beat window in one project, the same window DO-1's
derived outcome and IR-2's retro already use: the span of that session's stamped
beats, plus one minute. A session with no stamped beats has no unit, and assess
says so instead of guessing a window.

### DO-3, the benchmark lane (`fathom assess`)

The work products are read from disk and git only:

| Product | Source |
|---|---|
| diffs | `git log -p` for the commits whose commit time falls inside the window (committed work only; the working tree is not what shipped) |
| plan deltas | the checkbox lines under `docs/plans/` those commits changed: a row flipped to `[x]`, reopened, or added |
| gate results | `verify-ledger.jsonl` rows for this repo inside the window (claimed, verified and failing per plan, with origin), plus the session-ledger row's `verify_catches` |

A unit that landed no commits has nothing on disk to judge. Assess prints that,
spends nothing, and exits 0.

The questions are typed. The set is versioned by a digest over its templates, and
every ledger row carries that digest so readings from different sets are never pooled.

| id | kind | question |
|---|---|---|
| `coherent` | noul | The commits implement one coherent, finished change, not a partial, abandoned or broken one. |
| `claims_backed` | noul | Every plan row flipped to `[x]` is backed by code or tests in the diffs. Asked only when the unit flipped a row. |
| `tests_accompany` | noul | The behaviour changes in the diffs come with test changes that exercise them. |
| `quality` | choice | Five named levels: harmful, incomplete, adequate, solid, exemplary. |

`quality` is a choice over named levels rather than a `score`. The port's score
answer carries no probability spread, so its confidence always collapses to 1.0 by
the kit's `k ≤ 1` rule. A score would therefore always land in the proceed band,
which is exactly the false confidence the bands exist to prevent.

### IR-3, the intervention lane (`fathom assess --interventions`)

This lane reuses IR-2's retro model for the session. It sends only the rows the
mechanics cannot decide:

| Retro row | Lane |
|---|---|
| guard or obligations firing on F, outcome `committed` or `modified-uncommitted` | **judged**: did the change to F follow the guidance? |
| firing on F, outcome `untouched` | not judgeable from disk: nothing changed, and intent lives only in the transcript, which is refused. Named. |
| firing without a file or session stamp | unjoinable, counted and named as the retro already does |
| custodian attention | already decided mechanically (remedied or unremedied). Not sent. |

Each judged file gets one choice question with four options: `complied`,
`departed`, `not_applicable` and `cannot_tell`. `cannot_tell` is an explicit
option, so the judge never has to force a verdict. The state holds the guidance
and F's diff inside the window: committed hunks, plus the working-tree diff when
the outcome is `modified-uncommitted`.

The guidance is re-derived from disk at assess time, and its provenance is shown
on every judged row. For guards, it is the matching note in `.fathom/guard.cue`.
For obligations, it is the rules the loopback daemon serves for F now. IR-1
records *that* a firing happened, not *what* it said. A row whose guidance cannot
be recovered is named unjudgeable and never asked with empty guidance. Repeated
firings on one (channel, file) collapse to one question, because they judge the
same final diff. The lane asks at most 24 questions per run and names the rest.

### What both lanes share

- **DLP before egress.** The state passes through `llmsec`'s redactor, the same
  gate compile uses. A dry run prints the redacted state and the redaction count.
- **Caps are named.** Every diff section is capped: 8 KiB per file for IR-3, and
  48 KiB of state per run. Each cut is stated inside the state and in the output.
  A truncated run is marked `partial_state` and never enters a calibration sample.
- **A pre-flight token cap.** Estimated input tokens are the byte/4 heuristic
  compile uses. Above `--max-tokens` (default 40,000), assess refuses before any
  call and names the estimate.
- **`--dry-run` spends nothing.** It prints the exact redacted state, the questions,
  the question-set digest and the estimated tokens and cost. It needs neither the
  capability nor a provider, so a reader can always see what would be sent.
- **The per-run cost goes in the output.** The cost is the provider-reported tokens
  in and out, priced through the compile meter's table (`rateForModel`,
  `cost.PriceUSD`). A model with no price reads "cost unknown: no price for model
  M", and missing usage reads "tokens not reported by the provider". Neither is
  ever printed as zero. `fathom assess spend` totals the assess ledger through
  `metrics.AggregateSpend`, the aggregation behind `fathom metrics spend`, grouped
  by spend label, day or model.

## 2. The gates, in order

A live run must clear all four gates, and each refusal names its remedy:

1. **Capability `assess`**, default off, user or project scope. This is consent to
   a priced lane. Enforcement fails CLOSED: a malformed capabilities file refuses
   the run. That is the opposite direction from the safety capabilities (CR-7),
   because an unreadable file must never be read as consent to spend.
2. **The token cap.** It runs before any client is constructed, because some
   providers probe liveness when they are built (the codex-cli lane pings).
3. **A decision provider.** `[decide] provider = llm` resolves the existing llm
   profile machinery (`[decide].profile`, then `default_profile`, then the env
   overlay). `typesafe` uses its key indirection. `none` refuses.
4. **The `assess` site in `[decide] sites`.** It is in the default list, and a
   narrowed list that omits it refuses.

The isolation environment (§3) is set between gates 2 and 3, before the port
opens, so a refused run changes nothing.

## 3. IR-4, spend isolation (pre-registered 2026-09-25, before any assess run)

The owner's caveat, made structural. It is registered as `IR-4@2026-09-25`, and
every assess ledger row carries that string:

1. Every assess model call is metered on the **assess ledger**
   (`~/.fathom/assess-ledger.jsonl`, overridden by `FATHOM_ASSESS_LEDGER`) under
   the spend label `assess` (DO-3) or `assess-interventions` (IR-3). It is never
   metered on the session ledger. Retro spends no tokens (IR-2 is LLM-free by
   construction) and therefore adds nothing to any unit.
2. Before opening the port, assess sets `FATHOM_CHANNELS=off` and
   `FATHOM_SPEND_LABEL=<label>` in its own environment. A judge process that a CLI
   provider starts (a `claude -p` or `codex exec` child) inherits both. Every
   fathom hook in that child stays silent, so no brief reaches the judge and no arm
   budget is decremented. Any session-ledger row written anyway, for example by a
   harness whose own env overrides the switch, carries `spend_label`.
3. A session-ledger row with a `spend_label` is never an arm unit (`armUnitFor`),
   never a thread-bet unit (`threadBetCheck`) and never an admissible row in the
   flywheel's arm loop. Each exclusion is counted and named where it happens,
   never dropped silently.
4. Judging a unit never adds to that unit's figures. Assess reads the session
   ledger and never writes it.
5. Amending 1–4 after the first assess row exists requires a recorded amendment
   (T5). The rule can be amended only before data exists under it.

## 4. The decision port, and what Jev changes (the owner's standing rule)

**Where a typed question replaces a free-form judge.** Every judgment is typed:
noul for binary properties, choice for levels and outcomes. A free-form judge's
prose would need a parser, could not be banded, could not be compared across
providers, and would invite quoting as evidence (law 14). Typed answers aggregate,
calibrate, and swap providers without changing a line of the lane. `none`, `llm`
and `typesafe` are data behind one interface.

**Where it does not help.** A human adjudicator may want a reason, and the port
returns none. Large diffs hit the state caps, and the bands do not know the state
was truncated, so truncated runs are flagged and kept out of calibration. Jev's
advantage is cheap, fast, probability-shaped answers at volume, so it fits IR-3's
per-file questions better than DO-3's one heavy unit. Its model is unpriced in the
compile meter's table, so its cost reads "unknown" until someone prices it (a data
decision, `modelprice.go`).

**The calibrated-band policy.** The bands are the kit's: proceed at 0.9 or above,
unsure below 0.5.

- A **proceed**-band answer is the verdict.
- A **confirm**-band answer is shown as "leaning" and counted in no total.
- An **unsure** answer is "undecided".
- An invalid answer is named with its reason.

Every verdict is **uncalibrated** until its question-set digest passes M-A1 below.
While uncalibrated, verdicts stay on the local ledger, never enter the bundle, and
never feed an arm, bet or flywheel figure. A new question set resets calibration.

**Fail-open.** Assess is a measurement lane, so failing open means no fabricated
verdict and nothing else disturbed:

- A `none` provider or a disabled site refuses with its remedy and makes no call.
- A Decide error writes a `RecordError` row to `decisions.jsonl` and a ledger row
  carrying the error, then exits 2 (operational). No verdict is recorded.
- The mechanical lanes never depend on assess.
- No hook ever reaches assess.

**What would prove it** (measured on real units, never on fixtures):

- **M-A1, calibration.** On at least 10 adjudicated units per question-set digest,
  proceed-band verdicts agree with a human adjudicator at least 9 times in 10. This
  is ADR-044 decision 8's bar.
- **M-A2, well-posed.** `llm` and `typesafe` agree on at least 80% of proceed-band
  verdicts on the same units. Below that, the question is provider-specific, not a
  property of the work.
- **M-A3, discrimination.** On at least 10 pairs, `coherent` separates units DO-1
  already separates (landed and proven work versus spun) in the expected direction.

**What would kill it:**

- A question that fails M-A1 at every threshold (the route site's shape, ADR-042's
  2026-09-24 amendment) is removed and the digest bumps.
- If every question fails, DO-3 and IR-3 are recorded as refuted with the reading,
  and the capability stays off.
- A rationale field is added only if adjudicators mark more than half of
  proceed-band verdicts "unusable without a reason". Even then it is a lens and
  never a verdict source.

## 5. IR-5, the retro corpus feeds the flywheel (opt-in)

`fathom retro --write`, and the Stop hook's IT-13 retro, land one JSON artifact per
session under the project's state directory in `artifacts/retros/`. In this
repository that is `workspace/artifacts/retros/` (`[daemon] state_dir =
"workspace"`), and by default it is `$XDG_STATE_HOME/fathom/<repo-id>/`.

`fathom research bundle --include-retros` folds the artifacts whose window ends
inside the report window into one optional `retros` group. That group is part of
the closed CUE allowlist, is validated twice, and is additive, so a bundle without
the flag is byte-identical to before. It carries these counts:

- retros read, and unreadable artifacts (counted, never dropped)
- OFF-arm retros
- guard fires, obligation injections, silent-on-clean answers, and unjoinable firings
- per-channel outcome counts over a closed label set. The channels are `guard` and
  `obligations`. The outcomes are `committed`, `modified_uncommitted`, `untouched`
  and `unjoinable`, and anything else becomes `unrecognized`.
- custodian remedied and unremedied counts

The group carries no file, session id, path or verdict. Without the flag the group
is absent, which means "not read". With the flag but no corpus directory it is also
absent, and stderr names the directory. It is never an all-zero group standing in
for "not measured".

## 6. Out of scope for v1

- Assess verdicts in the bundle: uncalibrated verdicts never leave the machine.
- An observatory panel: the console wave (ADR-045) renders verbs, and `assess spend`
  is the verb one could render.
- Adjudication tooling: M-A1 is measured by hand first, as CV-M11 was.
- Automatic assess inside a declared bench window: explicit invocation only, until
  M-A1 passes.
- The served set of obligations at firing time: recording it is an IR-1 extension
  with a per-edit cost. It is deferred until IR-3's "guidance as served now" drift
  measurably matters.
