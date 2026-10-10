package com.acme.shop.capture;

import com.acme.shop.audit.AuditTrail;
import com.acme.shop.fulfillment.Warehouse;
import com.acme.shop.payments.Gateway;
import com.acme.shop.payments.Refunds;
import java.io.Serializable;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.logging.Handler;
import java.util.logging.LogRecord;
import java.util.logging.SimpleFormatter;
import org.apache.logging.log4j.Level;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.core.Appender;
import org.apache.logging.log4j.core.Layout;
import org.apache.logging.log4j.core.LogEvent;
import org.apache.logging.log4j.core.LoggerContext;
import org.apache.logging.log4j.core.appender.AbstractAppender;
import org.apache.logging.log4j.core.config.Configuration;
import org.apache.logging.log4j.core.config.LoggerConfig;
import org.apache.logging.log4j.core.config.Property;
import org.apache.logging.log4j.core.layout.PatternLayout;

/**
 * TL-4's log capture runner (docs/plans/telemetry-links.md): runs every
 * logging case through the real libraries (SLF4J bound to log4j2, the
 * log4j2 API, java.util.logging; jars installed by
 * calibration/incidents/install-log-deps.sh and put on the class path), and
 * writes one JSON line per case: the rendered log line as an operator would
 * paste it, and the truth, the call site the JVM's own stack reports
 * (StackWalker, inside the appender or handler, which run on the caller's
 * thread). The rendered line never carries the call site. Every case logs a
 * value carrying "SNTL", which no localize answer may echo.
 */
public final class LogCapture {
    private LogCapture() {
    }

    record Captured(String api, String line, Map<String, Object> truth) {
    }

    static final List<Captured> CAPTURED = new ArrayList<>();
    static String api = "";

    /** The innermost app frame outside this runner: the logger call. */
    static Map<String, Object> site() {
        StackWalker.StackFrame f = StackWalker.getInstance().walk(s -> s
                .filter(x -> x.getClassName().startsWith("com.acme.shop.") && !x.getClassName().startsWith("com.acme.shop.capture."))
                .findFirst().orElseThrow());
        String pkg = f.getClassName().substring(0, f.getClassName().lastIndexOf('.'));
        Map<String, Object> t = new LinkedHashMap<>();
        t.put("file", "src/main/java/" + pkg.replace('.', '/') + "/" + f.getFileName());
        t.put("line", f.getLineNumber());
        t.put("func", f.getMethodName());
        t.put("name", f.getMethodName());
        return t;
    }

    static final class CaptureAppender extends AbstractAppender {
        CaptureAppender(Layout<? extends Serializable> layout) {
            super("capture", null, layout, false, Property.EMPTY_ARRAY);
        }

        @Override
        public void append(LogEvent event) {
            String line = new String(getLayout().toByteArray(event), StandardCharsets.UTF_8).split("\n", 2)[0];
            CAPTURED.add(new Captured(api, line, site()));
        }
    }

    static final class CaptureHandler extends Handler {
        @Override
        public void publish(LogRecord record) {
            String line = getFormatter().format(record).split("\n", 2)[0];
            CAPTURED.add(new Captured("java.jul", line, site()));
        }

        @Override
        public void flush() {
        }

        @Override
        public void close() {
        }
    }

    static void configure() {
        LoggerContext ctx = (LoggerContext) LogManager.getContext(false);
        Configuration config = ctx.getConfiguration();
        PatternLayout layout = PatternLayout.newBuilder()
                .withPattern("%d{ISO8601} %-5level [%t] %c{1.} - %msg%n")
                .withConfiguration(config).build();
        Appender app = new CaptureAppender(layout);
        app.start();
        config.addAppender(app);
        LoggerConfig root = config.getRootLogger();
        for (String name : new ArrayList<>(root.getAppenders().keySet())) {
            root.removeAppender(name);
        }
        root.addAppender(app, Level.ALL, null);
        root.setLevel(Level.ALL);
        ctx.updateLoggers();

        System.setProperty("java.util.logging.SimpleFormatter.format", "%1$tFT%1$tT.%1$tL %4$s %3$s - %5$s%6$s%n");
        java.util.logging.Logger jul = java.util.logging.Logger.getLogger("");
        for (Handler h : jul.getHandlers()) {
            jul.removeHandler(h);
        }
        CaptureHandler h = new CaptureHandler();
        h.setFormatter(new SimpleFormatter());
        h.setLevel(java.util.logging.Level.ALL);
        jul.addHandler(h);
        jul.setLevel(java.util.logging.Level.ALL);
    }

    static String json(Object v) {
        if (v instanceof Number) {
            return v.toString();
        }
        if (v instanceof Map<?, ?> m) {
            StringBuilder b = new StringBuilder("{");
            for (Map.Entry<?, ?> e : m.entrySet()) {
                if (b.length() > 1) {
                    b.append(", ");
                }
                b.append(json(e.getKey().toString())).append(": ").append(json(e.getValue()));
            }
            return b.append('}').toString();
        }
        StringBuilder b = new StringBuilder("\"");
        for (char c : v.toString().toCharArray()) {
            switch (c) {
                case '"' -> b.append("\\\"");
                case '\\' -> b.append("\\\\");
                case '\n' -> b.append("\\n");
                case '\t' -> b.append("\\t");
                case '\r' -> b.append("\\r");
                default -> {
                    if (c < 0x20) {
                        b.append(String.format("\\u%04x", (int) c));
                    } else {
                        b.append(c);
                    }
                }
            }
        }
        return b.append('"').toString();
    }

