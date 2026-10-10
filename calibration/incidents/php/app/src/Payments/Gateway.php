<?php

declare(strict_types=1);

namespace App\Payments;

use Psr\Log\LoggerInterface;

/** The card gateway client: authorizations, captures and declines. */
class Gateway
{
    public function __construct(private LoggerInterface $logger, private string $merchant)
    {
    }

    public function authorize(int $amount, string $card): bool
    {
        $this->logger->info('authorizing {amount} cents for merchant {merchant} on card ending {last4}', [
            'amount' => $amount,
            'merchant' => $this->merchant,
            'last4' => substr($card, -4),
        ]);
        if ($amount <= 0) {
            $this->logger->warning('refusing authorization of {amount} cents for merchant {merchant}', ['amount' => $amount, 'merchant' => $this->merchant]);
            return false;
        }
        return true;
    }

    public function decline(string $authId, string $code): void
    {
        $this->logger->error(sprintf('authorization %s declined by issuer: code %s', $authId, $code));
    }

    public function timeout(string $authId, int $seconds): void
    {
        $this->logger->error("gateway timed out after {$seconds}s waiting on authorization $authId");
    }

    public function voidAuthorization(string $authId): void
    {
        try {
            throw new \RuntimeException('connection reset by peer');
        } catch (\RuntimeException $e) {
            $this->logger->error('void of authorization {auth} failed', ['auth' => $authId, 'exception' => $e]);
        }
    }

    public function capture(string $authId, int $amount): string
    {
        $this->logger->info('captured ' . $amount . ' cents for authorization ' . $authId);
        return $authId;
    }

    public function chargeback(string $paymentId, string $reason): void
    {
        $this->logger->notice('chargeback opened on payment {payment}: {reason}', ['payment' => $paymentId, 'reason' => $reason]);
    }
}
