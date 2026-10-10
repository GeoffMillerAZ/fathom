<?php

declare(strict_types=1);

// TL-7's capture runner (docs/plans/telemetry-links.md): runs every
// fault-injected case, reads the truth from PHP's own structured trace
// (the root cause's throw location, and the first userland function of its
// trace, which encloses it), and writes one JSON line per case carrying the
// raw trace and the payload shapes a notifier, a tracer, a Lambda runtime
// layer and an APM agent would send. The app root is rewritten to /app (a
// container's deploy root; /var/task for Lambda).

use App\Accounts\Auth;
use App\Accounts\User;
use App\Billing\Invoice;
use App\Catalog\Product;
use App\Catalog\Search;
use App\Orders\Cart;
use App\Orders\Order;
use App\Shipping\Shipment;

$appRoot = dirname(__DIR__);

spl_autoload_register(function (string $class) use ($appRoot): void {
    if (str_starts_with($class, 'App\\')) {
        require $appRoot . '/src/' . str_replace('\\', '/', substr($class, 4)) . '.php';
    }
});

$cases = [
    'billing-total-div0' => fn () => (new Invoice([100, 200], 'USD'))->total(0),
    'billing-tax-null' => fn () => (new Invoice([100], 'USD'))->applyTax('US'),
    'billing-parse-amount' => fn () => (new Invoice([1], 'USD'))->parseAndAdd('12a'),
    'billing-ctor-currency' => fn () => new Invoice([], 'EURO'),
    'billing-refund-static' => fn () => Invoice::refund(new Invoice([10], 'USD'), 9999),
    'shipping-round-div0' => fn () => (new Shipment(1200, 'DE'))->packed(0),
    'shipping-band-bounds' => fn () => (new Shipment(5000, 'DE'))->band(),
    'shipping-validate' => fn () => (new Shipment(10, ''))->validate(),
    'shipping-track-null' => fn () => (new Shipment(10, 'FR'))->track([]),
    'catalog-validate' => fn () => (new Product('bad', '1'))->validate(),
    'catalog-walk-cycle' => fn () => (new Search(['a' => 'b', 'b' => 'a']))->depth('a'),
    'catalog-rank-missing' => fn () => (new Search([]))->rank(['A'], 'Z'),
    'catalog-price-parse' => fn () => (new Product('ABC-1', 'x'))->price(),
    'orders-add-line' => fn () => (new Order(5))->addLine(0),
    'orders-coupon-null' => fn () => (new Cart(new Order(5), []))->applyCoupon('NOPE'),
    'orders-checkout-wrapped' => fn () => (new Cart(new Order(5), []))->checkout(99),
    'orders-summarize-closure' => function (): void {
        $o = new Order(5);
        $o->addLine(3);
        $o->summarize(0);
    },
    'orders-deep-chain' => fn () => (new Order(1))->deep(5),
    'accounts-rename' => fn () => (new User('a', 'a@b.c'))->rename('Single'),
    'accounts-verify' => fn () => (new Auth('s3cret'))->verify('token'),
    'accounts-hash-algorithm' => fn () => (new Auth('s'))->hashPassword('p', 'NOPE-256'),
    'accounts-domain' => fn () => (new User('a', 'nope'))->domain(),
];

function rootCause(Throwable $t): Throwable
{
    while ($t->getPrevious() !== null) {
        $t = $t->getPrevious();
    }
    return $t;
}

function isUserland(array $frame): bool
{
    $fn = $frame['function'] ?? '';
    if (str_starts_with($fn, '{closure')) {
        return true;
    }
    try {
        if (isset($frame['class'])) {
            return !(new ReflectionMethod($frame['class'], $fn))->isInternal();
        }
        return !(new ReflectionFunction($fn))->isInternal();
    } catch (ReflectionException) {
        return false;
    }
}

/** The bare name the index gives a function: a closure is its enclosing method's. */
function bareName(string $fn): string
{
    if (preg_match('/^\{closure:(?:[\w\\\\]+::)?(\w+)\(\)/', $fn, $m) === 1) {
        return $m[1];
    }
    return $fn;
}

function deployed(string $text, string $appRoot, string $root): string
{
    return str_replace($appRoot . '/', $root . '/', $text);
}

function rel(string $file, string $appRoot): string
{
    return str_starts_with($file, $appRoot . '/') ? substr($file, strlen($appRoot) + 1) : $file;
}

function qualified(array $frame): string
{
    return isset($frame['class']) ? $frame['class'] . $frame['type'] . $frame['function'] : ($frame['function'] ?? '');
}

