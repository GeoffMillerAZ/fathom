# Invariants

Enforcement describes what exists, not what a policy hopes will exist.
`test` names a repository assertion; `task check` invokes tests locally.
No continuously enforced fleet-wide CI claim is implied.

| ID | Commitment and scope | Enforced by |
|---|---|---|
| I1 | Consumers can stop at any applicable layer; no forced runtime/framework migration for token/CSS adoption. | stated: ADR-0017/0021; guidance wording tests only |
| I2 | CUE authors token/schema truth; committed generated artifacts regenerate identically. | task check generation diff, including terminal palette and synthwave |
| I3 | Tailwind binaries are checksum-pinned and verified; mismatch stays a failure. | code: internal/tailwind; package tests; task check |
| I4 | Shared rendered colors use semantic roles. Generated token definitions and terminal palette contain the actual values; UI renderers do not invent colors. | token role tests; viz/vizframe tests; broader CSS composition still requires review |
| I5 | L3a viz emits no class attributes and requires no Tailwind scan; L3b/L4 declare their CSS build requirements. | TestVizEmitsNoClassAttribute; docs/ui-components.md coverage test |
| I6 | Portable assets remain usable without a bundler, preflight or a consumer framework; kit CSS stays locally overridable. | motion/vendor tests for their covered artifacts; remaining artifacts reviewed/stated (ADR-0017) |
| I7 | Rendered untrusted labels are escaped; trusted-HTML escape hatches require review at the caller boundary. | render/package tests cover strings; end-to-end content safety is not proven by those tests |
| I8 | A profile never claims unavailable services, platform support, accessibility or performance. Unknown evidence is explicit. | stated; MCP guidance/current-doc tests cover selected text and document routing only |
| I9 | Graphical app domain code does not depend on native-shell bindings; HTTP profile security/recovery must be proven before native support is claimed. | ADR-0021/0026/0035/0046/0047; HTTP/stream, launch/refusal and server dependency-closure tests; [containment](../assessments/2026-09-14-native-containment.md), [DOM tasks](../assessments/2026-09-14-native-gallery-workflow.md), [camera-only denial](../assessments/2026-09-15-native-media-denial.md), [native process/HTTP recovery](../assessments/2026-09-15-native-process-recovery.md); full permissions, input and delivery proof incomplete |
| I10 | CLI success/failure/could-not-check are distinct; machine output stays usable without terminal color, interactive input or decorations. | explicit exits/reporting tests; [opt-in usage migration](../assessments/2026-09-16-cli-usage-migration.md) adds origin/compatibility, consumer race and13/13 sandbox checks; historical failures stay retained, default Execute remains unchanged and installed-host delivery is unproven |
| I11 | A meaning-bearing figure identifies its units, period, source, uncertainty and missingness where applicable; colors need an understandable encoding. | vizframe/analytics contracts, Gauge caption regressions and opt-in vizcheck static-output canaries; full consumer semantics, font geometry and browser proof remain separate |
| I12 | Essential tasks work by keyboard and without color-only meaning; movement respects reduced motion; focus and saved work survive updates. | motion tests cover reduced-motion CSS; cross-component accessibility/interaction is not yet a common gate |
| I13 | Commercial references retain provenance/access restrictions; modified source is not automatically independently distributable. | ZIP/hash/metadata and loopback/origin tests; prepared comparisons require exact receipt/file identity; browser containment and comprehensive export gate unproven |
| I14 | A reusable promotion replaces the original fork and carries fixture, docs and downstream verification appropriate to its layer. | stated ADR-0017/0018; vendor drift tests and synthetic host-routing evidence available; real-consumer promotion remains unproven |
| I15 | Plan completion is evidence-backed and missing prerequisites are never reported as passes. | fathom verify for claimed plan contracts; push-runner preflight below is a stated operator requirement, not automated enforcement; neither is visual/usability proof |
| I16 | Kit quality measurements stay local; inherited no-telemetry policy does not permit implicit usage reporting from consumers. | stated ADR-0014/0021; no new telemetry mechanism introduced |
| I17 | Originals stay accessible independently of conversion state. Converted target support requires revision-bound applicable evidence. Agent repairs cannot weaken the oracle or silently accept a new visual baseline. Normal replay needs no LLM. | Gallery original-recovery/MCP canaries; syntax/stored envelopes force not-verified; designconvert/designattempts replay/provenance checks; [designreceipts/designevidence](../design/target-evidence.md) bind private historical bytes with CLI/MCP/Gallery parity; [P06 semantic checker](../assessments/2026-09-14-navigation-semantic-proof.md) derives scoped task results, never certification; full target gates and independent oracle-change enforcement remain open |
| I18 | Explicit adoption checks fail on missing/drifted selected files or incompatible catalog choices; static declarations never become host/runtime/delivery proof. | internal/adoption and CLI failure canaries, including ADR-0065 selected-metadata drift; explicit consumer checks are not installed automation or clean-build proof |
| I19 | Selected Wails profiles retain independently usable browser core tasks; native-only exceptions and local/remote topologies are explicit. | ADR-0033 matrix; cgo-free browser and macOS share [14 real Gallery DOM-task checks](../assessments/2026-09-14-native-gallery-workflow.md); full input, failure and delivery matrix incomplete |

