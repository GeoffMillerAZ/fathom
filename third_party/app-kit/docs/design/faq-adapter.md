# P03 typed FAQ and native help disclosures

Status: implemented private candidate with [scoped browser/standalone evidence](../assessments/2026-09-14-faq-typed-adapter.md), not a certified capability.
[ADR-0041](../adr/0041-explicit-faq-reference-and-native-details.md) extends the
[Elements proof](../assessments/2026-09-14-elements-conversion.md), without changing
its rule, frozen source or existing attempts. Delivery follows ADR-0036.

## Tool entrypoint

`appkit data assets-adapt-faq -root PRIVATE_DIRECTORY` emits private JSON. Add either
`-out NEW_PRIVATE_DIRECTORY` for six immutable files or `-check PRIVATE_DIRECTORY`
for read-only exact replay. Export/check success is silent; exit 1 means observed
drift and 2 means invalid request/source or could-not-check. Existing exports are
never replaced or repaired. Read the exported `USAGE.md` before compiling a copy.

## Task, model and ownership

A person seeking help on a phone or desktop scans questions, opens relevant answers
and keeps several visible. Preserve source light/dark choices, not a new palette.
`FAQ` supplies `IDPrefix`, `Heading` and ordered `Items`; each `FAQItem` supplies `Key`,
`Question`, `Answer` and `Open`. All strings are escaped plain text, not HTML or templ.
The app owns wording/localization, authorization, composition and empty/error states.

Require 1–24 items. Prefixes match `[A-Za-z][A-Za-z0-9_-]{0,63}`; unique keys match
`[A-Za-z0-9_]{1,64}` without hyphens. IDs are `prefix-key`: excluding key hyphens
prevents ambiguous concatenation across distinct prefixes. Heading and
questions are nonblank UTF-8, at most 256/512 bytes; answers are nonblank UTF-8, at
most 8192 bytes. Reject controls except tab/newline in answers. Snapshot input items.
Reserve derived panel IDs and choose distinct prefixes across instances. Extracted
default source IDs and text must round-trip exactly.

| Constructor | Behavior | Runtime |
|---|---|---|
| `NewFAQReference` | Original definition-list/button/Elements structure and classes | Host-provided Elements 1.0.22 |
| `NewFAQDetails` | Native independent disclosures, wrapping/focus/44px targets | Browser HTML/CSS; no JavaScript |

Neither is an exclusive accordion. Initial `Open` is presentation input; subsequent
state belongs to the browser until navigation/remount. Remount uses the supplied
model again; preserving user state across morphs is not implemented. Native output
contains no Elements tags/commands. Reference output never gains native-only CSS.

## Source mapping and acceptance

Require exact P03 archive/member/hash and independent archive access. Start with
`TranslateElements`; refuse unknown structure/bindings. Extract heading, six item
defaults, IDs, open states, classes and icon geometry, never paste paid source into
tracked templates. Preserve default text whitespace and namespace-aware original DOM.
Native markup explicitly replaces list/term/button/custom-panel structure with
`details`/`summary` and answer containers. Keep palette, content, spacing and icon
geometry; map icons to native `open`. Scoped CSS owns marker suppression, targets,
focus and wrapping. This mapping is not claimed as vendor behavior.

- Exact replay refuses source, rule, defaults, template/model/CSS and runtime changes.
  Reference defaults match original DOM before/after templ formatting.
- Constructor tests cover malformed identities, duplicates, blank/oversized/invalid
  strings, excessive item counts, escaping, snapshots, cancellation and writer errors.
- Two differently prefixed instances operate independently at 390/1440, light/dark,
  keyboard/pointer and long content. Native answers remain reachable with JS disabled.
- Reference/source screenshots share pinned CSS/font/runtime and zero tolerance.
  Native screenshots are reviewed as adaptations, never used to claim parity.
- Measure source and native target/focus behavior separately. Full accessibility,
  touch, cross-browser and actual app adoption remain separate evidence gates.
- Reuse immutable fixed-file private delivery. Compile a copy in an app-owned module
  without app-kit imports or a sibling checkout. No install, route binding or mutation
  of previous exports. Every receipt remains `not-verified`.

## Revisit when

ADR-0041 names rich answers, deep links, exclusive/persisted state, morphing, source
drift, failed isolation/fidelity and repeated consumer demand as review triggers.
