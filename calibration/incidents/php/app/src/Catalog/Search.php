<?php

declare(strict_types=1);

namespace App\Catalog;

/** Catalog search: a category walk and a ranking. */
class Search
{
    /** @param array<string, string> $parents */
    public function __construct(private array $parents)
    {
    }

    public function depth(string $category): int
    {
        return $this->walk($category, 0);
    }

    private function walk(string $category, int $level): int
    {
        if ($level > 3) {
            throw new \RuntimeException('category cycle');
        }
        $parent = $this->parents[$category] ?? null;
        return $parent === null ? $level : $this->walk($parent, $level + 1);
    }

    /** @param list<string> $skus */
    public function rank(array $skus, string $sku): int
    {
        $at = array_search($sku, $skus, true);
        if ($at === false) {
            throw new \OutOfBoundsException('sku not ranked');
        }
        return strlen($skus[$at]);
    }
}
