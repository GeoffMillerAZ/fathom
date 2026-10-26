# Language onboarding: research outcome and worker guidance

Outcome under controller ruling LH-R2: **new onboarding preflight tooling and a general
starter-query importer are rejected for now**. No replay, scorer or predicate is approved.
The deliverable is an evidence-based checklist in the existing recipe and add-language skill.
The [unnumbered decision draft](language-onboarding-decision-draft.md) records both rejections
and their wins-if conditions without allocating a record ID.

## Evidence and null result

The [assessment](../assessments/2026-10-25-language-onboarding.md) surveys 15 additions,
20 public GPU review groups and 21 named GPU revision/review rounds. The expanded
[mechanical census](../assessments/data/2026-10-25-language-onboarding-mechanical.json)
contains 10 defects: nine post-first-reading followups plus MATLAB's pre-reading bytecode
self-repair. Eight are mechanical, two semantic. Required integrator regeneration is excluded;
the late integration `exec-noctx` case is separately scoped and not counted twice.

The original pre-oracle scheduling proposal has a supported ceiling of one addition-specific
case, below its three-case/two-addition bar. The later staged-hygiene proposal used predicates
written from the same examples used to score them. Its catch result would hold by construction,
not demonstrate general onboarding value. A cheap read-only screen already establishes the
historical shapes and their absence at the reference clean trees. Building a scorer and freeze
protocol would add work without an independently testable efficiency bet.

This is a **research/feasibility null for new tooling**, not a measured claim that all preflight
checks are useless. No arm replay ran, no causal time/round saving was established, and no
runtime behavior, support level, threshold, reading schema or generated file changes here.

## Read-only screen, not an efficacy experiment

The census stores public revision identities, commands and counts. Resolve the named revision
to an existing tree before running these count-only commands. Missing/unreadable source is
operational, never a zero. Replace `REV` with an existing commit from the census:

```sh
revision=${1:-743a91430}  # choose a commit from the census
# Both failures below are operational; neither may become a zero count.
git cat-file -e "$revision^{tree}" || exit 2
files=$(git ls-tree -r --name-only "$revision") || exit 2
printf '%s\n' "$files" | awk '/(^|\/)__pycache__\/|\.py[co]$/ {n++} END {print n+0}'
rc=0
matches=$(git grep -l -I -E '(/User[s]/[A-Za-z]|/hom[e]/[a-z]|/Volume[s]/[A-Za-z])' "$revision" -- calibration/) || rc=$?
case "$rc" in
  0) printf '%s\n' "$matches" | wc -l ;;
  1) printf '0\n' ;;  # git grep found no match, not an operational error
  *) exit 2 ;;
esac
```

MATLAB bytecode paths: 1 at `76d49713d`, 0 at repair `29b6419a5`, before its first reading.
PgBouncer bytecode paths: 2 at `f1cf2dd0c`, 0 at `c62caefcf`, counted as one defect.
Objective-C calibration files matching the broad host-root screen: 2 at `e1824a503` and
`f3ff6f461`, 0 at `441b225e6`. Both screens return 0 at the census cutoff `743a91430` and
reference main `5b84aa5b4`. Matching private values are not reproduced.

The path screen is broader than the proposed AST predicate. These are shape prevalence
counts, not execution of the full gate, proof of runtime cost, or review-round savings.
Ordinary bytecode ignores/debris checks and the existing hygiene scanner are the appropriate
seams for any separately authorized hygiene code. LH-R2 authorizes no such code in this lane.

## Governing intent and the cheap deliverable

G15, I54 and ADR-048 require independent tiered readings and generated tracking. I55 retains
source/license provenance; I38 requires incomplete evidence to be disclosed; I62 keeps
judgments out of proof. ADR-075 requires fragments and generated-file regeneration. Existing
query reuse remains the default. No new applicable revisit trigger justifies a subsystem.

The approved changes to [recipe §13](language-breadth.md#13-the-worker-recipe--one-queue-item-start-to-finish)
and `.claude/skills/add-language/SKILL.md` add checklist items for measured-file identity,
truth independence, per-subject evidence, mutation sensitivity, adversarial count caps,
recursive reading preservation, named ceiling candidates, runnable verification, holdout
isolation and release-note/debris checks before READY. Stale dependency, write, verify and
step-24 changelog instructions are aligned with current rules.

Issue #7 E2 round 2 owns recursive regrading, canonical/value-preserving reading serialization,
measured-kind preservation and missing-metric grading tests. At reference main `5b84aa5b4`,
those changes are not yet merged; the existing round-trip test covers top-level JSON values.
The recipe points to existing tests with a coverage caveat and requires manual nested evidence
until E2 lands. This lane adds no duplicate checker and does not claim an absent test passed.

## Rejected-for-now conditions

**Preflight wins if:** after ordinary hygiene fixes land, at least three addition-specific
mechanical escapes across at least two additions remain uncaught by existing checks, with
public bad/fixed pairs and an independent held-out evaluation that can change the decision.
A broader existing gate or guidance step should be preferred when it already covers them.

**Importer wins if:** recurring query-conversion failures remain beyond the current draft/probe
path, with permissive pinned sources and an independent held-out comparison demonstrating
less work without weaker extraction or provenance. A catalog of available queries alone is
not evidence that an importer pays.

## Decision models

None applies. Source identity, evidence counts and the worker checklist are deterministic.
Provider `none` is the complete path. No judgment, model call or paid API enters proof.
