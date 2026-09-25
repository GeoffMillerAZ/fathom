# Elements source-native interaction reference

Private G1/G4 pilot under ADR-0023 and I13/I17. This fixture preserves the four
frozen P03/P07/P08/P09 fragments; it neither changes the translator's refusal nor
promotes a converted target. Its purpose is to prove the interaction dependency
before deciding how a faithful Go/templ adaptation should preserve it.
The [2026-09-14 assessment](../../2026-09-14-elements-source-reference.md) records
the retained private bundle and 404 scoped browser assertions, with source gaps intact.

## Frozen conditions, before browser execution

- Exact archive/member identities come from the [pilot manifest](../../2026-09-13-conversion-pilot.md).
- Elements `1.0.22`, published tarball SHA-256
  `fc684f17d960fb60dadfdf9a0b49ba8fd295af4b8fd1715a26cd20b832ea245b`;
  runtime SHA-256 `120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013`.
  The runtime and its commercial license stay in private output, never kit source.
  Output retains the exact tarball for offline replay; it is not a public dependency.
- Cached checksum-pinned Tailwind4.3.0 and the existing pinned InterVariable font
  are chosen comparison conditions, not recovered portal settings. CSS scans only
  the selected sources; two builds must match exactly. Original classes stay intact.
- Geoff inspects behavior at a desktop and checks navigation at phone width.
  Exercise 390/1440 CSS pixels, light/dark, reduced motion and fonts ready.
- The local browser fixture serves verified artifact bytes only, on an ephemeral
  loopback address with a random path, no directory listing or mutation endpoint.
  Its independently replayed complete manifest is hard-pinned in `contract.mjs`;
  a new wrapper/toolchain/source baseline needs a separately reviewed revision.
  Scripts are local modules; no inline script/eval permission. Runtime-owned style
  attributes require inline CSS permission, not arbitrary script permission.
- Block off-origin requests and all form submissions. P08/P09 remote images remain
  unpinned and blocked; their screenshots are not complete visual baselines. P03/P07
  have no remote images. Placeholder links/actions do not become app workflows.

## Required interaction checks

P03: six panels, source initial state, Enter/Space/pointer toggle, unaffected siblings,
focus continuity and remove/remount. P07: open by keyboard/pointer, focus entry,
arrow movement, Escape return and outside dismissal. P08: dialog open, nested menu,
first Escape dismisses only the menu, second Escape dismisses the dialog and returns
focus to its opener. P09: responsive visibility, mobile disclosure toggle, account
menu Escape and resize behavior. Repeat connected/disconnected instances to detect
stale behavior; this is not a heap-leak proof.

All cases retain errors instead of aborting after the first red result. The test
records screenshots, local bytes, runtime errors and requests. A runtime-withheld
canary must fail the same disclosure-toggle assertion. Record keyboard screenshots
and computed focus observations separately from active-element assertions. Measure
control dimensions and overflow while overlays are open, not only after closing.
Missing touch, real zoom, screen-reader, Datastar morph and consumer task evidence
remain open; a passing Chrome matrix cannot certify every browser or a conversion.

## Reproduction

Download the exact published tarball into private scratch separately; preparation
never fetches dependencies or executes vendor code. From the app-kit checkout:

```sh
go run ./docs/assessments/fixtures/elements-reference \
  -kit-root /absolute/app-kit -runtime-tgz /private/elements-1.0.22.tgz \
  -out /private/new-elements-reference
node docs/assessments/fixtures/elements-reference/browser.mjs /private/new-elements-reference
```

Preparation requires an existing real 0700 parent and refuses an existing destination,
modified source/runtime/font or missing cached compiler. It retains directory handles
and detects replacement at checked boundaries; this is not malicious same-UID defense.
Keep artifacts and screenshots private. Normal Go tests use only
synthetic inputs; explicit local browser execution uses the approved isolated
Playwright1.63.0 harness and installed Chrome, without a signed-in profile.

From this fixture directory, `node --test contract_test.mjs` checks synthetic
manifest/request refusals. Set
`APPKIT_ELEMENTS_REFERENCE=/private/prepared-reference` to include explicit private
manifest/asset rehash canaries; without it that live check is skipped, not passed.
