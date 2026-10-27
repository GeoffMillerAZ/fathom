# Pulumi SDK static triage (IAC-4, GAP1-36)

Bench-only, no product wiring. Controller I-R5 permits cheap triage while NP-Q5
is unanswered. This is one authored TypeScript control, previously explored,
with installed AWS, Azure Native and GCP SDKs. It is neither a public subject
sample nor a holdout, and grants no support grade, ceiling or queue change.
Python and Go programs remain untried. No Pulumi CLI, program, preview, update,
provider plugin or cloud call is run.

## Frozen protocol

Before a fresh run, `runner.py` validates its `FILES` SHA-256 pins and six
`VERSIONS` against the existing `pulumi-probe` cache, then hashes every installed
file and relative symlink target without following symlinks. Absolute symlink
targets refuse before hashing, since they can encode private directory paths. Registry integrity strings are
metadata; this does not authenticate installed SDK bytes against registry archives.
The cache originated in the earlier exploratory probe. The source/compiler/lock
hashes are fixed before this fresh run; prior results influenced this selection.

Run only TypeScript `--noEmit` and scip-typescript `index` serially, with scrubbed
HOME/environment and OS network denial. The source function is never called.
`skipLibCheck: true` skips diagnostics inside SDK declaration files. Confirm a
live parent-owned loopback listener is reachable outside the sandbox and denied
with EPERM inside it. Deny cache writes, require actual nice >=10 and load <=9,
120-second command limits and 1 MiB file/output limits. Rehash the cache afterward.

Use ordinary `NewTSExtractor("typescript")` and library `scip.ImportIndex` from
the named main engine to count each SDK's native symbol occurrences and imported
direct/unresolved edges. These are not all calls. No full service ingest, resolver
mutation, L4 policy, dependency admission, anchors or persistent-store cost is tested.

## Reproduce

macOS sandbox-exec, Node and an installed SDK tree are prerequisites. The runner
never installs or downloads anything. The committed `fixture/package-lock.json`
retains the original dependency graph, versions, resolved URLs and integrities.
Only 47 optional `funding` fields and 5 free-text `deprecated` fields were
removed in the privacy pass. The six registry records in
`fixture/pins-before-install.json` retain versions, archive URLs and SHA-512
integrities; six redundant legacy SHA-1 fields are omitted. These are metadata, not SDK
payloads. `FILES` pins their committed bytes. The runner checks installed
versions and lock/integrity metadata; it does not prove installed bytes match
registry archives. It hashes the full SDK tree before and after each run.

Stage a fresh private cache directory whose basename is exactly `pulumi-probe`.
`PULUMI_TRIAGE_SDKS` must be the physical installed `node_modules` directory,
separate from the staged cache and scratch. The runner resolves this symlink and
denies writes to both locations in its private OS sandbox profile. Public launch
receipts report exit, elapsed time, load, nice and policy. Raw stdout/stderr
lengths and hashes are omitted because tools echo private paths. The raw index
and locally built helper binary are never hashed into public receipts. The Go
helper clones the parsed SCIP index, replaces only `Metadata.project_root` with
`file:///subject`, deterministically re-encodes the protobuf and publishes
`normalized_index_sha256`. It imports the original bytes; normalization changes
only the public digest, not the symbol population or importer input.

