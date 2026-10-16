# github-actions oracles

Two instruments for the `github-actions` doc-engine dialect. Neither shares code with fathom's extractor, and
neither is imported by fathom.

| Instrument | Tier | Reads |
|---|---|---|
| `workflow_parser_oracle.mjs` | A (definitions, references); C (`-xref-out`) | GitHub's own `@actions/workflow-parser` |
| `main.go` (the earlier oracle) | C as written; B at most | rhysd/actionlint's workflow AST, a yaml.v3 walk of action.yml |

The reading (`docs/bench/readings/lang/github-actions.json`) is measured with the first. The second stays as a
cross-check: on every file both accept, the two agree entity for entity (over the three corpus subjects, the
reading's 1,031 definitions, 203 needs pairs and 245 local-uses edges, and the out-of-scope generated files too;
2026-10-02).

## Why this is tier A (design §1.3)

§1.3's ladder: the format's reference parser, or the tool's own evaluation, is A; a widely used third-party
parser tool is B; an independent library or walk written against the format is C.

GitHub owns the workflow format. `@actions/workflow-parser` is GitHub's own implementation of it, published from
github.com/actions/languageservices (MIT) and run by GitHub's Actions language server and its VS Code extension.
Its README calls it "one of multiple implementations of the GitHub Actions workflow parser", driven by the
workflow schema (`workflow-v1.0.json`) and held to shared test data "that all implementations should pass": it is
GitHub's parser against GitHub's schema, not a reading of the format by a third party. `parseWorkflow` validates
each file against that schema and yields the token tree with the parser's own ranges; `convertWorkflowTemplate`
evaluates it (job ids, `needs` as tokens, the needs graph, which it rejects when a need names no job or makes a
cycle, and reusable-workflow references, which it loads through a file provider). Action metadata goes through
the same package's `parseAction` (the action schema, `action-v1.0.json`) and `convertActionTemplate`.

actionlint, by contrast, is a third-party linter with its own workflow AST: the B rung at most, which proves L1
excellent only (`tierCeiling`). The earlier oracle wrote C because it also read action.yml with a plain yaml.v3
walk; it is not relabelled.

What is not the parser's: nothing in the definitions or references. Every entity, name and line is a parser token
(a job's id key, a need's string token, a step mapping's first line, an action input's value mapping), and the
YAML is read by the package's own reader (eemeli/yaml), not by yaml.v3, so no base library is shared with the
dialect. What this program decides is only which files to give the parser (GitHub's rule: `*.yml`/`*.yaml`
directly under `.github/workflows`, and every `action.yml`/`action.yaml`) and the entity vocabulary the dialect
records (a `target` per workflow or action name, a `job`, a `step` with a written id, a positional `step`
(`jobs.<job>.steps#<index>`, `runs.steps#<index>`) for a run step without one, a `resource` per step `uses` and
one more, `pin:<uses>`, per remote action a file uses (GitHub's `{owner}/{repo}[/{path}]@{ref}` form, at its
first step; K20, the dialect's pin nodes), a `variable_decl` per action input and an `output` per action output,
each input or output whose value is a mapping).

A file the parser rejects is attested by nothing: it contributes no entity, is named on stderr, and is left out of
`-files-out`, which the reading passes to `lang-oracle -only` (pytorch's `.github/actions/linux-test/action.yml`,
rejected for two composite run steps without the required `shell`, an expression that names `secrets`, and no
`description`). A need the converter rejects is not a reference and is counted (`needsRejected`).

## Why the cross-references stay tier C

`-xref-out` writes a local `uses: ./path` at file granularity. A job's reusable workflow is the converter's own:
`parseFileReference` reads `./` from the repository root and the converter loads and parses the file
(`fetchReusableWorkflowDepth: 1`, offline: a remote reference is not fetched and is counted). A step's
`uses: ./dir` is not: the parser does not resolve actions, and the rule that a directory means its `action.yml`,
else `action.yaml`, is GitHub's documented runner rule applied by this program (it keeps only a metadata file
`parseAction` accepts). No offline GitHub tool evaluates it, so the file is tier C: L3 stays good
(LB-Q-github-actions-rest).

## actionlint's own resolution (LB-F457, wave lf8)

The row asked whether actionlint's OWN resolution, not this program's `uses: ./dir` rule, attests the file a local
`uses:` reaches. `main.go -resolve-out <file>` (resolve.go) asks actionlint v1.7.12's two caches over a Project rooted
at the subject: `LocalActionsCache.FindMetadata` for a step's `./dir` (it reads `dir/action.yaml`, then
`dir/action.yml`, and parses the file into its `ActionMetadata`) and `LocalReusableWorkflowCache.FindMetadata` for a
job's `./path` (it reads the file and accepts it only when its `on:` names `workflow_call`). Measured on the reading's
scope (pytorch v2.8.0's `.github`, the only subject with local uses; 245 edges, 170 reusable workflows and 75 step
actions), it attests 225: all 170 reusable workflows and 55 of the 75 step actions, every one equal to the reading's
truth (no edge of its own). The other 20 reach three composite actions (`filter-test-configs`, `chown-workspace`,
`upload-utilization-stats`) whose metadata actionlint refuses ("unexpected key \"type\" for definition of input")
and GitHub's `parseAction` accepts. No directory of the subjects holds both metadata files, where actionlint's order
(`action.yaml` first) and the runner's (`action.yml` first) differ. A composite action's own steps are not linted by
actionlint, so their `uses` values come from the yaml.v3 walk and only their resolution is actionlint's.

What it attests is therefore actionlint's reading, tier C (an independent parser library, owner decision D4), not
GitHub's evaluation: it cannot lift the family to A. No tier-A attestation covers either L3 family (a step's local
action; a run step's first command, LB-F310, which nothing evaluates), so the reading carries the written ceiling, a
`disclose` note beginning `L3 ceiling: ` that `-xref-out` writes for every subject with a local-uses edge
(`CEILING` in workflow_parser_oracle.mjs; LB-Q-github-actions-rest holds the trigger).

    (cd calibration/oracles/github-actions && go build -o <bin>/gha-actionlint . && go test ./...)
    <bin>/gha-actionlint -subject <checkout> -resolve-out <scratch>/gha-<subject>-actionlint.json > /dev/null

## Run

```sh
npm_config_cache=<cache> npm --prefix <tools> install @actions/workflow-parser@0.3.61
node calibration/oracles/github-actions/workflow_parser_oracle.mjs -tools <tools> -subject <checkout> \
  -xref-out <scratch>/gha-<subject>-xref.json -files-out <scratch>/gha-<subject>-files.txt > <scratch>/gha-<subject>.json
GHA_ORACLE_TOOLS=<tools> node --test calibration/oracles/github-actions/
```

Pass both JSON files to `lang-oracle -tool-oracle`, and `-only` the accepted files when the oracle rejected one.
The package is published for bundlers (its schema is a JSON import without an import attribute), so the program
registers a Node load hook that serves the package's own `.json` files unchanged.

Versions measured (2026-10-02): node 26.10.0; @actions/workflow-parser 0.3.61 (tarball sha256
fc0c4af6cc1f6c8a6f68e47e2093fc826a922c7244a0ffd2f8030a9303a59877), @actions/expressions 0.3.61
(a09137eed52067f4369a5fdff1e2beb3663a1e9388a4709a414ada2dd44c82cd), yaml 2.9.1
(4ef6c54cf559b8a207b7b518378230805a1c84af239b14960e8c67c7d59de5d3), cronstrue 2.59.0.
