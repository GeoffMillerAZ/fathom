# Bash oracle candidates — 2026-10-26

The canonical reading remains L1 excellent, L2 good at C (3,975/4,289 written
pairs, precision 1 / recall .9268), and L3 good at B (import agreement 1,
file-edge recall .9971). The old plan's .9799 call recall and assertion that
there is no SCIP producer are superseded. A released shell producer exists and
has been run over both complete original populations. No holdout joins the
canonical corpus, and no grade promotion follows from a resolved or executed
subset.

## Released producer

scip-shell v0.1.0, released revision `bd95dfa882c2` (full revision in the receipt),
declares MIT in Cargo.toml. The checked repository has that release and a later
main revision [`c8b292cdf577`](https://github.com/jelmer/scip-shell/tree/c8b292cdf577c5d514a26a9241b4b124b251a354) dated 2026-06-15. The
measurement uses the released tag, unchanged Cargo.lock, and a lane-local pinned
Rust SDK. Maintained-indexer qualification is not assumed: even granting the
native maintained-indexer candidate classification, the measured blind population prevents a complete
L2 promotion. Generic pair scoring falls back to product caller extents because
this producer supplies no native enclosing ranges. Those pair diagnostics are
therefore C and product-assisted; they are separate from the independent raw
occurrence census below. The native SCIP occurrence audit uses producer positions, symbols
and targets, never Fathom pairs filtered by producer availability. Variable and
source-path occurrences do not attest command targets. Every original file,
including extensionless scripts, is an explicit producer input; unindexed files
and computed commands remain in the denominator.

| Original scope | Native indexed files | All command sites | Native command occurrences attesting sites | Unattested sites | Native function sites | Native function definitions without enclosing ranges |
|---|---:|---:|---:|---:|---:|---:|
| NVM | 277/277 | 9,147 | 4,454 | 4,693/9,147 (51.31%) | 2,679 | 731/731 |
| Bats | 68/68 | 1,423 | 784 | 639/1,423 (44.91%) | 276 | 309/309 |

Computed commands are 1,514 and 53. Even excluding them, unattested literal
sites are 3,179/7,633 (41.65%) and 586/1,370 (42.77%), above 10% independently
for both subjects; unresolved sites are not dropped. These are occurrence-coverage
counts, not claimed L2 pair precision or recall. Generic pair diagnostics are
C: NVM agrees on 555/555 exported pairs against 3,043 static product predictions;
Bats agrees on 156/157 exported pairs against 932 product predictions. Agreement against this incomplete export is not
a whole-written product-accuracy estimate. Source-path symbols also join each symbol to every
file defining it; they do not attest the filename Bash actually opened. Neither
these joins nor sparse native command occurrences promote canonical L3.

## Native execution

GNU Bash 5.3.20, built with only its official twenty patches, supplies actual
function frames and compiler filenames. shfmt 3.14.1 supplies original source
scope and callers only. Source entry events witness an actual opened file;
definitions-only sources require changed native declare -F metadata. No guessed
CWD target or replacement source builtin supplies truth. Empty, failed or
ambiguous loads remain unattested. RETURN frames enumerate caller file, caller function and executed target using
native fields alone. Every absolute, in-root pair enters the census, even when
its line is ambiguous, dynamic, absent from shfmt, or has a different lexical
caller. Independent shfmt now decides only site-coverage hits. Grading is limited
to the unchanged original eligible caller files, and replay asserts the scorer
keeps exactly those native pairs. Full in-root and out-of-caller-scope counts are
both disclosed. Native opened filenames are executed facts; source comparison
retains C because independent syntax selects that population. Runtime precision,
whole-written L2 recall and whole-source L3 remain unmeasured.

The original NVM driver uses its declared Urchin 0.0.5 and its Makefile's test-bash
rule for every original test subdirectory. The pinned nvmrc Gitlink fixture is
loaded only into disposable copies, outside original measurement membership.
Bats uses its original bin/bats test driver. Native interpreter versions and
fullpath availability, driver failures, skips, timeouts and rejected modified
children are recorded. A zero literal-source denominator is unmeasured, never
perfect coverage. The untraced Bats control distinguishes shared from observer-only failures using
counts and a failed-name-set hash. Shared failures are not automatically attributed
to the environment; observer-only failures remain an explicit measurement limitation.

| Instrumented execution | Original files / bytes | Attested original command sites | Native executed pairs | Native pair recall | Driver outcome |
|---|---:|---:|---:|---:|---|
| NVM, declared Make/Urchin phases | 277 / 475,959 | 496/9,147 (5.42%) | 489 in-root / 457 eligible callers | 450/457 (.9847) | 9 phases attempted; first timed out at 120 seconds, its pair/site/unpaired contributions discarded; other eight exit 2 |
| Bats, declared suite | 68 / 162,377 | 254/1,423 (17.85%) | 248 in-root / 248 eligible callers | 225/248 (.9073) | 459 TAP results: 410 pass / 23 skip / 26 fail, exit 1; no timeout |

NVM's eight completed driver phases report 45 passes and 45 failures in total;
the timed-out phase's incomplete outcomes are not certified. Original requested
legacy Node/io.js binaries fail with an incompatible CPU type on this ARM host,
and macOS mktemp without a template can select its system confstr directory
instead of TMPDIR, where lane-only sandbox writes are denied. No declared runtime
version or dependency contract was replaced. These are environment/driver limits,
not language impossibility claims. A longer run of the first phase remains an
unmeasured coverage route, not a completed native oracle failure. Native events
from completed failing drivers still witness actual execution, without certifying
that the subject's tests passed. Function-only mode retains all 343 NVM literal
source sites (including one external absolute path) and six dynamic/no-argument
sites, with source coverage unmeasured. Bats has zero literal source sites and 34
dynamic/no-argument sites, also unmeasured. No L3 diagnostic runs for either.

The binding review found that the former 318/318 and 213/222 denominators were
selected by tier-C shfmt sites. Those figures are withdrawn as tier-A recall.
Re-decoding all 40 hash-verified raw files yields 489 NVM / 248 Bats in-root native
identities without syntax selection. NVM's 32 excluded identities occur in 17
executed `#!/bin/sh` scripts whose dotted basenames fall outside the shared
extension-first scope rule, which product discovery also applies. They are
product-discovery misses, rather than non-Bash noise. The scope rule accepts
`.sh`/`.bash`, then tries a shebang only when Go-style `filepath.Ext` is empty;
a descriptive basename containing another dot therefore never reaches shebang
dispatch. The pinned tree contains 30 such dotted-name `#!/bin/sh` files, of which
17 contributed these executed pairs. The 17 and 30 file counts are
operator-attested from the raw capture/source inventory at `0140aae`, not
recomputed by the committed checker; 32 is checked against the recorded native
pair population. The unchanged eligible-file population is
457 / 248, with 450 / 225 found. NVM's all-in-root recall bound is at most
**450/489 (.9202)**, beside **450/457 (.9847)** on eligible callers. No broader
full-population grade is inferred from either number.
Seven NVM and 23 Bats misses remain. They include computed command dispatch and
implicit ERR/EXIT callbacks, rather than only the nine previously retained ERR
callbacks. The difference from the historical selected-population counts is 132/139 and
12/26; that excluded subpopulation was strongly biased toward difficult cases. All eligible misses now remain
in the denominator. These executed binding populations differ from canonical
lexical syntax, so neither figure rewrites the canonical reading. Source-contract
miss diagnostics remain C and cannot filter native truth. Raw rows sharing an
accepted native identity may also refer to copied/generated definitions; the
receipt counts those separately without widening original caller scope.

The untraced Bats control yields 430 pass / 23 skip / 6 fail, exit 1. All six
failures are shared with tracing; 20 are observer-only and none control-only.
Several observer-only blocks expose the added RETURN observer in exact error-stack
assertions; this observer perturbs execution/diagnostics and is not transparent.
Shared failures include denied temporary-directory/device-descriptor writes;
not every shared failure is attributed to the environment. The receipt stores
counts and a failed-name-set hash, never subject test names or raw output.
Runtime evidence describes the instrumented execution only. Native versions
5.3.20 (fullpath enabled) and nested system 3.2.57 (fullpath unavailable) are
recorded separately. Certified site-unpaired rows are 24,926 for NVM and
49,532 for Bats. NVM's 22,071 rejected-phase rows are a separate diagnostic,
excluded from accepted pair, coverage and unpaired totals. Site-unpaired rows
include in-scope ambiguity, dynamic commands and lexical-caller mismatches as
well as relative/outside native frames; they do not select the native recall
population. The old 46,997 NVM aggregate mixed accepted and rejected phases and
is withdrawn. No original site is removed from coverage.

## Named candidates and limits

| Candidate / route | Version checked | Concrete result or limit |
|---|---|---|
| [GNU Bash native frames/builtins](https://www.gnu.org/software/bash/manual/bash.html) | 5.3.20; nested system 3.2.57 recorded separately | Actual native measurements above; executed-only function truth and witnessed source entries. Unsupported fullpath is disclosed. No coverage threshold grades whole-written L2. |
| [GNU Bash parser/check/trace flags](https://www.gnu.org/software/bash/manual/html_node/Invoking-Bash.html) | 5.3.20, GPL-3.0-or-later CLI; actual authored contract probe | File-mode pretty-print emits normalized Bash source without original positions or resolved-reference records; command-string mode executes the fixture instead. A third-party walk over printed source is not an owner-bound-reference export. Native execution hooks above obtain actual targets rather than treating printed command text as binding. |
| [GNU Bash xtrace / PS4 / BASH_XTRACEFD and BASH_COMMAND](https://www.gnu.org/software/bash/manual/html_node/Bash-Variables.html) | GNU 5.3.20; native xtrace also exists in system 3.2.57, which lacks BASH_XTRACEFD | Source-contract check: xtrace prints expanded executed commands; DEBUG's BASH_COMMAND exposes the running command text. These can refine attribution on multi-command/dynamic lines. Neither enumerates unexecuted references. The corrected native pair census needs neither a literal command nor unique shfmt site; xtrace is an additional coverage/attribution route, not dismissed as impossible. No original-subject xtrace run or accuracy claim. |
| [kcov Bash engine](https://github.com/SimonKagstrom/kcov/blob/v43/src/engines/bash-engine.cc) | v43, GPL-2.0 CLI | Versioned engine source uses PS4 / xtrace and BASH_XTRACEFD where supported to obtain executed-line coverage. It can extend runtime coverage; coverage reports do not supply whole-written resolved call pairs. Not installed or corpus-scored, so no measured failure is claimed. |
| [bashcov runner](https://github.com/infertux/bashcov/blob/v4.0.1/lib/bashcov/runner.rb) | RubyGems 4.0.1, MIT | Versioned runner enables xtrace with PS4 and BASH_XTRACEFD, then computes coverage. It can extend executed trace attribution; it supplies no unexecuted binding population or automatic whole-written L2 grade. Registry and source contracts checked, not installed/scored. |
| [Oils osh parser / AST dump](https://oils.pub/release/0.38.0/doc/getting-started.html) | 0.38.0 release docs; project Apache-2.0 licence | Documented `osh -n --ast-format text` dumps an independent Bash-compatible syntax tree: tier C for Bash. Oils also documents its own xtrace output route, but executing the subjects with an alternative interpreter would change the declared GNU runtime contract. No install, source translation or corpus-accuracy measurement was attempted. |
| [scip-shell](https://github.com/jelmer/scip-shell/tree/v0.1.0) | Released v0.1.0, bd95dfa882c2 | Entire original populations indexed and native occurrences audited; measured unattested commands above remain misses even under favorable A classification. Source symbols represent written paths rather than filenames actually opened by Bash. |
| [bash-language-server](https://www.npmjs.com/package/bash-language-server/v/5.8.1) | 5.8.1, MIT | Independent parser/name/source resolution, not the GNU engine or a separate maintained SCIP export; custom reference exports remain C. No measured corpus-accuracy claim is made for this route. |
| [ShellCheck](https://github.com/koalaman/shellcheck/tree/v0.11.0) | 0.11.0, GPL-3.0-or-later CLI | Independent static analysis and source-following diagnostics; JSON diagnostics do not supply a whole-source resolved caller/target table or actual opened filenames. Its static path cross-check already informs canonical B evidence. |
| [universal-ctags Sh parser](https://github.com/universal-ctags/ctags/blob/v6.2.1/parsers/sh.c) | 6.2.1, GPL-2.0-or-later CLI | Function declarations and written source-load roles; no shell call-reference role. The checked autoload role belongs to Zsh. Existing B declaration evidence does not grade Bash call targets. |
| [shfmt / mvdan syntax](https://github.com/mvdan/sh/tree/v3.14.1) | 3.14.1, BSD-3-Clause | Structured independent syntax gives the complete source/command population. It does not independently establish engine-resolved targets. |
| [bashlex](https://pypi.org/pypi/bashlex/0.18/json) | PyPI 0.18, GPL-3.0-or-later | Independent Bash syntax tree; custom call/reference extraction C, with no owner evaluation or maintained SCIP route established. Registry contract checked, not corpus-scored. |
| [bashparser](https://pypi.org/pypi/bashparser/0.17/json) | PyPI 0.17 | Independent AST transformations/analysis; C, not native evaluation. Registry licence field is empty; no claim that absence of that field prohibits use. Not corpus-scored. |
| [bash-parser](https://registry.npmjs.org/bash-parser/0.5.0) | npm 0.5.0, MIT | Independent AST, C; not a second native binding oracle. Registry contract checked, not corpus-scored. |
| [brush-parser](https://docs.rs/brush-parser/0.4.0/brush_parser/) | 0.4.0, MIT, released producer dependency | Independent shell parser used by scip-shell; the actual released producer is measured above. Parser reuse is not a second independent native evaluation. |
| [ShellSyntaxTree](https://api.nuget.org/v3-flatcontainer/shellsyntaxtree/0.4.0/shellsyntaxtree.nuspec) | NuGet 0.4.0, Apache-2.0 | Independent Bash/PowerShell AST, C; no native target population established. Not corpus-scored. |
| [Meziantou.Framework.Language.Shell](https://api.nuget.org/v3-flatcontainer/meziantou.framework.language.shell/3.0.5/meziantou.framework.language.shell.nuspec) | NuGet 3.0.5, MIT | Independent cross-shell syntax/diagnostics/editing, C; no owner binding or separate maintained SCIP route. Not corpus-scored. |
| [PsBash.Core](https://api.nuget.org/v3-flatcontainer/psbash.core/0.14.0/psbash.core.nuspec) / [PsBash.Transpiler](https://api.nuget.org/v3-flatcontainer/psbash.transpiler/0.14.0/psbash.transpiler.nuspec) | NuGet 0.14.0, MIT | Bash AST and PowerShell translation, C. Translated execution is not native Bash tier-A truth. |
| [Lukbes.CommandLineParser](https://api.nuget.org/v3-flatcontainer/lukbes.commandlineparser/1.1.0/lukbes.commandlineparser.nuspec) | NuGet 1.1.0, MIT | Parses a program's command-line arguments, outside Bash-source references. |
| [bashdb](https://github.com/Trepan-Debuggers/bashdb/tree/5.2-1.2.0) | Released 5.2-1.2.0 tag `0c7fc59`, GPL-2.0 | Source-contract check: debugger execution/backtraces, not an original whole-source reference export. It can extend executed tracing; coverage of unexecuted references and a whole-written L2 grade are not supplied. Not corpus-installed or scored; no accuracy failure is invented. |
| [bashprof](https://github.com/bashmgmt/bashprof/tree/30974a009e9e4128ab0ce4324aafa6896d3bbb0d) | Source 0.2.1 at `30974a009e9e`, MIT | Primary README/Cargo contract requires explicit BASHPROF_TIMETHIS call sites for measured spans. Adding them changes the subjects, and spans cover executed instrumented sites; no unchanged whole-source binding oracle. Not built or corpus-scored. |
| [GNU Bash type / compgen / declare](https://www.gnu.org/software/bash/manual/html_node/Bash-Builtins.html) | 5.3.20 | NVM 4,902/9,147 available; 4,245 unavailable (1,514 dynamic, 783 external-file, 1,948 missing). Bats 955/1,423 available; 468 unavailable (53 dynamic, 138 external-file, 277 missing). Native namespace answers in a disclosed library union; independent syntax supplies original sites/callers, not native contextual references. This availability audit is separate from call-pair grading. |
| [CentOS downstream Bash rpm-requires](https://gitlab.com/redhat/centos-stream/rpms/bash/-/raw/c8s/bash-requires.patch) | c8s spec 4.4.20-6, GPLv3+; patch SHA-256 `c0667761abba019ea120521bba129f2585f625c01f469b1e2b6877310b8a13e5` | Source-contract route, not installed/scored. Downstream parser patch emits requirement names, not original positioned caller/target pairs; no complete call-grade claim. Its documented command-substitution/conditional/eval omissions require separate coverage. GNU 5.3.20 upstream help does not advertise this downstream flag; no upstream source patch is applied. |
| DTrace | Sun D 1.19, local BEGIN probe | The actual probe is denied by system integrity protection. No sudo, OS-setting change or trace result is claimed. OS calls would also require a separate native caller/source-pair witness. |

## Search receipt (X-R4)

Checked 2026-10-25 to 2026-10-26. Primary pages and registry/API responses were
read locally; only software-contract summaries and counts are published. Search
results are not a proof of absence, and uninstalled candidates are not described
as measured accuracy failures. The concrete sources and queries follow.

| Required source | Search actually checked | Result / limit |
|---|---|---|
| Own toolchain | GNU Bash manual and actual 5.3.20 `--help`: parser/check flags, DEBUG/RETURN metadata, declare -F, fullpath; downstream [CentOS Bash spec](https://gitlab.com/redhat/centos-stream/rpms/bash/-/raw/c8s/bash.spec) and rpm-requires patch | Native routes measured; syntax/printed commands are not silently promoted to bound references. |
| SCIP / LSIF | [Sourcegraph full indexer list](https://sourcegraph.com/docs/code-navigation/writing-an-indexer) and [precise-navigation table](https://sourcegraph.com/docs/code-navigation/precise-code-navigation) and [LSIF.dev](https://lsif.dev/); GitHub repositories API queries `scip-bash` and `scip shell` | 39 scip-bash hits did not supply a Bash producer; the 69-result sibling query found released scip-shell, which was built and measured. The short scip README example list is non-exhaustive. |
| Language servers | [bash-language-server 5.8.1 registry](https://registry.npmjs.org/bash-language-server/5.8.1) and [reference/source resolver](https://github.com/bash-lsp/bash-language-server) | Independent static names, no engine-bound table or second producer established. |
| Package registries | PyPI `/pypi/bashlex/json` and `/pypi/bashparser/json`; npm `/bash-parser/latest` and `/bash-language-server/latest`; NuGet `query?q=bash parser` and exact nuspec URLs above; released [Cargo.toml](https://github.com/jelmer/scip-shell/blob/v0.1.0/Cargo.toml) / Cargo.lock | Named relevant candidates above; five NuGet hits included a CLI-argument parser outside this task. Missing licence metadata is not a claimed ban. |
| Runtime call / require tracing | [Original NVM Makefile](https://github.com/nvm-sh/nvm/blob/v0.40.5/Makefile), Urchin 0.0.5 registry, [Bats package task](https://github.com/bats-core/bats-core/blob/v1.12.0/package.json); GNU xtrace/PS4/BASH_XTRACEFD/BASH_COMMAND contracts; kcov v43 engine source, bashcov 4.0.1 registry/runner, Oils 0.38.0 AST/xtrace docs; bashdb/bashprof contracts above; local DTrace `-n BEGIN` probe | Declared harness measurements and bounded native observers above; failures and coverage retained. Direct-input probes and broken earlier hooks are not used as ceiling evidence. |

## Proof and disposition

Both source populations are the original complete eligible Git checkouts at the
canonical pins (full revisions in the structured receipt): NVM v0.40.5 `1889911f0841` and Bats
v1.12.0 `713504bc0224`. No source translation,
path exclusion, corpus shard, holdout membership, grammar, engine or epoch changes.
Original-source manifests agree before/after every final run. The exact recipe
section 13.3.1 command runs in `tracked_set.py`; only aggregate results are public:

| Git tracked set | Entries | Regular files / bytes | Symlinks | Gitlinks | Before = after SHA-256 |
|---|---:|---:|---:|---:|---|
| NVM | 404 | 402 / 1,312,327 | 1 | 1 | `97e64fd9b84af483db62e4ff7d8d4df2e4ae18a290152b90e2ea5c6d91692a15` |
| Bats | 364 | 354 / 650,570 | 10 | 0 | `f0bf8208af25476fc5fb48ea102ddca0cc44cf4a6a65cd5f45da39aff933e94e` |

Symlink targets are hashed without following them. The pinned Gitlink is counted;
its test-fixture contents execute only in copies and do not join the graded scope.
Full tracked sets include documentation and non-Bash inputs; original eligible
Bash scope is 277/68 files as above. Product discovery agrees in both files and
bytes. Original checkouts have no new untracked or ignored files after probes.
Dependencies, generated tests, runtime downloads and disposable Git metadata stay
in private runtime copies. No raw source/file list or subject people are committed.

The first-review correction passed 44 regressions, including a fresh isolated copy
with no pre-existing workspace. Its 31 unit mutants failed their focused assertion, then passed after
restoration. The correction remains pinned to the reviewed revision; the original
29 mutants/native-mode proof remain at the capture revision. No new subject
execution is claimed. The mutation receipt names each changed file, mutant,
runnable command, FAIL line and restored GREEN. Actual GNU/SCIP independence passes:
changing real product outputs to fabricated pairs leaves native source/function
raw digests and SCIP bytes unchanged; changing known source/function/alias inputs
changes the corresponding truth. Native fixtures witness six source edges,
zero shadowed-source edges, two edges under shadowed metadata builtins, preserved
strict error statuses and whole-unit native overflow refusal. A separate actual
function-only collection proves that a nonzero source population stays unmeasured
with no DEBUG source events. No disabled probe becomes 0% or perfect source coverage.
Historical 39-file proof/capture hashes are checked against the reviewed Git
revision; current decoder, replay, regressions and receipt code are separately
hash-checked. The offline replay verifies all raw digests before decoding and
requires identical native pairs with an empty syntax population. Product scoring
was rerun with indexbench built from the merged branch.
Q7 permits remaining checks under elevated host load at nice10 with at most two
Go workers; no throughput or canonical performance result is inferred.

Caps are whole-unit refusals, not accepted partial output. The committed suite
runs with `python3 -B -m unittest discover -s calibration/oracles/bash-trace -p
 'test_*.py'`; `prove_native.py` additionally exercises the real native record cap.

| Boundary | Production cap | Cap / cap+1 proof |
|---|---|---|
| Original files / source bytes / individual bytes / population | 2,048 / 64 MiB / 2 MiB / 1,000,000 | `test_scope_limits.ScopeRefusalTests.test_each_source_cap_accepts_n_then_refuses_n_plus_one` |
| Combined native pair/source union | 1,000,000 each | `test_scope_limits.NativeUnionTests.test_repeated_observations_do_not_expand_but_cap_plus_one_refuses` |
| Native function bytes / fields / field bytes | 32 MiB / 1,000,000 / 1 MiB | `test_function_decode` byte, field-count and field-bytes cap+1 tests |
| Native source bytes / fields / field bytes | 32 MiB / 1,000,000 / 1 MiB | `test_trace.NativeTrace.test_each_native_cap_refuses_limit_plus_one` |
| Per-process unique GNU function records, Bash >=4 only | 100,000 | Actual `prove_native.py` lower-cap overflow refuses the unit, never accepted prefix truth |
| Legacy Bash 3.2 raw records | No per-process cap; shared 32 MiB/field decoder limits, with 64 MiB OS limit | The recorded 3.2 processes are bounded by whole-unit refusal only; the >=4 native lower-cap proof does not cover them |
| SCIP bytes / documents / occurrences | 32 MiB / 2,048 / 1,000,000 | `test_scip_audit.ScipAuditTests.test_caps_accept_n_then_refuse_n_plus_one` |
| Parser output per stream / structural nesting | 32 MiB / 256 | `test_syntax_scope` parser-byte and parser-nesting cap+1 tests, bounded before decoding |
| Incremental source/call sites | 1,000,000 each | `test_syntax_scope` source-site cap+1 and call-site early-refusal tests |
| Namespace raw bytes / records / field bytes | 32 MiB / 1,000,000 / 1 MiB | `test_lookup.LookupTests.test_lookup_caps_accept_n_refuse_n_plus_one` |

Malformed/truncated records, duplicate or outside-scope SCIP documents, modified source, signals and timed-out children are refused rather
than silently accepted. Parser execution has a 60-second timeout; each driver has
the explicitly recorded time budget, not an inferred language limit. The child
64 MiB OS file limit is above the accepted 32 MiB trace cap, so a file truncated
at that boundary cannot be accepted as complete. No test is skipped for a race
mode; these Python boundary tests also run as their own non-Go suite.

Replay commands, precise pins and native prerequisites are in
[the committed probe README](../../calibration/oracles/bash-trace/README.md).
The [sanitized receipt](data/bash-oracle-2026-10-26/evidence.json) stores counts,
hashes, software versions, per-phase raw-evidence digests and exact code hashes.
`python3 -B calibration/oracles/bash-trace/evidence.py --check
 docs/assessments/data/bash-oracle-2026-10-26/evidence.json` verifies that receipt's
cross-route scope, tiers, populations and committed code; it does not rerun deleted
subjects or publish native raw outputs. Raw traces and subject metadata remain
private. No throughput claim is made from these supplementary probes; canonical
performance measurements and grades are unchanged.

The present ceiling is a measured limit of these versioned routes, not a claim
that Bash cannot have a better oracle. Whole-written L2 remains work on two axes:
the canonical .9268 recall is below the excellence bar, and C cannot become A by
validating selected pairs. Retry when a released maintained indexer supplies its
own caller extents and references over the full original population with at most
10% unattested sites, then remeasure product precision/recall without trimming the
scope. A higher executed-coverage trace alone cannot establish whole-written L2.
Independently retry L3 when permitted native source enumeration, independent of a
C-selected population, attests at least 90% of unchanged literal in-repo source
statements. That condition is not satisfied by the zero-denominator Bats route or
NVM's disabled source probe. Longer native driver runs and an environment that
runs the original legacy binaries are concrete future coverage routes, with no
whole-written promotion promised.

**Decision models.** None applies: deterministic native evidence collection and
fixed tier/scope policy, with no model or paid API.


Validation completed on the branch (Q7 scheduling exception, no performance claim):

```sh
python3 -B -m unittest discover -s calibration/oracles/bash-trace -p 'test_*.py'
python3 -B calibration/oracles/bash-trace/evidence.py --check docs/assessments/data/bash-oracle-2026-10-26/evidence.json
bash scripts/fathom-go.sh test -race -count=1 ./internal/langbench ./internal/verify
bash scripts/fathom-go.sh test -count=1 -run TestNoOrgSpecificNames ./cmd/fathom
bash scripts/fathom-go.sh run ./third_party/agentic-kit/cmd/release-notes -check
bash scripts/check-merge-debris.sh
bash scripts/check-merge-debris.sh --staged
```

The README gives the historical native and 31-unit-mutation proof commands,
including every input flag. This branch does not edit the canonical reading;
main's inherited notes-only refresh is checked field by field, excluding only
`notes`, against hash `aa7aa8e1` at `0140aae`. Holdout sets and the generated
board are unchanged. The controller retains full integration validation.

Binding review disposition: Major 1 is corrected by a native-only denominator
and an empty-syntax invariance check; Minor 1 by certified-phase aggregation and
separate rejected rows; Minor 2 by accurately limiting the per-process cap claim
to Bash >=4; Minor 3 by attaching this assessment's row to the index table; Minor 4
by the four versioned trace/parser contract rows above. Historical execution
artifacts were reused after digest checks; no new subject test pass is claimed.

## Deferred-minor follow-up

**Observation draft — dotted-basename script discovery.** The shared extension-
first rule in `internal/fusion/shebangdispatch.go` and the independent original
scope in `calibration/oracles/bash-source/bash_source_dump.py` both skip the 30
dotted-name `#!/bin/sh` scripts above; 17 yielded 32 native executed identities.
The 17 and 30 file counts are operator-attested from the raw capture/source
inventory at `0140aae`, rather than verified by the receipt checker.
This is a discovery gap, not an oracle inability. A separate controller-serialized
ENGINE change must decide whether an unrecognised extension may fall through to
shebang dispatch, then regrade affected languages. This lane records the draft
without allocating an observation id, as required by the program brief.

Offline re-decode requires raw captures at their recorded absolute path. Native
frame filenames retain that location, so a digest-identical archive restored
elsewhere does not reproduce coverage merely by changing `--native`. Restore the
original location first. The new authored regression runs a miniature capture
through the actual decoder: relocation refuses with a clear path/coverage message
and no output, while replay at the original root succeeds with the same bytes.
This uses the reviewer's documented-precondition option; it adds no rebasing or
new source execution. The current 56-regression suite passes, including the new
relocation diagnostic. The receipt pins historical replay/decoder hashes to the
reviewed revision and checks current code separately. Native counts stay
unchanged. This branch does not edit the canonical reading; main's inherited
notes-only refresh is checked field by field, excluding only `notes`, against
hash `aa7aa8e1` at `0140aae`. The receipt's `canonical_reading_changed: false`
means no edit by this lane, not byte equality across inherited history.

OL-R15 hardens that guard: a differing current reading hash requires `followup`,
and `historical_replay_revision` must identify a committed receipt whose
`canonical_reading_sha256` equals `followup.historical_canonical_reading_sha256`.
The revision is validated as 40 lowercase hexadecimal digits before Git runs;
historical Git reads use the repository root regardless of caller directory.
The guard-deletion and coordinated-revision-swap regressions first failed on
the reviewed code, then passed after the fix. Notes-only refreshes and an
unchanged historical reading remain accepted; metric changes stay refused.

OL-R15's guard mutation proof uses
`python3 -B -m unittest discover -s calibration/oracles/bash-trace -p 'test_evidence.py'`.
Each mutant below was applied separately, killed, then restored; the full
53-test suite passed after restoration. These four current guard mutants are
separate from the 31 historical native/replay mutants.

| File | Mutation | Refusing regression/result |
|---|---|---|
| `evidence.py` | Remove the required-`followup` assertion | `FAIL: test_metric_change_cannot_delete_followup_guard` |
| `evidence.py` | Remove the historical receipt hash equality | `FAIL: test_revision_swap_must_match_receipt_recorded_at_that_revision` |
| `redecode.py` | Remove `cwd=REPO` from historical Git reads | `FAIL: test_check_and_historical_hash_work_outside_repo` |
| `redecode.py` | Remove 40-hex revision validation | `ERROR: test_revision_is_validated_before_git` (unexpected unvalidated Git result) |

The PR122 re-check's S1e follow-up additionally requires every historical revision
to be an ancestor of HEAD (`git merge-base --is-ancestor`, with `cwd=REPO`). If
the receipt at that revision contains `followup`, the current receipt must
contain it with the same `historical_replay_revision`. The S1e regression first
reproduced guard deletion by selecting a later receipt; two further regressions
cover retargeting that inherited revision and a real, readable Git object outside
HEAD's ancestry. All three guards were separately mutated and refused by those
regressions, then restored; the full 56-test suite passed. No native replay or
re-score was performed.
The `evidence.py` inherited-block and replay-revision mutants produced
`FAIL: test_s1e_later_receipt_cannot_drop_followup` and
`FAIL: test_inherited_followup_cannot_retarget_historical_revision`;
the `redecode.py` ancestry mutant produced
`FAIL: test_historical_object_outside_head_ancestry_is_refused`, using the same
focused mutation-test command above.
