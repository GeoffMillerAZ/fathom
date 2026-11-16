# Ansible native loader: frozen full-population audit

The existing canonical reading remains L3 good, tier C: 694 definitions,
9 local references and 60 cross-file edges. This audit grants no new level.
The native loader is available and its requirements are obtainable; no
oracle-absence or dependency-impossibility ceiling is asserted.

## Population and method

Before reading subject contents, preregistration froze all three complete
released subjects in `calibration/corpus.d/ansible.yaml`, every tracked
regular YAML file, and every role root with `tasks/main.yml` or `main.yaml`.
This native candidate population is independent of fathom's classifier.
Every YAML candidate is attempted through native Playbook loading and
standalone task/handler loading; every role root through native Role loading.
Alternative task/handler attempts are diagnostics, never additive truth.
A rejected non-playbook YAML file is not evidence of a playbook failure rate.

| Subject | Frozen YAML | Role roots | Native playbook loads/YAML | Task-list loads/YAML | Role loads/roots |
|---|---:|---:|---:|---:|---:|
| mac-dev-playbook | 13 | 0 | 1/13 | 5/13 | 0/0 |
| ansible-role-docker | 22 | 1 | 2/22 | 8/22 | 1/1 |
| ansible-for-devops | 122 | 2 | 39/122 | 16/122 | 2/2 |

All 157 YAML candidates and three role roots were attempted. Rejections are
retained in raw output; no candidate is dropped to improve a grade. This is
a native-loader audit, not a replacement for the full vocabulary's reading.
Playbook and role loads alone yield 19, 31 and 151 distinct positioned
in-subject declarations, respectively; 0, 8 and 10 notify bindings; and
2, 6 and 2 unresolved records. These context-only counts exclude standalone
candidate attempts, do not cover all declarations, and are not recall ratios.

`Base.get_path()` supplies start positions; no end range is invented.
`StrategyBase.search_handlers_by_notification` supplies handler bindings.
The instrument mirrors core's role-handler compilation order. For the
pinned core, duplicate handlers in the same implicit block resolve to its
first handler; the control asserts the observed native result rather than
substituting the documentation's general last-loaded description.
Generated implicit meta tasks are counted separately. Dynamic task/role/vars
includes and templated declarations/notifications remain unresolved misses.
No host task, inventory action or automation playbook execution runs.

Every subject requirements file was read, including nested requirements.
The resolved 32 roles and five collections have a pre-load installed-file
hash manifest; the human-readable version-pin document
`calibration/oracles/ansible/native-loader-pins.json` was written after loading.
Raw installed-file hashes survive locally. Unversioned requirements use that frozen resolution,
not an implicit latest version at reproduction. Downloaded dependencies are
outside each subject, so their declarations remain counted as external
context and do not become subject definitions.

The recipe's exact tracked-set manifest command was run before and after all
native attempts. Receipts retain all files, not only YAML, and neither
subject checkout gained untracked or ignored files. These are tracked-set
approximations, not a harness discovery list:

| Subject | Files / bytes | Before = after SHA-256 |
|---|---:|---|
| mac-dev-playbook | 31 / 57084 | `cf9b9913c2ecc3961ddb590e8f2a682bb339582223a6b0fb30cf3197ae779426` |
| ansible-role-docker | 27 / 29683 | `f3ac4a21b1c4ea94c53c1c508e3f54653ce8789baba21934a11ca7acab91a18c` |
| ansible-for-devops | 237 / 228980 | `a03121c036078f65e36a1a7b3d3c5c52f1e2b87daaf211fc75116d4960d015e4` |

All three have zero symlinks and gitlinks. YAML totals are 13/22/122;
other tracked files remain receipt-covered but outside this loader's YAML
population. The 201 context declarations above are incomplete coverage,
not the existing 694 definitions under a new tier. Some original examples
use removed `include` syntax and modules that current core redirects to
collections their old requirements did not name. Those rejections are
specific, retained limits; no claim is made that an older core or additional
obtainable collection cannot load them.

## Public dispatch diagnostic and one-shot holdout

