package com.acme.shop.accounts;

/** A user account. */
public class User {
    private String name;
    private final String email;

    public User(String name, String email) {
        this.name = name;
        this.email = email;
    }

    public void rename(String full) {
        int space = full.indexOf(' ');
        this.name = full.substring(0, space);
    }

    public String domain() {
        String[] parts = email.split("@");
        return parts[1].toLowerCase();
    }

    public String name() {
        return name;
    }
}
