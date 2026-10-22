# Game development support: interim release readiness

The integrated changes reduce cold verification/context allocations, stop cancelled review work, and cap complete review results at 16,000 bytes. They also add a repeatable cold-arm harness and derived Unity assembly facts. They do not yet establish a release ready for this consumer: the 512 MiB cold gate failed, source-window and doctor fixes remain outside this snapshot, and all 35 consumer families are **not measured**. Native Unity, platform and host acceptance remain the consumer's work after upstream qualification.

This is an interim upstream inventory, not a release approval, consumer grade or GDEV completion claim. Source: local `origin/main` at `e31b5d71c898ff279bd1acf537221c16307a224f` (also this report's pre-edit HEAD). History inspected with `git log --oneline 47790a988..origin/main` and every fragment in `docs/changelog/unreleased/`. No remote fetch or consumer-corpus run was performed. [The design](../design/game-development-support.md) supplies the targets and independent witnesses. ADR-048/I54 require independently measured levels, ADR-067/I57/I65 retain evidence tiers and unknown residue, and I38 prevents blind reads from looking complete. This report changes no policy or decision and establishes no revisit trigger.

## P0 status

Merged implementation is distinguished from complete qualification. Commit identifiers below are on the inspected `origin/main`. These statuses do not flip plan rows.

| Plan row | Release status | Merged evidence and remaining qualification |
| --- | --- | --- |
| GDEV-01 | Harness merged; cold gate failed | `420cc5243` merges `8cdc3ee91` and `c6623a73e`. [Harness](../../scripts/gdev/README.md), [entry point](../../scripts/gdev/cold-arm.sh), [change note](../changelog/unreleased/gd-coldarm.md). Controller receipt **gdev01-arm-base-1**, fathom **47790a988**: fathom-on-fathom arm at **512 MiB** failed at ingest with an **873 MB sampled peak**. This is a baseline failure, not a candidate result or an estimate. The raw receipt is controller-held and is not copied into this checkout. No two complete passing arms are established. |
| GDEV-02 | Memory improvements merged; bounded-ingest acceptance open | `88e5b4719` merges `f3b125206`, with scope/fake corrections `1fc515e33` and `ef62a06b7`. [Change note](../changelog/unreleased/gd-memory.md): demand-driven verification/context graphs and scoped symbol projection. Allocation improvements do not prove physical peak, bounded ingest or a passing 512 MiB cold arm. |
| GDEV-03 | Review cancellation implementation merged; consumer replay open | `bfc5d0dd9` merges `f073c1180`, `8ab516c2e`, `6d6c5559d`, `43e860685` and `27fec6ffc`. [Change note](../changelog/unreleased/gd-review.md), [phase cancellation tests](../../internal/mcp/review_cancel_test.go) and [admission controls](../../internal/mcp/admission_test.go). Bounded reads, separate citation/revalidation windows, write fences and cancellation are implemented. This report does not rerun the full review suite or establish current consumer replay acceptance. |
| GDEV-04 | Whole-result cap merged; consumer envelope replay open | `11ec61f3d` merges `6e3276924` and `8683baace`, followed by change-note correction `e7ae09f64` and exclusion control `4baceb920`. [Change note](../changelog/unreleased/gd-budget.md), [whole-envelope tests](../../internal/mcp/review_budget_external_test.go), [metadata/refusal tests](../../internal/mcp/review_budget_test.go). The cap covers text, structured content and metadata together, preserves all eight status identities/counts and small-result byte parity, and names omissions/refusals. No new full consumer receipt is supplied here. |
| GDEV-05 | in review, not in this release | No source-window integration commit in the inspected range. Historical main-0001/0002/0008/0010/0011/0012 describe proposed work, not current-main acceptance. Dirty, refresh-race and unknown-identity source-window controls remain unqualified. |
| GDEV-06 | in review, not in this release | No module-wrapper/indirect-hook doctor fix in the inspected range. Existing [preflight root deadline fix](../changelog/unreleased/preflight-root-deadline.md) concerns preflight and does not establish honest doctor execution detection. |
| GDEV-07 | Open; no complete P0 control verdict | The [historical dossier](../contributions/game-development-support/README.md) retains cohort-specific partial success, refusal and incomplete-gate observations. Those are not current negative/replay receipts for all five P0 failures. The baseline failure above supplies no missing source-window or doctor verdict. |
| GDEV-08 | Admission/cancellation controls present; complete qualification open | Existing [admission controls](../../internal/mcp/admission_test.go) cover named 503/refusal, panic slot release and disconnect cancellation. Review-specific implementation is merged under GDEV-03. This inventory runs no new end-to-end MCP admission replay and claims no consumer slot-leak verdict. |

Unity groundwork is also merged: `e31b5d71c` merges `76a7a1b1c` and `cc3e1ee55`. [Derived ownership](../changelog/unreleased/gd-unity.md) and [folder/refresh corrections](../changelog/unreleased/gd-unity-2.md) contribute to GDEV-10 without completing native CompilationPipeline calibration.

## 35-family summary

GDEV-09 baseline matrix: interim, upstream-only, consumer not measured

The [JSON companion](game-development-support.json) contains exactly 35 records in design-matrix order. `upstream_quality` quotes the current tracker support cell. `upstream_level` is its recorded achieved level, or the explicit tolerated/unsupported/no-reading disposition. Calibration subjects, source commits, dates, oracle pins, metric values and nulls belong to the cited upstream reading, not to this release snapshot or the consumer. Quoted upstream grades are preserved without re-scoring their raw metrics. A full release reading still needs same-source consumer closure, denominators, failures, unknown residue, physical peak and response bytes.

`kind` is the inventory classification (language or tool format), because the design does not have a separate kind column. Local oracle availability means the named machinery is accessible, not that calibration passed. The maintainer machines have no qualified dotnet, Unity, Blender, Windows, Python 3.13 or browser automation for these readings. Available tools still need exact consumer inputs and pins. Specific concerns/layers remain unsupported in the tracker even though native evaluation machinery exists. Generic JSON/YAML/TOML and Unity assembly registration is only tolerated and has no measured calibration subject.

| Family | Target | Upstream quoted support and calibration subject | Consumer level | Named witness available here | Plan rows |
| --- | --- | --- | --- | --- | --- |
| csharp | L4 | [L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent)](../bench/readings/lang/csharp.json); App-vNext/Polly; JamesNK/Newtonsoft.Json | not measured | no: Unity 6000.3.15f1 Roslyn projects, compatible SCIP and native execution, each separately pinned | GDEV-10, GDEV-11 |
| python | L4 | [L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent)](../bench/readings/lang/python.json); fastapi; flask | not measured | no: Python 3.13 venv/Pillow and Blender 5.2.2 embedded Python; interpreter/import and native API checks | GDEV-12 |
| javascript | L4 | [L4 (L1 excellent, L2 excellent, L3 excellent, L4 good)](../bench/readings/lang/javascript.json); openlayers/openlayers | not measured | no: Pinned Node/browser compiler identity, Unity globals and embedded source ranges | GDEV-13 |
| cue | L4 | [L3 (L1 excellent, L2 excellent, L3 good)](../bench/readings/lang/cue.json); GeoffMillerAZ/fathom; cue-lang/cue; monogon/NetMeta | not measured | yes: Native CUE loader, imports, evaluator and LSP reference oracle | GDEV-14 |
| html | L3 | [L3 (L1 excellent, L2 excellent, L3 excellent)](../bench/readings/lang/html.json); bootstrap; html5-boilerplate; startbootstrap-sb-admin-2 | not measured | no: Template expansion, native DOM/link parse and actual browser DOM | GDEV-15 |
| css | L3 | [L3 (L1 good, L2 excellent, L3 excellent)](../bench/readings/lang/css.json); gitea; normalize.css; pico | not measured | no: Selector/link parse and computed style in actual browser DOM | GDEV-15 |
| shell | L3 | [L3 (L1 excellent, L2 good, L3 good)](../bench/readings/lang/bash.json); bats-core/bats-core; nvm-sh/nvm | not measured | yes: Declared Bash/Zsh dialect, working directory, quoting, exit/failure cases | GDEV-16 |
| powershell | L3 | [L3 (L1 excellent, L2 good, L3 excellent)](../bench/readings/lang/powershell.json); PowerShell/PSScriptAnalyzer; pester/Pester | not measured | no: Native AST, command/import resolution, real Windows invocation | GDEV-16 |
| nix | L3 conditional | [L3 (L1 good, L2 good, L3 good)](../bench/readings/lang/nix.json); mitchellh/nixos-config; nix-community/home-manager | not measured | no: Generated CUE drift plus exact Nix execution consumer once plane activates | GDEV-17 |
| json | L1 | [tolerated](../languages.md); no calibration subject | not measured | yes: Native decoder; domain references are separately qualified | GDEV-18 |
| yaml | L1 | [tolerated](../languages.md); no calibration subject | not measured | yes: Native decoder; domain references are separately qualified | GDEV-18 |
| toml | L1 | [tolerated](../languages.md); no calibration subject | not measured | yes: Native decoder; domain references are separately qualified | GDEV-18 |
| csv | L1 | [no upstream reading](../languages.md); no calibration subject | not measured | yes: Decoder, ledger schema/identity and source consumer | GDEV-18 |
| markdown | L3 | [L3 (L1 excellent, L2 good, L3 good)](../bench/readings/lang/markdown.json); facebook/docusaurus; rust-lang/book | not measured | yes: Headings/anchors/links against repository link consumer | GDEV-19 |
| task | L3 | [L3 (L1 excellent, L2 excellent, L3 excellent)](../bench/readings/lang/taskfile.json); Runtime-Radar/runtime-radar; go-task/examples; go-task/task | not measured | yes: Actual Task include, dependency and command expansion, including generated include names | GDEV-20 |
| npm | L3 | [L3 (L1 excellent, L2 excellent, L3 excellent)](../bench/readings/lang/package-json.json); babel; next.js | not measured | yes: Package scripts, package-lock graph, versions and integrity under npm loader | GDEV-20 |
| msbuild | L3 | [L3 (L1 excellent, L2 excellent, L3 excellent)](../bench/readings/lang/msbuild.json); App-vNext/Polly; dotnet/aspnetcore | not measured | no: Generated Unity project/source/reference evaluation and solution producer | GDEV-21 |
| nuget | L3 | [no upstream reading](../languages.md); no calibration subject | not measured | no: NuGet source policy and restore/reference evaluation | GDEV-21 |
| actions | L3 | [L3 (L1 excellent, L2 excellent, L3 good)](../bench/readings/lang/github-actions.json); actions/starter-workflows; cli/cli; open-telemetry/opentelemetry-demo; pytorch/pytorch | not measured | no: Workflow entities, invoked scripts and hosted run receipt | GDEV-22 |
| dockerfile | L3 | [L2 (L1 excellent, L2 excellent)](../bench/readings/lang/dockerfile.json); docker-cli; docker-compose; docker-library-python | not measured | no: Exact Dockerfile.context build and CUE image declaration/drift | GDEV-22 |
| lefthook | L1 | [L1 (L1 good)](../bench/readings/lang/git-hooks-config.json); evilmartians/lefthook; pre-commit/pre-commit-hooks | not measured | yes: Native hook loader and actual pre-push to Task to fresh strict verify | GDEV-23 |
| fathom-config | L2 | [L2 (L1 good, L2 good)](../bench/readings/lang/fathom-config.json); GeoffMillerAZ/fathom; GeoffMillerAZ/fathom (third_party/agentic-kit); fathom-config fixture: layered; fathom-config fixture: profiles | not measured | yes: Fathom config loader and resolved project-local references | GDEV-24 |
| fathom-concern-layer | L2 | [unsupported (disclosed)](../languages.md); no calibration subject | not measured | yes: Native concern/layer evaluation, unevaluated rules disclosed | GDEV-24 |
| fathom-plan | L3 | [L3 (L1 good, L2 good, L3 good)](../bench/readings/lang/fathom-plan-rows.json); GeoffMillerAZ/fathom | not measured | yes: Plan/context/dependency resolver and independent claim verification | GDEV-25 |
| domain-schema | L3 | [no upstream reading](../languages.md); no calibration subject | not measured | no: Each exact domain loader/schema, path/hash/ID links and invalid fixture | GDEV-26 |
| unity-assembly | L3 | [tolerated](../languages.md); no calibration subject | not measured | no: CompilationPipeline path/GUID ownership, platform/define/editor/player negatives | GDEV-10 |
| unity-serialized | L3 | [no upstream reading](../languages.md); no calibration subject | not measured | no: AssetDatabase/SerializedObject GUID, fileID, UnityEvent, prefab variant and subasset links | GDEV-27 |
| asset-manifest | L3 | [no upstream reading](../languages.md); no calibration subject | not measured | no: Maintained manifest validator, source/output hashes and engine import receipt | GDEV-28 |
| model-media | L3 catalog | [no upstream reading](../languages.md); no calibration subject | not measured | no: Manifest ownership, Blender/Unity/Pillow/ffprobe dependencies and receipts | GDEV-29 |
| nunit | L3 | [no upstream reading](../languages.md); no calibration subject | not measured | no: Source-bound Unity XML, parameterized identity and stale/mixed/replay refusals | GDEV-30 |
| cli-mcp-interface | L3 | [no upstream reading](../languages.md); no calibration subject | not measured | no: Exact binary/schema pins, real CLI and Codex/Claude MCP handshakes/calls | GDEV-31 |
| xml | not claimed | [not claimed](../languages.md); no calibration subject | not measured | no: Generic XML has no dialect grade; MSBuild/NuGet/NUnit have named consumers | GDEV-32 |
| text-record | not claimed | [not claimed](../languages.md); no calibration subject | not measured | no: License, prompt, ignore or policy file under its actual consumer | GDEV-32 |
| archive | not claimed | [not claimed](../languages.md); no calibration subject | not measured | no: Source lock, license and integrity receipts; contents are not a language grade | GDEV-32 |
| svg-interface | not claimed | [not claimed](../languages.md); no calibration subject | not measured | no: Browser DOM/render and embedded markup references, no dedicated SVG grade | GDEV-32 |

