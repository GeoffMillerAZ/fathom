# PHP SCIP alternatives and native executed-call probes

Bench-only supplementary measurements; no product dependency, canonical reading,
corpus shard, subject exclusion or automatic tier promotion. Requires Darwin ARM64,
Python **3.13+**, Go, C build tools, make, Perl, curl, Git and Node.js. All fetched
tools, SDKs, dependency caches, logs, traces and subjects belong under workspace/
or the worker's own cache. Set TMPDIR, GOTMPDIR, GOCACHE and GOMODCACHE there,
GOWORK=off, GOMAXPROCS=2 and GOFLAGS=-p=2. Use one heavy job at a time, niceness 10,
and start only at load <=6. Scripts wait before later heavy steps if load rises.
No global installation or shell configuration is required. Each --out is fresh.

## Static producer replay

The fetcher checks out the entire immutable Git tree, including export-ignore
files. Pins are in `fetch.py`: Guzzle 7.9.3, Monolog 3.9.0 and three independent
SCIP candidates. Runtime and released binary downloads have fixed SHA256 checks.

```sh
nice -n 10 python3.13 calibration/oracles/php-candidates/fetch.py --out workspace/tools/php-candidates
nice -n 10 python3.13 calibration/oracles/php-candidates/install.py --tools workspace/tools/php-candidates
nice -n 10 go build -o workspace/tools/indexbench ./cmd/indexbench
nice -n 10 python3.13 calibration/oracles/php-candidates/measure.py \
  --tools workspace/tools/php-candidates --indexbench workspace/tools/indexbench \
  --out workspace/tmp/php-static
nice -n 10 python3.13 calibration/oracles/php-candidates/audit_scope.py \
  --tools workspace/tools/php-candidates --measurement workspace/tmp/php-static
```

The static install preserves the existing committed scip-php recipe: Guzzle's
integration-test dependency is omitted there, Monolog's lock generation enabled,
and Composer scripts/plugins disabled with platform checks ignored. Resolved
versions and composer.json/lock hashes are recorded privately during execution;
those private snapshots may be discarded after measurement. Replaying dependency
constraints later is not a promise of byte-identical indexes. The full original
74/217 PHP source files must remain unchanged. Guzzle's integration dependency
is restored for the separate runtime replay below.

`unified_to_scip.php` transports the call-site fork's own SCIP JSON section to
protobuf, checking document and occurrence counts; it invents no references.
The harness defaults SCIP inputs to A; this replay explicitly passes -scip-tier C
for the unreleased forks and the not-yet-admitted released candidate. Its raw
readings remain private **diagnostics**, never canonical grades. The two
unreleased forks are C under OL-R9. Mago is run at released v0.3.0, with MIT declared in Cargo.toml/README;
maintained-indexer admission is not inferred from transport format. Its full
blind share fails even if A is granted. The public summary gives that limit.
`audit_scope.py` counts written pairs in files absent from the native index in
addition to within-index blindness. Syntax supplies the denominator only.

## Native runtime replay

```sh
nice -n 10 python3.13 calibration/oracles/php-candidates/install.py --tools workspace/tools/php-candidates --runtime
nice -n 10 python3.13 calibration/oracles/php-candidates/build_xdebug.py --tools workspace/tools/php-candidates
nice -n 10 python3.13 calibration/oracles/php-candidates/prepare_runtime.py --tools workspace/tools/php-candidates
nice -n 10 python3.13 calibration/oracles/php-candidates/run_suites.py \
  --tools workspace/tools/php-candidates --out workspace/tmp/php-traces85
nice -n 10 python3.13 calibration/oracles/php-candidates/runtime84.py \
  --tools workspace/tools/php-runtime84 --shared workspace/tools/php-candidates
nice -n 10 python3.13 calibration/oracles/php-candidates/build_xdebug.py --tools workspace/tools/php-runtime84
nice -n 10 python3.13 calibration/oracles/php-candidates/prepare_runtime.py --tools workspace/tools/php-candidates --restore
nice -n 10 python3.13 calibration/oracles/php-candidates/prepare_runtime.py --tools workspace/tools/php-runtime84
nice -n 10 python3.13 calibration/oracles/php-candidates/run_suites.py \
  --tools workspace/tools/php-runtime84 --out workspace/tmp/php-traces84
```

Xdebug 3.5.3 is compiled against each matching PHP SDK (8.5.8 and 8.4.23).
Private m4/Autoconf are built only if needed; source hashes are fixed. The SDK
installs headers/build scripts, not a global PHP. The second runtime shares
existing subject/dependency trees through workspace symlinks; it changes no
subject dependency contract. Composer --runtime restores the subject's declared
test dependencies, including Guzzle's integration-test package, before execution.

The original PHPUnit drivers run without filtering tests, deterministic seed 1,
with Guzzle's two declared local test servers. Occupied ports abort rather than
reuse or stop an existing service. Only newly created server/driver processes
are cleaned up. A known helper string redirect keeps Guzzle's server log inside
the subject workspace; before/after complete syntax dumps must agree on all
literal call sites, definitions and call pairs. No other PHP source is changed.
Use the same prepare command with --restore to restore those original bytes.
SDK/runtime PATH and all temporary paths are private. Traces are bounded to
2 GiB per file and drivers time out after 300 seconds; completion and driver
failure are independent receipt fields. PHPUnit failures/skips stay disclosed.

`trace.py` takes both pair endpoints from the native answers: Xdebug target and
PHP Reflection named-function/method ranges. The independent syntax walk supplies
literal scope and a full denominator, never a native caller/callee pair. Original
unexecuted sites remain unattested. Native parent mismatch, same-line duplicate
sites, possible nonliteral source on the same line and magic dispatch are not
silently attested. Aliases whose resolved name differs stay unattested. Optional
`--fixture` runs the committed tiny native fixture with an alias gap.

For traces collected before the conservative attribution checks changed,
`redecode.py` refreshes syntax scope and verifies the entire original literal-site
population is identical before reusing preserved native events. It does not
rerun tests or change their execution status. Fresh collections need no re-decode.

```sh
nice -n 10 python3.13 calibration/oracles/php-candidates/score_traces.py \
  --tools workspace/tools/php-runtime84 --indexbench workspace/tools/indexbench --traces workspace/tmp/php-traces84
nice -n 10 python3.13 calibration/oracles/php-candidates/score_traces.py \
  --tools workspace/tools/php-candidates --indexbench workspace/tools/indexbench --traces workspace/tmp/php-traces85
python3.13 calibration/oracles/php-candidates/publish.py \
  --measurement workspace/tmp/php-static --traces workspace/tmp/php-traces85 --traces workspace/tmp/php-traces84 \
  --out workspace/tmp/php-public/candidates.json
PYTHONDONTWRITEBYTECODE=1 python3.13 -m unittest discover -s calibration/oracles/php-candidates -p 'test_*.py'
```

OL-R9 grants A only on the executed runtime population, with coverage disclosed.
The raw harness also counts unexecuted predictions as false positives; its
precision is therefore **not** an executed-population precision measure and is
never published as one. Only executed-native-pair recall is reported, never
recall on unexecuted code. `publish.py` explicitly selects aggregate counts and
hashes; raw source/file lists, traces, PID/host paths and diagnostic grades remain
private. Public data has no raw subject text or copied file inventory.
