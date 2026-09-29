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
