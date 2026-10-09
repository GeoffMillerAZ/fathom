<?php

declare(strict_types=1);

namespace App\Billing;

/** Money helpers for billing. */
final class Util
{
    public static function round(int $cents, int $parts): int
    {
        return intdiv($cents, $parts) * $parts;
    }

    public static function parseAmount(string $raw): int
    {
        $doc = json_decode(trim($raw), true, 8, JSON_THROW_ON_ERROR);
        return (int) $doc['cents'];
    }
}
