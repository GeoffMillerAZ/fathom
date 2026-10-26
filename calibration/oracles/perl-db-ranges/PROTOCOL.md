# Perl compiler end-range audit

Preregistered before canonical runs. This audits a live candidate discovered by
X-R4; it does not assert that other Perl tooling has been exhausted.

- Compiler: Perl 5.34.1, executable checksum recorded in scratch metadata.
  `%DB::sub` is the compiler-populated debugger table documented by
  [perldebguts](https://perldoc.perl.org/5.34.2/perldebguts).
  `PERL5DB` with `-d -c` reads it at CHECK. Ordinary program bodies do not run,
  but BEGIN, UNITCHECK, CHECK and `use` can run. Both subjects are pinned public
  calibration code, compiled unchanged. The initial no-install trial precedes the isolated declared-dependency follow-up below.
- Name identity: Universal Ctags 6.2.1 supplies source-written subroutine names.
  A native record joins only one tag in the same file within two lines whose
  name is exactly the compiler key or its final package component. The match
  must also be unique in reverse. Ambiguous or missing matches abstain and
  are counted. End lines remain the compiler's, without source scanning.
  The compiler's start can be the body brace, so it is retained separately;
  the neutral companion uses the independently tagged declaration line.
- Full canonical subjects, unchanged: mojo v9.50 at
  `7b99f4adce82a45fd0965ef4dd56550b4f5ee89b` (Artistic-2.0) and Perl-Critic
  v1.154 at `382d701fd1129822c98fdcdb08403a733a24dc08`
  (Artistic-1.0-Perl OR GPL-1.0-or-later), as in `corpus.d/perl.yaml`.
  No `-only`, source rewrite, daemon, or corpus-shard edit. Dependency setup is isolated and limited to the subject declaration as specified below.
- Freeze adapter and synthetic controls before each full run. The compiler
  range companion sits beside the unchanged PPI 1.270 primary call oracle.
  Ctags remains L1 truth; PPI remains tier-C L2 truth. Under C-R9 the compiler companion is tier A only for the actual compiled population and contributes range counts only. Record all file counts, compilation failures,
  native/tag/join counts and range coverage. Unobserved definitions do not
  become successes. Packages, anonymous subs, forwards and unavailable compile
  dependencies remain disclosed limits.
- Run one heavy phase at a time under nice 10, launching only at one-minute
  load <=9. Compiler per-file limit 2 seconds, total 600 seconds including
  load waits; TERM then KILL on timeout. Do not silently drop timed-out files.
  Verify source Git identity and tracked source hashes before and after.
- Compare every original non-range count with the committed Perl reading.
  A difference must be explained before adoption. Use the existing thresholds:
  L1 range agreement >=.90 for excellent, plus all other gates. A numerical
  conditional range result does not establish complete compiler coverage.
  Retain L2's tier-C ceiling. Retiming, if needed, uses three unchanged runs.
- Holdout, frozen before first read and excluded from the canonical shard:
  PerlDancer/Dancer2 v2.2.2, commit
  `90b6e651c1f1a7c576072fdd039ef15afecb67a5`, Perl-5 terms
  (Artistic-1.0-Perl OR GPL-1.0-or-later), checked in the pinned
  [LICENSE](https://github.com/PerlDancer/Dancer2/blob/90b6e651c1f1a7c576072fdd039ef15afecb67a5/LICENSE)
  and `dist.ini`. It is unspent until the canonical adapter is frozen, then
  runs once without tuning. The earlier allowance that a failed or incomplete
  audit could remain unspent is **SUPERSEDED for all future holdouts**: first
  source read spends the subject, regardless of phase outcome.
- Publish only counts, hashes and repo-relative summaries. Raw source manifests,
  native keys, compiler diagnostics and per-file failures stay in scratch.
  Commit a dated assessment and actual measured reading separately from the
  canonical reading if coverage or oracle validity remains unresolved.

## Instrument erratum before holdout

The first canonical attempt completed mojo but refused Perl-Critic before compiling
it: its tracked in-tree directory alias tripped an over-broad symlink check. The
source manifest is a list of tracked regular Perl files, not a directory traversal.
The corrected collector permits an existing in-tree directory symlink without
following it, because the real target's tracked source files are already listed.
Source-file, outside-root and broken symlinks still reject. Added synthetic controls
cover each case. Re-freeze and rerun both complete canonical subjects; do not use
the first partial sequence as the final measurement. Dancer2 remains unspent.

## Preregistered isolated-dependency follow-up

The no-install canonical trial produced 1,329 ends paired within one line, but only 39.72%
of 3,346 matched definitions. Mojo had 10/280 compilation failures; Perl-Critic
had 251/263. It is a conditional candidate reading, never a canonical adoption.

Before claiming a compiler ceiling, try the subject's declared CPAN dependencies
in an isolated local library. Copy only its cpanfile into scratch, not back into
the subject. App::cpanminus 1.7048, Perl-5 license, archive SHA256
`59b60907ab9fa4f72ca2004fbe6054911439ae9a906890b4d842a87b25f20f3c`,
from `https://cpan.metacpan.org/authors/id/M/MI/MIYAGAWA/App-cpanminus-1.7048.tar.gz`,
runs with its own cache root, `--local-lib-contained`, HTTPS mirror-only,
`--installdeps --notest --no-lwp --no-wget --curl`, and retained distribution archives. Bound the
whole install to 600 seconds, nice10 after load<=9, TERM/KILL on timeout. Never
install globally, edit the subject, or apply dependency patches. Fingerprint the
local library before/after compiler runs and publish archive names/checksums.

If installation succeeds, rerun each full canonical compiler companion with only that subject's own declared dependencies. Perl-Critic uses its isolated cpanfile-backed prefix; Mojo cannot borrow that prefix. Leave the PPI 1.270 primary call oracle unchanged.
Record all remaining failures and coverage. If installation fails, record the
concrete failure and retain the partial no-install reading separately. No
canonical adoption from a conditional 1.0 range score. The Dancer2 holdout remains
unspent until this instrument/environment is frozen; no tuning on a spent subject.

Installer command erratum before any dependency-backed compiler run: cpanminus
1.7048 includes declared test requirements with `--installdeps`; it has no
`--with-test` flag. The initial setup attempt rejected that flag and its LWP
transport produced no index. Use the explicit cURL transport above. No compiler
run, acceptance rule, source pin, install cap, or holdout decision changed.

## C-R9 environment freeze before compliant measurements

C-R9 controller ruling as recorded in the project status history:
> Perl’s %DB::sub and B::Xref from the pinned interpreter are tier A for the population that interpreter actually compiles. Conditions: run it with no network and the subject’s own declared dependencies only (compilation executes BEGIN and use); report every file it cannot compile as unresolved, counted against coverage and never silently dropped.

C-R9 assigns `%DB::sub` and B::Xref tier A only for the population the pinned
interpreter actually compiles. Earlier range trials lacked verified network
denial and remain diagnostic. Rerun both complete canonical subjects after
freezing the controls below. Dancer2 remains unspent and outside the shard.

Every compile runs under macOS sandbox-exec with network operations denied.
A loopback TCP negative control must fail EPERM. Clear ambient Perl injection
variables, reset @INC to Config core roots, subject lib and the sole locked
local prefix, and reject loaded modules outside those roots. The per-compile
filesystem read allowlist also blocks explicit external `do` or `require`
that would escape %INC accounting. Subject-local setup such as t/lib remains
allowed. Record every failed compile as unresolved against the full source
population, including failures due to the restrictions.

Local dependencies require a prepared lock matching the subject, its tracked
declaration hash, and the independently measured library fingerprint. The
installation recipe is caller-attested as cpanminus --installdeps over the
unchanged declaration, not inferred by the runner. Hash the actual library
before and after, reject all library symlinks and any mismatch, retain saved
archive hashes, and disclose the setup. Only Perl-Critic's own cpanfile was
installed; Mojo uses core and its own source. No install occurs during compile.

The planned B::Xref census uses the same source and dependency restrictions,
includes all source units and failed files, and retains private raw dumps. Its
stock 1.07 synthetic control exposes a limit: direct sub calls have target
names, but modern named-method operations may appear as receiver/unknown
records without a method target. Count these gaps; never relabel them as
attested calls or promote the existing tier-C call reading from a partial
census. The primary PPI call oracle and acceptance thresholds remain unchanged.

## C-R9 Xref attribution and load-budget erratum

The first stock Xref census used `-d`. That result is withdrawn as invalid,
not gradeable: debugger COPs use `dbstate`, while B::Xref 1.07 only handles
`nextstate` when refreshing source file and line. This leaves stale locations
and can misattribute records. The corrected Xref control and census load
`FathomPerlCompileGuard.pm` explicitly, then run `-MO=Xref,-r -c` without
`-d` or a subject `-I` override. The guard establishes approved `@INC` and
checks loaded module paths at CHECK. The native control must verify direct
calls at fixture lines 6 and 7 and unknown-target method calls at lines 9 and
10. The withdrawn census supplies no oracle evidence and receives no grade.

The first compliant Mojo attempt had 30 unresolved files: 10 compile failures,
19 load-wait skips, and 1 compile timeout. Load wait is machine scheduling time,
not compiler runtime and not evidence of a language ceiling. Load waits count
against the 600-second total deadline; each 2-second compile budget starts only
after load falls to 9 or below. Every source remains in the denominator.

## Canonical result and bounded method control before holdout

The compliant repeat is complete: 527/543 units compiled for range evidence;
2,870/2,922 named subroutine ends paired uniquely, all agreeing with Fathom.
The candidate reading remains L2 good with PPI calls at tier C; L1 becomes
excellent. Uncompiled units remain unresolved and counted. Stock Xref's
7,747 direct records and 11,408 non-direct unknown records are a diagnostic
census, not a grade. Earlier debugger-mode Xref records are invalid.

Before the independent validation, a fixed synthetic METHOP adapter control
checks literal method-name availability. It exposes two literal names but
leaves an anonymous-closure call and an optimized constant call unresolved.
It changes no canonical source, PPI call rule, range join or acceptance bar.
The companion remains range-only; no call-oracle promotion is preregistered.
The committed count/hash collector projects the full repeat without subject
file lists or people. Dancer2 remains unspent until this instrument is frozen.

## Dancer2 one-shot disposition and next-holdout amendment

The frozen `500a867de528dfe651b3ec5b61d1779625c6e595` instrument spent
Dancer2 at its first clone. The complete one-shot run compiled 279/343 units,
paired 624/676 named ends, and the reading found 623 eligible ends, all agreeing
within one line. Median-of-three throughput was 2.29 MB/CPU-s. No minimum
compiler coverage or named-pairing coverage had been frozen before first read.
The result is diagnostic only: it cannot authorize canonical adoption, and
Dancer2 cannot be reused as a fresh holdout. The earlier conditional sentence
about an incomplete audit leaving it unspent does not apply after first read.

Before selecting or first reading any further holdout, commit a generic
count/hash collector and these independent population gates: at least 95% of
all tracked Perl units compile, and at least 95% of independently tagged named
ends pair uniquely. The harness must measure the full native population; a count mismatch fails
acceptance and remains published. Any Ctags-unknown file fails the named-pairing population
gate; a failed gate still gets a count/hash receipt. Every future phase record
must bind the full instrument and source commits, and build/grade/retime phases
must bind the grading binary SHA256. Dancer2's old phase records contain only
labels, exit states and launch loads, so their provenance is incomplete and
the diagnostic receipt says so. Require the subject's own declared dependencies only,
with an unchanged isolated library fingerprint and saved archive hashes; an
undeclared setup gives no borrowed dependencies. Every failed unit and every
unpaired tag remains in the denominator. Preserve the existing >=.90 range
agreement and all L1, timing and fault gates. No further holdout is pinned by
this amendment.

A nonzero or timed-out compiler or grade phase is a published spent failure,
including when the reading or retime artifact does not exist. The receipt
reports phase exit, timeout and launch load; unknown counts stay unknown.
Only a load-invalid retime may be repeated, and only over the same frozen
source, oracle artifacts, grading binary and rule, with no new source read or
compiler/grade run. Its failed phase remains in the private audit trail.
