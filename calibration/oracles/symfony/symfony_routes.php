<?php
// symfony_routes.php: the Symfony route oracle (tier A). Boots the subject's
// own kernel and reads the router's compiled RouteCollection, the collection
// `bin/console debug:router --format=json` prints (every #[Route] attribute,
// every config/routes import with its prefix, every bundle import), and
// prints the routes oracle shape (calibration/oracles/routes/README.md).
//
// Usage: php symfony_routes.php <app-root> <subject> <sha> <licence> [<env>]
//
// Symfony's, not ours: every path (prefixes already applied), every method
// set, every _controller. Ours: the scope filter and the spelling. A route is
// the repository's when its controller class lives outside vendor/, or when
// the repository's own config/routes.yaml or config/routes/*.yaml declares it
// by name (a TemplateController route); a bundle's imported routes and a
// generated route (the security logout route) are a package's. No methods is
// ANY; a controller is ShortClass.method.
error_reporting(E_ALL & ~E_DEPRECATED & ~E_USER_DEPRECATED);
[$self, $root, $subject, $sha, $licence, $env] = $argv + [null, null, '', '', '', 'prod'];
$root = realpath($root);
if ($root === false) { fwrite(STDERR, "usage: symfony_routes.php <app-root> <subject> <sha> <licence> [<env>]\n"); exit(2); }
chdir($root);
require $root . '/vendor/autoload.php';
if (class_exists(Symfony\Component\Dotenv\Dotenv::class) && is_file($root . '/.env')) {
    (new Symfony\Component\Dotenv\Dotenv())->bootEnv($root . '/.env');
}
$kernelClass = 'App\\Kernel';
$kernel = new $kernelClass($env, false);
$kernel->boot();
$router = $kernel->getContainer()->get('router');

// Route names the repository's YAML route files declare themselves.
$declared = [];
$files = array_merge(glob($root . '/config/routes.yaml') ?: [], glob($root . '/config/routes/*.yaml') ?: []);
foreach ($files as $f) {
    $doc = Symfony\Component\Yaml\Yaml::parseFile($f) ?? [];
    foreach ($doc as $k => $v) {
        if (str_starts_with((string)$k, 'when@')) {
            if ($k === 'when@' . $env && is_array($v)) foreach ($v as $kk => $vv) if (is_array($vv) && !isset($vv['resource'])) $declared[$kk] = true;
            continue;
        }
        if (is_array($v) && !isset($v['resource'])) $declared[$k] = true;
    }
}
$rel = function (?string $file) use ($root): ?string {
    if ($file === null || $file === '' || !str_starts_with($file, $root . '/')) return null;
    return substr($file, strlen($root) + 1);
};
$out = [];
$dropped = ['vendor' => 0, 'generated' => 0];
foreach ($router->getRouteCollection()->all() as $name => $route) {
    $ctl = $route->getDefault('_controller');
    if (is_string($ctl) && str_contains($ctl, '::')) {
        [$class, $method] = explode('::', $ctl, 2);
    } elseif (is_string($ctl) && class_exists($ctl)) {
        [$class, $method] = [$ctl, '__invoke'];
    } else {
        $class = null;
    }
    if ($class === null) { $dropped['generated']++; continue; }
    $handler = substr(strrchr('\\' . $class, '\\'), 1) . '.' . $method;
    try {
        $file = $rel((new ReflectionMethod($class, $method))->getFileName());
    } catch (Throwable $e) { $file = null; }
    $inRepo = $file !== null && !str_starts_with($file, 'vendor/');
    if (!$inRepo && !isset($declared[$name])) { $dropped['vendor']++; continue; }
    $methods = $route->getMethods();
    sort($methods);
    if (!$methods) $methods = ['ANY'];
    elseif (in_array('GET', $methods, true)) $methods = array_values(array_diff($methods, ['HEAD']));
    foreach ($methods as $m) {
        $out[] = ['method' => $m, 'path' => $route->getPath(), 'handler' => $handler];
    }
}
usort($out, fn($a, $b) => [$a['path'], $a['method'], $a['handler']] <=> [$b['path'], $b['method'], $b['handler']]);
$out = array_values(array_unique($out, SORT_REGULAR));
fwrite(STDERR, "symfony_routes: " . count($out) . " routes; dropped as a package's: {$dropped['vendor']}, generated: {$dropped['generated']}\n");
echo json_encode([
    'tool' => 'symfony-router',
    'tool_version' => Symfony\Component\HttpKernel\Kernel::VERSION . ' (env ' . $env . ')',
    'subject' => $subject,
    'sha' => $sha,
    'licence' => $licence,
    'routes' => $out,
], JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . "\n";
