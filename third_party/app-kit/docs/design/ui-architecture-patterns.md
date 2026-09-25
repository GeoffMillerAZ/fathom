# UI architecture patterns

The patterns below are historical guidance for the Go hypermedia profile.
[ADR-0021](../adr/0021-runtime-profiles-and-platform-proof.md) now scopes those
requirements: portable and terminal consumers choose their own applicable profile,
and static pages do not require SSE or NATS. For complete task/state contracts,
use [experience-system.md](experience-system.md).

How a kit app decides its UI architecture. This is the operational
companion to ADR-0003 (`../adr/0003-hypermedia-first-datastar.md`): the ADR
says *why* hypermedia-first; this document says *how*, for every screen
shape we actually build. The Assistant serves these patterns as guidance
(D11); the Gallery demonstrates each one as a working page (D12).

How to use it: find your scenario in §2. If it isn't there, you almost
certainly want §1 unchanged. If the screen needs more interactivity than
attributes give you, climb the ladder in §3 — one rung at a time, passing
each admission test. Source components per §4. Never do the things in §5.

Versions assumed throughout (all verified 2026-06-10): Datastar v1.0.2,
datastar-go v1.2.2, templ v0.3.1020, nats-server v2.14.x embedded per
ADR-0008, Tailwind v4.3 standalone binary per ADR-0002.

---

## 1. The default pattern: the kit shape

Every kit screen, unless a scenario below says otherwise:

1. **One long-lived GET SSE stream per tab** renders the page as an
   idempotent **fat-morph snapshot** — the whole meaningful region, not
   surgical fragments. Morphing diffs it; element state (focus, scroll,
   input values) survives.
2. **Commands are short POSTs.** They mutate server state (domain logic →
   SQLite for durable truth, JetStream KV for reactive state) and return
   204 or a small error patch. They do not return the new page.
3. **The spine closes the loop.** Domain writes land in NATS KV; the
   stream's KV watch fires; the handler re-renders the snapshot; Datastar
   morphs the DOM. Commands never need to know who is watching.
4. **Signals are ephemeral only**: form drafts in flight, open/closed
   menus, a highlighted row. Nothing durable, nothing another tab should
   see — that goes through a command.

### Wire flow

```
Tab (webview or browser)        Go server (one binary)         Spine (embedded NATS)
 |---- GET /notes (HTML) --------->|                                |
 |<--- full page, data-init --------|                                |
 |---- GET /notes/stream (SSE) --->|--- KV watch + subscriptions -->|
 |<=== patch: full snapshot =======|                                |
 |---- POST /notes/create -------->|-- domain write --> KV put ---->|
 |          (204)                  |<-------- watch fires ----------|
 |<=== patch: new snapshot ========|   re-render; morph diffs       |
```

