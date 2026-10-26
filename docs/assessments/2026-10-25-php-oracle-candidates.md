# PHP oracle candidates — 2026-10-25

The [canonical reading](../bench/readings/lang/php.json) stays L1/L3/L4
excellent and L2 good: written-call precision/recall 1/1 at tier B. The earlier
candidate search was incomplete. Three additional SCIP producers and an actual
Xdebug trace are now measured by the [committed recipe](../../calibration/oracles/php-candidates/README.md).
These are supplementary measurements, with no new canonical reading or corpus shard.

**OL-R7:** PHPStan and Psalm custom semantic exports remain C under ADR-048.
**OL-R9:** a fork is A only when maintained with tagged releases and run at a
released tag; unreleased forks/local patches are C. Xdebug records are A only
for calls the PHP engine executed, with coverage disclosed. Unexecuted code
never enters the runtime recall denominator.

## Whole-population SCIP measurements

The original Git checkouts contain all **74 Guzzle** and **217 Monolog** PHP
files, including files omitted by upstream export-ignore archives. The complete
syntax denominator is **2,571 / 4,558 unique written pairs**. The harness scopes
SCIP comparisons to emitted documents; the separate audit also counts every
written pair in an unindexed file. Neither endpoints nor truth pairs are copied
from the syntax dump. The dump supplies the population denominator only.

| Producer and checked pin | Release / licence / classification | Guzzle full unattested pairs | Monolog full unattested pairs |
|---|---|---:|---:|
| ArtemisMucaj/scip-php (Mago), `04b34f724c9464256891345153b5c2c0a6c23bc8` | Released v0.3.0; MIT declared in Cargo.toml and README, no standalone licence file. Six tags through 2026-04-06; A eligibility is conditional on maintained-indexer admission. Even granting A, the blind cap fails. | 597/2,571 (23.22%) | 713/4,558 (15.64%) |
| michalboryczko/scip-php, `40288605f28ea0111b42f81a8760ad5b79b8c3a0` | MIT; call-site/experimental fork, no tags, C under OL-R9 | 1,171/2,571 (45.55%) | 189/4,558 (4.15%) |
| devmdeniz/scip-php, `9af23514a898dd63a8123b02b567711fbef5dd99` | MIT; php-parser 5 / PHP 8.4 fork, no tags, C under OL-R9 | 1,661/2,571 (64.61%) | 1,228/4,558 (26.94%) |

The call-site fork was run with its documented experimental mode. Its native
SCIP section is transported to protobuf with document/occurrence counts checked;
no reference is synthesized. Its Guzzle index omits **17 original files holding
1,052 written pairs**, in addition to **119** blind pairs inside indexed files.
Reporting only 119/2,571 would falsely put it under the cap. Monolog alone is
under 5%, but its fork remains C and Guzzle still fails full-population coverage.
Mago emits 73/214 documents; the omitted 1/3 files contain zero written pairs.
Its within-index call precision/recall is .7639/.9990 and .8157/1.0000. Complete
counts, coverage accounting and hashes are in the aggregate receipt linked below.

## Native executed-call measurement

Xdebug **3.5.3**, machine-readable trace format **1**, is built against matching
private PHP SDK headers. The original `vendor/bin/phpunit` suites are run in
both **PHP 8.5.8** and **PHP 8.4.23**, within the subjects' declared PHP ranges.
Guzzle's declared integration-test dependency is restored for runtime runs;
its declared local test servers are started only after checking their ports
are free. One helper string redirects its hard-coded log into the workspace.
Before/after syntax dumps prove identical definition names/ranges and call pairs;
all original literal call sites stay in the denominator. The string is the only
PHP source instrumentation change, and is disclosed rather than claimed byte-identical.
Composer plugins/scripts are off; ignored platform requirements, resolved package
versions and lock hashes are retained privately. Missing optional integrations
and test failures are disclosed in aggregate; a completed trace does not imply
a passing test suite.

Native pair endpoints come from **Xdebug's actual target and PHP Reflection's
named caller ranges**. The syntax walk supplies only literal-site scope and
coverage. Alias mismatches, same-line duplicates, a dynamic call sharing the
source line, and ambiguous implicit magic dispatch remain unattested. Native
callbacks whose actual parent disagrees with the lexical caller are rejected.
All these sites remain in the original denominator. Regressions cover these
attribution traps, including a fake syntax caller that must never enter truth.

| Runtime / subject | Attested original literal sites | Executed native pair recall | Completed original suite |
|---|---:|---:|---|
| PHP 8.4.23 / Guzzle | 2,552/3,189 (80.03%) | 2,099/2,099 | 566 tests; 20 errors, 9 skipped |
| PHP 8.4.23 / Monolog | 4,625/5,968 (77.50%) | 3,564/3,569 (.9986) | 1,153 tests; 1 failure, 26 skipped |
| PHP 8.5.8 / Guzzle | 2,472/3,189 (77.52%) | 2,065/2,065 | 566 tests; 111 errors, 9 skipped |
| PHP 8.5.8 / Monolog | 4,615/5,968 (77.33%) | 3,558/3,563 (.9986) | 1,153 tests; 2 errors, 1 failure, 26 skipped |

