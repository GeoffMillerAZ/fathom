# Native select control and conditional dark-guide resume

## Prospective contract

- Need: the [dark/200 acquisition](2026-09-23-fathom-dark-guide-paint.md) failed
  Scheme selection before its guide task. Both chart rasters are already observed;
  do not repeat them. Consumer source, CSS, dependencies and running services stay
  unchanged. Use the same pinned synthetic fixture and a fresh private Chrome profile.
- Control: at actual Appearance200, application scripts disabled, serve one minimal
  two-option native select via the local route harness. Compare the historical
  `Home/ArrowDown/Tab` sequence with `Space/ArrowDown/Enter/Tab`, each after fresh
  navigation to Light. Reach the select by native Tab. Record selected value/index,
  options, focus and CDP accessibility states after every key. No selectOption,
  JavaScript value assignment or injected event may select the fixture value.
- Control acceptance: the candidate must commit Dark before Tab, retain it after
  Tab and place focus on the next button. Historical outcome is observed, not forced
  to fail. Control success is not an app interaction or physical-keyboard pass.
- Conditional next step: only after control success, freeze a distinct guide-only
  driver using that sequence. At200: show dark source, invalid radius `-1px` →422
  focused alert → native repair link → `1rem` candidate/diff → reset source. Require
  actual dark iframe theme/token, completed exact CSS, script-free opaque previews,
  keyboard outcomes and five target-visible no-clip compositor captures. Record
  iframe and top-level scroll/focus. Inspect exact PNGs independently.
- Cap: one control acquisition and, only on its pass, one guide-only acquisition,
  each ≤180s. No automatic retry or alternate sequence search; approximately45min
  including review and landing. Keep failures and executed bytes immutable.
- Return: observed input sequence and guide result, or a precise remaining blocker.
  No app repair is inferred from an automation failure. No human A/B, full visual,
  accessibility, performance/memory, release or G3/G4/core promotion follows.

## Results

**The control failed; the conditional guide lane was not run.** One acquisition
completed seven setup checks and failed the candidate's post-Enter selection check.
Its failed report stays unchanged. No further key sequence, browser acquisition or
consumer repair was attempted.

| Trace | Observed DOM and page accessibility state |
|---|---|
| Both initial states | Light, index0, exact two-option order, BODY focus, no scripts, real200%. Native Tab then focuses the Scheme select. |
| Historical Home | Light remains selected; combobox closed. |
| Historical ArrowDown / Tab | ArrowDown exposes an expanded combobox but Light stays selected; Tab closes it and moves focus to the next button. |
| Candidate Space / ArrowDown / Enter | Space exposes an expanded combobox; ArrowDown and Enter leave it expanded, with Light selected in DOM and page AX throughout. |
| Candidate Tab | Combobox closes and focus moves to the next button; value remains Light. |

The report contains all eleven per-key/initial states, option-selection flags,
focused-element identities and relevant page AX nodes. Native-popup transient
highlight may not be observable through page AX; no missing popup observation is
treated as a pass. It is not a screenshot, physical keyboard or screen-reader test.

This observes failed selection in a bounded minimal-control acquisition without
Fathom markup, CSS or app scripts. Those were not required for this observed failure;
an app-specific contribution to the real guide failure remains possible. It does not distinguish Chrome,
Playwright, OS popup handling, event timing or input routing. Do not describe the
candidate as a working keyboard recipe or claim a diagnosed browser bug.

The approximately2.35-second acquisition retains all fourteen unchanged input
hashes. Chrome and fixture exit0 with complete drains, no forced termination and a
refused final port. No blocked/request/page errors were recorded before the assertion.
The driver's final aggregate error check was not reached. Both trials' final values
and focus are recorded before the assertion; the failed result is not inferred from
missing output. The control uses route-fulfilled HTML, not Fathom's guide endpoint.

## Evidence and bounded handoff

Durable root: `/Volumes/m2ext/appkit-evidence/appkit-select-guide.of7K36NO`.
Staging: `/private/tmp/appkit-select-control.z6mvEJ8h`.
The acquisition archive includes the original contract, preparation bytes, copied
owner/helpers, exact driver and failed report, excluding the temporary Chrome profile.
The unchanged binary/full source remain in the preceding large-value packet.

| Artifact | SHA-256 |
|---|---|
| Prospective source/spec | `7f22244a87f6389fbc2b6aaa815d4d05ae0ee89044c395fc5c1b965c4bff493c` |
| Acquisition packet | `7ca24c2facd2336636035aeb000b04d1ac43eec8a6f5db3bc3cfcdd3639fc7f3` |
| Failed report | `826388df6a4712e022931ebbc1059e389a247502b0cb1d2cf7811840293ca901` |
| Executed contract | `bf19107d507af574a06457f9b630491a5bbc4b4a16f8030584c106a47637ab65` |
| Executed driver | `812b934040ea081c2b4e1766ad164dc0e50a021282b546ea58b10f79ea1f843c` |

The prepared [control driver](fixtures/fathom-large-zoom/select-control-browser.mjs)
matches the executed driver. After preserving that run, the
[preparer](fixtures/fathom-large-zoom/prepare-select-control.mjs) was reduced to the
control lane: the unused conditional guide branch is not shipped as working tooling.
It accepts an original large-value contract and an empty absolute staging directory;
use the contract's exact Node executable. The old preparer remains in the archive.
Preparing a packet does not run a browser or turn this failure into a pass.

**Park this input lane.** Resume only with a materially different, justified input
comparison and a way to observe actual popup selection, such as a physical/native
control comparison when the owner is available. Do not spend another batch on key
permutations or replay resolved chart captures. Keep the dark-guide keyboard gate
open, including its five unperformed captures. Other authorized kit work can
continue independently. No weighted completion, G3/G4/core, accessibility, memory,
performance or consumer-release promotion follows from this control.
