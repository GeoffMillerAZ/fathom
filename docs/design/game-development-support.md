# Game development support: upstream qualification contract

**Status:** Accepted 2026-10-22 as a qualification program (ADR-079), with the maintainer's order and dispositions in the plan header and the [disposition ledger](../contributions/game-development-support/disposition-ledger.md). Contributed as a proposed design the same day. It does not claim implementation, adoption, release, or a grade for the game corpus. The [consumer dossier](../contributions/game-development-support/README.md) holds the bounded observations, evidence manifest and historical source patch references. The main Fathom maintainer owns integration and may change architecture while preserving the acceptance gates here; the game team later qualifies a released candidate on its current private corpus.

## Intent and boundary

This applies ADR-048 and I54 (a support level is an independently measured reading, not registration), ADR-067 and I57/I65 (file facts have tier and counted unknown residue; heuristic links never gate), I38 (blind reads cannot look complete), and ADR-047 (a responsible tenant). ADR-073 keeps judgment at an optional decision port; provider none remains deterministic. No documented revisit trigger is established by this handoff alone. The 35-family inventory is a named consumer and may justify scheduling work, not automatic promotion of a parked future. Fathom supplies source, dependency, provenance, reachability and evidence facts to development agents. Unity, Blender, browsers, native language tools, Review Kit and people decide compilation, execution, visual quality, playability and fun.

The immediate consumer is a private Unity game at the pinned corpus, but upstream features remain organization-agnostic and must retain their existing support thresholds. Its private coverage inventory defines desired maximum applicable levels, not current upstream grades. Its historical toolchain calibration is not a live grade. The complete target inventory is reproduced below without requiring private repository access. At the source-before-candidate checkpoint, the native corpus had 205 C# sources and 763 tests; the consumer subsequently has 209 C# sources without a matching native rerun. Neither is silently relabeled as the other. A previous raw C# diagnostic had an L4 subgrade while L3 was good and other prerequisites absent; overall L4 excellent was not earned.

## Priority zero: make evidence safe to serve

These five failures must be corrected and measured before expanded semantics or a release claim.

1. **Consumer O13, bounded cold ingest.** Use a current roughly 13k-file, 85k-symbol subject under a 512 MiB physical-footprint budget. From separate cold starts, run two complete arms, each with 15 plans, 152 authored IDs, four CLI requests and four raw MCP requests. Retain complete budgeted responses, identity, exact corpus and host pins, duration, peak physical footprint and separate estimated-memory fields. No corpus cut, cache warming disguised as cold, larger budget or truncated response can make an arm pass. Admission, cancellation and refusal must be explicit.
2. **Consumer O21, review cancellation.** A stopped baseline review must terminate its work after the caller cancels, including across the daemon boundary. A replay proves no continued source scan or leaked governor slot. The ordinary success path and error path remain independently covered.
3. **Consumer O23, whole-result budget.** Apply the 16,000-byte ceiling to the whole MCP ToolResult: text, structured content, refresh metadata and suppressed content together. Preserve all eight status identities and counts, name omissions and keep small-result byte parity. A malformed oversized field cannot bypass accounting.
4. **Consumer O10, freshness.** Return exact source windows only for a proven fresh HEAD and read identity. Dirty-tree changes, a refresh in progress and unknown source identity produce a named refusal or unknown, never a stale exact citation. Cancellation and auto-refresh race cases need adversarial controls.
5. **Consumer O1, honest doctor.** Recognize module wrapper and indirect hook wiring without a false failure; where execution cannot be established, say unknown or timeout. Never convert a reachable file or a timed-out probe into an asserted installed-and-firing verdict.

The consumer IDs above are *consumer observations*, not Fathom main's global O-number namespace. Link their receipts through the dossier. Existing ADR-047 and I38 budgets and floors remain in force.

## Qualification rules

An **excellent** result for a family means its applicable level and every lower prerequisite pass on the same identified source/tool closure, with independent oracle, negative cases, resource gates and an unambiguous output. Preserve existing Fathom precision/recall thresholds; this design supplies no lower threshold. A tool family capped at L1, L2 or L3 cannot be made L4 by analogy. An unsupported or not-claimed target remains explicit. Report raw subgrades separately from the aggregate; never turn a strong subgrade into a whole-family grade. A tool's presence, a parser pass, a successful index, source reachability, runtime tests and human enjoyment are separate facts. Unknown is counted and carried, never converted to zero or success. If the exact compiler/interpreter/consumer is unavailable, that dimension is unqualified.

