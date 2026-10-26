# Perl compiler range candidate

This is an **unmeasured, compiler tier-A** range companion for the actual
population that compiles. Under C-R9, files that cannot compile remain
unresolved against coverage and stay in `units`; they are never dropped.
Perl 5.34.1 documents `%DB::sub` values as
`filename:startline-endline` in [perldebguts](https://perldoc.perl.org/5.34.2/perldebguts).
The [5.34.2 perlrun contract](https://perldoc.perl.org/5.34.2/perlrun)
states that `PERL5DB` supplies debugger setup for bare `-d`, and `-c` still
executes `BEGIN`, `UNITCHECK`, `CHECK`, and `use`. This is **not safe for
untrusted subjects**. All native Perl invocations use the fixed macOS sandbox
profile `(version 1) (allow default) (deny network*)`; execution fails closed
if `/usr/bin/sandbox-exec` is unavailable. A preflight loopback TCP control to
`127.0.0.1` must fail with `EPERM`, so connection-refused is not accepted as
proof of network denial. The subject body is not run. The tiny debugger observer
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
  --out workspace/tmp/perl-subject.ranges.json \
  --log-dir workspace/tmp/perl-subject-raw
```

For a local dependency prefix, pass `--perl-lib PREFIX/lib/perl5`, a prepared
`--dependency-lock LOCK.json`, and `--declared-dependencies cpanfile` (or the
subject-relative tracked declaration file). The lock format is:

```json
{
  "format": "perl-dependency-lock-v1",
  "subject": "subject-name",
  "declared_dependencies": {"path": "cpanfile", "sha256": "<file hash>"},
  "local_lib_perl5": {"files": 0, "bytes": 0, "sha256": "<tree hash>"}
}
```

Prepare `local_lib_perl5` with `dependencies.library_fingerprint()` after the
declared dependency recipe has been installed into that isolated prefix. This
is a caller-attested setup recipe; the runner never infers declared dependency
status from a path. It hashes the declaration and library before and after the
run and rejects a mismatched lock or changed library. Dependency metadata
contains hashes and subject-relative declaration data; compiler sandbox
metadata records the generated read allowlist. With no local prefix, non-core
dependencies remain unresolved.

`xref.py` is a separate diagnostic census for stock B::Xref 1.07. It applies
the same source snapshot, sandbox, dependency lock, timeouts, and include-path
controls. Its preflight records the exact core-relative B::Xref module path and
hash; compiler `-MO` loads with the same core roots first. Raw stdout and
stderr stay in the required private log directory. It counts direct calls,
explicit `meth` events, and target-unknown non-direct `subused` rows separately.
The result is diagnostic only and never receives a neutral grade.


The runner clears inherited `PERL5LIB`, `PERLLIB`, and `PERL5OPT` and never
puts the local dependency prefix in the process environment. Before each
subject compile, the debugger observer resets `@INC` to the pinned Perl
`Config` `privlibexp` and `archlibexp` core roots, subject `lib`, and the
explicitly locked local `lib/perl5` plus its `Config` `archname` directory.
Subject `use lib` directives may add subject paths such as `t/lib`. At `CHECK`,
every loaded module path must resolve under the core roots, the subject checkout,
or the locked local library. Other site/vendor paths fail the compile and leave
the file unresolved against coverage. All compiles still run in the fixed
network-denying sandbox. The per-compile profile also denies `file-read-data`
outside an allowlist containing the exact Perl executable, observer, and
`nice`, Perl Config core directories, the subject checkout, the locked local
library, `/` as a literal path for dyld cache discovery, Darwin loader
directories (`/usr/lib`, `/System/Library/dyld`, the two OS Cryptex dyld
directories, and `/System/Library/Frameworks`), and `/dev/null` and random
devices. Locale variables are fixed to `C` so Perl startup does not probe
host-specific locale files. This blocks transient `do` and `require` reads
even if subject code later deletes an `%INC` entry. File metadata access
remains available.

The manifest scans tracked, regular files with PPI call-oracle extensions
`.pm`, `.pl`, `.t`, `.psgi`, `.plx`, `.perl`, plus extensionless Perl shebang
scripts. Ignored/untracked files are excluded. A tracked directory symlink is
allowed only when it resolves to an existing directory inside the checkout;
the manifest does not traverse it. Tracked source-file, broken, and
outside-checkout symlinks are rejected. The separate local library fingerprint
rejects every symlink, including internal links.
It hashes every listed source file and checks Git HEAD/cleanliness, scope, and bytes before
and after. Each file runs `perl -d -c -I<subject>/lib` from the checkout root,
with `nice -n 10`, a 1-minute load ceiling of 9, a 2-second file timeout,
and a 10-minute total limit. Timed-out process groups receive TERM then KILL.
Every file stays in `units`; failures count in `errored`, with per-file reasons
in the sibling `.meta.json` artifact. That metadata also holds the manifest,
verified ctags version and binary SHA-256, compiler keys/native starts/ends,
compiled and unresolved file counts, network policy and EPERM control,
dependency context, and joined/unmatched/ambiguous counts;
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
