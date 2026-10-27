# Assessments

Dated, plain-language readings of things fathom does not run itself: the market, a neighbouring
tool, a survey of peer stacks, an owner-verdicted scorecard, a language queue, a video series, an
audit of the semantic layers. One file per assessment, named `YYYY-MM-DD-<slug>.md`; the data an
assessment rests on sits beside it in `data/`. This is the family convention (agentic-kit and
the review kit keep `docs/assessments/` the same way, ADR-055: one family, one contract).

An assessment is finished only when its summary is here **and** every decision it drove is in the
records (an ADR or a dated amendment, futures, scope, plan rows). The artifact that first showed
the reading is a readable copy, never the record: artifacts move, expire and are edited by their
readers, and nothing in a page can be verified against this repository.

## What lives here, and what does not

| Place | Holds | Why not here |
|---|---|---|
| `docs/assessments/` | A reading of other people's work or of an owner's judgment, and the evidence behind a decision: market maps, vendor documentation, a hand-verdicted scorecard, notes on a video | (this directory) |
| `docs/bench/` and `docs/bench/readings/` | Measurements fathom runs and can re-run: pre-registered bars, derived third-party truth, ledgers (ADR-031 to ADR-034) | An assessment is not re-runnable; filing it as a reading would dilute "a reading is re-runnable". Measurements that come out of a program (the test_ran base rate, language readings, PR-M0 to PR-M2) do go to `docs/bench/readings/` |
| `docs/research/` (not used) | nothing | The name collides with fathom's own vocabulary (the frozen `fathom research` verb, the research bundle of ADR-046 decision 6, `research-flywheel.md`); a directory of that name would be served and searched as if it were that feature |
| `docs/design/`, `docs/vision/`, `ADR.md` | Decisions and their designs | An assessment informs a decision and never makes one; it cites the record that did |

Two older documents of this kind sit at the top of `docs/` (`assessment-claude-code-integration.md`
and `graphify-comparison.html`); they may move in later with link updates.

## Conventions

- **Open with a plain-language verdict**: what the reading says in a few sentences, in words a
  newcomer follows, before any table.
- **Grade every claim.** Use these five words and no others: *measured* (fathom or the author read
  it from a log, a file or an API on a stated date), *estimate* (arithmetic over measured inputs),
  *claimed by a vendor* (their page, their benchmark, their funding figure), *hypothesis* (a
  saving or effect nobody has measured), *unmeasured* (a question with no reading yet). Say what the
  document does **not** claim.
- **Cite every decision it drove by id** (ADR, futures F, scope G, question Q, plan row) and never
  re-decide: the decision lives in the record, the assessment holds the evidence.
- **Never hold a status the product computes** (ADR-054, I54): a language note cites readings and
  `fathom support`; it keeps no level table. The same goes for plan progress (`fathom plans status`).
- **Third-party material is summarized in our own words**, with the source, the commit or read date
  and the licence. Short factual figures are fine; no long quotes, no branding, no course or
  marketing material (the MIT notices of the sources are kept where code or thresholds are reused).
- **Public product names only.** I19's denylist is about private hosts, emails and accounts;
  `TestNoOrgSpecificNames` runs over this directory like every other.

## Index

