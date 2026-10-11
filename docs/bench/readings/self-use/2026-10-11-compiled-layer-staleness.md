# Compiled rule layers and docs churn: what the 24-of-103 count shows (hygiene note G10, 2026-10-11)

Reading: a compile dry run of the fathom service's obligations layer at 18:05Z on 2026-10-11 named 24 of 103
sections to compile. No commit touched the layer's corpus between that day's compile runs and the dry run. The
layer's corpus took 106 commits in the 14 days from 2026-09-28 to 2026-10-11.

Plain reading: the first note read the 24 sections as "stale again within the hour after a recompile". The record
does not support that. No committed edit landed on the corpus between the compiles and the dry run, so the 24 are
better explained by the day's budget-stopped run (21 sections, stopped by its budget before the corpus was done)
and by the dry run's own overcount (hygiene note G9: it ignores idempotency and carry). What does stand is the rate
underneath: the files this layer compiles change about seven times a day on average, and 23 to 39 times on the busiest
days. A layer that needs model spend after every edit to its corpus cannot stay fresh in a repository that moves this
fast. In such a repository the raw-section fallback is what agents usually get, so its quality is the one that
matters. A repository whose rule documents move rarely keeps its compiled claims.

`G10` is the operator's hygiene-queue label for this finding (2026-10-11 notes, section Q3 RESULT). It is not scope
goal G10, which was voided on 2026-09-13.

## Provenance

- The compile pass of 2026-10-11 ran under an owner approval of $3 (a small hosted model through the Claude CLI
  profile). HTTP compiles at 08:02Z: the fathom service 17 sections ($0.92, budget-stopped) and a second service 13
  of 14 sections ($0.36). Direct runs: the fathom service 21 sections ($0.88, budget-stopped, 23.6 minutes), app-kit
  2 sections ($0.09), and a refusal as idempotent on the second service. Total $2.25. Graded *measured* (the
  compile runs' own records, read by the session that ran them).
- The dry run at 18:05Z named 24 of 103 sections. Graded *measured* by that session; not re-run here, because a
  re-run reads the live daemon's store, which has moved since.
- Corpus churn: `git log` over the obligations layer's corpus (`schema/semantic/layers/obligations/layer.cue`:
  `AGENTS.md`, `CLAUDE.md`, `ADR.md`, `docs/plans/README.md` and every nested `AGENTS.md` or `CLAUDE.md`, excluding
  `third_party/` and `testdata/`) at `65e5acfa`. Graded *measured*; the per-day rate is an *estimate* over it.
- No corpus commit between 2026-10-10 23:27:57Z (`8e6a4b8d`) and 2026-10-11 20:38:12Z (`1639b37c`). Graded
  *measured*. Whether the indexed checkout held uncommitted corpus edits at 18:05Z was not recorded (*unmeasured*).

## Counts

| Quantity | Value |
|---|---|
| Sections the dry run named to compile, of all sections (18:05Z) | 24 of 103 |
| Sections compiled that day by the budget-stopped direct run | 21 |
| Corpus commits, 2026-09-28 to 2026-10-11 (14 days) | 106 |
| Corpus commits per UTC day by commit date, 2026-09-28 to 2026-10-11 | 23, 39, 2, 4, 4, 3, 0, 0, 0, 0, 0, 6, 24, 1 |
| Corpus commits between the 2026-10-11 compiles and the dry run | 0 |

Re-run the churn count: `git log --since=2026-09-28T00:00Z --until=2026-10-12T00:00Z --format=%H -- ADR.md AGENTS.md
CLAUDE.md docs/plans/README.md ':(glob)**/AGENTS.md' ':(glob)**/CLAUDE.md' ':(exclude)third_party/**'
':(exclude)**/testdata/**' | wc -l`. Re-run the section count: `fathom compile --service fathom --layer obligations`
(a dry run by default; it spends nothing).

## What this does not claim

- Not that the 24 sections went stale from docs edits. The count does not separate sections the budget-stopped run
  never reached from sections whose text moved, and G9 says the dry run overcounts.
- Not a staleness rate per section. A commit to `ADR.md` usually touches one record, so the share of sections a day
  of commits moves is *unmeasured*; the conclusion rests on the commit rate, an *estimate*.
- Nothing about other repositories.

Design consequence (*hypothesis*, carried by the records that cite this reading): compile at a release tag or on a
weekly budget, once per unique section digest, and make raw-section serving good on its own. Records that cite it:
`docs/assessments/2026-10-11-index-snapshots.md` section 6.2 and
`docs/assessments/2026-10-11-making-the-vision-happen.md` section 1.
