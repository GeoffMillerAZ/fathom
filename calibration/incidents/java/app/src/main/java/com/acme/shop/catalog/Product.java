package com.acme.shop.catalog;

/** A catalog product. */
public class Product {
    private final String sku;
    private final String price;

    public Product(String sku, String price) {
        this.sku = sku;
        this.price = price;
    }

    public void validate() {
        if (!sku.matches("[A-Z]{3}-\\d+")) {
            throw new IllegalArgumentException("bad sku");
        }
    }

    public double price() {
        return Double.parseDouble(price.replace(",", "."));
    }

    public String sku() {
        return sku;
    }
}
