# scip-dart — tier-A SCIP index for Dart (LB-Q-dart)

The maintained indexer, `scip_dart` 1.7.0 (Apache-2.0, Workiva/scip-dart), a Dart program over the SDK's `analyzer`. Bench-only:
installed with the Dart SDK inside a scratch directory, nothing global and nothing in the product.

    curl -sL https://storage.googleapis.com/dart-archive/channels/stable/release/3.13.4/sdk/dartsdk-macos-arm64-release.zip -o dartsdk.zip
    unzip -q dartsdk.zip -d sdk        # dart 3.13.4 (stable), 625 MB unpacked
    export PATH=$PWD/sdk/dart-sdk/bin:$PATH PUB_CACHE=$PWD/pubcache
    dart pub global activate scip_dart # 1.7.0 (needs an SDK >= 3.12), ~84 MB of packages

Run once per package, in the package's own directory, after `dart pub get` (a Flutter package cannot resolve without the Flutter SDK
and is not measured). The root argument must be `.`: `scip_dart ./` treats the package's own pubspec as a nested package and writes an
index with no documents.

    cd <checkout>/packages/bloc && dart pub get && dart pub global run scip_dart . -o <scratch>/bloc.scip

One to seven seconds a package (`Analyzing Source` is nearly all of it). Packages measured (docs/bench/readings/lang/dart.json,
225 files): felangel/bloc `packages/{bloc,bloc_test,bloc_concurrency}` at bloc-v9.2.1 and dart-lang/http
`pkgs/{http,http_parser,http2,web_socket_channel,http_multi_server}` at http-v1.5.0.

What the index carries: `SymbolInformation.kind` on every symbol (Class, Mixin, Enum, TypeAlias, Extension, Constructor, Method,
Function, Variable, Property for an accessor, Field for a field or an enum constant, Namespace for an `import ... as p` prefix and a file,
Parameter, TypeParameter) and an `enclosing_range` on every definition, the whole declaration including its body, so call truth needs
no fallback. Its shapes, and how `internal/langbench` reads them (all scoped to `scip-dart` symbols or to the `dart` dialect):

- A private member (a leading `_`) and a local variable are `local N` symbols. The kind still says what a private one is, so a private
  field, accessor, variable, method or class is a definition (`SCIPDefs` reads its name from the source at the occurrence), and a call
  inside a private method belongs to that method (`SCIPOptions.PrivateMembers`); a local variable (kind unspecified) and a parameter are
  not definitions.
- An unnamed constructor is `Foo#<constructor>().`, named as its class (`Foo`, which is also how `Foo(...)` is written as a call); an
  accessor is `Foo#<get>x.` / `<set>x`, named x; a file is a `/` descriptor and an `import ... as p` prefix a Namespace, neither a
  definition.
- The producer's kind is authoritative: a `get`, `const` or `operator` written before a name is not a different kind (`keywordBefore` is
  for scip-typescript's declaration merging).
- A callable reference is a call site only where its name is written followed by `(` or by type arguments and `(`. The producer also
  references a tear-off (`onError: controller.addError`), an operator and the `super(...)` of an initializer; they are counted as
  `scip_calls_unwritten` and disclosed. A call through a function-typed parameter or field (`emit(state)`) is a reference to the
  parameter, not to a callable descriptor, so the index omits it and the dialect's pair counts against `call_precision`.
- `typed_recall` counts every in-index reference: 46% of them (9,076 of 19,736 across the eight packages) target a parameter, a type
  parameter or a local variable, which no extractor emits as a symbol (`docs/plans/language-breadth.md`, `LB-Q-dart-rest`).

