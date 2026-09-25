# Private evidence-catalog replay

This is a maintainer-only local test, not component certification. Use the existing
authorized private originals and retained P06 preparations. Source-only tests require
neither paid data nor a browser. Never fetch missing sources or install tools implicitly.

## Source-only checks

From the app-kit root:

```sh
GOWORK=off GOPROXY=off go test -race -count=1 \
  ./internal/designevidence ./internal/designreceipts ./internal/navigationproof \
  ./apps/gallery ./cmd/appkit ./cmd/appkit-mcp
```

These include synthetic immutable-store tests and actual MCP/Gallery JSON parity.
Ordinary runs skip the opt-in private bundle export. Explicit missing/stale inputs
fail; a skipped private test is not retained-file proof.

## Export the two frozen P06 packets

The exact fixture in `internal/designreceipts/p06_store_live_test.go` expects the
retained `navigation-typed.FGyrmV` packet and the separate `quality-2DrLVH` run inside
the fresh preparation below. The independently recorded source/output/task pins are
in the [assessment](../../2026-09-14-private-evidence-catalog.md). Do not derive replacement
trust pins from a supplied packet or silently relabel a new run as this historical one.

```sh
APPKIT_EVIDENCE_OUT=$(mktemp -d /tmp/appkit-target-evidence.XXXXXX)
APPKIT_P06_BUNDLE_OUT="$APPKIT_EVIDENCE_OUT" \
APPKIT_P06_RECEIPT_ROOT="$PWD/data/design-assets/navigation-typed.FGyrmV" \
APPKIT_P06_FRESH_ROOT=/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/appkit-p06-DPv8SY \
GOWORK=off GOPROXY=off go test -count=1 -v \
  -run '^TestP06EvidenceBundleExport$' ./internal/designreceipts
```

The exporter refuses to replace existing output files. It creates private
`p06-retained.json` and `p06-with-links-task.json`; their exact hashes are recorded in
the assessment. The second includes a prospective inner task inside a retrospective
aggregate; all nine full gates remain not-run. Keep these bundles private.

## Explicit immutable save

The original archive must already be imported under the selected root. These commands
write only its `evidence/` child. Identical repeat publication is idempotent; conflicting
or corrupted state is refused. Never delete a lock without investigating its owner.

```sh
GOWORK=off GOPROXY=off go build -o "$APPKIT_EVIDENCE_OUT/appkit" ./cmd/appkit
"$APPKIT_EVIDENCE_OUT/appkit" data assets-evidence-save \
  -root "$PWD/data/design-assets" -bundle "$APPKIT_EVIDENCE_OUT/p06-retained.json" \
  -contract-sha 854eaeb9483e5df1422a46fd6ab9b5a44e7ba070212dd519914a6baf257430ee \
  -source-sha 01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31 \
  -artifact-sha 8ab0e692df0b1521097ac04f3847c68ddf3c5e9acaca13c13e8aa0289d78bea6
"$APPKIT_EVIDENCE_OUT/appkit" data assets-evidence-save \
  -root "$PWD/data/design-assets" -bundle "$APPKIT_EVIDENCE_OUT/p06-with-links-task.json" \
  -contract-sha 320036c1cf137c45339a7f145d1f9cd50968f75865f93bf5324d59cb18642252 \
  -source-sha 01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31 \
  -artifact-sha 8ab0e692df0b1521097ac04f3847c68ddf3c5e9acaca13c13e8aa0289d78bea6
"$APPKIT_EVIDENCE_OUT/appkit" data assets-evidence \
  -root "$PWD/data/design-assets" -subject reference.navigation-adapter -limit 10
```

Use `assets-evidence-get -root ROOT -id evidence@SHA` for fresh historical integrity.
List metadata alone is not a recheck. No save/get command executes an artifact or
authenticates a recorded claim. Back up the private root; Git deliberately excludes it.

## Gallery and browser task

Build/run an isolated Gallery using that same root; no consumer dependency changes:

```sh
GOWORK=off GOPROXY=off go build -o "$APPKIT_EVIDENCE_OUT/gallery" ./apps/gallery/cmd/gallery
"$APPKIT_EVIDENCE_OUT/gallery" -addr 127.0.0.1:0 \
  -design-assets "$PWD/data/design-assets"
```

Use the actual printed origin in a second terminal, then open `/target-evidence`.
The browser fixture expects exactly the two recorded P06 entries for the subject, not
an arbitrary future catalog. Extra subject records require a new reviewed fixture;
never delete real records to satisfy the test. It uses the existing generated CSS;
regenerate with the kit's pinned CSS workflow after any utility markup change.

```sh
APPKIT_PLAYWRIGHT_MODULE=/absolute/approved/isolated/playwright/index.mjs \
  node docs/assessments/fixtures/target-evidence/browser.mjs \
  http://127.0.0.1:PRINTED_PORT /absolute/existing/private-output-directory
```

The script pins Playwright 1.63.0 and installed Chrome 152.0.7977.84, launches a fresh
headless browser, disables app JavaScript, permits only same-origin GET, and closes the
browser afterward. It writes a new private `browser-*` directory, JSON report and eight
screenshots. It never uses your signed-in profile or Tailwind portal. On success there
are 57 scoped assertions; failures remain evidence, not approved baseline updates.
Stop only the Gallery process you launched when finished. Reports/screenshots do not
contain bundled source, but keep them private along with the metadata they describe.

The [guide](../../../design/target-evidence.md) documents limits and API boundaries.
MCP tools require a rebuilt/restarted kit server; this fixture does not change global
configuration or prove invocation by an installed Claude/Codex host.
