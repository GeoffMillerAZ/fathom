<?php

declare(strict_types=1);

namespace App\Shipping;

/** A carrier formats a tracking number. */
final class Carrier
{
    public function __construct(private string $code)
    {
    }

    public function format(int $grams): string
    {
        return strtoupper($this->code) . '-' . $grams;
    }
}
