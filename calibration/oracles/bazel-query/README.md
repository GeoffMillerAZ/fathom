# bazel-query — tier-A oracle for Bazel (LB-Q-bazel)

Bazel's own loading phase, read as `indexbench lang-oracle`'s neutral oracle file. Python 3 standard library only,
bench-only, never a product dependency. Bazel 9.3.0 (bazelisk) and 8.7.0 (grpc's `tools/bazel` wrapper).

```sh
BAZELISK_HOME=<tools>/bazel/home python3 calibration/oracles/bazel-query/bazel_oracle.py \
    -subject <workspace dir> -output-user-root <scratch> > workspace/tmp/oracle/bazel-<subject>.json
python3 calibration/oracles/bazel-query/bazel_oracle.py -subject <workspace dir> -scope-globs    # for -only
indexbench lang-oracle -dialect bazel -subject <workspace dir> -subject-name <owner/repo> [-only <globs>] \
    -tool-oracle workspace/tmp/oracle/bazel-<subject>.json -out docs/bench/readings/lang/bazel.json
```

It runs `bazel query //... --output=streamed_jsonproto --proto:instantiation_stack --proto:definition_stack
--noimplicit_deps --keep_going` in the workspace. The query needs no network, but Bazel fetches the workspace's module
dependencies before it loads a package (O1347), so the first run per workspace does.

| Field | Source | What it is |
|---|---|---|
| `definitions` | a rule's `location` (the BUILD call) and the call's source text | `(BUILD file, x, line of the name literal, line of the closing parenthesis, target)` for each call whose literal `name = "x"` is a rule Bazel created, or the `generator_name` Bazel recorded on the rules a macro created |
| `references` | the same call | `(BUILD file, x, callee)` for the rule or macro the call names, and `(BUILD file, x, y)` for each label spelled `":y"` in the call that is a `rule_input` of a rule it created |
| `cross_references` | `instantiation_stack` and `definition_stack` | `(file, the target or function making the call, the .bzl file defining what it calls)`: consecutive frames of the instantiation stack (the BUILD call, the macro's frames) and the last frame to the stack of the `rule()` call. File granularity (`to` empty). A frame outside the workspace (`@rules_cc`) is dropped and counted |

This adapter uses the native loading phase for L1, rather than treating a source parser as native target truth. A rule's `location` is the BUILD call, so the oracle's end line is
the call's closing parenthesis, found by scanning the text with strings and comments skipped.

What the oracle leaves out, counted on stderr per subject: a call whose `name` is computed or whose macro creates no
rule named it (`name_computed`, e.g. grpc's `grpc_package`), a spelled label that is no input of the rules it created
(`label_not_input`), a frame outside the workspace (`xref_external`). `//...` does not reach a directory named in
`.bazelignore` or one with its own MODULE.bazel, WORKSPACE, WORKSPACE.bazel or REPO.bazel; `-scope-globs` prints the
reachable files for `-only`.

Runs (2026-10-19): bazelbuild/examples `rules` (54 rules, 54 definitions, 76 references, 39 cross references),
`configurations` (50 call sites, 67 references, 25), grpc/grpc v1.84.0 (11,178 rules, 2,466 call sites, 2,450
definitions, 3,297 references, 2,024 cross references).

## Native failure is not empty truth (tools L1 audit)

A nonzero query exit (including `--keep_going` partial results), a failed version
probe, or an empty version now exits nonzero before any oracle JSON is written.
Previously those paths could publish tier-A output with missing/partial or
unidentified native evidence. Tests use process-failure doubles only to exercise
this refusal; no mock result is used as calibration truth.

`python3 -m unittest discover -s calibration/oracles/bazel-query` checks both
failure paths. The native development-class audit uses Bazel 8.7.0 in batch
mode with bounded JVM/loading concurrency, a scratch output root and a scratch
repository cache. The version wrapper must pass `--version` directly, since
combining it with startup options makes Bazel reject it. L1 diagnostic readings
keep only the native definitions; they do not regrade L3 before D19.

## T-R4 L1 audit and X-R1 candidates

| Candidate / route | Version / identity | Concrete result |
|---|---|---|
| Native `bazel query` plus instantiation/definition stacks | Bazel 8.7.0; grpc 3252a89f | Query exits 0 and attests rule-producing calls. It does not turn package metadata or ignored WORKSPACE declarations into configured targets. Available native truth, not an oracle ceiling. |
| Existing query adapter | This directory; native 8.7.0 | Previously emitted tier A after query/version failure. Tests now require refusal, including partial exit 3. Native successful output is unchanged. |
| Existing source query | tree-sitter-starlark v1.3.0, as recorded in the dialect query provenance | A literal `name=` call is syntax, not proof of a generated target; the query does not evaluate an arbitrary macro body. Removing a subject-specific callee or every `_package` suffix would hide valid declarations elsewhere. |
| Cross-file macro-body evaluation | fathom 5b84aa5b4 | No such product evaluation exists in this dialect. Requires shared capability/design and affected-item regrades; T-R2 excludes ENGINE work this wave. Native measurement cannot supply that capability to the product. |

The native executable's published SHA256 is
`575f20fb23955e02f73519befd180df635b4ed0960c60f0e70fcc8d74014a713`.
The L1-only diagnostic intentionally discards the native oracle's reference and
cross-reference fields before invoking the recipe; L3 remains held for D19.
Use the existing native workspace-boundary scope receipt, with no new exclusion.

After building `workspace/tools/indexbench`, with Bazel 8.7.0 at
`workspace/tools/bazel` (batch mode is recommended on a shared machine):

```sh
mkdir -p workspace/tmp/bazel-l1
python3 calibration/oracles/bazel-query/bazel_oracle.py -subject workspace/calibration/bazel/grpc -bazel "$PWD/workspace/tools/bazel" -output-user-root "$PWD/workspace/tmp/bazel-output" > workspace/tmp/bazel-l1/native.json
python3 -c 'import json; p="workspace/tmp/bazel-l1/"; d=json.load(open(p+"native.json")); json.dump({k:d[k] for k in ("tool","version","tier","definitions")},open(p+"definitions.json","w"))'
scope=$(python3 calibration/oracles/bazel-query/bazel_oracle.py -subject workspace/calibration/bazel/grpc -scope-globs)
workspace/tools/indexbench lang-oracle -dialect bazel -subject workspace/calibration/bazel/grpc -subject-name grpc/grpc -tool-oracle workspace/tmp/bazel-l1/definitions.json -only "$scope" -out workspace/tmp/bazel-l1/reading.json -unmatched workspace/tmp/bazel-l1/unmatched.json
python3 calibration/oracles/bazel-query/audit_l1.py -subject workspace/calibration/bazel/grpc -unmatched workspace/tmp/bazel-l1/unmatched.json
python3 -m unittest discover -s calibration/oracles/bazel-query
```

Start with fresh output paths. The diagnostic's timing is not an independent
three-run throughput reading. Canonical `bazel.json` is unchanged. The callee
classifier reports unknowns instead of inventing a semantic classification;
it only groups source mismatches and never removes or adjudicates a definition.

## X-R4 sources searched, 2026-10-25

| Required source | Named candidate / concrete result |
|---|---|
| Owner toolchain / introspection | Bazel 8.7.0 query, cquery/aquery documentation, rule generator attributes and native instantiation/definition stacks checked. The existing successful query supplies native target truth; the blocker is product classification, not absent native output. |
| SCIP and LSIF lists plus GitHub | Read [Sourcegraph supported indexers](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP list](https://github.com/sourcegraph/scip#tools-using-scip), and Sourcegraph-maintained [LSIF.dev](https://lsif.dev/). No Starlark entity producer listed. GitHub queries `scip-bazel in:name` (one SCIP/SOPLEX optimization build) and `scip-starlark in:name` (zero) produced no applicable indexer. SCIP Java/Clang build integrations index their languages, not arbitrary Starlark macro target production. |
| Language servers | [Starpls 0.1.22](https://github.com/withered-magic/starpls/tree/v0.1.22) (Apache-2.0) source/interface inspected, not run: navigation/type checking loads Bazel repository mappings and source definitions. An independent LSP is C under P-T1 and its existence does not add cross-file macro behavior to the product. Native query already supplies the relevant A truth. |
| Registry parsers / type checkers / AST dumpers | Bazel Central Registry [buildifier_prebuilt 10.1.0](https://registry.bazel.build/modules/buildifier_prebuilt/) and [buildtools 10.1.0](https://github.com/bazelbuild/buildtools/tree/v10.1.0), plus Go package registry [Starlark syntax](https://pkg.go.dev/go.starlark.net/syntax) (4811d7e revision) checked. These expose parsing/formatting/linting; parsing a named call does not attest a generated target. No candidate was installed or called a failed native run. |
| Runtime call / require tracing | Bazel 8.7.0 [Starlark CPU profiling](https://bazel.build/versions/8.7.0/reference/command-line-reference#flag--starlark_cpu_profile), query generator attributes and [macro stack tracing](https://bazel.build/extending/legacy-macros) checked. Native instantiation stacks are already the adapter's evidence. Profiling/print traces do not supply the missing product macro classifier; no subject macro was changed to force a diagnostic. |

X-R3's mandatory repository-name scan passes separately; assessment publication
contains grouped counts and repo-relative summaries, not subject bodies or a
full file inventory. T-R2's shared ENGINE hold and D19's L3 gate remain binding.
