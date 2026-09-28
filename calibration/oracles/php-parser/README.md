# PhpParserDump — tier-B PHP L1 oracle (syntax only)

ADR-048 amendment D2: universal-ctags 6.2.1 is deficient as a PHP L1 oracle. It writes no end line for
any PHP tag (L1 would cap at good, design 1.2), has no enum kind (an `enum` is not tagged and its methods
come out as functions of the enclosing namespace: 8 kind mismatches on monolog's `Level`), and names a
namespace with its full path where the bare-name comparison wants the last segment. nikic/php-parser, PHP's
own parser library, is the stronger independent oracle. This script walks its AST and writes the neutral
oracle file (`definitions`, and `calls` unless asked not to) that `indexbench lang-oracle -tool-oracle`
reads, at tier B: a syntax dump, so callee names are as written, never resolved (the same tier as the
Roslyn, JavaParser and PowerShell-AST dumps). Bench-only: a PHP process run beside the harness, never a
product dependency. Nothing in the subject is executed; `vendor/` and `.git/` are skipped.

    # a static PHP 8.5.8 CLI (dl.static-php.dev, no install), composer.phar 2.x, and php-parser from scip-php's install
    git clone --depth 1 https://github.com/davidrjenni/scip-php && (cd scip-php && php composer.phar install --no-dev)
    php -d display_errors=stderr PhpParserDump.php scip-php/vendor/autoload.php <clone>/guzzle definitions-only > out/php-guzzle.defs.json
    php -d display_errors=stderr PhpParserDump.php scip-php/vendor/autoload.php <clone>/monolog definitions-only > out/php-monolog.defs.json
    # guzzle/guzzle 7.9.3: 74 files, 834 definitions, 3,189 call pairs; monolog 3.9.0: 217 files, 1,814 definitions, 5,968; 0 parse errors

`definitions-only` leaves `calls` out because `lang-oracle` takes one calls oracle per subject and a SCIP
index already is one; drop the argument to score calls against this dump instead (tier B: L2 good at most).
php-parser 5.7.0. Kinds are mapped by `dialects/php/dialect.json` `oracle_kinds.tool` (namespace, class,
interface, trait, enum, function, method).

What it counts, and does not:

- Definitions: `Stmt\Namespace_` (a named one; its `end` is the clause's own line for `namespace A\B;`, the
  closing brace for the braced form; its name is the last segment, `Cookie` for `GuzzleHttp\Cookie`),
  `Class_` (named only), `Interface_`, `Trait_`, `Enum_`, `Function_` and `ClassMethod`. `line` is the
  name's line, `end` the node's last line. An anonymous class is not a definition; its methods are.
  Properties, class constants and enum cases are not emitted (the dialect does not extract them).
- Calls: (innermost enclosing function or method, callee bare name) for every `FuncCall` with a literal
  name, `MethodCall`, `NullsafeMethodCall` and `StaticCall` with a literal name. `new X` is a type
  reference; `isset`, `empty`, `exit` and `eval` are their own node types and are not calls. Top-level
  calls are not pairs.
