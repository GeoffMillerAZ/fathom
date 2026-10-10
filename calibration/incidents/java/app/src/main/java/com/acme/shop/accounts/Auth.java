package com.acme.shop.accounts;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;

/** Authentication: tokens and password hashing. */
public class Auth {
    private final String secret;

    public Auth(String secret) {
        this.secret = secret;
    }

    public void verify(String token) {
        if (!token.endsWith(secret)) {
            throw new SecurityException("token rejected");
        }
    }

    public byte[] hashPassword(String password, String algorithm) {
        try {
            MessageDigest md = MessageDigest.getInstance(algorithm);
            return md.digest(password.getBytes());
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalArgumentException("unsupported algorithm", e);
        }
    }
}
