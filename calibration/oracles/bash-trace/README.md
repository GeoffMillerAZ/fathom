# Bash native source, executed-function and released SCIP probes

Bench-only supplementary evidence. GNU Bash's own frames and compiler-recorded
filenames supply native truth; shfmt supplies only the complete original command
and literal-source populations. The released scip-shell producer supplies its own
SCIP occurrences and targets. The native-event and raw-occurrence audits are
independent of product output. Generic SCIP pair diagnostics use product caller
ranges when native enclosing ranges are absent and therefore retain tier C.
Canonical readings and holdout membership are unchanged unless a separately
complete recipe proves a permitted regrade.

Requires Darwin ARM64, Python 3.13+, Git, curl, make and Go. Check `df -h ~`
before each heavy step; never start below 20 GiB free. Run one heavy job at a
time with `nice -n 10`, waiting for load <=6. Set TMPDIR/GOTMPDIR/GOCACHE/
GOMODCACHE/XDG paths under the lane's own cache or workspace/, GOWORK=off,
GOMAXPROCS=2 and GOFLAGS=-p=2. No global install, profile change or sudo.
Every measurement --out is fresh; resume is deliberately unsupported.

## Replay

```sh
nice -n 10 python3.13 -B calibration/oracles/bash-trace/fetch.py --out workspace/tools/bash-native
nice -n 10 python3.13 -B calibration/oracles/bash-trace/build_bash.py --out workspace/tools/bash-native
nice -n 10 python3.13 -B calibration/oracles/bash-trace/build_scip.py --out workspace/tools/bash-native
nice -n 10 bash scripts/fathom-go.sh build -o workspace/tools/indexbench ./cmd/indexbench
nice -n 10 python3.13 -B calibration/oracles/bash-trace/measure_scip.py \
  --tools workspace/tools/bash-native --indexbench workspace/tools/indexbench \
  --out workspace/artifacts/lang-bash/scip-final-c
nice -n 10 python3.13 -B calibration/oracles/bash-trace/collect.py \
  --repo . --subject workspace/tools/bash-native/bats --shfmt workspace/tools/bash-native/shfmt \
  --bash workspace/tools/bash-native/bash-5.3/bash --fullpath --driver bats --timeout 600 \
  --out workspace/artifacts/lang-bash/bats-final-v4 --cache workspace/tmp/bash-cache
nice -n 10 python3.13 -B calibration/oracles/bash-trace/control.py \
  --subject workspace/tools/bash-native/bats --bash workspace/tools/bash-native/bash-5.3/bash \
  --observed workspace/artifacts/lang-bash/bats-final-v4 \
  --out workspace/artifacts/lang-bash/bats-control-v4 --cache workspace/tmp/bash-control-cache
nice -n 10 python3.13 -B calibration/oracles/bash-trace/collect.py \
  --repo . --subject workspace/tools/bash-native/nvm --shfmt workspace/tools/bash-native/shfmt \
  --bash workspace/tools/bash-native/bash-5.3/bash --fullpath --driver urchin --functions-only --timeout 120 \
  --urchin workspace/tools/bash-native/urchin/package/urchin \
  --fixture workspace/tools/bash-native/nvmrc-fixture \
  --out workspace/artifacts/lang-bash/nvm-functions --cache workspace/tmp/bash-cache
nice -n 10 python3.13 -B calibration/oracles/bash-trace/replay.py \
  --tools workspace/tools/bash-native --indexbench workspace/tools/indexbench \
  --nvm-native workspace/artifacts/lang-bash/nvm-functions \
  --bats-native workspace/artifacts/lang-bash/bats-final-v4 --out workspace/artifacts/lang-bash/runtime-final
nice -n 10 python3.13 -B calibration/oracles/bash-trace/measure_lookup.py \
  --tools workspace/tools/bash-native --out workspace/artifacts/lang-bash/lookup-final-v3
python3.13 -B calibration/oracles/bash-trace/probe_contracts.py \
  --tools workspace/tools/bash-native --out workspace/artifacts/lang-bash/contracts
```

`bash-pins.json` pins GNU Bash 5.3 plus every official patch through 5.3.20;
there is no local semantic patch. GPL-3.0-or-later CLI evidence is never linked
into Fathom. `rust-pins.json` pins official Rust 1.99.1 components installed only
under the supplied output root. `build_scip.py` builds scip-shell v0.1.0 at its
released tag with Cargo.lock unchanged. Its released Cargo.toml declares MIT.
shfmt 3.14.1 is checksum-pinned. Optional legacy comparison `--driver nvm`
executes each original executable test input separately; it is a direct-input
probe, not NVM's declared harness or evidence that its suite passes.

