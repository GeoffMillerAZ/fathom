# scip-php — tier-A PHP index (calls, def_mapping, file edges)

Installed without sudo into a tools dir (nothing global):

    curl -L https://dl.static-php.dev/static-php-cli/common/php-8.5.8-cli-macos-aarch64.tar.gz | tar xz -C <tools>   # PHP 8.5.8, static CLI
    curl -L https://getcomposer.org/download/latest-stable/composer.phar -o <tools>/composer.phar
    git clone --depth 1 https://github.com/davidrjenni/scip-php <tools>/scip-php                                       # HEAD 71a5b117 (2026-05-02), MIT
    (cd <tools>/scip-php && <tools>/php <tools>/composer.phar install --no-dev)

scip-php has no release after v0.0.2 (2023); HEAD needs PHP 8.3 and reports `tool_version` 0.0.1. It must run
in the subject's root, which needs `composer.json`, `composer.lock` and an up-to-date `vendor/`.

Run (2026-09-28, both clean; the vendor trees and the two edits below are deleted with the clones):

    # guzzle/guzzle 7.9.3: composer install, minus guzzle/client-integration-tests (it needs the composer-bin
    # plugin's second install and plugins are off), then run
    <tools>/php -d memory_limit=-1 <tools>/composer.phar remove --dev guzzle/client-integration-tests --no-interaction --no-plugins --ignore-platform-reqs
    <tools>/php -d memory_limit=-1 <tools>/scip-php/bin/scip-php --memory-limit=8G         # 0.7 s, 57 documents
    # Seldaek/monolog 3.9.0: config.lock is false, so composer.lock never exists; flip it, then update
    <tools>/php -d memory_limit=-1 <tools>/composer.phar config lock true
    <tools>/php -d memory_limit=-1 <tools>/composer.phar update --no-interaction --no-plugins --ignore-platform-reqs   # 150 MB vendor
    <tools>/php -d memory_limit=-1 <tools>/scip-php/bin/scip-php --memory-limit=8G         # 2.7 s, 214 documents

What the index carries: no `enclosing_range` on any of its 4,334 definitions and no `SymbolInformation.kind`
(only the descriptor suffix: `#` type, `().` method or function, `.` property, constant or enum case), so a
definition's span for call attribution is the mapped fathom definition's own lines (D5) and
`internal/scip.ImportIndex` attributes no reference to it. It indexes `src/` and `tests/` (composer's
autoload and autoload-dev roots) and PHP's builtin functions through phpstorm-stubs, so `strlen` is a
callable reference. It records no reference for a method called on a vendor-typed receiver (a
`Psr\Http\Message\RequestInterface` parameter's `getUri()`, `getBody()`, `hasHeader()`) and none for a
constructor (`new X` is a reference to the class), which is why call_precision is measured short of the
syntactic truth the php-parser dump (`../php-parser/`) gives.

Re-run 2026-10-01 (wave la re-measure, fathom d8ed9a29): Homebrew `PHP 8.5.11 (cli) (NTS)` and `composer 2.10.3`
replace the static 8.5.8 CLI. scip-php cloned at `71a5b117ec4c5dd2af302e363410e604e5df309e` (author date 2026-04-27,
commit date 2026-05-02, `v0.0.2-816-g71a5b11`, `tool_version` 0.0.1) into `/Volumes/m2ext/tmp/lang-oracles/tools/scip-php`
and `composer install --no-dev` (5.8 s: php-parser v5.7.0, google/protobuf v5.34.1, phpstorm-stubs v2026.1,
phpdoc-parser 2.3.2). The recipe above ran unchanged in each clone's own workspace (guzzle `composer remove --dev
guzzle/client-integration-tests` 6.8 s, 8.5 MB vendor, scip-php 0.9 s, 827 KB index; monolog `config lock true` and
`composer update` 9.1 s, 150 MB vendor, scip-php 2.3 s, 2.4 MB index). The php-parser dump (`../php-parser/`, 5.7.0,
definitions-only) reads 74 files, 834 definitions, 3,189 call pairs on guzzle and 217, 1,814, 5,968 on monolog, 0
parse errors, as before. `lang-oracle -tool-oracle <dump> -scip <index>` re-read every L1, L2 and L4 metric and count
identically (call_precision .6924, def_mapping .7875), so no met level moved. New: `import_resolution` reads 1, 486
of 486 in-repo imports (monolog 400, guzzle 86; scip-php does write references on `use` lines), import edges
precision .8773 (486 of 554), recall 1, 43 of 897 imports resolved where the index names no file. typed_recall is
still unmeasured (no `enclosing_range`) and is run after F-A (item scip) lands the `ImportIndex` line-span fallback.
