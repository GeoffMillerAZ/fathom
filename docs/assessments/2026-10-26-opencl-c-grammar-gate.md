# OpenCL C native grammar gate

The experimental OpenCL C patch improves the dedicated grammar from **138/386** to
**63/386 ERROR-or-MISSING files**. A subsequent general semicolonless-macro grammar
repair lowers that result to **53/386 (13.73%)**, still above the unchanged 5% grammar gate.
The native candidate is proposed for refusal. This result adds no product WASM, dialect,
reading, language level or canonical corpus, and it trims no subject or file. Relative to the original patch, 10 additional files become clean with zero regressions or faults. The original 75 dedicated-to-patch transitions remain historical.

This is a grammar-gate disposition, not an oracle ceiling. Official LLVM 21.1.8 on macOS
ARM64 has qualified the existing reconstructed compiler pipeline at tier B. Other semantic
routes in the [search receipt](../../calibration/oracles/opencl-c/SEARCH.md) remain untried,
but none can repair a raw syntax candidate that fails admission.

## Measured gate

The screen retained all eight fixed primary projects: **386 files, 382 distinct Git blobs
and 4,256,615 bytes**. Every candidate parsed the same captured bytes. Duplicate blobs
remain separate file observations, all checkouts stayed unchanged, and the original
40 measurements plus the later paired 16 measurements completed with zero faults.

| Native candidate | Source pin | ERROR-or-MISSING | Rate | Disposition |
|---|---|---:|---:|---|
| OpenCL C patch plus semicolonless macro statements | tree-sitter-c v0.24.2 plus two pinned patches | 53/386 | 13.73% | Fails native gate |
| Experimental OpenCL C patch over tree-sitter-c | tree-sitter-c v0.24.2, `b780e47fc780ddc8da13afa35a3f4ed5c157823d`, plus pinned patch | 63/386 | 16.32% | Fails native gate |
| Dedicated tree-sitter-opencl | HEAD checked 2026-10-26, `8e1d24a57066b3cd1bb9685bbc1ca9de5c1b78fb` | 138/386 | 35.75% | Fails native gate |
| tree-sitter-c | v0.24.2, `b780e47fc780ddc8da13afa35a3f4ed5c157823d` | 380/386 | 98.45% | Fails native gate |
| tree-sitter-cpp | v0.23.4, `f41e1a044c8a84ea9fa8577fdd2eab92ec96de02` | 380/386 | 98.45% | Fails native gate |
| tree-sitter-cpp | HEAD `8b5b49eb196bec7040441bee33b2c9a4838d6967` | 380/386 | 98.45% | Fails native gate |

The original patch passed **137** corpus cases; the semicolonless candidate passed
**145** with a fresh generated parser (parser.c SHA-256
`a80b70ec3ff815bf6c80a2f86f2035ee2a0f1d838cc48c40f108a22fde4ef116`).
Passing that corpus does not override the complete-project gate. The
original [screen.json](data/opencl-c-grammar-gate-2026-10-26/screen.json) and
[builds.json](data/opencl-c-grammar-gate-2026-10-26/builds.json) preserve the historical
five-build descriptor and five original candidate metadata records. The added row above brings
the named grammar candidates to six. New receipts: [primary screen](data/opencl-c-grammar-gate-2026-10-26/semicolonless-screen.json),
[145-case build](data/opencl-c-grammar-gate-2026-10-26/semicolonless-build.json), and
[fresh original-patch rebuild](data/opencl-c-grammar-gate-2026-10-26/baseline-review-build.json).
[Verification](data/opencl-c-grammar-gate-2026-10-26/verification.json) records the 91
focused Python tests, patch-engine race check, required gates and privacy checks.

Hashcat remains diagnostic under the recorded structural exception. The pristine dedicated
and C grammars each fail 1,344/1,349 files there, with zero faults. The experimental patch
was not included in that diagnostic comparison. The mandatory pristine dedicated/C
measurements remain reported; no template or macro prepass was implemented. Hashcat's
computed inputs would need independently evidenced compiler context for later semantic grading.

## Original residuals and the general-grammar route

The original 63-file candidate is a failing native screen, not proof that its
raw grammar cannot admit every residual. The reproducible
[63-row subset receipt](data/opencl-c-grammar-gate-2026-10-26/residual-labels.json)
records each failing subject-relative path, first ERROR/MISSING kind and line,
source hash, Git blob and subject pin. These paths are explicitly authorized by
the binding review. Manual labels are separate from the mechanical failure count.

| Original residual class | Files |
|---|---:|
| clBLAS host-string templates | 40 |
| Semicolonless macro invocations | 10 |
| Macro-parameter fragments | 7 |
| Conditional-branch fragments | 2 |
| Macro-generated syntax | 2 |
| Raw missing semicolon | 1 |
| Conditional C++ linkage wrapper | 1 |

