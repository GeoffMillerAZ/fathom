# Next session — Throughline screen redesign on the kit

**State (2026-06-10, end of wiring session):** steps 1–2 of the original plan
are DONE and browser-verified; the display-settings surface landed too.
What remains is the redesign proper (original step 3) and component
promotion (step 4).

## Done — do not redo

- **Throughline tokens:** `throughline/web/css/tokens.cue` (kit-schema
  validated; all semantic roles + tier/crit families, light/dark, OKLCH).
- **CSS chain:** `task ui:tokens` → generated `web/css/theme.css` +
  `views/tokens_gen.go`; `task ui:css` → kit-pinned Tailwind v4.3.0
  standalone. Node fully removed (package.json, node_modules gone).
- **Single source proven:** throughline.css color vars alias
  `--ak-color-*`; dark literals deleted; light+dark verified in Playwright.
- **Display settings:** Appearance section (Light/System/Dark + text size
  via `--ak-font-scale`), no-flash init, effective-theme toggle. Verified:
  cleared → System/Default; Dark+Large → 20px root, survives reload.
- **Kit fix from feedback:** `EmitGoNames` output is now gofmt-formatted.

## Next — the redesign (do this with fresh eyes)

Per-surface, with the impeccable skill (shape → brief → build), TailwindPlus
MCP for raw material, Playwright verification per screen. Priority:

1. Case overview (the dashboard — tier/crit/status families finally earn
   their utilities)
2. Evidence/assertions table with click-to-trace provenance
3. Contradiction/impeachment view
4. Element-coverage board
5. Criticality work-queue
6. Bitemporal timeline

While redesigning, migrate templ classes off the legacy aliases
(`bg-surface`→`bg-surface-raised`, `bg-bg`→`bg-surface`,
`text-ink-soft`→`text-ink-muted`, `bg-surface-sunk`→`bg-surface-sunken`,
`accent-weak`→`accent-soft`) and delete the alias block in
`web/css/app.tw.css` when the count hits zero.

## Kit backlog — standing themes and queued items

### Owner direction (locked): dashboard + visualization components

Dashboard and visualization components are a standing kit theme — dashboards
are a common pattern across every app in the portfolio. `appkit/ui/viz` is
the start, not the end. Planned expansions (build when the next consumer
surfaces a concrete need):

- **Multi-series line chart** — time-series data over a shared x-axis
  (bitemporal timeline, bidding trends).
- **Heatmap / calendar ring** — activity density (evidence ingestion rate,
  work-queue throughput).
- **Bullet chart** — comparative meter with reference marker (actual vs.
  target coverage).
- **Gallery integration** — live knob-driven catalog for all viz components
  once the Gallery app is built (D12); viz is dogfood #2 after tokens.
- **Accessibility hardening** — today `<title>` + `aria-label` cover the
  basics; structured table alternatives (`<desc>`, data tables for screen
  readers) can be added when a consumer needs WCAG 2.1 AA full compliance.

### First consumers of `appkit/ui/viz`

1. **Throughline doc-35 phase 5 dashboards** — case overview (tier/crit
   tiles), element-coverage board, criticality work-queue. All five
   components are directly applicable.
2. **alaska_bidding** — bid funnel (Funnel), win-rate trend (Sparkline),
   coverage ratio (MeterRow).

### Owner direction (locked): agentic-assistant experience components

Agentic assistants are a common theme in ALL the owner's apps. app-kit owns
the UI + app-side plumbing for excellent assistant experiences (agentic-kit
owns reasoning/governance; judge the split per piece). Required creature
comforts of a modern LLM chat surface, kit-ified once Throughline proves the
shapes (its Expert Chat is the requirements probe, 2026-06-10 uplift):

- **Composer pattern**: Enter submits / Shift+Enter newline (IME-safe via
  isComposing), textarea autosize, in-flight disabled state + thinking
  indicator, error bubble + retry, per-context draft persistence,
  model-selector persistence.
- **Disclosure component**: collapsed-by-default expandable sections under
  assistant messages — sources/citations (with unverified flagging),
  safety flags, and (when streaming lands) model thoughts.
- **Message renderer contract**: server-side safe markdown (escaped raw
  HTML), code-block styling + copy, copy-message, hover timestamps.
- **Conversation ergonomics**: auto-scroll w/ jump-to-latest pill.
- Later: SSE streaming pattern, tool-use/citation streaming, attachment UX.

All of it themed via the token chain only (no hardcoded values — owner-
enforced standard).

### Settings/appearance pattern

Segmented control + localStorage/no-flash/data-theme protocol is
app-agnostic; alaska_bidding will want it verbatim. Promote to kit component
or pattern page when the second consumer appears.

