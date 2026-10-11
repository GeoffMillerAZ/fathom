# 42 - Every file accounted for: the repository graph, and the joins that compound it

Status: accepted 2026-10-10 (ADR-067, owner-architect under the owner's delegation, revised the same day after an
adversarial review, with the rulings ADR-067 records: W3 per subject, the guard on `runs` and `configures` from their
first row, I8 and ADR-023 amended, ADR-063 O11). Owner notes of 2026-10-10 in two messages (N1 to N11), with the instruction that nothing is
built because the owner thought of it: gate for excellence, reframe freely, think bigger. Plan:
`docs/plans/repository-facts.md` (RF-), plus rows merged into test-intelligence (TQ-), test-certifications (TC-),
reach (RH-), judgment-foundations (JF-), governance-learning (GL-), exploration-maps (XM-), served-not-kept (PR-),
feature-map (FM-), incident-maps (IM-M0 amended), telemetry-links (TL-) and language-breadth (LB-Q, LB-F). Goal G36
in `docs/intent/scope.md`. Futures F68 (reference maps) and F69 (judged sites offered at onboarding). F37 amended.
Invariant I8 amended, I57 extended to call edges, I65 stated. ADR-023 and ADR-055 amended, ADR-063 gains O11.
Builds on semantic layers (ADR-016), test certifications (ADR-043), conservation (ADR-044), language breadth
(ADR-048), the kitchen-sink gate and the table freeze (ADR-049 decisions 1 and 2), the system graph (ADR-050), the
feature map and the review family (ADR-055), judgment at the port and exploration maps (ADR-056), stewardship
(ADR-060), tech packs (ADR-061, proposed), telemetry links (ADR-062) and kit overlaps (ADR-063). The owner's note on
portable evidence export (N12, same day) is a separate design: vision 43 and ADR-068.

## 1. The question

The owner asked, on 2026-10-10:

- **N1.** The highest tier possible and the highest quality practical for the very common web and UI technologies
  (JavaScript, TypeScript, HTML, CSS "and that sort of stuff"), and for shell scripting, PowerShell and the common
  scripting languages.
- **N2.** Should fathom map which config files belong to what, and associate script files with the concepts they
  operate on? Maybe it already does.
- **N3.** A standard semantic layer common to every repository: one that always describes tests, one for scripts and
  utilities, one for configuration. Onboarding could optionally populate them, showing token estimates first, and
  use decision engines like Jev as much as possible to stay budget friendly.
- **N4.** Would fathom be valuable mapping features, business capabilities or other concepts to the code files that
  touch them?
- **N5.** Do not implement now. Put what survives on the roadmap durably, prioritize, and plot merges with existing
  features as fathom's place becomes clearer.
- **N6.** Reference implementations: link researched external repositories to this project's specifications, so
  later work finds existing solutions without redoing the research. A special kind of map, a new feature, or not
  worth it?
- **N7.** Compounding: the feature set has grown a lot. Where do features synergize in ways not yet thought of, for
  token efficiency, correctness, exploration, agent coding capability, expert systems?
- **N8.** Operational excellence: fathom will map well-architected reviews and help prevent incidents by finding the
  configuration or code gaps that cause outages. Could semantic layers track critical concerns such as memory
  management, or Kubernetes practice (init containers, resource limits and so on)? Does that cross into Review Kit's
  domain, or is there a synergy?
- **N9.** Opt-in or default: probably opt-in, unless cost says otherwise. If Jev or other cheap decision APIs, and
  agent-driven decision agents that generate the decision-API inputs, can populate these layers for much less, some
  could become standard (opt-out).
- **N10.** Adherence and superpowers: other ways fathom can drive excellence, adoption and adherence just by making
  the coding agent aware and more capable. Dependency mapping was discussed: is it roadmapped or decided?
- **N11.** Use all of this, plus intuition, to expand or upgrade the vision, goals, features and capabilities.

## 2. Before designing: what applies

- *Non-goals* (`docs/intent/scope.md`): no domain connectors, no dispatch loop (fathom never watches, schedules,
  spawns or retries), no transformation engine, no hosted backend, no incident command center, no net tool growth
  (law 4).
- *Principles*: P15 (fathom writes only what it owns), P17 (state is computed), P19 and P23 (a request pays for its
  answer, no polling), P20 (a named consumer before a build), P21 (split on an enforced boundary only), P22
  (measured, not argued), P24 (the decision-model bet: a model chooses among candidates, it never asserts a fact).
