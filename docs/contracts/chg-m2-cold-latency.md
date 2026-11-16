# CHG-M2 cold standalone latency method (W11-3)

Status: Draft for independent foreman review and W2-3 approval, frozen before runs.
Source base: `54bca03154ea35c39bf08acba4c3572e843da56a`. Owns this document only; no benchmark ran.
Plan: `docs/plans/realignment.md:149` CHG-M2; reading destination remains
`docs/bench/readings/realignment/chg-8-cold-latency.md`. CHG-8/BEN-4 are separate
implementation/delivery prerequisites, not completed by a fast extraction.

## Authority, intent and scope

W11-3 settles exactly five trials per dialect, median, every trial published and
the existing 500 ms cap. It replaces the unresolved U3 choice in the earlier
`reading-hook-latency.md` draft at `01d89ed2badc223ce7678c2e2b71ebb0f39232eb`
(twenty/max versus hundred/p99); neither alternative is now permitted. The old
proposal is read-only context, not a runnable tool or a hidden prerequisite for
understanding this method. W11-3 itself supplies authority; no REC-M3 analogy.

Intent-design routing: P1 governed evidence, P3/P7 named absence/existence checks,
I38 incomplete-read honesty, ADR-092's standalone library boundary. No new
revisit result, future capability, language support, public API or compatibility
change is asserted. Source-only facts below are distinct from runtime proof.
Reuses the existing command, supervisor, Reader, dialect inventory and native
fixtures. No duplicate benchmark product, parser, scoring engine, dependency,
knob, pack or fixture generator is added. W3-2 puts any necessary scratch runner
assertions/real RED in its implementing reading lane, never in this author draft.

## Existing instrument and exact workload

`cmd/fathom-lang/main.go:119,170-248,283,334` dispatches the existing standalone
extract and dialects commands. `cmdExtract` defaults to extractSupervised
(`supervise.go:228`), launching the SAME executable's private supervised child,
which constructs a new Reader (`supervise.go:88`). One invocation here handles
ONE file, no siblings/walk/pack/cache flags and no -in-process. Result JSON is
schema_version 1 with exactly one files entry containing path/error/result.
An unreadable file has no successful Result; valid CLI exit alone is insufficient.

`pkg/lang/reader.go:224,248,698` exposes Dialects/Extract and excludes unlinked
families and DoorReadable=false. `internal/langroute/register.go:818-841` refuses
swift/tcl/vimscript and excludes the four product-only fathom-* formats;
`internal/langlink/langlink.go:21` excludes cue/timoni with no optional CUE family.
`main.go` imports the default pkg/lang, no optional family. The manifest-derived
expected population at the source base is 114 IDs, including native go, templ,
generic-json and generic-toml. This is a SOURCE inventory, not an observed binary
roster. The actual pinned binary's `dialects` JSON must corroborate it before any
trial. A new/missing/duplicate ID or failed roster capture is INCOMPLETE, never
permission to shrink the population to the dialects that happen to run quickly.

One representative file per exposed dialect is selected BEFORE timing, by the
existing routing-test order in `pkg/lang/dialects_test.go:28-105`:

1. If ownSamples has the ID, use exactly its decoded src bytes at its path.
2. Otherwise consider every file under that ID's testdata directory except
   *.golden.json, sorted lexicographically by path relative to testdata.
3. In untimed preflight, use the pinned standalone default-supervised extract on
   each candidate in order, keeping every preflight result. Select the FIRST with
   exit 0, valid v1 JSON, one nonnull result, empty error, exact expected dialect,
   empty skipped and incomplete=false. OwnSamples has only its one candidate.
   No candidate passing these checks means named missing-workload evidence,
   INCOMPLETE for that dialect, not an excluded dialect or an invented fixture.
4. Freeze selected route, byte length/SHA256, source identity and raw routing
   receipt in the preregistration before trial 0. No later substitutions. A source
   or selection repair requires a new immutable preregistration before outcomes.

The annex pins EVERY candidate set (553 candidates across 114 IDs, 18 ownSamples
entries). Its first path is NOT a claim it routes correctly. All 553 candidates
are fixed source bytes; only one per dialect becomes the timed workload under
the rule above. Copy fixture bytes under a separate root per dialect, preserving
the relative route exactly; the timed root contains only the selected file.
No directory-wide scan or synthetic proposed-edit reconstruction is timed.
Reader's incomplete=false means no reported failure, not complete semantic
coverage: recovery/line scanners can accept syntactically broken fixtures
(`pkg/lang/types.go:57-64`). This is latency of this fixed corpus, not all file
sizes, every edit form or proof that CHG-8 reconstruction already exists.

## Cold conditions and the exact clock

Before collection, the reading owner pins the main revision containing this
approved method, clean source identity, exact binary SHA256/build/toolchain and
OS/architecture, source/grammar-lock hashes, working directory and subprocess
environment, exposed roster, selected bytes/routes, routing receipts, clock
implementation/resolution and process/capture adapters. Use a built binary;
never go run, a build/test/ingest or compilation as part of a trial. Build
preparation, if needed, belongs to the reading owner under the separate EX lease.

