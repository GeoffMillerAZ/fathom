# Perl compiler range candidate

This is an **unmeasured, compiler tier-B** range companion for named Perl
subroutines. Perl 5.34.1 documents `%DB::sub` values as
`filename:startline-endline` in [perldebguts](https://perldoc.perl.org/5.34.2/perldebguts).
The [5.34.2 perlrun contract](https://perldoc.perl.org/5.34.2/perlrun)
states that `PERL5DB` supplies debugger setup for bare `-d`, and `-c` still
executes `BEGIN`, `UNITCHECK`, `CHECK`, and `use`. This is **not safe for
untrusted subjects**. The subject body is not run. The tiny debugger observer
reads compiler-populated data at `CHECK`; it does not parse source or call
subject subroutines. Universal Ctags 6.2.1 independently supplies the
source-written subroutine name and declaration line. A native entry joins
only to a tag in the same file within two lines whose name matches the full
compiler key or its terminal component, and only when the match is unique in
both directions. The end line remains the native compiler value; unmatched
or ambiguous entries are withheld and counted in metadata.

Run with a clean, pinned full-commit Git checkout, a separately pinned Perl
5.34.1 and Universal Ctags 6.2.1 executables, and fresh output paths outside
the checkout:

```sh
python3 calibration/oracles/perl-db-ranges/self_test.py --perl /path/to/perl-5.34.1 --ctags /path/to/universal-ctags-6.2.1
python3 calibration/oracles/perl-db-ranges/runner.py \
  --perl /path/to/perl-5.34.1 --ctags /path/to/universal-ctags-6.2.1 \
  --subject subject=/path/to/clean-pinned-checkout \
  --out calibration/oracles/perl-db-ranges/subject.ranges.json \
  --log-dir calibration/oracles/perl-db-ranges/subject-raw
```

The manifest scans tracked, regular files with PPI call-oracle extensions
`.pm`, `.pl`, `.t`, `.psgi`, `.plx`, `.perl`, plus extensionless Perl shebang
scripts. Ignored/untracked files are excluded. A tracked directory symlink is
allowed only when it resolves to an existing directory inside the checkout;
the manifest does not traverse it. Tracked source-file, broken, and
outside-checkout symlinks are rejected.
It hashes every listed source file and checks Git HEAD/cleanliness, scope, and bytes before
and after. Each file runs `perl -d -c -I<subject>/lib` from the checkout root,
with `nice -n 10`, a 1-minute load ceiling of 9, a 2-second file timeout,
and a 10-minute total limit. Timed-out process groups receive TERM then KILL.
Every file stays in `units`; failures count in `errored`, with per-file reasons
in the sibling `.meta.json` artifact. That metadata also holds the manifest,
verified ctags version and binary SHA-256, compiler keys/native starts/ends,
and joined/unmatched/ambiguous counts;
the `.ranges.json` file contains only neutral range-companion fields.
Only compiler entries whose filename maps to the current source file and whose
range has positive ordered lines can enter `ranges`. Anonymous/eval and imported
outside-source entries are excluded. The range companion emits ctags's
source-written name and declaration line, paired to the compiler end. No Fathom
output is read. Full-corpus completeness, errors, safety, and range quality
remain to be measured before using it in a reading.

The runnable verification command uses `PERL_BINARY` and `CTAGS_BINARY` set to
available executables reporting exactly Perl 5.34.1 and Universal Ctags 6.2.1:

```sh
: "${PERL_BINARY:?set to the pinned Perl executable}"
: "${CTAGS_BINARY:?set to the pinned Universal Ctags executable}"
python3 calibration/oracles/perl-db-ranges/self_test.py --perl "$PERL_BINARY" --ctags "$CTAGS_BINARY"
```
