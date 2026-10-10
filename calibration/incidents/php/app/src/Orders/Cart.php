<?php

declare(strict_types=1);

namespace App\Orders;

/** A cart: coupons and checkout. */
class Cart
{
    /** @param array<string, Coupon> $coupons */
    public function __construct(private Order $order, private array $coupons)
    {
    }

    public function applyCoupon(string $code): int
    {
        $coupon = $this->coupons[$code] ?? null;
        return 100 - $coupon->off();
    }

    public function checkout(int $qty): void
    {
        try {
            $this->order->reserve($qty);
        } catch (\UnderflowException $e) {
            throw new \RuntimeException('checkout failed', 0, $e);
        }
    }
}