Each trial launches a NEW standalone parent and NEW supervised child. No shared
Reader, in-memory grammar pool, enabled wazero disk compilation cache, cache
prewarm, -cache, -pack, -walk, -siblings or -in-process. Keep the existing defaults:
30 s per-file deadline (`pkg/lang/reader.go:39`) and 4GiB child limit
(`cmd/fathom-lang/supervise.go:46`). These safety limits are NOT the 500 ms bar;
do not lower them to censor slow observations. Default values remain withheld;
no schema catalog/provider, daemon RPC or persistent store is read. No daemon is
started/stopped or reconfigured for this reading. An actual connection, missing
isolation evidence or unavailable process/supervision identity cannot earn PASS.

Frozen trial argv, flags BEFORE route:

```text
/usr/bin/time -p <pinned-fathom-lang> extract -root <selected-fixture-root> <selected-relative-route>
```

A standard-library parent capture adapter uses time.perf_counter_ns (monotonic):
START immediately before spawning that argv, STOP immediately after communicate
has drained stdout/stderr and the wrapper/standalone/supervised child have exited
and been waited for. Include process creation, wrapper overhead, supervisor and
Reader/grammar construction, file read/extraction, JSON encoding/output and pipe
drain. Do not subtract wrapper, startup or teardown, or start at child-ready /
Reader.Extract. Python/controller startup and writing the already captured raw
artifacts after STOP are outside. Record started/finished UTC for provenance,
but elapsed_ns is monotonic STOP-START, never subtraction of wall timestamps.
The /usr/bin/time real/user/sys output is a cross-check, not the headline clock;
rounded real=.50 alone cannot distinguish 500,000,000 ns from 500,000,001 ns.

Cold means fresh application/grammar state with disabled compilation cache.
It does not claim cold OS page/disk caches. No system cache purge, global config
change, binary rebuild between trials or subtraction of an estimated cache effect.
Preflight may warm OS pages, disclosed in the reading. Hold workload, binary and
environment fixed; record resource/lease conditions and disturbances. A process
restart caused by an observed fault does not become a fresh replacement trial.

## Trial accounting, median and verdict

Read the live gate/lease state first. The reading owner alone takes the existing
EX heavy lease for the timing collection, with no competing gate/build/ingest or
package test. No nested SH/EX acquisition. This author acquires neither lease nor
measurement. Run after the method is on main, once as W11-3 directs; no paid arm.

Freeze order: ascending dialect ID, trial_index 0,1,2,3,4 for each dialect. Exactly
five observed trials, each a fresh process, retaining ALL raw stdout/stderr,
argv/process identities, exit/timeout/interruption, UTC stamps and elapsed_ns.
Keep launch failures, extraction errors, slow cases, incomplete reads and killed
children. No outlier removal, replacement, favorable retry, fastest-of or warm-up
trial hidden in the five. An interruption leaves a named missing index; do not
silently rerun to fill it. Preflight is separately labelled and never one of five.

For five recorded elapsed integers, median_ns = sorted(elapsed_ns)[2], including
the durations of recorded unsuccessful trials if available. Successful workload
validity is a separate mandatory check on ALL five, never selection of a faster
subset. Median can be displayed beside a failure but cannot cure it. Missing
elapsed values mean no complete median; do not impute zero, cap/replace a timeout
or compute a median on four values. Every individual trial is published, including
trials above 500 ms even if the median passes. No maximum/p95/p99 alternative or
pooled-across-dialects statistic decides CHG-M2.

Per dialect: PASS requires full identity/coverage/preflight and all five valid
extractions, with median_ns <= 500,000,000. Equality passes; +1 ns fails. A complete
observed extraction/cold-condition violation or above-cap median is FAIL with its
witness. Missing/unsupported/unreadable roster, fixture, capture, fine clock,
process identity or trial is INCOMPLETE with named input/index, not product success.
If both failure and missing inputs exist, publish both and retain the observed
FAIL. Global PASS requires EVERY exposed dialect PASS; any observed FAIL gives
global FAIL with incomplete groups still listed; otherwise any missing group gives
INCOMPLETE. No dialect weight or silent exemption changes the cap.

Publish `CHG-M2 verdict: <PASS|FAIL|INCOMPLETE>` (replace the bracketed choice)
as the complete opening line
in the existing reading destination. Publish source/binary/fixture identities,
clock/cold/cache/daemon/lease conditions, one row for each dialect/trial with exact
elapsed ns and outcome/capture references, each dialect's median/verdict/coverage,
all unknowns, and overall verdict. Public records use repository-relative
artifact identifiers/portable aliases, no private paths or raw sensitive content;
full raw captures remain under gitignored workspace with immutable digests. Hashes
without readable source/captures do not constitute independently reviewable proof.
Do not flip CHG-M2 or CHG-8 boxes here; the foreman applies the normal verify gate.

## Exact prospective handoff and assertions (not existing runnable proof)

The standalone command and its source tests EXIST. A fine-clock collector,
process/daemon evidence adapter, frozen binary roster/selected-routing receipts,
five-trial raw captures, median grader and CHG-M2 reading DO NOT exist in this
source checkout. internal/hookpath/RebuildProposedFile is also absent. No guessed
command for those is advertised as executable product proof. The prior scratch
checker/layout is PROPOSED only and its old twenty/max schema cannot grade W11-3.