GDEV-32 not-claimed disposition: xml, text-record, archive and svg-interface remain not claimed.

Their actual consumers are explicit: XML uses named MSBuild/NuGet/NUnit consumers and has no generic dialect grade. Text records use the actual license, prompt, ignore or policy consumer. Archives require source lock, license and integrity receipts without a contents language grade. SVG interfaces require browser DOM/render and embedded markup references without a dedicated SVG grade. Their consumer level remains not measured.

## Consumer acceptance commands

Replace every `<PLACEHOLDER>` with the consumer's own identified input. These commands are a pending acceptance procedure, not evidence they ran. Use a new output directory outside the corpus for each cold arm, retain the same binary/corpus/budget, and preserve full responses and refusals. Never trim the corpus, warm an arm or raise the budget to obtain a pass. See [harness semantics](../../scripts/gdev/README.md): harness exit 0 alone is not budget proof, and sampled peaks can miss spikes. Use the consumer's 15-plan list and every authored ID from those plans (152 in the historical design cohort). If the current corpus differs, record the actual count and closure rather than silently selecting 152 IDs. The shipped Fathom plan/ID lists describe a different upstream subject.

Build the reviewed release from its exact tag or SHA in an existing source checkout with dependencies already provisioned. Do not overwrite a running daemon executable. A separately installed release binary may instead supply `<RELEASE_BINARY>` after its version/hash are recorded. `scripts/gdev` must come from the same reviewed release source.

