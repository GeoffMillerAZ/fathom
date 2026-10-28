# T-R9 / T-R11: Azure Policy oracle pending an x64 run

The controller copies `probe-linux-x64.workflow.yml` into `.github/workflows/`
on the disposable branch **`tools/azure-policy-probe-run`**, which is never
merged. Its push filter matches only that branch, including no tag pushes.
T-R11 requires a GitHub-hosted `ubuntu-22.04` x64 job with no `container:` job.
Do not use Docker/Rosetta, sudo, sysctl changes, system installs or Azure login.
Python 3.9+, util-linux `unshare`, `mount` and `setpriv`, and the ordinary .NET
Linux libraries must already exist. Missing prerequisites refuse without a
fallback. The self-contained payload needs no global .NET installation.

## Runner lifetime and action pins

The current T-R11 label remains in place. GitHub's [runner retirement notice](https://github.com/actions/runner-images/issues/14254)
sets its unsupported date to **2027-04-17**, with brownouts in March and April
2027. Revisit T-R11 before those brownouts or on any label refusal. The planned
successor is `ubuntu-24.04` x64; `ubuntu-26.04` x64 is another official option.
Changing the mandated label needs a controller ruling: newer images restrict
unprivileged user namespaces. The receipt's kernel capability values and
`failed_stage` diagnose that restriction; do not disable it or auto-select
`ubuntu-latest`.

