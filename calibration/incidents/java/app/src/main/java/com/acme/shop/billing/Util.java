package com.acme.shop.billing;

/** Money helpers for billing. */
public final class Util {
    private Util() {
    }

    public static long round(long cents, int parts) {
        long share = cents / parts;
        return share * parts;
    }

    public static long parseAmount(String raw) {
        String digits = raw.trim();
        return Long.parseLong(digits);
    }
}