- *Invariants*: I8 (`fathom verify` is static, `runtime_pass` always Unknown, amended by decision D16), I12 (the
  SessionStart primer is at most 3,200 bytes), I53 (a hook's deadline sits under its descriptor's timeout), I42
  (unconfigured output is byte-identical with provider none), I56 (fathom never downloads on its own authority), I57
  (a cross-boundary link below the typed tier never gates, extended to call edges by D3), I62 (a judgment is never a
  fact, a gate or a silent prune), I64 (a repository's config chooses behaviour only). The edit hook's own bound is
  not I12: it is the 320-token payload ceiling (`specLineTokenBudget`, `cmd/fathom/edithook.go:29`, ADR-016 Q21).
- *Decisions*: ADR-016 (a layer compiles a document corpus into cited claims at `doc-claimed` tier), ADR-023
  (verify is explicitly static and never asserts `runtime_pass`, amended by D16), ADR-041 ("earn the place": a
  consumer, an eval, a gate), ADR-049 decision 1 (every item names the letter it serves and its instrument) and
  decision 2 (no new table, column, cache or background job until RT-M1 reads PASS), ADR-050 (one contract-key link
  model, the `pkg:` kind, dependencies from local caches), ADR-051 decision 1 (ingested coverage is read and joined
  at request time, never stored), ADR-055 decisions 4, 7 and 8 (feature verdicts are human, rule content and
  dispositions stay in Review Kit), ADR-056 R1 to R8 (R1: a model never decides what is computable, every judged
  site opt-in per repository, J0 before any model reads code), ADR-061 (tech packs, proposed), ADR-062 (telemetry
  links, Stage B gated on census E), ADR-063 O1, O2, O4 and O6 (ci-kit owns the package closure, Review Kit judges
  over fathom's facts through its bridge, test-kit envelopes are the test-fact source, generated files carry
  provenance), ADR-064 (an interim fact lives in its own file's observations).
- *Owner rulings*: K15 (universal concepts in core, specifics in packs), K19 (the Stage A label-join kill stays fired,
  and a re-opened reading needs a rule committed before any holdout reading), K22 (fathom's automatic passes never
  fetch or run a toolchain a repository names), K23 (language order follows the queue's first-ranked languages) and
  K26 (a change over +20% store or +15% wall ships only with a recorded owner ruling). K22, K23 and K26 were recorded
  only in an operator note until now. The language lane commits them in `docs/design/language-breadth.md` section
  1.8 (LB-F455), since only that lane edits the language-breadth files.
- *Records outside fathom that bind section 6*: platform-engineering ADR-0095 decision 5 ("A resource-shaped
  requirement is refused at vet unless it joins policy-kit by canonId"), platform-engineering ADR-0015 (policy-kit:
  CUE-authored, CEL-first policy, unbuilt), Review Kit future F5 ("Do not build: infrastructure scanners or a second
  resource-policy evaluator").
- *Refusal already on the record*: JP-X2 refuses computable labels as model work: generated or vendored files,
  config versus code, public API, ownership, I/O reach.
- *Autonomy*: the bench and model spend hold of 2026-09-29 is in force (`docs/intent/autonomy.md`). A spending
  compile and enabling a decision-port site are ask-first. $0 replays, fixtures and scratch stores go on.
- *Futures*: F7 (the `ai-derived` tier), F37 (judged semantic columns), F50 (live maps), F52 (concern packs scoped
  by live-map queries), F59 (promises as code), F61 (runtime topology), F64 (pack-carried maps), F66 (telemetry name
  joins). None has fired. Census E reads .1451 against F66's .25 bar.
- *Fired triggers*: none. No Revisit-when, Promote-when or Drop-if condition in the records above has fired.
- *Record defects found while reading*, each corrected in the same change as this record:
  1. ADR-055 decision 8(c) says Review Kit concern packs "compile into fathom obligations at the edit (packs already
     exist)", and its Consequences bullet repeats it ("its concern packs continue compiling into fathom obligations
     at the edit"). No such compiler exists in either repository. The only path to the edit is the findings outbox
     import. Both places are amended.
  2. JF-1 and GL-10 are `[x]`, yet `~/.fathom/rule-outcomes.jsonl` holds 67 rows, the last written 2026-10-01T04:14Z
     by build fa1e1d16, still under the old event names (`complied`, not JF-1's `top_changed`). Their contracts pass
     in tests and the deployed writer is silent: a done-means-wired gap, recorded through `intent-incident` as
     INC-27. The first suspect is PR-11, still open: a rule served by the pre hook does not write its `served` row on
     the paired post call.
  3. KR-12 is `[x]` while the row it needs, KR-11, is `[ ]` with `needs: SG-37`, though `internal/docindex/depdocs.go`
     ships with tests. One of the two rows is wrong. At landing `fathom verify --strict` read KR-11 library-only
     (nothing in the product calls `IngestDepDocs`), so KR-12 was unchecked.
- *Corrected premises*:
  1. "Standard semantic layers" (N3): a layer is a compiler over documents. Tests, scripts and configuration are
     file facts a rule computes. The word fits the owner's intent and not the mechanism.
  2. "A CI evidence tier" (the 2C program candidate): test evidence ingest exists (`fathom tests ingest`, ADR-043,
     stored in `test_runs`). What is missing is the join: verify's `runtime_pass` is hard-coded Unknown
     (`internal/verify/verify.go:930`). Wiring it changes I8 and ADR-023, so it is an amendment, not a join alone
     (D16).
  3. Dependency mapping (N10) is decided and roadmapped, and has shipped nothing an agent can call (section 7).

## 3. The frame: four existing kinds, one repository graph

Every note but N1 asks for a typed association between units: a file and its role, a script and what runs it, a
config and what it governs, a feature and its code, a specification and a reference implementation, a workload and
the rule it must keep. Fathom already holds four kinds of record for associations, and each new idea picks one:

| Kind | What it holds | Who writes it | Freshness | Examples |
|---|---|---|---|---|
| **Facts** | edges and observations a rule computes | the ingest | the content hash, at ingest | call edges, route-ref, image-ref, log templates |
| **Claims** | what a governing document says, cited | the layer compiler | citation hashes, swept | obligations, ADR rules, spec bindings |
| **Maps** | what an agent learned matters, anchored | the session, at forced moments | anchor hashes, re-resolved | exploration maps, handoff maps |
| **Attested records** | what a producer outside the index attested, ingested or read as written | the producer (a runner, a kit, a hook) | the producer's commit binding and age | test-kit envelopes and `test_runs` (ADR-063 O4, ADR-043), coverage reports read at request time and never stored (ADR-051 decision 1, `cmd/fathom/coverage_ingest.go`), the findings outbox (ADR-040), the rule-outcome, session and hook ledgers |

**Rule: no new container.** A file's role is a fact. A rule about tests is a claim. "This upstream repository
implements our spec FR-7 well" is a map entry. A test run, a coverage report or an imported finding is an attested
record. A judged label stays inside the decision port until F7 gives it a tier.

### 3.1 What the measured state says

| Reading | Evidence |
|---|---|
| Fathom's proof verbs are Go-shaped | `wired` reads n/a for any non-Go symbol (`evalWired`, `internal/verify/verify.go:973`), product entry points are Go `main` and `init` only (`internal/analysis/callgraph/product.go`), a test is a symbol named `Test*` (`internal/analysis/impact/impact.go:171`), `testlayer` returns "" for anything but `_test.go`, the evidence join reads only Go `Test*` functions (`JoinCasesToSymbols`, `internal/testevidence/join.go:62`), and the `certifies:` marker is Go-only (`internal/extractor/certifiesmarker.go`, TC-26 deferred) |
| Non-code files are invisible to impact | `blast_radius` on `scripts/gate-lease.sh` returns 0 symbols (index fb7d9f3f). The task that runs it is indexed as the TaskfileTask `validate` (`Taskfile.yml:253`, `cmds: ["bash scripts/gate-lease.sh task validate:unleased"]`). An earlier probe that read "0 callers" looked up `Taskfile.validate`, got `def: null` and is not evidence |
| The run rule exists and is used twice | `ghaRunRefs` (`internal/fusion/gharun.go:248`) has two callers: GitHub Actions `run:` steps and Jenkins `sh` steps. It knows four command kinds (make, task, npm, script) and resolves a script only if the script defines a function |
| Eight rules decide "is this a test", two decide "is this vendored" | test: `isTestPath` (`internal/mcp/search_tool.go:1150`), `isTestSymbol` and `isTestFile` (`internal/certify/risk.go:79`, `:92`), `isTestFunc` (`internal/certify/portfolio.go:100`), `looksLikeTestFile` (`internal/conserve/plane_integrity.go:126`), `isTestPath` (`internal/analysis/routes/laravel.go:1409`), the `_test.go` gate of `ClassifyWith` (`internal/testlayer/classify.go:30`), the `Test` prefix in impact (`internal/analysis/impact/impact.go:171`). Vendored: `searchVendorSegments` (`internal/mcp/search_tool.go:128`), `vendoredPath` (`internal/custodian/audit.go:723`). Beside them sit about twenty directory-walk skip lists (`connector.go:471`, `intent/audit.go:523`, `conserve/covprofile.go:230` and others), two tool-semantics copies (`tsresolve.go:610`, tsc's default excludes, and `isTestVariant`, `typedcalls.go:1297`) and two bench graders (`contextbench/publicloc/patch.go:209`, `scenariobench/readings_falsedone.go:57`) |
| tsconfig is computed and thrown away | `internal/fusion/tsresolve.go` reads include, exclude, paths, extends and references at resolve time and stores none of it |
| Delivery is unexplained, not proven leaky | from 2026-10-03 to 2026-10-10 18:00Z the edit hook answered 3,367 file-target calls and emitted on 78 (2.3%), and on 5 of 963 Edit and Write calls. Whether that is a leak is unknown: 21,247 hook answers since 2026-10-03 carry no reason (GL-24 makes them record one). 1,704 post-bash writes (51%) were silent as `foreign-target`, a class that mixes worker clones, gitignored drops (`dropIgnoredWriteTargets`, `cmd/fathom/edithook_ignored.go:214`) and scratch writes (this design session's own `/private/tmp` writes among them), and those rows log no target path. The pin line skipped 744 of 745 checks as `no_summary` in its lifetime. The rule-outcome writer has written nothing since 2026-10-01 |
| A computed fact goes unread | no caller sets `RiskOptions.Coverage` (`internal/certify/portfolio.go:123`, `internal/mcp/analysis_tools.go:252`, `cmd/fathom/console_tests.go:74`), so every risk rank says "treated as uncovered". `localize` logs a zero-yield recency hop saying no churn is wired, while 180-day `FileChurn` (TQ-1) sits in the store. Churn is not recency, so that join waits on IM-M0 (J6) |
| `verify` reach is tier-blind | `reachable_from` passes through any edge, and `wired` gates `--strict` (`internal/verify/verify.go:898-909`). I57 is enforced for syslink only (`TestHeuristicLinksNeverGate`, `internal/syslink/link_test.go:77`). A new heuristic edge kind would gate today |
| Sibling agents re-derive | sibling re-reads are 69.9% of sub-agent reads, 15.90M tokens in 14 days (`docs/bench/readings/exploration-maps/rederivation.md`) |
| The governing rule misses the first touch | served at first touch on 3 of 8 guards tasks (VB-40) |

Two conclusions follow. First, the most valuable new capability is not a new layer: it is making the proof verbs
that already exist work on every file of every repository, not just on Go symbols. Second, delivery comes before
content: the edit hook's silences carry no reason, the rule-outcome writer is dark and the pin line is blind, so no
added content could be read for effect until those channels say what they do.

### 3.2 The repository graph

Fathom today is a symbol graph with documents attached. The upgrade is a repository graph: **every indexed file
answers five questions deterministically, at $0, through the verbs agents already call**:

1. *What is it?* Its role: source, test, fixture, script, config, ci, build, iac, schema, migration, docs,
   generated, vendored, or unknown, with the basis that decided it.
2. *What runs it?* The task, CI job, hook, package script or container command whose command line names it.
3. *What configures it?* The config files whose declared scope covers it (tsconfig, `.editorconfig`, workspace
   manifests, and pack-declared tool configs later).
4. *What proves it?* The tests that reach it, now for every language with test entries, and the certifying tests
   (Go today, other languages once TC-26's trigger fires).
5. *What governs it?* The rules served at its edit (existing).

A sixth answer comes from maps: what earlier sessions learned about it, and what reference implementation exists
for the specification it implements (F68).

Expert knowledge stays with its owner. Resource rules belong to policy-kit, review judgment belongs to Review Kit.
Fathom supplies the facts and the membership those rules select over, delivers their findings at the edit, and
proves a fix through certifying tests. Judgment applies only to the residue a rule counts as unknown.

## 4. The decisions

### D1. Four existing kinds of record, and no new container (N11)
Adopt. Section 3's table is the admission rule for every association the notes raise. Reason: it keeps the
kitchen-sink gate honest (ADR-049 decision 1) and stops a "semantic map" store from growing beside the existing ones.
The earlier draft's "three containers, never a fourth" is withdrawn: attested records were already a fourth kind.

### D2. The role facet (N2, N3): adopt, one classifier, computed on read
Amended 2026-10-11 (ADR-067 amendment): RF-M1 and its one re-read RF-M8 read CUT, so the classifier below and its
four declared bases were deleted while still unwired (RF-1, RF-3 and RF-4 refused). Only the unification stays: one
test and vendor rule in `internal/repofacts` (RF-2).
- One pure classifier, `repofacts.Role(path, dialect, attributes, header)`, with a closed vocabulary (section 3.2)
  and a basis order: `attribute` (`.gitattributes` `linguist-generated`, `linguist-vendored`,
  `linguist-documentation`, root and nested) over `header` (Go's `// Code generated ... DO NOT EDIT.` line and the
  ADR-063 O6 provenance header a kit writes on a generated file) over `dialect` (the file's dialect id) over
  `linguist` (Linguist's `vendor.yml`, `documentation.yml` and generated detectors, imported as pinned data the way
  LB-F48 and LB-F49 imported its languages) over `convention` (the core table, plus dialect-pack data).
- **Unify first.** The eight test rules and two vendor classifiers of section 3.1 call the classifier before any new
  consumer reads it, or the facet would be an eleventh divergent copy.
- **Out of scope, named**: directory-walk skip lists (they decide what to walk, not what a file is), copies of a
  tool's own semantics (tsc's default excludes, go/packages' test variants), and the bench graders (contextbench and
  scenariobench), which stay independent of the subject they grade.
- Computed on read: no `file_stats` column while the table freeze holds. A column is considered only after RT-M1
  reads PASS and a consumer measures the cost of filtering without one.
- `unknown` is a value, counted and printed, never hidden.
- Core: the universal convention table (K15). Framework conventions (`__tests__`, `*Tests.cs`, `spec/`) are dialect
  pack data. The convention table imports none of Linguist's data.
- Served through what exists: `search` hit flags, `symbol_context` on a file path, and the passport (D10).
- Graded honestly. The `attribute`, `header`, `dialect` and `linguist` bases are their source's own answer and claim
  no reading. Grading them against `github-linguist` would read 1 by construction. RF-M1 grades convention roles
  only, and only the one an independent oracle attests: `test`, against each runner's own collection (`go list
  -test`, `pytest --collect-only`, `jest --listTests`, `vitest list`, PHPUnit `--list-tests`). The other convention
  roles (fixture, script, config, build, migration) are served with basis `convention`, feed no rank and no gate, and
  get a reading only when a consumer proposes to read one for a decision (RF-D8).

### D3. `runs` edges (N2): adopt, as new callers of the run rule, after a guard
- **Guarded from the first row.** Verify's `reachable_from` and `wired` never pass on a `runs` or `configures` link,
  and I57 is extended from cross-boundary links to call edges, enforced by a test (RF-6). Today reach is tier-blind
  and `wired` gates `--strict`, so an unguarded heuristic `runs` edge would gate on arrival. The guard over every
  other heuristic call edge in the fleet (RF-5) waits on RF-M6, which counts, at $0, the proven rows across the
  fleet's plans whose reach passes only through a heuristic edge. The count is published whatever it reads (the
  owner-architect's ruling of 2026-10-10), and a nonzero count ships RF-5 with a `### Breaking` changelog line and a
  migration note naming the rows.
- `ghaRunRefs` gains callers: Taskfile `cmds`, `package.json` script bodies, composer `scripts`, Makefile recipes,
  lefthook and pre-commit entries, GitLab CI `script:`, Dockerfile `RUN`, `ENTRYPOINT` and `CMD`, and a Kubernetes
  container's `command` and `args`.
- New command words: `go run` and `go test` (to a main package or a test package), `node`, `python` and
  `python -m`, `npx` and `tsx`, `php` and `vendor/bin/*`, `pwsh -File`, `bash -c` (recursing once).
- A script with no function gets a file-level module node (the TypeScript `<module>` precedent), so it resolves.
- Stored like the GitHub Actions edges it extends: heuristic tier, in the existing edge tables, labelled. No new
  table.
- `blast_radius` and `test_impact` follow these edges and say so. "What breaks if I edit gate-lease.sh" gets an
  answer: the TaskfileTask `validate` that runs it, and the lefthook `validate` command that runs that task.
- Graded against each tool's own expansion (`make -n`, `task --dry`, `npm pkg get scripts`): a command kind under
  0.95 precision is dropped.

### D4. `configures` facts (N2): adopt, declared scope stored once, membership at read time
- A config file's declared scope is stored as an observation of its own file (the ADR-064 precedent): tsconfig
  `include`, `files`, `exclude`, `extends` and `references` (already computed by `tsresolve`), `.editorconfig`
  sections, workspace members, Dockerfile `COPY` sources, Nx and Turborepo inputs where present.
- Membership ("which config governs this file") is computed at read time by nearest-ancestor resolution, never
  stored as one edge per governed file. Reason: a root tsconfig governs thousands of files and the store budget
  (K26) would pay for each.
- Every `configures` answer carries the existing resolution tier `heuristic` (`internal/core/resolution_tier.go`).
  No `declared` tier is added: none exists, and a new tier would need its own record and its own gate semantics.
- eslint, jest, vite, webpack, babel and prettier configs are tech-pack data after ADR-061 is accepted, not core.
- The env and config provide-and-consume links stay system-graph SG-20 and SG-21, unchanged, behind SG-2.
- Graded against `tsc --listFilesOnly -p` (tier A) on three subjects: more than 2% disagreement keeps tsconfig at
  resolve-time use only.

### D5. Non-Go entry points and test entries (N10, N11): adopt, informational first
- Product entry points for `wired` beyond Go: `package.json` `bin`, `main` and `exports`, composer `bin`, pyproject
  `[project.scripts]` `module:func`, Python `if __name__ == "__main__"`, Cargo `[[bin]]` and `src/main.rs`, Java
  `public static void main`, C# `Main` and top-level statements, and any script a `runs` edge reaches from CI or a
  task.
- **Not breaking on arrival.** Non-Go rows read n/a today. Moving them to a verdict would turn many into
  library-only Unknown, which the `--strict` pre-push refuses. So a non-Go `wired` verdict ships informational: it
  is printed and never changes a row's status (RF-10). RF-M7 then counts, at $0, the fleet rows it would flip.
  Gating follows only in a later change that carries that count, a `### Breaking` line and a migration note (RF-D7).
- Test entries beyond Go: pytest `test_*` functions and `Test*` classes, jest, vitest and mocha `it`, `test` and
  `describe` blocks, JUnit `@Test`, xUnit `[Fact]` and `[Theory]`, NUnit `[Test]`, PHPUnit `test*` methods and
  `#[Test]` attributes, Rust `#[test]`. They feed `test_impact`, `related_tests` and `tests select`.
- An ingested JUnit or test-kit envelope joins them through the existing join: `JoinCasesToSymbols`
  (`internal/testevidence/join.go:62`, TC-3) is extended past `KindGoFunc`, never duplicated. If a non-Go service
  then ingests an envelope, TC-26's trigger fires and is recorded. Until TC-26 lands, a non-Go test reaches and is
  selected, but it cannot certify a rule.
- Reason: the languages K23 puts first (TypeScript, JavaScript, Python, Java and PHP), and C#, which the queue ranks
  above Go, get no verdict from the proof verbs today. A proof layer whose proof verbs answer only for Go is not yet a
  proof layer for them.

### D6. "Standard layers" for tests, scripts and config (N3): reject as compiled layers, merge
- Tests are D2 plus D5 plus test intelligence. Scripts are D2 plus D3. Configuration is D2 plus D4. Each is a
  deterministic facet served by existing verbs.
- Reasons: ADR-056 R1 (a model never decides what is computable) and JP-X2 refuse computable labels as model work,
  a layer's tier would be a lie (`doc-claimed` cites a document, these cite none), and the facets cost $0 where a
  compiled pass costs per repository.
- The model-shaped residue is purpose text and concept tags for a file, and test kind beyond what signatures and
  build tags decide. It merges into F37 (amended), which already holds "the residue no computation labels". It is
  promoted only when J0 passes, an ADR promotes F7, a named consumer exists and the cost estimator (JF-25) exists.
- Invariant I65 adds one clause to ADR-056 R1: the residue a file-fact rule prints as `unknown` is counted in every
  reading and answer that uses the facet. R1 already says the rest.

### D7. Onboarding with an estimate first (N3, N9): a priced section in `fathom init --dry-run`
- No new flag. RH-11 (`fathom init --dry-run`, open in `docs/plans/reach.md`) already lists what init would write
  and writes nothing. RH-13 adds a priced section to that output: the deterministic facets first ($0, on), then each
  compile layer priced as a range with IO-8's band (`dryRunOutRatioLow` and `dryRunOutRatioHigh`, 2 to 8 times the
  input, `internal/semantic/compile.go:231`, set after O8 once under-reported a compile 25 times), then each
  decision-port site as "unpriced" until JF-25 exists. JF-25, when built, prices through `internal/cost`
  (`loadPriceTable`, PE-16) and never a second price table.
- Consent is per line, and each line prints the command that enables it. `init` without `--dry-run` is unchanged.
- Reason: the owner asked for estimates before spend, and the O8 history plus the autonomy rule forbid an onboarding
  that spends by itself.

### D8. Features and business capabilities (N4): merge into the feature map
- ADR-055 already decided the feature map: authored features, computed facts, human verdicts. Role, run-by and
  configured-by join FM-4's column list (`ComputeFeatureFacts`), no new row. Co-change clusters from stored churn
  (TQ-1) become an FM-3 candidate source, proposed and never written (FM-15, the CodeScene precedent).
- Business-capability joins to tickets, CRM or revenue stay refused (FM-X1 to FM-X3).
- No sequencing change now. The feature map waits on SG-2, which waits on RT-M1. If RT-M1 still reads FAIL on
  2026-11-15, the feature map's first slice is reconsidered on the ADR-044 inventory keys (`route:`, `cli:`,
  `env:`), which exist today.

### D9. Reference implementations (N6): a map kind, deferred behind a $0 baseline (F68)
- Not a new feature. A reference map is an exploration map whose anchors point into another service, ingested from a
  clone the operator or agent made, flagged `reference` so it stays out of fleet health, pinned at that service's
  commit, carrying the licence from a pin manifest that reuses the calibration corpus shard schema (url, ref, sha,
  licence), with spec IDs in its purpose line. Its links are advisory and never gate.
- The licence is served with every anchor, and a non-OSI or copyleft source is marked "read, do not copy".
- Fathom never fetches (I56): the agent clones, fathom pins, reads and links.
- What exists today: `ref/README.md` is the hand-built version (unpinned shallow clones, no spec link), the
  calibration corpus is a curated set of pinned, licensed reference implementations that the language program
  already uses, and `search kind:docs pkg:` reads version-exact dependency docs once something in the product writes
  them (KR-11).
- The instrument is a $0 transcript scan, not the session ledger (`session-ledger.jsonl` carries no URL or clone
  field): the `git clone` and `gh repo clone` commands and the WebFetch calls to `github.com` URLs in session
  transcripts, grouped by upstream repository. Built only when one upstream was researched in at least 3 sessions
  within 30 days, across at least 2 repositories, or the owner names a consumer. Its first consumer is likely
  fathom's own language program, whose workers research the same oracles and parsers wave after wave.
- Free option, a Preserve line only: a map entry keeps its service for both anchor forms. Today a span anchor's
  `Entry.Path` is repository-relative with no service (`internal/explmap/model.go`), and a symbol anchor's dedup key
  passes through `NormalizeSymbol`, which drops the `svc::` qualifier (`internal/explmap/anchor.go:92`). A fork's
  identical code would collide on dedup and re-resolve into the wrong service.

### D10. Compounding and adherence (N7, N10): fix delivery before content, then the passport
- The joins in section 5 are adopted in that order. The first five are XS to S, repair delivery or wire a computed
  fact to its unread consumer, and need no new engine.
- The file passport comes after them. It extends SR-60's pre-edit line (`editHookBlastLine`, already printing "N
  callers, M tests reach this") at a session's first edit of a file with role, run by, configured by and certifying
  tests, plus map and reference pointers, deduplicated per session. It lives inside the edit hook's 320-token
  payload ceiling (ADR-016 Q21), takes at most 60 tokens of it, is dropped before spec lines when the budget runs
  short, and never displaces a rule (a test holds that). `symbol_context` on a file path answers the same block. No
  new MCP tool (ADR-056 R6).
- Its reading has a base rate. RF-M4 first replays 7 pre-ship days to compute how often a session later used an
  item the passport would have named, then reads 7 post-ship days. KEEP needs a lift over that base rate, not an
  absolute share alone.
- Reason: the passport carries D2 to D5 to the moment of the edit, the only moment fathom's delivery is guaranteed.
  It is built last because its effect cannot be read while the hook's silences carry no reason.
- **The general case comes before any adherence claim.** Every delivery number in this design comes from the
  owner's traffic, much of it waves editing clones. The general case is a single repository edited where it is
  checked out, and git worktrees. XM-M8 and RF-M4 report that stratum separately, and no adherence claim is made
  until it holds on its own with at least 100 first touches.

### D11. Operational excellence (N8): facts and membership in fathom, rules with their owners
Section 6 holds the boundary. In one line: fathom supplies the facts that make an ops rule checkable, the membership
of a workload in the policies and monitors that select it, delivery at the edit and proof by certifying tests.
Resource-shaped rules belong to policy-kit (platform-engineering ADR-0015) or, until it ships, the upstream Kyverno
policy library a repository already applies. Review Kit owns review judgment, severity and dispositions, refuses a
resource-shaped requirement unless it joins policy-kit by canonId (ADR-0095 decision 5) and builds no
infrastructure scanner (its F5). ADR-063 gains overlap O11 for policy-kit. Built now: Kubernetes per-container
facts over all three pod paths (LB-F451) and the observed-by join (TL-15), the second ops arm. ADR-055 decision 8(c)
is corrected in both places.

### D12. Opt-in versus default-on (N9): a measured promotion rule
Section 8 holds the rule. Deterministic facets may become default-on by a reading, with one wall-time bound (+3%)
and a named consumer metric per facet. Judged facets never become silently default-on: at most "offered by default
at onboarding", one consent per repository, after J0, calibration, a measured cost and a lifted spend hold (F69).
Agent-generated decision inputs are rejected as a separate component.

### D13. Web, scripting and ops formats at their practical ceilings (N1): adopt the queue edits
Section 9 holds the order. TypeScript to L4 excellent, HTML and CSS to L3 excellent under rulings W1 and W2, Vue and
Svelte to T1 L3 excellent graded by their own compilers (W4), Angular as a new T1 L3 sub-row, Bash to L3 good with
zsh and ksh folded in, PowerShell to T1 L3 good, then the ops formats that read good with no path forward
(Jenkinsfile to L3, Kubernetes, GitHub Actions and Helm each to excellent or a written ceiling), then Tailwind v4,
then the smaller languages at T1. The language rulings are renamed W1 to W4 so they cannot be confused with ADR-056
R1 to R8, which this same design cites.

### D14. Dependency mapping (N10): decided, roadmapped, unshipped, and RT-M1 is the critical path
No new decision. Section 7 states the status. The highest-leverage unblock in the whole roadmap is RT-M1 reading
PASS: it opens SG-2, and with it dependency usage and upgrade impact, the env and config links, the feature map and
the runtime topology. The 2026-10-09 FAIL was 107 minutes of swap-volume disk-floor refusals on 2026-10-03 (the
refuse floor is 5 GiB free), and every memory clause held. So the wave plan runs at most two waves at once, and no
wave starts during an RT-M1 window while the swap volume has under 15 GiB free.

### D15. The vision upgrade (N11): from symbol graph to repository graph
Fathom's identity line stays: the proof layer for code. What widens is "code": every file a repository carries,
not only its Go symbols. New goal G36, "every file accounted for", is a pre-registered bet with $0 readings
(`docs/intent/scope.md`). Its probe sample is a seeded random draw whose seed and subject SHAs are committed before
the probe runs. Product vision `docs/design/product-vision.md` gains one paragraph pointing here.

### D16. Verify reads attested runs, and stays static (J5): amend I8 and ADR-023
- I8 says "`fathom verify` is static (`runtime_pass` always Unknown)" and ADR-023 says verify is "explicitly
  STATIC, never asserts `runtime_pass`". Wiring ingested test evidence into `runtime_pass` contradicts both, so this
  record amends them instead of working around them.
- The amended meaning of static: verify reads attested evidence and never executes a test or a command.
  `runtime_pass` reads the newest attested run in `test_runs` for a row's certifying tests: Pass or Fail with the
  run's commit and age, Unknown when none exists. It stays informational and never changes a row's status.
- D0-1 (`docs/plans/sdd-reconciler-v0.md`, a deferred `cmd` oracle that would run commands for `runtime_pass`) is
  retired as superseded by TC-29: reading an attested run replaces running one.
- The `TestCmdVerify_*` tests that pin `runtime_pass` to Unknown are updated in the same change to pin Unknown
  without an attested run and Pass or Fail with one.

## 5. The compounding joins, ranked by gain per cost

Each join wires a fact fathom already computes to a consumer that does not read it, or repairs a channel that
loses what it carries. Category: T tokens, C correctness, E exploration, A agent capability, X expert systems.

| # | Join | Gain | Proving reading | Cost | Home |
|---|---|---|---|---|---|
| J1 | Restore the rule-outcome writer under the JF-1 names, into GL-11 rule effectiveness and GL-19 remedies, and make every silent edit-hook answer record its reason | C, X: governance that learns, and the hook's 2.3% emission share becomes explainable. GL-M2's 30-day clock and FD-M1 and FD-M2 start | rows per day above 0 for 7 days under the new names, doctor flags a silent writer, every hook answer carries a reason | XS | GL-24, GL-M6 |
| J2 | A destination class on the edit hook's beat (clone of an indexed repository, git worktree, gitignored, outside), then XM-M1's repository-relative fold for the clone and worktree classes into the hook's candidate roots | C, A: rules reach edits in clones and worktrees, and silences become attributable | XM-M8: 7 days of class-tagged rows before the fold, 7 after. The bar applies to the clone class only: its `foreign-target` share under 10% after the fold. Gitignored and outside are reported without a bar. The worktree class is reported as the general case | S | XM-36, XM-37 |
| J3 | A PreToolUse(Bash) HEAD stamp into the pin check, so the commit is found without git's summary line | C: the pin line stops skipping 744 of 745 checks. PR-M1's 30-day record clock restarts on a live instrument | skip share under 10% over 7 days | S | PR-13 |
| J4 | Ingested coverage into `RiskOptions.Coverage` in all three callers, from a named source and never a store: the attested Go profile pair (`internal/conserve`'s `coverage.meta.json` and profile) or a report path the caller names (lcov and the other TQ-10 to TQ-13 formats, joined by `joinForeignBlocks`, TQ-14) | C: the risk rank stops treating every symbol as uncovered | `coverage_supplied: true` naming its source on an lcov subject, TQ-M1 becomes readable | XS | TQ-26 |
| J5 | Attested test runs (`test_runs`, from `fathom tests ingest`) into verify's `runtime_pass`, under the I8 and ADR-023 amendment (D16) | C: done means proven gains a runtime criterion, still informational | `runtime_pass` reads Pass or Fail, not Unknown, on a row with an attested run, and the updated `TestCmdVerify_*` pass | S | TC-29 |
| J6 | Stored 180-day churn as a third arm of IM-M0 (the revert-corpus replay), never wired into `localize` on its own | E: if churn beats the seed-only arm at equal list length, IM-3's hop may read it beside the window's commits | the IM-M0 arm's own bar, fixed before the run | XS | IM-M0 (amended), no new row |
| J7 | Role plus `runs` plus non-Go entry points into `blast_radius`, `test_impact` and `wired` | A, C: impact and wiring answer for scripts, CI, TS, Python, PHP | the section 3.1 probes flip from 0, `wired` returns an informational verdict on TS, Python and Rust fixture rows | M | RF-1 to RF-10, TQ-27, TQ-28 |
| J8 | `runs`, `configures` and role as `review_diff` dimensions | C: config and script diffs are reviewed instead of passing silently | review-recall corpus rises from 6 of 42 (`docs/bench/readings/review-recall-loop-2026-09-18.md`) toward the 30% bar, 0 false findings | S after J7 | RF-13, RF-16 |
| J9 | The file passport at first touch, on SR-60's line | T, A: one line replaces several discovery reads, the governing rule arrives beside it | RF-M4 against its pre-ship base rate. VB-40 at 6 of 8 offline. The XM-M2-style A/B waits on the spend hold | M | RF-14, RF-17 |
| J10 | Kubernetes per-container facts plus policy membership (`internal/fusion/policyref.go`) | X: "a workload no limits policy selects" becomes a membership join over facts, ready for policy-kit's rules | field parity with a structural YAML decode, 100% on the calibration manifests, all three pod paths | S now, the join later | LB-F451, F61 |
| J11 | Observed-by: the ServiceMonitor and PodMonitor that scrape a workload (the shipped selector joins in `internal/fusion/routeref_mesh.go`) served on the workload beside J10 | X: "a workload no monitor scrapes" becomes a join. The alert and metric half stays inside Prometheus' own entities (K19), and metric-to-emitter stays F66 | precision at least 0.95 against the side pass's tier C oracle on the kube-prometheus and istio subjects | S | TL-15, TL-16 |

Joins outside the top eleven, recorded: co-change clusters into feature candidates (FM-15), findings near a failing
test on the test-run hook's line, the calibration corpus as the first reference map (F68), role-scoped rule
selection for VB-40 (a rule may name `role: test` instead of a glob, tried only if VB-40's misses trace to globs).

Order: J1 before anything that reads rule outcomes or interprets the hook's emission share, J2 and J3 before any
adherence A/B on workflow traffic, J4 before TQ-M1, the guard on `runs` links from their first row and RF-5's
fleet-wide guard only after RF-M6's count, J7 before J8 and J9.

## 6. Operational excellence: the boundary

| Who | Owns | Examples |
|---|---|---|
| **Fathom core** | code-grounded facts, membership, reach, delivery at the edit, proof by certifying tests | per-container requests, limits, liveness, readiness and startup probes, init containers as a fact of their own (today merged into one container list, `internal/extractor/k8sextract.go:149-152`, which also misses CronJob's `spec.jobTemplate`), replicas, read from all three pod paths. Kyverno and Gatekeeper membership (shipped, `policyref.go`). Monitor membership (shipped selector joins, served by TL-15). Imported findings at the edit (shipped). Later: the selector join from PodDisruptionBudget, HorizontalPodAutoscaler, Service and NetworkPolicy to the workload, reusing the policy-ref matcher (F61) |
| **policy-kit** (platform-engineering ADR-0015, unbuilt) | resource-shaped rules | the control, its CEL assert, its Kyverno and VAP forms, its framework crosswalk and coverage. Until it ships, the upstream Kyverno policy library a repository already applies, which fathom reads as membership |
| **Review Kit** | review judgment over practice, process and artifacts | concern packs, severity, dispositions, the well-architected taxonomy, typed criteria (with Jev) for the memory and leak residue. It refuses a resource-shaped requirement unless it joins policy-kit by canonId (ADR-0095 decision 5) and builds no infrastructure scanner or second resource-policy evaluator (its F5) |
| **Tech packs** (ADR-061, once accepted) | the carrier for a kit's guards and formats | scoped guards, dialect data, generated-file provenance. Never a resource rule: that would be the second evaluator Review Kit F5 refuses |

- *The observability arm* (ADR-062). Stage A shipped: frames, telemetry identities, log templates, and Alertmanager
  and Grafana joins inside Prometheus' own entities. TL-15 adds observed-by on the workload. An alert or metric
  reaches a workload only through Prometheus' entities and the monitor selectors: a label-to-workload join stays
  closed under K19 (label_join_recall .583), and re-opens only by K19's path (b), a rule committed before a holdout
  reading. Metric and span names to their emitters stay F66, gated on census E (.1451 against .25).
- *Memory management.* The computable part is linters run as findings or certifying tests: `go vet` lostcancel,
  bodyclose, noctx, goleak in tests declared `certifies: rule:`. That proof is Go-only until TC-26 lands the marker
  for other languages. The rest is judgment, in Review Kit's criteria lane, or in fathom only after J0.
- *Semantic layers for critical concerns* are F52's concern scope, unchanged. The earlier draft's static-scope
  amendment is withdrawn (section 13).
- *Consumer gate* (P20). No consumer with production manifests is named. Until one is, fathom builds only the
  per-container facts and TL-15, which earn their place as dialect quality and as a join over shipped edges. F59
  and F61 keep their clocks.
- *Correction.* ADR-055 decision 8(c) and its Consequences bullet read "via the findings outbox now. A kit's guards
  and formats may ride tech packs once ADR-061 is accepted. A resource-shaped rule stays policy-kit's."
- *Refused*: an ops kit (SV-X7), fathom-owned Kubernetes rule text (ADR-055 decision 7), fathom running scanners,
  a resource predicate in a tech pack (Review Kit F5, ADR-0095 decision 5), generic SARIF-as-pass, vendored Semgrep
  rules (not OSI-licensed).

## 7. Reference implementations and dependency mapping

*References today.* An agent cannot record "repository X at commit S implements our FR-7" and find it later. The
pieces exist apart: an external repository can be ingested as a service, `internal/sdd` links spec IDs to symbols
inside one service, maps anchor with hashes and re-resolve on drift, KR-6 and KR-7 link a doc note to code and flag
drift, and the calibration corpus pins url, ref, sha and licence. Nothing joins them, and map entries cannot name
another service's code without losing the service (D9). Verdict: D9, deferred as F68.

*Dependencies today.*

| | State |
|---|---|
| Decided | ADR-050 (the `pkg:` kind, local caches only, I56), ADR-063 O1 (ci-kit's `depgraph` owns the package closure, fathom the symbol graph), SG-X4 (no second resolver) |
| Roadmapped | SG-30 to SG-42 (the port, lockfile adapters, cache locator, dependency API index, usage map, upgrade impact, OSV reach), TQ-19 and TQ-20 (upgrade test reach), the feature map's dependency column |
| Shipped | `ParseContractKey` (SG-1), the version-exact `search kind:docs pkg:` reader (KR-12, unchecked at landing because nothing in the product writes dependency docs until KR-11 is wired), migration declarations with `pkg:` versions (CX-9). Nothing an agent calls to ask "who uses this dependency" |
| Blocked on | SG-2, the `contract_keys` and `system_links` tables, frozen until RT-M1 reads PASS |
| Refused | a second package graph, an SBOM product (ci-kit's lane) |

## 8. Opt-in versus default-on

**Default-on**, behind an opt-out key, only when every condition holds:

1. deterministic, $0, nothing leaves the machine, byte-stable (I42).
2. no new table, column, cache or background job before RT-M1 (ADR-049 decision 2).
3. inside the hook bounds (the 320-token edit payload, I12 for the primer, I53) and no new MCP tool.
4. its reading holds: precision at least 0.95 with a Wilson 95% lower bound at least 0.90 on a committed fixture
   spanning at least three ecosystems, unknowns not counted as correct.
5. ingest wall time at most +3% and store at most +2% (median over the corpus), or an owner ruling under K26. This
   is the one wall-time number in this design: above +3% a facet is computed at query time instead.
6. its named consumer's metric (below) holds or improves, and nothing gates on a heuristic basis.

| Facet | Its reading | Its consumer and the consumer's metric |
|---|---|---|
| Role | RF-M1 (the convention `test` role) | `search` hit flags: at most 2% change without a labelled gain. The risk rank's test exclusion agrees with RF-M1's oracle |
| `runs` | RF-M2 | `blast_radius`: RF-M5's "run by" answers agree with RF-M2's oracle, and the review-recall corpus (6 of 42) does not drop |
| `configures` | RF-M3 | `symbol_context`'s configured-by answer within RF-M3's 2% |
| Entry points | RF-M7 (the flip count) | `wired`: informational until RF-D7. Then no fathom plan row changes verdict without a line in the change |
| Test entries | the runners' own collection | `test_impact`: on fixtures, every selected test is in the runner's collection, and `JoinCasesToSymbols`' join rate is at least 0.95 |

The role facet, `runs`, `configures`, entry points and test entries are designed to pass. Each starts default-on
only after its reading.

**Opt-in** whenever any one holds: a model call (Jev included, the port stays provider none), egress, a per-repository
cost, a new table before RT-M1.

**Judged sites** climb a ladder: off, then opt-in per repository (today's rule, JF-4), then "offered by default at
onboarding" (pre-selected in the `init --dry-run` priced section, one consent per repository). The last rung needs J0
PASS, a calibration reading whose Wilson lower bound is at least 0.90 for its use (ADR-056 R3), a measured cost at
most $0.25 per 10,000 files on three subjects, the estimator within 2 times of actual on three consecutive runs, a
positive consumer A/B, F7 if the output is stored, and the spend hold lifted. "On whenever a provider is configured"
would amend JF-4 and ADR-056 R2. It is F69, promoted only when two sites have cleared the last rung. Cheapness never
promotes a site by itself.

**What it would cost** (arithmetic from the measured $0.22 to $0.27 per label for a full pass over fathom's 2,493 Go
files, `docs/design/judged-predicates.md` section 5, about $0.0001 per file. The rows below are extrapolated, not
measured. The provider's 20 requests per second binds before money does: 100,000 files take about 80 minutes):

| Files | Deterministic facets | Judged residue (10% unknown, one predicate) | A full judged pass without the facets |
|---|---|---|---|
| 1,000 | $0 | about $0.01 | about $0.10 |
| 10,000 | $0 | about $0.10 | about $1 |
| 100,000 | $0 | about $1 | about $10 |

Tokens are not the binding cost. Calibration is: a served band needs about 100 human labels per site with at most 3
errors, and K0 forbids faking them. The human label budget, not the provider bill, decides how many judged sites
can ever be offered.

**Agent-generated decision inputs** (N9): rejected as a separate component. A decision template is calibrated per
(site, template version, provider, model) at n of 100 or more (ADR-056 R3). A template generated per repository on
the fly can never be calibrated, so its answers would stay uncalibrated free-form questions that may never prune or
reorder (I62). The cheap form that survives already exists: the working session, already paid for, files candidates
at forced moments (maps, XM-8), and a fixed template with options drawn from index facts asks Jev to choose among
them (vision 40 section 4). No generator agent, no background job (the dispatch boundary).

**Jev, weighed** for every decision here. Role, `runs`, `configures`, entry points and test entries are exact or
heuristic rules: no site. The passport's ordering when it overflows could be a re-rank site, and the obligations
re-rank A/B read negative (paired mean -0.19), so no. Reference matching would be generation, the port's weak side.
The ops residue is judgment, and it lives in Review Kit's criteria. What would admit Jev: RF-M1's unknown residue
above 5% of files on a subject while J0 reads PASS. Then F37's first template is "role among declared layers" for
that residue, opt-in.

## 9. Web, scripting and ops formats (N1): the order and the ceilings

After wave lf4 (landed 2026-10-10, its readings in `docs/plans/language-breadth.md`: the TS and JS holdouts,
shebang dispatch, SQL and Bash ranges, CSS L1, HTML L2, the SCSS dialect):

| # | Item | Target | Oracle that makes "excellent" honest |
|---|---|---|---|
| 1 | TypeScript | T1 L4 excellent: member properties mapped, `.tsx` in measured scope (O749) | scip-typescript (A) |
| 1 | JavaScript | T1 L4 good on checkJs code, module bindings named. Untyped code is capped per subject (W3) | scip-typescript (A) |
| 2 | HTML | T1 L3 excellent: parse5 ranges, lychee `--offline` for src and href, L2 per W1 | parse5, lychee (A) |
| 2 | CSS, SCSS | CSS L2 and L3 per W2. SCSS gains dart-sass `loadedUrls` as its import oracle | lightningcss (W2), dart-sass (A) |
| 3 | Vue, Svelte | untiered to T1 L3 excellent (W4) | `@vue/compiler-sfc`, `svelte/compiler` (A) |
| 3 | Angular | new T1 L3 sub-row: template selector to component, `templateUrl` and `styleUrls` file edges, L4 through TS | `@angular/compiler` `parseTemplate` (A) |
| 4 | Bash | T1, L2 to L3 good: `source` and `.` edges, zsh and ksh routed to the bash dialect behind the 5% error-file gate | no tier A static oracle: good is the ceiling |
| 4 | PowerShell | untiered to T1 L3 good: `.psd1` RootModule and NestedModules, dot-source, `Import-Module` | `Test-ModuleManifest` (A) for manifests |
| 5 | Jenkinsfile | T2, L2 good to L3: `@Library` shared-library and `load` file edges | the Jenkins declarative linter (A under W4, a local controller in the measuring harness only), the Groovy AST walk stays tier C |
| 5 | Kubernetes | T3, L2 and L3 good to excellent or a written ceiling | Kubernetes' own label-selector matcher (`k8s.io/apimachinery` `labels.Selector`, A for what it attests: selector edges), verified at row start |
| 5 | GitHub Actions | T2, L3 good to excellent or a written ceiling: local `uses:` edges | actionlint's own resolution of local actions and reusable workflows (A if it attests the path), verified at row start |
| 5 | Helm | T3, L3 good: the ceiling written as a row (`oci://` dependencies have no offline truth), `file://` dependencies graded | `helm dependency list` (A) |
| 6 | Tailwind v4 | new T1 L2: `@theme` tokens, `@utility`, `@custom-variant`, usage edges for custom utilities and tokens only | `@tailwindcss/oxide` scanner (A), under K26 |
| 6 | CSS Modules, custom elements | exact edges only: `styles.foo` to `.foo`, `<x-el>` to `customElements.define` | the TS checker, the HTML parser |
| 7 | Less, Jupyter | to T1 (L3, L2) | `less` render imports (A), nbformat |
| 7 | Perl, Lua, Blade, Jinja2, Astro | to T1 (Lua, Blade, Jinja2 and Astro L3). T2 is the build, package and CI tier (`docs/design/language-breadth.md` section 18.3), so no language goes there | per row |
| 8 | Bundler aliases | measured first on three Vite or webpack subjects, built only if at least 5% of unresolved specifiers match an alias | Vite's resolver, enhanced-resolve (A) |

Route readers ride their framework rows, one item each: Vue Router, Nuxt and SvelteKit file routes, and the Angular
Router. Rails routes stay system-graph SG-22. ERB templates are disclosed now and queued at T1 L2 when a Rails
subject names the need.

Rulings, committed in the language design before any holdout reading (K19): **W1** (HTML L2 means same-document ID
references, the attributes the HTML standard defines as IDREF: `for`, `aria-labelledby`, `aria-describedby`,
`aria-controls`, `headers`, `list`, `form`, `href="#id"`) accepted. **W2** (lightningcss, a browser-engine CSS
parser, is tier A for what it attests: definitions, ranges and the `@import` graph, the Ruby trace precedent) accepted.
CSS `var()` references grade as name references within the import closure, disclosed. **W4** (a format's own
compiler or linter is its reference parser, section 1.3 as written) accepted. **W3** (O482, how the oracle-blind cap
weighs subjects) ruled by the owner-architect on 2026-10-10: per subject, so a typed subject cannot lift untyped code.

Written ceilings: React L3 good (only a tier C oracle defines a component, `react.json`), L2 calls good in Bash,
PowerShell, Lua and Perl (no oracle resolves calls in these dynamic languages), PHP L2 good for a different reason:
scip-php is tier A and reads call_precision .6924 against fathom's pairs, with 1,869 fathom-only pairs on receivers
scip-php leaves unresolved (`php.json`), so excellent waits on explaining those pairs, not on an oracle. Ruby L4
only on Sorbet-typed code, untyped JavaScript L4 capped per subject (W3), and markup and style formats stop at L3 (no SCIP
producer). Windows `.bat` and `.cmd`, Handlebars, EJS and Liquid are disclosed as unsupported, not built.

## 10. What merges where

| Note | Merges into | New rows |
|---|---|---|
| N1 | language-breadth queue (G15) | queue edits, LB-Q-angular, LB-Q-tailwind, LB-Q-jinja2, LB-F451 to LB-F458 |
| N2, N3 | repository facts (G36, new plan, no new engine) | RF-1 to RF-17, RF-M1 to RF-M7 |
| N3 (judged residue) | F37 amended | none |
| N3, N9 (onboarding) | reach, judgment-foundations | RH-13, JF-25 |
| N4 | feature map (G26) | FM-15, FM-4 amended |
| N6 | exploration maps (G30), F68 | XM-D10 |
| N7, N10 | governance-learning, exploration-maps, served-not-kept, test-intelligence, test-certifications, incident-maps | GL-24, GL-M6, XM-36, XM-37, XM-M8, PR-13, TQ-26, TC-29, IM-M0 amended |
| N10, N11 (test entries, D5) | test-intelligence (G23) | TQ-27, TQ-28 |
| N8 | language-breadth (facts), telemetry-links (observed-by), ADR-063 O11, F61 (later), policy-kit and Review Kit (rules) | LB-F451, TL-15, TL-16 |
| N9 | I65, F69 | none |
| N10 dependencies | system-graph, unchanged | none |

## 11. Sequence and budget

Nine waves, interleaved with the language program and the hygiene waves, **at most two running at once** (the RT-M1
disk floor, section D14), and no wave starts during an RT-M1 window while the swap volume has under 15 GiB free
(the roadmap edit in vision 38 section 9.4). In order: wave h5, after h4's ADR-066, carries J1, J2's destination class (so the fold has 7 pre-fold days of baseline), J4, J5, the IM-M0 amendment and
these records. The RT-M1 re-read on or after 2026-10-11 is a reading, not a wave. Vision 43's waves 1 and 2 (ADR-068)
ride as fillers beside the role classifier and the guard waves. Then the role classifier with J2's fold and J3, then
language wave lf5 (TS L4, HTML, CSS), then the guard, `runs` and entry points, then lf6 (Vue, Svelte, Angular, Bash,
PowerShell, disclosures), then `configures`, test entries, review dimensions and the priced dry run, then lf7 (the
ops formats, Kubernetes facts, observed-by), then the passport. The wave after that depends on RT-M1: SG-2 and the
feature map if it passed, Tailwind and the exact web edges if not.

Budget: no reading spends provider or agent budget (all are replays, fixtures and tool oracles on the operator's
machine). Implementation waves run on the normal worker budget. The total per wave is unmeasured. The only figure on
hand is an operator note's estimate of wave overhead alone (triage, integration, audit and the gate), 1.5 to 3
million tokens, which is not a total. Store growth from any row over +20% needs an owner ruling (K26).

## 12. Kill criteria in one place

| Reading | Kills |
|---|---|
| RF-M1: the convention `test` role under 0.95 precision or a Wilson lower bound under 0.90 after two waves | the convention rows that fail. Attribute, header, dialect and Linguist-derived roles stay (they are their source's answer). Amended 2026-10-11: with no product entry they were deleted (D2's amendment) |
| Unifying the classifiers changes more than 2% of search flags with no labelled gain | new roles. The unification stays |
| RF-M2: a command kind under 0.95 precision against the tool's own expansion after one fix round | that command kind |
| RF-M3: tsconfig membership disagrees with `tsc --listFilesOnly` on more than 2% of files | `configures` serving for tsconfig (resolve-time use stays) |
| Ingest overhead of the facets above +3% wall | computing at ingest. They move to query time |
| RF-M4: post-ship share under 10%, or no lift over the pre-ship base rate, or an edit-payload or hook-deadline breach, or a rule cut for the passport | the passport |
| A role view or `review_diff` dimension unread by any session for 30 days after it ships | that view. The facts stay |
| J4 or J5: the join's reading does not improve | the join, reverted. J1 to J3 are repairs, judged by GL-M6, XM-M8 and the pin skip share |
| J6: the churn arm does not beat the seed-only arm on IM-M0's pre-fixed bar | churn in `localize`, for good |
| Edit-hook emission on rule-scoped file edits still under 5% after XM-37, read with GL-24's reasons | delivery work. Rule scoping is fixed next instead |
| F68 baseline not met by 2027-01-31 | reference maps (the Preserve line stays). After building, 0 lookups in 60 days removes the kind |
| No ops consumer named by 2027-09-30 (F59's clock), or selector-join precision under 0.95 | the selector join |
| TL-16: observed-by precision under 0.95 against the side pass's oracle | observed-by serving |
| J0 reads FAIL | every judged facet and F69, until a new model generation re-reads J0 |
| JF-25's estimate off by more than 2 times | pre-selection at onboarding. Lines print "unpriced" |
| A web or ops item misses excellent after two waves | the item gets a written ceiling |
| Tailwind store growth over the K26 budget with no ruling | Tailwind usage edges. Token and utility definitions stay |

## 13. Rejected, with reasons

| Idea | Why not |
|---|---|
| Compiled "standard layers" for tests, scripts and config | ADR-056 R1, JP-X2, a tier that cites nothing, and per-repository cost for what a rule computes at $0 |
| Model labels for any computable file fact | ADR-056 R1 and JP-X2. I65 adds only that the residue is counted |
| A new container, a "semantic map" store or table | D1, the table freeze, and four existing kinds already cover every case raised |
| "Three containers, never a fourth" (the earlier draft) | false on its face: test envelopes, coverage files, the findings outbox and the ledgers were already a fourth kind |
| A separate agent that generates decision-port inputs | uncalibratable by construction (ADR-056 R3, I62), spend, and the dispatch boundary |
| Onboarding that spends by itself | O8's 25 times under-report and the autonomy rule |
| A new `fathom init --plan` flag | it duplicates RH-11's `--dry-run`, which gains the priced section instead (RH-13) |
| `ai-derived` output stored as `sme-stated` | a tier lie: `sme-stated` implies a human expert |
| A store column for role now | the table freeze. Compute on read until a consumer measures a cost |
| Grading Linguist-derived roles against `github-linguist` | the classifier imports Linguist's data, so precision is 1 by construction. Those roles claim no reading |
| Folding the bench graders and directory-walk skip lists into the classifier | the graders must stay independent of their subject, and a walk skip list decides what to walk, not what a file is |
| A `declared` resolution tier for `configures` | no such tier exists. `heuristic` already carries "never gates" |
| `runs` edges with no guard | verify reach is tier-blind and `wired` gates `--strict`, so a heuristic edge would gate on arrival |
| Non-Go `wired` gating at once | a breaking change: rows move from n/a to library-only Unknown, which the pre-push refuses. Informational first, then a counted, announced change |
| A new envelope join for non-Go tests | `JoinCasesToSymbols` (TC-3) is the join. It is extended, not copied |
| A new feature-map row for role, runs and configures (the earlier FM-16) | FM-4's `ComputeFeatureFacts` owns the column list. Amended instead |
| Churn wired into `localize` as its own row (the earlier VB-41) | it is IM-3's hop, gated on IM-M0, and 180-day churn is not recency. It becomes an IM-M0 arm |
| `runtime_pass` wired without amending I8 and ADR-023 | it would contradict an accepted invariant and decision silently. D16 amends both |
| The passport bounded by the primer cap (`TestWholePrimerCap`) | I12 bounds the SessionStart primer, not the edit hook. The 320-token edit payload is the bound |
| A 120-token passport | 37.5% of the edit payload. 60 tokens on SR-60's existing line, dropped first |
| Reading the hook's 2.3% emission as a leak | 21,247 answers carry no reason. Unknown until GL-24 records reasons |
| A `foreign-target` bar on the whole class (51% to 10%) | the class mixes clones, gitignored drops and scratch writes, and its rows log no path. The bar applies to the clone class after the beat records a class |
| Counting reference research from the session ledger | the ledger has no URL or clone field. Transcripts do |
| The F52 static-scope amendment for resource packs (a glob plus an entity kind such as `K8sWorkload`) | a pack predicate over resource facts is the second resource-policy evaluator Review Kit F5 refuses, and ADR-0095 decision 5 sends resource rules to policy-kit |
| A label join from an alert or metric to a workload | K19: the Stage A kill fired (label_join_recall .583). Prometheus' own entities and monitor selectors only |
| Lua, Perl, Blade, Astro and Jinja2 at queue tier T2 | T2 is the build, package and CI tier. They are languages, T1 |
| "No oracle resolves calls" as PHP's ceiling | scip-php is tier A. The ceiling is the 1,869 unexplained pairs |
| Language rulings named R1 to R4 | they collide with ADR-056 R1 to R8 in the same documents. Renamed W1 to W4 |
| Three waves at once through RT-M1 windows | the last window failed on disk-floor refusals. Two waves, and a free-disk floor for starts |
| Citing gitignored rulings (K22, K23, K26) or "the client" in committed records | unresolvable by `fathom intent audit` and by any reader (ADR-011). Rulings committed in the language design (LB-F455), "the queue's first-ranked languages" instead |
| A reference-implementation feature with its own tool, or a `git:` key now | law 4 and ADR-056 R6. A map kind covers it, and the key rides ADR-050's extensible kinds if F68 promotes |
| Fathom fetching reference repositories or dependencies | I56 |
| An in-repository "find similar existing code" finder | embeddings were removed under ADR-041 and lexical similarity is what `search` already does |
| Business-capability joins to tickets, CRM or revenue | FM-X1 to FM-X3 |
| An ops kit, fathom-owned Kubernetes rule text, fathom running scanners, SARIF-as-pass | SV-X7, ADR-055 decision 7, ADR-061 invariant (a), Review Kit F5 and F6 |
| A second package graph or an SBOM product | ADR-063 O1, ci-kit's lane |
| Usage edges for every Tailwind core utility, fuzzy global class to selector matching | store cost without a proven gain, and silently wrong edges |
| An "excellent" claim past a written ceiling | the oracle rule (ADR-048): a level is a committed reading |
| Live maps built early to serve concern scopes | XM-D1 stays deferred |

## 14. What we do not claim

- That the facets improve agent outcomes. RF-M1 to RF-M3 prove they are correct. An outcome claim needs an A/B, and
  the spend hold blocks it.
- That the passport saves tokens. RF-M4 is a replay against a base rate, a proxy until an XM-M2-style A/B runs.
- That anything here drives adherence outside the owner's traffic. The general case (one repository edited in
  place, git worktrees) is reported as its own stratum and claimed only once it holds with at least 100 first
  touches.
- That the edit hook leaks. Its silences carry no reason yet.
- That anyone would use reference maps. No baseline has been counted.
- That fathom evaluates a Kubernetes, policy or memory-management rule. It supplies facts and membership and
  delivers rules others own.
- That a non-Go test certifies anything. The marker is Go-only until TC-26.
- That Jev can populate anything today. J0 has not run.
- That `runs` edges are exact. They are heuristic, graded against each tool's own expansion, and never gate.
- That the cost table or the per-wave budget is measured. One is arithmetic on one committed reading, the other is
  an overhead estimate.
