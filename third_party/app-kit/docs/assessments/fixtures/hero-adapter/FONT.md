# P01 private font-delivery experiment

This opt-in fixture investigates the measured P01 asset budget without changing its
markup, image, colors, font family, weights, optical-size axis or original source.
It is not a public font API, catalog promotion or production asset pipeline.

## Prerequisites and execution

Use the retained P01 probe/reference from [PERFORMANCE.md](PERFORMANCE.md), an
existing private Python environment containing FontTools4.65.0/Brotli1.2.0,
installed HarfBuzz14.3.0 `hb-shape`, and the already-approved isolated Playwright
installation plus installed Chrome. No installation or network fetch occurs here.
The Python packages belong only to the private experiment, never app-kit or apps.

```sh
APPKIT_PLAYWRIGHT_MODULE=/absolute/approved/playwright/index.mjs \
node docs/assessments/fixtures/hero-adapter/font-proof.mjs \
  /absolute/retained-p01-probe \
  /absolute/retained-p01-probe/browser-NAME/receipt.json \
  /absolute/private-venv/bin/python \
  /absolute/hb-shape \
  /absolute/existing-private-output-parent
```

Every run creates a new0700 `font-proof-*` directory and snapshots ten maintained
source/config files before executing their retained copies. It runs21 Node checks,
two fresh subset builds, source-identity/existing-output refusal controls and70
HarfBuzz comparisons (seven selected-repertoire texts, five weights, two optical
sizes). CLI arguments, exits and complete outputs are retained with hashes.
The generator preserves all source name records, variable axes, retained glyph
outlines/advances/hints/variation deltas, and source table-presence requirements.
TTF files exist only for HarfBuzz; the browser receives the actual WOFF2 outputs.

The browser checks four paired hero viewports, a100-paragraph variable typography
fixture, subset-only Latin and original-only Greek requests, and a missing-subset
negative control. Positive lanes require exact response bytes, actual custom Inter
glyphs, native link order and identical paired PNGs/text/geometry. It never forces
the full font to download just to make a font-loading API report success.

## Delivery design and limits

The exact original `@font-face` stays first and unrestricted. A second face with
the same descriptors gives the selected subset priority. Its ranges are fixed in
`font-delivery.mjs` and checked against the Python generator. The full original
remains eligible for whole-cluster fallback; a disjoint complement was rejected
after it caused system-font substitution for an accented IPA character.

`document.fonts.check()` is diagnostic, not glyph evidence. Its matching-face
contract includes unloaded eligible faces and can also return true when a family
is missing. The fixture waits for font/layout settlement, then checks actual
rendered fonts and request identities. See the
[CSS Font Loading specification](https://www.w3.org/TR/css-font-loading/#font-face-set-check).
These checks are specific to the pinned controlled Inter source, not all languages,
all font features, other browsers or the Tailwind portal's original font provenance.

Keep all failed roots. A smaller file does not pass delivery or fidelity. A passing
font packet does not pass performance: separately run the unchanged numerical
matrix with `--font-candidate` and the explicit packet receipt digest. A warm
request-bound body failure remains a failure even when local glyphs look correct.