The current NVM route runs every original test subdirectory through its own
Makefile's `URCHIN=... TEST_SUITE=<directory> test-bash` rule. Its declared
Urchin 0.0.5 supplies setup/teardown and directory semantics. The exact nvmrc
Gitlink fixture is loaded only into disposable runtime copies. The graded
original checkout stays unchanged: 277 eligible files / 475,959 bytes. The
Gitlink remains counted in the tracked-set receipt and its contents remain
outside measured membership. bats-core uses its declared `bin/bats test` driver,
with the original 68 eligible files / 162,377 bytes. Its .bats/generated test
bodies can execute but never join the graded Bash population. Profiles are off for the observer entry shell; PROFILE is not overridden. Original defaults and
all failures, timeouts, load dependencies and interpreter versions are disclosed.

## Independent native truth

A DEBUG event with native FUNCNAME=source, BASH_SOURCE and BASH_LINENO witnesses
entry into a sourced file; native physical opening CWD comes from builtin pwd -P.
Cross-file function calls do not count as source entries. Before/after extdebug
metadata in an isolated metadata-only subshell can witness a definitions-only
source: native declare -F must report a newly changed compiler filename matching
the literal argument, or a unique changed absolute filename with the caller
excluded under GNU Bash's bash_source_fullpath option. No decoder-CWD guess,
replacement source builtin or fabricated subject globals. Empty files, failed
loads and ambiguous/unchanged native metadata provide no witness. Native RETURN
has already popped a sourced-file frame and cannot recover an empty file's name.

RETURN frames independently supply executed caller file, function and target
names. Every pair with both native filenames absolute and inside the original
checkout enters the census before any shfmt lookup. Ambiguous, dynamic, missing
syntax sites and differing lexical callers suppress only site-coverage hits.
The graded executed population is the census restricted to the unchanged eligible
original caller files. Replay asserts that the scorer retains exactly that set;
other in-root caller files are counted separately. Nothing is filtered by product
answers. Executed recall includes dynamic dispatch and implicit ERR/EXIT callbacks;
these differ from the canonical written syntax population. Runtime precision and
whole-written L2 recall are unmeasured. No runtime coverage threshold promises a
whole-written grade. Source comparisons remain C because independent syntax selects
that literal population; zero source sites are unmeasured, never perfect coverage.

The review correction reuses the recorded native executions, never re-executes
the subjects. `redecode.py` first verifies every raw digest and the original
instrument hashes against the committed capture revision, then re-decodes with
the current native-only pairing code. An empty syntax population must produce
identical native pairs. Historical capture/proof code and current decoder/check
code have separate hashes; no old execution is relabelled as a new collector run.

Both traps return zero: Bash restores the interrupted/function status itself.
Returning that status from the observer can trip errexit or a subject's inherited
ERR trap inside an otherwise protected conditional. The strict regression first
fails with each old observer, then passes with native statuses preserved. It also
requires actual CALL bytes and empty stderr, so a missing hook cannot pass.
GNU Bash 4+ deduplicates exact quoted native function records per process before
writing; no target, caller or source position is normalized or substituted.
Legacy 3.2 emits raw records without associative-array deduplication or a
per-process record cap; only the shared 32 MiB trace / field limits bound it.
An over-limit legacy trace refuses the whole unit, never an accepted prefix.
Some original tests explicitly invoke the system Bash as well as the pinned GNU
Bash. The receipt retains every observed native version/fullpath flag and the
system Bash binary hash/version; those calls are not relabelled as GNU 5.3.20.
The explicit --functions-only mode leaves the entire source-site denominator
recorded and source coverage unmeasured. It avoids DEBUG snapshot overhead
for an executed-function measurement; it cannot supply a source grade.
When a subject has no literal-source sites, collect functions without DEBUG
metadata snapshots, retaining native version/fullpath records.

SCIP gets every eligible original file explicitly, including extensionless input.
`scip_audit.py` reads the producer's own document/occurrence fields through the
committed wire decoder; no product output feeds it. It retains unindexed files
and dynamic commands in coverage. Only function/system-symbol occurrences at the
actual command position attest a literal command; variable/source-path occurrences
do not masquerade as resolved calls. The released producer omits native enclosing ranges. Generic pair scoring then
borrows product caller ranges, so those diagnostics are C and explicitly
product-assisted. The independent raw occurrence audit stays separate; it is
not replaced with the generic pairs or promoted by them. No complete grade is claimed.
The assessment records the release and maintenance search separately.

