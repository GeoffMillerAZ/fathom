# Native Blade grammar screen

This is a calibration command, not a product parser dependency. It screens a
grammar against **all** 254 canonical `.blade.php` files: Laravel Breeze 57 and
Filament 197. The subjects must be clean at the pinned commits, including no
untracked or ignored files. The script hashes every relative path and file's
bytes into one set digest, so a later receipt can detect corpus drift without
publishing the corpus file list or contents.

Pins and primary repositories:

| Input | Commit | Source |
|---|---|---|
| Breeze | `47f6b30a3bdf376a3f75cf9ad8e68ea3941048e2` | [laravel/breeze](https://github.com/laravel/breeze) |
| Filament | `85d51ef9e5c4e3db281c568315f535652639e09f` | [filamentphp/filament](https://github.com/filamentphp/filament) |
| deanrumsby Blade grammar | `e2ff089ded9351aeac9f4402225c5d49e8c0067d` | [deanrumsby/tree-sitter-blade](https://github.com/deanrumsby/tree-sitter-blade) |
| codehappyness Blade grammar | `9067259bebeed6be0d87b82f7ef93f20b2126156` | [codehappyness/tree-sitter-blade](https://github.com/codehappyness/tree-sitter-blade) |

With [tree-sitter CLI 0.26.11](https://github.com/tree-sitter/tree-sitter/releases/tag/v0.26.11)
installed, prepare clean pinned clones under `workspace/`:

```sh
mkdir -p workspace/calibration workspace/tools workspace/tmp
git clone https://github.com/laravel/breeze.git workspace/calibration/breeze
git -C workspace/calibration/breeze checkout --detach 47f6b30a3bdf376a3f75cf9ad8e68ea3941048e2
git clone https://github.com/filamentphp/filament.git workspace/calibration/filament
git -C workspace/calibration/filament checkout --detach 85d51ef9e5c4e3db281c568315f535652639e09f
git clone https://github.com/deanrumsby/tree-sitter-blade.git workspace/tools/blade-dean
git -C workspace/tools/blade-dean checkout --detach e2ff089ded9351aeac9f4402225c5d49e8c0067d
git clone https://github.com/codehappyness/tree-sitter-blade.git workspace/tools/blade-code
git -C workspace/tools/blade-code checkout --detach 9067259bebeed6be0d87b82f7ef93f20b2126156
```

Run each grammar independently from this repository root. Pass absolute paths
for clones located in another worktree's `workspace/`:

```sh
python3 calibration/oracles/blade-grammar-screen/screen.py \
  --grammar workspace/tools/blade-dean \
  --grammar-commit e2ff089ded9351aeac9f4402225c5d49e8c0067d \
  --breeze workspace/calibration/breeze --filament workspace/calibration/filament \
  --out workspace/tmp/blade-dean-screen
python3 calibration/oracles/blade-grammar-screen/screen.py \
  --grammar workspace/tools/blade-code \
  --grammar-commit 9067259bebeed6be0d87b82f7ef93f20b2126156 \
  --breeze workspace/calibration/breeze --filament workspace/calibration/filament \
  --out workspace/tmp/blade-code-screen
```

The script compiles `src/parser.c` and optional `src/scanner.c` or `.cc` with
`-O0` into a native library in the private output directory, then runs
`tree-sitter parse -q --lib-path ... --lang-name blade` on every gate file.
Each file has a 10-second timeout (`--timeout` changes it). The JSON receipt
records subject set hashes, grammar source and library hashes, aggregate
ERROR/MISSING counts, timeouts and infrastructure failures. Only zero timeouts
and infrastructure failures permit a gate pass. A nonzero exit means the gate
did not pass or the run failed; `failure.txt` explains setup/build failures.

The committed controls cover `@section`, `@include`, a custom directive,
`<x-alert>`, and `<x-slot>`. Their full ASTs go only to private output files;
the receipt carries AST hashes and node-type counts. Inspect those ASTs before
any admission: an ERROR rate at or below 12/254 does not establish the
structural directive and component nodes needed by L1.
