<?php

declare(strict_types=1);

namespace App\Orders;

/** An order and its lines. */
class Order
{
    /** @var list<int> */
    private array $quantities = [];

    public function __construct(private int $stock)
    {
    }

    public function addLine(int $qty): void
    {
        if ($qty <= 0) {
            throw new \InvalidArgumentException('quantity must be positive');
        }
        $this->quantities[] = $qty;
    }

    public function reserve(int $qty): void
    {
        if ($qty > $this->stock) {
            throw new \UnderflowException('not enough stock');
        }
        $this->stock -= $qty;
    }

    /** @return list<int> */
    public function summarize(int $divisor): array
    {
        return array_map(function (int $q) use ($divisor): int {
            return intdiv($q, $divisor);
        }, $this->quantities);
    }

    public function deep(int $n): int
    {
        return $this->level1($n);
    }

    private function level1(int $n): int
    {
        return $this->level2($n) + 1;
    }

    private function level2(int $n): int
    {
        return $this->level3($n) + 1;
    }

    private function level3(int $n): int
    {
        $this->reserve($n);
        return $this->stock;
    }
}
