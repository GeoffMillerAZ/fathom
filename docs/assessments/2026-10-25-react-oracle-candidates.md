# React oracle candidates — 2026-10-25

**Measured:** the [existing reading](../bench/readings/lang/react.json) remains
unchanged: component/prop recall .9658 and precision 1, render/hook and file-edge
precision/recall 1, with the tier-C population cap. The reopened search retains
LB-Q-react-rest's present target ceiling for the candidates below. The native
compiler selects functions, not the full class-component/declared-prop population;
a typed TypeScript graph alone does not attest those React roles. This is a scoped
candidate conclusion, not a claim that no stronger instrument can ever exist.

## Named candidates and concrete outcomes

| Candidate / route | Version | Check and why it does not lift the target today |
|---|---|---|
| react-docgen-typescript + legacy TypeScript checker | 2.4.0 + 5.9.3 | Existing corpus-measured oracle. getComponentInfo/getPropsInfo choose a heuristic population; the checker resolves its edges but does not make that selection native React truth. Tier C. |
| react-docgen | 8.0.4 | Executed exported/all-definition resolvers on the committed contract probe. Exported finds class/function, all additionally finds hidden function. Babel documentation-discovery rules supply the population; no React evaluation or complete semantic role inventory. Independent parser C. |
| React Compiler Babel plugin, infer | 1.0.0, @babel/core 7.28.4 | Executed: selects FunctionComponent and HiddenComponent, not ClassComponent. The absent class alone prevents a complete required-kind population; no declared-props export is supplied. |
| React Compiler annotation / syntax / all routes | 1.0.0 | Annotation executes with no selected function on unchanged unannotated input; all additionally selects Utility but still not the class. Vendor syntax mode requires Flow component/hook syntax, not these TypeScript subjects. Changing annotations/input would change the contract. |
| TypeScript native checker / AST transport | 7.0.2 | Installed current release and executed legacy-API probe: root createProgram/createCompilerHost are undefined; docgen2.4.0 fails reading the absent legacy React JSX enum. Its new unstable APIs are a possible typed-TS transport, not an independently supplied React role population. API incompatibility does not prove a new bridge impossible. A compiler syntax dump alone is at most B, which cannot lift L2/L3. |
| scip-typescript | 0.5.2 (public package/release contract inspected) | Native typed JavaScript/TypeScript occurrence index is A for that language contract, but SCIP class/function/property kinds do not independently identify all React components, props and hooks. Reusing docgen's role filter inherits its C population cap; no React-specific producer is claimed. Not corpus-executed in this recheck. |
| React rendering / react-dom server | React 19.3.1; current server API inspected | renderToStaticMarkup returns HTML for the supplied tree. It cannot inventory unrendered definitions or declared props never passed to that tree. Useful runtime behavior, not complete written component/prop truth. Source-contract check, not execution. |
| React Router route-object evaluator | 6.30.6, existing committed route recipe | Already A for its separate route-object contract. It does not attest component/props/hooks inventory. Imported path constants, lazy-module names and Next project gating are the existing product/seam gaps, not a stronger graph oracle. |

**Claimed by a vendor:** React documents
[compiler selection modes](https://react.dev/reference/react-compiler/compilationMode)
and [server rendering's HTML result](https://react.dev/reference/react-dom/server/renderToStaticMarkup).
**Measured, source inspection:** the existing
[oracle contract](../../calibration/oracles/react-docgen/README.md) explains its
role rules and the independent TypeScript edge resolution. The checked
[SCIP producer](https://github.com/sourcegraph/scip-typescript) supplies TypeScript
occurrences; the [SCIP schema](https://github.com/scip-code/scip/blob/d556dd3c/scip.proto)
has language-neutral symbol kinds, not a complete React-specific population.

## Reproducible contract probe

**Measured:** the [original fixture](data/react-oracle-2026-10-25/probe.tsx),
[runner](data/react-oracle-2026-10-25/probe.mjs) and
[summary](data/react-oracle-2026-10-25/summary.json) preserve the concrete results.
This tests instrument contracts, not real-corpus accuracy or a new grade. No toy
fixture or holdout enters a canonical shard.

```sh
npm install --prefix workspace/tools/react-candidates --ignore-scripts --no-audit --no-fund \
  @babel/core@7.28.4 babel-plugin-react-compiler@1.0.0 react-docgen@8.0.4 \
  react-docgen-typescript@2.4.0 typescript@7.0.2
nice -n 10 node docs/assessments/data/react-oracle-2026-10-25/probe.mjs \
  --tools workspace/tools/react-candidates --out workspace/tmp/react-replay
```

**Hypothesis:** a complete native React role population would permit a new
comparison, with every class/prop/hook and unresolved site counted. A better B
syntax instrument may help L1 but cannot make target L2/L3 excellent. The other
existing trigger remains separate: shared definition/type bindings and variant
queries for inline props, plus route-seam/project gating. No shared engine work,
level promotion or plan checkbox change is made here.
