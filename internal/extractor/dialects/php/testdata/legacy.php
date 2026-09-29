<!DOCTYPE html>
<html>
<?php
if (!function_exists('legacy_helper')) {
    /** A guarded global helper, the shape of a library's functions.php. */
    function legacy_helper(array &$items, ...$rest)
    {
        return count($items);
    }
}

function &ref_returning()
{
    static $cache = 1;
    return $cache;
}

function gen(): Generator
{
    yield 1;
}
?>
<p><?= legacy_helper($items) ?></p>
<?php
#[Attribute]
final class Marker
{
}

#[Route('/x'), Deprecated]
function attributed(): void
{
}

abstract class Model
{
    #[Pure]
    public function id(): int
    {
        return 1;
    }

    public function __call($name, $args)
    {
        return call_user_func_array([$this, $name], $args);
    }

    public static function __callStatic($name, $args)
    {
    }
}