All four native traces have their completion marker and stay below the 2 GiB
per-file cap. PHP 8.4.23 is the corrected compatible-runtime replay: it removes
PHP 8.5 deprecation interruptions and the two missing-private-PHP PATH errors
in the earlier Monolog run. Guzzle still encounters dependency/API and CA-bundle
test errors; Monolog retains an error-handler assertion failure and optional
integration skips. These failures limit exercised coverage; they are not a
claim that a repaired environment or broader driver could never improve it.
Both environments' five Monolog native-pair misses are case-only name spellings;
they remain misses under the current exact-name comparison, without normalization.
The executed-population result cannot lift the whole written-population grade.

The [aggregate measurement](data/php-oracle-2026-10-26/candidates.json) records
coverage, executed-native-pair recall, suite counts, failures/skips, trace
completion and hashes for both environments. The raw diagnostic harness's
precision includes unexecuted predictions, so it is **unmeasured** for this
executed-only comparison and is not published as a runtime precision grade.
No runtime result grades recall on unexecuted code. Raw trace, source/file lists,
private paths and subject content stay outside committed data.

## Other named candidates and concrete limits

| Candidate / route | Version / revision checked | Outcome |
|---|---|---|
| davidrjenni/scip-php | `71a5b117ec4c` (full pin in the existing scip-php recipe); tool_version 0.0.1; latest published v0.0.2 | Existing native reading still omits 1,869 pairs, about 31%. Current producer source equals the measured pin; no new receiver-resolution fix found there. |
| nikic/php-parser dump | 5.7.0, committed recipe | B syntax agreement; the canonical 1/1 result cannot lift L2 to excellent. |
| PHPStan Rules / Collectors | 2.3.2 | Analyzed Scope and nodes allow semantic exports. Those custom exports are C by OL-R7; this is a tier limit, not a claim that diagnostics are its only API. |
| Psalm analysis plugins | 7.0.0-rc10 | Expression/function/method hooks allow semantic exports; C by OL-R7. |
| Phan language server / analysis | 6.0.11, 2026-10-15 | Maintained independent analyzer with definition/reference facilities; custom export C under the same ADR-048/OL-R7 classification, no tier amendment. |
| phpactor language server | 2026.10.07.0 | Definitions/references available; third-party analyzer export C, not a maintained SCIP producer or owner runtime. |
| PHP engine observer API | PHP 8.5.8 and 8.4.23 SDK source | Measured through Xdebug 3.5.3: its xdebug_base_minit registers zend_observer_fcall_register for PHP >=8.1, including internal functions since 8.2; executor wrappers handle include/require/eval. Thus the owner observer route is actually exercised, with executed coverage disclosed, rather than dismissed as requiring another exporter. |
| Standalone runtime packaging | Static CLI 8.5.8/8.4.23 executed; FrankenPHP 1.13.3 Darwin ARM64 asset checked | Installable without a system PHP. Availability is no longer a blocker; packaging alone does not resolve unexecuted references. |

## Search sources (X-R4)

Checked 2026-10-25/26. These are bounded searches with their positive hits and limits.

