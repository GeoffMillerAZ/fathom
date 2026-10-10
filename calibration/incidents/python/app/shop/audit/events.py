"""Audit events (structlog, JSON and key-value rendering)."""

import structlog

audit = structlog.get_logger("shop.audit")


def login_failed(user_id, attempts):
    audit.warning("login_failed", user_id=user_id, attempts=attempts)


def role_granted(user_id, role, granted_by):
    audit.info("role_granted", user_id=user_id, role=role, granted_by=granted_by)


def export_denied(user_id, dataset):
    audit.error("export_denied", user_id=user_id, dataset=dataset)


def password_reset(user_id):
    audit.info("password_reset_requested", user_id=user_id)