The page carries `data-init="@get('/notes/stream')"` on its root
element; Datastar opens the stream with built-in retry (1000ms base, 2x
scaler, 30s max wait — https://data-star.dev/reference/actions).

### Wiring sketch (Go + templ + datastar-go)

```go
// ui/notes.templ — the snapshot is one component, rendered whole.
templ NotesPage(m NotesModel) {
	<main id="page" data-signals="{draft: ''}">
		@NoteList(m.Notes, m.Selected)
		@NoteForm(m.Form)
		@ToastRegion(m.Toasts)
	</main>
}
```

```go
// The stream: subscribe, render, repeat. This loop IS the UI.
func (a *App) notesStream(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	sess := web.Session(r)

	render := func() error {
		m := a.queryNotes(r.Context(), sess) // read SQLite + KV, build model
		return sse.PatchElementTempl(ui.NotesPage(m))
	}
	// web.Stream renders once, then re-renders on each coalesced change
	// (bursts of N changes -> 1 re-render). It is sugar over plain bus
	// subscriptions (canonical bus interface: ADR-0008), not a Bus method.
	web.Stream(r.Context(), render,
		web.WatchKV("notes."+sess.User+".>"),       // domain state
		web.WatchSubject("session."+sess.ID+".ui")) // session-scoped nudges
}

// A command: mutate, return almost nothing. The stream delivers truth.
func (a *App) createNote(w http.ResponseWriter, r *http.Request) {
	var sig struct {
		Draft string `json:"draft"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if verr := a.notes.Create(r.Context(), web.Session(r).User, sig.Draft); verr != nil {
		sse := datastar.NewSSE(w, r) // short-lived: patch only the form's errors
		_ = sse.PatchElementTempl(ui.NoteForm(formWithErrors(sig.Draft, verr)))
		return
	}
	w.WriteHeader(http.StatusNoContent) // KV write already woke every stream
}
```

`appkit/web` wraps the stream loop (session extraction, render-func
plumbing, coalescing, heartbeat) so an app writes a query function and a
topic list, nothing else.

### The NATS→SSE bridge

The bridge is the canonical GONAD wiring (Go + NATS + Datastar — the stack
Datastar's author promotes from inside Synadia; reference template
zangster300/northstar, v0.16.0, 2026-06-03): each HTTP stream goroutine
subscribes to session- and domain-scoped KV watches/subjects, re-renders
templ on change, and emits patches via datastar-go. Properties the kit's
`appkit/web.Stream` helper guarantees:

- **Coalescing.** KV watches deliver bursts; the loop drains pending
  notifications before re-rendering. N writes → 1 snapshot.
- **Throttle floor.** Re-renders are rate-limited (default 10/s per
  stream); high-frequency producers cannot melt the morpher.
- **Heartbeat.** Periodic SSE comments keep intermediaries from reaping
  idle streams and let the server detect dead tabs (this is also the
  browser-tab lifecycle heartbeat from ADR-0005).
- **Context discipline.** Everything hangs off `r.Context()`; tab closes,
  goroutine dies, subscriptions are released.

### Reconnection and multi-tab

Both are free, by construction — this is why fat morphs are the default:

- **Reconnect = re-render.** The snapshot is idempotent; there is no event
  replay, no missed-message problem, no resume token. Datastar retries with
  backoff; the first render after reconnect is the current truth.
  Background tabs drop GET streams by default (`openWhenHidden: false`) and
  re-establish on visibility — v1.0.0 resends the request payload on that
  reconnect (#1140), so the stream resumes identically.
- **Multi-tab = multiple streams, one truth.** Every tab watches the same
  KV; every write converges every tab within one render loop. No tab is
  special; no sync code exists.

Web deployments must run HTTP/2: browsers cap ~6 connections per origin on
HTTP/1.1 and each tab holds one stream. Loopback desktop doesn't care.
Enable brotli on the stream — repetitive snapshots compress ~200:1
(https://data-star.dev/guide/the_tao_of_datastar).

### Why no optimistic UI

The Loopback Invariant makes the server round trip sub-millisecond —
faster than a frame. Optimistic UI is a latency-hiding technique, and we
have no latency to hide; what it costs is a second source of truth that can
disagree with the first. The kit's rule: show server truth, always; use
loading indicators on the rare command that is actually slow (§2.3 handles
genuinely long work). This matches Datastar's official stance.

### App file layout

```
myapp/
  main.go          // at the app root; boot is one line: appkit.Run(myapp.New)
  app.cue          // app manifest: identity, nav, theme, capabilities
  ui/
    shell.templ    // chrome: nav, toast region, command palette mount
    pages/         // one snapshot component per page
    components/    // app-specific components (kit components imported)
  store/
    migrations/    // SQLite schema migrations
  assets/          // static assets, go:embed-ed
  build/           // shell + packaging configuration
  islands/         // not scaffolded by default: created when the first
                   // L2 island is admitted (pinned vendored JS, go:embed)
  domain/          // business logic; writes SQLite + bus, renders nothing