The frozen native declarations were deduplicated by their tool positions and
kind/name. Loaded playbook/role contexts take precedence over standalone
mode probes; orphan files use native task loading, or handler loading under
a handlers directory. Nothing in this selection reads extractor output.
Every frozen YAML file was passed to public `fathom-lang extract`, including
files another dialect owned and those no dialect read.

| Subject | Start/name/kind matches/native | Ratio | Positioned local binding matches/native | Unresolved records |
|---|---:|---:|---:|---:|
| mac-dev-playbook | 19/19 | 1.0000 | 0/0 (absent denominator) | 2 |
| ansible-role-docker | 46/57 | 0.8070 | 0/8 | 7 |
| ansible-for-devops | 167/187 | 0.8930 | 2/10 | 8 |
| Pooled canonical | 232/263 | 0.8821 | 2/18 | 17 |

These are diagnostic matches, not recipe support metrics. Local binding
matches require the actual native source and target positions in the same
file; a bare name or a cross-file unresolved reference is not treated as a
positioned binding. No L3 claim follows from these counts. Unresolved
records include eight dynamic task/vars includes, two templated declarations
and seven unbound notifications across the canonical subjects (the full
per-subject reason counts accompany the summary). The source-external
native population is retained separately: 300 declarations and 35 bindings.
Repeated alternative contexts are deduplicated; no context success silently
repairs an entirely rejected input unit. The candidate/rejection inventory
remains available beside the diagnostic denominator.

The separately retained cohort, originally described as a holdout, is ansible-role-nginx release 3.2.0 at
`dbd7d5719418e860d16370ceee06e7abbd67935a`: all 26 tracked YAML candidates,
with its role root, outside canonical shards and aggregation. Identity and
selection were preregistered before content reads. After canonical tuning,
it was read once: 26/36 declaration matches (0.7222), 0/6 positioned local
binding matches, eight unresolved dynamic includes. Its 34 tracked files /
32593 bytes had identical before/after manifest SHA-256
`8be98846f7ac637050e04af9b0ebedef80e6a562d17b332f047ec6ddf9b59161`,
zero symlinks/gitlinks and no generated subject inputs. Its raw output is
frozen; no holdout-driven extraction or scoring change followed.

Independent-holdout authority is **withdrawn**. Comparing the retained
premeasurement dependency manifest with this cohort finds 25/34 identical
files, including 19/26 YAML files and eight task/handler files. The canonical
run had already loaded the nginx dependency and retained 19 of its declarations.
Being outside canonical aggregation did not prevent this substantive exposure.
The one-shot 26/36, 0/6 and eight-miss result stays frozen as a contaminated
historical diagnostic; it is not independent confirmation. No replacement,
rerun, retuning or canonical regrade was performed.

## Containment and proof

Each unit runs with a clean credential-free environment, private config,
home and cache paths, denied network, denied writes outside its own work
folder, bounded CPU, wall time, file descriptors, input and output. User
files outside the lane and Python runtime are denied reads. No uncontained
fallback exists. The first default-deny sandbox profile aborted even a
system no-op before Python; the corrected policy preserves OS startup and
enforces explicit data/network/write restrictions. The initial eight authored native
controls then passed, including independent read/write/network probes.

The initial historical RED logs are import errors, not behavioral assertion
RED; they do not establish the requested test-first chronology. Controls
exercise positioned declarations, nested block/rescue/always tasks, native
role loading, notify binding and listener topics, no execution, malformed
input, whole-unit cap refusal, a separate population symlink control, and truth independence from
extractor-output files. Raw truth changes when an input declaration changes.
The committed-reading round-trip test passed with `-race`; the organization
name gate passed. Six focused oracle/scoring mutants were caught and the restored controls passed;
a seventh mutation of the process-fork boundary was then caught, with a restored green rerun.
The separate legacy candidate suite had two optional-tool skips; the named
new suites have no optional oracle skips.

## Search receipt and limitations

Five sources were searched on 2026-11-16 before construction:

