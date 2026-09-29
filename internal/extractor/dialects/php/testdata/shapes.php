<?php
declare(strict_types=1);

namespace App\Models;

use Countable;

interface Shape extends Countable
{
    const SIDES = 0;

    public function area(): float;

    public static function unit(): static;
}

trait Loggable
{
    protected ?string $logger = null;

    public function log(string $message): void
    {
        $this->logger?->info($message);
    }
}

abstract class Base implements Shape
{
    use Loggable;

    public const KIND = 'base';

    public function __construct(
        protected readonly string $name,
        private int $weight = 0,
    ) {
    }

    abstract protected function scale(float $factor): static;

    public static function make(string $name): static
    {
        return new static($name);
    }

    public function describe(
        string $prefix,
        int $width = 10,
        ?array $options = null
    ): string {
        return $prefix . $this->name;
    }
}

final class Circle extends Base
{
    public function area(): float
    {
        return M_PI * 2;
    }

    public function count(): int
    {
        return 1;
    }
}

enum Suit: string
{
    case Hearts = 'H';
    case Spades = 'S';

    const Wild = self::Spades;

    public function label(): string
    {
        return ucfirst($this->name);
    }

    public static function fromChar(string $c): self
    {
        return self::from($c);
    }
}

enum Plain
{
    case A;
    case B;
}

function helper(int $a, int $b = 2): int
{
    return $a + $b;
}

if (!function_exists('App\Models\guarded')) {
    function guarded(): bool
    {
        return true;
    }
}

$handler = function (int $x) {
    return $x * 2;
};
$double = fn(int $x): int => $x * 2;
$anon = new class {
    public function inner(): void
    {
    }
};