```bash
cd '<RELEASE_SOURCE_CHECKOUT>'
git checkout --detach '<RELEASE_TAG_OR_SHA>'
git rev-parse HEAD
git status --porcelain=v1
go build -p 4 -o '<NEW_RELEASE_BINARY>' ./cmd/fathom
'<NEW_RELEASE_BINARY>' --version
python3 -c 'import hashlib; print(hashlib.sha256(open("<NEW_RELEASE_BINARY>", "rb").read()).hexdigest())'
```

Run the two complete cold arms separately. Each invocation performs cold daemon ingest, strict verification of all listed plans, four CLI reads, MCP initialize/enumeration and four raw MCP calls. It isolates state and binds port 0. Keep `arm.json`, raw stdout/stderr, requests, samples and source/tool identities from each output. Evaluate actual sampled physical peak against 536870912 bytes, refusals, admission, truncation, errors and unknowns separately from memory estimates and sequence completion.

```bash
cd '<RELEASE_SOURCE_CHECKOUT>'
bash scripts/gdev/cold-arm.sh --subject '<CONSUMER_REPO>' --service '<SERVICE_ID>' \
  --plans '<CONSUMER_PLAN_LIST>' --ids '<CONSUMER_ID_LIST>' --budget 536870912 \
  --out '<NEW_ARM_1_DIR>' --fathom '<RELEASE_BINARY>'
bash scripts/gdev/cold-arm.sh --subject '<CONSUMER_REPO>' --service '<SERVICE_ID>' \
  --plans '<CONSUMER_PLAN_LIST>' --ids '<CONSUMER_ID_LIST>' --budget 536870912 \
  --out '<NEW_ARM_2_DIR>' --fathom '<RELEASE_BINARY>'
python3 -m json.tool '<NEW_ARM_1_DIR>/arm.json'
python3 -m json.tool '<NEW_ARM_2_DIR>/arm.json'
```

