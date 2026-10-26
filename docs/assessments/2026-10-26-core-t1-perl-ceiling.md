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
