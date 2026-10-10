"""Refunds against captured payments."""

import logging

log = logging.getLogger("shop.payments.refunds")


def request_refund(payment_id, amount, reason):
    log.info("refund of %s requested on payment %s: %s", amount, payment_id, reason)
    return {"payment": payment_id, "amount": amount}


def reject_refund(payment_id, limit):
    log.warning("refund on payment %s rejected: over the %d cent limit" % (payment_id, limit))


def settle_refund(payment_id, batch):
    log.log(logging.WARNING, "refund %s settled late in batch %s", payment_id, batch)


def reverse_refund(payment_id, operator):
    log.error("refund " + payment_id + " reversed by operator " + operator)


def flag_refund(payment_id):
    message = "manual review needed for refund " + payment_id
    log.warning(message)
