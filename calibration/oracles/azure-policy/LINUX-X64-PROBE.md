# T-R9 / T-R11: Azure Policy oracle pending an x64 run

The controller copies `probe-linux-x64.workflow.yml` from this directory into
`.github/workflows/` on a disposable **run branch that is never merged**. It
runs on push only. T-R11 requires a GitHub-hosted `ubuntu-22.04` x64 job, with
no `container:` job. Do not select `ubuntu-latest` or 24.04: its unprivileged
user-namespace restriction can refuse this route. Do not change sysctls, use
sudo, install system packages, authenticate to Azure or use Docker/Rosetta.
Python3.9+, util-linux unshare and the ordinary .NET Linux system libraries
must already exist; missing prerequisites produce a refusal, never a fallback.
The self-contained payload does not use a global .NET installation.

The job is ephemeral, has only contents:read permission and checks out with
`persist-credentials: false`. Cache, private HOME and output are outside the
checkout under RUNNER_TEMP. **Only the separate validated `receipt.json` is
uploaded. Raw LSP, evaluation, stderr, preflight and launcher logs never leave
the runner.** The private directory and downloaded payload are deleted on exit.
The shell launcher records no raw output in public job logs. The upload step
runs only after the launcher's validated-publication flag, never on a raw file
left by a failed worker. No later script, git command or credentials should be
used from the vendor-exposed checkout. The controller receives the artifact
and, after review, copies only the validated receipt into a fresh clean checkout.

## Pins and execution contract

Microsoft's MIT AzurePolicy extension0.2.6 supplies 455 linux-x64 files,
122,294,968 expanded bytes in two spans totaling52,578,398 bytes. The manifest
SHA256 is `53ab1997c413e02553a8f0b03225cfaed65b53fc7779ba003e10ec4ab67c1ac4`;
the apphost SHA256 is
`bb3603fa212f7444f86aae84e593654b8df9b7feebdd0e4d0b30085fa798f452`.
All files are reverified before any vendor execution. Preparation alone uses
network access; no vendor byte runs on a namespace-preflight refusal.

The defaults remain load<=6 and free>=20GiB for this lane. T-R11's ephemeral
runner uses explicit `-max-load 6 -min-free-gib 1`: the payload needs far less
than1GiB and the standard runner disk is smaller than the lane floor. Both
cache and output filesystems are checked, including during download. Run one
job at nice10. The workflow's launch command is equivalent to:

```sh
sha256sum -c calibration/oracles/azure-policy/probe-linux-checksums.sha256
python3 -B -m unittest discover -s calibration/oracles/azure-policy -p 'test_*.py'
nice -n 10 python3 -B calibration/oracles/azure-policy/probe_linux.py \
  -cache "$RUNNER_TEMP/azure-policy-private/cache" \
  -out "$RUNNER_TEMP/azure-policy-private/out" \
  -publish "$RUNNER_TEMP/azure-policy-public/receipt.json" \
  -min-free-gib 1 -max-load 6
```

Do not run the vendor on this ARM helper. The staged workflow redirects the
commands' output privately; the example above describes the command, not a
permission to publish stdout/stderr. See `run_probe_workflow.sh` for the exact
public-runner procedure. The workflow itself stays under calibration/oracles
until the controller copies it to the run branch.

Preflight distinguishes userns, netns, pidns, import and loopback failures.
`kernel_capabilities` contains only integer/null values for
apparmor_restrict_unprivileged_userns, unprivileged_userns_clone and
max_user_namespaces. The fixed `failed_stage` enum identifies other failures;
full exception text and preflight stderr remain private. The worker runs as
PID1 inside `unshare --user --map-root-user --net --pid --fork --kill-child`,
and measures exactly one network interface, loopback, before vendor Popen.
When PID1 exits the kernel terminates every descendant, including setsid
children. Normal and timeout paths also kill the launcher's process group.

The launcher retains the closed schema and expected identities in memory
before vendor execution. After the PID namespace exits it reads a bounded
regular, nonsymlink receipt, checks allowed fields/types/enums/hashes, rejects
invalid receipts as `receipt_invalid`, then rewrites and separately publishes
only validated JSON. No free-form exception, path or vendor string is public.
Known LSP capability flag names are allowlisted; unknown keys are counted.
The schema is `probe-linux-receipt.schema.json`, enforced by the standard-library
`receipt_contract.py`. No pip dependency is required.

The probe waits up to10s for loopback evaluation readiness, records each of
three authored controls independently, and attempts clean LSP shutdown. Only
clean shutdown and all three evaluation responses can produce
`probe_complete_requires_analysis` / exit0. Exit2 describes refusal, timeout,
invalid receipt or native failure. A failed evaluation does not skip the next
control. Unknown parameter and changed-fact variants keep an unused parameter.

Startup and these controls grant no tier or calibration metric. The C reading
is unchanged. A complete native population with positions and additional
unused/namespace/range controls remains necessary. L3 still waits for D19.
