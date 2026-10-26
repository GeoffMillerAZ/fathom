# Conftest configuration oracle

T-R1 targets only conftest.toml keys at L1. Rego is a separate language lane.

Existing-work search: the pinned native CLI exposes test, parse, verify,
document and other commands, but no own-config dump. `parse` reads an input
file, `verify` executes policy tests, and `document` reads policy metadata;
none is configuration truth. The relevant native entry point is
[commands.readInConfig](https://github.com/open-policy-agent/conftest/blob/744c5637a97d60d9cbc06ff41881749c2dc766b9/internal/commands/default.go),
which the real CLI's PersistentPreRunE calls. It is internal to the upstream
module, so the bench injects a test-only probe into that package in scratch,
compiles it, and removes the injected file. The loader itself is unmodified.

`build_native.py -subject <clean pinned source> -out <scratch binary>` checks
HEAD and cleanliness before/after the build. The source must be complete
(`git sparse-checkout disable` if necessary). It never changes a tracked file.
Run with isolated Go caches. The upstream source is Apache-2.0; it is never
vendored into this repository or imported by the product.

`conftest_oracle.py -subject <checkout> -native <binary> -work <scratch> -out <json>`
asks the native probe to load each config and reports keys the actual native
configuration registry says are in the file (`AllSettings` plus `InConfig`).
No command application body, plugin or Rego policy is executed. The membership
truth is native, tier A; tomli 2.2.1 (MIT) independently validates input and
locates source spans. Lines are placement only, never membership decisions.
A missing/unlocated native key is fatal. Malformed TOML files are counted.
The native probe emits key names only, not configuration values.

The public subject has one config with two single-line keys. This small scope
cannot establish broad configuration support. An authored multiline-value
fixture expects its complete span but the existing shared TOML decoder emits
the key's line as its end (1 instead of 4). Q4 retains that complete-option contract. The incomplete product dialect and its
unchanged failing golden stay on draft branch tools/conftest-r1 at 4c9dfee20;
this branch carries only the independent oracle and written ceiling. No golden
is relaxed and no shared engine is changed.

Tests: `python -m unittest discover -s calibration/oracles/conftest` using the
scratch environment with tomli. The `native_config_test.go.txt` source is
bench-only and deliberately not a package in the fathom module.

## Named candidate audit (X-R1)

| Candidate | Version / identity | Result or concrete limitation |
|---|---|---|
| Native `commands.readInConfig` + Viper registry | Conftest 744c5637a97d60d9cbc06ff41881749c2dc766b9 | Built and run; attests the public config's two keys. Internal function requires the test-only probe; succeeds as membership truth. |
| Native CLI `parse`, `verify`, `document`, `test` | Same source pin | Command implementations consume input data, policy tests, metadata and evaluation inputs respectively. None dumps this CLI's own configuration; no substitute for key membership. |
| Independent TOML positions | tomli 2.2.1 | Validates and locates the full multiline value (test expects lines 1–4); parser evidence alone cannot attest native membership. Used only for placement. |
| Product document decoder | go-toml/v2 v2.3.1 plus fathom `fromTOMLAt` at 5b84aa5b4 | Existing locator delegates array/scalar values without value-end positions. Draft regression reports 1–1 instead of 1–4. This is the implementation ceiling; another oracle cannot fix it. |
| Generic TOML extraction | fathom 5b84aa5b4 | Already reads keys, but does not prove Conftest-specific membership or the complete-option range contract. |

No missing oracle is claimed: native membership works. Q4 and T-R2 require
waiting for the shared locator fix, preserving the draft adversarial golden.

```sh
python3 calibration/oracles/conftest/build_native.py -subject workspace/calibration/conftest/conftest -out workspace/tools/conftest-config-oracle
python3 -m unittest discover -s calibration/oracles/conftest
mkdir -p workspace/tmp/conftest-native
python3 calibration/oracles/conftest/conftest_oracle.py -subject workspace/calibration/conftest/conftest -native workspace/tools/conftest-config-oracle -work workspace/tmp/conftest-native -out workspace/tmp/conftest-native/oracle.json
```

Install tomli 2.2.1 only in a scratch environment and run these commands using
that environment's Python, with isolated Go caches. The source pin is checked
by the builder; the upstream checkout is clean before and after compilation.
This emits native truth, not a language reading: no Conftest dialect is shipped
on this branch and no canonical or holdout grade is integrated.
