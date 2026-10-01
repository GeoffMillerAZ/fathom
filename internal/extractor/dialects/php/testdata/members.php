<?php
namespace Acme;

interface Sized
{
    const SIDES = 4;
}

class Box
{
    public const KIND = 'box', ALT = 'b';
    private static int $count = 0;
    protected ?string $label = null, $note = null;

    public function __construct(
        private readonly int $width,
        protected int $height = 1,
        $plain = 0,
    ) {
    }

    public function area(): int { return $this->width * $this->height; }
}

trait Tagged
{
    public array $tags = [];
}

enum Suit: string
{
    case Hearts = 'H';
    case Spades = 'S';
    const Wild = self::Spades;

    public function label(): string { return ucfirst($this->name); }
}
