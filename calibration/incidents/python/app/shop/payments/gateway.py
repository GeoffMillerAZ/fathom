"""The card gateway client: authorizations, captures and declines."""

import logging

logger = logging.getLogger(__name__)


class Gateway:
    def __init__(self, merchant):
        self.merchant = merchant

    def authorize(self, card, amount):
        logger.info("authorizing %s cents for merchant %s on card ending %s", amount, self.merchant, card[-4:])
        if amount <= 0:
            logger.warning("refusing authorization of %d cents for merchant %s", amount, self.merchant)
            return False
        return True

    def capture(self, auth_id, amount):
        logger.info("captured %(amount)s cents for authorization %(auth)s", {"amount": amount, "auth": auth_id})
        return auth_id

    def decline(self, auth_id, code):
        logger.error("authorization %s declined by issuer: code %s", auth_id, code)

    def timeout(self, auth_id, seconds):
        logger.error(f"gateway timed out after {seconds}s waiting on authorization {auth_id}")

    def void(self, auth_id):
        try:
            raise ConnectionResetError("connection reset by peer")
        except ConnectionResetError:
            logger.exception("void of authorization %s failed", auth_id)
