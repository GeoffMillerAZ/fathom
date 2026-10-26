# perl named ceiling routes (X-R1)

Measured baseline: [perl reading](../bench/readings/lang/perl.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Universal Ctags Perl parser | 6.2.1 | Existing tier-B L1 reading supplies no complete subroutine-end ranges, leaving range_agreement null. |
| PPI syntax tree / ppi-calls adapter | PPI 1.270, Perl 5.34.1 | The existing independent-parser call adapter is tier C and heuristic around barewords/block calls. PPI token locations are a possible end-span route, but that does not remove its tier-C excellent cap. |
| B::Xref compiler cross-reference | B::Xref 1.07 (core, Perl 5.34.1) | Compiled caller-to-callee records include lines. Its subdef line can be a glob’s first mention, not the declaration end, so it cannot repair L1 ranges. Loading subject modules needs the subject load path; this static dump is tier B and may replace the tier-C L2 call adapter. |
| Perl B::Deparse compiler dump | B::Deparse 1.57 (local system Perl) | It reconstructs source from op trees and may emit line directives, rather than original declaration closing spans. Reconstructed output is not an exact source-end oracle; no fabricated range reading is produced. |

Write the existing null-range and tier ceiling explicitly. Retry a source-preserving compiler/maintained indexer route, or a separate owner tier ruling; do not relabel a PPI parser as a compiler.

Governing row: `LB-Q-perl-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / compiler | [Perl 5.34 debugger internals](https://perldoc.perl.org/5.34.2/perldebguts) document `%DB::sub` values as `filename:startline-endline`. This is a **LIVE UNMEASURED** compiler-dump tier-B end-range route; only synthetic controls exist, not the complete subjects. |
| SCIP / LSIF | Shared catalogs and [GitHub `scip-perl` search](https://github.com/search?q=scip-perl&type=repositories) yielded no measured Perl indexer. |
| Language server | [PLS 0.906](https://fastapi.metacpan.org/v1/release/PLS) offers navigation, but exact original subroutine-end coverage is **LIVE UNMEASURED**. |
| Registry AST | [PPI registry 1.291](https://fastapi.metacpan.org/v1/release/PPI) is newer than measured 1.270, but remains an independent parser tier C; a version hit is no compiler-range proof. [B::Deparse](https://perldoc.perl.org/B::Deparse) reconstructs output rather than original ends. |
| Runtime tracing | [Devel::NYTProf](https://metacpan.org/pod/Devel%3A%3ANYTProf) observes executed calls; [perlrun -c](https://perldoc.perl.org/5.34.2/perlrun) still executes BEGIN/CHECK/use while omitting the program body. |

The measured reading and governing row above are unchanged; a registry or catalog hit alone changes no grade.
