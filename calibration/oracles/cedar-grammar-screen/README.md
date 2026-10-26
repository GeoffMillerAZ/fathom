# Cedar policy and schema grammar gate

This screen is macOS-only: the driver builds parsers with `cc -O0 -dynamiclib`.

Requires Python 3.12+, a C compiler, tree-sitter CLI 0.27.0 and the official
cedar-policy-cli 4.13.0. Keep tools, subjects, temporary files and outputs in
`workspace/`. This is a native syntax screen, not a WASM, throughput or
extraction-accuracy reading. It has no product dependency.

Fetch the two unchanged corpus subjects at their committed pins:

```sh
git clone https://github.com/cedar-policy/cedar-examples workspace/calibration/cedar-examples
git -C workspace/calibration/cedar-examples checkout c251f7d1ad171bd12dee5d1d7a1cceaec994518f
git clone https://github.com/cedar-policy/cedar-integration-tests workspace/calibration/cedar-integration-tests
git -C workspace/calibration/cedar-integration-tests checkout 1999ea249229e26cabb398a279fea721854a471d
mkdir -p workspace/tools workspace/artifacts
curl -fL https://codeload.github.com/SwornSystems/tree-sitter-cedar/tar.gz/112e886c976951095b43ad91929269f9c1cca17d -o workspace/tools/cedar-sworn.tar.gz
```

The grammar archive must identify the requested upstream commit in its Git PAX
header. The driver extracts it into a temporary workspace directory and builds
its separate `cedar` and `cedarschema` parsers using `cc -O0 -dynamiclib`.
Both LICENSE-MIT and LICENSE-APACHE are present at the measured pins.

The official [Cedar CLI release](https://github.com/cedar-policy/cedar/releases/tag/cedar-policy-cli-v4.13.0)
provides `cedar-policy-cli-aarch64-apple-darwin.tar.xz`, SHA-256
`53c3ddc467519a3371919a92da1167fe143d9eb8efe0b29fdde8237fa0fe9cae`.
The [tree-sitter CLI release](https://github.com/tree-sitter/tree-sitter/releases/tag/v0.27.0)
provides `tree-sitter-macos-arm64.gz`. Unpack local copies under workspace/tools,
verify their `--version` output, then run (paths below are examples):

```sh
python3.13 calibration/oracles/cedar-grammar-screen/screen.py \
  -cedar workspace/tools/cedar-cli4.13.0/cedar-policy-cli-aarch64-apple-darwin/cedar \
  -tree-sitter workspace/tools/tree-sitter-0.27.0 \
  -out workspace/artifacts/cedar-v0.12.0-screen.json
```

For the measured head, fetch its archive by commit and pass
`-archive workspace/tools/cedar-sworn-head.tar.gz -commit 2e9f10824728b13d4c019774b32ca0a883e97f96`.
Subject location flags `-examples` and `-integration` support local checkouts.
The versioned and head receipts are committed under `docs/assessments/data/`.

Every `.cedar` and `.cedarschema` file is offered to Cedar's own `check-parse`
first. Only its accepted population is the gate: the script requires exactly
20 policies/14 schemas and 22 policies/3 schemas, at clean pinned subject HEADs.
It invokes `tree-sitter parse -q --lib-path ... --lang-name ...` on each file.
Nonzero parse exits count ERROR/MISSING files; timeouts and other exit statuses
are separate faults. The 10-second per-file timeout is disclosed in the receipt.

Content hashes cover sorted subject-relative paths, NUL, raw SHA-256(content),
NUL. Error/fault set hashes cover each sorted relative path followed by NUL.
Receipts contain counts and hashes, never subject file lists or source content.
No holdout enters the canonical corpus. Observe the lane's nice/load/one-heavy-job
rules when reproducing.
