package com.acme.shop.shipping;

/** Weight helpers for shipping: the same basename as billing's Util. */
public final class Util {
    private Util() {
    }

    public static long round(long grams, int step) {
        long boxes = grams / step;
        return boxes * step;
    }

    public static int[] bands() {
        return new int[] {500, 1000, 2000};
    }
}
