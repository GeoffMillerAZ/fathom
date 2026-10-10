"""Warehouse picking and stock checks."""

import logging

logger = logging.getLogger(__name__)


class Warehouse:
    def __init__(self, site):
        self.site = site

    def pick(self, sku, bin_id):
        logger.info("picked {} from bin {} at {}".format(sku, bin_id, self.site))
        return sku

    def short_pick(self, sku, wanted, found):
        logger.warning(
            "short pick on %s: wanted %d, found %d in the bin",
            sku,
            wanted,
            found,
        )

    def restock(self, sku, qty):
        logger.info("restocked %s with %s units "
                    "after a cycle count", sku, qty)

    def quarantine(self, lot):
        logging.warning("lot %s quarantined at the receiving dock", lot)

    def recount(self, sku):
        logger.debug(f"recount scheduled for {sku} at site {self.site}")