    public static void main(String[] args) {
        configure();
        Gateway gw = new Gateway("SNTL-merchant-01");
        Warehouse wh = new Warehouse("SNTL-site-02");
        Map<String, Object[]> cases = new LinkedHashMap<>();
        cases.put("payments-authorize", new Object[] {"java.slf4j", (Runnable) () -> gw.authorize(1250, "4111111111111111")});
        cases.put("payments-refuse", new Object[] {"java.slf4j", (Runnable) () -> gw.authorize(-5, "SNTL0003")});
        cases.put("payments-decline", new Object[] {"java.slf4j", (Runnable) () -> gw.decline("SNTL-auth-04", "SNTL04")});
        cases.put("payments-timeout-string-format", new Object[] {"java.slf4j", (Runnable) () -> gw.timeout("SNTL-auth-05", 30)});
        cases.put("payments-void-throwable", new Object[] {"java.slf4j", (Runnable) () -> gw.voidAuthorization("SNTL-auth-06")});
        cases.put("payments-capture-concat", new Object[] {"java.slf4j", (Runnable) () -> gw.capture("SNTL-auth-07", 990)});
        cases.put("payments-chargeback-marker", new Object[] {"java.slf4j", (Runnable) () -> gw.chargeback("SNTL-pay-08", "fraud")});
        cases.put("refunds-request", new Object[] {"java.slf4j", (Runnable) () -> Refunds.request("SNTL-pay-09", 400, "damaged")});
        cases.put("refunds-reject", new Object[] {"java.slf4j", (Runnable) () -> Refunds.reject("SNTL-pay-10", 50000)});
        cases.put("refunds-settle-debug", new Object[] {"java.slf4j", (Runnable) () -> Refunds.settle("SNTL-pay-11", "B-7")});
        cases.put("refunds-flag-dynamic", new Object[] {"java.slf4j", (Runnable) () -> Refunds.flag("SNTL-pay-12")});
        cases.put("refunds-escaped-anchor", new Object[] {"java.slf4j", (Runnable) () -> Refunds.keepPlaceholder("SNTL-pay-13")});
        cases.put("warehouse-pick", new Object[] {"java.log4j2", (Runnable) () -> wh.pick("SNTL-sku-14", "A-4")});
        cases.put("warehouse-short-pick", new Object[] {"java.log4j2", (Runnable) () -> wh.shortPick("SNTL-sku-15", 6, 2)});
        cases.put("warehouse-restock-multiline", new Object[] {"java.log4j2", (Runnable) () -> wh.restock("SNTL-sku-16", 40)});
        cases.put("warehouse-quarantine-level", new Object[] {"java.log4j2", (Runnable) () -> wh.quarantine("SNTL-lot-17")});
        cases.put("warehouse-recount-debug", new Object[] {"java.log4j2", (Runnable) () -> wh.recount("SNTL-sku-18")});
        cases.put("warehouse-cycle-count", new Object[] {"java.log4j2", (Runnable) () -> wh.cycleCountFailed("SNTL-sku-19", 3)});
        cases.put("audit-login-failed", new Object[] {"java.jul", (Runnable) () -> AuditTrail.loginFailed("SNTL-user-20", 5)});
        cases.put("audit-role-granted", new Object[] {"java.jul", (Runnable) () -> AuditTrail.roleGranted("SNTL-user-21", "admin", "u-1")});
        cases.put("audit-export-denied", new Object[] {"java.jul", (Runnable) () -> AuditTrail.exportDenied("SNTL-user-22", "orders")});
        cases.put("audit-password-reset-concat", new Object[] {"java.jul", (Runnable) () -> AuditTrail.passwordReset("SNTL-user-23")});
        cases.put("audit-access-denied-quote", new Object[] {"java.jul", (Runnable) () -> AuditTrail.accessDenied("SNTL-user-24", "reports")});
        cases.put("audit-session-expired-fine", new Object[] {"java.jul", (Runnable) () -> AuditTrail.sessionExpired("SNTL-session-25")});
        StringBuilder out = new StringBuilder();
        for (Map.Entry<String, Object[]> c : cases.entrySet()) {
            api = (String) c.getValue()[0];
            CAPTURED.clear();
            ((Runnable) c.getValue()[1]).run();
            if (CAPTURED.isEmpty()) {
                System.err.println("case " + c.getKey() + " logged nothing");
                System.exit(1);
            }
            Captured last = CAPTURED.get(CAPTURED.size() - 1);
            Map<String, Object> row = new LinkedHashMap<>();
            row.put("case", c.getKey());
            row.put("lang", "java");
            row.put("api", last.api());
            row.put("truth", last.truth());
            row.put("payloads", Map.of("log", last.line()));
            out.append(json(row)).append('\n');
        }
        System.out.print(out);
        System.out.flush();
    }
}
