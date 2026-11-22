# Dependency edges for standalone readers

Status: accepted 2026-11-18, owner-architect ruling (ADR-114).
[Contract](../contracts/dependency-edges.md) · [Plan](../plans/dependency-edges.md).

A cache that omits an input may reuse a stale result. The CI kit is the first named
consumer. A standalone reader supplies a set of direct edges and a completeness verdict
for each file, including the reason and possible evidence source for every unknown.

## Intent check

- ADR-011 and I19 govern public text and fixtures. ADR-066 and I56 keep reads pure Go,
  offline, and free of toolchain execution. The test harness alone runs pinned oracles.
- ADR-063 O1 keeps package closure and hashing in the CI kit's dependency graph. This
  door reads each language's file-level dependency constructs. ADR-090 still governs
  observed reads. The named consumer fires TQ-38's trigger. DEP-8 owns the deferred audit.
- ADR-092 and I69 keep the standalone boundary and the `pkg/lang` closure at 80
  packages. The dependency door is a sibling feature over existing read evidence.
  ADR-112's builder direction supplies the precedent for a consumer conformance helper.
- The named consumer satisfies F99 for this door. Vision 46 §7 admits it: computable
  from bytes and a repository view, returns a set and verdict, rides the existing
  library and binary, and admits against independent tooling. No default tool is added.

## Sequence

1. Record the accepted boundary and preregistration (DEP-0).
2. Build the door, the Go control, and the binary and export (DEP-1 through DEP-3).
   Review the public API and serialized shape independently and obtain architect
   acceptance before integration.
3. Read TypeScript and JavaScript against the compiler's file list and resolution
   trace (DEP-M1), Python against per-entry standard-library module finding (DEP-M2),
   and Dockerfile against a pinned BuildKit parser (DEP-M3). If oracle rules
   preclude BuildKit, use a labelled sample and keep Dockerfile detect-only.
4. Add resolver, pack and declaration conformance with a runnable consumer example
   (DEP-7).
5. After the public door lands, wire the import boundary rules into the existing
   family import gate (BND-1 through BND-3). After DEP-7, two passing B1/B2 readings
   and architect acceptance permit moving the closure engine (DEP-P1a). Computed
   path filters follow that move (PF-1 through PF-3). The consumer switch, the
   under-keying audit and dialect breadth keep their plan triggers.

## Preregistered admission

Freeze public repositories, exact commits, deterministic file selection and oracle
versions before the first run. Use at least three repositories and 300 files per
dialect (30 files for Dockerfile). A smaller or ungraded population is inconclusive.
Run Go's `go list -deps -json` as the control. Test-only code may compute reachability.

| Bar | Rule |
|---|---|
| B1 | Zero oracle in-repository direct dependencies missing from files marked complete. |
| B2 | Zero wrong file targets among files marked complete. Count extra edges separately. |
| B3 | Byte-identical output on repeated runs and in a second checkout path. |
| B4 | Report complete-file share, unknown-edge share by code and extra-edge rate. Below 0.80 complete is “sound, low coverage” when the soundness bars pass. |
| B5 | A B1 or B2 failure leaves that dialect detect-only. |

No threshold changes after a run. Record per-repository and pooled counts, the
oracle's coverage gaps and the evidence that limits the verdict. A fresh reviewer
reruns each reading from a clean clone. Each dialect gets at most two correction
rounds before publishing the result at its measured ceiling.

## Coverage of the admitted readers

Preregistered 2026-11-22. DEP-M1 and DEP-M2 fix the development pools
at 1,224 TypeScript, 1,508 JavaScript and 1,314 Python files. Their complete
counts were 250, 153 and 121 respectively. COV-1 measures the blockers afresh.
ADR-114 governs completeness, ADR-011 governs published evidence, and
ADR-066/I56 and ADR-092/I69 govern offline operation and the package budget.

- **COV-D1** applies the original B1, B2, B3 and B5. B4 also reports closure.
  The holdout's complete share must reach 0.80 for the sound label.
