package com.acme.shop.orders;

import java.util.Map;

/** A cart: coupons and checkout. */
public class Cart {
    private final Order order;
    private final Map<String, Integer> coupons;

    public Cart(Order order, Map<String, Integer> coupons) {
        this.order = order;
        this.coupons = coupons;
    }

    public int applyCoupon(String code) {
        Integer off = coupons.get(code);
        return 100 - off;
    }

    public void checkout(int qty) {
        try {
            order.reserve(qty);
        } catch (IllegalStateException e) {
            throw new RuntimeException("checkout failed", e);
        }
    }
}
