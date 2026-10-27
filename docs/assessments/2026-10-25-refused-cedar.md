# Cedar: parser candidates and X-R4 source audit

Source audit 2026-10-25; native grammar screens measured 2026-10-26.
**Measured:** SwornSystems v0.12.0 and its current head each leave **1/59
CLI-accepted files in error (1.69%)**, with all 17 schemas parsing and zero
faults. This is below the 5% line (at most 2 files), so **LB-Q-cedar-rest's
grammar retry trigger has fired**. The old general parser refusal is withdrawn.
Plan the tags dialect on this grammar; no ADR-048 native-reader admission is
needed for that route. WASM, extraction accuracy and throughput remain
unmeasured. No corpus or plan checkbox changed.

Existing measurement: [Cedar oracle README](../../calibration/oracles/cedar/README.md) pins `cedar-policy-cli` 4.13.0 and reports 59/59 accepted files (42 policies, 17 schemas), with source lines recovered by the writer's statement scan, not CLI JSON positions. The [LB-Q-cedar-rest plan row](../plans/language-breadth.md) and [ADR-048 wave-pc amendment](../../ADR.md) record chrnorm/tree-sitter-cedar `5092204d` at 5/42 policy ERROR files, with no schema grammar. Cedar-go v1.8.0 rejects six template files and its schema AST lacks declaration positions. These are limits of the measured candidates at those pins, not a ceiling on all available parsers.

## Five routes checked

