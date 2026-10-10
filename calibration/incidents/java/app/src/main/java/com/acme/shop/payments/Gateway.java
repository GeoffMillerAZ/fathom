package com.acme.shop.payments;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.Marker;
import org.slf4j.MarkerFactory;

/** The card gateway client: authorizations, captures and declines. */
public class Gateway {
    private static final Logger log = LoggerFactory.getLogger(Gateway.class);
    private static final Marker AUDIT = MarkerFactory.getMarker("AUDIT");

    private final String merchant;

    public Gateway(String merchant) {
        this.merchant = merchant;
    }

    public boolean authorize(long amount, String card) {
        log.info("authorizing {} cents for merchant {} on card ending {}", amount, merchant, card.substring(card.length() - 4));
        if (amount <= 0) {
            log.warn("refusing authorization of {} cents for merchant {}", amount, merchant);
            return false;
        }
        return true;
    }

    public void decline(String authId, String code) {
        log.error("authorization {} declined by issuer: code {}", authId, code);
    }

    public void timeout(String authId, int seconds) {
        log.error(String.format("gateway timed out after %d s waiting on authorization %s", seconds, authId));
    }

    public void voidAuthorization(String authId) {
        try {
            throw new IllegalStateException("connection reset by peer");
        } catch (IllegalStateException e) {
            log.error("void of authorization {} failed", authId, e);
        }
    }

    public String capture(String authId, long amount) {
        log.info("captured " + amount + " cents for authorization " + authId);
        return authId;
    }

    public void chargeback(String paymentId, String reason) {
        log.warn(AUDIT, "chargeback opened on payment {}: {}", paymentId, reason);
    }
}
