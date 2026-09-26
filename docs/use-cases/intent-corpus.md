# Keeping durable intent honest

**Experimental** (ADR-026, wave 1–2 shipped; the oracle runtime is gated on
demand that does not exist yet — see *What is not built* below).

A long-lived repository accumulates statements about itself: invariants that
must never break, decisions with conditions for reopening them, non-goals,
sensitive surfaces and the evidence a change there needs. Written down, those
statements are load-bearing — an agent reads them and treats them as true.

Nothing checks whether they still are.

fathom's job here is narrow and it is not authorship: **read what a repo
already declares, check what an instrument can actually show, and list what
nothing shows.** It never writes intent, never resolves a conflict, never
scores a repository, and never turns an honest "nobody enforces this" into
debt.

## Who this is for

A repo carrying the Project Intent practice (`docs/intent/` router plus
decision records), or just decision records with `Revisit when` trailers.
Both are supported; so is neither, which is a choice rather than a defect and
is reported as one.

## The loop

```sh
fathom intent audit                  # what it declares, what shows, what does not
fathom intent audit --kind unproven  # just the debt, with a remedy per finding
fathom intent audit --json           # for a script; unbudgeted
fathom doctor                        # rows that summarize it all, and never FAIL
```

`audit` reads three sources and joins them:

1. **The corpus** — the router's docs map, the invariant table with its
   enforcement rung per row, every condition (`Revisit when`, `Promote when`,
   `Drop if`, `Done when`), the sensitive-surface table, and the decision
   records the router points at.
2. **The index** — one `verify_plan` call resolves every test and symbol the
   corpus names, cached per HEAD (6.6 s cold → 84 ms warm).
3. **The gates** — `Taskfile.yml` targets, `lefthook.yml` steps, `scripts/`,
   and the newest recorded verify-gate run per plan.

### What it will tell you

Worked example, this repository:

```
67 condition(s) (0 annotated, 12 terminal)
37 invariant(s) [code 4, gate 6, lint 4, stated 2, test 19, violated 2]
14 sensitive surface(s), 0 covered by guard.cue
unproven claims: 89   [U11 2, U12 62, U13 14, U2 7, U3 2, U4 2]
```

Read that as: most invariants are enforced by a test or a gate; two are
knowingly VIOLATED; sixty-two plan rows claim done inside a sensitive surface
without pinning which test guards them; and the edit-time channel for
sensitive surfaces is proposed but not wired.

None of that is a failing grade. It is a list with a remedy on every line.

### What it refuses to do

- **It does not turn `stated` into debt.** An invariant whose enforcement is
  "a human reads it" is honestly rung. Counting that as debt is how rungs get
  faked upward.
- **It does not evaluate a condition nobody annotated.** Conditions are prose
  by design. fathom reads a `{oracle: …}` annotation only where an author
  wrote one, and there is no LLM guessing what a sentence meant.
- **It does not write.** `intent export --guard` PROPOSES `.fathom/guard.cue`
  entries from the router's surfaces, stamped with provenance so they can go
  stale visibly. It prints them for you to paste.
- **It does not score.** No grade, no percentage, no leaderboard. A number
  that ranks repositories invites gaming the number.
- **It says what it could not compute.** Every unreadable source, unresolved
  test and unmatchable surface is listed with the reason. An audit that
  silently returns nothing looks exactly like a healthy repo.
- **It never lets blindness look like health** (ADR-027). If a corpus file
  cannot be READ — as opposed to being absent — every count becomes a named
  lower bound and the blindness leads the NOT-COMPUTED list:

  ```
  unproven claims: ≥ 85 (1 source(s) unreadable: docs/intent/invariants.md: permission denied)
    NOT COMPUTED — COVERAGE INCOMPLETE — every count below is a LOWER BOUND: …
  ```

  This was a real defect, measured: before the fix, an unreadable
  `invariants.md` made the audit report 0 invariants instead of 37 and half
  the debt, naming nothing. `--json` carries `coverage.complete` so a machine
  reader can tell a real zero from one nobody could see.

## The doctrine plane: where each vision stands, and which goals are bets

```sh
fathom intent doctrine               # every vision and goal, each finding with a remedy
fathom intent doctrine --kind bets   # just the goals
fathom intent doctrine --json        # for a script
```

Visions are promises, and checking their prose would be theater. What can be
checked is where each one stands on the pipeline the repo already practices —
vision → decision → plan rows → proven code — so that is all `doctrine`
computes, per vision: its stage from its own `Status:` line (or a status its
index entry leads with), the plans that carry it and their claimed rows, whether
its links and cited decision ids resolve, and the age of its newest dated
statement. A vision naming no status, trigger or plan is listed `unstated`; one
left for review over 30 days with no plan is listed `stale`. This repository,
2026-09-25: 35 visions — 12 landed, 10 planned, 4 for review (all stale), 9
unstated — and one README link to a plan that was archived.

A goal counts only as a pre-registered bet: an instrument this repo can run, a
threshold, a review date, written on its `Done when:` —

```
**G2. The A/B becomes admissible.** Done when: `fathom flywheel` reports ≥20 …
{bet: arm-bench, instrument: "fathom flywheel", threshold: "≥20 arm-homogeneous units per arm (MI-5), then a keep/retract verdict under law 12", review: 2026-10-25}
```

— or it is refused at load, naming what it lacks. When a review date passes
the bet is listed as due; the keep-or-kill decision is yours, recorded on the
goal. A bet block evaluates nothing and never counts as an oracle annotation.

## In a session

One line reaches the SessionStart primer, sharing the custodian's rather than
adding a second:

```
⚠️ attention: config 2 · unproven 8 (+4) · fired 1 — fathom doctor
```

It reads the recorded ledger only — no audit, no index call, no RPC — and it
goes silent when the last recorded audit is over 14 days old, because a stale
count is not evidence about the working tree.

`fathom fleet report` carries an `unproven` column per repo, read from the
same ledger. A repo with no recorded audit shows `n/a: no audit recorded`,
never a `0` that reads as clean.

## Turning it off

```cue
// .fathom/capabilities.cue
capabilities: intent_corpus: off: true
```

That silences the two doctor rows. The verb still answers on demand — the
switch turns off a reporting channel, not a capability.

## What is not built, and why

- **Oracle evaluation** (`index`, `plan`, `lexical`, `date` annotations that
  make a condition machine-checkable). Gated on K0: at least three
  hand-written annotations across two repositories. Today there are zero, and
  writing them ourselves would fake the demand signal the gate exists to
  measure. Whether the practice's own template accepts the annotation at all
  is an open question (Q18).
- **`decided` derivation** — same gate.
- **The observatory panel** — its host panel (SD-2) is unbuilt.
- **Anything leaving the machine.** Honesty-debt counts stay in local
  artifacts; nothing reaches the research bundle until that is decided (Q16).

## Maintained

This doc covers: `fathom intent audit` (`--root`, `--json`, `--kind`,
`--fresh`, `--top`), `fathom intent doctrine` (`--root`, `--json`, `--kind`,
`--top`), `fathom intent export --guard`, `fathom doctor`'s
`intent`, `unproven_claims` and `doctrine` rows (incl. the measured-stale count), the `intent-corpus` capability, the
session attention line, and `fathom fleet report`'s `unproven` column.
Update it when these change.
