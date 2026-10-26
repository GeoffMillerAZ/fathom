# perl named ceiling routes (X-R1)

Measured baseline: [perl reading](../bench/readings/lang/perl.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Universal Ctags Perl parser | 6.2.1 | Existing tier-B L1 reading supplies no complete subroutine-end ranges, leaving range_agreement null. |
| PPI syntax tree / ppi-calls adapter | PPI 1.270, Perl 5.34.1 | The existing independent-parser call adapter is tier C and heuristic around barewords/block calls. PPI token locations are a possible end-span route, but that does not remove its tier-C excellent cap. |
| B::Xref compiler cross-reference and `%DB::sub` | B::Xref 1.07, Perl 5.34.1 | `%DB::sub` records `file:start-end` when compile bit `0x10` is enabled; paired with B::Xref caller-to-callee records, Perl's compiler supplies the oracle for definition ends and calls. Under public ruling C-R9, tier A applies only to the actual compiled population: no network access, subject-declared dependencies only, and every file that cannot compile remains unresolved against coverage. |
| Perl B::Deparse compiler dump | B::Deparse 1.57 (local system Perl) | It reconstructs source from op trees and may emit line directives, rather than original declaration closing spans. Reconstructed output is not an exact source-end oracle; no fabricated range reading is produced. |

The `LB-Q-perl-rest` oracle route is `%DB::sub` plus B::Xref under public ruling C-R9: tier A covers only the actual compiled population, with no network access and subject-declared dependencies only; every file that cannot compile remains unresolved against coverage. Isolated CPAN dependency setup succeeded in 28.574s. The pre-ruling trial compared 1,329 paired definition ends and reported 39.72% all matched, with Mojo 10/280 and Perl::Critic 251/263 compile failures. Because no-network compliance was not verified for that trial, these are conditional diagnostic results, not a C-R9 grade. Rerun the pinned corpus without network access using only subject-declared dependencies, then report paired-end agreement and compiled/unresolved population counts with fresh independent validation. No exhausted-ceiling claim is supported.

Governing row: `LB-Q-perl-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / compiler | [Perl 5.34 debugger internals](https://perldoc.perl.org/5.34.2/perldebguts) document `%DB::sub` values as `filename:startline-endline`; paired with [B::Xref](https://perldoc.perl.org/5.34.2/B%3A%3AXref), this is the `LB-Q-perl-rest` oracle route under public ruling C-R9. Tier A applies only to the actual compiled population, with no network access, subject-declared dependencies only, and every uncompiled file unresolved against coverage. **LIVE UNMEASURED** pending a C-R9-compliant pinned-corpus rerun and fresh independent validation. The pre-ruling trial compared 1,329 paired definition ends and reported 39.72% all matched, with Mojo 10/280 and Perl::Critic 251/263 compile failures; no-network compliance was not verified, so these remain conditional diagnostics, not a grade. Isolated dependency setup succeeded in 28.574s. Next measurement: rerun without network access using only subject-declared dependencies, then report paired-end agreement and compiled/unresolved population counts. No exhausted-ceiling claim. |
| SCIP / LSIF | Shared catalogs and [GitHub `scip-perl` search](https://github.com/search?q=scip-perl&type=repositories) yielded no measured Perl indexer. |
| Language server | [PLS 0.906](https://fastapi.metacpan.org/v1/release/PLS), [Perl Navigator v0.8.20](https://github.com/bscan/PerlNavigator/tree/v0.8.20), and [Perl::LanguageServer 2.6.2](https://metacpan.org/release/Perl-LanguageServer) offer navigation; exact original subroutine-end coverage is **LIVE UNMEASURED**. |
| Registry AST / parser | [Perl::Tidy 20260826](https://metacpan.org/release/Perl-Tidy) `--dump-block-summary` (`-dbs`) reports each sub's opening-brace line and line count through its closing brace, yielding end lines directly. This independent parser is tier C and **LIVE UNMEASURED**, a cross-check for the compiler route. [PPI 1.291](https://fastapi.metacpan.org/v1/release/PPI) is newer than measured 1.270 but remains tier C; [B::Deparse](https://perldoc.perl.org/B::Deparse) reconstructs output rather than original ends. |
| Runtime tracing | [Devel::NYTProf](https://metacpan.org/pod/Devel%3A%3ANYTProf) observes executed calls; [perlrun -c](https://perldoc.perl.org/5.34.2/perlrun) still executes BEGIN/CHECK/use while omitting the program body. |

The prior measured reading remains unchanged pending the C-R9-compliant pinned-corpus rerun; no C-R9 grade or exhausted ceiling is claimed.