- Own toolchain: pinned ansible-core 2.21.4 loader/base/strategy source and
  Context7 `/ansible/ansible`, querying Playbook/Role loading, positions and
  native handler lookup. These APIs exist and are used.
- SCIP/LSIF: [Sourcegraph's catalog](https://sourcegraph.com/docs/code-navigation/precise-code-navigation),
  [SCIP tools](https://github.com/scip-code/scip), [LSIF.dev](https://lsif.dev/),
  and credential-free GitHub repository search `scip-ansible in:name`.
  Five substring hits, no named semantic Ansible YAML indexer. LSIF.dev is archived.
- Language servers: [official server documentation](https://ansible.github.io/vscode-ansible/editor/language-server/)
  and the existing pinned 26.6.0 source audit. Module navigation and delegated
  syntax/lint diagnostics are not a complete play/task/notify binding dump.
  This lane did not install or re-audit that candidate.
- Registry: [ansible-core 2.21.4](https://pypi.org/project/ansible-core/2.21.4/)
  installed bench-only (GPL-3.0-or-later). Existing measured ansible-lint
  26.10.1 and ansible-compat 26.10.0 remain candidates, not promoted truth.
- Runtime: [callback interface](https://docs.ansible.com/projects/ansible/latest/plugins/callback.html)
  and strategy/debug documentation. Callback/runner events attest visited
  tasks and require execution, so this lane uses static loading and lookup.

The native start-position/notify instrument does not establish end ranges,
all variable/include/role/inventory declaration kinds, or resolution of dynamic
includes. Shared inventory strategy work remains held. L3 grading is not
attempted. The separately preregistered released cohort was read once and
frozen; its independent-holdout authority is withdrawn. It never joins the canonical shard and will not be rerun. No plan checkbox, product extractor or generated tracker
changes. Decision models: none; truth and scoring are deterministic.

## Reproduction and retained evidence

Run the two named Python suites from the oracle README with a private test
scratch directory and the pinned runtime/dependencies; these bench Python
suites have no Go race mode. The recipe's extractor/connector/fusion checks
are run separately with `-race`. Exact native and public-output inputs,
per-unit statuses, rejections, truth/product digests, all before/after
manifests, dependency installation and pin receipts, initial RED, restored
GREEN, caps and mutation logs remain under ignored
`workspace/artifacts/lang-ansible/` until the parent critic completes review.
The parent owns cleanup. No subject content or wholesale path list is
committed. No reading-schema JSON, required-kinds table or generated tracker
is rewritten; the strict round-trip check covers the unchanged readings.

After the one-shot holdout, a containment-only correction denies
`process-fork`: a descendant could otherwise outlive the direct child's
wall timeout. Its expected probe failed before the correction. The original
measured source is retained locally; the summary records both source hashes.
This does not tune extraction, native truth selection or diagnostic scoring;
raw canonical/holdout results remain frozen and are not rerun or replaced.

Retained local timestamps corroborate that preregistration file creation and
native population manifests precede source reads. They are not immutable external
attestation. The first locale-dependent `shasum` command aborted, leaving its digest
receipt empty; the original unchanged preregistration file was hashed after
measurement with Python. This timing is disclosed in the summary rather than
asserting an earlier successful hash receipt. The native tracked manifests
were hashed successfully before loading and match their after receipts.

Mutation commands use `PYTHONPATH=calibration/oracles/ansible` and the pinned
Python runtime, under the shared lease. For each following one-line mutant,
`python -m unittest <test>` failed with an assertion, then restored green:

| Mutated file / change | Test |
|---|---|
| `native_loader.py`: replace native handler lookup with `found = []` | `test_native_loader.NativeLoaderTests.test_native_positions_binding_shadowing_listen_and_no_execution` |
| `native_loader.py`: disable dynamic-include detection | `test_native_loader.NativeLoaderTests.test_dynamic_and_unknown_notifications_are_misses` |
| `native_loader.py`: disable object-cap refusal | `test_native_loader.NativeLoaderTests.test_malformed_and_whole_unit_cap_plus_one` |
| `native_measure.py`: count all native declarations as matched | `test_native_measure.NativeMeasureTests.test_native_population_not_extractor_and_context_not_double_counted` |
| `native_measure.py`: set unresolved misses to zero | `test_native_measure.NativeMeasureTests.test_unresolved_dynamic_cases_remain_misses_and_external_is_disclosed` |
| `native_measure.py`: accept any target position | `test_native_measure.NativeMeasureTests.test_binding_requires_native_target_and_correct_source` |
| `native_loader.py`: remove `(deny process-fork)` | `test_native_loader.NativeLoaderTests.test_sandbox_blocks_network_and_outside_write` |

Caps: `test_native_loader.NativeLoaderTests.test_malformed_and_whole_unit_cap_plus_one`
uses a three-declaration cap and four declarations and asserts empty whole-unit
refusal. `test_native_loader.NativeLoaderTests.test_input_edge_depth_and_output_caps`
uses 2 MiB + 1 bytes at the direct entry boundary, four bindings at cap three,
65 authored nesting levels at cap 64, and 8 MiB + 1 output bytes. Each refuses
rather than serving a partial result. Imported files are bounded by native
unit CPU/time/count/output limits; the direct-entry byte cap does not claim
a separate per-import byte bound. The original actual CLI path did not reject in-subject symlinks;
its separate `population()` control did not establish that entry boundary.
Correction1 below enforces that boundary. Historically the native loader suite
recorded nine passing controls and the comparison suite three; this is retained
historical evidence, not a claim that those native controls were rerun now.

Validation commands (run under the lease):

```sh
bash scripts/fathom-go.sh test -race ./internal/extractor ./internal/connector ./internal/fusion -run 'Dialect|Tracker|Grammar|Classify|Unsupported'
bash scripts/fathom-go.sh test -race ./internal/langbench -run '^TestCommittedReadingsRoundTrip$' -count=1
bash scripts/fathom-go.sh test -count=1 -run '^TestNoOrgSpecificNames$' ./cmd/fathom
go run ./third_party/agentic-kit/cmd/release-notes -check
bash scripts/check-merge-debris.sh
```

No plan box was flipped, so no scratch verify success is claimed. No daemon,
paid model call, push, deployment or full gate ran. Full integration and the
independent critic remain with the parent. The lane's private Python runtime,
venv, dependencies, caches, config and corpora all reside in its workspace;
it requires no retained runtime from another lane.

The original delivered read control used a special-deny sentinel and could
pass with the general user-read restriction removed. That was a control defect;
the baseline production policy was not shown to leak user data. Correction1
replaces it with an owned outside-checkout synthetic canary governed by the
production boundary. No host, credential or user-data probe is used.


## Correction1: boundary and evidence corrections

Only the loader, comparison driver, their tests, README, this assessment and
the existing change fragment change. Pins, summary, plan, canonical readings,
source/receipt/corpus/holdout manifests and original proof logs stay frozen.
The published canonical **232/263**, **2/18**, **17 misses** and contaminated
one-shot **26/36**, **0/6**, **8 misses** are unchanged. Full native excellence
remains **UNPROVED**; canonical **L3 good C** remains unchanged.

- F1: distinct templated notifications retain their identity. One shared edge
  append guard bounds declarations' unresolved records, dynamic includes,
  templates, unbound notifications and handler targets before deduplication.
  `BoundaryTests.test_template_identity_and_every_edge_append_cap` proves
  three distinct misses at cap three, four attempts (including duplicates)
  refusing with empty declarations/bindings/unresolved, and mixed append paths.
- F2: the overlap above withdraws independent-holdout authority. Local file
  timestamps corroborate preregistration only; late preregistration digest and
  post-load human-readable pin chronology remain disclosed.
- F3: the comparison CLI requires the external frozen population and validates
  subject/SHA, YAML/role counts, every mode and unique unit, child envelope,
  and exact product membership. Incomplete/malformed collections refuse
  without publishing a ratio; complete collections expose expected/attempted
  and loaded/rejected/refused accounting. The retained actual receipts validate
  complete (39, 67, 368 canonical attempts; 79 contaminated-cohort attempts).
  This is not a finding that the published 263 denominator was truncated.
- F4: the read control uses an explicitly owned outside-checkout synthetic
  canary. Removing the general boundary makes it readable and makes the normal
  denial assertion fail; no sentinel-specific deny remains.
- F5: `run_unit` and the CLI check consumed paths and directory components,
  including in-subject links, before child launch. The subject's own role root
  remains valid. Controls mock the child and require it never be called for links.
- F6: malformed child JSON or schema becomes typed, classified empty refusal;
  subsequent units remain accounted for. Both CLIs invalidate owned stale
  success before processing and publish atomically. Path/symlink/hardlink input
  aliases are refused before invalidation. Private scratch must not be replaced
  concurrently; this is not an OS-level race-free filesystem API.

Correction1 first reproduced the exact original source with real behavioral
assertion RED: ten controls, twenty assertion failures, zero errors; four
additional cap/completeness/read-sensitivity checks, four assertion failures,
zero errors. The first correction harness attempt also had test errors and is
retained separately; it is not counted as clean RED. No current reproduction
can establish historical test-first chronology. The original commit-time hook
environment is not independently established by retained evidence.

Current correction controls (pure/mocked plus finite synthetic sandbox probes):

```sh
PYTHONPATH=calibration/oracles/ansible python -B -m unittest \
  test_native_loader.BoundaryTests test_native_measure
```

Set `FATHOM_ANSIBLE_TEST_TMP` to private scratch and
`FATHOM_ANSIBLE_OUTSIDE_CANARY` to the owned synthetic outside-checkout file;
run all sandbox and race checks under the language and machine leases.
No Ansible consumer, native population, holdout, throughput or SDK run occurs.
The historical nine authored-native controls are not rerun for correction1.

Mutation: in-memory copies of `native_loader.py`/`native_measure.py` remove
notification identity, edge cap, native coverage, product coverage, consumed
symlink checks, child schema validation, typed JSON refusal, each CLI's stale
invalidation, and the general read boundary. Each focused control fails an
assertion (zero errors), then the restored correction suite passes. The focused
tests are the corresponding `BoundaryTests.test_*` and
`CollectionBoundaryTests.test_*` controls; retained runnable mutation harness
and complete logs are under `workspace/tmp/fix1/`. No tracked source mutation
or native measurement is part of those checks.

Caps: the notification test above checks cap three/input four and cap one/input
two across mixed unresolved paths, asserting whole-unit empty refusal. Child
schema controls refuse partial/malformed success envelopes, not merely invalid
JSON. Collection controls include missing, duplicate, unexpected and malformed
native/product members and a self-consistent truncated header against the
external freeze. Normal mandatory hooks are verified currently by installed
script hashes and the actual correction commit trace; no bypass or allowlist
change is permitted. Current hook proof cannot establish the historical environment.


Current proof: 16 correction controls PASS; all ten targeted in-memory mutants
caught by assertion failures with zero test errors. Scoped existing Go checks
PASS under the lease:

```sh
bash scripts/fathom-go.sh test -race ./internal/langbench -run '^(TestCommittedReadingsRoundTrip|TestCommittedReadingsAreCanonical|TestCommittedToolingReadingsKeepMetricsOnRegrade|TestRequiredKindsThroughCanonical|TestTrackerDispatchColumnMatchesRegistry|TestValidateReading)$' -count=1
bash scripts/fathom-go.sh test -race ./internal/extractor ./internal/connector ./internal/fusion -run 'Dialect|Tracker|Grammar|Classify|Unsupported|Canonical' -count=1
bash scripts/fathom-go.sh test -count=1 -run '^TestNoOrgSpecificNames$' ./cmd/fathom
go run ./third_party/agentic-kit/cmd/release-notes -check
bash scripts/check-merge-debris.sh
```

These are scoped checks, not a full gate, native remeasurement, support
promotion or integration claim. No plan box or generated reading changed.
