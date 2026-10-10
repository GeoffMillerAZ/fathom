package com.acme.shop.billing;

import java.util.List;

/** An invoice: totals, tax and refunds. Fault-injected for TL-7. */
public class Invoice {
    private final List<Long> lines;
    private final String currency;

    public Invoice(List<Long> lines, String currency) {
        if (currency.length() != 3) {
            throw new IllegalArgumentException("currency must be an ISO code");
        }
        this.lines = lines;
        this.currency = currency;
    }

    public long total(int parts) {
        long sum = 0;
        for (long l : lines) {
            sum += l;
        }
        return Util.round(sum, parts);
    }

    public long applyTax(String region) {
        TaxTable table = TaxTable.lookup(region);
        return total(1) * table.rate() / 100;
    }

    public static long refund(Invoice inv, long amount) {
        long paid = inv.total(1);
        if (amount > paid) {
            throw new IllegalStateException("refund exceeds the amount paid");
        }
        return paid - amount;
    }

    public long parseAndAdd(String raw) {
        return total(1) + Util.parseAmount(raw);
    }

    public String currency() {
        return currency;
    }

    /** A tax table; an unknown region has none. */
    static final class TaxTable {
        private final int rate;

        private TaxTable(int rate) {
            this.rate = rate;
        }

        static TaxTable lookup(String region) {
            return "EU".equals(region) ? new TaxTable(20) : null;
        }

        int rate() {
            return rate;
        }
    }
}
