# Measuring a tool against a reference: is the oracle adequate?

Practice for a project that grades a tool (a symbol extractor, an indexer, a classifier,
a judgment site) against a reference answer, an oracle, and publishes the grade. It covers
how to check the instrument before trusting the reading, how to tier and cap oracles, how
to disclose an oracle's blind spots without inflating a score, and what to record with
every reading. Written 2026-09-29 from one consuming project's language-extraction
benchmark, where the measuring instruments turned out to be the bottleneck; the ledger
entry is [practices §31](practices.md#31-unaudited-oracles).

**Evidence tags.** [field] paid for by an incident or reading in a real run; the incident
is stated. [design] a designed practice with no result yet. [reading] a measurement by
one consuming project on its own data, not transferable to yours.

## Verdict first

- **What it is.** A reading of "the tool scored X against oracle Y" measures the tool and
  Y together. When the score is low, the tool, the oracle or the mapping between them may
  be wrong. Treat the oracle as an instrument with its own error, and audit it before you
  trust or publish a grade.
- **Where it pays.** Any benchmark that grades a tool against a third-party reference, a
  derived answer key or a labeled set, especially when workers fan out across many
  subjects and each reading becomes a claim about a whole language, format or workload.
- **Where it does not.** A reference that is exact by construction (a computed fact, a
  compiler's own answer for its own language) needs only the scope and mapping checks.
- **The one rule.** The grade a reading may earn is capped by the independence and audited
  adequacy of its oracle, and every exception granted to the tool is deterministic,
  disclosed and itself audited.
- **What is not claimed.** That any particular oracle is best. The tiers below are a
  method; the tools named are the ones the consumer met.

| # | Practice | Status |
|---|---|---|
| 1 | Sanity-check the instrument on one item before a wave | [field] |
| 2 | Tier the oracle by independence and cap the grade by tier | [field] |
| 3 | Use a stronger oracle only where the weak one is verifiably deficient | [field] |
| 4 | Disclose oracle blind spots through a deterministic check | [field] |
| 5 | Audit the exception set itself | [field] |
| 6 | Give the oracle the tool's scope and kind mapping | [field] |
| 7 | Record the basis and the oracle with every reading | [field] |

## 1. The instrument is part of the result [field]

**Incident.** A first wave of language-extraction readings looked like a verdict on the
extractors. On the second look the bottleneck was the measuring instruments: a tag
generator used as the reference lost class scope on real TypeScript and mis-tagged
several other languages, and harness defects (name normalization, an unmeasurable range,
a file-locator format) scored five of fifteen items below the first level for reasons
that had nothing to do with the tool. The wave cost about 3.8 million tokens for two
proven levels ([orchestrating-agents §2](orchestrating-agents.md#2-triage-before-dispatch-field)).
The fix was to decide, once and in writing, how the instruments work, then re-measure.

## 2. Rules

**Sanity-check before grading.** Push one item through the real harness end to end and
read a sample of disagreements by hand, in both directions (tool-only and oracle-only),
before believing any number. A verified mis-tag is a defect in the oracle; a
disagreement is only a disagreement.

**Tier the oracle by independence, and let the tier cap the grade.**

| Tier | What it is | Cap |
|---|---|---|
| A | an independent, compiler-grade or type-aware index for that language, produced by its own toolchain | none |
| B | a real parser or index dump from another runtime, or a general-purpose tagger with known gaps | none, unless a deficiency is disclosed; then "good" at most |
| C | a walk or query set run by a runtime independent of the tool, but not a full index for the language (a linter's walk, the grammar's own tag queries run elsewhere) | "good" at most |

An instrument that cannot be installed is recorded with why, and the reading keeps the
weaker oracle with its deficiency disclosed and the grade capped at "good". A reading
names the oracle, its version and its tier.

**Escalate only where the weak oracle is verifiably deficient.** "Verifiably" means a
demonstrated mis-tag, not a disagreement. The stronger instrument is a bench tool, never
a product dependency: install it under the measurement scratch directory, and a runtime
the product does not use (a JVM, a .NET SDK, Node) stays out of the product's tree. Where
no index exists for a level (call pairs in a shell dialect, say), run the grammar's
upstream tag queries in a separate runtime and grade at tier C.

**Give the oracle the tool's scope.** Run it over exactly the file list the tool would
read, with the same exclusions (generated, minified, vendored, test fixtures). Restrict
the subjects to the families the tool supports (one SQL reading limited itself to three
dialect families) and name the restriction and the unsupported families as a disclosed
gap.

**Make the kind mapping part of the instrument.** Compare only the kinds both sides
know; exclude the rest from both counts and report the excluded count. A dialect may not
exclude a kind that its own level definition requires; the harness refuses such a
reading.

## 3. Disclose blind spots without inflating the score [field]

When the oracle demonstrably misses real definitions, the tool would be punished as a
false positive for being right. Let a reading carry an adjudicated exception set, under
five conditions.

1. **Deterministic admission only, never a model.** An entry is admitted by a mechanical
   check: the tool emitted that name at exactly that location, inside the compared
   kinds, the oracle matched nothing to it, and the source text contains the name and the
   declared pattern.
2. **A class, a reason and a scope on every entry.** The class states why the oracle
   misses it; the scope is narrow (the consumer ended at module-level definitions).
3. **Relieve precision, never credit recall.** Admitted entries are removed from the
   precision denominator, so they no longer count as unmatched emissions; they never
   enter the recall numerator, so the escape hatch cannot inflate the number that says
   how much of the reference the tool found.
4. **Say so in the reading.** List the admitted entries under their subject, add a note,
   and count and name the claims that were refused.
5. **Audit the exception set with a seeded sample before publishing.**

## 4. Audit the escape hatch itself [field]

**Incident.** The first exception sets were inflated. A seeded sample of 45 admissions
per language found that 69% of sampled TypeScript admissions and 29% of JavaScript
admissions were locals or object-literal keys, exactly the things the oracle excludes on
purpose. Tightening the rule (a named scope, module level only) and re-measuring moved
TypeScript from "excellent" to "good" (symbol precision 0.9946 to 0.9381, admissions 769
to 234) and JavaScript from "excellent" to "unmet" (0.9908 to 0.8368). [reading]

**Rule.** A self-adjudicated exception mechanism is audited by a seeded sample before its
readings are published, stays deterministic, and any change to the oracle or the
exception rule re-grades earlier readings by re-measurement. Do not carry a grade earned
under the old rule.

## 5. Record with every reading [field]

Oracle name, version and tier; the mapping and kind list; the subject and its revision;
the basis of any throughput figure (CPU time, with the wall figure disclosed beside it,
never a mix of bases in one reading; see
[orchestrating-agents §3](orchestrating-agents.md#3-run-the-gate-while-workers-load-the-machine-field));
n; the excluded, admitted and refused counts; the restriction and disclosed gaps; the
date. A reading that omits its oracle cannot be compared with the next one.

## 6. Relation to the rest of the kit

[Practices §21](practices.md#21-self-derived-ground-truth-n1-evidence-and-precision-less-set-grading)
covers counting runs honestly (pass^k, paired intervals, precision beside recall) and why
a system must not grade itself against ground truth it derived. This guide covers the
reference itself: its independence, its blind spots and the exceptions granted for them.
[fast-judgment.md](fast-judgment.md) section 3 applies the same rule to labels: never let
the comparator's own family be the only truth. A reading that survives both is a
measurement; anything less is an estimate and should be graded as one
([changes.md §10](project-intent/changes.md#10-research-and-assessment-records)).

## Checklist

**Before a wave:** one item through the real harness; a hand-read sample of disagreements
in both directions; oracle, tier and scope written down.

**Before publishing a grade:** the tier cap applied; every exception deterministic and
sampled; precision relieved and recall untouched; the oracle named in the reading.

**After changing an instrument:** re-measure, re-grade, and say which readings moved.