/** Bugsnag PHP's stacktrace: the throw location first, each place named by the function that encloses it. */
function bugsnagFrames(Throwable $t, string $appRoot): array
{
    $trace = $t->getTrace();
    $frames = [];
    $file = $t->getFile();
    $line = $t->getLine();
    foreach ($trace as $i => $f) {
        $frames[] = ['file' => rel($file, $appRoot), 'lineNumber' => $line, 'method' => str_replace('->', '::', qualified($f)), 'inProject' => str_starts_with($file, $appRoot . '/')];
        $file = $f['file'] ?? '[internal]';
        $line = $f['line'] ?? 0;
    }
    $frames[] = ['file' => rel($file, $appRoot), 'lineNumber' => $line, 'method' => '[main]', 'inProject' => str_starts_with($file, $appRoot . '/')];
    return $frames;
}

/** The OpenTelemetry PHP SDK's exception.stacktrace: Java-style "at Class.function(file:line)" lines and "Caused by:" sections, emulated. */
function otelStack(Throwable $t, string $appRoot): string
{
    $out = '';
    for ($x = $t; $x !== null; $x = $x->getPrevious()) {
        $out .= ($x === $t ? '' : "\nCaused by: ") . get_class($x) . ': ' . $x->getMessage();
        $file = $x->getFile();
        $line = $x->getLine();
        foreach ($x->getTrace() as $f) {
            $fn = isset($f['class']) ? $f['class'] . '.' . $f['function'] : $f['function'];
            $out .= "\n\tat " . $fn . '(' . $file . ':' . $line . ')';
            $file = $f['file'] ?? '[internal]';
            $line = $f['line'] ?? 0;
        }
    }
    return deployed($out, $appRoot, '/app');
}

function attr(string $k, string|int $v): array
{
    return ['key' => $k, 'value' => is_int($v) ? ['intValue' => (string) $v] : ['stringValue' => $v]];
}

function record(string $id, Throwable $t, string $appRoot): string
{
    $root = rootCause($t);
    $name = '';
    foreach ($root->getTrace() as $f) {
        if (isUserland($f)) {
            $name = $f['function'];
            $func = qualified($f);
            break;
        }
    }
    $raw = deployed((string) $t, $appRoot, '/app');
    $exceptions = [];
    for ($x = $t; $x !== null; $x = $x->getPrevious()) {
        $exceptions[] = ['errorClass' => get_class($x), 'message' => $x->getMessage(), 'stacktrace' => bugsnagFrames($x, $appRoot)];
    }
    $bugsnag = ['apiKey' => str_repeat('0', 32), 'events' => [[
        'exceptions' => $exceptions,
        'breadcrumbs' => [['name' => 'checkout clicked', 'type' => 'user']],
        'user' => ['id' => 'u-17'],
        'metaData' => ['request' => ['path' => '/checkout']],
    ]]];
    $otlp = ['resourceSpans' => [[
        'resource' => ['attributes' => [attr('service.name', 'shop-php')]],
        'scopeSpans' => [['spans' => [[
            'name' => 'case',
            'attributes' => [attr('code.function', 'record'), attr('code.namespace', 'capture'), attr('code.filepath', '/app/capture/capture.php'), attr('code.lineno', __LINE__)],
            'events' => [['name' => 'exception', 'attributes' => [attr('exception.type', get_class($t)), attr('exception.message', $t->getMessage()), attr('exception.stacktrace', otelStack($t, $appRoot))]]],
        ]]]],
    ]]];
    // A PHP Lambda runtime layer reports getTraceAsString's lines.
    $lambda = ['errorType' => get_class($t), 'errorMessage' => $t->getMessage(), 'stack' => explode("\n", deployed($t->getTraceAsString(), $appRoot, '/var/task'))];
    $newrelic = ['results' => [['events' => [['error.class' => get_class($t), 'error.message' => $t->getMessage(), 'error.stack_trace' => $raw, 'appName' => 'shop-php']]]]];
    $flags = JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR;
    return json_encode([
        'case' => $id,
        'lang' => 'php',
        'truth' => ['file' => rel($root->getFile(), $appRoot), 'line' => $root->getLine(), 'func' => $func, 'name' => bareName($name)],
        'payloads' => [
            'raw' => $raw,
            'bugsnag' => deployed(json_encode($bugsnag, $flags), $appRoot, '/app'),
            'otlp' => json_encode($otlp, $flags),
            'lambda' => json_encode($lambda, $flags),
            'newrelic' => json_encode($newrelic, $flags),
        ],
    ], $flags);
}

foreach ($cases as $id => $fn) {
    try {
        $fn();
    } catch (Throwable $t) {
        echo record($id, $t, $appRoot), "\n";
        continue;
    }
    fwrite(STDERR, "case $id did not throw\n");
    exit(1);
}
