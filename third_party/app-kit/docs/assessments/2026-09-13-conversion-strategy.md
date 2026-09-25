# Conversion strategy evaluation — 2026-09-13

## Judgment

The owner's idea is sound as a controlled source-to-target engineering pipeline, not as
an arbitrary framework translator. Upfront investment can amortize across many apps,
but only if recurring adapter/vendor-update cost stays below the work it removes.
Go/CUE improve opportunities for typed contracts and reproducibility; no language alone
makes a UI deterministic, fast, accessible or operationally stable.

The strongest first target is official Tailwind Plus HTML into readable templ/static
HTML while preserving a proven interaction runtime. Catalyst/MatDash React-only behavior
needs selected ports or a client-native boundary. Do not strip mature focus/selection
logic and then spend the savings rebuilding it less completely.

## Source-grounded findings

- Tailwind's [2021 engineering account](https://tailwindcss.com/blog/building-react-and-vue-support-for-tailwind-ui)
  describes a controlled authoring format and AST transformations into HTML, preview,
  React and Vue outputs. The authors encoded interaction intent and relied on shared
  behavior libraries. This is precedent for constrained generation, not proof their
  private authoring representation can be recovered from arbitrary downloaded output.
- Tailwind's [Elements documentation](https://tailwindcss.com/plus/ui-blocks/documentation/elements)
  describes framework-independent HTML interaction primitives. Earlier authenticated
  portal inspection established availability; today's direct web fetch required login.
  Source/archive presence does not prove lifecycle compatibility with Datastar. The
  documented minimum browser set must be checked at the selected runtime pin.
- [templ supports static and server rendering](https://templ.guide/), with compiled Go
  components. Static output need not deploy a Go server. Its [React integration guide](https://templ.guide/syntax-and-usage/using-react-with-templ/)
  also demonstrates client islands, so a typed server boundary need not ban specialized
  browser interaction. These are supported mechanisms, not app-kit integration proof.
- [Datastar's guidance](https://data-star.dev/guide/the_tao_of_datastar) favors backend
  authority, limited signals, ordinary links and server projections. Accessibility
  remains the application's work. [Security docs](https://data-star.dev/reference/security)
  describe nonce CSP mode but explicitly warn that expressions are not sanitized and
  client values remain untrusted. Test the selected pin rather than copying a CSP claim.
- [WebAssembly's browser model](https://developer.mozilla.org/en-US/docs/WebAssembly/Guides/Concepts)
  complements JavaScript and Web APIs. [Workers](https://developer.mozilla.org/en-US/docs/Learn_web_development/Extensions/Performance/JavaScript)
  move computation off the main thread but cannot manipulate DOM directly. Inference:
  Wasm should earn a compute boundary, not be assumed to solve layout or UI state.
- Go distinguishes [browser and WASI targets](https://go.dev/wiki/WebAssembly); its
  [Wasm export/reactor support](https://go.dev/blog/wasmexport) is useful for host-called
  modules. Exact ABI, runtime feature support, startup and copying costs need a benchmark.
- [wasm3](https://github.com/wasm3/wasm3) identifies itself as an interpreter and declares
  minimal maintenance. [wazero](https://wazero.io/) is a pure-Go, no-cgo host candidate.
  Neither should be added to browser pages, and neither is selected by this evaluation.
- [Core NATS](https://docs.nats.io/learn/core-nats/) delivers ephemeral at-most-once
  messages; offline subscribers do not receive them later. JetStream adds persistence,
  while [pull-consumer controls](https://docs.nats.io/learn/jetstream/pull-consumers)
  bound delivery batches/timeouts. Inference: event infrastructure must match loss,
  replay and backpressure requirements; it cannot substitute for application state policy.

## Local evidence and limitations

The [purchase audit](2026-09-13-paid-design-assets.md) verifies 657 captured components,
10,452 code variants, and 17 newly imported whole-source archives. New UI-block bundles
include official HTML; the most difficult React conversion can often be avoided.
Eight changed components and six previously absent variants provide a real update
rehearsal. Original variants must remain available for comparison.

The current first-party catalog has entry-level profiles, maturity, source anchors and
proof command strings. It does not yet have source-to-target lineage, executable
conversion recipes or per-target verification receipts. No conversion throughput,
visual-equivalence rate, performance win or fleet adoption is claimed.

Chrome connection failed after the supported approved fresh-window recovery; live
browser proof remains pending. Public documentation research used the web tool because
Firecrawl account validation returned Unauthorized: Invalid token. No dependency was
installed and no purchased code was executed or uploaded during this evaluation.

## Recommended investment order

1. Freeze task contracts, source references and baseline environments; inventory all
   source families, including unsupported ones, without converting every copy.
2. Prove a stratified 12-case pilot spanning static, operational and client-owned work.
3. Make reusable syntax/interaction rules and independent failure canaries; preserve
   source fidelity before applying semantic tokens or product-specific variation.
4. Batch supported families, publish honest target receipts through Gallery/MCP, and
   rehearse a vendor update. Improve selection by task and proven target, not raw count.
5. Prove savings and task quality in two comparable consumers. Scale the families that
   repay their maintenance; retain source-native alternatives where a port does not.

Payback estimate: expected avoided adaptation/upgrade effort across future integrations
minus initial pipeline/port effort and continuing repairs, review and maintenance.
Measure it; do not choose a conversion percentage that rewards incomplete interactions.
The first valuable result is an excellent complete workflow, not thousands of green
syntax checks. [ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md)
records the decision; design docs own the pipeline and runtime contracts.
