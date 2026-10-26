# Dart Sass native SCSS resolver

This bench-only adapter asks Dart Sass's resolver which file a declared SCSS
dependency names. It parses each scoped `.scss` file with `Stylesheet.parseScss`
and calls `findDependencies`; `ImportCache.canonicalize` resolves `@use`,
`@forward`, Sass `@import`, and static `meta.load-css()` URLs. It never evaluates
a stylesheet, so a file with an undefined variable can still resolve. Only
`@import` sets `forImport: true`.

The adapter requires an independent primary JSON file. It preserves that file's
non-import fields and every positioned import. A unique native resolution fills
an empty `to_path`; an existing nonempty primary path survives when native
resolution is absent or unresolved. A contradictory resolved native target is a
hard conflict. Stdout uses the primary oracle's schema. Native diagnostics,
skips, failures, coverage counts, unmapped URLs, and resolved file pairs go to
a separate `--metadata` file.
The only non-import metadata change is producer attribution: `tool` becomes
`dart-sass+native-resolver`, and `version` becomes the primary version followed
by `; sass_api 18.1.0 (sass 1.106.1) import resolver`. Tier A and all other
primary fields remain unchanged, so both producers are explicit in the result.
`DependencyReport` exposes URL sets rather than spans, so this adapter never
invents line numbers. An absent primary statement for `@use`, `@forward`, or
`@import` fails composition, even when unresolved. `meta.load-css()` has no
position in `DependencyReport`; every native family URL is recorded as
unpositioned coverage with its resolution and file pair, even when a positioned
primary import shares the URL. Built-in,
external, CSS, and out-of-scope targets do not assert negative file edges. The
source SHA-256 map is captured before and after; source inventory or content
changes fail the run.

Native family counts represent unique URLs per source and dependency kind, not
syntactic occurrences. In particular, `findDependencies` does not traverse
`@include` content blocks, so its URL sets are not a complete import inventory.
The independent primary compiler's existing `to_path` attestation is retained
for such gaps; the adapter reports inherited attestations, missing native
statements, and unresolved primary coverage. A `meta.load-css()` URL remains in
unpositioned coverage even when a positioned `@use` for the same URL is present.

Pinned toolchain: Dart SDK 3.13.7, `sass_api` 18.1.0, and its matching `sass`
dependency 1.106.1. `sass_api` 18.1.0 archive SHA-256:
`fdca92b03a538106d64e01641b8bdeb0639364920ee7eb06f9964cb5ec1ace69`.

Copy the project into gitignored scratch before package setup. Pub configuration
and artifacts stay there, outside the calibration source tree:

```sh
ROOT=/path/to/clean/fathom-checkout
SCRATCH="$ROOT/workspace/tmp/core-t1/dart-sass-resolver"
DART_BINARY=/path/to/cached/dart-sdk/bin/dart
mkdir -p "$SCRATCH/project"
cp -R "$ROOT/calibration/oracles/dart-sass-resolver/." "$SCRATCH/project/"
(
  cd "$SCRATCH/project"
  PUB_CACHE="$SCRATCH/pub-cache" \
    DART_SUPPRESS_ANALYTICS=true CI=true "$DART_BINARY" pub get
)
```

`PRIMARY_JSON` is the independent primary output for the same subject and
scope. `--dir` narrows measurement to a directory under the source root. Repeat
`--load-path` for native Sass load paths. Load paths must be directories inside
the pinned subject, and the collector verifies their complete tracked SCSS
inventory, even outside `--dir`; untracked and ignored SCSS is rejected.

```sh
(
  cd "$SCRATCH/project"
  DART_SUPPRESS_ANALYTICS=true CI=true PUB_CACHE="$SCRATCH/pub-cache" \
    "$DART_BINARY" --packages="$SCRATCH/project/.dart_tool/package_config.json" \
      bin/resolver.dart /path/to/subject --dir src \
      --primary "$PRIMARY_JSON" --metadata "$SCRATCH/native-metadata.json" \
      --load-path /path/to/load-path
) > "$SCRATCH/primary-compatible.json"
```

Synthetic controls cover partials, directory `_index`, legacy `@import`,
undefined-variable files, missing and configured load paths, built-ins, CSS and
external URLs, positioned primary imports, preserved primary fields, and a
resolved `meta.load-css()` missing from the primary. That final case preserves
all primary imports, records an unpositioned coverage gap and native file pair,
and asserts no fabricated line. Missing `@use`/`@import` primary statements
remain errors. The controls write only under `workspace/tmp`:

```sh
(
  cd "$SCRATCH/project"
  DART_SUPPRESS_ANALYTICS=true CI=true PUB_CACHE="$SCRATCH/pub-cache" \
    "$DART_BINARY" run test/synthetic.dart
)
```

Pinned Dart SDK archive: `https://storage.googleapis.com/dart-archive/channels/stable/release/3.13.7/sdk/dartsdk-macos-arm64-release.zip`, SHA-256 `9f972ee1e1139cb4f845f35c7c77bef269dd2ec727b239fb17a2ebcb66d0fb2f` (225,890,953 bytes). SDK license BSD-3-Clause; Sass and sass_api MIT. The committed pubspec.lock pins package archives. Subject measurements use collect.py so clean source inventory, native attestations and primary preservation are checked.
