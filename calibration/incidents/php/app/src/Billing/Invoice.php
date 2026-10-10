<?php

declare(strict_types=1);

namespace App\Billing;

/** An invoice: totals, tax and refunds. */
class Invoice
{
    /** @param list<int> $lines */
    public function __construct(private array $lines, private string $currency)
    {
        if (strlen($currency) !== 3) {
            throw new \InvalidArgumentException('currency must be an ISO code');
        }
    }

    public function total(int $parts): int
    {
        return Util::round(array_sum($this->lines), $parts);
    }

    public function applyTax(string $region): int
    {
        $table = TaxTable::lookup($region);
        return intdiv($this->total(1) * $table->rate(), 100);
    }

    public function parseAndAdd(string $raw): int
    {
        return $this->total(1) + Util::parseAmount($raw);
    }

    public static function refund(Invoice $invoice, int $amount): int
    {
        $paid = $invoice->total(1);
        if ($amount > $paid) {
            throw new \LogicException('refund exceeds the amount paid');
        }
        return $paid - $amount;
    }
}
