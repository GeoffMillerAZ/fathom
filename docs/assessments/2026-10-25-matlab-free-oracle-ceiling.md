# MATLAB: free-oracle ceiling

Assessment date: 2026-10-25. The [committed reading](../bench/readings/lang/matlab.json) remains the authority for support levels and measurements. This survey records why the reviewed free tools do not yet provide call and resolution truth for the unchanged corpus; no new oracle run is claimed. Decision: `LB-Q-matlab-rest`.

## Candidate inventory

Source inspection on 2026-10-25 found these routes. None supplied an independent L2 call-pair dump or L3 file-resolution truth over the unchanged corpus. No new L2/L3 oracle run was made.

| Candidate and pin | Concrete ceiling on the existing corpus |
|---|---|
| [MISS_HIT Core 0.9.44](https://pypi.org/project/miss-hit-core/0.9.44/) | The existing L1 adapter emits definitions. `identifier(arguments)` remains an ambiguous `Reference`: `m_ast.py` says it could be indexing or a call, with semantic rewriting still TODO. The parser's definite command-syntax calls do not resolve all references. |
| [Full MISS_HIT 0.9.44](https://pypi.org/project/miss-hit/0.9.44/) | Its `m_sem.py` pass 1 adds package and class scopes but does not rewrite `Reference` into call or index nodes. It therefore adds no full-corpus L2 call truth. |
| [tree-sitter-matlab 1.3.1](https://docs.rs/crate/tree-sitter-matlab/1.3.1) | Its `function_call` node also denotes matrix and cell indexing. A syntax query cannot independently separate calls. |
| [McLab Tamer beeea16c](https://github.com/Sable/mclab-core/blob/beeea16c1c79b3005f3c46363b8c49d1928a937b/languages/Natlab/src/natlab/tame/callgraph/SimpleFunctionCollection.java#L84-L97) | Typed analysis needs input descriptions. The input-type-free [SimpleFunctionCollection](https://github.com/Sable/mclab-core/blob/beeea16c1c79b3005f3c46363b8c49d1928a937b/languages/Natlab/src/natlab/tame/callgraph/SimpleFunctionCollection.java#L148-L177) is entry-point-rooted, rejects scripts and classdef, and [throws on unresolved references](https://github.com/Sable/mclab-core/blob/beeea16c1c79b3005f3c46363b8c49d1928a937b/languages/Natlab/src/natlab/tame/callgraph/SimpleFunctionCollection.java#L205-L231). The pinned corpus contains scripts and classdef; a function-only graph would change its population. |
| [SMOP 0.41](https://pypi.org/project/smop/0.41/) (MIT) | Its [tagged resolver](https://raw.githubusercontent.com/victorlei/smop/0.41/smop/resolve.py) uses local definitions to classify undefined names as typical calls, but does not bind a callee to a source file. The [tagged parser](https://raw.githubusercontent.com/victorlei/smop/0.41/smop/parse.py) has no `classdef` grammar; the pinned corpus contains classdef files. No full-population L2 oracle result is established. |
| [MATLAB language server v1.3.13](https://github.com/mathworks/MATLAB-language-server/releases/tag/v1.3.13) | Its [README at v1.3.13](https://github.com/mathworks/MATLAB-language-server/blob/v1.3.13/README.md) requires MATLAB R2021b or later. Free server source does not remove the licensed runtime requirement. |
| [GNU Octave 11.1.0](https://docs.octave.org/octave-11.1.0.pdf) | The §13.6 profiler records executed calls, not a static whole-corpus call dump. Octave semantics also differ from MATLAB semantics, so this GPL interpreter would need separate-process use and would not establish independent L2 MATLAB truth. |
| [`requiredFilesAndProducts`, R2026b documentation](https://www.mathworks.com/help/matlab/ref/matlab.codetools.requiredfilesandproducts.html) | The documentation reviewed 2026-10-25 describes file dependencies and requires [licensed MATLAB](https://www.mathworks.com/help/install/install-products.html); it does not report L2 call pairs. **R2026b is the documentation version only: no MATLAB release was surveyed or run.** An L3 measurement would require an approved installation and the same corpus. |

## Ceiling and retry

No free L2/L3 oracle has been run that distinguishes function calls from indexing over the unchanged corpus. Do not raise the reading, lower the target thresholds, or fabricate input types to produce call truth.

Retry on a free analyzer that covers classes, scripts and function resolution over the pinned subjects with an independent call dump, or approval and availability of MATLAB's own analyzer. McLab's restricted graph does not meet that condition. No oracle throughput or coverage is asserted here.
