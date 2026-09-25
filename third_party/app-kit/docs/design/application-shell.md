# Application-shell reference journey

Experimental Gallery `/shell` demonstrates synthetic saves/recovery, not a production account/session API.
[Experience standards](experience-system.md) and [scope](../intent/scope.md) own promotion.

## Task, scene and reference

Geoff updates his display name/preferences, recovers rejected changes and finds help
without losing navigation context. Explicit light/dark overrides, semantic colors,
system type, clear sections and visible mobile links support daytime/evening use.

The complete Catalyst settings composition informed description/control columns,
section dividers, readable spacing and a clear final action row. Exact private source:
archive `catalyst@94acb41796a8a97fdef44bf5b2d18f42919e43300cbca90077775f6b36dfa529`,
member `catalyst-ui-kit/demo/typescript/src/app/(app)/settings/page.tsx`, SHA-256
`1ca7f4e14738e917390815dc38d48047588eb324090d337f5c718b9d2ec6fa21`.
Verified archive reads only; no vendor execution or claimed React visual/behavior parity.

Car Assistant's `/profiles` and Growing Gardens' `/data/settings` hold domain inputs,
not accounts. Adoption is deferred; fixtures must not replace app-owned auth/persistence.

## Working routes and states

| Route | Task |
|---|---|
| `/shell` (`/shell/home` alias) | saved session summary and configured workspace entry |
| `/shell/activity` | bounded synthetic change history, not a durable audit log |
| `/shell/profile` | display name, read-only identity, validation and versioned save |
| `/shell/settings` | time zone, starting page, digest preference and separate appearance |
| `/shell/help` | native disclosures with actual recovery/about destinations |
| `/shell/about` | allowlisted copyable build summary, no automatic diagnostic upload |
| `/shell/action` | POST-only save, fixture, recovery, simulated sign-in/out or reset |
| `/shell/events` | explicitly requested synthetic snapshot stream; no automatic page connection |

HTML GET pages accept only optional `theme=light|dark`; internal links use the browser's
persisted appearance choice. Actions accept no query parameters. Unknown routes return
404, wrong methods 405, malformed fields 400, wrong media 415 and bodies over 8 KiB 413.
These local routes require loopback peer/Host and same-origin requests, with no-store,
nosniff and nonce-based script CSP. Every POST needs the session's random CSRF token.
`Referrer-Policy: same-origin` preserves native POST provenance; null/cross-origin stays refused.

The handler owns at most 128 synthetic sessions, each with a fixed 30-minute lifetime,
random HttpOnly/SameSite=Strict cookie scoped to `/shell`, and no disk/database writes.
TLS also sets Secure. Reads do not extend expiry. Capacity refuses new sessions with
503 without evicting live ones; expired submissions return 403 and do not create state.
Reset changes only the current session. This is not a reusable production session layer.

Profile/settings writes recheck signed-in status, editor permission, data availability
and optimistic version under the session lock. Invalid fields return 422 with linked
errors and retained input. Read-only writes return 403, signed-out writes 401. A stale
version returns 409 without overwriting newer values; the response shows both the saved
snapshot and retained draft. **Apply reviewed draft** is an explicit new, version-checked
submission, not automatic last-write-wins. One-shot save failure returns 503 without
saving and permits retry. Success returns 303 to a real page with one-use saved feedback.

Loading is a paused specimen; loading/error recovery explicitly restores ready state.
Unavailable/signed-out submissions retain copyable drafts before recovery navigation,
not an authenticated draft persistence service.
The digest is a preference fixture only: no email or job is sent or scheduled.
## Composition and app-owned seams

`dashboard.RenderShell` owns the frame; `RenderNav` owns safe typed destinations and
current matching. The shell's skip destination now has `tabindex=-1`. Profile and
settings use existing form controls; optional checkbox `Invalid`/`HasHelp` now matches
text-control error/help wiring without changing old default markup. Message nodes
remain the caller's responsibility. Appearance keeps its existing browser-local API.

Experimental `dashboard.AccountDestinations{Profile, Settings, Help, About}` defines
optional app-owned `NavItem`s once. Pass `account.Group("Your account")` alongside
primary groups to `RenderNav` for sidebar and account navigation. Custom links still
need app-owned URL policy; escaping alone does not reproduce disabled-item treatment.
Neither URL nor label means omitted; a URL gets a default role label; a label alone
stays disabled. `RenderNotice(Notice{Kind, Title, Body})` escapes feedback and separates
status from error/conflict attention; the app, not HTTP inference in the kit, selects it.
The compiled `ExampleAccountDestinations_Group` shows composition. Existing APIs remain.
Gallery now consumes both helpers; its stores, permission switches, draft scripts,
forms and saves stay private fixtures. Keep identity/schema/persistence/recovery app-owned.
The [incubation](../adr/0063-incubate-account-presentation-recipe.md) is not permanent
promotion; `shell.account-workflow` remains experimental. [Extraction proof](../assessments/2026-09-16-account-presentation-recipe.md) covers215 browser checks and source refusal; MCP serves this exact guide.

