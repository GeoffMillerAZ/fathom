# P10: complete native settings composition

Accepted design: [ADR-0044](../adr/0044-native-catalyst-settings-composition.md).
Current state: a privately generated complete Go/templ form, typed policy and native
HTTP fixture have [scoped compiler/browser proof](../assessments/2026-09-14-catalyst-native-form.md).
Optional [local feedback/reset](../assessments/2026-09-14-catalyst-enhancement.md), [503 recovery/actual zoom](../assessments/2026-09-15-catalyst-recovery-zoom.md), [painted native focus](../assessments/2026-09-15-catalyst-native-focus.md) and [multiline draft/count repair](../assessments/2026-09-15-catalyst-textarea-transport.md) have scoped browser/replay proof; CLI/catalog exposure and certification remain open. The [hardened React reference](../assessments/2026-09-13-catalyst-hardened-reference.md)
remains independently usable; its older original failures were not rewritten as passes.

## Product and ownership

Geoff needs the purchased settings form's complete visual composition in a Go app,
with dependable editing and correction on desktop and narrow screens. Preserve all
five sections, paired description/control columns, restrained separators, typography,
light/dark appearance and complete supplied choices. Do not substitute a few generic
fields. Form-only fidelity is distinct from the source demo's navigation/account shell.
Consumers own that shell, labels/values, allowed options, auth, routes and persistence.

The native candidate replaces custom selects and a hidden-input/custom-checkbox pair
with actual browser controls. The closed control should retain the source hierarchy;
native popup rendering is deliberately platform-owned. Minimum 44px controls, explicit
error links and first-invalid focus are acceptance requirements, not source assertions.
No palette/token flattening is included; a kit-token adaptation would need separate review.
Generated consumers must serve `SettingsNativeStyles()` from `catalyst_native.go`'s
embedded `catalyst_native.css` after retained source CSS. It owns native focus and
control-size rules; these are not disposable fixture decoration or Headless state.

## Frozen source lineage

- Original: pilot P10, Catalyst archive `94acb41796a8a97fdef44bf5b2d18f42919e43300cbca90077775f6b36dfa529`,
  `catalyst-ui-kit/demo/typescript/src/app/(app)/settings/page.tsx`, member
  `1ca7f4e14738e917390815dc38d48047588eb324090d337f5c718b9d2ec6fa21`.
- Behavior reference: hardened-v3, selected source manifest
  `f8de11343773dbb8dbeb53bb713d684fee23fcd36604ddff921b07da57450ee2`.
- Original React behavior and the derivative's deliberate fixes remain separate
  assessments. A hydrated DOM capture is an input reference, not original TSX bytes.
- Freeze captured form, selected compiled CSS, complete pinned font/flag URLs, option
  data, default snapshot and source verifier/collector identities before adaptation.
  Source preparation requires the explicit existing private project and local origin.

## Native task contract

| Concern | Required behavior |
|---|---|
| Values | Exact app-selected draft retained; no trim, coercion or truncation on failure. Bound requests before processing. |
| Bio | Optional, at most240 Unicode code points of the textarea API value (HTML newline semantics); combining/ZWJ/modifiers remain separate. Preserve raw decoded values; reject malformed UTF-8 before counting. Serialization must preserve leading/trailing line breaks. |
| Email | Optional example ASCII addr-spec policy; reject display names and normalization. This is an authored example policy, not universal email validation. |
| Country/region | Exact canonical membership; preserve a region only if it belongs to the new country, otherwise require an explicit choice. |
| Submission | Native named controls; no serialized country/region-list authority, duplicate keys or unknown fields. Go checks independently of browser feedback. |
| Local feedback | Optional controller updates count/dependent options without requests, retains Country focus, clears stale success and handles correction. |
| No JavaScript | Fields remain usable; all canonical region choices available with country context, then ordinary HTTP validation/correction. Never a hydration-disabled form. |
| Reset | Native snapshot/focus restoration uses a separate reset form; Enter always validates. Opt-in local Reset restores all ten fields without replacing the focused button or making a request. |
| Focus | Bio before email before country/region before currency; app-defined framing errors remain distinct. Backend refusal focuses retained-draft/retry status; forward Tab follows Reset then Validate in source order. |
| Authority | Rendering/validation does not invent persistence; the initial fixture clearly states nothing is saved. Callbacks and auth belong to the host. |

## Implementation sequence and proof

1. Implemented: repeat the verified private source capture; 54 retained input/image
   files match byte-for-byte. Source-only tests remain independent of licensed data.
2. Implemented: pinned DOM/templ transformation and typed values/policy preserve all
   five sections with native controls. First-party helpers contain no paid markup.
3. Scoped proof: standalone compilation, escaping, malformed-input refusal, complete
   options, repeated-instance IDs and native draft/reset/Enter tasks; optional local
   feedback, disposal/remount and same-node reset; native CSS companion and exact sixteen-file binary replay.
4. Scoped target proof: 390/1440 tasks and real200% browser zoom, both themes and native/
   enhanced modes; full-frame capture guards preserve historical failures. Complete
   paired source/target fidelity and physical-input/full contrast review remain open.
   Bounded keyboard-outline pixels, computed text-color contrast and form-owned AX
   names/error descriptions pass; concurrent autofocus observation remains unresolved.
5. Scoped recovery proof: no input-time requests, all ten fields retained on503, native
   retry, first-invalid/status focus and44px geometry. The [measurement contract](catalyst-settings-performance.md)
   separates source/target form tasks from shell payload; measured budgets remain open.
   The [separate v2 cached transport](../assessments/2026-09-15-catalyst-font-alias-delivery.md) repairs actual font-URL coverage; [supplemental warm identity](../assessments/2026-09-15-catalyst-warm-font-identity.md) links two fonts, but direct warm-body and complete delivery/performance gates remain open.
   Responding503 is not dropped-connection recovery.
6. Only then expose an experimental CLI/catalog reference, private replay/delivery and
   revision-bound evidence. Full gates and real-consumer promotion remain additional work.

Rendering alone does not complete this sequence. No general TSX parser, Headless clone,
new public widget system, universal settings backend or automatic target promotion.
P10 is already part of G1/G4's admitted pilot; Windows and deferred apps stay out of scope.
