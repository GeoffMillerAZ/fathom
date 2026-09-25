# P05 typed email field and native recovery

Status: implemented private candidate with bounded synthetic/browser evidence.
This is the third typed pilot candidate after P02/P04, not target certification.
The frozen syntax/compiler denominator stays 439/657; zero targets are certified.

## What is reusable

`appkit data assets-adapt-form` verifies frozen P05 and produces a deterministic,
CUE-validated private artifact. Optional `-out`/`-check` reuse the existing internal
no-clobber assurance; five files contain the template, standalone model, defaults,
guidance and artifact. JSON remains the default. No backend, new MCP server, compiler
invocation, dependency install or public vendor-channel output is added to delivery.

`NewEmailField(EmailField)` validates presentation bounds and identifiers before output.
It retains malformed email values as escaped text so an app can display rejections.
Nonempty Error preserves source-error styling; empty Error selects the reviewed neutral
palette and removes the error icon, paragraph and error ARIA. Required and Autofocus
are opt-in. The app owns distinct/reserved IDs, form actions, validation and persistence.
CSS must scan both generated form.go class constants and form.templ.

The [task contract](../design/form-validation-adapter.md) explicitly expands the original
error-only specimen. The [reproduction guide](fixtures/form-adapter/README.md) covers
usage and test commands. ADR0023/0034/0036 and I7/I8/I12/I13/I17 govern this work.
No accepted decision was superseded, future promoted or compatibility surface broken.

## Evidence observed

| Check | Result and boundary |
|---|---|
| Frozen source | 1,208 bytes, application-ui archive b26cbf6e…; exact member/SHA remain P05 in the frozen manifest |
| CLI replay/delivery | 6,996-byte JSON deterministic; five-file export, exact check, no-clobber and drift refusals; source/catalog unchanged |
| Synthetic adapter/runtime |source/spoof/shape/palette refusals, escaped bindings, bounds, optional attributes, exact parsed DOM, cancellation/writer errors; unformatted/formatted templ compilation passes |
| Copied module | 13 preparation checks pass with its own go.mod/go.sum, cached checksums, missing-generation failure canary, actual original/default parsed DOM and no app-kit runtime dependency |
| HTTP workflow | initial 200 → domain-invalid 422 → corrected/injected 503 → retry 303 → confirmation GET; input retained, one successful synthetic save, refresh does not resubmit |
| Request/concurrency |bounded sessions/body/cardinality, scoped cookie/token, loopback/Host/origin refusals; same-token concurrency permits one request; generated-module race tests pass |
| Browser | 304 checks: 300 pass, 4 retained original size failures, 0 new/fidelity/workflow failures; 16 journeys at 390/1440px × light/dark × keyboard/pointer × ordinary/long label |
| Visual source/default |all four viewport/theme pairs have equal pre-screenshot DOM and byte-identical screenshots; chosen Inter loaded; desktop source and narrow dark recovery visually inspected |
| Package milestone |full `go test ./internal/designadapt ./cmd/appkit` and scoped vet pass; no new full-repository test claim |

Browser tests use 48 native POSTs across 16 successful recovery journeys, application
JavaScript disabled, strict CSP and local synthetic data only. They also prove native
malformed-address refusal without POST, associated field errors/focus, focused save
summary, recovery link, visible input focus, input/button geometry and refresh behavior.
Autofocus synchronization has a 1000ms ceiling and records elapsed time; it is not a
product latency measurement or programmatic focus repair. No pixel threshold/mask,
assertion deletion, source recoloring or original-failure exclusion made this pass.

Versions: Go 1.26.3 darwin/arm64, templ v0.3.1020, Tailwind 4.3.0, Playwright 1.63.0,
installed Chrome 152.0.7977.84. Inter v4.1 and compiled CSS are controlled test choices,
not recovered portal environment provenance. Shared stylesheet including font: 482,615 bytes.

## Failures and corrections retained

- Templ conditional blocks initially introduced whitespace-only DOM nodes. Go conditional
  composition fixed the generator; the original parsed-DOM oracle stayed unchanged.
- Initial `/var` versus `/private/var` module comparison failed. Both actual module paths
  now resolve canonically; no dependency or source exclusion was relaxed.
- The fixture repeated [INC-16](../intent/incidents.md): `no-referrer` caused real Chrome
  native POST `Origin:null` and 403. `same-origin` fixes native posts; null/foreign origins
  remain refused. HTTP-only tests had passed, demonstrating their limitation again.
- The first browser harness compared one DOM before screenshot tools and one afterward.
  Both are now sampled before capture, with no normalization. A second run found title
  copy mismatch and an autofocus observation race. Copy now states the failure clearly;
  independently reviewed bounded native-focus observation does not set focus or weaken
  assertions. The corresponding HTTP title assertion changed with the visible copy.
- Original input height is 36px in all four reference configurations. The adapter retains
  it. Only the explicit fixture adds 44px controls/links, clear underlines and theme-aware
  focus. Those improvements are not silently claimed for the delivered component.

## Retained evidence and remaining work

Shared discovery now includes `reference.form-adapter` in the existing catalog,
Gallery and terminal reference, plus the `form-adapter` MCP guide/resource. All
26 previous catalog records remain unchanged; the 27th describes experimental
CLI tooling, not certified web compatibility. Catalog and actual in-memory MCP/HTTP
tests cover source-free discovery, exact records, guidance and web-profile exclusions.
Generation still requires the independently verified private original. Rebuild
embedded-catalog binaries and restart their processes to expose the new record;
installed agent hosts were not restarted or reverified. No renderer changed, so
this metadata follow-up does not claim a new browser run or modify the capture below.

Ignored private capture: `data/design-assets/form-typed.cse6H8`, 49 payload files,
1,168,279 bytes; manifest SHA256:
`312aae53e9bc50085cb7b52d38ffd2f975a627cf04fca0f858ad5d26fed88c93`.
It retains copied build inputs/generated Go, artifact, source, CSS/fonts/license,
implementation snapshots, 16 screenshots, reports and selected earlier failure reports.
Executables, module caches and complete failed-run payloads are deliberately omitted.
Git does not back up this directory. Preserve it separately; keep derivatives private.

Artifact JSON SHA256:
`5adac4663faac7f4a25352d9092535e9be27b50de1921699a8471455fafb2602`.
The preparation report carries source/compiler/style identities; matching hashes are
generation evidence, not a certified-target receipt or authorization to redistribute.

Actual app integration, complete contrast,
zoom/touch/IME/screen-reader coverage, slow/offline recovery, human task acceptance,
full performance budgets and family-wide conversion remain open. The fixture is not
a production auth/CSRF/persistence implementation or an email-delivery service.
No G1–G6 outcome, deferred consumer or platform was marked complete.