```

---

## 2. Scenario patterns

Each pattern: when to use, wiring, key components, gotchas. All are
variations on §1 — none abandon it.

### 2.1 CRUD / forms

**When:** any create/edit surface. The bread and butter.

**Wiring:**
```
input fields --data-bind--> signals (the draft, ephemeral)
submit --POST /thing/save--> ReadSignals -> validate -> {
  ok:   write domain -> KV fires -> snapshot moves on (form resets/closes)
  fail: short SSE patch of the form fragment with field errors
}
```

**Validation flow.** Three tiers, cheapest first: (1) native HTML
(`required`, `pattern`, `min`) — free, instant; (2) server validation on
submit — the authoritative tier, always present; Go domain validation for
domain rules, CUE (`cuecontext`) for anything schema-shaped (D7); (3)
debounced per-field POST only for checks that need the server (uniqueness,
cross-field) — debounce via `data-on`'s debounce modifier, 300–500ms,
never raw per-keystroke (§5.2).

**Error rendering.** Two homes for error state, chosen deliberately:
- *Transient* (normal forms): errors ride the command's short SSE patch of
  the form fragment. A concurrent snapshot re-render will wipe them —
  acceptable for single-shot forms, and the inputs themselves survive
  morphing because the draft lives in signals.
- *Durable* (wizards, long drafts): the draft and its errors live in a
  per-session KV entry, so the snapshot itself renders them and they
  survive reconnects and tab moves. See §2.6.

**Components:** `kit/form`, `kit/field` (label+input+error slot, ARIA
wired), `kit/button` with busy indicator bound to a request-in-flight
signal.

**Gotchas:** don't render the draft's value from the server while the user
types (morph would fight the keyboard — server renders the *initial*
value; the live value is the signal); reset signals explicitly on success
(patch signals via `MarshalAndPatchSignals`).

### 2.2 Dashboard / live data

**When:** many independently-updating regions: stats, feeds, monitors. The
Throughline home screen shape.

**Wiring:** identical to §1 — producers (pollers, watchers, job events)
publish to subjects/KV; the one stream watches them all; one snapshot.
Resist per-panel streams: one tab, one stream.

**Key components:** `kit/stat`, `kit/sparkline` (SVG, server-rendered),
`kit/feed`. Charts beyond static SVG are an L2 island (§3).

**Gotchas:** rely on the bridge's coalescing + throttle floor — a 100Hz
producer must not mean 100 renders/s; if snapshot render cost grows
painful (profile first), split the *render* into per-region templ
components and patch only changed regions — still one stream, still
idempotent per region; keep time-formatting deterministic (server renders
timestamps; a `data-text` relative-time off a clock signal is acceptable
ephemera).

### 2.3 Long-running jobs with progress

**When:** imports, exports, indexing, batch AI calls — anything beyond ~1s.

**Wiring:**
```
POST /import  -> jobs.Start(...) -> 204 immediately
                     |
        goroutine (context.WithoutCancel from request)
                     |-- p.Set(i, total, msg) --> KV jobs.<id>
                     |-- terminal state --------> KV + SQLite history
stream: watches jobs.> for the session -> snapshot renders progress bar
```

```go
func (a *App) startImport(w http.ResponseWriter, r *http.Request) {
	a.jobs.Start(r.Context(), "import", func(ctx context.Context, p *jobs.Progress) error {
		for i, f := range files {
			if err := importOne(ctx, f); err != nil {
				return err
			}
			p.Set(i+1, len(files), f.Name) // -> KV put -> every stream re-renders
		}
		return nil
	})
	w.WriteHeader(http.StatusNoContent)
}
```

**Key components:** `kit/progress`, `kit/job-row` (status, progress,
cancel button → POST `/jobs/{id}/cancel`).

**Gotchas:** the job's context must be the app's, not the request's (the
POST returns instantly; `context.WithoutCancel` or a kit-owned parent);
progress in KV means reconnect/multi-tab/multi-window all show the bar for
free; write terminal results to SQLite (KV is reactive state, not history
— D9); cap progress-publish rate in `jobs.Progress` itself so tight loops
don't spam the bus.

### 2.4 List + detail (master/detail)

**When:** mail-style two-pane screens; entity browsers.

**Wiring — default: identity in the URL.** `/notes/123` is a real
navigation; the server renders list-with-selection + detail as one
snapshot; back/forward and deep links work because the browser owns
history. This is not client-side routing — every URL is a server render.

**Alternative — selection as state:** for tool-like panes where history
shouldn't grow (a log viewer's selected row), selection is a command
(`POST /select`) into per-window KV; the snapshot renders both panes. Use
sparingly; URLs are better.

**Key components:** `kit/split-pane` (CSS-only sizing), `kit/list-row`
with `data-on-click="@post(...)"` or a plain `<a href>`.

**Gotchas:** keyboard selection (j/k) on the URL variant means full
navigations — fine on loopback, do not "optimize" it into client routing;
preserve list scroll position (morph keeps it if the list element identity
is stable — keep stable `id`s on rows).

### 2.5 Command palette (kit-standard, keyboard-first)

**When:** every kit app. The palette ships in the shell by default;
keyboard-first is kit policy.

**Wiring:**
```
global keydown (mod+k) -> toggles `palOpen` signal (pure client, L1)
input --data-bind--> palQ signal --debounced POST /palette--> server:
    search command registry (app.cue nav + registered Go commands
    + Assistant-backed docs search) -> SSE patch of result list