## Evidence needed beyond unit tests

Before an authorized push, resolve/check the hook runner in the **same execution
environment** used by Git. Prepend the selected Go toolchain to the existing PATH;
do not replace PATH and accidentally remove the runner. A generated hook can print
`Can't find lefthook in PATH` yet exit0 (INC-70). Missing or skipped gate jobs are
not verification; an explicit post-push check uses `lefthook run pre-push --all-files`
because an already-synchronized remote otherwise yields no matching push files.
This is a reviewed invocation requirement, not a repaired installed-hook guarantee.

A browser specimen must exercise a person's task, including errors and recovery.
Record viewport, theme, input method, dataset, expected result and observed result.
Performance claims name hardware, workload, payload, latency and instrument.
Visual comparisons need stable data/font/rendering conditions and reviewed changes.
Screenshots cannot prove keyboard behavior, a correct metric or streaming recovery.
I17 also requires retaining executed harness/configuration bytes before editing a tested
revision, not hashes alone. The P09 delivery probe saves exclusive private snapshots;
the [Fathom candidate runner](../assessments/fixtures/fathom-handler/README.md) now does
likewise for browser/config/Go fixtures. This is local enforcement, not fleet-wide coverage ([INC-43](incidents.md)).
Intermediate failing Go tests are harnesses too: freeze their bytes before repair.
Warm preparation/closure collectors also require pre-execution snapshots; missing source leaves diagnostic logs, not replayable evidence (INC-63).
Useful-view timing cannot end or navigate onward while required asset requests lack
completed responses; `phase_wait.test.mjs` covers the acquisition barrier (INC-52).
Target/context causal claims require recorded ID and time lineage through the relevant
workload; a distinct context or blank-page control alone is insufficient (INC-53).
This is a review requirement, not an automatically enforced causal classifier.
Owner cleanup must bound exit and output-drain waits separately; unresolved children
and truncated capture fail. Local `owned_process.test.mjs` enforces this boundary (INC-54), not fleet-wide cleanup.
Browser measurement callbacks also need bounded waits inside the owner watchdog;
record instrumentation changes and preserve application-script policy (INC-59).
Minimum idle windows recheck their monotonic deadline; a requested timer delay is not elapsed evidence (`memory.test.mjs`, INC-66).
Freeze the resolved executable/version and run that selection; a mismatch cannot satisfy
the declared acquisition even if assertions pass ([terminal receipt correction](../assessments/2026-09-16-terminal-acceptance-matrix.md#retained-interpreter-mismatch)).
The recovery Node-test entrypoint rejects missing retained-root identity; other direct test commands are not universally guarded.
If missing, retain logs as non-replayable diagnostics; a later green snapshot cannot upgrade them.
For I19, native AX command success is not a user-task pass: require the expected visible
value and enabled state after the action, retaining failed postconditions ([INC-50](incidents.md)).

## Revisit when

- A stated rule fails in a consumer: reproduce it and raise the narrowest applicable
  control; record the outcome in incidents.md.
- A test is renamed or a gate stops invoking it: update this table in that change.
- A policy cannot distinguish a legitimate divergence from a defect: narrow its scope
  before making it a blocking control.