Re-run 2026-10-01 (wave la re-measure, fathom d8ed9a29): Homebrew `dart 3.13.4 (stable)`, `dart pub global activate
scip_dart` 1.7.0 into `PUB_CACHE=<toolcache>/pub` (4 s), then `dart pub get` and `dart pub global run scip_dart . -o`
in each of the eight packages, 3.1 to 5.0 s a package with the pub get (bloc 4.4, bloc_test 3.4, bloc_concurrency
3.1, http 5.0, http_parser 3.7, http2 4.6, web_socket_channel 3.8, http_multi_server 3.1). `lang-oracle -scip
-scip-defs` ran per package against the real package root (0.6 to 1.2 s each) and re-read every L1, L2 and L4 metric
and count identically (sym_recall .9995, call_recall .9971, call_precision .9648, def_mapping .9995, typed_recall
.4331), so no met level moved. New: `import_resolution` reads 1, on 6 of 6 in-repo imports, and that is the oracle's
reach, not the dialect's: scip-dart writes no occurrence on an import or export URI (`import '../http.dart' as
http;` carries only the prefix's definition) and none with the Import symbol role; the only occurrences on an import
line are references to the names a `show` or `hide` clause lists (`export 'src/client.dart' hide zoneClient;`), so 6
of the 752 imports (http 2, http2 4) have an oracle, 389 resolve to a file of the subject with no attestation, and
import edges read precision .0127 (5 of 394), recall .8333 (5 of 6). typed_recall predates F-A (item scip) and is
re-run after that merge.

Re-run 2026-10-01 (wave lb remeasure, fathom fdd2bb59, scip_dart 1.7.0, Homebrew dart 3.13.4): the eight packages re-indexed
and re-measured with the recipe unchanged. typed_recall reads .8464 (11,551 of 13,647 in-index edges) with LB-F55's local
references out of the denominator (6,089 of 19,736); L1, L2 and L3 read as at d8ed9a29.

Re-run 2026-10-02 (wave lx item dart, fathom cd4c1e9e, scip_dart 1.7.0, Homebrew dart 3.13.4, PUB_CACHE in the toolcache): the eight
packages re-indexed with the recipe unchanged and `lang-oracle -scip -scip-defs` run per package against the package root. Two things
about the index changed how it is read. First, scip-dart numbers its `local N` symbols per document from 0 and SCIP scopes a local
symbol to its own document, so a reference to `local 3` is never an edge to another file; the file-edge truth read them as global and
held 577 false edges of 1,194 (LB-F245, O532), leaving 617. Second, the index defines an `import ... as p` prefix as a Namespace and
writes a reference to it at every `p.name(...)`: 242 of the 252 typed edges the SCIP import leaves unmapped (O537). Readings: sym_recall
1, call_recall .9971, call_precision .9648, file_edge_recall .7844 (484 of 617), file_edge_precision .9918, def_mapping 1, typed_recall
.9762 with S1 (LB-F239); every level L1 to L4 reads excellent. To classify the file edges the dialect misses, run `-unmatched <file>`
(oracle_only file edges) and look each pair's occurrences up in the index: the kind of the symbol, the text before the name (`.`, `@`,
`show`) and whether the line is a `///` comment sort them into the classes of O533..O536.

## Issue 191 — preregistered evidence-floor refresh (2026-11-16)

The measurements above are historical. This refresh retains the original eight
pure-Dart package subjects at their existing immutable pins and adds
`dart-lang/test` at `269bdfdea30accc5ec3cd8e41012b024a0a88910`, package `pkgs/test`
(BSD-3-Clause). Selection precedes any fathom/scip-dart reading: a maintained
pure-Dart testing framework with multi-file runner/platform implementations and
named import combinators. Every Dart file the existing harness discovers in that
package is included; no subject source is edited and no output-based exclusion
is permitted. The corpus shard and queue register this augmentation before the
reading. Non-recipe or incomplete populations remain feasibility checks only.

Criterion: retain D19's minimum of 20 attested cross-file import references and
all existing precision/recall/tenancy bars. Use native `scip_dart` 1.7.0
(Apache-2.0), source tag commit `8d017a25874efb8513617e85e508a573692cbb63`,
pub archive SHA-256 `ffd322e41e5e3d31bfa787f1179af7ad4e668b2fb42b94f3e8d788240a312a58`,
and Dart SDK 3.13.4, revision `b530c21f7de367b94fb04787bfed9d8e989d75e8`,
macOS arm64 archive SHA-256
`b36bdca9cd4bf987e1453e1e3ec45fb2c2a8c4df3246b510b66d3922bab04463`.
Resolved producer dependency: analyzer 14.7.0 (pub archive SHA-256
`6c39bd5486a310e56217bed7cb1691377959f31be4227e41750107db7e6dd55c`);
local producer lock SHA-256
`1f0c78acc18abe12daa9333f4ddc7f353d6d0075d3dfc2be3234a55087bb4a2f`.
The new subject workspace dependency lock SHA-256 is
`e60b630088f0de8f710d734f417385e1d5705d6cdd1ea43403abe203e5c8977d`;
all lockfiles remain under local tool/corpus state for critic replay.
Each package runs `dart pub get` then `dart pub global run scip_dart . -o <index>`;
the root is `.` as required by the producer. The harness is
`indexbench lang-oracle -dialect dart -subject <package> -subject-name <name>
-subject-sha <repository-commit> -scip <index> -scip-defs -out <candidate> -unmatched <mismatches>`, once per
package, followed by `indexbench lang-loop -reading <candidate> dart`.
No canonical write is permitted on a level regression.