| Date | File | The verdict in one line | Decisions it drove |
|---|---|---|---|
| 2026-09-27 | [agentic-coding-landscape](2026-09-27-agentic-coding-landscape.md) | The market has many ways to find code and describe work and almost none to prove things about code; keep building and point at that gap, with reach before new engines | ADR-049, ADR-050, ADR-051, vision 38 sections 5 to 7 |
| 2026-09-27 | [fathom-and-claude-os](2026-09-27-fathom-and-claude-os.md) | Neighbours, not competitors (about 25 percent overlap at session-start context); keep building with a sharper focus; one embedded file, many indexes, no extra servers | ADR-049 amendment 2026-09-29, F55, RH-11, RH-12, GL-23, SR-X1, RH-X4 |
| 2026-09-27 | [peer-stacks-and-rust](2026-09-27-peer-stacks-and-rust.md), [data](data/2026-09-27-peer-stacks.json) | Fathom's stack is among the leanest in the field and Go is not the bottleneck; do not rewrite in Rust | ADR-049 decision 11, vision 38 section 8, F29, RH-M5 |
| 2026-09-27 | [feature-scorecard](2026-09-27-feature-scorecard.md), [data](data/2026-09-27-feature-scorecard.json) | Eighty-six features, each scored and verdicted by the owner (30 keep, 24 improve, 12 merge, 10 ditch, 10 new); the strongest is done-means-proven, the weakest is governance that does not bite | ADR-053, ADR-054, FM-M1, F51 |
| 2026-09-28 | [jev-ten-levels](2026-09-28-jev-ten-levels.md) | Level 10 works in fathom if the test run does the asking: compute first, judge only what is left, keep Jev optional per feature | ADR-056, vision 40, F34 to F49, G27 to G32 |
| 2026-09-28 | [semantic-layers-evidence](2026-09-28-semantic-layers-evidence.md) | Keep the layers and improve them: cheap, checked and self-healing, but served is not obeyed and nothing learns yet | ADR-016, ADR-038, ADR-057, GL-10 to GL-18, F53, F7 |
| 2026-09-29 | [language-atlas](2026-09-29-language-atlas.md) | A 142-item queue and a four-level ladder make "supports language X" a reading; the bottleneck moved from dialects to the measuring instruments | ADR-048, ADR-054, G15, vision 37 |
| 2026-10-11 | [reviews-and-incidents](2026-10-11-reviews-and-incidents.md) | Partly yes: fathom supplies deterministic cross-file joins, facts with coverage and receipts behind well-architected, security and incident reviews; the review kit judges, the policy kit and the adopter's change-risk tierer own rules and tiers, an external agent acts, and fathom never grades, gates or mitigates | ADR-069 decisions 1 to 8, the ADR-067 decision 12 and ADR-068 decision 4 amendments, vision 44, G37, F71, F72, F4, F32, F59, WAE- |
| 2026-10-11 | [index-snapshots](2026-10-11-index-snapshots.md) | Yes as files CI exports and the operator serves, not a server fathom operates; one new subsystem (`import snapshot`); instances ship index snapshots and never call each other; parked until the owner starts it, and the environment daemon waits on the owner's U1 answer | ADR-069 decisions 9 to 15, F3, F27, F70, F72 to F74, G37's IXS-M0, IXS- |
| 2026-10-11 | [making-the-vision-happen](2026-10-11-making-the-vision-happen.md) | Fathom is the center of the evidence plane, not of the platform: the best context source for the review kit, the adopter's change-risk tierer and incident agents, with no new service; the review kit's reliability catalog is the critical path, and owner hours bind before worker-days | ADR-069, vision 44, G37 (kill 2027-01-31), owner rulings D1 to D19 |
| 2026-10-16 | [claude-code-mods](2026-10-16-claude-code-mods.md) | Yes, in two steps and only as a display: Go and markdown first, then one small TypeScript module in an optional plugin that draws what fathom's hooks already told the agent; never a gate | ADR-072, the `No Node toolchain` carve-out, ADR-029 amended, F2 in part, G40, Q36, plan MOD- |
| 2026-10-16 | [interception-seams](2026-10-16-interception-seams.md) | Thirteen hook seams; two Very high (the readouts view, a deterministic claim check at Stop), each Medium or higher one ends in a 7-day reading with no false answer | readouts design section 20, G41, F79, RO-37 to RO-47, MOD-20, IXS-14 |
| 2026-10-16 | [decision-models](2026-10-16-decision-models.md) | Decision models pay now only as $0 scaffolding (kill, consent, disclosure, a fitted table, the `ask` spine at provider none); whether a fast model reads code is a stage-gated experiment, at most $2 then $8; the port is hexagonal | ADR-073, I66, G42, Q37, F34 in part, F80, F81, KS-, EXP-, AQ-, RJ-, JG- |
| 2026-10-17 | [intent-practice](2026-10-17-intent-practice.md) | Keep the practice (cheap, read): delivery was the weak link, so v0.15.0 retires `INTENT.md` and generates edit-time intent from the router; seven alternatives rejected for now with wins-if lines; K1 to K6 are due 2026-11-17 and unread | ADR-074, IV-1 to IV-10, F86, F9, O1134 to O1137 |
| 2026-10-25 | [refused-cedar](2026-10-25-refused-cedar.md) | Cedar retry trigger fired: SwornSystems policy/schema grammars each leave 1/59 CLI-accepted files in error; use the normal grammar route, not native admission | LB-Q-cedar, LB-Q-cedar-rest, ADR-048 wave-pc amendment |
| 2026-10-25 | [jinja2 grammar replacement](2026-10-25-refused-jinja2.md) | The patched MIT grammar reaches L3 excellent on 520 templates at 5.17 MB/CPU-s median; three Jinja extensions dispatch, while `.html` needs separate work | LB-Q-jinja2, LB-Q-jinja2-rest, P-R1 |
| 2026-10-25 | [refused-blade](2026-10-25-refused-blade.md) | Blade refusal holds: omitted source grammars measure 94/254 and 35/254 errors; all six repositories and independent full-population Laravel oracle routes are named | LB-Q-blade |
| 2026-10-25 | [matlab-free-oracle-ceiling](2026-10-25-matlab-free-oracle-ceiling.md) | Reviewed free tools do not yet supply independent call and resolution truth across the unchanged MATLAB corpus | LB-Q-matlab-rest |
| 2026-10-25 | [wgsl-native-l3-ceiling](2026-10-25-wgsl-native-l3-ceiling.md) | Standard WGSL has no native import denominator; existing WESL and naga_oil resolvers need separately ruled queue populations | LB-Q-wgsl-rest |
| 2026-10-25 | [refused-swift](2026-10-25-refused-swift.md) | Measured: the 36/642 ERROR-node count reproduces the original reading; the current ERROR-or-MISSING gate is 123/642, so the refusal holds | LB-Q-swift-rest reconciliation and P-R1 retry route under ADR-048 |
| 2026-10-25 | [sql-native-oracle](2026-10-25-sql-oracle-ceiling.md), [data](data/sql-oracle-2026-10-25/measurement-summary.json) | Native catalogs cover all 17 inline FK pairs; PostgreSQL 18 and matching pgvector remove the false ceiling | ADR-048, LB-Q-sql-rest; OL-R1 correction |
| 2026-10-25 | [lua-oracle-candidates](2026-10-25-lua-oracle-ceiling.md), [data](data/lua-oracle-2026-10-25/probe-summary.json) | LuaLS and EmmyLua need a tier ruling; native Telescope trace reaches 81.01%, below 90% | ADR-048, LB-Q-lua-rest; independent L2/L3 triggers |
| 2026-10-25 | [react-oracle-candidates](2026-10-25-react-oracle-candidates.md), [probe](data/react-oracle-2026-10-25/summary.json) | Compiler selection omits class components; docgen and typed TS graphs do not supply complete native React roles | ADR-048, LB-Q-react-rest |
| 2026-10-25 | [rego-oracle-candidates](2026-10-25-rego-oracle-candidates.md) | OPA already provides complete native A import truth; OPA1.21.4 and Regal0.43.0 routes cannot repair missing product import edges | ADR-048, LB-Q-rego-rest |
| 2026-10-25 | [jupyter-oracle-candidates](2026-10-25-jupyter-oracle-ceiling.md), [modern diagnostic](data/jupyter-oracle-2026-10-25/modern-comparison-summary.json) | Direct notebook coverage 1,945/2,071; pair diagnostics are AST-derived C, not resolved targets; canonical support unchanged | ADR-048, LB-Q-jupyter-rest, OL-R4 |
| 2026-10-25 | [language-onboarding](2026-10-25-language-onboarding.md), [GPU cases](data/2026-10-25-language-onboarding-cases.json), [mechanical](data/2026-10-25-language-onboarding-mechanical.json), [criteria / null](data/2026-10-25-language-onboarding-criteria.json) | New preflight/importer rejected for now; ten source-history defects inform the existing worker checklist | ADR-048, ADR-073, ADR-075, G15; LH-R2 and [unnumbered draft](../design/language-onboarding-decision-draft.md) |
| 2026-10-25 | [php-oracle-candidates](2026-10-25-php-oracle-candidates.md) | Three SCIP alternatives and native Xdebug traces measured over both full subjects; complete-population gaps disclosed | ADR-048, LB-Q-php-rest, OL-R7/OL-R9 |
| 2026-10-25 | [ruby-oracle-candidates](2026-10-25-ruby-oracle-candidates.md) | Released TypeProf A full-population and TracePoint A executed-only measured; unresolved references count as misses; canonical grades unchanged | ADR-048, LB-Q-ruby-rest |
| 2026-10-25 | [powershell-oracle-candidates](2026-10-25-powershell-oracle-candidates.md) | Native 7.6.9 binder derives declared helpers from owner AST; function-only preambles, complete helper crossings and shadowed origins disclosed; 3126/18842 and 580/5838 unresolved (PSA closure sensitivity 582); canonical grades unchanged | ADR-048, LB-Q-powershell-rest |
| 2026-10-25 | [sentinel-oracle-candidates](2026-10-25-sentinel-oracle-candidates.md) | Open parser pinned/measured: 30/395 full-discovery errors; policy-body split 2/90; runtime publication barred | LB-Q-sentinel-rest, OL-R6/OL-R8 |
| 2026-10-25 | [tools-pgbouncer](2026-10-25-tools-pgbouncer.md) | Native auth database population and source audit; r4 position proof follows in the next assessment | LB-Q-pgbouncer |
| 2026-10-26 | [pulumi-sdk-triage](2026-10-26-pulumi-sdk-triage.md), [data](data/pulumi-sdk-triage-2026-10-26/receipt.json) | TypeScript-only partial triage; compiler/indexer and owner-runtime dispositions; full multi-language question remains open | Feeds IAC-4/GAP1-36; does not satisfy them |
| 2026-10-26 | [core-t1-astro-ceiling](2026-10-26-core-t1-astro-ceiling.md) | Measured extractor gap has an existing oracle; serialize its shared-query repair and re-read language plus route consumers | C-R4 (#20), LB-Q-astro-rest, K29 |
| 2026-10-26 | [core-t1-cpp-ceiling](2026-10-26-core-t1-cpp-ceiling.md) | Full-population producer audit completes; incumbent and three pinned parser alternatives fail admission, with no canonical adoption. | LB-Q-cpp-rest |
| 2026-10-26 | [core-t1-css-ceiling](2026-10-26-core-t1-css-ceiling.md) | Browser-attested proposal ranges grade at tier C; parser errors and first-party population enumeration remain open. | LB-Q-css-rest |
| 2026-10-26 | [crossplane-provider-crds](2026-10-26-crossplane-provider-crds.md) | Full 244-file tier-C grade matches 428/428 definitions, 86/86 references and 33/33 cross-file references; nine-subject native probe validates 21 provider bases, all from three network subjects selected after a zero-match first probe; L2/L3 native binding remains open | GAP1-34, LB-Q-crossplane-rest, P-T1 |
| 2026-10-26 | [core-t1-cue-ceiling](2026-10-26-core-t1-cue-ceiling.md) | The package-key file target limits recall; LSP definitions support a future SCIP writer. | LB-Q-cue-rest |
| 2026-10-26 | [core-t1-go-cost-ceiling](2026-10-26-core-t1-go-cost-ceiling.md) | Field-only member symbols fail the wall gate; runtime reverted and no new grade claimed | C-R2, C-R5 (#20), LB-Q-go-rest |
| 2026-10-26 | [core-t1-javascript-ceiling](2026-10-26-core-t1-javascript-ceiling.md) | The public C-R3 ruling retains the denominator and cost rules. | LB-Q-javascript-rest |
| 2026-10-26 | [core-t1-kotlin-ceiling](2026-10-26-core-t1-kotlin-ceiling.md) | Two pinned 0.13.2 producer routes fail before emitting an index; the typed-range reader cause remains unproved. | LB-Q-kotlin-rest |
| 2026-10-26 | [core-t1-perl-ceiling](2026-10-26-core-t1-perl-ceiling.md) | C-R9 native ranges: 2,870 ends agreeing within one line, L1 excellent/L2 C good candidate; stock call census and literal-method control retain population gaps. | LB-Q-perl-rest |
| 2026-10-26 | [core-t1-sass-grammar](2026-10-26-core-t1-sass-grammar.md) | Measured licensed candidate fails the complete-source grammar gate; refusal has concrete syntax causes | LB-Q-sass-rest, X-R1 |
| 2026-10-26 | [core-t1-scala-ceiling](2026-10-26-core-t1-scala-ceiling.md) | The end-line repair and SemanticDB build remain shared-engine and build routes. | LB-Q-scala-rest |
| 2026-10-26 | [core-t1-scss-oracle](2026-10-26-core-t1-scss-oracle.md) | Native-only gains: canonical subjects 0, original Bourbon 0, Foundation 25 post hoc; full Bourbon L3 unmet (.5294) post hoc; canonical 0.5.1 counts preserved | LB-Q-scss-rest, W5, X-R1 |
| 2026-10-26 | [core-t1-scss-load-path-gap](2026-10-26-core-t1-scss-load-path-gap.md) | Draft ENGINE finding owned by core-t1: configured in-repo load paths, Bourbon 45/85; separate from fresh native-oracle validation | LB-Q-scss, LB-Q-scss-rest |
| 2026-10-26 | [core-t1-svelte-ceiling](2026-10-26-core-t1-svelte-ceiling.md) | The public C-R1 ruling retains the component kind ceiling. | LB-Q-svelte-rest |
| 2026-10-26 | [core-t1-tailwind-ceiling](2026-10-26-core-t1-tailwind-ceiling.md) | The pinned compiler parser has no full source rule ends. | LB-Q-tailwind-rest |
| 2026-10-26 | [core-t1-typescript-ceiling](2026-10-26-core-t1-typescript-ceiling.md) | Object-literal method extraction remains the concrete route. | LB-Q-typescript-rest |
| 2026-10-26 | [support-gap-sweep](2026-10-26-support-gap-sweep.md) | The board overstates runtime support: Python and Ruby manifests, Swift with Xcode, SQL dialects and the template engines are the top gaps, and formats the board calls disclosed are silent at runtime | GAP1-1 to GAP1-40 (language-breadth.gap1.md), LB-F453 |
| 2026-10-26 | [tools-kong-native](2026-10-26-tools-kong-native.md) | Native 37-pair/1,332-definition population; filename-selected four-pair/365-definition subset grants no support grade | LB-Q-kong-rest |
| 2026-10-26 | [opencl-c-triage](2026-10-26-opencl-c-triage.md), [compiler qualification](2026-10-26-opencl-c-compiler.md), [grammar gate](2026-10-26-opencl-c-grammar-gate.md) | Original patch 63/386; semicolonless repair 53/386 (13 non-clBLAS) fires G-R6 intermediate trigger; reused holdout 2/53 is diagnostic; native admission still fails, no support level | ADR-048; LB-Q-opencl disposition is controller-owned |
| 2026-10-26 | [tools-pgbouncer-native-positions](2026-10-26-tools-pgbouncer-native-positions.md) | Native callback brackets attest all 161 definition lines/ranges; L1/L2 excellent | LB-Q-pgbouncer |
| 2026-10-26 | [tools-nginx-native](2026-10-26-tools-nginx-native.md) | Native full upstream population diagnostic; canonical L3 unchanged | LB-Q-nginx-rest |
| 2026-10-26 | [tools-nginx-native-positions](2026-10-26-tools-nginx-native-positions.md) | Complete native lexer population attests 71/71 names/ranges and 1/1 references; canonical L3 unchanged | LB-Q-nginx-rest |
| 2026-10-26 | [glsl-registered-extensions](2026-10-26-glsl-registered-extensions.md), [paired data](data/glsl-g-r3-2026-10-26/canonical-baseline-core-to-full-registry-deltas.json) | Corrected macro precision and array trees retain L2 good; digest-only rejected candidates and fresh pinned statement/extension holdouts | G-R3/G-R4/G-R5, LB-Q-glsl |
| 2026-10-26 | [kustomize-d19-evidence](2026-10-26-kustomize-d19-evidence.md) | Main D19 regrade gives L3 excellent with metric-own denominators 35/35. The count floor depends on microservices-demo; L1 good and level_met need it and the kustomize examples. Native identity and no-op costs recorded | LB-Q-kustomize-rest, D19, P-T1 |
| 2026-10-26 | [terraform-two-hop-calibration](2026-10-26-terraform-two-hop-calibration.md), [data](data/terraform-two-hop-2026-10-26/summary.json) | Exact pinned native truth stays at 3,528 pairs before and after; the repaired engine finds all 3,528 | GAP1-32, LB-Q-terraform-rest, ADR-048 |
| 2026-10-26 | [iac-docxref-impact](2026-10-26-iac-docxref-impact.md), [data](data/2026-10-26-iac-docxref-impact.json) | Two-hop engine restores 1,381 native Terraform pairs; other doc strategies are isolated and fixture-replayed | IAC-5, IAC-6, IAC-9, IAC-11, GAP1-32, ADR-048, ADR-075 |
| 2026-10-26 | [next-phase-plan](2026-10-26-next-phase-plan.md) | Fathom cannot yet prove it helps: land the P0 fixes and drafts, move bench honesty into code, then run four preregistered experiments with written predictions | vision 38 §9.16, NP-Q1 to NP-Q6 |
| 2026-10-26 | [hlsl-scope-replay](2026-10-26-hlsl-scope-replay.md), [unscoped receipt](data/hlsl-scope-replay-2026-10-26/unscoped-regrade-receipt.json) | G-R7 SQL/CUDA no-scope compatibility proof; exact-main HLSL replay automation on the owner-approved ubuntu-24.04 runner | ENGINE #45, G-R7 |
| 2026-10-26 | [refused-angular-ceiling](2026-10-26-refused-angular-ceiling.md) | R-R4 spent: canonical 2/72, ng-matero 6/77 spent, ng-alain 55/55 context-deferred; no pooled grade and no implementation ships | LB-Q-angular-rest, R-R2, R-R3, R-R4 |
| 2026-10-26 | [bash-oracle-candidates](2026-10-26-bash-oracle-candidates.md), [receipt](data/bash-oracle-2026-10-26/evidence.json) | Released SCIP leaves literal command coverage below the bar; native namespace and executed-call probes disclose their scope and observer limits | LB-Q-bash-rest, P-T1, X-R4 |
| 2026-10-26 | [slang-triage](2026-10-26-slang-triage.md) | G-R10 bounded search, 466 dispositions; prospective 2,752-file primary and separate 461-file holdout with overlap disclosed, unmeasured | G-R1; NP-Q5 gates implementation |
| 2026-10-26 | [iac-provider-schema-feasibility](2026-10-26-iac-provider-schema-feasibility.md), [native data](data/2026-10-26-provider-schema-native-probe.json), [size data](data/2026-10-26-provider-schema-export-sizes.json) | Three offline native schema exports, 12 authored traversals and 12 attributable field refusals; dependency-plane policy, anchors and store costs remain open | IAC-2, IAC-7, IAC-12, IAC-16, SG-36, SG-37 |
| 2026-10-26 | [terragrunt-triage](2026-10-26-terragrunt-triage.md), [data](data/terragrunt-triage-2026-10-26/receipt.json) | Owner stack/DAG/language-server candidates; content-based cache inventory, native attempts need owner approval | Feeds IAC-20/GAP1-37; does not satisfy them |
| 2026-10-26 | [aws-cdk-triage](2026-10-26-aws-cdk-triage.md), [data](data/aws-cdk-triage-2026-10-26/receipt.json) | Owner synth/tree/trace and ordinary language candidates; content-based path-free cache inventory | Feeds IAC-20/GAP1-37; does not satisfy them |
| 2026-10-27 | [tools-protobuf-d19](2026-10-27-tools-protobuf-d19.md), [data](data/protobuf-d19-2026-10-27/receipt.json) | Dialect L3 unmet to excellent under D19 at native A: recall107/116, precision107/107; incumbent dispatch and rpc signatures stay unchanged | LB-Q-protobuf-rest, D19 |

## Companion pages

The owner keeps private companion pages for several of these readings; they are not linked here
because readers cannot open them, and the files above are the record.