- **COV-D2** takes the first two unused public subjects in corpus order
  with at least 100 selected files and a runnable pinned oracle. TypeScript
  also needs config chains inside the tracked tree. The pool must have at
  least 300 files. Use one eligible subject if the list yields only one, and
  hold for a ruling if none meets the rule. The baseline permits one sealed
  aggregate table per dialect: files, oracle covered, complete, B1–B3, and
  unknown totals by code. Reader authors have no access to file or line
  results. The final holdout campaign runs once. One correction may turn
  complete files incomplete, followed by one rerun.
- **COV-D3** measures the share of selected files whose transitive closure
  over local reader edges contains only complete files, before and after
  on both pools. A directory edge includes its direct files. Report
  separately closures blocked solely by files outside measured reader
  coverage. Test-only code computes reachability. There is no closure bar.
- **COV-D4** ranks one-mechanism buckets by greedy marginal complete-file
  unlock over per-file blocking sets. A file unlocks when all its blockers
  close. Admit a lever at 2% of development files or as a prerequisite of
  a lever meeting that bar. Cap at five per dialect. Stop at the bar and
  publish the remaining buckets and their `decidable_by`.
- **COV-D5** first uses the repository view to resolve. When every member
  of a finite set of possible local targets is visible, it may report all
  targets and charge extra edges to B4. A pooled extra-edge rate increase
  over five percentage points requires a ruling. Account for unsupported
  literal constructs after those two choices. Computed operands stay
  unknown. No subject execution, network, toolchain or external installed
  files inform the product.
- **COV-D6** requires evidence from the repository view before ruling out
  locality for an external claim. The harness independently inventories
  local package and module names and counts a complete file's external
  claim to one of those names as a B1 miss.
- **COV-D7** counts development files blocked solely by known target
  identity without a pin. A share of 10% or more requires a ruling before
  adding a second verdict.
- **COV-D8** freezes exported declarations, closed sets, JSON and flags.
  A proposed public change requires a ruling with the Go doc diff.
  JSDoc and other type-only imports use the existing import edge.
- **COV-D9** extends native truth only with an independent adapter or
  pinned native oracle chosen before the changed reader runs. It executes
  no subject code, installs no dependencies and uses no network. Record
  the cached version and digest, mutation control, and agreement with the
  current oracle wherever both have truth. List disagreements. A fresh
  reviewer must confirm the extension and every disagreement on a complete
  file must be explained. Complete files without truth make B1 and B2
  inconclusive.
- **COV-D10** runs Go's first admission with three public repositories
  and at least 300 files pinned before the first reading. The native oracle
  is `go list` for membership, import resolution, build exclusions,
  embeds and cgo. At most two correction rounds. Go is the sole exception
  to the deferred DEP-9 dialect queue.
- **COV-D11** stops a dialect's levers if the holdout baseline fails B1
  or B2. Escalate the defect and correct conservatively for at most two
  rounds. B5 applies if the defect remains.

Final B1, B2 including COV-D6, and B3 must pass on both pools. Every complete
file needs oracle truth. At 0.80 or more complete on the holdout the verdict
is **sound**. Below 0.80 publish **sound, low coverage, improved** with the
measured ceiling when soundness passes. If soundness fails after the
permitted conservative correction, revert the offending lever or the
dialect's program changes when the lever cannot be isolated. Publish
**reverted** and retain the earlier admission. The value bar is twice the
baseline holdout complete share. The program meets its bar when at least
two of the three admitted dialects double with soundness passing. Passing
gains land even if the program misses the value bar.

### Holdout pins and selection

Use the order in `calibration/clone.sh`: the root corpus manifest first, then
the corresponding dialect shard in recorded order. Skip by repository identity
across both lists. Resolve annotated tags to the commits they name. Select all
tracked regular files with case-sensitive suffixes `.py` for Python,
`.cjs`, `.js`, `.jsx`, `.mjs` for JavaScript, and `.cts`, `.mts`,
`.ts`, `.tsx` for TypeScript. No symlinks, gitlinks, untracked files or
submodule contents enter the selection. Resolution sees the full tracked
repository. The selection digest hashes byte-sorted paths, each followed by NUL.

