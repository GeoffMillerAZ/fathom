# Perl compiler range candidate

This is a **measured, compiler tier-A** range companion for the actual
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
proof of network denial. The subject body is not run. The compile guard
`FathomPerlCompileGuard.pm` resets include paths and validates modules at
`CHECK`; range compilation also reads compiler-populated debugger data, without
parsing source or calling subject subroutines. Universal Ctags 6.2.1 independently supplies the
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
hash; compiler `-MO` loads with the same core roots first. Xref must run without
`-d`: debugger COPs use `dbstate`, which stock B::Xref 1.07 does not handle,
leaving stale file and line attribution. The guard is explicitly loaded before
Xref. Raw stdout and stderr stay in the private log directory. It counts direct
calls, explicit `meth` events, and target-unknown non-direct `subused` rows
separately. Earlier `-d` census output is withdrawn as invalid; Xref remains
diagnostic only and never receives a neutral grade.


The runner clears inherited `PERL5LIB`, `PERLLIB`, and `PERL5OPT` and never
puts the local dependency prefix in the process environment. Before each
subject compile, the guard resets `@INC` to the pinned Perl
`Config` `privlibexp` and `archlibexp` core roots, subject `lib`, and the
explicitly locked local `lib/perl5` plus its `Config` `archname` directory.
Subject `use lib` directives may add subject paths such as `t/lib`. At `CHECK`,
every loaded module path must resolve under the core roots, the subject checkout,
or the locked local library. Other site/vendor paths fail the compile and leave
the file unresolved against coverage. All compiles still run in the fixed
network-denying sandbox. The per-compile profile also denies `file-read-data`
outside an allowlist containing the exact Perl executable, guard, and
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
and after. Each range compile runs `perl -d -c` with the guard establishing approved include paths, from the checkout root,
with `nice -n 10`, after load falls to 9 or below, and a 2-second compile
timeout that starts after the load wait. Load waits count against the 10-minute
total limit but not the compile timeout. Timed-out process groups receive TERM then KILL.
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
output is read. The compliant canonical repeat measured the coverage and range quality below.
This is a candidate audit, pending the preregistered independent validation.

The runnable verification command uses `PERL_BINARY` and `CTAGS_BINARY` set to
available executables reporting exactly Perl 5.34.1 and Universal Ctags 6.2.1:

```sh
: "${PERL_BINARY:?set to the pinned Perl executable}"
: "${CTAGS_BINARY:?set to the pinned Universal Ctags executable}"
python3 calibration/oracles/perl-db-ranges/self_test.py --perl "$PERL_BINARY" --ctags "$CTAGS_BINARY"
```

## Compliant canonical repeat and native call limits

The C-R9 repeat at instrument commit
`2fefe5d42420761f5d36509129e6f08592adbb8b` compiles 270/280 Mojo units and
257/263 Perl-Critic units for ranges. All sixteen unresolved units remain in
coverage. Reciprocal unique joins yield 1,286/1,308 and 1,584/1,614 named
subroutine ends, respectively: 2,870 within-one-line matches, 98.22% of named tags and
85.77% of all 3,346 matched definitions. The actual harness reading is L1
excellent and L2 good, with range agreement 1 and every non-range metric and
count unchanged. Its range metric stays tier B under the primary Ctags tier
cap; the native source is tier A only for the actual compiled population.
The PPI 1.270 call oracle remains tier C. Median-of-three throughput is
2.43 MB/CPU-s (2.45, 2.43, 2.42) in the final harness-only repeat. Its four
grade/build phases launched at one-minute loads 5.60, 5.63, 5.74 and 5.74.
The earlier 2.41 triple was preliminary. The historical canonical 3.14
MB/CPU-s used a different harness and is not a controlled throughput comparison.
No canonical reading or corpus shard is
changed by this audit.

Stock B::Xref 1.07 is diagnostic: 270/280 and 255/263 units compile, producing
4,812/2,935 direct-call records and 8,316/3,092 non-direct records with unknown
targets. Records can repeat across compilations and are not unique edge counts.
No method targets are emitted. The older debugger-mode census is withdrawn.

