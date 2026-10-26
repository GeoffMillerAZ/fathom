# perl named ceiling routes (X-R1)

The committed reading is unchanged: level L2, grades L1 good, L2 good, L3 not-attempted, L4 not-attempted.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Universal Ctags Perl parser | 6.2.1 | Existing tier-B L1 reading supplies no complete subroutine-end ranges, leaving range_agreement null. |
| PPI syntax tree / ppi-calls adapter | PPI 1.270, Perl 5.34.1 | The existing independent-parser call adapter is tier C and heuristic around barewords/block calls. PPI token locations are a possible end-span route, but that does not remove its tier-C excellent cap. |
| Perl B::Deparse compiler dump | B::Deparse 1.57 (local system Perl) | It reconstructs source from op trees and may emit line directives, rather than original declaration closing spans. Reconstructed output is not an exact source-end oracle; no fabricated range reading is produced. |

Write the existing null-range and tier ceiling explicitly. Retry a source-preserving compiler/maintained indexer route, or a separate owner tier ruling; do not relabel a PPI parser as a compiler.

Governing row: `LB-Q-perl-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.