Each reading is machine-readable: family, kind, applicable target, achieved level and quality, lower-level metrics, oracle tier and independent identity, source SHA/tree and dirty state, tool and package pins, cohort scope and exclusions, samples and denominator, failures, unknown residue, timing and physical footprint, result bytes/cap, and evidence links. Produce all 35 records, including not-claimed families. Publish a reviewed release candidate with migration notes, host-specific interface results and a consumer acceptance pack. The game agent tests the actual current corpus later; upstream cannot grade it by copying historical readings.

## Target matrix

All 35 rows come from the consumer inventory. “Target” is a ceiling or requested applicable level, not earned status. The oracle column names the independent witness or runtime consumer; the plan IDs route work.

| Family | Target | Scope and independent oracle | Plan |
| --- | --- | --- | --- |
| csharp | L4 | Unity 6000.3.15f1 Roslyn projects, compatible SCIP and native execution, each separately pinned | GDEV-10, GDEV-11 |
| python | L4 | Python 3.13 venv/Pillow and Blender 5.2.2 embedded Python; interpreter/import and native API checks | GDEV-12 |
| javascript | L4 | Pinned Node/browser compiler identity, Unity globals and embedded source ranges | GDEV-13 |
| cue | L4 | Native CUE loader, imports, evaluator and LSP reference oracle | GDEV-14 |
| html | L3 | Template expansion, native DOM/link parse and actual browser DOM | GDEV-15 |
| css | L3 | Selector/link parse and computed style in actual browser DOM | GDEV-15 |
| shell | L3 | Declared Bash/Zsh dialect, working directory, quoting, exit/failure cases | GDEV-16 |
| powershell | L3 | Native AST, command/import resolution, real Windows invocation | GDEV-16 |
| nix | L3 conditional | Generated CUE drift plus exact Nix execution consumer once plane activates | GDEV-17 |
| json | L1 | Native decoder; domain references are separately qualified | GDEV-18 |
| yaml | L1 | Native decoder; domain references are separately qualified | GDEV-18 |
| toml | L1 | Native decoder; domain references are separately qualified | GDEV-18 |
| csv | L1 | Decoder, ledger schema/identity and source consumer | GDEV-18 |
| markdown | L3 | Headings/anchors/links against repository link consumer | GDEV-19 |
| task | L3 | Actual Task include, dependency and command expansion, including generated include names | GDEV-20 |
| npm | L3 | Package scripts, package-lock graph, versions and integrity under npm loader | GDEV-20 |
| msbuild | L3 | Generated Unity project/source/reference evaluation and solution producer | GDEV-21 |
| nuget | L3 | NuGet source policy and restore/reference evaluation | GDEV-21 |
| actions | L3 | Workflow entities, invoked scripts and hosted run receipt | GDEV-22 |
| dockerfile | L3 | Exact Dockerfile.context build and CUE image declaration/drift | GDEV-22 |
| lefthook | L1 | Native hook loader and actual pre-push to Task to fresh strict verify | GDEV-23 |
| fathom-config | L2 | Fathom config loader and resolved project-local references | GDEV-24 |
| fathom-concern-layer | L2 | Native concern/layer evaluation, unevaluated rules disclosed | GDEV-24 |
| fathom-plan | L3 | Plan/context/dependency resolver and independent claim verification | GDEV-25 |
| domain-schema | L3 | Each exact domain loader/schema, path/hash/ID links and invalid fixture | GDEV-26 |
| unity-assembly | L3 | CompilationPipeline path/GUID ownership, platform/define/editor/player negatives | GDEV-10 |
| unity-serialized | L3 | AssetDatabase/SerializedObject GUID, fileID, UnityEvent, prefab variant and subasset links | GDEV-27 |
| asset-manifest | L3 | Maintained manifest validator, source/output hashes and engine import receipt | GDEV-28 |
| model-media | L3 catalog | Manifest ownership, Blender/Unity/Pillow/ffprobe dependencies and receipts | GDEV-29 |
| nunit | L3 | Source-bound Unity XML, parameterized identity and stale/mixed/replay refusals | GDEV-30 |
| cli-mcp-interface | L3 | Exact binary/schema pins, real CLI and Codex/Claude MCP handshakes/calls | GDEV-31 |
| xml | not claimed | Generic XML has no dialect grade; MSBuild/NuGet/NUnit have named consumers | GDEV-32 |
| text-record | not claimed | License, prompt, ignore or policy file under its actual consumer | GDEV-32 |
| archive | not claimed | Source lock, license and integrity receipts; contents are not a language grade | GDEV-32 |
| svg-interface | not claimed | Browser DOM/render and embedded markup references, no dedicated SVG grade | GDEV-32 |

