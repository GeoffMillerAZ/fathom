# G5 terminal-target acceptance reconciliation

**Status: the finite G5 interaction and two-consumer outcome is satisfied.**
This combines the Charm reference with explicit Bosun/Surveyor adoption, not a
new terminal framework, installed-binary release or full host certification.
Historical failures keep their original identities and dispositions.

The fixed G5 exit test was: one bounded Charm reference demonstrates keyboard/mouse
parity, focus/selection, scroll/resize, cancellation, terminal restoration and capture
opt-out; machine output/exit contracts remain stable; improvements integrate into two
relevant CLI consumers. Learn from Crush/Herdr without copying their implementation
or requiring another TUI stack. The authorized usage1→2 migration is an explicit,
documented compatibility change in those two consumers, not a silent kit default.

## Requirement and evidence

| G5 clause | Observed evidence |
|---|---|
| Bounded Charm interaction reference | `apps/terminal` searches/inspects the shared read-only first-party catalog using the existing Bubble Tea runner; no fleet operation or purchased source executes |
| Keyboard/mouse parity and focus/selection | Actual PTY Enter and mouse Open reach the same ID; Back preserves selection; model/event-loop tests cover editing focus, controls and stale callbacks |
| Scroll/resize | Owned PTY wheel moves three lines, PageDown/PageUp move one visible detail page and resize preserves detail/selection; exact rows and full redraws are asserted |
| Cancellation | Typed Ctrl+C0, OS SIGINT2, SIGTERM0 and deadline fixture identity assertions pass; one signal owner retained |
| Restoration | All termios fields match after a canonical read; independent keeper/parent observations agree, shutdown protocols are present and no forced cleanup is accepted |
| Capture opt-out | F2 releases/re-enables capture; undersized layout suspends it while preserving user preference; keyboard remains usable |
| Machine output/exit contract | Current unchanged terminal race suite proves noninteractive JSON/no-run, pipe refusal, query/write/cancellation errors; kit's ordinary Execute and explicit exits remain compatible |
| Two relevant consumer integrations | Bosun and Surveyor use shared acknowledged-error reporting and the opt-in origin classifier; synthetic regressions, fresh builds and the fixed13-check sandbox matrix pass |
| Crush/Herdr reference without second stack | ADR-0018 records Crush as design reference only; the bounded reference delivers inspect/back, keyboard-first navigation, mouse parity and opt-out on the existing Charm stack |

The [terminal contract](../design/terminal-gallery.md), [input proof](2026-09-15-terminal-input-controls.md)
and [usage migration](2026-09-16-cli-usage-migration.md) own the detailed semantics.
The historical ECHO-drift and frame-readiness negative controls remain relevant;
the new acquisition does not claim to have rerun every historical canary.

## Current-code acquisition

Private root: `/Volumes/m2ext/appkit-evidence/appkit-cli-migration.PTQu01`.
The selected source, configuration, harnesses, exact binaries and raw outputs are
retained. They are bounded working-tree evidence, not complete consumer dependency
archives, signed receipts or permanent backups.

`terminal-supported/runs/appkit-terminal-pty-kmc9z020/receipt.json` records a single
macOS26.6.2 arm64/Python3.14.6 run: seven journeys,144 assertions, no failures,
forced cleanup or cleanup errors; post-run recorded-PID checks find no surviving
test processes. Per-case counts are29/28/30/14/14/14/15 for keyboard-mouse-resize,
bidi-filter, small-viewport, Ctrl+C, SIGINT, SIGTERM and deadline respectively.
The deadline helper's zero means its expected error-identity assertions passed.

`terminal-machine/result.json` records one unchanged
`go test -p=2 -race -count=1 -json ./apps/terminal/...`: three packages,96 passing
test events including parent/subtest events, no failures/skips. Its65 selected
source files match before/after; the earlier CLI migration separately proves the
kit's92 default-policy test events and the origin/upstream/consumer suites.

| Artifact | SHA-256 |
|---|---|
| Supported PTY receipt | `e12e410baf14df693ead9df9815d4b6424e6c48a618f6295a289f25a0bfa4aea` |
| Supported frozen configuration | `ae4aae07f02b83bdb0fdbb949889f76f411247da0372c01cfa878ea8a3a4dd5d` |
| Supported frozen-input manifest | `2941ab890d92f764b70427b8ae3afe5077908f1d2ccab973104a6642923679de` |
| Terminal binary | `68a5d94c25debfcfd216f8efb77b412a79ef397bbc55996a88be4e566257bd99` |
| Deadline fixture | `0d2e5c75b7424410c912546c1cc352b67cd4be20feb970b6aa2a12201c81dbdc` |
| Machine-suite result | `4d5ae8c93f5961ba4fef0bb52a5a4133416aa063a3eb8037a3f8589e4258de5c` |
| Machine source/config manifest | `9916d8f4b7a99456634ec593601d28a461bfdcedc324af3c31b10102edf0ca3c` |

### Retained interpreter mismatch

The first run under `terminal-final/` passed144 assertions with Python3.9.6, but its
pre-run config mistakenly named PATH Python3.14.6. The agent chose `/usr/bin/python3`
while constructing a minimal environment; no3.14 failure caused that substitution.
Receipt `141779aef5659758ae11d4c13f2cbe932fb02c9287b777b991ba373a1175fc1e`
correctly records3.9.6. Keep it as off-documented-runtime supplemental evidence;
do not rewrite the frozen configuration or count it as the specified acquisition.
The separate supported run freezes/executes the resolved3.14 interpreter and reuses
the same binary/harness bytes. The oracle and production code were not changed.

## Scope disposition

The intent audit finds G5's Done when met, the top-level CLI availability rows stale,
and ADR-0039/0048/0052's migration triggers resolved by accepted ADR-0070. No future
is promoted, new TUI stack built or permanent widget API inferred. I10's scoped
enforcement now includes actual consumer usage codes; I1/I8/I16 remain stated broader
rules, not strengthened by test counts. No plan box or static index establishes this
acceptance. Scope removes the completed goal while retaining its link and this matrix.

Installed fleet binaries, operational host tasks, graphical emulator/light-dark/font
review, physical input, clipboard, assistive technology, Windows and full release
delivery are not proved. The PTY uses NO_COLOR and no background-polarity reply.
Existing consumer local replaces remain G4 delivery debt. The other app-kit outcomes
stay active; this acceptance does not certify the entire kit or converted catalog.