| Required category | Source actually checked | Result |
|---|---|---|
| Owner toolchain | [PHP CLI](https://www.php.net/manual/en/features.commandline.options.php), matching PHP release SDK `Zend/zend_observer.h`, [Reflection](https://www.php.net/manual/en/book.reflection.php) | Engine execution/observer and native source ranges identified; actual Xdebug/Reflection route measured above. |
| SCIP / LSIF | [Sourcegraph full supported table](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP ecosystem](https://scip-code.org/), [full archived LSIF.dev table](https://lsif.dev/), [GitHub scip-php repository query](https://github.com/search?q=scip-php&type=repositories) | Nine repository hits: four PHP producers, all named above; remaining hits concern unrelated optimization/ML tools or spelling variants. The scip README's ten examples are non-exhaustive and supply no absence proof. |
| Language servers | Released [Phan](https://github.com/phan/phan/tree/6.0.11), [phpactor](https://github.com/phpactor/phpactor/tree/2026.10.07.0), PHPStan/Psalm extension contracts | Definition/reference routes considered, with custom exports classified C rather than dismissed for requiring adapters. |
| Package registry | [Packagist Phan versions](https://repo.packagist.org/p2/phan/phan.json), [phpactor versions](https://repo.packagist.org/p2/phpactor/phpactor.json), declared producer composer.json/Cargo.toml and existing php-parser install | Pinned release versions and parser/dependency contracts checked; no owner/maintained-SCIP status inferred from a parser dependency. |
| Runtime call / require tracing | [Xdebug function-trace contract](https://xdebug.org/docs/trace), [format 1](https://xdebug.org/docs/trace#computerized), original PHPUnit drivers | Built and measured instead of dismissed for lacking a whole written-call index; OL-R9 supplies executed-only A classification. |

Producer evidence: [call-site fork](https://github.com/michalboryczko/scip-php/tree/40288605f28ea0111b42f81a8760ad5b79b8c3a0),
[Mago release](https://github.com/ArtemisMucaj/scip-php/releases/tag/v0.3.0),
[PHP 8.4 fork](https://github.com/devmdeniz/scip-php/tree/9af23514a898dd63a8123b02b567711fbef5dd99),
[PHPStan collectors](https://phpstan.org/developing-extensions/collectors),
[Psalm hooks](https://psalm.dev/docs/running_psalm/plugins/authoring_plugins/).

## Disposition

The named static candidates do not establish excellent L2 over both complete
subjects. The current native test drivers also leave original written sites
unattested; their actual executed pairs are A, with coverage and failures retained.
The retry conditions are counted full-scope native coverage under LB-F175's 10%
blind cap, or a native runtime driver covering at least 90% of the original written
population with executed-only scoring preserved. No exclusion, new subject,
C-to-A relabeling or tier amendment is used to obtain a passing result.

## Provenance checks and remaining handoff work

The supplementary measurements used product revision `51254ab4f`; they precede
the subsequent main integration and do not claim a new support level. Raw
subjects, SDK builds, downloads and private traces were discarded after the
measurements. Public counts and hashes remain; dependency-lock snapshots are
not published, so later constraint resolution may change replay bytes.

The subsequent proof replay rebuilt Xdebug 3.5.3 against PHP 8.5.8. Runnable
command: `nice -n 10 python3.13 -B calibration/oracles/php-candidates/prove_native.py --tools workspace/tools/php-proof --indexbench workspace/tools/indexbench --out workspace/artifacts/lang-php/native-proof`.
The [aggregate proof receipt](data/php-oracle-2026-10-26/native-proof.json)
records the tool/code hashes. Replacing real `lang-probe` output with fake
calls changes its digest but leaves the raw native oracle digest identical
(`04aaad2b…820d6c`). Changing the known native fixture input from `strlen` to
`strspn` changes exactly one native truth pair (`b42fe8bc…19285`). Both native
collections assert two of four literal sites, leaving alias and same-line
dynamic calls unattested. No extractor-output artifact is an oracle input.

The [scope receipt](data/php-oracle-2026-10-26/scope-proof.json) comes from
`nice -n 10 python3.13 -B calibration/oracles/php-candidates/prove_scope.py --tools workspace/tools/php-proof --out workspace/artifacts/lang-php/scope`.
It runs the exact design section 13.3.1 manifest body before and after a fresh
complete syntax-population audit on both original Git pins. Both tracked-set
digests stay equal; canonical PHP counts/bytes and all 2,571/4,558 written pairs
agree, with zero generated ignored/untracked paths. The manifest covers all
tracked file kinds; non-PHP files explain its larger totals. Neither subject
has a symlink or gitlink, and submodule contents are not claimed. Syntax skips
`.git`, dot directories, `vendor` and `node_modules`; none hides an original
eligible PHP file in these clean trees. This is a reconstruction at the same
pins, not a retrospective manifest of the discarded native-suite runs.

Mutation: `python3.13 -B calibration/oracles/php-candidates/prove_mutations.py`
substitutes the syntax caller for the Reflection caller, removes the same-line
nonliteral guard, bypasses whole-collection validation, then removes the
unindexed-file population. Each has a focused `FAIL:` assertion: respectively
`test_native_caller_is_not_the_syntax_walk_pair`,
`test_dynamic_call_on_same_line_cannot_attest_unexecuted_literal`,
`test_total_trace_byte_cap_plus_one`, and
`test_omitted_documents_are_unattested_even_when_local_blind_share_is_small`.
The committed driver restores each mutant in `finally` and reruns green.
The final suite passes **16 tests**. Full output is kept privately at
`workspace/artifacts/lang-php/mutation.log`.

Caps: `python3.13 -B -m unittest discover -s calibration/oracles/php-candidates -p test_trace_caps.py`
passes **12 tests**. Each resource cap has a reduced-limit cap+1 case: trace
files 2/1, reflection files 2/1, total/per-file trace bytes and reflection JSON
bytes (each actual input size / size minus one), a 34-byte line / 33-byte limit,
and native stack depth 3/2. Each asserts whole-unit refusal, `refusal.json` and
absence of even a stale `oracle.json`. Truncation, footer text inside a payload,
timeout and signal also refuse; a completed failing suite remains disclosed.
Production limits are 64/64 files, 8 GiB total trace, 2 GiB per trace, 64 MiB
reflection JSON, 1 MiB per line and depth 4096. Full output is kept at
`workspace/artifacts/lang-php/caps.log`. The collector no longer emits partial
tier-A truth when collection is incomplete or over a cap.

Post-integration handoff checks pass: focused `Queue|Plan|Disposition|Rest`
tests under `-race`, release-notes `-check`, merge-debris in working-tree and
staged modes, and `TestNoOrgSpecificNames`. The canonical reading remains
unchanged; these supplementary measurements establish the counted limits above.
All binding review findings are addressed: the three producers and actual
runtime route are measured, Phan/phpactor are named, OL-R7 is identified, and
the assessment index preserves the integrated sibling rows with normal spacing.
