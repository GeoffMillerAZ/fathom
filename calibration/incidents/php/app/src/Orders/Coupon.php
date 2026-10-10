<?php

declare(strict_types=1);

namespace App\Orders;

/** A coupon takes a percentage off. */
final class Coupon
{
    public function __construct(private int $off)
    {
    }

    public function off(): int
    {
        return $this->off;
    }
}