The bounded `FathomPerlNamedMethodXref` 0.01-probe derivative reads literal
METHOP names from the compiler, without resolving a receiver class. Its fixed
control attests two direct calls, two literal method names and one unresolved
dynamic method. The anonymous-closure method and optimized constant call each
remain absent and unresolved. This is a synthetic candidate control, not a
full-source call oracle or a grade. It does not alter the PPI reading.
`B::Concise` 1.004 can display compiler METHOP names too; its named-stash walk
and optimized op tree do not by themselves enumerate all original source calls.
The derivative's gaps do not establish that every possible compiler adapter
is exhausted.

```sh
python3 calibration/oracles/perl-db-ranges/method_probe.py --self-test
python3 calibration/oracles/perl-db-ranges/method_probe.py --perl "$PERL_BINARY" --output-dir workspace/tmp/perl-method-control
python3 calibration/oracles/perl-db-ranges/receipt.py --self-test
python3 calibration/oracles/perl-db-ranges/grade.py --input-dir "$PERL_NATIVE_RESULTS" --out-dir "$PERL_GRADE_RESULTS" --subject "mojo=$MOJO_ROOT" --subject "Perl-Critic=$PERL_CRITIC_ROOT" --ctags "$CTAGS_BINARY" --native-instrument-commit 2fefe5d42420761f5d36509129e6f08592adbb8b
python3 calibration/oracles/perl-db-ranges/receipt.py --input-dir "$PERL_NATIVE_RESULTS" --grade-dir "$PERL_GRADE_RESULTS" --reading "$PERL_GRADE_RESULTS/reading.json" --baseline docs/bench/readings/lang/perl.json --instrument-commit 2fefe5d42420761f5d36509129e6f08592adbb8b --harness-commit "$PERL_HARNESS_COMMIT" --archive-metadata "$PERL_DEPENDENCY_ARCHIVES_PRIVATE" --output workspace/tmp/perl-native-receipt.json
```

`PERL_NATIVE_RESULTS` is the directory containing the two runners' private
metadata, neutral companions, stock Xref summaries, phase receipts and actual
harness output. The receipt collector validates source identities, units,
EPERM denial, dependency fingerprints, input digests and unchanged non-range
counts/metrics. It publishes explicit counts and hashes only, never raw
manifest entries, native names, subject diagnostics or copyright metadata.
The method control imposes a 32 MiB live file limit on each output stream;
its self-test attempts the limit plus one byte. Both commands refuse existing
output paths. Dancer2 was spent once at frozen instrument `500a867de528dfe651b3ec5b61d1779625c6e595`.
That run is diagnostic only: 279/343 units compiled, 624/676 named ends paired,
623 eligible ends agreed within one line, and median throughput was 2.29
MB/CPU-s. The native source manifest contains 343 Perl units while the actual
harness discovery contains 332 files, a count gap of 11. This is a population
difference, not eleven proven missing range records; the collector reports both
counts and refuses a complete-population gate for a future holdout with such a
gap. No compiler-coverage or named-pairing floor was committed before
the first source read, so this result cannot authorize canonical adoption or
be reused as a fresh holdout.

The committed holdout collector reads the private one-shot artifacts without
rerunning an oracle. It checks the source manifest, spend and phase sequence,
EPERM control, dependency lock and saved archive hashes, paired versus eligible
counts, binary hashes and three retimings. Its frozen-instrument file hashes
come from Git object `500a867...`, not the later working-tree files. Dancer2's
legacy phases lack per-phase instrument/source/binary identity, which the
receipt discloses; a future holdout must bind those identities on each phase.
Coverage failures still produce count/hash receipts.

The collector also publishes a spent phase failure if compiler, grade or
retime stops early. It records each phase's exit, timeout and launch load;
missing reading, timing or compiler counts are null rather than successes.
Source and instrument identity checks remain binding on all available inputs.
For a complete reading, collect the count/hash receipt with:

