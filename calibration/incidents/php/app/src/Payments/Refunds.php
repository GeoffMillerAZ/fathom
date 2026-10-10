<?php

declare(strict_types=1);

namespace App\Payments;

use Psr\Log\LoggerInterface;
use Psr\Log\LogLevel;

/** Refunds against captured payments. */
class Refunds
{
    public function __construct(private LoggerInterface $logger)
    {
    }

    public function request(string $paymentId, int $amount, string $reason): void
    {
        $this->logger->info('refund of {amount} requested on payment {payment}: {reason}', ['amount' => $amount, 'payment' => $paymentId, 'reason' => $reason]);
    }

    public function reject(string $paymentId, int $limit): void
    {
        $this->logger->warning(sprintf('refund on payment %s rejected: over the %d cent limit', $paymentId, $limit));
    }

    public function settle(string $paymentId, string $batch): void
    {
        $this->logger->debug('refund {payment} settled late in batch {batch}', ['payment' => $paymentId, 'batch' => $batch]);
    }

    public function flag(string $paymentId): void
    {
        $message = 'manual review needed for refund ' . $paymentId;
        $this->logger->warning($message);
    }

    public function credit(string $refundId, int $amount): void
    {
        $this->logger->log(LogLevel::INFO, 'refund {refund} issued as store credit of {amount}', ['refund' => $refundId, 'amount' => $amount]);
    }
}
