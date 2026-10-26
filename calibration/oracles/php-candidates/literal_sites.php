<?php
// Syntax-only denominator. No row from this file is native call truth.
declare(strict_types=1);
require $argv[1];
use PhpParser\Node;
use PhpParser\NodeTraverser;
use PhpParser\NodeVisitorAbstract;

final class LiteralSites extends NodeVisitorAbstract {
    public array $sites = [];
    public array $nonliteral = [];
    private array $stack = [];
    public function __construct(private string $path) {}
    public function enterNode(Node $node) {
        if ($node instanceof Node\Stmt\Function_ || $node instanceof Node\Stmt\ClassMethod) {
            $this->stack[] = $node->name->toString();
        }
        $name = null;
        if ($node instanceof Node\Expr\FuncCall && $node->name instanceof Node\Name) {
            $name = $node->name->getLast();
        } elseif (($node instanceof Node\Expr\MethodCall || $node instanceof Node\Expr\NullsafeMethodCall
                   || $node instanceof Node\Expr\StaticCall) && $node->name instanceof Node\Identifier) {
            $name = $node->name->toString();
        }
        if ($name !== null && $this->stack) {
            $this->sites[] = ['path' => $this->path, 'line' => $node->getStartLine(), 'end' => $node->getEndLine(),
                'from' => end($this->stack), 'written_name' => $name];
        }
        if (($node instanceof Node\Expr\FuncCall || $node instanceof Node\Expr\MethodCall
             || $node instanceof Node\Expr\NullsafeMethodCall || $node instanceof Node\Expr\StaticCall)
            && $name === null || $node instanceof Node\Expr\New_) {
            $this->nonliteral[] = ['path' => $this->path, 'line' => $node->getStartLine(), 'end' => $node->getEndLine()];
        }
        return null;
    }
    public function leaveNode(Node $node) {
        if ($node instanceof Node\Stmt\Function_ || $node instanceof Node\Stmt\ClassMethod) {
            array_pop($this->stack);
        }
        return null;
    }
}
$root = realpath($argv[2]);
$parser = (new PhpParser\ParserFactory())->createForNewestSupportedVersion();
$files = new RecursiveIteratorIterator(new RecursiveCallbackFilterIterator(
    new RecursiveDirectoryIterator($root, FilesystemIterator::SKIP_DOTS),
    static fn ($file) => !$file->isDir() || (!str_starts_with($file->getFilename(), '.')
        && !in_array($file->getFilename(), ['vendor', 'node_modules'], true))));
$sites = [];
$nonliteral = [];
$count = 0;
foreach ($files as $file) {
    if (!$file->isFile() || $file->getExtension() !== 'php') { continue; }
    $path = substr($file->getPathname(), strlen($root)+1);
    $visitor = new LiteralSites($path);
    $traverser = new NodeTraverser($visitor);
    $traverser->traverse($parser->parse(file_get_contents($file->getPathname())));
    array_push($sites, ...$visitor->sites);
    array_push($nonliteral, ...$visitor->nonliteral);
    $count++;
}
echo json_encode(['classification' => 'B syntax denominator only', 'files' => $count, 'sites' => $sites,
    'nonliteral_call_lines' => $nonliteral], JSON_THROW_ON_ERROR), PHP_EOL;
