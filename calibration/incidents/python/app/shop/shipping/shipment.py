"""A shipment: weight, validation and tracking."""

from shop.shipping import util


class Shipment:
    def __init__(self, grams, country):
        self.grams = grams
        self.country = country

    def packed(self, step):
        return util.round_cents(self.grams, step)

    def band(self):
        bands = util.bands()
        i = self.grams // 1000
        return bands[i + 2]

    def validate(self):
        if not self.country:
            raise ValueError("shipment has no destination")

    def track(self, carriers):
        carrier = carriers.get(self.country)
        return carrier.upper() + "-" + str(self.grams)