## Boundaries and proofs

Each NVM phase retains separate raw source/function traces, driver status and
log hashes. Replay checks those hashes; no phase overwrites an earlier witness.
Combined native function pairs and source edges each cap at 1,000,000, with
whole-unit refusal at cap+1. Children run sequentially in disposable copies. `runtime_copy.py` preserves
symlink targets and copies the pinned Git metadata, retaining driver semantics
without writing to the original checkout. Shared-worktree Git files are refused. Both runs use fresh per-phase HOME, npm, temporary and XDG roots inside
their output directory. Host shell/NVM/Git/npm overrides are removed; PROFILE
is not overridden. macOS mktemp without a template can still prefer its system
confstr directory and is denied there; no global write exception is used. sandbox-exec permits writes only
to output/cache roots and /dev/null. Hash every original source after each child;
source-modified, timed-out or signalled child events are rejected. A live owned
supervisor records completion then terminates its own group, including itself;
no recycled or unrelated process group is targeted. Driver failure is never a
passing-test claim. The untraced control runs the same original Bats driver and
write sandbox without BASH_ENV or oracle hooks, and records test counts and the
overlap of failed tests using aggregate counts and a name-set hash only. Native
observer frames can alter tests that assert exact error-stack output; a traced
failure is not attributed to the environment without the control comparison.
The exact section 13.3.1 tracked-set manifest body runs before
and after each complete measurement; publish counts/hashes only. Hash symlinks
without following them and count Gitlinks without claiming their contents.

Caps refuse the entire affected unit: 2,048 original files, 64 MiB total source,
2 MiB per source file, 1,000,000 source/command sites, 32 MiB per native trace,
1,000,000 NUL fields, 1 MiB per field, and 100,000 unique native function records
per GNU Bash >=4 process. Bash 3.2 has no per-process record cap; its bounds
are the decoder's shared 32 MiB/field limits and 64 MiB OS file limit. Native overflow writes a refusal marker, never an accepted
partial oracle. The child's OS file limit is 64 MiB, strictly above the decoder's
32 MiB cap, so OS-truncated evidence cannot pass. SCIP has 32 MiB, 2,048 documents
and 1,000,000 occurrences. Source/parser/SCIP/namespace fixture caps may only lower production limits.
Native decoder fixtures exercise their fixed production defaults at smaller bounds.
The fixtures accept N and refuse N+1;
malformed/truncated fields, duplicate/outside-scope documents and invalid caps
also refuse. A refused full-driver probe is reported as unavailable evidence,
not an oracle or language impossibility.
shfmt output is written to lane-local bounded files before JSON decoding: 32 MiB
per output stream and structural depth 256. The child file limit is one byte
above the accepted output limit so cap+1 is detected before decoding. Parser
failure, timeout, malformed output or excess nesting refuses the whole unit.
Source and command sites are capped during construction, before later files can
be processed. A regression places an invalid parser input after the overflowing
site to prove that refusal happens first.

```sh
python3 -B -m unittest discover -s calibration/oracles/bash-trace -p 'test_*.py'
nice -n 10 python3.13 -B calibration/oracles/bash-trace/prove_native.py \
  --tools workspace/tools/bash-native --indexbench workspace/tools/indexbench \
  --out workspace/artifacts/lang-bash/proof-final --allow-loaded-host
nice -n 10 python3.13 -B calibration/oracles/bash-trace/prove_mutations.py --tools workspace/tools/bash-native --out workspace/artifacts/lang-bash/mutations-final
```

The actual GNU/SCIP proof puts real lang-probe output in both subject and native
output directories, changes it to fabricated pairs and requires unchanged raw
source/function/SCIP truth digests. Known source-file/function/alias input changes
must change native truth. It also checks definitions-only and shadowed-builtin
loads, no shadowed-source witness, strict status preservation and native cap
refusal. The original twenty-nine mutation tests kill changed target/caller/line pairing, byte/field/scope/
document caps, guessed CWD, nonzero observer status, variable-as-command and
missing-file denominator logic, parser bytes/nesting, incremental site caps,
dropped external source sites, skipped tests counted as passes and incomplete
driver outcomes and promotion from perfect C-selected source coverage, symlink/Git-copy semantics, product-range-assisted SCIP tier and combined native observation caps and writable child scratch, native namespace inventory admission and lookup caps, then restore GREEN.

