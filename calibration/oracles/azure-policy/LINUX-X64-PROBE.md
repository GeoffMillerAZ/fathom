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

## Outstanding runner fixture

The stacked-proc parser tests currently use authored mount topology examples.
They are not a captured runner fixture. A fixture grounded in a real runner
mountinfo capture, with source identity and privacy-safe projection, is still
required by the binding round-3 controller finding. The branch is not READY
and the workflow must not execute the vendor until that requirement and review
are complete. This does not waive the mandatory Linux kernel control.