Form action rows can use `RenderButton` for cancel/help links. Nonempty unsafe or
Disabled destinations render noninteractive labels, not submit controls; relative
references and absolute HTTP(S) remain available. App-owned route authorization and
unavailability explanations remain necessary ([ADR-0079](../adr/0079-disable-unsafe-form-button-links.md)).

Experimental `form.RenderSettingsSections([]form.SettingsSection{...})` now owns the
five Profile/Settings description/control rows. Titles/descriptions/heading IDs are
escaped; duplicate or invalid IDs and blank titles return an error without partial HTML.
Call before writing the response. Supply an h2 parent, document-unique IDs and reviewed
content from existing labeled controls; scan `appkit/ui/form/*.go` for CSS. Rows stack
below sm, use two columns above and wrap long text in both columns, including native
control labels/help/errors; reviewed content may override inherited wrapping.
The renderer adds no form,
fieldset, label, submit action, script or save authority. Appearance remains independent.
`shell.settings-sections` and [ADR-0064](../adr/0064-incubate-responsive-settings-sections.md)
keep this presentation experimental; Gallery's controllers are not consumer APIs.
The [section proof](../assessments/2026-09-16-settings-section-recipe.md) retains72 long-text
layout checks and215 real-route form checks; neither is two-consumer acceptance.
The [control-column repair](../assessments/2026-09-22-settings-control-reflow.md) adds
76 bounded no-script text/native-key checks without changing controls or save behavior;
standalone controls and full zoom/accessibility acceptance remain separate.

Saving works without scripts. The optional Gallery script tracks dirty fields, warns
on departure where permitted and labels native submission progress. Keyboard **Resume
editing** returns focus to the first changed enabled field; input/change recovery
preserves current focus. It cannot undo an accepted write; retries recheck version.
An interrupted save leaves a document-local **outcome unknown** state. Continued edits,
input/change pairs, history restoration and reverting to the old baseline cannot resolve
it. Keep the warning, departure guard and explicit Save available; a new submission
still shows pending progress and carries the original version until a server response.
Do not repeatedly rewrite an unchanged polite live-region message during typing.
[Browser-Stop recovery](../assessments/2026-09-14-shell-save-recovery.md) proves retained drafts and explicit conflict review.
[Inline Resume](../assessments/2026-09-14-shell-inline-recovery.md) adds 24 pending-navigation cases, twice: automated keyboard activation closes the held request before fixture release; accepted writes still require conflict review.
No automatic repost is added. [Pinned Chrome history proof](../assessments/2026-09-14-shell-history.md): native autocomplete-off keeps network reloads canonical; BFCache keeps draft/version pairs. [Profile typing proof](../assessments/2026-09-15-shell-typing-recovery.md) adds eight held-save journeys, continued-edit/baseline-revert recovery and real canceled-departure dialogs. [Settings input proof](../assessments/2026-09-15-shell-settings-input.md) independently exercises both selects and the checkbox:24 journeys/1,218 checks; Profile regression418. Other engines, IME, select popup/pointer paths and pending-save cache remain open.

## Sensitive native forms: consumer return

RAC Settings exposed a secret in a GET URL when its JS-only form lost enhancement.
The [consumer repair](../assessments/2026-09-14-resume-settings-native.md) keeps the
Synthwave shell and local writer, adding actual native save/error/recovery. Lessons:

- Declare native method/action; prove real no-script POST under served headers.
- Keep exact-origin/CSRF checks and bounded singleton fields; never broadly exempt
  forms from a JSON guard. A kit input component supplies none of this authority.
- Secret-bearing forms use POST/303/GET with only non-secret error state retained;
  re-enter secrets after failure. Never reflect them into URLs, cookies or responses.
- Refuse credentials before generic hosted mutation queues read/serialize a body.
- A stale setup form must say existing credentials were kept, not claim a new save.