For host checks, use a separately provisioned local acceptance daemon and its actual `<LOOPBACK_BASE_URL>` (scheme, loopback address and allocated port, no `/rpc` suffix), with this release binary and the current consumer already indexed as `<SERVICE_ID>`. The cold-arm daemons stop at the end of each arm. Do not register a stopped arm endpoint or edit global settings. Preserve existing project configuration and use a new `<PROJECT_SERVER_NAME>` for this acceptance run.

First inspect the installed clients and the existing project configuration. Merge the following server entry into `<CONSUMER_REPO>/.codex/config.toml`, preserving every other entry. Project trust is an existing operator decision, not a reason to change sandbox policy. If the project is untrusted, record the check as unavailable rather than editing global trust.

```toml
[mcp_servers.<PROJECT_SERVER_NAME>]
url = "<LOOPBACK_BASE_URL>/rpc/agent"
```

```bash
cd '<CONSUMER_REPO>'
codex --version
codex mcp list --json
codex mcp get '<PROJECT_SERVER_NAME>' --json
'<RELEASE_BINARY>' doctor --harness codex --path '<CONSUMER_REPO>' \
  --service '<SERVICE_ID>' --url '<LOOPBACK_BASE_URL>' --json
```

Verify the returned Codex URL against the project entry. The commands inspect configuration and doctor evidence; they do not prove that Codex completed MCP calls. Record unsupported client syntax or unavailable execution explicitly. [Local Codex adoption reference](../adoption/codex.md).