Search receipt (2026-11-16; public sources, no authenticated requests):

- Own toolchain: `https://dart.dev/tools/dart-compile` and
  `https://dart.dev/tools/dart-vm` document compilation/runtime tooling; neither
  retrieved page establishes an existing import-resolution dump compatible with
  this harness. A new adapter is untried, not absent.
- SCIP/LSIF: `https://raw.githubusercontent.com/sourcegraph/scip/main/README.md`
  lists Workiva/scip-dart. GitHub repository search `scip-dart` finds
  `scip-code/scip-dart`; its public tags include 1.7.0, 1.8.0 and 1.8.1.
  The pinned 1.7.0 producer is the required oracle; newer versions are untried.
  The LSIF site's root and guessed source index/README URLs returned HTTP 404;
  this is a disclosed access gap, not evidence that no other producer exists.
- Language server: `dart-lang/sdk`'s `pkg/analysis_server/README.md` and
  `pkg/analysis_server/tool/lsp_spec/README.md` document the native analysis
  server, LSP support and `dart language-server`. Resolution via this server is
  a named, untried candidate; defining its measurement adapter is outside this lane.
- Registry: `https://pub.dev/api/search?q=analyzer` returns `analyzer`,
  `analyzer_plugin`, `pana` and others. `scip_dart` 1.7.0's version metadata pins
  `analyzer: ^14.0.0` and SDK `>=3.12.0 <4.0.0`. These semantic APIs already
  exist; no new oracle contract is inferred from them.
- Runtime tracing: Dart VM/service documentation was queried
  (`https://dart.dev/tools/dart-vm`, `https://dart.dev/tools/vm-service`).
  The service page returned HTTP 404. Runtime observation is untried and cannot
  establish static imports in unexecuted files; no absence claim is made.

No dialect, grammar, shared engine, oracle scope/pairing or grading logic changes
are proposed. ADR-048 and invariants I38/I54 govern the evidence and prevent a
support claim above its reading. O533–O537 remain historical findings, not fixed
by selecting a new subject. No new architecture decision, future implementation
or compatibility change is needed.

### Result and ceiling

Measured at source `8fd89510675fe8416338d348a81883680cac9e98`, tree
`88c70f1c11158ae51426efba54759a56383c57aa`: nine packages, 332 files,
1,674,386 bytes. The native indexes and each package's harness result were read
once. The first harness invocation was rejected before extraction because the
package subdirectory needed `-subject-sha`; the corrected invocation supplied
its repository's exact pinned commit and reused the existing native index.

The attempted canonical augmentation does **not** meet the evidence floor:
import_resolution 6/6 = 1 over **6 of 1,415 imports**, still below D19's 20.
The new package contributes **0 of 643 imports**, with 21 native import targets
outside its measured files; 210 imports resolve without native attestation.
All original eight packages retain their 225 measured files and their six import
attestations. The candidate pooled file-edge recall is 613/764 = .8024 and
precision 613/618 = .9919. L1 and L2 are excellent, L3 good, L4 unmet; the computed
level is L3. Definition mapping is 2,623/4,243 = .6182 and typed recall
13,139/29,090 = .4517. The original population alone still maps 2,064/2,064
definitions and 10,346/10,598 typed edges; the new population explains the delta.

