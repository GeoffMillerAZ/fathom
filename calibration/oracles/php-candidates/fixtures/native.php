<?php
namespace NativeProbe;
use function NativeProbe\target as alias_target;
function target(string $value): int {
    return strlen($value);
}
function driver(): void {
    target('x');
    alias_target('x');
}
function dynamic_driver(): void { $call = __NAMESPACE__.'\\target'; if (false) { target('x'); } $call('x'); }
driver();
dynamic_driver();
