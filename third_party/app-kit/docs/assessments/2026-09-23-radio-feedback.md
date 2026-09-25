# Native radio-group feedback

## Prospective packet

Baseline `2a506b8`. `RenderRadioGroup` displays help/errors but supplies neither
description relationships nor explicit server-error invalid state. Repair the existing form contract,
not its native selection model. I4/I7/I12/I15 apply; no architecture, runtime,
consumer, dependency or installed-host change is authorized by this packet.

Keep input IDs, field names/values, requiredness and disabled-fieldset inheritance.
Error takes priority over help. For a nonempty Name, encode its bytes as hexadecimal
in a deterministic `ak-radio-<hex>-help|error` message ID, and associate every radio
with that message. Empty Name retains no generated relationship. Callers already
own meaningful group names and document-wide ID uniqueness. Set invalid state and
the existing danger focus role on errors; do not clear server errors on key input.
No-message output stays byte-identical. Native radio-circle styling is not promised.

## Fixed acceptance

- Parsed-DOM tests cover active help/error relationships, escaping, stable whitespace-
  free references, distinct groups, empty names, native attributes and exact no-message
  compatibility. Every relationship resolves to its rendered message.
- Freeze failed baseline tests and subsequent corrected oracle separately. Initial
  review found an invalid expectation that fieldset-disabled radios carry their own
  disabled attribute; preserve native inheritance instead of changing production.
- Run scoped race tests and vet; build frozen baseline/candidate synthetic responses.
- Same browser oracle in 1440/light and 390/dark: read error, Tab/Arrow to Weekly past
  disabled Paused, retain Email in a second group with a hostile field name, submit
  a native GET, then read the app-owned reviewed response with error cleared and help
  restored. Admit only the exact synthetic submitted fields; nothing is saved.
- Inspect actual Gallery forms and synthetic error/review screenshots. Require no
  page overflow, visible keyboard focus, completed exact document/CSS, no unexpected
  requests, and Chrome AX descriptions/invalid state. This is not screen-reader or
  whole-accessibility certification. JavaScript stays disabled.
- Retain executable, source, fixture, browser harness and response identities before
  acquisition, failures included. Reuse bounded browser-process cleanup. Limit this
  packet to 45 minutes from 10:37 UTC and at most one retained browser-harness repair.

Private evidence: `/Volumes/m2ext/appkit-evidence/appkit-radio-feedback.GvbYjsoS`.
No catalog/standards gate is promoted merely by these scoped checks.

## Results

`red` retains the original two failing tests. `red-reviewed` retains the corrected
disabled-inheritance oracle against unchanged production; both feedback tests still
fail. Its response bytes match the original baseline. Neither failed run is a pass.
`green` passes 1,210 test/subtest events with race detection across form, Gallery,
MCP and qualityplan, plus scoped vet and an offline fixture build. Tests preserve
exact no-message markup, original IDs, names/values and native attributes.

The frozen browser driver runs the same oracle against both snapshots using Chrome
153.0.8010.53, Playwright 1.62.1 and Node v26.3.0. At 1440×1000/light and
390×844/dark, baseline fails ten of 40 checks: missing error/help associations and
corresponding AX descriptions, including the actual Gallery radio group. Native
requiredness already makes enabled baseline radios invalid when empty; that is not
server-error wiring. The candidate passes all 40 checks, with no unexpected requests,
timeouts, forced termination or incomplete output capture.

Both snapshots retain native Tab/Arrow selection past the disabled option, exact
Weekly/Email form values and strict synthetic GET submission. The candidate's reviewed
response removes the server error/invalid attributes and restores associated help.
The fixture owns that response, not a new kit controller or persistence service.
Source-bound actual Gallery responses also pass native Tab/Arrow interaction.

All six candidate captures were inspected: error/focused, reviewed and actual Gallery
in both lanes. Text is readable without clipping; layout changes from two columns to
stacked rows. Focus capture includes the danger-token shadow and the browser's native
outline; native radio-circle recoloring is not claimed. Chrome AX captures establish
the tested descriptions/invalid state, not screen-reader usability or full accessibility.

Candidate fixture SHA-256:
`488f20b681418b0a0428d7f08319a783f6956954d72123743b5a23198579be36`.
Candidate response SHA-256:
`d7653a107fa0b6401be59eb162e47988068d4ee2d960b161d1ca22407165cf6c`.
Baseline/candidate browser inputs retain the executed driver, process-owner helper,
fixture, prospective contract and tool identities. The executed source overlays did
not drift. Independent production review found no remaining concrete issue.

Adopters rebuild the selected Go source and scanned CSS, then review markup snapshots
and their own form task. No consumer, installed tool, dependency or default theme was
changed. CUE links this evidence without promoting any standard or quality gate.
