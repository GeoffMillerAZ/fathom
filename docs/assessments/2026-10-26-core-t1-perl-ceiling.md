# Perl native ranges and remaining call-oracle limits

The [canonical reading](../bench/readings/lang/perl.json) remains unchanged.
A [separate native-range candidate](../bench/readings/perl-native-c9-2026-10-26.json)
measures L1 excellent and L2 good: range agreement 1 over 2,870 paired ends.
PPI calls remain tier C. This is a measured partial result, pending the frozen
Dancer2 validation, and makes no exhausted-tooling claim.

| Candidate or route | Pinned version | Evidence and concrete limit |
|---|---|---|
| Universal Ctags Perl parser | 6.2.1 | Supplies independent names and declaration lines, but not complete subroutine ends. The range metric retains its conservative primary tier-B cap. |
| PPI call adapter | PPI 1.270, Perl 5.34.1 | Existing independent-parser truth, tier C. Call precision .9822 and recall .9993 are unchanged; this route cannot establish L2 excellent under the tier rule. |
| Compiler debugger table | Perl 5.34.1, `%DB::sub` | Measured under C-R9: 527/543 units compile; 2,870/2,922 named subroutine ends pair uniquely. All sixteen uncompiled units remain unresolved against coverage. Native ends are tier A only for the actual compiled population. |
| Stock compiler cross-reference | B::Xref 1.07, Perl 5.34.1 | Complete-subject census records 7,747 direct-call records and 11,408 non-direct target-unknown records. No modern method target is emitted. Repeated compilation records are not unique edge counts. This incomplete census is not a call grade. |
| Named METHOP derivative | FathomPerlNamedMethodXref 0.01-probe over B::Xref 1.07 | Fixed control exposes two literal method names, two direct calls and one dynamic unknown. One anonymous-closure method and one optimized constant source call remain absent/unresolved. This is a bounded synthetic control, not a complete source-call oracle. |
| Compiler op-tree renderer | B::Concise 1.004, Perl 5.34.1 | Installed source was inspected: it can render METHOP names. Its named-stash traversal and optimized op tree do not establish complete original-source call enumeration. The method control above demonstrates the anonymous-CV and optimized-call gaps; no full-corpus Concise grade is claimed. |
| Compiler deparser | B::Deparse 1.57, Perl 5.34.1 | Reconstructs source from optimized op trees. Reconstructed text is not an exact original declaration-end or complete original-source call oracle. |
| Language server | PLS 0.906 | Navigation exists, but complete original-source range/call coverage is unmeasured. It is an independent analyzer, tier C under P-T1, so availability alone does not lift L2 excellent. |
| Newer independent parser | PPI 1.291 | Newer than the measured 1.270; remains tier C. No compiler authority follows from a registry version. |
| Runtime call profiler | Devel::NYTProf 6.16, Perl-5 license | Observes executed calls only. No complete static source-call population follows from an execution trace; ordinary subject bodies are not executed in this audit. |

## X-R4 five-source audit

Searched 2026-10-26. The source ledger records routes and misses, rather than
asserting that an unobserved tool cannot exist.

