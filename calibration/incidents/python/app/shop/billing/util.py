"""Money helpers for billing."""

import json


def round_cents(cents, parts):
    share = cents // parts
    return share * parts


def parse_amount(raw):
    doc = json.loads(raw.strip())
    return doc["cents"]
