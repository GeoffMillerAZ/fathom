# Private CLI execution-adapter experiment

The owner authorized the isolated extension to the [default-off origin candidate](2026-09-15-cli-invocation-origins.md).
Active app-kit, Bosun and Surveyor repositories, public APIs, dependency pins and installed
binaries were not changed. No operational command or real data was used.

## What changed in the isolated copy

Retained root: `/Volumes/m2ext/appkit-evidence/appkit-cli-candidate-authorized.5NutFn/optin/`.
`overlay/command.go` is selected only by a private Go overlay. It preserves default
Execute, and adds experiment-only execution/classifier wiring: explicit Exit codes
win; only complete recognized invocation-origin compositions earn exit2; mixed or
unknown errors retain default policy. It copies the previously reviewed test policy,
not message-based matching. Both copied entrypoints explicitly enable candidate origins.
Existing copied entrypoint tests and the sandbox probe remain unchanged.

The initial private layout could not resolve sibling-relative module replacements.
That pre-build refusal remains in `logs/first-layout-refusal.*`. A private `kits/`
sibling layout then built offline. This was not an active-repository pin repair.

## Results

| Check | Result |
|---|---|
| Full candidate invocation-origin race oracle |149 passing test events, zero failures/skips |
| Each copied consumer package race suite |11 pass /3 fail /0 skip |
| Unchanged sandbox matrix |13/13 pass, runner exit0 |
| Read, write and loopback-network denial controls |All three pass before candidate binaries run |
| Bosun/Surveyor help and version |Exit0 |
| Four unknown-flag/unknown-command cases |Exit2 with expected diagnostics |
| Selected active source comparisons |Unchanged |

The three failure events per consumer are the unknown-flag and unknown-command tests
still expecting exit1, plus their failing parent. They are **real compatibility
failures**, not waived tests. The opt-in candidate intentionally changes those outcomes;
the unchanged sandbox oracle independently requires2. Thus this experiment demonstrates
the selected new policy, not a green existing-consumer migration or release readiness.

| Retained artifact | SHA-256 |
|---|---|
| `logs/candidate-origin-full.jsonl` |`a3bc1506577b0fa8fe8bd80e0ca20319a0bb912f7398b1b88e6ede40ff64e0ef`|
| `trial/run-receipt.json` |`2c899930cbaf8e4cbe2b275163dcb68105b8774116e7a0341f13031c2d9d1d04`|
| Bosun candidate binary |`c7da2ae0efee36836ee25992c0fc538f378ef6c6a7013afc1b64a14cd776bcf7`|
| Surveyor candidate binary |`5547c4965b97a5bfd507b85381bc210e303b8a4777aaa841c2b1b3ed1aed4ed2`|

Root review read the actual overlay and compared its classification with the retained
test-only policy, then inspected the successful runner receipt. Earlier default-off
candidate and active-policy failure receipts remain separate and unchanged.

Remaining decisions: dependency-patch maintenance, explicit public API/migration design,
consumer policy approval and actual delivered upgrade. No automatic classification is
available in the active kit. G5, installed-host proof and terminal interaction acceptance
are not completed by this fixed noninteractive matrix.