Keep raw source, traces, SCIP documentation, file lists and dependency metadata
private: they can contain subject people, usernames, authors/emails, CODEOWNERS
or copyright holders. Commit only sanitized counts, hashes and software contract
summaries. Delete measured subjects, source builds, SDKs and downloads after final
capture. Holdouts never enter a canonical corpus shard; no board regeneration,
shared engine change, epoch bump or plan-checkbox edit belongs to this probe.

The capture exporter consumes the fresh output directories named above and the
proof outputs. It includes only count/hash summaries, never source/file lists,
raw traces, SCIP documents or TAP names:

```sh
python3 -B calibration/oracles/bash-trace/evidence.py \
  --capture workspace/artifacts/lang-bash --tools workspace/tools/bash-native
python3 -B calibration/oracles/bash-trace/evidence.py --check \
  docs/assessments/data/bash-oracle-2026-10-26/evidence.json
```

The committed check revalidates cross-route populations, unchanged manifests,
tier caps, mutation results, truth independence and exact collector/proof hashes.
It verifies the receipt; it does not claim to rerun deleted calibration subjects.

For the final independence refresh only, owner Q7 permits `--allow-loaded-host`
at nice10 after the disk guard, with no performance claim. Without that explicit
flag, the proof still waits for load <=6. Other original native captures retain
their recorded guards. Q7 also permits remaining Go checks with at most two
workers; it does not waive disk limits or permit concurrent heavy jobs.

## Binding first-review correction

The corrected native census is NVM 489 in-root identities / 457 eligible callers,
450 found (.9847), and Bats 248 / 248, 225 found (.9073). The 32 NVM identities
outside the original eligible caller-file set are disclosed, not silently dropped.
shfmt still attests only 496/9,147 and 254/1,423 written sites. Certified NVM
site-unpaired rows are 24,926; rejected phase rows are 22,071, reported separately.

After the same disk/cache/nice prerequisites above, offline replay needs only
original sources, shfmt and indexbench; it needs no new GNU Bash/Rust build:

```sh
nice -n 10 python3 -B calibration/oracles/bash-trace/fetch.py --scope-only --out workspace/tools/bash-review
nice -n 10 python3 -B calibration/oracles/bash-trace/redecode.py --native workspace/artifacts/lang-bash/nvm-functions --subject workspace/tools/bash-review/nvm --shfmt workspace/tools/bash-review/shfmt --out workspace/artifacts/lang-bash/redecoded-r1/nvm --capture-revision db8faed7ba3feb5a02dbcd4548f6ab1ac9850e5c
nice -n 10 python3 -B calibration/oracles/bash-trace/redecode.py --native workspace/artifacts/lang-bash/bats-final-v4 --subject workspace/tools/bash-review/bats --shfmt workspace/tools/bash-review/shfmt --out workspace/artifacts/lang-bash/redecoded-r1/bats --capture-revision db8faed7ba3feb5a02dbcd4548f6ab1ac9850e5c
nice -n 10 bash scripts/fathom-go.sh build -o workspace/tools/indexbench ./cmd/indexbench
nice -n 10 python3 -B calibration/oracles/bash-trace/replay.py --tools workspace/tools/bash-review --indexbench workspace/tools/indexbench --nvm-native workspace/artifacts/lang-bash/redecoded-r1/nvm --bats-native workspace/artifacts/lang-bash/redecoded-r1/bats --out workspace/artifacts/lang-bash/runtime-review2
nice -n 10 python3 -B calibration/oracles/bash-trace/prove_mutations.py --unit-only --out workspace/artifacts/lang-bash/mutations-review1
python3 -B calibration/oracles/bash-trace/evidence.py --capture workspace/artifacts/lang-bash --tools workspace/tools/bash-review --historical-base db8faed7ba3feb5a02dbcd4548f6ab1ac9850e5c --runtime-folder runtime-review2 --mutations-folder mutations-review1 --nvm-folder redecoded-r1/nvm --bats-folder redecoded-r1/bats
```

All 31 current unit mutants fail and restore GREEN, including independent native
pairs on ambiguous/dynamic lines and complete rejection of timed phase measures.
The original 29-mutation/native execution proof is retained as historical evidence
at the capture revision. It is not claimed as a new execution of modified code.

The historical capture commit must be available in the branch history for the
receipt checker and offline re-decoder. A shallow clone must fetch that ancestor
first. Historical checks validate committed bytes at that revision; current
code hashes validate the modified decoder and receipt reader.