```sh
# Set both variables to operator-owned absolute paths; no workstation path is assumed.
export PULUMI_TRIAGE_CACHE='<private-staging-root>/pulumi-probe'
export PULUMI_TRIAGE_SDKS='<private-installed-tree>/node_modules'
mkdir -p "$PULUMI_TRIAGE_CACHE"
cp calibration/oracles/pulumi-sdk-triage/fixture/{index.ts,tsconfig.json,package.json,package-lock.json,pins-before-install.json} "$PULUMI_TRIAGE_CACHE/"
ln -s "$PULUMI_TRIAGE_SDKS" "$PULUMI_TRIAGE_CACHE/node_modules"
PYTHONPYCACHEPREFIX=workspace/tmp/pycache python3 -m unittest discover -s calibration/oracles/pulumi-sdk-triage -p 'test_*.py'
python3 calibration/oracles/pulumi-sdk-triage/runner.py --cache-root "$PULUMI_TRIAGE_CACHE" --scratch workspace/tmp/pulumi-sdk-fresh --out workspace/artifacts/pulumi-sdk-native.json
# Build/run serially at nice >=10 and load <=9; no module downloads (GOPROXY=off).
bash calibration/oracles/pulumi-sdk-triage/test_importer.sh
python3 calibration/oracles/pulumi-sdk-triage/scheduled.py go build -trimpath -o workspace/artifacts/pulumi-sdk-importer ./workspace/tmp/pulumi-sdk-helper
python3 calibration/oracles/pulumi-sdk-triage/scheduled.py workspace/artifacts/pulumi-sdk-importer workspace/tmp/pulumi-sdk-fresh > workspace/artifacts/pulumi-sdk-import.json
# A different fresh root must produce the identical public importer result.
python3 calibration/oracles/pulumi-sdk-triage/runner.py --cache-root "$PULUMI_TRIAGE_CACHE" --scratch workspace/tmp/pulumi-sdk-another-longer-root --out workspace/artifacts/pulumi-sdk-native-second.json
python3 calibration/oracles/pulumi-sdk-triage/scheduled.py workspace/artifacts/pulumi-sdk-importer workspace/tmp/pulumi-sdk-another-longer-root > workspace/artifacts/pulumi-sdk-import-second.json
cmp workspace/artifacts/pulumi-sdk-import.json workspace/artifacts/pulumi-sdk-import-second.json
```

If the installed tree is lost, restore it separately from the staged cache
using an operator-provided npm cache holding the exact lockfile tarballs:

```sh
export PULUMI_TRIAGE_NPM_CACHE='<operator-provided-offline-npm-cache>'
export PULUMI_TRIAGE_INSTALL='<fresh-private-install-directory>'
mkdir -p "$PULUMI_TRIAGE_INSTALL"
cp calibration/oracles/pulumi-sdk-triage/fixture/{package.json,package-lock.json} "$PULUMI_TRIAGE_INSTALL/"
(cd "$PULUMI_TRIAGE_INSTALL" && npm ci --offline --ignore-scripts --fund=false --audit=false --cache "$PULUMI_TRIAGE_NPM_CACHE")
export PULUMI_TRIAGE_SDKS="$PULUMI_TRIAGE_INSTALL/node_modules"
```

An empty npm cache cannot satisfy `--offline`; populate it from an approved
mirror first. The runner's frozen compiler/indexer entry hashes and all six
package versions must still match. No install runs inside the probe.

The test script and Go launch recipes enforce actual nice >=10 and load <=9
through `scheduled.py`, logging only those measurements to stderr. Do not run this concurrently
with another heavy job. Native logs/index remain private. Publish checked-in
input hashes, installed byte-conservation evidence, the normalized SCIP digest
and counts. No stdout/stderr, raw-index, helper-binary or build/import-log hash
is published. Scheduling measurements naturally vary between launches; they
are operational evidence, not part of the root-independent result comparison.

The Python privacy test launches real children that print their cwd to both
streams, fixes the clock/load independently, and requires identical public
launch records across different root lengths with no raw stream digest/length.
The Go test runs the actual importer helper on indices differing only in root
and requires identical public JSON, then changes a document language and
requires a different normalized index digest. Two fresh native indexer runs
repeat the public-JSON comparison. Tests and native comparisons must be green
before publishing a receipt.

Owner sources checked 2026-10-26: [Pulumi TypeScript](https://www.pulumi.com/docs/iac/languages-sdks/javascript/)
and [scip-typescript](https://github.com/sourcegraph/scip-typescript).

Partial [assessment](../../../docs/assessments/2026-10-26-pulumi-sdk-triage.md): feeds IAC-4/GAP1-36 without satisfying their completion path.
