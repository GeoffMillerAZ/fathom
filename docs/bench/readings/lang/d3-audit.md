# D3 adjudication audit (2026-09-28)

**Verdict: the first D3 sets were inflated; the admission rule is tightened and the
readings re-measured. TypeScript drops from L1 excellent to L1 good, JavaScript from
L1 excellent to no level met.**

Rule audited: an oracle-miss admission is valid only when the oracle genuinely failed to
report a real top-level or member definition of a scored kind — never a dialect's
over-extraction the oracle deliberately excludes (locals, parameters, object-literal keys,
re-exports, duplicates).

Method: a seeded random sample (`random.seed(20260928)`) of 45 admitted entries per
language from the readings at `24c3cc17`; each file:line opened in the calibration subject
at its pinned SHA (zod `1fb56a5c`, nest `9ff83d54`, axios `cdcfd214`, express `df0abc93`)
and classified (a) a real module/member definition the oracle missed, (b) over-extraction
the oracle excludes, (c) duplicate or misattributed.

| Language | Admitted | Sample | (a) | (b) | (c) | Invalid |
|---|---|---|---|---|---|---|
| TypeScript | 769 | 45 | 14 | 29 | 2 | 69% |
| JavaScript | 248 | 45 | 32 | 13 | 0 | 29% |

Invalid examples: nest `packages/core/injector/abstract-instance-resolver.ts:60`
`pluckInstance` (arrow const inside a method); zod `packages/zod/src/v4/classic/schemas.ts:411`
`includes` (object-literal key passed to `_installLazyMethods`); nest
`integration/scopes/e2e/transient-scope.spec.ts:333` `constructor` (a local class's
constructor, claimed as object-literal-method); axios `lib/helpers/composeSignals.js:13`
`onabort`; express `test/app.render.js:270` `View.prototype.render` assigned inside a test.

Change (`cdae696c`): a class must name a scope, and `module` is the only one — the
definition's line starts at column 0 and no definition fathom emitted encloses it. The
object-literal-method class is withdrawn.

Re-audit of the tightened sets (seed 7, 40 per language): TypeScript 0 of 40 invalid (all
module-level arrow-function consts); JavaScript 1 of 40 (2.5%: express
`lib/router/index.js:43`, `var proto = module.exports = function` named `exports`).

| Language | Admitted | sym_precision | L1 |
|---|---|---|---|
| TypeScript | 769 → 234 | 0.9946 → 0.9381 | excellent → good |
| JavaScript | 248 → 188 | 0.9908 → 0.8368 | excellent → unmet |
