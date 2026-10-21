# Typed loading and downstream verification

Verdict: select the connector's package roots and source-load their dependency
closure once per module load. Keep the 10x guard, typed test reachability and failure disclosure.
Final acceptance of source loading still requires the gates below.

## Findings and dispositions

- **F1, fixed in code:** the syntax baseline honors connector exclusions, including
  this repository's `third_party/`, but CT-5 passed nil to typed enrichment and
  selected the entire module. Production passed the discovered files but discarded
  unrelated packages only after loading them. Select sorted package-directory
  patterns within each owned module before loading, retain the existing per-file
  filter, and pass the same discovered scope to both CT-5 runs. Nil remains an
  explicit whole-module request. Tests cover selected/ignored broken packages,
  nested modules, external test variants, resolution, caching and empty loads.
- **F2, fixed in code:** gate preflight starts its shared deadline before Git root
  discovery. Expiration during that discovery returned before the diagnostic and
  observation. A deterministic discovery seam reproduces the missing hint/row.
  On deadline only, the existing filesystem work-tree check permits an honest
  not-checked report. Non-repositories, cancellation and the tested `GIT_DIR`,
  `GIT_WORK_TREE` and `GIT_COMMON_DIR` redirects remain silent. The deadline is unchanged.
- **F3, rejected explanation:** cache trimming alone does not explain the failing
  downstream ratio. At source 8d11ad68 and candidate kit afb436d9, fresh external
  caches still failed at 19.56x after CLI build and 16.08x after CLI plus all-package
  build. The original unequal-cohort trials are not comparable to the repaired
  measurement. No threshold relaxation, fake metadata, replacement golden or
  cache-priming target is adopted.

## Proof boundary

Behavioral regressions failed before each repair and focused race checks pass.
The matched-cohort direct run with a fresh external compiler cache failed at
13.80x: syntax CPU 13.942883 s, typed CPU 192.473197 s, 464 packages,
zero load failures, 188,998 in-scope edges. The same-store cached typed run took
7.989458 CPU s and loaded no packages. This diagnostic did not run the declared
CLI/all-package builds first. Full `task validate` subsequently passed with the
ordinary configured compiler cache: the non-race CT-5 target passed (121.877 s),
fusion's complete race suite passed (304.403 s), all 2,234 command tests passed
in four shards, and the standalone build/stranger adoption/configuration checks
passed. Successful CT-5 output suppresses its exact passing CPU ratio. This
record does not establish production latency or an absolute subsecond SLA. The
archived July CT-5 result used a smaller corpus and a warm compiler cache.

These are bug repairs under P19 (work follows scope), I23 (typed test reachability),
I25 (failed/empty loads stay visible) and ADR-070 (mirror ownership), with no new
dependency, exported protocol, ownership rule or decision-model site.

## F4: fresh-clone export compilation

The declared downstream run at kit 5e939468 and Fathom ea9cdc7d failed CT-5
after both builds and all 2,234 command tests passed. Its matched population was
6,496 discovered files, 3,829 Go files, 464 packages and 188,998 in-scope edges.
Typed CPU 164.430820 s / syntax CPU 13.328030 s = 12.34x. The earlier warm-checkout
passes do not establish a fresh-clone pass.

A diagnostic clone separated self and child CPU: the first typed pass used
14.933209 self CPU s plus 146.328266 child CPU s. In pinned x/tools v0.45.0,
`NeedTypes` without `NeedDeps` selects `go list -export -test`, compiling export
data before the selected source is type-checked again. `NeedDeps` uses the
supported source-loading path instead. Keep full ASTs and type information;
body elision was rejected because it creates false unused-import errors.

The same instrumented corpus with plain `NeedDeps`, under `GOMEMLIMIT=4GiB`,
used 26.161503 typed CPU s (3.661837 in children) / 15.387000 syntax CPU s =
1.70x. Its 189,000 edges include two diagnostic logging calls; the selected
graph otherwise matched. Maximum RSS was 3,448,619,008 bytes. This is diagnostic
evidence, not a final-candidate gate result or a portable performance promise.

A managed `FATHOM_TENANCY_BUDGET=4GiB` diagnostic ingest completed in 81.28 s,
with zero failed typed packages and maximum RSS 3,211,870,208 bytes. Reported
physical footprint peaked at 4,349,316,984 bytes, above 4 GiB, while the existing
push canary measures maximum RSS. Source loading trades more retained dependency
AST/type data for less compiler work. Do not claim universal 4 GiB fit from this
RSS result, or change the canary metric to hide the distinction.

Source loading also exposed a pre-existing failure-disclosure bug: dependency
errors were not checked before a selected root's cache hit. The same invalid
dependency body was silently accepted with the old mode. Traverse actual import
errors before reuse, retain the existing failed-package preservation path, and
bound the diagnostic. Regression coverage requires fresh failure, cached failure
without a false hit, named dependency errors, recovery after repair, deterministic
cycle-safe diagnostics, and preserved scope/test/cgo/reconciliation behavior.

The final source-loading candidate passed full `task validate`: the non-race
CT-5 target (90.126 s), complete fusion race suite (367.253 s), all 2,234
command tests and the other declared build/adoption/configuration checks.
Independent review found no blocker. Publication runs exact-tree plan proof
and the unchanged RSS canary again through the normal hook. Fresh-clone kit
integration has a separate receipt in agentic-kit's
`docs/evidence/catalog-kit/2026-10-21/downstream.md`.
