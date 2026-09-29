# The language atlas: the queue, the ladder and what the first waves taught

Created: 2026-09-27 (queue and ladder); updated 2026-09-29 with the round-1 and round-2 lessons;
summarized into the repository 2026-09-28.
Status: a design and lessons note. It cites readings and the generated tracker for every level and
keeps no level table of its own (ADR-054, I54): what a language is claimed at is computed by
`fathom support` from `docs/bench/readings/lang/<id>.json`, and the milestone counts per wave live in
`docs/bench/readings/lang/milestones.md`.
Decisions it informs: ADR-048 (language breadth and depth) with its dated amendments, goal G15,
`docs/vision/37-language-breadth.md`, `docs/vision/38-proof-layer-roadmap.md` section 9.3.

## Verdict

"Supports language X" is now a measured reading and not a claim. A queue of 142 items (77 languages and
65 tool formats), a four-level ladder with two grades, and a harness that computes `level_met` from
metrics and oracle tiers replaced a table of unmeasured estimates. The first waves showed that the
bottleneck moved from the dialects to the **measuring instruments**: an oracle that cannot see a real
definition, or an escape hatch that admits the wrong ones, inflates a reading as surely as a weak
extractor deflates it. The honest position after the D3 audit is that fewer languages are at a level
than the first readings said, and that each level that remains is one a stranger can re-derive.

## The queue (design, 2026-09-27; figures *measured* from the queue file that day)

142 items: 77 languages and 65 tool formats (Dockerfile, Kubernetes, Helm, Terraform, GitHub Actions,
OpenAPI, protobuf, package manifests and more, which also feed the system graph of ADR-050).

| Cut | Counts |
|---|---|
| Waves 1 to 5 | 18, 18, 34, 30, 42 |
| Engine | tags queries 98; document-path 36; native 5; tags plus document 3 |
| Target level | L1 28; L2 51; L3 49; L4 14 |

Each item carries its grammar source, licence, oracle, calibration subjects (real repositories at pinned
commits), `depends_on`, and the worker model (Sonnet). The queue file is
`docs/plans/language-breadth.queue.json`; the plan has one `LB-Q-<id>` row per item.

**Grammar-quality risk policy.** Items whose grammar may not parse real code (VB.NET above 5 percent
parse error, VBA, ABAP, Mojo) get a refused or deferred disposition instead of a forced level: honest
dispositions over inflated coverage.

**Dialect packs deferred.** Shipping dialects as packs outside the binary (LB-F14, LB-F27) is deferred
while dialects stay in the binary; the support matrix tracks the grammar blob size against a 40 MiB
cap (ADR-048 decision 5, ADR-054).

## The ladder

| Level | Meaning | Oracle |
|---|---|---|
| L1 Symbols | Every definition: name, kind, package, exact line range, and a signature for callables | universal-ctags (tier B) or SCIP definitions (tier A) |
| L2 References and calls | Every call site inside a definition matched to the callee by name | SCIP references (A) or the grammar's own upstream tags query (C) |
| L3 Import resolution | Imports resolved to the file they point at, so a call across files becomes an edge | SCIP cross-file references (A) |
| L4 Typed edges | Edges resolved by understanding types, natively or by importing a SCIP index | SCIP (A) or a native checker |

Each level is graded excellent or good; thresholds are in `docs/design/language-breadth.md` section 1.2.
Tool formats use L1 to L3 and never claim L4. An unmeasurable metric is `not_measured`, never a failure,
and caps the level at good. A tier-D oracle (a grep proxy or an author-labelled sample) never proves
excellence (I54).

### Milestones: what was decided and which figures stand

The owner's 2026-09-27 memo (note N5) asked to raise the ladder and gave indicative figures for M2
(40 languages at L1, 25 at L2, 15 at L3, 10 at L4, 40 tool formats) and M3 (the full 142-item queue).
Those figures are **not used**: vision 37 section 5 already defined M2 and M3 and they stand, as named
in vision 38 section 9.3 and goal G15.

| Milestone | Languages | Tool formats |
|---|---|---|
| M1 (after waves 1 and 2) | 20 at L1 excellent, 12 at L2, 8 at L3, 6 at L4 | 15 at target |
| M2 (after waves 3 to 5) | 45 at L1 (35 excellent), 25 at L2, 14 at L3, 10 at L4 | 30 |
| M3 (backlog drained) | 60 at L1, 35 at L2, 20 at L3, 12 at L4 | 40 |

## The baseline (2026-09-27)

17 languages and 15 tool formats had some level in the old table and 110 of 142 queue items sat at L0,
but **every level was an unmeasured estimate** with no committed reading: only Go (and partly SQL) filled
`Signature`, and call-edge recall was 0.03 for TypeScript and 0.12 for Python (CV-M7). The first honest
figures came with the first readings; see `milestones.md`, not this note.

Foundation rows first: 5 of 29 were done at the first count (the reading schema and `LevelMet`, the
`lang-probe` tool, calibration shards, `LoadQueue` with a queue and plan consistency test, and the CUE
dialect manifest); the harness reached 25 of 27 (29 minus the two deferred dialect-pack rows) before the
second wave of workers.

