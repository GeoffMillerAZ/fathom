package com.acme.shop.audit;

import java.util.logging.Level;
import java.util.logging.Logger;

/** Audit events (java.util.logging). */
public final class AuditTrail {
    private static final Logger LOGGER = Logger.getLogger(AuditTrail.class.getName());

    private AuditTrail() {
    }

    public static void loginFailed(String user, int attempts) {
        LOGGER.log(Level.WARNING, "login failed for user {0} after {1} attempts", new Object[] {user, attempts});
    }

    public static void roleGranted(String user, String role, String grantedBy) {
        LOGGER.log(Level.INFO, "role {0} granted to {1} by {2}", new Object[] {role, user, grantedBy});
    }

    public static void exportDenied(String user, String dataset) {
        LOGGER.log(Level.SEVERE, "export of {0} denied for {1}", new Object[] {dataset, user});
    }

    public static void passwordReset(String user) {
        LOGGER.info("password reset requested for " + user);
    }

    public static void accessDenied(String user, String resource) {
        LOGGER.log(Level.INFO, "user {0} can''t access {1}", new Object[] {user, resource});
    }

    public static void sessionExpired(String session) {
        LOGGER.log(Level.FINE, "session {0} expired after inactivity", session);
    }
}
