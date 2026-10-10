package com.acme.shop.capture;

import com.acme.shop.accounts.Auth;
import com.acme.shop.accounts.User;
import com.acme.shop.billing.Invoice;
import com.acme.shop.catalog.Product;
import com.acme.shop.catalog.Search;
import com.acme.shop.orders.Cart;
import com.acme.shop.orders.Order;
import com.acme.shop.shipping.Shipment;
import java.io.PrintWriter;
import java.io.StringWriter;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * TL-7's capture runner (docs/plans/telemetry-links.md): runs every
 * fault-injected case, reads the truth from the runtime's own structured
 * trace (the root cause's innermost frame inside the app), and writes one
 * JSON line per case carrying the raw trace and the payload shapes a
 * notifier, a tracer, the Lambda runtime and an APM agent would send.
 */
public final class Capture {
    private Capture() {
    }

    interface Fault {
        void run() throws Exception;
    }

    public static void main(String[] args) {
        Map<String, Fault> cases = new LinkedHashMap<>();
        cases.put("billing-total-div0", () -> new Invoice(List.of(100L, 200L), "USD").total(0));
        cases.put("billing-tax-null", () -> new Invoice(List.of(100L), "USD").applyTax("US"));
        cases.put("billing-parse-amount", () -> new Invoice(List.of(1L), "USD").parseAndAdd("12a"));
        cases.put("billing-ctor-currency", () -> new Invoice(List.of(), "EURO"));
        cases.put("billing-refund-static", () -> Invoice.refund(new Invoice(List.of(10L), "USD"), 9999));
        cases.put("shipping-round-div0", () -> new Shipment(1200, "DE").packed(0));
        cases.put("shipping-band-bounds", () -> new Shipment(5000, "DE").band());
        cases.put("shipping-validate", () -> new Shipment(10, "").validate());
        cases.put("shipping-track-null", () -> new Shipment(10, "FR").track(Map.of()));
        cases.put("catalog-validate", () -> new Product("bad", "1").validate());
        cases.put("catalog-walk-cycle", () -> new Search(Map.of("a", "b", "b", "a")).depth("a"));
        cases.put("catalog-rank-missing", () -> new Search(Map.of()).rank(List.of("A"), "Z"));
        cases.put("catalog-price-parse", () -> new Product("ABC-1", "x").price());
        cases.put("orders-add-line", () -> new Order(5).addLine(0));
        cases.put("orders-coupon-null", () -> new Cart(new Order(5), Map.of()).applyCoupon("NOPE"));
        cases.put("orders-checkout-wrapped", () -> new Cart(new Order(5), Map.of()).checkout(99));
        cases.put("orders-summarize-lambda", () -> {
            Order o = new Order(5);
            o.addLine(3);
            o.summarize(0);
        });
        cases.put("orders-deep-chain", () -> new Order(1).deep(5));
        cases.put("accounts-rename", () -> new User("a", "a@b.c").rename("Single"));
        cases.put("accounts-verify", () -> new Auth("s3cret").verify("token"));
        cases.put("accounts-hash-algorithm", () -> new Auth("s").hashPassword("p", "NOPE-256"));
        cases.put("accounts-domain", () -> new User("a", "nope").domain());

        for (Map.Entry<String, Fault> c : cases.entrySet()) {
            try {
                c.getValue().run();
                System.err.println("case " + c.getKey() + " did not throw");
                System.exit(1);
            } catch (Throwable t) {
                System.out.println(line(c.getKey(), t));
            }
        }
    }

    static Throwable root(Throwable t) {
        while (t.getCause() != null && t.getCause() != t) {
            t = t.getCause();
        }
        return t;
    }

    static boolean inApp(StackTraceElement e) {
        return e.getClassName().startsWith("com.acme.shop.") && !e.getClassName().startsWith("com.acme.shop.capture.");
    }

    static String repoPath(StackTraceElement e) {
        String cls = e.getClassName();
        String pkg = cls.substring(0, cls.lastIndexOf('.'));
        return "src/main/java/" + pkg.replace('.', '/') + "/" + e.getFileName();
    }

    /** The bare name the index gives the injected function. */
    static String bareName(StackTraceElement e) {
        String m = e.getMethodName();
        String cls = e.getClassName().substring(e.getClassName().lastIndexOf('.') + 1);
        if (cls.contains("$")) {
            cls = cls.substring(cls.lastIndexOf('$') + 1);
        }
        if (m.equals("<init>")) {
            return cls;
        }
        if (m.startsWith("lambda$")) {
            return m.split("\\$")[1];
        }
        return m;
    }

    static String line(String id, Throwable t) {
        Throwable r = root(t);
        StackTraceElement truth = null;
        for (StackTraceElement e : r.getStackTrace()) {
            if (inApp(e)) {
                truth = e;
                break;
            }
        }
        StringWriter sw = new StringWriter();
        t.printStackTrace(new PrintWriter(sw));
        String raw = sw.toString();

        Map<String, Object> out = new LinkedHashMap<>();
        out.put("case", id);
        out.put("lang", "java");
        Map<String, Object> tr = new LinkedHashMap<>();
        tr.put("file", repoPath(truth));
        tr.put("line", truth.getLineNumber());
        tr.put("func", truth.getClassName() + "." + truth.getMethodName());
        tr.put("name", bareName(truth));
        out.put("truth", tr);
        Map<String, Object> p = new LinkedHashMap<>();
        p.put("raw", raw);
        p.put("bugsnag", Json.write(bugsnag(t)));
        p.put("otlp", Json.write(otlp(t, raw)));
        p.put("lambda", Json.write(lambda(t)));
        p.put("newrelic", Json.write(newrelic(t, raw)));
        out.put("payloads", p);
        return Json.write(out);
    }

    /** Bugsnag's event shape: the error first, then each cause. */
    static Map<String, Object> bugsnag(Throwable t) {
        List<Object> exceptions = new ArrayList<>();
        for (Throwable x = t; x != null; x = x.getCause()) {
            List<Object> frames = new ArrayList<>();
            for (StackTraceElement e : x.getStackTrace()) {
                Map<String, Object> f = new LinkedHashMap<>();
                f.put("file", e.getFileName());
                f.put("lineNumber", e.getLineNumber());
                f.put("method", e.getClassName() + "." + e.getMethodName());
                f.put("inProject", e.getClassName().startsWith("com.acme.shop."));
                frames.add(f);
            }
            Map<String, Object> ex = new LinkedHashMap<>();
            ex.put("errorClass", x.getClass().getName());
            ex.put("message", String.valueOf(x.getMessage()));
            ex.put("stacktrace", frames);
            exceptions.add(ex);
            if (x.getCause() == x) {
                break;
            }
        }
        Map<String, Object> ev = new LinkedHashMap<>();
        ev.put("exceptions", exceptions);
        ev.put("breadcrumbs", List.of(Map.of("name", "checkout clicked", "type", "user")));
        ev.put("user", Map.of("id", "u-17"));
        ev.put("metaData", Map.of("request", Map.of("path", "/checkout")));
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("apiKey", "00000000000000000000000000000000");
        body.put("events", List.of(ev));
        return body;
    }

    static Map<String, Object> attr(String k, Object v) {
        Map<String, Object> val = new LinkedHashMap<>();
        if (v instanceof Integer) {
            val.put("intValue", String.valueOf(v));
        } else {
            val.put("stringValue", String.valueOf(v));
        }
        Map<String, Object> a = new LinkedHashMap<>();
        a.put("key", k);
        a.put("value", val);
        return a;
    }

    /** An OTLP JSON span with the SDK's exception event; the Java agent sets code.namespace and code.function. */
    static Map<String, Object> otlp(Throwable t, String raw) {
        Map<String, Object> event = new LinkedHashMap<>();
        event.put("name", "exception");
        event.put("attributes", List.of(attr("exception.type", t.getClass().getName()), attr("exception.message", String.valueOf(t.getMessage())), attr("exception.stacktrace", raw)));
        Map<String, Object> span = new LinkedHashMap<>();
        span.put("name", "case");
        span.put("attributes", List.of(attr("code.namespace", "com.acme.shop.capture.Capture"), attr("code.function", "main")));
        span.put("events", List.of(event));
        Map<String, Object> rs = new LinkedHashMap<>();
        rs.put("resource", Map.of("attributes", List.of(attr("service.name", "shop-java"))));
        rs.put("scopeSpans", List.of(Map.of("spans", List.of(span))));
        return Map.of("resourceSpans", List.of(rs));
    }

    /** The Lambda Java runtime's error response, causes nested. */
    static Map<String, Object> lambda(Throwable t) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("errorMessage", String.valueOf(t.getMessage()));
        m.put("errorType", t.getClass().getName());
        List<Object> st = new ArrayList<>();
        for (StackTraceElement e : t.getStackTrace()) {
            st.add(e.getClassName() + "." + e.getMethodName() + "(" + e.getFileName() + ":" + e.getLineNumber() + ")");
        }
        m.put("stackTrace", st);
        if (t.getCause() != null && t.getCause() != t) {
            m.put("cause", lambda(t.getCause()));
        }
        return m;
    }

    /** A New Relic error event (an NRQL result row) carrying error.stack_trace. */
    static Map<String, Object> newrelic(Throwable t, String raw) {
        Map<String, Object> ev = new LinkedHashMap<>();
        ev.put("error.class", t.getClass().getName());
        ev.put("error.message", String.valueOf(t.getMessage()));
        ev.put("error.stack_trace", raw);
        ev.put("appName", "shop-java");
        return Map.of("results", List.of(Map.of("events", List.of(ev))));
    }

    /** A minimal JSON writer: maps, lists, strings, numbers, booleans. */
    static final class Json {
        private Json() {
        }

        static String write(Object o) {
            StringBuilder b = new StringBuilder();
            write(b, o);
            return b.toString();
        }

        static void write(StringBuilder b, Object o) {
            if (o instanceof Map<?, ?> m) {
                b.append('{');
                boolean first = true;
                for (Map.Entry<?, ?> e : m.entrySet()) {
                    if (!first) {
                        b.append(',');
                    }
                    first = false;
                    str(b, String.valueOf(e.getKey()));
                    b.append(':');
                    write(b, e.getValue());
                }
                b.append('}');
            } else if (o instanceof List<?> l) {
                b.append('[');
                for (int i = 0; i < l.size(); i++) {
                    if (i > 0) {
                        b.append(',');
                    }
                    write(b, l.get(i));
                }
                b.append(']');
            } else if (o instanceof Number || o instanceof Boolean) {
                b.append(o);
            } else {
                str(b, String.valueOf(o));
            }
        }

        static void str(StringBuilder b, String s) {
            b.append('"');
            for (char c : s.toCharArray()) {
                switch (c) {
                    case '"' -> b.append("\\\"");
                    case '\\' -> b.append("\\\\");
                    case '\n' -> b.append("\\n");
                    case '\r' -> b.append("\\r");
                    case '\t' -> b.append("\\t");
                    default -> {
                        if (c < 0x20) {
                            b.append(String.format("\\u%04x", (int) c));
                        } else {
                            b.append(c);
                        }
                    }
                }
            }
            b.append('"');
        }
    }
}
