# OpenCL C native grammar gate

The experimental OpenCL C patch improves the dedicated grammar from **138/386** to
**63/386 ERROR-or-MISSING files**, but 16.32% remains above the unchanged 5% grammar gate.
The native candidate is proposed for refusal. This result adds no product WASM, dialect,
reading, language level or canonical corpus, and it trims no subject or file. Relative to the dedicated grammar, 75 files become clean and none regress to errors or faults.

This is a grammar-gate disposition, not an oracle ceiling. Official LLVM 21.1.8 on macOS
ARM64 has qualified the existing reconstructed compiler pipeline at tier B. Other semantic
routes in the [search receipt](../../calibration/oracles/opencl-c/SEARCH.md) remain untried,
but none can repair a raw syntax candidate that fails admission.

## Measured gate

The screen retained all eight fixed primary projects: **386 files, 382 distinct Git blobs
and 4,256,615 bytes**. Every candidate parsed the same captured bytes. Duplicate blobs
remain separate file observations, all checkouts stayed unchanged, and every one of the 40
primary measurements completed with zero faults.

| Native candidate | Source pin | ERROR-or-MISSING | Rate | Disposition |
|---|---|---:|---:|---|
| Experimental OpenCL C patch over tree-sitter-c | tree-sitter-c v0.24.2, `b780e47fc780ddc8da13afa35a3f4ed5c157823d`, plus pinned patch | 63/386 | 16.32% | Fails native gate |
| Dedicated tree-sitter-opencl | HEAD checked 2026-10-26, `8e1d24a57066b3cd1bb9685bbc1ca9de5c1b78fb` | 138/386 | 35.75% | Fails native gate |
| tree-sitter-c | v0.24.2, `b780e47fc780ddc8da13afa35a3f4ed5c157823d` | 380/386 | 98.45% | Fails native gate |
| tree-sitter-cpp | v0.23.4, `f41e1a044c8a84ea9fa8577fdd2eab92ec96de02` | 380/386 | 98.45% | Fails native gate |
| tree-sitter-cpp | HEAD `8b5b49eb196bec7040441bee33b2c9a4838d6967` | 380/386 | 98.45% | Fails native gate |

The patched builder also passed all **137 authored and inherited corpus cases** with a fresh
generated parser. Passing that corpus does not override the complete-project gate. The
public receipt targets are [screen.json](data/opencl-c-grammar-gate-2026-10-26/screen.json)
and [builds.json](data/opencl-c-grammar-gate-2026-10-26/builds.json).
[Verification](data/opencl-c-grammar-gate-2026-10-26/verification.json) records the 83
focused Python tests, patch-engine race check, required gates and privacy checks.

Hashcat remains diagnostic under the recorded structural exception. The pristine dedicated
and C grammars each fail 1,344/1,349 files there, with zero faults. The experimental patch
was not included in that diagnostic comparison. The mandatory pristine dedicated/C
measurements remain reported; no template or macro prepass was implemented. Hashcat's
computed inputs would need independently evidenced compiler context for later semantic grading.

## Why the patch still fails

A manual audit of all 63 retained failing files assigns the following mutually exclusive
classes. These labels explain the diagnostics; the mechanical gate counts every
ERROR-or-MISSING file independently of the labels:

| Residual class | Files | Why the raw grammar cannot admit them |
|---|---:|---|
| clBLAS host-string templates | 40 | The tracked `.cl` files are host-side template fragments rather than complete raw OpenCL C translation units. |
| Macro expansion required | 11 | Parsing depends on macro-expanded syntax unavailable to the raw-byte grammar screen. |
| Configuration fragments | 11 | The files require independently selected build configuration or assembly context. |
| Conditional C++ linkage | 1 | Opening and closing C++ linkage braces are split across separate conditional directives; parsing needs the matching configuration context. |

Removing files would change the population and is prohibited. No template or macro
preprocessing is implemented; G-R6 bars the requested clBLAS prepass at these counts. Even if the 40 clBLAS templates were set aside, the other
23 failures still exceed the gate's maximum 19 failing files. OpenCL receives no exceptional
threshold.

