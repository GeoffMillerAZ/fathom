# The Blade grammar against the 5% ERROR-file gate (LB-Q-blade)

**Verdict: EmranMR/tree-sitter-blade fails the gate at every commit measured. On the queue's two calibration
subjects, 26 of the 254 `.blade.php` files (10.2%) at the grammar's `main` head leave an ERROR node, against
§14.2's 5% kill line; the best tag, v0.11.0, leaves 20 of 254 (7.9%). On BookStack, a Blade-heavy Laravel
application added as a third subject, it leaves 216 of 325 files (66.5%) in error. The item is refused, and
`.blade.php` stays disclosed as unsupported.**

Measured 2026-10-24, before any dialect or reading; nothing from the measurement is committed but this note.

## Grammar

| | EmranMR/tree-sitter-blade |
|---|---|
| Licence | MIT (`LICENSE` at every commit measured) |
| Commits | `main` `b5291d1ba207a8ebb8383b2ecb8a8a6535210a50` (v0.12.3 + 21); tags v0.12.3, v0.11.0, v0.9.2 |
| Shipped parser | `src/parser.c` (v0.11.0: 10.1 MB, ABI 14, no scanner; v0.12.3: 17.9 MB, `main`: 23.1 MB, ABI 15, with `src/scanner.c`) |
| Queries shipped | `highlights.scm`, `injections.scm`; no `tags.scm`, so no third-party definitions query exists for the L1 tier-C oracle |

## Method

`tree-sitter-cli` 0.26.11 `parse -q --lib-path <lib> --lang-name blade <file>` over every `*.blade.php` file of
each subject at its shallow-clone head, each grammar compiled by `gcc -O0 -shared` from its own `src/`; a file
counts when the command exits non-zero (an ERROR or MISSING node). Subject heads (shallow clones on
2026-10-24, MIT each): laravel/breeze `47f6b30a3bdf376a3f75cf9ad8e68ea3941048e2`, filamentphp/filament
`85d51ef9e5c4e3db281c568315f535652639e09f`, BookStackApp/BookStack `045c6a98cac21ea86906674bb91d01f375460db6`.

| Subject | Files | `main` b5291d1 | v0.12.3 | v0.11.0 | v0.9.2 |
|---|---|---|---|---|---|
| laravel/breeze | 57 | 1 (1.8%) | 1 | 1 | 2 |
| filamentphp/filament | 197 | 25 (12.7%) | 25 | 19 (9.6%) | 33 |
| **both queued subjects (the gate)** | **254** | **26 (10.2%)** | **26 (10.2%)** | **20 (7.9%)** | **35 (13.8%)** |
| BookStackApp/BookStack (added) | 325 | 216 (66.5%) | 221 | 202 | 202 |

Representative failures, `main`: a directive call with named arguments (`@filamentScripts(withCore: true)`),
a dynamic tag name (`<{{ $tag }}`), an HTML attribute value after a Blade echo
(`value="john@example.com"` inside a component tag), and `@stack`, `@yield`-heavy layouts that the grammar
reads as unterminated blocks (v0.11.0 opens a `directive_start` for `@stack` and swallows the rest of the file
into an ERROR).

## Build constraint

The grammar's `parser.c` is 10-23 MB. Compiled to the engine's wasm32-wasi module with zig 0.16.0 (clang),
v0.11.0 builds (1.25 MB raw, 186 KB gzip, 49 s); v0.12.0, v0.12.3 and `main` are killed by the 14 GB
memory limit of the measuring VM at every optimisation level, `-O0` included. The wasm blob for `main`
therefore could not be built here; its error rate was measured with a native build, which is the same
`parser.c`. The gate does not turn on this: no pin measured passes it.

## Not measured

Laravel's view finder oracle (tier A) installs here (`illuminate/view` 11.51.0 by Composer, source
installs; PHP 8.3.6), but with no parsing grammar there is no dialect to grade against it. The component
tags (`<x-forms.input>`, `<livewire:counter>`) are plain `text` nodes in v0.11.0, so even a parsing grammar
would give no structural capture of them.

## Trigger to retry

A tree-sitter Blade grammar commit that parses laravel/breeze and filamentphp/filament with at most 5% of
files in error, built into a blob on a machine with enough memory for its parser. Then LB-Q-blade is worked
as the recipe (§13) says, with BookStack's `resources/views` (and a Laravel `FileViewFinder` oracle) as the
file-edge subject.