The binding review identified a general statement/top-level-only macro route.
The new candidate uses the existing identifier token, requires a physical newline
after the argument list, and gives complete declarations higher dynamic priority.
It passes 145 corpus cases, including malformed assignment/newline-boundary
negatives and unchanged valid macro-typed declarations. One previous semicolonless
negative now intentionally becomes a macro statement, not a function definition.
Raw syntax cannot distinguish a bare ordinary call from a macro invocation with
the same bytes; this extension is not a compiler-validity assertion.

The route measures **53/386** on the whole frozen pool: 40 clBLAS and **13 non-clBLAS**
failures, with 10 error-to-clean transitions and zero regressions or faults. Its new
residual classes are 40 host-string templates, 7 macro-parameter fragments,
1 semicolonless invocation, 2 conditional-branch fragments, 1 macro-generated
syntax, 1 raw missing semicolon and 1 conditional C++ wrapper. The original 63-row
labels above remain unchanged. The G-R6 intermediate trigger of at most **19**
non-clBLAS failures has **fired**.
That intermediate trigger does not require the whole pool already to pass 5%,
and it does not itself authorize preprocessing. A later prepass must be runtime
product behavior, recognize a convention used by two independent projects,
preserve every file and receive the owner ruling required by G-R6.

## Named candidates and routes

| Candidate or route | Version or pin | Concrete disposition |
|---|---|---|
| lefp/tree-sitter-opencl | `8e1d24a57066b3cd1bb9685bbc1ca9de5c1b78fb`, MIT | Complete fixed-pool screen fails 138/386. The PyPI 0.0.2 package is this grammar repackaged with bindings. |
| Semicolonless tree-sitter-c patch | tree-sitter-c v0.24.2 / `b780e47f`, MIT, two pinned patches | 145/145 corpus cases pass; unchanged pool fails 53/386. No product grammar registered. |
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
The original 4a4f55c29 candidate included no post-holdout tuning; its final
patch-format cleanup preserved exact grammar/corpus/parser bytes. The binding
review now authorizes a new grammar route after this holdout was read. Its [remeasurement](data/opencl-c-grammar-gate-2026-10-26/semicolonless-holdout.json)
remains **2/53**, zero faults and unchanged source, but is a **reused diagnostic after
tuning**, not independent validation, and cannot rescue a failing primary pool.

The [bounded convention search](../../calibration/oracles/opencl-c/template-convention-search.json)
now includes authenticated replacements for the four initial HTTP 401 requests:
all four scoped queries returned HTTP 200 and zero hits. Later scoped
`__kernel` positive controls returned 3, 21, 37 and 28 hits, establishing index
participation for every candidate, without proving complete file indexing. Three global exact-token
queries returned 32, 92 and 92 hits across the same four numeric repository IDs.
Every query fits one complete 100-item page with `incomplete_results=false`.
The matches are clBLAS and three derived template trees, not a second independent
implementation. The query-hit union contains 25 indexed template paths per repository, a
subset of the pinned reference directory's 40 `.cl` blobs. Indexed-hit exact
blob matches are 25/25/25/22; only the last tree has three differing blobs.
These are indexed-hit counts, not full-directory counts for the derived trees.
Non-subject names and URLs are omitted from the current receipt under X-R9.
The original hand-picked candidate pins, sample URLs, hashes and findings
were already public in v1 at main `440a94365`; the receipt links that exact
historical record and each candidate row. They have not become private.
The v1 owner question is preserved and remains governed by G-R6; this update
grants no prepass authorization.
This closes the avoidable authentication gap, not the service's indexed-search
limits. The intermediate numeric trigger still authorizes no runtime prepass.

Retry native admission only when a new licensed grammar or an independently validated
product-context solution clears the **whole unchanged fixed pool at no more than 5%
ERROR-or-MISSING under G-R6**. If the route performs template or macro preprocessing, its
convention must first be evidenced by at least two independent OpenCL projects and must
preserve every file the oracle grades. Mere availability of another parser, compiler or
language server does not fire the trigger.

ADR-048 and LB-Q-opencl govern the controller's queue disposition. This assessment proposes
the native refusal only. It changes no queue row, plan checkbox, ADR or generated file.

## Review corrections

The fresh-checkout compdb test failed once because `workspace/` did not exist.
The test now creates that parent, and all original 83 tests passed from a fresh
no-hardlinks clone that began without `workspace/`. The residual-audit tests and
fresh native replay are additional checks, not part of the original 83 count.

[The public X-R8 receipt](data/opencl-c-grammar-gate-2026-10-26/xr8-sites.json)
lists the eight original scanner sites and the methods actually used. The two
API-response hashes explicitly use compact JSON serialization, not client-invariant
raw bytes, and OpenCLio now uses one canonical repository owner. Verification
contains the same public site/method list; it no longer requires a private LOG.
The patch-directory README identifies both candidates as calibration-only, failed
admission and unregistered. Nothing in that directory authorizes a product lock row.
