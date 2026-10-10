'use strict';

// A cart: coupons and checkout.
class Cart {
  constructor(order, coupons) {
    this.order = order;
    this.coupons = coupons;
  }

  applyCoupon(code) {
    const coupon = this.coupons[code];
    return 100 - coupon.off;
  }

  checkout(qty) {
    this.order.reserve(qty);
  }
}

module.exports = { Cart };