### Brand remap emitter

`app.tw.css` hand-carries indigo/gray → brand OKLCH ramps for TailwindPlus
components; an `appkit gen` flag could emit ramps from two hue numbers.
Build when a second app needs ramp generation.

### `appkit` CLI distribution

Throughline invokes it as `go run -C ../app-kit ./cmd/appkit` (sibling
checkout, no Go dependency coupling — deliberate). When a second machine or
CI needs it: `go install` story + version pin.

### templ tool-directive pattern

Throughline pins via `go run pkg@version` in its Taskfile; the kit scaffold
should ship the tool directive instead.

### HITL (human-in-the-loop) — standing kit theme (owner-directed 2026-06-11)

Corrections, annotations, and feedback UIs will recur in every app. First
consumer: Throughline's transcript-correction surface (inline edit preserving
the original, required reason, reviewer identity, dismiss-as-verified, audit
trail) built on agentic-kit `foundations/review/correction-ledger`. Extraction
candidates once a second consumer appears:
- **CorrectionField pattern**: markup + CSS contract for original-vs-corrected
  inline editing with reason + identity capture.
- **AudioPlayer w/ segment sync**: native `<audio>` + timed-segment highlight/
  click-to-seek (pure-Go HTML generator like `ui/viz`; Throughline has the
  proven markup/JS in views/transcripts_render.go + static/app.js).
- **Work-queue card** shape for review items (priority, deep link, inline form).

### credresolve consumed downstream

agentic-kit `runtimes/credresolve` (Anthropic credential chain incl. macOS
Keychain) now backs Throughline's settings connection-methods panel — app-kit
apps wanting "Claude Code /login just works" should consume it the same way.

### Assistant-response rendering — standing kit theme (owner-directed 2026-06-11)

A good chat experience limits noise but keeps depth reachable, and recognizes
entities. Proven in Throughline (surfaces/web/static/app.js
`enhanceAssistantMessages` + chat prompt rule 7) — extraction candidates:
- **Progressive disclosure**: answer lead stays visible; everything after
  folds behind "Show full analysis (N more sections)". Pairs with a prompt
  rule instructing answer-first structure. Never alters the model text —
  only folds it.
- **Entity affordances**: blocks mentioning an address / email / phone get a
  clearly-external helper-link row appended UNDER the block (Map ↗, Zillow ↗,
  mailto, tel). The rendered text is never rewritten; links are user-initiated
  navigation (egress-safe). Generalize as a pluggable detector→affordance
  registry (dates→calendar, case numbers→court lookup, tracking numbers, …).
- Tables/code already ride the markdown pipeline; the kit component should
  bundle: markdown (default-safe) + disclosure folding + entity affordances +
  citation chips as one "AssistantMessage" renderer.

### Assistant-rendering theme — refinements proven 2026-06-12 (Throughline)

Two corrections from live use, both belong in the kit AssistantMessage spec:
- **Region-biased entity affordances**: a bare "123 Main St, Springfield" sent
  Maps to MA and Zillow to OR. Affordance detectors need an app-supplied
  context hint (Throughline: case jurisdiction rendered as
  `<body data-region-hint="AZ">`; appended to the geo query only when the
  address lacks a state/ZIP, and only when the hint looks like a state code).
  Kit shape: the detector registry takes a context object, not just text.
- **Evidence row (citations never fold away)**: collapsed "Sources (N)"
  disclosures hide the *why*. Hoist deduped citation chips into an
  always-visible "Evidence:" row directly under the answer lead (cap ~6,
  "+N more" opens the full list; chips keep their provenance-drawer wiring).
  Disclosure folding and citation visibility are separate concerns: fold
  prose, never provenance.

### Chat thread management — standing kit theme (owner-directed 2026-06-12)

"Chat management will be extremely important in all our apps we make with LLM
chat interfaces." First consumer: Throughline threads sidebar. Kit candidates
once proven there: thread list component (pin/rename/archive, active
highlight, relative times, keyboard nav), search-with-results-panel pattern
(threads group + message-snippet group, fuzzy ranking via agentic-kit
`foundations/retrieval/fuzzy`, arrow-key navigation), event-sourced thread
lifecycle vocabulary (created/renamed/pinned/archived) for append-only apps.

### OAuth loopback flow — lifecycle lesson (2026-06-12)

PKCE loopback flows MUST outlive the HTTP request that starts them
(context.WithoutCancel + explicit timeout); a request-scoped context closes
the listener and the provider's redirect lands on a dead port
(ERR_CONNECTION_REFUSED after consent). Same bug class as request-scoped SSE
streams. Any kit OAuth/flow runtime should own its lifecycle internally.
