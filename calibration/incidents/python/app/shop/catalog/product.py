"""A catalog product."""

import re

SKU = re.compile(r"^[A-Z]{3}-\d+$")


class Product:
    def __init__(self, sku, price):
        self.sku = sku
        self.price_text = price

    def validate(self):
        if not SKU.match(self.sku):
            raise ValueError("bad sku")

    def price(self):
        return float(self.price_text.replace(",", "."))
