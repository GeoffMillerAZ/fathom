package com.acme.shop.fulfillment;

import org.apache.logging.log4j.Level;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

/** Warehouse picking and stock checks (the log4j2 API). */
public class Warehouse {
    private static final Logger LOGGER = LogManager.getLogger(Warehouse.class);

    private final String site;

    public Warehouse(String site) {
        this.site = site;
    }

    public String pick(String sku, String bin) {
        LOGGER.info("picked {} from bin {} at {}", sku, bin, site);
        return sku;
    }

    public void shortPick(String sku, int wanted, int found) {
        LOGGER.warn("short pick on {}: wanted {}, found {} in the bin", sku, wanted, found);
    }

    public void restock(String sku, int qty) {
        LOGGER.info(
                "restocked {} with {} units after a cycle count",
                sku,
                qty);
    }

    public void quarantine(String lot) {
        LOGGER.log(Level.WARN, "lot {} quarantined at the receiving dock", lot);
    }

    public void recount(String sku) {
        LOGGER.debug("recount scheduled for {} at site {}", sku, site);
    }

    public void cycleCountFailed(String sku, int attempts) {
        LOGGER.error("cycle count failed for {} after {} attempts", sku, attempts);
    }
}
