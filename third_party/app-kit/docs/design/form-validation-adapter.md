# P05 email field and native recovery contract

Status: candidate implemented with [bounded evidence](../assessments/2026-09-14-form-typed-adapter.md), not a certified target.
Governed by [ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md),
[private delivery](../adr/0036-share-private-delivery-assurance.md) and the frozen
[P05 source](../assessments/2026-09-13-conversion-pilot.md). This explicitly expands
P05 beyond the vendor's error-only specimen; it does not expand the twelve-case count.

## Task and ownership

A settings user enters a contact email, receives an actionable rejection, corrects
the retained value, encounters a recoverable save failure, and retries successfully.
The native browser owns editing, selection, focus and form submission. The app owns
validation, authorization, CSRF, persistence and success truth. Go/templ ordinary HTTP
is sufficient; no JavaScript, Datastar, Wasm, NATS or validation framework is required.

## Reviewed presentation mapping

`EmailField` carries ID, Name, Label, Value, Placeholder, Error, Required and Autofocus.
`NewEmailField` validates bounded presentation data before output, not email validity.
Strings remain escaped; IDs/names have a conservative ASCII grammar. Blank values are
valid presentation inputs; malformed or oversized presentation inputs return errors.
The app must assign distinct IDs, reserve each `<ID>-error` for its description, and
handle constructor errors before response output.

With nonempty Error, retain the source layout, classes, icon, label, linked error and
`aria-invalid=true`. Defaults extracted from the independently verified original must
reproduce its parsed DOM. Required/Autofocus default false, so they add no attributes.
All original text/value/ID bindings remain plain data, never templ source or raw HTML.

With empty Error, omit the error icon and paragraph, `aria-describedby` and
`aria-invalid`. Keep layout and spacing; map only the error palette to a declared
neutral input palette: light text gray-900, outline gray-300, placeholder gray-400,
focus indigo-600; dark text white, outline white/10, placeholder gray-500 and focus
indigo-500. This clean state is an authored adaptation, not supplied vendor behavior.
Any unrecognized source structure or palette token fails closed, never drops a class.

No automatic focus is imposed. The fixture opts into Autofocus on invalid submissions;
save failures use an app-owned page-level message, not a false field-validation error.
That full-page response has a descriptive title and an autofocused, tabindex=-1 error
summary linking to the retained field. This is distinct from field-invalid autofocus.
The source remains independently accessible. No whole purchased body enters Git.

## Native workflow fixture and evidence required

The synthetic fixture is private rendered output from reproducible first-party test
code, not a reusable account backend. No real email is sent or persisted.
Native `type=email` constraints remain enabled: malformed input is blocked locally.
The server rejects the syntactically valid `blocked@example.test` by an explicit
synthetic domain rule, allowing the 422 path to be tested without disabling validation.
An isolated in-memory fixture session is the single state authority. It starts with
one injected save failure; valid submissions consume that failure once, and a retry
stores only the synthetic value for the confirmation GET. Each test starts a fresh
session and discards it afterward. Request tokens, body/cardinality checks and scoped
cookie handling in the fixture are test boundaries, not production auth/CSRF assurance.
Use `Referrer-Policy: same-origin` with strict native POST origin checks; `no-referrer`
can serialize a legitimate browser POST's Origin as null ([INC-16](../intent/incidents.md)).
Retain the null-origin refusal; verify actual browser headers, not only HTTP fixtures.

| State | Required observation |
|---|---|
| Initial GET | Neutral field, descriptive label, native submit, no false error |
| Invalid POST | 422, submitted value retained, specific associated error, focus on field |
| Corrected POST with injected save failure | 503, value retained, app-level retry guidance, field not falsely invalid |
| Retry success | 303 to a confirmation GET, value truthfully shown as synthetic, refresh does not resubmit |
| Slow request | Browser owns native navigation progress; no invented optimistic success |
| Bad request | Bounded body and expected field cardinality; reject without a success claim |

Test deterministic generation/replay, independent source hash rejection, missing
bindings, altered classes, malformed IDs, escaping and clean/error DOM invariants.
Compile a private copy with the pinned templ tool. Exercise actual native submissions
with JavaScript disabled at 390/1440 CSS px, light/dark, keyboard/pointer; inspect
ordinary and long labels, focused errors and retry states. Compare original/default
render at identical CSS/font/viewport settings. Original deficiencies remain visible;
fixture-only usability changes are recorded separately, never normalized into fidelity.

An export reuses existing internal no-clobber assurance and remains immutable private
input. No new public exporter, catalog certificate or app-kit runtime dependency.
Schema/replay/compile results alone never establish browser quality or app adoption.

## Revisit when

- Multiple fields need summaries, cross-field errors or repeated-group identity.
- A real app needs partial updates, offline drafts or optimistic persistence.
- The frozen source changes, clean-state mapping loses fidelity, or native recovery
  fails a keyboard/narrow-screen task. Review mappings/oracles before changing them.
