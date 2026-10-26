<?php
// Native compiler/runtime source ranges, kept in private workspace receipts.
declare(strict_types=1);
register_shutdown_function(static function (): void {
    register_shutdown_function(static function (): void {
        $root = realpath((string) getenv('PHP_ORACLE_SUBJECT'));
        $out = (string) getenv('PHP_ORACLE_META_DIR');
        $functions = [];
        $add = static function (ReflectionFunctionAbstract $function, string $identity) use ($root, &$functions): void {
            $file = $function->getFileName();
            if (!$file || !str_starts_with($file, $root.DIRECTORY_SEPARATOR)) {
                return;
            }
            $key = $identity.'|'.$file.'|'.$function->getStartLine();
            $functions[$key] = ['file' => $file, 'identity' => $identity,
                'name' => $function->getName(), 'start' => $function->getStartLine(),
                'end' => $function->getEndLine()];
        };
        foreach (get_defined_functions()['user'] as $name) {
            $add(new ReflectionFunction($name), $name);
        }
        foreach (get_declared_classes() as $name) {
            foreach ((new ReflectionClass($name))->getMethods() as $method) {
                $add($method, $method->getDeclaringClass()->getName().'::'.$method->getName());
            }
        }
        file_put_contents($out.'/reflection-'.getmypid().'.json', json_encode([
            'php' => PHP_VERSION, 'xdebug' => phpversion('xdebug'), 'functions' => array_values($functions),
        ], JSON_THROW_ON_ERROR));
    });
});