Checkout v5 and upload-artifact v7 are pinned by full SHA. The pinned [upload
action source](https://github.com/actions/upload-artifact/blob/cf430e030ddbb5b0abf93d22962f4752f3646cd9/action.yml)
declares Node 24. The job has only `contents: read` permission and uses
`persist-credentials: false`. It has no login or secrets. Only the separate,
outer-validated `receipt.json` is uploaded with one-day retention. Raw LSP,
evaluation, stderr, preflight and launcher logs never leave the runner.

## Payload and writable boundary

Microsoft's MIT AzurePolicy extension 0.2.6 supplies 455 linux-x64 files,
122,294,968 expanded bytes in two spans totaling 52,578,398 bytes. The manifest
SHA-256 is `53ab1997c413e02553a8f0b03225cfaed65b53fc7779ba003e10ec4ab67c1ac4`;
the apphost SHA-256 is
`bb3603fa212f7444f86aae84e593654b8df9b7feebdd0e4d0b30085fa798f452`.
All files are verified before execution. The launcher copies the payload into
`out/worker/payload` and re-verifies the private copy. The download cache stays
outside the worker's writable area.

Execution requires user, network, PID and mount namespaces:
`unshare --user --map-root-user --net --pid --fork --kill-child --mount --mount-proc`.
The worker is PID 1 and sees only loopback. Before any vendor byte runs,
`filesystem_isolation.py` makes mount propagation private and bind-mounts
only the worker scratch. It selects reachable, topmost mounts using mount/parent IDs and remounts those
outside scratch read-only, deepest first. Hidden mounts and descendants of
covered mounts are not remounted. There is no root-on-root bind or chroot:
round 2's stacked-root design exposed a writable `/..` route and is withdrawn.
The worker cwd is scratch. Its private payload, HOME, TMP and raw logs are
writable there; outer logs, publication, checkout and action code are outside
that writable mount and included in the protected-path checks.

The vendor command uses util-linux `setpriv` to drop its bounding, inheritable
and ambient capabilities and set `no_new_privs`. A separate authored process
runs under the same restrictions and checks effective/permitted/bounding/
inheritable/ambient capabilities are zero. It deliberately opens a new file
for write in the launcher-supplied checkout, runner HOME, RUNNER_TEMP and
cache, and opens the running shell launcher for write. Every path is tested with
plain, `/..`-prefixed and `/proc/..`-prefixed spellings without normalizing
away `..`. **Every attempt must fail with EROFS**; EACCES alone is not
accepted. A positive scratch write must succeed. Preflight and the real worker both run these controls before vendor
Popen. Any success outside scratch refuses before vendor execution.

Only measured success admits the schema-constant
`filesystem_readonly_outside_out: true` and `vendor_capabilities_dropped: true`
flags plus `filesystem_dotdot_writes_refused: true` and a write-probe count
(15 with the four configured directories and launcher, three spellings each).
Any receipt asserting native execution must carry them. The receipt binds both the launcher and filesystem-helper hashes.
The mount table has a 1,024-entry cap; cap + 1 refuses. No mount or privilege
failure has an unisolated fallback. The [mount namespace contract](https://man7.org/linux/man-pages/man7/mount_namespaces.7.html)
locks inherited read-only flags in less privileged child user namespaces;
[setpriv](https://man7.org/linux/man-pages/man1/setpriv.1.html) prevents the vendor
from retaining mount capabilities or gaining privileges at exec.

## Launch and receipt contract

Lane defaults remain load at most 6 and free space at least 20 GiB. T-R11's
smaller ephemeral runner explicitly selects `-max-load 6 -min-free-gib 1`;
the payload plus private copy needs less than 1 GiB. Check cache/output
filesystems, including during download, and run one job at nice 10.
Preparation alone uses the network. No vendor byte runs on preflight refusal.

```sh
sha256sum -c calibration/oracles/azure-policy/probe-linux-checksums.sha256
python3 -B -m unittest discover -s calibration/oracles/azure-policy -p 'test_*.py'
python3 -B calibration/oracles/azure-policy/probe_filesystem_controls.py -out workspace/artifacts/azure-policy-filesystem-controls
nice -n 10 python3 -B calibration/oracles/azure-policy/probe_linux.py \
  -cache "$RUNNER_TEMP/azure-policy-private/cache" \
  -out "$RUNNER_TEMP/azure-policy-private/out" \
  -publish "$RUNNER_TEMP/azure-policy-public/receipt.json" \
  -min-free-gib 1 -max-load 6
```

Do not run vendor code on the ARM helper. The staged `run_probe_workflow.sh`
redirects all command output privately and deletes private data on exit. Its
whole body is parsed as a shell function before vendor startup. The workflow
stays under calibration/oracles until the controller copies it to the run
branch. The suite includes one real Linux namespace regression, skipped only off
Linux. It independently attempts persistent writes through all three path
spellings, both with and without an old proc submount hidden by fresh procfs.
`probe_filesystem_controls.py` requires that test on Linux and rejects every
skip there. A control failure invokes only the diagnostic refusal path; it
cannot download or start the vendor. Local ARM checks are not a Linux receipt.

Preflight distinguishes userns, netns, pidns, mountns, import and loopback
failures; filesystem_controls identifies failed required regressions after
preflight otherwise succeeds. Kernel capability values are integer/null readings of
apparmor_restrict_unprivileged_userns, unprivileged_userns_clone and
max_user_namespaces. Exceptions and stderr remain private. PID namespace exit
kills all descendants, including detached processes. Normal and timeout
paths also kill the launcher's process group.

The outer launcher pins the closed schema and expected identities in memory
before execution. After namespace teardown it reads a capped regular,
nonsymlink worker receipt and checks its identity and exit/status agreement.
It rewrites only validated JSON to a separate public path. Tail failures
produce an outer-written `receipt_invalid` refusal and exit 2. Launcher logs
use exclusive no-follow creation, and receipt temporary names are fresh and
exclusive. The upload gate is set only after validated outer publication.
No raw vendor string, path or exception is public. LSP capability names are
allowlisted; unknown keys are only counted. No pip dependency is required.

The worker waits up to 10 s for evaluation readiness and runs each of three
authored variants independently. Only all responses and clean shutdown yield
`probe_complete_requires_analysis` / exit 0. Exit 2 covers refusal, timeout,
invalid receipt and native failure. Startup grants no tier or calibration
metric. The C reading stays unchanged; a full native population and position,
unused-declaration, namespace and range controls remain necessary. L3 waits
for D19. The controller reviews the artifact and imports only that validated
receipt into a fresh clean checkout.

## T-R12 runner-derived fixture

The controller supplied `runner-mountinfo-ubuntu-22.04.txt` verbatim from public
GitHub-hosted run **40394016064**, image **ubuntu22/20261012.322**, kernel
**6.8.0-1064-azure**. Its 29 records include the inherited procfs, systemd
binfmt autofs and the binfmt filesystem mounted above it. The parser test
selects 28 reachable mounts with one topmost binfmt path. A second test adds
one explicitly authored procfs overmount row (not part of the capture) and
requires 27 reachable mounts; neither hidden binfmt entry is remounted.
The hidden-mount mutant is tested against this runner-derived case.

The same public run established that `unshare --user --map-root-user --net`
succeeds on 22.04. This is user/network namespace evidence only, not proof
of the full PID/mount isolation or any vendor API. The controller's 24.04
contrast (kernel **6.17.0-1022-azure**) refused at uid_map. Its single captured
stderr line is `runner-userns-ubuntu-24.04.stderr.txt`; a replay verifies refusal
at userns before preparation or vendor startup, with only stderr count/hash
in the receipt. This is a captured negative-control replay, not a local Linux
execution. Neither observation grants a calibration metric or a tier.

## First actual probe: run 40407284519 (T-R13)

The controller imported the unchanged, counts/flags-only
`probe-receipt-run40407284519.json` from public
[run 40407284519](https://github.com/GeoffMillerAZ/fathom/actions/runs/40407284519),
on the disposable `tools/azure-policy-probe-run` branch, ubuntu-22.04 x64.
That run branch is never merged. The receipt SHA-256 is
`ac6dee711f8f4e7dab06b242ea55f2361d95a3e38219838590e3c201c75d626c`.
Its launcher, filesystem helper and input hashes match reviewed source
`d938ba273`. Its original closed v2 schema is frozen separately as
`probe-receipt-run40407284519.schema.json`; newer schemas do not rewrite history.

The vendor executed after measured namespace/write controls (15 EROFS denials,
capabilities dropped, loopback only). Initialization then failed with EOFError,
native return code -25 and zero stderr/LSP bytes. Linux signal 25 is SIGXFSZ.
The 8 MiB process file-size bound is the working hypothesis; this receipt has
no file-attribution evidence, so neither the failing path nor a sufficient new
bound is measured. T-R13 requires diagnostics on the rerun. No API completion,
calibration metric or tier follows from this isolated startup failure.

N3-m1: a mount stacked directly on the namespace root now refuses before any
remount. The previous reachability algorithm could hide the real submounts in
that shape. An authored regression covers this latent case; it is distinct from
the unchanged real runner fixture, which has no stacked root.

## T-R13a rerun: finite child bound and write attribution

T-R13a admits **one instrumented 256 MiB rerun**, with no separate 8 MiB
trace-only run. `RLIMIT_FSIZE` is a process limit, not a per-directory quota
([Linux getrlimit](https://man7.org/linux/man-pages/man2/getrlimit.2.html)).
The private scratch remains the only writable filesystem subtree. After the
separately size-bounded preparation/copy, launcher and worker soft FSIZE limits
are 8 MiB, with a finite 256 MiB inherited hard ceiling. Only the restricted
vendor child raises its soft limit to 256 MiB. This is a provisional diagnostic
bound, not a measured sufficient size or a statement about the first failing file.
There is no unlimited fallback. A second SIGXFSZ is useful evidence, not success.

Stderr and LSP are pipes with 8 MiB capture/protocol bounds; the vendor never
inherits a direct writable launcher-output descriptor. Bounded no-follow regular
log reads also refuse oversized, replaced, symlink or FIFO logs. Trace capture
has a separate 8 MiB bound outside writable scratch. The receipt remains at
its stricter **128 KiB** read **and write** cap, below the 8 MiB output budget.
Capture/parser-integrity failures refuse diagnostics explicitly; bounded row/snapshot overflow retains a visibly partial diagnostic and preserves the validated worker result.

`/usr/bin/strace` must already exist on the runner. Its binary hash and numeric
version are recorded. No apt/global install or untraced fallback occurs. The
[Ubuntu 22.04 strace 5.16 manual](https://manpages.ubuntu.com/manpages/jammy/man1/strace.1.html)
documents the used flags: follow forks, decode descriptor paths, zero buffer
string length, and one quiet flag (retain exit/signal records). Only write,
pwrite/writev, truncate/allocation, copy/send/splice and exec calls are selected.
Raw trace paths, argv, buffers, LSP, evaluation and stderr remain private and are
deleted by the shell launcher. The raw trace is never an artifact.

The tracer runs outside a nested vendor PID namespace. An authored launcher
closes all extra descriptors before creating that PID/mount/proc view, which
remounts everything outside the same scratch read-only. The vendor cannot see
the outer tracer through procfs and inherits no trace-pipe writer. An authored
PID1 waits for the zero-cap vendor child; the vendor has ordinary process signal
semantics, rather than PID1's special semantics. The existing full outer
user/network/PID/mount isolation and 15 EROFS controls remain required.

After the complete worker namespace dies, the outer launcher parses the
protected trace and snapshots HOME/TMP without following symlinks. Current
closed schema **v3** includes `write_diagnostics`:

- `files`: at most 64 rows with allowlisted scratch-relative HOME/TMP names,
  successful write-family return-byte totals, final sizes/types and requested
  allocation or explicit-offset extents. Unsafe names have `relative_path: null`.
  Noncanonical HOME/TMP spellings (dot or empty components) keep distinct private
  raw keys and publish null names. Dot-dot always uses `other`, since a symlink
  can change its target area. Every `other` target, including noncanonical paths,
  payload paths and memfd labels, collapses to fixed classes with no per-path
  identity, preserving the row budget for HOME/TMP evidence. `dotnet_doublemapper` is a fixed kind enum.
  No path is hashed. No raw trace hash or raw trace byte count is published,
  because both can depend on absolute runner roots; `trace_within_cap` records
  the bound instead. Only the strace executable's content hash/version is public.
- `largest_private_file`: largest inspected final regular HOME/TMP file, with its
  relative name (or null when redacted) and size. It is retained even if the row
  array is full. A partial snapshot cannot claim the largest file globally.
- `exceeds_previous_limit` and `files_over_previous_limit`: final size or requested
  extent above 8 MiB; the count covers retained rows. These are rerun observations,
  not evidence of which file caused the earlier uninstrumented death.
- `limit_file`: a same-thread EFBIG followed by SIGXFSZ with one distinct private
  target. Only an allowlisted HOME/TMP relative name may be published; other names
  are null and retain only fixed class/operation/extent. Distinctness is checked
  privately, with no digest or private identity serialized. Multiple targets or a
  signal alone yield null. The native return code comes from exec/exit evidence.
- `files_truncated`, `omitted_file_events`, `snapshot_truncated`: explicit partial
  coverage. Omitted events count row lookups beyond capacity, not distinct files.
  Snapshot traversal admits 256 directories and inspects at most 4,096 entries.
  Before applying that cap it selects names in lexical order, independently of
  readdir order, retaining at most 4,097 directory entries in memory. Selection
  scans directory names to EOF under the workflow's ten-minute job timeout; unselected entries
  are not statted. It never follows symlinks, and retains the largest inspected
  file independently of row capacity.
- `diagnostic_status`: complete, partial, refused or contradicted. Parser-integrity failures
  (including unmatched resumed calls, undecoded descriptor kinds or trace overflow)
  cannot turn an already validated worker initialize/return-code/stderr/LSP result
  into an empty launcher receipt. Diagnostic refusal has a fixed exception class
  and `write_diagnostics: null`; it grants no trace-attested execution claim.
  A parsed trace missing the vendor exec or exit contradicts a worker execution
  claim: keep its parsed diagnostics and worker initialize/stderr/LSP fields,
  but set `native_executed: null` (unknown), `native_returncode: null`,
  `status: probe_refused`, `failed_stage: tracing` and
  `diagnostic_status: contradicted`. The closed schema forbids retaining a native
  execution claim or asserting non-execution in that outcome. A trace-attested exit supersedes the worker
  wrapper return code; a wrapper/native code difference alone is not contradiction.

T-R13a permits only bounded relative HOME/TMP names: ASCII letters, digits, dot,
underscore and hyphen; no dot/dot-dot components; at most 128 bytes per component
and 512 per path. Other names are redacted per row. Relative truncate targets with
unknown cwd use an anonymous fixed class rather than a guessed file. A regression
publishes byte-identical receipts under differently sized scratch and HOME roots.
The maximum-shaped closed receipt, including 64 maximal rows, largest/limit records
and all optional worker fields, encodes to 65,585 bytes under the 128 KiB cap. Allocation extent, bytes
written, mmap stores and final logical size remain distinct quantities.

The runner suite now requires **two** Linux kernel tests with zero skips. The
new authored control uses the production tracer/namespace/child chain, proves
no extra descriptor reaches the child and the outer tracer is absent from its
PID view, checks the inherited 256 MiB bound, lowers only the authored process
to 16 KiB and deliberately extends a TMP file to 32 KiB. It must attribute five
written bytes and the exact EFBIG/SIGXFSZ target. It executes no vendor code.
Off Linux exactly these two tests skip; missing strace or a namespace/control
failure on Linux blocks preparation/vendor execution. Local mocked controls
are not a successful Linux run. Canonical readings and tiers remain unchanged.


## T-R15/T-R16 authored-control refusal and diagnostic gate

Run 40487438655 refused at `filesystem_controls`, with `native_executed: false`.
The vendor never ran; T-R15 leaves the single T-R13a rerun unspent. Its original
counts/flags receipt is `probe-receipt-run40487438655.json`. Diagnostic run
40494795380 then identified the authored Linux write control: 93 tests, one
error (`exact file-size control trace failed`), followed by the generic
boundary-suite refusal.

The setup bug precedes path classification: `mount_readonly(home.parent)` changes
cwd to the scratch root. The nested launcher inherited it, so the authored
relative `tmp/limit-control` open addressed a nonexistent scratch-root TMP
rather than private HOME/TMP. A local regression reproduces ENOENT (encoded
exit66) for both authored and vendor launch modes before the fix. The child now
starts with explicit `cwd=home`; the same actual relative-open regression exits0
and writes five bytes under HOME/TMP. The mount, capability, PID, descriptor and
file-size controls are retained. A separate absolute-fd trace control confirms
that `ftruncate` on a `-yy` decoded target still attests `area: tmp`, relative
`home/tmp/limit-control`, native returncode -25 and the 32 KiB failed extent.
This authored trace is not a captured Linux trace; runner confirmation remains
mandatory.

On assertion failure, the authored control's test error describes the compared
native and outer returncodes, execution/signal/overflow flags, limit-file
area/relative path/extent, and expected file's final size/write bytes. These
bounded authored fields appear only in test diagnostics, never receipt.json;
raw trace, stderr and absolute paths are not included. The production receipt
instead permits only fixed enum `failure_detail: required_authored_controls_failed`
at this launcher gate, never an exception message. Its complete maximum-shaped
receipt is 65,585 bytes, below 131,072.

Local validation is insufficient for READY: the controller must run both commands
below on a GitHub-hosted ubuntu-22.04 x64 candidate, without the vendor. All 97
tests must run with zero errors/skips, and all 15 mutants must fail then restore
green. Off Linux exactly two designated kernel tests skip. Do not relax either
kernel test or the suite gate to proceed. Any new refusal requires another fix
and diagnostic run before READY, then the admitted vendor probe.

```sh
python3 -B -m unittest discover -s calibration/oracles/azure-policy -p 'test_*.py' -v
python3 -B calibration/oracles/azure-policy/probe_filesystem_controls.py -out "$RUNNER_TEMP/controls"
```
