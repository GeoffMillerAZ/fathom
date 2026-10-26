# SCSS primary parser 0.5.1

The independent primary remains `calibration/oracles/dart-sass/sass_oracle.cjs`.
It reads `sass-parser` 0.5.1 for definitions, spans and reference names, and
uses top-level Sass 1.105.1 for successful-compile load attestations. The
parser itself requires `sass: ^1.106.1`, so its nested Sass is pinned to
1.106.1. The native import resolver is separately pinned to Dart SDK 3.13.7,
`sass_api` 18.1.0 and Sass 1.106.1 in [README.md](README.md). The composite
preserves primary definitions, calls, exclusions, tier and positioned imports;
only import targets can gain native evidence. An absent positioned primary
statement fails composition. No native URL is assigned an imagined source line.

Set up Node packages in isolated `workspace/` scratch, never in the source
checkout. Copy `primary-package.json` as the scratch `package.json` so npm
writes a lockfile. `--no-save` with an otherwise empty prefix may install
`node_modules` without a top-level lockfile; do not use it.

```sh
ROOT=/path/to/fathom-checkout
SCRATCH="$ROOT/workspace/tmp/scss-primary-051"
SCSS_SUBJECT=/path/to/clean/subject
mkdir -p "$SCRATCH/npm"
cp "$ROOT/calibration/oracles/dart-sass-resolver/primary-package.json" "$SCRATCH/npm/package.json"
npm install --prefix "$SCRATCH/npm" --ignore-scripts --no-audit --no-fund \
  --package-lock=true --registry=https://registry.npmjs.org
test -z "$(git -C "$SCSS_SUBJECT" status --porcelain --untracked-files=all)"
git -C "$SCSS_SUBJECT" ls-files '*.scss' | LC_ALL=C sort \
  > "$SCRATCH/all-tracked-scss.txt"
NODE_PATH="$SCRATCH/npm/node_modules" node \
  "$ROOT/calibration/oracles/dart-sass/sass_oracle.cjs" \
  "$SCSS_SUBJECT" "$SCRATCH/all-tracked-scss.txt" \
  > "$SCRATCH/primary.json"
```

That plain command applies to subjects without runner setup, such as Primer.
The canonical Bootstrap 0.5.1 primary used the subject runner's sass-true
9.0.0 Sass load path and its unchanged preamble. In the private canonical
output, 122 parser inputs had 3 rejections; 44 compiles succeeded
(38 standalone plus 6 of 84 preamble fallbacks), and 78 finally failed. Reproduce
the setup with the pinned subject, using the same package version:

```sh
npm install --prefix "$SCRATCH/sass-true" sass-true@9.0.0 \
  --ignore-scripts --no-audit --no-fund --package-lock=true \
  --registry=https://registry.npmjs.org
printf '%s' '$true-terminal-output: false; @import "true";' \
  > "$SCRATCH/bootstrap-preamble.scss"
test "$(shasum -a 256 "$SCRATCH/bootstrap-preamble.scss" | cut -d ' ' -f 1)" = \
  8574bf7e82ae2e6057d0a2d76363eb1246a2535a0ed3d2dcf6dcb94fb913f7a8
NODE_PATH="$SCRATCH/npm/node_modules" node \
  "$ROOT/calibration/oracles/dart-sass/sass_oracle.cjs" \
  "$SCSS_SUBJECT" "$SCRATCH/all-tracked-scss.txt" \
  -preamble "$SCRATCH/bootstrap-preamble.scss" \
  -load-path "$SCRATCH/sass-true/node_modules/sass-true/sass" \
  > "$SCRATCH/bootstrap-primary.json"
```

The historical canonical run used an already pinned sass-true 9.0.0 package
in private workspace tools. Its primary diagnostics report one load path,
the preamble above, 6 successful fallback compiles and 78 failed fallback
attempts. The committed counts/hash receipt records these setup facts without
publishing source file lists.

Check the generated lockfile before measuring. Its `node_modules/sass-parser`
entry must be 0.5.1, MIT, from
`https://registry.npmjs.org/sass-parser/-/sass-parser-0.5.1.tgz`, integrity
`sha512-NCtAtKn4sPpfH3EOnHRJRo4XokLivql8vFYSIHXiKUiPLIBUaaQYZvqDTTCZgIRvmJUWKnnzl11RMQ1XIcm2QQ==`.
Its top-level `node_modules/sass` must be 1.105.1, integrity
`sha512-1CnAWPyrrnVQzXGB/DUep1vYCKCm1CIBr+YxjjiW6wk9qtzemnGP4qicH56T2F9S4Hjtv/Ne3jDz8JqfdDU4oA==`.
Its `node_modules/sass-parser/node_modules/sass` must be 1.106.1, integrity
`sha512-kD+Gi66vycZcbJt+42NC4z4PXNMcCajVrJPhqoKKXy5cOBMmJQt052CxIBcRjwYXeImHYOpu/ydWWGZQIenl7A==`.
The parser's exact PostCSS dependency is 8.5.30, integrity
`sha512-zkd+e9S1AQHhmaBKye9zlFHWjOwVgqXl1q3ECYy8561K7qxR2cREf/STLskulBFd78k6AOFeA+ff1TtEzk9bAA==`.
The registry record is `https://registry.npmjs.org/sass-parser/0.5.1`.
The frozen lock and package metadata, not a version range, determine the
runtime used for a reading. Preserve the old primary reading as a comparison
artifact. A new holdout starts only after the canonical instrument and
throughput run are frozen; it never enters `calibration/corpus.d`.
