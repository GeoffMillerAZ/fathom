<?php
// PhpParserDump — tier-B PHP L1/L2 oracle (syntax only). ADR-048 amendment D2.
//
// Walks every *.php file under <subject> (vendor/ and .git/ skipped) with
// nikic/php-parser and writes `indexbench lang-oracle -tool-oracle`'s neutral
// JSON (definitions with end lines, calls) to stdout. A syntax dump: callee
// names are as written, never resolved. Bench-only, never a product
// dependency; nothing in the subject is executed.
//
// usage: php PhpParserDump.php <autoload.php of a php-parser install> <subject> [definitions-only] > out.json
// (definitions-only omits `calls`: lang-oracle takes one calls oracle per subject, and a
// SCIP index already is one)
//
// Definitions (kind names: namespace class interface trait enum function
// method; names are bare, as the neutral shape requires — a namespace is its
// last segment, `Cookie` for GuzzleHttp\Cookie): a braced namespace spans its braces; a `namespace A\B;` clause is
// the definition's own line (the tags convention, the clause is the
// definition; it scopes to the end of the file, which is not a range worth
// comparing). A method or function `line` is its name's line, `end` the last
// line of the node. Anonymous classes are not definitions; their methods are.
// Calls: (innermost enclosing function or method, callee bare name) for every
// function call with a literal name, method call, nullsafe method call and
// static call with a literal name; `new X` is a type reference, isset/empty/
// exit/eval are their own nodes. Top-level calls are not pairs.

declare(strict_types=1);

use PhpParser\Node;
use PhpParser\NodeTraverser;
use PhpParser\NodeVisitorAbstract;
use PhpParser\ParserFactory;
use PhpParser\ErrorHandler;

require $argv[1];
$root = rtrim($argv[2], '/');

final class Dump extends NodeVisitorAbstract
{
    /** @var array<int,array<string,mixed>> */
    public array $defs = [];
    /** @var array<int,array<string,mixed>> */
    public array $calls = [];
    /** @var string[] */
    private array $stack = [];

    public function __construct(private string $path)
    {
    }

    public function enterNode(Node $n)
    {
        $def = null;
        if ($n instanceof Node\Stmt\Namespace_ && $n->name !== null) {
            $end = $n->getAttribute("kind") === Node\Stmt\Namespace_::KIND_SEMICOLON ? $n->getStartLine() : $n->getEndLine();
            $this->add('namespace', $n->name->getLast(), $n->name->getStartLine(), $end);
        } elseif ($n instanceof Node\Stmt\Class_ && $n->name !== null) {
            $this->add('class', $n->name->toString(), $n->name->getStartLine(), $n->getEndLine());
        } elseif ($n instanceof Node\Stmt\Interface_) {
            $this->add('interface', $n->name->toString(), $n->name->getStartLine(), $n->getEndLine());
        } elseif ($n instanceof Node\Stmt\Trait_) {
            $this->add('trait', $n->name->toString(), $n->name->getStartLine(), $n->getEndLine());
        } elseif ($n instanceof Node\Stmt\Enum_) {
            $this->add('enum', $n->name->toString(), $n->name->getStartLine(), $n->getEndLine());
        } elseif ($n instanceof Node\Stmt\Function_) {
            $this->add('function', $n->name->toString(), $n->name->getStartLine(), $n->getEndLine());
            $def = $n->name->toString();
        } elseif ($n instanceof Node\Stmt\ClassMethod) {
            $this->add('method', $n->name->toString(), $n->name->getStartLine(), $n->getEndLine());
            $def = $n->name->toString();
        }
        if ($def !== null) {
            $this->stack[] = $def;
            return null;
        }
        $callee = null;
        if ($n instanceof Node\Expr\FuncCall && $n->name instanceof Node\Name) {
            $callee = $n->name->getLast();
        } elseif (($n instanceof Node\Expr\MethodCall || $n instanceof Node\Expr\NullsafeMethodCall
                || $n instanceof Node\Expr\StaticCall) && $n->name instanceof Node\Identifier) {
            $callee = $n->name->toString();
        }
        if ($callee !== null && $this->stack) {
            $this->calls[] = ['path' => $this->path, 'from' => end($this->stack), 'to' => $callee];
        }
        return null;
    }

    public function leaveNode(Node $n)
    {
        if ($n instanceof Node\Stmt\Function_ || $n instanceof Node\Stmt\ClassMethod) {
            array_pop($this->stack);
        }
        return null;
    }

    private function add(string $kind, string $name, int $line, int $end): void
    {
        $this->defs[] = ['path' => $this->path, 'name' => $name, 'line' => $line, 'end' => $end, 'kind' => $kind];
    }
}

$parser = (new ParserFactory())->createForNewestSupportedVersion();
$files = [];
$it = new RecursiveIteratorIterator(new RecursiveCallbackFilterIterator(
    new RecursiveDirectoryIterator($root, FilesystemIterator::SKIP_DOTS),
    function (SplFileInfo $f) {
        return !($f->isDir() && in_array($f->getFilename(), ['vendor', '.git'], true));
    }
));
foreach ($it as $f) {
    if ($f->isFile() && substr($f->getFilename(), -4) === '.php') {
        $files[] = $f->getPathname();
    }
}
sort($files);

$defs = [];
$calls = [];
$failed = 0;
foreach ($files as $file) {
    $rel = substr($file, strlen($root) + 1);
    $errors = new ErrorHandler\Collecting();
    $ast = $parser->parse((string) file_get_contents($file), $errors);
    if ($errors->hasErrors()) {
        $failed++;
    }
    if ($ast === null) {
        continue;
    }
    $v = new Dump($rel);
    $t = new NodeTraverser();
    $t->addVisitor($v);
    $t->traverse($ast);
    array_push($defs, ...$v->defs);
    array_push($calls, ...$v->calls);
}
fwrite(STDERR, sprintf("%d files, %d definitions, %d call pairs, %d with parse errors\n", count($files), count($defs), count($calls), $failed));
echo json_encode([
    'tool' => 'php-parser',
    'version' => \Composer\InstalledVersions::getVersion('nikic/php-parser'),
    'tier' => 'B',
    'definitions' => $defs,
    'calls' => ($argv[3] ?? '') === 'definitions-only' ? [] : $calls,
], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_PRETTY_PRINT), "\n";
