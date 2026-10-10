"""An order and its lines."""


class OutOfStock(Exception):
    pass


class Order:
    def __init__(self, stock):
        self.quantities = []
        self.stock = stock

    def add_line(self, qty):
        if qty <= 0:
            raise ValueError("quantity must be positive")
        self.quantities.append(qty)

    def reserve(self, qty):
        if qty > self.stock:
            raise OutOfStock("not enough stock")
        self.stock -= qty

    def summarize(self, divisor):
        def line_share(q):
            return q // divisor

        return [line_share(q) for q in self.quantities]

    def deep(self, n):
        return self._level1(n)

    def _level1(self, n):
        return self._level2(n) + 1

    def _level2(self, n):
        return self._level3(n) + 1

    def _level3(self, n):
        self.reserve(n)
        return self.stock
