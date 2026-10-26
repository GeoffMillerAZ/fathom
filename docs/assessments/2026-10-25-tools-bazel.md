# Bazel L1: native truth available, macro classification held

T-R4 permits this L1 audit. Canonical bazel.json stays unchanged: aggregate
symbol precision .9554 (2505/2622); L3 evidence and regrading wait for D19.
A native Bazel 8.7.0 query on grpc 3252a89f exits 0. Its existing workspace
boundary receipt scopes the diagnostic; no hard file or declaration is removed.

The L1-only diagnostic reads 2401/2518 definitions (precision .9535, recall 1)
on grpc alone. Those are the canonical grpc subject's counts, not an extractor
improvement. All 117 extra definitions were inspected with the committed
`calibration/oracles/bazel-query/audit_l1.py` source classifier:

| Source callee | Extra definitions |
|---|---:|
| grpc_package | 110 |
| bind | 2 |
| custom_exec_properties and exec_properties.custom_exec_properties | 2 |
| python_register_toolchains, system_python, system_pip_parse | 3 |

The pinned grpc `bazel/grpc_build_system.bzl:817` body changes package visibility
and calls `native.package`; it creates no target. These 110 are genuine syntax
over-reads, not a missing native oracle. The remaining seven are WORKSPACE or
MODULE declarations absent from the selected native loading result. They remain
in the denominator; no adjudication, kind remapping or exclusion hides them.

A literal `name=` argument does not imply an instantiated target. Hardcoding
the subject's macro name violates the org-agnostic contract; rejecting every
`_package` suffix would suppress actual rule-producing macros. A general fix
must identify the callee and its behavior across files or evaluate the build,
a product capability beyond the source query and excluded by T-R2's shared
ENGINE hold. The named, versioned candidate/route results and reproduction
commands are in the oracle README. Native truth is available and working;
this is an implementation ceiling, not an assertion that another tool is absent.

Separately, tests exposed that the adapter emitted tier A after native query
or version failure. It now refuses all failed queries, including partial exit 3,
and missing versions before writing JSON. Native successful results remain
unchanged. The diagnostic has no independent three-run throughput claim.
No product extraction, epoch, generated surface or checkbox changed.
Decision models: none; native query acceptance and source mismatch counts are
deterministic and invoke no model.