**Exact scope seam, not fixed here:** the unmodified producer indexes 242 documents
for the new package, including 135 paths beginning `../`; the harness measures
107 files under the package. The index has 48,707 occurrences and **zero Import
roles**. L4 counts 2,179 native definitions against 559 mapped definitions and
18,492 native edges against 2,793 mapped typed edges for this package. A governed
package/workspace extent contract for these native L4 counts is needed before
changing their denominator or filtering the producer output. No engine, API,
truth contract, scope/pairing rule or synthetic import truth was changed.

`indexbench lang-loop -reading workspace/artifacts/lang-dart/candidate.json dart`
exits 1: `regression: L4 excellent -> unmet, level_met L4 -> L3`. Therefore
`WRITE=1` is not used and `docs/bench/readings/lang/dart.json` remains byte-for-byte
historical (SHA-256 `b489f0811b38095e6e54c4a0926bb166f2a3191e54c362322897e3c64a2c5fd1`).
The new corpus pin is preregistration/retry metadata, not a successful new grade.
Scratch candidate, mismatch sidecars and native indexes remain local for a fresh
critic; no support claim or plan checkbox is promoted.

Retry only when an independently selected immutable pure-Dart population has at
least 20 in-scope native-attested import references (or at least 14 additional
ones beside the existing six), **and** the package/workspace extent of its native
index is governed before replay. Merely choosing another pure-Dart package or
installing the already-available newer producer is not this trigger. Revisit
O533–O537 only under their separately owned engine/typed contracts.

All metrics below use tier A native scip-dart truth; each subject's import metric
with fewer than 20 references is thin, unmeasured for grading, and caps L3 at good.
A zero denominator is unavailable evidence, never a passing ratio.

| Subject | Files / bytes | Import agreement / denominator | File-edge recall | File-edge precision | Definition mapping | Typed recall |
|---|---|---|---|---|---|---|
| `bloc/packages/bloc` | 43 / 137963 | 0/0 (unmeasured) | 100/125 = 0.8000 | 100/100 = 1.0000 | 362/362 = 1.0000 | 1690/1690 = 1.0000 |
| `bloc/packages/bloc_concurrency` | 11 / 13146 | 0/0 (unmeasured) | 9/9 = 1.0000 | 9/9 = 1.0000 | 30/30 = 1.0000 | 85/85 = 1.0000 |
| `bloc/packages/bloc_test` | 32 / 66404 | 0/0 (unmeasured) | 43/43 = 1.0000 | 43/43 = 1.0000 | 126/126 = 1.0000 | 674/685 = 0.9839 |
| `dart-test/pkgs/test` | 107 / 750892 | 0/0 (unmeasured) | 129/147 = 0.8776 | 129/130 = 0.9923 | 559/2179 = 0.2565 | 2793/18492 = 0.1510 |
| `http/pkgs/http` | 48 / 176556 | 2/2 = 1.0000 | 131/199 = 0.6583 | 131/132 = 0.9924 | 278/278 = 1.0000 | 1529/1737 = 0.8803 |
| `http/pkgs/http2` | 53 / 374830 | 4/4 = 1.0000 | 165/191 = 0.8639 | 165/168 = 0.9821 | 985/985 = 1.0000 | 5057/5065 = 0.9984 |
| `http/pkgs/http_multi_server` | 5 / 29335 | 0/0 (unmeasured) | 3/5 = 0.6000 | 3/3 = 1.0000 | 60/60 = 1.0000 | 120/141 = 0.8511 |
| `http/pkgs/http_parser` | 19 / 74423 | 0/0 (unmeasured) | 15/21 = 0.7143 | 15/15 = 1.0000 | 112/112 = 1.0000 | 767/768 = 0.9987 |
| `http/pkgs/web_socket_channel` | 14 / 50837 | 0/0 (unmeasured) | 18/24 = 0.7500 | 18/18 = 1.0000 | 111/111 = 1.0000 | 424/427 = 0.9930 |

### Measured-set receipts and checklist

The exact §13.3.1 tracked-set recipe ran before and after each native index step.
Local receipt prefix: `workspace/artifacts/lang-dart/<package>-{before,after}`
(`.json`, `.manifest`, `.untracked`). Both digests are identical per row below;
there are no gitlinks. Symlinks are hashed by target bytes and not followed.
The manifests cover all tracked file types; measured files/bytes in the previous
table exactly match the tracked **regular `.dart`** subset for every package.
The new package's one symlink is outside that regular-file subset. No tracked
Dart input is removed to improve a score, and no submodule content is implied.

