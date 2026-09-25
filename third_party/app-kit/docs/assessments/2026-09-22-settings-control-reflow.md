# Settings control-column text reflow

## Prospective task and boundary

Baseline `34a0cc5ca6efa60ef8521c6dcacf9574dae4d198`. The accepted
[G2 matrix](2026-09-16-shell-acceptance-matrix.md) already covers real save/recovery
and separate long section-heading/description layouts. Its
[fixture](../../appkit/ui/form/settings_browser_test.go) uses short field/control
labels; it does not prove long unbroken labels, help/error text or radio options
inside the control column. Do not replay or reopen accepted G2.

Task: read a long application-defined preference identifier or validation reference,
edit the related field, and change boolean/radio preferences on narrow or wide pages.
Use the existing Catalyst-informed description/control layout, existing light/dark
roles and native controls. Source review finds wrapping on the description column
but only `min-w-0` on the control column. Reproduce this exact gap before claiming it.

The intended repair extends inherited `overflow-wrap:anywhere` to the control column.
No new API, dependency, CSS selector, primitive markup, theme, truncation, app state or
save authority. Caller-provided reviewed HTML remains caller-owned; explicit child
nowrap/preformatted layouts can override inheritance and need their own review.
I4/I7/I12/I17, ADR-0064/0069/0072 apply. No architecture trigger or future is promoted;
this is a bounded repair within the existing responsive-composition responsibility.

## Frozen acceptance and budget

- Unit RED/GREEN requires the control-column wrapping utility and unchanged nested
  native control output, exact complete long strings and no new scripts.
- One original/candidate browser acquisition: 390 and 1440 CSS pixels, light and dark
  (eight cells). The same Go-produced fixture and frozen current Gallery CSS apply.
  Baseline must positively reproduce long field label/help/error and checkbox/radio
  text overflow. Candidate Range bounds must fit the control column and viewport;
  tolerance is 1 CSS pixel, not a post-failure relaxation.
- Exact strings, label/control and error/help associations remain. Native checkbox
  and radio hit boxes stay at least 12 CSS pixels wide; short control geometry stays
  equal before/after. Native Tab/typing/Space/arrow keys change only intended fields
  with a visible focus indicator. Do not replace user input with DOM value writes.
- Completed document/CSS bodies and fonts precede geometry. Fresh isolated Chrome
  profile/contexts, no scripts/service workers, exact virtual loopback route allowlist.
  Retain selected captures and inspect them. No real server, submission or save claim.
- At most 30 minutes of implementation/acquisition; browser owner bound 120 seconds.
  At most one separately retained corrective run for a proven harness defect, not a
  genuine UI failure. Keep all failed observations and executed inputs.

Private evidence root: `/Volumes/m2ext/appkit-evidence/appkit-form-reflow.HfHbbXVF`.
This fixture measures composed settings presentation only: standalone form primitives,
actual 200% zoom, native select popups, assistive technology, full shell state/lifecycle,
consumer adoption and full core standards remain separate. A passing slice cannot
promote their inventory status or change the original G2 acceptance.

## Results

The unchanged application candidate passes the bounded repair: **76 checks across
eight cells**, zero unexpected requests and clean unforced Chrome exit. Every original
cell reproduces all eight overflowing text targets; every candidate keeps their Range
fragments inside the control column and viewport. Native typing/Space/ArrowRight,
visible focus changes, exact FormData/associations and short-control geometry pass.
All six retained captures were inspected: baseline narrow pages visibly overflow;
candidate narrow/wide light/dark pages wrap complete strings. Checkbox/radio controls
retain their native sizing and centered multiline alignment. This is not a touch-target
or contrast certification. The five-section fixture adds125 HTML bytes, no CSS or JS.

Frozen unit RED fails the new wrapping assertion on the original; GREEN passes.
`go test -mod=readonly -count=1 -race -timeout=120s ./appkit/ui/form
./appkit/ui/dashboard ./apps/gallery` passes, including unchanged legacy checkbox-byte
tests. Go1.26.3 Darwin/arm64, Node26.3.0, Playwright1.62.1 and Chrome153.0.8010.53
are bound in retained manifests; the browser inputs also bind the actual framework.
Both Go fixture builds pass against isolated tracked-source archives. No clean-cache
or cross-host reproducibility is claimed. No quality status or consumer pin is changed.

### Failed acquisition and one correction

`browser/` remains failed: baseline's Tab-selected input retained selection after the
driver's `End`, so typing replaced the value; `scrollIntoViewIfNeeded` then timed out
on element stability with scripts disabled. It recorded eight assertions, one failed
value assertion plus the timeout, zero completed cells and clean exit. It is not a
candidate failure or pass. `browser-02/` changes only the frozen driver: native
ArrowRight collapses selection before appending, with independent before/after caret
assertions; full-page capture omits the unnecessary scroll helper. App/fixture/CSS and
original geometry/value expectations stay identical. The single allowed correction
passes within the original acquisition budget; no further browser run was performed.

### Replay and retained identities

[Maintained fixture and replay](fixtures/settings-control-reflow/README.md). Paths below
are relative to the private evidence root above; full source, executable, environment,
HTML and runtime identities are in its `build/result.json` and browser manifests.

| Artifact | SHA-256 |
|---|---|
| `baseline.tar` | `d76808553744a4fb8bb66d98e1e10a080518ccebbf0c16ff418c5c9c85768089` |
| `build/baseline-source.tar` | `ac52175ad907e2e7ebe76d2013f5f51c02b1b89cf5aac22b0b0d2855d2a3da1d` |
| `build/candidate-source.tar` | `28e1c50c8e835c3f9875042e1754a97bef1432922bf158831015c4775cdd85ff` |
| `red/result.json` | `6175d698e89018c7e9111ddaa57c648e9f5f17891e8024dc074e8c1988a841fc` |
| `green/result.json` | `06c9f2ea13e7209fa0269772dc0206f09c5f3eb88f2c3637f267a372b2b04ae3` |
| `regressions/result.json` | `b4dc21a0f9c7cea9e9965ff4028f8e3c44bcadb2670d61225646771c969cfcb7` |
| `browser/report.json` (failed) | `26355bbf5e3a359e250755b1eb689a2534019b2c138552abed13ec803d9a95e0` |
| `browser-inputs-02/manifest.json` | `0d62dd10b6139d49509add2059fa09b1b5fbc6261af408d3c19546f5ca5df36d` |
| `browser-02/report.json` | `d6d27bb09bf26432a93ddd05022c52666422c205752911cb30873b9115448c95` |

INC-68 records the control-column coverage gap and local regression. Closed INC-14
was compressed without removing its outcome (prior form remains in `34a0cc5`),
keeping the incident log within150 lines. Fathom O37 separately records caller/tool
parameter ambiguity; it is not a claim that correctly scoped references are broken.
