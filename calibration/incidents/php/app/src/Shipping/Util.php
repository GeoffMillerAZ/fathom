<?php

declare(strict_types=1);

namespace App\Shipping;

/** Weight helpers for shipping: the same basename as billing's Util. */
final class Util
{
    public static function round(int $grams, int $step): int
    {
        return intdiv($grams, $step) * $step;
    }

    /** @return list<int> */
    public static function bands(): array
    {
        return [500, 1000, 2000];
    }
}