## Lessons from round 1: triage before dispatch (2026-09-28)

Wave 1 launched 15 items at once. Nine were blocked on one unbuilt foundation row (LB-F18) and five more
scored below L1 on harness defects (name normalization, an unmeasurable range, a TOML locator): about
3.8 million tokens bought two proven levels. The rule that came out of it: **run a cheap `depends_on`
check (every dependency is `[x]` or `[-]`) plus a harness sanity check on one item, and fix foundation
gaps with one Opus worker first, before dispatching a wave.** The loop of `docs/design/language-breadth.md`
section 15 already dispatches only entries whose dependencies are done; the sanity check on one item is
the addition.

## Lessons from round 2: the instruments (owner decisions D1 to D7, 2026-09-28)

Wave 1b showed the bottleneck was now the measuring instruments: ctags loses class scope on real
TypeScript and mis-tags in other languages. Seven owner decisions bind the harness and every later
reading (recorded in ADR-048's dated amendment and `docs/design/language-breadth.md` section 1.5):

| # | Decision | Why |
|---|---|---|
| D1 | Throughput is **CPU time** (bytes per CPU-second of the extraction process, via getrusage), not wall clock; readings record `throughput_basis` and disclose wall throughput | readings taken while other workers run must not fail on contention |
| D2 | A stronger oracle where ctags is verifiably deficient: TypeScript and JavaScript against scip-typescript, Java against scip-java or a JavaParser dump, C# against scip-dotnet or a Roslyn dump, SQL restricted to supported families with disclosed gaps; JVM, .NET and Node tools are bench instruments only, never product dependencies | an instrument that cannot be installed is recorded with why and L1 caps at good |
| D3 | **Oracle-deficiency disclosure**: an adjudicated set of dialect-only definitions the oracle verifiably misses, admitted only by a deterministic check (`langbench.Adjudicate`, never a model), leaving `sym_precision`'s denominator and never recall's numerator | an honest way to credit a dialect the oracle cannot see |
| D4 | Cross-file joins for tool formats: repository-root and Kubernetes label-selector joins, GitHub Actions cross references, a local image tag resolved to the Dockerfile that builds it | lifts L3 for infrastructure dialects |
| D5 | An L2 calls oracle for every dialect: where no SCIP producer exists (bash, SQL), call pairs come from the grammar's upstream tags query run by an independent runtime at tier C (L2 good at most); a producer with no `enclosing_range` no longer zeroes calls | every dialect gets an independent L2 oracle |
| D6 | A queue dependency is met at L1 once the parent has a committed reading at L1 or better, not only when its row is `[x]` | parent rows can stay open for L4 for months |
| D7 | C# `using X; Y.Z();` resolves (an unaliased using is a wildcard import), and Java's `import a.*; Y.z()` resolves the same way | found by the round-2 re-measurement |

**The D3 audit (2026-09-28) is the lesson.** A seeded sample of 45 admitted entries per language found
the first D3 sets inflated: 69 percent of sampled TypeScript admissions and 29 percent of JavaScript
admissions were locals or object-literal keys the oracle deliberately excludes. The admission rule was
tightened (a class must name a scope, and `module` is the only one) and both readings re-measured.
TypeScript moved from L1 excellent to good (`sym_precision` 0.9946 to 0.9381, 769 admitted entries down
to 234) and JavaScript from L1 excellent to unmet (0.9908 to 0.8368). A re-audit of the tightened sets
(seed 7, 40 per language) found 0 of 40 invalid for TypeScript and 1 of 40 for JavaScript. The reading
is `docs/bench/readings/lang/d3-audit.md`. **A self-adjudicated escape hatch must itself be audited and
must stay deterministic.**

## The worker recipe

Sonnet workers, at most two at once; results merged serially into the integration line; `task validate`
per wave; pushed through the private-clone gate; the next eligible items chosen by `indexbench lang-next`
after triage. Foundation rows and triage are Opus work; queue items are Sonnet, one fresh worker each,
with a checked recipe (`docs/design/language-breadth.md` section 13).

## What it drove

| Decision | Where |
|---|---|
| Language breadth as a measured program, not a claim | ADR-048; G15; `docs/plans/language-breadth.md` |
| The ladder's milestones (vision 37 section 5, not the memo's figures) | vision 38 section 9.3 |
| Support levels as a computed projection | ADR-054; I54 |
| Owner decisions D1 to D7 | ADR-048 amendment 2026-09-28 |
| Milestone counts per wave | `docs/bench/readings/lang/milestones.md` |
| Triage before dispatch | this note, beside the loop of `docs/design/language-breadth.md` section 15 |

## What this does not claim

That any language is at any level (the readings and `fathom support` say that); that M1, M2 or M3 is
close (the generated tracker line says so); that ctags, SCIP producers or tags queries are equally good
oracles (D2 and D5 say they are not); that the queue is final (an Opus triage re-targets, re-pins or
re-orders it and records why).

## Sources

`docs/plans/language-breadth.queue.json`, `docs/design/language-breadth.md`, `docs/languages.md` (the
generated tracker), and `docs/bench/readings/lang/` (the committed readings, `d3-audit.md`,
`milestones.md`).
