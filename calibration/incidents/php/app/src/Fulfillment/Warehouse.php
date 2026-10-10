<?php

declare(strict_types=1);

namespace App\Fulfillment;

use Psr\Log\LoggerInterface;

/** Warehouse picking and stock checks. */
class Warehouse
{
    public function __construct(private LoggerInterface $log, private string $site)
    {
    }

    public function pick(string $sku, string $bin): string
    {
        $this->log->info('picked {sku} from bin {bin} at {site}', ['sku' => $sku, 'bin' => $bin, 'site' => $this->site]);
        return $sku;
    }

    public function shortPick(string $sku, int $wanted, int $found): void
    {
        $this->log->warning('short pick on {sku}: wanted {wanted}, found {found} in the bin', ['sku' => $sku, 'wanted' => $wanted, 'found' => $found]);
    }

    public function restock(string $sku, int $qty): void
    {
        $this->log->info(
            'restocked {sku} with {qty} units after a cycle count',
            [
                'sku' => $sku,
                'qty' => $qty,
            ]
        );
    }

    public function quarantine(string $lot): void
    {
        $this->log->log('warning', 'lot {lot} quarantined at the receiving dock', ['lot' => $lot]);
    }

    public function recount(string $sku): void
    {
        $this->log->debug("recount scheduled for $sku at site {$this->site}");
    }

    public function cycleCountFailed(string $sku, int $attempts): void
    {
        $this->log->critical('cycle count failed for {sku} after {attempts} attempts', ['sku' => $sku, 'attempts' => $attempts]);
    }
}
