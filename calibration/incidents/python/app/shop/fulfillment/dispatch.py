"""Dispatch: handing parcels to carriers (structlog, console rendering)."""

import structlog

log = structlog.get_logger("shop.fulfillment.dispatch")


def hand_over(parcel_id, carrier):
    log.info("parcel_handed_over", parcel_id=parcel_id, carrier=carrier)


def missed_pickup(carrier, window):
    log.warning("carrier_missed_pickup", carrier=carrier, window=window)


def label_rejected(parcel_id, reason):
    log.error("label_rejected", parcel_id=parcel_id, reason=reason)


def manifest_closed(manifest_id, parcels):
    log.info("manifest %s closed with %d parcels", manifest_id, parcels)