```sh
python3 calibration/oracles/perl-db-ranges/holdout_receipt.py --self-test
python3 calibration/oracles/perl-db-ranges/holdout_receipt.py --scratch "$PERL_HOLDOUT_RESULTS" --instrument-repo . --instrument-commit 500a867de528dfe651b3ec5b61d1779625c6e595 --output workspace/tmp/perl-dancer2-diagnostic-receipt.json
```

For a future fresh subject, the same collector requires its full source pin,
subject name and `--coverage-floor .95`; the floor is checked against all
tracked Perl units and all independent named tags before a passing receipt.

`holdout.py` is the committed one-shot orchestration for that future subject.
Freeze its source commit, remote, license and clean instrument commit in the
protocol before invoking it. The runner refuses an existing scratch directory,
writes a source-commit spend register under the Git common directory shared by
all worktrees and
`holdout-spent.json` before clone, and records full source/instrument
commits on every phase plus the built binary hash on build, grade and retime.
It uses the pinned Perl/PPI/Ctags inputs, installs only the subject's own
tracked CPAN declaration into an isolated prefix when one exists, then runs
the unchanged PPI primary, native range companion, fresh indexbench build,
grade and three retimings. Compiler network and write restrictions remain in
`runner.py`. It calls `holdout_receipt.py` after a completed run or a
post-spend failure. A failed phase still exits nonzero after publishing its
count/hash receipt with false acceptance and unknown counts left null.
Load waiting and execution each have an independent bounded budget; the phase
row names load, launch, contract and interrupt failures even if no child exits.
Before build and collection, the runner rechecks the instrument commit and
cleanliness. Collection executes committed instrument files copied into scratch.
Identity or consistency mismatches still refuse publication. Retime records
bind the immutable source, oracle, binary and reading digest before launch;
a missed coverage or population gate also remains a false criterion.
No replacement subject has been selected or run by this instrument addition.
Before running, commit exactly one `## Next holdout pin` fenced JSON block in
`PROTOCOL.md` with string fields `subject`, `source_commit`, `remote`, `license`,
and `dependency_declaration` (`none` or the tracked CPAN declaration). The
runner reads that block from the instrument commit and requires exact argument
matches. A fresh clone has no spend register, so the committed pin and the
measured-commit denylist must be updated after a real spend before using a
fresh clone. An old frozen commit alone cannot know about a later spend.

```sh
python3 calibration/oracles/perl-db-ranges/holdout.py --self-test
python3 calibration/oracles/perl-db-ranges/holdout.py --instrument-repo . --instrument-commit "$FROZEN_INSTRUMENT_COMMIT" --subject "$FROZEN_SUBJECT" --source-commit "$FROZEN_SOURCE_COMMIT" --remote "$FROZEN_REMOTE" --scratch "$FRESH_HOLDOUT_SCRATCH" --perl "$PERL_BINARY" --ctags "$CTAGS_BINARY" --cpanm "$CPANM_BINARY" --cpanm-archive "$CPANM_ARCHIVE"
```

The harness-only repeat requires a clean full-commit checkout and configured
`TMPDIR`, `GOTMPDIR` and `GOCACHE` directories outside it. Set
`PERL_GRADE_RESULTS` to a fresh path under this checkout's `workspace/` or
configured `TMPDIR`; set `PERL_HARNESS_COMMIT` to the clean commit used by
`grade.py`. The committed runner builds the binary once from that source,
records its hash and commit on every phase, regrades the two frozen primary
and range bundles, and retimes three runs. It reruns no Perl compiler or
call oracle. The receipt retains distinct native-instrument and grading
commits and refuses repeated/missing timing labels or phase identity drift.

Mutation controls: disabling the adapter's literal-name callback fails the
fixed expected/observed method assertion. Replacing the receipt's native-array
length with the array fails the privacy assertion. Restoring the unchanged
files passes both controls. Caps: a 1,024-byte file-limit control writes
1,024 bytes then attempts one additional byte; the second write fails and
the first remains exactly bounded.
