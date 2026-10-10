<?php

declare(strict_types=1);

namespace App\Shipping;

/** A shipment: weight, validation and tracking. */
class Shipment
{
    public function __construct(private int $grams, private string $country)
    {
    }

    public function packed(int $step): int
    {
        return Util::round($this->grams, $step);
    }

    public function band(): int
    {
        $bands = Util::bands();
        $i = intdiv($this->grams, 1000);
        return $bands[$i + 2] ?? throw new \OutOfRangeException('no band for this weight');
    }

    public function validate(): void
    {
        if ($this->country === '') {
            throw new \DomainException('shipment has no destination');
        }
    }

    /** @param array<string, Carrier> $carriers */
    public function track(array $carriers): string
    {
        $carrier = $carriers[$this->country] ?? null;
        return $carrier->format($this->grams);
    }
}
