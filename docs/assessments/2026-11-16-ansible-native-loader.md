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
The resolved 32 roles and five collections are frozen before native loading
in `calibration/oracles/ansible/native-loader-pins.json`; raw installed-file
hashes survive locally. Unversioned requirements use that frozen resolution,
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

The separate holdout is ansible-role-nginx release 3.2.0 at
`dbd7d5719418e860d16370ceee06e7abbd67935a`: all 26 tracked YAML candidates,
with its role root, outside canonical shards and aggregation. Identity and
selection were preregistered before content reads. After canonical tuning,
it was read once: 26/36 declaration matches (0.7222), 0/6 positioned local
binding matches, eight unresolved dynamic includes. Its 34 tracked files /
32593 bytes had identical before/after manifest SHA-256
`8be98846f7ac637050e04af9b0ebedef80e6a562d17b332f047ec6ddf9b59161`,
zero symlinks/gitlinks and no generated subject inputs. Its raw output is
frozen; no holdout-driven extraction or scoring change followed.

## Containment and proof

Each unit runs with a clean credential-free environment, private config,
home and cache paths, denied network, denied writes outside its own work
folder, bounded CPU, wall time, file descriptors, input and output. User
files outside the lane and Python runtime are denied reads. No uncontained
fallback exists. The first default-deny sandbox profile aborted even a
system no-op before Python; the corrected policy preserves OS startup and
enforces explicit data/network/write restrictions. The initial eight authored native
controls then passed, including independent read/write/network probes.

The initial RED import failure was observed before implementation. Controls
exercise positioned declarations, nested block/rescue/always tasks, native
role loading, notify binding and listener topics, no execution, malformed
input, whole-unit cap refusal, symlink refusal, and truth independence from
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
attempted. The separately preregistered released holdout was read once and frozen;
it never joins the canonical shard and will not be rerun. No plan checkbox, product extractor or generated tracker
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

Preregistration file creation and native population manifests precede source
reads. The first locale-dependent `shasum` command aborted, leaving its digest
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
a separate per-import byte bound. Symlinks and path escapes are refused.
The named native loader suite has nine passing controls; the comparison
suite has three. These are the only new Python suites this lane claims.

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

The delivered read-denial control uses an authored private sentinel, so the
suite requires no workstation file or credential read. The profile also
denies user-file data outside its lane and Python runtime. Both this portable
probe correction and fork denial are containment changes only.
