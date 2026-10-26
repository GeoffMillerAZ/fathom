# PHP SCIP alternatives and native executed-call probes

Bench-only supplementary measurements; no product dependency, canonical reading,
corpus shard, subject exclusion or automatic tier promotion. Requires Darwin ARM64,
Python **3.13+**, Go, C build tools, make, Perl, curl, Git and Node.js. All fetched
tools, SDKs, dependency caches, logs, traces and subjects belong under workspace/
or the worker's own cache. Set TMPDIR, GOTMPDIR, GOCACHE and GOMODCACHE there,
GOWORK=off, GOMAXPROCS=2 and GOFLAGS=-p=2. Use one heavy job at a time, niceness 10,
and start only at load <=6. Scripts wait before later heavy steps if load rises.
Before every download/build/oracle step, `ready()` prints `df -h` and refuses
to start below 20 GiB free. Delete measured subjects/builds after saving receipts.
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

## Proof replay and bounded collection

The minimal fixture replay needs only `fetch.py --only callsite-fork --only php
--only composer`, `install.py --only callsite-fork`, `build_xdebug.py`, and the
indexbench build above. It does not fetch the three subject/indexer trees that
the fixture does not need. Then run:

```sh
nice -n 10 python3.13 -B calibration/oracles/php-candidates/prove_native.py \
  --tools workspace/tools/php-proof --indexbench workspace/tools/indexbench \
  --out workspace/artifacts/lang-php/native-proof
python3.13 -B calibration/oracles/php-candidates/prove_mutations.py
python3.13 -B -m unittest discover -s calibration/oracles/php-candidates -p test_trace_caps.py
nice -n 10 python3.13 -B calibration/oracles/php-candidates/prove_scope.py \
  --tools workspace/tools/php-proof --out workspace/artifacts/lang-php/scope
```

The native proof collects the committed fixture and confirms deterministic
re-decoding of identical native inputs. Its isolated extractor-output artifact
is outside collector inputs, so changing it is not an adversarial independence
probe. Independence is structural: only native trace/reflection files and
explicit php-parser syntax scope enter `decode()`; no extractor pairs do.
Changing the fixture's native call from `strlen` to `strspn` must change exactly
that native pair. Both collections assert 2/4 sites; the alias and same-line
dynamic traps stay unattested. These are proof fixtures, never corpus members.

`tracked_set.py` is the exact Python body of design recipe section 13.3.1.
Run before and after measurement using identical subject/subpath inputs:

```sh
SUBJECT=workspace/tools/php-candidates/guzzle SUBPATH=. \
  RECEIPT=workspace/artifacts/lang-php/guzzle-before \
  python3.13 -B calibration/oracles/php-candidates/tracked_set.py
# Repeat with RECEIPT ending in -after; repeat both for monolog.
```

The manifest includes all tracked kinds, not just PHP, and is a discovery
approximation. `.untracked` includes ignored paths. Inspect it for generated
language input, reconcile original PHP counts/bytes with the canonical reading,
and publish only counts/hashes. Restore the disclosed runtime helper string and
Composer metadata before taking the final original-source receipt. Dependencies
remain excluded under `vendor/`, never removed from original source scope.
`prove_scope.py` runs the exact manifest body around a fresh whole-population
syntax audit and asserts canonical counts/bytes, the 2,571/4,558 original pairs
and zero generated untracked/ignored paths. It also hashes the sorted-JSON
literal-site population and asserts equality with every published runtime
`population_sha256` for that subject. Fetch the two original subjects
first with `fetch.py --only guzzle --only monolog`. This reconstructs the pinned
source scope; it does not invent retrospective manifests for discarded suites.

`trace.py` refuses the entire collection on missing footer/reflection, timeout,
signal, more than 64 trace files or 64 reflection files, over 8 GiB total trace,
over 2 GiB per trace, over 64 MiB reflection JSON, a trace line over 1 MiB or a
native stack depth over 4096. It emits `refusal.json` and no `oracle.json`.
`test_trace_caps.py` tests every cap at a reduced limit with limit+1 input,
plus truncation, footer text inside payload, timeout and signal. Completed suite
failures remain disclosed; they do not become a claim of passing tests.

Fresh-tree regression (creates a temporary recipe tree with no workspace):

```sh
python3.13 -B -m unittest discover -s calibration/oracles/php-candidates -p test_fresh_checkout.py
```

Historical runtime rows used the `996bc6635` collector, before strict completion
and whole-collection limits. Attribution is unchanged; discarded traces were
not revalidated. Their resolved package-version maps and lock snapshots were
discarded before publication and cannot be reconstructed retrospectively. Future
collections must retain sanitized dependency versions and lock hashes, without
subject people or identity metadata (X-R5).
