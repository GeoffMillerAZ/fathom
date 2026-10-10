<?php

declare(strict_types=1);

namespace App\Catalog;

/** A catalog product. */
class Product
{
    public function __construct(private string $sku, private string $price)
    {
    }

    public function validate(): void
    {
        if (preg_match('/^[A-Z]{3}-\d+$/', $this->sku) !== 1) {
            throw new \UnexpectedValueException('bad sku');
        }
    }

    public function price(): int
    {
        return intdiv(10000, (int) $this->price);
    }
}