The reading owner receives this precise workspace-only handoff, no production
file grant: `workspace/tmp/reading-hook-latency/{capture.py,checker.py,contracts.py,
test_checker.py,manifest.json,normalized.json,result.json}`, raw/, selected/ and
fixtures/<exact-test-name>/{input.json,expected.json}. Reuse the earlier scratch
protocol entrypoint names and native dialect/fixture/trial adapters; implement only
what is missing and bind this method's five/median policy. No new public schema
fields, package or external dependency. Raw native outputs remain v1 dialects /
extract JSON plus fine monotonic capture and existing time -p diagnostics.

The prospective capture adapter command is `python3 capture.py collect --manifest
manifest.json --output raw`. It validates the frozen manifest, records actual
process/supervision/daemon-isolation evidence, executes exactly the fixed argv
and order once under the reading owner's EX lease, captures monotonic elapsed
and native outputs without modification, and retains failed/partial attempts.
This interface is not implemented or invoked by the author; inability to supply
its evidence is a named missing handoff, never fabricated cold-process proof.

The manifest binds source/binary/environment, raw artifact digests and source refs,
roster, all expected dialects, selected fixture hashes/routes/preflight receipts and
policy authority before trials. The existing proposed u3_policy slot must read
id=five_median, repetitions=5, quantile=0.5, maximum_statistic=false,
budget_ns=500000000 with immutable W11-3 ruling refs. Its old pending/twenty_max/
hundred_p99 enum cannot remain accepted. This narrows existing scratch policy
slots, introduces no future field and requires an exact adapted source handback
before invoking a checker. No author-side schema/tool implementation is claimed.

contracts.py guard statically verifies all five named ContractTests methods and
all cases below; validate checks the frozen artifacts/policy identities and closed
adapter fields before normalize/grade. checker.py normalize preserves native
lineage/counts/errors and never synthesizes absent facts; grade recomputes the
five-trial median and per-dialect/global mapping above. Exit 0 means valid grading,
including FAIL/INCOMPLETE, not a passing reading. Missing input is explicit coverage
and INCOMPLETE; unsupported adapter or inability to read/write is nonzero, never a
synthetic measurement or a performance FAIL attributed to product code.

| Prospective ContractTests method | Every required assertion (W3-2, real RED belongs to implementer) |
|---|---|
| TestColdDialectRosterFreeze | all exposed IDs included; refused/product-only/unlinked exclusions independently grounded; absent roster/new ID/missing candidate INCOMPLETE; bytes/routes/hashes/preflight frozen before timing; a changed fixture/binary requires new preregistration, never outcome-selected substitution |
| TestColdExtractCommandShape | flags before one route; fresh default supervised parent/child each trial; startup/JSON/teardown inside clock; no cache/pack/walk/siblings/daemon/warm/in-process; missing process/daemon/clock evidence INCOMPLETE; rounded .50 cannot decide 1 ns boundary |
| TestColdExtractCorrectness | exact dialect/non-null v1 result/empty error/empty skipped/incomplete=false/exit0; malformed/empty output, wrong route, unreadable, nonzero, killed child fail; missing capture unknown; values withheld; source routing test is not cold proof |
| TestColdTrialAccounting | exactly indices 0..4 per dialect; missing/duplicate/extra index, absent elapsed and unreadable/torn capture cannot PASS; failure/timeout/slow outlier retained; no replacement or median-of-four; preflight separately excluded; cold process identity changes each trial but binary/fixture identity stays fixed |
| TestColdBudgetGrader | five elapsed values [1,2,500000000,700000000,900000000] pass latency only, all valid required; [1,2,500000001,700000000,900000000] fail; observed invalid extraction still fails with fast median; any missing dialect/index INCOMPLETE; observed FAIL plus missing remains FAIL and names gap; reject old20/max or100/p99/pending policy; global AND never pooled median |

These commands are PROSPECTIVE and UNRUN; guard all files first, after the reading
owner supplies the exact source/fixtures and lease-compliant raw handoff. They are
synthetic adapter checks/normalization only, not a benchmark launch:

```sh
set -eu
cd workspace/tmp/reading-hook-latency
for f in capture.py checker.py contracts.py test_checker.py manifest.json; do test -f "$f" || exit 2; done
python3 contracts.py guard
python3 -m unittest test_checker.ContractTests.TestColdDialectRosterFreeze
python3 -m unittest test_checker.ContractTests.TestColdExtractCommandShape
python3 -m unittest test_checker.ContractTests.TestColdExtractCorrectness
python3 -m unittest test_checker.ContractTests.TestColdTrialAccounting
python3 -m unittest test_checker.ContractTests.TestColdBudgetGrader
python3 contracts.py validate --kind manifest --input manifest.json
python3 checker.py normalize --manifest manifest.json --output normalized.json
python3 checker.py grade --manifest manifest.json --normalized normalized.json --output result.json
```