| Package | Tracked regular files / bytes | Symlinks | Before = after SHA-256 |
|---|---|---|---|
| `bloc` | 49 / 211344 | 0 | `f710efa6689924e1747c89bd964a4f873aecd6dfd0991516e62f126a7aea9095` |
| `bloc_concurrency` | 19 / 36073 | 0 | `25cf31b26956fccc7742dc010c5b4897cd558fbaed21eb63081f011558de7b29` |
| `bloc_test` | 39 / 100300 | 0 | `afd3e207dec873fcee42c434fdc18db053f53d46b59d82aad7db94b6040f7b7c` |
| `http` | 53 / 200208 | 0 | `e2bcc11a432421cb87672e2358343717c2f0b962efc583abcf397d997e993e52` |
| `http2` | 64 / 386948 | 0 | `0d815e9e1d51a9d0ebcb7388c52113b67c26fbd58dc9efcba8178203cd9198a8` |
| `http_multi_server` | 12 / 36456 | 0 | `a29ac88180c7e1e8e4534980db47c930c3dbcd013228fe9e1df7d71e803578d1` |
| `http_parser` | 25 / 81680 | 0 | `59533c34277c3c9b36cd9a085a2031a00ce2aa8d186734055780ec3facc3a2fd` |
| `test` | 129 / 1656211 | 1 | `33a73b0b74472871743def96cc2a5555956d9ee31f95a726b6f565a691527316` |
| `web_socket_channel` | 20 / 60994 | 0 | `bf63f18368902fbfd2b0ad890ebd7154cd73f7a584a64cf8ef9f821bb834380a` |

Discovery exclusions are the connector's fixed `vendor`, `.git`, `testdata`,
`generated`, `node_modules`, `.idea`, `.vscode`, `.claude` directories, its default
cache directories (`.cache`, `.tools`, `target`, `dist`, `build`, `__pycache__`,
`.venv`, `.gradle`, `.next`, `.turbo`, `coverage`), applicable ignore rules,
nonregular files and non-Dart ownership. The harness's measured regular-Dart
counts/bytes reconcile exactly with the tracked regular-Dart subset. Each old
package adds three ignored/untracked dependency/config files after pub get;
the new package adds one. Inspection of every `.untracked` receipt found **zero
new `.dart` inputs**. The tracked-set recipe is an approximation, not a claim
that the harness emits an exact discovery file list. No holdout was used, moved
into canonical calibration, or claimed independent after tuning.

Truth independence was checked on a separate three-file native probe, never
used to grade Dart. `indexbench lang-probe -dialect dart <probe>/lib/b.dart`
produced saved extractor output, which was replaced with `perturbed extractor
output: no symbols`. Re-running scip-dart over the unchanged probe left the raw
index SHA-256 identical:
`76ba3eaa810bc019307e8d95bbf7d99a4a52bd02123932ec1319cc7bf0503ec2`.
Changing its known import/reference from class A in a.dart to class C in c.dart
changed the raw index to
`cd2c9d24ba19ed8a064a9648c264501b295f265d597761f19fa8e9bed0463ded`.
Both comparisons passed. This proves the native truth does not read the saved
extractor output; it does not establish coverage of unexecuted or out-of-scope
facts. Local probe and binaries are retained for review.

Tenancy: zero faults and zero ERROR-or-MISSING files were reported. The candidate's
single-pass CPU throughput is 1.86 MB/s with maximum file time 38 ms; no separate
ERROR-only/MISSING-only census was taken, and no T1 median-of-three throughput
claim is made. Throughput is not the ceiling. No canonical reading or nested
Reading-schema file changed, so strict round-trip passed and no serialization
or required-kind diff is introduced. No oracle tier/scope/pairing logic or caps
changed: mutation/cap+1 tests for changed logic are not applicable. No actual
code correction was attempted, so there is no corrective RED test or fixture.
No box flips occurred; scratch plan verification was not used to claim one.
No observable behavior changed, so no change-note fragment is added.

Scoped checks passed (no skips):