For non-secret drafts, retain Gallery's 422/409 recovery; session policy stays app-owned.
The [consumer-fit audit](../assessments/2026-09-15-shell-settings-input.md#consumer-fit-reuse-behavior-not-invented-authority)
finds shared RAC/Fathom navigation, not two comparable account-save backends. Keep
recovery guidance app-owned until repeated needs prove a shared runtime boundary.
Fathom's inspected read-only routes need degraded-data navigation proof, not invented
mutations. RAC's sensitive credential flow must never inherit non-secret draft retention.
The [authorized reader repair](../assessments/2026-09-15-fathom-handler-read-failure.md#authorized-reader-adoption)
fixes unreadable-as-zero with actual-handler/browser proof. The subsequent
[authorized display repair](../assessments/2026-09-16-fathom-display-adoption.md) fixes
aggregate freshness and local diagnostic/grade clipping with synthetic handler/browser
proof. Other missingness displays and full consumer acceptance remain open. Correct
exported models still cannot replace source-reader fault injection through the real
handler and browser; scoped reader/display proof is not full acceptance.

The [executed G2 follow-up](../assessments/2026-09-16-fathom-shell-acceptance.md)
passes four actual-handler narrow/wide light/dark journeys with native navigation,
long service labels and exact recovered traffic after the unchanged30s cache expiry.
Fixed primary navigation and long service-label stress are distinct observations.
Consumer source/pins stay unchanged. The [G2 acceptance matrix](../assessments/2026-09-16-shell-acceptance-matrix.md)
joins this proof with Gallery and RAC's actual Settings recovery; its finite behavior/
adoption exit test is met, not the broader starter quality or release gates.
A test-only cache reset cannot prove production recovery; G2's behavior acceptance
is separate from permanent promotion of every experimental helper.
Measure styled geometry only after bounded stylesheet readiness, never by polling
until overflow disappears. RAC's retained pre-CSS failure and missing-CSS/styled-overflow
controls demonstrate why; first-paint/degraded-network claims need separate measurements.

## Explicit streaming transport specimen

`GET /shell/events` observes the same saved synthetic state. It requires one live,
signed-in session cookie and exact `Accept: text/event-stream`, with no query/body;
it never starts or extends a session. Same-origin/loopback guards remain mandatory.
Each connection receives the current full snapshot, then coalesced saved-state updates.
An optional canonical `Last-Event-ID` does not request history: reconnect resnapshots,
including for an old or future cursor. Version is a decimal string, not a JS number.
The allowlisted projection omits cookies, CSRF tokens and fixture failure controls.

Bounds are four connections per session, 32 total, 8 KiB per frame and five seconds
per write/flush; capacity returns 429. Disconnect releases resources; fixed expiry or
sign-out attempts a redacted terminal patch and closes. Already in-flight bytes cannot
be recalled. Writes do not hold the store lock, and intermediate states may coalesce.
The command cancels request contexts before bounded shutdown on SIGINT/SIGTERM.

Signed-in Activity alone now offers an **experimental pinned Datastar v1.0.3 observer**.
Connect explicitly requests a full snapshot; Stop, pagehide or hiding the tab aborts it.
Showing the page never reconnects automatically. One request settles before another starts.
The dedicated empty form sends no signals, edit versions or CSRF token; retries are zero.
Only this page adds same-origin `connect-src` and a per-response `data-nonce`; no
`unsafe-eval`/`unsafe-inline`, external CDN, profile/settings client or automatic stream.
Observed activity uses text nodes and exact decimal versions. Paused/failed connections
label retained data stale; null/401/403 clears observer content and its signal namespace.
Reconnect accepts a new session's full snapshot. It never replaces a draft or save version.
No-script/module-failure fallback is the complete ordinary page with Reload activity.
Pin, upstream source findings, tests and open browser gates: [observer evidence](../assessments/2026-09-13-datastar-observer.md).
A lost POST response leaves its outcome unknown; read saved state before an explicit
version-checked retry. Neither this stream nor a 409 provides mutation replay semantics.
[Evidence and remaining platform matrix](../assessments/2026-09-13-streaming-proof.md).

## Evidence and remaining work

The [local performance baseline](../assessments/2026-09-16-shell-performance.md) passes
30 ready/error/retry observations on an exact binary, including no-script dark mode,
within declared payload/navigation budgets. Failed harness attempts remain retained;
useful-paint, input/update, memory, warm-cache and degraded-condition proof remain open.

`task check` covers HTTP parsing, sessions, 422/409/503 recovery, permission/expiry/capacity,
concurrency, escaping, labels, diagnostics and CSP/CSRF/authority refusal; Node checks cover
draft/focus state and the test transport/evidence canaries. Browser runs remain opt-in.
`TestShellDemoLoopbackJourney` covers 34 HTTP requests; opt-in `TestShellDemoLiveJourney`
uses `APPKIT_PROBE_SHELL_URL`. Synthetic budgets: HTML ≤64 KiB (rejected drafts ≤256 KiB),
draft script ≤8 KiB, observer scripts ≤48 KiB raw/≤20 KiB individually gzipped; no latency claim.
[Browser probes](../../apps/gallery/browser/README.md): 125 shell and 26 observer checks;
[original receipt](../assessments/2026-09-13-shell-browser.md) plus [1025 interruption checks](../assessments/2026-09-14-shell-settings-recovery.md)
over 24 Profile/Settings scenarios, replayed with fresh contexts. Stop acknowledgement is not client
disconnect: fault injection waits for the latter before teardown. Exact extra writes fail.
Full keyboard/contrast, zoom, IME/select popup/pointer paths, native and human performance
acceptance remain open. CA/GGB stay deferred; G2 is satisfied by the combined RAC/Fathom
evidence in the acceptance matrix above, not by RAC alone or by these historical probes.

RAC's [navigation delivery review](../assessments/2026-09-14-navigation-delivery.md)
keeps hit area, active-text contrast and focus contrast as separate gates. Use actual
visibility, not layout rectangles, for closed native disclosures; traverse app-owned
header actions before assuming a menu's tab position. A skip fragment can restore
focus after reload, so independent keyboard trials start fresh documents. Source/CSS
hashes, browser behavior and sibling-free builds each need their own proof.
