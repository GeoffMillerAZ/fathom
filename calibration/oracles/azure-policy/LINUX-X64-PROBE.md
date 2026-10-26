# T-R9: Azure Policy oracle pending an x64 run

This is a staged probe for the controller's Linux x64 runner. Python 3.9+ and
util-linux `unshare` are required; no pip package, global install, Rosetta,
Docker, Azure login, credential or paid service is used. The runner needs
working unprivileged user/network namespaces and the normal native .NET Linux
system libraries. A namespace or native dependency refusal is a useful receipt;
there is no unisolated execution fallback or automatic system installation.

The source is Microsoft's MIT AzurePolicy extension 0.2.6. The pinned ZIP
directory selects all 455 Linux x64 files (122,294,968 expanded bytes), fetched
through two native-only spans totaling 52,578,398 bytes. The complete manifest
SHA-256 is `53ab1997c413e02553a8f0b03225cfaed65b53fc7779ba003e10ec4ab67c1ac4`;
the apphost SHA-256 is `bb3603fa212f7444f86aae84e593654b8df9b7feebdd0e4d0b30085fa798f452`.
The archive, directory and source identities are in `extension-source-r1.json`;
`native-linux-payload-r1.json` records preparation only, performed without
execution on this lane. No archive member inventory is committed.

From the branch checkout, use fresh output/cache directories under workspace:

```sh
df -h ~
uptime
sha256sum -c calibration/oracles/azure-policy/probe-linux-checksums.sha256
python3 -B -m unittest discover -s calibration/oracles/azure-policy -p 'test_*.py'
nice -n 10 python3 -B calibration/oracles/azure-policy/probe_linux.py -cache workspace/oracle/azure-policy-linux -out workspace/artifacts/azure-policy-linux-r1
```

Run one heavy job at a time, with load at most 6 and at least 20 GiB free.
The driver checks both limits and the full pinned payload before executing.
Preparation alone uses the network. Vendor execution runs in a fresh network
namespace with loopback enabled, a clean environment and an empty private HOME.
No external route exists there. It captures the LSP initialize response, cache
command, definition/use queries, and the documented loopback evaluation API on
three authored controls: valid, unknown parameter and changed input fact.
An unused parameter remains in every control. There is no calibration clone.

The expected receipt shape is `probe-linux-receipt.schema.json`. Read
`workspace/artifacts/azure-policy-linux-r1/receipt.json` after either exit 0
(probe completed, analysis still required) or exit 2 (refusal/failure/timeout).
The receipt contains counts, response hashes and capability flags, not raw
native messages. Raw LSP, evaluation and stderr outputs stay private alongside
it. Preserve those for diagnosis; do not commit them without X-R3/X-R5 review.
Delete the downloaded payload after the receipt and diagnostics are packaged.

Startup and these controls cannot establish tier A recall. A complete native
population, unused/namespace/range controls and full canonical measurement
remain required. The current C reading is unchanged. L3 remains held by D19.
