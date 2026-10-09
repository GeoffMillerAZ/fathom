package com.acme.shop.shipping;

import java.util.Map;

/** A shipment: weight, validation and tracking. */
public class Shipment {
    private final long grams;
    private final String country;

    public Shipment(long grams, String country) {
        this.grams = grams;
        this.country = country;
    }

    public long packed(int step) {
        return Util.round(grams, step);
    }

    public int band() {
        int[] bands = Util.bands();
        int i = (int) (grams / 1000);
        return bands[i + 2];
    }

    public void validate() {
        if (country == null || country.isEmpty()) {
            throw new IllegalStateException("shipment has no destination");
        }
    }

    public String track(Map<String, String> carriers) {
        String carrier = carriers.get(country);
        return carrier.toUpperCase() + "-" + grams;
    }
}
