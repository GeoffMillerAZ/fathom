# Blade: measured parser limit and X-R4 source audit

Source audit 2026-10-25; additional native grammar screens measured 2026-10-26.
**Measured:** neither omitted licensed source grammar passes the same 254-file
parse gate: deanrumsby has 94 errors (37.01%) and codehappyness 35 (13.78%),
with zero timeouts or infrastructure failures. The best recorded EmranMR pin
remains 20/254 (7.87%), above the allowable 12. No extraction accuracy, WASM,
throughput, SCIP, LSP or runtime-trace grade is claimed. The refusal holds on
the named candidate list below. No canonical shard or plan checkbox changed.

Existing measurement: [Blade reading](../bench/readings/lang/blade-grammar.md) pins EmranMR/tree-sitter-blade `b5291d1ba207` at 26/254 ERROR files and best v0.11.0 at 20/254, both over 5%; newer parser C could not build to WASM on the 14 GB VM. The earlier source survey records `illuminate/view` 11.51.0 `FileViewFinder` as an installed, ungraded view-edge oracle. The upstream [Laravel BladeCompiler API](https://api.laravel.com/docs/11.x/Illuminate/View/Compilers/BladeCompiler.html) documents `compileString`, and the [Laravel view docs](https://laravel.com/docs/11.x/views) document `view:cache`. For an L3 file-to-file oracle, pin Laravel 11.51.0, compile every canonical `.blade.php` file with `BladeCompiler::compileString`, enumerate every emitted view reference independently of fathom's parse, and resolve each name with `FileViewFinder` under the application's configured paths. Count dynamic names as unresolvable rather than dropping them. Generated PHP lacks Blade source columns, but L3 file-to-file edges do not need columns. Application boot/config still bounds resolution; the measured product grammar gate remains refused.

## Product grammar candidates (F1)

The repeatable [GitHub repository search](https://github.com/search?q=tree-sitter-blade&type=repositories)
returned these six repositories. Licence decisions use files at the pinned
source, not GitHub's inferred licence label. All native gate runs use
`tree-sitter-cli 0.26.11 parse -q --lib-path ... --lang-name blade`, compiled
from the candidate C sources with `cc -O0`, 10 seconds per file. The subjects
are the same clean pinned laravel/breeze (57 files, 126356 bytes) and
filamentphp/filament (197 files, 589903 bytes). Nothing was excluded.

| Candidate / pin | Licence at pin | Concrete disposition |
|---|---|---|
| [EmranMR](https://github.com/EmranMR/tree-sitter-blade/tree/b5291d1ba207a8ebb8383b2ecb8a8a6535210a50), main `b5291d1ba207`, v0.12.3, v0.11.0, v0.9.2 | MIT | Prior native errors 26/26/20/35 of 254. Every pin fails <=12. Above v0.11.0, the prior WASM build also hit its memory limit. |
| [deanrumsby](https://github.com/deanrumsby/tree-sitter-blade/tree/e2ff089ded9351aeac9f4402225c5d49e8c0067d) `e2ff089ded93` (package 0.1.0) | MIT, LICENSE checked | **Measured 94/254** errors, 0 timeouts/infrastructure failures; parser 298033 bytes plus scanner. Small enough to avoid the recorded source-size issue, but syntax fails the gate. |
| [codehappyness](https://github.com/codehappyness/tree-sitter-blade/tree/9067259bebeed6be0d87b82f7ef93f20b2126156) `9067259bebee` (package 0.9.2) | MIT, LICENSE checked | **Measured 35/254** errors, 0 timeouts/infrastructure failures; parser 9379521 bytes. Native build succeeds, syntax fails the gate. |
| [KaranAhlawat](https://github.com/KaranAhlawat/tree-sitter-blade/tree/2e73ddedbba590721b7df009b399f5b43d2acc79) `2e73ddedbba5` (package 0.0.1) | No licence file; package declares GPLv3 | Experimental source is outside ADR-048's allowlist. Not built or counted. |
| [lovesowing](https://github.com/lovesowing/tree-sitter-blade/tree/9764b7b5788ec57a6110eb2a3367835bd10f46cd) `9764b7b5788e` (package 0.1.0) | Package says MIT, no licence file | No src/parser.c or complete parser source. Not a buildable licensed candidate at this pin. |
| [gcavanunez/tree-sitter-blade-wasm](https://github.com/gcavanunez/tree-sitter-blade-wasm/tree/985d55c07d8b692ae4f9d484e10578c413fc4bf9) `985d55c07d8b` (package 0.0.1) | Pinned LICENSE says MIT (GitHub classification was NOASSERTION) | Repackages EmranMR for web-tree-sitter; no parser C source. It is not an independent grammar or the self-contained core+grammar WASM shape admitted by ADR-048. |

### Structure controls and reproduction

A low ERROR rate alone would not admit a grammar. The committed authored
controls exercise named sections, includes, a custom directive, a component
and a named slot. Deanrumsby reports ERROR for the section/custom-directive
control; its component/slot has HTML element/tag_name/attribute structure,
which could support name captures, but does not repair the syntax gate.
Codehappyness reports MISSING in the directive control and reads the whole
component/slot control as one text node, leaving no structural name capture.
No L1 grade follows from either screen.

Reproduce with `calibration/oracles/blade-grammar-screen/screen.py` and its
README. Exact aggregate receipts, source/subject hashes and control-node
counts are in `data/2026-10-26-blade-{dean,code}-native.json`. The authored
control ASTs can be regenerated under workspace; subject ASTs/file lists are
not committed. The script's exit 1 means a measured gate refusal; an
infrastructure exception produces no receipt and must not be called a result.

## Five routes checked

1. **Own compiler/runtime dump/introspection.** Laravel `BladeCompiler::compileString` and `FileViewFinder` above provide an unrun L3 oracle route. Precise Blade source spans would need additional work for a source-positioned grade. [Laravel Surveyor v0.3.0](https://packagist.org/packages/laravel/surveyor) analyzes PHP and Laravel classes; its README does not describe Blade or view references, so it is not a Blade oracle. [Laravel Ranger v0.5.1](https://packagist.org/packages/laravel/ranger) builds on Surveyor and collects application components; its README likewise does not describe Blade or view references. No compiler or edge run here.
2. **SCIP and LSIF.** The [Sourcegraph recommended indexer list](https://sourcegraph.com/docs/code-navigation/writing-an-indexer) has **10 language rows** (none PHP/Blade), and the [SCIP protocol repository list](https://github.com/scip-code/scip/blob/main/README.md) has **10 named entries**, including community `scip-php` for PHP but none for Blade. Separately, archived [LSIF.dev's implementation table](https://lsif.dev/) has **16 rows**, none PHP/Blade; its page says LSIF has been superseded by SCIP. [Sourcegraph's LSIF migration note](https://sourcegraph.com/docs/admin/how-to/lsif-scip-migration) says LSIF upload is converted to SCIP. Public GitHub repository searches for [`scip-php`](https://github.com/search?q=scip-php&type=repositories), [`scip-laravel`](https://github.com/search?q=scip-laravel&type=repositories), and [`scip-blade`](https://github.com/search?q=scip-blade&type=repositories) exposed the named PHP implementations below; `scip-blade` returned zero repositories at inspection. This is not evidence that PHP indexing covers Blade.

   - [davidrjenni/scip-php](https://github.com/davidrjenni/scip-php), upstream Composer PHP producer: [Packagist v0.0.2](https://packagist.org/packages/davidrjenni/scip-php) pins `b12e0593cfb7`; README requires Composer manifest/lock and installed vendor/autoloader. No Blade claims.
   - [michalboryczko/scip-php](https://github.com/michalboryczko/scip-php/blob/main/README.md), explicitly a **fork** of David Jenni's indexer, with call-site/value tracking and `index.scip` plus `index.json`; inspected README blob `c5a333408e8a`, search-result commit [`40288605`](https://github.com/michalboryczko/scip-php/commit/40288605f28ea0111b42f81a8760ad5b79b8c3a0). No Blade claim or release pin established.
   - [ArtemisMucaj/scip-php](https://github.com/ArtemisMucaj/scip-php/blob/main/README.md), a **separate Rust/Mago implementation**, PSR-4 `.php` discovery and standard SCIP; inspected README blob `7b2172d1586c`, search-result commit [`04b34f72`](https://github.com/ArtemisMucaj/scip-php/commit/04b34f724c9464256891345153b5c2c0a6c23bc8). No Blade claim or release pin established.
   - [bnomei/scip-laravel](https://github.com/bnomei/scip-laravel), [Packagist v1.0.0](https://packagist.org/packages/bnomei/scip-laravel) pins `6f9a033b46fe`; it vendors and patches the upstream PHP indexer and explicitly emits Blade view/component edges and supported Blade/Volt references on Laravel 11–13/PHP >=8.3. Its default `full` mode boots the app; `safe` mode applies runtime overrides. Probe document and edge recall on pinned subjects before admission.
   - [jdrly/scip-laravel](https://github.com/jdrly/scip-laravel/blob/main/README.md), separate standalone implementation, inspected README blob `1ff2d60175f4`, search-result commit [`3a755289`](https://github.com/jdrly/scip-laravel/commit/3a755289b52e851f2c08d4b7d099fbc54e2e9483). README calls it early development and says current output is internal JSON; final SCIP protobuf comes later. Its route/provider extraction does not yet establish Blade view edges. No release pin established.

   [jelmer/scip-tools `scip-tree-sitter`](https://github.com/jelmer/scip-tools/blob/main/scip-tree-sitter/README.md) is separately a syntax-token SCIP emitter for PHP with **no navigation**; it is not one of the three named `scip-php` implementations. None of these producers was run here.
3. **Language servers.** [laravel/lsp](https://github.com/laravel/lsp) [Packagist v0.0.32](https://packagist.org/packages/laravel/lsp), `557d8c959616`, handles PHP and Blade view/component completions, hovers, diagnostics and document links; definition support resolves enabled document links. Its development dependencies include `stillat/blade-parser ^2.1`. [laravel-ls/laravel-ls v0.1.0](https://github.com/laravel-ls/laravel-ls/releases/tag/v0.1.0) (release page short commit `c306833`; README blob at that tag `ccf444e4922e`) documents view go-to-definition but describes itself as early development. [Phpactor release 2026.07.22.0](https://github.com/phpactor/phpactor/releases/tag/2026.07.22.0) (release page short commit `1b3b0f3`; README blob at that tag `71843241f933`) is a PHP server, not evidence of Blade edge coverage. These are inspected, unrun routes. Probe pinned `laravel/lsp` on fixed view and component edges as an LSP oracle. Search `site:github.com "blade language server" "definition"` did not add a better independently documented Blade server.
4. **Registry parsers/typecheckers/AST dumps.** Actual Packagist [search API `blade-parser`](https://packagist.org/search.json?q=blade-parser) returned **4 packages**: `stillat/blade-parser`, `fortephp/forte`, `tacman/blade-parser`, `dionkeldei/php-blade-parser`. [Stillat/blade-parser v2.1.0](https://packagist.org/packages/stillat/blade-parser), `23a962f99bca`, parses Blade nodes, documents/workspaces and validates templates; no corpus grade. [Forte v1.1.0](https://repo.packagist.org/p2/fortephp/forte.json), `9c9597d0b708`, is a distinct fault-tolerant Blade AST library with diagnostics, first-class components/directives, and deep echo traversal ([upstream README](https://github.com/fortephp/forte)); no corpus grade. [tacman/blade-parser `dev-master`](https://packagist.org/packages/tacman/blade-parser), `76b61854b270`, carries the older `Stillat\BladeParser` API and README; it has no tagged release or Packagist license metadata, so it is not an independent parser candidate. [dionkeldei/php-blade-parser 0.2](https://repo.packagist.org/p2/dionkeldei/php-blade-parser.json), MIT at `d8def58ec163`, describes Blade template variable detection, not a general Blade AST or view-edge oracle; no independent lineage claim is made. Packagist [search `scip-php`](https://packagist.org/search.json?q=scip-php) returned 5 fuzzy matches, only David Jenni's package a SCIP PHP indexer; [search `blade-language-server`](https://packagist.org/search.json?q=blade-language-server) and [`laravel-lsp`](https://packagist.org/search.json?q=laravel-lsp) returned zero exact-name hits despite `laravel/lsp` existing, so search ranking is bounded. [blade-formatter](https://www.npmjs.com/package/blade-formatter) and its [Prettier plugin](https://www.npmjs.com/package/%40shufo/prettier-plugin-blade) offer formatting, but the plugin says the formatter does not generate a lexer AST and may break complex indentation. Pin and probe Stillat and Forte for parse acceptance, node positions, and edge extraction.
5. **Runtime call/require tracing.** [Xdebug 3.5.3](https://xdebug.org/download) was the stable release listed at inspection (3.6.0alpha1 is preview); its [function traces](https://xdebug.org/docs/trace) capture executed PHP calls and arguments, and [profiler docs](https://xdebug.org/docs/profiler) describe `include`/`require` entries with filenames. A Laravel render with Xdebug can witness actual view and component selection, but only for executed requests, and source paths can be compiled PHP cache paths. No trace was run. Search `Laravel view composer rendering event view path rendered views trace official docs` returned Laravel view docs, not a turnkey static Blade edge dump. Runtime witness is an independent sampled check, not a corpus-wide parser oracle.

## Retry and product admission

The measured gate remains 20/254 for the best recorded grammar, above the
allowable floor(0.05 * 254) = 12. An allowlisted grammar or reviewed P-R1
source patch must clear at least 20 - 12 = 8 gate-error files on the same
pinned corpus before a dialect grade. No Blade patch was measured here.
The PHP/Laravel producers and AST libraries above are real routes to evaluate
for independent definitions, calls and view edges after product admission;
their existence is not proof that the Blade parser gate has passed. The six
named repositories above include both newly measured licensed source candidates;
neither passes, so the recorded refusal stands. A future pass must also expose
the names and structural nodes L1 needs, followed by the normal WASM and full
independent oracle measurement. BookStack remains a separate future check, never
an addition to the canonical shard.