```sh
bash scripts/fathom-go.sh test -race -count=1 -run 'TestScoreSCIPDartLocalSymbolsAreDocumentScoped|TestSCIPImportRefs|TestCommittedReadingsRoundTrip|TestLanguagesTrackerIsGenerated' ./internal/langbench
bash scripts/fathom-go.sh test -race -count=1 -run 'TestDartFileSpecsParts|TestSplitTSMatches_EndCaptureExtendsDefinition|TestExtractFileSpecs/dart' ./internal/extractor
bash scripts/fathom-go.sh test -race -count=1 -run 'TestResolveFileEdges_Dart|TestIngest_DartReachesTheDartDialect' ./internal/fusion
bash scripts/fathom-go.sh test -count=1 -run TestNoOrgSpecificNames ./cmd/fathom
bash scripts/fathom-go.sh run ./third_party/agentic-kit/cmd/release-notes -check
bash scripts/check-merge-debris.sh
bash scripts/check-merge-debris.sh --staged
command gofmt -l internal/langbench/scip_dart.go internal/langbench/scip_dart_test.go internal/extractor/dartfilespecs_test.go
```

For a fresh rerun, obtain the immutable corpus and SDK/producer pins above into
`workspace/`, set `PUB_CACHE` and `XDG_CONFIG_HOME` within `workspace/tools/` and
`DART_SUPPRESS_ANALYTICS=true`, and run each package's native index from `.` after
`dart pub get`. Retain the native indexes under `workspace/tmp/scip/<name>.scip`.
Run the §13.3.1 manifest recipe around those native invocations. Then, from the
fathom root, with a fresh candidate output path:

```sh
export PATH="$PWD/workspace/tools/sdk/dart-sdk/bin:$PATH"
export PUB_CACHE="$PWD/workspace/tools/pubcache"
export XDG_CONFIG_HOME="$PWD/workspace/tools/config" DART_SUPPRESS_ANALYTICS=true
mkdir -p workspace/tmp/scip workspace/artifacts/lang-dart
for spec in 'bloc packages/bloc bloc' 'bloc packages/bloc_test bloc_test' 'bloc packages/bloc_concurrency bloc_concurrency' 'http pkgs/http http' 'http pkgs/http_parser http_parser' 'http pkgs/http2 http2' 'http pkgs/web_socket_channel web_socket_channel' 'http pkgs/http_multi_server http_multi_server' 'dart-test pkgs/test test'; do
  read -r repo sub name <<EOF_SPEC
$spec
EOF_SPEC
  output="$PWD/workspace/tmp/scip/$name.scip"
  (cd "workspace/calibration/$repo/$sub" && dart pub get && dart pub global run scip_dart . -o "$output")
done
bash scripts/fathom-go.sh build -o workspace/tools/indexbench ./cmd/indexbench
for spec in 'bloc packages/bloc bloc' 'bloc packages/bloc_test bloc_test' 'bloc packages/bloc_concurrency bloc_concurrency' 'http pkgs/http http' 'http pkgs/http_parser http_parser' 'http pkgs/http2 http2' 'http pkgs/web_socket_channel web_socket_channel' 'http pkgs/http_multi_server http_multi_server' 'dart-test pkgs/test test'; do
  read -r repo sub name <<EOF_SPEC
$spec
EOF_SPEC
  workspace/tools/indexbench lang-oracle -dialect dart \
    -subject "workspace/calibration/$repo/$sub" -subject-name "$repo/$sub" \
    -subject-sha "$(git -C "workspace/calibration/$repo" rev-parse HEAD)" \
    -scip "workspace/tmp/scip/$name.scip" -scip-defs \
    -out workspace/artifacts/lang-dart/candidate.json \
    -unmatched "workspace/artifacts/lang-dart/$name-unmatched.json"
done
workspace/tools/indexbench lang-loop -reading workspace/artifacts/lang-dart/candidate.json dart
```

The final command is expected to exit 1 for this frozen population. It replays
raw counts and skips the oracle/probe stages explicitly; it is not a second
canonical measurement. `WRITE=1` remains barred by the measured regression.
The full gate, daemon changes, publishing and corpus cleanup were not run.
Corpus and local tool state are retained for the separate fresh critic.
