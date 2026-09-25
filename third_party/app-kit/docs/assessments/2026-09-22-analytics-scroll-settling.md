# Analytical table keyboard-scroll settling

## Prospective R1/R4 maintenance contract

- Starting kit:95151b4acbeb09a73c8c4c1b61a574b8eb9690ea, clean main.
  The [previous batch](2026-09-22-bar-resolution-and-guide-discovery.md) preserved four
  symmetric narrow failures in its exact baseline/candidate pair; none is waived.
- Need: establish whether a keyboard user can reach all analytical table columns.
  Hypothesis: immediate post-key sampling precedes native scroll progress. A bounded
  observation of the same action can distinguish that from absent native movement.
- Preserve I11/I12/I17 and ADR-0072. No UI/CSS/value/oracle changes, consumer edits,
  global changes, dependency installation, provider calls or wider promotion.
- Inputs: reuse the exact baseline/candidate fixture binaries and CSS from
  `/Volumes/m2ext/appkit-evidence/appkit-bar-resolution.zA3lsjzJ`, after SHA verification against
  the previous receipt. Node26.3.0 executable SHA256
  `56694c81b093cc8da273fa017cf91765b3653e5f64f16727976ffaa87b2b6b31`,
  Chrome153.0.8010.53 executable SHA256
  `af09314952c541583cc380057318e0a812a2dd1d7627327d564fc2e991736345`,
  Playwright1.62.1 entry SHA256
  `a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f`.
- One corrected pair: retain all eight original semantic/table/CSV/transport cases;
  for the four narrow cells, reach the table with native Tab and require overflow,
  active focus and visible2px outline. Retain the immediate sample after ArrowRight,
  then poll host-side every25ms, at most750ms, for actual positive movement. No
  animation-frame callbacks, scripted scrolling after input, or application scripts.
- Negative control: the same observer must time out without movement during150ms
  of focused no-input observation. Task: after one positive ArrowRight, send twelve
  further native ArrowRights, require the rightmost Meaning column fully visible
  and scroll at its right edge within750ms, then twelve ArrowLefts and require the
  initial left edge within750ms. Retain coordinate/timestamp samples and screenshots.
- Semantic expectations remain identical: two baseline fallback failures, no
  candidate semantic failures, exact table/CSV values and identical readable HTML.
  No unexpected check, transport, cleanup or timeout failure may count as passed.
- Cost: at most30minutes, one paired acquisition under a120-second owner watchdog.
  No corrective rerun in this batch. A source/runtime mismatch or acquisition defect
  yields a retained failure and handoff, not another matrix or acceptance rewrite.
- Owner/evidence: app-kit maintainer; private root
  `/Volumes/m2ext/appkit-evidence/appkit-scroll-settling.RezlaC4C`. Freeze exact executed source,
  driver and helper before launch; retain historical and new reports separately.
  If proven, return the bounded observation lesson and update only scoped evidence
  links. Human A/B, actual zoom, full a11y, consumer rollout and quality promotion
  remain independent gates. No new runtime helper/API is warranted by this test.

## Result

One acquisition, exit0. Eight cells complete88 checks:86 pass and exactly two
expected old-renderer semantic failures; no unexpected failures, errors or timeout.
No application, CSS, dependencies or consumer bytes changed.

| Narrow cell | Immediate position | First observed positive position / elapsed | Right edge | Return |
|---|---|---|---|---|
| Baseline spread | 0 | 3px /34.1ms | 215px | 0 |
| Baseline readable | 0 | 1px /34.3ms | 188px | 0 |
| Candidate spread | 0 | 1px /32.8ms | 215px | 0 |
| Candidate readable | 0 | 4px /31.4ms | 188px | 0 |

Elapsed time above starts at the bounded observer, after the immediate sample;
it is not a precise input-to-render latency measurement. Each no-input control
remains zero throughout its150ms window. The rightmost Meaning header is fully
inside the region at the right edge; native leftward travel returns to zero in
every cell. Focus,2px outline and zero outer-window horizontal displacement remain
checked. Right/left terminal positions are observed within157–188ms of their
respective observers, below the fixed750ms cap. Two candidate screenshots were
inspected at the right and restored-left positions; screenshots alone are not the
keyboard proof.

These observations support premature sampling as the prior oracle failure in this
exact local fixture/runtime. They do not establish every platform's behavior or
exclude every possible autonomous movement outside the150ms negative-control window.
The prior failed receipt stays failed; this is a separately contracted acquisition,
not a rewrite of its result or a relaxation of the expected user task.

All original value/meaning/missingness/CSV and semantic controls remain. Both
readable HTML documents and both CSV selections match baseline/candidate exactly.
Each document/CSS body completes before task observations. CSP, zero application
scripts, local transport and no unexpected console errors pass. Both owned servers
and Chrome exit with complete capture, no forced stops or unresolved children;
both ports refuse afterward.

Pre-run independent review found a server ownership gap inherited from the earlier
driver: a READY timeout could reject before the child entered the cleanup list.
The new driver registers ownership before awaiting READY and handles spawn errors;
the pre-review bytes remain in `pre-run-source.tar`, but were never executed.
Only the corrected frozen driver ran. This acquisition exercises successful startup
and cleanup, not every failed-start path.

## Durable evidence and returned lesson

The executed driver and full report are [stored together](fixtures/analytics-scroll-settling/README.md).
Private source snapshots remain in `/Volumes/m2ext/appkit-evidence/appkit-scroll-settling.RezlaC4C`:

| Artifact | SHA-256 |
|---|---|
| `pre-run-source.tar`, unexecuted review revision | `dcb5a321339f97e1d3b3aad6389ac8a489d7dfedf2cac6efc75f4cd822172bc7` |
| `executed-source.tar`, prospective contract/driver/helper/fixture | `3bb768bf7870e12ebaa62442a4b809644c7dfd9467572700d9c5b0ed72253767` |
| executed `browser.mjs` | `246574e2b5f624e8440a2f270301dce435c503d474368dade6c98eaaad1263bc` |
| `browser-run-01/report.json` | `ff3d4fe804a83260b4ee2c079c7174d8984f0b246c8004d909bf8a13b9f46036` |

The practical lesson belongs to analytics browser guidance: native input dispatch
is not visible-state completion. Retain the immediate sample, observe an explicit
postcondition under a fixed deadline, and include an unchanged-input negative
control. Do not add runtime JavaScript or artificial scrolling to make a native
task pass. Earlier successful recipe/signed harnesses waited100–150ms; this driver
makes observed progress and terminal-column access explicit instead of guessing a
longer delay. This is local evidence machinery, not a public waiting API.

`core.analytics` links this exact scoped keyboard evidence without promotion.
The retained previous failure is linked to this follow-up; all prior test values,
budgets and receipts remain unchanged. Human A/B, other browsers/actual zoom,
full accessibility, consumer delivery and overall G3 acceptance remain open.
The current diagnostic is closed; no full matrix or additional packet runs here.
