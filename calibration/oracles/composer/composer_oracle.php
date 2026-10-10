<?php
// composer_oracle.php — the language-breadth harness's oracle for the composer
// doc-engine dialect (design §1.3). It writes indexbench lang-oracle's
// tool-oracle JSON (-tool-oracle), tier A:
//
//   php composer_oracle.php --phar <composer.phar> --subject <checkout> [--files-out <file>] > composer-<subject>.json
//
// The tool is Composer itself. Each composer.json is loaded the way Composer
// loads it for any command: Composer\Factory::create() builds the root package
// (Composer\Package\Loader\RootPackageLoader), which is what `composer validate`,
// `composer show --self` and `composer run-script --list` all read. Nothing in
// this program reads the manifest's structure itself except the line placer
// below; every name comes from the root package:
//
//   - definitions: one `package` for a manifest that names itself (the root
//     package's pretty name; an unnamed manifest is Composer's `__root__`, no
//     definition), at line 1 (the entity is the manifest itself, as the cargo
//     oracle places its package); one `task` for each script of the root
//     package (RootPackageInterface::getScripts(), what `composer run-script
//     --list` lists).
//   - references: (package, dependency) for each link of getRequires() and
//     getDevRequires(), platform packages (php, ext-*, lib-*) included, as
//     Composer lists them; and (script, script) for each listener that is a
//     script reference by Composer's own rule, EventDispatcher::isComposerScript()
//     (read with reflection: a command beginning with "@" that is not "@php ..."
//     or "@putenv ..."), not "@composer ...", and names a script the package
//     defines (Composer warns "a reference to a non-existent script" otherwise).
//
// The tool writes no line, so a script is placed at the line its key is written
// on: a small JSON scanner (jsonKeyLines) finds each member of the "scripts"
// object. That is the only structure this program reads itself, and it decides
// no name. A dependency is written as the manifest spells it (Composer
// lower-cases a link's target).
//
// A manifest Composer rejects (a load error) is named on stderr, contributes
// nothing and is left out of --files-out. Bench-only; never a product dependency.

declare(strict_types=1);

$opts = getopt('', ['phar:', 'subject:', 'files-out::']);
if (!isset($opts['phar'], $opts['subject'])) {
    fwrite(STDERR, "usage: php composer_oracle.php --phar <composer.phar> --subject <checkout> [--files-out <file>]\n");
    exit(2);
}
require 'phar://' . $opts['phar'] . '/vendor/autoload.php';

use Composer\Composer;
use Composer\EventDispatcher\EventDispatcher;
use Composer\Factory;
use Composer\IO\NullIO;

/**
 * jsonKeyLines maps every object member of a JSON document to the line its key
 * is written on, keyed by the member's path (names joined with "\0"). A strict
 * enough scanner for a manifest Composer already accepted.
 */
function jsonKeyLines(string $raw): array
{
    $n = strlen($raw);
    $i = 0;
    $line = 1;
    $out = [];
    $skip = function () use (&$i, &$line, $raw, $n): void {
        while ($i < $n && ctype_space($raw[$i])) {
            if ($raw[$i] === "\n") {
                $line++;
            }
            $i++;
        }
    };
    $str = function () use (&$i, $raw): string {
        $start = ++$i; // past the opening quote
        while ($raw[$i] !== '"') {
            $i += $raw[$i] === '\\' ? 2 : 1;
        }
        $s = substr($raw, $start, $i - $start);
        $i++;
        return (string) json_decode('"' . $s . '"');
    };
    $value = null;
    $value = function (array $path) use (&$value, &$i, &$line, &$out, $raw, $n, $skip, $str): void {
        $skip();
        $c = $raw[$i];
        if ($c === '{') {
            $i++;
            $skip();
            while ($raw[$i] !== '}') {
                $keyLine = $line;
                $key = $str();
                $skip();
                $i++; // the colon
                $p = array_merge($path, [$key]);
                $out[implode("\0", $p)] = $keyLine;
                $value($p);
                $skip();
                if ($raw[$i] === ',') {
                    $i++;
                    $skip();
                }
            }
            $i++;
        } elseif ($c === '[') {
            $i++;
            $skip();
            while ($raw[$i] !== ']') {
                $value($path);
                $skip();
                if ($raw[$i] === ',') {
                    $i++;
                    $skip();
                }
            }
            $i++;
        } elseif ($c === '"') {
            $str();
        } else {
            while ($i < $n && strpos(",]} \t\r\n", $raw[$i]) === false) {
                $i++;
            }
        }
    };
    $value([]);
    return $out;
}

