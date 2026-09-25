# Frozen native disclosure diagnostic source

Preserved during the 2026-09-18 workspace landing from the local
`.native-scroll-handoff-work.FHB3fc` preparation directory. These are historical
first-party diagnostic variants, not production changes or a new native acceptance.
The [assessment](../../2026-09-16-native-disclosure-scroll.md) records the exact
experiment, source/binary/config identities and **failed** disclosure/fixture outcome.

- `native_workflow_driver.swift`: exact bounded AX workflow and one advertised scroll
  fallback; source SHA-256 `f91b08749630de5122966f4d88c5ab7378cfd539825597beba2b121386ac5fbb`.
- `native_workflow_driver_test.swift`: exact pure selector/refusal tests; SHA-256
  `051df6408c8b09b95b80996039810482bfd254c3457b11ff134969fa85544e72`.
- `launchservices_probe.mjs`: exact historical lifecycle/workflow integration; SHA-256
  `d9614dcb7c734d0b3821a1b2bdb45cf6118fecaaa96c5abbfbba241b2644d032`.

This is not a standalone runnable directory. The probe deliberately still requires
the original pinned workflow binary, checks/config JSON and adjacent
`native_delivery_probe.mjs`; the latter and unchanged lifecycle tests are maintained
in [the Wails spike](../../../../spikes/wails-gallery/). Private runtime inputs and
receipts remain at the assessment's retained locations and are not committed.

To revisit, select R6 in the [resumption guide](../../../design/stabilization-and-resumption.md),
verify all source/runtime/config identities and prepare a new isolated invocation.
Do not overwrite the maintained generic probe, weaken the strict outcome checks or
rerun this documented no-op scroll simply to seek a pass. No native application was
launched and no permission, signing or installation setting changed during preservation.
