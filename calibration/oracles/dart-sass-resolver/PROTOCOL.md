# SCSS resolver provenance protocol

## Registered comparison

Tier A resolver measurement uses the native Dart Sass adapter at
`calibration/oracles/dart-sass-resolver/bin/resolver.dart`. The independent
primary remains `sass-parser` 0.4.57 with Sass 1.105.1. The adapter may add
resolved `to_path` values to matching primary imports. It must preserve primary
imports, fields, and counts, and must not invent source lines. Record every
parse and unresolved failure and report total primary coverage. Empty resolution
does not imply a negative file edge. Fathom output is never an oracle input, and
this work adds no canonical corpus shard.

## Freeze and preregistration

Freeze and commit this protocol, wrapper, and native adapter before reading any
fresh holdout source. Every run names the complete 40-character subject Git SHA.
The subject must be a clean Git root. The wrapper hashes every tracked regular
`.scss` file in the requested scope and rejects tracked symlinks, untracked or
ignored SCSS, source changes, and adapter fingerprint differences. `--dir`
means the complete tracked inventory beneath that directory, never a file list
or hand-selected subset. The default scope is the full subject.

Every `--load-path` must resolve to a directory inside the pinned subject. The
collector checks the full subject SCSS inventory before and after measurement,
including load-path directories outside `--dir`, so untracked or ignored SCSS
cannot participate as a resolver target. Each `meta.load-css()` family URL is
reported as unpositioned native coverage with its native path pair, even when a
positioned primary import shares the URL. Missing positioned `@use`, `@forward`, or `@import`
statements remain a hard error. `DependencyReport` uses URL sets, so family
counts mean unique URLs per source and dependency kind, not syntactic occurrences.
A `meta.load-css()` URL remains unpositioned even when a positioned family uses
the same URL.

Registered native tier: Dart SDK 3.13.7, `sass_api` 18.1.0, and `sass` 1.106.1.
The run record includes the primary JSON SHA-256, adapter SHA-256, Dart and
package pins, package-config SHA-256, source commit, source inventory hash and
count, scope, subject-relative load paths and their SCSS inventory pin, and
adapter coverage/failure counts. It omits public source filelists and absolute
tool/load-path locations. Raw adapter
metadata stays outside the subject in a mode-0600 private workspace file.

The frozen historical references are bootstrap v5.3.8
(`25aa8cc0b32f0d1a54be575347e6d84b70b1acd7`) and primer/css v22.3.2
(`be70cda91066df154f76e634d7ce993f205d282e`). `spentFoundation` v6.9.0
(`f5c7d29db04e459d94a998df9d8fd4a4447e767b`) is post hoc only. `freshSpectre`
v0.5.9 (`8847251d71b4dac27e8407cbdb71ae89ce156a43`) is registered as MIT;
its source has not yet been read. No fresh holdout source may be inspected before
the freeze commit.

## Running

Prepare a private scratch copy of the adapter and run `dart pub get` there.
Keep primary JSON, package configuration, native metadata, output, and logs
outside the subject. The wrapper applies `nice -n 10`, refuses launch when the
1-minute load average exceeds 9, and terminates the process group after 600
seconds. Package setup and corpus execution are deliberately separate from the
synthetic controls.

```sh
python3 /path/to/provenance-checkout/calibration/oracles/dart-sass-resolver/collect.py \
  --subject /path/to/clean/subject \
  --expected-sha FULL_40_CHARACTER_SHA \
  --primary /private/workspace/primary.json \
  --dart /cached/dart-sdk/bin/dart \
  --package-config /private/workspace/project/.dart_tool/package_config.json \
  --output /private/workspace/native-primary.json \
  --metadata /private/workspace/native-metadata.json \
  [--dir src] [--load-path /path/to/clean/subject/styles]
```

`collect.py --selftest` exercises clean scoped and full inventory, dirty subject,
tracked and untracked SCSS symlink, and primary hash controls without Dart,
package setup, or corpus reads. Synthetic adapter behavior belongs to
`test/synthetic.dart`; it also runs only against synthetic inputs.
