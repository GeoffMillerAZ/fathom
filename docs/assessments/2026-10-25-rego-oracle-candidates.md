# Rego oracle candidates — 2026-10-25

**Measured:** [rego.json](../bench/readings/lang/rego.json) remains L1/L2
excellent and L3 good. Native OPA truth is already tier A: import resolution 1,
file-edge precision .9949 and recall .3858 (581/1,506). The oracle names every
file of an imported package; the product emits only files reached through used
rules. This is a product edge shortfall, not a native-oracle availability ceiling.

## Named candidates and concrete outcomes

| Candidate / route | Version / revision | Checked outcome |
|---|---|---|
| OPA reference parser / existing opa-parse adapter | Measured 1.18.2; current release 1.21.4 | Existing native A definitions, written calls and imports. All imported-package files are already in truth, including unused ones. A newer parser cannot make the product emit missing import-statement edges. No fresh 1.21.4 corpus accuracy claim. |
| OPA compiler / eval, including trace and explain | 1.21.4 public command contract | Native evaluation or compiler information can resolve a selected rule/query. L1/L2 are already excellent. Evaluated/used dependency truth is distinct from every file named by an import statement; substituting it would narrow the established L3 truth. |
| OPA deps | 1.21.4 release-matched cmd/deps.go inspected | Requires exactly one query and reports base/virtual document dependencies, not every imported-but-unused package file. Useful for query dependency analysis; not a replacement contract for the missing L3 pairs. |
| OPA capabilities | 1.21.4 public CLI contract | Describes supported built-ins/features, not source call/import occurrences. Cannot supply the missing product edges. |
| Regal lint / LSP / debugger | 0.43.0 release README, OPA1.21.0 dependency badge | Lint emits violations; the server supplies go-to-definition and the debugger evaluates policies. Those are real existing routes, but neither more references nor another import graph adds missing edges to fathom. No claim that Regal has been corpus-executed here, or that its server cannot be adapted. |
| SCIP producer registry | scip-code/scip d556dd3c, inspected 2026-10-25 | No Rego producer in the checked list. This does not prove universal absence. L3 already has native A truth, so a new producer alone would not resolve this measured recall gap. |

**Source inspection:** [committed native oracle contract](../../calibration/oracles/opa-parse/README.md),
[OPA deps 1.21.4](https://github.com/open-policy-agent/opa/blob/v1.21.4/cmd/deps.go),
[OPA CLI](https://www.openpolicyagent.org/docs/cli),
[Regal 0.43.0](https://github.com/open-policy-agent/regal/blob/v0.43.0/README.md) and
[checked SCIP registry](https://github.com/scip-code/scip/blob/d556dd3c/README.md).
This is a versioned source-contract recheck, not a new performance/accuracy run.

**Hypothesis / retry:** retain LB-Q-rego-rest's shared fusion trigger: draw an
import statement's own edges to each file of the named package, then remeasure
with the existing oracle. The old `no Rego SCIP indexer` assertion is narrowed to
the checked registry rather than treated as proof of absence. A new semantic
producer can be assessed independently, with its complete population disclosed;
it must not replace unused-import truth with a smaller used-reference graph.
No shared engine/epoch, canonical corpus, reading, generated file or plan box is
changed in this branch. A shared resolver change needs the controller's ENGINE
serialization and every affected language's regrade.