arrow keys -> move `palIdx` signal (pure client)
Enter -> POST /palette/run with palQ + palIdx -> command executes ->
    palette closes (signal patch), effects arrive via the stream
```

**Key components:** `kit/palette` (dialog, input, result list slot),
command registry in `appkit/cmd` — commands declared in Go, surfaced from
the app manifest's nav section so the palette knows every screen by
construction.

**Gotchas:** open/closed and highlight index are exactly what signals are
for — do not round-trip arrow keys; do round-trip the search (server owns
the registry; results are rendered HTML, not JSON — §5.5); make every
palette command a real POST handler so agents and tests can invoke them
without a keyboard.

### 2.6 Wizard / multi-step

**When:** setup flows, multi-page forms, anything with a draft spanning
steps.

**Wiring:** step identity in the URL (`/setup/2` — back button = previous
step, free); the draft in a per-session KV entry keyed by wizard instance;
each step's POST validates *that step's* CUE schema and advances; the
final POST validates the whole draft, commits to SQLite, deletes the
draft. Because the draft is KV state rendered into the snapshot (§2.1
durable tier), a crashed tab resumes mid-wizard with errors intact.

**Key components:** `kit/steps` (progress header), per-step form
components reusing §2.1.

**Gotchas:** validate per-step on advance but re-validate everything at
commit (steps can invalidate each other); give drafts a TTL so abandoned
wizards don't accrete; never store secrets in the draft KV entry —
capability-interface keychain only (D14).

### 2.7 Multi-window desktop

**When:** desktop apps with inspector/detail/utility windows (Wails v3
multi-window, ADR-0006).

**Wiring:** each window is a webview pointed at the loopback URL with its
own route (`/windows/main`, `/windows/inspector`) — each route is a normal
page with its own per-tab stream (a window is a tab, as far as the kit
shape cares). Shared state lives in KV; cross-window updates are just the
render loop. **Cross-window communication never goes through shell JS
bindings** (forbidden by ADR-0005): a window issues a command; other
windows see the KV change.

**Key components:** `appkit/shell` window helpers (open/close/position via
the capability interface), per-window KV namespace
(`ui.win.<id>.*`) for layout state worth persisting.

**Gotchas:** scope "selected item" carefully — app-scoped KV means every
window follows the selection (inspector pattern, usually right);
window-scoped KV means independent panes (compare pattern); choose
explicitly per window; window lifecycle events (closed) must clean up
window-scoped KV entries.

### 2.8 Notifications / toasts

**When:** transient feedback ("saved", "import finished"), warnings.

**Wiring:** toasts are *state with a TTL*, not DOM side effects. A
`kit/toast` service appends to a session-scoped KV list with expiry; the
shell's snapshot renders the toast region from that list; dismiss is a
POST that removes the entry; expiry is a server timer that removes it and
lets the loop re-render. Native OS notifications (window unfocused) go
through the capability interface (ADR-0005) from the same service call.

**Gotchas:** this design exists because fat morphs *will* wipe any toast
that was injected client-side or appended outside the snapshot — if it
isn't in the model, it doesn't exist; keep TTL logic server-side (no
client timers); toast actions ("Undo") are commands like any other.

### 2.9 Settings screens

**When:** every app; settings are config, and config is CUE (D7).

**Wiring:** the settings form renders from the app's CUE-validated config;
save is a POST → validate via embedded `cuecontext` against the config
schema → write config → publish `config.changed` → every stream
re-renders, so the whole app reflects the change instantly (theme flips,
feature toggles). Validation errors render per §2.1 with the CUE error
mapped to fields.

**Key components:** `kit/settings-page` (sections from schema structure),
`kit/field` per setting. Long-term, knob components generated from the CUE
schema — the Gallery already needs that machinery for component knobs.

**Gotchas:** CUE error messages are the project's weakest DX (per the CUE
research, 2026-06) — map them to field-level messages in `appkit/config`,
don't surface raw unification errors to the screen; secrets are not
config: keychain via capability interface, with settings holding only a
reference.

### 2.10 Empty states / onboarding

**When:** first run, zero-data screens, feature discovery.

**Wiring:** an empty state is just a server-side branch in the snapshot —
`if len(m.Notes) == 0 { @kit.EmptyState(...) }`. Onboarding is KV flags
(`onboarding.<step>` per user); checklists render from those flags and
each completed action's command sets its flag; dismissing onboarding is a
command. No client-side tour library, ever.

**Key components:** `kit/empty-state` (icon, message, one primary action),
`kit/checklist`.

**Gotchas:** design empty states at component-authoring time, not as an
afterthought — the Gallery renders every component's empty variant as a
knob; make the primary action *do* the thing (create the first note), not
link to docs.

---

## 3. The escape-hatch ladder

For interactivity that attributes can't express: canvas, code editors,
maps, audio. Climb one rung at a time; each rung has an admission test.
Failing the test means you stay on the rung below and simplify the design.

**L0 — pure attributes (the default).**
Server renders truth; `data-bind`/`data-on`/`data-show` wire intent.
*Admission test:* none — this is where everything starts and where ~90% of
screens end.

**L1 — inline `data-on` expressions.**
Small client-only behaviors written as expressions in the attribute:
toggling a signal, focusing an element, palette arrow keys.
*Admission test:* the behavior is purely ephemeral view manipulation; the
expression fits on one line; no state outlives the element; deleting it
loses polish, not correctness.

**L2 — vendored JS island as a web component, on the signals bridge.**
A custom element wrapping specialized rendering (CodeMirror-class editor,
map, canvas chart), vendored as one pinned file, embedded via `go:embed`.
*Admission test (all must hold):* the interaction needs frame-rate-local
feedback or third-party rendering that cannot round-trip; the component is
expressible as props-in/events-out; it never fetches, never routes, never
owns durable state; it is vendorable as a single version-pinned artifact.

**The signals-bridge contract** (the only legal island interface):

- **Props in via attributes.** The server renders the island's attributes
  in the snapshot; morphing updates them on re-render;
  `attributeChangedCallback` is the island's input channel. (Signal-driven
  attributes via Datastar's attribute-binding are allowed for ephemeral
  props; server-rendered attributes are the default.)
- **Events out via `CustomEvent`.** The island dispatches named, bubbling
  CustomEvents; the page listens with `data-on-<event>` and turns them
  into signals updates or commands. For form-shaped values, the island
  may mirror its value into a hidden input bound to a signal — then it
  participates in §2.1 unchanged.
- **Pinned and embedded.** One file, version in the filename, checksum
  recorded, served by the app itself:

```go
//go:embed islands/kit-editor.v1.min.js
var editorJS []byte // pinned vendored bundle: version in filename,
// upstream version + checksum recorded alongside; no CDN
```

```html
<kit-editor id="cfg" language="cue" value={ m.ConfigText }
            data-on-editor-change="@post('/config/draft')">