Do not run capture.py merely because it exists: the third-machine reading owner
runs collection only after the method is on main and its pre-timing manifest,
source adapters, real assertion checks and exclusive lease are established.
No benchmark, test execution, actual RED, capture or measured outcome in this draft.

## Pinned per-dialect candidate inventory

All source paths/bytes below refer to 54bca03154ea35c39bf08acba4c3572e843da56a. Source sets are determined by the
routing-test rule, not timing. For each ordinary set, enumerate all regular files
recursively except *.golden.json; relative routes sort lexicographically. OwnSamples
means the exact path/src pair at `pkg/lang/dialects_test.go:53-72`: decode the Go
string literal (backticks verbatim) into UTF-8, never hash its quote/escape token.
For each set form an array of objects with path (relative route), sha256 (materialized
bytes), bytes (integer length). Serialize using Python standard-library
`json.dumps(rows, ensure_ascii=True, sort_keys=True, separators=(',', ':')) + '\n'`
and SHA256 those UTF-8 bytes. This pins every candidate, including route-miss or
negative fixtures. Recompute against the source snapshot before using the table.
Native materialized hashes must match decoded bytes, not the old draft's literal-
token hashes. Actual roster/routing still needs runtime preflight, not this table.

| Expected source ID | Candidate source | Count | First candidate route (not claimed valid) | Candidate-set SHA256 |
|---|---|---:|---|---|
| ada | `internal/extractor/dialects/ada/testdata` | 6 | `broken.adb` | ec3a2ecdb37a2247391ca11b1e7f79266ac2ce7fd0f6c3e4a475b597055f8dee |
| alertmanager | `internal/extractor/dialects/alertmanager/testdata` | 3 | `alertmanager.yml` | a455d1f26334d5aa1249e94b8aacefcb59187d86e1e9ca9e349db3db6a660281 |
| ansible | `internal/extractor/dialects/ansible/testdata` | 7 | `defaults.yml` | f98816eb16ed844de964a81c42e269696f7c097a57bd77ba3deb105d7c06b80a |
| argo | `internal/extractor/dialects/argo/testdata` | 6 | `applications.yaml` | 985a8829b377fb4c03028cbbc064134afc34dfa8ceb7a47b7fb5d01623d9d9eb |
| arm-templates | `internal/extractor/dialects/arm-templates/testdata` | 7 | `artifacts.json` | 7c7dc5aa5057c0d9083b4fc70ff486da594d89465751df0e28dfea7aa3e9df87 |
| astro | `internal/extractor/dialects/astro/testdata` | 4 | `broken.astro` | a36a654f320189fb028ccdea0eaf65ed35f6eb1628d47c471b92e6bc18671b1a |
| aws-cdk | `internal/extractor/dialects/aws-cdk/testdata` | 3 | `java/cdk.json` | 6d4c9ac9bf7fa39cd80d9ec9589802918018aecf0e509a2d2d293ac334ca967b |
| aws-iam-policy | `internal/extractor/dialects/aws-iam-policy/testdata` | 5 | `basic.json` | fa26f0e6d06114cdd3c2ca8b450444c7c3a08eb45422ee55e321e9f4596e4ce3 |
| azure-pipelines | `internal/extractor/dialects/azure-pipelines/testdata` | 3 | `invalid.yml` | b1b5d86e57ccce3ad8e99d432e22b34445ab0e969f107302fc9aaeb7ed6577e0 |
| azure-policy | `internal/extractor/dialects/azure-policy/testdata` | 6 | `definition.json` | 7382758ab8bec8ea462f00d9a53e929c9e8eb07586c13f123ea23b46e89173de |
| bash | `internal/extractor/dialects/bash/testdata` | 4 | `broken.sh` | d83f096a0eec31b7663f0e3b930eb4a4bbcfe8a85ca86785f82251d57d8b27ec |
| bazel | `internal/extractor/dialects/bazel/testdata` | 5 | `BUILD` | edbf28925510986adb8ec9cfbebac39541186a23c958276f2cf56f586385f20c |
| bicep | `internal/extractor/dialects/bicep/testdata` | 5 | `broken.bicep` | 50fd0c7a10ea4aedb5aa3ad3e884e4788a03ca753591d2c50585b3b8c143910b |
| c | `internal/extractor/dialects/c/testdata` | 5 | `broken.c` | 0fe95af1cfd8e70c760972e91744fbc1946645c2e75f507b137422cedd47fe97 |
| caddyfile | `internal/extractor/dialects/caddyfile/testdata` | 3 | `single.caddyfile` | c6d6aac2a980d8469e1ab02fc6df8b669168413e539b59b4719f7f8c76b7362b |
| cargo | `ownSamples[cargo]` | 1 | `Cargo.toml` | b5cdc8a8842b8714d78b15f1956966a7804a211e67b958acb6db5de3a68398d2 |
| checkov-policies | `internal/extractor/dialects/checkov-policies/testdata` | 9 | `broken/checks/resource/broken.py` | 38d0d50001884d4cf48798827ee693e6f353b2b0490de6327980bdff277ec4da |
| circleci | `ownSamples[circleci]` | 1 | `.circleci/config.yml` | 201aaeb1379314c4a478397e36334862338875cc48af0c451218955efd8b5727 |
| cloud-custodian | `internal/extractor/dialects/cloud-custodian/testdata` | 4 | `basic.yaml` | 0f40e55f8622a497a5a5accdb0b57fb6dc239649ca09b2002670a6d014305e27 |
| cloudformation | `internal/extractor/dialects/cloudformation/testdata` | 8 | `basic.yaml` | 3f4f1f827b16046035637831269b0efa79699eecab9ca454629e6d381f361cae |
| cmake | `internal/extractor/dialects/cmake/testdata` | 5 | `broken.cmake` | c743dfc7444df916c7e3f0ae40d57a44a31dd2c69af2b49d9ba15ce13aa4a645 |
| commonlisp | `internal/extractor/dialects/commonlisp/testdata` | 6 | `broken.lisp` | 02eefd413d31422ddcc3233bead2f6b2f0ef1b07155e46f4bf821ad38e5a92db |
| compile-commands | `ownSamples[compile-commands]` | 1 | `compile_commands.json` | dc81d27e36cfd4438528c478c256ab7be8e1f2d0334802518fc2f33f955c7f95 |
| composer | `ownSamples[composer]` | 1 | `composer.json` | 8cbab37cc97be87797da6cb2bcffe4a2d3ec8792f0bc5d8a78ea42f024128533 |
| conan | `internal/extractor/dialects/conan/testdata` | 6 | `broken/conanfile.py` | 9ae6dd664be737497faac3fb2623fb979f6c47696202ad22293a2943baadd0f4 |
| cpp | `internal/extractor/dialects/cpp/testdata` | 10 | `broken.cpp` | 1fa0b73eb55699bc2ef03b413310163a21e28d9bc1478858159d753255f059ed |
| crossplane | `internal/extractor/dialects/crossplane/testdata` | 6 | `composition-legacy.yaml` | 5c98f7015c53ec34f365a2e4fb876312d81e3f498d9e48149f8eb5f4fb346e52 |
| csharp | `internal/extractor/dialects/csharp/testdata` | 9 | `branches.cs` | 5e4ad23e6e30184b444a7e592fb7e29edf206f76d610809e7d6f4edb32e500c7 |
| css | `internal/extractor/dialects/css/testdata` | 8 | `broken.css` | c2952855b88995650e887c1c4712dc62d8f454d333a43243430d38b0b5fc46cf |
| cuda | `internal/extractor/dialects/cuda/testdata` | 7 | `align.cu` | 9d4bd856e118111b35b67f4f6a589ac921f394d64cf2611daf905ad56fde695f |
| dart | `internal/extractor/dialects/dart/testdata` | 8 | `broken.dart` | d192e22cfa9219ff100f81cdadd8c3aa590af66990bf61c86416655c31184f39 |
| docker-compose | `internal/extractor/dialects/docker-compose/testdata` | 7 | `basic.yaml` | f72ea5fd70fe121c24fb58146c7f9e206c7a33cad5b9d520c2efb8a9634dcb15 |
| dockerfile | `internal/extractor/dialects/dockerfile/testdata` | 4 | `args-env.dockerfile` | 2f5fab4e32cd4e8a621ea2c148122ddfd63ca9a189ae87cc6468ba95ecf6c518 |
| elixir | `internal/extractor/dialects/elixir/testdata` | 5 | `broken.ex` | 0f4a8096c79665bdd48ca7e39d61935718b1ebec88106de6a8653debbc7bc704 |
| emacs-lisp | `internal/extractor/dialects/emacs-lisp/testdata` | 6 | `broken.el` | aa44ea12ef7d4c3c639eb7685a1cee7296c836e3e0ec87917460ef2d22b2c2ff |
| envoy | `internal/extractor/dialects/envoy/testdata` | 8 | `bootstrap.json` | 8475f00d37b64c77e2f202e47b29170f405852e2f4d0c6ee7929048d8ea1fe19 |
| flux | `internal/extractor/dialects/flux/testdata` | 5 | `kustomizations.yaml` | 72996cd073dd124f49832f64ff2e3d31bd87634a1edd80781534031cd9120293 |
| gatekeeper | `internal/extractor/dialects/gatekeeper/testdata` | 5 | `constraint.yaml` | 24cf1f043e0359235e3cbac402dc409c908f373dbef6b832ed89d581048dced0 |
| generic-json | `ownSamples[generic-json]` | 1 | `data.json` | f9478b54919362c6ed83e62ba57d4348bacb45f21b6b7e6ca84918e0979b6535 |
| generic-toml | `ownSamples[generic-toml]` | 1 | `data.toml` | 4059bec15df3dc1a314f624ace36e96768644cf848d54dcdbcf23cc49c91f358 |
| git-hooks-config | `ownSamples[git-hooks-config]` | 1 | `.pre-commit-config.yaml` | 543559cdbf9717c475aa9acf7df204859666bde508498e0c5caafda11d5b1f02 |
| github-actions | `ownSamples[github-actions]` | 1 | `.github/workflows/ci.yml` | a9b11e67a979e219d68bb3c3b54e9acc229975aa10dd8819b0e81b5d7921390d |
| gitlab-ci | `ownSamples[gitlab-ci]` | 1 | `.gitlab-ci.yml` | 68d0c932bcc0827bb26604a179b396ea58b129772425c37924dd012f60f05aa8 |
| glsl | `internal/extractor/dialects/glsl/testdata` | 6 | `broken.glsl` | d4dc08330af7b700e5779d051c0eb457f0ec18067ab72ba4c5ca0bd3374a0c46 |
| go | `ownSamples[go]` | 1 | `a/a.go` | 39eb42ac2453862f7b335bfff9fd24f4743e560dc30e45eb094da8eaa29c99f5 |
| go-mod | `ownSamples[go-mod]` | 1 | `go.mod` | 286d38f0930e1c4f68aa061eb1d25b6c331d46dde05676d71463739deb72b951 |
| gradle | `internal/extractor/dialects/gradle/testdata` | 8 | `app.gradle` | bf5ac2bc9281009f77faa89871dad2ac1d490d47eb7f3d71df13be61b1809cd5 |
| grafana-dashboard | `internal/extractor/dialects/grafana-dashboard/testdata` | 7 | `alerting.yaml` | e668728ff78cb71e288cf0a82e01ca3039e4af92228eb7ec9c44832179312a3a |
| graphql | `internal/extractor/dialects/graphql/testdata` | 4 | `broken.graphql` | 69477c81c7a288f6aaf5595e44f0142f7bc70b8b4a04fb3a0a61bfd9788020eb |
| haproxy | `internal/extractor/dialects/haproxy/testdata` | 3 | `basic.cfg` | 34933327a4efc03e038bd4a56de26b6a948898e1c586d1331e010d6f253280a5 |
| hcl-tools | `internal/extractor/dialects/hcl-tools/testdata` | 5 | `build.pkr.hcl` | fc4f0dde63b0d2623b8c4d9081a2ec6fdad9a2fb73c83738df8753c89014cc16 |
| helm | `internal/extractor/dialects/helm/testdata` | 13 | `broken/Chart.yaml` | b0debd6cdef9c3ebe7c0b9a6b50d333084340b8a2b1ea25d4804d25c7ce7f440 |
| hlsl | `internal/extractor/dialects/hlsl/testdata` | 10 | `bindings.hlsl` | 15b523a28520bdd48c9337d10f20eee8d01cb8dfcf1fde44da7bfc1074e45837 |
| html | `internal/extractor/dialects/html/testdata` | 3 | `broken.html` | 757fb43bc820b3f2c3be3532b1b4b1e266bdea72710a757a4229c81416deaef3 |
| java | `internal/extractor/dialects/java/testdata` | 4 | `broken.java` | d780427909c0837976fa82908c9ad39132aba291b8b8b49d66b04d57132cdfda |
| javascript | `internal/extractor/dialects/javascript/testdata` | 9 | `assign.js` | 9f9dcae421fcf1243673b4b2859d83f90ca08c826d4eb97e3f9befee8aa75121 |
| jenkinsfile | `internal/extractor/dialects/jenkinsfile/testdata` | 5 | `buildApp.groovy` | 29e5a54d6fc09f7797dcdea4b022eda819a9fbf5c2851f50c065ec24e33a72f6 |
| jinja2 | `internal/extractor/dialects/jinja2/testdata` | 5 | `base.html.j2` | 05d54c9187794261ddb39c2ec9ec74724ed4da16be147c177bedb9af9cd65c3f |
| json-schema | `internal/extractor/dialects/json-schema/testdata` | 5 | `legacy.json` | 410968902bb8edda109b9a6de835a6449cc80ecbaf2774344a38fe1c2895ef8d |
| jupyter | `internal/extractor/dialects/jupyter/testdata` | 7 | `basic.ipynb` | 256f3f9d85cf627adad7223d5b91b91e52d46ae47fd4d379096eb25b95a389fd |
| kong | `internal/extractor/dialects/kong/testdata` | 4 | `flat.yaml` | 42c3b72255778dda813351082424b0ef026059a72f2d034e5d93d3335a4976e5 |
| kotlin | `internal/extractor/dialects/kotlin/testdata` | 6 | `broken.kt` | 6253d5e723cec00a8cad281002d4b3cdce6c050eaeb72cdf421eb45c0deca509 |
| kubernetes | `internal/extractor/dialects/kubernetes/testdata` | 8 | `basic.yaml` | b3102026001f8848c7fa1f7021eebb5ce624922bbbf70451a775ac40ad010b68 |
| kustomize | `ownSamples[kustomize]` | 1 | `kustomization.yaml` | 3d22e176d930e2e4b6f36a0cab4e9880cafad36607879465c69a55d9b5c9258f |
| kyverno | `internal/extractor/dialects/kyverno/testdata` | 6 | `cluster-policy.yaml` | d60cc0b24410b934e1b044bf22763adcc56b7006d1c8bee2195000b1cee1d8a6 |
| lua | `internal/extractor/dialects/lua/testdata` | 7 | `bodies.lua` | cbc0a64d6fa208ab53e73d45ece69738ef7e923ac5654f3d1f79670b9c8c5a96 |
| luau | `internal/extractor/dialects/luau/testdata` | 4 | `broken.luau` | 07a3fe765d39a679183eaf5c21727b452acb877d75df37e63b6a310ea9f03578 |
| makefile | `internal/extractor/dialects/makefile/testdata` | 7 | `basic.mk` | 6f2f7137cf71da9b742d9eee424591f901ddfd3917221b5a7b1fb3347b169648 |
| markdown | `internal/extractor/dialects/markdown/testdata` | 7 | `basic.md` | d4b7b77babc00aaa47ac8cd5f34f7eda0b5524489b2429cdbc36d16ad18b2289 |
| matlab | `internal/extractor/dialects/matlab/testdata` | 6 | `abstract.m` | fe8a2156b8133dd5cb282af4788f6a8dc8abd23b2d37ab176b6a3c3892beea5c |
| maven | `ownSamples[maven]` | 1 | `pom.xml` | 8d2aec233e5c3ebe1fa1bbb0812e0585a9a746c57c93f23bd04b0b7649b28b67 |
| meson | `ownSamples[meson]` | 1 | `meson.build` | 91dfd26a2d8ef08d483f529561d47a494c02c5b08af0e37e4171d9286b9420b1 |
| msbuild | `internal/extractor/dialects/msbuild/testdata` | 6 | `Directory.Build.props` | 1a033f6d8de5bb8ea5e33049b632f75fb12261018aeb231bfc63421666d089db |
| nginx | `internal/extractor/dialects/nginx/testdata` | 5 | `fragment.conf` | 0a1d303ff4722857620c8291e0352a3d6632ec93a577cef0cb8cad2d18cfecae |
| nix | `internal/extractor/dialects/nix/testdata` | 7 | `broken.nix` | 05ea855ddc38d8f8a7ae74f5561926f54b217e3bc05db1e4960c050f1a99fe14 |
| objective-c | `internal/extractor/dialects/objective-c/testdata` | 8 | `Shape.h` | 481da2f6b4272c4b0e92ed0590970d25101e78b6cc08756fc3f8337ddc6548be |
| openapi | `internal/extractor/dialects/openapi/testdata` | 5 | `broken.yaml` | 6e1e760bae8263960555dd27bf6fb82b42360e4451f85e847ce5fabb4c320ebc |
| otel-collector | `internal/extractor/dialects/otel-collector/testdata` | 3 | `gateway.yaml` | f3c9c6f0d6c774d5044e7b6913d972e69ec0cf31c448172301844c3a57355a02 |
| package-json | `ownSamples[package-json]` | 1 | `package.json` | 25527aee22cc1bcb32d51af1d073409d47fc22541d15270da94b4a7a4257809a |
| perl | `internal/extractor/dialects/perl/testdata` | 7 | `basic.t` | ffc5dc8dd6c6afae8291d545e85080cb79cffa0744045d4d9446074e714de12b |
| pgbouncer | `internal/extractor/dialects/pgbouncer/testdata` | 3 | `broken.ini` | 661120a0a2a346cf04a14b29c4b6394b251c5ba82c6fe1521b35b63b3a2fafbe |
| php | `internal/extractor/dialects/php/testdata` | 7 | `braced.php` | 38fb2ef99362fb9208726abf939a65bd857e4d8a44043548f34ffe13ef2c0019 |
| powershell | `internal/extractor/dialects/powershell/testdata` | 7 | `broken.ps1` | 3c31c1433b66e60f37d8753d3bc3d76ecc9770e6d0a5fdbf2d056b0c53caaca0 |
| prisma | `internal/extractor/dialects/prisma/testdata` | 4 | `broken.prisma` | 7d5b558c74a3c2c2adb7043b626a6038b4ce7de354bb1ca844f50111b2ca8737 |
| prometheus | `internal/extractor/dialects/prometheus/testdata` | 4 | `checkout.rules.yml` | 13a87ae6fddcfea890078b29d6afc7f6cf8734dd540e0c9d9d11b25ce0f68ee8 |
| protobuf | `internal/extractor/dialects/protobuf/testdata` | 4 | `broken.proto` | 1e78c438aee87db8b6a1a97e60637873ffc2318ac91b12e313008a9dcf178e2b |
| proxysql | `internal/extractor/dialects/proxysql/testdata` | 3 | `proxysql-broken.cnf` | 5344ee8cd7f5f028634fb64cafd02a40f1706a7b62873953a575dc45e80891e0 |
| pulumi-yaml | `internal/extractor/dialects/pulumi-yaml/testdata` | 3 | `project/Pulumi.yaml` | 1edad5f6defddf05f7a7439c13dea1fbcba883a68474a6bea4b2a61fc16a1f6b |
| python | `internal/extractor/dialects/python/testdata` | 6 | `broken.py` | 0a0db08a04cc921588b708d58deb6141e4756fe9769fa7a59eace84f088a02a7 |
| python-packaging | `ownSamples[python-packaging]` | 1 | `pyproject.toml` | 0728a85eee070c97983a79417e225aa7d7d33d2e329e3cc9ce8451ab81ca51a0 |
| r | `internal/extractor/dialects/r/testdata` | 5 | `broken.R` | e7228668edef60c446d350fa30c9109999a81c6c70097709b1ebe00d0f28cdc8 |
| rego | `internal/extractor/dialects/rego/testdata` | 5 | `authz.rego` | adc955177a6119e3a0ebb1a7049f90a043f99471e6e9180e151ac1dd12c7a050 |
| ruby | `internal/extractor/dialects/ruby/testdata` | 7 | `broken.rb` | a49854139f4feb4eb3c79a7fa22a1181a4e9901149b7468686fae583421c3a42 |
| rust | `internal/extractor/dialects/rust/testdata` | 8 | `broken.rs` | dd61d85cd62d2d70b9c1a4ca20ab522d8d55eec1f67cf79b0988bb38a0743f24 |
| scala | `internal/extractor/dialects/scala/testdata` | 5 | `broken.scala` | 9b2cb703b79cf254c52e48d3636e9c0ef9c73809d28e024a712a21924eee2a98 |
| scss | `internal/extractor/dialects/scss/testdata` | 3 | `broken.scss` | bcefd3a9e381969eb947230a06b2b050207a30d13ec6ea87ce97ac9cafe18697 |
| serverless | `internal/extractor/dialects/serverless/testdata` | 4 | `api/serverless.yml` | 7c7700baef187aeef37a4cef2046d71beefac4a1fb314f282de43f57d3e2a0b5 |
| simulink | `internal/extractor/dialects/simulink/testdata` | 8 | `broken.mdl` | ce7d823cf8f0d9eb39ff4e01eaac1ba1e5f292d916df4cfc050d7921d842c5b9 |
| solidity | `internal/extractor/dialects/solidity/testdata` | 4 | `broken.sol` | e066d6bbbff791140f4f1082d560992b90d01e39310731bc178410f06ce6b60a |
| sql | `internal/extractor/dialects/sql/testdata` | 4 | `broken.sql` | 921fa682fb4b610024fe48df1a1efe33f8e05288cfba3bc525cf266bf369d70c |
| svelte | `internal/extractor/dialects/svelte/testdata` | 3 | `broken.svelte` | 533ed962bb1d1b9a41e44065645aee76f5a13fe91f825ec3750882adef5ed3d3 |
| taskfile | `internal/extractor/dialects/taskfile/testdata` | 5 | `basic.yml` | c91387525f6382828435a5446868f2a4d50067d8bd4d58b6625642717833f7ca |
| templ | `ownSamples[templ]` | 1 | `ui/a.templ` | 68411b3ead1144b081ec4862d65ce5f20e0f73d1a8786d3a3b18bf3efd3a2116 |
| terraform | `internal/extractor/dialects/terraform/testdata` | 4 | `basic.tf` | c10f2f44a3ee86b34786aa90c1fe0c6033329817f680810fc016993afdf3aa69 |
| terragrunt | `internal/extractor/dialects/terragrunt/testdata` | 4 | `external.hcl` | 84364f45a2af00c9626825422ddfdbd065382611f18fb4a37e779ed6a4c1810e |
| thrift | `internal/extractor/dialects/thrift/testdata` | 4 | `audit.thrift` | bbbb137483e69ef447c0982e2509a7b34ab1a1119d2b07882bc089c83fd283d3 |
| tiltfile | `internal/extractor/dialects/tiltfile/testdata` | 3 | `Tiltfile` | 7cff4e239a226d669829a3d5ef597324aff163d43027048ee13d89f3ef201a16 |
| traefik | `internal/extractor/dialects/traefik/testdata` | 5 | `dynamic.yml` | 45ce233b8c6c0f1a86e24e34b1cf4c40671ef42e09260e33d1e14521d83bbee3 |
| typescript | `internal/extractor/dialects/typescript/testdata` | 8 | `bindings.tsx` | 3d8be9bfd845e4ed020311ac162f041fa3657ecb8b8ff2dafde8bf06259ac1fe |
| unity-asmdef | `ownSamples[unity-asmdef]` | 1 | `Assets/Core.asmdef` | 4bfbd3876a8e152b25bc28111893ba2910a2624a9da1bab2020be88ba53f3364 |
| vcpkg | `internal/extractor/dialects/vcpkg/testdata` | 4 | `broken/vcpkg.json` | 057075c8f536140e9fb8bbb90a145a11b9fd883c6548c395fb02aa8bf4983952 |
| vhdl | `internal/extractor/dialects/vhdl/testdata` | 5 | `alu.vhd` | fc5ed4130b766abc6ae4f7b17f63e2f5d399e2475d73fe3a77b67f53b33a9566 |
| vue | `internal/extractor/dialects/vue/testdata` | 4 | `both.vue` | 272be81c48b48931a689f2ee7fc791c9932caa9811cdf5236540e74f84e8a046 |
| wgsl | `internal/extractor/dialects/wgsl/testdata` | 4 | `broken.wgsl` | e9bda751ea2bdf2d23f9f2d2bd231fb481ac69bd36938db0ea59ed6c11f0e937 |

## Decision models

None. Timing, deterministic selection/accounting and median are local computation
with no provider, paid arm or language-support decision. No future capability is built.