Do not add claims for unused custom shaders, UI Toolkit or Input Actions. Generated Nix can qualify only if its execution plane is active. Do not imply binary asset contents were indexed because a manifest, hash or metadata reference was.

## Semantic work and refusal controls

**Unity C#.** Use actual Unity 6000.3.15f1 project generation, language version, conditional defines, reference closure and assembly ownership. Attribute callbacks, events, NUnit and runtime entrypoints have distinct evidence. The Rules and Application layers stay engine-free. Native Roslyn/CompilationPipeline and compatible SCIP are independent witnesses; pin schema/compiler and report mismatches, not a silent union. PE/MVID-to-selected-source association needs a positive and a deliberately wrong-assembly negative. Namespace declaration membership is not an actual-use file dependency. Prove source windows and typed calls at their own tiers. A semantic L4 subgrade does not lift weak L1/L3 prerequisites or compilation/runtime gaps.

**Python, JS and CUE.** Pin Python 3.13 environment and Pillow; separately execute Blender 5.2.2's embedded interpreter. Label dynamic bpy results present, unknown or unavailable; static indexing is not typed operator proof. JS must map embedded ranges to host source and distinguish Node, browser and Unity globals with true compiler identity. CUE must use its native loader/import/evaluator and LSP references, preserve no-egress behavior and retain invalid constraint/import controls. Each language gets false-positive and false-negative samples from its actual toolchain.

**Web, scripts and tool files.** HTML/CSS claims stop at what DOM, link and computed-style controls prove. Bash/Zsh and PowerShell each need dialect, cwd, quoting, command resolution and failure tests; Windows PowerShell execution is a separate host proof. Tool files resolve through their native consumer: Task, npm lock, MSBuild, NuGet, Actions, Dockerfile, Fathom config, concern layers and plans. Generic JSON/YAML/TOML parsing never confers domain-schema L3. Native Unity serialization checks GUID/fileID and prefab/UnityEvent/subasset relationships. Asset manifests and media receipts establish identity and provenance, never art quality. NUnit XML needs current source/test identity, malformed inputs and replay/stale/mixed-source refusals.

**CLI/MCP.** Prove actual initialize, tool enumeration and bounded calls through Codex CLI and Claude project-local registration separately. Preserve user global settings, credentials and sandbox policy. Record missing host access as unavailable. No source or model egress is required. Exact argv, schema, result, error and refusal identities matter more than an “installed” label.

## Decision models

No model is needed to answer source identity, compiler binding, dependency edges, oracle scores, resource limits or evidence completeness. Provider none computes these deterministically and emits unknown when a witness is absent. A future opt-in decision-port question could ask which *already measured* failure family deserves maintainer attention; it could not assert a new code fact, pass a gate or judge whether art or gameplay is good. Its deterministic fallback is priority order P0, then prerequisite dependency order, then measured consumer impact. The operator may opt out through the existing decision-port controls. Measure any future site against that fallback before enabling it; no such site or provider is proposed for this handoff.

## Handoff and acceptance

The maintainer first reconciles the dossier's 27 historical source commits and two separate experimental O23 commits against current main; the patch order is a review map, not a cherry-pick recipe. Redacted diff-only units cannot be applied. Integrate only changes that meet upstream invariants and original test gates. Run focused RED/GREEN controls, the upstream full gate and resource/cold arms, then release with all 35 machine records and migration notes. Only afterward does the game team run its current private corpus, native Unity tests and host-specific MCP checks. Fathom's release qualification and the game's adoption decision are two distinct approvals.
