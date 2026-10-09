<?php

declare(strict_types=1);

namespace App\Billing;

/** A tax table; an unknown region has none. */
final class TaxTable
{
    private function __construct(private int $rate)
    {
    }

    public static function lookup(string $region): ?self
    {
        return $region === 'EU' ? new self(20) : null;
    }

    public function rate(): int
    {
        return $this->rate;
    }
}
