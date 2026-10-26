# Perl compiler end-range audit

Preregistered before canonical runs. This audits a live candidate discovered by
X-R4; it does not assert that other Perl tooling has been exhausted.

- Compiler: Perl 5.34.1, executable checksum recorded in scratch metadata.
  `%DB::sub` is the compiler-populated debugger table documented by
  [perldebguts](https://perldoc.perl.org/5.34.2/perldebguts).
  `PERL5DB` with `-d -c` reads it at CHECK. Ordinary program bodies do not run,
  but BEGIN, UNITCHECK, CHECK and `use` can run. Both subjects are pinned public
  calibration code, compiled without editing or installing their dependencies.
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
  No `-only`, source rewrite, dependency install, daemon, or corpus-shard edit.
- Freeze adapter and synthetic controls before each full run. The compiler
  range companion sits beside the unchanged PPI 1.270 primary call oracle.
  Ctags remains L1 truth; PPI remains tier-C L2 truth. This tier-B companion
  contributes range counts only. Record all file counts, compilation failures,
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
  runs once without tuning. A failed or incomplete audit can leave it unspent.
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

The no-install canonical trial produced 1,329 exact paired ends, but only 39.72%
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

If installation succeeds, rerun both full canonical compiler companions under
that explicit PERL5LIB, leaving the PPI 1.270 primary call oracle unchanged.
Record all remaining failures and coverage. If installation fails, record the
concrete failure and retain the partial no-install reading separately. No
canonical adoption from a conditional 1.0 range score. The Dancer2 holdout remains
unspent until this instrument/environment is frozen; no tuning on a spent subject.

Installer command erratum before any dependency-backed compiler run: cpanminus
1.7048 includes declared test requirements with `--installdeps`; it has no
`--with-test` flag. The initial setup attempt rejected that flag and its LWP
transport produced no index. Use the explicit cURL transport above. No compiler
run, acceptance rule, source pin, install cap, or holdout decision changed.
