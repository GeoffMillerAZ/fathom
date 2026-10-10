package com.acme.shop.orders;

import java.util.ArrayList;
import java.util.List;

/** An order and its lines. */
public class Order {
    private final List<Integer> quantities = new ArrayList<>();
    private int stock;

    public Order(int stock) {
        this.stock = stock;
    }

    public void addLine(int qty) {
        if (qty <= 0) {
            throw new IllegalArgumentException("quantity must be positive");
        }
        quantities.add(qty);
    }

    public void reserve(int qty) {
        if (qty > stock) {
            throw new IllegalStateException("not enough stock");
        }
        stock -= qty;
    }

    public List<Integer> summarize(int divisor) {
        List<Integer> out = new ArrayList<>();
        quantities.forEach(q -> out.add(q / divisor));
        return out;
    }

    public int deep(int n) {
        return level1(n);
    }

    private int level1(int n) {
        return level2(n) + 1;
    }

    private int level2(int n) {
        return level3(n) + 1;
    }

    private int level3(int n) {
        reserve(n);
        return stock;
    }
}