</kit-editor>
```

- **The island is a leaf.** It may not import other islands, talk to the
  bus, open connections, or know URLs. Everything crosses the bridge.

**L3 — dedicated page with custom JS (rare, documented).**
The component *is* the page — a full DAW-style editor, a game surface.
*Admission test:* the bridge contract demonstrably breaks (chatty
bidirectional state that saturates the attribute/event interface); the
page gets its own mini-ADR in the app repo naming what it gives up
(snapshot convergence, multi-tab) and its exit plan. The rest of the app
remains on the kit shape; the L3 page still receives durable state via
commands + SSE, not a private JSON API (§5.5).

The boundary discipline: Datastar Pro attributes (ADR-0003) are dev-time
and private-overlay only — they never appear in island contracts or any
OSS-able layer.

---

## 4. Component sourcing

Priority order when a screen needs a component:

1. **Kit components first** (`appkit/ui`): templ components styled
   exclusively with kit design tokens (CUE-defined, D7, emitted as
   Tailwind `@theme` CSS). If the kit has it, use it; if it almost has
   it, extend the kit — that's the propagation flywheel (VISION §3).
2. **TailwindPlus corpus via the Assistant — as design vocabulary.** 657
   components × ~12 variants (D10), queried through the Assistant's
   search-then-get tools in HTML + Tailwind v4 form. It is *reference
   material to adapt* into templ + kit tokens, never markup to paste
   verbatim with raw palette classes — and it is licensed: derived kit
   components are ours, the corpus itself ships in nothing OSS (D10,
   VISION non-goals).
3. **Vendored templUI / DatastarUI as starting material.** templUI
   v1.12.0 (2026-06-01, MIT core, shadcn-style "own your code" CLI) and
   DatastarUI (shadcn-style templ+Datastar components, MIT, active
   2026-06) get vendored *into the kit once* (D13), adapted to kit tokens
   and the kit shape, then maintained as ours. Their JS conventions are
   replaced by Datastar attributes or a bridge island; upstream divergence
   is accepted — that's what "own your code" means.
4. **Never runtime-import third-party component libraries.** No module
   dependency on templUI/DatastarUI/daisyUI-for-templ; no CSS or JS from a
   CDN, ever. A component the kit didn't absorb is a component the kit
   doesn't have yet.

Icons: one convention, Lucide via per-icon templ components
(linker-pruned; per the 2026-06 templ ecosystem survey) wrapped as
`kit/icon`. Apps never pick their own icon package.

---

## 5. Anti-patterns

Named so reviews and agents can cite them by number.

**5.1 Signals as a state store.** Accumulating app state in signals —
lists, entities, derived data — rebuilds the SPA inside Datastar and
forks truth away from the server. Signals hold drafts and ephemera; if
losing it on reload would matter, it's a command and KV/SQLite.
*Instead:* §1, rule 4.

**5.2 Chatty per-keystroke POSTs without debounce.** Loopback is fast;
it is not free. A keystroke storm is wasted renders and bus churn.
*Instead:* `data-on`'s debounce modifier (300–500ms) for live search and
validation; plain submit for everything else.

**5.3 Optimistic UI.** Rendering predicted state, then reconciling. We
have sub-ms round trips and an explicit framework stance against it.
*Instead:* server truth + loading indicator; §2.3 for genuinely long work.

**5.4 Client-side routing.** Hash routers, pushState page swaps, "SPA
mode". The browser already has a router; every URL is a server render,
and back/forward must work by construction. *Instead:* real navigations
(§2.4); per-window routes (§2.7).

**5.5 JSON APIs for your own UI.** Endpoints returning JSON for client
code to template into DOM — that's a second rendering engine and a schema
to drift. The kit's UI wire format is HTML patches over SSE, full stop.
JSON crosses the wire only as signals payloads (Datastar's own contract)
or for genuinely external consumers — and an external API is a product
decision, not a UI shortcut. *Instead:* render templ on the server; patch
elements.

---

*Cross-references:* ADR-0003 (hypermedia-first),
`../adr/0004-templ-templates.md` (templates),
`../adr/0005-loopback-invariant.md` (transport),
`../adr/0008-embedded-nats-spine.md` (the loop's engine),
`../adr/0014-security-baseline.md` (CSP, data-ignore fencing),
`../adr/0012-gallery-living-style-guide.md` (where every pattern here is
demonstrated live).
