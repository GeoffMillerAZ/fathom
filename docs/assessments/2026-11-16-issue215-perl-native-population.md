# Issue 215: Perl native population remains incomplete

Perl remains canonical **L2 good** (L1 good, L2 good), with the existing
unmeasured range metric and tier-C calls unchanged. The separate C-R9 native
range reading remains L1 excellent/L2 good: 527/543 units compiled,
2,870/2,922 named ends paired, agreement 1 on paired ends. These are historical
counts, not a new corpus run. No Dancer2 source was read or reused.

## Compiler control and disposition

The authored [control](../../calibration/oracles/perl-db-ranges/compiler_population_control.pl)
uses Perl 5.34.1 / B::Concise 1.004, compile-only under a network-denying sandbox.
Native CV pad introspection exposes **1 anonymous CV** and its `target` call.
The named `native_constant()` call instead becomes `const[IV 7]` with a FOLD
flag and no named target. A literal `7` becomes the same value without that
flag. The flag discloses folding, not the original callee. Setting the
[documented debugger optimization bit](https://perldoc.perl.org/5.34.1/perlvar#%24%5EP)
(`PERL5DB='BEGIN { $^P = 0x14; }'`, `-d`) retains this loss. It does not
rescue the call target. Ordinary control bodies never execute.

The named target assertion was run against both outputs. Both are RED at
`test_native_population_retains_anonymous_and_constant_calls`: anonymous
enumeration passes; `gv[*CompilerPopulationControl::native_constant]` is absent.
This is a measured limitation of the post-optimization route, **not** an
exhausted-tooling ceiling or a completed native adapter. No complete compiled
population, native call grade, or support upgrade is claimed.

Historical probe commands (the test invocations below produced RED before the
correction described at the end). Run from the repository root, after sourcing
the lane environment and under the common heavy wrapper (the wrapper acquires both locks, no nested lease):

```sh
LC_ALL=C /usr/bin/sandbox-exec -p '(version 1) (allow default) (deny network*)' \
  /usr/bin/perl -c calibration/oracles/perl-db-ranges/compiler_population_control.pl \
  > workspace/artifacts/lang-perl/population-normal.log 2>&1
LC_ALL=C PERL5DB='BEGIN { $^P = 0x14; }' \
  /usr/bin/sandbox-exec -p '(version 1) (allow default) (deny network*)' \
  /usr/bin/perl -d -c calibration/oracles/perl-db-ranges/compiler_population_control.pl \
  > workspace/artifacts/lang-perl/population-noopt.log 2>&1
PERL_POPULATION_TRACE=workspace/artifacts/lang-perl/population-normal.log \
  python3 calibration/oracles/perl-db-ranges/test_compiler_population.py
PERL_POPULATION_TRACE=workspace/artifacts/lang-perl/population-noopt.log \
  python3 calibration/oracles/perl-db-ranges/test_compiler_population.py
```

## Fresh holdout and missing work

The `Next holdout pin` in the existing main-branch
[protocol](../../calibration/oracles/perl-db-ranges/PROTOCOL.md) was committed
before any subject source read. Path-Tiny `release-0.148` was selected through
`git ls-remote --tags https://github.com/dagolden/Path-Tiny.git` and
[release metadata](https://fastapi.metacpan.org/v1/release/DAGOLDEN/Path-Tiny-0.148)
(Apache-2.0). It remains **unspent**: no clone, first read, dependencies,
compiler, or harness run. The protocol's existing location/JSON format was
checked on main first (T11); the commit hook passed without bypass.

Retry requires a compiler-derived original-call record before folding loses
its target, together with recursive anonymous-CV enumeration; then the full
native population must be graded through the existing runner and collector.
At least 95% compiled units, at least 95% paired named tags, zero Ctags-unknown
files, equal native/harness populations, unresolved units and all existing
range/call/tenancy bars remain mandatory. A stronger control must retain the
constant's original target, not substitute its numeric value or a parser's
label. The current synthetic control neither spends nor validates the holdout.

Integration request: supply/review the native pre-optimization call producer
for the existing neutral call oracle (`path`, `from`, `to`) and preserve its
unresolved call/unit census alongside the collector. No shared engine,
resolver, root dependency, registration, daemon or door package was changed.
This is HOLD for native compiled-population grading, not READY for excellent.

## Five-source receipt (2026-11-16 UTC)

All requests below returned HTTP 200, except the explicitly noted license
lookup. Raw responses and count/hash receipt remain in ignored
`workspace/artifacts/lang-perl/search/`; no subject contents or file lists are
published. The earlier named-candidate audit remains applicable in
[the C-R9 assessment](2026-10-26-core-t1-perl-ceiling.md).

| Source | Query and result |
|---|---|
| Own compiler | Installed B::Xref 1.07 / B::Concise 1.004, Perl 5.34.1 (Perl-5 license), and [versioned Concise docs](https://perldoc.perl.org/5.34.1/B::Concise). Anonymous CV control above succeeds; original optimized call target is missing. No complete-corpus grade. |
| SCIP / LSIF | [SCIP catalog](https://github.com/scip-code/scip), [navigation catalog](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), `https://api.github.com/search/repositories?q=scip-perl` (2 hits). `jelmer/scip-perl` is a real candidate; the other hit is unrelated. Its README describes Rust `perl-parser`, heuristic receiver inference and skipped parse failures, rather than a Perl-native compiler census. At the inspected HEAD, repository license metadata was null and `/license` returned 404: license unavailable, not globally absent. No source code was copied or installed and no grade was measured. A SCIP filename alone does not prove complete compiler coverage. |
| Language servers | [PLS metadata](https://fastapi.metacpan.org/v1/release/PLS): 0.906, Perl-5 license. Navigation/independent analyzer, not a measured complete native compiler population; tier C under P-T1. Perl Navigator 0.8.20 and Perl::LanguageServer 2.6.2 remain the named, unmeasured candidates in the linked prior audit. |
| Package registry | [PPI metadata](https://fastapi.metacpan.org/v1/release/PPI): 1.291, Perl-5 license. Independent parser; using it to reconstruct compiler-elided calls would not turn those calls into native truth. Perl::Tidy 20260826 block summaries remain an unmeasured independent range cross-check, not native call truth. |
| Runtime tracing | [NYTProf metadata](https://fastapi.metacpan.org/v1/release/Devel-NYTProf): 6.16, Perl-5 license. Executed-call route, not complete compile-only original-call enumeration; untried on the holdout. |

Mutation: no grading, tier, pairing or scope logic changed in this item;
only a diagnostic compiler control and its explicit target assertion were added.
Caps: no new population reader or expanded oracle format; existing bounded
runner/collector unchanged. Tracked-set manifests and reading round-trips are
not new evidence here: no subject or Reading JSON was read or changed.
Controller owns the fresh critic and any post-critic corpus cleanup.

## First correction: passing known-limitation control

The historical RED above remains evidence of the missing capability. Before
editing the test, both current native probes again exited 0 and the original
unittest again failed at the exact missing constant target. Receipts, including
the original prototype outputs, remain under `workspace/artifacts/lang-perl/`;
the fresh correction receipts are under `fix1/` there.

The committed test is now `test_native_population_known_constant_target_loss`.
It launches the real compile-only probe in both normal and debugger modes;
`PERL_POPULATION_TRACE` is no longer an input. Each mode must exit 0, report
syntax OK, enumerate exactly one anonymous CV, retain its ordinary `target`
call, emit the constant/literal/anonymous sections and fold the constant to 7.
Only then does it require the exact
`gv[*CompilerPopulationControl::native_constant]` target to be absent in the
constant-caller section. Recovery fails with a report-revisit message. Compile
failure, missing CVs or broken output fail independently; no skip or expected
failure hides them. This is a bounded producer control, not a governed corpus
reading under T13. Native producer HOLD and every full compiled-population bar
above remain unproved; the holdout remains UNSPENT.

After sourcing the lane environment, run under the common heavy wrapper:

```sh
python3 calibration/oracles/perl-db-ranges/test_compiler_population.py -v
```

Mutation: the test consumer was fed three perturbations of successful real native
results: insert the exact constant target before the literal section, remove the
CV=1 record, and change process status to failure. Each produced two focused
assertion failures (normal/noopt), zero errors/skips. Restoring the original
consumer produced `OK` in both modes. Full logs: `fix1/mutation.log` and
`fix1/restored-green.log`. Reproduce the mutations under the same wrapper with:

```sh
python3 - <<'PYCONTROL'
from pathlib import Path
import unittest
p = Path('calibration/oracles/perl-db-ranges/test_compiler_population.py')
source = p.read_text()
mutants = [
    "raw = raw.replace('CONTROL=literal_caller\\n', 'gv[*CompilerPopulationControl::native_constant]\\nCONTROL=literal_caller\\n')",
    "raw = raw.replace('ANONYMOUS_CVS=1\\n', 'ANONYMOUS_CVS=0\\n')",
    'probe.returncode = 1',
]
for mutant in mutants:
    namespace = dict(__file__=str(p), __name__='mutation_control')
    changed = source.replace('raw = probe.stdout', 'raw = probe.stdout; ' + mutant)
    exec(compile(changed, str(p), 'exec'), namespace)
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(namespace['CompilerPopulation'])
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    if len(result.failures) != 2 or result.errors or result.skipped:
        raise SystemExit('expected assertion failures in both modes')
PYCONTROL
```

Caps: unchanged; no new population reader. No Go source, Reading JSON, corpus,
pin, grade or plan row changed. Verification passed (all exit 0), under the
common heavy wrapper:

```sh
bash scripts/fathom-go.sh test -race -count=1 -run '^TestNoOrgSpecificNames$' ./cmd/fathom
go run ./third_party/agentic-kit/cmd/release-notes -check
bash scripts/check-merge-debris.sh
```

No full gate or support claim follows from this control. Fresh critic review
remains controller-owned.
