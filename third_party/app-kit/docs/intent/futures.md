# Futures

These are options, not current build orders. Current priority lives only in scope.md;
[ADR-0072's resume packets](../design/stabilization-and-resumption.md) park remaining campaign work without weakening its gates.
Preserve an option only when the choice costs no extra abstraction or code today.

## F1. Car Assistant and Growing Gardens BI adoption

Confidence: owner-deferred. Horizon: when either app becomes a current priority.
[ADR-0032](../adr/0032-defer-car-assistant-and-growing-gardens-adoption.md) parks this named pair, not general promotion evidence.
Do not build: consumer contracts/selections/routers, shell migrations, host setup or
upgrade/browser proof now; do not repeat approval prompts or auto-select other apps.
Preserve: the dated inventory, existing integrations and approved GGB finance repair.
**Promote when:** the owner explicitly resumes a named app/task; refresh its context,
dirty-work evidence and scoped authority. Each app resumes separately; G2/G3/G4 and
ADR-0017/0018/0020/0024 evidence requirements remain, with no completion implied.
**Drop if:** the owner explicitly retires the app or its adoption need; retain history.

## F2. Conjure as an additional UI reference

Confidence: owner-offered. Horizon: after the current Resume-as-Code/Fathom slice.
Do not build: a third simultaneous UI migration merely to increase consumer counts.
Preserve: Conjure as an authorized candidate for a distinct interaction or delivery task.
**Promote when:** the current pair leaves a meaningful kit boundary untested and a
specific Conjure UI task can exercise it; refresh its intent and source before selecting.
**Drop if:** the current pair supplies the same evidence without another migration.

## F3. Personal-site component family

Confidence: likely. Horizon: after named themes and a real shared task. The owner wants
the personal site's design to compound; presets/workbench already exist, with remaining G1/G4 acceptance deferred separately from this future.
Do not build: a copied website, generic persona engine, or a new React component package.
Preserve: source/revision and shell, filter, context-selection and disclosure candidates
in [theme-workbench](../design/theme-workbench.md); retain app-owned identity and behavior. The [read-only sizing](../assessments/2026-09-16-personal-site-component-scope.md) recommends one small-to-medium series-navigation candidate, not a whole website port or a new framework-native package; implementation/dogfood remains unadmitted pending its exact scope.
**Promote when:** two actual consumers need the same composition and an isolated recipe
meets keyboard/mobile/state, theme and upgrade evidence under ADR-0017.
**Drop if:** existing kit recipes plus a theme solve the task without copied structure.

<a id="windows-desktop-delivery"></a>

## F4. Windows desktop delivery

Confidence: likely. Horizon: when needed. The owner has no current Windows use case;
[ADR-0027](../adr/0027-web-and-macos-now-windows-on-demand.md) removes it from active G6.
Do not build: Windows-only adapters, installers, VMs, tests or repeated cross-build work now.
Preserve: the shared HTTP/domain boundary and existing isolated cross-platform source.
**Promote when:** the owner names a current Windows consumer/task and an authorized test
host; then schedule the full ADR-0021 matrix before claiming Windows support.
**Drop if:** the owner no longer wants Windows delivery or browser delivery meets the task.

## F5. Native iOS and Android

Confidence: likely. Horizon: later. The owner wants the same product investment to
reach phones; Wails mobile is still experimental.
Do not build: a speculative mobile runtime, native widget catalog or store pipeline.
Preserve: keep business logic independent of shell bindings; responsive web is useful now.
**Promote when:** a named mobile task needs native capability and a pinned runtime
candidate passes real-device HTTP/POST/stream/lifecycle/input/accessibility tests.
**Drop if:** native adds no useful capability over the app's responsive web delivery.

## F6. WASM validation or accelerated visualization islands

Confidence: plausible. Horizon: later. Go/CUE/WASM fit the owner's strengths, but startup,
download and memory cost must buy something for a specific interactive workload.
Do not build: a universal Go-to-frontend compiler or mandatory WASM boot for ordinary pages.
ADR-0023 admits conversion/runtime evaluation, not a Wasm engine or offline sync build.
Preserve: pure data transforms remain separate from rendering, where already equally simple.
**Promote when:** a named consumer demonstrates a client-side validation/compute need
and a measured prototype beats simpler alternatives at its payload/latency budget.
**Drop if:** transfer/startup cost or a duplicate authority model erases the benefit.

## F7. High-volume analytic renderers

Confidence: plausible. Horizon: later. SVG cannot be assumed to serve unlimited marks.
Do not build: a chart-engine abstraction, Canvas/WebGL adapter family or client data store.
Preserve: keep metric definitions and data reduction outside SVG rendering functions.
**Promote when:** a real dataset misses a recorded interaction budget after aggregation,
and an isolated renderer improves the same task while preserving accessible data output.
**Drop if:** query reduction or existing primitives meet the task without that complexity.

## F8. Distributed event and offline synchronization profile

Confidence: plausible. Horizon: later. NATS offers useful replay/fan-out primitives;
synchronization and conflict resolution still require application semantics.
Do not build: leaf-node sync, multi-device conflict rules or cross-app event taxonomies.
Preserve: one explicit authority for mutations and app-owned domain events.
**Promote when:** two devices/processes must coordinate for a named task and a bounded
NATS experiment passes reconnect, duplicates/order, retention and access tests.
**Drop if:** single-process state and ordinary HTTP cover the requirement.

## F9. Framework-native packages

Confidence: plausible. Horizon: later. Artifact consumption already supports non-Go apps.
Do not build: an npm distribution, React wrapper set or cross-framework component IR.
Preserve: portable CSS/JS and renderer-independent recipe descriptions already have value.
**Promote when:** two active consumers need the same native interaction package and
artifact adoption cannot meet their verified task without recurring duplication.
**Drop if:** a consumer-native composition or existing upstream package solves it cleanly.