function manifests(string $root): array
{
    $out = [];
    $it = new RecursiveIteratorIterator(new RecursiveCallbackFilterIterator(
        new RecursiveDirectoryIterator($root, FilesystemIterator::SKIP_DOTS),
        function ($f) {
            return !($f->isDir() && in_array($f->getFilename(), ['.git', 'vendor', 'node_modules'], true));
        }
    ));
    foreach ($it as $f) {
        if ($f->isFile() && $f->getFilename() === 'composer.json') {
            $out[] = substr($f->getPathname(), strlen(rtrim($root, '/')) + 1);
        }
    }
    sort($out);
    return $out;
}

$subject = rtrim($opts['subject'], '/');
$version = Composer::getVersion();
$defs = [];
$refs = [];
$accepted = [];
$stats = ['manifests' => 0, 'rejected' => 0, 'unnamed' => 0, 'scripts' => 0, 'scriptRefs' => 0, 'requires' => 0];

foreach (manifests($subject) as $rel) {
    $stats['manifests']++;
    $abs = $subject . '/' . $rel;
    try {
        $composer = Factory::create(new NullIO(), $abs, true);
    } catch (Throwable $e) {
        $stats['rejected']++;
        fwrite(STDERR, "  rejected by composer: $rel: " . strtok($e->getMessage(), "\n") . "\n");
        continue;
    }
    $accepted[] = $rel;
    $pkg = $composer->getPackage();
    $raw = (string) file_get_contents($abs);
    $decoded = json_decode($raw, true) ?: [];
    $lines = jsonKeyLines($raw);

    $name = $pkg->getPrettyName();
    if ($name === '__root__') {
        $stats['unnamed']++;
    } else {
        $defs[] = ['path' => $rel, 'name' => $name, 'line' => 1, 'kind' => 'package'];
        // dependencies as the manifest spells them, found through Composer's links
        $written = [];
        foreach (['require', 'require-dev'] as $section) {
            foreach (array_keys($decoded[$section] ?? []) as $k) {
                $written[strtolower((string) $k)] = (string) $k;
            }
        }
        $seen = [];
        foreach (array_merge($pkg->getRequires(), $pkg->getDevRequires()) as $link) {
            $to = $written[$link->getTarget()] ?? $link->getTarget();
            if (!isset($seen[$to])) {
                $seen[$to] = true;
                $refs[] = ['path' => $rel, 'from' => $name, 'to' => $to];
                $stats['requires']++;
            }
        }
    }

    $scripts = $pkg->getScripts();
    $isScript = new ReflectionMethod(EventDispatcher::class, 'isComposerScript');
    $dispatcher = $composer->getEventDispatcher();
    foreach ($scripts as $script => $listeners) {
        $stats['scripts']++;
        $defs[] = ['path' => $rel, 'name' => (string) $script, 'line' => $lines["scripts\0$script"] ?? 1, 'kind' => 'task'];
        $to = [];
        foreach ((array) $listeners as $callable) {
            if (!is_string($callable) || !$isScript->invoke($dispatcher, $callable) || strpos($callable, '@composer ') === 0) {
                continue;
            }
            $target = explode(' ', substr($callable, 1))[0];
            if (isset($scripts[$target])) {
                $to[$target] = true;
            }
        }
        foreach (array_keys($to) as $t) {
            $refs[] = ['path' => $rel, 'from' => (string) $script, 'to' => (string) $t];
            $stats['scriptRefs']++;
        }
    }
}

usort($defs, fn($a, $b) => [$a['path'], $a['line'], $a['kind'], $a['name']] <=> [$b['path'], $b['line'], $b['kind'], $b['name']]);
usort($refs, fn($a, $b) => [$a['path'], $a['from'], $a['to']] <=> [$b['path'], $b['from'], $b['to']]);
echo json_encode([
    'tool' => 'composer (Factory::create root package: getPrettyName, getScripts, getRequires, getDevRequires; EventDispatcher::isComposerScript)',
    'version' => "composer $version",
    'tier' => 'A',
    'definitions' => $defs,
    'references' => $refs,
], JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE), "\n";
if (isset($opts['files-out']) && is_string($opts['files-out'])) {
    file_put_contents($opts['files-out'], implode('', array_map(fn($f) => $f . "\n", $accepted)));
}
fwrite(STDERR, 'composer oracle: ' . json_encode($stats) . "\n");
