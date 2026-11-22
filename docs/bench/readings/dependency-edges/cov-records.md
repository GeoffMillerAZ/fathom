# COV-5 · Dependency coverage records

## Final campaign receipts

All three attempts ended before aggregate publication. The first failed
attempt was retained under ESCALATE-22. The replacement under ESCALATE-23
used the frozen reader, populations and bars after metadata checks,
independent seal review and two byte-identical rehearsals. The last
operation under ESCALATE-24 passed the exact metadata and seal gates, two
byte-identical non-held rehearsals and a separate injected-native-failure
rehearsal. Its new exclusive journal cites both earlier failed journals.

| Attempt | Harness revision | Journal SHA-256 |
|---|---|---|
| Original | `598ec9ded011ded5b93fa04805c07c168226c21f` | `91b36b4a463cfe1fde13b519abc3f8f9cbca5cc3ef02ea77c1c423dce59f8bf7` |
| Replacement | `b67d4fe3e4cc9d324c9cca3c19b699b3c7500132` | `7e191d908b378c0db3e53ae1622c868c02f9e4493679ddb7b71577166039c5be` |
| Last | `e6266dd5d13ec1edc7127696aa78314bd3f13358` | `353c2dfdce19cdbcdc736d8ff1cf1c983f15300cbc5d44ec43ef1a6acd05053b` |

## Program disposition

The last operation records `CalledProcessError` during native collection
in the holdout pool. It published no aggregate. The two final readings
and the separate TypeScript confirmation are unverified. The program
doubling bar is unmeasured. The landed levers retain their development
and closed R5 evidence, and the dialect admission labels stay as recorded.
[COV-M1](cov-m1.md) · [COV-M2](cov-m2.md) · [COV-M3](cov-m3.md)

## Native truth availability

The replacement stopped when a native truth process exited during sealed
collection. Repair of a truth tool on sealed repositories belongs to a
later program. ESCALATE-24 isolates a nonzero native exit to its dialect
and pool. The entire affected dialect pool is ungraded. A reader-side
complete count may still be reported with that label. The other dialects
retain their independent grades, and confirmation stays separate. An
unverified dialect contributes zero toward the program's doubling bar.

The injected rehearsal checked this behavior on frozen development
inputs. Its JavaScript row contained only the exception class, the
1,508-file denominator and 448 ungraded reader completions. Python,
TypeScript and confirmation aggregates matched the normal rehearsal.

## Development ceilings

COV-D4 admits a mechanism when it greedily unlocks at least 2% of the
dialect's development files, or is a demonstrated prerequisite of such a
mechanism. Files with several blockers count toward an unlock only after
all of their blockers are resolved. The bounds below use the frozen
development pool and the reviewed native evidence.

| Dialect | Remaining mechanism | Measured bound | `decidable_by` |
|---|---|---|---|
| Python | Residual dynamic imports, from-lists and runtime hooks | 71 files are blocked solely by `dynamic-import` and lack accepted file truth. | declaration, observation |
| Python | Missing configuration or bindings for imports | 608 files are blocked solely by `missing-config`. Of these, 138 have accepted file truth and eight have a native tracked edge at every blocked site. | config-file, declaration, toolchain |
| JavaScript | Remaining JSDoc type syntax and attachment cases | 185 files are blocked solely by JSDoc. 150 have native local operands on their physical lines across forms. The tested Closure prefix and function forms reach 26 and 13 files, or 40 jointly. Their marginal unlocks are 14 and 27 in the two orders, below 31/1,508. An `@import`-only mechanism reaches at most five. | config-file, observation, toolchain |
| JavaScript | Explicit compiler types | 125 files are blocked solely by configured types and 483 also have located construct blockers. The pinned compiler resolves none of the 608 directives inside the tracked view. | config-file, toolchain |
| JavaScript | Other unresolved aliases, unpinned externals and dynamic imports | These remain observed blockers. Each mechanism requires a separate greedy proof. | config-file, lockfile, observation, toolchain |
| TypeScript | Explicit compiler types and unsupported reference forms | 136 files are blocked solely by explicit compiler types. The configured subpath has no native binding under the pinned tool, so a pinned package version supplies no file truth. | config-file, toolchain |
| TypeScript | Alias imports needing generated declarations or additional configuration | 144 alias-blocked files require untracked generated declarations. In a separate 188-file config-chain group, 21 have native local truth for all unresolved alias operands. | config-file, toolchain |
| TypeScript | Synthetic compiler helper imports and parse errors | The declaration-only helper subset is at most 1/1,224. The pinned compilers can select a helper target even when emitted JavaScript is identical. Parse errors still require an independent accounting of dependency constructs. | config-file, lockfile, toolchain |

The landed Python from-list rule added 102 fully graded development files.
The landed JavaScript JSDoc rule added 295. The conservative TypeScript
locality correction closed its one round-one R5 check and kept development
counts at 250/1,224. The original holdout baselines remain the value
denominators. [Census and projections](cov-census.md)

## Batch input refusal

Paths containing parentheses or square brackets caused three hard input
refusals in the TypeScript holdout. Each class affected two paths with one
overlap. The test harness retains all three files in the population as
incomplete, reports refusals separately and checks the rejected set in B3.
A refused target can block closure. In the public API, one inadmissible
path refuses the whole batch. Per-file recovery is reserved for a separate
decision.
