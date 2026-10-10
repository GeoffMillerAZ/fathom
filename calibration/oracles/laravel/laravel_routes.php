<?php
// laravel_routes.php: the Laravel route oracle (tier A). Boots the subject's own
// application exactly as `php artisan route:list` does (the console kernel's
// bootstrap loads every route file through the app's RouteServiceProvider or
// bootstrap/app.php withRouting) and prints the router's route collection as
// the routes oracle shape (calibration/oracles/routes/README.md).
//
// Usage: php -d display_errors=stderr laravel_routes.php <app-root> <subject> <sha> <licence>
//
// Laravel's, not ours: every method, URI and action. Ours: the scope filter (a
// route whose action lives under vendor/ is a package's, not the repository's)
// and the spelling (HEAD dropped beside GET, the seven verbs of Route::any as
// ANY, an action as ShortClass.method, a closure as "").
error_reporting(E_ALL & ~E_DEPRECATED & ~E_USER_DEPRECATED);
[$self, $root, $subject, $sha, $licence] = $argv + [null, null, '', '', ''];
$root = realpath($root);
if ($root === false) { fwrite(STDERR, "usage: laravel_routes.php <app-root> <subject> <sha> <licence>\n"); exit(2); }
chdir($root);
require $root . '/vendor/autoload.php';
$app = require $root . '/bootstrap/app.php';
$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();

$any = ['DELETE', 'GET', 'HEAD', 'OPTIONS', 'PATCH', 'POST', 'PUT'];
$rel = function (?string $file) use ($root): ?string {
    if ($file === null || $file === '' || !str_starts_with($file, $root . '/')) return null;
    return substr($file, strlen($root) + 1);
};
$out = [];
$dropped = ['vendor' => 0];
foreach ($app['router']->getRoutes() as $route) {
    $methods = $route->methods();
    sort($methods);
    $uses = $route->getAction('uses');
    $handler = '';
    if ($uses instanceof Closure) {
        $file = $rel((new ReflectionFunction($uses))->getFileName());
        if ($file === null || str_starts_with($file, 'vendor/')) { $dropped['vendor']++; continue; }
    } elseif (is_string($uses)) {
        [$class, $method] = array_pad(explode('@', ltrim($uses, '\\'), 2), 2, '__invoke');
        $short = substr(strrchr('\\' . $class, '\\'), 1);
        $handler = $short . '.' . $method;
        $cf = null;
        try { $cf = $rel((new ReflectionClass($class))->getFileName()); } catch (Throwable $e) { $cf = null; }
        $declaredInRepo = in_array($class, ['Illuminate\\Routing\\ViewController', 'Illuminate\\Routing\\RedirectController'], true);
        if (!$declaredInRepo && ($cf === null || str_starts_with($cf, 'vendor/'))) { $dropped['vendor']++; continue; }
    }
    if ($methods === $any) {
        $ms = ['ANY'];
    } else {
        $ms = in_array('GET', $methods, true) ? array_values(array_diff($methods, ['HEAD'])) : $methods;
    }
    $path = '/' . ltrim($route->uri(), '/');
    foreach ($ms as $m) {
        $out[] = ['method' => $m, 'path' => $path, 'handler' => $handler];
    }
}
usort($out, fn($a, $b) => [$a['path'], $a['method'], $a['handler']] <=> [$b['path'], $b['method'], $b['handler']]);
$out = array_values(array_unique($out, SORT_REGULAR));
fwrite(STDERR, "laravel_routes: " . count($out) . " routes; dropped as a package's: {$dropped['vendor']}\n");
echo json_encode([
    'tool' => 'laravel-router',
    'tool_version' => $app->version(),
    'subject' => $subject,
    'sha' => $sha,
    'licence' => $licence,
    'routes' => $out,
], JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . "\n";
