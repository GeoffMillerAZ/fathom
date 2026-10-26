# PowerShell owner binder and executed-function probes

Supplementary evidence under OL-R11; no canonical reading, corpus shard, product
code, generated board or tier-policy change. Requires Darwin ARM64, Python 3.13+,
Git, curl and Go. Use one heavy job at a time, `nice -n 10`, load <=6, and check
`df -h ~` before each heavy step; refuse below 20 GiB free. All tools, dependency
caches, temporary test data, logs and source belong under workspace/ or the
worker's own cache. Set TMPDIR/GOTMPDIR/GOCACHE/GOMODCACHE/XDG paths there,
GOWORK=off, GOMAXPROCS=2 and GOFLAGS=-p=2. No global install or profile is needed.
Every measurement --out is fresh. Remove measured subjects/builds/downloads after
saving sanitized aggregate evidence; never publish subject people, names/emails,
usernames, CODEOWNERS, copyright holders, source or wholesale file lists.

## Replay

```sh
nice -n 10 python3.13 -B calibration/oracles/powershell-native/fetch.py --out workspace/tools/powershell-native
nice -n 10 python3.13 -B calibration/oracles/powershell-native/complete_runtime_inputs.py --tools workspace/tools/powershell-native
nice -n 10 python3.13 -B calibration/oracles/powershell-native/prepare_runtime.py --tools workspace/tools/powershell-native
nice -n 10 bash scripts/fathom-go.sh build -o workspace/tools/indexbench ./cmd/indexbench
nice -n 10 python3.13 -B calibration/oracles/powershell-native/measure_binder.py \
  --tools workspace/tools/powershell-native --indexbench workspace/tools/indexbench \
  --out workspace/artifacts/lang-powershell/binder
nice -n 10 python3.13 -B calibration/oracles/powershell-native/run_traces.py \
  --tools workspace/tools/powershell-native --binder workspace/artifacts/lang-powershell/binder \
  --out workspace/artifacts/lang-powershell/runtime
nice -n 10 python3.13 -B calibration/oracles/powershell-native/run_traces.py \
  --tools workspace/tools/powershell-native --binder workspace/artifacts/lang-powershell/binder \
  --out workspace/artifacts/lang-powershell/main-phase --only pester --pester-main
python3.13 -B calibration/oracles/powershell-native/combine_runtime.py \
  --default workspace/artifacts/lang-powershell/runtime/pester \
  --remaining workspace/artifacts/lang-powershell/main-phase/pester \
  --population workspace/artifacts/lang-powershell/binder/pester/native-answers.json \
  --out workspace/artifacts/lang-powershell/combined
python3.13 -B calibration/oracles/powershell-native/clean_score_scope.py \
  --tools workspace/tools/powershell-native --out workspace/artifacts/lang-powershell/score-scope.json
nice -n 10 python3.13 -B calibration/oracles/powershell-native/score.py \
  --tools workspace/tools/powershell-native --binder workspace/artifacts/lang-powershell/binder \
  --runtime-pester workspace/artifacts/lang-powershell/combined \
  --runtime-psa workspace/artifacts/lang-powershell/runtime \
  --out workspace/artifacts/lang-powershell/scored --indexbench workspace/tools/indexbench
```

Pins/checksums are in `fetch.py`. PowerShell 7.6.9, matching Pester 6.2.0 and
PSScriptAnalyzer 1.25.0 modules are isolated. PSScriptAnalyzer's own declared
Pester **5.7.1** test dependency is installed separately; its old `-Pending`
tests must not be measured under Pester 6.2. The original pinned repository's
runtime documentation/compatibility profiles are restored outside the graded
`Tests` subtree; the source measurement still covers exactly that subtree.

Pester's own non-inline build script uses the matching released DLL/format data,
without a C# source build or source edits. Native function definitions then
point at original `src` files. Generated `bin` is load-state input only and is
outside frozen original Git membership. After all native collections, remove
that generated wrapper before product scoring: the filesystem discovery would
otherwise count its three extra PowerShell files. `clean_score_scope.py` records
counts/bytes and asserts original tracked source remains unchanged.

## Native truth and scope

`ResolveCalls.ps1` parses every original tracked PowerShell source file, including
UTF-16 input. The native AST supplies scope/caller declarations. Actual target
names and aliases come from `$ExecutionContext.InvokeCommand.GetCommand`, never
from Fathom output. Autoload is disabled before the preamble. Six explicit
pinned-runtime modules are imported and asserted: Archive, Host, Management,
Security, ThreadJob and Utility. The matching subject manifest and its original
declared test harness are loaded. Pester uses Axiom, p and the exact TestHelpers
body compiled from its original driver AST; PSScriptAnalyzer uses declared
Pester 5.7.1. Dynamic TestHelpers metadata is captured directly from New-Module,
which exports the functions without registering in Get-Module. No test body is
dot-sourced or executed merely to satisfy a query. Native loader-owned source files
use that module's session; ordinary test files use the global session, so private
module functions cannot answer unrelated test-file queries. The receipt records
module names/versions/code hashes, definition-origin counts/content hashes and
all unresolved sites partitioned into member, harness, same-file/BeforeAll,
private-module, external-application and missing classes. These classes diagnose
source context without inventing targets. External applications, undeclared state, unbound functions
and literal member invocations remain unresolvable, never dropped or counted
as hits. No fixture is dot-sourced to manufacture a successful binding.

