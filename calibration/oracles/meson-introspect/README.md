# meson-introspect — tier-A oracle for Meson (LB-Q-meson)

Meson's own parser and evaluation, written as `indexbench lang-oracle`'s neutral oracle file. Python 3 with
`mesonbuild` importable (Homebrew's meson 1.12.1 installs it beside python@3.14), bench-only, never a product
dependency. `brew install meson`; run the script with `/opt/homebrew/opt/python@3.14/bin/python3.14`.

```sh
PY=/opt/homebrew/opt/python@3.14/bin/python3.14
$PY calibration/oracles/meson-introspect/meson_oracle.py -subject <checkout> -build <scratch dir> \
    [-D option=value]... [-no-evaluate] > workspace/tmp/oracle/meson-<subject>.json
indexbench lang-oracle -dialect meson -subject <checkout> -subject-name <owner/repo> \
    -tool-oracle workspace/tmp/oracle/meson-<subject>.json -out docs/bench/readings/lang/meson.json
```

| Field | Source | What it is |
|---|---|---|
| `definitions` | `mesonbuild.mparser` (the format's reference parser), every `meson.build`, `meson.options` and `meson_options.txt` | `project()`, a target call named by its first string argument, the variable a target call is assigned to, `subproject()`, `option()`, and a `dependency()`/`declare_dependency()` assignment named by its variable, each at the line of its name and ending at the call's `)`. A call whose first argument is not a string literal names no target |
| `references` | the same AST | the identifiers in a call's `link_with`, `link_whole`, `dependencies` and `depends` arguments, owned by the target or dependency object |
| `cross_references` | `meson setup` then `meson introspect --targets` | `(file the linking target is defined in, target, file the linked target is defined in, linked target)` where a target's linker line names another's output file, or it `depends` on it, and the files differ |

`-no-evaluate` parses only: nothing is configured, `cross_references` is left out of the file (not written empty), and
`lang-oracle` leaves L3 unmeasured for the subject instead of scoring its edges against nothing. The parse accepts
`testcase ... endtestcase` (Meson's own test-suite clause) by setting `MESON_RUNNING_IN_PROJECT_TESTS`, which Meson reads
only for that; a file Meson's parser still refuses (the suite's deliberately invalid ones) is named on stderr and has no
truth.

First runs (2026-10-01): harfbuzz/harfbuzz 14.5.1 (163 definitions, 96 references, 211 cross_references over 173
targets), GNOME/libxml2 v2.15.4 (59, 6, 16 over 30 targets), mesonbuild/meson 1.12.1 parse only (5,400 definitions, 908
references across 1,606 files; 18 files refused).