## Named candidates and routes

| Candidate or route | Version or pin | Concrete disposition |
|---|---|---|
| lefp/tree-sitter-opencl | `8e1d24a57066b3cd1bb9685bbc1ca9de5c1b78fb`, MIT | Complete fixed-pool screen fails 138/386. The PyPI 0.0.2 package is this grammar repackaged with bindings. |
| Experimental tree-sitter-c patch | tree-sitter-c v0.24.2 / `b780e47f`, MIT, patch digest pinned in `grammar-candidates.json` | Fresh generation and 137/137 corpus cases pass, but the unchanged pool fails 63/386. |
| tree-sitter-c | v0.24.2 / `b780e47f`, MIT | Stock C lacks OpenCL syntax and fails 380/386. |
| tree-sitter-cpp | v0.23.4 / `f41e1a04`, MIT | Stock release fails 380/386 and adds no OpenCL-specific recovery. |
| tree-sitter-cpp | HEAD / `8b5b49eb`, MIT | Current pinned HEAD also fails 380/386, so newer C++ grammar work does not change the result. |
| Upstream LLVM compiler pipeline | LLVM 21.1.8 macOS ARM64 official archive, Apache-2.0 with LLVM exceptions | Qualified at reconstructed tier B. It supplies later semantic truth but is not a raw tree-sitter grammar or admission repair. Other planned platforms remain unqualified. |
| `pycparserext` plus `pycparser` | 2026.1 plus 3.0, MIT plus BSD-3-Clause | Untried AST route with start coordinates and syntactic calls, but no end ranges, binding or typechecking. It cannot substitute for the product grammar gate. |
| `openclio` | 0.3, MIT | Untried Clang-backed kernel-argument classifier. Its narrow role and compiler dependency do not supply a native grammar. |
| scip-clang | in-tree v0.4.0 / `1860adf8`; current v0.4.1 / `14423f07`, Apache-2.0 | Stock v0.4.0 refuses `.cl` main files before semantic analysis; extension and alias routes remain untried. Semantic output would not cure raw parse admission. |
| clangd and Galarius OpenCL language server | LLVM `e4840295`, Apache-2.0 with LLVM Exceptions; Galarius `c3b8b8c`, MIT | Both are untried semantic routes requiring compile or runtime context. Neither is an admitted product grammar. |

No semantic-tool ceiling is claimed. Package, SCIP/LSIF, language-server, runtime and
grammar searches contain explicit misses and untried alternatives in `SEARCH.md`.
[Exact bounded search and license receipts](../../calibration/oracles/opencl-c/search-receipts.json)
record query URLs, access results, response digests and the named license evidence.

## Holdout and retry trigger

The Intel compute-samples holdout was pinned before tuning at
`1b2d5d2afff61cdc7370778849f848b76cee00a6`. Every one of its 53 files
(177,237 bytes) remains in its separate measurement. The dedicated grammar fails
25/53, C and both C++ candidates fail 50/53, and the experimental patch fails
2/53 (3.77%), all with zero faults. All checkout identities stayed unchanged.
The [holdout receipt](data/opencl-c-grammar-gate-2026-10-26/holdout.json) records
its frozen rule, source identities and canonical-membership audit. No holdout
file, pin or result enters a canonical corpus shard or the 386-file gate.
Passing this separate holdout cannot rescue the failing primary pool, and no
post-holdout grammar tuning is included. A final patch-format cleanup removed only
empty context prefixes; exact grammar, corpus and generated parser bytes stayed
unchanged, and the full primary/holdout rerun reproduced every count.

Retry native admission only when a new licensed grammar or an independently validated
product-context solution clears the **whole unchanged fixed pool at no more than 5%
ERROR-or-MISSING under G-R6**. If the route performs template or macro preprocessing, its
convention must first be evidenced by at least two independent OpenCL projects and must
preserve every file the oracle grades. Mere availability of another parser, compiler or
language server does not fire the trigger.

ADR-048 and LB-Q-opencl govern the controller's queue disposition. This assessment proposes
the native refusal only. It changes no queue row, plan checkbox, ADR or generated file.