Pester's original source scope is 272 files / 2,713,762 bytes. PSScriptAnalyzer
has 241 original PowerShell files / 865,136 bytes in `Tests`, including 17 files
with NULs that product discovery classifies as binary; the canonical product
scope is 224 / 775,189. Retain all 17 in native truth and disclose the difference.
Native recovered AST sites in four parse-error files are not a complete-syntax
claim. Native parser errors and tree-sitter ERROR/MISSING rates are different
metrics. Resolved-only pair diagnostics cannot hide unresolved original sites.

The exact section 13.3.1 tracked-set manifest body is `tracked_set.py`, run before
and after static/runtime collection. Reconcile all tracked kinds against the
PowerShell subset. Hash symlink targets without following links; gitlinks do not
cover submodule contents. Inspect generated/untracked/ignored paths and publish
only counts/hashes. Original file membership never follows generated output.

## Executed truth

`TraceFunctions.ps1` observes native command-breakpoint function frames. Their
InvocationInfo supplies actual FunctionInfo, alias spelling, source file/line/
column and the native caller frame. Decoder sites are scope only: exact source
position and caller guard, actual native target. Cmdlets, members, ambiguous
callbacks and child-process calls have no such witness and remain unattested.
They stay in the full original-site coverage denominator. Only observed executed
pairs enter recall; runtime precision is unmeasured. Coverage cannot trigger a
whole-written-population L2 grade under P-T1/OL-R11.

PowerShell 7.6.9 script-qualified command breakpoints dereference a null
InvocationInfo.ScriptName during New-Module. A no-op breakpoint reproduces the
Pester assertion failures. A global breakpoint avoids this owner bug; native
source positions and the frozen original site table provide scope filtering.
The generic proof fixture includes New-Module and kills a regression to the
script-qualified hook. `$PSDebugContext` is null in this action; native call-stack
FunctionInfo is the actual target source, not that unavailable context variable.

Original drivers are Pester `test.ps1 -NoBuild`, its own declared remaining phase
`-SkipPTests`, and PSScriptAnalyzer's four test directories with `Invoke-Pester
-CI` at 5.7.1. Default Pester stops after three ANSI-output test failures that also
occur without tracing; the separate declared phase exercises the remaining tests.
The union retains the same full population. Driver failures/skips are disclosed.
Timeout/signal/missing output refuses the whole unit; only owned process groups
are terminated. No runtime success is inferred from a passing process exit alone.
`driver_outcome.py` parses original driver summaries into counts-only phase
receipts; a zero-exit observer is separate from a driver that stopped after
failed P tests. Each phase pins TraceFunctions.ps1 and trace_decode.py hashes.
`score.py` writes private unmatched pairs and publishes their counts-only
attribution: actual generated mock rebinding, other target rebinding, case-only
and missing written pairs. Native targets and recall stay unchanged; neither
GUID normalization nor dropping inconvenient misses is allowed. Decoder row
rejections are counted separately by missing/ambiguous site, invocation spelling
and caller mismatch. See the assessment for controller product routing.

## Proofs and limits

```sh
python3.13 -B -m unittest discover -s calibration/oracles/powershell-native -p 'test_*.py'
nice -n 10 python3.13 -B calibration/oracles/powershell-native/prove_native.py \
  --tools workspace/tools/powershell-native --indexbench workspace/tools/indexbench \
  --out workspace/artifacts/lang-powershell/proof
nice -n 10 python3.13 -B calibration/oracles/powershell-native/prove_mutations.py \
  --tools workspace/tools/powershell-native --indexbench workspace/tools/indexbench \
  --out workspace/artifacts/lang-powershell/mutations
```

```sh
nice -n 10 python3.13 -B calibration/oracles/powershell-native/prove_harness.py \
  --tools workspace/tools/powershell-native --indexbench workspace/tools/indexbench \
  --out workspace/artifacts/lang-powershell/harness
```

This runs the complete native binder and asserts both original subjects' declared
harness/load state and the complete unresolved partition. Run it while Pester's
load-only bin still exists, before clean_score_scope.py. Its regression first
fails against the reviewed narrow session, then passes with declared helpers.

The real fixture proves module privacy, alias targets, unknown/member/external
misses, disabled undeclared autoload and the New-Module regression. Real extractor
output is placed in the subject and native output directories, then changed to
fabricated pairs; native truth digests must stay equal. Changing the native alias
from NativeOne to NativeTwo changes exactly two known caller/target pairs in each
route. No extractor pairs enter native truth.

Fourteen mutants cover caller/alias pairing, callbacks, caps, both declared phases,
native tier, private scope, autoload, script-qualified breakpoints, omitted
declared harness, explicit runtime baseline, stopped-phase status and mock attribution. Each must
reach `FAIL:` and restore GREEN. Pure regressions cover exact-column ambiguity,
unexecuted member retention, malformed JSON, wrong runtime, raw refusal and each
cap at reduced-limit cap+1. Hard limits: 2,048 original files, 64 MiB source,
1,000,000 static sites/native rows, 128 MiB raw trace and a fixed two-phase union
with the same total-byte/row bounds. All refuse the entire affected unit; native
cap refusal emits no accepted partial oracle.
