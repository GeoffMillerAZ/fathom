package com.acme.shop.payments;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Refunds against captured payments. */
public final class Refunds {
    private static final Logger LOG = LoggerFactory.getLogger("com.acme.shop.payments.refunds");

    private Refunds() {
    }

    public static void request(String paymentId, long amount, String reason) {
        LOG.info("refund of {} requested on payment {}: {}", amount, paymentId, reason);
    }

    public static void reject(String paymentId, long limit) {
        LOG.warn("refund on payment {} rejected: over the {} cent limit", paymentId, limit);
    }

    public static void settle(String paymentId, String batch) {
        LOG.debug("refund {} settled late in batch {}", paymentId, batch);
    }

    public static void flag(String paymentId) {
        String message = "manual review needed for refund " + paymentId;
        LOG.warn(message);
    }

    public static void keepPlaceholder(String paymentId) {
        LOG.info("refund {} kept its \\{} placeholder in the memo", paymentId);
    }
}