| Source class | Sources searched and result |
|---|---|
| Own compiler/runtime | [perldebguts](https://perldoc.perl.org/5.34.2/perldebguts), [perlrun](https://perldoc.perl.org/5.34.2/perlrun), installed B::Xref 1.07, B::Concise 1.004 and B::Deparse 1.57. `%DB::sub` and stock Xref are now measured under C-R9. Installed source and the committed synthetic derivative expose the limits above. Attempts to retrieve exact 5.34.1 B::Concise/Xref pages and tagged GitHub source through the web reader failed; the pinned installed core sources supplied the inspection. |
| SCIP / LSIF | [Sourcegraph navigation catalog](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP catalog](https://github.com/scip-code/scip#readme), legacy Sourcegraph LSIF repositories and [GitHub scip-perl search](https://github.com/search?q=scip-perl&type=repositories). No measured Perl producer was identified. Catalog misses are bounded search results. |
| Language servers | [PLS release metadata](https://fastapi.metacpan.org/v1/release/PLS), 0.906: navigation candidate, no measured complete native call oracle. |
| Package registry / AST dumpers | [PPI metadata](https://fastapi.metacpan.org/v1/release/PPI), 1.291, and the core B backend sources above. PPI remains independent-parser tier C; the compiler routes have the measured limits above. |
| Runtime tracing | [NYTProf metadata](https://fastapi.metacpan.org/v1/release/Devel-NYTProf), 6.16, and perlrun compile-only behavior. Tracing covers executed paths; `-c` still executes BEGIN/CHECK/use, so compile safety and coverage must be controlled. |

## C-R9 compliant canonical measurement

Both full pinned subjects compile unchanged, with network operations denied
and only core, subject-owned paths and each subject's own declared dependencies.
The loopback negative control fails EPERM. Mojo uses no borrowed dependency
prefix. Perl-Critic uses its cpanfile-backed isolated library, whose before/after
fingerprint is identical. Every failed or timed-out unit remains unresolved.
The old debugger-mode Xref census is withdrawn because `dbstate` does not update
stock Xref's source attribution; the corrected census uses non-debugger COPs.

| Subject | Range-compiled units | Unresolved | Paired named ends | Paired-end agreement | Xref-compiled units | Direct / unknown records |
|---|---:|---:|---:|---:|---:|---:|
| Mojo | 270/280 (96.43%) | 10 | 1,286/1,308 (98.32%) | 1,286/1,286 | 270/280 | 4,812 / 8,316 |
| Perl-Critic | 257/263 (97.72%) | 6 | 1,584/1,614 (98.14%) | 1,584/1,584 | 255/263 | 2,935 / 3,092 |

Pooled: 2,870/2,922 named ends (98.22%), or 2,870/3,346 all matched
definitions (85.77%). Anonymous/eval/outside-source entries and ambiguous
joins receive no invented ends. All non-range counts and numeric metrics equal
the canonical reading. Native evidence is A only for its compiled population;
the actual reading's range metric is B under the independent Ctags tier cap.
Median-of-three throughput is 2.43 MB/CPU-s (2.45, 2.43, 2.42), zero faults.

The [committed collector receipt](data/2026-10-26-core-t1-perl-native-c9.json)
was generated by `calibration/oracles/perl-db-ranges/receipt.py`. It validates
all phase identities, exact timing labels, source/count accounting, EPERM and
dependency fingerprints, and publishes counts and hashes only. The native compiler instrument commit is `2fefe5d424`; the harness-only
repeat uses `c234acd6c2`. Their full Git object IDs are retained in the
receipt, with the grading binary hash on every phase.
The [synthetic method-control result](data/2026-10-26-core-t1-perl-method-control.json)
is explicitly a derivative, not stock Xref output. B::Concise's installed
module SHA256 is `c86db93c6a0aa92d36aea7d01089d9e875303cbf1d85ec727befff38148696c8`;
B::Deparse's is `e6c84651dbafdce6793b73bbbff67a65a06c55cb9e8375891d8ec3838d07f371`.

## Disposition and next trigger

The prior absent-native-oracle premise is withdrawn. The native range route
is measured and passes its conditional range comparison; fresh independent
validation on the preregistered, still-unread Dancer2 remains required before
canonical adoption. The remaining L2 ceiling applies to the current PPI oracle.
A future native call adapter must enumerate anonymous CVs and account for
compiler-elided source calls, retain dynamic unknowns, and grade the full
compiled population with every failed unit unresolved. The committed method
control exposes one unresolved closure and one unresolved constant call today.
A literal-name renderer alone does not satisfy that trigger.

Verification commands and input requirements are in
[the committed instrument README](../../calibration/oracles/perl-db-ranges/README.md).
The receipt self-test rejects source/unit/fingerprint/network contradictions,
duplicate timing labels, grade-commit drift and accidental private-array
publication. Disabling the literal-name callback fails the fixed native method
control; restoring it passes. The output-cap control refuses limit plus one.
Governing row: `LB-Q-perl-rest`. No extractor, corpus shard, canonical reading,
epoch or generated board changes.