| Dialect | Public repository | Commit | Selected files | Selection SHA-256 |
|---|---|---|---:|---|
| Python | django/django | `c14b756185c88f7f2eb745ff061f3c221fea9de7` | 2,820 | `249607273bd5d422a036f13ecf5c1c3593c8c35a248ce48add6ed5ef59b7b7d2` |
| JavaScript | expressjs/express | `df0abc9333a3398b97b71f6ea7cd77d5ea3e9f97` | 152 | `259357e846a0a3c33fc6684e04b166f7e17ba3fc71e13ae25f5f60895c94eb66` |
| JavaScript | axios/axios | `cdcfd214c169a1acba8e267ab8e77ff4dfec3105` | 165 | `13f0ac4ebf8043a4950165aa8a6cc0f77c94af28262cc63128599c36b3b732ea` |
| TypeScript | nestjs/nest | `9ff83d540f50e96eb0093b4eb0dd96191aa6a285` | 1,673 | `5c53f4a879f68f2a08fcb3051fedc06f74fc4be1a96a252ecdfe2fb56aebd2e2` |
| TypeScript | colinhacks/zod | `1fb56a5c18c27102dbc92260a4007c7732a0ccca` | 404 | `5e0afbc75c1db4f0d47c085d7efe48d0f5f04e167ccbe3ac25b8eeb3f415ce1e` |

Each pool clears 300 files. Python has one eligible subject. Use the
[DEP-M2 CPython pin](../../calibration/oracles/deps-python/pins.json) with the
repository root as its sole search root. The other two dialects use the
[DEP-M1 Node and compiler pins](../../calibration/oracles/deps-tsjs/pins.json).
The eligibility checks ran the pinned oracles without a product read. Both
TypeScript subjects' config inheritance and project references resolve inside
their tracked trees. Native losses due to unavailable generated project
outputs remain losses under the oracle's grading rules.

| Dialect | Corpus and position | Skipped candidate | Reason |
|---|---|---|---|
| Python | Root 1 | fastapi/fastapi | DEP-M2 development subject |
| Python | Root 2 | pallets/flask | DEP-M2 development subject |
| Python | Shard 1 | fastapi/fastapi | DEP-M2 development subject |
| Python | Shard 2 | pallets/flask | DEP-M2 development subject |
| JavaScript | Root 3 | webpack/webpack | DEP-M1 development subject |
| JavaScript | Root 4 | lodash/lodash | After both selections. Its pin also has only 55 selected files. |
| JavaScript | Shard 1 | eslint/eslint | DEP-M1 development subject |
| JavaScript | Shard 2 | openlayers/openlayers | DEP-M1 development subject |
| TypeScript | Root 3 | typeorm/typeorm | After both selections |
| TypeScript | Shard 1 | gcanti/fp-ts | DEP-M1 development subject |
| TypeScript | Shard 2 | ReactiveX/rxjs | DEP-M1 development subject |
| TypeScript | Shard 3 | react-bootstrap/react-bootstrap | DEP-M1 development subject |
| TypeScript | Shard 4 | tailwindlabs/headlessui | DEP-M1 development subject |
| TypeScript | Shard 5 | palantir/blueprint | After both selections |


## Library first uses

ADR-115 allows a same-change first use and dated keep-or-cut reading to satisfy
Vision 46 §7 clause 5 for library members. Boundary rules consume direct edges
and return violated, clean or unverified. The gate's first use covers this
repository's family boundaries and compares seeded violations with the Go import
graph. BND-M1 counts actual evaluations, pre-merge findings and outside use 30 days
after landing. It keeps the capability when a violation is caught more clearly
than by the older tests or an outside use is observed.

The engine move preserves the existing generic implementation. Its family
closure test enforces independent imports for the dependency graph and
understanding packages. Data crosses through the declared-edge export.
The separate consumer-switch phase owns changing the other repository.

Path filters read the workflow event patterns and join them with a declared map
from workflows or jobs to entry paths. They return missed, unverified or covered
and name unused patterns. The first use is a fixture with seeded misses and a
clean control. The CI kit's consumer switch supplies the first real wiring and
starts the 30-day PF-M1 window.

## Decision models

None applies. Dependency constructs, resolution, completeness and admission are
computed from repository inputs and compared with independent evidence. Provider
`none` covers the entire feature. Consumers opt in by invoking the door.
