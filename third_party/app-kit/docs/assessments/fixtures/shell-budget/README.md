# Shell Profile/Settings local budget probe

This fixture establishes a local, unthrottled v3 baseline for the existing Gallery
shell's Profile, Settings, and native load-failure → retry task. It is not a production
SLO, warm-cache/startup result, native-host result, human-task metric, visual review,
or consumer acceptance claim. It does not rebuild or modify Gallery.

`contract.cue` is the source of truth. Before browser execution the probe uses the
repository's `go tool cue export -e contract contract.cue`, freezes the concrete JSON,
the CUE source, harness, exact supplied binary/CSS, and its config in a fresh private
evidence directory. The supplied artifacts must match the contract hashes.

The two lanes are 1440×1000 light with JavaScript enabled, and 320×900 dark with
JavaScript disabled. Each makes three Profile-ready and Settings-ready document
navigations, then a native fixture load failure (POST 303 → Profile 503) followed by
retry (POST 303 → Profile 200) and state assertions. The resource-accounting lane
explicitly disables cache. It permits only loopback GET for `/shell/profile`,
`/shell/settings`, `/static/app.css`, plus exact pinned `/static/shell-demo.js` in the
JS-on lane only; POST is limited to `/shell/action`.
Each document navigation is its own budget observation: a ready GET, or one native
POST → 303 → final GET chain with its exact asset set (CSS in JS-off; CSS plus pinned
shell-demo.js in JS-on). All response bodies are read and
counted as decoded bytes. An unavailable redirect body is zero only with affirmative
`Content-Length: 0` and no transfer/content encoding, recorded as protocol-declared
empty; nonzero or unknown bodies fail. This framing rule does not infer an absent body
from a missing read. The v3 source-grounded correction preserves retained incomplete
v1/v2 runs and does not change numeric limits. Every observation records computed root
polarity plus nontransparent body background/text colors and requires the lane polarity.
Unknown/forbidden requests, non-finite timing, task failure, or cleanup failure are
failures—not zeroes.

Preparation-only checks:

```sh
go tool cue export -e contract contract.cue
node --check probe.mjs
node --test probe_test.mjs
```

Use the already installed Playwright entry from the Gallery-quality fixture and the
installed Chrome; no dependency is installed:

```sh
APPKIT_PLAYWRIGHT_MODULE=/Users/geoff/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs \
  node probe.mjs /absolute/private/evidence-parent \
  /Volumes/m2ext/appkit-gallery-quality.spQ1NymK/gallery \
  /Volumes/m2ext/appkit-gallery-quality.spQ1NymK/app.css
```
