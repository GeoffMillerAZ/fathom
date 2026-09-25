# ADR-0079: Disable unsafe form-button links without creating form actions

- **Status:** Accepted
- **Type:** security
- **Date:** 2026-09-23
- **Decision drivers:** reusable safe defaults, native disabled semantics, static-site compatibility

## Context

RenderButton escapes Href as HTML, but escaping does not constrain URL schemes.
Its link branch also ignores Disabled. A settings cancel/help action can therefore
remain executable or navigable when the caller intended an unavailable action.
Known kit callers use fragments; indexed consumer callers use local routes.
The public Href field does not restrict ordinary document-relative static links.

## Decision

Keep Button's fields and native empty-Href button behavior. In nonempty-Href mode,
accept parsed relative references and absolute HTTP(S) with a nonempty hostname.
Reject leading/trailing whitespace, raw C0/DEL controls, backslashes, protocol-relative
references, malformed parse and other or opaque schemes. Preserve accepted bytes;
do not trim or rewrite a refused destination. HTTP(S) is not a trusted-origin claim.

Unsafe or explicitly Disabled links render a span with role="link", aria-disabled,
escaped label and explicit disabled styling. It has no href, tabindex, form attributes
or hover/focus affordance. Never convert a refused link into a submit button. The
app supplies any explanation, allowed origins/routes, permissions and action policy.
No client script, dependency, shared routing service or new exported API is needed.

This is a breaking tightening of Href/Disabled and rendered HTML. Adopters rebuild
source/scanned CSS, review snapshots and test their form task. Replace non-web scheme
uses with separately reviewed app-owned links; do not bypass refusal by stripping a
scheme. Valid enabled links and empty-Href buttons retain their existing bytes.
Consumer source/pins and dashboard navigation's separate matching policy stay unchanged.

## Alternatives considered

- Retain href with aria-disabled alone: native activation would still navigate.
- Use a disabled button fallback: changes the element's meaning and form contract.
- Accept only root-relative URLs: unnecessarily breaks static-file and fragment use.
- Export one global URL policy: navigation matching and app-origin rules have different
  present needs; this narrow repair does not justify a new shared public abstraction.

## Evidence boundary

Parsed-DOM canaries and a frozen no-script settings task must prove refusal, keyboard
skip, pointer inactivity, valid navigation and unchanged native submit. See the
[assessment](../assessments/2026-09-23-button-destinations.md). Narrow browser/AX proof
does not establish screen-reader, full accessibility, security or consumer acceptance.

## Revisit when

- A named consumer needs a refused scheme or legitimate destination shape.
- A browser interprets an accepted shape as an executable or non-web destination.
- Disabled actions activate, submit, capture focus or convey an enabled affordance.
