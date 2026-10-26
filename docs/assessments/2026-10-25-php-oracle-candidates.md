# PHP oracle candidates — 2026-10-25

**Measured:** the [existing reading](../bench/readings/lang/php.json) remains
L1/L3/L4 excellent and L2 good. L2 written-call precision/recall are 1/1 from the
B syntax dump; native scip-php call precision .6924/recall1 has about31% blind
pairs, above the10% cap. No new corpus run or accuracy improvement is claimed.

**Owner ruling (Q6):** custom PHPStan2.3.2 and Psalm7.0.0-rc10 reference exports
remain tier C under ADR-048, following the Lua OL-R2 precedent. No amendment
permits A. Their semantic extension APIs are real candidates, not rejected for
lack of a pre-existing adapter; even complete exports at the ruled tier cannot
lift this L2 target to excellent.

## Named candidates and concrete outcomes

| Candidate / route | Version / revision checked | Concrete outcome |
|---|---|---|
| davidrjenni/scip-php | HEAD71a5b117ec4c, tool_version0.0.1; published release0.0.2 | Fresh upstream HEAD check equals the already measured pin. Existing own-call-site truth omits vendor/builtin/untyped receiver occurrences:1,869 fathom-only pairs,1,258 monolog and611 guzzle. No published source change to remeasure for broader native truth. |
| nikic/php-parser call dump | 5.7.0, existing committed recipe | Native PHP AST syntax dump, B under the established Kotlin-PSI precedent. P/R1 proves written syntax agreement, not resolved-call truth; B cannot make L2 excellent. |
| PHPStan JSON / Rules / Collectors | 2.3.2 release metadata; official extension docs inspected | Standard JSON is diagnostics. Rules/Collectors receive AST nodes and analyzed Scope, so a custom semantic reference export is feasible. Owner Q6 fixes that export at C; not a target-level improvement even if full-population measurement succeeded. No false claim that diagnostics are its only interface. |
| Psalm JSON / analysis plugins | 7.0.0-rc10 release metadata; official plugin docs inspected | AfterExpressionAnalysis, AfterFunctionCallAnalysis and AfterMethodCallAnalysis hooks expose analysis events; a custom semantic export is feasible. Owner Q6 keeps it C. No adapter built or blind sites waived, because that classification cannot lift L2. |
| Standalone PHP runtime route | Static CLI8.5.8 existing recipe; FrankenPHP1.13.3 Darwin ARM64 release asset checked | Lane-local runtime installation is available; lack of system PHP is not a ceiling. A runtime alone does not export a full written-call index, and installing it does not change the PHPStan/Psalm tier ruling. Neither alternative was newly corpus-executed here. |
| Maintained SCIP registry route | scip-code/scip d556dd3c inspected 2026-10-25 | The known PHP producer leads to the unchanged scip-php source above; no additional PHP producer found in that checked list. Scoped registry evidence, not proof that no other producer could exist. |

**Source inspection:** [executed native recipe and blind-site evidence](../../calibration/oracles/scip-php/README.md),
[scip-php source](https://github.com/davidrjenni/scip-php/tree/71a5b117ec4c),
[PHPStan collectors](https://phpstan.org/developing-extensions/collectors),
[PHPStan rules](https://phpstan.org/developing-extensions/rules),
[Psalm plugin events](https://psalm.dev/docs/running_psalm/plugins/authoring_plugins/),
[FrankenPHP1.13.3](https://github.com/php/frankenphp/releases/tag/v1.13.3) and
[checked SCIP registry](https://github.com/scip-code/scip/blob/d556dd3c/README.md).
Current release numbers identify the candidates checked; the older corpus
numbers are explicitly inherited from committed measurements.

**Hypothesis / retry:** retain LB-Q-php-rest for these candidates and the owner
classification. Reopen on scip-php resolving the missing receiver classes, a new
native/maintained producer with complete written-call coverage, or an explicit
tier amendment. The existing blind-site trigger still requires a counted share
under10%; it does not authorize removing31% of truth or changing subjects. No
shared engine/epoch, canonical holdout, reading, generated file or plan checkbox
is changed.
