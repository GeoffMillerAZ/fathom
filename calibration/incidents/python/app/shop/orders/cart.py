"""A cart: coupons and checkout."""

from shop.orders.order import OutOfStock


class CheckoutFailed(Exception):
    pass


class Cart:
    def __init__(self, order, coupons):
        self.order = order
        self.coupons = coupons

    def apply_coupon(self, code):
        coupon = self.coupons.get(code)
        return 100 - coupon["off"]

    def checkout(self, qty):
        try:
            self.order.reserve(qty)
        except OutOfStock as e:
            raise CheckoutFailed("checkout failed") from e
