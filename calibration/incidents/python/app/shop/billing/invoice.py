"""An invoice: totals, tax and refunds."""

from shop.billing import util

TAX_TABLES = {"EU": 20}


class Invoice:
    def __init__(self, lines, currency):
        if len(currency) != 3:
            raise ValueError("currency must be an ISO code")
        self.lines = lines
        self.currency = currency

    def total(self, parts):
        return util.round_cents(sum(self.lines), parts)

    def apply_tax(self, region):
        rate = TAX_TABLES.get(region)
        return self.total(1) * rate // 100

    def parse_and_add(self, raw):
        return self.total(1) + util.parse_amount(raw)

    @staticmethod
    def refund(invoice, amount):
        paid = invoice.total(1)
        if amount > paid:
            raise ArithmeticError("refund exceeds the amount paid")
        return paid - amount