Claude Code's project-scoped command writes the local `.mcp.json`. Use a previously unused server name and inspect the existing file first. Preserve all other servers and existing credentials.

```bash
cd '<CONSUMER_REPO>'
claude --version
claude mcp add --transport http --scope project '<PROJECT_SERVER_NAME>' '<LOOPBACK_BASE_URL>/rpc/agent'
claude mcp list
claude mcp get '<PROJECT_SERVER_NAME>'
'<RELEASE_BINARY>' doctor --harness claude --path '<CONSUMER_REPO>' \
  --service '<SERVICE_ID>' --url '<LOOPBACK_BASE_URL>' --json
git diff -- .codex/config.toml .mcp.json
```

Project-only registration is not a host qualification verdict. Retain initialize, tools/list and bounded tools/call responses from **each actual host**, including schema/version, arguments, complete ToolResult bytes, errors, cancellations and named refusals. The cold arms supply raw protocol controls but cannot establish host-client execution. This report launches no model-backed host sessions. Any host check requiring one remains pending for an authorized consumer run. Doctor's uncertain/module-wrapper/indirect-hook results remain open under GDEV-06 and must be inspected rather than treated as proof. [Claude adoption reference](../adoption/claude-code.md), [doctor exit/status contract](../usage/doctor.md).

## Known limits

- **Unity inner-Assets-folder case:** the derived project-root detector treats each path segment named `Assets` as a possible project root. An inner `Assets` folder within an existing project's asset tree can select the wrong root/ownership boundary. A nested Unity project below a repository root and an inner asset folder are different cases; the latter is not qualified by the merged nested-project controls.
- **Metadata freshness:** Unity metadata refresh needs a full ingest. Incremental refresh preserves declaration identities and discloses that requirement; it does not recompute complete assembly ownership.
- **Conditional semantics:** platform and define rules are recorded, not evaluated for a selected build. Derived editor/player labels do not establish compilation under the consumer's actual platform, symbols or package closure.
- **Native ownership:** CompilationPipeline calibration on Unity 6000.3.15f1 is pending. Repository-derived assembly facts cannot establish native ownership, compilation, callbacks, asset import or test execution.
- **Grade scope:** the historical 205-source/763-test cohort and subsequent 209-source consumer state are different closures. Neither the dossier nor current upstream calibration grades the current consumer. No extra claims cover unused custom shaders, UI Toolkit or Input Actions. Nix remains conditional on its active execution plane.
- **Release gates:** no two passing 512 MiB cold arms, complete current P0 replay pack, same-closure consumer matrix or full release gate is supplied by this docs-only lane. The controller owns inspection, plan verification, candidate approval and publication. No plan row was changed and no unsupported completion marker is emitted.