1. **Own compiler/runtime dump/introspection.** The official [Cedar repo](https://github.com/cedar-policy/cedar) supplies Rust CLI/SDK and validator. [CLI changelog](https://github.com/cedar-policy/cedar/blob/main/cedar-policy-cli/CHANGELOG.md) documents `check-parse`, `translate-policy`, and `translate-schema`; local 4.13.0 oracle actually exercised those commands. [Schema docs](https://docs.cedarpolicy.com/schema/schema.html) describe Cedar/JSON schema translation. Native Rust reader remains an **admission decision** under ADR-048, not a default product route; seek owner ruling before work on that second reader.
2. **SCIP and LSIF.** [Sourcegraph's recommended SCIP table](https://sourcegraph.com/docs/code-navigation/writing-an-indexer) has 10 language rows, the [SCIP repository's list](https://github.com/scip-code/scip/blob/main/README.md) has 10 named indexers, and archived [LSIF.dev's table](https://lsif.dev/) has 16 rows; none names Cedar. [Sourcegraph's migration note](https://sourcegraph.com/docs/admin/how-to/lsif-scip-migration) says LSIF upload is converted to SCIP. A public [GitHub repository search for `scip-cedar`](https://github.com/search?q=scip-cedar&type=repositories) returned zero repositories on the check date. Searches for [Cedar policy SCIP indexer](https://github.com/search?q=cedar+policy+SCIP+indexer&type=repositories), [SCIP in cedar-policy/cedar](https://github.com/search?q=repo%3Acedar-policy%2Fcedar+SCIP&type=code), and [LSIF in cedar-policy/cedar](https://github.com/search?q=repo%3Acedar-policy%2Fcedar+LSIF&type=code) found no Cedar-specific indexer in this inspection. This is a bounded searched-list result, not a blanket absence claim.
3. **Language servers.** The official Cedar workspace lists `cedar-language-server`; inspected [workspace Cargo manifest](https://github.com/cedar-policy/cedar/blob/f25dcca001788ffaa1b74871a65d97052b1653b2/cedar-language-server/Cargo.toml) blob `05152004fab5` calls it **0.0.1** and pins Cedar core/policy/formatter **4.14.0**. This workspace package is not listed by the crates.io search below. Separately, official [vscode-cedar](https://github.com/cedar-policy/vscode-cedar) documents policy/schema validation, outline/breadcrumb navigation, and go-to-definition for entity types and actions on `*.cedar`/`*.cedarschema`. Its [package.json](https://github.com/cedar-policy/vscode-cedar/blob/e592133f33e4e9d2dd93ea7ee2e003e7e4e28e66/package.json) blob `63c9d97dff5a` states **0.10.6 preview**, and [changelog](https://github.com/cedar-policy/vscode-cedar/blob/e592133f33e4e9d2dd93ea7ee2e003e7e4e28e66/CHANGELOG.md) blob `886323d2159f` says that version uses Cedar SDK **4.13.0**. The VS Code extension is not assumed to be the same binary as the workspace LSP. Both are inspected, unrun routes; probe LSP definition responses on the 59 files.
4. **Registry parsers/typecheckers/AST dumps.** Actual crates.io search API queries [`cedar-policy-core`](https://crates.io/api/v1/crates?q=cedar-policy-core&per_page=10) and [`cedar-language-server`](https://crates.io/api/v1/crates?q=cedar-language-server&per_page=10) returned the former with **4.14.0** as default and the latter with **zero published crates**. The [`cedar-policy`](https://crates.io/api/v1/crates/cedar-policy) registry entry also reports 4.14.0 as default. An [npm registry search for `@swornsystems/tree-sitter-cedar`](https://registry.npmjs.org/-/v1/search?text=%40swornsystems%2Ftree-sitter-cedar&size=20) identifies the **0.12.0** package. Its [pinned package manifest](https://github.com/SwornSystems/tree-sitter-cedar/blob/112e886c976951095b43ad91929269f9c1cca17d/package.json) gives the SwornSystems repository (formerly DuskSystems) and **MIT OR Apache-2.0** licence; the [grammar manifest](https://github.com/SwornSystems/tree-sitter-cedar/blob/112e886c976951095b43ad91929269f9c1cca17d/tree-sitter.json) lists separate `cedar`, `cedarschema` and `cedarentities` grammars. Release tag `v0.12.0` resolves to `112e886c9769`; inspected head `2e9f10824728`. The [GitHub repository search for `tree-sitter-cedar`](https://github.com/search?q=tree-sitter-cedar&type=repositories) is the repeatable candidate discovery query. The **bench-pinned 4.13.0** [`cedar-policy-core` parser API](https://docs.rs/cedar-policy-core/4.13.0/cedar_policy_core/parser/index.html) exposes policy/template parsing, CST, EST and `Loc` source ranges; [`cedar-policy` 4.13.0](https://docs.rs/cedar-policy/4.13.0/cedar_policy/) exposes `Schema`, `Validator`, `PolicySet`, `Template`. No 4.14.0 corpus run or source-position inspection was done. The CLI JSON translations provide semantic names/references but no source positions in the local writer. The native parse path remains subject to ADR-048 if no grammar passes the policy and schema bar.
5. **Runtime call/require tracing.** [Cedar authorization docs](https://docs.cedarpolicy.com/auth/authorization.html) and [`Diagnostics` API](https://docs.rs/cedar-policy/4.13.0/cedar_policy/struct.Diagnostics.html) expose determining policy IDs (`reason`) and evaluation errors for a request. Such traces witness policies used in a particular authorization decision, not schema declaration or static policy-to-schema edges. Search `Cedar evaluation diagnostics determining policies diagnostics reason policy ids official docs` found these primary sources; no import/require runtime mechanism applies to standalone Cedar policies/schemas as documented.

## Measured grammar candidates and next step

| Candidate | Source licence / pin | Result and disposition |
|---|---|---|
| chrnorm/tree-sitter-cedar | MIT by LICENCE, `5092204d` | Prior 5/42 policy errors; no schema grammar. Superseded as the best known route. |
| cedar-go | Apache-2.0, v1.8.0 `cda92d0d`; inspected head `24e8275d` | Prior six template-file rejections; schema AST lacks declaration positions. Not needed to reopen a native-reader admission now. |
| [SwornSystems v0.12.0](https://github.com/SwornSystems/tree-sitter-cedar/tree/112e886c976951095b43ad91929269f9c1cca17d) | MIT OR Apache-2.0; `112e886c9769`, both licence files checked | **1/42 policies + 0/17 schemas = 1/59**, zero faults. Native parser gate passes. |
| [SwornSystems head](https://github.com/SwornSystems/tree-sitter-cedar/tree/2e9f10824728b13d4c019774b32ca0a883e97f96) | MIT OR Apache-2.0; `2e9f10824728`, both licence files checked | **1/42 policies + 0/17 schemas = 1/59**, zero faults. Generated parsers differ from the release, result agrees. |

Every file of the unchanged subjects was first offered to the independent
cedar-policy-cli 4.13.0. It accepted all 59, with no rejected or dropped files:

| Subject pin | Policies / ERROR or MISSING | Schemas / ERROR or MISSING | Bytes |
|---|---|---|---:|
| cedar-policy/cedar-examples `c251f7d1ad17` | 20 / 1 | 14 / 0 | 49651 |
| cedar-policy/cedar-integration-tests `1999ea249229` | 22 / 0 | 3 / 0 | 9770 |

The pinned grammar ships `cedar/queries/tags.scm`, which captures the policy
`@id` annotation as `@definition.policy`, and
`cedarschema/queries/tags.scm`, which captures namespace, entity, action and
common-type declarations. These cover the proposed L1 definition kinds, but
their extraction accuracy has not been graded. The sole failing policy has an
ERROR inside `@id("template")` at line 2, columns 13-15, on the policy-name
capture path. The cedar-examples policy error-set SHA-256 is
`8b8cafa077a7c2cbdaf3fb8dbd35b3d589bd623126b96ecaad023b2861d2c158`
(1/20); the integration subject has 0/22 policy errors and 0/3 schema errors.

Both runs use tree-sitter CLI 0.27.0 `parse -q --lib-path ... --lang-name
cedar|cedarschema`, each parser compiled with `cc -O0 -dynamiclib`. A ten-second
per-file timeout counts separately as a fault. Clean pinned subject HEADs,
expected reference populations and grammar archive commit IDs are checked.
The committed driver `calibration/oracles/cedar-grammar-screen/screen.py` and
README reproduce the screen. Exact aggregate receipts are
`data/2026-10-26-cedar-{v0.12.0,head}-native.json`; they contain counts and hashes,
not subject file lists or source. Upstream's corpus draws from cedar-examples,
while the integration subject contributes 25 files with no parse errors. This
is calibration coverage, not unseen generalization. Before grading, screen an
independent sample outside the grammar's own test corpus as a holdout, without
adding it to the canonical shard.

The recommendation is to pin/build the policy and schema grammar through the
normal tags/WASM recipe, author the dialect's required definition kinds and
governed-scope references, and grade every unchanged CLI-accepted file against
the independent committed oracle before dispatch or an epoch change. This
native syntax screen claims no product level. The existing plan explicitly
reserves the queue kind/L3 model for the owner: Cedar has no import statements;
its proposed L3 is a policy-to-schema governed-scope edge (`policy_ref`,
`service-name`), not invented import edges. Settle that model before the full
L3 implementation. A native-reader exception is no longer the next decision.
